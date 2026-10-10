package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/email"
	"github.com/passwall/passwall-server/internal/repository"
)

// Organization invitations (Invitation v2).
//
// organization_invitations is the single source of truth: a pending invitation
// holds a seat, and the organization_users row is created only when the
// invitee accepts. Every multi-row change runs in one transaction under an
// organization row lock; emails go out after commit.

const (
	defaultInvitationExpiryDays = 7
	maxInvitationExpiryDays     = 30
	invitationResendCooldown    = time.Minute
	maxInvitationSends          = 10
	invitationEmailTimeout      = 15 * time.Second
)

// Stable error codes returned to clients.
const (
	InvitationCodeNotFound        = "INVITATION_NOT_FOUND"
	InvitationCodeNotPending      = "INVITATION_NOT_PENDING"
	InvitationCodeExpired         = "INVITATION_EXPIRED"
	InvitationCodeExists          = "INVITATION_EXISTS"
	InvitationCodeAlreadyMember   = "ALREADY_MEMBER"
	InvitationCodeOrgKeyRequired  = "ORG_KEY_REQUIRED"
	InvitationCodePersonalVault   = "PERSONAL_VAULT"
	InvitationCodeInviteSelf      = "CANNOT_INVITE_SELF"
	InvitationCodeOwnerRole       = "OWNER_ROLE_FORBIDDEN"
	InvitationCodeInvalidRole     = "INVALID_ROLE"
	InvitationCodeResendTooSoon   = "RESEND_TOO_SOON"
	InvitationCodeResendLimit     = "RESEND_LIMIT_REACHED"
	InvitationCodeSingleOrgPolicy = "SINGLE_ORGANIZATION_POLICY"
	InvitationCodeInvalidOrgKey   = "INVALID_ORG_KEY"
)

// InvitationError is a typed invitation failure with an HTTP status.
type InvitationError struct {
	Code    string
	Message string
	Status  int
}

func (e *InvitationError) Error() string { return e.Message }

func invitationErr(status int, code, message string) error {
	return &InvitationError{Code: code, Message: message, Status: status}
}

// OrgInvitationDeps wires invitation storage, transactions and email.
type OrgInvitationDeps struct {
	Invitations  repository.OrganizationInvitationRepository
	Preferences  repository.PreferencesRepository
	TxManager    repository.TxManager
	EmailSender  email.Sender
	EmailBuilder *email.EmailBuilder
}

// OrganizationServiceOption configures optional organization service collaborators.
type OrganizationServiceOption func(*organizationService)

// WithOrganizationEntitlements enables entitlement checks.
func WithOrganizationEntitlements(entitlements OrganizationEntitlementService) OrganizationServiceOption {
	return func(s *organizationService) { s.entitlements = entitlements }
}

// WithOrganizationInvitations enables Invitation v2.
func WithOrganizationInvitations(deps OrgInvitationDeps) OrganizationServiceOption {
	return func(s *organizationService) { s.invites = &deps }
}

// InviteResult reports the created invitation and whether its email went out.
type InviteResult struct {
	Invitation *domain.OrganizationInvitation
	EmailSent  bool
}

func (s *organizationService) withinTx(ctx context.Context, fn func(context.Context) error) error {
	if s.invites == nil || s.invites.TxManager == nil {
		return fn(ctx)
	}
	return s.invites.TxManager.WithinTx(ctx, fn)
}

func (s *organizationService) requireInvites() error {
	if s.invites == nil || s.invites.Invitations == nil {
		return fmt.Errorf("organization invitations are not configured")
	}
	return nil
}

// invitationExpiry reads the organization's invitation_expiry_days setting.
func (s *organizationService) invitationExpiry(ctx context.Context, orgID uint, now time.Time) time.Time {
	days := defaultInvitationExpiryDays
	if s.invites != nil && s.invites.Preferences != nil {
		prefs, err := s.invites.Preferences.ListByOwner(ctx, domain.OrgSettingOwnerType, orgID, domain.OrgSettingSectionMembers)
		if err == nil {
			for _, p := range prefs {
				if p != nil && p.Key == domain.OrgSettingKeyInvitationExpiryDays {
					if n, convErr := strconv.Atoi(strings.TrimSpace(p.Value)); convErr == nil && n > 0 {
						days = min(n, maxInvitationExpiryDays)
					}
				}
			}
		}
	}
	return now.Add(time.Duration(days) * 24 * time.Hour)
}

