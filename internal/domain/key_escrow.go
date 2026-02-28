package domain

import (
	"time"
)

// KeyEscrowStatus represents the status of an escrowed key
type KeyEscrowStatus string

const (
	KeyEscrowStatusActive  KeyEscrowStatus = "active"
	KeyEscrowStatusRevoked KeyEscrowStatus = "revoked"
)

// KeyEscrow stores an organization's symmetric key encrypted with the org escrow key.
// When the user logs in via SSO, the server decrypts this key and returns it
// so the client can unlock org vault items without a master password.
// Personal vault items remain locked (require master password).
type KeyEscrow struct {
	ID        uint      `gorm:"primary_key" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	UserID         uint `json:"user_id" gorm:"not null;index;constraint:OnDelete:CASCADE"`
	OrganizationID uint `json:"organization_id" gorm:"not null;index;constraint:OnDelete:CASCADE"`

	// Org Key encrypted with org escrow key (AES-256-GCM)
	// Format: base64(nonce + ciphertext + tag)
	// DB column kept as wrapped_user_key for backward compat; semantically this is the org key.
	WrappedOrgKey string `json:"-" gorm:"column:wrapped_user_key;type:text;not null"`

	KeyVersion int             `json:"key_version" gorm:"not null;default:1"`
	Status     KeyEscrowStatus `json:"status" gorm:"type:varchar(20);not null;default:'active'"`

	// Associations
	User         *User         `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Organization *Organization `json:"organization,omitempty" gorm:"foreignKey:OrganizationID"`
}

// TableName specifies the table name
func (KeyEscrow) TableName() string {
	return "key_escrows"
}

// IsActive returns true if the escrow is active
func (ke *KeyEscrow) IsActive() bool {
	return ke.Status == KeyEscrowStatusActive
}

// OrgEscrowKey stores the organization's escrow key encrypted with the server master key.
// Each organization gets its own escrow key for isolation.
type OrgEscrowKey struct {
	ID        uint      `gorm:"primary_key" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	OrganizationID uint `json:"organization_id" gorm:"not null;uniqueIndex;constraint:OnDelete:CASCADE"`

	// Org escrow key encrypted with server's EscrowMasterKey (AES-256-GCM)
	// Format: base64(nonce + ciphertext + tag)
	EncryptedKey string `json:"-" gorm:"type:text;not null"`

	KeyVersion int             `json:"key_version" gorm:"not null;default:1"`
	Status     KeyEscrowStatus `json:"status" gorm:"type:varchar(20);not null;default:'active'"`

	// Associations
	Organization *Organization `json:"organization,omitempty" gorm:"foreignKey:OrganizationID"`
}

// TableName specifies the table name
func (OrgEscrowKey) TableName() string {
	return "org_escrow_keys"
}

// --- Request / Response DTOs ---

// EnrollKeyEscrowRequest is sent by the client during one-time key escrow enrollment.
// The client decrypts the org key using their user key and sends it (base64).
// Only the org key is escrowed — personal vault key is never sent.
type EnrollKeyEscrowRequest struct {
	OrgKey string `json:"org_key" binding:"required"` // base64-encoded raw Org Key (512-bit SymmetricKey)
}

// KeyEscrowStatusResponse for API responses
type KeyEscrowStatusResponse struct {
	Enabled    bool `json:"enabled"`
	Enrolled   bool `json:"enrolled"`
	KeyVersion int  `json:"key_version,omitempty"`
}
