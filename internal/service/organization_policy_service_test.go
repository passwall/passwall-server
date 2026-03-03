package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Fake repositories for policy tests ─────────────────────────────────────────

type fakePolicyRepo struct {
	policies map[string]*domain.OrganizationPolicy // keyed by "orgID:type"
}

func newFakePolicyRepo() *fakePolicyRepo {
	return &fakePolicyRepo{policies: make(map[string]*domain.OrganizationPolicy)}
}

func policyKey(orgID uint, policyType domain.PolicyType) string {
	return fmt.Sprintf("%d:%s", orgID, policyType)
}

func (f *fakePolicyRepo) add(p *domain.OrganizationPolicy) {
	f.policies[policyKey(p.OrganizationID, p.Type)] = p
}

func (f *fakePolicyRepo) Create(_ context.Context, p *domain.OrganizationPolicy) error {
	key := policyKey(p.OrganizationID, p.Type)
	if _, exists := f.policies[key]; exists {
		return repository.ErrAlreadyExists
	}
	if p.ID == 0 {
		p.ID = uint(len(f.policies) + 1)
	}
	f.policies[key] = p
	return nil
}

func (f *fakePolicyRepo) GetByID(_ context.Context, id uint) (*domain.OrganizationPolicy, error) {
	for _, p := range f.policies {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (f *fakePolicyRepo) GetByOrgAndType(_ context.Context, orgID uint, policyType domain.PolicyType) (*domain.OrganizationPolicy, error) {
	p, ok := f.policies[policyKey(orgID, policyType)]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return p, nil
}

func (f *fakePolicyRepo) ListByOrganization(_ context.Context, orgID uint) ([]*domain.OrganizationPolicy, error) {
	var result []*domain.OrganizationPolicy
	for _, p := range f.policies {
		if p.OrganizationID == orgID {
			result = append(result, p)
		}
	}
	return result, nil
}

func (f *fakePolicyRepo) ListEnabledByOrganization(_ context.Context, orgID uint) ([]*domain.OrganizationPolicy, error) {
	var result []*domain.OrganizationPolicy
	for _, p := range f.policies {
		if p.OrganizationID == orgID && p.Enabled {
			result = append(result, p)
		}
	}
	return result, nil
}

func (f *fakePolicyRepo) Update(_ context.Context, p *domain.OrganizationPolicy) error {
	key := policyKey(p.OrganizationID, p.Type)
	f.policies[key] = p
	return nil
}

func (f *fakePolicyRepo) Delete(_ context.Context, id uint) error {
	for key, p := range f.policies {
		if p.ID == id {
			delete(f.policies, key)
			return nil
		}
	}
	return repository.ErrNotFound
}

// fakeSubRepo simulates subscription lookup
type fakeSubRepo struct {
	subs map[uint]*domain.Subscription
}

func newFakeSubRepo() *fakeSubRepo {
	return &fakeSubRepo{subs: make(map[uint]*domain.Subscription)}
}

func (f *fakeSubRepo) setOrgPlan(orgID uint, planCode string) {
	f.subs[orgID] = &domain.Subscription{
		ID:             orgID,
		OrganizationID: orgID,
		State:          domain.SubStateActive,
		Plan:           &domain.Plan{Code: planCode},
	}
}

func (f *fakeSubRepo) GetByOrganizationID(_ context.Context, orgID uint) (*domain.Subscription, error) {
	sub, ok := f.subs[orgID]
	if !ok {
		return &domain.Subscription{Plan: nil}, nil
	}
	return sub, nil
}

// ─── Test builder ───────────────────────────────────────────────────────────────

const (
	policyTestOrgID     = uint(1)
	policyTestOwnerID   = uint(100)
	policyTestAdminID   = uint(101)
	policyTestMemberID  = uint(200)
	policyTestManagerID = uint(201)
)

type policyTestSetup struct {
	service     OrganizationPolicyService
	policyRepo  *fakePolicyRepo
	orgUserRepo *fakeOrgUserRepo
	subRepo     *fakeSubRepo
}

func newPolicyTestSetup(plan string) *policyTestSetup {
	policyRepo := newFakePolicyRepo()
	orgUserRepo := newFakeOrgUserRepo()
	subRepo := newFakeSubRepo()

	orgUserRepo.add(&domain.OrganizationUser{
		OrganizationID: policyTestOrgID,
		UserID:         policyTestOwnerID,
		Role:           domain.OrgRoleOwner,
		Status:         domain.OrgUserStatusAccepted,
	})
	orgUserRepo.add(&domain.OrganizationUser{
		OrganizationID: policyTestOrgID,
		UserID:         policyTestAdminID,
		Role:           domain.OrgRoleAdmin,
		Status:         domain.OrgUserStatusAccepted,
	})
	orgUserRepo.add(&domain.OrganizationUser{
		OrganizationID: policyTestOrgID,
		UserID:         policyTestMemberID,
		Role:           domain.OrgRoleMember,
		Status:         domain.OrgUserStatusAccepted,
	})
	orgUserRepo.add(&domain.OrganizationUser{
		OrganizationID: policyTestOrgID,
		UserID:         policyTestManagerID,
		Role:           domain.OrgRoleManager,
		Status:         domain.OrgUserStatusAccepted,
	})

	subRepo.setOrgPlan(policyTestOrgID, plan)

	svc := NewOrganizationPolicyService(policyRepo, orgUserRepo, subRepo, noopLogger{})

	return &policyTestSetup{
		service:     svc,
		policyRepo:  policyRepo,
		orgUserRepo: orgUserRepo,
		subRepo:     subRepo,
	}
}

func boolPtr(b bool) *bool { return &b }

// ─── RBAC Tests ─────────────────────────────────────────────────────────────────

func TestListPolicies_OwnerCanAccess(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	policies, err := s.service.ListByOrganization(ctx, policyTestOrgID, policyTestOwnerID)
	require.NoError(t, err)
	assert.NotEmpty(t, policies, "owner should see available policies")
}

func TestListPolicies_AdminCanAccess(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-monthly")
	ctx := context.Background()

	policies, err := s.service.ListByOrganization(ctx, policyTestOrgID, policyTestAdminID)
	require.NoError(t, err)
	assert.NotEmpty(t, policies)
}

func TestListPolicies_MemberDenied(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	_, err := s.service.ListByOrganization(ctx, policyTestOrgID, policyTestMemberID)
	assert.ErrorIs(t, err, repository.ErrForbidden, "regular member should not list policies")
}

func TestListPolicies_ManagerDenied(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	_, err := s.service.ListByOrganization(ctx, policyTestOrgID, policyTestManagerID)
	assert.ErrorIs(t, err, repository.ErrForbidden, "manager should not manage policies")
}

func TestListPolicies_NonMemberDenied(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	_, err := s.service.ListByOrganization(ctx, policyTestOrgID, 9999)
	assert.ErrorIs(t, err, repository.ErrForbidden, "non-member should be denied")
}

func TestUpdatePolicy_MemberDenied(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	_, err := s.service.UpdatePolicy(ctx, policyTestOrgID, policyTestMemberID,
		domain.PolicyRequireTwoFactor, &domain.UpdateOrganizationPolicyRequest{Enabled: boolPtr(true)})
	assert.ErrorIs(t, err, repository.ErrForbidden, "regular member should not update policies")
}

func TestUpdatePolicy_ManagerDenied(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	_, err := s.service.UpdatePolicy(ctx, policyTestOrgID, policyTestManagerID,
		domain.PolicyRequireTwoFactor, &domain.UpdateOrganizationPolicyRequest{Enabled: boolPtr(true)})
	assert.ErrorIs(t, err, repository.ErrForbidden, "manager should not update policies")
}

func TestGetPolicy_MemberDenied(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	_, err := s.service.GetByType(ctx, policyTestOrgID, policyTestMemberID, domain.PolicyRequireTwoFactor)
	assert.ErrorIs(t, err, repository.ErrForbidden, "regular member should not get policy details")
}

// ─── Plan Gating Tests ──────────────────────────────────────────────────────────

func TestListPolicies_FreePlan_ShowsNoPolicies(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("free")
	ctx := context.Background()

	policies, err := s.service.ListByOrganization(ctx, policyTestOrgID, policyTestOwnerID)
	require.NoError(t, err)
	assert.Empty(t, policies, "free plan should have no available policies")
}

func TestListPolicies_TeamPlan_ShowsOnlyTeamPolicies(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("team-monthly")
	ctx := context.Background()

	policies, err := s.service.ListByOrganization(ctx, policyTestOrgID, policyTestOwnerID)
	require.NoError(t, err)

	for _, p := range policies {
		tier, ok := domain.GetPolicyTier(p.Type)
		require.True(t, ok)
		assert.Equal(t, domain.PolicyTierTeam, tier,
			"team plan should only see team-tier policies, got %s (%s)", p.Type, tier)
	}
	assert.NotEmpty(t, policies, "team plan should have some policies")
}

func TestListPolicies_BusinessPlan_IncludesTeamAndBusiness(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("business-yearly")
	ctx := context.Background()

	policies, err := s.service.ListByOrganization(ctx, policyTestOrgID, policyTestOwnerID)
	require.NoError(t, err)

	tierSet := make(map[domain.PolicyTier]bool)
	for _, p := range policies {
		tier, _ := domain.GetPolicyTier(p.Type)
		tierSet[tier] = true
		assert.NotEqual(t, domain.PolicyTierEnterprise, tier,
			"business plan should not include enterprise policies, got %s", p.Type)
	}
	assert.True(t, tierSet[domain.PolicyTierTeam], "business plan should include team-tier policies")
	assert.True(t, tierSet[domain.PolicyTierBusiness], "business plan should include business-tier policies")
}

func TestListPolicies_EnterprisePlan_IncludesAll(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	policies, err := s.service.ListByOrganization(ctx, policyTestOrgID, policyTestOwnerID)
	require.NoError(t, err)
	assert.Equal(t, len(domain.AllPolicyDefinitions()), len(policies),
		"enterprise plan should show all policy definitions")
}

func TestUpdatePolicy_PlanGating_TeamCannotEnableEnterprise(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("team-monthly")
	ctx := context.Background()

	_, err := s.service.UpdatePolicy(ctx, policyTestOrgID, policyTestOwnerID,
		domain.PolicyFirewallRules, &domain.UpdateOrganizationPolicyRequest{Enabled: boolPtr(true)})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires")
}

func TestUpdatePolicy_PlanGating_TeamCannotEnableBusiness(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("team-monthly")
	ctx := context.Background()

	_, err := s.service.UpdatePolicy(ctx, policyTestOrgID, policyTestOwnerID,
		domain.PolicySingleOrganization, &domain.UpdateOrganizationPolicyRequest{Enabled: boolPtr(true)})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires")
}

// ─── Policy Enable / Disable Tests ──────────────────────────────────────────────

func TestUpdatePolicy_EnableSimplePolicy(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	result, err := s.service.UpdatePolicy(ctx, policyTestOrgID, policyTestOwnerID,
		domain.PolicyRequireTwoFactor, &domain.UpdateOrganizationPolicyRequest{Enabled: boolPtr(true)})
	require.NoError(t, err)
	assert.True(t, result.Enabled)
	assert.Equal(t, domain.PolicyRequireTwoFactor, result.Type)
	assert.Equal(t, policyTestOrgID, result.OrganizationID)
}

func TestUpdatePolicy_DisablePolicy(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	// Enable first
	_, err := s.service.UpdatePolicy(ctx, policyTestOrgID, policyTestOwnerID,
		domain.PolicyRequireTwoFactor, &domain.UpdateOrganizationPolicyRequest{Enabled: boolPtr(true)})
	require.NoError(t, err)

	// Disable
	result, err := s.service.UpdatePolicy(ctx, policyTestOrgID, policyTestOwnerID,
		domain.PolicyRequireTwoFactor, &domain.UpdateOrganizationPolicyRequest{Enabled: boolPtr(false)})
	require.NoError(t, err)
	assert.False(t, result.Enabled)
}

func TestUpdatePolicy_InvalidType(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	_, err := s.service.UpdatePolicy(ctx, policyTestOrgID, policyTestOwnerID,
		"nonexistent_policy", &domain.UpdateOrganizationPolicyRequest{Enabled: boolPtr(true)})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown policy type")
}

func TestUpdatePolicy_WithData(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	data := domain.PolicyData{
		"min_length":        float64(14),
		"require_uppercase": true,
		"require_lowercase": true,
		"require_numbers":   true,
		"require_special":   true,
	}

	result, err := s.service.UpdatePolicy(ctx, policyTestOrgID, policyTestOwnerID,
		domain.PolicyMasterPWRequirements, &domain.UpdateOrganizationPolicyRequest{
			Enabled: boolPtr(true),
			Data:    data,
		})
	require.NoError(t, err)
	assert.True(t, result.Enabled)
	assert.Equal(t, float64(14), result.Data["min_length"])
	assert.Equal(t, true, result.Data["require_uppercase"])
}

func TestUpdatePolicy_UpdateDataWithoutChangingEnabled(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	// Enable with initial data
	_, err := s.service.UpdatePolicy(ctx, policyTestOrgID, policyTestOwnerID,
		domain.PolicyMasterPWRequirements, &domain.UpdateOrganizationPolicyRequest{
			Enabled: boolPtr(true),
			Data:    domain.PolicyData{"min_length": float64(8)},
		})
	require.NoError(t, err)

	// Update only data (Enabled is nil)
	result, err := s.service.UpdatePolicy(ctx, policyTestOrgID, policyTestOwnerID,
		domain.PolicyMasterPWRequirements, &domain.UpdateOrganizationPolicyRequest{
			Data: domain.PolicyData{"min_length": float64(16)},
		})
	require.NoError(t, err)
	assert.True(t, result.Enabled, "enabled state should be preserved")
	assert.Equal(t, float64(16), result.Data["min_length"])
}

func TestUpdatePolicy_AuditTrail_LastEnabledByUserID(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	_, err := s.service.UpdatePolicy(ctx, policyTestOrgID, policyTestOwnerID,
		domain.PolicyRequireTwoFactor, &domain.UpdateOrganizationPolicyRequest{Enabled: boolPtr(true)})
	require.NoError(t, err)

	p, err := s.policyRepo.GetByOrgAndType(ctx, policyTestOrgID, domain.PolicyRequireTwoFactor)
	require.NoError(t, err)
	require.NotNil(t, p.LastEnabledByUserID)
	assert.Equal(t, policyTestOwnerID, *p.LastEnabledByUserID)
}

func TestUpdatePolicy_AuditTrail_LastDisabledByUserID(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	// Enable, then disable
	_, _ = s.service.UpdatePolicy(ctx, policyTestOrgID, policyTestOwnerID,
		domain.PolicyRequireTwoFactor, &domain.UpdateOrganizationPolicyRequest{Enabled: boolPtr(true)})

	_, err := s.service.UpdatePolicy(ctx, policyTestOrgID, policyTestAdminID,
		domain.PolicyRequireTwoFactor, &domain.UpdateOrganizationPolicyRequest{Enabled: boolPtr(false)})
	require.NoError(t, err)

	p, err := s.policyRepo.GetByOrgAndType(ctx, policyTestOrgID, domain.PolicyRequireTwoFactor)
	require.NoError(t, err)
	require.NotNil(t, p.LastDisabledByUserID)
	assert.Equal(t, policyTestAdminID, *p.LastDisabledByUserID)
}

// ─── Dependency Chain Tests ─────────────────────────────────────────────────────

func TestUpdatePolicy_EnableWithUnmetDependency(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	// require_sso depends on single_organization
	_, err := s.service.UpdatePolicy(ctx, policyTestOrgID, policyTestOwnerID,
		domain.PolicyRequireSSO, &domain.UpdateOrganizationPolicyRequest{Enabled: boolPtr(true)})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), string(domain.PolicySingleOrganization))
}

