package gormrepo

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openStatsDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(
		&domain.UserActivity{},
		&domain.Token{},
		&domain.Organization{},
		&domain.OrganizationUser{},
		&domain.Collection{},
		&domain.OrganizationItem{},
	))
	return db
}

func TestAdminDirectoryStats_LoadAccountSignals(t *testing.T) {
	t.Parallel()

	db := openStatsDB(t)
	repo := NewAdminDirectoryStatsRepository(db)
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	cutoff := now.Add(-domain.UserActivityRetention)
	inside := cutoff.Add(time.Minute)
	outside := cutoff.Add(-time.Second)
	newest := inside.Add(2 * time.Hour)

	require.NoError(t, db.Create(&domain.UserActivity{UserID: 7, ActivityType: domain.ActivityTypeSignIn, CreatedAt: inside, IPAddress: "203.0.113.9", UserAgent: "secret-agent"}).Error)
	require.NoError(t, db.Create(&domain.UserActivity{UserID: 7, ActivityType: domain.ActivityTypeSignIn, CreatedAt: outside}).Error)
	require.NoError(t, db.Create(&domain.UserActivity{UserID: 7, ActivityType: domain.ActivityTypeItemUpdated, CreatedAt: inside.Add(time.Hour), Details: `{"title":"do-not-leak"}`}).Error)
	require.NoError(t, db.Create(&domain.UserActivity{UserID: 7, ActivityType: domain.ActivityTypeFailedSignIn, CreatedAt: newest}).Error)
	require.NoError(t, db.Create(&domain.UserActivity{UserID: 8, ActivityType: domain.ActivityTypeSignIn, CreatedAt: inside}).Error)

	deviceA := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	deviceB := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	deviceC := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	require.NoError(t, db.Create(tokenRow(7, deviceA, "vault", now.Add(time.Hour), "access-token-do-not-leak")).Error)
	require.NoError(t, db.Create(tokenRow(7, deviceA, "vault", now.Add(time.Hour), "refresh-token-do-not-leak")).Error)
	require.NoError(t, db.Create(tokenRow(7, deviceB, "extension", now.Add(time.Hour), "extension-token-do-not-leak")).Error)
	require.NoError(t, db.Create(tokenRow(7, uuid.Nil, "mobile", now.Add(time.Hour), "mobile-token-do-not-leak")).Error)
	require.NoError(t, db.Create(tokenRow(7, deviceC, "desktop", now.Add(-time.Hour), "expired-token-do-not-leak")).Error)
	require.NoError(t, db.Create(tokenRow(8, deviceC, "desktop", now.Add(time.Hour), "other-user-token")).Error)

	orgLive := organizationRow(1, "Personal Vault secret name", nil)
	orgInvited := organizationRow(2, "Invited Org", nil)
	deletedAt := now
	orgDeleted := organizationRow(3, "Deleted Org", &deletedAt)
	orgSuspended := organizationRow(4, "Suspended Org", nil)
	orgOther := organizationRow(5, "Other Org", nil)
	require.NoError(t, db.Create(orgLive).Error)
	require.NoError(t, db.Create(orgInvited).Error)
	require.NoError(t, db.Create(orgDeleted).Error)
	require.NoError(t, db.Create(orgSuspended).Error)
	require.NoError(t, db.Create(orgOther).Error)

	require.NoError(t, db.Create(membershipRow(1, 7, domain.OrgUserStatusConfirmed)).Error)
	require.NoError(t, db.Create(membershipRow(2, 7, domain.OrgUserStatusInvited)).Error)
	require.NoError(t, db.Create(membershipRow(3, 7, domain.OrgUserStatusAccepted)).Error)
	require.NoError(t, db.Create(membershipRow(4, 7, domain.OrgUserStatusSuspended)).Error)
	require.NoError(t, db.Create(membershipRow(5, 8, domain.OrgUserStatusConfirmed)).Error)

	require.NoError(t, db.Create(collectionRow(1, 1, "Tax Documents", nil)).Error)
	require.NoError(t, db.Create(collectionRow(2, 1, "Deleted Collection", &deletedAt)).Error)
	require.NoError(t, db.Create(collectionRow(3, 2, "Invited Collection", nil)).Error)
	require.NoError(t, db.Create(collectionRow(4, 3, "Deleted Org Collection", nil)).Error)
	require.NoError(t, db.Create(collectionRow(5, 4, "Suspended Collection", nil)).Error)

	insertOrgItem(t, db, 7, domain.ItemTypeCard, nil, "ciphertext-do-not-leak", "Visa title do not leak")
	insertOrgItem(t, db, 7, domain.ItemTypePassword, &deletedAt, "deleted-ciphertext", "deleted title")
	insertOrgItem(t, db, 8, domain.ItemTypeSecureNote, nil, "other-ciphertext", "other title")

	signals, err := repo.LoadAccountSignals(context.Background(), []*domain.User{
		{ID: 7},
		{ID: 8},
		{ID: 9},
		nil,
	}, now)
	require.NoError(t, err)

	user7 := signals[7]
	require.NotNil(t, user7.LastActivityAt)
	assert.True(t, user7.LastActivityAt.Equal(newest))
	assert.Equal(t, 1, user7.SignInCount)
	assert.Equal(t, 3, user7.ActivityCount)
	assert.Equal(t, map[domain.ItemType]int{domain.ItemTypeCard: 1}, user7.OrgItemsByType)
	assert.Equal(t, 2, user7.OrganizationCount)
	assert.Equal(t, 2, user7.CollectionCount)
	assert.Equal(t, 2, user7.DeviceCount)
	assert.Equal(t, 3, user7.ClientCount)

	user8 := signals[8]
	assert.Nil(t, user8.LastActivityAt)
	assert.Equal(t, 1, user8.SignInCount)
	assert.Equal(t, 1, user8.ActivityCount)
	assert.Equal(t, map[domain.ItemType]int{domain.ItemTypeSecureNote: 1}, user8.OrgItemsByType)
	assert.Equal(t, 1, user8.OrganizationCount)
	assert.Equal(t, 0, user8.CollectionCount)
	assert.Equal(t, 1, user8.DeviceCount)
	assert.Equal(t, 1, user8.ClientCount)

	user9 := signals[9]
	assert.Equal(t, 0, user9.ActivityCount)
	assert.Equal(t, 0, user9.OrganizationCount)

	encoded, err := json.Marshal(signals)
	require.NoError(t, err)
	body := string(encoded)
	for _, secret := range []string{
		"203.0.113.9",
		"secret-agent",
		"do-not-leak",
		"ciphertext-do-not-leak",
		"Visa title",
		"Tax Documents",
		"Personal Vault",
		deviceA.String(),
		deviceB.String(),
		"access-token",
		"vault",
		"extension",
		"mobile",
		"desktop",
	} {
		assert.NotContains(t, body, secret)
	}
}

