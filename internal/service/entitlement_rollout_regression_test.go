package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
)

func newDowngradedPolicyService(
	t *testing.T,
	entitlements *entitlementAuthorizationStub,
) (OrganizationPolicyService, *fakePolicyRepo) {
	t.Helper()
	setup := newPolicyTestSetup("team-monthly")
	setup.policyRepo.add(&domain.OrganizationPolicy{
		OrganizationID: policyTestOrgID,
		Type:           domain.PolicyFirewallRules,
		Enabled:        true,
		Data:           domain.PolicyData{},
	})
	service := NewOrganizationPolicyService(
		setup.policyRepo,
		setup.orgUserRepo,
		setup.subRepo,
		noopLogger{},
		entitlements,
	)
	return service, setup.policyRepo
}

func TestListPoliciesKeepsEnforcedPoliciesVisibleAfterDowngrade(t *testing.T) {
	entitlements := &entitlementAuthorizationStub{
		snapshot: &domain.EntitlementSnapshot{
			Features: domain.PlanFeatures{Policies: true},
		},
	}
	service, _ := newDowngradedPolicyService(t, entitlements)

	policies, err := service.ListByOrganization(context.Background(), policyTestOrgID, policyTestOwnerID)
	if err != nil {
		t.Fatalf("ListByOrganization() error = %v", err)
	}
	var found bool
	for _, policy := range policies {
		if policy.Type == domain.PolicyFirewallRules {
			found = policy.Enabled
		}
		if policy.Type == domain.PolicySingleOrganization {
			t.Fatal("unentitled and disabled business policy must stay hidden")
		}
	}
	if !found {
		t.Fatal("enabled business policy disappeared after downgrade")
	}
}

func TestDisablingPolicyOnlyRequiresDisableCapability(t *testing.T) {
	entitlements := &entitlementAuthorizationStub{
		errors: map[domain.Capability]error{
			domain.CapabilityBusinessPoliciesManage: ErrFeatureNotAvailable,
		},
	}
	service, repo := newDowngradedPolicyService(t, entitlements)
	ctx := context.Background()

	if _, err := service.UpdatePolicy(ctx, policyTestOrgID, policyTestOwnerID, domain.PolicyFirewallRules,
		&domain.UpdateOrganizationPolicyRequest{Enabled: boolPtr(false)}); err != nil {
		t.Fatalf("disable UpdatePolicy() error = %v", err)
	}
	if len(entitlements.calls) != 1 || entitlements.calls[0] != domain.CapabilityPoliciesDisable {
		t.Fatalf("disable authorization calls = %v, want policies.disable", entitlements.calls)
	}
	policy, err := repo.GetByOrgAndType(ctx, policyTestOrgID, domain.PolicyFirewallRules)
	if err != nil || policy.Enabled {
		t.Fatalf("policy after disable = %+v, err = %v", policy, err)
	}

	_, err = service.UpdatePolicy(ctx, policyTestOrgID, policyTestOwnerID, domain.PolicyFirewallRules,
		&domain.UpdateOrganizationPolicyRequest{Enabled: boolPtr(true)})
	if !errors.Is(err, ErrFeatureNotAvailable) {
		t.Fatalf("re-enable UpdatePolicy() error = %v, want ErrFeatureNotAvailable", err)
	}
}

type scimOrgUserRepoStub struct {
	repository.OrganizationUserRepository
	orgUser     *domain.OrganizationUser
	updated     *domain.OrganizationUser
	deleteCalls int
}

func (r *scimOrgUserRepoStub) GetByOrgAndUser(context.Context, uint, uint) (*domain.OrganizationUser, error) {
	clone := *r.orgUser
	return &clone, nil
}

func (r *scimOrgUserRepoStub) Update(_ context.Context, orgUser *domain.OrganizationUser) error {
	r.updated = orgUser
	return nil
}

func (r *scimOrgUserRepoStub) Delete(context.Context, uint) error {
	r.deleteCalls++
	return nil
}

func frozenSCIMEntitlements() *entitlementAuthorizationStub {
	return &entitlementAuthorizationStub{
		errors: map[domain.Capability]error{
			domain.CapabilitySCIMManage:   ErrAccountFrozen,
			domain.CapabilityMemberInvite: ErrAccountFrozen,
		},
	}
}

