package storage

import (
	"fmt"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/pass-wall/passwall-server/internal/config"
	"github.com/pass-wall/passwall-server/internal/storage/bankaccount"
	"github.com/pass-wall/passwall-server/internal/storage/creditcard"
	"github.com/pass-wall/passwall-server/internal/storage/email"
	"github.com/pass-wall/passwall-server/internal/storage/login"
	"github.com/pass-wall/passwall-server/internal/storage/note"
	"github.com/pass-wall/passwall-server/internal/storage/token"
)

// Database is the concrete store provider.
type Database struct {
	db       *gorm.DB
	logins   LoginRepository
	cards    CreditCardRepository
	accounts BankAccountRepository
	notes    NoteRepository
	emails   EmailRepository
	tokens   TokenRepository
}

// New opens a database according to configuration.
func New(cfg *config.DatabaseConfiguration) (*Database, error) {
	var db *gorm.DB
	var err error

	db, err = gorm.Open("postgres", "host="+cfg.Host+" port="+cfg.Port+" user="+cfg.Username+" dbname="+cfg.Name+"  sslmode=disable password="+cfg.Password)
	if err != nil {
		return nil, fmt.Errorf("could not open postgresql connection: %w", err)
	}

	db.LogMode(cfg.LogMode)

	return &Database{
		db:       db,
		logins:   login.NewRepository(db),
		cards:    creditcard.NewRepository(db),
		accounts: bankaccount.NewRepository(db),
		notes:    note.NewRepository(db),
		emails:   email.NewRepository(db),
		tokens:   token.NewRepository(db),
	}, nil
}

// Create inserts the value into database.
func (db *Database) Create(value interface{}) {
	db.db.Create(value)
}

// Find finds the records that match given conditions.
func (db *Database) Find(value interface{}, where ...interface{}) {
	if len(where) > 0 {
		db.db.Find(value, where)
	} else {
		db.db.Find(value)
	}

}

// Logins returns the LoginRepository.
func (db *Database) Logins() LoginRepository {
	return db.logins
}

// CreditCards returns the CreditCardRepository.
func (db *Database) CreditCards() CreditCardRepository {
	return db.cards
}

// BankAccounts returns the BankAccountRepository.
func (db *Database) BankAccounts() BankAccountRepository {
	return db.accounts
}

// Notes returns the BankAccountRepository.
func (db *Database) Notes() NoteRepository {
	return db.notes
}

// Emails returns the BankAccountRepository.
func (db *Database) Emails() EmailRepository {
	return db.emails
}

// Tokens returns the TokenRepository.
func (db *Database) Tokens() TokenRepository {
	return db.tokens
}

func (db *Database) Ping() error {
	return db.db.DB().Ping()
}
