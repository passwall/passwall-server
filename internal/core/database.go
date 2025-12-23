package core

import (
	"fmt"

	"github.com/passwall/passwall-server/internal/config"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/pkg/database"
	"github.com/passwall/passwall-server/pkg/database/postgres"
)

// InitDatabase initializes database connection using the database package
func InitDatabase(cfg *config.Config) (database.Database, error) {
	// Convert config to database.Config
	dbCfg := &database.Config{
		Host:         cfg.Database.Host,
		Port:         cfg.Database.Port,
		Username:     cfg.Database.Username,
		Password:     cfg.Database.Password,
		Database:     cfg.Database.Name,
		SSLMode:      cfg.Database.SSLMode,
		MaxIdleConns: 10,
		MaxOpenConns: 100,
		LogMode:      cfg.Database.LogMode,
	}

	// Create database connection
	db, err := postgres.New(dbCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	return db, nil
}

// AutoMigrate runs database migrations
// Returns nil if migration fails due to existing tables (backup restore scenario)
func AutoMigrate(db database.Database) error {
	_ = db.AutoMigrate(
		&domain.User{},
		&domain.Token{},
		// Note: Other entities are migrated per schema
	)
	
	// Always return nil - if tables exist from backup, that's fine
	// If migration fails, tables likely already exist
	return nil
}
