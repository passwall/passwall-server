package gormrepo

import (
	"context"
	"errors"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"gorm.io/gorm"
)

type organizationPolicyRepository struct {
	db *gorm.DB
}

// NewOrganizationPolicyRepository creates a new organization policy repository
func NewOrganizationPolicyRepository(db *gorm.DB) repository.OrganizationPolicyRepository {
	return &organizationPolicyRepository{db: db}
}

func (r *organizationPolicyRepository) Create(ctx context.Context, policy *domain.OrganizationPolicy) error {
	return dbFromContext(ctx, r.db).Create(policy).Error
}

func (r *organizationPolicyRepository) GetByID(ctx context.Context, id uint) (*domain.OrganizationPolicy, error) {
	var policy domain.OrganizationPolicy
	if err := dbFromContext(ctx, r.db).First(&policy, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &policy, nil
}

func (r *organizationPolicyRepository) GetByOrgAndType(ctx context.Context, orgID uint, policyType domain.PolicyType) (*domain.OrganizationPolicy, error) {
	var policy domain.OrganizationPolicy
	if err := dbFromContext(ctx, r.db).
		Where("organization_id = ? AND type = ?", orgID, policyType).
		First(&policy).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &policy, nil
}

func (r *organizationPolicyRepository) ListByOrganization(ctx context.Context, orgID uint) ([]*domain.OrganizationPolicy, error) {
	var policies []*domain.OrganizationPolicy
	if err := dbFromContext(ctx, r.db).
		Where("organization_id = ?", orgID).
		Order("type ASC").
		Find(&policies).Error; err != nil {
		return nil, err
	}
	return policies, nil
}

func (r *organizationPolicyRepository) ListEnabledByOrganization(ctx context.Context, orgID uint) ([]*domain.OrganizationPolicy, error) {
	var policies []*domain.OrganizationPolicy
	if err := dbFromContext(ctx, r.db).
		Where("organization_id = ? AND enabled = ?", orgID, true).
		Order("type ASC").
		Find(&policies).Error; err != nil {
		return nil, err
	}
	return policies, nil
}

func (r *organizationPolicyRepository) Update(ctx context.Context, policy *domain.OrganizationPolicy) error {
	return dbFromContext(ctx, r.db).Save(policy).Error
}

func (r *organizationPolicyRepository) Delete(ctx context.Context, id uint) error {
	return dbFromContext(ctx, r.db).Delete(&domain.OrganizationPolicy{}, id).Error
}
