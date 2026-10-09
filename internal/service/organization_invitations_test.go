package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/email"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/repository/gormrepo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// noLockInvitationRepo skips SELECT ... FOR UPDATE, which SQLite lacks.
type noLockInvitationRepo struct {
	repository.OrganizationInvitationRepository
}

func (noLockInvitationRepo) LockOrganization(context.Context, uint) error { return nil }

type inviteCapturingSender struct{ messages []*email.EmailMessage }

func (s *inviteCapturingSender) Send(_ context.Context, m *email.EmailMessage) error {
	s.messages = append(s.messages, m)
	return nil
}
func (s *inviteCapturingSender) Provider() email.Provider { return email.ProviderSMTP }
func (s *inviteCapturingSender) Close() error             { return nil }

type inviteFixture struct {
	db      *gorm.DB
	svc     *organizationService
	invRepo repository.OrganizationInvitationRepository
	sender  *inviteCapturingSender
	org     *domain.Organization
	owner   *domain.User
}

func newInviteFixture(t *testing.T) *inviteFixture {
	t.Helper()
	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(
		&domain.Role{}, &domain.Permission{}, &domain.User{}, &domain.Organization{},
		&domain.OrganizationUser{}, &domain.OrganizationInvitation{}, &domain.Team{},
		&domain.TeamUser{}, &domain.Collection{}, &domain.CollectionTeam{}, &domain.Preference{},
	))

	owner := &domain.User{UUID: uuid.New(), Email: "owner@example.com", Name: "Olivia Owner", RoleID: 2}
	require.NoError(t, db.Create(owner).Error)
	org := &domain.Organization{UUID: uuid.New(), PublicID: "orgpublic001", Name: "Acme"}
	require.NoError(t, db.Create(org).Error)
	require.NoError(t, db.Create(&domain.OrganizationUser{
		UUID: uuid.New(), OrganizationID: org.ID, UserID: owner.ID, Role: domain.OrgRoleOwner,
		Status: domain.OrgUserStatusConfirmed, EncryptedOrgKey: "2.owner",
	}).Error)

	builder, err := email.NewEmailBuilder("https://vault.example.test", "hello@passwall.io")
	require.NoError(t, err)
	sender := &inviteCapturingSender{}
	invRepo := noLockInvitationRepo{gormrepo.NewOrganizationInvitationRepository(db)}
	svc := &organizationService{
		orgRepo:            gormrepo.NewOrganizationRepository(db),
		orgUserRepo:        gormrepo.NewOrganizationUserRepository(db),
		userRepo:           gormrepo.NewUserRepository(db),
		teamRepo:           gormrepo.NewTeamRepository(db),
		teamUserRepo:       gormrepo.NewTeamUserRepository(db),
		collectionRepo:     gormrepo.NewCollectionRepository(db),
		collectionTeamRepo: gormrepo.NewCollectionTeamRepository(db),
		logger:             noopLogger{},
		invites: &OrgInvitationDeps{
			Invitations:  invRepo,
			Preferences:  gormrepo.NewPreferencesRepository(db),
			TxManager:    gormrepo.NewTxManager(db),
			EmailSender:  sender,
			EmailBuilder: builder,
		},
	}
	return &inviteFixture{db: db, svc: svc, invRepo: invRepo, sender: sender, org: org, owner: owner}
}

func (f *inviteFixture) addUser(t *testing.T, emailAddr string) *domain.User {
	t.Helper()
	u := &domain.User{UUID: uuid.New(), Email: emailAddr, Name: strings.Split(emailAddr, "@")[0], RoleID: 2}
	require.NoError(t, f.db.Create(u).Error)
	return u
}

func requireInvitationCode(t *testing.T, err error, code string) {
	t.Helper()
	var typed *InvitationError
	require.Truef(t, errors.As(err, &typed), "error %v is not an InvitationError", err)
	assert.Equal(t, code, typed.Code)
}

