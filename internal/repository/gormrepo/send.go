package gormrepo

import (
	"context"
	"errors"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	uuid "github.com/satori/go.uuid"
	"gorm.io/gorm"
)

type sendRepository struct {
	db *gorm.DB
}

func NewSendRepository(db *gorm.DB) repository.SendRepository {
	return &sendRepository{db: db}
}

func (r *sendRepository) Create(ctx context.Context, send *domain.Send) error {
	if send.UUID == uuid.Nil {
		send.UUID = uuid.NewV4()
	}
	return r.db.WithContext(ctx).Create(send).Error
}

func (r *sendRepository) GetByUUID(ctx context.Context, uuidStr string) (*domain.Send, error) {
	var send domain.Send
	err := r.db.WithContext(ctx).
		Preload("Creator").
		Where("uuid = ? AND deleted_at IS NULL", uuidStr).
		First(&send).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &send, nil
}

func (r *sendRepository) GetByAccessID(ctx context.Context, accessID string) (*domain.Send, error) {
	var send domain.Send
	err := r.db.WithContext(ctx).
		Preload("Creator").
		Where("access_id = ? AND deleted_at IS NULL", accessID).
		First(&send).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &send, nil
}

func (r *sendRepository) ListByCreator(ctx context.Context, creatorID uint) ([]*domain.Send, error) {
	var sends []*domain.Send
	err := r.db.WithContext(ctx).
		Where("creator_id = ? AND deleted_at IS NULL", creatorID).
		Order("created_at DESC").
		Find(&sends).Error
	if err != nil {
		return nil, err
	}
	return sends, nil
}

func (r *sendRepository) Update(ctx context.Context, send *domain.Send) error {
	send.Creator = nil
	return r.db.WithContext(ctx).Save(send).Error
}

func (r *sendRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&domain.Send{}, id).Error
}

func (r *sendRepository) SoftDelete(ctx context.Context, id uint) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&domain.Send{}).
		Where("id = ?", id).
		Update("deleted_at", now).Error
}

func (r *sendRepository) IncrementAccessCount(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).
		Model(&domain.Send{}).
		Where("id = ?", id).
		UpdateColumn("access_count", gorm.Expr("access_count + 1")).Error
}

func (r *sendRepository) DeleteExpired(ctx context.Context) (int64, error) {
	now := time.Now()
	result := r.db.WithContext(ctx).
		Where("deletion_date <= ? AND deleted_at IS NULL", now).
		Delete(&domain.Send{})
	return result.RowsAffected, result.Error
}
