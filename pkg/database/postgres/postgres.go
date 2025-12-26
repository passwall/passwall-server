package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/passwall/passwall-server/pkg/database"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// postgresDB implements the Database interface for PostgreSQL
type postgresDB struct {
	db *gorm.DB
}

// New creates a new PostgreSQL database connection
func New(cfg *database.Config) (database.Database, error) {
	if cfg == nil {
		cfg = database.DefaultConfig()
	}

	// Build DSN (Data Source Name)
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host,
		cfg.Port,
		cfg.Username,
		cfg.Password,
		cfg.Database,
		cfg.SSLMode,
	)

	// Configure GORM logger
	logLevel := logger.Silent
	if cfg.LogMode {
		logLevel = logger.Info
	}

	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
		PrepareStmt: true, // Prepare statement for better performance
		// Disable automatic association saving to prevent issues when entities
		// are loaded with Preload(). This prevents GORM from trying to save
		// associations during Update operations, which can cause foreign key
		// values to revert to preloaded values.
		// If you need to save associations explicitly, use Association() API.
		FullSaveAssociations: false,
	}

	// Open database connection
	db, err := gorm.Open(postgres.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Get underlying SQL database
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	// Set connection pool settings
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &postgresDB{db: db}, nil
}

// DB returns the underlying *gorm.DB instance
func (p *postgresDB) DB() *gorm.DB {
	return p.db
}

// Close closes the database connection
func (p *postgresDB) Close() error {
	sqlDB, err := p.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql.DB: %w", err)
	}
	return sqlDB.Close()
}

// Ping checks if the database is reachable
func (p *postgresDB) Ping(ctx context.Context) error {
	sqlDB, err := p.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql.DB: %w", err)
	}
	return sqlDB.PingContext(ctx)
}

// AutoMigrate runs auto migration for given models
func (p *postgresDB) AutoMigrate(dst ...interface{}) error {
	return p.db.AutoMigrate(dst...)
}

// Transaction executes a function within a transaction
func (p *postgresDB) Transaction(ctx context.Context, fn func(*gorm.DB) error) error {
	return p.db.WithContext(ctx).Transaction(fn)
}
