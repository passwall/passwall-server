package gormrepo

import (
	"context"
	"errors"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"gorm.io/gorm"
)

type folderRepository struct {
	db *gorm.DB
}

// NewFolderRepository creates a new folder repository
func NewFolderRepository(db *gorm.DB) repository.FolderRepository {
	return &folderRepository{db: db}
}

func (r *folderRepository) Create(ctx context.Context, folder *domain.Folder) error {
	return r.db.WithContext(ctx).Create(folder).Error
}

func (r *folderRepository) GetByID(ctx context.Context, id uint, userID uint) (*domain.Folder, error) {
	var folder domain.Folder

	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		First(&folder).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}

	return &folder, nil
}

func (r *folderRepository) GetByUserID(ctx context.Context, userID uint) ([]*domain.Folder, error) {
	var folders []*domain.Folder

	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("name ASC").
		Find(&folders).Error

	if err != nil {
		return nil, err
	}

	return folders, nil
}

func (r *folderRepository) GetByUserIDAndName(ctx context.Context, userID uint, name string) (*domain.Folder, error) {
	var folder domain.Folder

	err := r.db.WithContext(ctx).
		Where("user_id = ? AND name = ?", userID, name).
		First(&folder).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}

	return &folder, nil
}

func (r *folderRepository) Update(ctx context.Context, folder *domain.Folder, userID uint) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", folder.ID, userID).
		Updates(folder)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return repository.ErrNotFound
	}

	return nil
}

func (r *folderRepository) Delete(ctx context.Context, id uint, userID uint) error {
	// Check if folder has items
	var itemCount int64
	if err := r.db.WithContext(ctx).
		Model(&domain.Item{}).
		Where("folder_id = ? AND deleted_at IS NULL", id).
		Count(&itemCount).Error; err != nil {
		return err
	}

	if itemCount > 0 {
		return errors.New("cannot delete folder: contains items")
	}

	// Delete folder
	result := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&domain.Folder{})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return repository.ErrNotFound
	}

	return nil
}
