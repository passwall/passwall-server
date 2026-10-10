package authz

import (
	"context"
	"fmt"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
)

type CollectionUserAccessReader interface {
	GetByCollectionAndOrgUser(ctx context.Context, collectionID, orgUserID uint) (*domain.CollectionUser, error)
}

type CollectionTeamAccessReader interface {
	ListByCollection(ctx context.Context, collectionID uint) ([]*domain.CollectionTeam, error)
}

type TeamMembershipReader interface {
	ListByOrgUser(ctx context.Context, orgUserID uint) ([]*domain.TeamUser, error)
}

// HasUnrestrictedItemAccess reports whether a member sees every collection's
// items without grants: legacy access-all members, and owners/admins unless
// the organization turned off "owners and admins can manage all
// collections". Managing collection access is separate (see
// CanAdministerCollections).
func HasUnrestrictedItemAccess(orgUser *domain.OrganizationUser) bool {
	if orgUser == nil {
		return false
	}
	// For owners and admins the organization setting decides; their
	// access_all flag (set when organizations are created) is redundant.
	if orgUser.IsAdmin() {
		return orgUser.Organization == nil || orgUser.Organization.AdminsManageAllCollections
	}
	// Legacy access-all members keep seeing everything.
	return orgUser.AccessAll
}

// CanAdministerCollections reports whether a member can manage any
// collection's settings and access regardless of grants (owners and admins).
func CanAdministerCollections(orgUser *domain.OrganizationUser) bool {
	return orgUser != nil && orgUser.IsAdmin()
}

type CollectionAccess struct {
	CanRead       bool
	CanWrite      bool
	CanAdmin      bool
	HidePasswords bool
}

// Permission expresses the merged access as a single level.
func (a *CollectionAccess) Permission() domain.CollectionPermission {
	if a == nil {
		return domain.CollectionPermissionNone
	}
	return domain.CollectionPermissionFromFlags(a.CanRead, a.CanWrite, a.CanAdmin, a.HidePasswords)
}

// ItemPermissions is what the access allows on an item in the collection.
func (a *CollectionAccess) ItemPermissions() *domain.ItemPermissions {
	return domain.ItemPermissionsFor(a.Permission())
}

// grantMerge combines grants the way Bitwarden does: the most permissive
// grant wins, so passwords stay hidden only when every grant hides them.
type grantMerge struct {
	result   *CollectionAccess
	anyGrant bool
	allHide  bool
}

func (m *grantMerge) add(canRead, canWrite, canAdmin, hide bool) {
	if !canRead && !canWrite && !canAdmin {
		return
	}
	m.result.CanRead = true
	m.result.CanWrite = m.result.CanWrite || canWrite || canAdmin
	m.result.CanAdmin = m.result.CanAdmin || canAdmin
	if !m.anyGrant {
		m.allHide = true
	}
	m.anyGrant = true
	if !hide || canAdmin {
		m.allHide = false
	}
}

func (m *grantMerge) finish() *CollectionAccess {
	m.result.HidePasswords = m.anyGrant && m.allHide
	return m.result
}

func ComputeCollectionAccess(
	ctx context.Context,
	orgUser *domain.OrganizationUser,
	collectionID uint,
	collectionUserRepo CollectionUserAccessReader,
	collectionTeamRepo CollectionTeamAccessReader,
	teamUserRepo TeamMembershipReader,
) (*CollectionAccess, error) {
	if orgUser == nil {
		return &CollectionAccess{}, repository.ErrForbidden
	}

	// Unrestricted members (see HasUnrestrictedItemAccess) see everything.
	if HasUnrestrictedItemAccess(orgUser) {
		return &CollectionAccess{
			CanRead:  true,
			CanWrite: true,
			CanAdmin: true,
		}, nil
	}

	merge := &grantMerge{result: &CollectionAccess{}}

	direct, err := collectionUserRepo.GetByCollectionAndOrgUser(ctx, collectionID, orgUser.ID)
	if err == nil && direct != nil {
		merge.add(direct.CanRead, direct.CanWrite, direct.CanAdmin, direct.HidePasswords)
	} else if err != nil && err != repository.ErrNotFound {
		return nil, fmt.Errorf("failed to load direct collection access: %w", err)
	}

	teamUsers, err := teamUserRepo.ListByOrgUser(ctx, orgUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load team membership: %w", err)
	}
	if len(teamUsers) == 0 {
		return merge.finish(), nil
	}

	teamIDs := make(map[uint]struct{}, len(teamUsers))
	for _, tu := range teamUsers {
		teamIDs[tu.TeamID] = struct{}{}
	}

	teamAccess, err := collectionTeamRepo.ListByCollection(ctx, collectionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load team collection access: %w", err)
	}
	for _, ta := range teamAccess {
		if _, ok := teamIDs[ta.TeamID]; !ok {
			continue
		}
		merge.add(ta.CanRead, ta.CanWrite, ta.CanAdmin, ta.HidePasswords)
	}

	return merge.finish(), nil
}
