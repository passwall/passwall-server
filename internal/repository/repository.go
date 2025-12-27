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

// LoginRepository defines login data access methods
type LoginRepository interface {
	GetByID(ctx context.Context, id uint) (*domain.Login, error)
	List(ctx context.Context) ([]*domain.Login, error)
	Create(ctx context.Context, login *domain.Login) error
	Update(ctx context.Context, login *domain.Login) error
	Delete(ctx context.Context, id uint) error
	Migrate(schema string) error
}

// BankAccountRepository defines bank account data access methods
type BankAccountRepository interface {
	GetByID(ctx context.Context, id uint) (*domain.BankAccount, error)
	List(ctx context.Context) ([]*domain.BankAccount, error)
	Create(ctx context.Context, account *domain.BankAccount) error
	Update(ctx context.Context, account *domain.BankAccount) error
	Delete(ctx context.Context, id uint) error
	Migrate(schema string) error
}

// CreditCardRepository defines credit card data access methods
type CreditCardRepository interface {
	GetByID(ctx context.Context, id uint) (*domain.CreditCard, error)
	List(ctx context.Context) ([]*domain.CreditCard, error)
	Create(ctx context.Context, card *domain.CreditCard) error
	Update(ctx context.Context, card *domain.CreditCard) error
	Delete(ctx context.Context, id uint) error
	Migrate(schema string) error
}

// NoteRepository defines note data access methods
type NoteRepository interface {
	GetByID(ctx context.Context, id uint) (*domain.Note, error)
	List(ctx context.Context) ([]*domain.Note, error)
	Create(ctx context.Context, note *domain.Note) error
	Update(ctx context.Context, note *domain.Note) error
	Delete(ctx context.Context, id uint) error
	Migrate(schema string) error
}

// EmailRepository defines email data access methods
type EmailRepository interface {
	GetByID(ctx context.Context, id uint) (*domain.Email, error)
	List(ctx context.Context) ([]*domain.Email, error)
	Create(ctx context.Context, email *domain.Email) error
	Update(ctx context.Context, email *domain.Email) error
	Delete(ctx context.Context, id uint) error
	Migrate(schema string) error
}

// ServerRepository defines server data access methods
type ServerRepository interface {
	GetByID(ctx context.Context, id uint) (*domain.Server, error)
	List(ctx context.Context) ([]*domain.Server, error)
	Create(ctx context.Context, server *domain.Server) error
	Update(ctx context.Context, server *domain.Server) error
	Delete(ctx context.Context, id uint) error
	Migrate(schema string) error
}

// UserRepository defines user data access methods
type UserRepository interface {
	GetByID(ctx context.Context, id uint) (*domain.User, error)
	GetByUUID(ctx context.Context, uuid string) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByCredentials(ctx context.Context, email, masterPassword string) (*domain.User, error)
	GetBySchema(ctx context.Context, schema string) (*domain.User, error)
	List(ctx context.Context, filter ListFilter) ([]*domain.User, *ListResult, error)
	Create(ctx context.Context, user *domain.User) error
	Update(ctx context.Context, user *domain.User) error
	Delete(ctx context.Context, id uint, schema string) error
	Migrate() error
	CreateSchema(schema string) error
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
