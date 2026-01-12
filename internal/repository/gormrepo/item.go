package gormrepo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/pkg/database"
	"gorm.io/gorm"
)

type itemRepository struct {
	db *gorm.DB
}

// NewItemRepository creates a new item repository
func NewItemRepository(db *gorm.DB) repository.ItemRepository {
	return &itemRepository{db: db}
}

func (r *itemRepository) Create(ctx context.Context, schema string, item *domain.Item) error {
	// Use a transaction + SET LOCAL so search_path never leaks to pooled connections.
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := database.ValidateSchemaName(schema); err != nil {
			return err
		}
		safeSchema := database.SanitizeIdentifier(schema)
		if err := tx.Exec(fmt.Sprintf("SET LOCAL search_path TO %s", safeSchema)).Error; err != nil {
			return err
		}
		return tx.Create(item).Error
	})
}

func (r *itemRepository) FindByID(ctx context.Context, schema string, id uint) (*domain.Item, error) {
	var item domain.Item

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := database.ValidateSchemaName(schema); err != nil {
			return err
		}
		safeSchema := database.SanitizeIdentifier(schema)
		if err := tx.Exec(fmt.Sprintf("SET LOCAL search_path TO %s", safeSchema)).Error; err != nil {
			return err
		}
		return tx.Where("id = ? AND deleted_at IS NULL", id).First(&item).Error
	})

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}

	return &item, nil
}

func (r *itemRepository) FindByUUID(ctx context.Context, schema string, uuidStr string) (*domain.Item, error) {
	var item domain.Item

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := database.ValidateSchemaName(schema); err != nil {
			return err
		}
		safeSchema := database.SanitizeIdentifier(schema)
		if err := tx.Exec(fmt.Sprintf("SET LOCAL search_path TO %s", safeSchema)).Error; err != nil {
			return err
		}
		return tx.Where("uuid = ? AND deleted_at IS NULL", uuidStr).First(&item).Error
	})

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}

	return &item, nil
}

func (r *itemRepository) FindBySupportID(ctx context.Context, schema string, supportID int64) (*domain.Item, error) {
	var item domain.Item

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := database.ValidateSchemaName(schema); err != nil {
			return err
		}
		safeSchema := database.SanitizeIdentifier(schema)
		if err := tx.Exec(fmt.Sprintf("SET LOCAL search_path TO %s", safeSchema)).Error; err != nil {
			return err
		}
		return tx.Where("support_id = ? AND deleted_at IS NULL", supportID).First(&item).Error
	})

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}

	return &item, nil
}

func (r *itemRepository) FindAll(ctx context.Context, schema string, filter repository.ItemFilter) ([]*domain.Item, int64, error) {
	var items []*domain.Item
	var total int64

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Set schema using SET LOCAL so it cannot leak outside this transaction.
		if err := database.ValidateSchemaName(schema); err != nil {
			return err
		}
		safeSchema := database.SanitizeIdentifier(schema)
		if err := tx.Exec(fmt.Sprintf("SET LOCAL search_path TO %s", safeSchema)).Error; err != nil {
			return err
		}

		// Base query
		query := tx.Model(&domain.Item{}).Where("deleted_at IS NULL")

		// Apply filters
		if filter.ItemType != nil {
			query = query.Where("item_type = ?", *filter.ItemType)
		}

		if filter.IsFavorite != nil {
			query = query.Where("is_favorite = ?", *filter.IsFavorite)
		}

		if filter.FolderID != nil {
			query = query.Where("folder_id = ?", *filter.FolderID)
		}

		if filter.AutoFill != nil {
			query = query.Where("auto_fill = ?", *filter.AutoFill)
		}

		if filter.AutoLogin != nil {
			query = query.Where("auto_login = ?", *filter.AutoLogin)
		}

		// Search in metadata
		if filter.Search != "" {
			searchPattern := "%" + filter.Search + "%"
			query = query.Where(
				"metadata->>'name' ILIKE ? OR metadata->>'uri_hint' ILIKE ?",
				searchPattern,
				searchPattern,
			)
		}

		// Filter by uri_hint (domain only)
		// Matches exact domain OR any subdomain of that domain.
		if len(filter.URIHints) > 0 {
			clauses := make([]string, 0, len(filter.URIHints))
			args := make([]interface{}, 0, len(filter.URIHints)*2)
			for _, hint := range filter.URIHints {
				if hint == "" {
					continue
				}
				clauses = append(clauses, "(metadata->>'uri_hint' = ? OR metadata->>'uri_hint' LIKE ?)")
				args = append(args, hint, "%."+hint)
			}
			if len(clauses) > 0 {
				query = query.Where(strings.Join(clauses, " OR "), args...)
			}
		}

		// Filter by tags
		if len(filter.Tags) > 0 {
			for _, tag := range filter.Tags {
				query = query.Where("metadata->'tags' @> ?", fmt.Sprintf(`["%s"]`, tag))
			}
		}

		// Count total
		if err := query.Count(&total).Error; err != nil {
			return err
		}

		// Pagination
		// Note: Validation is done at service layer, repository just applies the values
		offset := (filter.Page - 1) * filter.PerPage
		query = query.Offset(offset).Limit(filter.PerPage)

		// Order by
		query = query.Order("created_at DESC")

		// Execute query
		if err := query.Find(&items).Error; err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *itemRepository) Update(ctx context.Context, schema string, item *domain.Item) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := database.ValidateSchemaName(schema); err != nil {
			return err
		}
		safeSchema := database.SanitizeIdentifier(schema)
		if err := tx.Exec(fmt.Sprintf("SET LOCAL search_path TO %s", safeSchema)).Error; err != nil {
			return err
		}
		return tx.Save(item).Error
	})
}

func (r *itemRepository) Delete(ctx context.Context, schema string, id uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := database.ValidateSchemaName(schema); err != nil {
			return err
		}
		safeSchema := database.SanitizeIdentifier(schema)
		if err := tx.Exec(fmt.Sprintf("SET LOCAL search_path TO %s", safeSchema)).Error; err != nil {
			return err
		}
		return tx.Model(&domain.Item{}).Where("id = ?", id).Update("deleted_at", time.Now()).Error
	})
}

func (r *itemRepository) HardDelete(ctx context.Context, schema string, id uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := database.ValidateSchemaName(schema); err != nil {
			return err
		}
		safeSchema := database.SanitizeIdentifier(schema)
		if err := tx.Exec(fmt.Sprintf("SET LOCAL search_path TO %s", safeSchema)).Error; err != nil {
			return err
		}
		return tx.Unscoped().Delete(&domain.Item{}, id).Error
	})
}
