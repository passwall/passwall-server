package service

import (
	"context"
	"fmt"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
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

func (s *userService) Create(ctx context.Context, user *domain.User) error {
	// Validate
	if user.Email == "" {
		return repository.ErrInvalidInput
	}

	// Hash password if provided
	if user.MasterPassword != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.MasterPassword), bcrypt.DefaultCost)
		if err != nil {
			s.logger.Error("failed to hash password", "error", err)
			return fmt.Errorf("failed to hash password: %w", err)
		}
		user.MasterPassword = string(hashedPassword)
	}

	if err := s.repo.Create(ctx, user); err != nil {
		s.logger.Error("failed to create user", "email", user.Email, "error", err)
		return err
	}

	s.logger.Info("user created", "id", user.ID, "email", user.Email)
	return nil
}

func (s *userService) Update(ctx context.Context, id uint, user *domain.User) error {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("user not found for update", "id", id, "error", err)
		return err
	}

	// Update fields (partial update pattern)
	if user.Name != "" {
		existing.Name = user.Name
	}
	if user.Email != "" {
		existing.Email = user.Email
	}
	if user.Role != "" {
		existing.Role = user.Role
	}

	if err := s.repo.Update(ctx, existing); err != nil {
		s.logger.Error("failed to update user", "id", id, "error", err)
		return err
	}

	s.logger.Info("user updated", "id", id, "email", existing.Email)
	return nil
}

func (s *userService) Delete(ctx context.Context, id uint, schema string) error {
	// Check if user exists
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		s.logger.Error("user not found for deletion", "id", id, "error", err)
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
	// Verify old password
	user, err := s.repo.GetByCredentials(ctx, req.Email, req.OldMasterPassword)
	if err != nil {
		s.logger.Warn("invalid credentials for password change", "email", req.Email)
		return repository.ErrUnauthorized
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewMasterPassword), bcrypt.DefaultCost)
	if err != nil {
		s.logger.Error("failed to hash new password", "email", req.Email, "error", err)
		return fmt.Errorf("failed to hash password: %w", err)
	}

	user.MasterPassword = string(hashedPassword)

	if err := s.repo.Update(ctx, user); err != nil {
		s.logger.Error("failed to update user password", "id", user.ID, "error", err)
		return err
	}

	s.logger.Info("master password changed", "user_id", user.ID, "email", user.Email)
	return nil
}

