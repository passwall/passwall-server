package service

import (
	"context"
	"regexp"
	"strings"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
)

const (
	defaultAppearanceTheme = "dark"
	defaultAppearanceFont  = "inter"
)

var appearanceFontRe = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)

type userAppearancePreferencesService struct {
	prefs  repository.PreferencesRepository
	logger Logger
}

func NewUserAppearancePreferencesService(
	prefs repository.PreferencesRepository,
	logger Logger,
) UserAppearancePreferencesService {
	return &userAppearancePreferencesService{
		prefs:  prefs,
		logger: logger,
	}
}

func (s *userAppearancePreferencesService) GetForUser(ctx context.Context, userID uint) (*domain.UserAppearancePreferences, error) {
	out := &domain.UserAppearancePreferences{
		UserID: userID,
		Theme:  defaultAppearanceTheme,
		Font:   defaultAppearanceFont,
	}

	rows, err := s.prefs.ListByOwner(ctx, PreferenceOwnerUser, userID, "appearance")
	if err != nil {
		return nil, err
	}
	for _, p := range rows {
		if p == nil || p.Type != "string" {
			continue
		}
		switch p.Key {
		case "theme":
			if isValidTheme(p.Value) {
				out.Theme = p.Value
			}
		case "font":
			if appearanceFontRe.MatchString(p.Value) {
				out.Font = p.Value
			}
		}
	}

	return out, nil
}

func (s *userAppearancePreferencesService) UpdateForUser(ctx context.Context, userID uint, req *domain.UpdateUserAppearancePreferencesRequest) (*domain.UserAppearancePreferences, error) {
	if req == nil {
		return nil, repository.ErrInvalidInput
	}

	current, err := s.GetForUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	updates := make([]*domain.Preference, 0, 2)

	if req.Theme != nil {
		theme := strings.TrimSpace(strings.ToLower(*req.Theme))
		if !isValidTheme(theme) {
			return nil, repository.ErrInvalidInput
		}
		current.Theme = theme
		updates = append(updates, &domain.Preference{
			OwnerType: PreferenceOwnerUser,
			OwnerID:   userID,
			Section:   "appearance",
			Key:       "theme",
			Type:      "string",
			Value:     theme,
		})
	}

	if req.Font != nil {
		font := strings.TrimSpace(strings.ToLower(*req.Font))
		if !appearanceFontRe.MatchString(font) {
			return nil, repository.ErrInvalidInput
		}
		current.Font = font
		updates = append(updates, &domain.Preference{
			OwnerType: PreferenceOwnerUser,
			OwnerID:   userID,
			Section:   "appearance",
			Key:       "font",
			Type:      "string",
			Value:     font,
		})
	}

	if err := s.prefs.UpsertMany(ctx, updates); err != nil {
		return nil, err
	}

	return current, nil
}

func isValidTheme(theme string) bool {
	switch theme {
	case "dark", "light", "system":
		return true
	default:
		return false
	}
}
