package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	uuid "github.com/satori/go.uuid"
	"gorm.io/gorm"
)

type organizationService struct {
	orgRepo            repository.OrganizationRepository
	orgUserRepo        repository.OrganizationUserRepository
	userRepo           repository.UserRepository
	teamRepo           repository.TeamRepository
	teamUserRepo       repository.TeamUserRepository
	collectionRepo     repository.CollectionRepository
	collectionTeamRepo repository.CollectionTeamRepository
	paymentService     PaymentService
	invitationService  InvitationService
	subRepo            interface {
		Create(ctx context.Context, sub *domain.Subscription) error
		GetByOrganizationID(ctx context.Context, orgID uint) (*domain.Subscription, error)
	}
	planRepo interface {
		GetByCode(ctx context.Context, code string) (*domain.Plan, error)
	}
	logger Logger
}

// NewOrganizationService creates a new organization service
func NewOrganizationService(
	orgRepo repository.OrganizationRepository,
	orgUserRepo repository.OrganizationUserRepository,
	userRepo repository.UserRepository,
	teamRepo repository.TeamRepository,
	teamUserRepo repository.TeamUserRepository,
	collectionRepo repository.CollectionRepository,
	collectionTeamRepo repository.CollectionTeamRepository,
	paymentService PaymentService,
	invitationService InvitationService,
	subRepo interface {
		Create(ctx context.Context, sub *domain.Subscription) error
		GetByOrganizationID(ctx context.Context, orgID uint) (*domain.Subscription, error)
	},
	planRepo interface {
		GetByCode(ctx context.Context, code string) (*domain.Plan, error)
	},
	logger Logger,
) OrganizationService {
	return &organizationService{
		orgRepo:            orgRepo,
		orgUserRepo:        orgUserRepo,
		userRepo:           userRepo,
		teamRepo:           teamRepo,
		teamUserRepo:       teamUserRepo,
		collectionRepo:     collectionRepo,
		collectionTeamRepo: collectionTeamRepo,
		paymentService:     paymentService,
		invitationService:  invitationService,
		subRepo:            subRepo,
		planRepo:           planRepo,
		logger:             logger,
	}
}

const (
	defaultTeamName       = "All Members"
	defaultTeamDesc       = "System default team (cannot be deleted)"
	defaultCollectionName = "General"
	defaultCollectionDesc = "System default collection (cannot be deleted)"
)

func (s *organizationService) ensureDefaultTeam(ctx context.Context, orgID uint) (*domain.Team, error) {
	team, err := s.teamRepo.GetDefaultByOrganization(ctx, orgID)
	if err == nil && team != nil {
		return team, nil
	}
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}

	team = &domain.Team{
		OrganizationID:       orgID,
		Name:                 defaultTeamName,
		Description:          defaultTeamDesc,
		AccessAllCollections: false,
		IsDefault:            true,
		ExternalID:           nil, // reserved for LDAP/AD sync
	}

	if err := s.teamRepo.Create(ctx, team); err != nil {
		return nil, err
	}

	return team, nil
}

func (s *organizationService) ensureDefaultCollection(ctx context.Context, orgID uint) (*domain.Collection, error) {
	col, err := s.collectionRepo.GetDefaultByOrganization(ctx, orgID)
	if err == nil && col != nil {
		return col, nil
	}
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}

	col = &domain.Collection{
		OrganizationID: orgID,
		Name:           defaultCollectionName,
		Description:    defaultCollectionDesc,
		IsPrivate:      false,
		IsDefault:      true,
		ExternalID:     nil, // reserved for LDAP/AD sync
	}

	if err := s.collectionRepo.Create(ctx, col); err != nil {
		return nil, err
	}

	return col, nil
}

func (s *organizationService) ensureDefaultCollectionTeamAccess(ctx context.Context, collectionID uint, teamID uint) error {
	existing, err := s.collectionTeamRepo.GetByCollectionAndTeam(ctx, collectionID, teamID)
	if err == nil && existing != nil {
		return nil
	}
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return err
	}

	return s.collectionTeamRepo.Create(ctx, &domain.CollectionTeam{
		CollectionID:  collectionID,
		TeamID:        teamID,
		CanRead:       true,
		CanWrite:      false,
		CanAdmin:      false,
		HidePasswords: false,
	})
}

