package database

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, "localhost", cfg.Host)
	assert.Equal(t, "5432", cfg.Port)
	assert.Equal(t, "postgres", cfg.Username)
	assert.Equal(t, "passwall", cfg.Database)
	assert.Equal(t, "disable", cfg.SSLMode)
	assert.Equal(t, 10, cfg.MaxIdleConns)
	assert.Equal(t, 100, cfg.MaxOpenConns)
}

func TestNewMock(t *testing.T) {
	db, err := NewMock()
	require.NoError(t, err)
	require.NotNil(t, db)

	// Test DB() method
	gormDB := db.DB()
	assert.NotNil(t, gormDB)

	// Test Ping
	err = db.Ping(context.Background())
	assert.NoError(t, err)

	// Test AutoMigrate with a simple struct
	type TestModel struct {
		ID   uint   `gorm:"primarykey"`
		Name string `gorm:"size:100"`
	}

	err = db.AutoMigrate(&TestModel{})
	assert.NoError(t, err)

	// Test Transaction
	err = db.Transaction(context.Background(), func(tx *gorm.DB) error {
		return tx.Create(&TestModel{Name: "test"}).Error
	})
	assert.NoError(t, err)

	// Test Close
	err = db.Close()
	assert.NoError(t, err)
}

func TestMockDB_CustomFunctions(t *testing.T) {
	db, err := NewMock()
	require.NoError(t, err)

	mockDB := db.(*MockDB)

	// Test custom close function
	closeCalled := false
	mockDB.SetCloseFunc(func() error {
		closeCalled = true
		return nil
	})

	err = db.Close()
	assert.NoError(t, err)
	assert.True(t, closeCalled)

	// Test custom ping function
	pingCalled := false
	mockDB.SetPingFunc(func(ctx context.Context) error {
		pingCalled = true
		return nil
	})

	err = db.Ping(context.Background())
	assert.NoError(t, err)
	assert.True(t, pingCalled)
}
