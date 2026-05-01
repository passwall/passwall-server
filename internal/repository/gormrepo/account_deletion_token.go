package gormrepo

import (
	"context"
	"errors"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"gorm.io/gorm"
)

type accountDeletionTokenRepository struct {
	db *gorm.DB
}

// NewAccountDeletionTokenRepository creates a new account deletion token repository.
func NewAccountDeletionTokenRepository(db *gorm.DB) repository.AccountDeletionTokenRepository {
	return &accountDeletionTokenRepository{db: db}
}

func (r *accountDeletionTokenRepository) Create(ctx context.Context, token *domain.AccountDeletionToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

func (r *accountDeletionTokenRepository) GetByUUID(ctx context.Context, tokenUUID string) (*domain.AccountDeletionToken, error) {
	var token domain.AccountDeletionToken
	err := r.db.WithContext(ctx).Where("uuid = ?", tokenUUID).First(&token).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &token, nil
}

func (r *accountDeletionTokenRepository) DeleteByUUID(ctx context.Context, tokenUUID string) error {
	return r.db.WithContext(ctx).Where("uuid = ?", tokenUUID).Delete(&domain.AccountDeletionToken{}).Error
}

func (r *accountDeletionTokenRepository) DeleteByUserID(ctx context.Context, userID uint) error {
	return r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&domain.AccountDeletionToken{}).Error
}

func (r *accountDeletionTokenRepository) DeleteExpired(ctx context.Context) (int64, error) {
	result := r.db.WithContext(ctx).Where("expires_at < ?", time.Now()).Delete(&domain.AccountDeletionToken{})
	return result.RowsAffected, result.Error
}

func (r *accountDeletionTokenRepository) Migrate() error {
	return r.db.AutoMigrate(&domain.AccountDeletionToken{})
}
