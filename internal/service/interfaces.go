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

// Encryptor handles encryption and decryption operations
type Encryptor interface {
	Encrypt(plaintext, passphrase string) (string, error)
	Decrypt(ciphertext, passphrase string) (string, error)
	EncryptModel(model interface{}, passphrase string) error
	DecryptModel(model interface{}, passphrase string) error
}

// AuthService defines the business logic for authentication
type AuthService interface {
	SignUp(ctx context.Context, req *domain.SignUpRequest) (*domain.User, error)
	SignIn(ctx context.Context, creds *domain.Credentials) (*domain.AuthResponse, error)
	RefreshToken(ctx context.Context, refreshToken string) (*domain.TokenDetails, error)
	ValidateToken(ctx context.Context, token string) (*domain.TokenClaims, error)
	SignOut(ctx context.Context, userID int) error
	ValidateSchema(ctx context.Context, schema string) error
}

// LoginService defines the business logic for logins
// Schema is extracted from context automatically
type LoginService interface {
	GetByID(ctx context.Context, id uint) (*domain.Login, error)
	List(ctx context.Context) ([]*domain.Login, error)
	Create(ctx context.Context, login *domain.Login) error
	Update(ctx context.Context, id uint, login *domain.Login) error
	Delete(ctx context.Context, id uint) error
	BulkUpdate(ctx context.Context, logins []*domain.Login) error
}

// UserService defines the business logic for users
type UserService interface {
	GetByID(ctx context.Context, id uint) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	List(ctx context.Context) ([]*domain.User, error)
	Create(ctx context.Context, user *domain.User) error
	Update(ctx context.Context, id uint, user *domain.User) error
	Delete(ctx context.Context, id uint, schema string) error
	ChangeMasterPassword(ctx context.Context, req *domain.ChangeMasterPasswordRequest) error
}

// BankAccountService defines the business logic for bank accounts
// Schema is extracted from context automatically
type BankAccountService interface {
	GetByID(ctx context.Context, id uint) (*domain.BankAccount, error)
	List(ctx context.Context) ([]*domain.BankAccount, error)
	Create(ctx context.Context, account *domain.BankAccount) error
	Update(ctx context.Context, id uint, account *domain.BankAccount) error
	Delete(ctx context.Context, id uint) error
	BulkUpdate(ctx context.Context, accounts []*domain.BankAccount) error
}

// CreditCardService defines the business logic for credit cards
// Schema is extracted from context automatically
type CreditCardService interface {
	GetByID(ctx context.Context, id uint) (*domain.CreditCard, error)
	List(ctx context.Context) ([]*domain.CreditCard, error)
	Create(ctx context.Context, card *domain.CreditCard) error
	Update(ctx context.Context, id uint, card *domain.CreditCard) error
	Delete(ctx context.Context, id uint) error
	BulkUpdate(ctx context.Context, cards []*domain.CreditCard) error
}

// NoteService defines the business logic for notes
// Schema is extracted from context automatically
type NoteService interface {
	GetByID(ctx context.Context, id uint) (*domain.Note, error)
	List(ctx context.Context) ([]*domain.Note, error)
	Create(ctx context.Context, note *domain.Note) error
	Update(ctx context.Context, id uint, note *domain.Note) error
	Delete(ctx context.Context, id uint) error
	BulkUpdate(ctx context.Context, notes []*domain.Note) error
}

// EmailService defines the business logic for emails
// Schema is extracted from context automatically
type EmailService interface {
	GetByID(ctx context.Context, id uint) (*domain.Email, error)
	List(ctx context.Context) ([]*domain.Email, error)
	Create(ctx context.Context, email *domain.Email) error
	Update(ctx context.Context, id uint, email *domain.Email) error
	Delete(ctx context.Context, id uint) error
	BulkUpdate(ctx context.Context, emails []*domain.Email) error
}

// ServerService defines the business logic for servers
// Schema is extracted from context automatically
type ServerService interface {
	GetByID(ctx context.Context, id uint) (*domain.Server, error)
	List(ctx context.Context) ([]*domain.Server, error)
	Create(ctx context.Context, server *domain.Server) error
	Update(ctx context.Context, id uint, server *domain.Server) error
	Delete(ctx context.Context, id uint) error
	BulkUpdate(ctx context.Context, servers []*domain.Server) error
}

// VerificationService defines the business logic for email verification
type VerificationService interface {
	GenerateCode(ctx context.Context, email string) (string, error)
	VerifyCode(ctx context.Context, email, code string) error
	ResendCode(ctx context.Context, email string) (string, error)
	CleanupExpiredCodes(ctx context.Context) error
}
