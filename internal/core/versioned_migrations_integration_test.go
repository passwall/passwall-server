package core

import (
	"context"
	"testing"
	"time"

	"github.com/passwall/passwall-server/pkg/database"
	databasePostgres "github.com/passwall/passwall-server/pkg/database/postgres"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	containerPostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestMigrateDatabaseBootstrapsAndIsIdempotent(t *testing.T) {
	testcontainers.SkipIfProviderIsNotHealthy(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	const (
		databaseName = "passwall_migrations"
		databaseUser = "passwall"
		password     = "migration-test-password"
	)

	container, err := containerPostgres.Run(
		ctx,
		"postgres:18-alpine",
		containerPostgres.WithDatabase(databaseName),
		containerPostgres.WithUsername(databaseUser),
		containerPostgres.WithPassword(password),
		containerPostgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	testcontainers.CleanupContainer(t, container)

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "5432/tcp")
	require.NoError(t, err)

	db, err := databasePostgres.New(&database.Config{
		Host:            host,
		Port:            port.Port(),
		Username:        databaseUser,
		Password:        password,
		Database:        databaseName,
		SSLMode:         "disable",
		MaxIdleConns:    2,
		MaxOpenConns:    4,
		ConnMaxLifetime: 60,
		ConnMaxIdleTime: 30,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	require.NoError(t, MigrateDatabase(ctx, db))
	require.True(t, db.DB().Migrator().HasTable("users"))

	sqlDB, err := db.DB().DB()
	require.NoError(t, err)
	version, err := goose.GetDBVersionContext(ctx, sqlDB)
	require.NoError(t, err)
	require.EqualValues(t, 6, version)

	require.NoError(t, MigrateDatabase(ctx, db))
	version, err = goose.GetDBVersionContext(ctx, sqlDB)
	require.NoError(t, err)
	require.EqualValues(t, 6, version)
}
