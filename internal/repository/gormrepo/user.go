package gormrepo

import (
	"context"
	"errors"

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
	err := dbFromContext(ctx, r.db).Preload("Role.Permissions").Where("id = ?", id).First(&user).Error
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

	err := dbFromContext(ctx, r.db).Preload("Role.Permissions").Where("uuid = ?", uuid).First(&user).Error
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
	err := dbFromContext(ctx, r.db).Preload("Role.Permissions").Where("LOWER(email) = LOWER(?)", email).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) CountByRoleID(ctx context.Context, roleID uint) (int64, error) {
	var count int64
	err := dbFromContext(ctx, r.db).Model(&domain.User{}).Where("role_id = ?", roleID).Count(&count).Error
	return count, err
}

func (r *userRepository) List(ctx context.Context, filter repository.ListFilter) ([]*domain.User, *repository.ListResult, error) {
	var users []*domain.User
	var total int64

	query := dbFromContext(ctx, r.db).Model(&domain.User{}).Preload("Role")

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, nil, err
	}

	// Apply filters
	if filter.Search != "" {
		searchPattern := "%" + filter.Search + "%"
		query = query.Where("LOWER(name) LIKE LOWER(?) OR LOWER(email) LIKE LOWER(?)",
			searchPattern, searchPattern)
	}

	if filter.RoleID > 0 {
		query = query.Where("role_id = ?", filter.RoleID)
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
	return dbFromContext(ctx, r.db).Create(user).Error
}

func (r *userRepository) Update(ctx context.Context, user *domain.User) error {
	// Note: FullSaveAssociations is set to false in GORM config, but we still
	// clear the Role pointer as a defense-in-depth measure to prevent any
	// potential issues with preloaded associations.
	user.Role = nil

	return dbFromContext(ctx, r.db).Save(user).Error
}

func (r *userRepository) Delete(ctx context.Context, id uint) error {
	// Hard delete user (not soft delete) to allow re-registration with same email
	return dbFromContext(ctx, r.db).Unscoped().Delete(&domain.User{}, id).Error
}

func (r *userRepository) Migrate() error {
	return r.db.AutoMigrate(&domain.User{})
}
