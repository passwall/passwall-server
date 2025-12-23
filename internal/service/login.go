package service

import (
	"context"
	"fmt"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
)

type loginService struct {
	repo      repository.LoginRepository
	encryptor Encryptor
	logger    Logger
}

// NewLoginService creates a new login service
func NewLoginService(repo repository.LoginRepository, encryptor Encryptor, logger Logger) LoginService {
	return &loginService{
		repo:      repo,
		encryptor: encryptor,
		logger:    logger,
	}
}

func (s *loginService) GetByID(ctx context.Context, id uint) (*domain.Login, error) {
	login, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("failed to get login", "id", id, "error", err)
		return nil, err
	}

	// Decrypt encrypted fields
	if err := s.encryptor.DecryptModel(login, ""); err != nil {
		s.logger.Error("failed to decrypt login", "id", id, "error", err)
		return nil, fmt.Errorf("failed to decrypt login: %w", err)
	}

	s.logger.Debug("login retrieved", "id", id)
	return login, nil
}

func (s *loginService) List(ctx context.Context) ([]*domain.Login, error) {
	logins, err := s.repo.List(ctx)
	if err != nil {
		s.logger.Error("failed to list logins", "error", err)
		return nil, err
	}

	// Decrypt all logins
	decryptedCount := 0
	for _, login := range logins {
		if err := s.encryptor.DecryptModel(login, ""); err != nil {
			s.logger.Warn("failed to decrypt login, skipping", "id", login.ID, "error", err)
			continue
		}
		decryptedCount++
	}

	s.logger.Debug("logins listed", "total", len(logins), "decrypted", decryptedCount)
	return logins, nil
}

func (s *loginService) Create(ctx context.Context, login *domain.Login) error {
	// Validate
	if err := s.validateLogin(login); err != nil {
		return err
	}

	// Encrypt sensitive fields
	if err := s.encryptor.EncryptModel(login, ""); err != nil {
		s.logger.Error("failed to encrypt login", "error", err)
		return fmt.Errorf("failed to encrypt login: %w", err)
	}

	if err := s.repo.Create(ctx, login); err != nil {
		s.logger.Error("failed to create login", "error", err)
		return err
	}

	s.logger.Info("login created", "id", login.ID, "title", login.Title)
	return nil
}

func (s *loginService) Update(ctx context.Context, id uint, login *domain.Login) error {
	// Validate
	if err := s.validateLogin(login); err != nil {
		return err
	}

	// Check if login exists (no need to decrypt, just check existence)
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("login not found for update", "id", id, "error", err)
		return err
	}

	// Encrypt sensitive fields before updating
	if err := s.encryptor.EncryptModel(login, ""); err != nil {
		s.logger.Error("failed to encrypt login", "id", id, "error", err)
		return fmt.Errorf("failed to encrypt login: %w", err)
	}

	// Update using GORM's Save which updates all fields
	// Set the ID to existing record's ID
	login.ID = existing.ID
	login.CreatedAt = existing.CreatedAt // Preserve timestamps

	if err := s.repo.Update(ctx, login); err != nil {
		s.logger.Error("failed to update login", "id", id, "error", err)
		return err
	}

	s.logger.Info("login updated", "id", id)
	return nil
}

func (s *loginService) Delete(ctx context.Context, id uint) error {
	// Simple existence check using GORM Exists pattern (lighter than GetByID)
	// No need to fetch and decrypt the whole record
	if err := s.repo.Delete(ctx, id); err != nil {
		s.logger.Error("failed to delete login", "id", id, "error", err)
		return err
	}

	s.logger.Info("login deleted", "id", id)
	return nil
}

func (s *loginService) BulkUpdate(ctx context.Context, logins []*domain.Login) error {
	// Use repository transaction if available, or handle errors gracefully
	successCount := 0
	var lastErr error

	for _, login := range logins {
		if err := s.Update(ctx, login.ID, login); err != nil {
			s.logger.Warn("bulk update failed for login", "id", login.ID, "error", err)
			lastErr = err
			// Continue with other logins instead of failing completely
			continue
		}
		successCount++
	}

	s.logger.Info("bulk update completed", "total", len(logins), "success", successCount)

	// Return error if all failed
	if successCount == 0 && lastErr != nil {
		return fmt.Errorf("bulk update failed: %w", lastErr)
	}

	return nil
}

// validateLogin validates login fields
func (s *loginService) validateLogin(login *domain.Login) error {
	if login.Title == "" {
		return repository.ErrInvalidInput
	}

	// URL is optional but if provided, could validate format
	// Username and Password are optional (some logins might only have title/notes)

	return nil
}