func TestAdminDirectoryStats_EmptyInput(t *testing.T) {
	t.Parallel()

	db := openStatsDB(t)
	repo := NewAdminDirectoryStatsRepository(db)
	signals, err := repo.LoadAccountSignals(context.Background(), nil, time.Time{})
	require.NoError(t, err)
	assert.Empty(t, signals)
}

func tokenRow(userID int, deviceID uuid.UUID, app string, expiry time.Time, token string) *domain.Token {
	return &domain.Token{
		UserID:      userID,
		UUID:        uuid.New(),
		SessionUUID: uuid.New(),
		DeviceID:    deviceID,
		App:         app,
		Kind:        "access",
		Token:       token,
		ExpiryTime:  expiry,
	}
}

func organizationRow(id uint, name string, deletedAt *time.Time) *domain.Organization {
	return &domain.Organization{
		ID:              id,
		UUID:            uuid.New(),
		Name:            name,
		BillingEmail:    "billing@example.com",
		EncryptedOrgKey: "org-key-do-not-leak",
		Status:          domain.OrgStatusActive,
		IsActive:        true,
		DeletedAt:       deletedAt,
	}
}

func membershipRow(orgID, userID uint, status domain.OrganizationUserStatus) *domain.OrganizationUser {
	return &domain.OrganizationUser{
		UUID:            uuid.New(),
		OrganizationID:  orgID,
		UserID:          userID,
		Role:            domain.OrgRoleMember,
		EncryptedOrgKey: "wrapped-key-do-not-leak",
		Status:          status,
	}
}

func collectionRow(id, orgID uint, name string, deletedAt *time.Time) *domain.Collection {
	return &domain.Collection{
		ID:             id,
		UUID:           uuid.New(),
		OrganizationID: orgID,
		Name:           name,
		DeletedAt:      deletedAt,
	}
}

func insertOrgItem(t *testing.T, db *gorm.DB, userID uint, itemType domain.ItemType, deletedAt *time.Time, data, title string) {
	t.Helper()
	metadata := `{"name":"` + title + `","uri_hint":"https://secret.example/vault"}`
	err := db.Exec(`INSERT INTO organization_items
		(uuid, support_id, created_at, updated_at, deleted_at, organization_id, item_type, data, metadata, created_by_user_id, revision, sync_version, is_favorite, reprompt, auto_fill, auto_login)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 1, 0, 0, 1, 0)`,
		uuid.New().String(),
		int64(userID)*1000+int64(itemType),
		time.Now().UTC(),
		time.Now().UTC(),
		deletedAt,
		1,
		int16(itemType),
		data,
		metadata,
		userID,
	).Error
	require.NoError(t, err)
}