// InviteMember creates a pending invitation and emails the invitee.
func (s *organizationService) InviteMember(ctx context.Context, orgID, inviterUserID uint, req *domain.CreateOrgInvitationRequest) (*InviteResult, error) {
	if err := s.requireInvites(); err != nil {
		return nil, err
	}
	req.Role = domain.NormalizeOrgRole(req.Role)
	if !isSupportedOrgRole(req.Role) {
		return nil, invitationErr(400, InvitationCodeInvalidRole, "invalid organization role")
	}
	inviteeEmail := domain.NormalizeInvitationEmail(req.Email)

	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		return nil, invitationErr(404, InvitationCodeNotFound, "organization not found")
	}
	if org.IsPersonal {
		return nil, invitationErr(400, InvitationCodePersonalVault, "members cannot be invited to a personal vault; create a shared organization")
	}
	inviterMembership, err := s.orgUserRepo.GetByOrgAndUser(ctx, orgID, inviterUserID)
	if err != nil || !inviterMembership.IsAdmin() {
		return nil, repository.ErrForbidden
	}
	if req.Role == domain.OrgRoleOwner && inviterMembership.Role != domain.OrgRoleOwner {
		return nil, invitationErr(403, InvitationCodeOwnerRole, "only an owner can invite another owner")
	}
	inviter, err := s.userRepo.GetByID(ctx, inviterUserID)
	if err != nil {
		return nil, fmt.Errorf("load inviter: %w", err)
	}
	if domain.NormalizeInvitationEmail(inviter.Email) == inviteeEmail {
		return nil, invitationErr(400, InvitationCodeInviteSelf, "you cannot invite yourself")
	}
	if s.entitlements != nil {
		if err := s.entitlements.Authorize(ctx, orgID, domain.CapabilityMemberInvite); err != nil {
			return nil, err
		}
	}

	invitee, err := s.userRepo.GetByEmail(ctx, inviteeEmail)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, fmt.Errorf("load invitee: %w", err)
	}

	now := time.Now()
	var invitation *domain.OrganizationInvitation
	err = s.withinTx(ctx, func(txCtx context.Context) error {
		if err := s.invites.Invitations.LockOrganization(txCtx, orgID); err != nil {
			return err
		}
		if invitee != nil {
			if _, err := s.orgUserRepo.GetByOrgAndUser(txCtx, orgID, invitee.ID); err == nil {
				return invitationErr(409, InvitationCodeAlreadyMember, "this person is already a member of the organization")
			}
			if err := s.checkSingleOrganizationPolicy(txCtx, orgID, invitee.ID); err != nil {
				if !errors.Is(err, ErrSingleOrganizationPolicy) {
					return err
				}
				return invitationErr(409, InvitationCodeSingleOrgPolicy, err.Error())
			}
		}
		if existing, err := s.invites.Invitations.GetPendingByOrgAndEmail(txCtx, orgID, inviteeEmail); err == nil {
			if existing.EffectiveStatus(now) == domain.OrgInvitationPending {
				return invitationErr(409, InvitationCodeExists, "an invitation is already pending for this email; resend it instead")
			}
			existing.Status = domain.OrgInvitationExpired
			if err := s.invites.Invitations.Update(txCtx, existing); err != nil {
				return fmt.Errorf("expire stale invitation: %w", err)
			}
		} else if !errors.Is(err, repository.ErrNotFound) {
			return fmt.Errorf("check pending invitation: %w", err)
		}
		if err := s.ensureSeatAvailable(txCtx, orgID); err != nil {
			return err
		}

		invitation = &domain.OrganizationInvitation{
			OrganizationID:  orgID,
			Email:           inviteeEmail,
			Role:            req.Role,
			AccessAll:       req.AccessAll,
			Status:          domain.OrgInvitationPending,
			InvitedByUserID: inviterUserID,
			ExpiresAt:       s.invitationExpiry(txCtx, orgID, now),
			LastSentAt:      &now,
			SendCount:       1,
		}
		// A registered invitee with a public key gets the org key right away.
		// Without one (no account yet, or no key pair because they never used
		// the web vault) the member is confirmed by an admin after accepting.
		if key := strings.TrimSpace(req.EncryptedOrgKey); invitee != nil && key != "" {
			invitation.EncryptedOrgKey = &key
		}
		return s.invites.Invitations.Create(txCtx, invitation)
	})
	if err != nil {
		return nil, err
	}

	invitation.Organization = org
	invitation.InvitedBy = inviter
	sent := s.sendInvitationEmail(ctx, invitation, invitee == nil)
	s.logger.Info("organization invitation created", "org_id", orgID, "invitation_id", invitation.ID, "requires_signup", invitee == nil, "email_sent", sent)
	return &InviteResult{Invitation: invitation, EmailSent: sent}, nil
}

