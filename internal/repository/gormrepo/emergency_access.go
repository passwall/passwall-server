package gormrepo

import (
	"context"
	"errors"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	uuid "github.com/satori/go.uuid"
	"gorm.io/gorm"
)

type emergencyAccessRepository struct {
	db *gorm.DB
}

func NewEmergencyAccessRepository(db *gorm.DB) repository.EmergencyAccessRepository {
	return &emergencyAccessRepository{db: db}
}

func (r *emergencyAccessRepository) Create(ctx context.Context, ea *domain.EmergencyAccess) error {
	if ea.UUID == uuid.Nil {
		ea.UUID = uuid.NewV4()
	}
	return r.db.WithContext(ctx).Create(ea).Error
}

func (r *emergencyAccessRepository) GetByUUID(ctx context.Context, uuidStr string) (*domain.EmergencyAccess, error) {
	var ea domain.EmergencyAccess
	err := r.db.WithContext(ctx).
		Preload("Grantor").
		Preload("Grantee").
		Where("uuid = ?", uuidStr).
		First(&ea).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &ea, nil
}

func (r *emergencyAccessRepository) ListByGrantor(ctx context.Context, grantorID uint) ([]*domain.EmergencyAccess, error) {
	var list []*domain.EmergencyAccess
	err := r.db.WithContext(ctx).
		Preload("Grantee").
		Where("grantor_id = ? AND status != ?", grantorID, domain.EAStatusRevoked).
		Order("created_at DESC").
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (r *emergencyAccessRepository) ListByGrantee(ctx context.Context, granteeID uint) ([]*domain.EmergencyAccess, error) {
	var list []*domain.EmergencyAccess
	err := r.db.WithContext(ctx).
		Preload("Grantor").
		Where("grantee_id = ? AND status != ?", granteeID, domain.EAStatusRevoked).
		Order("created_at DESC").
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (r *emergencyAccessRepository) ListByGranteeEmail(ctx context.Context, email string) ([]*domain.EmergencyAccess, error) {
	var list []*domain.EmergencyAccess
	err := r.db.WithContext(ctx).
		Preload("Grantor").
		Where("grantee_email = ? AND status = ?", email, domain.EAStatusInvited).
		Order("created_at DESC").
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (r *emergencyAccessRepository) ListConfirmedByGrantor(ctx context.Context, grantorID uint) ([]*domain.EmergencyAccess, error) {
	var list []*domain.EmergencyAccess
	err := r.db.WithContext(ctx).
		Where("grantor_id = ? AND status IN ?", grantorID, []domain.EmergencyAccessStatus{
			domain.EAStatusConfirmed,
			domain.EAStatusRecoveryApproved,
		}).
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (r *emergencyAccessRepository) Update(ctx context.Context, ea *domain.EmergencyAccess) error {
	ea.Grantor = nil
	ea.Grantee = nil
	return r.db.WithContext(ctx).Save(ea).Error
}

func (r *emergencyAccessRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&domain.EmergencyAccess{}, id).Error
}