func TestUpdatePolicy_EnableWithSatisfiedDependency(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	// Enable the dependency first
	_, err := s.service.UpdatePolicy(ctx, policyTestOrgID, policyTestOwnerID,
		domain.PolicySingleOrganization, &domain.UpdateOrganizationPolicyRequest{Enabled: boolPtr(true)})
	require.NoError(t, err)

	// Now enable the dependent policy
	result, err := s.service.UpdatePolicy(ctx, policyTestOrgID, policyTestOwnerID,
		domain.PolicyRequireSSO, &domain.UpdateOrganizationPolicyRequest{Enabled: boolPtr(true)})
	require.NoError(t, err)
	assert.True(t, result.Enabled)
}

func TestUpdatePolicy_DisableDependencyWithActiveDependents(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	// Enable single_organization, then require_sso
	_, err := s.service.UpdatePolicy(ctx, policyTestOrgID, policyTestOwnerID,
		domain.PolicySingleOrganization, &domain.UpdateOrganizationPolicyRequest{Enabled: boolPtr(true)})
	require.NoError(t, err)

	_, err = s.service.UpdatePolicy(ctx, policyTestOrgID, policyTestOwnerID,
		domain.PolicyRequireSSO, &domain.UpdateOrganizationPolicyRequest{Enabled: boolPtr(true)})
	require.NoError(t, err)

	// Try to disable single_organization (require_sso depends on it)
	_, err = s.service.UpdatePolicy(ctx, policyTestOrgID, policyTestOwnerID,
		domain.PolicySingleOrganization, &domain.UpdateOrganizationPolicyRequest{Enabled: boolPtr(false)})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot disable")
	assert.Contains(t, err.Error(), string(domain.PolicyRequireSSO))
}

