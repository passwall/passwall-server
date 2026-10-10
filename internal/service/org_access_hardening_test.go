package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/repository/gormrepo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// accessFixture extends the invitation fixture with a second organization,
// teams, collections and the services that govern access to them.
type accessFixture struct {
	*inviteFixture
	teams       TeamService
	collections CollectionService
	shares      ItemShareService
	otherOrg    *domain.Organization
}

func newAccessFixture(t *testing.T) *accessFixture {
	t.Helper()
	f := newInviteFixture(t)
	require.NoError(t, f.db.AutoMigrate(&domain.CollectionUser{}, &domain.OrganizationItem{}, &domain.ItemShare{}))

	other := &domain.Organization{UUID: uuid.New(), PublicID: "orgpublic002", Name: "Other"}
	require.NoError(t, f.db.Create(other).Error)

	orgUserRepo := gormrepo.NewOrganizationUserRepository(f.db)
	teamUserRepo := gormrepo.NewTeamUserRepository(f.db)
	collectionUserRepo := gormrepo.NewCollectionUserRepository(f.db)
	collectionTeamRepo := gormrepo.NewCollectionTeamRepository(f.db)
	itemRepo := gormrepo.NewOrganizationItemRepository(f.db)

	return &accessFixture{
		inviteFixture: f,
		otherOrg:      other,
		teams: NewTeamService(
			gormrepo.NewTeamRepository(f.db), teamUserRepo, orgUserRepo,
			gormrepo.NewOrganizationRepository(f.db), noopLogger{},
		),
		collections: NewCollectionService(
			gormrepo.NewCollectionRepository(f.db), collectionUserRepo, collectionTeamRepo,
			orgUserRepo, gormrepo.NewTeamRepository(f.db), teamUserRepo,
			gormrepo.NewOrganizationRepository(f.db), itemRepo, nil, noopLogger{},
		),
		shares: NewItemShareService(
			gormrepo.NewItemShareRepository(f.db), itemRepo, orgUserRepo,
			collectionUserRepo, collectionTeamRepo, teamUserRepo,
			gormrepo.NewUserRepository(f.db), nil, nil, noopLogger{},
		),
	}
}

func (f *accessFixture) member(t *testing.T, org *domain.Organization, emailAddr string, role domain.OrganizationRole, status domain.OrganizationUserStatus) (*domain.User, *domain.OrganizationUser) {
	t.Helper()
	u := f.addUser(t, emailAddr)
	ou := &domain.OrganizationUser{
		UUID: uuid.New(), OrganizationID: org.ID, UserID: u.ID, Role: role,
		Status: status, EncryptedOrgKey: "2.key",
	}
	require.NoError(t, f.db.Create(ou).Error)
	return u, ou
}

func (f *accessFixture) team(t *testing.T, org *domain.Organization, name string, members ...*domain.OrganizationUser) *domain.Team {
	t.Helper()
	team := &domain.Team{UUID: uuid.New(), OrganizationID: org.ID, Name: name}
	require.NoError(t, f.db.Create(team).Error)
	for _, m := range members {
		require.NoError(t, f.db.Create(&domain.TeamUser{TeamID: team.ID, OrganizationUserID: m.ID}).Error)
	}
	return team
}

func (f *accessFixture) collection(t *testing.T, org *domain.Organization, name string) *domain.Collection {
	t.Helper()
	c := &domain.Collection{UUID: uuid.New(), OrganizationID: org.ID, Name: name}
	require.NoError(t, f.db.Create(c).Error)
	return c
}

func (f *accessFixture) grant(t *testing.T, c *domain.Collection, ou *domain.OrganizationUser, write, admin, hide bool) {
	t.Helper()
	require.NoError(t, f.db.Create(&domain.CollectionUser{
		CollectionID: c.ID, OrganizationUserID: ou.ID,
		CanRead: true, CanWrite: write, CanAdmin: admin, HidePasswords: hide,
	}).Error)
}

