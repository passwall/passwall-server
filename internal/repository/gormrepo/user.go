package gormrepo

import (
	"context"
	"errors"
	"fmt"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/pkg/database"
	"gorm.io/gorm"
)

type userRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *gorm.DB) repository.UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) GetByID(ctx context.Context, id uint) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).Preload("Role.Permissions").Where("id = ?", id).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetByUUID(ctx context.Context, uuid string) (*domain.User, error) {
	var user domain.User

	err := r.db.WithContext(ctx).Preload("Role.Permissions").Where("uuid = ?", uuid).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User

	// Only get non-deleted users (hard delete won't return anything anyway)
	err := r.db.WithContext(ctx).Preload("Role.Permissions").Where("email = ?", email).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetBySchema(ctx context.Context, schema string) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).Preload("Role.Permissions").Where("schema = ?", schema).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) List(ctx context.Context, filter repository.ListFilter) ([]*domain.User, *repository.ListResult, error) {
	var users []*domain.User
	var total int64

	query := r.db.WithContext(ctx).Model(&domain.User{}).Preload("Role")

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, nil, err
	}

	// Apply filters
	if filter.Search != "" {
		searchPattern := "%" + filter.Search + "%"
		query = query.Where("name LIKE ? OR email LIKE ? OR role LIKE ?",
			searchPattern, searchPattern, searchPattern)
	}

	// Count filtered
	var filtered int64
	if err := query.Count(&filtered).Error; err != nil {
		return nil, nil, err
	}

	// Apply pagination
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
		if filter.Offset > 0 {
			query = query.Offset(filter.Offset)
		}
	}

	// Apply sorting with whitelist protection against ORDER BY injection
	if filter.Sort != "" && filter.Order != "" {
		// Whitelist of allowed columns for sorting
		allowedSortColumns := []string{"id", "name", "email", "role", "created_at", "updated_at"}

		// Validate order direction
		if err := database.ValidateOrderDirection(filter.Order); err == nil {
			// Check if sort column is in whitelist
			if database.IsAllowedSortColumn(filter.Sort, allowedSortColumns) {
				query = query.Order(filter.Sort + " " + filter.Order)
			}
		}
	}

	err := query.Find(&users).Error
	if err != nil {
		return nil, nil, err
	}

	result := &repository.ListResult{
		Total:    total,
		Filtered: filtered,
	}

	return users, result, nil
}

func (r *userRepository) Create(ctx context.Context, user *domain.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *userRepository) Update(ctx context.Context, user *domain.User) error {
	// Note: FullSaveAssociations is set to false in GORM config, but we still
	// clear the Role pointer as a defense-in-depth measure to prevent any
	// potential issues with preloaded associations.
	user.Role = nil

	return r.db.WithContext(ctx).Save(user).Error
}

func (r *userRepository) Delete(ctx context.Context, id uint, schema string) error {
	// Drop schema with proper validation and sanitization
	if schema != "" && schema != "public" {
		// Validate schema name to prevent SQL injection
		if err := database.ValidateSchemaName(schema); err != nil {
			// Log validation error but continue to delete user
		} else {
			// Safely quote the schema identifier
			safeSchema := database.SanitizeIdentifier(schema)
			dropSQL := fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", safeSchema)
			if err := r.db.WithContext(ctx).Exec(dropSQL).Error; err != nil {
				// Log error but continue to delete user
			}
		}
	}
	// Hard delete user (not soft delete) to allow re-registration with same email
	return r.db.WithContext(ctx).Unscoped().Delete(&domain.User{}, id).Error
}

func (r *userRepository) Migrate() error {
	return r.db.AutoMigrate(&domain.User{})
}

func (r *userRepository) CreateSchema(schema string) error {
	if schema != "" && schema != "public" {
		// Validate schema name to prevent SQL injection
		if err := database.ValidateSchemaName(schema); err != nil {
			return fmt.Errorf("invalid schema name: %w", err)
		}

		// Safely quote the schema identifier
		safeSchema := database.SanitizeIdentifier(schema)
		createSQL := fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", safeSchema)
		return r.db.Exec(createSQL).Error
	}
	return nil
}

