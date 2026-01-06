package gormrepo

import (
	"context"
	"errors"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"gorm.io/gorm"
)

type excludedDomainRepository struct {
	db *gorm.DB
}

// NewExcludedDomainRepository creates a new excluded domain repository
func NewExcludedDomainRepository(db *gorm.DB) repository.ExcludedDomainRepository {
	return &excludedDomainRepository{db: db}
}

func (r *excludedDomainRepository) Create(ctx context.Context, excludedDomain *domain.ExcludedDomain) error {
	return r.db.WithContext(ctx).Create(excludedDomain).Error
}

func (r *excludedDomainRepository) GetByUserID(ctx context.Context, userID uint) ([]*domain.ExcludedDomain, error) {
	var domains []*domain.ExcludedDomain

	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("domain ASC").
		Find(&domains).Error

	if err != nil {
		return nil, err
	}

	return domains, nil
}

func (r *excludedDomainRepository) GetByUserIDAndDomain(ctx context.Context, userID uint, domainStr string) (*domain.ExcludedDomain, error) {
	var ed domain.ExcludedDomain

	err := r.db.WithContext(ctx).
		Where("user_id = ? AND domain = ?", userID, domainStr).
		First(&ed).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}

	return &ed, nil
}

func (r *excludedDomainRepository) Delete(ctx context.Context, id uint, userID uint) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&domain.ExcludedDomain{})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return repository.ErrNotFound
	}

	return nil
}

func (r *excludedDomainRepository) DeleteByDomain(ctx context.Context, userID uint, domainStr string) error {
	result := r.db.WithContext(ctx).
		Where("user_id = ? AND domain = ?", userID, domainStr).
		Delete(&domain.ExcludedDomain{})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return repository.ErrNotFound
	}

	return nil
}
