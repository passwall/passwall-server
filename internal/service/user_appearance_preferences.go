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
	repo   repository.UserAppearancePreferencesRepository
	logger Logger
}

func NewUserAppearancePreferencesService(
	repo repository.UserAppearancePreferencesRepository,
	logger Logger,
) UserAppearancePreferencesService {
	return &userAppearancePreferencesService{
		repo:   repo,
		logger: logger,
	}
}

func (s *userAppearancePreferencesService) GetForUser(ctx context.Context, userID uint) (*domain.UserAppearancePreferences, error) {
	prefs, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		if err == repository.ErrNotFound {
			// Defaults for new users (not persisted until first update).
			return &domain.UserAppearancePreferences{
				UserID: userID,
				Theme:  defaultAppearanceTheme,
				Font:   defaultAppearanceFont,
			}, nil
		}
		return nil, err
	}
	return prefs, nil
}

func (s *userAppearancePreferencesService) UpdateForUser(ctx context.Context, userID uint, req *domain.UpdateUserAppearancePreferencesRequest) (*domain.UserAppearancePreferences, error) {
	if req == nil {
		return nil, repository.ErrInvalidInput
	}

	current, err := s.GetForUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	if req.Theme != nil {
		theme := strings.TrimSpace(strings.ToLower(*req.Theme))
		if !isValidTheme(theme) {
			return nil, repository.ErrInvalidInput
		}
		current.Theme = theme
	}

	if req.Font != nil {
		font := strings.TrimSpace(strings.ToLower(*req.Font))
		if !appearanceFontRe.MatchString(font) {
			return nil, repository.ErrInvalidInput
		}
		current.Font = font
	}

	if err := s.repo.Upsert(ctx, current); err != nil {
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

