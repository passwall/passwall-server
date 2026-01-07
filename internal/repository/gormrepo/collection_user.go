package gormrepo

import (
	"context"
	"errors"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"gorm.io/gorm"
)

type collectionUserRepository struct {
	db *gorm.DB
}

// NewCollectionUserRepository creates a new collection user repository
func NewCollectionUserRepository(db *gorm.DB) repository.CollectionUserRepository {
	return &collectionUserRepository{db: db}
}

func (r *collectionUserRepository) Create(ctx context.Context, cu *domain.CollectionUser) error {
	return r.db.WithContext(ctx).Create(cu).Error
}

func (r *collectionUserRepository) GetByID(ctx context.Context, id uint) (*domain.CollectionUser, error) {
	var cu domain.CollectionUser
	err := r.db.WithContext(ctx).
		Preload("Collection").
		Preload("OrganizationUser").
		Preload("OrganizationUser.User").
		Where("id = ?", id).
		First(&cu).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &cu, nil
}

func (r *collectionUserRepository) GetByCollectionAndOrgUser(ctx context.Context, collectionID, orgUserID uint) (*domain.CollectionUser, error) {
	var cu domain.CollectionUser
	err := r.db.WithContext(ctx).
		Preload("Collection").
		Preload("OrganizationUser").
		Preload("OrganizationUser.User").
		Where("collection_id = ? AND organization_user_id = ?", collectionID, orgUserID).
		First(&cu).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &cu, nil
}

func (r *collectionUserRepository) ListByCollection(ctx context.Context, collectionID uint) ([]*domain.CollectionUser, error) {
	var cus []*domain.CollectionUser
	err := r.db.WithContext(ctx).
		Preload("OrganizationUser").
		Preload("OrganizationUser.User").
		Where("collection_id = ?", collectionID).
		Order("created_at ASC").
		Find(&cus).Error

	if err != nil {
		return nil, err
	}
	return cus, nil
}

func (r *collectionUserRepository) ListByOrgUser(ctx context.Context, orgUserID uint) ([]*domain.CollectionUser, error) {
	var cus []*domain.CollectionUser
	err := r.db.WithContext(ctx).
		Preload("Collection").
		Where("organization_user_id = ?", orgUserID).
		Order("created_at ASC").
		Find(&cus).Error

	if err != nil {
		return nil, err
	}
	return cus, nil
}

func (r *collectionUserRepository) Update(ctx context.Context, cu *domain.CollectionUser) error {
	// Clear associations
	cu.Collection = nil
	cu.OrganizationUser = nil

	return r.db.WithContext(ctx).Save(cu).Error
}

func (r *collectionUserRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&domain.CollectionUser{}, id).Error
}

func (r *collectionUserRepository) DeleteByCollectionAndOrgUser(ctx context.Context, collectionID, orgUserID uint) error {
	return r.db.WithContext(ctx).
		Where("collection_id = ? AND organization_user_id = ?", collectionID, orgUserID).
		Delete(&domain.CollectionUser{}).Error
}

