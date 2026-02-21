package gormrepo

import (
	"context"
	"errors"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"gorm.io/gorm"
)

type scimTokenRepository struct {
	db *gorm.DB
}

// NewSCIMTokenRepository creates a new SCIM token repository
func NewSCIMTokenRepository(db *gorm.DB) repository.SCIMTokenRepository {
	return &scimTokenRepository{db: db}
}

func (r *scimTokenRepository) Create(ctx context.Context, token *domain.SCIMToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

func (r *scimTokenRepository) GetByID(ctx context.Context, id uint) (*domain.SCIMToken, error) {
	var token domain.SCIMToken
	if err := r.db.WithContext(ctx).First(&token, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &token, nil
}

func (r *scimTokenRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*domain.SCIMToken, error) {
	var token domain.SCIMToken
	if err := r.db.WithContext(ctx).
		Where("token_hash = ? AND is_active = ?", tokenHash, true).
		First(&token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &token, nil
}

func (r *scimTokenRepository) ListByOrganization(ctx context.Context, orgID uint) ([]*domain.SCIMToken, error) {
	var tokens []*domain.SCIMToken
	if err := r.db.WithContext(ctx).
		Where("organization_id = ?", orgID).
		Order("created_at DESC").
		Find(&tokens).Error; err != nil {
		return nil, err
	}
	return tokens, nil
}

func (r *scimTokenRepository) Update(ctx context.Context, token *domain.SCIMToken) error {
	return r.db.WithContext(ctx).Save(token).Error
}

func (r *scimTokenRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&domain.SCIMToken{}, id).Error
}
