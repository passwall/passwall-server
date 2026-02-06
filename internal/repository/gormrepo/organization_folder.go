package gormrepo

import (
	"context"
	"errors"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"gorm.io/gorm"
)

type organizationFolderRepository struct {
	db *gorm.DB
}

func NewOrganizationFolderRepository(db *gorm.DB) repository.OrganizationFolderRepository {
	return &organizationFolderRepository{db: db}
}

func (r *organizationFolderRepository) Create(ctx context.Context, folder *domain.OrganizationFolder) error {
	return r.db.WithContext(ctx).Create(folder).Error
}

func (r *organizationFolderRepository) GetByID(ctx context.Context, id uint) (*domain.OrganizationFolder, error) {
	var folder domain.OrganizationFolder
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&folder).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &folder, nil
}

func (r *organizationFolderRepository) GetByOrganization(ctx context.Context, orgID uint) ([]*domain.OrganizationFolder, error) {
	var folders []*domain.OrganizationFolder
	err := r.db.WithContext(ctx).
		Where("organization_id = ?", orgID).
		Order("name ASC").
		Find(&folders).Error
	if err != nil {
		return nil, err
	}
	return folders, nil
}

func (r *organizationFolderRepository) GetByOrganizationAndName(ctx context.Context, orgID uint, name string) (*domain.OrganizationFolder, error) {
	var folder domain.OrganizationFolder
	err := r.db.WithContext(ctx).
		Where("organization_id = ? AND name = ?", orgID, name).
		First(&folder).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &folder, nil
}

func (r *organizationFolderRepository) Update(ctx context.Context, folder *domain.OrganizationFolder) error {
	result := r.db.WithContext(ctx).
		Where("id = ?", folder.ID).
		Updates(folder)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *organizationFolderRepository) Delete(ctx context.Context, id uint) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&domain.OrganizationFolder{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return repository.ErrNotFound
	}
	return nil
}
