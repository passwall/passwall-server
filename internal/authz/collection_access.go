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

type CollectionAccess struct {
	CanRead  bool
	CanWrite bool
	CanAdmin bool
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

	// Org admins and access_all users are unrestricted.
	if orgUser.IsAdmin() || orgUser.AccessAll {
		return &CollectionAccess{
			CanRead:  true,
			CanWrite: true,
			CanAdmin: true,
		}, nil
	}

	result := &CollectionAccess{}

	direct, err := collectionUserRepo.GetByCollectionAndOrgUser(ctx, collectionID, orgUser.ID)
	if err == nil && direct != nil {
		result.CanRead = result.CanRead || direct.CanRead
		result.CanWrite = result.CanWrite || direct.CanWrite
		result.CanAdmin = result.CanAdmin || direct.CanAdmin
	} else if err != nil && err != repository.ErrNotFound {
		return nil, fmt.Errorf("failed to load direct collection access: %w", err)
	}

	teamUsers, err := teamUserRepo.ListByOrgUser(ctx, orgUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load team membership: %w", err)
	}
	if len(teamUsers) == 0 {
		return result, nil
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
		result.CanRead = result.CanRead || ta.CanRead
		result.CanWrite = result.CanWrite || ta.CanWrite
		result.CanAdmin = result.CanAdmin || ta.CanAdmin
	}

	return result, nil
}
