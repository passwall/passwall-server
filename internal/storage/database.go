package storage

import (
	"errors"
	"fmt"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/pass-wall/passwall-server/internal/storage/bankaccount"
	"github.com/pass-wall/passwall-server/internal/storage/creditcard"
	"github.com/pass-wall/passwall-server/internal/storage/login"
)

// Database is the concrete store provider.
type Database struct {
	db       *gorm.DB
	logins   LoginRepository
	cards    CreditCardRepository
	accounts BankAccountRepository
}

// New opens a database according to configuration.
func New(cfg *Configuration) (*Database, error) {
	var db *gorm.DB
	var err error

	switch cfg.Driver {
	case "sqlite":
		path := cfg.DBPath
		if cfg.DBPath == "" {
			return nil, errors.New("sqlite db path should not be empty")
		}
		db, err = gorm.Open("sqlite3", path)
		if err != nil {
			return nil, fmt.Errorf("could not open sqlite database: %w", err)
		}
	case "postgres":
		db, err = gorm.Open("postgres", "host="+cfg.Host+" port="+cfg.Port+" user="+cfg.Username+" dbname="+cfg.DBName+"  sslmode=disable password="+cfg.Password)
		if err != nil {
			return nil, fmt.Errorf("could not open postgresql connection: %w", err)
		}
	case "mysql":
		db, err = gorm.Open("mysql", cfg.Username+":"+cfg.Password+"@tcp("+cfg.Host+":"+cfg.Port+")/"+cfg.DBName+"?charset=utf8&parseTime=True&loc=Local")
		if err != nil {
			return nil, fmt.Errorf("could not open mysql connection: %w", err)
		}
	default:
		return nil, fmt.Errorf("could not recognize database type %q", cfg.Driver)
	}

	db.LogMode(cfg.LogMode)

	return &Database{
		db:       db,
		logins:   login.NewRepository(db),
		cards:    creditcard.NewRepository(db),
		accounts: bankaccount.NewRepository(db),
	}, nil
}

// Create inserts the value into database.
func (db *Database) Create(value interface{}) {
	db.db.Create(value)
}

// Find finds the records that match given conditions.
func (db *Database) Find(value interface{}, where ...interface{}) {
	db.db.Find(value, where)
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
