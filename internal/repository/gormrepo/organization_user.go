package gormrepo

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"gorm.io/gorm"
)

type organizationUserRepository struct {
	db *gorm.DB
}

// NewOrganizationUserRepository creates a new organization user repository
func NewOrganizationUserRepository(db *gorm.DB) repository.OrganizationUserRepository {
	return &organizationUserRepository{db: db}
}

func (r *organizationUserRepository) Create(ctx context.Context, orgUser *domain.OrganizationUser) error {
	// Generate UUID if not set
	if orgUser.UUID == uuid.Nil {
		orgUser.UUID = uuid.New()
	}

	return dbFromContext(ctx, r.db).Create(orgUser).Error
}

func (r *organizationUserRepository) GetByID(ctx context.Context, id uint) (*domain.OrganizationUser, error) {
	var orgUser domain.OrganizationUser
	err := dbFromContext(ctx, r.db).
		Preload("Organization").
		Preload("User").
		Where("id = ?", id).
		First(&orgUser).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &orgUser, nil
}

func (r *organizationUserRepository) GetByUUID(ctx context.Context, uuidStr string) (*domain.OrganizationUser, error) {
	var orgUser domain.OrganizationUser
	err := dbFromContext(ctx, r.db).
		Preload("Organization").
		Preload("User").
		Where("uuid = ?", uuidStr).
		First(&orgUser).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &orgUser, nil
}

func (r *organizationUserRepository) GetByOrgAndUser(ctx context.Context, orgID, userID uint) (*domain.OrganizationUser, error) {
	var orgUser domain.OrganizationUser
	err := dbFromContext(ctx, r.db).
		Preload("Organization").
		Preload("User").
		Where("organization_id = ? AND user_id = ?", orgID, userID).
		First(&orgUser).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &orgUser, nil
}

func (r *organizationUserRepository) GetActiveByOrgAndUser(ctx context.Context, orgID, userID uint) (*domain.OrganizationUser, error) {
	var orgUser domain.OrganizationUser
	err := dbFromContext(ctx, r.db).
		Joins("JOIN organizations ON organizations.id = organization_users.organization_id").
		Preload("Organization").
		Preload("User").
		Where(
			"organization_users.organization_id = ? AND organization_users.user_id = ? AND organization_users.status IN ? AND organizations.is_active = true AND organizations.deleted_at IS NULL",
			orgID,
			userID,
			[]domain.OrganizationUserStatus{domain.OrgUserStatusAccepted, domain.OrgUserStatusConfirmed},
		).
		First(&orgUser).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &orgUser, nil
}

func (r *organizationUserRepository) ListByOrganization(ctx context.Context, orgID uint) ([]*domain.OrganizationUser, error) {
	var orgUsers []*domain.OrganizationUser
	err := dbFromContext(ctx, r.db).
		Preload("User").
		Where("organization_id = ?", orgID).
		Order("created_at ASC").
		Find(&orgUsers).Error

	if err != nil {
		return nil, err
	}
	return orgUsers, nil
}

func (r *organizationUserRepository) ListOwnersByOrganizationIDs(ctx context.Context, orgIDs []uint) (map[uint]*domain.OrganizationUser, error) {
	out := make(map[uint]*domain.OrganizationUser, len(orgIDs))
	if len(orgIDs) == 0 {
		return out, nil
	}
	var owners []*domain.OrganizationUser
	err := dbFromContext(ctx, r.db).
		Preload("User").
		Where("organization_id IN ? AND role = ?", orgIDs, domain.OrgRoleOwner).
		Order("created_at ASC").
		Find(&owners).Error
	if err != nil {
		return nil, err
	}
	for _, owner := range owners {
		if owner == nil || owner.User == nil {
			continue
		}
		if _, exists := out[owner.OrganizationID]; !exists {
			out[owner.OrganizationID] = owner
		}
	}
	return out, nil
}

func (r *organizationUserRepository) ListByUser(ctx context.Context, userID uint) ([]*domain.OrganizationUser, error) {
	var orgUsers []*domain.OrganizationUser
	err := dbFromContext(ctx, r.db).
		Preload("Organization").
		Where("user_id = ?", userID).
		Order("created_at ASC").
		Find(&orgUsers).Error

	if err != nil {
		return nil, err
	}
	return orgUsers, nil
}

func (r *organizationUserRepository) Update(ctx context.Context, orgUser *domain.OrganizationUser) error {
	// Clear associations to prevent GORM from trying to update them
	orgUser.Organization = nil
	orgUser.User = nil

	return dbFromContext(ctx, r.db).Save(orgUser).Error
}

func (r *organizationUserRepository) Delete(ctx context.Context, id uint) error {
	// Hard delete - will cascade delete team memberships
	return dbFromContext(ctx, r.db).Unscoped().Delete(&domain.OrganizationUser{}, id).Error
}

func (r *organizationUserRepository) CountInvited(ctx context.Context, orgID uint) (int, error) {
	var count int64
	err := dbFromContext(ctx, r.db).
		Model(&domain.OrganizationUser{}).
		Where("organization_id = ? AND status = ?", orgID, domain.OrgUserStatusInvited).
		Count(&count).Error

	return int(count), err
}

func (r *organizationUserRepository) ListPendingInvitations(ctx context.Context, userEmail string) ([]*domain.OrganizationUser, error) {
	var orgUsers []*domain.OrganizationUser

	// Subquery to get user ID by email
	subQuery := r.db.Model(&domain.User{}).Select("id").Where("email = ?", userEmail)

	err := dbFromContext(ctx, r.db).
		Preload("Organization").
		Where("user_id IN (?) AND status = ?", subQuery, domain.OrgUserStatusInvited).
		Order("invited_at DESC").
		Find(&orgUsers).Error

	if err != nil {
		return nil, err
	}
	return orgUsers, nil
}
