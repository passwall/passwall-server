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

// CheckJoinPolicies reports whether organization policies allow userID to
// become a member of orgID.
func (s *organizationService) CheckJoinPolicies(ctx context.Context, orgID, userID uint) error {
	return s.checkSingleOrganizationPolicy(ctx, orgID, userID)
}

// ProvisionMember adds userID to orgID as a "provisioned" member (SSO JIT):
// no org key yet, an admin confirms them later. The seat check and the insert
// run under the organization lock, like invitation acceptance, so concurrent
// sign-ins cannot exceed the plan's seats.
func (s *organizationService) ProvisionMember(ctx context.Context, orgID, userID uint) (*domain.OrganizationUser, error) {
	var member *domain.OrganizationUser
	err := s.withinTx(ctx, func(txCtx context.Context) error {
		if s.invites != nil && s.invites.Invitations != nil {
			if err := s.invites.Invitations.LockOrganization(txCtx, orgID); err != nil {
				return err
			}
		}
		existing, err := s.orgUserRepo.GetByOrgAndUser(txCtx, orgID, userID)
		if err == nil && existing != nil {
			member = existing
			return nil
		}
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			return err
		}
		if s.entitlements != nil {
			if err := s.entitlements.Authorize(txCtx, orgID, domain.CapabilityMemberInvite); err != nil {
				return err
			}
		}
		if err := s.checkSingleOrganizationPolicy(txCtx, orgID, userID); err != nil {
			return err
		}
		now := time.Now()
		member = &domain.OrganizationUser{
			UUID:            uuid.New(),
			OrganizationID:  orgID,
			UserID:          userID,
			Role:            domain.OrgRoleMember,
			EncryptedOrgKey: "pending_key_exchange",
			Status:          domain.OrgUserStatusProvisioned,
			InvitedAt:       &now,
		}
		if err := s.orgUserRepo.Create(txCtx, member); err != nil {
			return fmt.Errorf("failed to create membership: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return member, nil
}
