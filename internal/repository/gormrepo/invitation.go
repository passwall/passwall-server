package gormrepo

import (
	"context"
	"errors"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"gorm.io/gorm"
)

type invitationRepository struct {
	db *gorm.DB
}

// NewInvitationRepository creates the referral invitation repository.
func NewInvitationRepository(db *gorm.DB) repository.InvitationRepository {
	return &invitationRepository{db: db}
}

func (r *invitationRepository) referrals(ctx context.Context) *gorm.DB {
	return dbFromContext(ctx, r.db).Model(&domain.Invitation{}).Where("organization_id IS NULL")
}

func (r *invitationRepository) Create(ctx context.Context, invitation *domain.Invitation) error {
	return dbFromContext(ctx, r.db).Create(invitation).Error
}

func (r *invitationRepository) GetActiveByEmail(ctx context.Context, email string) (*domain.Invitation, error) {
	var invitation domain.Invitation
	err := r.referrals(ctx).
		Where("LOWER(email) = LOWER(?) AND used_at IS NULL AND expires_at > ?", email, time.Now()).
		First(&invitation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, repository.ErrNotFound
	}
	return &invitation, err
}

func (r *invitationRepository) ListByCreator(ctx context.Context, createdBy uint) ([]*domain.Invitation, error) {
	var invitations []*domain.Invitation
	err := r.referrals(ctx).
		Where("created_by = ?", createdBy).
		Order("created_at DESC").
		Limit(200).
		Find(&invitations).Error
	return invitations, err
}

func (r *invitationRepository) CountByCreatorSince(ctx context.Context, createdBy uint, since time.Time) (int, error) {
	var count int64
	err := r.referrals(ctx).Where("created_by = ? AND created_at >= ?", createdBy, since).Count(&count).Error
	return int(count), err
}

func (r *invitationRepository) MarkUsedByEmail(ctx context.Context, email string, usedAt time.Time) error {
	return r.referrals(ctx).
		Where("LOWER(email) = LOWER(?) AND used_at IS NULL", email).
		Update("used_at", usedAt).Error
}

func (r *invitationRepository) DeleteByEmail(ctx context.Context, email string) error {
	return dbFromContext(ctx, r.db).Where("LOWER(email) = LOWER(?)", email).Delete(&domain.Invitation{}).Error
}
