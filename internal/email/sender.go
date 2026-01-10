package email

import (
	"context"
	"fmt"

	"github.com/passwall/passwall-server/internal/config"
)

// Provider represents the email sending provider
type Provider string

const (
	ProviderSMTP     Provider = "smtp"
	ProviderAWSSES   Provider = "aws-ses"
	ProviderGmailAPI Provider = "gmail-api"
)

// EmailMessage represents an email to be sent
type EmailMessage struct {
	To      string   // Primary recipient
	From    string   // Sender email address
	Subject string   // Email subject
	Body    string   // HTML body content
	CC      []string // Carbon copy recipients (optional)
	BCC     []string // Blind carbon copy recipients (optional)
}

// Sender defines the interface for sending emails
// Email clients should only know how to send, not what to send
type Sender interface {
	// Send sends an email message
	Send(ctx context.Context, message *EmailMessage) error

	// Provider returns the current provider being used
	Provider() Provider

	// Close closes any open connections and cleans up resources
	Close() error
}

// Logger interface for logging
type Logger interface {
	Debug(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

// Config holds email sender configuration
type Config struct {
	EmailConfig *config.EmailConfig
	FrontendURL string
	Logger      Logger
}

// NewSender creates a new email sender based on configuration
// It automatically detects the provider based on available credentials
// Priority: Gmail API > AWS SES > SMTP
func NewSender(cfg Config) (Sender, error) {
	if cfg.EmailConfig == nil {
		return nil, fmt.Errorf("email config is required")
	}

	if cfg.Logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	// Detect and create appropriate provider
	provider := detectProvider(cfg.EmailConfig)

	cfg.Logger.Info("Attempting to initialize email sender", "provider", provider)

	switch provider {
	case ProviderGmailAPI:
		// Gmail API selected - must succeed, no fallback
		return newGmailSender(cfg)

	case ProviderAWSSES:
		// AWS SES selected - must succeed, no fallback
		return newSESSender(cfg)

	case ProviderSMTP:
		return newSMTPSender(cfg)

	default:
		return nil, fmt.Errorf("unsupported email provider: %s", provider)
	}
}

// detectProvider determines which email provider to use based on configuration
func detectProvider(cfg *config.EmailConfig) Provider {
	// Priority 1: Gmail API
	// Note: refresh_token is optional - will be auto-generated if missing
	if cfg.GmailClientID != "" && cfg.GmailClientSecret != "" {
		return ProviderGmailAPI
	}

	// Priority 2: AWS SES
	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		return ProviderAWSSES
	}

	// Priority 3: SMTP (fallback)
	return ProviderSMTP
}

// ValidateConfig validates email configuration for a specific provider
func ValidateConfig(cfg *config.EmailConfig, provider Provider) error {
	// Common validation
	if cfg.FromEmail == "" {
		return fmt.Errorf("from_email is required")
	}

	switch provider {
	case ProviderGmailAPI:
		if cfg.GmailClientID == "" {
			return fmt.Errorf("gmail_client_id is required for Gmail API")
		}
		if cfg.GmailClientSecret == "" {
			return fmt.Errorf("gmail_client_secret is required for Gmail API")
		}
		// Note: refresh_token will be auto-generated if missing via interactive setup

	case ProviderAWSSES:
		if cfg.AccessKey == "" {
			return fmt.Errorf("access_key is required for AWS SES")
		}
		if cfg.SecretKey == "" {
			return fmt.Errorf("secret_key is required for AWS SES")
		}

	case ProviderSMTP:
		if cfg.Host == "" {
			return fmt.Errorf("host is required for SMTP")
		}
		if cfg.Port == "" {
			return fmt.Errorf("port is required for SMTP")
		}
		if cfg.Username == "" {
			return fmt.Errorf("username is required for SMTP")
		}
		if cfg.Password == "" {
			return fmt.Errorf("password is required for SMTP")
		}

	default:
		return fmt.Errorf("unknown provider: %s", provider)
	}

	return nil
}
