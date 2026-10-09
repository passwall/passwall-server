package service

import (
	"context"
	"errors"
	"testing"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/pkg/constants"
)

type adminUpdateUserRepoStub struct {
	repository.UserRepository
	users   map[uint]*domain.User
	updated *domain.User
}

func (r *adminUpdateUserRepoStub) GetByID(_ context.Context, id uint) (*domain.User, error) {
	u, ok := r.users[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	clone := *u
	return &clone, nil
}

func (r *adminUpdateUserRepoStub) CountByRoleID(_ context.Context, roleID uint) (int64, error) {
	var n int64
	for _, u := range r.users {
		if u.RoleID == roleID {
			n++
		}
	}
	return n, nil
}

func (r *adminUpdateUserRepoStub) Update(_ context.Context, u *domain.User) error {
	r.updated = u
	return nil
}

func TestUpdateByAdminGuards(t *testing.T) {
	admin, member := constants.RoleIDAdmin, constants.RoleIDMember
	invalidRole := uint(9)
	newEmail := "changed@example.com"
	newName := "Renamed"

	for _, tt := range []struct {
		name    string
		users   map[uint]*domain.User
		actor   uint
		target  uint
		req     domain.UpdateUserRequest
		wantErr error
	}{
		{
			name:  "promote member to admin",
			users: map[uint]*domain.User{1: {ID: 1, RoleID: admin}, 2: {ID: 2, RoleID: member}},
			actor: 1, target: 2, req: domain.UpdateUserRequest{RoleID: &admin},
		},
		{
			name:  "unknown role rejected",
			users: map[uint]*domain.User{1: {ID: 1, RoleID: admin}, 2: {ID: 2, RoleID: member}},
			actor: 1, target: 2, req: domain.UpdateUserRequest{RoleID: &invalidRole}, wantErr: ErrAdminRoleInvalid,
		},
		{
			name:  "cannot change own role",
			users: map[uint]*domain.User{1: {ID: 1, RoleID: admin}, 2: {ID: 2, RoleID: admin}},
			actor: 1, target: 1, req: domain.UpdateUserRequest{RoleID: &member}, wantErr: ErrAdminSelfRoleChange,
		},
		{
			name:  "system user role protected",
			users: map[uint]*domain.User{1: {ID: 1, RoleID: admin}, 2: {ID: 2, RoleID: admin, IsSystemUser: true}},
			actor: 1, target: 2, req: domain.UpdateUserRequest{RoleID: &member}, wantErr: ErrSystemUserProtected,
		},
		{
			name:  "system user email protected",
			users: map[uint]*domain.User{1: {ID: 1, RoleID: admin}, 2: {ID: 2, RoleID: admin, IsSystemUser: true, Email: "op@example.com"}},
			actor: 1, target: 2, req: domain.UpdateUserRequest{Email: &newEmail}, wantErr: ErrSystemUserProtected,
		},
		{
			name:  "system user name still editable",
			users: map[uint]*domain.User{1: {ID: 1, RoleID: admin}, 2: {ID: 2, RoleID: admin, IsSystemUser: true}},
			actor: 1, target: 2, req: domain.UpdateUserRequest{Name: &newName},
		},
		{
			name:  "last admin cannot be demoted",
			users: map[uint]*domain.User{1: {ID: 1, RoleID: member}, 2: {ID: 2, RoleID: admin}},
			actor: 1, target: 2, req: domain.UpdateUserRequest{RoleID: &member}, wantErr: ErrLastAdmin,
		},
		{
			name:  "demote one of two admins",
			users: map[uint]*domain.User{1: {ID: 1, RoleID: admin}, 2: {ID: 2, RoleID: admin}},
			actor: 1, target: 2, req: domain.UpdateUserRequest{RoleID: &member},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := &adminUpdateUserRepoStub{users: tt.users}
			svc := &userService{repo: repo, logger: noopLogger{}}
			before, after, err := svc.UpdateByAdmin(context.Background(), tt.actor, tt.target, &tt.req)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) || repo.updated != nil {
					t.Fatalf("err = %v (updated=%v), want %v and no write", err, repo.updated != nil, tt.wantErr)
				}
				return
			}
			if err != nil || after == nil || repo.updated == nil {
				t.Fatalf("UpdateByAdmin() = %v, %v", after, err)
			}
			if before.ID != tt.target {
				t.Fatalf("before snapshot = %+v", before)
			}
		})
	}
}