func (s *organizationService) sendInvitationEmail(ctx context.Context, inv *domain.OrganizationInvitation, needsSignup bool) bool {
	if s.invites.EmailSender == nil || s.invites.EmailBuilder == nil || inv.Organization == nil || inv.InvitedBy == nil {
		return false
	}
	inviterName := strings.TrimSpace(inv.InvitedBy.Name)
	if inviterName == "" {
		inviterName = inv.InvitedBy.Email
	}
	message, err := s.invites.EmailBuilder.BuildOrgInvitationEmail(inv.Email, inviterName, inv.Organization.Name, string(inv.Role), inv.ExpiresAt, needsSignup)
	if err != nil {
		s.logger.Error("failed to build invitation email", "invitation_id", inv.ID, "error", err)
		return false
	}
	sendCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), invitationEmailTimeout)
	defer cancel()
	if err := s.invites.EmailSender.Send(sendCtx, message); err != nil {
		s.logger.Error("failed to send invitation email", "invitation_id", inv.ID, "error", err)
		return false
	}
	return true
}

// ListInvitations returns the organization's invitations for owners and admins.
// status "" returns all; "pending" also covers rows that just expired.
func (s *organizationService) ListInvitations(ctx context.Context, orgID, requestingUserID uint, status string) ([]*domain.OrganizationInvitation, error) {
	if err := s.requireInvites(); err != nil {
		return nil, err
	}
	if err := s.checkPermission(ctx, orgID, requestingUserID, true); err != nil {
		return nil, err
	}
	var statuses []domain.OrganizationInvitationStatus
	if status != "" && status != "all" {
		statuses = []domain.OrganizationInvitationStatus{domain.OrganizationInvitationStatus(status)}
	}
	return s.invites.Invitations.ListByOrganization(ctx, orgID, statuses)
}

// ResendInvitation re-sends a pending (or expired) invitation with a fresh expiry.
func (s *organizationService) ResendInvitation(ctx context.Context, orgID, invitationID, requestingUserID uint) (*InviteResult, error) {
	if err := s.requireInvites(); err != nil {
		return nil, err
	}
	if err := s.checkPermission(ctx, orgID, requestingUserID, true); err != nil {
		return nil, err
	}
	now := time.Now()
	var inv *domain.OrganizationInvitation
	err := s.withinTx(ctx, func(txCtx context.Context) error {
		if err := s.invites.Invitations.LockOrganization(txCtx, orgID); err != nil {
			return err
		}
		var err error
		inv, err = s.loadOrgInvitation(txCtx, orgID, invitationID)
		if err != nil {
			return err
		}
		switch inv.EffectiveStatus(now) {
		case domain.OrgInvitationPending:
		case domain.OrgInvitationExpired:
			// Re-opening an expired invitation takes a seat again.
			if existing, err := s.invites.Invitations.GetPendingByOrgAndEmail(txCtx, orgID, inv.Email); err == nil && existing.ID != inv.ID {
				return invitationErr(409, InvitationCodeExists, "another invitation is already pending for this email")
			}
			if err := s.ensureSeatAvailable(txCtx, orgID); err != nil {
				return err
			}
		default:
			return invitationErr(409, InvitationCodeNotPending, "only pending or expired invitations can be resent")
		}
		if inv.LastSentAt != nil && now.Sub(*inv.LastSentAt) < invitationResendCooldown {
			return invitationErr(429, InvitationCodeResendTooSoon, "wait a minute before resending this invitation")
		}
		if inv.SendCount >= maxInvitationSends {
			return invitationErr(429, InvitationCodeResendLimit, "this invitation was sent too many times; revoke it and invite again")
		}
		inv.Status = domain.OrgInvitationPending
		inv.ExpiresAt = s.invitationExpiry(txCtx, orgID, now)
		inv.LastSentAt = &now
		inv.SendCount++
		return s.invites.Invitations.Update(txCtx, inv)
	})
	if err != nil {
		return nil, err
	}
	_, lookupErr := s.userRepo.GetByEmail(ctx, inv.Email)
	sent := s.sendInvitationEmail(ctx, inv, errors.Is(lookupErr, repository.ErrNotFound))
	return &InviteResult{Invitation: inv, EmailSent: sent}, nil
}

