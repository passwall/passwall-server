package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/passwall/passwall-server/internal/authz"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
)

type organizationItemService struct {
	itemRepo           repository.OrganizationItemRepository
	collectionRepo     repository.CollectionRepository
	collectionUserRepo repository.CollectionUserRepository
	collectionTeamRepo repository.CollectionTeamRepository
	teamUserRepo       repository.TeamUserRepository
	orgUserRepo        repository.OrganizationUserRepository
	featureService     FeatureService
	logger             Logger
}

// NewOrganizationItemService creates a new organization item service
func NewOrganizationItemService(
	itemRepo repository.OrganizationItemRepository,
	collectionRepo repository.CollectionRepository,
	collectionUserRepo repository.CollectionUserRepository,
	collectionTeamRepo repository.CollectionTeamRepository,
	teamUserRepo repository.TeamUserRepository,
	orgUserRepo repository.OrganizationUserRepository,
	featureService FeatureService,
	logger Logger,
) OrganizationItemService {
	return &organizationItemService{
		itemRepo:           itemRepo,
		collectionRepo:     collectionRepo,
		collectionUserRepo: collectionUserRepo,
		collectionTeamRepo: collectionTeamRepo,
		teamUserRepo:       teamUserRepo,
		orgUserRepo:        orgUserRepo,
		featureService:     featureService,
		logger:             logger,
	}
}

// CreateOrgItemRequest for creating organization items
type CreateOrgItemRequest struct {
	CollectionID *uint               `json:"collection_id,omitempty"`
	ItemType     domain.ItemType     `json:"item_type" validate:"required"`
	Data         string              `json:"data" validate:"required"` // Encrypted with Org Key
	Metadata     domain.ItemMetadata `json:"metadata" validate:"required"`
	IsFavorite   bool                `json:"is_favorite"`
	FolderID     *uint               `json:"folder_id,omitempty"`
	Reprompt     bool                `json:"reprompt"`
	AutoFill     *bool               `json:"auto_fill,omitempty"`
	AutoLogin    *bool               `json:"auto_login,omitempty"`
}

// UpdateOrgItemRequest for updating organization items
type UpdateOrgItemRequest struct {
	CollectionID *uint                `json:"collection_id,omitempty"`
	Data         *string              `json:"data,omitempty"`
	Metadata     *domain.ItemMetadata `json:"metadata,omitempty"`
	IsFavorite   *bool                `json:"is_favorite,omitempty"`
	FolderID     *uint                `json:"folder_id,omitempty"`
	Reprompt     *bool                `json:"reprompt,omitempty"`
	AutoFill     *bool                `json:"auto_fill,omitempty"`
	AutoLogin    *bool                `json:"auto_login,omitempty"`
}

type OrganizationItemsV2Request struct {
	Cursor        string
	Limit         int
	SinceRevision int64
	// ClientHidesPasswords: the client enforces hidden passwords itself, so
	// hidden items keep their encrypted data (needed for autofill).
	ClientHidesPasswords bool
	// IncludePermissions adds per-item permissions to the response.
	IncludePermissions bool
}

type OrganizationItemsV2Response struct {
	Items      []*domain.OrganizationItemSyncDTO `json:"items"`
	NextCursor *string                           `json:"next_cursor"`
	Revision   int64                             `json:"revision"`
}

type organizationItemsCursor struct {
	SinceRevision int64 `json:"since_revision"`
	HeadRevision  int64 `json:"head_revision"`
	LastRevision  int64 `json:"last_revision"`
	LastID        uint  `json:"last_id"`
}