func TestInviteMemberValidation(t *testing.T) {
	f := newInviteFixture(t)
	ctx := context.Background()
	registered := f.addUser(t, "bob@example.com")

	// A registered invitee without a key pair still gets an invitation; an
	// admin confirms them after they accept.
	keyless, err := f.svc.InviteMember(ctx, f.org.ID, f.owner.ID, &domain.CreateOrgInvitationRequest{Email: "bob@example.com", Role: domain.OrgRoleMember})
	require.NoError(t, err)
	assert.False(t, keyless.Invitation.HasOrgKey())
	assert.Contains(t, f.sender.messages[0].Body, "https://vault.example.test/invitations", "registered invitee gets the in-app link")
	_, err = f.svc.RevokeInvitation(ctx, f.org.ID, keyless.Invitation.ID, f.owner.ID)
	require.NoError(t, err)
	f.sender.messages = nil

	_, err = f.svc.InviteMember(ctx, f.org.ID, f.owner.ID, &domain.CreateOrgInvitationRequest{Email: "OWNER@example.com", Role: domain.OrgRoleMember})
	requireInvitationCode(t, err, InvitationCodeInviteSelf)

	res, err := f.svc.InviteMember(ctx, f.org.ID, f.owner.ID, &domain.CreateOrgInvitationRequest{Email: " New.Person@Example.com ", Role: domain.OrgRoleManager})
	require.NoError(t, err)
	assert.Equal(t, "new.person@example.com", res.Invitation.Email)
	assert.False(t, res.Invitation.HasOrgKey())
	assert.True(t, res.EmailSent)
	require.Len(t, f.sender.messages, 1)
	assert.Contains(t, f.sender.messages[0].Body, "/sign-up?email=new.person%40example.com")
	assert.Contains(t, f.sender.messages[0].Subject, "Acme")

	_, err = f.svc.InviteMember(ctx, f.org.ID, f.owner.ID, &domain.CreateOrgInvitationRequest{Email: "new.person@example.com", Role: domain.OrgRoleMember})
	requireInvitationCode(t, err, InvitationCodeExists)

	require.NoError(t, f.db.Create(&domain.OrganizationUser{
		UUID: uuid.New(), OrganizationID: f.org.ID, UserID: registered.ID, Role: domain.OrgRoleMember,
		Status: domain.OrgUserStatusConfirmed, EncryptedOrgKey: "2.k",
	}).Error)
	_, err = f.svc.InviteMember(ctx, f.org.ID, f.owner.ID, &domain.CreateOrgInvitationRequest{Email: "bob@example.com", Role: domain.OrgRoleMember, EncryptedOrgKey: "4.rsa"})
	requireInvitationCode(t, err, InvitationCodeAlreadyMember)

	// No membership row is created for a pending invitation.
	var count int64
	require.NoError(t, f.db.Model(&domain.OrganizationUser{}).Where("organization_id = ?", f.org.ID).Count(&count).Error)
	assert.EqualValues(t, 2, count)
}

func TestInvitationUsesOrganizationExpirySetting(t *testing.T) {
	f := newInviteFixture(t)
	require.NoError(t, f.db.Create(&domain.Preference{
		OwnerType: domain.OrgSettingOwnerType, OwnerID: f.org.ID, Section: domain.OrgSettingSectionMembers,
		Key: domain.OrgSettingKeyInvitationExpiryDays, Value: "3", Type: "number",
	}).Error)
	res, err := f.svc.InviteMember(context.Background(), f.org.ID, f.owner.ID, &domain.CreateOrgInvitationRequest{Email: "a@example.com", Role: domain.OrgRoleMember})
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now().Add(72*time.Hour), res.Invitation.ExpiresAt, time.Minute)
}

func TestAcceptInvitationWithSharedKey(t *testing.T) {
	f := newInviteFixture(t)
	ctx := context.Background()
	bob := f.addUser(t, "bob@example.com")
	eve := f.addUser(t, "eve@example.com")

	res, err := f.svc.InviteMember(ctx, f.org.ID, f.owner.ID, &domain.CreateOrgInvitationRequest{Email: "bob@example.com", Role: domain.OrgRoleMember, EncryptedOrgKey: "4.rsaWrapped"})
	require.NoError(t, err)
	assert.Contains(t, f.sender.messages[0].Body, "https://vault.example.test/invitations")

	received, err := f.svc.ListReceivedInvitations(ctx, bob.ID)
	require.NoError(t, err)
	require.Len(t, received, 1)
	dto := domain.ToReceivedInvitationDTO(received[0])
	assert.Equal(t, "Acme", dto.Organization.Name)
	assert.Equal(t, "orgpublic001", dto.Organization.PublicID)
	assert.Equal(t, "Olivia Owner", dto.InvitedBy.Name)
	assert.False(t, dto.RequiresAdminConfirmation)

	_, err = f.svc.AcceptReceivedInvitation(ctx, res.Invitation.ID, eve.ID, "2.x")
	requireInvitationCode(t, err, InvitationCodeNotFound)

	_, err = f.svc.AcceptReceivedInvitation(ctx, res.Invitation.ID, bob.ID, "")
	requireInvitationCode(t, err, InvitationCodeOrgKeyRequired)
	_, err = f.svc.AcceptReceivedInvitation(ctx, res.Invitation.ID, bob.ID, "4.stillRsa")
	requireInvitationCode(t, err, InvitationCodeInvalidOrgKey)

	member, err := f.svc.AcceptReceivedInvitation(ctx, res.Invitation.ID, bob.ID, "2.userWrapped")
	require.NoError(t, err)
	assert.Equal(t, domain.OrgUserStatusAccepted, member.Status)
	assert.Equal(t, "2.userWrapped", member.EncryptedOrgKey)

	var teamMemberships int64
	require.NoError(t, f.db.Model(&domain.TeamUser{}).Where("organization_user_id = ?", member.ID).Count(&teamMemberships).Error)
	assert.EqualValues(t, 1, teamMemberships, "accepted member joins the default team")

	stored, err := f.invRepo.GetByID(ctx, res.Invitation.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.OrgInvitationAccepted, stored.Status)
	require.NotNil(t, stored.AcceptedUserID)
	assert.Equal(t, bob.ID, *stored.AcceptedUserID)

	_, err = f.svc.AcceptReceivedInvitation(ctx, res.Invitation.ID, bob.ID, "2.userWrapped")
	requireInvitationCode(t, err, InvitationCodeNotPending)
}

