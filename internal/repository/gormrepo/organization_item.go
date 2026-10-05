package gormrepo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"gorm.io/gorm"
)

type organizationItemRepository struct {
	db *gorm.DB
}

// NewOrganizationItemRepository creates a new organization item repository
func NewOrganizationItemRepository(db *gorm.DB) repository.OrganizationItemRepository {
	return &organizationItemRepository{db: db}
}

func (r *organizationItemRepository) Create(ctx context.Context, item *domain.OrganizationItem) error {
	// Generate UUID if not set
	if item.UUID == uuid.Nil {
		item.UUID = uuid.New()
	}

	return dbFromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		revision, err := incrementOrganizationRevision(tx, item.OrganizationID)
		if err != nil {
			return err
		}
		item.Revision = revision
		query := tx
		if item.SupportID == 0 {
			query = query.Omit("SupportID")
		}
		return query.Create(item).Error
	})
}

func (r *organizationItemRepository) GetByID(ctx context.Context, id uint) (*domain.OrganizationItem, error) {
	var item domain.OrganizationItem
	err := dbFromContext(ctx, r.db).
		Preload("Organization").
		Preload("Collection").
		Preload("CreatedBy").
		Where("id = ? AND deleted_at IS NULL", id).
		First(&item).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &item, nil
}

func (r *organizationItemRepository) GetByUUID(ctx context.Context, uuidStr string) (*domain.OrganizationItem, error) {
	var item domain.OrganizationItem
	err := dbFromContext(ctx, r.db).
		Preload("Organization").
		Preload("Collection").
		Preload("CreatedBy").
		Where("uuid = ? AND deleted_at IS NULL", uuidStr).
		First(&item).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &item, nil
}

func (r *organizationItemRepository) GetBySupportID(ctx context.Context, supportID int64) (*domain.OrganizationItem, error) {
	var item domain.OrganizationItem
	err := dbFromContext(ctx, r.db).
		Preload("Organization").
		Preload("Collection").
		Preload("CreatedBy").
		Where("support_id = ? AND deleted_at IS NULL", supportID).
		First(&item).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &item, nil
}