func (s *organizationItemService) Create(ctx context.Context, orgID, userID uint, req *CreateOrgItemRequest) (*domain.OrganizationItem, error) {
	// Check if user is member of organization
	orgUser, err := s.orgUserRepo.GetActiveByOrgAndUser(ctx, orgID, userID)
	if err != nil {
		return nil, repository.ErrForbidden
	}
	allowed, err := s.featureService.CanCreateItem(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to verify item entitlement: %w", err)
	}
	if !allowed {
		return nil, repository.ErrForbidden
	}
	if req.ItemType == domain.ItemTypePasskey {
		allowed, err = s.featureService.CanUsePasskeys(ctx, orgID)
		if err != nil {
			return nil, fmt.Errorf("failed to verify passkey entitlement: %w", err)
		}
		if !allowed {
			return nil, repository.ErrForbidden
		}
	}

	// Enforce "no orphan items": always place items in a collection.
	// If client doesn't specify a collection, we use the org's default collection.
	if req.CollectionID == nil {
		def, err := s.collectionRepo.GetDefaultByOrganization(ctx, orgID)
		if err != nil {
			return nil, fmt.Errorf("default collection unavailable: %w", err)
		}
		req.CollectionID = &def.ID
	}

	// If collection specified, check access
	if req.CollectionID != nil {
		collection, err := s.collectionRepo.GetByID(ctx, *req.CollectionID)
		if err != nil {
			return nil, fmt.Errorf("collection not found: %w", err)
		}

		if collection.OrganizationID != orgID {
			return nil, repository.ErrForbidden
		}

		// Check collection access (enforce write permission for non-admin users)
		if !orgUser.IsAdmin() && !orgUser.AccessAll {
			access, err := authz.ComputeCollectionAccess(
				ctx,
				orgUser,
				*req.CollectionID,
				s.collectionUserRepo,
				s.collectionTeamRepo,
				s.teamUserRepo,
			)
			if err != nil {
				return nil, err
			}
			if !access.CanWrite && !access.CanAdmin {
				return nil, repository.ErrForbidden
			}
		}
	}

	// Validate item type
	if !req.ItemType.IsValid() {
		return nil, fmt.Errorf("invalid item type: %d", req.ItemType)
	}

	// Create item
	item := &domain.OrganizationItem{
		UUID:            uuid.New(),
		OrganizationID:  orgID,
		CollectionID:    req.CollectionID,
		ItemType:        req.ItemType,
		Data:            req.Data, // Already encrypted with Org Key
		Metadata:        req.Metadata,
		IsFavorite:      req.IsFavorite,
		FolderID:        req.FolderID,
		Reprompt:        req.Reprompt,
		CreatedByUserID: userID,
	}
	if req.AutoFill != nil {
		item.AutoFill = *req.AutoFill
	}
	if req.AutoLogin != nil {
		item.AutoLogin = *req.AutoLogin
	}

	if err := s.itemRepo.Create(ctx, item); err != nil {
		s.logger.Error("failed to create organization item", "org_id", orgID, "error", err)
		return nil, fmt.Errorf("failed to create item: %w", err)
	}

	s.logger.Info("organization item created", "item_id", item.ID, "org_id", orgID, "user_id", userID)
	return item, nil
}

func (s *organizationItemService) GetByID(ctx context.Context, id, userID uint) (*domain.OrganizationItem, error) {
	item, err := s.itemRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("item not found: %w", err)
	}

	// Check if user has access to organization
	orgUser, err := s.orgUserRepo.GetActiveByOrgAndUser(ctx, item.OrganizationID, userID)
	if err != nil {
		return nil, repository.ErrForbidden
	}
	allowed, err := s.featureService.CanReadVault(ctx, item.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to verify vault read entitlement: %w", err)
	}
	if !allowed {
		return nil, repository.ErrForbidden
	}

	if orgUser.IsAdmin() || orgUser.AccessAll {
		return item, nil
	}

	// Legacy safety for orphaned items: only creator can access.
	if item.CollectionID == nil {
		if item.CreatedByUserID != userID {
			return nil, repository.ErrForbidden
		}
		return item, nil
	}

	access, err := authz.ComputeCollectionAccess(
		ctx,
		orgUser,
		*item.CollectionID,
		s.collectionUserRepo,
		s.collectionTeamRepo,
		s.teamUserRepo,
	)
	if err != nil {
		return nil, err
	}
	if !access.CanRead {
		return nil, repository.ErrForbidden
	}

	return item, nil
}

