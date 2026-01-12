package repository

import (
	"context"

	"github.com/passwall/passwall-server/internal/domain"
)

// ItemRepository defines the interface for item data access
type ItemRepository interface {
	Create(ctx context.Context, schema string, item *domain.Item) error
	FindByID(ctx context.Context, schema string, id uint) (*domain.Item, error)
	FindByUUID(ctx context.Context, schema string, uuid string) (*domain.Item, error)
	FindBySupportID(ctx context.Context, schema string, supportID int64) (*domain.Item, error)
	FindAll(ctx context.Context, schema string, filter ItemFilter) ([]*domain.Item, int64, error)
	Update(ctx context.Context, schema string, item *domain.Item) error
	Delete(ctx context.Context, schema string, id uint) error
	HardDelete(ctx context.Context, schema string, id uint) error
}

// ItemFilter - Filter options for listing items
type ItemFilter struct {
	ItemType   *domain.ItemType
	IsFavorite *bool
	FolderID   *uint
	Tags       []string
	Search     string
	// URIHints filters by metadata.uri_hint (domain only, no paths).
	// If multiple are provided, the result matches ANY of them.
	URIHints  []string
	AutoFill  *bool
	AutoLogin *bool
	Page      int
	PerPage   int
}
