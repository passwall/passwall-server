package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/pkg/constants"
)

type userService struct {
	repo               repository.UserRepository
	tokenRepo          repository.TokenRepository
	orgRepo            repository.OrganizationRepository
	orgUserRepo        repository.OrganizationUserRepository
	orgItemRepo        repository.OrganizationItemRepository
	teamUserRepo       repository.TeamUserRepository
	collectionUserRepo repository.CollectionUserRepository
	itemShareRepo      repository.ItemShareRepository
	invitationRepo     repository.InvitationRepository
	userActivityRepo   repository.UserActivityRepository
	logger             Logger
	txManager          repository.TxManager
	vaultProvisioner   PersonalVaultProvisioner
}

// NewUserService creates a new user service
func NewUserService(
	repo repository.UserRepository,
	tokenRepo repository.TokenRepository,
	orgRepo repository.OrganizationRepository,
	orgUserRepo repository.OrganizationUserRepository,
	orgItemRepo repository.OrganizationItemRepository,
	teamUserRepo repository.TeamUserRepository,
	collectionUserRepo repository.CollectionUserRepository,
	itemShareRepo repository.ItemShareRepository,
	invitationRepo repository.InvitationRepository,
	userActivityRepo repository.UserActivityRepository,
	logger Logger,
	txManager repository.TxManager,
	vaultProvisioner PersonalVaultProvisioner,
) UserService {
	return &userService{
		repo:               repo,
		tokenRepo:          tokenRepo,
		orgRepo:            orgRepo,
		orgUserRepo:        orgUserRepo,
		orgItemRepo:        orgItemRepo,
		teamUserRepo:       teamUserRepo,
		collectionUserRepo: collectionUserRepo,
		itemShareRepo:      itemShareRepo,
		invitationRepo:     invitationRepo,
		userActivityRepo:   userActivityRepo,
		logger:             logger,
		txManager:          txManager,
		vaultProvisioner:   vaultProvisioner,
	}
}

func (s *userService) GetByID(ctx context.Context, id uint) (*domain.User, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("failed to get user", "id", id, "error", err)
		return nil, err
	}
	s.logger.Debug("user retrieved", "id", id)
	return user, nil
}

func (s *userService) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	return s.repo.GetByEmail(ctx, email)
}

func (s *userService) List(ctx context.Context) ([]*domain.User, error) {
	users, _, err := s.repo.List(ctx, repository.ListFilter{})
	if err != nil {
		s.logger.Error("failed to list users", "error", err)
		return nil, err
	}

	s.logger.Debug("users listed", "count", len(users))
	return users, nil
}

// Typed errors for platform-admin user edits.
var (
	ErrAdminRoleInvalid    = errors.New("role must be admin or member")
	ErrAdminSelfRoleChange = errors.New("you cannot change your own role")
	ErrAdminSelfDelete     = errors.New("you cannot delete your own account here")
	ErrSystemUserProtected = errors.New("system users cannot have their role or email changed")
	ErrLastAdmin           = errors.New("at least one admin must remain")
)

func (s *userService) ListPage(ctx context.Context, filter repository.ListFilter) ([]*domain.User, *repository.ListResult, error) {
	users, result, err := s.repo.List(ctx, filter)
	if err != nil {
		s.logger.Error("failed to list users", "error", err)
		return nil, nil, err
	}
	return users, result, nil
}

func (s *userService) UpdateByAdmin(ctx context.Context, actorID, id uint, req *domain.UpdateUserRequest) (domain.User, *domain.User, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return domain.User{}, nil, err
	}
	before := *user

	roleChanges := req.RoleID != nil && *req.RoleID != user.RoleID
	if roleChanges {
		if *req.RoleID != constants.RoleIDAdmin && *req.RoleID != constants.RoleIDMember {
			return before, nil, ErrAdminRoleInvalid
		}
		if actorID == id {
			return before, nil, ErrAdminSelfRoleChange
		}
	}
	emailChanges := req.Email != nil && *req.Email != user.Email
	if user.IsSystemUser && (roleChanges || emailChanges) {
		return before, nil, ErrSystemUserProtected
	}
	if roleChanges && user.RoleID == constants.RoleIDAdmin {
		admins, err := s.repo.CountByRoleID(ctx, constants.RoleIDAdmin)
		if err != nil {
			return before, nil, fmt.Errorf("count admins: %w", err)
		}
		if admins <= 1 {
			return before, nil, ErrLastAdmin
		}
	}

	req.ApplyTo(user)
	if err := s.Update(ctx, id, user); err != nil {
		return before, nil, err
	}
	return before, user, nil
}

