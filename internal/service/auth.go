package service

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
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
	"github.com/passwall/passwall-server/pkg/hash"
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
	folderRepo       repository.FolderRepository
	orgRepo          repository.OrganizationRepository
	orgUserRepo      repository.OrganizationUserRepository
	invitationRepo   repository.InvitationRepository
	activityService  UserActivityService
	emailSender      email.Sender
	emailBuilder     *email.EmailBuilder
	config           *AuthConfig
	logger           Logger
}

// NewAuthService creates a new authentication service
func NewAuthService(
	userRepo repository.UserRepository,
	tokenRepo repository.TokenRepository,
	verificationRepo repository.VerificationRepository,
	folderRepo repository.FolderRepository,
	orgRepo repository.OrganizationRepository,
	orgUserRepo repository.OrganizationUserRepository,
	invitationRepo repository.InvitationRepository,
	activityService UserActivityService,
	emailSender email.Sender,
	emailBuilder *email.EmailBuilder,
	config *AuthConfig,
	logger Logger,
) AuthService {
	return &authService{
		userRepo:         userRepo,
		tokenRepo:        tokenRepo,
		verificationRepo: verificationRepo,
		folderRepo:       folderRepo,
		orgRepo:          orgRepo,
		orgUserRepo:      orgUserRepo,
		invitationRepo:   invitationRepo,
		activityService:  activityService,
		emailSender:      emailSender,
		emailBuilder:     emailBuilder,
		config:           config,
		logger:           logger,
	}
}

