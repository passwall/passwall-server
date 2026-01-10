package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/pkg/constants"
	uuid "github.com/satori/go.uuid"
)

// ItemService interface for vault items
type ItemService interface {
	Create(ctx context.Context, schema string, req *CreateItemRequest) (*domain.Item, error)
	GetByID(ctx context.Context, schema string, id uint) (*domain.Item, error)
	GetByUUID(ctx context.Context, schema string, uuid string) (*domain.Item, error)
	GetBySupportID(ctx context.Context, schema string, supportID int64) (*domain.Item, error)
	List(ctx context.Context, schema string, filter repository.ItemFilter) (*ItemListResponse, error)
	Update(ctx context.Context, schema string, id uint, req *UpdateItemRequest) (*domain.Item, error)
	Delete(ctx context.Context, schema string, id uint) error
	HardDelete(ctx context.Context, schema string, id uint) error
}

type itemService struct {
	repo   repository.ItemRepository
	logger Logger
}

// NewItemService creates a new item service
func NewItemService(repo repository.ItemRepository, logger Logger) ItemService {
	return &itemService{
		repo:   repo,
		logger: logger,
	}
}

// CreateItemRequest - Request to create an item
type CreateItemRequest struct {
	ItemType   domain.ItemType     `json:"item_type" validate:"required"`
	Data       string              `json:"data" validate:"required"`
	Metadata   domain.ItemMetadata `json:"metadata" validate:"required"`
	IsFavorite bool                `json:"is_favorite"`
	FolderID   *uint               `json:"folder_id,omitempty"`
	Reprompt   bool                `json:"reprompt"`
	AutoFill   *bool               `json:"auto_fill,omitempty"`
	AutoLogin  *bool               `json:"auto_login,omitempty"`
}

// UpdateItemRequest - Request to update an item
type UpdateItemRequest struct {
	Data       *string              `json:"data,omitempty"`
	Metadata   *domain.ItemMetadata `json:"metadata,omitempty"`
	IsFavorite *bool                `json:"is_favorite,omitempty"`
	FolderID   *uint                `json:"folder_id,omitempty"`
	Reprompt   *bool                `json:"reprompt,omitempty"`
	AutoFill   *bool                `json:"auto_fill,omitempty"`
	AutoLogin  *bool                `json:"auto_login,omitempty"`
}

// ItemListResponse - Paginated response
type ItemListResponse struct {
	Items   []*domain.Item `json:"items"`
	Total   int64          `json:"total"`
	Page    int            `json:"page"`
	PerPage int            `json:"per_page"`
}

// Create implements ItemService
func (s *itemService) Create(ctx context.Context, schema string, req *CreateItemRequest) (*domain.Item, error) {
	// Validate item type
	if !req.ItemType.IsValid() {
		return nil, fmt.Errorf("invalid item type: %d", req.ItemType)
	}

	// Validate metadata
	if err := s.validateMetadata(req.Metadata); err != nil {
		return nil, fmt.Errorf("invalid metadata: %w", err)
	}

	// Set defaults for auto-fill/auto-login
	autoFill := true
	autoLogin := false
	if req.AutoFill != nil {
		autoFill = *req.AutoFill
	}
	if req.AutoLogin != nil {
		autoLogin = *req.AutoLogin
	}

	// Create item
	item := &domain.Item{
		UUID:       uuid.NewV4(),
		ItemType:   req.ItemType,
		Data:       req.Data,
		Metadata:   req.Metadata,
		IsFavorite: req.IsFavorite,
		FolderID:   req.FolderID,
		Reprompt:   req.Reprompt,
		AutoFill:   autoFill,
		AutoLogin:  autoLogin,
	}

	// Store in repository
	if err := s.repo.Create(ctx, schema, item); err != nil {
		s.logger.Error("failed to create item", "type", item.ItemType, "error", err)
		return nil, fmt.Errorf("failed to create item: %w", err)
	}

	s.logger.Info("item created",
		"id", item.ID,
		"support_id", item.FormatSupportID(),
		"type", item.ItemType.String(),
		"uuid", item.UUID)

	return item, nil
}

func (s *itemService) GetByID(ctx context.Context, schema string, id uint) (*domain.Item, error) {
	item, err := s.repo.FindByID(ctx, schema, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get item: %w", err)
	}

	return item, nil
}

func (s *itemService) GetByUUID(ctx context.Context, schema string, uuidStr string) (*domain.Item, error) {
	item, err := s.repo.FindByUUID(ctx, schema, uuidStr)
	if err != nil {
		return nil, fmt.Errorf("failed to get item: %w", err)
	}

	return item, nil
}