func (s *userService) Create(ctx context.Context, user *domain.User) error {
	return errors.New("users are created through signup with client-side key generation")
}

func (s *userService) Update(ctx context.Context, id uint, user *domain.User) error {
	// user parameter already contains the updates applied
	// Just save to database
	if err := s.repo.Update(ctx, user); err != nil {
		s.logger.Error("failed to update user", "id", id, "error", err)
		return err
	}

	s.logger.Info("user updated", "id", id, "email", user.Email, "role_id", user.RoleID)
	return nil
}

func (s *userService) Delete(ctx context.Context, id uint) error {
	return s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		return s.deleteAccount(txCtx, id)
	})
}

func (s *userService) deleteAccount(ctx context.Context, id uint) error {
	// Check if user exists
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("user not found for deletion", "id", id, "error", err)
		return err
	}

	// Prevent deletion of system users (e.g., super admin)
	if user.IsSystemUser {
		s.logger.Warn("attempted to delete system user", "id", id, "email", user.Email)
		return repository.ErrForbidden
	}

	// Prevent deleting a user that would leave organizations without an owner.
	// Admins must first transfer ownership or delete the organization(s) via DeleteWithOrganizations.
	ownershipCheck, err := s.CheckOwnership(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to check organization ownership: %w", err)
	}
	if ownershipCheck != nil && ownershipCheck.IsSoleOwner {
		return fmt.Errorf("user is the sole owner of one or more organizations; transfer ownership or delete the organization(s) first")
	}

	if err := s.itemShareRepo.DeleteBySharedWithUser(ctx, id); err != nil {
		s.logger.Error("failed to delete item shares shared with user", "id", id, "error", err)
		return err
	}

	// Cleanup organization memberships before deleting the user to avoid FK violations
	// on organization_users.user_id and dependent team/collection membership tables.
	orgMemberships, err := s.orgUserRepo.ListByUser(ctx, id)
	if err != nil {
		s.logger.Error("failed to list user organization memberships", "id", id, "error", err)
		return err
	}
	for _, orgMembership := range orgMemberships {
		if orgMembership == nil {
			continue
		}

		teamUsers, err := s.teamUserRepo.ListByOrgUser(ctx, orgMembership.ID)
		if err != nil {
			s.logger.Error("failed to list team memberships while deleting user", "id", id, "org_user_id", orgMembership.ID, "error", err)
			return err
		}
		for _, tu := range teamUsers {
			if err := s.teamUserRepo.Delete(ctx, tu.ID); err != nil {
				s.logger.Error("failed to delete team membership while deleting user", "id", id, "team_user_id", tu.ID, "error", err)
				return err
			}
		}

		collectionUsers, err := s.collectionUserRepo.ListByOrgUser(ctx, orgMembership.ID)
		if err != nil {
			s.logger.Error("failed to list collection memberships while deleting user", "id", id, "org_user_id", orgMembership.ID, "error", err)
			return err
		}
		for _, cu := range collectionUsers {
			if err := s.collectionUserRepo.Delete(ctx, cu.ID); err != nil {
				s.logger.Error("failed to delete collection membership while deleting user", "id", id, "collection_user_id", cu.ID, "error", err)
				return err
			}
		}

		if err := s.orgUserRepo.Delete(ctx, orgMembership.ID); err != nil {
			s.logger.Error("failed to delete organization membership while deleting user", "id", id, "org_user_id", orgMembership.ID, "error", err)
			return err
		}
	}

	if err := s.userActivityRepo.DeleteByUserID(ctx, id); err != nil {
		s.logger.Error("failed to delete user activities while deleting user", "id", id, "error", err)
		return err
	}

	// Remove all invitations for this email to avoid re-invite collisions.
	if err := s.invitationRepo.DeleteByEmail(ctx, user.Email); err != nil {
		s.logger.Error("failed to delete invitations while deleting user", "id", id, "email", user.Email, "error", err)
		return err
	}

	if err := s.tokenRepo.Delete(ctx, int(id)); err != nil {
		s.logger.Error("failed to delete tokens while deleting user", "id", id, "error", err)
		return err
	}

	if err := s.orgItemRepo.ClearCreatorByUserID(ctx, id); err != nil {
		s.logger.Error("failed to clear organization item creator references", "id", id, "error", err)
		return err
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		s.logger.Error("failed to delete user", "id", id, "error", err)
		return err
	}
	if user.PersonalOrganizationID != 0 {
		if err := s.orgRepo.PurgePersonal(ctx, user.PersonalOrganizationID); err != nil && !errors.Is(err, repository.ErrNotFound) {
			s.logger.Error("failed to purge personal organization", "id", id, "org_id", user.PersonalOrganizationID, "error", err)
			return err
		}
	}

	s.logger.Info("user deleted", "id", id)
	return nil
}

