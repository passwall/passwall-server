package gormrepo

import (
	"context"
	"errors"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type userNotificationPreferencesRepository struct {
	db *gorm.DB
}

func NewUserNotificationPreferencesRepository(db *gorm.DB) repository.UserNotificationPreferencesRepository {
	return &userNotificationPreferencesRepository{db: db}
}

func (r *userNotificationPreferencesRepository) GetByUserID(ctx context.Context, userID uint) (*domain.UserNotificationPreferences, error) {
	var prefs domain.UserNotificationPreferences
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&prefs).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &prefs, nil
}

func (r *userNotificationPreferencesRepository) Upsert(ctx context.Context, prefs *domain.UserNotificationPreferences) error {
	if prefs == nil {
		return repository.ErrInvalidInput
	}

	// Ensure UpdatedAt changes on updates.
	prefs.UpdatedAt = time.Now()

	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"communication_emails",
			"marketing_emails",
			"social_emails",
			"security_emails",
			"updated_at",
		}),
	}).Create(prefs).Error
}

