package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrTwoFactorRequired     = errors.New("two-factor authentication required")
	ErrInvalidTOTP           = errors.New("invalid TOTP code")
	ErrTwoFactorNotSetup     = errors.New("two-factor authentication not set up")
	ErrTwoFactorAlreadySetup = errors.New("two-factor authentication already enabled")
	ErrInvalid2FAToken       = errors.New("invalid or expired two-factor token")
)

const (
	twoFactorTokenDuration = 5 * time.Minute
	twoFactorIssuer        = "Passwall"
	recoveryCodeCount      = 8
	recoveryCodeLength     = 10
)

// SetupTwoFactor generates a TOTP secret and recovery codes for the user.
// The user must confirm with a valid TOTP code before 2FA is active.
func (s *authService) SetupTwoFactor(ctx context.Context, userID uint) (*domain.TwoFactorSetupResponse, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	if user.TwoFactorEnabled {
		return nil, ErrTwoFactorAlreadySetup
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      twoFactorIssuer,
		AccountName: user.Email,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate TOTP key: %w", err)
	}

	recoveryCodes, err := generateRecoveryCodes(recoveryCodeCount, recoveryCodeLength)
	if err != nil {
		return nil, fmt.Errorf("failed to generate recovery codes: %w", err)
	}

	// Store secret and hashed recovery codes (not yet enabled)
	secret := key.Secret()
	hashedCodes, err := hashRecoveryCodes(recoveryCodes)
	if err != nil {
		return nil, fmt.Errorf("failed to hash recovery codes: %w", err)
	}
	codesJSON, _ := json.Marshal(hashedCodes)
	codesStr := string(codesJSON)

	user.TwoFactorSecret = &secret
	user.TwoFactorRecoveryCodes = &codesStr

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to save 2FA setup: %w", err)
	}

	return &domain.TwoFactorSetupResponse{
		Secret:        secret,
		QRCodeURL:     key.URL(),
		RecoveryCodes: recoveryCodes,
	}, nil
}

// ConfirmTwoFactor validates a TOTP code and enables 2FA on the account.
func (s *authService) ConfirmTwoFactor(ctx context.Context, userID uint, code string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	if user.TwoFactorEnabled {
		return ErrTwoFactorAlreadySetup
	}

	if user.TwoFactorSecret == nil || *user.TwoFactorSecret == "" {
		return ErrTwoFactorNotSetup
	}

	if !totp.Validate(code, *user.TwoFactorSecret) {
		return ErrInvalidTOTP
	}

	user.TwoFactorEnabled = true
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to enable 2FA: %w", err)
	}

	s.logger.Info("two-factor authentication enabled", "user_id", userID)
	return nil
}

// DisableTwoFactor verifies master password + TOTP, then disables 2FA.
func (s *authService) DisableTwoFactor(ctx context.Context, userID uint, masterPasswordHash string, totpCode string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	if !user.TwoFactorEnabled {
		return ErrTwoFactorNotSetup
	}

	// Verify master password
	if err := bcrypt.CompareHashAndPassword(
		[]byte(user.MasterPasswordHash),
		[]byte(masterPasswordHash),
	); err != nil {
		return ErrUnauthorized
	}

	// Verify TOTP code or recovery code
	if !s.verifyTOTPOrRecovery(user, totpCode) {
		return ErrInvalidTOTP
	}

	user.TwoFactorEnabled = false
	user.TwoFactorSecret = nil
	user.TwoFactorRecoveryCodes = nil

	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to disable 2FA: %w", err)
	}

	s.logger.Info("two-factor authentication disabled", "user_id", userID)
	return nil
}

// GetTwoFactorStatus returns the 2FA status for the user.
func (s *authService) GetTwoFactorStatus(ctx context.Context, userID uint) (*domain.TwoFactorStatusResponse, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	return &domain.TwoFactorStatusResponse{Enabled: user.TwoFactorEnabled}, nil
}

