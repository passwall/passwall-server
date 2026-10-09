package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Admin step-up: a system admin re-proves possession of the master password
// and a second factor before a destructive platform action. The result is a
// short-lived token bound to the user and the current session.

const (
	adminStepUpSubject  = "admin-step-up"
	adminStepUpDuration = 5 * time.Minute
)

var (
	// ErrStepUpInvalid means the step-up token is missing, expired or belongs
	// to another user or session.
	ErrStepUpInvalid = errors.New("admin step-up required")
	// ErrStepUpFailed means the master password or second factor was wrong.
	ErrStepUpFailed = errors.New("admin step-up verification failed")
)

type adminStepUpClaims struct {
	UserID      uint      `json:"user_id"`
	SessionUUID uuid.UUID `json:"sid"`
	jwt.RegisteredClaims
}

// IssueAdminStepUp verifies the master password hash and a TOTP or recovery
// code, then returns a step-up token valid for adminStepUpDuration.
// Users without 2FA get ErrTwoFactorRequired.
func (s *authService) IssueAdminStepUp(ctx context.Context, userID uint, sessionUUID uuid.UUID, masterPasswordHash, code string) (string, time.Time, error) {
	if sessionUUID == uuid.Nil {
		return "", time.Time{}, ErrStepUpInvalid
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("user not found: %w", err)
	}
	if !user.TwoFactorEnabled || user.TwoFactorSecret == nil {
		return "", time.Time{}, ErrTwoFactorRequired
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.MasterPasswordHash), []byte(masterPasswordHash)); err != nil {
		return "", time.Time{}, ErrStepUpFailed
	}
	if !s.verifyTOTPOrRecovery(user, code) {
		return "", time.Time{}, ErrStepUpFailed
	}

	now := time.Now()
	expiresAt := now.Add(adminStepUpDuration)
	claims := adminStepUpClaims{
		UserID:      user.ID,
		SessionUUID: sessionUUID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.NewString(),
			Subject:   adminStepUpSubject,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.config.JWTSecret))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign step-up token: %w", err)
	}
	return token, expiresAt, nil
}

// VerifyAdminStepUp checks that the token was issued to this user in this
// session and has not expired.
func (s *authService) VerifyAdminStepUp(tokenString string, userID uint, sessionUUID uuid.UUID) error {
	if tokenString == "" || sessionUUID == uuid.Nil {
		return ErrStepUpInvalid
	}
	token, err := jwt.ParseWithClaims(tokenString, &adminStepUpClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.config.JWTSecret), nil
	})
	if err != nil {
		return ErrStepUpInvalid
	}
	claims, ok := token.Claims.(*adminStepUpClaims)
	if !ok || !token.Valid || claims.Subject != adminStepUpSubject ||
		claims.UserID != userID || claims.SessionUUID != sessionUUID {
		return ErrStepUpInvalid
	}
	return nil
}
