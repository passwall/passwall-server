package domain

import (
	"time"

	"github.com/passwall/passwall-server/pkg/constants"
	uuid "github.com/satori/go.uuid"
)

// User represents a user account in the system
type User struct {
	ID        uint      `gorm:"primary_key" json:"id"`
	UUID      uuid.UUID `gorm:"type:uuid;type:varchar(100);" json:"uuid"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// DeletedAt removed - using hard delete to allow re-registration with same email
	Name             string     `json:"name" gorm:"type:varchar(255)"`
	Email            string     `json:"email" gorm:"type:varchar(255);uniqueIndex;not null"`
	MasterPassword   string     `json:"-" gorm:"type:varchar(255);not null"` // Never expose in JSON
	Secret           string     `json:"-" gorm:"type:text"`                  // Encryption secret
	Schema           string     `json:"schema" gorm:"type:varchar(255);uniqueIndex;not null"`
	RoleID           uint       `json:"role_id" gorm:"not null;default:2;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"` // Foreign key with constraints
	Role             *Role      `json:"role,omitempty" gorm:"foreignKey:RoleID"`
	ConfirmationCode string     `json:"-" gorm:"type:varchar(10)"`
	EmailVerifiedAt  time.Time  `json:"email_verified_at"`
	LastSignInAt     *time.Time `json:"last_sign_in_at" gorm:"type:timestamp"`
	IsMigrated       bool       `json:"is_migrated" gorm:"default:false"`
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
