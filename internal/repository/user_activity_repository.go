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
	// GetLastSignInTimes returns the newest stored sign-in time for each user that has one.
	// Users with no sign-in row are omitted. Callers must treat a missing entry as unknown.
	GetLastSignInTimes(ctx context.Context, userIDs []uint) (map[uint]time.Time, error)
	List(ctx context.Context, filter ActivityFilter) ([]*domain.UserActivity, int64, error)
	ListByUserIDs(ctx context.Context, userIDs []uint, limit int, offset int) ([]*domain.UserActivity, error)
	DeleteByUserID(ctx context.Context, userID uint) error
	DeleteOldActivities(ctx context.Context, olderThan time.Duration) (int64, error)
}

// ActivityFilter for filtering activities
type ActivityFilter struct {
	UserID       *uint
	ActivityType *domain.ActivityType
	Limit        int
	Offset       int
}
