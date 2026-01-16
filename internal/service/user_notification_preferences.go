package service

import (
	"context"
	"strconv"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
)

type userNotificationPreferencesService struct {
	prefs  repository.PreferencesRepository
	logger Logger
}

func NewUserNotificationPreferencesService(
	prefs repository.PreferencesRepository,
	logger Logger,
) UserNotificationPreferencesService {
	return &userNotificationPreferencesService{
		prefs:  prefs,
		logger: logger,
	}
}

func (s *userNotificationPreferencesService) GetForUser(ctx context.Context, userID uint) (*domain.UserNotificationPreferences, error) {
	// Defaults for new users (not persisted until first update).
	out := &domain.UserNotificationPreferences{
		UserID:              userID,
		CommunicationEmails: false,
		MarketingEmails:     false,
		SocialEmails:        false,
		SecurityEmails:      true,
	}

	rows, err := s.prefs.ListByOwner(ctx, PreferenceOwnerUser, userID, "notifications")
	if err != nil {
		return nil, err
	}

	for _, p := range rows {
		if p == nil || p.Type != "boolean" {
			continue
		}
		v, err := strconv.ParseBool(p.Value)
		if err != nil {
			continue
		}

		switch p.Key {
		case "communication_emails":
			out.CommunicationEmails = v
		case "marketing_emails":
			out.MarketingEmails = v
		case "social_emails":
			out.SocialEmails = v
		}
	}

	// Enforce invariant
	out.SecurityEmails = true
	return out, nil
}

func (s *userNotificationPreferencesService) UpdateForUser(ctx context.Context, userID uint, req *domain.UpdateUserNotificationPreferencesRequest) (*domain.UserNotificationPreferences, error) {
	if req == nil {
		return nil, repository.ErrInvalidInput
	}

	current, err := s.GetForUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	updates := make([]*domain.Preference, 0, 4)

	if req.CommunicationEmails != nil {
		current.CommunicationEmails = *req.CommunicationEmails
		updates = append(updates, &domain.Preference{
			OwnerType: PreferenceOwnerUser,
			OwnerID:   userID,
			Section:   "notifications",
			Key:       "communication_emails",
			Type:      "boolean",
			Value:     strconv.FormatBool(*req.CommunicationEmails),
		})
	}
	if req.MarketingEmails != nil {
		current.MarketingEmails = *req.MarketingEmails
		updates = append(updates, &domain.Preference{
			OwnerType: PreferenceOwnerUser,
			OwnerID:   userID,
			Section:   "notifications",
			Key:       "marketing_emails",
			Type:      "boolean",
			Value:     strconv.FormatBool(*req.MarketingEmails),
		})
	}
	if req.SocialEmails != nil {
		current.SocialEmails = *req.SocialEmails
		updates = append(updates, &domain.Preference{
			OwnerType: PreferenceOwnerUser,
			OwnerID:   userID,
			Section:   "notifications",
			Key:       "social_emails",
			Type:      "boolean",
			Value:     strconv.FormatBool(*req.SocialEmails),
		})
	}

	// Security emails are mandatory; ignore any client attempt to disable.
	current.SecurityEmails = true

	if err := s.prefs.UpsertMany(ctx, updates); err != nil {
		return nil, err
	}

	return current, nil
}
