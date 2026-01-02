package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/email"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/pkg/constants"
	"github.com/passwall/passwall-server/pkg/database"
	uuid "github.com/satori/go.uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrExpiredToken    = errors.New("token expired or invalid")
	ErrUnauthorized    = errors.New("unauthorized")
	ErrInvalidPassword = errors.New("invalid password")
)

type AuthConfig struct {
	JWTSecret            string
	AccessTokenDuration  string
	RefreshTokenDuration string
}

type authService struct {
	userRepo         repository.UserRepository
	tokenRepo        repository.TokenRepository
	verificationRepo repository.VerificationRepository
	emailSender      email.Sender
	config           *AuthConfig
	logger           Logger
}

// NewAuthService creates a new authentication service
func NewAuthService(
	userRepo repository.UserRepository,
	tokenRepo repository.TokenRepository,
	verificationRepo repository.VerificationRepository,
	emailSender email.Sender,
	config *AuthConfig,
	logger Logger,
) AuthService {
	return &authService{
		userRepo:         userRepo,
		tokenRepo:        tokenRepo,
		verificationRepo: verificationRepo,
		emailSender:      emailSender,
		config:           config,
		logger:           logger,
	}
}

func (s *authService) SignUp(ctx context.Context, req *domain.SignUpRequest) (*domain.User, error) {
	// Check if user already exists
	existingUser, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err == nil && existingUser != nil {
		return nil, repository.ErrAlreadyExists
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.MasterPassword), bcrypt.DefaultCost)
	if err != nil {
		s.logger.Error("failed to hash password", "error", err)
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Generate secret and schema
	secret, err := generateSecureKey(32)
	if err != nil {
		s.logger.Error("failed to generate secret", "error", err)
		return nil, fmt.Errorf("failed to generate secret: %w", err)
	}

	schema := generateSchema(req.Email)

	// Create user (unverified by default)
	user := &domain.User{
		UUID:           uuid.NewV4(),
		Name:           req.Name,
		Email:          req.Email,
		MasterPassword: string(hashedPassword),
		Secret:         secret,
		Schema:         schema,
		RoleID:         constants.RoleIDMember, // Default: Member role
		IsVerified:     false,                  // Requires email verification
		IsMigrated:     true,                   // New users don't need migration
	}

	// Create schema
	if err := s.userRepo.CreateSchema(schema); err != nil {
		s.logger.Error("failed to create schema", "schema", schema, "error", err)
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	// Migrate all tables in user schema (logins, notes, emails, etc.)
	if err := s.userRepo.MigrateUserSchema(schema); err != nil {
		s.logger.Error("failed to migrate user schema tables", "schema", schema, "error", err)
		return nil, fmt.Errorf("failed to migrate user schema: %w", err)
	}

	// Save user
	if err := s.userRepo.Create(ctx, user); err != nil {
		s.logger.Error("failed to create user", "email", req.Email, "error", err)
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Generate verification code
	code, err := generateRandomVerificationCode(6)
	if err != nil {
		s.logger.Error("failed to generate verification code", "error", err)
		// Don't fail signup, but log the error
		return user, nil
	}

	// Save verification code
	verificationCode := &domain.VerificationCode{
		Email:     req.Email,
		Code:      code,
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}

	if err := s.verificationRepo.Create(ctx, verificationCode); err != nil {
		s.logger.Error("failed to save verification code", "email", req.Email, "error", err)
		// Don't fail signup, user can request resend
		return user, nil
	}

	// Send verification email (async to not block signup)
	go func() {
		emailCtx := context.Background()
		if err := s.emailSender.SendVerificationEmail(emailCtx, req.Email, req.Name, code); err != nil {
			s.logger.Error("failed to send verification email", "email", req.Email, "error", err)
		}
	}()

	s.logger.Info("user signup successful", "email", req.Email, "user_id", user.ID)
	return user, nil
}

// generateRandomVerificationCode generates a random alphanumeric code
func generateRandomVerificationCode(length int) (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	code := make([]byte, length)
	
	for i := range code {
		b := make([]byte, 1)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		code[i] = charset[int(b[0])%len(charset)]
	}
	
	return string(code), nil
}

func (s *authService) SignIn(ctx context.Context, creds *domain.Credentials) (*domain.AuthResponse, error) {
	// Find user by credentials
	user, err := s.userRepo.GetByCredentials(ctx, creds.Email, creds.MasterPassword)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) || errors.Is(err, repository.ErrUnauthorized) {
			return nil, ErrUnauthorized
		}
		s.logger.Error("authentication failed", "email", creds.Email, "error", err)
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	// Check if email is verified
	if !user.IsVerified {
		s.logger.Warn("signin attempt with unverified email", "email", creds.Email)
		return nil, errors.New("email not verified")
	}

	// Delete old tokens before creating new ones
	if err := s.tokenRepo.Delete(ctx, int(user.ID)); err != nil {
		// Log error but continue - don't block login if cleanup fails
	}

	// Create tokens
	tokenDetails, err := s.createToken(user)
	if err != nil {
		return nil, fmt.Errorf("failed to create token: %w", err)
	}

	// Store tokens in database
	if err := s.tokenRepo.Create(ctx, int(user.ID), tokenDetails.AtUUID, tokenDetails.AccessToken, tokenDetails.AtExpiresTime); err != nil {
		return nil, fmt.Errorf("failed to store access token: %w", err)
	}

	if err := s.tokenRepo.Create(ctx, int(user.ID), tokenDetails.RtUUID, tokenDetails.RefreshToken, tokenDetails.RtExpiresTime); err != nil {
		return nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

	// Update last sign in timestamp
	now := time.Now()
	user.LastSignInAt = &now
	if err := s.userRepo.Update(ctx, user); err != nil {
		// Log error but don't fail login if timestamp update fails
	}

	return &domain.AuthResponse{
		AccessToken:  tokenDetails.AccessToken,
		RefreshToken: tokenDetails.RefreshToken,
		Type:         "Bearer", // Token type - backward compatible
		UserID:       user.ID,
		Email:        user.Email,
		Name:         user.Name,
		Schema:       user.Schema,
		Role:         user.GetRoleName(), // User role - backward compatible (mobile app uses this)
		Secret:       user.Secret,        // Required by extension for PBKDF2 encryption
		IsMigrated:   user.IsMigrated,    // Migration status for extension
		DateOfBirth:  user.DateOfBirth,
		Language:     user.Language,
	}, nil
}

func (s *authService) RefreshToken(ctx context.Context, refreshToken string) (*domain.TokenDetails, error) {
	// Verify refresh token
	token, err := s.verifyToken(refreshToken)
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, ErrUnauthorized
	}

	// Get user UUID from claims
	userUUIDStr, ok := claims["user_uuid"].(string)
	if !ok {
		return nil, ErrUnauthorized
	}

	// Get user
	user, err := s.userRepo.GetByUUID(ctx, userUUIDStr)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// Delete old tokens
	if err := s.tokenRepo.Delete(ctx, int(user.ID)); err != nil {
		// Log error but continue
	}

	// Create new tokens
	tokenDetails, err := s.createToken(user)
	if err != nil {
		return nil, fmt.Errorf("failed to create token: %w", err)
	}

	// Store new tokens
	if err := s.tokenRepo.Create(ctx, int(user.ID), tokenDetails.AtUUID, tokenDetails.AccessToken, tokenDetails.AtExpiresTime); err != nil {
		return nil, fmt.Errorf("failed to store access token: %w", err)
	}

	if err := s.tokenRepo.Create(ctx, int(user.ID), tokenDetails.RtUUID, tokenDetails.RefreshToken, tokenDetails.RtExpiresTime); err != nil {
		return nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

	return tokenDetails, nil
}

func (s *authService) ValidateToken(ctx context.Context, tokenString string) (*domain.TokenClaims, error) {
	// Verify JWT signature and expiration
	token, err := s.verifyToken(tokenString)
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, ErrUnauthorized
	}

	userUUID, _ := claims["user_uuid"].(string)
	tokenUUID, _ := claims["uuid"].(string)
	exp, _ := claims["exp"].(float64)

	// SECURITY: Check if token exists in database (revocation check)
	dbToken, err := s.tokenRepo.GetByUUID(ctx, tokenUUID)
	if err != nil {
		// Token not found in DB = revoked/logged out
		return nil, ErrExpiredToken
	}

	// SECURITY: Double-check token expiration from database
	if dbToken.IsExpired() {
		// Clean up expired token
		_ = s.tokenRepo.DeleteByUUID(ctx, tokenUUID)
		return nil, ErrExpiredToken
	}

	// Get user to get user ID, email, schema, and role
	user, err := s.userRepo.GetByUUID(ctx, userUUID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	return &domain.TokenClaims{
		UserID: user.ID,
		Email:  user.Email,
		Schema: user.Schema,
		Role:   user.GetRoleName(),
		UUID:   uuid.FromStringOrNil(tokenUUID),
		Exp:    int64(exp),
	}, nil
}

func (s *authService) SignOut(ctx context.Context, userID int) error {
	return s.tokenRepo.Delete(ctx, userID)
}

func (s *authService) createToken(user *domain.User) (*domain.TokenDetails, error) {
	accessTokenDuration := resolveTokenExpireDuration(s.config.AccessTokenDuration)
	refreshTokenDuration := resolveTokenExpireDuration(s.config.RefreshTokenDuration)

	td := &domain.TokenDetails{
		AtExpiresTime: time.Now().Add(accessTokenDuration),
		RtExpiresTime: time.Now().Add(refreshTokenDuration),
		AtUUID:        uuid.NewV4(),
		RtUUID:        uuid.NewV4(),
	}

	// Create access token
	atClaims := jwt.MapClaims{
		"authorized": user.IsAdmin(),
		"user_uuid":  user.UUID.String(),
		"role":       user.GetRoleName(),
		"exp":        td.AtExpiresTime.Unix(),
		"uuid":       td.AtUUID.String(),
	}

	at := jwt.NewWithClaims(jwt.SigningMethodHS256, atClaims)
	accessToken, err := at.SignedString([]byte(s.config.JWTSecret))
	if err != nil {
		return nil, err
	}
	td.AccessToken = accessToken

	// Create refresh token
	rtClaims := jwt.MapClaims{
		"user_uuid": user.UUID.String(),
		"exp":       td.RtExpiresTime.Unix(),
		"uuid":      td.RtUUID.String(),
	}

	rt := jwt.NewWithClaims(jwt.SigningMethodHS256, rtClaims)
	refreshToken, err := rt.SignedString([]byte(s.config.JWTSecret))
	if err != nil {
		return nil, err
	}
	td.RefreshToken = refreshToken

	return td, nil
}

func (s *authService) verifyToken(tokenString string) (*jwt.Token, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.config.JWTSecret), nil
	})
	if err != nil {
		return token, ErrExpiredToken
	}
	return token, nil
}