func (s *organizationItemService) ListByOrganization(ctx context.Context, orgID, userID uint, filter repository.OrganizationItemFilter) ([]*domain.OrganizationItem, int64, error) {
	// Check if user is member of organization
	orgUser, err := s.orgUserRepo.GetActiveByOrgAndUser(ctx, orgID, userID)
	if err != nil {
		return nil, 0, repository.ErrForbidden
	}

	allowedCollectionIDs := map[uint]struct{}{}
	restrictByCollections := !orgUser.IsAdmin() && !orgUser.AccessAll
	if restrictByCollections {
		allowedCollections, err := s.collectionRepo.ListForUser(ctx, orgID, userID)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to get allowed collections: %w", err)
		}
		for _, c := range allowedCollections {
			allowedCollectionIDs[c.ID] = struct{}{}
		}
	}

	// Validate collection belongs to org if provided
	if filter.CollectionID != nil {
		collection, err := s.collectionRepo.GetByID(ctx, *filter.CollectionID)
		if err != nil {
			return nil, 0, fmt.Errorf("collection not found: %w", err)
		}
		if collection.OrganizationID != orgID {
			return nil, 0, repository.ErrForbidden
		}
		if restrictByCollections {
			if _, ok := allowedCollectionIDs[*filter.CollectionID]; !ok {
				return nil, 0, repository.ErrForbidden
			}
		}
	}

	filter.OrganizationID = orgID
	if restrictByCollections {
		filter.RestrictToCollections = true
		filter.AllowedCollectionIDs = make([]uint, 0, len(allowedCollectionIDs))
		for collectionID := range allowedCollectionIDs {
			filter.AllowedCollectionIDs = append(filter.AllowedCollectionIDs, collectionID)
		}
	}
	if filter.PerPage == 0 {
		filter.PerPage = 1000
	}
	if filter.Page == 0 {
		filter.Page = 1
	}

	items, total, err := s.itemRepo.ListByOrganization(ctx, filter)
	if err != nil {
		s.logger.Error("failed to list organization items", "org_id", orgID, "error", err)
		return nil, 0, fmt.Errorf("failed to list items: %w", err)
	}

	return items, total, nil
}

func (s *organizationItemService) ListV2(ctx context.Context, orgID, userID uint, req OrganizationItemsV2Request) (*OrganizationItemsV2Response, error) {
	orgUser, err := s.orgUserRepo.GetActiveByOrgAndUser(ctx, orgID, userID)
	if err != nil {
		return nil, repository.ErrForbidden
	}
	if req.SinceRevision < 0 {
		return nil, repository.ErrInvalidInput
	}
	if req.Limit <= 0 {
		req.Limit = 100
	}
	if req.Limit > 500 {
		req.Limit = 500
	}

	cursor := organizationItemsCursor{SinceRevision: req.SinceRevision}
	if req.Cursor != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(req.Cursor)
		if err != nil || json.Unmarshal(decoded, &cursor) != nil {
			return nil, repository.ErrInvalidInput
		}
		if req.SinceRevision != 0 && req.SinceRevision != cursor.SinceRevision {
			return nil, repository.ErrInvalidInput
		}
	}

	filter := repository.OrganizationItemV2Filter{
		OrganizationID: orgID,
		AfterRevision:  cursor.LastRevision,
		AfterID:        cursor.LastID,
		SinceRevision:  cursor.SinceRevision,
		UpToRevision:   cursor.HeadRevision,
		Limit:          req.Limit,
	}
	if !orgUser.IsAdmin() && !orgUser.AccessAll {
		collections, err := s.collectionRepo.ListForUser(ctx, orgID, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to get allowed collections: %w", err)
		}
		filter.RestrictToCollections = true
		filter.AllowedCollectionIDs = make([]uint, 0, len(collections))
		for _, collection := range collections {
			filter.AllowedCollectionIDs = append(filter.AllowedCollectionIDs, collection.ID)
		}
	}

	items, headRevision, err := s.itemRepo.ListV2(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list item changes: %w", err)
	}
	hasMore := len(items) > req.Limit
	if hasMore {
		items = items[:req.Limit]
	}

	dtos := make([]*domain.OrganizationItemSyncDTO, 0, len(items))
	accessCache := make(map[uint]*authz.CollectionAccess)
	for _, item := range items {
		dto := domain.ToOrganizationItemSyncDTO(item)
		if !dto.Deleted && item.CollectionID != nil {
			collectionID := *item.CollectionID
			access, ok := accessCache[collectionID]
			if !ok {
				access, err = s.GetCollectionAccess(ctx, orgID, userID, collectionID)
				if err != nil {
					return nil, repository.ErrForbidden
				}
				accessCache[collectionID] = access
			}
			if req.IncludePermissions {
				dto.Permissions = access.ItemPermissions()
			}
			if access.HidePasswords {
				dto.HidePasswords = true
				if !req.ClientHidesPasswords {
					dto.Data = ""
				}
			}
		} else if !dto.Deleted && req.IncludePermissions {
			// Items without a collection are visible only to their creator
			// and to unrestricted members, who have full access to them.
			dto.Permissions = domain.FullItemPermissions()
		}
		dtos = append(dtos, dto)
	}

	var nextCursor *string
	if hasMore && len(items) > 0 {
		last := items[len(items)-1]
		next := organizationItemsCursor{
			SinceRevision: cursor.SinceRevision,
			HeadRevision:  headRevision,
			LastRevision:  last.Revision,
			LastID:        last.ID,
		}
		encoded, err := json.Marshal(next)
		if err != nil {
			return nil, err
		}
		value := base64.RawURLEncoding.EncodeToString(encoded)
		nextCursor = &value
	}

	return &OrganizationItemsV2Response{
		Items:      dtos,
		NextCursor: nextCursor,
		Revision:   headRevision,
	}, nil
}