func (r *organizationItemRepository) ListByOrganization(ctx context.Context, filter repository.OrganizationItemFilter) ([]*domain.OrganizationItem, int64, error) {
	var items []*domain.OrganizationItem
	var total int64

	// Base query
	query := dbFromContext(ctx, r.db).Model(&domain.OrganizationItem{}).Where("deleted_at IS NULL")

	// Apply organization filter (required)
	query = query.Where("organization_id = ?", filter.OrganizationID)

	if filter.RestrictToCollections {
		if len(filter.AllowedCollectionIDs) == 0 {
			query = query.Where("1 = 0")
		} else if filter.AllowOrgWide {
			query = query.Where("collection_id IS NULL OR collection_id IN ?", filter.AllowedCollectionIDs)
		} else {
			query = query.Where("collection_id IN ?", filter.AllowedCollectionIDs)
		}
	}

	// Apply collection filter
	if filter.CollectionID != nil {
		query = query.Where("collection_id = ?", *filter.CollectionID)
	}

	// Apply item type filter
	if filter.ItemType != nil {
		query = query.Where("item_type = ?", *filter.ItemType)
	}

	// Apply favorite filter
	if filter.IsFavorite != nil {
		query = query.Where("is_favorite = ?", *filter.IsFavorite)
	}

	// Apply folder filter
	if filter.FolderID != nil {
		query = query.Where("folder_id = ?", *filter.FolderID)
	}

	// Apply auto-fill filter
	if filter.AutoFill != nil {
		query = query.Where("auto_fill = ?", *filter.AutoFill)
	}

	// Apply auto-login filter
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

	// Filter by tags
	if len(filter.Tags) > 0 {
		for _, tag := range filter.Tags {
			query = query.Where("metadata->'tags' @> ?", fmt.Sprintf(`["%s"]`, tag))
		}
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Pagination
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 {
		filter.PerPage = 50
	}
	if filter.PerPage > 5000 {
		filter.PerPage = 5000
	}

	offset := (filter.Page - 1) * filter.PerPage
	query = query.Offset(offset).Limit(filter.PerPage)

	// Order by
	query = query.Order("created_at DESC")

	// Preload associations
	query = query.Preload("Collection").Preload("CreatedBy")

	// Execute query
	if err := query.Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r *organizationItemRepository) ListV2(ctx context.Context, filter repository.OrganizationItemV2Filter) ([]*domain.OrganizationItem, int64, error) {
	db := dbFromContext(ctx, r.db)
	var revision int64
	if err := db.Model(&domain.Organization{}).
		Select("revision").
		Where("id = ? AND is_active = true AND deleted_at IS NULL", filter.OrganizationID).
		Scan(&revision).Error; err != nil {
		return nil, 0, err
	}

	query := db.Unscoped().
		Model(&domain.OrganizationItem{}).
		Where("organization_id = ?", filter.OrganizationID)
	if filter.SinceRevision > 0 {
		query = query.Where("revision > ?", filter.SinceRevision)
	}
	if filter.UpToRevision > 0 {
		revision = filter.UpToRevision
	}
	query = query.Where("revision <= ?", revision)
	if filter.AfterRevision > 0 || filter.AfterID > 0 {
		query = query.Where("(revision, id) > (?, ?)", filter.AfterRevision, filter.AfterID)
	}
	if filter.RestrictToCollections {
		if len(filter.AllowedCollectionIDs) == 0 {
			query = query.Where("1 = 0")
		} else {
			query = query.Where("collection_id IN ?", filter.AllowedCollectionIDs)
		}
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	var items []*domain.OrganizationItem
	if err := query.
		Order("revision ASC, id ASC").
		Limit(limit + 1).
		Preload("Collection").
		Preload("CreatedBy").
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, revision, nil
}

func (r *organizationItemRepository) ListByCollection(ctx context.Context, collectionID uint) ([]*domain.OrganizationItem, error) {
	var items []*domain.OrganizationItem
	err := dbFromContext(ctx, r.db).
		Preload("CreatedBy").
		Where("collection_id = ? AND deleted_at IS NULL", collectionID).
		Order("created_at DESC").
		Find(&items).Error

	if err != nil {
		return nil, err
	}
	return items, nil
}

func (r *organizationItemRepository) MoveItemsToCollection(ctx context.Context, fromCollectionID uint, toCollectionID uint) error {
	return dbFromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		var items []domain.OrganizationItem
		if err := tx.Select("id", "organization_id").
			Where("collection_id = ? AND deleted_at IS NULL", fromCollectionID).
			Order("id ASC").
			Find(&items).Error; err != nil {
			return err
		}
		for _, item := range items {
			revision, err := incrementOrganizationRevision(tx, item.OrganizationID)
			if err != nil {
				return err
			}
			if err := tx.Model(&domain.OrganizationItem{}).
				Where("id = ?", item.ID).
				Updates(map[string]any{"collection_id": toCollectionID, "revision": revision}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *organizationItemRepository) CountByOrganizationID(ctx context.Context, orgID uint) (int, error) {
	var count int64
	err := dbFromContext(ctx, r.db).
		Model(&domain.OrganizationItem{}).
		Where("organization_id = ? AND deleted_at IS NULL", orgID).
		Count(&count).Error
	return int(count), err
}

func (r *organizationItemRepository) Update(ctx context.Context, item *domain.OrganizationItem) error {
	// Clear associations
	item.Organization = nil
	item.Collection = nil
	item.CreatedBy = nil

	return dbFromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		revision, err := incrementOrganizationRevision(tx, item.OrganizationID)
		if err != nil {
			return err
		}
		item.Revision = revision
		return tx.Save(item).Error
	})
}

func (r *organizationItemRepository) Delete(ctx context.Context, id uint) error {
	// Hard delete
	return dbFromContext(ctx, r.db).Unscoped().Delete(&domain.OrganizationItem{}, id).Error
}

func (r *organizationItemRepository) SoftDelete(ctx context.Context, id uint) error {
	return dbFromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		var item domain.OrganizationItem
		if err := tx.Select("id", "organization_id").Where("id = ? AND deleted_at IS NULL", id).First(&item).Error; err != nil {
			return err
		}
		revision, err := incrementOrganizationRevision(tx, item.OrganizationID)
		if err != nil {
			return err
		}
		return tx.Model(&domain.OrganizationItem{}).
			Where("id = ?", id).
			Updates(map[string]any{"deleted_at": time.Now().UTC(), "revision": revision}).Error
	})
}

func (r *organizationItemRepository) HardDelete(ctx context.Context, id uint) error {
	return dbFromContext(ctx, r.db).Unscoped().Delete(&domain.OrganizationItem{}, id).Error
}

func (r *organizationItemRepository) ClearCreatorByUserID(ctx context.Context, userID uint) error {
	return dbFromContext(ctx, r.db).
		Model(&domain.OrganizationItem{}).
		Where("created_by_user_id = ?", userID).
		Update("created_by_user_id", nil).Error
}

func incrementOrganizationRevision(tx *gorm.DB, organizationID uint) (int64, error) {
	var revision int64
	err := tx.Raw(`
UPDATE organizations
SET revision = revision + 1
WHERE id = ? AND is_active = true AND deleted_at IS NULL
RETURNING revision
`, organizationID).Scan(&revision).Error
	if err != nil {
		return 0, err
	}
	if revision == 0 {
		return 0, repository.ErrNotFound
	}
	return revision, nil
}
