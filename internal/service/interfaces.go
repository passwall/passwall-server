package service

import (
	"context"

	"github.com/passwall/passwall-server/internal/domain"
)

// Logger defines the logging interface
type Logger interface {
	Debug(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

// AuthService defines the business logic for authentication
type AuthService interface {
	SignUp(ctx context.Context, req *domain.SignUpRequest) (*domain.User, error)
	SignIn(ctx context.Context, creds *domain.Credentials) (*domain.AuthResponse, error)
	PreLogin(ctx context.Context, email string) (*domain.PreLoginResponse, error)
	ChangeMasterPassword(ctx context.Context, req *domain.ChangeMasterPasswordRequest) error
	RefreshToken(ctx context.Context, refreshToken string) (*domain.TokenDetails, error)
	ValidateToken(ctx context.Context, token string) (*domain.TokenClaims, error)
	SignOut(ctx context.Context, userID int) error
	ValidateSchema(ctx context.Context, schema string) error
}

// NOTE: Legacy service interfaces removed (Login, BankAccount, CreditCard, Note, Email, Server)
// All item types now use ItemService with flexible items architecture

// UserService defines the business logic for users
type UserService interface {
	GetByID(ctx context.Context, id uint) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	List(ctx context.Context) ([]*domain.User, error)
	Create(ctx context.Context, user *domain.User) error
	CreateByAdmin(ctx context.Context, req *domain.CreateUserByAdminRequest) (*domain.User, error)
	Update(ctx context.Context, id uint, user *domain.User) error
	Delete(ctx context.Context, id uint, schema string) error
	ChangeMasterPassword(ctx context.Context, req *domain.ChangeMasterPasswordRequest) error
}

// VerificationService defines the business logic for email verification
type VerificationService interface {
	GenerateCode(ctx context.Context, email string) (string, error)
	VerifyCode(ctx context.Context, email, code string) error
	ResendCode(ctx context.Context, email string) (string, error)
	CleanupExpiredCodes(ctx context.Context) error
}

// ExcludedDomainService defines the business logic for excluded domains
type ExcludedDomainService interface {
	Create(ctx context.Context, userID uint, req *domain.CreateExcludedDomainRequest) (*domain.ExcludedDomain, error)
	GetByUserID(ctx context.Context, userID uint) ([]*domain.ExcludedDomain, error)
	Delete(ctx context.Context, id uint, userID uint) error
	DeleteByDomain(ctx context.Context, userID uint, domain string) error
	IsExcluded(ctx context.Context, userID uint, domain string) (bool, error)
}

// FolderService defines the business logic for folders
type FolderService interface {
	Create(ctx context.Context, userID uint, req *domain.CreateFolderRequest) (*domain.Folder, error)
	GetByUserID(ctx context.Context, userID uint) ([]*domain.Folder, error)
	Update(ctx context.Context, id uint, userID uint, req *domain.UpdateFolderRequest) (*domain.Folder, error)
	Delete(ctx context.Context, id uint, userID uint) error
}
