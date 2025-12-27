package gormrepo

import (
	"context"
	"errors"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"golang.org/x/crypto/bcrypt"
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

func (r *userRepository) GetByCredentials(ctx context.Context, email, masterPassword string) (*domain.User, error) {
	user, err := r.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	// Compare the password with the bcrypt hash
	err = bcrypt.CompareHashAndPassword([]byte(user.MasterPassword), []byte(masterPassword))
	if err != nil {
		return nil, repository.ErrUnauthorized
	}

	return user, nil
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

	// Apply sorting
	if filter.Sort != "" && filter.Order != "" {
		query = query.Order(filter.Sort + " " + filter.Order)
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
	// Drop schema
	if schema != "" && schema != "public" {
		if err := r.db.WithContext(ctx).Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			// Log error but continue to delete user
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
		return r.db.Exec("CREATE SCHEMA IF NOT EXISTS " + schema).Error
	}
	return nil
}

