package domain

import (
	"testing"
)

func TestItemType_String(t *testing.T) {
	tests := []struct {
		itemType ItemType
		expected string
	}{
		{ItemTypePassword, "Password"},
		{ItemTypeSecureNote, "SecureNote"},
		{ItemTypeCard, "Card"},
		{ItemTypeBankAccount, "BankAccount"},
		{ItemTypeEmail, "Email"},
		{ItemTypeServer, "Server"},
		{ItemTypeIdentity, "Identity"},
		{ItemTypeSSHKey, "SSHKey"},
		{ItemTypeAddress, "Address"},
		{ItemTypePasskey, "Passkey"},
		{ItemTypeCustom, "Custom"},
		{ItemType(0), "Unknown"},
		{ItemType(100), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := tt.itemType.String()
			if got != tt.expected {
				t.Errorf("ItemType(%d).String() = %q, want %q", tt.itemType, got, tt.expected)
			}
		})
	}
}

func TestItemType_IsValid(t *testing.T) {
	validTypes := []ItemType{
		ItemTypePassword, ItemTypeSecureNote, ItemTypeCard,
		ItemTypeBankAccount, ItemTypeEmail, ItemTypeServer,
		ItemTypeIdentity, ItemTypeSSHKey, ItemTypeAddress,
		ItemTypePasskey, ItemTypeCustom,
	}

	for _, it := range validTypes {
		t.Run(it.String(), func(t *testing.T) {
			if !it.IsValid() {
				t.Errorf("ItemType(%d).IsValid() = false, want true", it)
			}
		})
	}

	invalidTypes := []ItemType{0, -1, 11, 50, 98, 100}
	for _, it := range invalidTypes {
		t.Run("invalid_"+it.String(), func(t *testing.T) {
			if it.IsValid() {
				t.Errorf("ItemType(%d).IsValid() = true, want false", it)
			}
		})
	}
}

func TestItemTypePasskey_Value(t *testing.T) {
	if ItemTypePasskey != 10 {
		t.Errorf("ItemTypePasskey = %d, want 10", ItemTypePasskey)
	}
}
