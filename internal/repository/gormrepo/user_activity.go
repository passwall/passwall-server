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
	return dbFromContext(ctx, r.db).Create(activity).Error
}

func (r *userActivityRepository) GetByUserID(ctx context.Context, userID uint, limit int) ([]*domain.UserActivity, error) {
	var activities []*domain.UserActivity

	query := dbFromContext(ctx, r.db).
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

	err := dbFromContext(ctx, r.db).
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

// GetLastSignInTimes returns the newest sign-in timestamp stored for each requested user.
// Only activity_type "signin" is considered. An empty input returns an empty map.
func (r *userActivityRepository) GetLastSignInTimes(ctx context.Context, userIDs []uint) (map[uint]time.Time, error) {
	out := make(map[uint]time.Time)
	if len(userIDs) == 0 {
		return out, nil
	}

	type row struct {
		UserID      uint      `gorm:"column:user_id"`
		LastLoginAt time.Time `gorm:"column:last_login_at"`
	}
	var rows []row
	err := dbFromContext(ctx, r.db).
		Model(&domain.UserActivity{}).
		Select("user_id, MAX(created_at) AS last_login_at").
		Where("user_id IN ? AND activity_type = ?", userIDs, domain.ActivityTypeSignIn).
		Group("user_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.UserID == 0 || row.LastLoginAt.IsZero() {
			continue
		}
		out[row.UserID] = row.LastLoginAt
	}
	return out, nil
}

func (r *userActivityRepository) List(ctx context.Context, filter repository.ActivityFilter) ([]*domain.UserActivity, int64, error) {
	var activities []*domain.UserActivity
	var total int64

	query := dbFromContext(ctx, r.db).Model(&domain.UserActivity{})

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

func (r *userActivityRepository) ListByUserIDs(ctx context.Context, userIDs []uint, limit int, offset int) ([]*domain.UserActivity, error) {
	var activities []*domain.UserActivity

	if len(userIDs) == 0 {
		return activities, nil
	}

	query := dbFromContext(ctx, r.db).
		Where("user_id IN ?", userIDs).
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&activities).Error; err != nil {
		return nil, err
	}

	return activities, nil
}

func (r *userActivityRepository) DeleteByUserID(ctx context.Context, userID uint) error {
	return dbFromContext(ctx, r.db).
		Where("user_id = ?", userID).
		Delete(&domain.UserActivity{}).Error
}

func (r *userActivityRepository) DeleteOldActivities(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoffTime := time.Now().Add(-olderThan)

	result := dbFromContext(ctx, r.db).
		Where("created_at < ?", cutoffTime).
		Delete(&domain.UserActivity{})

	if result.Error != nil {
		return 0, result.Error
	}

	return result.RowsAffected, nil
}
