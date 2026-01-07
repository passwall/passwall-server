package gormrepo

import (
	"context"
	"errors"
	"fmt"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	uuid "github.com/satori/go.uuid"
	"gorm.io/gorm"
)

type organizationRepository struct {
	db *gorm.DB
}

// NewOrganizationRepository creates a new organization repository
func NewOrganizationRepository(db *gorm.DB) repository.OrganizationRepository {
	return &organizationRepository{db: db}
}

func (r *organizationRepository) Create(ctx context.Context, org *domain.Organization) error {
	// Generate UUID if not set
	if org.UUID == uuid.Nil {
		org.UUID = uuid.NewV4()
	}

	return r.db.WithContext(ctx).Create(org).Error
}

func (r *organizationRepository) GetByID(ctx context.Context, id uint) (*domain.Organization, error) {
	var org domain.Organization
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&org).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &org, nil
}

func (r *organizationRepository) GetByUUID(ctx context.Context, uuidStr string) (*domain.Organization, error) {
	var org domain.Organization
	err := r.db.WithContext(ctx).Where("uuid = ?", uuidStr).First(&org).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &org, nil
}

func (r *organizationRepository) List(ctx context.Context, filter repository.ListFilter) ([]*domain.Organization, *repository.ListResult, error) {
	var orgs []*domain.Organization
	var total int64

	query := r.db.WithContext(ctx).Model(&domain.Organization{})

	// Count total (active organizations only)
	if err := query.Where("is_active = ?", true).Count(&total).Error; err != nil {
		return nil, nil, err
	}

	// Apply search filter
	if filter.Search != "" {
		searchPattern := "%" + filter.Search + "%"
		query = query.Where("name LIKE ? OR billing_email LIKE ?", searchPattern, searchPattern)
	}

	// Count filtered
	var filtered int64
	if err := query.Count(&filtered).Error; err != nil {
		return nil, nil, err
	}

	// Apply pagination
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	// Apply sorting
	orderBy := "created_at DESC"
	if filter.Sort != "" {
		order := "ASC"
		if filter.Order == "desc" {
			order = "DESC"
		}
		orderBy = fmt.Sprintf("%s %s", filter.Sort, order)
	}
	query = query.Order(orderBy)

	// Execute query
	if err := query.Find(&orgs).Error; err != nil {
		return nil, nil, err
	}

	result := &repository.ListResult{
		Total:    total,
		Filtered: filtered,
	}

	return orgs, result, nil
}

func (r *organizationRepository) ListForUser(ctx context.Context, userID uint) ([]*domain.Organization, error) {
	var orgs []*domain.Organization

	err := r.db.WithContext(ctx).
		Joins("JOIN organization_users ON organization_users.organization_id = organizations.id").
		Where("organization_users.user_id = ? AND organization_users.status IN ?", userID, []domain.OrganizationUserStatus{
			domain.OrgUserStatusAccepted,
			domain.OrgUserStatusConfirmed,
		}).
		Where("organizations.is_active = ?", true).
		Order("organizations.name ASC").
		Find(&orgs).Error

	if err != nil {
		return nil, err
	}

	return orgs, nil
}

func (r *organizationRepository) Update(ctx context.Context, org *domain.Organization) error {
	return r.db.WithContext(ctx).Save(org).Error
}

func (r *organizationRepository) Delete(ctx context.Context, id uint) error {
	// Hard delete - will cascade delete all related records
	return r.db.WithContext(ctx).Unscoped().Delete(&domain.Organization{}, id).Error
}

func (r *organizationRepository) GetMemberCount(ctx context.Context, orgID uint) (int, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&domain.OrganizationUser{}).
		Where("organization_id = ? AND status IN ?", orgID, []domain.OrganizationUserStatus{
			domain.OrgUserStatusAccepted,
			domain.OrgUserStatusConfirmed,
		}).
		Count(&count).Error

	return int(count), err
}

func (r *organizationRepository) GetTeamCount(ctx context.Context, orgID uint) (int, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&domain.Team{}).
		Where("organization_id = ?", orgID).
		Count(&count).Error

	return int(count), err
}

func (r *organizationRepository) GetCollectionCount(ctx context.Context, orgID uint) (int, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&domain.Collection{}).
		Where("organization_id = ? AND deleted_at IS NULL", orgID).
		Count(&count).Error

	return int(count), err
}

