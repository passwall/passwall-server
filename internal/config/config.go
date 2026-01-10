package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all application configuration
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Email    EmailConfig    `mapstructure:"email"`
	Stripe   StripeConfig   `mapstructure:"stripe"`
}

// ServerConfig contains server-related configuration
type ServerConfig struct {
	Env                        string  `mapstructure:"env"`
	Host                       string  `mapstructure:"host"`
	Port                       string  `mapstructure:"port"`
	Domain                     string  `mapstructure:"domain"`
	Passphrase                 string  `mapstructure:"passphrase"`
	Secret                     string  `mapstructure:"secret"`
	Timeout                    int     `mapstructure:"timeout"`
	GeneratedPasswordLength    int     `mapstructure:"generated_password_length"`
	AccessTokenExpireDuration  string  `mapstructure:"access_token_expire_duration"`
	RefreshTokenExpireDuration string  `mapstructure:"refresh_token_expire_duration"`
	FrontendURL                string  `mapstructure:"frontend_url"`
	RecaptchaSecretKey         string  `mapstructure:"recaptcha_secret_key"`
	RecaptchaThreshold         float64 `mapstructure:"recaptcha_threshold"`
}

// DatabaseConfig contains database-related configuration
type DatabaseConfig struct {
	Name     string `mapstructure:"name"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	LogMode  bool   `mapstructure:"log_mode"`
	SSLMode  string `mapstructure:"ssl_mode"`
}

// EmailConfig contains email-related configuration
type EmailConfig struct {
	Host      string `mapstructure:"host"`
	Port      string `mapstructure:"port"`
	Username  string `mapstructure:"username"`
	Password  string `mapstructure:"password"`
	FromName  string `mapstructure:"from_name"`
	FromEmail string `mapstructure:"from_email"`
	Admin     string `mapstructure:"admin"`
	BCC       string `mapstructure:"bcc"` // BCC email for all outgoing emails
	// AWS SES specific fields
	AccessKey string `mapstructure:"access_key"` // AWS Access Key ID
	SecretKey string `mapstructure:"secret_key"` // AWS Secret Access Key
	Region    string `mapstructure:"region"`     // AWS Region (e.g., us-east-1)
	// Gmail API specific fields
	GmailClientID     string `mapstructure:"gmail_client_id"`     // Gmail OAuth2 Client ID
	GmailClientSecret string `mapstructure:"gmail_client_secret"` // Gmail OAuth2 Client Secret
	GmailRefreshToken string `mapstructure:"gmail_refresh_token"` // Gmail OAuth2 Refresh Token
}

// StripeConfig contains Stripe payment configuration
type StripeConfig struct {
	SecretKey      string `mapstructure:"secret_key"`       // Stripe Secret Key (sk_test_... or sk_live_...)
	WebhookSecret  string `mapstructure:"webhook_secret"`   // Webhook signing secret
	PublishableKey string `mapstructure:"publishable_key"`  // Publishable key for frontend
	
	// Plan definitions
	Plans []PlanConfig `mapstructure:"plans"`
}

// PlanConfig defines a subscription plan
type PlanConfig struct {
	Code           string         `mapstructure:"code"`            // Plan code (e.g., "personal-monthly")
	Name           string         `mapstructure:"name"`            // Display name (e.g., "Premium")
	BillingCycle   string         `mapstructure:"billing_cycle"`   // "monthly" or "yearly"
	PriceCents     int            `mapstructure:"price_cents"`     // Price in cents
	Currency       string         `mapstructure:"currency"`        // Currency code (e.g., "USD")
	TrialDays      int            `mapstructure:"trial_days"`      // Trial period in days
	MaxUsers       *int           `mapstructure:"max_users"`       // Max users (nil = unlimited)
	MaxCollections *int           `mapstructure:"max_collections"` // Max collections (nil = unlimited)
	MaxItems       *int           `mapstructure:"max_items"`       // Max items (nil = unlimited)
	StripePriceID  string         `mapstructure:"stripe_price_id"` // Stripe Price ID
	Features       PlanFeatures   `mapstructure:"features"`        // Feature flags
}

// PlanFeatures defines feature availability for a plan
type PlanFeatures struct {
	Sharing         bool `mapstructure:"sharing"`          // Item sharing enabled
	Teams           bool `mapstructure:"teams"`            // Team management enabled
	Audit           bool `mapstructure:"audit"`            // Audit logs enabled
	SSO             bool `mapstructure:"sso"`              // Single Sign-On enabled
	APIAccess       bool `mapstructure:"api_access"`       // API access enabled
	PrioritySupport bool `mapstructure:"priority_support"` // Priority support enabled
}

// LoaderOptions contains options for loading configuration
type LoaderOptions struct {
	ConfigFile string
	EnvPrefix  string
}

// Load loads configuration from file and environment variables
func Load(opts ...LoaderOptions) (*Config, error) {
	opt := LoaderOptions{
		ConfigFile: "./config/config.yml",
		EnvPrefix:  "PW",
	}
	if len(opts) > 0 {
		opt = opts[0]
	}

	v := viper.New()

	// Set configuration file
	if opt.ConfigFile != "" {
		v.SetConfigFile(opt.ConfigFile)
		v.SetConfigType("yaml")
	}

	// Set defaults first
	setDefaults(v)

	// Bind specific environment variables BEFORE AutomaticEnv
	// This is important for backwards compatibility
	bindEnvVariables(v)

	// Enable automatic env variable reading
	v.SetEnvPrefix(opt.EnvPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Check if config file exists
	if opt.ConfigFile != "" {
		if _, err := os.Stat(opt.ConfigFile); os.IsNotExist(err) {
			// Config file doesn't exist, create it
			if err := createDefaultConfigFile(v, opt.ConfigFile); err != nil {
				return nil, fmt.Errorf("failed to create default config file: %w", err)
			}
		}

		// Read config file
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Unmarshal configuration
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Server validation
	if c.Server.Port == "" {
		return fmt.Errorf("server.port is required")
	}
	if c.Server.Passphrase == "" || c.Server.Passphrase == "add-your-key-to-here" {
		return fmt.Errorf("server.passphrase must be set to a secure value")
	}
	if c.Server.Secret == "" || c.Server.Secret == "add-your-key-to-here" {
		return fmt.Errorf("server.secret must be set to a secure value")
	}
	if c.Server.FrontendURL == "" {
		return fmt.Errorf("server.frontend_url is required for email links, CORS, and OAuth redirects")
	}

	// Database validation
	if c.Database.Host == "" {
		return fmt.Errorf("database.host is required")
	}
	if c.Database.Name == "" {
		return fmt.Errorf("database.name is required")
	}
	if c.Database.Username == "" {
		return fmt.Errorf("database.username is required")
	}

	return nil
}

// setDefaults sets default values for all configuration options
func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.env", "prod")
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", "3625")
	v.SetDefault("server.domain", "https://vault.passwall.io")
	v.SetDefault("server.passphrase", generateSecureKey())
	v.SetDefault("server.secret", generateSecureKey())
	v.SetDefault("server.timeout", 24)
	v.SetDefault("server.generated_password_length", 16)
	v.SetDefault("server.access_token_expire_duration", "30m")
	v.SetDefault("server.refresh_token_expire_duration", "15d")
	v.SetDefault("server.frontend_url", "http://localhost:5173")
	v.SetDefault("server.recaptcha_secret_key", "")
	v.SetDefault("server.recaptcha_threshold", 0.5)

	// Database defaults
	v.SetDefault("database.name", "passwall")
	v.SetDefault("database.username", "postgres")
	v.SetDefault("database.password", "password")
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", "5432")
	v.SetDefault("database.log_mode", false)
	v.SetDefault("database.ssl_mode", "disable")

	// Email defaults
	v.SetDefault("email.host", "smtp.passwall.io")
	v.SetDefault("email.port", "25")
	v.SetDefault("email.username", "hello@passwall.io")
	v.SetDefault("email.password", "password")
	v.SetDefault("email.from_name", "Passwall")
	v.SetDefault("email.from_email", "hello@passwall.io")
	v.SetDefault("email.admin", "hello@passwall.io")
	v.SetDefault("email.bcc", "")
	v.SetDefault("email.access_key", "")
	v.SetDefault("email.secret_key", "")
	v.SetDefault("email.region", "us-east-1")
	v.SetDefault("email.gmail_client_id", "")
	v.SetDefault("email.gmail_client_secret", "")
	v.SetDefault("email.gmail_refresh_token", "")
}

// bindEnvVariables binds environment variables for backwards compatibility
func bindEnvVariables(v *viper.Viper) {
	// Server bindings
	v.BindEnv("server.env", "PW_ENV")
	v.BindEnv("server.host", "PW_SERVER_HOST")
	v.BindEnv("server.port", "PORT", "PW_SERVER_PORT")
	v.BindEnv("server.domain", "DOMAIN", "PW_SERVER_DOMAIN")
	v.BindEnv("server.passphrase", "PW_SERVER_PASSPHRASE")
	v.BindEnv("server.secret", "PW_SERVER_SECRET")
	v.BindEnv("server.timeout", "PW_SERVER_TIMEOUT")
	v.BindEnv("server.generated_password_length", "PW_SERVER_GENERATED_PASSWORD_LENGTH")
	v.BindEnv("server.access_token_expire_duration", "PW_SERVER_ACCESS_TOKEN_EXPIRE_DURATION")
	v.BindEnv("server.refresh_token_expire_duration", "PW_SERVER_REFRESH_TOKEN_EXPIRE_DURATION")
	v.BindEnv("server.frontend_url", "PW_SERVER_FRONTEND_URL", "FRONTEND_URL")
	v.BindEnv("server.recaptcha_secret_key", "PW_RECAPTCHA_SECRET_KEY", "RECAPTCHA_SECRET_KEY")
	v.BindEnv("server.recaptcha_threshold", "PW_RECAPTCHA_THRESHOLD", "RECAPTCHA_THRESHOLD")

	// Database bindings
	v.BindEnv("database.name", "PW_DB_NAME", "POSTGRES_DB")
	v.BindEnv("database.username", "PW_DB_USERNAME", "POSTGRES_USER")
	v.BindEnv("database.password", "PW_DB_PASSWORD", "POSTGRES_PASSWORD")
	v.BindEnv("database.host", "PW_DB_HOST", "POSTGRES_HOST")
	v.BindEnv("database.port", "PW_DB_PORT", "POSTGRES_PORT")
	v.BindEnv("database.log_mode", "PW_DB_LOG_MODE")
	v.BindEnv("database.ssl_mode", "PW_DB_SSL_MODE")

	// Email bindings
	v.BindEnv("email.host", "PW_EMAIL_HOST")
	v.BindEnv("email.port", "PW_EMAIL_PORT")
	v.BindEnv("email.username", "PW_EMAIL_USERNAME")
	v.BindEnv("email.password", "PW_EMAIL_PASSWORD")
	v.BindEnv("email.from_name", "PW_EMAIL_FROM_NAME")
	v.BindEnv("email.from_email", "PW_EMAIL_FROM_EMAIL")
	v.BindEnv("email.admin", "PW_EMAIL_ADMIN")
	v.BindEnv("email.bcc", "PW_EMAIL_BCC")
	v.BindEnv("email.access_key", "PW_EMAIL_ACCESS_KEY", "AWS_ACCESS_KEY_ID")
	v.BindEnv("email.secret_key", "PW_EMAIL_SECRET_KEY", "AWS_SECRET_ACCESS_KEY")
	v.BindEnv("email.region", "PW_EMAIL_REGION", "AWS_REGION", "AWS_DEFAULT_REGION")
	v.BindEnv("email.gmail_client_id", "PW_EMAIL_GMAIL_CLIENT_ID", "GMAIL_CLIENT_ID")
	v.BindEnv("email.gmail_client_secret", "PW_EMAIL_GMAIL_CLIENT_SECRET", "GMAIL_CLIENT_SECRET")
	v.BindEnv("email.gmail_refresh_token", "PW_EMAIL_GMAIL_REFRESH_TOKEN", "GMAIL_REFRESH_TOKEN")
}

// createDefaultConfigFile creates a config file with default values
func createDefaultConfigFile(v *viper.Viper, configFile string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(configFile)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}
	}

	// Write default config
	if err := v.SafeWriteConfigAs(configFile); err != nil {
		if os.IsExist(err) {
			return nil // Config file already exists
		}
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// generateSecureKey generates a cryptographically secure random key
func generateSecureKey() string {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		// Fallback to a placeholder that will fail validation
		return "add-your-key-to-here"
	}
	return base64.StdEncoding.EncodeToString(key)
}
