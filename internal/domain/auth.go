package domain

import (
	"time"

	uuid "github.com/satori/go.uuid"
)

// SignUpRequest represents a user registration request
type SignUpRequest struct {
	Name               string     `json:"name" validate:"max=100"`
	Email              string     `json:"email" validate:"required,email"`
	MasterPasswordHash string     `json:"master_password_hash" validate:"required"` // HKDF(masterKey, info="auth")
	ProtectedUserKey   string     `json:"protected_user_key" validate:"required"`   // EncString: "2.iv|ct|mac"
	KdfConfig          *KdfConfig `json:"kdf_config" validate:"required"`
	KdfSalt            string     `json:"kdf_salt" validate:"required"`          // hex-encoded random salt from client
	EncryptedOrgKey    string     `json:"encrypted_org_key" validate:"required"` // Organization key encrypted with User Key
}

// Validate validates the signup request
func (r *SignUpRequest) Validate() error {
	if r.Email == "" {
		return ErrValidation{Field: "email", Message: "email is required"}
	}
	if r.MasterPasswordHash == "" {
		return ErrValidation{Field: "master_password_hash", Message: "master password hash is required"}
	}
	if r.ProtectedUserKey == "" {
		return ErrValidation{Field: "protected_user_key", Message: "protected user key is required"}
	}
	if r.KdfConfig == nil {
		return ErrValidation{Field: "kdf_config", Message: "KDF configuration is required"}
	}
	if r.KdfSalt == "" {
		return ErrValidation{Field: "kdf_salt", Message: "KDF salt is required"}
	}
	if r.EncryptedOrgKey == "" {
		return ErrValidation{Field: "encrypted_org_key", Message: "encrypted organization key is required"}
	}

	// Validate KDF config
	if err := r.KdfConfig.Validate(); err != nil {
		return err
	}

	return nil
}

// Credentials represents user login credentials
type Credentials struct {
	Email              string `json:"email" validate:"required,email"`
	MasterPasswordHash string `json:"master_password_hash" validate:"required"`
	// Optional device identifier to keep a stable session per app/device.
	// Example: Vault can persist this in localStorage to avoid orphan sessions after tab close.
	DeviceID string `json:"device_id,omitempty"`
	// Optional app identifier (used with device_id). Expected: vault|extension|mobile|desktop
	App string `json:"app,omitempty"`
}

// AuthResponse represents the authentication response
type AuthResponse struct {
	AccessToken           string       `json:"access_token"`
	RefreshToken          string       `json:"refresh_token"`
	Type                  string       `json:"type"`                               // "Bearer"
	AccessTokenExpiresAt  int64        `json:"access_token_expires_at,omitempty"`  // unix seconds
	RefreshTokenExpiresAt int64        `json:"refresh_token_expires_at,omitempty"` // unix seconds
	ProtectedUserKey      string       `json:"protected_user_key"`
	KdfConfig             *KdfConfig   `json:"kdf_config"`
	User                  *UserAuthDTO `json:"user"`
}

// TokenDetails represents JWT token details
type TokenDetails struct {
	AccessToken           string    `json:"access_token"`
	RefreshToken          string    `json:"refresh_token"`
	AccessTokenExpiresAt  int64     `json:"access_token_expires_at,omitempty"`  // unix seconds
	RefreshTokenExpiresAt int64     `json:"refresh_token_expires_at,omitempty"` // unix seconds
	AtExpiresTime         time.Time `json:"-"`
	RtExpiresTime         time.Time `json:"-"`
	AtUUID                uuid.UUID `json:"-"`
	RtUUID                uuid.UUID `json:"-"`
	SessionUUID           uuid.UUID `json:"-"`
}

// TokenClaims represents JWT token claims
type TokenClaims struct {
	UserID uint      `json:"user_id"`
	Email  string    `json:"email"`
	Schema string    `json:"schema"`
	Role   string    `json:"role"`
	UUID   uuid.UUID `json:"uuid"`
	Exp    int64     `json:"exp"`
}

// UserAuthDTO represents user data in auth responses
type UserAuthDTO struct {
	ID         uint   `json:"id"`
	UUID       string `json:"uuid"`
	Email      string `json:"email"`
	Name       string `json:"name"`
	Schema     string `json:"schema"`
	Role       string `json:"role"`
	IsVerified bool   `json:"is_verified"`
	Language   string `json:"language"`
}

// PreLoginRequest represents prelogin request
type PreLoginRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// PreLoginResponse represents prelogin response (KDF config for client)
type PreLoginResponse struct {
	KdfType        KdfType `json:"kdf_type"`
	KdfIterations  int     `json:"kdf_iterations"`
	KdfMemory      *int    `json:"kdf_memory,omitempty"`
	KdfParallelism *int    `json:"kdf_parallelism,omitempty"`
	KdfSalt        string  `json:"kdf_salt"` // hex-encoded salt for master key derivation
}

// ChangeMasterPasswordRequest represents master password change request
type ChangeMasterPasswordRequest struct {
	// NOTE: This endpoint is authenticated (JWT). The server will use the
	// authenticated user (context) and ignore any client-provided email.
	Email string `json:"email,omitempty"`

	OldMasterPasswordHash string `json:"old_master_password_hash" validate:"required"`

	NewMasterPasswordHash string `json:"new_master_password_hash" validate:"required"`
	NewProtectedUserKey   string `json:"new_protected_user_key" validate:"required"`

	// Optional: rotate KDF settings and salt
	NewKdfConfig *KdfConfig `json:"new_kdf_config,omitempty"`
	NewKdfSalt   string     `json:"new_kdf_salt,omitempty"`
}

// ErrValidation represents a validation error
type ErrValidation struct {
	Field   string
	Message string
}

func (e ErrValidation) Error() string {
	return e.Message
}