// DeleteForRecovery deletes user and enforces recovery-delete policy.
func (s *userService) DeleteForRecovery(ctx context.Context, userID uint) error {
	return s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		return s.deleteForRecovery(txCtx, userID)
	})
}

func (s *userService) deleteForRecovery(ctx context.Context, userID uint) error {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.IsSystemUser {
		return repository.ErrForbidden
	}

	orgMemberships, err := s.orgUserRepo.ListByUser(ctx, userID)
	if err != nil {
		return err
	}

	ownedOrgIDs := make([]uint, 0)
	for _, orgMembership := range orgMemberships {
		if orgMembership == nil || orgMembership.Role != domain.OrgRoleOwner {
			continue
		}
		org, orgErr := s.orgRepo.GetByID(ctx, orgMembership.OrganizationID)
		if orgErr != nil {
			if errors.Is(orgErr, repository.ErrNotFound) {
				continue
			}
			return orgErr
		}
		if org.IsPersonal {
			continue
		}
		ownedOrgIDs = append(ownedOrgIDs, org.ID)
	}

	for _, orgID := range ownedOrgIDs {
		if err := s.orgRepo.Delete(ctx, orgID); err != nil {
			s.logger.Error("failed to delete owned organization during recovery delete", "user_id", userID, "org_id", orgID, "error", err)
			return err
		}
	}

	return s.deleteAccount(ctx, userID)
}

func (s *userService) ChangeMasterPassword(ctx context.Context, req *domain.ChangeMasterPasswordRequest) error {
	return errors.New("use AuthService.ChangeMasterPassword for zero-knowledge encryption")
}

// CheckOwnership checks if user is sole owner of any organizations
func (s *userService) CheckOwnership(ctx context.Context, userID uint) (*domain.OwnershipCheckResult, error) {
	s.logger.Debug("checking ownership for user", "user_id", userID)

	// Get all organizations where user is a member
	orgUsers, err := s.orgUserRepo.ListByUser(ctx, userID)
	if err != nil {
		s.logger.Error("failed to get user's organizations", "user_id", userID, "error", err)
		return nil, err
	}

	result := &domain.OwnershipCheckResult{
		IsSoleOwner:   false,
		Organizations: []domain.SoleOwnerOrganization{},
	}

	for _, orgUser := range orgUsers {
		// Only check organizations where user is owner
		if orgUser.Role != domain.OrgRoleOwner {
			continue
		}

		// Get organization details
		org, err := s.orgRepo.GetByID(ctx, orgUser.OrganizationID)
		if err != nil {
			s.logger.Error("failed to get organization", "org_id", orgUser.OrganizationID, "error", err)
			continue
		}
		// Personal Vault organizations are never deletable; don't block user deletion on them.
		if org.IsPersonal {
			continue
		}

		// Count total owners in this organization
		allOrgUsers, err := s.orgUserRepo.ListByOrganization(ctx, orgUser.OrganizationID)
		if err != nil {
			s.logger.Error("failed to get organization users", "org_id", orgUser.OrganizationID, "error", err)
			continue
		}

		ownerCount := 0
		totalMembers := len(allOrgUsers)
		for _, ou := range allOrgUsers {
			if ou.Role == domain.OrgRoleOwner {
				ownerCount++
			}
		}

		// If this user is the sole owner
		if ownerCount == 1 {
			result.IsSoleOwner = true
			result.Organizations = append(result.Organizations, domain.SoleOwnerOrganization{
				ID:          org.ID,
				PublicID:    org.PublicID,
				Name:        org.Name,
				MemberCount: totalMembers,
				CanTransfer: totalMembers > 1, // Can transfer if there are other members
			})
		}
	}

	s.logger.Info("ownership check completed", "user_id", userID, "is_sole_owner", result.IsSoleOwner, "org_count", len(result.Organizations))
	return result, nil
}

