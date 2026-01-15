package service

import (
	"context"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
)

type userNotificationPreferencesService struct {
	repo   repository.UserNotificationPreferencesRepository
	logger Logger
}

func NewUserNotificationPreferencesService(
	repo repository.UserNotificationPreferencesRepository,
	logger Logger,
) UserNotificationPreferencesService {
	return &userNotificationPreferencesService{
		repo:   repo,
		logger: logger,
	}
}

func (s *userNotificationPreferencesService) GetForUser(ctx context.Context, userID uint) (*domain.UserNotificationPreferences, error) {
	prefs, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		if err == repository.ErrNotFound {
			// Defaults for new users (not persisted until first update).
			return &domain.UserNotificationPreferences{
				UserID:              userID,
				CommunicationEmails: false,
				MarketingEmails:     false,
				SocialEmails:        false,
				SecurityEmails:      true,
			}, nil
		}
		return nil, err
	}

	// Enforce invariant
	prefs.SecurityEmails = true
	return prefs, nil
}

func (s *userNotificationPreferencesService) UpdateForUser(ctx context.Context, userID uint, req *domain.UpdateUserNotificationPreferencesRequest) (*domain.UserNotificationPreferences, error) {
	if req == nil {
		return nil, repository.ErrInvalidInput
	}

	current, err := s.GetForUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	if req.CommunicationEmails != nil {
		current.CommunicationEmails = *req.CommunicationEmails
	}
	if req.MarketingEmails != nil {
		current.MarketingEmails = *req.MarketingEmails
	}
	if req.SocialEmails != nil {
		current.SocialEmails = *req.SocialEmails
	}

	// Security emails are mandatory; ignore any client attempt to disable.
	current.SecurityEmails = true

	if err := s.repo.Upsert(ctx, current); err != nil {
		return nil, err
	}

	return current, nil
}

