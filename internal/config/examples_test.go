package config_test

import (
	"testing"

	"github.com/passwall/passwall-server/internal/config"
	"github.com/stretchr/testify/assert"
)

// Example 1: Using mock config in a simple test
func TestExample_BasicMockUsage(t *testing.T) {
	// Create a mock config
	cfg := config.NewMockConfig()

	// Use it in your test
	assert.Equal(t, "localhost", cfg.Server.Host)
	assert.Equal(t, "3625", cfg.Server.Port)

	// Mock config is pre-validated and ready to use
	err := cfg.Validate()
	assert.NoError(t, err)
}

// Example 2: Using builder for custom scenarios
func TestExample_BuilderPattern(t *testing.T) {
	// Build a custom config for specific test needs
	cfg := config.NewMockBuilder().
		WithServerPort("8080").
		WithDatabaseName("my_test_db").
		WithPassphrase("my-custom-passphrase").
		Build()

	assert.Equal(t, "8080", cfg.Server.Port)
	assert.Equal(t, "my_test_db", cfg.Database.Name)
	assert.Equal(t, "my-custom-passphrase", cfg.Server.Passphrase)
}

// Example 3: Testing database connection scenarios
func TestExample_DatabaseScenarios(t *testing.T) {
	tests := []struct {
		name   string
		config *config.Config
	}{
		{
			name: "local database",
			config: config.NewMockBuilder().
				WithDatabaseHost("localhost").
				WithDatabasePort("5432").
				Build(),
		},
		{
			name: "remote database with SSL",
			config: config.NewMockBuilder().
				WithDatabaseHost("db.production.com").
				WithDatabasePort("5432").
				WithDatabaseSSLMode("require").
				Build(),
		},
		{
			name: "test database",
			config: config.NewMockBuilder().
				WithDatabaseName("test_db").
				WithDatabaseUser("test_user").
				Build(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use the config in your database connection tests
			assert.NotEmpty(t, tt.config.Database.Host)
			assert.NotEmpty(t, tt.config.Database.Name)
		})
	}
}

// Example 4: Testing authentication with different token durations
func TestExample_AuthTokenScenarios(t *testing.T) {
	// Short-lived tokens for testing
	shortTokenCfg := config.NewMockBuilder().
		WithAccessTokenDuration("5m").
		WithRefreshTokenDuration("10m").
		Build()

	// Long-lived tokens for testing
	longTokenCfg := config.NewMockBuilder().
		WithAccessTokenDuration("24h").
		WithRefreshTokenDuration("30d").
		Build()

	assert.Equal(t, "5m", shortTokenCfg.Server.AccessTokenExpireDuration)
	assert.Equal(t, "24h", longTokenCfg.Server.AccessTokenExpireDuration)
}

// Example 5: Testing environment-specific behavior
func TestExample_EnvironmentSpecificConfig(t *testing.T) {
	environments := []struct {
		env    string
		config *config.Config
	}{
		{
			env: "development",
			config: config.NewMockBuilder().
				WithServerEnv("dev").
				WithServerHost("localhost").
				Build(),
		},
		{
			env: "production",
			config: config.NewMockBuilder().
				WithServerEnv("prod").
				WithServerHost("0.0.0.0").
				Build(),
		},
		{
			env: "testing",
			config: config.NewMockBuilder().
				WithServerEnv("test").
				Build(),
		},
	}

	for _, env := range environments {
		t.Run(env.env, func(t *testing.T) {
			// Test environment-specific logic
			assert.NotEmpty(t, env.config.Server.Env)
		})
	}
}

// Example 6: Using mock config with custom server configuration
func TestExample_CustomServerConfig(t *testing.T) {
	customServer := config.ServerConfig{
		Env:                        "staging",
		Host:                       "staging.internal",
		Port:                       "3000",
		Passphrase:                 "staging-pass",
		Secret:                     "staging-secret",
		Timeout:                    30,
		AccessTokenExpireDuration:  "20m",
		RefreshTokenExpireDuration: "2d",
	}

	cfg := config.NewMockBuilder().
		WithCustomServer(customServer).
		Build()

	assert.Equal(t, "staging", cfg.Server.Env)
	assert.Equal(t, 30, cfg.Server.Timeout)
}

// Example 7: Parallel test execution with isolated configs
func TestExample_ParallelTests(t *testing.T) {
	t.Run("test_1", func(t *testing.T) {
		t.Parallel()
		cfg := config.NewMockBuilder().
			WithServerPort("8081").
			Build()
		assert.Equal(t, "8081", cfg.Server.Port)
	})

	t.Run("test_2", func(t *testing.T) {
		t.Parallel()
		cfg := config.NewMockBuilder().
			WithServerPort("8082").
			Build()
		assert.Equal(t, "8082", cfg.Server.Port)
	})

	// Each test gets its own isolated config
}

// Example 8: Table-driven tests with different configs
func TestExample_TableDrivenTests(t *testing.T) {
	tests := []struct {
		name        string
		buildConfig func() *config.Config
		wantPort    string
		wantEnv     string
	}{
		{
			name: "default mock config",
			buildConfig: func() *config.Config {
				return config.NewMockConfig()
			},
			wantPort: "3625",
			wantEnv:  "test",
		},
		{
			name: "custom port",
			buildConfig: func() *config.Config {
				return config.NewMockBuilder().
					WithServerPort("9000").
					Build()
			},
			wantPort: "9000",
			wantEnv:  "test",
		},
		{
			name: "production config",
			buildConfig: func() *config.Config {
				return config.NewMockBuilder().
					WithServerEnv("prod").
					WithServerPort("443").
					Build()
			},
			wantPort: "443",
			wantEnv:  "prod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.buildConfig()
			assert.Equal(t, tt.wantPort, cfg.Server.Port)
			assert.Equal(t, tt.wantEnv, cfg.Server.Env)
		})
	}
}
