package service

import (
	"context"
	"fmt"

	"github.com/passwall/passwall-server/internal/domain"
)

// PermissionService handles authorization and permission checks
type PermissionService interface {
	Can(ctx context.Context, userID uint, orgID uint, permission string) (bool, error)
	GetEffectiveRole(ctx context.Context, userID uint, orgID uint) (domain.OrganizationRole, error)
	CheckSubscriptionOverride(ctx context.Context, orgID uint) (bool, error)
	GetUserPermissions(ctx context.Context, userID uint, orgID uint) ([]string, error)
}

type permissionService struct {
	orgRepo OrganizationService
	subRepo interface {
		GetByOrganizationID(ctx context.Context, orgID uint) (*domain.Subscription, error)
	}
}

// NewPermissionService creates a new permission service
func NewPermissionService(
	orgService OrganizationService,
	subRepo interface {
		GetByOrganizationID(ctx context.Context, orgID uint) (*domain.Subscription, error)
	},
) PermissionService {
	return &permissionService{
		orgRepo: orgService,
		subRepo: subRepo,
	}
}

// GetEffectiveRole returns the effective role considering subscription state
func (s *permissionService) GetEffectiveRole(ctx context.Context, userID uint, orgID uint) (domain.OrganizationRole, error) {
	// Get membership
	membership, err := s.orgRepo.GetMembership(ctx, userID, orgID)
	if err != nil {
		return "", fmt.Errorf("failed to get membership: %w", err)
	}

	if membership == nil {
		return "", fmt.Errorf("user is not a member of organization")
	}

	// Check subscription override
	shouldOverride, err := s.CheckSubscriptionOverride(ctx, orgID)
	if err != nil {
		return "", fmt.Errorf("failed to check subscription: %w", err)
	}

	// If subscription is expired, override to read-only
	if shouldOverride {
		return "read_only", nil
	}

	return membership.Role, nil
}

// Can checks if a user has a specific permission in an organization
func (s *permissionService) Can(ctx context.Context, userID uint, orgID uint, permission string) (bool, error) {
	// Get effective role (with subscription override)
	role, err := s.GetEffectiveRole(ctx, userID, orgID)
	if err != nil {
		return false, err
	}

	// Check permission matrix
	return domain.Can(role, permission), nil
}

// CheckSubscriptionOverride checks if subscription state requires read-only override
func (s *permissionService) CheckSubscriptionOverride(ctx context.Context, orgID uint) (bool, error) {
	// Get subscription
	sub, err := s.subRepo.GetByOrganizationID(ctx, orgID)
	if err != nil {
		// If no subscription found, treat as free/expired
		return true, nil
	}

	// Check if subscription allows write operations
	if !sub.CanWrite() {
		return true, nil
	}

	return false, nil
}

// GetUserPermissions returns all permissions for a user in an organization
func (s *permissionService) GetUserPermissions(ctx context.Context, userID uint, orgID uint) ([]string, error) {
	// Get effective role (with subscription override)
	role, err := s.GetEffectiveRole(ctx, userID, orgID)
	if err != nil {
		return nil, err
	}

	return domain.GetPermissions(role), nil
}