func (s *itemService) GetBySupportID(ctx context.Context, schema string, supportID int64) (*domain.Item, error) {
	item, err := s.repo.FindBySupportID(ctx, schema, supportID)
	if err != nil {
		return nil, fmt.Errorf("failed to get item: %w", err)
	}

	return item, nil
}

func (s *itemService) List(ctx context.Context, schema string, filter repository.ItemFilter) (*ItemListResponse, error) {
	// Validate and set pagination defaults
	// This is the SINGLE SOURCE OF TRUTH for pagination logic
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 {
		filter.PerPage = constants.DefaultPageSize
	}
	if filter.PerPage > constants.MaxPageSize {
		filter.PerPage = constants.MaxPageSize
	}

	// Fetch from repository
	items, total, err := s.repo.FindAll(ctx, schema, filter)
	if err != nil {
		s.logger.Error("failed to list items", "error", err)
		return nil, fmt.Errorf("failed to list items: %w", err)
	}

	return &ItemListResponse{
		Items:   items,
		Total:   total,
		Page:    filter.Page,
		PerPage: filter.PerPage,
	}, nil
}

func (s *itemService) Update(ctx context.Context, schema string, id uint, req *UpdateItemRequest) (*domain.Item, error) {
	// Get existing item
	item, err := s.repo.FindByID(ctx, schema, id)
	if err != nil {
		return nil, fmt.Errorf("item not found: %w", err)
	}

	// Update fields
	if req.Data != nil {
		item.Data = *req.Data
	}
	if req.Metadata != nil {
		if err := s.validateMetadata(*req.Metadata); err != nil {
			return nil, fmt.Errorf("invalid metadata: %w", err)
		}
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

	// Update in repository
	if err := s.repo.Update(ctx, schema, item); err != nil {
		s.logger.Error("failed to update item", "id", id, "error", err)
		return nil, fmt.Errorf("failed to update item: %w", err)
	}

	s.logger.Info("item updated", "id", item.ID, "uuid", item.UUID)
	return item, nil
}

func (s *itemService) Delete(ctx context.Context, schema string, id uint) error {
	if err := s.repo.Delete(ctx, schema, id); err != nil {
		s.logger.Error("failed to delete item", "id", id, "error", err)
		return fmt.Errorf("failed to delete item: %w", err)
	}

	s.logger.Info("item deleted (soft)", "id", id)
	return nil
}

func (s *itemService) HardDelete(ctx context.Context, schema string, id uint) error {
	if err := s.repo.HardDelete(ctx, schema, id); err != nil {
		s.logger.Error("failed to hard delete item", "id", id, "error", err)
		return fmt.Errorf("failed to hard delete item: %w", err)
	}

	s.logger.Info("item deleted (hard)", "id", id)
	return nil
}

// validateMetadata validates metadata fields
func (s *itemService) validateMetadata(metadata domain.ItemMetadata) error {
	// Name is required
	if metadata.Name == "" {
		return fmt.Errorf("metadata name is required")
	}

	// Name length check
	if len(metadata.Name) > 255 {
		return fmt.Errorf("metadata name too long (max 255 characters)")
	}

	// Check for email-like patterns (PII leak)
	if strings.Contains(metadata.Name, "@") {
		return fmt.Errorf("metadata name should not contain email addresses")
	}

	// URIHint should be domain only (no paths)
	if metadata.URIHint != "" {
		if strings.Contains(metadata.URIHint, "/") {
			return fmt.Errorf("uri_hint should be domain only (no paths)")
		}
		if len(metadata.URIHint) > 255 {
			return fmt.Errorf("uri_hint too long (max 255 characters)")
		}
	}

	// Tags validation
	if len(metadata.Tags) > 20 {
		return fmt.Errorf("too many tags (max 20)")
	}
	for _, tag := range metadata.Tags {
		if len(tag) > 50 {
			return fmt.Errorf("tag too long (max 50 characters)")
		}
		// Check for sensitive patterns in tags
		if containsSensitivePattern(tag) {
			return fmt.Errorf("tag contains sensitive pattern: %s", tag)
		}
	}

	return nil
}

// containsSensitivePattern checks for sensitive words in strings
func containsSensitivePattern(s string) bool {
	sensitive := []string{"password", "secret", "key", "pin", "ssn", "account"}
	lower := strings.ToLower(s)

	for _, word := range sensitive {
		if strings.Contains(lower, word) {
			return true
		}
	}

	return false
}
