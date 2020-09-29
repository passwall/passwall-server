package storage

import (
	"fmt"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/passwall/passwall-server/internal/config"
	"github.com/passwall/passwall-server/internal/storage/bankaccount"
	"github.com/passwall/passwall-server/internal/storage/creditcard"
	"github.com/passwall/passwall-server/internal/storage/email"
	"github.com/passwall/passwall-server/internal/storage/login"
	"github.com/passwall/passwall-server/internal/storage/note"
	"github.com/passwall/passwall-server/internal/storage/server"
	"github.com/passwall/passwall-server/internal/storage/subscription"
	"github.com/passwall/passwall-server/internal/storage/token"
	"github.com/passwall/passwall-server/internal/storage/user"
)

// Database is the concrete store provider.
type Database struct {
	db            *gorm.DB
	logins        LoginRepository
	cards         CreditCardRepository
	accounts      BankAccountRepository
	notes         NoteRepository
	emails        EmailRepository
	tokens        TokenRepository
	users         UserRepository
	servers       ServerRepository
	subscriptions SubscriptionRepository
}

func DBConn(cfg *config.DatabaseConfiguration) (*gorm.DB, error) {
	var db *gorm.DB
	var err error

	db, err = gorm.Open("postgres", "host="+cfg.Host+" port="+cfg.Port+" user="+cfg.Username+" dbname="+cfg.Name+"  sslmode=disable password="+cfg.Password)
	if err != nil {
		return nil, fmt.Errorf("could not open postgresql connection: %w", err)
	}

	db.LogMode(cfg.LogMode)

	return db, err
}

// New opens a database according to configuration.
func New(db *gorm.DB) *Database {
	return &Database{
		db:            db,
		logins:        login.NewRepository(db),
		cards:         creditcard.NewRepository(db),
		accounts:      bankaccount.NewRepository(db),
		notes:         note.NewRepository(db),
		emails:        email.NewRepository(db),
		tokens:        token.NewRepository(db),
		users:         user.NewRepository(db),
		servers:       server.NewRepository(db),
		subscriptions: subscription.NewRepository(db),
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

// Users returns the UserRepository.
func (db *Database) Users() UserRepository {
	return db.users
}

// Servers returns the UserRepository.
func (db *Database) Servers() ServerRepository {
	return db.servers
}

// Subscriptions returns the UserRepository.
func (db *Database) Subscriptions() SubscriptionRepository {
	return db.subscriptions
}

// Ping checks if database is up
func (db *Database) Ping() error {
	return db.db.DB().Ping()
}
