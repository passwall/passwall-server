package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/pkg/constants"
	uuid "github.com/satori/go.uuid"
	"golang.org/x/crypto/bcrypt"
)

type userService struct {
	repo               repository.UserRepository
	orgRepo            repository.OrganizationRepository
	orgUserRepo        repository.OrganizationUserRepository
	teamUserRepo       repository.TeamUserRepository
	collectionUserRepo repository.CollectionUserRepository
	itemShareRepo      repository.ItemShareRepository
	folderRepo         repository.FolderRepository
	invitationRepo     repository.InvitationRepository
	userActivityRepo   repository.UserActivityRepository
	logger             Logger
}

// NewUserService creates a new user service
func NewUserService(
	repo repository.UserRepository,
	orgRepo repository.OrganizationRepository,
	orgUserRepo repository.OrganizationUserRepository,
	teamUserRepo repository.TeamUserRepository,
	collectionUserRepo repository.CollectionUserRepository,
	itemShareRepo repository.ItemShareRepository,
	folderRepo repository.FolderRepository,
	invitationRepo repository.InvitationRepository,
	userActivityRepo repository.UserActivityRepository,
	logger Logger,
) UserService {
	return &userService{
		repo:               repo,
		orgRepo:            orgRepo,
		orgUserRepo:        orgUserRepo,
		teamUserRepo:       teamUserRepo,
		collectionUserRepo: collectionUserRepo,
		itemShareRepo:      itemShareRepo,
		folderRepo:         folderRepo,
		invitationRepo:     invitationRepo,
		userActivityRepo:   userActivityRepo,
		logger:             logger,
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

	for _, user := range users {
		if user == nil || user.Schema == "" {
			continue
		}

		count, err := s.repo.GetItemCount(ctx, user.Schema)
		if err != nil {
			s.logger.Debug("failed to get user item count", "user_id", user.ID, "error", err)
			continue
		}
		user.ItemCount = &count
	}

	s.logger.Debug("users listed", "count", len(users))
	return users, nil
}

func (s *userService) Create(ctx context.Context, user *domain.User) error {
	return errors.New("use CreateByAdmin for admin-created users with proper encryption setup")
}

// CreateByAdmin creates a user by admin (with proper zero-knowledge setup)
func (s *userService) CreateByAdmin(ctx context.Context, req *domain.CreateUserByAdminRequest) (*domain.User, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Check if user already exists
	existingUser, err := s.repo.GetByEmail(ctx, req.Email)
	if err == nil && existingUser != nil {
		return nil, repository.ErrAlreadyExists
	}

	// Hash the master password hash with bcrypt (defense in depth)
	// Client sends: HKDF(masterKey, info="auth")
	// Server stores: bcrypt(HKDF(masterKey, info="auth"))
	hashedPassword, err := bcrypt.GenerateFromPassword(
		[]byte(req.MasterPasswordHash),
		bcrypt.DefaultCost,
	)
	if err != nil {
		s.logger.Error("failed to hash password", "error", err)
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create Personal Vault organization first so we can satisfy NOT NULL org pointers on user.
	creatorEmail := req.Email
	creatorName := req.Name
	org := &domain.Organization{
		Name:               "Personal Vault",
		BillingEmail:       req.Email,
		EncryptedOrgKey:    req.EncryptedOrgKey,
		IsActive:           true,
		IsDefault:          false, // legacy; do not use
		IsPersonal:         true,
		CreatedByUserEmail: &creatorEmail,
		CreatedByUserName:  &creatorName,
	}
	if err := s.orgRepo.Create(ctx, org); err != nil {
		return nil, fmt.Errorf("failed to create personal organization: %w", err)
	}

	schema := generateSchemaFromEmail(req.Email)

	// Set role
	roleID := constants.RoleIDMember
	if req.RoleID != nil {
		roleID = *req.RoleID
	}

	// Create user with zero-knowledge fields from admin
	user := &domain.User{
		UUID:                   uuid.NewV4(),
		Name:                   req.Name,
		Email:                  req.Email,
		MasterPasswordHash:     string(hashedPassword),
		ProtectedUserKey:       req.ProtectedUserKey, // EncString from admin
		Schema:                 schema,
		PersonalOrganizationID: org.ID,
		DefaultOrganizationID:  org.ID,
		KdfType:                req.KdfConfig.Type,
		KdfIterations:          req.KdfConfig.Iterations,
		KdfMemory:              req.KdfConfig.Memory,
		KdfParallelism:         req.KdfConfig.Parallelism,
		KdfSalt:                req.KdfSalt, // Random salt from admin
		RoleID:                 roleID,
		IsVerified:             true, // Admin-created users are auto-verified
	}

	// Create schema
	if err := s.repo.CreateSchema(schema); err != nil {
		s.logger.Error("failed to create schema", "schema", schema, "error", err)
		now := time.Now()
		org.IsActive = false
		org.Status = domain.OrgStatusDeleted
		org.DeletedAt = &now
		_ = s.orgRepo.Update(ctx, org)
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	// Migrate all tables in user schema
	if err := s.repo.MigrateUserSchema(schema); err != nil {
		s.logger.Error("failed to migrate user schema tables", "schema", schema, "error", err)
		now := time.Now()
		org.IsActive = false
		org.Status = domain.OrgStatusDeleted
		org.DeletedAt = &now
		_ = s.orgRepo.Update(ctx, org)
		return nil, fmt.Errorf("failed to migrate user schema: %w", err)
	}

	// Save user
	if err := s.repo.Create(ctx, user); err != nil {
		s.logger.Error("failed to create user", "email", req.Email, "error", err)
		now := time.Now()
		org.IsActive = false
		org.Status = domain.OrgStatusDeleted
		org.DeletedAt = &now
		_ = s.orgRepo.Update(ctx, org)
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Create default folders for new user (same behavior as normal signup).
	if err := s.createDefaultFolders(ctx, user.ID); err != nil {
		s.logger.Error("failed to create default folders", "user_id", user.ID, "error", err)
		// Don't fail user creation if folder creation fails
	}

	// Attach user to org + finalize ownership metadata
	now := time.Now()
	orgUser := &domain.OrganizationUser{
		OrganizationID:  org.ID,
		UserID:          user.ID,
		Role:            domain.OrgRoleOwner,
		EncryptedOrgKey: req.EncryptedOrgKey,
		AccessAll:       true,
		Status:          domain.OrgUserStatusConfirmed,
		InvitedAt:       &now,
		AcceptedAt:      &now,
	}
	if err := s.orgUserRepo.Create(ctx, orgUser); err != nil {
		return nil, fmt.Errorf("failed to add user to personal organization: %w", err)
	}

	creatorID := user.ID
	org.CreatedByUserID = &creatorID
	org.PersonalOwnerUserID = &creatorID
	org.IsPersonal = true
	if err := s.orgRepo.Update(ctx, org); err != nil {
		return nil, fmt.Errorf("failed to finalize personal organization: %w", err)
	}

	s.logger.Info("user created by admin (zero-knowledge)",
		"id", user.ID,
		"email", req.Email,
		"role_id", roleID,
		"kdf_type", user.KdfType.String(),
		"iterations", user.KdfIterations,
		"is_verified", true)

	return user, nil
}

func generateSchemaFromEmail(email string) string {
	return "user_" + uuid.NewV5(uuid.NamespaceURL, email).String()[:8]
}

// createDefaultFolders creates default folders for a new user.
func (s *userService) createDefaultFolders(ctx context.Context, userID uint) error {
	for _, folderName := range constants.DefaultFolders {
		folder := &domain.Folder{
			UUID:   uuid.NewV4(),
			UserID: userID,
			Name:   folderName,
		}

		if err := s.folderRepo.Create(ctx, folder); err != nil {
			s.logger.Error("failed to create default folder", "folder", folderName, "user_id", userID, "error", err)
			// Continue creating other folders even if one fails
			continue
		}
	}

	s.logger.Info("created default folders", "user_id", userID, "count", len(constants.DefaultFolders))
	return nil
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

func (s *userService) Delete(ctx context.Context, id uint, schema string) error {
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

	// Retire personal vault organization so it doesn't remain active after user deletion.
	// Keep the row for audit continuity, but detach ownership pointers from deleted user.
	if user.PersonalOrganizationID != 0 {
		personalOrg, err := s.orgRepo.GetByID(ctx, user.PersonalOrganizationID)
		if err != nil && err != repository.ErrNotFound {
			s.logger.Error("failed to load personal organization while deleting user", "id", id, "org_id", user.PersonalOrganizationID, "error", err)
			return err
		}
		if err == nil && personalOrg != nil && personalOrg.IsPersonal {
			now := time.Now()
			personalOrg.IsActive = false
			personalOrg.Status = domain.OrgStatusDeleted
			personalOrg.DeletedAt = &now
			personalOrg.PersonalOwnerUserID = nil
			personalOrg.CreatedByUserID = nil
			if err := s.orgRepo.Update(ctx, personalOrg); err != nil {
				s.logger.Error("failed to retire personal organization while deleting user", "id", id, "org_id", personalOrg.ID, "error", err)
				return err
			}
		}
	}

	if err := s.folderRepo.DeleteByUserID(ctx, id); err != nil {
		s.logger.Error("failed to delete folders while deleting user", "id", id, "error", err)
		return err
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

	if err := s.repo.Delete(ctx, id, schema); err != nil {
		s.logger.Error("failed to delete user", "id", id, "schema", schema, "error", err)
		return err
	}

	s.logger.Info("user deleted", "id", id, "schema", schema)
	return nil
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
func (s *userService) DeleteWithOrganizations(ctx context.Context, userID uint, organizationIDs []uint, schema string) error {
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
	if err := s.Delete(ctx, userID, schema); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	s.logger.Info("user and organizations deleted successfully", "user_id", userID, "deleted_org_count", len(organizationIDs))
	return nil
}
