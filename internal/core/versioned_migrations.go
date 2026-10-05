package core

import (
	"context"
	"embed"
	"fmt"
	"time"

	"github.com/passwall/passwall-server/pkg/database"
	"github.com/passwall/passwall-server/pkg/logger"
	"github.com/pressly/goose/v3"
	"gorm.io/gorm"
)

const migrationAdvisoryLock = "passwall_schema_migration"

//go:embed migrations/*.sql
var migrationFiles embed.FS

// MigrateDatabase applies embedded, versioned SQL migrations. AutoMigrate is
// used only to bootstrap an empty database to the version-1 baseline.
func MigrateDatabase(ctx context.Context, db database.Database) error {
	gormDB := db.DB().WithContext(ctx)
	sqlDB, err := gormDB.DB()
	if err != nil {
		return fmt.Errorf("get migration sql database: %w", err)
	}

	lockConn, err := sqlDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("open migration lock connection: %w", err)
	}
	defer func() { _ = lockConn.Close() }()

	if _, err := lockConn.ExecContext(
		ctx,
		"SELECT pg_advisory_lock(hashtext($1))",
		migrationAdvisoryLock,
	); err != nil {
		return fmt.Errorf("acquire migration advisory lock: %w", err)
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = lockConn.ExecContext(
			unlockCtx,
			"SELECT pg_advisory_unlock(hashtext($1))",
			migrationAdvisoryLock,
		)
	}()

	if err := bootstrapMigrationBaseline(gormDB); err != nil {
		return err
	}

	goose.SetBaseFS(migrationFiles)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("configure migration dialect: %w", err)
	}
	if err := goose.UpContext(ctx, sqlDB, "migrations"); err != nil {
		return fmt.Errorf("apply versioned migrations: %w", err)
	}

	logger.Infof("✓ Database schema migrated successfully")
	return nil
}

func bootstrapMigrationBaseline(db *gorm.DB) error {
	var schemaExists bool
	if err := db.Raw(
		"SELECT to_regclass('public.users') IS NOT NULL",
	).Scan(&schemaExists).Error; err != nil {
		return fmt.Errorf("detect migration baseline: %w", err)
	}
	if schemaExists {
		return nil
	}

	if err := autoMigrateSchema(db); err != nil {
		return fmt.Errorf("bootstrap migration baseline: %w", err)
	}
	return nil
}
