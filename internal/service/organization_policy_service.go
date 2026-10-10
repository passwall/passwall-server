package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
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
	// GetEffectivePolicies returns the client-enforced policies that bind
	// userID, merged across organizations.
	GetEffectivePolicies(ctx context.Context, userID uint) (*domain.EffectivePoliciesResponse, error)
}

type organizationPolicyService struct {
	policyRepo  repository.OrganizationPolicyRepository
	orgUserRepo repository.OrganizationUserRepository
	subRepo     interface {
		GetByOrganizationID(ctx context.Context, orgID uint) (*domain.Subscription, error)
	}
	logger       Logger
	entitlements OrganizationEntitlementService
}

// NewOrganizationPolicyService creates a new organization policy service
func NewOrganizationPolicyService(
	policyRepo repository.OrganizationPolicyRepository,
	orgUserRepo repository.OrganizationUserRepository,
	subRepo interface {
		GetByOrganizationID(ctx context.Context, orgID uint) (*domain.Subscription, error)
	},
	logger Logger,
	entitlements ...OrganizationEntitlementService,
) OrganizationPolicyService {
	service := &organizationPolicyService{
		policyRepo:  policyRepo,
		orgUserRepo: orgUserRepo,
		subRepo:     subRepo,
		logger:      logger,
	}
	if len(entitlements) > 0 {
		service.entitlements = entitlements[0]
	}
	return service
}