// MigrateUserSchema creates all necessary tables in a user's schema
func (r *userRepository) MigrateUserSchema(schema string) error {
	if schema == "" || schema == "public" {
		return fmt.Errorf("invalid schema name: %s", schema)
	}

	// Validate schema name to prevent SQL injection
	if err := database.ValidateSchemaName(schema); err != nil {
		return fmt.Errorf("invalid schema name: %w", err)
	}

	// Safely quote the schema identifier
	safeSchema := database.SanitizeIdentifier(schema)

	// Set the search path to the user's schema
	setPathSQL := fmt.Sprintf("SET search_path TO %s", safeSchema)
	if err := r.db.Exec(setPathSQL).Error; err != nil {
		return fmt.Errorf("failed to set search path: %w", err)
	}

	// Create items table (modern flexible items)
	if err := r.db.AutoMigrate(&domain.Item{}); err != nil {
		_ = r.db.Exec("SET search_path TO public").Error
		return fmt.Errorf("failed to create items table: %w", err)
	}

	// Create sequences for sync and support IDs
	if err := r.db.Exec("CREATE SEQUENCE IF NOT EXISTS items_revision_seq").Error; err != nil {
		_ = r.db.Exec("SET search_path TO public").Error
		return fmt.Errorf("failed to create revision sequence: %w", err)
	}

	if err := r.db.Exec(`
		CREATE SEQUENCE IF NOT EXISTS items_support_id_seq
		START WITH 1000000000000000000
		INCREMENT BY 1
		NO MAXVALUE
		CACHE 100
	`).Error; err != nil {
		_ = r.db.Exec("SET search_path TO public").Error
		return fmt.Errorf("failed to create support_id sequence: %w", err)
	}

	// Create trigger for auto-updating revision and support_id
	if err := r.db.Exec(`
		CREATE OR REPLACE FUNCTION update_item_metadata()
		RETURNS TRIGGER AS $$
		BEGIN
			IF TG_OP = 'INSERT' THEN
				NEW.support_id = nextval('items_support_id_seq');
			END IF;
			
			NEW.revision = nextval('items_revision_seq');
			NEW.updated_at = NOW();
			
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql
	`).Error; err != nil {
		_ = r.db.Exec("SET search_path TO public").Error
		return fmt.Errorf("failed to create trigger function: %w", err)
	}

	// Drop trigger first (separate exec)
	if err := r.db.Exec("DROP TRIGGER IF EXISTS item_metadata_trigger ON items").Error; err != nil {
		_ = r.db.Exec("SET search_path TO public").Error
		return fmt.Errorf("failed to drop trigger: %w", err)
	}

	// Create trigger (separate exec)
	if err := r.db.Exec(`
		CREATE TRIGGER item_metadata_trigger
			BEFORE INSERT OR UPDATE ON items
			FOR EACH ROW
			EXECUTE FUNCTION update_item_metadata()
	`).Error; err != nil {
		_ = r.db.Exec("SET search_path TO public").Error
		return fmt.Errorf("failed to create trigger: %w", err)
	}

	// Create indexes for performance
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_items_support_id ON items(support_id)",
		"CREATE INDEX IF NOT EXISTS idx_items_revision ON items(revision DESC)",
		"CREATE INDEX IF NOT EXISTS idx_items_type ON items(item_type) WHERE deleted_at IS NULL",
		"CREATE INDEX IF NOT EXISTS idx_items_favorite ON items(is_favorite) WHERE deleted_at IS NULL AND is_favorite = true",
		"CREATE INDEX IF NOT EXISTS idx_items_autofill ON items(auto_fill) WHERE item_type = 1 AND auto_fill = true AND deleted_at IS NULL",
		"CREATE INDEX IF NOT EXISTS idx_items_metadata_gin ON items USING gin(metadata jsonb_path_ops)",
	}

	for _, indexSQL := range indexes {
		if err := r.db.Exec(indexSQL).Error; err != nil {
			_ = r.db.Exec("SET search_path TO public").Error
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	// NOTE: Legacy tables removed - all item types now use the items table

	// Reset search path to public
	if err := r.db.Exec("SET search_path TO public").Error; err != nil {
		return fmt.Errorf("failed to reset search path: %w", err)
	}

	return nil
}