func TestAcceptInvitationWithoutKeyNeedsAdminConfirmation(t *testing.T) {
	f := newInviteFixture(t)
	ctx := context.Background()
	res, err := f.svc.InviteMember(ctx, f.org.ID, f.owner.ID, &domain.CreateOrgInvitationRequest{Email: "newbie@example.com", Role: domain.OrgRoleMember})
	require.NoError(t, err)

	newbie := f.addUser(t, "Newbie@Example.com") // signs up later, different casing
	member, err := f.svc.AcceptReceivedInvitation(ctx, res.Invitation.ID, newbie.ID, "")
	require.NoError(t, err)
	assert.Equal(t, domain.OrgUserStatusProvisioned, member.Status)

	require.Len(t, f.sender.messages, 2, "invitation + admin notification")
	notice := f.sender.messages[1]
	assert.Equal(t, "owner@example.com", notice.To)
	assert.Contains(t, notice.Body, "/organizations/orgpublic001/members")
}

func TestDeclineRevokeAndExpiry(t *testing.T) {
	f := newInviteFixture(t)
	ctx := context.Background()
	bob := f.addUser(t, "bob@example.com")

	declined, err := f.svc.InviteMember(ctx, f.org.ID, f.owner.ID, &domain.CreateOrgInvitationRequest{Email: "bob@example.com", Role: domain.OrgRoleMember, EncryptedOrgKey: "4.k"})
	require.NoError(t, err)
	require.NoError(t, f.svc.DeclineReceivedInvitation(ctx, declined.Invitation.ID, bob.ID))
	_, err = f.svc.AcceptReceivedInvitation(ctx, declined.Invitation.ID, bob.ID, "2.k")
	requireInvitationCode(t, err, InvitationCodeNotPending)

	// A declined invitation does not block a new one.
	revoked, err := f.svc.InviteMember(ctx, f.org.ID, f.owner.ID, &domain.CreateOrgInvitationRequest{Email: "bob@example.com", Role: domain.OrgRoleMember, EncryptedOrgKey: "4.k"})
	require.NoError(t, err)
	_, err = f.svc.RevokeInvitation(ctx, f.org.ID, revoked.Invitation.ID, f.owner.ID)
	require.NoError(t, err)
	_, err = f.svc.RevokeInvitation(ctx, f.org.ID, revoked.Invitation.ID, f.owner.ID)
	requireInvitationCode(t, err, InvitationCodeNotPending)
	received, err := f.svc.ListReceivedInvitations(ctx, bob.ID)
	require.NoError(t, err)
	assert.Empty(t, received)

	_, err = f.svc.RevokeInvitation(ctx, f.org.ID, revoked.Invitation.ID, bob.ID)
	assert.ErrorIs(t, err, repository.ErrForbidden, "non-members cannot manage invitations")

	expired, err := f.svc.InviteMember(ctx, f.org.ID, f.owner.ID, &domain.CreateOrgInvitationRequest{Email: "late@example.com", Role: domain.OrgRoleMember})
	require.NoError(t, err)
	require.NoError(t, f.db.Model(&domain.OrganizationInvitation{}).Where("id = ?", expired.Invitation.ID).
		Updates(map[string]interface{}{"expires_at": time.Now().Add(-time.Hour), "last_sent_at": time.Now().Add(-2 * time.Hour)}).Error)
	late := f.addUser(t, "late@example.com")
	_, err = f.svc.AcceptReceivedInvitation(ctx, expired.Invitation.ID, late.ID, "")
	requireInvitationCode(t, err, InvitationCodeExpired)

	n, err := f.svc.ExpireOverdueInvitations(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 1, n)

	resent, err := f.svc.ResendInvitation(ctx, f.org.ID, expired.Invitation.ID, f.owner.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.OrgInvitationPending, resent.Invitation.Status)
	assert.True(t, resent.Invitation.ExpiresAt.After(time.Now().Add(6*24*time.Hour)))
	assert.Equal(t, 2, resent.Invitation.SendCount)
	_, err = f.svc.ResendInvitation(ctx, f.org.ID, expired.Invitation.ID, f.owner.ID)
	requireInvitationCode(t, err, InvitationCodeResendTooSoon)

	member, err := f.svc.AcceptReceivedInvitation(ctx, expired.Invitation.ID, late.ID, "")
	require.NoError(t, err)
	assert.Equal(t, domain.OrgUserStatusProvisioned, member.Status)
}
