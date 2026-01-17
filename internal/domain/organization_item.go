package domain

import (
	"fmt"
	"time"

	uuid "github.com/satori/go.uuid"
)

// OrganizationItem represents a vault item shared within an organization
// Unlike personal items (in user schemas), org items are in the public schema
// and encrypted with the organization key instead of user key
type OrganizationItem struct {
	ID        uint       `gorm:"primary_key" json:"id"`
	UUID      uuid.UUID  `gorm:"type:uuid;not null" json:"uuid"`
	SupportID int64      `gorm:"not null" json:"support_id"` // Human-readable ID for support
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" gorm:"index"`

	// Organization and collection
	OrganizationID uint  `json:"organization_id" gorm:"not null;index;constraint:OnDelete:CASCADE"`
	CollectionID   *uint `json:"collection_id,omitempty" gorm:"index;constraint:OnDelete:SET NULL"` // NULL means org-wide item

	// Sync
	Revision    int64 `json:"revision" gorm:"not null;default:0"`
	SyncVersion int   `json:"sync_version" gorm:"not null;default:1"`

	// Item type and data
	ItemType ItemType     `json:"item_type" gorm:"not null"`
	Data     string       `json:"data" gorm:"type:text;not null"` // Encrypted with Org Key
	Metadata ItemMetadata `json:"metadata" gorm:"type:jsonb;not null"`

	// User preferences
	IsFavorite bool  `json:"is_favorite" gorm:"default:false"`
	FolderID   *uint `json:"folder_id,omitempty"` // Organization folder (future feature)
	Reprompt   bool  `json:"reprompt" gorm:"default:false"`

	// Browser extension features (Password items only)
	AutoFill  bool `json:"auto_fill" gorm:"default:true"`
	AutoLogin bool `json:"auto_login" gorm:"default:false"`

	ArchivedAt *time.Time `json:"archived_at,omitempty"`

	// Creator info
	CreatedByUserID uint `json:"created_by_user_id" gorm:"not null;index"`

	// Associations
	Organization *Organization `json:"organization,omitempty" gorm:"foreignKey:OrganizationID"`
	Collection   *Collection   `json:"collection,omitempty" gorm:"foreignKey:CollectionID"`
	CreatedBy    *User         `json:"created_by,omitempty" gorm:"foreignKey:CreatedByUserID"`
}

// TableName specifies the table name
func (OrganizationItem) TableName() string {
	return "organization_items"
}

// IsDeleted checks if item is soft deleted
func (oi *OrganizationItem) IsDeleted() bool {
	return oi.DeletedAt != nil
}

// IsArchived checks if item is archived
func (oi *OrganizationItem) IsArchived() bool {
	return oi.ArchivedAt != nil
}

// FormatSupportID formats support ID for display
func (oi *OrganizationItem) FormatSupportID() string {
	idStr := fmt.Sprintf("%019d", oi.SupportID)
	return fmt.Sprintf("%s %s %s %s %s",
		idStr[0:4],
		idStr[4:8],
		idStr[8:12],
		idStr[12:16],
		idStr[16:19],
	)
}

// OrganizationItemDTO for API responses
type OrganizationItemDTO struct {
	ID                 uint         `json:"id"`
	UUID               uuid.UUID    `json:"uuid"`
	SupportID          int64        `json:"support_id"`
	SupportIDFormatted string       `json:"support_id_formatted"`
	OrganizationID     uint         `json:"organization_id"`
	CollectionID       *uint        `json:"collection_id,omitempty"`
	ItemType           ItemType     `json:"item_type"`
	Data               string       `json:"data"` // Still encrypted
	Metadata           ItemMetadata `json:"metadata"`
	IsFavorite         bool         `json:"is_favorite"`
	FolderID           *uint        `json:"folder_id,omitempty"`
	Reprompt           bool         `json:"reprompt"`
	AutoFill           bool         `json:"auto_fill"`
	AutoLogin          bool         `json:"auto_login"`
	Revision           int64        `json:"revision"`
	SyncVersion        int          `json:"sync_version"`
	CreatedByUserID    uint         `json:"created_by_user_id"`
	CreatedByUserEmail string       `json:"created_by_user_email,omitempty"`
	CreatedAt          time.Time    `json:"created_at"`
	UpdatedAt          time.Time    `json:"updated_at"`
	ArchivedAt         *time.Time   `json:"archived_at,omitempty"`
}

// CreateOrganizationItemRequest for API requests
type CreateOrganizationItemRequest struct {
	CollectionID *uint        `json:"collection_id,omitempty"`
	ItemType     ItemType     `json:"item_type" validate:"required"`
	Data         string       `json:"data" validate:"required"` // Encrypted with Org Key
	Metadata     ItemMetadata `json:"metadata" validate:"required"`
	IsFavorite   bool         `json:"is_favorite"`
	FolderID     *uint        `json:"folder_id,omitempty"`
	Reprompt     bool         `json:"reprompt"`
	AutoFill     *bool        `json:"auto_fill,omitempty"`
	AutoLogin    *bool        `json:"auto_login,omitempty"`
}

// UpdateOrganizationItemRequest for API requests
type UpdateOrganizationItemRequest struct {
	CollectionID *uint         `json:"collection_id,omitempty"`
	Data         *string       `json:"data,omitempty"`
	Metadata     *ItemMetadata `json:"metadata,omitempty"`
	IsFavorite   *bool         `json:"is_favorite,omitempty"`
	FolderID     *uint         `json:"folder_id,omitempty"`
	Reprompt     *bool         `json:"reprompt,omitempty"`
	AutoFill     *bool         `json:"auto_fill,omitempty"`
	AutoLogin    *bool         `json:"auto_login,omitempty"`
}

