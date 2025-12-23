package service

import (
	"context"
	"fmt"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
)

type bankAccountService struct {
	repo      repository.BankAccountRepository
	encryptor Encryptor
	logger    Logger
}

// NewBankAccountService creates a new bank account service
func NewBankAccountService(repo repository.BankAccountRepository, encryptor Encryptor, logger Logger) BankAccountService {
	return &bankAccountService{
		repo:      repo,
		encryptor: encryptor,
		logger:    logger,
	}
}

func (s *bankAccountService) GetByID(ctx context.Context, id uint) (*domain.BankAccount, error) {
	account, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("failed to get bank account", "id", id, "error", err)
		return nil, err
	}

	if err := s.encryptor.DecryptModel(account, ""); err != nil {
		s.logger.Error("failed to decrypt bank account", "id", id, "error", err)
		return nil, fmt.Errorf("failed to decrypt bank account: %w", err)
	}

	s.logger.Debug("bank account retrieved", "id", id)
	return account, nil
}

func (s *bankAccountService) List(ctx context.Context) ([]*domain.BankAccount, error) {
	accounts, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, account := range accounts {
		if err := s.encryptor.DecryptModel(account, ""); err != nil {
			continue
		}
	}

	return accounts, nil
}

func (s *bankAccountService) Create(ctx context.Context, account *domain.BankAccount) error {
	// Validate
	if account.BankName == "" {
		return repository.ErrInvalidInput
	}

	if err := s.encryptor.EncryptModel(account, ""); err != nil {
		s.logger.Error("failed to encrypt bank account", "error", err)
		return fmt.Errorf("failed to encrypt bank account: %w", err)
	}

	if err := s.repo.Create(ctx, account); err != nil {
		s.logger.Error("failed to create bank account", "error", err)
		return err
	}

	s.logger.Info("bank account created", "id", account.ID, "name", account.BankName)
	return nil
}

func (s *bankAccountService) Update(ctx context.Context, id uint, account *domain.BankAccount) error {
	// Check existence
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("bank account not found for update", "id", id, "error", err)
		return err
	}

	// Encrypt sensitive fields
	if err := s.encryptor.EncryptModel(account, ""); err != nil {
		s.logger.Error("failed to encrypt bank account", "id", id, "error", err)
		return fmt.Errorf("failed to encrypt bank account: %w", err)
	}

	// Update using full model
	account.ID = existing.ID
	account.CreatedAt = existing.CreatedAt

	if err := s.repo.Update(ctx, account); err != nil {
		s.logger.Error("failed to update bank account", "id", id, "error", err)
		return err
	}

	s.logger.Info("bank account updated", "id", id)
	return nil
}

func (s *bankAccountService) Delete(ctx context.Context, id uint) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		s.logger.Error("failed to delete bank account", "id", id, "error", err)
		return err
	}

	s.logger.Info("bank account deleted", "id", id)
	return nil
}

func (s *bankAccountService) BulkUpdate(ctx context.Context, accounts []*domain.BankAccount) error {
	for _, account := range accounts {
		if err := s.Update(ctx, account.ID, account); err != nil {
			return err
		}
	}
	return nil
}

