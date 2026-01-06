package email

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/passwall/passwall-server/internal/config"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// gmailSender implements email sending via Gmail API
type gmailSender struct {
	config          *config.EmailConfig
	frontendURL     string
	logger          Logger
	service         *gmail.Service
	templateManager *TemplateManager
}

// newGmailSender creates a new Gmail API email sender
func newGmailSender(cfg Config) (Sender, error) {
	// Validate all required credentials
	if cfg.EmailConfig.GmailClientID == "" {
		return nil, fmt.Errorf("gmail_client_id is required for Gmail API")
	}

	if cfg.EmailConfig.GmailClientSecret == "" {
		return nil, fmt.Errorf("gmail_client_secret is required for Gmail API")
	}

	if cfg.EmailConfig.GmailRefreshToken == "" {
		cfg.Logger.Error("Gmail API refresh token missing in config")
		cfg.Logger.Info("To get refresh token, use OAuth2 Playground:")
		cfg.Logger.Info("1. Go to: https://developers.google.com/oauthplayground/")
		cfg.Logger.Info("2. Settings (⚙️) → Use your own OAuth credentials")
		cfg.Logger.Info("3. Select Gmail API v1 → gmail.send scope")
		cfg.Logger.Info("4. Authorize and exchange code for tokens")
		cfg.Logger.Info("5. Copy refresh_token to config.yml")
		return nil, fmt.Errorf("gmail_refresh_token is required for Gmail API - see logs for setup instructions")
	}

	templateManager, err := NewTemplateManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create template manager: %w", err)
	}

	sender := &gmailSender{
		config:          cfg.EmailConfig,
		frontendURL:     cfg.FrontendURL,
		logger:          cfg.Logger,
		templateManager: templateManager,
	}

	// Initialize Gmail API service
	if err := sender.initService(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to initialize Gmail API service: %w", err)
	}

	cfg.Logger.Info("Gmail API email sender initialized successfully")
	return sender, nil
}

// initService initializes the Gmail API service
func (s *gmailSender) initService(ctx context.Context) error {
	// Create OAuth2 config
	oauth2Config := &oauth2.Config{
		ClientID:     s.config.GmailClientID,
		ClientSecret: s.config.GmailClientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       []string{gmail.GmailSendScope},
	}

	// Create token from refresh token
	token := &oauth2.Token{
		RefreshToken: s.config.GmailRefreshToken,
		TokenType:    "Bearer",
	}

	// Create HTTP client with OAuth2
	httpClient := oauth2Config.Client(ctx, token)

	// Create Gmail service
	service, err := gmail.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return fmt.Errorf("failed to create Gmail service: %w", err)
	}

	s.service = service
	return nil
}

// SendVerificationEmail sends a verification email via Gmail API
func (s *gmailSender) SendVerificationEmail(ctx context.Context, to, name, code string) error {
	// Build template data
	data, err := BuildVerificationEmail(s.frontendURL, to, name, code)
	if err != nil {
		return fmt.Errorf("failed to build email data: %w", err)
	}

	// Render template
	htmlBody, err := s.templateManager.Render(TemplateVerification, data)
	if err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	// Send email
	subject := "Verify Your Passwall Account"
	if err := s.sendEmail(ctx, to, subject, htmlBody); err != nil {
		s.logger.Error("failed to send verification email via Gmail API",
			"to", to,
			"error", err)
		return err
	}

	s.logger.Info("verification email sent successfully via Gmail API", "to", to)
	return nil
}

// Provider returns the provider type
func (s *gmailSender) Provider() Provider {
	return ProviderGmailAPI
}

// Close closes the Gmail sender
func (s *gmailSender) Close() error {
	// Gmail API doesn't require explicit cleanup
	return nil
}

// sendEmail sends an email via Gmail API
func (s *gmailSender) sendEmail(ctx context.Context, to, subject, htmlBody string) error {
	// Set timeout
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Get sender information
	fromEmail := s.config.FromEmail
	if fromEmail == "" {
		fromEmail = s.config.Username
	}

	fromName := s.config.FromName
	if fromName == "" {
		fromName = "Passwall"
	}

	// Build email message
	message, err := s.buildMessage(fromEmail, fromName, to, subject, htmlBody)
	if err != nil {
		return fmt.Errorf("failed to build message: %w", err)
	}

	// Send email
	_, err = s.service.Users.Messages.Send("me", message).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("Gmail API SendEmail failed: %w", err)
	}

	s.logger.Debug("Gmail API email sent successfully", "to", to)
	return nil
}

// buildMessage builds a Gmail API message in RFC 2822 format
func (s *gmailSender) buildMessage(fromEmail, fromName, to, subject, htmlBody string) (*gmail.Message, error) {
	var buf bytes.Buffer

	// Construct from address
	fromAddress := fmt.Sprintf("%s <%s>", fromName, fromEmail)

	// Build RFC 2822 email headers
	buf.WriteString(fmt.Sprintf("From: %s\r\n", fromAddress))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", to))

	// Add BCC if configured
	if s.config.BCC != "" {
		buf.WriteString(fmt.Sprintf("Bcc: %s\r\n", s.config.BCC))
	}

	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	buf.WriteString("Content-Transfer-Encoding: base64\r\n")
	buf.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))
	buf.WriteString("\r\n")

	// Encode body in base64
	encodedBody := base64.StdEncoding.EncodeToString([]byte(htmlBody))
	buf.WriteString(encodedBody)

	// Encode entire message in base64url
	encodedMessage := base64.URLEncoding.EncodeToString(buf.Bytes())

	return &gmail.Message{
		Raw: encodedMessage,
	}, nil
}