func TestUpdateMemberRoleIsScopedToOrganization(t *testing.T) {
	f := newAccessFixture(t)
	ctx := context.Background()
	_, foreign := f.member(t, f.otherOrg, "foreign@example.com", domain.OrgRoleMember, domain.OrgUserStatusConfirmed)

	accessAll := true
	err := f.svc.UpdateMemberRole(ctx, f.org.ID, foreign.ID, f.owner.ID, &domain.UpdateOrgUserRoleRequest{
		Role: domain.OrgRoleAdmin, AccessAll: &accessAll,
	})
	require.ErrorIs(t, err, repository.ErrNotFound)

	var reloaded domain.OrganizationUser
	require.NoError(t, f.db.First(&reloaded, foreign.ID).Error)
	assert.Equal(t, domain.OrgRoleMember, reloaded.Role)
	assert.False(t, reloaded.AccessAll)
}

func TestSuspendedAdminLosesAdminActions(t *testing.T) {
	f := newAccessFixture(t)
	ctx := context.Background()
	admin, _ := f.member(t, f.org, "admin@example.com", domain.OrgRoleAdmin, domain.OrgUserStatusSuspended)
	_, target := f.member(t, f.org, "target@example.com", domain.OrgRoleMember, domain.OrgUserStatusConfirmed)

	err := f.svc.UpdateMemberRole(ctx, f.org.ID, target.ID, admin.ID, &domain.UpdateOrgUserRoleRequest{Role: domain.OrgRoleAdmin})
	require.ErrorIs(t, err, repository.ErrForbidden)
}

func TestTeamMemberChangesAreScopedToTeam(t *testing.T) {
	f := newAccessFixture(t)
	ctx := context.Background()
	_, ownerMembership := f.member(t, f.org, "teamadmin@example.com", domain.OrgRoleAdmin, domain.OrgUserStatusConfirmed)
	_ = ownerMembership
	_, foreignMember := f.member(t, f.otherOrg, "foreign@example.com", domain.OrgRoleMember, domain.OrgUserStatusConfirmed)
	ownTeam := f.team(t, f.org, "Own")
	foreignTeam := f.team(t, f.otherOrg, "Foreign", foreignMember)

	var foreignTeamUser domain.TeamUser
	require.NoError(t, f.db.Where("team_id = ?", foreignTeam.ID).First(&foreignTeamUser).Error)

	err := f.teams.UpdateMember(ctx, ownTeam.ID, foreignTeamUser.ID, f.owner.ID, &domain.UpdateTeamUserRequest{IsManager: true})
	require.ErrorIs(t, err, repository.ErrNotFound)
	err = f.teams.RemoveMember(ctx, ownTeam.ID, foreignTeamUser.ID, f.owner.ID)
	require.ErrorIs(t, err, repository.ErrNotFound)

	var stillThere domain.TeamUser
	require.NoError(t, f.db.First(&stillThere, foreignTeamUser.ID).Error)
	assert.False(t, stillThere.IsManager)
}

func TestTeamManagerCannotAppointManagers(t *testing.T) {
	f := newAccessFixture(t)
	ctx := context.Background()
	manager, managerMembership := f.member(t, f.org, "lead@example.com", domain.OrgRoleMember, domain.OrgUserStatusConfirmed)
	_, newcomer := f.member(t, f.org, "new@example.com", domain.OrgRoleMember, domain.OrgUserStatusConfirmed)
	_, suspended := f.member(t, f.org, "gone@example.com", domain.OrgRoleMember, domain.OrgUserStatusSuspended)
	team := &domain.Team{UUID: uuid.New(), OrganizationID: f.org.ID, Name: "Engineering"}
	require.NoError(t, f.db.Create(team).Error)
	require.NoError(t, f.db.Create(&domain.TeamUser{TeamID: team.ID, OrganizationUserID: managerMembership.ID, IsManager: true}).Error)

	err := f.teams.AddMember(ctx, team.ID, manager.ID, &domain.AddTeamUserRequest{OrganizationUserID: newcomer.ID, IsManager: true})
	require.ErrorIs(t, err, repository.ErrForbidden)

	require.NoError(t, f.teams.AddMember(ctx, team.ID, manager.ID, &domain.AddTeamUserRequest{OrganizationUserID: newcomer.ID}))

	err = f.teams.AddMember(ctx, team.ID, manager.ID, &domain.AddTeamUserRequest{OrganizationUserID: suspended.ID})
	require.Error(t, err)

	var added domain.TeamUser
	require.NoError(t, f.db.Where("team_id = ? AND organization_user_id = ?", team.ID, newcomer.ID).First(&added).Error)
	err = f.teams.UpdateMember(ctx, team.ID, added.ID, manager.ID, &domain.UpdateTeamUserRequest{IsManager: true})
	require.ErrorIs(t, err, repository.ErrForbidden)
}

