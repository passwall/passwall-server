package gormrepo

import (
	"testing"

	"github.com/passwall/passwall-server/internal/domain"
)

func TestApplyCanonicalPlanContract(t *testing.T) {
	tests := []struct {
		code                   string
		maxUsers               *int
		maxItems               *int
		maxCollections         *int
		expiry                 domain.ExpiryBehavior
		teams, audit, sso      bool
		policies, api, support bool
		businessPolicies       bool
		enterprisePolicies     bool
	}{
		{"free-monthly", intValue(1), intValue(100), intValue(10), domain.ExpiryBehaviorDowngradeToFree, false, false, false, false, false, false, false, false},
		{"pro-yearly", intValue(1), nil, nil, domain.ExpiryBehaviorDowngradeToFree, false, false, false, false, false, false, false, false},
		{"family-monthly", intValue(6), nil, nil, domain.ExpiryBehaviorFreeze, false, false, false, false, false, false, false, false},
		{"team-yearly", intValue(10), nil, nil, domain.ExpiryBehaviorFreeze, true, false, false, true, false, false, false, false},
		{"business-monthly", nil, nil, nil, domain.ExpiryBehaviorFreeze, true, true, true, true, false, false, true, false},
		{"enterprise-yearly", nil, nil, nil, domain.ExpiryBehaviorFreeze, true, true, true, true, false, false, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			plan := &domain.Plan{Code: tt.code}
			applyCanonicalPlanContract(plan)
			assertOptionalInt(t, "max users", plan.MaxUsers, tt.maxUsers)
			assertOptionalInt(t, "max items", plan.MaxItems, tt.maxItems)
			assertOptionalInt(t, "max collections", plan.MaxCollections, tt.maxCollections)
			if plan.MaxDevices != nil {
				t.Fatal("max devices must be unlimited")
			}
			if plan.ExpiryBehavior != tt.expiry {
				t.Fatalf("expiry behavior = %q, want %q", plan.ExpiryBehavior, tt.expiry)
			}
			if plan.Features.Teams != tt.teams ||
				plan.Features.Audit != tt.audit ||
				plan.Features.SSO != tt.sso ||
				plan.Features.Policies != tt.policies ||
				plan.Features.APIAccess != tt.api ||
				plan.Features.PrioritySupport != tt.support ||
				plan.Features.BusinessPolicies != tt.businessPolicies ||
				plan.Features.EnterprisePolicies != tt.enterprisePolicies {
				t.Fatalf("unexpected feature contract: %+v", plan.Features)
			}
		})
	}
}

func assertOptionalInt(t *testing.T, name string, got, want *int) {
	t.Helper()
	if got == nil || want == nil {
		if got != nil || want != nil {
			t.Fatalf("%s = %v, want %v", name, got, want)
		}
		return
	}
	if *got != *want {
		t.Fatalf("%s = %d, want %d", name, *got, *want)
	}
}
