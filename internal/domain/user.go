package domain

import (
	"time"

	uuid "github.com/satori/go.uuid"
)

// User represents a user account in the system
type User struct {
	ID               uint       `gorm:"primary_key" json:"id"`
	UUID             uuid.UUID  `gorm:"type:uuid;type:varchar(100);" json:"uuid"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty" gorm:"index"`
	Name             string     `json:"name" gorm:"type:varchar(255)"`
	Email            string     `json:"email" gorm:"type:varchar(255);uniqueIndex;not null"`
	MasterPassword   string     `json:"-" gorm:"type:varchar(255);not null"` // Never expose in JSON
	Secret           string     `json:"-" gorm:"type:text"` // Encryption secret
	Schema           string     `json:"schema" gorm:"type:varchar(255);uniqueIndex;not null"`
	Role             string     `json:"role" gorm:"type:varchar(50);default:'user'"`
	ConfirmationCode string     `json:"-" gorm:"type:varchar(10)"`
	EmailVerifiedAt  time.Time  `json:"email_verified_at"`
	IsMigrated       bool       `json:"is_migrated" gorm:"default:false"`
}

// TableName specifies the table name for User
func (User) TableName() string {
	return "users"
}