func TestUpdatePolicy_DisableDependencyAfterDependentDisabled(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	// Enable chain
	_, err := s.service.UpdatePolicy(ctx, policyTestOrgID, policyTestOwnerID,
		domain.PolicySingleOrganization, &domain.UpdateOrganizationPolicyRequest{Enabled: boolPtr(true)})
	require.NoError(t, err)
	_, err = s.service.UpdatePolicy(ctx, policyTestOrgID, policyTestOwnerID,
		domain.PolicyRequireSSO, &domain.UpdateOrganizationPolicyRequest{Enabled: boolPtr(true)})
	require.NoError(t, err)

	// Disable the dependent first
	_, err = s.service.UpdatePolicy(ctx, policyTestOrgID, policyTestOwnerID,
		domain.PolicyRequireSSO, &domain.UpdateOrganizationPolicyRequest{Enabled: boolPtr(false)})
	require.NoError(t, err)

	// Now disabling the dependency should succeed
	_, err = s.service.UpdatePolicy(ctx, policyTestOrgID, policyTestOwnerID,
		domain.PolicySingleOrganization, &domain.UpdateOrganizationPolicyRequest{Enabled: boolPtr(false)})
	require.NoError(t, err)
}

func TestUpdatePolicy_MultipleDependentsBlockDisable(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	// single_organization is a dependency for: require_sso, session_timeout, account_recovery, default_uri_match
	_, err := s.service.UpdatePolicy(ctx, policyTestOrgID, policyTestOwnerID,
		domain.PolicySingleOrganization, &domain.UpdateOrganizationPolicyRequest{Enabled: boolPtr(true)})
	require.NoError(t, err)

	_, err = s.service.UpdatePolicy(ctx, policyTestOrgID, policyTestOwnerID,
		domain.PolicySessionTimeout, &domain.UpdateOrganizationPolicyRequest{Enabled: boolPtr(true)})
	require.NoError(t, err)

	_, err = s.service.UpdatePolicy(ctx, policyTestOrgID, policyTestOwnerID,
		domain.PolicyAccountRecovery, &domain.UpdateOrganizationPolicyRequest{Enabled: boolPtr(true)})
	require.NoError(t, err)

	// Try to disable single_organization (both session_timeout and account_recovery depend on it)
	_, err = s.service.UpdatePolicy(ctx, policyTestOrgID, policyTestOwnerID,
		domain.PolicySingleOrganization, &domain.UpdateOrganizationPolicyRequest{Enabled: boolPtr(false)})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot disable")
}

