package service

import (
	"context"
	"fmt"

	"github.com/passwall/passwall-server/internal/domain"
)

// FeatureService handles feature gating based on subscription plans
type FeatureService interface {
	CanCreateCollection(ctx context.Context, orgID uint) (bool, error)
	CanInviteUser(ctx context.Context, orgID uint) (bool, error)
	CanCreateItem(ctx context.Context, orgID uint) (bool, error)
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
	orgService interface {
		GetByID(ctx context.Context, id uint, userID uint) (*domain.Organization, error)
		GetMemberCount(ctx context.Context, orgID uint) (int, error)
		GetCollectionCount(ctx context.Context, orgID uint) (int, error)
	}
	subRepo interface {
		GetByOrganizationID(ctx context.Context, orgID uint) (*domain.Subscription, error)
	}
	itemRepo interface {
		CountByOrganizationID(ctx context.Context, orgID uint) (int, error)
	}
}

// NewFeatureService creates a new feature service
func NewFeatureService(
	orgService interface {
		GetByID(ctx context.Context, id uint, userID uint) (*domain.Organization, error)
		GetMemberCount(ctx context.Context, orgID uint) (int, error)
		GetCollectionCount(ctx context.Context, orgID uint) (int, error)
	},
	subRepo interface {
		GetByOrganizationID(ctx context.Context, orgID uint) (*domain.Subscription, error)
	},
	itemRepo interface {
		CountByOrganizationID(ctx context.Context, orgID uint) (int, error)
	},
) FeatureService {
	return &featureService{
		orgService: orgService,
		subRepo:    subRepo,
		itemRepo:   itemRepo,
	}
}

var (
	ErrSubscriptionExpired = fmt.Errorf("subscription has expired")
	ErrPlanLimitReached    = fmt.Errorf("plan limit reached")
	ErrFeatureNotAvailable = fmt.Errorf("feature not available in current plan")
)

// getSubscriptionWithPlan retrieves subscription with plan for an organization
func (s *featureService) getSubscriptionWithPlan(ctx context.Context, orgID uint) (*domain.Subscription, error) {
	sub, err := s.subRepo.GetByOrganizationID(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}

	// Check if subscription allows write operations
	if !sub.CanWrite() {
		return nil, ErrSubscriptionExpired
	}

	return sub, nil
}

// CanInviteUser checks if organization can invite new users
func (s *featureService) CanInviteUser(ctx context.Context, orgID uint) (bool, error) {
	sub, err := s.getSubscriptionWithPlan(ctx, orgID)
	if err != nil {
		return false, err
	}

	// Seat-based plans: if seats are set, enforce by seats.
	if sub.SeatsPurchased != nil && *sub.SeatsPurchased > 0 {
		currentUsers, err := s.orgService.GetMemberCount(ctx, orgID)
		if err != nil {
			return false, fmt.Errorf("failed to get member count: %w", err)
		}
		if currentUsers >= *sub.SeatsPurchased {
			return false, ErrPlanLimitReached
		}
		return true, nil
	}

	// Non-seat-based plans: fall back to max_users limit if set
	if sub.Plan.MaxUsers != nil {
		currentUsers, err := s.orgService.GetMemberCount(ctx, orgID)
		if err != nil {
			return false, fmt.Errorf("failed to get member count: %w", err)
		}

		if currentUsers >= *sub.Plan.MaxUsers {
			return false, ErrPlanLimitReached
		}
	}

	return true, nil
}

// CanCreateCollection checks if organization can create new collections
func (s *featureService) CanCreateCollection(ctx context.Context, orgID uint) (bool, error) {
	sub, err := s.getSubscriptionWithPlan(ctx, orgID)
	if err != nil {
		return false, err
	}

	// Check max collections limit
	if sub.Plan.MaxCollections != nil {
		currentCollections, err := s.orgService.GetCollectionCount(ctx, orgID)
		if err != nil {
			return false, fmt.Errorf("failed to get collection count: %w", err)
		}

		if currentCollections >= *sub.Plan.MaxCollections {
			return false, ErrPlanLimitReached
		}
	}

	return true, nil
}