func (s *organizationService) ensureOrgUserInDefaultTeam(ctx context.Context, orgID uint, orgUserID uint) error {
	team, err := s.ensureDefaultTeam(ctx, orgID)
	if err != nil {
		return err
	}

	existing, err := s.teamUserRepo.GetByTeamAndOrgUser(ctx, team.ID, orgUserID)
	if err == nil && existing != nil {
		return nil
	}
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return err
	}

	return s.teamUserRepo.Create(ctx, &domain.TeamUser{
		TeamID:             team.ID,
		OrganizationUserID: orgUserID,
		IsManager:          false,
	})
}

func (s *organizationService) Create(ctx context.Context, userID uint, req *domain.CreateOrganizationRequest) (*domain.Organization, error) {
	creator, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, repository.ErrForbidden
	}
	creatorEmail := creator.Email
	creatorName := creator.Name

	// Create organization (plan limits are derived from subscriptions + plans)
	org := &domain.Organization{
		Name:               req.Name,
		BillingEmail:       req.BillingEmail,
		EncryptedOrgKey:    req.EncryptedOrgKey,
		IsActive:           true,
		CreatedByUserID:    &userID,
		CreatedByUserEmail: &creatorEmail,
		CreatedByUserName:  &creatorName,
	}

	if err := s.orgRepo.Create(ctx, org); err != nil {
		s.logger.Error("failed to create organization", "error", err)
		return nil, fmt.Errorf("failed to create organization: %w", err)
	}

	// Add creator as owner
	now := time.Now()
	orgUser := &domain.OrganizationUser{
		OrganizationID:  org.ID,
		UserID:          userID,
		Role:            domain.OrgRoleOwner,
		EncryptedOrgKey: req.EncryptedOrgKey, // Owner's copy of org key
		AccessAll:       true,
		Status:          domain.OrgUserStatusConfirmed,
		InvitedAt:       &now,
		AcceptedAt:      &now,
	}

	if err := s.orgUserRepo.Create(ctx, orgUser); err != nil {
		s.logger.Error("failed to add owner to organization", "org_id", org.ID, "user_id", userID, "error", err)
		// Rollback: delete organization
		_ = s.orgRepo.Delete(ctx, org.ID)
		return nil, fmt.Errorf("failed to add owner: %w", err)
	}

	// Ensure system default Team + Collection exist (and owner is in default team).
	// These defaults MUST NOT use ExternalID to keep future LDAP/AD sync safe.
	defTeam, err := s.ensureDefaultTeam(ctx, org.ID)
	if err != nil {
		_ = s.orgRepo.Delete(ctx, org.ID)
		return nil, fmt.Errorf("failed to ensure default team: %w", err)
	}
	defCol, err := s.ensureDefaultCollection(ctx, org.ID)
	if err != nil {
		_ = s.orgRepo.Delete(ctx, org.ID)
		return nil, fmt.Errorf("failed to ensure default collection: %w", err)
	}
	if err := s.ensureDefaultCollectionTeamAccess(ctx, defCol.ID, defTeam.ID); err != nil {
		_ = s.orgRepo.Delete(ctx, org.ID)
		return nil, fmt.Errorf("failed to ensure default collection access: %w", err)
	}
	if err := s.ensureOrgUserInDefaultTeam(ctx, org.ID, orgUser.ID); err != nil {
		_ = s.orgRepo.Delete(ctx, org.ID)
		return nil, fmt.Errorf("failed to ensure default team membership: %w", err)
	}

	// Ensure every organization has a subscription row (source-of-truth invariant).
	// New orgs created after initial seeding MUST get a default free subscription.
	const freePlanCode = "free-monthly"
	if _, err := s.subRepo.GetByOrganizationID(ctx, org.ID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			freePlan, err := s.planRepo.GetByCode(ctx, freePlanCode)
			if err != nil || freePlan == nil {
				// Rollback: delete organization
				_ = s.orgRepo.Delete(ctx, org.ID)
				return nil, fmt.Errorf("failed to load free plan (%s): %w", freePlanCode, err)
			}

			now := time.Now()
			sub := &domain.Subscription{
				UUID:           uuid.NewV4(),
				OrganizationID: org.ID,
				PlanID:         freePlan.ID,
				State:          domain.SubStateActive,
				StartedAt:      &now,
			}

			if err := s.subRepo.Create(ctx, sub); err != nil {
				// Rollback: delete organization
				_ = s.orgRepo.Delete(ctx, org.ID)
				return nil, fmt.Errorf("failed to create default subscription: %w", err)
			}
		} else {
			// Unexpected DB error reading subscription
			_ = s.orgRepo.Delete(ctx, org.ID)
			return nil, fmt.Errorf("failed to ensure default subscription: %w", err)
		}
	}

	s.logger.Info("organization created", "org_id", org.ID, "owner_id", userID, "name", org.Name)
	return org, nil
}

