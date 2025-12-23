package service

import (
	"context"
	"fmt"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
)

type emailService struct {
	repo      repository.EmailRepository
	encryptor Encryptor
	logger    Logger
}

// NewEmailService creates a new email service
func NewEmailService(repo repository.EmailRepository, encryptor Encryptor, logger Logger) EmailService {
	return &emailService{
		repo:      repo,
		encryptor: encryptor,
		logger:    logger,
	}
}

func (s *emailService) GetByID(ctx context.Context, id uint) (*domain.Email, error) {
	email, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("failed to get email", "id", id, "error", err)
		return nil, err
	}

	if err := s.encryptor.DecryptModel(email, ""); err != nil {
		s.logger.Error("failed to decrypt email", "id", id, "error", err)
		return nil, fmt.Errorf("failed to decrypt email: %w", err)
	}

	s.logger.Debug("email retrieved", "id", id)
	return email, nil
}

func (s *emailService) List(ctx context.Context) ([]*domain.Email, error) {
	emails, err := s.repo.List(ctx)
	if err != nil {
		s.logger.Error("failed to list emails", "error", err)
		return nil, err
	}

	decryptedCount := 0
	for _, email := range emails {
		if err := s.encryptor.DecryptModel(email, ""); err != nil {
			s.logger.Warn("failed to decrypt email, skipping", "id", email.ID, "error", err)
			continue
		}
		decryptedCount++
	}

	s.logger.Debug("emails listed", "total", len(emails), "decrypted", decryptedCount)
	return emails, nil
}

func (s *emailService) Create(ctx context.Context, email *domain.Email) error {
	if email.Title == "" {
		return repository.ErrInvalidInput
	}

	if err := s.encryptor.EncryptModel(email, ""); err != nil {
		s.logger.Error("failed to encrypt email", "error", err)
		return fmt.Errorf("failed to encrypt email: %w", err)
	}

	if err := s.repo.Create(ctx, email); err != nil {
		s.logger.Error("failed to create email", "error", err)
		return err
	}

	s.logger.Info("email created", "id", email.ID, "title", email.Title)
	return nil
}

func (s *emailService) Update(ctx context.Context, id uint, email *domain.Email) error {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("email not found for update", "id", id, "error", err)
		return err
	}

	if err := s.encryptor.EncryptModel(email, ""); err != nil {
		s.logger.Error("failed to encrypt email", "id", id, "error", err)
		return fmt.Errorf("failed to encrypt email: %w", err)
	}

	email.ID = existing.ID
	email.CreatedAt = existing.CreatedAt

	if err := s.repo.Update(ctx, email); err != nil {
		s.logger.Error("failed to update email", "id", id, "error", err)
		return err
	}

	s.logger.Info("email updated", "id", id)
	return nil
}

func (s *emailService) Delete(ctx context.Context, id uint) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		s.logger.Error("failed to delete email", "id", id, "error", err)
		return err
	}

	s.logger.Info("email deleted", "id", id)
	return nil
}

func (s *emailService) BulkUpdate(ctx context.Context, emails []*domain.Email) error {
	successCount := 0
	var lastErr error

	for _, email := range emails {
		if err := s.Update(ctx, email.ID, email); err != nil {
			s.logger.Warn("bulk update failed for email", "id", email.ID, "error", err)
			lastErr = err
			continue
		}
		successCount++
	}

	s.logger.Info("bulk update completed", "total", len(emails), "success", successCount)

	if successCount == 0 && lastErr != nil {
		return fmt.Errorf("bulk update failed: %w", lastErr)
	}

	return nil
}
