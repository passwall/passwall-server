package service

import (
	"context"
	"errors"
	"testing"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
)

type deviceLimitTokenRepoStub struct {
	repository.TokenRepository
	activeSessions int
	countCalled    bool
}

func (s *deviceLimitTokenRepoStub) CountActiveSessionsByUserID(
	context.Context,
	int,
) (int, error) {
	s.countCalled = true
	return s.activeSessions, nil
}

type deviceLimitEntitlementStub struct {
	snapshot *domain.EntitlementSnapshot
}

func (s deviceLimitEntitlementStub) Resolve(
	context.Context,
	uint,
) (*domain.EntitlementSnapshot, error) {
	return s.snapshot, nil
}

func (deviceLimitEntitlementStub) Authorize(
	context.Context,
	uint,
	domain.Capability,
) error {
	return nil
}

func TestEnforceDeviceLimitUsesEntitlementLimit(t *testing.T) {
	user := &domain.User{PersonalOrganizationID: 7}

	t.Run("unlimited skips session counting", func(t *testing.T) {
		tokens := &deviceLimitTokenRepoStub{activeSessions: 50}
		service := &authService{
			tokenRepo: tokens,
			entitlements: deviceLimitEntitlementStub{snapshot: &domain.EntitlementSnapshot{
				Limits: domain.EntitlementLimits{MaxDevices: nil},
			}},
		}

		if err := service.enforceDeviceLimit(context.Background(), user); err != nil {
			t.Fatalf("enforceDeviceLimit() error = %v", err)
		}
		if tokens.countCalled {
			t.Fatal("unlimited entitlement should not count active sessions")
		}
	})

	t.Run("finite override enforces active session count", func(t *testing.T) {
		maxDevices := 1
		tokens := &deviceLimitTokenRepoStub{activeSessions: 1}
		service := &authService{
			tokenRepo: tokens,
			entitlements: deviceLimitEntitlementStub{snapshot: &domain.EntitlementSnapshot{
				Limits: domain.EntitlementLimits{MaxDevices: &maxDevices},
			}},
			orgUserRepo: newFakeOrgUserRepo(),
		}

		err := service.enforceDeviceLimit(context.Background(), user)
		if !errors.Is(err, ErrDeviceLimit) {
			t.Fatalf("enforceDeviceLimit() error = %v, want ErrDeviceLimit", err)
		}
	})
}
