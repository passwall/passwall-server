package gormrepo

import (
	"context"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"gorm.io/gorm"
)

type organizationEntitlementOverrideRepository struct {
	db *gorm.DB
}

func NewOrganizationEntitlementOverrideRepository(
	db *gorm.DB,
) repository.OrganizationEntitlementOverrideRepository {
	return &organizationEntitlementOverrideRepository{db: db}
}

func (r *organizationEntitlementOverrideRepository) ListActiveByOrganization(
	ctx context.Context,
	orgID uint,
	now time.Time,
) ([]*domain.OrganizationEntitlementOverride, error) {
	var overrides []*domain.OrganizationEntitlementOverride
	err := dbFromContext(ctx, r.db).
		Where("organization_id = ? AND (expires_at IS NULL OR expires_at > ?)", orgID, now).
		Order("created_at ASC").
		Find(&overrides).Error
	return overrides, err
}
