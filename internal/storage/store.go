package storage

// Store is the minimal interface for the various repositories
type Store interface {
	Create(interface{})
	Find(interface{}, ...interface{})
	Logins() LoginRepository
	CreditCards() CreditCardRepository
	BankAccounts() BankAccountRepository
}

// Configuration is the required paramters to set up a DB instance
// Default value is set on configuration.go
type Configuration struct {
	Driver   string
	DBName   string
	DBPath   string
	Username string
	Password string
	Host     string
	Port     string
	LogMode  bool
}
