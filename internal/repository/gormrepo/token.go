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

type tokenRepository struct {
	db *gorm.DB
}

// NewTokenRepository creates a new token repository
func NewTokenRepository(db *gorm.DB) repository.TokenRepository {
	return &tokenRepository{db: db}
}

func (r *tokenRepository) GetByUUID(ctx context.Context, uuid string) (*domain.Token, error) {
	var token domain.Token
	err := r.db.WithContext(ctx).Where("uuid = ?", uuid).First(&token).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &token, nil
}

func (r *tokenRepository) Create(ctx context.Context, userID int, tokenUUID uuid.UUID, token string, expiryTime time.Time) error {
	t := &domain.Token{
		UserID:     userID,
		UUID:       tokenUUID,
		Token:      token,
		ExpiryTime: expiryTime,
	}
	return r.db.WithContext(ctx).Create(t).Error
}

func (r *tokenRepository) Delete(ctx context.Context, userID int) error {
	return r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&domain.Token{}).Error
}

func (r *tokenRepository) DeleteByUUID(ctx context.Context, uuid string) error {
	return r.db.WithContext(ctx).Where("uuid = ?", uuid).Delete(&domain.Token{}).Error
}

func (r *tokenRepository) Migrate() error {
	return r.db.AutoMigrate(&domain.Token{})
}

