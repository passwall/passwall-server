package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/passwall/passwall-server/internal/domain"
)

// FeatureService handles feature gating based on subscription plans
type FeatureService interface {
	CanCreateCollection(ctx context.Context, orgID uint) (bool, error)
	CanInviteUser(ctx context.Context, orgID uint) (bool, error)
	CanCreateItem(ctx context.Context, orgID uint) (bool, error)
	CanReadVault(ctx context.Context, orgID uint) (bool, error)
	CanWriteVault(ctx context.Context, orgID uint) (bool, error)
	CanDeleteVault(ctx context.Context, orgID uint) (bool, error)
	CanAutofill(ctx context.Context, orgID uint) (bool, error)
	CanUseTeams(ctx context.Context, orgID uint) (bool, error)
	CanAccessAudit(ctx context.Context, orgID uint) (bool, error)
	CanUseSSO(ctx context.Context, orgID uint) (bool, error)
	CanUseBreachMonitoring(ctx context.Context, orgID uint) (bool, error)
	CanUsePasskeys(ctx context.Context, orgID uint) (bool, error)
	CanUseSharedItems(ctx context.Context, orgID uint) (bool, error)
	CanUseSecureSend(ctx context.Context, orgID uint) (bool, error)
	CanUseEmergencyAccess(ctx context.Context, orgID uint) (bool, error)
	GetFeatures(ctx context.Context, orgID uint) (*domain.PlanFeatures, error)
}

type featureService struct {
	entitlements OrganizationEntitlementService
}

// NewFeatureService creates a new feature service
func NewFeatureService(entitlements OrganizationEntitlementService) FeatureService {
	return &featureService{entitlements: entitlements}
}

var (
	ErrSubscriptionExpired = errors.New("subscription has expired")
	ErrPlanLimitReached    = errors.New("plan limit reached")
	ErrFeatureNotAvailable = errors.New("feature not available in current plan")
)

func (s *featureService) authorize(
	ctx context.Context,
	orgID uint,
	capability domain.Capability,
) (bool, error) {
	if err := s.entitlements.Authorize(ctx, orgID, capability); err != nil {
		return false, err
	}
	return true, nil
}

func (s *featureService) CanWriteVault(ctx context.Context, orgID uint) (bool, error) {
	return s.authorize(ctx, orgID, domain.CapabilityItemUpdate)
}

func (s *featureService) CanDeleteVault(ctx context.Context, orgID uint) (bool, error) {
	return s.authorize(ctx, orgID, domain.CapabilityItemDelete)
}

func (s *featureService) CanReadVault(ctx context.Context, orgID uint) (bool, error) {
	return s.authorize(ctx, orgID, domain.CapabilityVaultRead)
}

func (s *featureService) CanAutofill(ctx context.Context, orgID uint) (bool, error) {
	return s.authorize(ctx, orgID, domain.CapabilityVaultAutofill)
}

// CanInviteUser checks if organization can invite new users
func (s *featureService) CanInviteUser(ctx context.Context, orgID uint) (bool, error) {
	return s.authorize(ctx, orgID, domain.CapabilityMemberInvite)
}

// CanCreateCollection checks if organization can create new collections
func (s *featureService) CanCreateCollection(ctx context.Context, orgID uint) (bool, error) {
	return s.authorize(ctx, orgID, domain.CapabilityCollectionCreate)
}

// CanCreateItem checks if organization can create new items
func (s *featureService) CanCreateItem(ctx context.Context, orgID uint) (bool, error) {
	return s.authorize(ctx, orgID, domain.CapabilityItemCreate)
}

// CanUseTeams checks if organization can use teams feature
func (s *featureService) CanUseTeams(ctx context.Context, orgID uint) (bool, error) {
	return s.authorize(ctx, orgID, domain.CapabilityTeamsManage)
}

// CanAccessAudit checks if organization can access audit logs
func (s *featureService) CanAccessAudit(ctx context.Context, orgID uint) (bool, error) {
	return s.authorize(ctx, orgID, domain.CapabilityAuditRead)
}

// CanUseSSO checks if organization can use SSO
func (s *featureService) CanUseSSO(ctx context.Context, orgID uint) (bool, error) {
	return s.authorize(ctx, orgID, domain.CapabilitySSOManage)
}

// CanUseBreachMonitoring checks if organization can use dark web / breach monitoring
func (s *featureService) CanUseBreachMonitoring(ctx context.Context, orgID uint) (bool, error) {
	return s.authorize(ctx, orgID, domain.CapabilityBreachMonitoringRead)
}

// CanUsePasskeys checks if organization can use passkeys feature
func (s *featureService) CanUsePasskeys(ctx context.Context, orgID uint) (bool, error) {
	return s.authorize(ctx, orgID, domain.CapabilityPasskeyCreate)
}

// CanUseSharedItems checks if organization can use shared items feature
func (s *featureService) CanUseSharedItems(ctx context.Context, orgID uint) (bool, error) {
	return s.authorize(ctx, orgID, domain.CapabilitySharingCreate)
}

// CanUseSecureSend checks if organization can use secure send feature
func (s *featureService) CanUseSecureSend(ctx context.Context, orgID uint) (bool, error) {
	return s.authorize(ctx, orgID, domain.CapabilitySecureSendCreate)
}

// CanUseEmergencyAccess checks if organization can use emergency access feature
func (s *featureService) CanUseEmergencyAccess(ctx context.Context, orgID uint) (bool, error) {
	return s.authorize(ctx, orgID, domain.CapabilityEmergencyAccessCreate)
}

// GetFeatures returns all features available to an organization
func (s *featureService) GetFeatures(ctx context.Context, orgID uint) (*domain.PlanFeatures, error) {
	snapshot, err := s.entitlements.Resolve(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("resolve entitlements: %w", err)
	}
	features := snapshot.Features
	return &features, nil
}