// ─── GetByType Tests ────────────────────────────────────────────────────────────

func TestGetByType_NonPersisted_ReturnsDefault(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	policy, err := s.service.GetByType(ctx, policyTestOrgID, policyTestOwnerID, domain.PolicyRequireTwoFactor)
	require.NoError(t, err)
	assert.Equal(t, domain.PolicyRequireTwoFactor, policy.Type)
	assert.False(t, policy.Enabled, "non-persisted policy should default to disabled")
	assert.Empty(t, policy.Data)
}

func TestGetByType_Persisted_ReturnsActual(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	// Enable a policy
	_, err := s.service.UpdatePolicy(ctx, policyTestOrgID, policyTestOwnerID,
		domain.PolicyRequireTwoFactor, &domain.UpdateOrganizationPolicyRequest{Enabled: boolPtr(true)})
	require.NoError(t, err)

	policy, err := s.service.GetByType(ctx, policyTestOrgID, policyTestOwnerID, domain.PolicyRequireTwoFactor)
	require.NoError(t, err)
	assert.True(t, policy.Enabled)
}

func TestGetByType_InvalidType(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	_, err := s.service.GetByType(ctx, policyTestOrgID, policyTestOwnerID, "invalid_type")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown policy type")
}

// ─── Enforcement Query Tests (no auth check) ───────────────────────────────────

