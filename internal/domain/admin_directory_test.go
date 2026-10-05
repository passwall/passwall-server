package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminDirectoryItemTypeKeys(t *testing.T) {
	t.Parallel()

	cases := []struct {
		itemType ItemType
		key      string
	}{
		{ItemTypePassword, "password"},
		{ItemTypeSecureNote, "secure_note"},
		{ItemTypeCard, "card"},
		{ItemTypeBankAccount, "bank_account"},
		{ItemTypeEmail, "email"},
		{ItemTypeServer, "server"},
		{ItemTypeIdentity, "identity"},
		{ItemTypeSSHKey, "ssh_key"},
		{ItemTypeAddress, "address"},
		{ItemTypePasskey, "passkey"},
		{ItemTypeCustom, "custom"},
	}
	for _, tc := range cases {
		key, ok := adminDirectoryItemTypeKey(tc.itemType)
		assert.True(t, ok, tc.key)
		assert.Equal(t, tc.key, key)
	}
	_, ok := adminDirectoryItemTypeKey(ItemType(77))
	assert.False(t, ok)
}

func TestDirectoryItemCounts(t *testing.T) {
	t.Parallel()

	unknown := directoryItemCounts(false, map[ItemType]int{ItemTypePassword: 4})
	assert.Nil(t, unknown.ItemCount)
	assert.Nil(t, unknown.ByType)

	empty := directoryItemCounts(true, nil)
	require.NotNil(t, empty.ItemCount)
	assert.Equal(t, 0, *empty.ItemCount)
	require.NotNil(t, empty.ByType)
	assert.Empty(t, empty.ByType)

	counted := directoryItemCounts(true, map[ItemType]int{
		ItemTypePassword: 2,
		ItemTypeCard:     0,
		ItemType(77):     3,
		ItemTypeSSHKey:   -1,
	})
	require.NotNil(t, counted.ItemCount)
	assert.Equal(t, 5, *counted.ItemCount)
	assert.Equal(t, map[string]int{"password": 2, "unknown": 3}, counted.ByType)
}

func TestNewAdminDirectoryUser_NilUser(t *testing.T) {
	t.Parallel()

	dto := NewAdminDirectoryUser(nil, nil, nil, AdminDirectorySignals{SignInCount: 4})
	assert.Equal(t, AdminDirectorySubscriptionUnknown, dto.Subscription.Status)
	assert.Equal(t, 0, dto.SignInCount)

	encoded, err := json.Marshal(dto)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "password")
}

func TestNewAdminDirectoryUser_UsageNullsAndZeros(t *testing.T) {
	t.Parallel()

	user := &User{ID: 3, Email: "a@example.com", IsVerified: true}
	active := time.Date(2026, 1, 2, 3, 4, 5, 0, time.FixedZone("UTC+1", 60*60))
	dto := NewAdminDirectoryUser(user, nil, nil, AdminDirectorySignals{
		LastActivityAt:    &active,
		OrganizationCount: -2,
		CollectionCount:   1,
	})
	assert.True(t, dto.EmailVerified)
	require.NotNil(t, dto.LastActivityAt)
	assert.True(t, dto.LastActivityAt.Equal(active.UTC()))
	require.NotNil(t, dto.OrganizationItems.ItemCount)
	assert.Equal(t, 0, *dto.OrganizationItems.ItemCount)
	assert.Empty(t, dto.OrganizationItems.ByType)
	assert.Equal(t, 0, dto.OrganizationCount)
	assert.Equal(t, 1, dto.CollectionCount)
	assert.Nil(t, dto.LastLoginAt)
}
