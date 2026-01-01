package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
)

const (
	verificationCodeLength = 6
	verificationCodeExpiry = 15 * time.Minute
)

var (
	ErrCodeExpired = errors.New("verification code has expired")
	ErrCodeInvalid = errors.New("verification code is invalid")
)

type verificationService struct {
	repo     repository.VerificationRepository
	userRepo repository.UserRepository
	logger   Logger
}

// NewVerificationService creates a new verification service
func NewVerificationService(
	repo repository.VerificationRepository,
	userRepo repository.UserRepository,
	logger Logger,
) VerificationService {
	return &verificationService{
		repo:     repo,
		userRepo: userRepo,
		logger:   logger,
	}
}

// GenerateCode generates a new verification code for an email
func (s *verificationService) GenerateCode(ctx context.Context, email string) (string, error) {
	// Generate random alphanumeric code
	code, err := generateRandomCode(verificationCodeLength)
	if err != nil {
		s.logger.Error("failed to generate verification code", "error", err)
		return "", fmt.Errorf("failed to generate code: %w", err)
	}

	// Create verification code record
	verificationCode := &domain.VerificationCode{
		Email:     strings.ToLower(email),
		Code:      code,
		ExpiresAt: time.Now().Add(verificationCodeExpiry),
	}

	if err := s.repo.Create(ctx, verificationCode); err != nil {
		s.logger.Error("failed to save verification code", "email", email, "error", err)
		return "", fmt.Errorf("failed to save verification code: %w", err)
	}

	s.logger.Info("verification code generated", "email", email)
	return code, nil
}

// VerifyCode verifies a code and marks the user as verified
func (s *verificationService) VerifyCode(ctx context.Context, email, code string) error {
	email = strings.ToLower(email)

	// Get verification code from database
	verificationCode, err := s.repo.GetByEmailAndCode(ctx, email, code)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			s.logger.Warn("invalid verification code", "email", email, "code", code)
			return ErrCodeInvalid
		}
		s.logger.Error("failed to get verification code", "email", email, "error", err)
		return fmt.Errorf("failed to verify code: %w", err)
	}

	// Check if code is expired
	if verificationCode.IsExpired() {
		s.logger.Warn("verification code expired", "email", email, "expired_at", verificationCode.ExpiresAt)
		// Clean up expired code
		_ = s.repo.DeleteByEmail(ctx, email)
		return ErrCodeExpired
	}

	// Get user by email
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		s.logger.Error("failed to get user for verification", "email", email, "error", err)
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Check if already verified
	if user.IsVerified {
		s.logger.Info("user already verified", "email", email)
		// Clean up verification code
		_ = s.repo.DeleteByEmail(ctx, email)
		return nil
	}

	// Mark user as verified
	user.IsVerified = true

	if err := s.userRepo.Update(ctx, user); err != nil {
		s.logger.Error("failed to update user verification status", "email", email, "error", err)
		return fmt.Errorf("failed to update user: %w", err)
	}

	// Delete verification code after successful verification
	if err := s.repo.DeleteByEmail(ctx, email); err != nil {
		s.logger.Warn("failed to delete verification code", "email", email, "error", err)
		// Not a critical error, continue
	}

	s.logger.Info("email verified successfully", "email", email, "user_id", user.ID)
	return nil
}

// ResendCode generates and returns a new verification code
func (s *verificationService) ResendCode(ctx context.Context, email string) (string, error) {
	email = strings.ToLower(email)

	// Check if user exists
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			s.logger.Warn("resend code requested for non-existent user", "email", email)
			return "", repository.ErrNotFound
		}
		s.logger.Error("failed to get user for resend", "email", email, "error", err)
		return "", fmt.Errorf("failed to get user: %w", err)
	}

	// Check if already verified
	if user.IsVerified {
		s.logger.Info("resend code requested for already verified user", "email", email)
		return "", errors.New("email already verified")
	}

	// Generate new code
	code, err := s.GenerateCode(ctx, email)
	if err != nil {
		return "", err
	}

	s.logger.Info("verification code resent", "email", email)
	return code, nil
}

// CleanupExpiredCodes removes expired verification codes from database
func (s *verificationService) CleanupExpiredCodes(ctx context.Context) error {
	count, err := s.repo.DeleteExpired(ctx)
	if err != nil {
		s.logger.Error("failed to cleanup expired codes", "error", err)
		return err
	}

	if count > 0 {
		s.logger.Info("cleaned up expired verification codes", "count", count)
	}

	return nil
}

// generateRandomCode generates a random alphanumeric code of specified length
func generateRandomCode(length int) (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	code := make([]byte, length)

	for i := range code {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		code[i] = charset[num.Int64()]
	}

	return string(code), nil
}
