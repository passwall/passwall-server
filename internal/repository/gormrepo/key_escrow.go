package gormrepo

import (
	"context"
	"errors"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"gorm.io/gorm"
)

// --- KeyEscrow Repository ---

type keyEscrowRepository struct {
	db *gorm.DB
}

func NewKeyEscrowRepository(db *gorm.DB) repository.KeyEscrowRepository {
	return &keyEscrowRepository{db: db}
}

func (r *keyEscrowRepository) Create(ctx context.Context, escrow *domain.KeyEscrow) error {
	return r.db.WithContext(ctx).Create(escrow).Error
}

func (r *keyEscrowRepository) GetByUserAndOrg(ctx context.Context, userID, orgID uint) (*domain.KeyEscrow, error) {
	var escrow domain.KeyEscrow
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND organization_id = ? AND status = ?", userID, orgID, domain.KeyEscrowStatusActive).
		First(&escrow).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &escrow, nil
}

func (r *keyEscrowRepository) ListByOrganization(ctx context.Context, orgID uint) ([]*domain.KeyEscrow, error) {
	var escrows []*domain.KeyEscrow
	if err := r.db.WithContext(ctx).
		Where("organization_id = ?", orgID).
		Preload("User").
		Order("created_at DESC").
		Find(&escrows).Error; err != nil {
		return nil, err
	}
	return escrows, nil
}

func (r *keyEscrowRepository) Update(ctx context.Context, escrow *domain.KeyEscrow) error {
	return r.db.WithContext(ctx).Save(escrow).Error
}

func (r *keyEscrowRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&domain.KeyEscrow{}, id).Error
}

func (r *keyEscrowRepository) DeleteByUserAndOrg(ctx context.Context, userID, orgID uint) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND organization_id = ?", userID, orgID).
		Delete(&domain.KeyEscrow{}).Error
}

// --- OrgEscrowKey Repository ---

type orgEscrowKeyRepository struct {
	db *gorm.DB
}

func NewOrgEscrowKeyRepository(db *gorm.DB) repository.OrgEscrowKeyRepository {
	return &orgEscrowKeyRepository{db: db}
}

func (r *orgEscrowKeyRepository) Create(ctx context.Context, key *domain.OrgEscrowKey) error {
	return r.db.WithContext(ctx).Create(key).Error
}

func (r *orgEscrowKeyRepository) GetByOrganizationID(ctx context.Context, orgID uint) (*domain.OrgEscrowKey, error) {
	var key domain.OrgEscrowKey
	if err := r.db.WithContext(ctx).
		Where("organization_id = ? AND status = ?", orgID, domain.KeyEscrowStatusActive).
		First(&key).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &key, nil
}

func (r *orgEscrowKeyRepository) Update(ctx context.Context, key *domain.OrgEscrowKey) error {
	return r.db.WithContext(ctx).Save(key).Error
}

func (r *orgEscrowKeyRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&domain.OrgEscrowKey{}, id).Error
}