func (s *organizationService) GetByID(ctx context.Context, id uint, userID uint) (*domain.Organization, error) {
	// Get user's membership (contains their encrypted org key copy)
	orgUser, err := s.orgUserRepo.GetByOrgAndUser(ctx, id, userID)
	if err != nil {
		return nil, repository.ErrForbidden
	}

	org, err := s.orgRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	// Fetch stats
	if memberCount, err := s.orgRepo.GetMemberCount(ctx, org.ID); err == nil {
		org.MemberCount = &memberCount
	}

	if teamCount, err := s.orgRepo.GetTeamCount(ctx, org.ID); err == nil {
		org.TeamCount = &teamCount
	}

	if collectionCount, err := s.orgRepo.GetCollectionCount(ctx, org.ID); err == nil {
		org.CollectionCount = &collectionCount
	}

	// Set user's encrypted org key copy (each user has their own)
	org.EncryptedOrgKey = orgUser.EncryptedOrgKey

	return org, nil
}

func (s *organizationService) List(ctx context.Context, userID uint) ([]*domain.Organization, error) {
	orgs, err := s.orgRepo.ListForUser(ctx, userID)
	if err != nil {
		s.logger.Error("failed to list organizations", "user_id", userID, "error", err)
		return nil, fmt.Errorf("failed to list organizations: %w", err)
	}

	// Fetch stats for each organization
	for _, org := range orgs {
		// Get member count
		if memberCount, err := s.orgRepo.GetMemberCount(ctx, org.ID); err == nil {
			org.MemberCount = &memberCount
		}

		// Get team count
		if teamCount, err := s.orgRepo.GetTeamCount(ctx, org.ID); err == nil {
			org.TeamCount = &teamCount
		}

		// Get collection count
		if collectionCount, err := s.orgRepo.GetCollectionCount(ctx, org.ID); err == nil {
			org.CollectionCount = &collectionCount
		}
	}

	return orgs, nil
}

func (s *organizationService) Update(ctx context.Context, id uint, userID uint, req *domain.UpdateOrganizationRequest) (*domain.Organization, error) {
	// Check if user is owner or admin
	if err := s.checkPermission(ctx, id, userID, true); err != nil {
		return nil, err
	}

	org, err := s.orgRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("organization not found: %w", err)
	}

	// Update fields
	if req.Name != nil {
		org.Name = *req.Name
	}
	if req.BillingEmail != nil {
		org.BillingEmail = *req.BillingEmail
	}

	if err := s.orgRepo.Update(ctx, org); err != nil {
		s.logger.Error("failed to update organization", "org_id", id, "error", err)
		return nil, fmt.Errorf("failed to update organization: %w", err)
	}

	s.logger.Info("organization updated", "org_id", id, "user_id", userID)
	return org, nil
}

func (s *organizationService) Delete(ctx context.Context, id uint, userID uint) error {
	// Only owner can delete organization
	orgUser, err := s.orgUserRepo.GetByOrgAndUser(ctx, id, userID)
	if err != nil {
		return repository.ErrNotFound
	}

	if !orgUser.IsOwner() {
		return repository.ErrForbidden
	}

	// Default/personal organizations cannot be deleted by their owner.
	// (They can still be deleted by admin user-deletion flows.)
	org, err := s.orgRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if org.IsDefault {
		return fmt.Errorf("cannot delete default organization")
	}

	// Get organization to check for active subscription
	// Cancel active subscription before deleting organization
	// Note: Subscription cancellation is now handled by SubscriptionService
	// The subscription will be soft-deleted along with the organization
	s.logger.Info("deleting organization", "org_id", id)

	if err := s.orgRepo.Delete(ctx, id); err != nil {
		s.logger.Error("failed to delete organization", "org_id", id, "error", err)
		return fmt.Errorf("failed to delete organization: %w", err)
	}

	s.logger.Info("organization deleted", "org_id", id, "user_id", userID)
	return nil
}

