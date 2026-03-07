package domain

import (
	"time"

	"github.com/google/uuid"
)

// SendType represents the type of send content
type SendType string

const (
	SendTypeText SendType = "text"
)

// Send represents a secure, shareable piece of data with expiration and access controls.
// Zero-knowledge: data is encrypted client-side, the decryption key lives only in the URL fragment.
type Send struct {
	ID        uint       `gorm:"primary_key" json:"id"`
	UUID      uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex" json:"uuid"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" gorm:"index"`

	// Short random slug for URL (12 chars)
	AccessID string `json:"access_id" gorm:"type:varchar(24);not null;uniqueIndex"`

	CreatorID      uint `json:"creator_id" gorm:"not null;index;constraint:OnDelete:CASCADE"`
	OrganizationID uint `json:"organization_id" gorm:"not null;index"`

	Name string   `json:"name" gorm:"type:varchar(255);not null"`
	Type SendType `json:"type" gorm:"type:varchar(20);not null;default:'text'"`

	// Encrypted text (EncString format, encrypted with client-generated SendKey)
	Data  string  `json:"data" gorm:"type:text;not null"`
	Notes *string `json:"notes,omitempty" gorm:"type:text"`

	// Optional bcrypt hash for extra password protection
	Password *string `json:"-" gorm:"type:varchar(255)"`

	MaxAccessCount *int `json:"max_access_count,omitempty"`
	AccessCount    int  `json:"access_count" gorm:"not null;default:0"`

	ExpirationDate *time.Time `json:"expiration_date,omitempty" gorm:"index"`
	DeletionDate   time.Time  `json:"deletion_date" gorm:"not null;index"`

	Disabled  bool `json:"disabled" gorm:"not null;default:false"`
	HideEmail bool `json:"hide_email" gorm:"not null;default:false"`

	// Associations
	Creator *User `json:"creator,omitempty" gorm:"foreignKey:CreatorID"`
}

func (Send) TableName() string {
	return "sends"
}

// IsExpired checks if the send has expired
func (s *Send) IsExpired() bool {
	if s.ExpirationDate != nil && s.ExpirationDate.Before(time.Now()) {
		return true
	}
	return false
}

// IsAccessLimitReached checks if access count has been exceeded
func (s *Send) IsAccessLimitReached() bool {
	if s.MaxAccessCount != nil && s.AccessCount >= *s.MaxAccessCount {
		return true
	}
	return false
}

// HasPassword checks if the send is password-protected
func (s *Send) HasPassword() bool {
	return s.Password != nil && *s.Password != ""
}

// SendDTO for API responses (creator view)
type SendDTO struct {
	ID             uint       `json:"id"`
	UUID           uuid.UUID  `json:"uuid"`
	AccessID       string     `json:"access_id"`
	CreatorID      uint       `json:"creator_id"`
	OrganizationID uint       `json:"organization_id"`
	Name           string     `json:"name"`
	Type           SendType   `json:"type"`
	Data           string     `json:"data"`
	Notes          *string    `json:"notes,omitempty"`
	HasPassword    bool       `json:"has_password"`
	MaxAccessCount *int       `json:"max_access_count,omitempty"`
	AccessCount    int        `json:"access_count"`
	ExpirationDate *time.Time `json:"expiration_date,omitempty"`
	DeletionDate   time.Time  `json:"deletion_date"`
	Disabled       bool       `json:"disabled"`
	HideEmail      bool       `json:"hide_email"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func ToSendDTO(s *Send) *SendDTO {
	if s == nil {
		return nil
	}
	return &SendDTO{
		ID:             s.ID,
		UUID:           s.UUID,
		AccessID:       s.AccessID,
		CreatorID:      s.CreatorID,
		OrganizationID: s.OrganizationID,
		Name:           s.Name,
		Type:           s.Type,
		Data:           s.Data,
		Notes:          s.Notes,
		HasPassword:    s.HasPassword(),
		MaxAccessCount: s.MaxAccessCount,
		AccessCount:    s.AccessCount,
		ExpirationDate: s.ExpirationDate,
		DeletionDate:   s.DeletionDate,
		Disabled:       s.Disabled,
		HideEmail:      s.HideEmail,
		CreatedAt:      s.CreatedAt,
		UpdatedAt:      s.UpdatedAt,
	}
}

// SendAccessDTO for recipient view (limited info)
type SendAccessDTO struct {
	AccessID       string     `json:"access_id"`
	Name           string     `json:"name"`
	Type           SendType   `json:"type"`
	Data           string     `json:"data"`
	Notes          *string    `json:"notes,omitempty"`
	CreatorEmail   string     `json:"creator_email,omitempty"`
	HasPassword    bool       `json:"has_password"`
	ExpirationDate *time.Time `json:"expiration_date,omitempty"`
}

// CreateSendRequest for creating a new send
type CreateSendRequest struct {
	Name           string     `json:"name" validate:"required"`
	OrganizationID uint       `json:"organization_id" validate:"required"`
	Type           SendType   `json:"type" validate:"required"`
	Data           string     `json:"data" validate:"required"`
	Notes          *string    `json:"notes,omitempty"`
	Password       *string    `json:"password,omitempty"`
	MaxAccessCount *int       `json:"max_access_count,omitempty"`
	ExpirationDate *time.Time `json:"expiration_date,omitempty"`
	DeletionDate   *time.Time `json:"deletion_date,omitempty"`
	HideEmail      bool       `json:"hide_email"`
}

// UpdateSendRequest for updating an existing send
type UpdateSendRequest struct {
	Name           *string    `json:"name,omitempty"`
	Data           *string    `json:"data,omitempty"`
	Notes          *string    `json:"notes,omitempty"`
	Password       *string    `json:"password,omitempty"`
	MaxAccessCount *int       `json:"max_access_count,omitempty"`
	ExpirationDate *time.Time `json:"expiration_date,omitempty"`
	DeletionDate   *time.Time `json:"deletion_date,omitempty"`
	Disabled       *bool      `json:"disabled,omitempty"`
	HideEmail      *bool      `json:"hide_email,omitempty"`
}
