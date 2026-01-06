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
