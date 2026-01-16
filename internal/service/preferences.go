package service

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
)

const (
	PreferenceOwnerUser         = "user"
	PreferenceOwnerOrganization = "organization"
)

var (
	prefSectionRe = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)
	prefKeyRe     = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)
)

type preferencesService struct {
	repo   repository.PreferencesRepository
	logger Logger
}

type PreferencesService interface {
	ListForUser(ctx context.Context, userID uint, section string) ([]*domain.Preference, error)
	UpsertForUser(ctx context.Context, userID uint, req *domain.UpsertPreferencesRequest) ([]*domain.Preference, error)
}

func NewPreferencesService(repo repository.PreferencesRepository, logger Logger) PreferencesService {
	return &preferencesService{repo: repo, logger: logger}
}

func (s *preferencesService) ListForUser(ctx context.Context, userID uint, section string) ([]*domain.Preference, error) {
	return s.repo.ListByOwner(ctx, PreferenceOwnerUser, userID, strings.ToLower(strings.TrimSpace(section)))
}

func (s *preferencesService) UpsertForUser(ctx context.Context, userID uint, req *domain.UpsertPreferencesRequest) ([]*domain.Preference, error) {
	if req == nil {
		return nil, repository.ErrInvalidInput
	}

	prefs := make([]*domain.Preference, 0, len(req.Preferences))
	for _, in := range req.Preferences {
		section := strings.ToLower(strings.TrimSpace(in.Section))
		key := strings.ToLower(strings.TrimSpace(in.Key))
		typ := strings.ToLower(strings.TrimSpace(in.Type))
		val := in.Value

		if !prefSectionRe.MatchString(section) || !prefKeyRe.MatchString(key) {
			return nil, repository.ErrInvalidInput
		}
		if typ == "" {
			typ = "string"
		}
		if !isValidPreferenceType(typ) {
			return nil, repository.ErrInvalidInput
		}
		if err := validatePreferenceValue(typ, val); err != nil {
			return nil, repository.ErrInvalidInput
		}

		prefs = append(prefs, &domain.Preference{
			OwnerType: PreferenceOwnerUser,
			OwnerID:   userID,
			Section:   section,
			Key:       key,
			Type:      typ,
			Value:     val,
		})
	}

	if err := s.repo.UpsertMany(ctx, prefs); err != nil {
		return nil, err
	}

	// Return the updated snapshot for the involved sections (best-effort).
	// If multiple sections were updated, caller can refetch filtered by section.
	return s.repo.ListByOwner(ctx, PreferenceOwnerUser, userID, "")
}

func isValidPreferenceType(t string) bool {
	switch t {
	case "string", "number", "boolean", "json":
		return true
	default:
		return false
	}
}

func validatePreferenceValue(t string, v string) error {
	switch t {
	case "string":
		// Accept any string; keep a reasonable bound to avoid abuse.
		if len(v) > 64*1024 {
			return repository.ErrInvalidInput
		}
		return nil
	case "boolean":
		if v != "true" && v != "false" {
			return repository.ErrInvalidInput
		}
		return nil
	case "number":
		// JSON number parsing is stricter than float parsing and matches what we'd store.
		var n json.Number
		if err := json.Unmarshal([]byte(v), &n); err == nil {
			_, err2 := n.Float64()
			return err2
		}
		// Fallback: allow plain numeric strings without JSON wrapping.
		var f float64
		return json.Unmarshal([]byte(strings.TrimSpace(v)), &f)
	case "json":
		var tmp any
		return json.Unmarshal([]byte(v), &tmp)
	default:
		return repository.ErrInvalidInput
	}
}