// CanCreateItem checks if organization can create new items
func (s *featureService) CanCreateItem(ctx context.Context, orgID uint) (bool, error) {
	sub, err := s.getSubscriptionWithPlan(ctx, orgID)
	if err != nil {
		return false, err
	}

	// Check max items limit
	if sub.Plan.MaxItems != nil {
		currentItems, err := s.itemRepo.CountByOrganizationID(ctx, orgID)
		if err != nil {
			return false, fmt.Errorf("failed to get item count: %w", err)
		}

		if currentItems >= *sub.Plan.MaxItems {
			return false, ErrPlanLimitReached
		}
	}

	return true, nil
}

// CanUseTeams checks if organization can use teams feature
func (s *featureService) CanUseTeams(ctx context.Context, orgID uint) (bool, error) {
	return s.checkBooleanFeature(ctx, orgID, func(f domain.PlanFeatures) bool {
		return f.Teams
	})
}

// CanAccessAudit checks if organization can access audit logs
func (s *featureService) CanAccessAudit(ctx context.Context, orgID uint) (bool, error) {
	return s.checkBooleanFeature(ctx, orgID, func(f domain.PlanFeatures) bool {
		return f.Audit
	})
}

// CanUseSSO checks if organization can use SSO
func (s *featureService) CanUseSSO(ctx context.Context, orgID uint) (bool, error) {
	return s.checkBooleanFeature(ctx, orgID, func(f domain.PlanFeatures) bool {
		return f.SSO
	})
}

// CanUseBreachMonitoring checks if organization can use dark web / breach monitoring
func (s *featureService) CanUseBreachMonitoring(ctx context.Context, orgID uint) (bool, error) {
	return s.checkBooleanFeature(ctx, orgID, func(f domain.PlanFeatures) bool {
		return f.BreachMonitoring
	})
}

// CanUsePasskeys checks if organization can use passkeys feature
func (s *featureService) CanUsePasskeys(ctx context.Context, orgID uint) (bool, error) {
	return s.checkBooleanFeature(ctx, orgID, func(f domain.PlanFeatures) bool {
		return f.Passkeys
	})
}

// CanUseSharedItems checks if organization can use shared items feature
func (s *featureService) CanUseSharedItems(ctx context.Context, orgID uint) (bool, error) {
	return s.checkBooleanFeature(ctx, orgID, func(f domain.PlanFeatures) bool {
		return f.SharedItems
	})
}

// CanUseSecureSend checks if organization can use secure send feature
func (s *featureService) CanUseSecureSend(ctx context.Context, orgID uint) (bool, error) {
	return s.checkBooleanFeature(ctx, orgID, func(f domain.PlanFeatures) bool {
		return f.SecureSend
	})
}

// CanUseEmergencyAccess checks if organization can use emergency access feature
func (s *featureService) CanUseEmergencyAccess(ctx context.Context, orgID uint) (bool, error) {
	return s.checkBooleanFeature(ctx, orgID, func(f domain.PlanFeatures) bool {
		return f.EmergencyAccess
	})
}

// GetFeatures returns all features available to an organization
func (s *featureService) GetFeatures(ctx context.Context, orgID uint) (*domain.PlanFeatures, error) {
	sub, err := s.subRepo.GetByOrganizationID(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}

	if sub.Plan == nil {
		return nil, fmt.Errorf("subscription plan not loaded")
	}

	return &sub.Plan.Features, nil
}

func (s *featureService) checkBooleanFeature(
	ctx context.Context,
	orgID uint,
	isEnabled func(domain.PlanFeatures) bool,
) (bool, error) {
	sub, err := s.getSubscriptionWithPlan(ctx, orgID)
	if err != nil {
		return false, err
	}

	if !isEnabled(sub.Plan.Features) {
		return false, ErrFeatureNotAvailable
	}

	return true, nil
}
