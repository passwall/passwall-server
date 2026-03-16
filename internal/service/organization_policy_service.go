package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
)

// OrganizationPolicyService defines the business logic for organization policies
type OrganizationPolicyService interface {
	// CRUD
	ListByOrganization(ctx context.Context, orgID, userID uint) ([]*domain.OrganizationPolicyDTO, error)
	GetByType(ctx context.Context, orgID, userID uint, policyType domain.PolicyType) (*domain.OrganizationPolicyDTO, error)
	UpdatePolicy(ctx context.Context, orgID, userID uint, policyType domain.PolicyType, req *domain.UpdateOrganizationPolicyRequest) (*domain.OrganizationPolicyDTO, error)

	// Enforcement queries (used by other services and middleware)
	IsPolicyEnabled(ctx context.Context, orgID uint, policyType domain.PolicyType) (bool, error)
	GetPolicyData(ctx context.Context, orgID uint, policyType domain.PolicyType) (domain.PolicyData, error)
	ListEnabledPolicies(ctx context.Context, orgID uint) ([]*domain.OrganizationPolicyDTO, error)
	GetActivePolicySummary(ctx context.Context, orgID, userID uint) (map[domain.PolicyType]domain.PolicyData, error)
}

type organizationPolicyService struct {
	policyRepo  repository.OrganizationPolicyRepository
	orgUserRepo repository.OrganizationUserRepository
	subRepo     interface {
		GetByOrganizationID(ctx context.Context, orgID uint) (*domain.Subscription, error)
	}
	logger Logger
}

// NewOrganizationPolicyService creates a new organization policy service
func NewOrganizationPolicyService(
	policyRepo repository.OrganizationPolicyRepository,
	orgUserRepo repository.OrganizationUserRepository,
	subRepo interface {
		GetByOrganizationID(ctx context.Context, orgID uint) (*domain.Subscription, error)
	},
	logger Logger,
) OrganizationPolicyService {
	return &organizationPolicyService{
		policyRepo:  policyRepo,
		orgUserRepo: orgUserRepo,
		subRepo:     subRepo,
		logger:      logger,
	}
}

func (s *organizationPolicyService) ListByOrganization(ctx context.Context, orgID, userID uint) ([]*domain.OrganizationPolicyDTO, error) {
	if err := s.requireOrgAdmin(ctx, orgID, userID); err != nil {
		return nil, err
	}

	orgPlan, err := s.getOrganizationPlan(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to determine organization plan: %w", err)
	}

	persisted, err := s.policyRepo.ListByOrganization(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to list policies: %w", err)
	}

	persistedMap := make(map[domain.PolicyType]*domain.OrganizationPolicy, len(persisted))
	for _, p := range persisted {
		persistedMap[p.Type] = p
	}

	definitions := domain.AllPolicyDefinitions()
	dtos := make([]*domain.OrganizationPolicyDTO, 0, len(definitions))

	for _, def := range definitions {
		if !domain.TierMeetsMinimum(orgPlan, def.Tier) {
			continue
		}

		if p, ok := persistedMap[def.Type]; ok {
			dtos = append(dtos, domain.ToOrganizationPolicyDTO(p))
		} else {
			dtos = append(dtos, &domain.OrganizationPolicyDTO{
				OrganizationID: orgID,
				Type:           def.Type,
				Enabled:        false,
				Data:           make(domain.PolicyData),
			})
		}
	}

	return dtos, nil
}

func (s *organizationPolicyService) GetByType(ctx context.Context, orgID, userID uint, policyType domain.PolicyType) (*domain.OrganizationPolicyDTO, error) {
	if err := s.requireOrgAdmin(ctx, orgID, userID); err != nil {
		return nil, err
	}

	if !domain.IsValidPolicyType(policyType) {
		return nil, fmt.Errorf("unknown policy type: %s", policyType)
	}

	policy, err := s.policyRepo.GetByOrgAndType(ctx, orgID, policyType)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return &domain.OrganizationPolicyDTO{
				OrganizationID: orgID,
				Type:           policyType,
				Enabled:        false,
				Data:           make(domain.PolicyData),
			}, nil
		}
		return nil, fmt.Errorf("failed to get policy: %w", err)
	}

	return domain.ToOrganizationPolicyDTO(policy), nil
}

