package domain

import (
	"encoding/json"
	"fmt"
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
	ItemTypePasskey     ItemType = 10 // Passkey/WebAuthn/FIDO2 credential
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
	case ItemTypePasskey:
		return "Passkey"
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
		ItemTypeIdentity, ItemTypeSSHKey, ItemTypeAddress, ItemTypePasskey, ItemTypeCustom,
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
