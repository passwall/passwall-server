package service

import (
	"context"
	"fmt"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
)

type organizationService struct {
	orgRepo     repository.OrganizationRepository
	orgUserRepo repository.OrganizationUserRepository
	userRepo    repository.UserRepository
	logger      Logger
}

// NewOrganizationService creates a new organization service
func NewOrganizationService(
	orgRepo repository.OrganizationRepository,
	orgUserRepo repository.OrganizationUserRepository,
	userRepo repository.UserRepository,
	logger Logger,
) OrganizationService {
	return &organizationService{
		orgRepo:     orgRepo,
		orgUserRepo: orgUserRepo,
		userRepo:    userRepo,
		logger:      logger,
	}
}

func (s *organizationService) Create(ctx context.Context, userID uint, req *domain.CreateOrganizationRequest) (*domain.Organization, error) {
	// Validate plan
	plan := domain.OrganizationPlan(req.Plan)
	if req.Plan == "" {
		plan = domain.PlanFree
	}
	if plan != domain.PlanFree && plan != domain.PlanBusiness && plan != domain.PlanEnterprise {
		return nil, fmt.Errorf("invalid plan: %s", req.Plan)
	}

	// Set plan limits
	var maxUsers, maxCollections int
	switch plan {
	case domain.PlanFree:
		maxUsers = 2
		maxCollections = 2
	case domain.PlanBusiness:
		maxUsers = 999
		maxCollections = 999
	case domain.PlanEnterprise:
		maxUsers = 9999
		maxCollections = 9999
	}

	// Create organization
	org := &domain.Organization{
		Name:            req.Name,
		BillingEmail:    req.BillingEmail,
		Plan:            plan,
		MaxUsers:        maxUsers,
		MaxCollections:  maxCollections,
		EncryptedOrgKey: req.EncryptedOrgKey,
		IsActive:        true,
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

	s.logger.Info("organization created", "org_id", org.ID, "owner_id", userID, "name", org.Name)
	return org, nil
}

func (s *organizationService) GetByID(ctx context.Context, id uint, userID uint) (*domain.Organization, error) {
	// Check if user is member of organization
	if err := s.checkMembership(ctx, id, userID); err != nil {
		return nil, err
	}

	org, err := s.orgRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	return org, nil
}

func (s *organizationService) List(ctx context.Context, userID uint) ([]*domain.Organization, error) {
	orgs, err := s.orgRepo.ListForUser(ctx, userID)
	if err != nil {
		s.logger.Error("failed to list organizations", "user_id", userID, "error", err)
		return nil, fmt.Errorf("failed to list organizations: %w", err)
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
	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("organization not found: %w", err)
	}

	memberCount, err := s.orgRepo.GetMemberCount(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get member count: %w", err)
	}

	if memberCount >= org.MaxUsers {
		return nil, fmt.Errorf("organization has reached max users limit (%d)", org.MaxUsers)
	}

	// Get invitee user by email
	invitee, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// Check if user is already a member
	existing, err := s.orgUserRepo.GetByOrgAndUser(ctx, orgID, invitee.ID)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("user is already a member of this organization")
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

	s.logger.Info("user invited to organization", "org_id", orgID, "invitee_email", req.Email, "role", req.Role)

	// TODO: Send invitation email

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

	s.logger.Info("invitation accepted", "org_id", orgUser.OrganizationID, "user_id", userID)
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