// MoveItemToCollectionRequest for moving items between collections
type MoveItemToCollectionRequest struct {
	CollectionID *uint `json:"collection_id"` // NULL to move to org-wide
}

// ToOrganizationItemDTO converts OrganizationItem to DTO
func ToOrganizationItemDTO(oi *OrganizationItem) *OrganizationItemDTO {
	if oi == nil {
		return nil
	}

	dto := &OrganizationItemDTO{
		ID:                 oi.ID,
		UUID:               oi.UUID,
		SupportID:          oi.SupportID,
		SupportIDFormatted: oi.FormatSupportID(),
		OrganizationID:     oi.OrganizationID,
		CollectionID:       oi.CollectionID,
		ItemType:           oi.ItemType,
		Data:               oi.Data,
		Metadata:           oi.Metadata,
		IsFavorite:         oi.IsFavorite,
		FolderID:           oi.FolderID,
		Reprompt:           oi.Reprompt,
		AutoFill:           oi.AutoFill,
		AutoLogin:          oi.AutoLogin,
		Revision:           oi.Revision,
		SyncVersion:        oi.SyncVersion,
		CreatedByUserID:    oi.CreatedByUserID,
		CreatedAt:          oi.CreatedAt,
		UpdatedAt:          oi.UpdatedAt,
		ArchivedAt:         oi.ArchivedAt,
	}

	// Add creator info if loaded
	if oi.CreatedBy != nil {
		dto.CreatedByUserEmail = oi.CreatedBy.Email
	}

	return dto
}

// Scan implements sql.Scanner for ItemMetadata (already defined in item.go)
// Value implements driver.Valuer for ItemMetadata (already defined in item.go)

// SharePersonalItemRequest for sharing a personal item to organization
type SharePersonalItemRequest struct {
	PersonalItemUUID string `json:"personal_item_uuid" validate:"required"`
	CollectionID     *uint  `json:"collection_id,omitempty"`
	Data             string `json:"data" validate:"required"` // Re-encrypted with Org Key
}

// ItemShare represents a direct share of a personal item to another user/team
// This is different from organization items - it's for ad-hoc sharing
type ItemShare struct {
	ID        uint      `gorm:"primary_key" json:"id"`
	UUID      uuid.UUID `gorm:"type:uuid;not null" json:"uuid"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Item reference (in owner's schema)
	ItemUUID   uuid.UUID `json:"item_uuid" gorm:"type:uuid;not null;index"`
	UserSchema string    `json:"user_schema" gorm:"type:varchar(255);not null"` // Owner's schema

	OwnerID uint `json:"owner_id" gorm:"not null;index;constraint:OnDelete:CASCADE"`

	// Share target (either user or team)
	SharedWithUserID *uint `json:"shared_with_user_id,omitempty" gorm:"index;constraint:OnDelete:CASCADE"`
	SharedWithTeamID *uint `json:"shared_with_team_id,omitempty" gorm:"index;constraint:OnDelete:CASCADE"`

	// Permissions
	CanView      bool `json:"can_view" gorm:"default:true"`
	CanEdit      bool `json:"can_edit" gorm:"default:false"`
	CanShare     bool `json:"can_share" gorm:"default:false"` // Can re-share to others

	// Encrypted item key wrapped for recipient
	EncryptedKey string `json:"-" gorm:"type:text;not null"`

	// Expiration
	ExpiresAt *time.Time `json:"expires_at,omitempty"`

	// Associations
	Owner          *User `json:"owner,omitempty" gorm:"foreignKey:OwnerID"`
	SharedWithUser *User `json:"shared_with_user,omitempty" gorm:"foreignKey:SharedWithUserID"`
	SharedWithTeam *Team `json:"shared_with_team,omitempty" gorm:"foreignKey:SharedWithTeamID"`
}

// TableName specifies the table name
func (ItemShare) TableName() string {
	return "item_shares"
}

// IsExpired checks if the share has expired
func (is *ItemShare) IsExpired() bool {
	return is.ExpiresAt != nil && is.ExpiresAt.Before(time.Now())
}

// ItemShareDTO for API responses
type ItemShareDTO struct {
	ID               uint       `json:"id"`
	UUID             uuid.UUID  `json:"uuid"`
	ItemUUID         uuid.UUID  `json:"item_uuid"`
	OwnerID          uint       `json:"owner_id"`
	OwnerEmail       string     `json:"owner_email,omitempty"`
	SharedWithUserID *uint      `json:"shared_with_user_id,omitempty"`
	SharedWithTeamID *uint      `json:"shared_with_team_id,omitempty"`
	SharedWithEmail  string     `json:"shared_with_email,omitempty"`
	SharedWithName   string     `json:"shared_with_name,omitempty"`
	CanView          bool       `json:"can_view"`
	CanEdit          bool       `json:"can_edit"`
	CanShare         bool       `json:"can_share"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

// CreateItemShareRequest for API requests
type CreateItemShareRequest struct {
	ItemUUID         string     `json:"item_uuid" validate:"required"`
	SharedWithUserID *uint      `json:"shared_with_user_id,omitempty"`
	SharedWithTeamID *uint      `json:"shared_with_team_id,omitempty"`
	CanView          bool       `json:"can_view"`
	CanEdit          bool       `json:"can_edit"`
	CanShare         bool       `json:"can_share"`
	EncryptedKey     string     `json:"encrypted_key" validate:"required"` // Item key wrapped for recipient
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
}

// Note: ItemMetadata.Scan() and ItemMetadata.Value() are already defined in item.go
// They are shared between Item and OrganizationItem
