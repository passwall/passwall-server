package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_DefaultConfig(t *testing.T) {
	// Create temporary config file
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yml")

	cfg, err := Load(LoaderOptions{
		ConfigFile: configFile,
		EnvPrefix:  "PW",
	})

	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Verify defaults
	assert.Equal(t, "prod", cfg.Server.Env)
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, "3625", cfg.Server.Port)
	assert.NotEmpty(t, cfg.Server.Passphrase)
	assert.NotEmpty(t, cfg.Server.Secret)
	assert.Equal(t, 24, cfg.Server.Timeout)

	assert.Equal(t, "passwall", cfg.Database.Name)
	assert.Equal(t, "postgres", cfg.Database.Username)
	assert.Equal(t, "localhost", cfg.Database.Host)
	assert.Equal(t, "5432", cfg.Database.Port)
}

func TestLoad_WithEnvironmentVariables(t *testing.T) {
	// Set environment variables using t.Setenv (automatic cleanup)
	t.Setenv("PW_SERVER_PORT", "8080")
	t.Setenv("PW_DB_NAME", "test_db")
	t.Setenv("PW_DB_HOST", "testhost")

	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yml")

	cfg, err := Load(LoaderOptions{
		ConfigFile: configFile,
		EnvPrefix:  "PW",
	})

	require.NoError(t, err)
	assert.Equal(t, "8080", cfg.Server.Port)
	assert.Equal(t, "test_db", cfg.Database.Name)
	assert.Equal(t, "testhost", cfg.Database.Host)
}

func TestLoad_BackwardsCompatibleEnvVars(t *testing.T) {
	// Test backwards compatible environment variables
	// These must be set BEFORE calling Load()
	t.Setenv("PORT", "9000")
	t.Setenv("POSTGRES_DB", "legacy_db")
	t.Setenv("POSTGRES_USER", "legacy_user")

	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yml")

	cfg, err := Load(LoaderOptions{
		ConfigFile: configFile,
		EnvPrefix:  "PW",
	})

	require.NoError(t, err)
	assert.Equal(t, "9000", cfg.Server.Port)
	assert.Equal(t, "legacy_db", cfg.Database.Name)
	assert.Equal(t, "legacy_user", cfg.Database.Username)
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: &Config{
				Server: ServerConfig{
					Port:       "3625",
					Passphrase: "valid-passphrase",
					Secret:     "valid-secret",
				},
				Database: DatabaseConfig{
					Host:     "localhost",
					Name:     "passwall",
					Username: "user",
				},
			},
			wantErr: false,
		},
		{
			name: "missing server port",
			config: &Config{
				Server: ServerConfig{
					Passphrase: "valid-passphrase",
					Secret:     "valid-secret",
				},
				Database: DatabaseConfig{
					Host:     "localhost",
					Name:     "passwall",
					Username: "user",
				},
			},
			wantErr: true,
			errMsg:  "server.port is required",
		},
		{
			name: "invalid passphrase",
			config: &Config{
				Server: ServerConfig{
					Port:       "3625",
					Passphrase: "add-your-key-to-here",
					Secret:     "valid-secret",
				},
				Database: DatabaseConfig{
					Host:     "localhost",
					Name:     "passwall",
					Username: "user",
				},
			},
			wantErr: true,
			errMsg:  "server.passphrase must be set to a secure value",
		},
		{
			name: "missing database host",
			config: &Config{
				Server: ServerConfig{
					Port:       "3625",
					Passphrase: "valid-passphrase",
					Secret:     "valid-secret",
				},
				Database: DatabaseConfig{
					Name:     "passwall",
					Username: "user",
				},
			},
			wantErr: true,
			errMsg:  "database.host is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGenerateSecureKey(t *testing.T) {
	key1 := generateSecureKey()
	key2 := generateSecureKey()

	// Keys should be non-empty
	assert.NotEmpty(t, key1)
	assert.NotEmpty(t, key2)

	// Keys should be different (extremely unlikely to be the same)
	assert.NotEqual(t, key1, key2)

	// Keys should be base64 encoded
	assert.Regexp(t, "^[A-Za-z0-9+/]+=*$", key1)
	assert.Regexp(t, "^[A-Za-z0-9+/]+=*$", key2)
}

func TestInit_BackwardsCompatibility(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yml")

	cfg, err := Init(configFile)

	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "prod", cfg.Server.Env)
}

func TestLoad_NonExistentConfigFile(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yml")

	// Config file doesn't exist, should create it
	cfg, err := Load(LoaderOptions{
		ConfigFile: configFile,
		EnvPrefix:  "PW",
	})

	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Verify file was created
	_, err = os.Stat(configFile)
	assert.NoError(t, err, "config file should have been created")
}