func (s *organizationItemService) ListByCollection(ctx context.Context, collectionID, userID uint) ([]*domain.OrganizationItem, error) {
	// Get collection
	collection, err := s.collectionRepo.GetByID(ctx, collectionID)
	if err != nil {
		return nil, fmt.Errorf("collection not found: %w", err)
	}

	// Check if user has access to organization
	orgUser, err := s.orgUserRepo.GetActiveByOrgAndUser(ctx, collection.OrganizationID, userID)
	if err != nil {
		return nil, repository.ErrForbidden
	}

	// Check access (admins/access_all can view all; others are scoped to assigned collections)
	if !orgUser.IsAdmin() && !orgUser.AccessAll {
		access, err := authz.ComputeCollectionAccess(
			ctx,
			orgUser,
			collectionID,
			s.collectionUserRepo,
			s.collectionTeamRepo,
			s.teamUserRepo,
		)
		if err != nil {
			return nil, err
		}
		if !access.CanRead {
			return nil, repository.ErrForbidden
		}
	}

	items, err := s.itemRepo.ListByCollection(ctx, collectionID)
	if err != nil {
		s.logger.Error("failed to list collection items", "collection_id", collectionID, "error", err)
		return nil, fmt.Errorf("failed to list items: %w", err)
	}

	return items, nil
}

func (s *organizationItemService) Update(ctx context.Context, id, userID uint, req *UpdateOrgItemRequest) (*domain.OrganizationItem, error) {
	// Get item
	item, err := s.itemRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("item not found: %w", err)
	}

	// Check access
	orgUser, err := s.orgUserRepo.GetActiveByOrgAndUser(ctx, item.OrganizationID, userID)
	if err != nil {
		return nil, repository.ErrForbidden
	}
	allowed, err := s.featureService.CanWriteVault(ctx, item.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to verify write entitlement: %w", err)
	}
	if !allowed {
		return nil, repository.ErrForbidden
	}

	// Non-admin users need write/admin on current collection.
	if !orgUser.IsAdmin() && !orgUser.AccessAll {
		// Legacy safety for orphaned items: only creator can edit.
		if item.CollectionID == nil {
			if item.CreatedByUserID != userID {
				return nil, repository.ErrForbidden
			}
		} else {
			access, err := authz.ComputeCollectionAccess(
				ctx,
				orgUser,
				*item.CollectionID,
				s.collectionUserRepo,
				s.collectionTeamRepo,
				s.teamUserRepo,
			)
			if err != nil {
				return nil, err
			}
			if !access.CanWrite && !access.CanAdmin {
				return nil, repository.ErrForbidden
			}
		}
	}

	// If moving to another collection, require write/admin on destination too.
	if req.CollectionID != nil {
		collection, err := s.collectionRepo.GetByID(ctx, *req.CollectionID)
		if err != nil {
			return nil, fmt.Errorf("collection not found: %w", err)
		}
		if collection.OrganizationID != item.OrganizationID {
			return nil, repository.ErrForbidden
		}
		if !orgUser.IsAdmin() && !orgUser.AccessAll {
			access, err := authz.ComputeCollectionAccess(
				ctx,
				orgUser,
				*req.CollectionID,
				s.collectionUserRepo,
				s.collectionTeamRepo,
				s.teamUserRepo,
			)
			if err != nil {
				return nil, err
			}
			if !access.CanWrite && !access.CanAdmin {
				return nil, repository.ErrForbidden
			}
		}
	}

	// Update fields
	if req.CollectionID != nil {
		item.CollectionID = req.CollectionID
	}
	if req.Data != nil {
		item.Data = *req.Data
	}
	if req.Metadata != nil {
		item.Metadata = *req.Metadata
	}
	if req.IsFavorite != nil {
		item.IsFavorite = *req.IsFavorite
	}
	if req.FolderID != nil {
		item.FolderID = req.FolderID
	}
	if req.Reprompt != nil {
		item.Reprompt = *req.Reprompt
	}
	if req.AutoFill != nil {
		item.AutoFill = *req.AutoFill
	}
	if req.AutoLogin != nil {
		item.AutoLogin = *req.AutoLogin
	}

	if err := s.itemRepo.Update(ctx, item); err != nil {
		s.logger.Error("failed to update organization item", "item_id", id, "error", err)
		return nil, fmt.Errorf("failed to update item: %w", err)
	}

	s.logger.Info("organization item updated", "item_id", id, "user_id", userID)
	return item, nil
}

