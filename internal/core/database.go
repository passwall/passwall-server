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
		&domain.Preference{},
		&domain.Invitation{},
		&domain.CompatTelemetryEvent{},
		&domain.TelemetryAIVerdict{},
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

	// Organization policies
	if err := db.AutoMigrate(
		&domain.OrganizationPolicy{},
	); err != nil {
		return fmt.Errorf("failed to migrate organization policy tables: %w", err)
	}

	// Emergency Access & Sends
	if err := db.AutoMigrate(
		&domain.EmergencyAccess{},
		&domain.Send{},
	); err != nil {
		return fmt.Errorf("failed to migrate emergency access / send tables: %w", err)
	}

	// Breach Monitoring tables
	if err := db.AutoMigrate(
		&domain.MonitoredEmail{},
		&domain.BreachRecord{},
	); err != nil {
		return fmt.Errorf("failed to migrate breach monitoring tables: %w", err)
	}

	// SSO & SCIM tables (Enterprise features)
	if err := db.AutoMigrate(
		&domain.SSOConnection{},
		&domain.SSOState{},
		&domain.SCIMToken{},
		&domain.OrgEscrowKey{},
		&domain.KeyEscrow{},
	); err != nil {
		return fmt.Errorf("failed to migrate SSO/SCIM tables: %w", err)
	}

	// Backfill public_id for any organizations that don't have one yet,
	// then create a unique index. This is idempotent.
	if err := backfillOrgPublicIDs(db); err != nil {
		return fmt.Errorf("failed to backfill organization public_ids: %w", err)
	}

	logger.Infof("✓ Database schema migrated successfully")
	return nil
}

func backfillOrgPublicIDs(db database.Database) error {
	gormDB := db.DB()

	var orgs []domain.Organization
	if err := gormDB.Where("public_id IS NULL OR public_id = ''").Find(&orgs).Error; err != nil {
		return err
	}

	if len(orgs) > 0 {
		logger.Infof("Backfilling public_id for %d organization(s)...", len(orgs))
	}

	for i := range orgs {
		pid, err := domain.GeneratePublicID()
		if err != nil {
			return fmt.Errorf("generate public_id for org %d: %w", orgs[i].ID, err)
		}
		if err := gormDB.Model(&orgs[i]).Update("public_id", pid).Error; err != nil {
			return fmt.Errorf("update public_id for org %d: %w", orgs[i].ID, err)
		}
	}

	if len(orgs) > 0 {
		logger.Infof("✓ Backfilled public_id for %d organization(s)", len(orgs))
	}

	if err := gormDB.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_organizations_public_id
		ON organizations (public_id)
		WHERE public_id IS NOT NULL AND public_id != ''
	`).Error; err != nil {
		return fmt.Errorf("create unique index on public_id: %w", err)
	}

	return nil
}
