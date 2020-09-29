package app

import (
	"log"

	"github.com/passwall/passwall-server/internal/storage"
)

// MigrateSystemTables runs auto migration for the system models (Token and User),
// will only add missing fields won't delete/change current data in the store.
func MigrateSystemTables(s storage.Store) {
	if err := s.Tokens().Migrate(); err != nil {
		log.Println(err)
	}
	if err := s.Users().Migrate(); err != nil {
		log.Println(err)
	}
	if err := s.Subscriptions().Migrate(); err != nil {
		log.Println(err)
	}
}

// MigrateUserTables runs auto migration for user models in user schema,
// will only add missing fields won't delete/change current data in the store.
func MigrateUserTables(s storage.Store, schema string) {
	if err := s.Logins().Migrate(schema); err != nil {
		log.Println(err)
	}
	if err := s.CreditCards().Migrate(schema); err != nil {
		log.Println(err)
	}
	if err := s.BankAccounts().Migrate(schema); err != nil {
		log.Println(err)
	}
	if err := s.Notes().Migrate(schema); err != nil {
		log.Println(err)
	}
	if err := s.Emails().Migrate(schema); err != nil {
		log.Println(err)
	}
	if err := s.Servers().Migrate(schema); err != nil {
		log.Println(err)
	}
}
