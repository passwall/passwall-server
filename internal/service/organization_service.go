package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"gorm.io/gorm"
)

type organizationService struct {
	orgRepo            repository.OrganizationRepository
	orgUserRepo        repository.OrganizationUserRepository
	userRepo           repository.UserRepository
	teamRepo           repository.TeamRepository
	teamUserRepo       repository.TeamUserRepository
	collectionRepo     repository.CollectionRepository
	collectionUserRepo repository.CollectionUserRepository
	collectionTeamRepo repository.CollectionTeamRepository
	policyRepo         repository.OrganizationPolicyRepository
	paymentService     PaymentService
	subRepo            interface {
		Create(ctx context.Context, sub *domain.Subscription) error
		GetByOrganizationID(ctx context.Context, orgID uint) (*domain.Subscription, error)
	}
	planRepo interface {
		GetByCode(ctx context.Context, code string) (*domain.Plan, error)
	}
	logger       Logger
	entitlements OrganizationEntitlementService
	invites      *OrgInvitationDeps
}

// NewOrganizationService creates a new organization service
func NewOrganizationService(
	orgRepo repository.OrganizationRepository,
	orgUserRepo repository.OrganizationUserRepository,
	userRepo repository.UserRepository,
	teamRepo repository.TeamRepository,
	teamUserRepo repository.TeamUserRepository,
	collectionRepo repository.CollectionRepository,
	collectionUserRepo repository.CollectionUserRepository,
	collectionTeamRepo repository.CollectionTeamRepository,
	policyRepo repository.OrganizationPolicyRepository,
	paymentService PaymentService,
	subRepo interface {
		Create(ctx context.Context, sub *domain.Subscription) error
		GetByOrganizationID(ctx context.Context, orgID uint) (*domain.Subscription, error)
	},
	planRepo interface {
		GetByCode(ctx context.Context, code string) (*domain.Plan, error)
	},
	logger Logger,
	options ...OrganizationServiceOption,
) OrganizationService {
	service := &organizationService{
		orgRepo:            orgRepo,
		orgUserRepo:        orgUserRepo,
		userRepo:           userRepo,
		teamRepo:           teamRepo,
		teamUserRepo:       teamUserRepo,
		collectionRepo:     collectionRepo,
		collectionUserRepo: collectionUserRepo,
		collectionTeamRepo: collectionTeamRepo,
		policyRepo:         policyRepo,
		paymentService:     paymentService,
		subRepo:            subRepo,
		planRepo:           planRepo,
		logger:             logger,
	}
	for _, option := range options {
		option(service)
	}
	return service
}

