package gormrepo

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestUserActivityRepository_KeepsAdminAudit(t *testing.T) {
	name := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&domain.UserActivity{}))
	repo := NewUserActivityRepository(db)
	ctx := context.Background()
	old := time.Now().Add(-200 * 24 * time.Hour)

	for _, a := range []*domain.UserActivity{
		{UserID: 1, ActivityType: domain.ActivityTypeSignIn, CreatedAt: old},
		{UserID: 1, ActivityType: domain.ActivityTypeAdminUserDeleted, CreatedAt: old},
		{UserID: 2, ActivityType: domain.ActivityTypeSignIn},
		{UserID: 2, ActivityType: domain.ActivityTypeAdminMailSent},
		// "admin" without the underscore must not be mistaken for an audit row.
		{UserID: 2, ActivityType: domain.ActivityType("adminish"), CreatedAt: old},
	} {
		require.NoError(t, db.Create(a).Error)
	}

	deleted, err := repo.DeleteOldActivities(ctx, 90*24*time.Hour)
	require.NoError(t, err)
	assert.EqualValues(t, 2, deleted, "old sign-in and look-alike type removed, admin audit kept")

	require.NoError(t, repo.DeleteByUserID(ctx, 2))
	var remaining []domain.UserActivity
	require.NoError(t, db.Order("id").Find(&remaining).Error)
	types := make([]domain.ActivityType, 0, len(remaining))
	for _, a := range remaining {
		types = append(types, a.ActivityType)
	}
	assert.ElementsMatch(t, []domain.ActivityType{domain.ActivityTypeAdminUserDeleted, domain.ActivityTypeAdminMailSent}, types)
}
