package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
)

// UserActivityService defines the business logic for user activities
type UserActivityService interface {
	LogActivity(ctx context.Context, req *domain.CreateActivityRequest) error
	GetUserActivities(ctx context.Context, userID uint, limit int) ([]*domain.UserActivity, error)
	GetLastSignIn(ctx context.Context, userID uint) (*domain.UserActivity, error)
	ListActivities(ctx context.Context, filter repository.ActivityFilter) ([]*domain.UserActivity, int64, error)
	ListActivitiesByUserIDs(ctx context.Context, userIDs []uint, limit int, offset int) ([]*domain.UserActivity, error)
	CleanupOldActivities(ctx context.Context, olderThan time.Duration) (int64, error)
}

type userActivityService struct {
	repo   repository.UserActivityRepository
	logger Logger
}

// NewUserActivityService creates a new user activity service
func NewUserActivityService(repo repository.UserActivityRepository, logger Logger) UserActivityService {
	return &userActivityService{
		repo:   repo,
		logger: logger,
	}
}

func (s *userActivityService) LogActivity(ctx context.Context, req *domain.CreateActivityRequest) error {
	activity := &domain.UserActivity{
		UserID:       req.UserID,
		ActivityType: req.ActivityType,
		IPAddress:    req.IPAddress,
		UserAgent:    req.UserAgent,
		Details:      req.Details,
		CreatedAt:    time.Now(),
	}

	if err := s.repo.Create(ctx, activity); err != nil {
		s.logger.Error("failed to log activity",
			"user_id", req.UserID,
			"type", req.ActivityType,
			"error", err)
		// Don't return error - activity logging is non-critical
		return nil
	}

	s.logger.Debug("activity logged",
		"user_id", req.UserID,
		"type", req.ActivityType,
		"ip", req.IPAddress)

	return nil
}

func (s *userActivityService) GetUserActivities(ctx context.Context, userID uint, limit int) ([]*domain.UserActivity, error) {
	return s.repo.GetByUserID(ctx, userID, limit)
}

func (s *userActivityService) GetLastSignIn(ctx context.Context, userID uint) (*domain.UserActivity, error) {
	return s.repo.GetLastActivity(ctx, userID, domain.ActivityTypeSignIn)
}

func (s *userActivityService) ListActivities(ctx context.Context, filter repository.ActivityFilter) ([]*domain.UserActivity, int64, error) {
	return s.repo.List(ctx, filter)
}

func (s *userActivityService) ListActivitiesByUserIDs(ctx context.Context, userIDs []uint, limit int, offset int) ([]*domain.UserActivity, error) {
	return s.repo.ListByUserIDs(ctx, userIDs, limit, offset)
}

func (s *userActivityService) CleanupOldActivities(ctx context.Context, olderThan time.Duration) (int64, error) {
	count, err := s.repo.DeleteOldActivities(ctx, olderThan)
	if err != nil {
		s.logger.Error("failed to cleanup old activities", "error", err)
		return 0, err
	}

	if count > 0 {
		s.logger.Info("cleaned up old activities", "count", count)
	}

	return count, nil
}

// Helper function to create activity details as JSON
func CreateActivityDetails(data map[string]interface{}) string {
	if data == nil {
		return ""
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return ""
	}

	return string(bytes)
}
