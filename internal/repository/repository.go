package repository

import (
	"context"
	"errors"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	uuid "github.com/satori/go.uuid"
)

// Common errors
var (
	ErrNotFound      = errors.New("record not found")
	ErrAlreadyExists = errors.New("record already exists")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrInvalidInput  = errors.New("invalid input")
	ErrForbidden     = errors.New("operation forbidden")
)

// ListFilter represents common list filter parameters
type ListFilter struct {
	Search string
	Limit  int
	Offset int
	Sort   string
	Order  string
}

// ListResult represents list query results with pagination info
type ListResult struct {
	Total    int64
	Filtered int64
}

// NOTE: Legacy repository interfaces removed (Login, BankAccount, CreditCard, Note, Email, Server)
// All item types now use ItemRepository with flexible items architecture

// UserRepository defines user data access methods
type UserRepository interface {
	GetByID(ctx context.Context, id uint) (*domain.User, error)
	GetByUUID(ctx context.Context, uuid string) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetBySchema(ctx context.Context, schema string) (*domain.User, error)
	List(ctx context.Context, filter ListFilter) ([]*domain.User, *ListResult, error)
	Create(ctx context.Context, user *domain.User) error
	Update(ctx context.Context, user *domain.User) error
	Delete(ctx context.Context, id uint, schema string) error
	Migrate() error
	CreateSchema(schema string) error
	MigrateUserSchema(schema string) error
}

// TokenRepository defines token data access methods
type TokenRepository interface {
	Create(ctx context.Context, userID int, tokenUUID uuid.UUID, token string, expiresAt time.Time) error
	GetByUUID(ctx context.Context, uuid string) (*domain.Token, error)
	Delete(ctx context.Context, userID int) error
	DeleteByUUID(ctx context.Context, uuid string) error
	DeleteExpired(ctx context.Context) (int64, error)
	Cleanup(ctx context.Context) error
	Migrate() error
}

// RoleRepository defines role data access methods
type RoleRepository interface {
	GetByID(ctx context.Context, id uint) (*domain.Role, error)
	GetByName(ctx context.Context, name string) (*domain.Role, error)
	List(ctx context.Context) ([]*domain.Role, error)
	GetPermissions(ctx context.Context, roleID uint) ([]string, error)
	Migrate() error
}

// PermissionRepository defines permission data access methods
type PermissionRepository interface {
	GetByID(ctx context.Context, id uint) (*domain.Permission, error)
	GetByName(ctx context.Context, name string) (*domain.Permission, error)
	List(ctx context.Context) ([]*domain.Permission, error)
	Migrate() error
}

// VerificationRepository defines verification code data access methods
type VerificationRepository interface {
	Create(ctx context.Context, code *domain.VerificationCode) error
	GetByEmailAndCode(ctx context.Context, email, code string) (*domain.VerificationCode, error)
	DeleteByEmail(ctx context.Context, email string) error
	DeleteExpired(ctx context.Context) (int64, error)
	Migrate() error
}
