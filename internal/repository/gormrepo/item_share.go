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

type itemShareRepository struct {
	db *gorm.DB
}

// NewItemShareRepository creates a new item share repository
func NewItemShareRepository(db *gorm.DB) repository.ItemShareRepository {
	return &itemShareRepository{db: db}
}

func (r *itemShareRepository) Create(ctx context.Context, share *domain.ItemShare) error {
	// Generate UUID if not set
	if share.UUID == uuid.Nil {
		share.UUID = uuid.NewV4()
	}

	return r.db.WithContext(ctx).Create(share).Error
}

func (r *itemShareRepository) GetByID(ctx context.Context, id uint) (*domain.ItemShare, error) {
	var share domain.ItemShare
	err := r.db.WithContext(ctx).
		Preload("Owner").
		Preload("SharedWithUser").
		Preload("SharedWithTeam").
		Where("id = ?", id).
		First(&share).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &share, nil
}

func (r *itemShareRepository) GetByUUID(ctx context.Context, uuidStr string) (*domain.ItemShare, error) {
	var share domain.ItemShare
	err := r.db.WithContext(ctx).
		Preload("Owner").
		Preload("SharedWithUser").
		Preload("SharedWithTeam").
		Where("uuid = ?", uuidStr).
		First(&share).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &share, nil
}

func (r *itemShareRepository) ListByItemUUID(ctx context.Context, itemUUID uuid.UUID) ([]*domain.ItemShare, error) {
	var shares []*domain.ItemShare
	err := r.db.WithContext(ctx).
		Preload("SharedWithUser").
		Preload("SharedWithTeam").
		Where("item_uuid = ?", itemUUID).
		Order("created_at DESC").
		Find(&shares).Error

	if err != nil {
		return nil, err
	}
	return shares, nil
}

func (r *itemShareRepository) ListByOwner(ctx context.Context, ownerID uint) ([]*domain.ItemShare, error) {
	var shares []*domain.ItemShare
	err := r.db.WithContext(ctx).
		Preload("SharedWithUser").
		Preload("SharedWithTeam").
		Where("owner_id = ?", ownerID).
		Order("created_at DESC").
		Find(&shares).Error

	if err != nil {
		return nil, err
	}
	return shares, nil
}

func (r *itemShareRepository) ListSharedWithUser(ctx context.Context, userID uint) ([]*domain.ItemShare, error) {
	var shares []*domain.ItemShare
	err := r.db.WithContext(ctx).
		Preload("Owner").
		Where("shared_with_user_id = ?", userID).
		Where("expires_at IS NULL OR expires_at > ?", time.Now()).
		Order("created_at DESC").
		Find(&shares).Error

	if err != nil {
		return nil, err
	}
	return shares, nil
}

func (r *itemShareRepository) ListSharedWithTeam(ctx context.Context, teamID uint) ([]*domain.ItemShare, error) {
	var shares []*domain.ItemShare
	err := r.db.WithContext(ctx).
		Preload("Owner").
		Preload("SharedWithTeam").
		Where("shared_with_team_id = ?", teamID).
		Where("expires_at IS NULL OR expires_at > ?", time.Now()).
		Order("created_at DESC").
		Find(&shares).Error

	if err != nil {
		return nil, err
	}
	return shares, nil
}

func (r *itemShareRepository) Update(ctx context.Context, share *domain.ItemShare) error {
	// Clear associations
	share.Owner = nil
	share.SharedWithUser = nil
	share.SharedWithTeam = nil

	return r.db.WithContext(ctx).Save(share).Error
}

func (r *itemShareRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&domain.ItemShare{}, id).Error
}

func (r *itemShareRepository) DeleteExpired(ctx context.Context) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("expires_at IS NOT NULL AND expires_at <= ?", time.Now()).
		Delete(&domain.ItemShare{})

	return result.RowsAffected, result.Error
}

