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
	// FindAll returns the entities matching the arguments.
	FindAll(argsStr map[string]string, argsInt map[string]int, schema string) ([]model.Login, error)
	// FindByID finds the entity regarding to its ID.
	FindByID(id uint, schema string) (*model.Login, error)
	// Save stores the entity to the repository
	Save(login *model.Login, schema string) (*model.Login, error)
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
	// FindAll returns the entities matching the arguments.
	FindAll(argsStr map[string]string, argsInt map[string]int, schema string) ([]model.CreditCard, error)
	// FindByID finds the entity regarding to its ID.
	FindByID(id uint, schema string) (*model.CreditCard, error)
	// Save stores the entity to the repository
	Save(card *model.CreditCard, schema string) (*model.CreditCard, error)
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
	// FindAll returns the entities matching the arguments.
	FindAll(argsStr map[string]string, argsInt map[string]int, schema string) ([]model.BankAccount, error)
	// FindByID finds the entity regarding to its ID.
	FindByID(id uint, schema string) (*model.BankAccount, error)
	// Save stores the entity to the repository
	Save(account *model.BankAccount, schema string) (*model.BankAccount, error)
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
	// FindAll returns the entities matching the arguments.
	FindAll(argsStr map[string]string, argsInt map[string]int, schema string) ([]model.Note, error)
	// FindByID finds the entity regarding to its ID.
	FindByID(id uint, schema string) (*model.Note, error)
	// Save stores the entity to the repository
	Save(account *model.Note, schema string) (*model.Note, error)
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
	// FindAll returns the entities matching the arguments.
	FindAll(argsStr map[string]string, argsInt map[string]int, schema string) ([]model.Email, error)
	// FindByID finds the entity regarding to its ID.
	FindByID(id uint, schema string) (*model.Email, error)
	// Save stores the entity to the repository
	Save(account *model.Email, schema string) (*model.Email, error)
	// Delete removes the entity from the store
	Delete(id uint, schema string) error
	// Migrate migrates the repository
	Migrate(schema string) error
}

// TokenRepository ...
// TODO: Add explanation to functions in TokenRepository
type TokenRepository interface {
	Any(uuid string) (model.Token, bool)
	Save(userid int, uuid uuid.UUID, tkn string, expriydate time.Time, transmissionKey string)
	Delete(userid int)
	DeleteByUUID(uuid string)
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
	// FindByEmail finds the entity regarding to its Email.
	FindByEmail(email string) (*model.User, error)
	// FindByCredentials finds the entity regarding to its Email and Master Password.
	FindByCredentials(email, masterPassword string) (*model.User, error)
	// Save stores the entity to the repository
	Save(login *model.User) (*model.User, error)
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
	// FindAll returns the entities matching the arguments.
	FindAll(argsStr map[string]string, argsInt map[string]int, schema string) ([]model.Server, error)
	// FindByID finds the entity regarding to its ID.
	FindByID(id uint, schema string) (*model.Server, error)
	// Save stores the entity to the repository
	Save(server *model.Server, schema string) (*model.Server, error)
	// Delete removes the entity from the store
	Delete(id uint, schema string) error
	// Migrate migrates the repository
	Migrate(schema string) error
}

// SubscriptionRepository interface is the common interface for a repository
// Each method checks the entity type.
type SubscriptionRepository interface {
	// All returns all the data in the repository.
	All() ([]model.Subscription, error)
	// FindAll returns the entities matching the arguments.
	FindAll(argsStr map[string]string, argsInt map[string]int) ([]model.Subscription, error)
	// FindByID finds the entity regarding to its ID.
	FindByID(id uint) (*model.Subscription, error)
	// FindByEmail finds the entity regarding to its email.
	FindByEmail(email string) (*model.Subscription, error)
	// FindBySubscriptionID finds the entity regarding to its Subscription ID.
	FindBySubscriptionID(id uint) (*model.Subscription, error)
	// Save stores the entity to the repository
	Save(subscription *model.Subscription) (*model.Subscription, error)
	// Delete removes the entity from the store
	Delete(id uint) error
	// Migrate migrates the repository
	Migrate() error
}
