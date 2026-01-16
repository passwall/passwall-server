package gormrepo

import (
	"context"
	"errors"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	uuid "github.com/satori/go.uuid"
	"gorm.io/gorm"
)

type collectionRepository struct {
	db *gorm.DB
}

// NewCollectionRepository creates a new collection repository
func NewCollectionRepository(db *gorm.DB) repository.CollectionRepository {
	return &collectionRepository{db: db}
}

func (r *collectionRepository) Create(ctx context.Context, collection *domain.Collection) error {
	// Generate UUID if not set
	if collection.UUID == uuid.Nil {
		collection.UUID = uuid.NewV4()
	}

	return r.db.WithContext(ctx).Create(collection).Error
}

func (r *collectionRepository) GetByID(ctx context.Context, id uint) (*domain.Collection, error) {
	var collection domain.Collection
	err := r.db.WithContext(ctx).
		Preload("Organization").
		Where("id = ? AND deleted_at IS NULL", id).
		First(&collection).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &collection, nil
}

func (r *collectionRepository) GetByUUID(ctx context.Context, uuidStr string) (*domain.Collection, error) {
	var collection domain.Collection
	err := r.db.WithContext(ctx).
		Preload("Organization").
		Where("uuid = ? AND deleted_at IS NULL", uuidStr).
		First(&collection).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &collection, nil
}

func (r *collectionRepository) GetByName(ctx context.Context, orgID uint, name string) (*domain.Collection, error) {
	var collection domain.Collection
	err := r.db.WithContext(ctx).
		Where("organization_id = ? AND name = ? AND deleted_at IS NULL", orgID, name).
		First(&collection).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &collection, nil
}

func (r *collectionRepository) GetDefaultByOrganization(ctx context.Context, orgID uint) (*domain.Collection, error) {
	var collection domain.Collection
	err := r.db.WithContext(ctx).
		Where("organization_id = ? AND is_default = true AND deleted_at IS NULL", orgID).
		First(&collection).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}

	return &collection, nil
}

func (r *collectionRepository) ListByOrganization(ctx context.Context, orgID uint) ([]*domain.Collection, error) {
	var collections []*domain.Collection
	err := r.db.WithContext(ctx).
		Where("organization_id = ? AND deleted_at IS NULL", orgID).
		Order("name ASC").
		Find(&collections).Error

	if err != nil {
		return nil, err
	}
	return collections, nil
}

func (r *collectionRepository) ListForUser(ctx context.Context, orgID, userID uint) ([]*domain.Collection, error) {
	var collections []*domain.Collection

	// Complex query: Get collections where user has direct access OR team access
	// First, get organization_user_id for the user
	var orgUserID uint
	err := r.db.WithContext(ctx).
		Model(&domain.OrganizationUser{}).
		Select("id").
		Where("organization_id = ? AND user_id = ?", orgID, userID).
		Scan(&orgUserID).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// User not in organization, return empty list
			return []*domain.Collection{}, nil
		}
		return nil, err
	}

	// Use Raw SQL with UNION for better performance
	query := `
		SELECT DISTINCT collections.*
		FROM collections
		WHERE collections.organization_id = ? 
		  AND collections.deleted_at IS NULL
		  AND (
			  -- Direct user access
			  EXISTS (
				  SELECT 1 FROM collection_users 
				  WHERE collection_users.collection_id = collections.id 
				    AND collection_users.organization_user_id = ?
			  )
			  OR
			  -- Team access
			  EXISTS (
				  SELECT 1 FROM collection_teams
				  INNER JOIN team_users ON team_users.team_id = collection_teams.team_id
				  WHERE collection_teams.collection_id = collections.id
				    AND team_users.organization_user_id = ?
			  )
		  )
		ORDER BY collections.name ASC
	`

	err = r.db.WithContext(ctx).
		Raw(query, orgID, orgUserID, orgUserID).
		Scan(&collections).Error

	if err != nil {
		return nil, err
	}

	return collections, nil
}

func (r *collectionRepository) Update(ctx context.Context, collection *domain.Collection) error {
	// Clear associations
	collection.Organization = nil
	collection.UserAccess = nil
	collection.TeamAccess = nil
	collection.Items = nil

	return r.db.WithContext(ctx).Save(collection).Error
}

func (r *collectionRepository) Delete(ctx context.Context, id uint) error {
	// Hard delete - will cascade delete collection_users, collection_teams
	return r.db.WithContext(ctx).Unscoped().Delete(&domain.Collection{}, id).Error
}

func (r *collectionRepository) SoftDelete(ctx context.Context, id uint) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&domain.Collection{}).
		Where("id = ?", id).
		Update("deleted_at", now).Error
}

func (r *collectionRepository) GetItemCount(ctx context.Context, collectionID uint) (int, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&domain.OrganizationItem{}).
		Where("collection_id = ? AND deleted_at IS NULL", collectionID).
		Count(&count).Error

	return int(count), err
}

func (r *collectionRepository) GetUserCount(ctx context.Context, collectionID uint) (int, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&domain.CollectionUser{}).
		Where("collection_id = ?", collectionID).
		Count(&count).Error

	return int(count), err
}

func (r *collectionRepository) GetTeamCount(ctx context.Context, collectionID uint) (int, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&domain.CollectionTeam{}).
		Where("collection_id = ?", collectionID).
		Count(&count).Error

	return int(count), err
}
