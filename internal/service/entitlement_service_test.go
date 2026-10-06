package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
)

type entitlementSubscriptionReaderStub struct {
	subscription *domain.Subscription
	err          error
}

func (s entitlementSubscriptionReaderStub) GetByOrganizationID(
	_ context.Context,
	_ uint,
) (*domain.Subscription, error) {
	return s.subscription, s.err
}

type entitlementPlanReaderStub struct {
	free *domain.Plan
}

func (s entitlementPlanReaderStub) GetByCode(_ context.Context, code string) (*domain.Plan, error) {
	if code == "free-monthly" && s.free != nil {
		return s.free, nil
	}
	return nil, errors.New("plan not found")
}

type entitlementUsageReaderStub struct {
	users       int
	collections int
}

func (s entitlementUsageReaderStub) GetByID(context.Context, uint) (*domain.Organization, error) {
	return &domain.Organization{PublicID: "public-org-7"}, nil
}

func (s entitlementUsageReaderStub) GetMemberCount(context.Context, uint) (int, error) {
	return s.users, nil
}

func (s entitlementUsageReaderStub) GetCollectionCount(context.Context, uint) (int, error) {
	return s.collections, nil
}

type entitlementItemCounterStub struct {
	items int
}

func (s entitlementItemCounterStub) CountByOrganizationID(context.Context, uint) (int, error) {
	return s.items, nil
}

func intPointer(value int) *int {
	return &value
}

func newEntitlementServiceForTest(
	subscription *domain.Subscription,
	free *domain.Plan,
	usage domain.EntitlementUsage,
) *organizationEntitlementService {
	service := NewOrganizationEntitlementService(
		entitlementSubscriptionReaderStub{subscription: subscription},
		entitlementPlanReaderStub{free: free},
		entitlementUsageReaderStub{users: usage.Users, collections: usage.Collections},
		entitlementItemCounterStub{items: usage.Items},
		nil,
		"https://vault.passwall.io",
		WithEntitlementEnforcementModes("*=enforce", nil),
	).(*organizationEntitlementService)
	service.now = func() time.Time {
		return time.Date(2026, 10, 6, 20, 0, 0, 0, time.UTC)
	}
	return service
}

