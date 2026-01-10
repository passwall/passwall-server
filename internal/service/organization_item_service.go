package service

import (
	"context"
	"fmt"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	uuid "github.com/satori/go.uuid"
)

type organizationItemService struct {
	itemRepo       repository.OrganizationItemRepository
	collectionRepo repository.CollectionRepository
	orgUserRepo    repository.OrganizationUserRepository
	logger         Logger
}

// NewOrganizationItemService creates a new organization item service
func NewOrganizationItemService(
	itemRepo repository.OrganizationItemRepository,
	collectionRepo repository.CollectionRepository,
	orgUserRepo repository.OrganizationUserRepository,
	logger Logger,
) OrganizationItemService {
	return &organizationItemService{
		itemRepo:       itemRepo,
		collectionRepo: collectionRepo,
		orgUserRepo:    orgUserRepo,
		logger:         logger,
	}
}

// CreateOrgItemRequest for creating organization items
type CreateOrgItemRequest struct {
	CollectionID *uint               `json:"collection_id,omitempty"`
	ItemType     domain.ItemType     `json:"item_type" validate:"required"`
	Data         string              `json:"data" validate:"required"` // Encrypted with Org Key
	Metadata     domain.ItemMetadata `json:"metadata" validate:"required"`
	IsFavorite   bool                `json:"is_favorite"`
	Reprompt     bool                `json:"reprompt"`
}

// UpdateOrgItemRequest for updating organization items
type UpdateOrgItemRequest struct {
	CollectionID *uint                `json:"collection_id,omitempty"`
	Data         *string              `json:"data,omitempty"`
	Metadata     *domain.ItemMetadata `json:"metadata,omitempty"`
	IsFavorite   *bool                `json:"is_favorite,omitempty"`
	Reprompt     *bool                `json:"reprompt,omitempty"`
}

func (s *organizationItemService) Create(ctx context.Context, orgID, userID uint, req *CreateOrgItemRequest) (*domain.OrganizationItem, error) {
	// Check if user is member of organization
	orgUser, err := s.orgUserRepo.GetByOrgAndUser(ctx, orgID, userID)
	if err != nil {
		return nil, repository.ErrForbidden
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
		
		// Check collection access (simplified - admins and access_all users can create)
		if !orgUser.IsAdmin() && !orgUser.AccessAll {
			// TODO: Check collection_users for write permission
			return nil, repository.ErrForbidden
		}
	}

	// Validate item type
	if !req.ItemType.IsValid() {
		return nil, fmt.Errorf("invalid item type: %d", req.ItemType)
	}

	// Create item
	item := &domain.OrganizationItem{
		UUID:            uuid.NewV4(),
		OrganizationID:  orgID,
		CollectionID:    req.CollectionID,
		ItemType:        req.ItemType,
		Data:            req.Data, // Already encrypted with Org Key
		Metadata:        req.Metadata,
		IsFavorite:      req.IsFavorite,
		Reprompt:        req.Reprompt,
		CreatedByUserID: userID,
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
	_, err = s.orgUserRepo.GetByOrgAndUser(ctx, item.OrganizationID, userID)
	if err != nil {
		return nil, repository.ErrForbidden
	}

	// TODO: Check collection access if item is in a collection

	return item, nil
}

func (s *organizationItemService) ListByCollection(ctx context.Context, collectionID, userID uint) ([]*domain.OrganizationItem, error) {
	// Get collection
	collection, err := s.collectionRepo.GetByID(ctx, collectionID)
	if err != nil {
		return nil, fmt.Errorf("collection not found: %w", err)
	}

	// Check if user has access to organization
	orgUser, err := s.orgUserRepo.GetByOrgAndUser(ctx, collection.OrganizationID, userID)
	if err != nil {
		return nil, repository.ErrForbidden
	}

	// Check access (simplified - admins and access_all can view)
	if !orgUser.IsAdmin() && !orgUser.AccessAll {
		// TODO: Check collection_users for read permission
		return nil, repository.ErrForbidden
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
	orgUser, err := s.orgUserRepo.GetByOrgAndUser(ctx, item.OrganizationID, userID)
	if err != nil {
		return nil, repository.ErrForbidden
	}

	// Only admins, creators, or users with write permission can update
	if !orgUser.IsAdmin() && item.CreatedByUserID != userID {
		// TODO: Check collection write permission
		return nil, repository.ErrForbidden
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
	if req.Reprompt != nil {
		item.Reprompt = *req.Reprompt
	}

	if err := s.itemRepo.Update(ctx, item); err != nil {
		s.logger.Error("failed to update organization item", "item_id", id, "error", err)
		return nil, fmt.Errorf("failed to update item: %w", err)
	}

	s.logger.Info("organization item updated", "item_id", id, "user_id", userID)
	return item, nil
}

func (s *organizationItemService) Delete(ctx context.Context, id, userID uint) error {
	// Get item
	item, err := s.itemRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("item not found: %w", err)
	}

	// Check access
	orgUser, err := s.orgUserRepo.GetByOrgAndUser(ctx, item.OrganizationID, userID)
	if err != nil {
		return repository.ErrForbidden
	}

	// Only admins or creator can delete
	if !orgUser.IsAdmin() && item.CreatedByUserID != userID {
		return repository.ErrForbidden
	}

	if err := s.itemRepo.SoftDelete(ctx, id); err != nil {
		s.logger.Error("failed to delete organization item", "item_id", id, "error", err)
		return fmt.Errorf("failed to delete item: %w", err)
	}

	s.logger.Info("organization item deleted", "item_id", id, "user_id", userID)
	return nil
}

