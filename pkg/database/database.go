package database

import (
	"context"

	"gorm.io/gorm"
)

// Database defines the interface for database operations
// This abstraction allows for easy mocking and testing
type Database interface {
	// DB returns the underlying *gorm.DB instance
	DB() *gorm.DB

	// Close closes the database connection
	Close() error

	// Ping checks if the database is reachable
	Ping(ctx context.Context) error

	// AutoMigrate runs auto migration for given models
	AutoMigrate(dst ...interface{}) error

	// Transaction executes a function within a transaction
	Transaction(ctx context.Context, fn func(*gorm.DB) error) error
}

// Config holds database configuration
type Config struct {
	Host            string
	Port            string
	Username        string
	Password        string
	Database        string
	SSLMode         string
	MaxIdleConns    int
	MaxOpenConns    int
	ConnMaxLifetime int // in seconds
	LogMode         bool
}

// DefaultConfig returns default database configuration
func DefaultConfig() *Config {
	return &Config{
		Host:            "localhost",
		Port:            "5432",
		Username:        "postgres",
		Password:        "password",
		Database:        "passwall",
		SSLMode:         "disable",
		MaxIdleConns:    10,
		MaxOpenConns:    100,
		ConnMaxLifetime: 3600, // 1 hour
		LogMode:         false,
	}
}
