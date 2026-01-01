package gormrepo

import (
	"context"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"gorm.io/gorm"
)

type verificationRepository struct {
	db *gorm.DB
}

// NewVerificationRepository creates a new verification repository
func NewVerificationRepository(db *gorm.DB) repository.VerificationRepository {
	return &verificationRepository{db: db}
}

// Create creates a new verification code
func (r *verificationRepository) Create(ctx context.Context, code *domain.VerificationCode) error {
	// Delete any existing codes for this email first
	if err := r.DeleteByEmail(ctx, code.Email); err != nil {
		// Log but don't fail - it's OK if there are no codes to delete
	}

	return r.db.WithContext(ctx).Create(code).Error
}

// GetByEmailAndCode gets a verification code by email and code
func (r *verificationRepository) GetByEmailAndCode(ctx context.Context, email, code string) (*domain.VerificationCode, error) {
	var verificationCode domain.VerificationCode
	err := r.db.WithContext(ctx).
		Where("email = ? AND code = ?", email, code).
		First(&verificationCode).Error

	if err == gorm.ErrRecordNotFound {
		return nil, repository.ErrNotFound
	}

	return &verificationCode, err
}

// DeleteByEmail deletes all verification codes for an email
func (r *verificationRepository) DeleteByEmail(ctx context.Context, email string) error {
	return r.db.WithContext(ctx).
		Where("email = ?", email).
		Delete(&domain.VerificationCode{}).Error
}

// DeleteExpired deletes expired verification codes
func (r *verificationRepository) DeleteExpired(ctx context.Context) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("expires_at < ?", time.Now()).
		Delete(&domain.VerificationCode{})

	return result.RowsAffected, result.Error
}

// Migrate runs database migrations for verification codes
func (r *verificationRepository) Migrate() error {
	return r.db.AutoMigrate(&domain.VerificationCode{})
}
