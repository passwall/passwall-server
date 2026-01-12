package repository

import (
	"context"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
)

// UserActivityRepository defines the interface for user activity operations
type UserActivityRepository interface {
	Create(ctx context.Context, activity *domain.UserActivity) error
	GetByUserID(ctx context.Context, userID uint, limit int) ([]*domain.UserActivity, error)
	GetLastActivity(ctx context.Context, userID uint, activityType domain.ActivityType) (*domain.UserActivity, error)
	List(ctx context.Context, filter ActivityFilter) ([]*domain.UserActivity, int64, error)
	ListByUserIDs(ctx context.Context, userIDs []uint, limit int, offset int) ([]*domain.UserActivity, error)
	DeleteOldActivities(ctx context.Context, olderThan time.Duration) (int64, error)
}

// ActivityFilter for filtering activities
type ActivityFilter struct {
	UserID       *uint
	ActivityType *domain.ActivityType
	Limit        int
	Offset       int
}
