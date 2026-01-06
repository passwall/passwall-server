package service

import (
	"context"
	"fmt"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	uuid "github.com/satori/go.uuid"
)

type folderService struct {
	repo   repository.FolderRepository
	logger Logger
}

// NewFolderService creates a new folder service
func NewFolderService(
	repo repository.FolderRepository,
	logger Logger,
) FolderService {
	return &folderService{
		repo:   repo,
		logger: logger,
	}
}

func (s *folderService) Create(ctx context.Context, userID uint, req *domain.CreateFolderRequest) (*domain.Folder, error) {
	// Check if folder name already exists
	existing, err := s.repo.GetByUserIDAndName(ctx, userID, req.Name)
	if err != nil && err != repository.ErrNotFound {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("folder name already exists")
	}

	// Create folder
	folder := &domain.Folder{
		UUID:   uuid.NewV4(),
		UserID: userID,
		Name:   req.Name,
	}

	if err := s.repo.Create(ctx, folder); err != nil {
		s.logger.Error("failed to create folder", "user_id", userID, "name", req.Name, "error", err)
		return nil, err
	}

	s.logger.Info("folder created", "user_id", userID, "folder_id", folder.ID, "name", req.Name)
	return folder, nil
}

func (s *folderService) GetByUserID(ctx context.Context, userID uint) ([]*domain.Folder, error) {
	return s.repo.GetByUserID(ctx, userID)
}

func (s *folderService) Update(ctx context.Context, id uint, userID uint, req *domain.UpdateFolderRequest) (*domain.Folder, error) {
	// Get existing folder
	folder, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, fmt.Errorf("folder not found")
		}
		return nil, err
	}

	// Check if new name conflicts with another folder
	if req.Name != folder.Name {
		existing, err := s.repo.GetByUserIDAndName(ctx, userID, req.Name)
		if err != nil && err != repository.ErrNotFound {
			return nil, err
		}
		if existing != nil && existing.ID != folder.ID {
			return nil, fmt.Errorf("folder name already exists")
		}
	}

	// Update folder
	folder.Name = req.Name

	if err := s.repo.Update(ctx, folder, userID); err != nil {
		s.logger.Error("failed to update folder", "folder_id", id, "user_id", userID, "error", err)
		return nil, err
	}

	s.logger.Info("folder updated", "folder_id", id, "user_id", userID)
	return folder, nil
}

func (s *folderService) Delete(ctx context.Context, id uint, userID uint) error {
	if err := s.repo.Delete(ctx, id, userID); err != nil {
		if err.Error() == "cannot delete folder: contains items" {
			return fmt.Errorf("cannot delete folder: contains items")
		}
		if err == repository.ErrNotFound {
			return fmt.Errorf("folder not found")
		}
		s.logger.Error("failed to delete folder", "folder_id", id, "user_id", userID, "error", err)
		return err
	}

	s.logger.Info("folder deleted", "folder_id", id, "user_id", userID)
	return nil
}
