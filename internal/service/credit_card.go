package service

import (
	"context"
	"fmt"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
)

type creditCardService struct {
	repo      repository.CreditCardRepository
	encryptor Encryptor
	logger    Logger
}

// NewCreditCardService creates a new credit card service
func NewCreditCardService(repo repository.CreditCardRepository, encryptor Encryptor, logger Logger) CreditCardService {
	return &creditCardService{
		repo:      repo,
		encryptor: encryptor,
		logger:    logger,
	}
}

func (s *creditCardService) GetByID(ctx context.Context, id uint) (*domain.CreditCard, error) {
	card, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("failed to get credit card", "id", id, "error", err)
		return nil, err
	}

	if err := s.encryptor.DecryptModel(card, ""); err != nil {
		s.logger.Error("failed to decrypt credit card", "id", id, "error", err)
		return nil, fmt.Errorf("failed to decrypt credit card: %w", err)
	}

	s.logger.Debug("credit card retrieved", "id", id)
	return card, nil
}

func (s *creditCardService) List(ctx context.Context) ([]*domain.CreditCard, error) {
	cards, err := s.repo.List(ctx)
	if err != nil {
		s.logger.Error("failed to list credit cards", "error", err)
		return nil, err
	}

	decryptedCount := 0
	for _, card := range cards {
		if err := s.encryptor.DecryptModel(card, ""); err != nil {
			s.logger.Warn("failed to decrypt credit card, skipping", "id", card.ID, "error", err)
			continue
		}
		decryptedCount++
	}

	s.logger.Debug("credit cards listed", "total", len(cards), "decrypted", decryptedCount)
	return cards, nil
}

func (s *creditCardService) Create(ctx context.Context, card *domain.CreditCard) error {
	if card.CardName == "" {
		return repository.ErrInvalidInput
	}

	if err := s.encryptor.EncryptModel(card, ""); err != nil {
		s.logger.Error("failed to encrypt credit card", "error", err)
		return fmt.Errorf("failed to encrypt credit card: %w", err)
	}

	if err := s.repo.Create(ctx, card); err != nil {
		s.logger.Error("failed to create credit card", "error", err)
		return err
	}

	s.logger.Info("credit card created", "id", card.ID, "name", card.CardName)
	return nil
}

func (s *creditCardService) Update(ctx context.Context, id uint, card *domain.CreditCard) error {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("credit card not found for update", "id", id, "error", err)
		return err
	}

	if err := s.encryptor.EncryptModel(card, ""); err != nil {
		s.logger.Error("failed to encrypt credit card", "id", id, "error", err)
		return fmt.Errorf("failed to encrypt credit card: %w", err)
	}

	card.ID = existing.ID
	card.CreatedAt = existing.CreatedAt

	if err := s.repo.Update(ctx, card); err != nil {
		s.logger.Error("failed to update credit card", "id", id, "error", err)
		return err
	}

	s.logger.Info("credit card updated", "id", id)
	return nil
}

func (s *creditCardService) Delete(ctx context.Context, id uint) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		s.logger.Error("failed to delete credit card", "id", id, "error", err)
		return err
	}

	s.logger.Info("credit card deleted", "id", id)
	return nil
}

func (s *creditCardService) BulkUpdate(ctx context.Context, cards []*domain.CreditCard) error {
	successCount := 0
	var lastErr error

	for _, card := range cards {
		if err := s.Update(ctx, card.ID, card); err != nil {
			s.logger.Warn("bulk update failed for credit card", "id", card.ID, "error", err)
			lastErr = err
			continue
		}
		successCount++
	}

	s.logger.Info("bulk update completed", "total", len(cards), "success", successCount)

	if successCount == 0 && lastErr != nil {
		return fmt.Errorf("bulk update failed: %w", lastErr)
	}

	return nil
}
