package database

import (
	"context"
	"fmt"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// MockDB is a mock database implementation for testing
// Uses SQLite (Pure Go, CGO-free) for in-memory testing
type MockDB struct {
	db        *gorm.DB
	closeFunc func() error
	pingFunc  func(ctx context.Context) error
}

// NewMock creates a new mock database using SQLite in-memory (CGO-free)
// Uses modernc.org/sqlite driver (Pure Go implementation)
func NewMock() (Database, error) {
	// Use SQLite in-memory database for testing
	// modernc.org/sqlite is automatically used by gorm when available (Pure Go, no CGO)
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create mock database: %w", err)
	}

	return &MockDB{
		db: db,
		closeFunc: func() error {
			sqlDB, err := db.DB()
			if err != nil {
				return err
			}
			return sqlDB.Close()
		},
		pingFunc: func(ctx context.Context) error {
			sqlDB, err := db.DB()
			if err != nil {
				return err
			}
			return sqlDB.PingContext(ctx)
		},
	}, nil
}

// NewMockWithDB creates a mock database with a custom *gorm.DB
func NewMockWithDB(db *gorm.DB) Database {
	return &MockDB{
		db: db,
		closeFunc: func() error {
			return nil
		},
		pingFunc: func(ctx context.Context) error {
			return nil
		},
	}
}

// DB returns the underlying *gorm.DB instance
func (m *MockDB) DB() *gorm.DB {
	return m.db
}

// Close closes the database connection
func (m *MockDB) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

// Ping checks if the database is reachable
func (m *MockDB) Ping(ctx context.Context) error {
	if m.pingFunc != nil {
		return m.pingFunc(ctx)
	}
	return nil
}

// AutoMigrate runs auto migration for given models
func (m *MockDB) AutoMigrate(dst ...interface{}) error {
	return m.db.AutoMigrate(dst...)
}

// Transaction executes a function within a transaction
func (m *MockDB) Transaction(ctx context.Context, fn func(*gorm.DB) error) error {
	return m.db.WithContext(ctx).Transaction(fn)
}

// SetCloseFunc sets a custom close function for testing
func (m *MockDB) SetCloseFunc(fn func() error) {
	m.closeFunc = fn
}

// SetPingFunc sets a custom ping function for testing
func (m *MockDB) SetPingFunc(fn func(ctx context.Context) error) {
	m.pingFunc = fn
}