// ErrInvalidOrganizationPlan is returned when an organization is created for a
// plan, billing cycle, or seat count the plan catalog cannot satisfy.
var ErrInvalidOrganizationPlan = errors.New("invalid organization plan")

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
	initialPlan, seats, err := s.resolveInitialPlan(ctx, req)
	if err != nil {
		return nil, err
	}

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
	// Plan-first setups get a draft subscription for the selected shared plan; it
	// grants no paid access until checkout activates it. Everything else starts free.
	if _, err := s.subRepo.GetByOrganizationID(ctx, org.ID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			now := time.Now()
			sub := &domain.Subscription{
				UUID:           uuid.New(),
				OrganizationID: org.ID,
				PlanID:         initialPlan.ID,
				State:          domain.SubStateActive,
				StartedAt:      &now,
			}
			if req.IsSharedPlanSetup() {
				sub.State = domain.SubStateDraft
				sub.StartedAt = nil
				sub.SeatsPurchased = &seats
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

	s.logger.Info("organization created", "org_id", org.ID, "owner_id", userID, "name", org.Name, "plan", initialPlan.Code)
	return org, nil
}

// resolveInitialPlan returns the plan a new organization starts on and the
// seat count to reserve. Shared plans are resolved from plan + billing cycle;
// anything else falls back to the free plan.
func (s *organizationService) resolveInitialPlan(ctx context.Context, req *domain.CreateOrganizationRequest) (*domain.Plan, int, error) {
	const freePlanCode = "free-monthly"
	if req.Plan != "" && req.Plan != string(domain.PlanFree) && !req.IsSharedPlanSetup() {
		return nil, 0, fmt.Errorf("%w: unsupported plan %q", ErrInvalidOrganizationPlan, req.Plan)
	}

	if !req.IsSharedPlanSetup() {
		plan, err := s.planRepo.GetByCode(ctx, freePlanCode)
		if err != nil || plan == nil {
			return nil, 0, fmt.Errorf("failed to load free plan (%s): %w", freePlanCode, err)
		}
		return plan, 1, nil
	}

	cycle := req.BillingCycle
	if cycle == "" {
		cycle = string(domain.BillingCycleYearly)
	}
	if cycle != string(domain.BillingCycleMonthly) && cycle != string(domain.BillingCycleYearly) {
		return nil, 0, fmt.Errorf("%w: unsupported billing cycle %q", ErrInvalidOrganizationPlan, req.BillingCycle)
	}

	planCode := fmt.Sprintf("%s-%s", req.Plan, cycle)
	plan, err := s.planRepo.GetByCode(ctx, planCode)
	if err != nil || plan == nil || !plan.IsActive {
		return nil, 0, fmt.Errorf("%w: plan %s is not available", ErrInvalidOrganizationPlan, planCode)
	}

	seats := req.Seats
	if seats < 1 {
		seats = 1
	}
	if plan.MaxUsers != nil && seats > *plan.MaxUsers {
		return nil, 0, fmt.Errorf("%w: %s allows at most %d users", ErrInvalidOrganizationPlan, plan.Name, *plan.MaxUsers)
	}
	return plan, seats, nil
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
	if s.entitlements != nil {
		if err := s.entitlements.Authorize(ctx, id, domain.CapabilityOrganizationUpdate); err != nil {
			return nil, err
		}
	}

	org, err := s.orgRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("organization not found: %w", err)
	}

	// Update fields
	if req.Name != nil {
		if org.IsPersonal && *req.Name != "Personal Vault" {
			return nil, repository.ErrForbidden
		}
		org.Name = *req.Name
	}
	if req.BillingEmail != nil {
		org.BillingEmail = *req.BillingEmail
	}
	if req.ChangesCollectionManagement() {
		// Admins must not be able to widen their own reach.
		requester, err := s.orgUserRepo.GetActiveByOrgAndUser(ctx, id, userID)
		if err != nil || !requester.IsOwner() {
			return nil, repository.ErrForbidden
		}
		if org.IsPersonal {
			return nil, repository.ErrForbidden
		}
		if req.AdminsManageAllCollections != nil {
			org.AdminsManageAllCollections = *req.AdminsManageAllCollections
		}
		if req.CollectionCreationLimited != nil {
			org.CollectionCreationLimited = *req.CollectionCreationLimited
		}
		if req.ManageCanDeleteCollections != nil {
			org.ManageCanDeleteCollections = *req.ManageCanDeleteCollections
		}
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
	if s.entitlements != nil {
		if err := s.entitlements.Authorize(ctx, id, domain.CapabilityOrganizationDelete); err != nil {
			return err
		}
	}

	// Personal Vault organizations are never deletable.
	org, err := s.orgRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if org.IsPersonal {
		return repository.ErrForbidden
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

// ensureSeatAvailable rejects invitations once members plus pending
// invitations reach the organization's purchased seats or plan user limit.
func (s *organizationService) ensureSeatAvailable(ctx context.Context, orgID uint) error {
	if s.entitlements == nil {
		return nil
	}
	snapshot, err := s.entitlements.Resolve(ctx, orgID)
	if err != nil {
		return err
	}
	if snapshot == nil || snapshot.Limits.MaxUsers == nil {
		return nil
	}

	members, err := s.orgUserRepo.ListByOrganization(ctx, orgID)
	if err != nil {
		return fmt.Errorf("failed to count members: %w", err)
	}
	occupied := 0
	for _, member := range members {
		if member != nil && member.Status != domain.OrgUserStatusSuspended {
			occupied++
		}
	}
	if s.invites != nil && s.invites.Invitations != nil {
		pending, err := s.invites.Invitations.CountPendingByOrganization(ctx, orgID, time.Now())
		if err != nil {
			return fmt.Errorf("failed to count pending invitations: %w", err)
		}
		occupied += pending
	}

	if occupied >= *snapshot.Limits.MaxUsers {
		return &EntitlementDeniedError{
			Capability: domain.CapabilityMemberInvite,
			Reason:     domain.EntitlementReasonPlanLimitReached,
			Cause:      ErrPlanLimitReached,
		}
	}
	return nil
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
	req.Role = domain.NormalizeOrgRole(req.Role)
	if !isSupportedOrgRole(req.Role) {
		return fmt.Errorf("invalid organization role: %s", req.Role)
	}

	// Check if requesting user can manage users
	if err := s.checkPermission(ctx, orgID, requestingUserID, true); err != nil {
		return err
	}
	if s.entitlements != nil {
		if err := s.entitlements.Authorize(ctx, orgID, domain.CapabilityMemberUpdate); err != nil {
			return err
		}
	}

	// Only current owner can assign owner role.
	requesterMembership, err := s.orgUserRepo.GetByOrgAndUser(ctx, orgID, requestingUserID)
	if err != nil {
		return repository.ErrForbidden
	}
	if req.Role == domain.OrgRoleOwner && requesterMembership.Role != domain.OrgRoleOwner {
		return repository.ErrForbidden
	}

	orgUser, err := s.orgUserRepo.GetByID(ctx, orgUserID)
	if err != nil {
		return fmt.Errorf("member not found: %w", err)
	}
	if orgUser.OrganizationID != orgID {
		return repository.ErrNotFound
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
	if s.entitlements != nil {
		if err := s.entitlements.Authorize(ctx, orgID, domain.CapabilityMemberRemove); err != nil {
			return err
		}
	}

	orgUser, err := s.orgUserRepo.GetByID(ctx, orgUserID)
	if err != nil {
		return fmt.Errorf("member not found: %w", err)
	}

	// Prevent cross-organization member deletion with a mismatched org_user_id.
	if orgUser.OrganizationID != orgID {
		return repository.ErrForbidden
	}

	// Cannot remove owner
	if orgUser.Role == domain.OrgRoleOwner {
		return fmt.Errorf("cannot remove owner from organization")
	}

	// Cleanup team memberships before deleting org membership to avoid FK violations
	// on environments where constraints may not cascade.
	teamUsers, err := s.teamUserRepo.ListByOrgUser(ctx, orgUserID)
	if err != nil {
		s.logger.Error("failed to list team memberships before removing member", "org_user_id", orgUserID, "error", err)
		return fmt.Errorf("failed to remove member: %w", err)
	}
	for _, tu := range teamUsers {
		if err := s.teamUserRepo.Delete(ctx, tu.ID); err != nil {
			s.logger.Error("failed to remove team membership before removing member", "org_user_id", orgUserID, "team_user_id", tu.ID, "error", err)
			return fmt.Errorf("failed to remove member: %w", err)
		}
	}

	// Cleanup direct collection memberships as well.
	collectionUsers, err := s.collectionUserRepo.ListByOrgUser(ctx, orgUserID)
	if err != nil {
		s.logger.Error("failed to list collection memberships before removing member", "org_user_id", orgUserID, "error", err)
		return fmt.Errorf("failed to remove member: %w", err)
	}
	for _, cu := range collectionUsers {
		if err := s.collectionUserRepo.Delete(ctx, cu.ID); err != nil {
			s.logger.Error("failed to remove collection membership before removing member", "org_user_id", orgUserID, "collection_user_id", cu.ID, "error", err)
			return fmt.Errorf("failed to remove member: %w", err)
		}
	}

	if err := s.orgUserRepo.Delete(ctx, orgUserID); err != nil {
		s.logger.Error("failed to remove member", "org_user_id", orgUserID, "error", err)
		return fmt.Errorf("failed to remove member: %w", err)
	}

	s.logger.Info("member removed from organization", "org_id", orgID, "org_user_id", orgUserID)
	return nil
}

// ErrOrgKeyAlreadyUserWrapped is returned when a member tries to replace an
// organization key that is already wrapped with their user key.
var ErrOrgKeyAlreadyUserWrapped = errors.New("organization key is already wrapped with the user key")

// ErrInvalidOrgKeyEncoding is returned when the replacement key is not an
// EncString wrapped with the user key.
var ErrInvalidOrgKeyEncoding = errors.New("organization key must be an EncString wrapped with the user key")

// RewrapOwnOrgKey replaces the caller's RSA-wrapped copy of the organization
// key with a copy wrapped by their user key. An admin confirms keyless members
// by wrapping the key with the member's RSA public key; clients that only
// handle symmetric keys (extension, mobile, desktop) cannot open that copy, so
// the first client that can unwraps it and stores the symmetric form. Only the
// member's own copy changes and only while it is still RSA-wrapped.
func (s *organizationService) RewrapOwnOrgKey(ctx context.Context, orgID, userID uint, encryptedOrgKey string) error {
	if !isUserKeyEncString(encryptedOrgKey) {
		return ErrInvalidOrgKeyEncoding
	}
	membership, err := s.orgUserRepo.GetActiveByOrgAndUser(ctx, orgID, userID)
	if err != nil {
		return repository.ErrForbidden
	}
	if membership.EncryptedOrgKey == "" {
		return repository.ErrForbidden
	}
	if strings.HasPrefix(membership.EncryptedOrgKey, "2.") {
		return ErrOrgKeyAlreadyUserWrapped
	}
	membership.EncryptedOrgKey = encryptedOrgKey
	if err := s.orgUserRepo.Update(ctx, membership); err != nil {
		return fmt.Errorf("failed to store organization key: %w", err)
	}
	s.logger.Info("member organization key rewrapped with user key", "org_id", orgID, "org_user_id", membership.ID)
	return nil
}

// isUserKeyEncString checks the "2.iv|ciphertext|mac" shape of a key wrapped
// with a user key (AES-CBC-256 + HMAC-SHA256).
func isUserKeyEncString(value string) bool {
	if !strings.HasPrefix(value, "2.") {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(value, "2."), "|")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if _, err := base64.StdEncoding.DecodeString(part); err != nil || part == "" {
			return false
		}
	}
	return true
}

func (s *organizationService) ConfirmProvisionedMember(ctx context.Context, orgID, orgUserID uint, requestingUserID uint, encryptedOrgKey string) error {
	if encryptedOrgKey == "" {
		return fmt.Errorf("encrypted_org_key is required")
	}

	// Check if requesting user can manage users (owner or admin)
	if err := s.checkPermission(ctx, orgID, requestingUserID, true); err != nil {
		return err
	}

	orgUser, err := s.orgUserRepo.GetByID(ctx, orgUserID)
	if err != nil {
		return fmt.Errorf("member not found: %w", err)
	}

	// Verify the member belongs to this organization
	if orgUser.OrganizationID != orgID {
		return repository.ErrForbidden
	}

	// Can only confirm provisioned members
	if orgUser.Status != domain.OrgUserStatusProvisioned {
		return fmt.Errorf("member is not in provisioned status (current: %s)", orgUser.Status)
	}
	if s.entitlements != nil {
		if err := s.entitlements.Authorize(ctx, orgID, domain.CapabilityMemberInvite); err != nil {
			return err
		}
	}

	if err := s.checkSingleOrganizationPolicy(ctx, orgID, orgUser.UserID); err != nil {
		return err
	}

	// Set the encrypted org key and update status to confirmed
	orgUser.EncryptedOrgKey = encryptedOrgKey
	orgUser.Status = domain.OrgUserStatusConfirmed

	if err := s.orgUserRepo.Update(ctx, orgUser); err != nil {
		s.logger.Error("failed to confirm provisioned member", "org_user_id", orgUserID, "error", err)
		return fmt.Errorf("failed to confirm provisioned member: %w", err)
	}

	// Ensure confirmed user is in default team
	if err := s.ensureOrgUserInDefaultTeam(ctx, orgID, orgUser.ID); err != nil {
		return fmt.Errorf("failed to ensure default team membership: %w", err)
	}

	s.logger.Info("provisioned member confirmed", "org_id", orgID, "org_user_id", orgUserID)
	return nil
}

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

// checkPermission requires an active (accepted/confirmed) membership, so a
// suspended or not-yet-confirmed admin cannot act on the organization.
func (s *organizationService) checkPermission(ctx context.Context, orgID, userID uint, requireAdmin bool) error {
	orgUser, err := s.orgUserRepo.GetActiveByOrgAndUser(ctx, orgID, userID)
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

func isSupportedOrgRole(role domain.OrganizationRole) bool {
	switch role {
	case domain.OrgRoleOwner, domain.OrgRoleAdmin, domain.OrgRoleMember, domain.OrgRoleBilling:
		return true
	default:
		return false
	}
}

// checkSingleOrganizationPolicy verifies that neither the target organization nor the
// user's existing organizations enforce the Single Organization policy.
func (s *organizationService) checkSingleOrganizationPolicy(ctx context.Context, targetOrgID uint, userID uint) error {
	if s.policyRepo == nil {
		return nil
	}

	// Fail closed: a lookup error must not let a member slip past the policy.
	policyEnabled := func(orgID uint) (bool, error) {
		policy, err := s.policyRepo.GetByOrgAndType(ctx, orgID, domain.PolicySingleOrganization)
		if errors.Is(err, repository.ErrNotFound) {
			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("failed to check single organization policy: %w", err)
		}
		return policy != nil && policy.Enabled, nil
	}

	memberships, err := s.orgUserRepo.ListByUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to check existing memberships: %w", err)
	}

	// Other shared organizations the user actively belongs to. Personal vaults
	// never count: every user has one.
	var others []uint
	for _, m := range memberships {
		if m.OrganizationID == targetOrgID {
			continue
		}
		if m.Status != domain.OrgUserStatusAccepted && m.Status != domain.OrgUserStatusConfirmed {
			continue
		}
		if m.Organization != nil && m.Organization.IsPersonal {
			continue
		}
		others = append(others, m.OrganizationID)
	}
	if len(others) == 0 {
		return nil
	}

	targetEnforces, err := policyEnabled(targetOrgID)
	if err != nil {
		return err
	}
	if targetEnforces {
		return fmt.Errorf("organization policy requires single organization membership; user belongs to another organization")
	}
	for _, orgID := range others {
		enforces, err := policyEnabled(orgID)
		if err != nil {
			return err
		}
		if enforces {
			return fmt.Errorf("user's existing organization enforces single organization membership")
		}
	}
	return nil
}
