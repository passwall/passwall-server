package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/passwall/passwall-server/internal/domain"
)

// scimOrgUserRepo keeps memberships in memory with list/update/delete support.
type scimOrgUserRepo struct {
	*fakeOrgUserRepo
}

func (r scimOrgUserRepo) ListByOrganization(_ context.Context, orgID uint) ([]*domain.OrganizationUser, error) {
	var out []*domain.OrganizationUser
	for _, m := range r.members {
		if m.OrganizationID == orgID {
			out = append(out, m)
		}
	}
	return out, nil
}

// Update writes into the stored record so tests holding it see the change.
func (r scimOrgUserRepo) Update(_ context.Context, ou *domain.OrganizationUser) error {
	for _, m := range r.members {
		if m.ID == ou.ID {
			*m = *ou
		}
	}
	return nil
}

func (r scimOrgUserRepo) Delete(_ context.Context, id uint) error {
	for k, m := range r.members {
		if m.ID == id {
			delete(r.members, k)
		}
	}
	return nil
}

func newSCIMEnv(t *testing.T) (*scimService, scimOrgUserRepo, *fakeUserRepo, *fakeSSOConnRepo) {
	t.Helper()
	users := newFakeUserRepo()
	members := scimOrgUserRepo{newFakeOrgUserRepo()}
	conns := newFakeSSOConnRepo()
	svc := NewSCIMService(nil, users, members, nil, nil, noopLogger{}, testBaseURL).(*scimService)
	WithProvisioningGuards(svc, conns, nil)
	return svc, members, users, conns
}

func addSCIMMember(members scimOrgUserRepo, users *fakeUserRepo, id uint, email string, role domain.OrganizationRole, status domain.OrganizationUserStatus, key string) *domain.OrganizationUser {
	user := &domain.User{ID: id, Email: email, IsVerified: true}
	users.add(user)
	ou := &domain.OrganizationUser{ID: id, OrganizationID: testOrgID, UserID: id, Role: role, Status: status, EncryptedOrgKey: key, User: user}
	members.add(ou)
	return ou
}

func TestSCIMReactivation_WithoutKeyStaysProvisioned(t *testing.T) {
	t.Parallel()
	svc, members, users, _ := newSCIMEnv(t)
	ou := addSCIMMember(members, users, 5, "bob@acme.com", domain.OrgRoleMember, domain.OrgUserStatusSuspended, pendingOrgKey)

	_, err := svc.UpdateUser(context.Background(), testOrgID, "5", &domain.SCIMUser{Active: true})
	require.NoError(t, err)
	assert.Equal(t, domain.OrgUserStatusProvisioned, ou.Status, "no org key: an admin must still confirm")

	ou.Status = domain.OrgUserStatusSuspended
	ou.EncryptedOrgKey = "4.wrapped-key"
	_, err = svc.PatchUser(context.Background(), testOrgID, "5", &domain.SCIMPatchOp{
		Operations: []domain.SCIMPatchOpItem{{Op: "Replace", Path: "active", Value: true}},
	})
	require.NoError(t, err)
	assert.Equal(t, domain.OrgUserStatusConfirmed, ou.Status)
}

func TestSCIMPatch_PathlessWithoutActiveDoesNotSuspend(t *testing.T) {
	t.Parallel()
	svc, members, users, _ := newSCIMEnv(t)
	ou := addSCIMMember(members, users, 5, "bob@acme.com", domain.OrgRoleMember, domain.OrgUserStatusConfirmed, "4.k")

	_, err := svc.PatchUser(context.Background(), testOrgID, "5", &domain.SCIMPatchOp{
		Operations: []domain.SCIMPatchOpItem{{Op: "replace", Value: map[string]interface{}{"externalId": "ext-5", "displayName": "Bob"}}},
	})
	require.NoError(t, err)
	assert.Equal(t, domain.OrgUserStatusConfirmed, ou.Status)
	require.NotNil(t, ou.ExternalID)
	assert.Equal(t, "ext-5", *ou.ExternalID)

	_, err = svc.PatchUser(context.Background(), testOrgID, "5", &domain.SCIMPatchOp{
		Operations: []domain.SCIMPatchOpItem{{Op: "replace", Value: map[string]interface{}{"active": false}}},
	})
	require.NoError(t, err)
	assert.Equal(t, domain.OrgUserStatusSuspended, ou.Status)
}