func TestOrganizationEntitlementService_Resolve(t *testing.T) {
	free := &domain.Plan{
		Code:           "free-monthly",
		PriceCents:     0,
		MaxUsers:       intPointer(1),
		MaxCollections: intPointer(10),
		MaxItems:       intPointer(100),
		ExpiryBehavior: domain.ExpiryBehaviorDowngradeToFree,
	}
	pro := &domain.Plan{
		Code:           "pro-yearly",
		PriceCents:     1900,
		ExpiryBehavior: domain.ExpiryBehaviorDowngradeToFree,
		Features: domain.PlanFeatures{
			Passkeys: true,
		},
	}
	team := &domain.Plan{
		Code:           "team-yearly",
		PriceCents:     2990,
		ExpiryBehavior: domain.ExpiryBehaviorFreeze,
		Features: domain.PlanFeatures{
			Teams: true,
		},
	}
	now := time.Date(2026, 10, 6, 20, 0, 0, 0, time.UTC)
	future := now.Add(24 * time.Hour)
	past := now.Add(-24 * time.Hour)

	tests := []struct {
		name                    string
		subscription            *domain.Subscription
		usage                   domain.EntitlementUsage
		wantState               domain.AccessState
		wantPlan                string
		wantCreate              bool
		wantUpdate              bool
		wantAutofill            bool
		wantFeatureCapability   domain.Capability
		wantFeatureAvailability bool
	}{
		{
			name:         "free within quota",
			subscription: &domain.Subscription{State: domain.SubStateActive, Plan: free},
			usage:        domain.EntitlementUsage{Users: 1, Collections: 2, Items: 20},
			wantState:    domain.AccessStateFree,
			wantPlan:     "free-monthly",
			wantCreate:   true,
			wantUpdate:   true,
			wantAutofill: true,
		},
		{
			name:         "free at quota blocks create without over quota state",
			subscription: &domain.Subscription{State: domain.SubStateActive, Plan: free},
			usage:        domain.EntitlementUsage{Users: 1, Collections: 2, Items: 100},
			wantState:    domain.AccessStateFree,
			wantPlan:     "free-monthly",
			wantCreate:   false,
			wantUpdate:   true,
			wantAutofill: true,
		},
		{
			name:         "free over quota remains manageable",
			subscription: &domain.Subscription{State: domain.SubStateActive, Plan: free},
			usage:        domain.EntitlementUsage{Users: 1, Collections: 2, Items: 101},
			wantState:    domain.AccessStateFreeOverQuota,
			wantPlan:     "free-monthly",
			wantCreate:   false,
			wantUpdate:   true,
			wantAutofill: true,
		},
		{
			name:                    "expired personal paid plan resolves against free contract",
			subscription:            &domain.Subscription{State: domain.SubStateExpired, Plan: pro},
			usage:                   domain.EntitlementUsage{Users: 1, Collections: 2, Items: 20},
			wantState:               domain.AccessStateFree,
			wantPlan:                "free-monthly",
			wantCreate:              true,
			wantUpdate:              true,
			wantAutofill:            true,
			wantFeatureCapability:   domain.CapabilityPasskeyCreate,
			wantFeatureAvailability: false,
		},
		{
			name:                    "expired shared plan freezes mutations and autofill",
			subscription:            &domain.Subscription{State: domain.SubStateExpired, Plan: team},
			usage:                   domain.EntitlementUsage{Users: 4, Collections: 5, Items: 50},
			wantState:               domain.AccessStateFrozen,
			wantPlan:                "team-yearly",
			wantCreate:              false,
			wantUpdate:              false,
			wantAutofill:            false,
			wantFeatureCapability:   domain.CapabilityTeamsManage,
			wantFeatureAvailability: false,
		},
		{
			name:         "canceled plan remains paid through provider period",
			subscription: &domain.Subscription{State: domain.SubStateCanceled, Plan: team, RenewAt: &future},
			usage:        domain.EntitlementUsage{Users: 4, Collections: 5, Items: 50},
			wantState:    domain.AccessStatePaid,
			wantPlan:     "team-yearly",
			wantCreate:   true,
			wantUpdate:   true,
			wantAutofill: true,
		},
		{
			name:         "ended personal cancellation resolves to free",
			subscription: &domain.Subscription{State: domain.SubStateCanceled, Plan: pro, RenewAt: &past},
			usage:        domain.EntitlementUsage{Users: 1, Collections: 2, Items: 20},
			wantState:    domain.AccessStateFree,
			wantPlan:     "free-monthly",
			wantCreate:   true,
			wantUpdate:   true,
			wantAutofill: true,
		},
		{
			name:         "past due shared plan retains access inside grace",
			subscription: &domain.Subscription{State: domain.SubStatePastDue, Plan: team, GracePeriodEndsAt: &future},
			usage:        domain.EntitlementUsage{Users: 4, Collections: 5, Items: 50},
			wantState:    domain.AccessStatePaid,
			wantPlan:     "team-yearly",
			wantCreate:   true,
			wantUpdate:   true,
			wantAutofill: true,
		},
		{
			name:         "past due without grace fails closed",
			subscription: &domain.Subscription{State: domain.SubStatePastDue, Plan: team},
			usage:        domain.EntitlementUsage{Users: 4, Collections: 5, Items: 50},
			wantState:    domain.AccessStateFrozen,
			wantPlan:     "team-yearly",
			wantCreate:   false,
			wantUpdate:   false,
			wantAutofill: false,
		},
		{
			name:         "trial retains access until trial end",
			subscription: &domain.Subscription{State: domain.SubStateTrialing, Plan: pro, TrialEndsAt: &future},
			usage:        domain.EntitlementUsage{Users: 1, Collections: 2, Items: 20},
			wantState:    domain.AccessStatePaid,
			wantPlan:     "pro-yearly",
			wantCreate:   true,
			wantUpdate:   true,
			wantAutofill: true,
		},
		{
			name:         "trial without an end fails closed",
			subscription: &domain.Subscription{State: domain.SubStateTrialing, Plan: pro},
			usage:        domain.EntitlementUsage{Users: 1, Collections: 2, Items: 20},
			wantState:    domain.AccessStateFree,
			wantPlan:     "free-monthly",
			wantCreate:   true,
			wantUpdate:   true,
			wantAutofill: true,
		},
		{
			name:         "draft shared subscription fails closed",
			subscription: &domain.Subscription{State: domain.SubStateDraft, Plan: team},
			usage:        domain.EntitlementUsage{Users: 4, Collections: 5, Items: 50},
			wantState:    domain.AccessStateFrozen,
			wantPlan:     "team-yearly",
			wantCreate:   false,
			wantUpdate:   false,
			wantAutofill: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := newEntitlementServiceForTest(tt.subscription, free, tt.usage)
			snapshot, err := service.Resolve(context.Background(), 7)
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if snapshot.AccessState != tt.wantState {
				t.Fatalf("AccessState = %q, want %q", snapshot.AccessState, tt.wantState)
			}
			if snapshot.EffectivePlan != tt.wantPlan {
				t.Fatalf("EffectivePlan = %q, want %q", snapshot.EffectivePlan, tt.wantPlan)
			}
			if snapshot.ManageBillingURL != "https://vault.passwall.io/organizations/public-org-7/billing" {
				t.Fatalf("ManageBillingURL = %q", snapshot.ManageBillingURL)
			}
			assertCapability(t, snapshot, domain.CapabilityItemCreate, tt.wantCreate)
			assertCapability(t, snapshot, domain.CapabilityItemUpdate, tt.wantUpdate)
			assertCapability(t, snapshot, domain.CapabilityVaultAutofill, tt.wantAutofill)
			if tt.wantFeatureCapability != "" {
				assertCapability(
					t,
					snapshot,
					tt.wantFeatureCapability,
					tt.wantFeatureAvailability,
				)
			}
		})
	}
}

