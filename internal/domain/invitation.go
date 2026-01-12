package domain

import (
	"errors"
	"time"
)

// Invitation represents a user invitation
type Invitation struct {
	ID        uint      `gorm:"primary_key" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// NOTE: Email is NOT unique because a user can have multiple invitations
	// (e.g. multiple organization invites). Code remains unique.
	Email     string     `json:"email" gorm:"type:varchar(255);index;not null"`
	Code      string     `json:"code" gorm:"type:varchar(64);uniqueIndex;not null"`
	RoleID    uint       `json:"role_id" gorm:"not null"`
	CreatedBy uint       `json:"created_by" gorm:"not null"` // Admin user ID
	ExpiresAt time.Time  `json:"expires_at" gorm:"not null"`
	UsedAt    *time.Time `json:"used_at,omitempty"`

	// Organization invitation fields (optional)
	OrganizationID  *uint   `json:"organization_id,omitempty" gorm:"index"`
	OrgRole         *string `json:"org_role,omitempty" gorm:"type:varchar(50)"`
	EncryptedOrgKey *string `json:"encrypted_org_key,omitempty" gorm:"type:text"`
	AccessAll       bool    `json:"access_all" gorm:"default:false"`
}

// TableName specifies the table name for Invitation
func (Invitation) TableName() string {
	return "invitations"
}

// IsExpired checks if invitation is expired
func (i *Invitation) IsExpired() bool {
	return time.Now().After(i.ExpiresAt)
}

// IsUsed checks if invitation is already used
func (i *Invitation) IsUsed() bool {
	return i.UsedAt != nil
}

// CreateInvitationRequest represents invitation creation request
type CreateInvitationRequest struct {
	Email       string  `json:"email" validate:"required,email"`
	RoleID      uint    `json:"role_id" validate:"required"`
	Description *string `json:"description,omitempty"` // Optional personal note

	// Organization invitation fields (optional)
	OrganizationID  *uint   `json:"organization_id,omitempty"`
	OrgRole         *string `json:"org_role,omitempty"`
	EncryptedOrgKey *string `json:"encrypted_org_key,omitempty"`
	AccessAll       *bool   `json:"access_all,omitempty"`
}

// Validate validates the invitation request
func (r *CreateInvitationRequest) Validate() error {
	if r.Email == "" {
		return errors.New("email is required")
	}
	if r.RoleID == 0 {
		return errors.New("role_id is required")
	}
	return nil
}