func resolveTokenExpireDuration(config string) time.Duration {
	duration, _ := strconv.ParseInt(config[0:len(config)-1], 10, 64)
	timeFormat := config[len(config)-1:]

	switch timeFormat {
	case "s":
		return time.Duration(time.Second.Nanoseconds() * duration)
	case "m":
		return time.Duration(time.Minute.Nanoseconds() * duration)
	case "h":
		return time.Duration(time.Hour.Nanoseconds() * duration)
	case "d":
		return time.Duration(time.Hour.Nanoseconds() * 24 * duration)
	default:
		return time.Duration(time.Minute.Nanoseconds() * 30)
	}
}

func generateSchema(email string) string {
	return "user_" + uuid.NewV5(uuid.NamespaceURL, email).String()[:8]
}

func generateSecureKey(length int) (string, error) {
	key := make([]byte, length)
	_, err := rand.Read(key)
	if err != nil {
		return "", err
	}
	return uuid.NewV4().String(), nil
}

// ValidateSchema validates that a schema exists in the database
// This is used when an admin tries to access another user's data
func (s *authService) ValidateSchema(ctx context.Context, schema string) error {
	// Strict schema format validation to prevent SQL injection
	if err := database.ValidateSchemaName(schema); err != nil {
		return fmt.Errorf("invalid schema format: %w", err)
	}

	// Additional security check: "public" schema should not be allowed for user data
	if schema == "public" {
		return errors.New("public schema is not allowed for user data access")
	}

	// Check if a user with this schema exists
	_, err := s.userRepo.GetBySchema(ctx, schema)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return errors.New("schema not found")
		}
		return fmt.Errorf("failed to validate schema: %w", err)
	}

	return nil
}