func TestOrganizationEntitlementService_AuthorizeReturnsStableErrors(t *testing.T) {
	team := &domain.Plan{
		Code:           "team-yearly",
		PriceCents:     2990,
		ExpiryBehavior: domain.ExpiryBehaviorFreeze,
	}
	service := newEntitlementServiceForTest(
		&domain.Subscription{State: domain.SubStateExpired, Plan: team},
		&domain.Plan{Code: "free-monthly"},
		domain.EntitlementUsage{},
	)

	err := service.Authorize(context.Background(), 7, domain.CapabilityItemUpdate)
	if !errors.Is(err, ErrAccountFrozen) {
		t.Fatalf("Authorize() error = %v, want ErrAccountFrozen", err)
	}
}

func newFrozenTeamEntitlementService(options ...OrganizationEntitlementOption) *organizationEntitlementService {
	team := &domain.Plan{
		Code:           "team-yearly",
		PriceCents:     2990,
		ExpiryBehavior: domain.ExpiryBehaviorFreeze,
		Features:       domain.PlanFeatures{Teams: true, SSO: true, Policies: true},
	}
	service := NewOrganizationEntitlementService(
		entitlementSubscriptionReaderStub{subscription: &domain.Subscription{
			State: domain.SubStateExpired,
			Plan:  team,
		}},
		entitlementPlanReaderStub{free: &domain.Plan{Code: "free-monthly"}},
		entitlementUsageReaderStub{},
		entitlementItemCounterStub{},
		nil,
		"https://vault.passwall.io",
		options...,
	).(*organizationEntitlementService)
	service.now = func() time.Time {
		return time.Date(2026, 10, 6, 20, 0, 0, 0, time.UTC)
	}
	return service
}

func TestOrganizationEntitlementService_DefaultModesKeepLegacyEnforcement(t *testing.T) {
	service := newFrozenTeamEntitlementService()
	ctx := context.Background()

	if err := service.Authorize(ctx, 7, domain.CapabilityItemUpdate); !errors.Is(err, ErrAccountFrozen) {
		t.Fatalf("Authorize(item.update) error = %v, want legacy enforcement ErrAccountFrozen", err)
	}
	if err := service.Authorize(ctx, 7, domain.CapabilityVaultAutofill); err != nil {
		t.Fatalf("Authorize(vault.autofill) error = %v, want nil while new gate is off", err)
	}

	snapshot, err := service.Resolve(ctx, 7)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got := snapshot.Capabilities[domain.CapabilityItemUpdate]; got.Allowed || got.Reason != domain.EntitlementReasonAccountFrozen {
		t.Fatalf("item.update decision = %+v, want enforced denial", got)
	}
	autofill := snapshot.Capabilities[domain.CapabilityVaultAutofill]
	if !autofill.Allowed || autofill.AdvisoryReason != domain.EntitlementReasonAccountFrozen {
		t.Fatalf("vault.autofill decision = %+v, want allowed with advisory reason", autofill)
	}
}