func (s *authService) SignUp(ctx context.Context, req *domain.SignUpRequest) (*domain.User, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Check if user already exists
	existingUser, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err == nil && existingUser != nil {
		return nil, repository.ErrAlreadyExists
	}

	// Hash the master password hash with bcrypt (defense in depth)
	// Client sends: HKDF(masterKey, info="auth") (base64-encoded string)
	// Server stores: bcrypt(HKDF(masterKey, info="auth"))
	hashedPassword, err := bcrypt.GenerateFromPassword(
		[]byte(req.MasterPasswordHash),
		bcrypt.DefaultCost,
	)
	if err != nil {
		s.logger.Error("failed to hash password", "error", err)
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	schema := generateSchema(req.Email)

	// Create user with modern encryption fields
	user := &domain.User{
		UUID:               uuid.NewV4(),
		Name:               req.Name,
		Email:              req.Email,
		MasterPasswordHash: string(hashedPassword),
		ProtectedUserKey:   req.ProtectedUserKey, // EncString: "2.iv|ct|mac"
		Schema:             schema,
		KdfType:            req.KdfConfig.Type,
		KdfIterations:      req.KdfConfig.Iterations,
		KdfMemory:          req.KdfConfig.Memory,
		KdfParallelism:     req.KdfConfig.Parallelism,
		KdfSalt:            req.KdfSalt, // Random salt from client
		RoleID:             constants.RoleIDMember,
		IsVerified:         false,
	}

	// Create schema
	if err := s.userRepo.CreateSchema(schema); err != nil {
		s.logger.Error("failed to create schema", "schema", schema, "error", err)
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	// Migrate all tables in user schema
	if err := s.userRepo.MigrateUserSchema(schema); err != nil {
		s.logger.Error("failed to migrate user schema tables", "schema", schema, "error", err)
		return nil, fmt.Errorf("failed to migrate user schema: %w", err)
	}

	// Save user
	if err := s.userRepo.Create(ctx, user); err != nil {
		s.logger.Error("failed to create user", "email", req.Email, "error", err)
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Create default folders for new user
	if err := s.createDefaultFolders(ctx, user.ID); err != nil {
		s.logger.Error("failed to create default folders", "user_id", user.ID, "error", err)
		// Don't fail signup if folder creation fails
	}

	// Create personal organization for new user
	if err := s.createPersonalOrganization(ctx, user, req.EncryptedOrgKey); err != nil {
		s.logger.Error("failed to create personal organization", "user_id", user.ID, "error", err)
		// Don't fail signup if organization creation fails
	}

	// Note: Organization invitations remain pending - user will see them after sign-in
	// and can accept/decline them manually

	// Generate verification code
	code, err := generateRandomVerificationCode(6)
	if err != nil {
		s.logger.Error("failed to generate verification code", "error", err)
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
		return user, nil
	}

	// Send verification email (async)
	go func() {
		emailCtx := context.Background()
		
		// Build verification email message
		message, err := s.emailBuilder.BuildVerificationEmail(req.Email, req.Name, code)
		if err != nil {
			s.logger.Error("failed to build verification email", "email", req.Email, "error", err)
			return
		}
		
		// Send email
		if err := s.emailSender.Send(emailCtx, message); err != nil {
			s.logger.Error("failed to send verification email", "email", req.Email, "error", err)
		}
	}()

	// Log account creation activity
	go func() {
		_ = s.activityService.LogActivity(context.Background(), &domain.CreateActivityRequest{
			UserID:       user.ID,
			ActivityType: domain.ActivityTypeAccountCreated,
			Details: CreateActivityDetails(map[string]interface{}{
				"kdf_type":   user.KdfType.String(),
				"iterations": user.KdfIterations,
			}),
		})
	}()

	s.logger.Info("user signup successful (zero-knowledge)",
		"email", req.Email,
		"kdf_type", user.KdfType.String(),
		"iterations", user.KdfIterations)

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
	// Find user by email
	user, err := s.userRepo.GetByEmail(ctx, creds.Email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrUnauthorized
		}
		s.logger.Error("failed to get user", "email", creds.Email, "error", err)
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	// Verify master password hash
	// Client sent: HKDF(masterKey, info="auth") (base64-encoded string)
	// Server has: bcrypt(HKDF(masterKey, info="auth"))
	if err := bcrypt.CompareHashAndPassword(
		[]byte(user.MasterPasswordHash),
		[]byte(creds.MasterPasswordHash),
	); err != nil {
		s.logger.Warn("invalid password attempt", "email", creds.Email)
		return nil, ErrUnauthorized
	}

	// Check if email is verified
	if !user.IsVerified {
		s.logger.Warn("signin attempt with unverified email", "email", creds.Email)
		return nil, errors.New("email not verified")
	}

	// Delete old tokens
	if err := s.tokenRepo.Delete(ctx, int(user.ID)); err != nil {
		s.logger.Warn("failed to delete old tokens", "user_id", user.ID)
	}

	// Create JWT tokens
	tokenDetails, err := s.createToken(user)
	if err != nil {
		return nil, fmt.Errorf("failed to create token: %w", err)
	}

	// Store tokens
	if err := s.tokenRepo.Create(ctx, int(user.ID), tokenDetails.AtUUID, tokenDetails.AccessToken, tokenDetails.AtExpiresTime); err != nil {
		return nil, fmt.Errorf("failed to store access token: %w", err)
	}

	if err := s.tokenRepo.Create(ctx, int(user.ID), tokenDetails.RtUUID, tokenDetails.RefreshToken, tokenDetails.RtExpiresTime); err != nil {
		return nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

	// Log signin activity (async, non-blocking)
	// Note: IP and UserAgent should be passed from handler context
	go func() {
		_ = s.activityService.LogActivity(context.Background(), &domain.CreateActivityRequest{
			UserID:       user.ID,
			ActivityType: domain.ActivityTypeSignIn,
			// IP and UserAgent will be set in handler middleware
		})
	}()

	// Return auth response with protected user key
	// Client will decrypt User Key with their Master Key
	return &domain.AuthResponse{
		AccessToken:      tokenDetails.AccessToken,
		RefreshToken:     tokenDetails.RefreshToken,
		Type:             "Bearer",
		AccessTokenExpiresAt:  tokenDetails.AccessTokenExpiresAt,
		RefreshTokenExpiresAt: tokenDetails.RefreshTokenExpiresAt,
		ProtectedUserKey: user.ProtectedUserKey, // Encrypted, client will decrypt
		KdfConfig:        user.GetKdfConfig(),
		User: &domain.UserAuthDTO{
			ID:         user.ID,
			UUID:       user.UUID.String(),
			Email:      user.Email,
			Name:       user.Name,
			Schema:     user.Schema,
			Role:       user.GetRoleName(),
			IsVerified: user.IsVerified,
			Language:   user.Language,
		},
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

	// SECURITY: Require token UUID and verify token is currently active (prevents replay after rotation)
	tokenUUID, _ := claims["uuid"].(string)
	if tokenUUID == "" {
		return nil, ErrUnauthorized
	}

	// Get user UUID from claims
	userUUIDStr, ok := claims["user_uuid"].(string)
	if !ok {
		return nil, ErrUnauthorized
	}

	// SECURITY: Check token presence + expiry in DB (revocation) and validate token hash matches.
	dbToken, err := s.tokenRepo.GetByUUID(ctx, tokenUUID)
	if err != nil {
		return nil, ErrExpiredToken
	}
	if dbToken.IsExpired() {
		_ = s.tokenRepo.DeleteByUUID(ctx, tokenUUID)
		return nil, ErrExpiredToken
	}
	// TokenRepository stores SHA-256 hash of the token string.
	expectedHash := dbToken.Token
	actualHash := hash.SHA256(refreshToken)
	if subtle.ConstantTimeCompare([]byte(expectedHash), []byte(actualHash)) != 1 {
		// Token doesn't match stored hash -> treat as invalid/revoked
		_ = s.tokenRepo.DeleteByUUID(ctx, tokenUUID)
		return nil, ErrExpiredToken
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

	// SECURITY: Validate token hash matches DB (TokenRepository stores SHA-256 of token string).
	// This prevents accepting any token with a valid UUID/exp unless it matches the currently active token value.
	expectedHash := dbToken.Token
	actualHash := hash.SHA256(tokenString)
	if subtle.ConstantTimeCompare([]byte(expectedHash), []byte(actualHash)) != 1 {
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
	td.AccessTokenExpiresAt = td.AtExpiresTime.Unix()

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
	td.RefreshTokenExpiresAt = td.RtExpiresTime.Unix()

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

// PreLogin returns user's KDF configuration
// Required before signin to derive correct Master Key on client
func (s *authService) PreLogin(ctx context.Context, email string) (*domain.PreLoginResponse, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		// Don't reveal if user exists (prevents user enumeration)
		// Return default config with fake salt
		fakeSalt := []byte(email + "fake-salt-for-non-existent-user-padding-to-32-bytes-xxxx")
		return &domain.PreLoginResponse{
			KdfType:       domain.KdfTypePBKDF2,
			KdfIterations: domain.PBKDF2DefaultIterations,
			KdfSalt:       fmt.Sprintf("%x", fakeSalt[:32]),
		}, nil
	}

	// Get user's KDF config
	kdfConfig := user.GetKdfConfig()

	// Validate against downgrade attacks
	if err := kdfConfig.ValidateForPrelogin(); err != nil {
		s.logger.Error("KDF downgrade attack detected", "email", email, "error", err)
		// Return default config to prevent attack (with fake salt)
		fakeSalt := []byte(email + "downgrade-attack-fake-salt-padding-32-bytes-xxxxx")
		return &domain.PreLoginResponse{
			KdfType:       domain.KdfTypePBKDF2,
			KdfIterations: domain.PBKDF2DefaultIterations,
			KdfSalt:       fmt.Sprintf("%x", fakeSalt[:32]),
		}, nil
	}

	return &domain.PreLoginResponse{
		KdfType:        user.KdfType,
		KdfIterations:  user.KdfIterations,
		KdfMemory:      user.KdfMemory,
		KdfParallelism: user.KdfParallelism,
		KdfSalt:        user.KdfSalt,
	}, nil
}

// ChangeMasterPassword changes user's master password
// Zero-knowledge: Client re-wraps User Key with new Master Key
func (s *authService) ChangeMasterPassword(ctx context.Context, req *domain.ChangeMasterPasswordRequest) error {
	// Find user
	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		return ErrUnauthorized
	}

	// Verify current password hash
	if err := bcrypt.CompareHashAndPassword(
		[]byte(user.MasterPasswordHash),
		[]byte(req.CurrentPasswordHash),
	); err != nil {
		return ErrUnauthorized
	}

	// Hash new master password hash
	newHashedPassword, err := bcrypt.GenerateFromPassword(
		[]byte(req.NewMasterPasswordHash),
		bcrypt.DefaultCost,
	)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}

	// Update user
	user.MasterPasswordHash = string(newHashedPassword)
	user.ProtectedUserKey = req.NewProtectedUserKey

	// Update KDF config if provided
	if req.NewKdfConfig != nil {
		if err := req.NewKdfConfig.Validate(); err != nil {
			return fmt.Errorf("invalid KDF config: %w", err)
		}
		user.KdfType = req.NewKdfConfig.Type
		user.KdfIterations = req.NewKdfConfig.Iterations
		user.KdfMemory = req.NewKdfConfig.Memory
		user.KdfParallelism = req.NewKdfConfig.Parallelism
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		s.logger.Error("failed to update user password", "user_id", user.ID, "error", err)
		return fmt.Errorf("failed to update password: %w", err)
	}

	// Delete all tokens (force re-login on all devices)
	if err := s.tokenRepo.Delete(ctx, int(user.ID)); err != nil {
		s.logger.Warn("failed to delete tokens after password change", "user_id", user.ID)
	}

	s.logger.Info("master password changed successfully",
		"user_id", user.ID,
		"new_kdf", user.KdfType.String())

	return nil
}

func generateSchema(email string) string {
	return "user_" + uuid.NewV5(uuid.NamespaceURL, email).String()[:8]
}

// createDefaultFolders creates default folders for a new user
func (s *authService) createDefaultFolders(ctx context.Context, userID uint) error {
	for _, folderName := range constants.DefaultFolders {
		folder := &domain.Folder{
			UUID:   uuid.NewV4(),
			UserID: userID,
			Name:   folderName,
		}

		if err := s.folderRepo.Create(ctx, folder); err != nil {
			s.logger.Error("failed to create default folder", "folder", folderName, "user_id", userID, "error", err)
			// Continue creating other folders even if one fails
			continue
		}
	}

	s.logger.Info("created default folders", "user_id", userID, "count", len(constants.DefaultFolders))
	return nil
}

// createPersonalOrganization creates a personal organization for a new user
func (s *authService) createPersonalOrganization(ctx context.Context, user *domain.User, encryptedOrgKey string) error {
	// Create personal organization
	// Note: Plan limits will be set via free subscription (created automatically during seeding)
	org := &domain.Organization{
		Name:            fmt.Sprintf("%s's Workspace", user.Name),
		BillingEmail:    user.Email,
		EncryptedOrgKey: encryptedOrgKey, // Organization key encrypted with User Key
		IsActive:        true,
	}

	if err := s.orgRepo.Create(ctx, org); err != nil {
		return fmt.Errorf("failed to create organization: %w", err)
	}

	// Add user as owner
	now := time.Now()
	orgUser := &domain.OrganizationUser{
		OrganizationID:  org.ID,
		UserID:          user.ID,
		Role:            domain.OrgRoleOwner,
		EncryptedOrgKey: encryptedOrgKey, // User's copy of org key
		AccessAll:       true,
		Status:          domain.OrgUserStatusConfirmed,
		InvitedAt:       &now,
		AcceptedAt:      &now,
	}

	if err := s.orgUserRepo.Create(ctx, orgUser); err != nil {
		// Try to delete the organization if we can't add the user
		_ = s.orgRepo.Delete(ctx, org.ID)
		return fmt.Errorf("failed to add user to organization: %w", err)
	}

	s.logger.Info("created personal organization",
		"user_id", user.ID,
		"org_id", org.ID,
		"org_name", org.Name)

	return nil
}

// processPendingOrgInvitations checks for pending organization invitations and adds user to organizations
func (s *authService) processPendingOrgInvitations(ctx context.Context, user *domain.User) error {
	// Check if there's a pending invitation for this email
	invitations, err := s.invitationRepo.GetAllByEmail(ctx, user.Email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			// No pending invitation - this is normal
			return nil
		}
		return fmt.Errorf("failed to check pending invitations: %w", err)
	}

	// Find the first org invitation (if any). We keep this simple to avoid surprising
	// auto-joins across multiple orgs; can be extended later.
	var invitation *domain.Invitation
	for _, inv := range invitations {
		if inv == nil {
			continue
		}
		if inv.OrganizationID != nil && inv.OrgRole != nil && inv.EncryptedOrgKey != nil {
			invitation = inv
			break
		}
	}
	if invitation == nil {
		return nil
	}

	// Check if user is already in the organization
	existing, err := s.orgUserRepo.GetByOrgAndUser(ctx, *invitation.OrganizationID, user.ID)
	if err == nil && existing != nil {
		s.logger.Info("user already in organization", "org_id", *invitation.OrganizationID, "user_id", user.ID)
		// Mark invitation as used
		now := time.Now()
		invitation.UsedAt = &now
		_ = s.invitationRepo.Delete(ctx, invitation.ID)
		return nil
	}

	// Add user to organization
	now := time.Now()
	orgUser := &domain.OrganizationUser{
		OrganizationID:  *invitation.OrganizationID,
		UserID:          user.ID,
		Role:            domain.OrganizationRole(*invitation.OrgRole),
		EncryptedOrgKey: *invitation.EncryptedOrgKey,
		AccessAll:       invitation.AccessAll,
		Status:          domain.OrgUserStatusAccepted, // Auto-accepted since they signed up via invitation
		InvitedAt:       &invitation.CreatedAt,
		AcceptedAt:      &now,
	}

	if err := s.orgUserRepo.Create(ctx, orgUser); err != nil {
		return fmt.Errorf("failed to add user to organization: %w", err)
	}

	// Mark invitation as used and delete
	invitation.UsedAt = &now
	if err := s.invitationRepo.Delete(ctx, invitation.ID); err != nil {
		s.logger.Error("failed to delete used invitation", "invitation_id", invitation.ID, "error", err)
		// Don't fail - user is already added to org
	}

	s.logger.Info("user auto-joined organization from pending invitation",
		"user_id", user.ID,
		"org_id", *invitation.OrganizationID,
		"org_role", *invitation.OrgRole)

	return nil
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
