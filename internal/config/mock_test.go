package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewMockConfig(t *testing.T) {
	cfg := NewMockConfig()

	assert.NotNil(t, cfg)
	assert.Equal(t, "test", cfg.Server.Env)
	assert.Equal(t, "localhost", cfg.Server.Host)
	assert.Equal(t, "3625", cfg.Server.Port)
	assert.Equal(t, "test-passphrase-for-encryption", cfg.Server.Passphrase)
	assert.Equal(t, "test-secret-for-jwt-tokens", cfg.Server.Secret)

	assert.Equal(t, "passwall_test", cfg.Database.Name)
	assert.Equal(t, "localhost", cfg.Database.Host)
	assert.Equal(t, "test_user", cfg.Database.Username)

	// Mock config should be valid
	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestMockConfigBuilder_Basic(t *testing.T) {
	cfg := NewMockBuilder().
		WithServerPort("8080").
		WithDatabaseName("custom_test_db").
		Build()

	assert.Equal(t, "8080", cfg.Server.Port)
	assert.Equal(t, "custom_test_db", cfg.Database.Name)
	// Other fields should have defaults
	assert.Equal(t, "test", cfg.Server.Env)
}

func TestMockConfigBuilder_ChainedCalls(t *testing.T) {
	cfg := NewMockBuilder().
		WithServerHost("0.0.0.0").
		WithServerPort("9000").
		WithServerEnv("production").
		WithPassphrase("custom-passphrase").
		WithSecret("custom-secret").
		WithAccessTokenDuration("30m").
		WithRefreshTokenDuration("7d").
		Build()

	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, "9000", cfg.Server.Port)
	assert.Equal(t, "production", cfg.Server.Env)
	assert.Equal(t, "custom-passphrase", cfg.Server.Passphrase)
	assert.Equal(t, "custom-secret", cfg.Server.Secret)
	assert.Equal(t, "30m", cfg.Server.AccessTokenExpireDuration)
	assert.Equal(t, "7d", cfg.Server.RefreshTokenExpireDuration)
}

func TestMockConfigBuilder_DatabaseConfig(t *testing.T) {
	cfg := NewMockBuilder().
		WithDatabaseHost("db.example.com").
		WithDatabasePort("5433").
		WithDatabaseName("my_test_db").
		WithDatabaseUser("admin").
		WithDatabasePassword("secure123").
		WithDatabaseSSLMode("require").
		Build()

	assert.Equal(t, "db.example.com", cfg.Database.Host)
	assert.Equal(t, "5433", cfg.Database.Port)
	assert.Equal(t, "my_test_db", cfg.Database.Name)
	assert.Equal(t, "admin", cfg.Database.Username)
	assert.Equal(t, "secure123", cfg.Database.Password)
	assert.Equal(t, "require", cfg.Database.SSLMode)
}

func TestMockConfigBuilder_EmailConfig(t *testing.T) {
	cfg := NewMockBuilder().
		WithEmailHost("smtp.example.com").
		WithEmailPort("587").
		WithEmailFrom("noreply@example.com", "Example App").
		Build()

	assert.Equal(t, "smtp.example.com", cfg.Email.Host)
	assert.Equal(t, "587", cfg.Email.Port)
	assert.Equal(t, "noreply@example.com", cfg.Email.FromEmail)
	assert.Equal(t, "Example App", cfg.Email.FromName)
}

func TestMockConfigBuilder_CustomStructs(t *testing.T) {
	customServer := ServerConfig{
		Env:        "staging",
		Port:       "4000",
		Passphrase: "staging-passphrase",
		Secret:     "staging-secret",
	}

	cfg := NewMockBuilder().
		WithCustomServer(customServer).
		Build()

	assert.Equal(t, "staging", cfg.Server.Env)
	assert.Equal(t, "4000", cfg.Server.Port)
	assert.Equal(t, "staging-passphrase", cfg.Server.Passphrase)
	assert.Equal(t, "staging-secret", cfg.Server.Secret)
}

// Example: Using mock config in a service test
func ExampleNewMockConfig_serviceTest() {
	// In your service tests, just import the config package and use the mock
	cfg := NewMockConfig()

	// Use the config in your service initialization
	_ = cfg.Server.Port
	_ = cfg.Database.Host
	// ... your service code
}

// Example: Using builder for specific test scenarios
func ExampleNewMockBuilder_customScenario() {
	// Create a config with specific values for your test
	cfg := NewMockBuilder().
		WithServerPort("8080").
		WithDatabaseName("integration_test_db").
		WithPassphrase("my-test-passphrase").
		Build()

	// Use in your test
	_ = cfg
}

// Example: Testing different environments
func ExampleNewMockBuilder_environments() {
	// Development config
	devCfg := NewMockBuilder().
		WithServerEnv("dev").
		WithDatabaseHost("localhost").
		Build()

	// Production config
	prodCfg := NewMockBuilder().
		WithServerEnv("prod").
		WithDatabaseHost("prod-db.internal").
		WithDatabaseSSLMode("require").
		Build()

	_ = devCfg
	_ = prodCfg
}