// VerifyTwoFactorSignIn validates a 2FA token + TOTP code and returns a full AuthResponse.
func (s *authService) VerifyTwoFactorSignIn(ctx context.Context, twoFactorToken string, totpCode string) (*domain.AuthResponse, error) {
	claims, err := s.parseTwoFactorToken(twoFactorToken)
	if err != nil {
		return nil, ErrInvalid2FAToken
	}

	userID := claims.UserID
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	if !user.TwoFactorEnabled || user.TwoFactorSecret == nil {
		return nil, ErrTwoFactorNotSetup
	}

	// Verify TOTP code or recovery code
	if !s.verifyTOTPOrRecovery(user, totpCode) {
		return nil, ErrInvalidTOTP
	}

	// 2FA passed — issue full tokens (reuse session UUID from the 2FA token)
	sessionUUID := claims.SessionUUID
	deviceUUID := claims.DeviceUUID

	// Replace tokens for this client session first (same behavior as standard signin).
	if deviceUUID != uuid.Nil {
		_ = s.tokenRepo.DeleteBySessionUUID(ctx, sessionUUID.String())
	}

	// Enforce device limit after successful 2FA as well.
	if err := s.enforceDeviceLimit(ctx, user); err != nil {
		if errors.Is(err, ErrDeviceLimit) && claims.LogoutOther {
			if revokeErr := s.tokenRepo.Delete(ctx, int(user.ID)); revokeErr != nil {
				return nil, fmt.Errorf("failed to revoke active sessions: %w", revokeErr)
			}
		} else {
			return nil, err
		}
	}

	tokenDetails, err := s.createToken(user, sessionUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to create token: %w", err)
	}

	if err := s.tokenRepo.Create(ctx, int(user.ID), tokenDetails.SessionUUID, deviceUUID, claims.App, "access", tokenDetails.AtUUID, tokenDetails.AccessToken, tokenDetails.AtExpiresTime); err != nil {
		return nil, fmt.Errorf("failed to store access token: %w", err)
	}
	if err := s.tokenRepo.Create(ctx, int(user.ID), tokenDetails.SessionUUID, deviceUUID, claims.App, "refresh", tokenDetails.RtUUID, tokenDetails.RefreshToken, tokenDetails.RtExpiresTime); err != nil {
		return nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

	policyReqs := s.collectPolicyRequirements(ctx, user)
	twoFactorSetupReq := s.checkTwoFactorSetupRequired(ctx, user)

	return &domain.AuthResponse{
		AccessToken:           tokenDetails.AccessToken,
		RefreshToken:          tokenDetails.RefreshToken,
		Type:                  "Bearer",
		AccessTokenExpiresAt:  tokenDetails.AccessTokenExpiresAt,
		RefreshTokenExpiresAt: tokenDetails.RefreshTokenExpiresAt,
		ProtectedUserKey:      user.ProtectedUserKey,
		KdfConfig:             user.GetKdfConfig(),
		User: &domain.UserAuthDTO{
			ID:                     user.ID,
			UUID:                   user.UUID.String(),
			Email:                  user.Email,
			Name:                   user.Name,
			Schema:                 user.Schema,
			Role:                   user.GetRoleName(),
			IsVerified:             user.IsVerified,
			Language:               user.Language,
			TwoFactorEnabled:       user.TwoFactorEnabled,
			PersonalOrganizationID: user.PersonalOrganizationID,
			DefaultOrganizationID:  user.DefaultOrganizationID,
		},
		RequireTwoFactorSetup: twoFactorSetupReq,
		PolicyRequirements:    policyReqs,
	}, nil
}

// twoFactorClaims is used for the short-lived 2FA verification JWT.
type twoFactorClaims struct {
	UserID      uint      `json:"user_id"`
	SessionUUID uuid.UUID `json:"sid"`
	DeviceUUID  uuid.UUID `json:"did"`
	App         string    `json:"app"`
	LogoutOther bool      `json:"lod"`
	jwt.RegisteredClaims
}

// createTwoFactorToken creates a short-lived JWT that allows 2FA verification.
func (s *authService) createTwoFactorToken(user *domain.User, sessionUUID, deviceUUID uuid.UUID, app string, logoutOther bool) (string, error) {
	claims := twoFactorClaims{
		UserID:      user.ID,
		SessionUUID: sessionUUID,
		DeviceUUID:  deviceUUID,
		App:         app,
		LogoutOther: logoutOther,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(twoFactorTokenDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   "2fa",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.config.JWTSecret))
}

// parseTwoFactorToken validates a 2FA verification token.
func (s *authService) parseTwoFactorToken(tokenString string) (*twoFactorClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &twoFactorClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.config.JWTSecret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*twoFactorClaims)
	if !ok || !token.Valid || claims.Subject != "2fa" {
		return nil, ErrInvalid2FAToken
	}

	return claims, nil
}

// verifyTOTPOrRecovery checks a TOTP code or a recovery code.
func (s *authService) verifyTOTPOrRecovery(user *domain.User, code string) bool {
	if user.TwoFactorSecret != nil && totp.Validate(code, *user.TwoFactorSecret) {
		return true
	}
	// Try recovery code
	return s.tryRecoveryCode(user, code)
}

// tryRecoveryCode checks if the code is a valid (unused) recovery code and consumes it.
func (s *authService) tryRecoveryCode(user *domain.User, code string) bool {
	if user.TwoFactorRecoveryCodes == nil || *user.TwoFactorRecoveryCodes == "" {
		return false
	}

	var hashedCodes []string
	if err := json.Unmarshal([]byte(*user.TwoFactorRecoveryCodes), &hashedCodes); err != nil {
		return false
	}

	for i, hashed := range hashedCodes {
		if bcrypt.CompareHashAndPassword([]byte(hashed), []byte(code)) == nil {
			// Consume the recovery code
			hashedCodes = append(hashedCodes[:i], hashedCodes[i+1:]...)
			codesJSON, _ := json.Marshal(hashedCodes)
			codesStr := string(codesJSON)
			user.TwoFactorRecoveryCodes = &codesStr
			if err := s.userRepo.Update(context.Background(), user); err != nil {
				s.logger.Error("failed to consume recovery code", "user_id", user.ID, "error", err)
				return false
			}
			s.logger.Info("recovery code used", "user_id", user.ID)
			return true
		}
	}
	return false
}

func generateRecoveryCodes(count, length int) ([]string, error) {
	codes := make([]string, count)
	for i := range codes {
		b := make([]byte, length/2+1)
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
		codes[i] = hex.EncodeToString(b)[:length]
	}
	return codes, nil
}

func hashRecoveryCodes(codes []string) ([]string, error) {
	hashed := make([]string, len(codes))
	for i, code := range codes {
		h, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		hashed[i] = string(h)
	}
	return hashed, nil
}
