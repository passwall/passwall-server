package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/email"
	"github.com/passwall/passwall-server/internal/repository/gormrepo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestReferralInvitations(t *testing.T) {
	name := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&domain.Role{}, &domain.Permission{}, &domain.User{}, &domain.Invitation{}))
	require.NoError(t, db.Create(&domain.User{UUID: uuid.New(), Email: "member@example.com", RoleID: 2}).Error)

	builder, err := email.NewEmailBuilder("https://vault.example.test", "hello@passwall.io")
	require.NoError(t, err)
	sender := &inviteCapturingSender{}
	repo := gormrepo.NewInvitationRepository(db)
	svc := NewInvitationService(repo, gormrepo.NewUserRepository(db), sender, builder, noopLogger{})
	ctx := context.Background()

	_, err = svc.CreateReferral(ctx, "MEMBER@example.com", 1, "Ada")
	requireInvitationCode(t, err, ReferralCodeAlreadyRegistered)

	inv, err := svc.CreateReferral(ctx, " Friend@Example.com ", 1, "Ada")
	require.NoError(t, err)
	assert.Equal(t, "friend@example.com", inv.Email)
	assert.Nil(t, inv.OrganizationID)
	require.Len(t, sender.messages, 1)

	_, err = svc.CreateReferral(ctx, "friend@example.com", 1, "Ada")
	requireInvitationCode(t, err, ReferralCodeExists)

	for i := 0; i < maxReferralsPerDay-1; i++ {
		_, err := svc.CreateReferral(ctx, fmt.Sprintf("p%d@example.com", i), 1, "Ada")
		require.NoError(t, err)
	}
	_, err = svc.CreateReferral(ctx, "one-more@example.com", 1, "Ada")
	requireInvitationCode(t, err, ReferralCodeLimit)

	require.NoError(t, repo.MarkUsedByEmail(ctx, "FRIEND@example.com", time.Now()))
	sent, err := svc.ListSentReferrals(ctx, 1)
	require.NoError(t, err)
	joined := 0
	for _, s := range sent {
		if s.IsUsed() {
			joined++
		}
	}
	assert.Equal(t, 1, joined)
}
