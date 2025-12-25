package domain

import (
	"time"

	uuid "github.com/satori/go.uuid"
)

// Credentials represents login credentials
type Credentials struct {
	Email          string `json:"email" binding:"required,email"`
	MasterPassword string `json:"master_password" binding:"required,min=6,max=100"`
}

// SignUpRequest represents registration request
type SignUpRequest struct {
	Name           string `json:"name" binding:"max=100"`
	Email          string `json:"email" binding:"required,email"`
	MasterPassword string `json:"master_password" binding:"required,min=6,max=100"`
}

// TokenDetails contains access and refresh token information
type TokenDetails struct {
	AccessToken   string    `json:"access_token"`
	RefreshToken  string    `json:"refresh_token"`
	AtExpiresTime time.Time `json:"at_expires_time"`
	RtExpiresTime time.Time `json:"rt_expires_time"`
	AtUUID        uuid.UUID `json:"at_uuid"`
	RtUUID        uuid.UUID `json:"rt_uuid"`
}

// TokenClaims represents JWT token claims
type TokenClaims struct {
	UserID      uint      `json:"user_id"`
	Email       string    `json:"email"`
	Schema      string    `json:"schema"`
	Role        string    `json:"role"`
	Permissions []string  `json:"permissions,omitempty"` // Optional: list of permission names
	UUID        uuid.UUID `json:"uuid"`
	Exp         int64     `json:"exp"`
}

// AuthResponse represents authentication response
type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Type         string `json:"type"` // Token type (e.g., "Bearer") - BACKWARD COMPATIBLE
	UserID       uint   `json:"user_id"`
	Email        string `json:"email"`
	Name         string `json:"name"`
	Schema       string `json:"schema"`
	Role         string `json:"role"`        // User role (e.g., "user", "admin") - BACKWARD COMPATIBLE
	Secret       string `json:"secret"`      // User's encryption secret (required by extension)
	IsMigrated   bool   `json:"is_migrated"` // Migration status flag
}

// ChangeMasterPasswordRequest represents a password change request
type ChangeMasterPasswordRequest struct {
	Email             string `json:"email" binding:"required,email"`
	OldMasterPassword string `json:"old_master_password" binding:"required"`
	NewMasterPassword string `json:"new_master_password" binding:"required,min=6,max=100"`
}
