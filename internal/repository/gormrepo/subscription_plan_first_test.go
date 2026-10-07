package gormrepo

import (
	"context"
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

func openSubscriptionDB(t *testing.T) *gorm.DB {
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
	require.NoError(t, db.AutoMigrate(&domain.Organization{}, &domain.Subscription{}))
	return db
}

func createPlanFirstOrg(t *testing.T, db *gorm.DB, id uint, personal bool, ownerID uint, createdAt time.Time) {
	t.Helper()
	org := &domain.Organization{
		ID:           id,
		UUID:         uuid.New(),
		Name:         "Org",
		BillingEmail: "billing@example.com",
		IsPersonal:   personal,
		CreatedAt:    createdAt,
	}
	if personal {
		org.PersonalOwnerUserID = &ownerID
	} else {
		org.CreatedByUserID = &ownerID
	}
	require.NoError(t, db.Create(org).Error)
}

func createSubscriptionRow(t *testing.T, db *gorm.DB, orgID uint, state domain.SubscriptionState, stripeID *string) {
	t.Helper()
	require.NoError(t, db.Create(&domain.Subscription{
		UUID:                 uuid.New(),
		OrganizationID:       orgID,
		PlanID:               1,
		State:                state,
		StripeSubscriptionID: stripeID,
	}).Error)
}

func TestSubscriptionRepository_HasPaidHistoryForUser(t *testing.T) {
	db := openSubscriptionDB(t)
	repo := NewSubscriptionRepository(db)
	ctx := context.Background()
	now := time.Now()

	stripeID := "sub_123"
	createPlanFirstOrg(t, db, 1, false, 7, now)
	createSubscriptionRow(t, db, 1, domain.SubStateCanceled, &stripeID)
	createPlanFirstOrg(t, db, 2, true, 8, now)
	createSubscriptionRow(t, db, 2, domain.SubStateActive, nil)

	used, err := repo.HasPaidHistoryForUser(ctx, 7, false)
	require.NoError(t, err)
	assert.True(t, used, "canceled Stripe subscription on a shared org counts as history")

	used, err = repo.HasPaidHistoryForUser(ctx, 7, true)
	require.NoError(t, err)
	assert.False(t, used, "shared history must not block a personal trial")

	used, err = repo.HasPaidHistoryForUser(ctx, 8, true)
	require.NoError(t, err)
	assert.False(t, used, "free subscription without Stripe ID is not paid history")
}

func TestSubscriptionRepository_ListAbandonedDraftOrganizationIDs(t *testing.T) {
	db := openSubscriptionDB(t)
	repo := NewSubscriptionRepository(db)
	now := time.Now()
	old := now.Add(-10 * 24 * time.Hour)

	createPlanFirstOrg(t, db, 1, false, 7, old) // abandoned draft
	createSubscriptionRow(t, db, 1, domain.SubStateDraft, nil)

	createPlanFirstOrg(t, db, 2, false, 7, now) // draft, too recent
	createSubscriptionRow(t, db, 2, domain.SubStateDraft, nil)

	stripeID := "sub_456"
	createPlanFirstOrg(t, db, 3, false, 7, old) // completed checkout
	createSubscriptionRow(t, db, 3, domain.SubStateDraft, nil)
	createSubscriptionRow(t, db, 3, domain.SubStateTrialing, &stripeID)

	createPlanFirstOrg(t, db, 4, false, 7, old) // legacy free org
	createSubscriptionRow(t, db, 4, domain.SubStateActive, nil)

	createPlanFirstOrg(t, db, 5, false, 7, old) // already deleted
	createSubscriptionRow(t, db, 5, domain.SubStateDraft, nil)
	require.NoError(t, db.Model(&domain.Organization{}).Where("id = ?", 5).Update("deleted_at", now).Error)

	ids, err := repo.ListAbandonedDraftOrganizationIDs(context.Background(), now.Add(-7*24*time.Hour))
	require.NoError(t, err)
	assert.Equal(t, []uint{1}, ids)
}
