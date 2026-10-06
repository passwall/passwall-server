package service

import (
	"context"
	"errors"
	"testing"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
)

type itemShareMembershipReaderStub struct {
	membership *domain.OrganizationUser
	err        error
}

func (s itemShareMembershipReaderStub) GetActiveByOrgAndUser(
	_ context.Context,
	_, _ uint,
) (*domain.OrganizationUser, error) {
	return s.membership, s.err
}

type itemShareCollectionUserReaderStub struct {
	access *domain.CollectionUser
	err    error
}

func (s itemShareCollectionUserReaderStub) GetByCollectionAndOrgUser(
	_ context.Context,
	_, _ uint,
) (*domain.CollectionUser, error) {
	return s.access, s.err
}

type itemShareCollectionTeamReaderStub struct {
	access []*domain.CollectionTeam
	err    error
}

func (s itemShareCollectionTeamReaderStub) ListByCollection(
	_ context.Context,
	_ uint,
) ([]*domain.CollectionTeam, error) {
	return s.access, s.err
}

type itemShareTeamMembershipReaderStub struct {
	memberships []*domain.TeamUser
	err         error
}

func (s itemShareTeamMembershipReaderStub) ListByOrgUser(
	_ context.Context,
	_ uint,
) ([]*domain.TeamUser, error) {
	return s.memberships, s.err
}

func TestAuthorizeItemShare(t *testing.T) {
	collectionID := uint(10)
	baseItem := &domain.OrganizationItem{
		OrganizationID:  7,
		CollectionID:    &collectionID,
		CreatedByUserID: 42,
	}

	tests := []struct {
		name       string
		userID     uint
		item       *domain.OrganizationItem
		membership *domain.OrganizationUser
		direct     *domain.CollectionUser
		wantErr    error
	}{
		{
			name:    "rejects non-member",
			userID:  42,
			item:    baseItem,
			wantErr: repository.ErrForbidden,
		},
		{
			name:   "rejects read-only collection access",
			userID: 42,
			item:   baseItem,
			membership: &domain.OrganizationUser{
				ID: 5, OrganizationID: 7, UserID: 42, Role: domain.OrgRoleMember,
			},
			direct:  &domain.CollectionUser{CanRead: true},
			wantErr: repository.ErrForbidden,
		},
		{
			name:   "allows collection write access",
			userID: 42,
			item:   baseItem,
			membership: &domain.OrganizationUser{
				ID: 5, OrganizationID: 7, UserID: 42, Role: domain.OrgRoleMember,
			},
			direct: &domain.CollectionUser{CanRead: true, CanWrite: true},
		},
		{
			name:   "allows organization admin",
			userID: 42,
			item:   baseItem,
			membership: &domain.OrganizationUser{
				ID: 5, OrganizationID: 7, UserID: 42, Role: domain.OrgRoleAdmin,
			},
		},
		{
			name:   "rejects non-creator for legacy orphan",
			userID: 99,
			item: &domain.OrganizationItem{
				OrganizationID: 7, CreatedByUserID: 42,
			},
			membership: &domain.OrganizationUser{
				ID: 5, OrganizationID: 7, UserID: 99, Role: domain.OrgRoleMember,
			},
			wantErr: repository.ErrForbidden,
		},
		{
			name:   "allows creator for legacy orphan",
			userID: 42,
			item: &domain.OrganizationItem{
				OrganizationID: 7, CreatedByUserID: 42,
			},
			membership: &domain.OrganizationUser{
				ID: 5, OrganizationID: 7, UserID: 42, Role: domain.OrgRoleMember,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			membershipErr := error(nil)
			if tt.membership == nil {
				membershipErr = repository.ErrNotFound
			}
			directErr := error(nil)
			if tt.direct == nil {
				directErr = repository.ErrNotFound
			}
			svc := &itemShareService{
				orgUserRepo: itemShareMembershipReaderStub{
					membership: tt.membership,
					err:        membershipErr,
				},
				collectionUsers: itemShareCollectionUserReaderStub{
					access: tt.direct,
					err:    directErr,
				},
				collectionTeams: itemShareCollectionTeamReaderStub{},
				teamMemberships: itemShareTeamMembershipReaderStub{},
			}

			err := svc.authorizeItemShare(context.Background(), tt.userID, tt.item)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("authorizeItemShare() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
