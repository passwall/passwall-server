package gormrepo

import (
	"context"
	"errors"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/pkg/hash"
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

func (r *tokenRepository) Create(ctx context.Context, userID int, sessionUUID uuid.UUID, deviceID uuid.UUID, app string, kind string, tokenUUID uuid.UUID, token string, expiryTime time.Time) error {
	// SECURITY: Hash the token before storing in database
	// This prevents token theft if database is compromised
	hashedToken := hash.SHA256(token)

	t := &domain.Token{
		UserID:      userID,
		UUID:        tokenUUID,
		SessionUUID: sessionUUID,
		DeviceID:    deviceID,
		App:         app,
		Kind:        kind,
		Token:       hashedToken,
		ExpiryTime:  expiryTime,
	}
	return r.db.WithContext(ctx).Create(t).Error
}

func (r *tokenRepository) CountActiveSessionsByUserID(ctx context.Context, userID int) (int, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&domain.Token{}).
		Where("user_id = ? AND expiry_time > ? AND kind IN ?", userID, time.Now(), []string{"access", ""}).
		Distinct("session_uuid").
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func (r *tokenRepository) Delete(ctx context.Context, userID int) error {
	return r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&domain.Token{}).Error
}

func (r *tokenRepository) DeleteByUUID(ctx context.Context, uuid string) error {
	return r.db.WithContext(ctx).Where("uuid = ?", uuid).Delete(&domain.Token{}).Error
}

func (r *tokenRepository) DeleteBySessionUUID(ctx context.Context, sessionUUID string) error {
	return r.db.WithContext(ctx).Where("session_uuid = ?", sessionUUID).Delete(&domain.Token{}).Error
}

func (r *tokenRepository) DeleteExpired(ctx context.Context) (int64, error) {
	result := r.db.WithContext(ctx).Where("expiry_time < ?", time.Now()).Delete(&domain.Token{})
	return result.RowsAffected, result.Error
}

func (r *tokenRepository) Cleanup(ctx context.Context) error {
	_, err := r.DeleteExpired(ctx)
	return err
}

func (r *tokenRepository) Migrate() error {
	return r.db.AutoMigrate(&domain.Token{})
}