func TestSCIM_LastOwnerIsProtected(t *testing.T) {
	t.Parallel()
	svc, members, users, _ := newSCIMEnv(t)
	owner := addSCIMMember(members, users, 1, "owner@acme.com", domain.OrgRoleOwner, domain.OrgUserStatusConfirmed, "4.k")

	err := svc.DeleteUser(context.Background(), testOrgID, "1")
	assert.ErrorIs(t, err, ErrSCIMLastOwner)
	_, err = svc.UpdateUser(context.Background(), testOrgID, "1", &domain.SCIMUser{Active: false})
	assert.ErrorIs(t, err, ErrSCIMLastOwner)
	_, err = svc.PatchUser(context.Background(), testOrgID, "1", &domain.SCIMPatchOp{
		Operations: []domain.SCIMPatchOpItem{{Op: "replace", Path: "active", Value: false}},
	})
	assert.ErrorIs(t, err, ErrSCIMLastOwner)
	assert.Equal(t, domain.OrgUserStatusConfirmed, owner.Status)

	addSCIMMember(members, users, 2, "owner2@acme.com", domain.OrgRoleOwner, domain.OrgUserStatusConfirmed, "4.k")
	assert.NoError(t, svc.DeleteUser(context.Background(), testOrgID, "1"), "another owner remains")
}

func TestSCIMFilter(t *testing.T) {
	t.Parallel()
	svc, members, users, _ := newSCIMEnv(t)
	bob := addSCIMMember(members, users, 5, "bob@acme.com", domain.OrgRoleMember, domain.OrgUserStatusConfirmed, "4.k")
	ext := "ext-5"
	bob.ExternalID = &ext
	addSCIMMember(members, users, 6, "carol@acme.com", domain.OrgRoleMember, domain.OrgUserStatusConfirmed, "4.k")

	for _, filter := range []string{`userName eq "BOB@acme.com"`, `externalId eq "ext-5"`, `emails.value eq "bob@acme.com"`} {
		res, err := svc.ListUsers(context.Background(), testOrgID, filter, 1, 100)
		require.NoError(t, err, filter)
		require.Len(t, res.Resources, 1, filter)
	}
	res, err := svc.ListUsers(context.Background(), testOrgID, `userName eq "nobody@acme.com"`, 1, 100)
	require.NoError(t, err)
	assert.Empty(t, res.Resources)

	_, err = svc.ListUsers(context.Background(), testOrgID, `name.givenName sw "B"`, 1, 100)
	assert.ErrorIs(t, err, ErrSCIMInvalidFilter, "unsupported filters are rejected, not ignored")
}

func TestSCIMCreateUser_RequiresVerifiedDomain(t *testing.T) {
	t.Parallel()
	svc, _, users, conns := newSCIMEnv(t)
	users.add(&domain.User{ID: 9, Email: "dave@acme.com", IsVerified: true})
	req := &domain.SCIMUser{UserName: "dave@acme.com", Active: true}

	_, err := svc.CreateUser(context.Background(), testOrgID, req)
	assert.ErrorIs(t, err, ErrSCIMDomainNotVerified)

	conns.add(&domain.SSOConnection{ID: 3, OrganizationID: 99, Domain: "acme.com"})
	_, err = svc.CreateUser(context.Background(), testOrgID, req)
	assert.ErrorIs(t, err, ErrSCIMDomainNotVerified, "verified by another organization")

	conns.add(&domain.SSOConnection{ID: 4, OrganizationID: testOrgID, Domain: "acme.io"})
	users.add(&domain.User{ID: 10, Email: "erin@acme.io", IsVerified: true})
	created, err := svc.CreateUser(context.Background(), testOrgID, &domain.SCIMUser{UserName: "erin@acme.io", Active: true})
	require.NoError(t, err)
	assert.Equal(t, "erin@acme.io", created.UserName)
}
