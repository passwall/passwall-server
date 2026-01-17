package domain

import (
	"time"

	"github.com/passwall/passwall-server/pkg/constants"
	uuid "github.com/satori/go.uuid"
)

// KdfType represents the key derivation function type
type KdfType int

const (
	KdfTypePBKDF2   KdfType = 0
	KdfTypeArgon2id KdfType = 1
)

// String returns the string representation of KdfType
func (k KdfType) String() string {
	switch k {
	case KdfTypePBKDF2:
		return "PBKDF2-SHA256"
	case KdfTypeArgon2id:
		return "Argon2id"
	default:
		return "Unknown"
	}
}

// User represents a user account in the system
type User struct {
	ID        uint      `gorm:"primary_key" json:"id"`
	UUID      uuid.UUID `gorm:"type:uuid;type:varchar(100);" json:"uuid"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Name   string `json:"name" gorm:"type:varchar(255)"`
	Email  string `json:"email" gorm:"type:varchar(255);uniqueIndex;not null"`
	Schema string `json:"schema" gorm:"type:varchar(255);uniqueIndex;not null"`

	// Modern Zero-Knowledge Encryption Fields
	MasterPasswordHash string `json:"-" gorm:"type:varchar(255);not null"` // bcrypt(HKDF(masterKey, info="auth"))
	ProtectedUserKey   string `json:"-" gorm:"type:text;not null"`         // EncString: "2.iv|ct|mac"

	// KDF Configuration (per user, configurable)
	KdfType        KdfType `json:"kdf_type" gorm:"not null;default:0"`            // 0=PBKDF2, 1=Argon2id
	KdfIterations  int     `json:"kdf_iterations" gorm:"not null;default:600000"` // Default: 600K
	KdfMemory      *int    `json:"kdf_memory,omitempty"`                          // For Argon2 (MB)
	KdfParallelism *int    `json:"kdf_parallelism,omitempty"`                     // For Argon2 (threads)
	KdfSalt        string  `json:"-" gorm:"type:varchar(64);not null"`            // hex-encoded 32 bytes, random per user

	// RSA Keys for Organization Sharing (optional, generated when joining first org)
	RSAPublicKey     *string `json:"rsa_public_key,omitempty" gorm:"type:text"` // RSA-2048 public key (PEM format)
	RSAPrivateKeyEnc *string `json:"-" gorm:"type:text"`                        // RSA private key encrypted with User Key (EncString)

	// User metadata
	RoleID       uint   `json:"role_id" gorm:"not null;default:2;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
	Role         *Role  `json:"role,omitempty" gorm:"foreignKey:RoleID"`
	IsVerified   bool   `json:"is_verified" gorm:"default:false"`
	IsSystemUser bool   `json:"is_system_user" gorm:"default:false;index"` // System users (e.g., super admin) cannot be deleted
	Language     string `json:"language" gorm:"type:varchar(10);default:'en'"`

	// Stats (runtime calculated, not stored in DB)
	ItemCount *int `json:"item_count,omitempty" gorm:"-"`
}

// TableName specifies the table name for User
func (User) TableName() string {
	return "users"
}

// GetRoleName returns the role name with proper null handling
func (u *User) GetRoleName() string {
	// First try to get from preloaded Role
	if u.Role != nil {
		return u.Role.Name
	}
	// Fallback to RoleID if Role is not preloaded (e.g., after Update which clears associations)
	if u.RoleID == constants.RoleIDAdmin {
		return constants.RoleAdmin
	}
	return constants.RoleMember // Default fallback using constant
}

// IsAdmin checks if user is an admin
func (u *User) IsAdmin() bool {
	return u.GetRoleName() == constants.RoleAdmin
}

// HasPermission checks if user has a specific permission (requires Role.Permissions to be loaded)
func (u *User) HasPermission(permission string) bool {
	if u.Role == nil || u.Role.Permissions == nil {
		return false
	}
	for _, p := range u.Role.Permissions {
		if p.Name == permission {
			return true
		}
	}
	return false
}

// OwnershipCheckResult represents organizations where user is sole owner
type OwnershipCheckResult struct {
	IsSoleOwner   bool                    `json:"is_sole_owner"`
	Organizations []SoleOwnerOrganization `json:"organizations"`
}

// SoleOwnerOrganization represents an organization where user is the sole owner
type SoleOwnerOrganization struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	MemberCount int    `json:"member_count"`
	CanTransfer bool   `json:"can_transfer"` // True if there are other members to transfer to
}

// TransferOwnershipRequest represents a request to transfer organization ownership
type TransferOwnershipRequest struct {
	UserID         uint `json:"user_id" binding:"required"`
	OrganizationID uint `json:"organization_id" binding:"required"`
	NewOwnerUserID uint `json:"new_owner_user_id" binding:"required"`
}

// DeleteWithOrganizationsRequest represents a request to delete user with their organizations
type DeleteWithOrganizationsRequest struct {
	OrganizationIDs []uint `json:"organization_ids" binding:"required"`
}
