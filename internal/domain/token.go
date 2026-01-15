package domain

import (
	"time"

	uuid "github.com/satori/go.uuid"
)

// Token represents authentication tokens stored in the database
type Token struct {
	ID         int       `gorm:"primary_key" json:"id"`
	UserID     int       `json:"user_id" gorm:"index;not null"`
	UUID       uuid.UUID `gorm:"type:uuid;type:varchar(100);uniqueIndex;not null" json:"uuid"`
	// SessionUUID groups access+refresh tokens belonging to the same login/session.
	// This allows multiple concurrent sessions (e.g. vault + extension) without revoking each other.
	SessionUUID uuid.UUID `gorm:"type:uuid;index" json:"-"`
	// DeviceID is an optional stable identifier for a device/app installation.
	// For Vault we currently align DeviceID with SessionUUID to avoid orphan sessions after tab close.
	DeviceID uuid.UUID `gorm:"type:uuid;index" json:"-"`
	// App identifies the client type: vault|extension|mobile|desktop
	App string `gorm:"type:varchar(16);index" json:"-"`
	// Kind is either "access" or "refresh" (optional for legacy rows).
	Kind string `gorm:"type:varchar(16);index" json:"-"`
	Token      string    `gorm:"type:text;not null" json:"-"`
	ExpiryTime time.Time `json:"expiry_time" gorm:"index;not null"`
}

// TableName specifies the table name for Token
func (Token) TableName() string {
	return "tokens"
}

// IsExpired checks if the token has expired
func (t *Token) IsExpired() bool {
	return time.Now().After(t.ExpiryTime)
}
