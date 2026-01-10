package repository

import (
	"context"

	"github.com/passwall/passwall-server/internal/domain"
)

// FolderRepository defines the interface for folder operations
type FolderRepository interface {
	// Create adds a new folder for a user
	Create(ctx context.Context, folder *domain.Folder) error

	// GetByID returns a folder by ID (only if it belongs to the user)
	GetByID(ctx context.Context, id uint, userID uint) (*domain.Folder, error)

	// GetByUserID returns all folders for a user
	GetByUserID(ctx context.Context, userID uint) ([]*domain.Folder, error)

	// GetByUserIDAndName checks if a folder name exists for a user
	GetByUserIDAndName(ctx context.Context, userID uint, name string) (*domain.Folder, error)

	// Update updates a folder (only if it belongs to the user)
	Update(ctx context.Context, folder *domain.Folder, userID uint) error

	// Delete removes a folder by ID (only if it belongs to the user)
	// Returns error if folder contains items
	Delete(ctx context.Context, schema string, id uint, userID uint) error
}