func TestCollectionManagerCannotEscalate(t *testing.T) {
	f := newAccessFixture(t)
	ctx := context.Background()
	manager, managerMembership := f.member(t, f.org, "manager@example.com", domain.OrgRoleMember, domain.OrgUserStatusConfirmed)
	_, peer := f.member(t, f.org, "peer@example.com", domain.OrgRoleMember, domain.OrgUserStatusConfirmed)
	vault := f.collection(t, f.org, "Production")
	f.grant(t, vault, managerMembership, true, true, true)
	ownTeam := f.team(t, f.org, "Ops", managerMembership)
	otherTeam := f.team(t, f.org, "Finance", peer)

	visible := &domain.GrantCollectionAccessRequest{CanRead: true, CanWrite: true, CanAdmin: true}
	hidden := &domain.GrantCollectionAccessRequest{CanRead: true, HidePasswords: true}

	// Cannot lift their own restrictions, directly or through their team.
	require.ErrorIs(t, f.collections.GrantUserAccess(ctx, vault.ID, managerMembership.ID, manager.ID, visible), repository.ErrForbidden)
	require.ErrorIs(t, f.collections.GrantTeamAccess(ctx, vault.ID, ownTeam.ID, manager.ID, hidden), repository.ErrForbidden)
	// Cannot grant password visibility they do not have.
	require.ErrorIs(t, f.collections.GrantUserAccess(ctx, vault.ID, peer.ID, manager.ID, visible), repository.ErrForbidden)
	require.ErrorIs(t, f.collections.GrantTeamAccess(ctx, vault.ID, otherTeam.ID, manager.ID, visible), repository.ErrForbidden)
	// Can grant within their own level.
	require.NoError(t, f.collections.GrantUserAccess(ctx, vault.ID, peer.ID, manager.ID, hidden))
	require.NoError(t, f.collections.GrantTeamAccess(ctx, vault.ID, otherTeam.ID, manager.ID, hidden))

	// Organization admins are not capped.
	require.NoError(t, f.collections.GrantUserAccess(ctx, vault.ID, peer.ID, f.owner.ID, visible))
}

func TestListForUserIgnoresGrantsWithoutRead(t *testing.T) {
	f := newAccessFixture(t)
	ctx := context.Background()
	user, membership := f.member(t, f.org, "reader@example.com", domain.OrgRoleMember, domain.OrgUserStatusConfirmed)
	readable := f.collection(t, f.org, "Readable")
	unreadable := f.collection(t, f.org, "Unreadable")
	f.grant(t, readable, membership, false, false, false)
	require.NoError(t, f.db.Create(&domain.CollectionUser{CollectionID: unreadable.ID, OrganizationUserID: membership.ID}).Error)
	require.NoError(t, f.db.Model(&domain.CollectionUser{}).
		Where("collection_id = ?", unreadable.ID).Update("can_read", false).Error)

	collections, err := gormrepo.NewCollectionRepository(f.db).ListForUser(ctx, f.org.ID, user.ID)
	require.NoError(t, err)
	require.Len(t, collections, 1)
	assert.Equal(t, readable.ID, collections[0].ID)
}