func (s *organizationService) InviteUser(ctx context.Context, orgID uint, inviterUserID uint, req *domain.InviteUserToOrgRequest) (*domain.OrganizationUser, error) {
	// Check if inviter can manage users
	if err := s.checkPermission(ctx, orgID, inviterUserID, true); err != nil {
		return nil, err
	}

	// Check organization limits
	memberCount, err := s.orgRepo.GetMemberCount(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get member count: %w", err)
	}

	// Get plan limits from subscription
	maxUsers, err := s.getMaxUsers(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get plan limits: %w", err)
	}

	if memberCount >= maxUsers {
		return nil, fmt.Errorf("organization has reached max users limit (%d)", maxUsers)
	}

	// Get invitee user by email
	invitee, err := s.userRepo.GetByEmail(ctx, req.Email)

	// CASE 1: User is not registered yet
	if err != nil {
		// User not found - create pending invitation
		inviter, err := s.userRepo.GetByID(ctx, inviterUserID)
		if err != nil {
			return nil, fmt.Errorf("failed to get inviter info: %w", err)
		}

		orgRoleStr := string(req.Role)
		accessAll := req.AccessAll

		// Create invitation with organization info
		invitationReq := &domain.CreateInvitationRequest{
			Email:           req.Email,
			RoleID:          2, // Member role for platform access
			OrganizationID:  &orgID,
			OrgRole:         &orgRoleStr,
			EncryptedOrgKey: &req.EncryptedOrgKey,
			AccessAll:       &accessAll,
		}

		_, err = s.invitationService.CreateInvitation(ctx, invitationReq, inviterUserID, inviter.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to create invitation: %w", err)
		}

		s.logger.Info("pending invitation created for non-registered user",
			"org_id", orgID,
			"email", req.Email,
			"org_role", req.Role)

		// Return nil (no org user created yet - will be created after signup)
		return nil, nil
	}

	// CASE 2: User is registered
	// Check if user is already a member
	existing, err := s.orgUserRepo.GetByOrgAndUser(ctx, orgID, invitee.ID)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("user is already a member of this organization")
	}

	// Create an invitation record as well (unified invitation management + email).
	// This enables a single "Invitations" area to manage both platform and org invites.
	inviter, err := s.userRepo.GetByID(ctx, inviterUserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get inviter info: %w", err)
	}

	orgRoleStr := string(req.Role)
	accessAll := req.AccessAll
	invitationReq := &domain.CreateInvitationRequest{
		Email:           req.Email,
		RoleID:          2, // Member role for platform access (invitee already exists, but keep consistent)
		OrganizationID:  &orgID,
		OrgRole:         &orgRoleStr,
		EncryptedOrgKey: &req.EncryptedOrgKey,
		AccessAll:       &accessAll,
	}
	if _, err := s.invitationService.CreateInvitation(ctx, invitationReq, inviterUserID, inviter.Name); err != nil {
		return nil, fmt.Errorf("failed to create invitation: %w", err)
	}

	// Create organization user (invitation)
	now := time.Now()
	orgUser := &domain.OrganizationUser{
		OrganizationID:  orgID,
		UserID:          invitee.ID,
		Role:            req.Role,
		EncryptedOrgKey: req.EncryptedOrgKey,
		AccessAll:       req.AccessAll,
		Status:          domain.OrgUserStatusInvited,
		InvitedAt:       &now,
	}

	if err := s.orgUserRepo.Create(ctx, orgUser); err != nil {
		s.logger.Error("failed to invite user", "org_id", orgID, "user_email", req.Email, "error", err)
		return nil, fmt.Errorf("failed to invite user: %w", err)
	}

	// Ensure invited user is in default team (prevents orphan memberships).
	// Keep LDAP sync safe by never using ExternalID for defaults.
	if err := s.ensureOrgUserInDefaultTeam(ctx, orgID, orgUser.ID); err != nil {
		return nil, fmt.Errorf("failed to ensure default team membership: %w", err)
	}
	if defTeam, err := s.ensureDefaultTeam(ctx, orgID); err == nil && defTeam != nil {
		if defCol, err := s.ensureDefaultCollection(ctx, orgID); err == nil && defCol != nil {
			_ = s.ensureDefaultCollectionTeamAccess(ctx, defCol.ID, defTeam.ID)
		}
	}

	s.logger.Info("user invited to organization", "org_id", orgID, "invitee_email", req.Email, "role", req.Role)
	// Email is sent via InvitationService (above).

	return orgUser, nil
}

func (s *organizationService) GetMembers(ctx context.Context, orgID uint, requestingUserID uint) ([]*domain.OrganizationUser, error) {
	// Check if user is member
	if err := s.checkMembership(ctx, orgID, requestingUserID); err != nil {
		return nil, err
	}

	members, err := s.orgUserRepo.ListByOrganization(ctx, orgID)
	if err != nil {
		s.logger.Error("failed to get members", "org_id", orgID, "error", err)
		return nil, fmt.Errorf("failed to get members: %w", err)
	}

	return members, nil
}