func TestIsPolicyEnabled_NotPersisted(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	enabled, err := s.service.IsPolicyEnabled(ctx, policyTestOrgID, domain.PolicyRequireTwoFactor)
	require.NoError(t, err)
	assert.False(t, enabled)
}

func TestIsPolicyEnabled_Enabled(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	s.policyRepo.add(&domain.OrganizationPolicy{
		ID: 1, UUID: uuid.NewV4(), OrganizationID: policyTestOrgID,
		Type: domain.PolicyRequireTwoFactor, Enabled: true,
	})

	enabled, err := s.service.IsPolicyEnabled(ctx, policyTestOrgID, domain.PolicyRequireTwoFactor)
	require.NoError(t, err)
	assert.True(t, enabled)
}

func TestIsPolicyEnabled_Disabled(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	s.policyRepo.add(&domain.OrganizationPolicy{
		ID: 1, UUID: uuid.NewV4(), OrganizationID: policyTestOrgID,
		Type: domain.PolicyRequireTwoFactor, Enabled: false,
	})

	enabled, err := s.service.IsPolicyEnabled(ctx, policyTestOrgID, domain.PolicyRequireTwoFactor)
	require.NoError(t, err)
	assert.False(t, enabled)
}

func TestGetPolicyData_EnabledReturnsData(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	expectedData := domain.PolicyData{"min_length": float64(12)}
	s.policyRepo.add(&domain.OrganizationPolicy{
		ID: 1, UUID: uuid.NewV4(), OrganizationID: policyTestOrgID,
		Type: domain.PolicyMasterPWRequirements, Enabled: true, Data: expectedData,
	})

	data, err := s.service.GetPolicyData(ctx, policyTestOrgID, domain.PolicyMasterPWRequirements)
	require.NoError(t, err)
	assert.Equal(t, float64(12), data["min_length"])
}

