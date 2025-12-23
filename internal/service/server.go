package service

import (
	"context"
	"fmt"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
)

type serverService struct {
	repo      repository.ServerRepository
	encryptor Encryptor
	logger    Logger
}

// NewServerService creates a new server service
func NewServerService(repo repository.ServerRepository, encryptor Encryptor, logger Logger) ServerService {
	return &serverService{
		repo:      repo,
		encryptor: encryptor,
		logger:    logger,
	}
}

func (s *serverService) GetByID(ctx context.Context, id uint) (*domain.Server, error) {
	server, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("failed to get server", "id", id, "error", err)
		return nil, err
	}

	if err := s.encryptor.DecryptModel(server, ""); err != nil {
		s.logger.Error("failed to decrypt server", "id", id, "error", err)
		return nil, fmt.Errorf("failed to decrypt server: %w", err)
	}

	s.logger.Debug("server retrieved", "id", id)
	return server, nil
}

func (s *serverService) List(ctx context.Context) ([]*domain.Server, error) {
	servers, err := s.repo.List(ctx)
	if err != nil {
		s.logger.Error("failed to list servers", "error", err)
		return nil, err
	}

	decryptedCount := 0
	for _, server := range servers {
		if err := s.encryptor.DecryptModel(server, ""); err != nil {
			s.logger.Warn("failed to decrypt server, skipping", "id", server.ID, "error", err)
			continue
		}
		decryptedCount++
	}

	s.logger.Debug("servers listed", "total", len(servers), "decrypted", decryptedCount)
	return servers, nil
}

func (s *serverService) Create(ctx context.Context, server *domain.Server) error {
	if server.Title == "" {
		return repository.ErrInvalidInput
	}

	if err := s.encryptor.EncryptModel(server, ""); err != nil {
		s.logger.Error("failed to encrypt server", "error", err)
		return fmt.Errorf("failed to encrypt server: %w", err)
	}

	if err := s.repo.Create(ctx, server); err != nil {
		s.logger.Error("failed to create server", "error", err)
		return err
	}

	s.logger.Info("server created", "id", server.ID, "title", server.Title)
	return nil
}

func (s *serverService) Update(ctx context.Context, id uint, server *domain.Server) error {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("server not found for update", "id", id, "error", err)
		return err
	}

	if err := s.encryptor.EncryptModel(server, ""); err != nil {
		s.logger.Error("failed to encrypt server", "id", id, "error", err)
		return fmt.Errorf("failed to encrypt server: %w", err)
	}

	server.ID = existing.ID
	server.CreatedAt = existing.CreatedAt

	if err := s.repo.Update(ctx, server); err != nil {
		s.logger.Error("failed to update server", "id", id, "error", err)
		return err
	}

	s.logger.Info("server updated", "id", id)
	return nil
}

func (s *serverService) Delete(ctx context.Context, id uint) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		s.logger.Error("failed to delete server", "id", id, "error", err)
		return err
	}

	s.logger.Info("server deleted", "id", id)
	return nil
}

func (s *serverService) BulkUpdate(ctx context.Context, servers []*domain.Server) error {
	successCount := 0
	var lastErr error

	for _, server := range servers {
		if err := s.Update(ctx, server.ID, server); err != nil {
			s.logger.Warn("bulk update failed for server", "id", server.ID, "error", err)
			lastErr = err
			continue
		}
		successCount++
	}

	s.logger.Info("bulk update completed", "total", len(servers), "success", successCount)

	if successCount == 0 && lastErr != nil {
		return fmt.Errorf("bulk update failed: %w", lastErr)
	}

	return nil
}
