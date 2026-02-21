package authz

import (
	"context"
	"errors"
	"testing"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
)

type fakeCollectionUserRepo struct {
	grant *domain.CollectionUser
	err   error
}

func (f *fakeCollectionUserRepo) GetByCollectionAndOrgUser(_ context.Context, _, _ uint) (*domain.CollectionUser, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.grant == nil {
		return nil, repository.ErrNotFound
	}
	return f.grant, nil
}

type fakeCollectionTeamRepo struct {
	grants []*domain.CollectionTeam
	err    error
}

func (f *fakeCollectionTeamRepo) ListByCollection(_ context.Context, _ uint) ([]*domain.CollectionTeam, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.grants, nil
}

type fakeTeamUserRepo struct {
	memberships []*domain.TeamUser
	err         error
}

func (f *fakeTeamUserRepo) ListByOrgUser(_ context.Context, _ uint) ([]*domain.TeamUser, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.memberships, nil
}

func TestComputeCollectionAccess_AdminAndAccessAll(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("admin gets full access", func(t *testing.T) {
		t.Parallel()

		orgUser := &domain.OrganizationUser{Role: domain.OrgRoleAdmin}
		access, err := ComputeCollectionAccess(
			ctx,
			orgUser,
			1,
			&fakeCollectionUserRepo{},
			&fakeCollectionTeamRepo{},
			&fakeTeamUserRepo{},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !access.CanRead || !access.CanWrite || !access.CanAdmin {
			t.Fatalf("expected full access for admin, got %#v", access)
		}
	})

	t.Run("access_all gets full access", func(t *testing.T) {
		t.Parallel()

		orgUser := &domain.OrganizationUser{Role: domain.OrgRoleMember, AccessAll: true}
		access, err := ComputeCollectionAccess(
			ctx,
			orgUser,
			1,
			&fakeCollectionUserRepo{},
			&fakeCollectionTeamRepo{},
			&fakeTeamUserRepo{},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !access.CanRead || !access.CanWrite || !access.CanAdmin {
			t.Fatalf("expected full access for access_all, got %#v", access)
		}
	})
}

func TestComputeCollectionAccess_MergesDirectAndTeamGrants(t *testing.T) {
	t.Parallel()

	orgUser := &domain.OrganizationUser{
		ID:   99,
		Role: domain.OrgRoleMember,
	}
	access, err := ComputeCollectionAccess(
		context.Background(),
		orgUser,
		100,
		&fakeCollectionUserRepo{
			grant: &domain.CollectionUser{
				CanRead:  true,
				CanWrite: false,
				CanAdmin: false,
			},
		},
		&fakeCollectionTeamRepo{
			grants: []*domain.CollectionTeam{
				{TeamID: 10, CanRead: false, CanWrite: true, CanAdmin: false},
				{TeamID: 20, CanRead: false, CanWrite: false, CanAdmin: true},
			},
		},
		&fakeTeamUserRepo{
			memberships: []*domain.TeamUser{
				{TeamID: 10},
				{TeamID: 20},
			},
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !access.CanRead || !access.CanWrite || !access.CanAdmin {
		t.Fatalf("expected merged access to be full, got %#v", access)
	}
}

func TestComputeCollectionAccess_ErrorPropagation(t *testing.T) {
	t.Parallel()

	testErr := errors.New("db down")
	orgUser := &domain.OrganizationUser{ID: 7, Role: domain.OrgRoleMember}

	_, err := ComputeCollectionAccess(
		context.Background(),
		orgUser,
		1,
		&fakeCollectionUserRepo{err: testErr},
		&fakeCollectionTeamRepo{},
		&fakeTeamUserRepo{},
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
