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

// NewInvitationRepository creates a new invitation repository
func NewInvitationRepository(db *gorm.DB) repository.InvitationRepository {
	return &invitationRepository{db: db}
}

func (r *invitationRepository) Create(ctx context.Context, invitation *domain.Invitation) error {
	return r.db.WithContext(ctx).Create(invitation).Error
}

func (r *invitationRepository) GetByEmail(ctx context.Context, email string) (*domain.Invitation, error) {
	var invitation domain.Invitation
	err := r.db.WithContext(ctx).
		Where("email = ? AND used_at IS NULL AND expires_at > ?", email, time.Now()).
		First(&invitation).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &invitation, nil
}

func (r *invitationRepository) GetByCode(ctx context.Context, code string) (*domain.Invitation, error) {
	var invitation domain.Invitation
	err := r.db.WithContext(ctx).
		Where("code = ? AND used_at IS NULL AND expires_at > ?", code, time.Now()).
		First(&invitation).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &invitation, nil
}

func (r *invitationRepository) GetByID(ctx context.Context, id uint) (*domain.Invitation, error) {
	var invitation domain.Invitation
	err := r.db.WithContext(ctx).First(&invitation, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &invitation, nil
}

func (r *invitationRepository) GetAllByEmail(ctx context.Context, email string) ([]*domain.Invitation, error) {
	var invitations []*domain.Invitation
	err := r.db.WithContext(ctx).
		Where("email = ? AND used_at IS NULL AND expires_at > ?", email, time.Now()).
		Order("created_at DESC").
		Find(&invitations).Error
	if err != nil {
		return nil, err
	}
	return invitations, nil
}

func (r *invitationRepository) GetByCreator(ctx context.Context, createdBy uint) ([]*domain.Invitation, error) {
	var invitations []*domain.Invitation
	err := r.db.WithContext(ctx).
		Where("created_by = ? AND expires_at > ?", createdBy, time.Now()).
		Order("created_at DESC").
		Find(&invitations).Error
	if err != nil {
		return nil, err
	}
	return invitations, nil
}

func (r *invitationRepository) Update(ctx context.Context, invitation *domain.Invitation) error {
	return r.db.WithContext(ctx).Save(invitation).Error
}

func (r *invitationRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&domain.Invitation{}, id).Error
}

func (r *invitationRepository) DeleteByEmail(ctx context.Context, email string) error {
	return r.db.WithContext(ctx).
		Where("email = ?", email).
		Delete(&domain.Invitation{}).Error
}

func (r *invitationRepository) DeleteExpired(ctx context.Context) error {
	return r.db.WithContext(ctx).
		Where("expires_at < ? OR used_at IS NOT NULL", time.Now()).
		Delete(&domain.Invitation{}).Error
}

func (r *invitationRepository) Migrate() error {
	return r.db.AutoMigrate(&domain.Invitation{})
}