// TransferOwnership transfers organization ownership to another user
func (s *userService) TransferOwnership(ctx context.Context, req *domain.TransferOwnershipRequest) error {
	s.logger.Debug("transferring ownership", "user_id", req.UserID, "org_id", req.OrganizationID, "new_owner_id", req.NewOwnerUserID)

	// Get current owner's org membership
	currentOrgUser, err := s.orgUserRepo.GetByOrgAndUser(ctx, req.OrganizationID, req.UserID)
	if err != nil {
		s.logger.Error("failed to get current owner's membership", "error", err)
		return fmt.Errorf("current user is not a member of this organization")
	}

	// Verify current user is owner
	if currentOrgUser.Role != domain.OrgRoleOwner {
		return fmt.Errorf("current user is not an owner of this organization")
	}

	// Get new owner's org membership
	newOrgUser, err := s.orgUserRepo.GetByOrgAndUser(ctx, req.OrganizationID, req.NewOwnerUserID)
	if err != nil {
		s.logger.Error("failed to get new owner's membership", "error", err)
		return fmt.Errorf("new owner is not a member of this organization")
	}

	// Update roles: demote current owner to admin, promote new user to owner
	currentOrgUser.Role = domain.OrgRoleAdmin
	if err := s.orgUserRepo.Update(ctx, currentOrgUser); err != nil {
		s.logger.Error("failed to demote current owner", "error", err)
		return fmt.Errorf("failed to transfer ownership: %w", err)
	}

	newOrgUser.Role = domain.OrgRoleOwner
	if err := s.orgUserRepo.Update(ctx, newOrgUser); err != nil {
		// Rollback: restore current owner's role
		currentOrgUser.Role = domain.OrgRoleOwner
		_ = s.orgUserRepo.Update(ctx, currentOrgUser)

		s.logger.Error("failed to promote new owner", "error", err)
		return fmt.Errorf("failed to transfer ownership: %w", err)
	}

	s.logger.Info("ownership transferred successfully", "org_id", req.OrganizationID, "from_user", req.UserID, "to_user", req.NewOwnerUserID)
	return nil
}

// DeleteWithOrganizations deletes user along with specified organizations
func (s *userService) DeleteWithOrganizations(ctx context.Context, userID uint, organizationIDs []uint) error {
	return s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		return s.deleteWithOrganizations(txCtx, userID, organizationIDs)
	})
}

func (s *userService) deleteWithOrganizations(ctx context.Context, userID uint, organizationIDs []uint) error {
	s.logger.Debug("deleting user with organizations", "user_id", userID, "org_ids", organizationIDs)

	// Verify user is sole owner of all specified organizations
	ownershipCheck, err := s.CheckOwnership(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to check ownership: %w", err)
	}

	// Verify all specified orgs are in the sole owner list
	for _, orgID := range organizationIDs {
		found := false
		for _, org := range ownershipCheck.Organizations {
			if org.ID == orgID {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("user is not sole owner of organization %d", orgID)
		}
	}

	// Delete all specified organizations
	for _, orgID := range organizationIDs {
		if err := s.orgRepo.Delete(ctx, orgID); err != nil {
			s.logger.Error("failed to delete organization", "org_id", orgID, "error", err)
			return fmt.Errorf("failed to delete organization %d: %w", orgID, err)
		}
		s.logger.Info("organization deleted", "org_id", orgID)
	}

	// Now delete the user (organization_users records will be cascade deleted)
	if err := s.deleteAccount(ctx, userID); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	s.logger.Info("user and organizations deleted successfully", "user_id", userID, "deleted_org_count", len(organizationIDs))
	return nil
}