func TestGetPolicyData_DisabledReturnsNil(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	s.policyRepo.add(&domain.OrganizationPolicy{
		ID: 1, UUID: uuid.NewV4(), OrganizationID: policyTestOrgID,
		Type: domain.PolicyMasterPWRequirements, Enabled: false,
		Data: domain.PolicyData{"min_length": float64(12)},
	})

	data, err := s.service.GetPolicyData(ctx, policyTestOrgID, domain.PolicyMasterPWRequirements)
	require.NoError(t, err)
	assert.Nil(t, data, "disabled policy should return nil data for enforcement")
}

func TestGetPolicyData_NotPersisted(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	data, err := s.service.GetPolicyData(ctx, policyTestOrgID, domain.PolicyMasterPWRequirements)
	require.NoError(t, err)
	assert.Nil(t, data)
}

func TestListEnabledPolicies(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	s.policyRepo.add(&domain.OrganizationPolicy{
		ID: 1, UUID: uuid.NewV4(), OrganizationID: policyTestOrgID,
		Type: domain.PolicyRequireTwoFactor, Enabled: true,
	})
	s.policyRepo.add(&domain.OrganizationPolicy{
		ID: 2, UUID: uuid.NewV4(), OrganizationID: policyTestOrgID,
		Type: domain.PolicyMasterPWRequirements, Enabled: false,
	})
	s.policyRepo.add(&domain.OrganizationPolicy{
		ID: 3, UUID: uuid.NewV4(), OrganizationID: policyTestOrgID,
		Type: domain.PolicySingleOrganization, Enabled: true,
	})

	policies, err := s.service.ListEnabledPolicies(ctx, policyTestOrgID)
	require.NoError(t, err)
	assert.Len(t, policies, 2)

	types := make(map[domain.PolicyType]bool)
	for _, p := range policies {
		types[p.Type] = true
		assert.True(t, p.Enabled)
	}
	assert.True(t, types[domain.PolicyRequireTwoFactor])
	assert.True(t, types[domain.PolicySingleOrganization])
}

