package storage

// Store is the minimal interface for the various repositories
type Store interface {
	Create(interface{})
	Find(interface{}, ...interface{})
	Logins() LoginRepository
	CreditCards() CreditCardRepository
	BankAccounts() BankAccountRepository
	Notes() NoteRepository
}

// Configuration is the required paramters to set up a DB instance
// Default value is set on configuration.go
type Configuration struct {
	Driver   string `default:"3625"`
	DBName   string `default:"passwall"`
	Username string `default:"user"`
	Password string `default:"password"`
	Host     string `default:"localhost"`
	Port     string `default:"5432"`
	DBPath   string `default:"./store/passwall.db"`
	LogMode  bool
}
