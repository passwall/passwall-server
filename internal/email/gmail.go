package email

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/passwall/passwall-server/internal/config"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// gmailSender implements email sending via Gmail API
type gmailSender struct {
	config  *config.EmailConfig
	logger  Logger
	service *gmail.Service
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

	sender := &gmailSender{
		config: cfg.EmailConfig,
		logger: cfg.Logger,
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

// Send sends an email via Gmail API
func (s *gmailSender) Send(ctx context.Context, message *EmailMessage) error {
	if message == nil {
		return fmt.Errorf("email message is required")
	}

	if message.To == "" {
		return fmt.Errorf("recipient (To) is required")
	}

	if message.Subject == "" {
		return fmt.Errorf("subject is required")
	}

	if message.Body == "" {
		return fmt.Errorf("body is required")
	}

	// Set timeout
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Get sender information
	fromEmail := message.From
	if fromEmail == "" {
		fromEmail = s.config.FromEmail
		if fromEmail == "" {
			fromEmail = s.config.Username
		}
	}

	if fromEmail == "" {
		return fmt.Errorf("sender email (From) is required")
	}

	fromName := s.config.FromName
	if fromName == "" {
		fromName = "Passwall"
	}

	// Build BCC list (message BCC + config BCC)
	bccList := make([]string, 0)
	bccList = append(bccList, message.BCC...)
	if s.config.BCC != "" {
		bccList = append(bccList, s.config.BCC)
	}

	// Build email message
	gmailMessage, err := s.buildMessage(fromEmail, fromName, message, bccList)
	if err != nil {
		return fmt.Errorf("failed to build message: %w", err)
	}

	// Send email
	_, err = s.service.Users.Messages.Send("me", gmailMessage).Context(ctx).Do()
	if err != nil {
		s.logger.Error("failed to send email via Gmail API",
			"to", message.To,
			"subject", message.Subject,
			"error", err)
		return fmt.Errorf("Gmail API SendEmail failed: %w", err)
	}

	s.logger.Info("email sent successfully via Gmail API",
		"to", message.To,
		"subject", message.Subject)

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

// buildMessage builds a Gmail API message in RFC 2822 format
func (s *gmailSender) buildMessage(fromEmail, fromName string, message *EmailMessage, bccList []string) (*gmail.Message, error) {
	var buf bytes.Buffer

	// Construct from address
	fromAddress := fmt.Sprintf("%s <%s>", fromName, fromEmail)

	// Build RFC 2822 email headers
	buf.WriteString(fmt.Sprintf("From: %s\r\n", fromAddress))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", message.To))

	// Add CC if present
	if len(message.CC) > 0 {
		buf.WriteString(fmt.Sprintf("Cc: %s\r\n", strings.Join(message.CC, ", ")))
	}

	// Add BCC if present
	if len(bccList) > 0 {
		buf.WriteString(fmt.Sprintf("Bcc: %s\r\n", strings.Join(bccList, ", ")))
	}

	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", message.Subject))
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	buf.WriteString("Content-Transfer-Encoding: base64\r\n")
	buf.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))
	buf.WriteString("\r\n")

	// Encode body in base64
	encodedBody := base64.StdEncoding.EncodeToString([]byte(message.Body))
	buf.WriteString(encodedBody)

	// Encode entire message in base64url
	encodedMessage := base64.URLEncoding.EncodeToString(buf.Bytes())

	return &gmail.Message{
		Raw: encodedMessage,
	}, nil
}