func TestSharesStayInsideTheOrganization(t *testing.T) {
	f := newAccessFixture(t)
	ctx := context.Background()
	sharer, sharerMembership := f.member(t, f.org, "sharer@example.com", domain.OrgRoleMember, domain.OrgUserStatusConfirmed)
	colleague, _ := f.member(t, f.org, "colleague@example.com", domain.OrgRoleMember, domain.OrgUserStatusConfirmed)
	outsider := f.addUser(t, "outsider@example.com")
	vault := f.collection(t, f.org, "Shared")
	f.grant(t, vault, sharerMembership, true, false, false)
	// Raw insert: SQLite cannot encode the JSONB metadata column.
	itemUUID := uuid.New()
	require.NoError(t, f.db.Exec(
		`INSERT INTO organization_items (uuid, support_id, organization_id, collection_id, item_type, data, metadata, created_by_user_id, revision, created_at, updated_at)
		 VALUES (?, 1, ?, ?, ?, ?, CAST('{}' AS BLOB), ?, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		itemUUID.String(), f.org.ID, vault.ID, domain.ItemTypePassword, "2.cipher", sharer.ID,
	).Error)
	item := &domain.OrganizationItem{UUID: itemUUID}

	_, err := f.shares.Create(ctx, sharer.ID, &CreateItemShareRequest{
		ItemUUID: item.UUID.String(), SharedWithUserID: &outsider.ID, EncryptedKey: "rsa",
	})
	require.ErrorIs(t, err, ErrSecureSharingUnavailable)

	_, err = f.shares.Create(ctx, sharer.ID, &CreateItemShareRequest{
		ItemUUID: item.UUID.String(), SharedWithEmail: "nobody@example.com", EncryptedKey: "rsa",
	})
	require.ErrorIs(t, err, ErrSecureSharingUnavailable)

	created, err := f.shares.Create(ctx, sharer.ID, &CreateItemShareRequest{
		ItemUUID: item.UUID.String(), SharedWithUserID: &colleague.ID, EncryptedKey: "rsa",
	})
	require.NoError(t, err)

	received, err := f.shares.ListReceived(ctx, colleague.ID)
	require.NoError(t, err)
	require.Len(t, received, 1)

	// Once the sharer loses write access, the share stops working.
	require.NoError(t, f.db.Model(&domain.CollectionUser{}).
		Where("organization_user_id = ?", sharerMembership.ID).Update("can_write", false).Error)
	received, err = f.shares.ListReceived(ctx, colleague.ID)
	require.NoError(t, err)
	assert.Empty(t, received)
	_, err = f.shares.GetByUUID(ctx, colleague.ID, created.Share.UUID.String())
	require.ErrorIs(t, err, repository.ErrForbidden)
}

func TestRewrapOwnOrgKey(t *testing.T) {
	f := newAccessFixture(t)
	ctx := context.Background()
	const userWrapped = "2.aXZpdml2aXZpdml2aXZpdg==|Y2lwaGVydGV4dA==|bWFjbWFjbWFj"

	rsaUser, rsaMember := f.member(t, f.org, "rsa@example.com", domain.OrgRoleMember, domain.OrgUserStatusConfirmed)
	require.NoError(t, f.db.Model(rsaMember).Update("encrypted_org_key", "cnNhLXdyYXBwZWQta2V5").Error)

	// Only a well-formed user-key EncString is accepted.
	for _, bad := range []string{"cnNhLXdyYXBwZWQ=", "2.only-one-part", "2.a|b", "2.!!|!!|!!"} {
		require.ErrorIs(t, f.svc.RewrapOwnOrgKey(ctx, f.org.ID, rsaUser.ID, bad), ErrInvalidOrgKeyEncoding, bad)
	}

	require.NoError(t, f.svc.RewrapOwnOrgKey(ctx, f.org.ID, rsaUser.ID, userWrapped))
	var reloaded domain.OrganizationUser
	require.NoError(t, f.db.First(&reloaded, rsaMember.ID).Error)
	assert.Equal(t, userWrapped, reloaded.EncryptedOrgKey)

	// A copy that is already user-wrapped cannot be replaced.
	require.ErrorIs(t, f.svc.RewrapOwnOrgKey(ctx, f.org.ID, rsaUser.ID, userWrapped), ErrOrgKeyAlreadyUserWrapped)

	// Non-members and suspended members are refused.
	stranger := f.addUser(t, "stranger@example.com")
	require.ErrorIs(t, f.svc.RewrapOwnOrgKey(ctx, f.org.ID, stranger.ID, userWrapped), repository.ErrForbidden)
	suspendedUser, suspended := f.member(t, f.org, "suspended@example.com", domain.OrgRoleMember, domain.OrgUserStatusSuspended)
	require.NoError(t, f.db.Model(suspended).Update("encrypted_org_key", "cnNhLXdyYXBwZWQta2V5").Error)
	require.ErrorIs(t, f.svc.RewrapOwnOrgKey(ctx, f.org.ID, suspendedUser.ID, userWrapped), repository.ErrForbidden)
}
