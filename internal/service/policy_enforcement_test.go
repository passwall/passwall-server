package service

import (
	"context"
	"errors"
	"testing"

	"github.com/passwall/passwall-server/internal/domain"
)

type mockOrganizationPolicyService struct {
	enabledByType map[domain.PolicyType]bool
	dataByType    map[domain.PolicyType]domain.PolicyData
	enabledErr    error
	dataErr       error
}

func (m *mockOrganizationPolicyService) ListByOrganization(ctx context.Context, orgID, userID uint) ([]*domain.OrganizationPolicyDTO, error) {
	return nil, nil
}

func (m *mockOrganizationPolicyService) GetByType(ctx context.Context, orgID, userID uint, policyType domain.PolicyType) (*domain.OrganizationPolicyDTO, error) {
	return nil, nil
}

func (m *mockOrganizationPolicyService) UpdatePolicy(ctx context.Context, orgID, userID uint, policyType domain.PolicyType, req *domain.UpdateOrganizationPolicyRequest) (*domain.OrganizationPolicyDTO, error) {
	return nil, nil
}

func (m *mockOrganizationPolicyService) IsPolicyEnabled(ctx context.Context, orgID uint, policyType domain.PolicyType) (bool, error) {
	if m.enabledErr != nil {
		return false, m.enabledErr
	}
	return m.enabledByType[policyType], nil
}

func (m *mockOrganizationPolicyService) GetPolicyData(ctx context.Context, orgID uint, policyType domain.PolicyType) (domain.PolicyData, error) {
	if m.dataErr != nil {
		return nil, m.dataErr
	}
	return m.dataByType[policyType], nil
}

func (m *mockOrganizationPolicyService) ListEnabledPolicies(ctx context.Context, orgID uint) ([]*domain.OrganizationPolicyDTO, error) {
	return nil, nil
}

func (m *mockOrganizationPolicyService) GetActivePolicySummary(ctx context.Context, orgID, userID uint) (map[domain.PolicyType]domain.PolicyData, error) {
	return nil, nil
}

func TestPolicyEnforcement_CheckCardTypeAllowed(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	orgID := uint(1)

	t.Run("allows when policy disabled", func(t *testing.T) {
		t.Parallel()
		svc := NewPolicyEnforcementService(&mockOrganizationPolicyService{
			enabledByType: map[domain.PolicyType]bool{
				domain.PolicyRemoveCardType: false,
			},
		})

		if err := svc.CheckCardTypeAllowed(ctx, orgID); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("blocks when policy enabled", func(t *testing.T) {
		t.Parallel()
		svc := NewPolicyEnforcementService(&mockOrganizationPolicyService{
			enabledByType: map[domain.PolicyType]bool{
				domain.PolicyRemoveCardType: true,
			},
		})

		err := svc.CheckCardTypeAllowed(ctx, orgID)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestPolicyEnforcement_CheckPersonalVaultAllowed(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	orgID := uint(1)

	t.Run("owner bypasses policy", func(t *testing.T) {
		t.Parallel()
		svc := NewPolicyEnforcementService(&mockOrganizationPolicyService{
			enabledByType: map[domain.PolicyType]bool{
				domain.PolicyDisablePersonalVault: true,
			},
		})

		if err := svc.CheckPersonalVaultAllowed(ctx, orgID, domain.OrgRoleOwner); err != nil {
			t.Fatalf("expected no error for owner, got %v", err)
		}
	})

	t.Run("member blocked when enabled", func(t *testing.T) {
		t.Parallel()
		svc := NewPolicyEnforcementService(&mockOrganizationPolicyService{
			enabledByType: map[domain.PolicyType]bool{
				domain.PolicyDisablePersonalVault: true,
			},
		})

		err := svc.CheckPersonalVaultAllowed(ctx, orgID, domain.OrgRoleMember)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestPolicyEnforcement_GetPasswordExpirationPolicy(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	orgID := uint(1)

	t.Run("returns nil when no policy data", func(t *testing.T) {
		t.Parallel()
		svc := NewPolicyEnforcementService(&mockOrganizationPolicyService{
			dataByType: map[domain.PolicyType]domain.PolicyData{},
		})

		policy, err := svc.GetPasswordExpirationPolicy(ctx, orgID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if policy != nil {
			t.Fatalf("expected nil policy, got %+v", policy)
		}
	})

	t.Run("parses configured max age days", func(t *testing.T) {
		t.Parallel()
		svc := NewPolicyEnforcementService(&mockOrganizationPolicyService{
			dataByType: map[domain.PolicyType]domain.PolicyData{
				domain.PolicyPasswordExpiration: {"max_age_days": float64(120)},
			},
		})

		policy, err := svc.GetPasswordExpirationPolicy(ctx, orgID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if policy == nil || policy.MaxAgeDays != 120 {
			t.Fatalf("expected max_age_days=120, got %+v", policy)
		}
	})

	t.Run("falls back to default when non-positive", func(t *testing.T) {
		t.Parallel()
		svc := NewPolicyEnforcementService(&mockOrganizationPolicyService{
			dataByType: map[domain.PolicyType]domain.PolicyData{
				domain.PolicyPasswordExpiration: {"max_age_days": float64(0)},
			},
		})

		policy, err := svc.GetPasswordExpirationPolicy(ctx, orgID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if policy == nil || policy.MaxAgeDays != 90 {
			t.Fatalf("expected max_age_days default 90, got %+v", policy)
		}
	})

	t.Run("propagates policy data errors", func(t *testing.T) {
		t.Parallel()
		expectedErr := errors.New("policy data read failed")
		svc := NewPolicyEnforcementService(&mockOrganizationPolicyService{
			dataErr: expectedErr,
		})

		_, err := svc.GetPasswordExpirationPolicy(ctx, orgID)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