func TestGetActivePolicySummary(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	s.policyRepo.add(&domain.OrganizationPolicy{
		ID: 1, UUID: uuid.NewV4(), OrganizationID: policyTestOrgID,
		Type: domain.PolicyRequireTwoFactor, Enabled: true,
		Data: domain.PolicyData{"grace_period_hours": float64(48)},
	})
	s.policyRepo.add(&domain.OrganizationPolicy{
		ID: 2, UUID: uuid.NewV4(), OrganizationID: policyTestOrgID,
		Type: domain.PolicyMasterPWRequirements, Enabled: false,
		Data: domain.PolicyData{"min_length": float64(12)},
	})

	summary, err := s.service.GetActivePolicySummary(ctx, policyTestOrgID, policyTestOwnerID)
	require.NoError(t, err)
	assert.Len(t, summary, 1)
	assert.Contains(t, summary, domain.PolicyRequireTwoFactor)
	assert.NotContains(t, summary, domain.PolicyMasterPWRequirements,
		"disabled policies should not appear in active summary")
}

func TestGetActivePolicySummary_NonMemberDenied(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	_, err := s.service.GetActivePolicySummary(ctx, policyTestOrgID, 9999)
	assert.ErrorIs(t, err, repository.ErrForbidden,
		"non-member should not access active policy summary")
}

func TestGetActivePolicySummary_MemberAllowed(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	s.policyRepo.add(&domain.OrganizationPolicy{
		ID: 1, UUID: uuid.NewV4(), OrganizationID: policyTestOrgID,
		Type: domain.PolicyRequireTwoFactor, Enabled: true,
	})

	summary, err := s.service.GetActivePolicySummary(ctx, policyTestOrgID, policyTestMemberID)
	require.NoError(t, err)
	assert.Len(t, summary, 1, "regular member should see active policies for their org")
}

// ─── Multi-Organization Isolation Tests ─────────────────────────────────────────

func TestPolicyIsolation_DifferentOrgsIndependent(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	otherOrgID := uint(2)
	otherOwnerID := uint(300)

	s.orgUserRepo.add(&domain.OrganizationUser{
		OrganizationID: otherOrgID,
		UserID:         otherOwnerID,
		Role:           domain.OrgRoleOwner,
		Status:         domain.OrgUserStatusAccepted,
	})
	s.subRepo.setOrgPlan(otherOrgID, "enterprise-yearly")

	// Enable policy in org 1
	_, err := s.service.UpdatePolicy(ctx, policyTestOrgID, policyTestOwnerID,
		domain.PolicyRequireTwoFactor, &domain.UpdateOrganizationPolicyRequest{Enabled: boolPtr(true)})
	require.NoError(t, err)

	// Policy should not be enabled in org 2
	enabled, err := s.service.IsPolicyEnabled(ctx, otherOrgID, domain.PolicyRequireTwoFactor)
	require.NoError(t, err)
	assert.False(t, enabled, "policies should be isolated between organizations")
}