func (s *organizationService) UpdateMemberRole(ctx context.Context, orgID, orgUserID uint, requestingUserID uint, req *domain.UpdateOrgUserRoleRequest) error {
	// Check if requesting user can manage users
	if err := s.checkPermission(ctx, orgID, requestingUserID, true); err != nil {
		return err
	}

	orgUser, err := s.orgUserRepo.GetByID(ctx, orgUserID)
	if err != nil {
		return fmt.Errorf("member not found: %w", err)
	}

	// Cannot change owner role
	if orgUser.Role == domain.OrgRoleOwner {
		return fmt.Errorf("cannot change owner role")
	}

	// Update role
	orgUser.Role = req.Role
	if req.AccessAll != nil {
		orgUser.AccessAll = *req.AccessAll
	}

	if err := s.orgUserRepo.Update(ctx, orgUser); err != nil {
		s.logger.Error("failed to update member role", "org_user_id", orgUserID, "error", err)
		return fmt.Errorf("failed to update member role: %w", err)
	}

	s.logger.Info("member role updated", "org_id", orgID, "org_user_id", orgUserID, "new_role", req.Role)
	return nil
}

func (s *organizationService) RemoveMember(ctx context.Context, orgID, orgUserID uint, requestingUserID uint) error {
	// Check if requesting user can manage users
	if err := s.checkPermission(ctx, orgID, requestingUserID, true); err != nil {
		return err
	}

	orgUser, err := s.orgUserRepo.GetByID(ctx, orgUserID)
	if err != nil {
		return fmt.Errorf("member not found: %w", err)
	}

	// Cannot remove owner
	if orgUser.Role == domain.OrgRoleOwner {
		return fmt.Errorf("cannot remove owner from organization")
	}

	if err := s.orgUserRepo.Delete(ctx, orgUserID); err != nil {
		s.logger.Error("failed to remove member", "org_user_id", orgUserID, "error", err)
		return fmt.Errorf("failed to remove member: %w", err)
	}

	s.logger.Info("member removed from organization", "org_id", orgID, "org_user_id", orgUserID)
	return nil
}

func (s *organizationService) AcceptInvitation(ctx context.Context, orgUserID uint, userID uint) error {
	orgUser, err := s.orgUserRepo.GetByID(ctx, orgUserID)
	if err != nil {
		return fmt.Errorf("invitation not found: %w", err)
	}

	// Check if user is the invitee
	if orgUser.UserID != userID {
		return repository.ErrForbidden
	}

	// Check if already accepted
	if orgUser.Status != domain.OrgUserStatusInvited {
		return fmt.Errorf("invitation already processed")
	}

	// Update status
	now := time.Now()
	orgUser.Status = domain.OrgUserStatusAccepted
	orgUser.AcceptedAt = &now

	if err := s.orgUserRepo.Update(ctx, orgUser); err != nil {
		s.logger.Error("failed to accept invitation", "org_user_id", orgUserID, "error", err)
		return fmt.Errorf("failed to accept invitation: %w", err)
	}

	// Ensure accepted user is in default team.
	if err := s.ensureOrgUserInDefaultTeam(ctx, orgUser.OrganizationID, orgUser.ID); err != nil {
		return fmt.Errorf("failed to ensure default team membership: %w", err)
	}

	s.logger.Info("invitation accepted", "org_id", orgUser.OrganizationID, "user_id", userID)
	return nil
}

