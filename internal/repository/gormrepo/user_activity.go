package gormrepo

import (
	"context"
	"errors"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"gorm.io/gorm"
)

type userActivityRepository struct {
	db *gorm.DB
}

// NewUserActivityRepository creates a new user activity repository
func NewUserActivityRepository(db *gorm.DB) repository.UserActivityRepository {
	return &userActivityRepository{db: db}
}

func (r *userActivityRepository) Create(ctx context.Context, activity *domain.UserActivity) error {
	return r.db.WithContext(ctx).Create(activity).Error
}

func (r *userActivityRepository) GetByUserID(ctx context.Context, userID uint, limit int) ([]*domain.UserActivity, error) {
	var activities []*domain.UserActivity

	query := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&activities).Error; err != nil {
		return nil, err
	}

	return activities, nil
}

func (r *userActivityRepository) GetLastActivity(ctx context.Context, userID uint, activityType domain.ActivityType) (*domain.UserActivity, error) {
	var activity domain.UserActivity

	err := r.db.WithContext(ctx).
		Where("user_id = ? AND activity_type = ?", userID, activityType).
		Order("created_at DESC").
		First(&activity).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}

	return &activity, nil
}

func (r *userActivityRepository) List(ctx context.Context, filter repository.ActivityFilter) ([]*domain.UserActivity, int64, error) {
	var activities []*domain.UserActivity
	var total int64

	query := r.db.WithContext(ctx).Model(&domain.UserActivity{})

	// Apply filters
	if filter.UserID != nil {
		query = query.Where("user_id = ?", *filter.UserID)
	}

	if filter.ActivityType != nil {
		query = query.Where("activity_type = ?", *filter.ActivityType)
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	// Order by newest first
	query = query.Order("created_at DESC")

	if err := query.Find(&activities).Error; err != nil {
		return nil, 0, err
	}

	return activities, total, nil
}

func (r *userActivityRepository) DeleteOldActivities(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoffTime := time.Now().Add(-olderThan)

	result := r.db.WithContext(ctx).
		Where("created_at < ?", cutoffTime).
		Delete(&domain.UserActivity{})

	if result.Error != nil {
		return 0, result.Error
	}

	return result.RowsAffected, nil
}
