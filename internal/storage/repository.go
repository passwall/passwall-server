package storage

import (
	"time"

	"github.com/pass-wall/passwall-server/model"
	uuid "github.com/satori/go.uuid"
)

// LoginRepository interface is the common interface for a repository
// Each method checks the entity type.
type LoginRepository interface {
	// All returns all the data in the repository.
	All() ([]model.Login, error)
	// FindAll returns the entities matching the arguments.
	FindAll(argsStr map[string]string, argsInt map[string]int) ([]model.Login, error)
	// FindByID finds the entity regarding to its ID.
	FindByID(id uint) (model.Login, error)
	// Save stores the entity to the repository
	Save(login model.Login) (model.Login, error)
	// Delete removes the entity from the store
	Delete(id uint) error
	// Migrate migrates the repository
	Migrate() error
}

// CreditCardRepository interface is the common interface for a repository
// Each method checks the entity type.
type CreditCardRepository interface {
	// All returns all the data in the repository.
	All() ([]model.CreditCard, error)
	// FindAll returns the entities matching the arguments.
	FindAll(argsStr map[string]string, argsInt map[string]int) ([]model.CreditCard, error)
	// FindByID finds the entity regarding to its ID.
	FindByID(id uint) (model.CreditCard, error)
	// Save stores the entity to the repository
	Save(card model.CreditCard) (model.CreditCard, error)
	// Delete removes the entity from the store
	Delete(id uint) error
	// Migrate migrates the repository
	Migrate() error
}

// BankAccountRepository interface is the common interface for a repository
// Each method checks the entity type.
type BankAccountRepository interface {
	// All returns all the data in the repository.
	All() ([]model.BankAccount, error)
	// FindAll returns the entities matching the arguments.
	FindAll(argsStr map[string]string, argsInt map[string]int) ([]model.BankAccount, error)
	// FindByID finds the entity regarding to its ID.
	FindByID(id uint) (model.BankAccount, error)
	// Save stores the entity to the repository
	Save(account model.BankAccount) (model.BankAccount, error)
	// Delete removes the entity from the store
	Delete(id uint) error
	// Migrate migrates the repository
	Migrate() error
}

// BankAccountRepository interface is the common interface for a repository
// Each method checks the entity type.
type NoteRepository interface {
	// All returns all the data in the repository.
	All() ([]model.Note, error)
	// FindAll returns the entities matching the arguments.
	FindAll(argsStr map[string]string, argsInt map[string]int) ([]model.Note, error)
	// FindByID finds the entity regarding to its ID.
	FindByID(id uint) (model.Note, error)
	// Save stores the entity to the repository
	Save(account model.Note) (model.Note, error)
	// Delete removes the entity from the store
	Delete(id uint) error
	// Migrate migrates the repository
	Migrate() error
}

type TokenRepository interface {
	Any(uuid string) bool
	Save(userid int, uuid uuid.UUID, tkn string, expriydate time.Time)
	Delete(userid int)
	DeleteByUUID(uuid string)
	Migrate() error
}
