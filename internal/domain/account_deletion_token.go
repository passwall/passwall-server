package domain

import (
	"time"

	"github.com/google/uuid"
)

// AccountDeletionToken stores a single-use token for auth-free recovery delete flow.
type AccountDeletionToken struct {
	ID        uint      `gorm:"primary_key" json:"id"`
	UUID      uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();uniqueIndex;not null" json:"uuid"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	TokenHash string    `gorm:"type:varchar(64);not null" json:"-"`
	ExpiresAt time.Time `gorm:"not null;index" json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName specifies the table name for AccountDeletionToken.
func (AccountDeletionToken) TableName() string {
	return "account_deletion_tokens"
}

// IsExpired checks if token is expired.
func (t *AccountDeletionToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}
