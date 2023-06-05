package storage

import (
	"time"

	"github.com/passwall/passwall-server/model"
	uuid "github.com/satori/go.uuid"
)

// LoginRepository interface is the common interface for a repository
// Each method checks the entity type.
type LoginRepository interface {
	// All returns all the data in the repository.
	All(schema string) ([]model.Login, error)
	// FindByID finds the entity regarding to its ID.
	FindByID(id uint, schema string) (*model.Login, error)
	// Update stores the entity to the repository
	Update(login *model.Login, schema string) (*model.Login, error)
	// Create stores the entity to the repository
	Create(login *model.Login, schema string) (*model.Login, error)
	// Delete removes the entity from the store
	Delete(id uint, schema string) error
	// Migrate migrates the repository
	Migrate(schema string) error
}

// CreditCardRepository interface is the common interface for a repository
// Each method checks the entity type.
type CreditCardRepository interface {
	// All returns all the data in the repository.
	All(schema string) ([]model.CreditCard, error)
	// FindByID finds the entity regarding to its ID.
	FindByID(id uint, schema string) (*model.CreditCard, error)
	// Update stores the entity to the repository
	Update(card *model.CreditCard, schema string) (*model.CreditCard, error)
	// Create stores the entity to the repository
	Create(card *model.CreditCard, schema string) (*model.CreditCard, error)
	// Delete removes the entity from the store
	Delete(id uint, schema string) error
	// Migrate migrates the repository
	Migrate(schema string) error
}

// BankAccountRepository interface is the common interface for a repository
// Each method checks the entity type.
type BankAccountRepository interface {
	// All returns all the data in the repository.
	All(schema string) ([]model.BankAccount, error)
	// FindByID finds the entity regarding to its ID.
	FindByID(id uint, schema string) (*model.BankAccount, error)
	// Update stores the entity to the repository
	Update(account *model.BankAccount, schema string) (*model.BankAccount, error)
	// Create stores the entity to the repository
	Create(account *model.BankAccount, schema string) (*model.BankAccount, error)
	// Delete removes the entity from the store
	Delete(id uint, schema string) error
	// Migrate migrates the repository
	Migrate(schema string) error
}

// NoteRepository interface is the common interface for a repository
// Each method checks the entity type.
type NoteRepository interface {
	// All returns all the data in the repository.
	All(schema string) ([]model.Note, error)
	// FindByID finds the entity regarding to its ID.
	FindByID(id uint, schema string) (*model.Note, error)
	// Update stores the entity to the repository
	Update(account *model.Note, schema string) (*model.Note, error)
	// Create stores the entity to the repository
	Create(account *model.Note, schema string) (*model.Note, error)
	// Delete removes the entity from the store
	Delete(id uint, schema string) error
	// Migrate migrates the repository
	Migrate(schema string) error
}

// EmailRepository interface is the common interface for a repository
// Each method checks the entity type.
type EmailRepository interface {
	// All returns all the data in the repository.
	All(schema string) ([]model.Email, error)
	// FindByID finds the entity regarding to its ID.
	FindByID(id uint, schema string) (*model.Email, error)
	// Update stores the entity to the repository
	Update(account *model.Email, schema string) (*model.Email, error)
	// Create stores the entity to the repository
	Create(account *model.Email, schema string) (*model.Email, error)
	// Delete removes the entity from the store
	Delete(id uint, schema string) error
	// Migrate migrates the repository
	Migrate(schema string) error
}

// TokenRepository ...
type TokenRepository interface {
	// FindByUUID finds the entity regarding to its UUID.
	FindByUUID(uuid string) (model.Token, error)
	// Create stores the entity to the repository
	Create(userid int, uuid uuid.UUID, tkn string, expriydate time.Time)
	// Delete removes the entity regarding to its User ID
	Delete(userid int)
	// DeleteByUUID removes the entity regarding to its UUID
	DeleteByUUID(uuid string)
	// Migrate migrates the repository
	Migrate() error
}

// UserRepository interface is the common interface for a repository
// Each method checks the entity type.
type UserRepository interface {
	// All returns all the data in the repository.
	All() ([]model.User, error)
	// FindAll returns the entities matching the arguments.
	FindAll(argsStr map[string]string, argsInt map[string]int) ([]model.User, error)
	// FindByID finds the entity regarding to its ID.
	FindByID(id uint) (*model.User, error)
	// FindByUUID finds the entity regarding to its UUID.
	FindByUUID(uuid string) (*model.User, error)
	// FindByEmail finds the entity regarding to its Email.
	FindByEmail(email string) (*model.User, error)
	// FindByCredentials finds the entity regarding to its Email and Master Password.
	FindByCredentials(email, masterPassword string) (*model.User, error)
	// Update stores the entity to the repository
	Update(login *model.User) (*model.User, error)
	// Create stores the entity to the repository
	Create(login *model.User) (*model.User, error)
	// Delete removes the entity from the store
	Delete(id uint, schema string) error
	// Migrate migrates the repository
	Migrate() error
	// CreateSchema creates schema for user
	CreateSchema(schema string) error
}

// ServerRepository interface is the common interface for a repository
// Each method checks the entity type.
type ServerRepository interface {
	// All returns all the data in the repository.
	All(schema string) ([]model.Server, error)
	// FindByID finds the entity regarding to its ID.
	FindByID(id uint, schema string) (*model.Server, error)
	// Update stores the entity to the repository
	Update(server *model.Server, schema string) (*model.Server, error)
	// Create stores the entity to the repository
	Create(server *model.Server, schema string) (*model.Server, error)
	// Delete removes the entity from the store
	Delete(id uint, schema string) error
	// Migrate migrates the repository
	Migrate(schema string) error
}