func (s *organizationPolicyService) ListByOrganization(ctx context.Context, orgID, userID uint) ([]*domain.OrganizationPolicyDTO, error) {
	if err := s.requireOrgAdmin(ctx, orgID, userID); err != nil {
		return nil, err
	}

	var (
		orgPlan        domain.OrganizationPlan
		policyFeatures *domain.PlanFeatures
	)
	if s.entitlements != nil {
		snapshot, err := s.entitlements.Resolve(ctx, orgID)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve organization entitlements: %w", err)
		}
		policyFeatures = &snapshot.Features
	} else {
		var err error
		orgPlan, err = s.getOrganizationPlan(ctx, orgID)
		if err != nil {
			return nil, fmt.Errorf("failed to determine organization plan: %w", err)
		}
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
		persistedPolicy, isPersisted := persistedMap[def.Type]
		// Enabled policies stay visible after a downgrade so admins can see
		// what is still enforced and disable it.
		stillEnforced := isPersisted && persistedPolicy.Enabled
		if !stillEnforced && policyFeatures != nil && !policyTierEnabled(*policyFeatures, def.Tier) {
			continue
		}
		if !stillEnforced && policyFeatures == nil && !domain.TierMeetsMinimum(orgPlan, def.Tier) {
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

	// Turning a policy off only relaxes restrictions the org already chose,
	// so it must stay possible after a downgrade or while frozen.
	disableOnly := req.Enabled != nil && !*req.Enabled && req.Data == nil
	tier, _ := domain.GetPolicyTier(policyType)
	if s.entitlements != nil {
		capability := policyCapabilityForTier(tier)
		if disableOnly {
			capability = domain.CapabilityPoliciesDisable
		}
		if err := s.entitlements.Authorize(ctx, orgID, capability); err != nil {
			return nil, err
		}
	} else if !disableOnly {
		orgPlan, err := s.getOrganizationPlan(ctx, orgID)
		if err != nil {
			return nil, fmt.Errorf("failed to determine organization plan: %w", err)
		}
		if !domain.TierMeetsMinimum(orgPlan, tier) {
			return nil, fmt.Errorf("policy %s requires %s plan or higher", policyType, tier)
		}
	}

	// Enabling: validate dependency chain
	enabling := req.Enabled != nil && *req.Enabled
	if enabling && !domain.IsPolicyAvailable(policyType) {
		return nil, domain.ErrPolicyNotAvailable
	}
	var cleanData domain.PolicyData
	if req.Data != nil {
		var err error
		if cleanData, err = domain.ValidatePolicyData(policyType, req.Data); err != nil {
			return nil, err
		}
	}
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

	wasEnabled := policy.Enabled
	previousEnabledAt := policy.Data["enabled_at"]

	if req.Enabled != nil {
		policy.Enabled = *req.Enabled
		if *req.Enabled {
			policy.LastEnabledByUserID = &userID
		} else {
			policy.LastDisabledByUserID = &userID
		}
	}

	if cleanData != nil {
		policy.Data = cleanData
	}
	if policy.Data == nil {
		policy.Data = make(domain.PolicyData)
	}

	// 2FA grace period: it starts when the policy is switched on (again) and
	// is not restarted by later configuration changes.
	if policyType == domain.PolicyRequireTwoFactor && policy.Enabled {
		if enabling && !wasEnabled {
			policy.Data["enabled_at"] = time.Now().UTC().Format(time.RFC3339)
		} else if previousEnabledAt != nil {
			policy.Data["enabled_at"] = previousEnabledAt
		} else if _, ok := policy.Data["enabled_at"]; !ok {
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
	member, err := s.orgUserRepo.GetActiveByOrgAndUser(ctx, orgID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, repository.ErrForbidden
		}
		return nil, err
	}

	policies, err := s.policyRepo.ListEnabledByOrganization(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to list enabled policies: %w", err)
	}

	summary := make(map[domain.PolicyType]domain.PolicyData, len(policies))
	for _, p := range policies {
		// Members learn that a policy applies, not how security controls are
		// configured (IP allow lists, lockout thresholds).
		if !member.IsAdmin() && adminOnlyPolicyData[p.Type] {
			summary[p.Type] = domain.PolicyData{}
			continue
		}
		summary[p.Type] = p.Data
	}
	return summary, nil
}

func (s *organizationPolicyService) GetEffectivePolicies(ctx context.Context, userID uint) (*domain.EffectivePoliciesResponse, error) {
	memberships, err := s.orgUserRepo.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list memberships: %w", err)
	}
	sort.Slice(memberships, func(i, j int) bool { return memberships[i].OrganizationID < memberships[j].OrganizationID })

	type collected struct {
		datas []domain.PolicyData
		orgs  []domain.EffectivePolicyOrg
	}
	byType := map[domain.PolicyType]*collected{}
	for _, m := range memberships {
		if m == nil || (m.Status != domain.OrgUserStatusAccepted && m.Status != domain.OrgUserStatusConfirmed) {
			continue
		}
		if m.Organization != nil && m.Organization.IsPersonal {
			continue
		}
		policies, err := s.policyRepo.ListEnabledByOrganization(ctx, m.OrganizationID)
		if err != nil {
			return nil, fmt.Errorf("failed to list enabled policies: %w", err)
		}
		for _, p := range policies {
			if !domain.IsPolicyAvailable(p.Type) || !domain.PolicyAppliesToMember(p.Type, m.Role) {
				continue
			}
			c := byType[p.Type]
			if c == nil {
				c = &collected{}
				byType[p.Type] = c
			}
			c.datas = append(c.datas, p.Data)
			org := domain.EffectivePolicyOrg{ID: m.OrganizationID}
			if m.Organization != nil {
				org.PublicID = m.Organization.PublicID
				org.Name = m.Organization.Name
			}
			c.orgs = append(c.orgs, org)
		}
	}

	resp := &domain.EffectivePoliciesResponse{Policies: []domain.EffectivePolicy{}}
	for t, c := range byType {
		resp.Policies = append(resp.Policies, domain.EffectivePolicy{
			Type:          t,
			Data:          domain.MergePolicyData(t, c.datas),
			Organizations: c.orgs,
		})
	}
	domain.SortEffectivePolicies(resp.Policies)
	return resp, nil
}

// adminOnlyPolicyData are policies whose configuration is hidden from
// non-admin members.
var adminOnlyPolicyData = map[domain.PolicyType]bool{
	domain.PolicyFirewallRules:    true,
	domain.PolicyFailedLoginLimit: true,
}

// --- Helpers ---

func (s *organizationPolicyService) requireOrgAdmin(ctx context.Context, orgID, userID uint) error {
	orgUser, err := s.orgUserRepo.GetActiveByOrgAndUser(ctx, orgID, userID)
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

func policyCapabilityForTier(tier domain.PolicyTier) domain.Capability {
	switch tier {
	case domain.PolicyTierBusiness:
		return domain.CapabilityBusinessPoliciesManage
	case domain.PolicyTierEnterprise:
		return domain.CapabilityEnterprisePoliciesManage
	default:
		return domain.CapabilityPoliciesManage
	}
}

func policyTierEnabled(features domain.PlanFeatures, tier domain.PolicyTier) bool {
	switch tier {
	case domain.PolicyTierBusiness:
		return features.BusinessPolicies
	case domain.PolicyTierEnterprise:
		return features.EnterprisePolicies
	default:
		return features.Policies
	}
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
