package service

import (
	"context"
	"fmt"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
)

type noteService struct {
	repo      repository.NoteRepository
	encryptor Encryptor
	logger    Logger
}

// NewNoteService creates a new note service
func NewNoteService(repo repository.NoteRepository, encryptor Encryptor, logger Logger) NoteService {
	return &noteService{
		repo:      repo,
		encryptor: encryptor,
		logger:    logger,
	}
}

func (s *noteService) GetByID(ctx context.Context, id uint) (*domain.Note, error) {
	note, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("failed to get note", "id", id, "error", err)
		return nil, err
	}

	if err := s.encryptor.DecryptModel(note, ""); err != nil {
		s.logger.Error("failed to decrypt note", "id", id, "error", err)
		return nil, fmt.Errorf("failed to decrypt note: %w", err)
	}

	s.logger.Debug("note retrieved", "id", id)
	return note, nil
}

func (s *noteService) List(ctx context.Context) ([]*domain.Note, error) {
	notes, err := s.repo.List(ctx)
	if err != nil {
		s.logger.Error("failed to list notes", "error", err)
		return nil, err
	}

	decryptedCount := 0
	for _, note := range notes {
		if err := s.encryptor.DecryptModel(note, ""); err != nil {
			s.logger.Warn("failed to decrypt note, skipping", "id", note.ID, "error", err)
			continue
		}
		decryptedCount++
	}

	s.logger.Debug("notes listed", "total", len(notes), "decrypted", decryptedCount)
	return notes, nil
}

func (s *noteService) Create(ctx context.Context, note *domain.Note) error {
	if note.Title == "" {
		return repository.ErrInvalidInput
	}

	if err := s.encryptor.EncryptModel(note, ""); err != nil {
		s.logger.Error("failed to encrypt note", "error", err)
		return fmt.Errorf("failed to encrypt note: %w", err)
	}

	if err := s.repo.Create(ctx, note); err != nil {
		s.logger.Error("failed to create note", "error", err)
		return err
	}

	s.logger.Info("note created", "id", note.ID, "title", note.Title)
	return nil
}

func (s *noteService) Update(ctx context.Context, id uint, note *domain.Note) error {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("note not found for update", "id", id, "error", err)
		return err
	}

	if err := s.encryptor.EncryptModel(note, ""); err != nil {
		s.logger.Error("failed to encrypt note", "id", id, "error", err)
		return fmt.Errorf("failed to encrypt note: %w", err)
	}

	note.ID = existing.ID
	note.CreatedAt = existing.CreatedAt

	if err := s.repo.Update(ctx, note); err != nil {
		s.logger.Error("failed to update note", "id", id, "error", err)
		return err
	}

	s.logger.Info("note updated", "id", id)
	return nil
}

func (s *noteService) Delete(ctx context.Context, id uint) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		s.logger.Error("failed to delete note", "id", id, "error", err)
		return err
	}

	s.logger.Info("note deleted", "id", id)
	return nil
}

func (s *noteService) BulkUpdate(ctx context.Context, notes []*domain.Note) error {
	successCount := 0
	var lastErr error

	for _, note := range notes {
		if err := s.Update(ctx, note.ID, note); err != nil {
			s.logger.Warn("bulk update failed for note", "id", note.ID, "error", err)
			lastErr = err
			continue
		}
		successCount++
	}

	s.logger.Info("bulk update completed", "total", len(notes), "success", successCount)

	if successCount == 0 && lastErr != nil {
		return fmt.Errorf("bulk update failed: %w", lastErr)
	}

	return nil
}
