package domain

import (
	"encoding/json"
	"fmt"
	"time"

	uuid "github.com/satori/go.uuid"
)

// ItemType - Enum for vault item types
type ItemType int16

const (
	ItemTypePassword    ItemType = 1
	ItemTypeSecureNote  ItemType = 2
	ItemTypeCard        ItemType = 3
	ItemTypeBankAccount ItemType = 4
	ItemTypeEmail       ItemType = 5
	ItemTypeServer      ItemType = 6
	ItemTypeIdentity    ItemType = 7
	ItemTypeSSHKey      ItemType = 8
	ItemTypeAddress     ItemType = 9  // Address/Location
	ItemTypeCustom      ItemType = 99 // User-defined
)

// String returns the string representation of ItemType
func (it ItemType) String() string {
	switch it {
	case ItemTypePassword:
		return "Password"
	case ItemTypeSecureNote:
		return "SecureNote"
	case ItemTypeCard:
		return "Card"
	case ItemTypeBankAccount:
		return "BankAccount"
	case ItemTypeEmail:
		return "Email"
	case ItemTypeServer:
		return "Server"
	case ItemTypeIdentity:
		return "Identity"
	case ItemTypeSSHKey:
		return "SSHKey"
	case ItemTypeAddress:
		return "Address"
	case ItemTypeCustom:
		return "Custom"
	default:
		return "Unknown"
	}
}

// IsValid checks if item type is valid
func (it ItemType) IsValid() bool {
	validTypes := []ItemType{
		ItemTypePassword, ItemTypeSecureNote, ItemTypeCard,
		ItemTypeBankAccount, ItemTypeEmail, ItemTypeServer,
		ItemTypeIdentity, ItemTypeSSHKey, ItemTypeAddress, ItemTypeCustom,
	}
	for _, vt := range validTypes {
		if it == vt {
			return true
		}
	}
	return false
}

// FieldType - Custom field types
type FieldType int

const (
	FieldTypeText    FieldType = 0 // Plain text
	FieldTypeHidden  FieldType = 1 // Password/secret (encrypted)
	FieldTypeBoolean FieldType = 2 // Checkbox
	FieldTypeLinked  FieldType = 3 // Linked to another field
)

// ItemMetadata - Searchable metadata (NOT encrypted)
type ItemMetadata struct {
	Name     string   `json:"name"`                // Required: display name
	URIHint  string   `json:"uri_hint,omitempty"`  // For passwords: domain for autofill
	Brand    string   `json:"brand,omitempty"`     // For cards: Visa, Mastercard, etc.
	Category string   `json:"category,omitempty"`  // Custom category
	Tags     []string `json:"tags,omitempty"`      // User tags for organization
	IconHint string   `json:"icon_hint,omitempty"` // For UI (favicon URL hint)
}

// Item - Universal vault item entity
type Item struct {
	ID        uint       `gorm:"primary_key" json:"id"`
	UUID      uuid.UUID  `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	SupportID int64      `gorm:"uniqueIndex;not null" json:"support_id"` // Human-readable ID for support
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" gorm:"index"`

	// Sync
	Revision    int64 `json:"revision" gorm:"not null;default:0"`
	SyncVersion int   `json:"sync_version" gorm:"not null;default:1"`

	ItemType ItemType     `json:"item_type" gorm:"not null"`
	Data     string       `json:"data" gorm:"type:text;not null"` // Encrypted JSON
	Metadata ItemMetadata `json:"metadata" gorm:"type:jsonb;not null"`

	// User preferences
	IsFavorite bool  `json:"is_favorite" gorm:"default:false"`
	FolderID   *uint `json:"folder_id,omitempty"`
	Reprompt   bool  `json:"reprompt" gorm:"default:false"`

	// Browser extension features (Password items only)
	AutoFill  bool `json:"auto_fill" gorm:"default:true"`   // Enable auto-fill
	AutoLogin bool `json:"auto_login" gorm:"default:false"` // Enable auto-submit

	ArchivedAt *time.Time `json:"archived_at,omitempty"`
}

// TableName specifies the table name for Item
func (Item) TableName() string {
	return "items"
}

// IsDeleted checks if item is soft deleted
func (i *Item) IsDeleted() bool {
	return i.DeletedAt != nil
}

// IsArchived checks if item is archived
func (i *Item) IsArchived() bool {
	return i.ArchivedAt != nil
}

// FormatSupportID formats support ID for display
// Example: 1855215206460051939 → "1855 2152 0646 0051 939"
func (i *Item) FormatSupportID() string {
	idStr := fmt.Sprintf("%019d", i.SupportID) // Pad to 19 digits

	// Group into 5 groups of 4, 4, 4, 4, 3 digits
	return fmt.Sprintf("%s %s %s %s %s",
		idStr[0:4],
		idStr[4:8],
		idStr[8:12],
		idStr[12:16],
		idStr[16:19],
	)
}

// Scan implements sql.Scanner for ItemMetadata (JSONB)
func (m *ItemMetadata) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan ItemMetadata: expected []byte, got %T", value)
	}

	return json.Unmarshal(bytes, m)
}

// Value implements driver.Valuer for ItemMetadata (JSONB)
func (m ItemMetadata) Value() (interface{}, error) {
	if m.Name == "" {
		return "{}", nil
	}
	return json.Marshal(m)
}