func TestPolicyIsolation_CrossOrgAdminCannotAccess(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	otherOrgID := uint(2)
	s.subRepo.setOrgPlan(otherOrgID, "enterprise-yearly")

	// Owner of org 1 tries to access org 2 policies
	_, err := s.service.ListByOrganization(ctx, otherOrgID, policyTestOwnerID)
	assert.ErrorIs(t, err, repository.ErrForbidden,
		"admin of org 1 should not access org 2 policies")
}

// ─── Edge Cases ─────────────────────────────────────────────────────────────────

func TestUpdatePolicy_DoubleEnable_Idempotent(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	_, err := s.service.UpdatePolicy(ctx, policyTestOrgID, policyTestOwnerID,
		domain.PolicyRequireTwoFactor, &domain.UpdateOrganizationPolicyRequest{Enabled: boolPtr(true)})
	require.NoError(t, err)

	result, err := s.service.UpdatePolicy(ctx, policyTestOrgID, policyTestOwnerID,
		domain.PolicyRequireTwoFactor, &domain.UpdateOrganizationPolicyRequest{Enabled: boolPtr(true)})
	require.NoError(t, err)
	assert.True(t, result.Enabled, "double-enable should be idempotent")
}

func TestUpdatePolicy_DoubleDisable_Idempotent(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	result, err := s.service.UpdatePolicy(ctx, policyTestOrgID, policyTestOwnerID,
		domain.PolicyRequireTwoFactor, &domain.UpdateOrganizationPolicyRequest{Enabled: boolPtr(false)})
	require.NoError(t, err)
	assert.False(t, result.Enabled)
}

func TestListPolicies_MergesPersistedWithDefinitions(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	// Pre-persist one enabled policy
	s.policyRepo.add(&domain.OrganizationPolicy{
		ID: 1, UUID: uuid.NewV4(), OrganizationID: policyTestOrgID,
		Type: domain.PolicyRequireTwoFactor, Enabled: true,
	})

	policies, err := s.service.ListByOrganization(ctx, policyTestOrgID, policyTestOwnerID)
	require.NoError(t, err)

	allDefs := domain.AllPolicyDefinitions()
	assert.Equal(t, len(allDefs), len(policies),
		"should return one DTO per definition, not per persisted row")

	var enabledCount int
	for _, p := range policies {
		if p.Enabled {
			enabledCount++
		}
	}
	assert.Equal(t, 1, enabledCount, "only the pre-persisted policy should be enabled")
}

func TestUpdatePolicy_OwnerAndAdmin_BothCanManage(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	// Owner enables
	_, err := s.service.UpdatePolicy(ctx, policyTestOrgID, policyTestOwnerID,
		domain.PolicyRequireTwoFactor, &domain.UpdateOrganizationPolicyRequest{Enabled: boolPtr(true)})
	require.NoError(t, err)

	// Admin disables
	result, err := s.service.UpdatePolicy(ctx, policyTestOrgID, policyTestAdminID,
		domain.PolicyRequireTwoFactor, &domain.UpdateOrganizationPolicyRequest{Enabled: boolPtr(false)})
	require.NoError(t, err)
	assert.False(t, result.Enabled)
}

// ─── Plan Code Suffix Handling Tests ────────────────────────────────────────────

func TestListPolicies_PlanCodeWithMonthlySuffix(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("business-monthly")
	ctx := context.Background()

	policies, err := s.service.ListByOrganization(ctx, policyTestOrgID, policyTestOwnerID)
	require.NoError(t, err)
	assert.NotEmpty(t, policies, "business-monthly should resolve to business tier")

	for _, p := range policies {
		tier, _ := domain.GetPolicyTier(p.Type)
		assert.NotEqual(t, domain.PolicyTierEnterprise, tier)
	}
}

func TestListPolicies_PlanCodeWithYearlySuffix(t *testing.T) {
	t.Parallel()
	s := newPolicyTestSetup("enterprise-yearly")
	ctx := context.Background()

	policies, err := s.service.ListByOrganization(ctx, policyTestOrgID, policyTestOwnerID)
	require.NoError(t, err)
	assert.Equal(t, len(domain.AllPolicyDefinitions()), len(policies),
		"enterprise-yearly should resolve to enterprise tier and show all policies")
}
