package config

// NewMockConfig creates a mock configuration for testing purposes
// This can be used in any test across the project without needing actual config files
func NewMockConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Env:                        "test",
			Host:                       "localhost",
			Port:                       "3625",
			Domain:                     "http://localhost:3625",
			Dir:                        "/tmp/test",
			Passphrase:                 "test-passphrase-for-encryption",
			Secret:                     "test-secret-for-jwt-tokens",
			Timeout:                    10,
			GeneratedPasswordLength:    16,
			AccessTokenExpireDuration:  "15m",
			RefreshTokenExpireDuration: "1h",
			APIKey:                     "test-api-key",
		},
		Database: DatabaseConfig{
			Name:     "passwall_test",
			Username: "test_user",
			Password: "test_password",
			Host:     "localhost",
			Port:     "5432",
			LogMode:  false,
			SSLMode:  "disable",
		},
		Email: EmailConfig{
			Host:      "smtp.test.local",
			Port:      "25",
			Username:  "test@passwall.io",
			Password:  "test_password",
			FromName:  "Passwall Test",
			FromEmail: "test@passwall.io",
			Admin:     "admin@passwall.io",
			APIKey:    "test-email-api-key",
		},
	}
}

// MockConfigBuilder provides a fluent interface for building mock configs
type MockConfigBuilder struct {
	config *Config
}

// NewMockBuilder creates a new mock config builder
func NewMockBuilder() *MockConfigBuilder {
	return &MockConfigBuilder{
		config: NewMockConfig(),
	}
}

// WithServerPort sets the server port
func (b *MockConfigBuilder) WithServerPort(port string) *MockConfigBuilder {
	b.config.Server.Port = port
	return b
}

// WithServerHost sets the server host
func (b *MockConfigBuilder) WithServerHost(host string) *MockConfigBuilder {
	b.config.Server.Host = host
	return b
}

// WithServerEnv sets the server environment
func (b *MockConfigBuilder) WithServerEnv(env string) *MockConfigBuilder {
	b.config.Server.Env = env
	return b
}

// WithPassphrase sets the server passphrase
func (b *MockConfigBuilder) WithPassphrase(passphrase string) *MockConfigBuilder {
	b.config.Server.Passphrase = passphrase
	return b
}

// WithSecret sets the JWT secret
func (b *MockConfigBuilder) WithSecret(secret string) *MockConfigBuilder {
	b.config.Server.Secret = secret
	return b
}

// WithAccessTokenDuration sets access token expiration
func (b *MockConfigBuilder) WithAccessTokenDuration(duration string) *MockConfigBuilder {
	b.config.Server.AccessTokenExpireDuration = duration
	return b
}

// WithRefreshTokenDuration sets refresh token expiration
func (b *MockConfigBuilder) WithRefreshTokenDuration(duration string) *MockConfigBuilder {
	b.config.Server.RefreshTokenExpireDuration = duration
	return b
}

// WithDatabaseHost sets the database host
func (b *MockConfigBuilder) WithDatabaseHost(host string) *MockConfigBuilder {
	b.config.Database.Host = host
	return b
}

// WithDatabasePort sets the database port
func (b *MockConfigBuilder) WithDatabasePort(port string) *MockConfigBuilder {
	b.config.Database.Port = port
	return b
}

// WithDatabaseName sets the database name
func (b *MockConfigBuilder) WithDatabaseName(name string) *MockConfigBuilder {
	b.config.Database.Name = name
	return b
}

// WithDatabaseUser sets the database username
func (b *MockConfigBuilder) WithDatabaseUser(username string) *MockConfigBuilder {
	b.config.Database.Username = username
	return b
}

// WithDatabasePassword sets the database password
func (b *MockConfigBuilder) WithDatabasePassword(password string) *MockConfigBuilder {
	b.config.Database.Password = password
	return b
}

// WithDatabaseSSLMode sets the database SSL mode
func (b *MockConfigBuilder) WithDatabaseSSLMode(sslMode string) *MockConfigBuilder {
	b.config.Database.SSLMode = sslMode
	return b
}

// WithEmailHost sets the email host
func (b *MockConfigBuilder) WithEmailHost(host string) *MockConfigBuilder {
	b.config.Email.Host = host
	return b
}

// WithEmailPort sets the email port
func (b *MockConfigBuilder) WithEmailPort(port string) *MockConfigBuilder {
	b.config.Email.Port = port
	return b
}

// WithEmailFrom sets the email from address
func (b *MockConfigBuilder) WithEmailFrom(email, name string) *MockConfigBuilder {
	b.config.Email.FromEmail = email
	b.config.Email.FromName = name
	return b
}

// WithCustomServer sets a custom server config
func (b *MockConfigBuilder) WithCustomServer(server ServerConfig) *MockConfigBuilder {
	b.config.Server = server
	return b
}

// WithCustomDatabase sets a custom database config
func (b *MockConfigBuilder) WithCustomDatabase(database DatabaseConfig) *MockConfigBuilder {
	b.config.Database = database
	return b
}

// WithCustomEmail sets a custom email config
func (b *MockConfigBuilder) WithCustomEmail(email EmailConfig) *MockConfigBuilder {
	b.config.Email = email
	return b
}

// Build returns the built config
func (b *MockConfigBuilder) Build() *Config {
	return b.config
}