func TestOrganizationEntitlementService_WildcardModeOverridesLegacyDefault(t *testing.T) {
	service := newFrozenTeamEntitlementService(WithEntitlementEnforcementModes("*=off", nil))
	if err := service.Authorize(context.Background(), 7, domain.CapabilityItemUpdate); err != nil {
		t.Fatalf("Authorize(item.update) error = %v, want nil with *=off", err)
	}
	snapshot, err := service.Resolve(context.Background(), 7)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if !snapshot.Capabilities[domain.CapabilityItemUpdate].Allowed {
		t.Fatal("client snapshot must match server behavior when enforcement is off")
	}
}

func TestOrganizationEntitlementService_FrozenAllowsAccessReduction(t *testing.T) {
	service := newFrozenTeamEntitlementService(WithEntitlementEnforcementModes("*=enforce", nil))
	for _, capability := range []domain.Capability{
		domain.CapabilityMemberRemove,
		domain.CapabilityOrganizationDelete,
		domain.CapabilityAccessRevoke,
		domain.CapabilityPoliciesDisable,
		domain.CapabilitySCIMDeprovision,
	} {
		if err := service.Authorize(context.Background(), 7, capability); err != nil {
			t.Fatalf("Authorize(%s) error = %v, want allowed while frozen", capability, err)
		}
	}
	for _, capability := range []domain.Capability{
		domain.CapabilitySCIMManage,
		domain.CapabilityMemberInvite,
		domain.CapabilityPoliciesManage,
	} {
		if err := service.Authorize(context.Background(), 7, capability); !errors.Is(err, ErrAccountFrozen) {
			t.Fatalf("Authorize(%s) error = %v, want ErrAccountFrozen", capability, err)
		}
	}
}

func TestOrganizationEntitlementService_FreeOverQuotaReasonForPaidFeatures(t *testing.T) {
	free := &domain.Plan{Code: "free-monthly", MaxItems: intPointer(100)}
	service := newEntitlementServiceForTest(
		&domain.Subscription{State: domain.SubStateActive, Plan: free},
		free,
		domain.EntitlementUsage{Users: 1, Items: 101},
	)
	snapshot, err := service.Resolve(context.Background(), 7)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if snapshot.AccessState != domain.AccessStateFreeOverQuota {
		t.Fatalf("access_state = %s, want free_over_quota", snapshot.AccessState)
	}
	if got := snapshot.Capabilities[domain.CapabilityItemCreate].Reason; got != domain.EntitlementReasonPlanLimitReached {
		t.Fatalf("item.create reason = %s, want plan_limit_reached", got)
	}
	if got := snapshot.Capabilities[domain.CapabilityTeamsManage].Reason; got != domain.EntitlementReasonFeatureUnavailable {
		t.Fatalf("teams.manage reason = %s, want feature_not_available", got)
	}
}

func TestOrganizationEntitlementService_MissingSubscriptionIsTyped(t *testing.T) {
	service := NewOrganizationEntitlementService(
		entitlementSubscriptionReaderStub{err: repository.ErrNotFound},
		entitlementPlanReaderStub{},
		entitlementUsageReaderStub{},
		entitlementItemCounterStub{},
		nil,
		"",
	)
	if _, err := service.Resolve(context.Background(), 7); !errors.Is(err, ErrEntitlementSubscriptionUnavailable) {
		t.Fatalf("Resolve() error = %v, want ErrEntitlementSubscriptionUnavailable", err)
	}
}

func TestApplyEntitlementOverridesRejectsMalformedLimits(t *testing.T) {
	maxUsers := 6
	limits := domain.EntitlementLimits{MaxUsers: &maxUsers}
	features := domain.PlanFeatures{Passkeys: true}
	applyEntitlementOverrides(
		[]*domain.OrganizationEntitlementOverride{
			{
				Key:   "limit.max_users",
				Value: domain.EntitlementOverrideValue{"value": "unlimited"},
			},
			{
				Key:   "feature.passkeys",
				Value: domain.EntitlementOverrideValue{"value": "enabled"},
			},
		},
		&features,
		&limits,
	)

	if limits.MaxUsers == nil || *limits.MaxUsers != 6 {
		t.Fatalf("malformed override changed max users to %v", limits.MaxUsers)
	}
	if !features.Passkeys {
		t.Fatal("malformed override disabled passkeys")
	}
}

