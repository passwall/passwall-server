package gormrepo

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func openAdminListDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := openSubscriptionDB(t)
	require.NoError(t, db.AutoMigrate(
		&domain.User{}, &domain.OrganizationUser{}, &domain.Team{},
		&domain.Collection{}, &domain.OrganizationItem{},
	))
	return db
}

func createAdminListOwner(t *testing.T, db *gorm.DB, orgID, userID uint, email, name string) {
	t.Helper()
	require.NoError(t, db.Create(&domain.User{ID: userID, UUID: uuid.New(), Email: email, Name: name}).Error)
	require.NoError(t, db.Create(&domain.OrganizationUser{
		UUID: uuid.New(), OrganizationID: orgID, UserID: userID,
		Role: domain.OrgRoleOwner, Status: domain.OrgUserStatusConfirmed,
	}).Error)
}

func TestOrganizationRepository_AdminListBatchesOwnersAndCounts(t *testing.T) {
	db := openAdminListDB(t)
	ctx := context.Background()
	now := time.Now()
	createPlanFirstOrg(t, db, 1, false, 10, now)
	createPlanFirstOrg(t, db, 2, false, 20, now)
	createAdminListOwner(t, db, 1, 10, "alice@example.com", "Alice")
	createAdminListOwner(t, db, 2, 20, "bob@example.com", "Bob")
	require.NoError(t, db.Create(&domain.OrganizationUser{
		UUID: uuid.New(), OrganizationID: 1, UserID: 11, Role: domain.OrgRoleMember, Status: domain.OrgUserStatusAccepted,
	}).Error)
	require.NoError(t, db.Create(&domain.OrganizationUser{
		UUID: uuid.New(), OrganizationID: 1, UserID: 12, Role: domain.OrgRoleMember, Status: domain.OrgUserStatusInvited,
	}).Error)

	counts, err := NewOrganizationRepository(db).GetCountsByIDs(ctx, []uint{1, 2, 3})
	require.NoError(t, err)
	assert.Equal(t, 2, counts[1].Members, "invited members are not counted")
	assert.Equal(t, 1, counts[2].Members)
	assert.Equal(t, repository.OrganizationCounts{}, counts[3])

	owners, err := NewOrganizationUserRepository(db).ListOwnersByOrganizationIDs(ctx, []uint{1, 2})
	require.NoError(t, err)
	require.Len(t, owners, 2)
	assert.Equal(t, "alice@example.com", owners[1].User.Email)
	assert.Equal(t, "bob@example.com", owners[2].User.Email)
}

func TestOrganizationRepository_AdminListOwnerSearchAndEndingSoon(t *testing.T) {
	db := openAdminListDB(t)
	ctx := context.Background()
	now := time.Now()
	require.NoError(t, db.Create(&domain.Plan{ID: 1, Code: "free-monthly", Name: "Free"}).Error)
	require.NoError(t, db.Create(&domain.Plan{ID: 2, Code: "pro-yearly", Name: "Pro", PriceCents: 3600}).Error)

	for id := uint(1); id <= 4; id++ {
		createPlanFirstOrg(t, db, id, true, id*10, now.Add(time.Duration(id)*time.Minute))
		createAdminListOwner(t, db, id, id*10, "owner"+string(rune('0'+id))+"@example.com", "Owner")
	}
	soon := now.Add(3 * 24 * time.Hour)
	later := now.Add(60 * 24 * time.Hour)
	stripeID := "sub_live"
	for _, sub := range []*domain.Subscription{
		{OrganizationID: 1, PlanID: 2, State: domain.SubStateActive, RenewAt: &soon},                                  // manual, ending soon
		{OrganizationID: 2, PlanID: 2, State: domain.SubStateActive, RenewAt: &later},                                 // manual, later
		{OrganizationID: 3, PlanID: 2, State: domain.SubStateActive, RenewAt: &soon, StripeSubscriptionID: &stripeID}, // stripe renewal
		{OrganizationID: 4, PlanID: 1, State: domain.SubStateActive},                                                  // catalog free
	} {
		sub.UUID = uuid.New()
		require.NoError(t, db.Create(sub).Error)
	}
	repo := NewOrganizationRepository(db)

	cutoff := now.Add(7 * 24 * time.Hour)
	orgs, res, err := repo.List(ctx, repository.ListFilter{ManualGrantEndsBefore: &cutoff})
	require.NoError(t, err)
	require.Len(t, orgs, 1)
	assert.EqualValues(t, 1, orgs[0].ID)
	assert.EqualValues(t, 1, res.Filtered)

	orgs, _, err = repo.List(ctx, repository.ListFilter{Search: "OWNER3@", SearchOwners: true})
	require.NoError(t, err)
	require.Len(t, orgs, 1)
	assert.EqualValues(t, 3, orgs[0].ID)

	orgs, _, err = repo.List(ctx, repository.ListFilter{Search: "owner3@"})
	require.NoError(t, err)
	assert.Empty(t, orgs, "owner search must be opt-in")

	orgs, _, err = repo.List(ctx, repository.ListFilter{SortByAccessEndAsc: true})
	require.NoError(t, err)
	require.Len(t, orgs, 4)
	assert.EqualValues(t, 4, orgs[3].ID, "organizations without an end date sort last")
	assert.EqualValues(t, 2, orgs[2].ID)
}