// RevokeInvitation cancels a pending invitation and frees its seat.
func (s *organizationService) RevokeInvitation(ctx context.Context, orgID, invitationID, requestingUserID uint) (*domain.OrganizationInvitation, error) {
	if err := s.requireInvites(); err != nil {
		return nil, err
	}
	if err := s.checkPermission(ctx, orgID, requestingUserID, true); err != nil {
		return nil, err
	}
	now := time.Now()
	var inv *domain.OrganizationInvitation
	err := s.withinTx(ctx, func(txCtx context.Context) error {
		if err := s.invites.Invitations.LockOrganization(txCtx, orgID); err != nil {
			return err
		}
		var err error
		inv, err = s.loadOrgInvitation(txCtx, orgID, invitationID)
		if err != nil {
			return err
		}
		if inv.Status != domain.OrgInvitationPending {
			return invitationErr(409, InvitationCodeNotPending, "only pending invitations can be revoked")
		}
		inv.Status = domain.OrgInvitationRevoked
		inv.RespondedAt = &now
		inv.RevokedByUserID = &requestingUserID
		return s.invites.Invitations.Update(txCtx, inv)
	})
	return inv, err
}

func (s *organizationService) loadOrgInvitation(ctx context.Context, orgID, invitationID uint) (*domain.OrganizationInvitation, error) {
	inv, err := s.invites.Invitations.GetByID(ctx, invitationID)
	if errors.Is(err, repository.ErrNotFound) || (err == nil && inv.OrganizationID != orgID) {
		return nil, invitationErr(404, InvitationCodeNotFound, "invitation not found")
	}
	return inv, err
}

// ListReceivedInvitations returns pending invitations addressed to the user.
func (s *organizationService) ListReceivedInvitations(ctx context.Context, userID uint) ([]*domain.OrganizationInvitation, error) {
	if err := s.requireInvites(); err != nil {
		return nil, err
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load user: %w", err)
	}
	return s.invites.Invitations.ListPendingByEmail(ctx, user.Email, time.Now())
}

// loadReceivedInvitation returns the invitation if it is addressed to the user.
func (s *organizationService) loadReceivedInvitation(ctx context.Context, invitationID uint, user *domain.User) (*domain.OrganizationInvitation, error) {
	inv, err := s.invites.Invitations.GetByID(ctx, invitationID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, invitationErr(404, InvitationCodeNotFound, "invitation not found")
	}
	if err != nil {
		return nil, err
	}
	// Do not reveal invitations addressed to someone else.
	if domain.NormalizeInvitationEmail(inv.Email) != domain.NormalizeInvitationEmail(user.Email) {
		return nil, invitationErr(404, InvitationCodeNotFound, "invitation not found")
	}
	switch inv.EffectiveStatus(time.Now()) {
	case domain.OrgInvitationPending:
		return inv, nil
	case domain.OrgInvitationExpired:
		return nil, invitationErr(410, InvitationCodeExpired, "this invitation has expired; ask an administrator to resend it")
	default:
		return nil, invitationErr(409, InvitationCodeNotPending, "this invitation is no longer pending")
	}
}

