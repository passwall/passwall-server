package core

import (
	"fmt"

	"github.com/passwall/passwall-server/internal/config"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/pkg/database"
	"github.com/passwall/passwall-server/pkg/database/postgres"
	"github.com/passwall/passwall-server/pkg/logger"
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
// This creates all tables from scratch with their FINAL structure
// For production updates of existing databases, use SQL migration files in /migrations/
func AutoMigrate(db database.Database) error {
	// Create Item table first (used by personal vault)
	if err := db.AutoMigrate(&domain.Item{}); err != nil {
		return fmt.Errorf("failed to migrate Item: %w", err)
	}

	// Core auth & user tables
	if err := db.AutoMigrate(
		&domain.Role{},
		&domain.Permission{},
		&domain.User{},
		&domain.Token{},
		&domain.VerificationCode{},
		&domain.UserActivity{},
	); err != nil {
		return fmt.Errorf("failed to migrate core auth tables: %w", err)
	}

	// User-related tables
	if err := db.AutoMigrate(
		&domain.ExcludedDomain{},
		&domain.Folder{},
		&domain.Preference{},
		&domain.Invitation{},
	); err != nil {
		return fmt.Errorf("failed to migrate user-related tables: %w", err)
	}

	// SaaS subscription tables (NEW - Phase 1)
	if err := db.AutoMigrate(
		&domain.Plan{},
		&domain.Subscription{},
		&domain.WebhookEvent{},
		// Note: Invoices are fetched directly from Stripe (no DB table needed)
	); err != nil {
		return fmt.Errorf("failed to migrate subscription tables: %w", err)
	}

	// Organization tables
	if err := db.AutoMigrate(
		&domain.Organization{},
		&domain.OrganizationUser{},
		&domain.Team{},
		&domain.TeamUser{},
		&domain.Collection{},
		&domain.CollectionUser{},
		&domain.CollectionTeam{},
		&domain.OrganizationFolder{},
		&domain.OrganizationItem{},
		&domain.ItemShare{},
	); err != nil {
		return fmt.Errorf("failed to migrate organization tables: %w", err)
	}

	logger.Infof("✓ Database schema migrated successfully")
	return nil
}