func (s *organizationService) AddExistingMember(ctx context.Context, orgUser *domain.OrganizationUser) error {
	// Check if user is already a member
	existing, err := s.orgUserRepo.GetByOrgAndUser(ctx, orgUser.OrganizationID, orgUser.UserID)
	if err == nil && existing != nil {
		// If there's a pending org membership invitation, accept it instead of failing.
		if existing.Status == domain.OrgUserStatusInvited {
			now := time.Now()
			existing.Status = domain.OrgUserStatusAccepted
			existing.AcceptedAt = &now
			// Keep the org role / access_all from the existing record to avoid privilege escalation
			// from any client-controlled invitation metadata.
			if err := s.orgUserRepo.Update(ctx, existing); err != nil {
				return fmt.Errorf("failed to accept existing invitation: %w", err)
			}

			s.logger.Info("existing org invitation accepted",
				"org_id", existing.OrganizationID,
				"user_id", existing.UserID,
				"role", existing.Role)
			return nil
		}

		return fmt.Errorf("user is already a member of this organization")
	}

	// Set timestamps
	now := time.Now()
	if orgUser.InvitedAt == nil {
		orgUser.InvitedAt = &now
	}
	if orgUser.AcceptedAt == nil {
		orgUser.AcceptedAt = &now
	}

	// Create organization user membership
	if err := s.orgUserRepo.Create(ctx, orgUser); err != nil {
		s.logger.Error("failed to add existing member",
			"org_id", orgUser.OrganizationID,
			"user_id", orgUser.UserID,
			"error", err)
		return fmt.Errorf("failed to add member: %w", err)
	}

	// Ensure membership is not orphaned: add to default team.
	if err := s.ensureOrgUserInDefaultTeam(ctx, orgUser.OrganizationID, orgUser.ID); err != nil {
		return fmt.Errorf("failed to ensure default team membership: %w", err)
	}

	s.logger.Info("existing member added to organization",
		"org_id", orgUser.OrganizationID,
		"user_id", orgUser.UserID,
		"role", orgUser.Role)

	return nil
}

// DeclineInvitationForUser removes a pending org membership invitation for the invitee.
// This is used when the user declines an organization invitation via the unified invitations flow.
func (s *organizationService) DeclineInvitationForUser(ctx context.Context, orgID uint, userID uint) error {
	orgUser, err := s.orgUserRepo.GetByOrgAndUser(ctx, orgID, userID)
	if err != nil {
		// No org membership record exists (e.g. invite was only stored as an Invitation row)
		if err == repository.ErrNotFound {
			return nil
		}
		return err
	}

	// Only allow declining if it's still a pending invite
	if orgUser.Status != domain.OrgUserStatusInvited {
		return fmt.Errorf("invitation already processed")
	}

	if err := s.orgUserRepo.Delete(ctx, orgUser.ID); err != nil {
		return fmt.Errorf("failed to decline org invitation: %w", err)
	}

	s.logger.Info("org invitation declined",
		"org_id", orgID,
		"user_id", userID,
		"org_user_id", orgUser.ID)

	return nil
}

// Helper methods for permission checking

func (s *organizationService) checkMembership(ctx context.Context, orgID, userID uint) error {
	_, err := s.orgUserRepo.GetByOrgAndUser(ctx, orgID, userID)
	if err != nil {
		if err == repository.ErrNotFound {
			return repository.ErrForbidden
		}
		return err
	}
	return nil
}

// GetMembership retrieves a user's membership in an organization
func (s *organizationService) GetMembership(ctx context.Context, userID uint, orgID uint) (*domain.OrganizationUser, error) {
	orgUser, err := s.orgUserRepo.GetByOrgAndUser(ctx, orgID, userID)
	if err != nil {
		return nil, err
	}
	return orgUser, nil
}

// GetMemberCount returns the number of members in an organization
func (s *organizationService) GetMemberCount(ctx context.Context, orgID uint) (int, error) {
	return s.orgRepo.GetMemberCount(ctx, orgID)
}

// GetCollectionCount returns the number of collections in an organization
func (s *organizationService) GetCollectionCount(ctx context.Context, orgID uint) (int, error) {
	return s.orgRepo.GetCollectionCount(ctx, orgID)
}

func (s *organizationService) checkPermission(ctx context.Context, orgID, userID uint, requireAdmin bool) error {
	orgUser, err := s.orgUserRepo.GetByOrgAndUser(ctx, orgID, userID)
	if err != nil {
		if err == repository.ErrNotFound {
			return repository.ErrForbidden
		}
		return err
	}

	if requireAdmin && !orgUser.IsAdmin() {
		return repository.ErrForbidden
	}

	return nil
}

// getMaxUsers returns max users limit from subscription plan
func (s *organizationService) getMaxUsers(ctx context.Context, orgID uint) (int, error) {
	sub, err := s.subRepo.GetByOrganizationID(ctx, orgID)
	if err != nil {
		return 0, fmt.Errorf("failed to get subscription for org %d: %w", orgID, err)
	}
	if sub.Plan == nil {
		return 0, fmt.Errorf("subscription plan not loaded for org %d", orgID)
	}

	// Check if plan has max users limit
	if sub.Plan.MaxUsers != nil {
		return *sub.Plan.MaxUsers, nil
	}

	// Unlimited users (business/enterprise)
	return 999999, nil
}
