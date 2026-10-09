package gormrepo

import (
	"context"
	"errors"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type organizationInvitationRepository struct {
	db *gorm.DB
}

// NewOrganizationInvitationRepository creates the organization invitation repository.
func NewOrganizationInvitationRepository(db *gorm.DB) repository.OrganizationInvitationRepository {
	return &organizationInvitationRepository{db: db}
}

func (r *organizationInvitationRepository) Create(ctx context.Context, inv *domain.OrganizationInvitation) error {
	inv.Email = domain.NormalizeInvitationEmail(inv.Email)
	return dbFromContext(ctx, r.db).Create(inv).Error
}

func (r *organizationInvitationRepository) Update(ctx context.Context, inv *domain.OrganizationInvitation) error {
	return dbFromContext(ctx, r.db).Omit(clause.Associations).Save(inv).Error
}

func (r *organizationInvitationRepository) GetByID(ctx context.Context, id uint) (*domain.OrganizationInvitation, error) {
	var inv domain.OrganizationInvitation
	err := dbFromContext(ctx, r.db).
		Preload("Organization").
		Preload("InvitedBy").
		First(&inv, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, repository.ErrNotFound
	}
	return &inv, err
}

// GetPendingByOrgAndEmail returns the pending row (possibly past its expiry).
func (r *organizationInvitationRepository) GetPendingByOrgAndEmail(ctx context.Context, orgID uint, email string) (*domain.OrganizationInvitation, error) {
	var inv domain.OrganizationInvitation
	err := dbFromContext(ctx, r.db).
		Where("organization_id = ? AND LOWER(email) = ? AND status = ?", orgID, domain.NormalizeInvitationEmail(email), domain.OrgInvitationPending).
		First(&inv).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, repository.ErrNotFound
	}
	return &inv, err
}

func (r *organizationInvitationRepository) ListByOrganization(ctx context.Context, orgID uint, statuses []domain.OrganizationInvitationStatus) ([]*domain.OrganizationInvitation, error) {
	query := dbFromContext(ctx, r.db).
		Preload("InvitedBy").
		Where("organization_id = ?", orgID)
	if len(statuses) > 0 {
		query = query.Where("status IN ?", statuses)
	}
	var invs []*domain.OrganizationInvitation
	err := query.Order("created_at DESC").Limit(500).Find(&invs).Error
	return invs, err
}

// ListPendingByEmail returns unexpired pending invitations addressed to email.
func (r *organizationInvitationRepository) ListPendingByEmail(ctx context.Context, email string, now time.Time) ([]*domain.OrganizationInvitation, error) {
	var invs []*domain.OrganizationInvitation
	err := dbFromContext(ctx, r.db).
		Preload("Organization").
		Preload("InvitedBy").
		Where("LOWER(email) = ? AND status = ? AND expires_at > ?", domain.NormalizeInvitationEmail(email), domain.OrgInvitationPending, now).
		Order("created_at DESC").
		Find(&invs).Error
	return invs, err
}

// CountPendingByOrganization counts unexpired pending invitations (they hold a seat).
func (r *organizationInvitationRepository) CountPendingByOrganization(ctx context.Context, orgID uint, now time.Time) (int, error) {
	var count int64
	err := dbFromContext(ctx, r.db).
		Model(&domain.OrganizationInvitation{}).
		Where("organization_id = ? AND status = ? AND expires_at > ?", orgID, domain.OrgInvitationPending, now).
		Count(&count).Error
	return int(count), err
}

// ExpireOverdue marks pending invitations past their expiry as expired.
func (r *organizationInvitationRepository) ExpireOverdue(ctx context.Context, now time.Time) (int64, error) {
	result := dbFromContext(ctx, r.db).
		Model(&domain.OrganizationInvitation{}).
		Where("status = ? AND expires_at <= ?", domain.OrgInvitationPending, now).
		Updates(map[string]interface{}{"status": domain.OrgInvitationExpired, "updated_at": now})
	return result.RowsAffected, result.Error
}

// LockOrganization serializes invitation and membership changes per organization.
func (r *organizationInvitationRepository) LockOrganization(ctx context.Context, orgID uint) error {
	var org domain.Organization
	err := dbFromContext(ctx, r.db).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id").
		First(&org, orgID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return repository.ErrNotFound
	}
	return err
}
