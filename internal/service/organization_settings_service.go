package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
)

// OrganizationSettingsService defines business logic for organization settings
type OrganizationSettingsService interface {
	ListByOrganization(ctx context.Context, orgID, userID uint, section string) ([]*domain.PreferenceDTO, error)
	UpsertForOrganization(ctx context.Context, orgID, userID uint, req *domain.UpsertPreferencesRequest) ([]*domain.PreferenceDTO, error)
	GetSettingsDefinitions() []domain.OrgSettingsDefinition
}

type organizationSettingsService struct {
	prefRepo    repository.PreferencesRepository
	orgUserRepo repository.OrganizationUserRepository
	logger      Logger
}

// NewOrganizationSettingsService creates a new organization settings service
func NewOrganizationSettingsService(
	prefRepo repository.PreferencesRepository,
	orgUserRepo repository.OrganizationUserRepository,
	logger Logger,
) OrganizationSettingsService {
	return &organizationSettingsService{
		prefRepo:    prefRepo,
		orgUserRepo: orgUserRepo,
		logger:      logger,
	}
}

func (s *organizationSettingsService) ListByOrganization(ctx context.Context, orgID, userID uint, section string) ([]*domain.PreferenceDTO, error) {
	if err := s.requireSettingsAccess(ctx, orgID, userID); err != nil {
		return nil, err
	}

	prefs, err := s.prefRepo.ListByOwner(ctx, domain.OrgSettingOwnerType, orgID, strings.ToLower(strings.TrimSpace(section)))
	if err != nil {
		return nil, fmt.Errorf("failed to list organization settings: %w", err)
	}

	dtos := make([]*domain.PreferenceDTO, 0, len(prefs))
	for _, p := range prefs {
		dtos = append(dtos, domain.ToPreferenceDTO(p))
	}
	return dtos, nil
}

func (s *organizationSettingsService) UpsertForOrganization(ctx context.Context, orgID, userID uint, req *domain.UpsertPreferencesRequest) ([]*domain.PreferenceDTO, error) {
	if err := s.requireSettingsAdmin(ctx, orgID, userID); err != nil {
		return nil, err
	}

	if req == nil || len(req.Preferences) == 0 {
		return nil, repository.ErrInvalidInput
	}

	allowedKeys := buildAllowedKeysMap()

	prefs := make([]*domain.Preference, 0, len(req.Preferences))
	for _, in := range req.Preferences {
		section := strings.ToLower(strings.TrimSpace(in.Section))
		key := strings.ToLower(strings.TrimSpace(in.Key))
		typ := strings.ToLower(strings.TrimSpace(in.Type))
		val := in.Value

		lookupKey := section + "." + key
		if _, ok := allowedKeys[lookupKey]; !ok {
			return nil, fmt.Errorf("unknown organization setting: %s.%s", section, key)
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
			OwnerType: domain.OrgSettingOwnerType,
			OwnerID:   orgID,
			Section:   section,
			Key:       key,
			Type:      typ,
			Value:     val,
		})
	}

	if err := s.prefRepo.UpsertMany(ctx, prefs); err != nil {
		s.logger.Error("failed to upsert organization settings", "org_id", orgID, "error", err)
		return nil, fmt.Errorf("failed to update organization settings: %w", err)
	}

	s.logger.Info("organization settings updated", "org_id", orgID, "user_id", userID, "count", len(prefs))

	updated, err := s.prefRepo.ListByOwner(ctx, domain.OrgSettingOwnerType, orgID, "")
	if err != nil {
		return nil, fmt.Errorf("failed to reload settings: %w", err)
	}

	dtos := make([]*domain.PreferenceDTO, 0, len(updated))
	for _, p := range updated {
		dtos = append(dtos, domain.ToPreferenceDTO(p))
	}
	return dtos, nil
}

func (s *organizationSettingsService) GetSettingsDefinitions() []domain.OrgSettingsDefinition {
	return domain.AllOrgSettingsDefinitions()
}

// --- Helpers ---

func (s *organizationSettingsService) requireSettingsAccess(ctx context.Context, orgID, userID uint) error {
	orgUser, err := s.orgUserRepo.GetByOrgAndUser(ctx, orgID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return repository.ErrForbidden
		}
		return err
	}
	// Owner, Admin, Manager can view settings
	if orgUser.Role != domain.OrgRoleOwner &&
		orgUser.Role != domain.OrgRoleAdmin &&
		orgUser.Role != domain.OrgRoleManager {
		return repository.ErrForbidden
	}
	return nil
}

func (s *organizationSettingsService) requireSettingsAdmin(ctx context.Context, orgID, userID uint) error {
	orgUser, err := s.orgUserRepo.GetByOrgAndUser(ctx, orgID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return repository.ErrForbidden
		}
		return err
	}
	if !orgUser.IsAdmin() {
		return repository.ErrForbidden
	}
	return nil
}

func buildAllowedKeysMap() map[string]struct{} {
	defs := domain.AllOrgSettingsDefinitions()
	m := make(map[string]struct{}, len(defs))
	for _, d := range defs {
		m[d.Section+"."+d.Key] = struct{}{}
	}
	return m
}
