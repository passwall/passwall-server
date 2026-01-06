package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/pkg/constants"
	uuid "github.com/satori/go.uuid"
	"golang.org/x/crypto/bcrypt"
)

type userService struct {
	repo   repository.UserRepository
	logger Logger
}

// NewUserService creates a new user service
func NewUserService(repo repository.UserRepository, logger Logger) UserService {
	return &userService{
		repo:   repo,
		logger: logger,
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

	schema := generateSchemaFromEmail(req.Email)

	// Set role
	roleID := constants.RoleIDMember
	if req.RoleID != nil {
		roleID = *req.RoleID
	}

	// Create user with zero-knowledge fields from admin
	user := &domain.User{
		UUID:               uuid.NewV4(),
		Name:               req.Name,
		Email:              req.Email,
		MasterPasswordHash: string(hashedPassword),
		ProtectedUserKey:   req.ProtectedUserKey, // EncString from admin
		Schema:             schema,
		KdfType:            req.KdfConfig.Type,
		KdfIterations:      req.KdfConfig.Iterations,
		KdfMemory:          req.KdfConfig.Memory,
		KdfParallelism:     req.KdfConfig.Parallelism,
		KdfSalt:            req.KdfSalt, // Random salt from admin
		RoleID:             roleID,
		IsVerified:         true, // Admin-created users are auto-verified
	}

	// Create schema
	if err := s.repo.CreateSchema(schema); err != nil {
		s.logger.Error("failed to create schema", "schema", schema, "error", err)
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	// Migrate all tables in user schema
	if err := s.repo.MigrateUserSchema(schema); err != nil {
		s.logger.Error("failed to migrate user schema tables", "schema", schema, "error", err)
		return nil, fmt.Errorf("failed to migrate user schema: %w", err)
	}

	// Save user
	if err := s.repo.Create(ctx, user); err != nil {
		s.logger.Error("failed to create user", "email", req.Email, "error", err)
		return nil, fmt.Errorf("failed to create user: %w", err)
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
