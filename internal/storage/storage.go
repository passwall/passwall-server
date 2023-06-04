package storage

// Store is the minimal interface for the various repositories
type Store interface {
	Logins() LoginRepository
	CreditCards() CreditCardRepository
	BankAccounts() BankAccountRepository
	Notes() NoteRepository
	Emails() EmailRepository
	Tokens() TokenRepository
	Users() UserRepository
	Servers() ServerRepository
	Ping() error
}