func TestOrganizationEntitlementService_RegistryConformance(t *testing.T) {
	personalFeatures := domain.PlanFeatures{
		Sharing: true, SharedItems: true, SecureSend: true, Passkeys: true,
		EmergencyAccess: true, SecurityInsights: true, BreachMonitoring: true,
	}
	teamFeatures := personalFeatures
	teamFeatures.Teams = true
	teamFeatures.Policies = true
	businessFeatures := teamFeatures
	businessFeatures.Audit = true
	businessFeatures.SSO = true
	businessFeatures.BusinessPolicies = true
	enterpriseFeatures := businessFeatures
	enterpriseFeatures.EnterprisePolicies = true

	free := &domain.Plan{
		Code: "free-monthly", MaxUsers: intPointer(1),
		MaxCollections: intPointer(10), MaxItems: intPointer(100),
		ExpiryBehavior: domain.ExpiryBehaviorDowngradeToFree,
	}
	plans := []*domain.Plan{
		free,
		{
			Code: "pro-monthly", PriceCents: 199, MaxUsers: intPointer(1),
			ExpiryBehavior: domain.ExpiryBehaviorDowngradeToFree,
			Features:       personalFeatures,
		},
		{
			Code: "family-monthly", PriceCents: 499, MaxUsers: intPointer(6),
			ExpiryBehavior: domain.ExpiryBehaviorFreeze,
			Features:       personalFeatures,
		},
		{
			Code: "team-monthly", PriceCents: 999, MaxUsers: intPointer(10),
			ExpiryBehavior: domain.ExpiryBehaviorFreeze,
			Features:       teamFeatures,
		},
		{
			Code: "business-monthly", PriceCents: 1999,
			ExpiryBehavior: domain.ExpiryBehaviorFreeze,
			Features:       businessFeatures,
		},
		{
			Code: "enterprise-yearly", PriceCents: 9999,
			ExpiryBehavior: domain.ExpiryBehaviorFreeze,
			Features:       enterpriseFeatures,
		},
	}
	states := []domain.SubscriptionState{domain.SubStateActive, domain.SubStateExpired}

	for _, plan := range plans {
		for _, state := range states {
			name := plan.Code + "/" + string(state)
			t.Run(name, func(t *testing.T) {
				service := newEntitlementServiceForTest(
					&domain.Subscription{State: state, Plan: plan},
					free,
					domain.EntitlementUsage{},
				)
				snapshot, err := service.Resolve(context.Background(), 7)
				if err != nil {
					t.Fatalf("Resolve() error = %v", err)
				}
				for capability, rule := range domain.CapabilityRegistry() {
					decision, exists := snapshot.Capabilities[capability]
					if !exists {
						t.Fatalf("missing capability %q", capability)
					}
					wantAllowed := containsAccessState(rule.AllowedStates, snapshot.AccessState)
					if rule.FeatureKey != "" {
						wantAllowed = wantAllowed &&
							entitlementFeatureEnabled(snapshot.Features, rule.FeatureKey)
					}
					if rule.LimitKey != "" {
						current, limit := entitlementLimit(snapshot, rule.LimitKey)
						wantAllowed = wantAllowed && !isAtLimit(current, limit)
					}
					if decision.Allowed != wantAllowed {
						t.Fatalf(
							"capability %q allowed = %v, want %v (access=%q)",
							capability,
							decision.Allowed,
							wantAllowed,
							snapshot.AccessState,
						)
					}
				}
			})
		}
	}
}

func assertCapability(
	t *testing.T,
	snapshot *domain.EntitlementSnapshot,
	capability domain.Capability,
	want bool,
) {
	t.Helper()
	decision, ok := snapshot.Capabilities[capability]
	if !ok {
		t.Fatalf("capability %q missing", capability)
	}
	if decision.Allowed != want {
		t.Fatalf("capability %q allowed = %v, want %v", capability, decision.Allowed, want)
	}
}