// AcceptReceivedInvitation creates the membership. With a shared key the member
// is active immediately; without one they wait for admin confirmation.
func (s *organizationService) AcceptReceivedInvitation(ctx context.Context, invitationID, userID uint, encryptedOrgKey string) (*domain.OrganizationUser, error) {
	if err := s.requireInvites(); err != nil {
		return nil, err
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load user: %w", err)
	}
	// Resolve the organization outside the lock so the lock order stays
	// organization -> invitation for every writer.
	preview, err := s.loadReceivedInvitation(ctx, invitationID, user)
	if err != nil {
		return nil, err
	}
	orgID := preview.OrganizationID
	encryptedOrgKey = strings.TrimSpace(encryptedOrgKey)

	now := time.Now()
	var member *domain.OrganizationUser
	var inv *domain.OrganizationInvitation
	err = s.withinTx(ctx, func(txCtx context.Context) error {
		if err := s.invites.Invitations.LockOrganization(txCtx, orgID); err != nil {
			return err
		}
		var err error
		inv, err = s.loadReceivedInvitation(txCtx, invitationID, user)
		if err != nil {
			return err
		}
		if inv.HasOrgKey() {
			if encryptedOrgKey == "" {
				return invitationErr(400, InvitationCodeOrgKeyRequired, "unlock your vault to accept: the organization key must be re-encrypted for your account")
			}
			if !strings.HasPrefix(encryptedOrgKey, "2.") {
				return invitationErr(400, InvitationCodeInvalidOrgKey, "the organization key must be encrypted with your user key")
			}
		}
		if s.entitlements != nil {
			if err := s.entitlements.Authorize(txCtx, orgID, domain.CapabilityMemberInvite); err != nil {
				return err
			}
		}
		if err := s.checkSingleOrganizationPolicy(txCtx, orgID, userID); err != nil {
			if !errors.Is(err, ErrSingleOrganizationPolicy) {
				return err
			}
			return invitationErr(409, InvitationCodeSingleOrgPolicy, err.Error())
		}

		if existing, err := s.orgUserRepo.GetByOrgAndUser(txCtx, orgID, userID); err == nil && existing != nil {
			// Already a member (e.g. joined through SSO): close the invitation.
			member = existing
		} else {
			member = &domain.OrganizationUser{
				OrganizationID: orgID,
				UserID:         userID,
				Role:           inv.Role,
				AccessAll:      inv.AccessAll,
				InvitedAt:      &inv.CreatedAt,
				AcceptedAt:     &now,
			}
			if inv.HasOrgKey() {
				member.EncryptedOrgKey = encryptedOrgKey
				member.Status = domain.OrgUserStatusAccepted
			} else {
				member.Status = domain.OrgUserStatusProvisioned
			}
			if err := s.orgUserRepo.Create(txCtx, member); err != nil {
				return fmt.Errorf("create membership: %w", err)
			}
			if err := s.ensureOrgUserInDefaultTeam(txCtx, orgID, member.ID); err != nil {
				return fmt.Errorf("add to default team: %w", err)
			}
		}

		inv.Status = domain.OrgInvitationAccepted
		inv.RespondedAt = &now
		inv.AcceptedUserID = &userID
		return s.invites.Invitations.Update(txCtx, inv)
	})
	if err != nil {
		return nil, err
	}

	if member.Status == domain.OrgUserStatusProvisioned {
		s.notifyAdminsAwaitingConfirmation(ctx, orgID, user.Email)
	}
	s.logger.Info("organization invitation accepted", "org_id", orgID, "invitation_id", invitationID, "user_id", userID, "status", member.Status)
	return member, nil
}

// DeclineReceivedInvitation marks the invitation declined.
func (s *organizationService) DeclineReceivedInvitation(ctx context.Context, invitationID, userID uint) error {
	if err := s.requireInvites(); err != nil {
		return err
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("load user: %w", err)
	}
	preview, err := s.loadReceivedInvitation(ctx, invitationID, user)
	if err != nil {
		return err
	}
	now := time.Now()
	return s.withinTx(ctx, func(txCtx context.Context) error {
		if err := s.invites.Invitations.LockOrganization(txCtx, preview.OrganizationID); err != nil {
			return err
		}
		inv, err := s.loadReceivedInvitation(txCtx, invitationID, user)
		if err != nil {
			return err
		}
		inv.Status = domain.OrgInvitationDeclined
		inv.RespondedAt = &now
		return s.invites.Invitations.Update(txCtx, inv)
	})
}

// ExpireOverdueInvitations marks pending invitations past their expiry.
func (s *organizationService) ExpireOverdueInvitations(ctx context.Context) (int64, error) {
	if err := s.requireInvites(); err != nil {
		return 0, err
	}
	return s.invites.Invitations.ExpireOverdue(ctx, time.Now())
}

func (s *organizationService) notifyAdminsAwaitingConfirmation(ctx context.Context, orgID uint, memberEmail string) {
	if s.invites.EmailSender == nil || s.invites.EmailBuilder == nil {
		return
	}
	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		return
	}
	members, err := s.orgUserRepo.ListByOrganization(ctx, orgID)
	if err != nil {
		return
	}
	sendCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), invitationEmailTimeout)
	defer cancel()
	for _, m := range members {
		if m == nil || m.User == nil || !m.IsAdmin() {
			continue
		}
		if m.Status != domain.OrgUserStatusAccepted && m.Status != domain.OrgUserStatusConfirmed {
			continue
		}
		message, err := s.invites.EmailBuilder.BuildOrgMemberAwaitingConfirmationEmail(m.User.Email, org.Name, org.PublicID, memberEmail)
		if err != nil {
			continue
		}
		if err := s.invites.EmailSender.Send(sendCtx, message); err != nil {
			s.logger.Error("failed to notify admin about member awaiting confirmation", "org_id", orgID, "error", err)
		}
	}
}
