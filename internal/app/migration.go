package app

import (
	"fmt"

	"github.com/passwall/passwall-server/internal/storage"
	"github.com/passwall/passwall-server/pkg/logger"
)

// MigrateSystemTables runs auto migration for the system models (Token and User),
// will only add missing fields won't delete/change current data in the store.
func MigrateSystemTables(s storage.Store) {
	if err := s.Tokens().Migrate(); err != nil {
		logger.Errorf("failed to migrate tokens: %v", err)
	}
	if err := s.Users().Migrate(); err != nil {
		logger.Errorf("failed to migrate users: %v", err)
	}
}

// MigrateUserTables runs auto migration for user models in user schema,
// will only add missing fields won't delete/change current data in the store.
func MigrateUserTables(s storage.Store, schema string) error {
	if schema == "" {
		return fmt.Errorf("schema is empty")
	}

	if err := s.Logins().Migrate(schema); err != nil {
		logger.Errorf("failed to migrate logins: %v", err)
		return err
	}
	if err := s.CreditCards().Migrate(schema); err != nil {
		logger.Errorf("failed to migrate credit cards: %v", err)
		return err
	}
	if err := s.BankAccounts().Migrate(schema); err != nil {
		logger.Errorf("failed to migrate bank accounts: %v", err)
		return err
	}
	if err := s.Notes().Migrate(schema); err != nil {
		logger.Errorf("failed to migrate notes: %v", err)
		return err
	}
	if err := s.Emails().Migrate(schema); err != nil {
		logger.Errorf("failed to migrate emails: %v", err)
		return err
	}
	if err := s.Servers().Migrate(schema); err != nil {
		logger.Errorf("failed to migrate servers: %v", err)
		return err
	}
	return nil
}
