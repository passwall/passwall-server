package service

import (
	"context"
	"errors"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
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
	if user.Email == "" {
		return repository.ErrInvalidInput
	}

	if err := s.repo.Create(ctx, user); err != nil {
		s.logger.Error("failed to create user", "email", user.Email, "error", err)
		return err
	}

	s.logger.Info("user created", "id", user.ID, "email", user.Email)
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