func (s *organizationPolicyService) UpdatePolicy(ctx context.Context, orgID, userID uint, policyType domain.PolicyType, req *domain.UpdateOrganizationPolicyRequest) (*domain.OrganizationPolicyDTO, error) {
	if err := s.requireOrgAdmin(ctx, orgID, userID); err != nil {
		return nil, err
	}

	if !domain.IsValidPolicyType(policyType) {
		return nil, fmt.Errorf("unknown policy type: %s", policyType)
	}

	// Plan gating
	orgPlan, err := s.getOrganizationPlan(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to determine organization plan: %w", err)
	}

	tier, _ := domain.GetPolicyTier(policyType)
	if !domain.TierMeetsMinimum(orgPlan, tier) {
		return nil, fmt.Errorf("policy %s requires %s plan or higher", policyType, tier)
	}

	// Enabling: validate dependency chain
	enabling := req.Enabled != nil && *req.Enabled
	if enabling {
		deps := domain.GetPolicyDependencies(policyType)
		for _, dep := range deps {
			depEnabled, err := s.IsPolicyEnabled(ctx, orgID, dep)
			if err != nil {
				return nil, fmt.Errorf("failed to check dependency %s: %w", dep, err)
			}
			if !depEnabled {
				return nil, fmt.Errorf("policy %s requires %s to be enabled first", policyType, dep)
			}
		}
	}

	// Disabling: check if other policies depend on this one
	disabling := req.Enabled != nil && !*req.Enabled
	if disabling {
		dependents, err := s.findDependents(ctx, orgID, policyType)
		if err != nil {
			return nil, fmt.Errorf("failed to check dependents: %w", err)
		}
		if len(dependents) > 0 {
			return nil, fmt.Errorf("cannot disable %s: the following policies depend on it: %v", policyType, dependents)
		}
	}

	// Upsert the policy
	policy, err := s.policyRepo.GetByOrgAndType(ctx, orgID, policyType)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, fmt.Errorf("failed to get policy: %w", err)
	}

	if policy == nil {
		policy = &domain.OrganizationPolicy{
			UUID:           uuid.New(),
			OrganizationID: orgID,
			Type:           policyType,
			Enabled:        false,
			Data:           make(domain.PolicyData),
		}
		if err := s.policyRepo.Create(ctx, policy); err != nil {
			return nil, fmt.Errorf("failed to create policy: %w", err)
		}
	}

	if req.Enabled != nil {
		policy.Enabled = *req.Enabled
		if *req.Enabled {
			policy.LastEnabledByUserID = &userID
		} else {
			policy.LastDisabledByUserID = &userID
		}
	}

	if req.Data != nil {
		policy.Data = req.Data
	}

	// Auto-populate enabled_at for 2FA policy when first enabled
	if enabling && policyType == domain.PolicyRequireTwoFactor {
		if _, hasEnabledAt := policy.Data["enabled_at"]; !hasEnabledAt {
			if policy.Data == nil {
				policy.Data = make(domain.PolicyData)
			}
			policy.Data["enabled_at"] = time.Now().UTC().Format(time.RFC3339)
		}
		if _, hasGrace := policy.Data["grace_period_days"]; !hasGrace {
			policy.Data["grace_period_days"] = float64(7)
		}
	}

	if err := s.policyRepo.Update(ctx, policy); err != nil {
		s.logger.Error("failed to update policy", "org_id", orgID, "type", policyType, "error", err)
		return nil, fmt.Errorf("failed to update policy: %w", err)
	}

	action := "updated"
	if enabling {
		action = "enabled"
	} else if disabling {
		action = "disabled"
	}
	s.logger.Info("organization policy "+action, "org_id", orgID, "type", policyType, "user_id", userID)

	return domain.ToOrganizationPolicyDTO(policy), nil
}

// --- Enforcement queries (no auth check, called by backend services) ---

func (s *organizationPolicyService) IsPolicyEnabled(ctx context.Context, orgID uint, policyType domain.PolicyType) (bool, error) {
	policy, err := s.policyRepo.GetByOrgAndType(ctx, orgID, policyType)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return policy.Enabled, nil
}

func (s *organizationPolicyService) GetPolicyData(ctx context.Context, orgID uint, policyType domain.PolicyType) (domain.PolicyData, error) {
	policy, err := s.policyRepo.GetByOrgAndType(ctx, orgID, policyType)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if !policy.Enabled {
		return nil, nil
	}
	return policy.Data, nil
}

func (s *organizationPolicyService) ListEnabledPolicies(ctx context.Context, orgID uint) ([]*domain.OrganizationPolicyDTO, error) {
	policies, err := s.policyRepo.ListEnabledByOrganization(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to list enabled policies: %w", err)
	}

	dtos := make([]*domain.OrganizationPolicyDTO, len(policies))
	for i, p := range policies {
		dtos[i] = domain.ToOrganizationPolicyDTO(p)
	}
	return dtos, nil
}

func (s *organizationPolicyService) GetActivePolicySummary(ctx context.Context, orgID, userID uint) (map[domain.PolicyType]domain.PolicyData, error) {
	if err := s.requireOrgMember(ctx, orgID, userID); err != nil {
		return nil, err
	}

	policies, err := s.policyRepo.ListEnabledByOrganization(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to list enabled policies: %w", err)
	}

	summary := make(map[domain.PolicyType]domain.PolicyData, len(policies))
	for _, p := range policies {
		summary[p.Type] = p.Data
	}
	return summary, nil
}

// --- Helpers ---

func (s *organizationPolicyService) requireOrgAdmin(ctx context.Context, orgID, userID uint) error {
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

func (s *organizationPolicyService) requireOrgMember(ctx context.Context, orgID, userID uint) error {
	_, err := s.orgUserRepo.GetByOrgAndUser(ctx, orgID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return repository.ErrForbidden
		}
		return err
	}
	return nil
}

func (s *organizationPolicyService) getOrganizationPlan(ctx context.Context, orgID uint) (domain.OrganizationPlan, error) {
	sub, err := s.subRepo.GetByOrganizationID(ctx, orgID)
	if err != nil {
		return domain.PlanFree, fmt.Errorf("failed to get subscription: %w", err)
	}

	if sub.Plan == nil {
		return domain.PlanFree, nil
	}

	planCode := sub.Plan.Code
	base := planCode
	for _, suffix := range []string{"-monthly", "-yearly"} {
		if len(base) > len(suffix) && base[len(base)-len(suffix):] == suffix {
			base = base[:len(base)-len(suffix)]
			break
		}
	}
	return domain.OrganizationPlan(base), nil
}

// findDependents returns enabled policies in this org that depend on the given policy type.
func (s *organizationPolicyService) findDependents(ctx context.Context, orgID uint, policyType domain.PolicyType) ([]domain.PolicyType, error) {
	enabled, err := s.policyRepo.ListEnabledByOrganization(ctx, orgID)
	if err != nil {
		return nil, err
	}

	var dependents []domain.PolicyType
	for _, p := range enabled {
		deps := domain.GetPolicyDependencies(p.Type)
		for _, dep := range deps {
			if dep == policyType {
				dependents = append(dependents, p.Type)
				break
			}
		}
	}
	return dependents, nil
}