func (s *organizationItemService) Delete(ctx context.Context, id, userID uint) (*domain.OrganizationItem, error) {
	// Get item
	item, err := s.itemRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("item not found: %w", err)
	}

	// Check access
	orgUser, err := s.orgUserRepo.GetActiveByOrgAndUser(ctx, item.OrganizationID, userID)
	if err != nil {
		return nil, repository.ErrForbidden
	}
	allowed, err := s.featureService.CanDeleteVault(ctx, item.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to verify delete entitlement: %w", err)
	}
	if !allowed {
		return nil, repository.ErrForbidden
	}

	// Only admins or creator can delete
	if !orgUser.IsAdmin() && !orgUser.AccessAll {
		// Legacy safety for orphaned items: only creator can delete.
		if item.CollectionID == nil {
			if item.CreatedByUserID != userID {
				return nil, repository.ErrForbidden
			}
		} else {
			access, err := authz.ComputeCollectionAccess(
				ctx,
				orgUser,
				*item.CollectionID,
				s.collectionUserRepo,
				s.collectionTeamRepo,
				s.teamUserRepo,
			)
			if err != nil {
				return nil, err
			}
			if !access.CanWrite && !access.CanAdmin {
				return nil, repository.ErrForbidden
			}
		}
	}

	if err := s.itemRepo.SoftDelete(ctx, id); err != nil {
		s.logger.Error("failed to delete organization item", "item_id", id, "error", err)
		return nil, fmt.Errorf("failed to delete item: %w", err)
	}

	s.logger.Info("organization item deleted", "item_id", id, "user_id", userID)
	return item, nil
}

func (s *organizationItemService) GetCollectionAccess(ctx context.Context, orgID, userID, collectionID uint) (*authz.CollectionAccess, error) {
	orgUser, err := s.orgUserRepo.GetActiveByOrgAndUser(ctx, orgID, userID)
	if err != nil {
		return nil, repository.ErrForbidden
	}

	return authz.ComputeCollectionAccess(
		ctx,
		orgUser,
		collectionID,
		s.collectionUserRepo,
		s.collectionTeamRepo,
		s.teamUserRepo,
	)
}

// GetAutofillSecret returns the full item data for autofill purposes,
// bypassing hide_passwords redaction. Access is still checked.
func (s *organizationItemService) GetAutofillSecret(ctx context.Context, itemID, userID uint) (*domain.OrganizationItem, error) {
	item, err := s.itemRepo.GetByID(ctx, itemID)
	if err != nil {
		return nil, fmt.Errorf("item not found: %w", err)
	}

	orgUser, err := s.orgUserRepo.GetActiveByOrgAndUser(ctx, item.OrganizationID, userID)
	if err != nil {
		return nil, repository.ErrForbidden
	}
	allowed, err := s.featureService.CanAutofill(ctx, item.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to verify autofill entitlement: %w", err)
	}
	if !allowed {
		return nil, repository.ErrForbidden
	}

	if orgUser.IsAdmin() || orgUser.AccessAll {
		return item, nil
	}

	if item.CollectionID == nil {
		if item.CreatedByUserID != userID {
			return nil, repository.ErrForbidden
		}
		return item, nil
	}

	access, err := authz.ComputeCollectionAccess(
		ctx,
		orgUser,
		*item.CollectionID,
		s.collectionUserRepo,
		s.collectionTeamRepo,
		s.teamUserRepo,
	)
	if err != nil {
		return nil, err
	}
	if !access.CanRead {
		return nil, repository.ErrForbidden
	}

	return item, nil
}
