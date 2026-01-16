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

type userAppearancePreferencesRepository struct {
	db *gorm.DB
}

func NewUserAppearancePreferencesRepository(db *gorm.DB) repository.UserAppearancePreferencesRepository {
	return &userAppearancePreferencesRepository{db: db}
}

func (r *userAppearancePreferencesRepository) GetByUserID(ctx context.Context, userID uint) (*domain.UserAppearancePreferences, error) {
	var prefs domain.UserAppearancePreferences
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&prefs).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &prefs, nil
}

func (r *userAppearancePreferencesRepository) Upsert(ctx context.Context, prefs *domain.UserAppearancePreferences) error {
	if prefs == nil {
		return repository.ErrInvalidInput
	}

	// Ensure UpdatedAt changes on updates.
	prefs.UpdatedAt = time.Now()

	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"theme",
			"font",
			"updated_at",
		}),
	}).Create(prefs).Error
}