func TestSCIMDeprovisioningWorksWhileFrozen(t *testing.T) {
	ctx := context.Background()
	newService := func(repo *scimOrgUserRepoStub, entitlements *entitlementAuthorizationStub) *scimService {
		return &scimService{orgUserRepo: repo, entitlements: entitlements, logger: noopLogger{}}
	}

	deleteRepo := &scimOrgUserRepoStub{orgUser: &domain.OrganizationUser{ID: 3, Status: domain.OrgUserStatusConfirmed}}
	if err := newService(deleteRepo, frozenSCIMEntitlements()).DeleteUser(ctx, 7, "11"); err != nil {
		t.Fatalf("DeleteUser() error = %v, want deprovisioning allowed", err)
	}
	if deleteRepo.deleteCalls != 1 {
		t.Fatalf("DeleteUser() delete calls = %d, want 1", deleteRepo.deleteCalls)
	}

	suspendRepo := &scimOrgUserRepoStub{orgUser: &domain.OrganizationUser{ID: 3, Status: domain.OrgUserStatusConfirmed}}
	_, err := newService(suspendRepo, frozenSCIMEntitlements()).PatchUser(ctx, 7, "11", &domain.SCIMPatchOp{
		Operations: []domain.SCIMPatchOpItem{{Op: "replace", Path: "active", Value: false}},
	})
	if err != nil {
		t.Fatalf("PatchUser(active=false) error = %v, want suspension allowed", err)
	}
	if suspendRepo.updated == nil || suspendRepo.updated.Status != domain.OrgUserStatusSuspended {
		t.Fatalf("PatchUser(active=false) persisted %+v, want suspended", suspendRepo.updated)
	}

	reactivateRepo := &scimOrgUserRepoStub{orgUser: &domain.OrganizationUser{ID: 3, Status: domain.OrgUserStatusSuspended}}
	_, err = newService(reactivateRepo, frozenSCIMEntitlements()).UpdateUser(ctx, 7, "11", &domain.SCIMUser{Active: true})
	if !errors.Is(err, ErrAccountFrozen) {
		t.Fatalf("UpdateUser(active=true) error = %v, want ErrAccountFrozen", err)
	}
	if reactivateRepo.updated != nil {
		t.Fatal("frozen organization reactivated a SCIM user")
	}
}

func TestWebhookReservationStates(t *testing.T) {
	ctx := context.Background()
	failure := "boom"
	tests := []struct {
		name     string
		stored   *domain.WebhookEvent
		wantSkip bool
		wantErr  error
	}{
		{name: "new event is reserved"},
		{
			name:     "processed event is skipped",
			stored:   &domain.WebhookEvent{StripeEventID: "evt", CreatedAt: time.Now(), ProcessedAt: ptrTime(time.Now())},
			wantSkip: true,
		},
		{
			name:    "in-flight event asks provider to retry",
			stored:  &domain.WebhookEvent{StripeEventID: "evt", CreatedAt: time.Now()},
			wantErr: ErrWebhookInProgress,
		},
		{
			name:   "stale reservation is reprocessed",
			stored: &domain.WebhookEvent{StripeEventID: "evt", CreatedAt: time.Now().Add(-webhookReservationTTL - time.Minute)},
		},
		{
			name:   "failed event is reprocessed",
			stored: &domain.WebhookEvent{StripeEventID: "evt", CreatedAt: time.Now(), Error: &failure},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newWebhookEventRepoStub()
			if tt.stored != nil {
				repo.events["evt"] = tt.stored
			}
			skip, err := reserveWebhookEvent(ctx, repo, "evt", "test")
			if !errors.Is(err, tt.wantErr) || skip != tt.wantSkip {
				t.Fatalf("reserveWebhookEvent() = (%v, %v), want (%v, %v)", skip, err, tt.wantSkip, tt.wantErr)
			}
		})
	}
}

func TestHandleProviderCanceledExpiresWhenProviderAlreadyEnded(t *testing.T) {
	renewAt := time.Now().Add(20 * 24 * time.Hour)
	endedAt := time.Now().Add(-time.Minute)
	plan := &domain.Plan{ID: 2, Code: "pro-monthly", ExpiryBehavior: domain.ExpiryBehaviorDowngradeToFree}
	repo := &lifecycleSubscriptionRepo{sub: &domain.Subscription{
		ID: 9, OrganizationID: 7, PlanID: plan.ID,
		State: domain.SubStateActive, Plan: plan, RenewAt: &renewAt,
	}}
	service := NewSubscriptionService(
		repo, lifecyclePlanRepo{free: &domain.Plan{ID: 1, Code: "free-monthly"}},
		nil, nil, nil, nil, nil, nil,
	)

	if err := service.HandleProviderCanceled(context.Background(), 9, &endedAt); err != nil {
		t.Fatalf("HandleProviderCanceled() error = %v", err)
	}
	if repo.sub.State != domain.SubStateExpired {
		t.Fatalf("immediately ended subscription kept access: %+v", repo.sub)
	}
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
