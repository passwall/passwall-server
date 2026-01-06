package email

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/passwall/passwall-server/internal/config"
)

// sesSender implements email sending via AWS SES
type sesSender struct {
	config          *config.EmailConfig
	frontendURL     string
	logger          Logger
	client          *sesv2.Client
	templateManager *TemplateManager
	region          string
}

// newSESSender creates a new AWS SES email sender
func newSESSender(cfg Config) (Sender, error) {
	if err := ValidateConfig(cfg.EmailConfig, ProviderAWSSES); err != nil {
		return nil, fmt.Errorf("invalid AWS SES config: %w", err)
	}

	templateManager, err := NewTemplateManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create template manager: %w", err)
	}

	sender := &sesSender{
		config:          cfg.EmailConfig,
		frontendURL:     cfg.FrontendURL,
		logger:          cfg.Logger,
		templateManager: templateManager,
	}

	// Initialize AWS SES client
	if err := sender.initClient(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to initialize AWS SES client: %w", err)
	}

	cfg.Logger.Info("AWS SES email sender initialized", "region", sender.region)
	return sender, nil
}

// initClient initializes the AWS SES client
func (s *sesSender) initClient(ctx context.Context) error {
	// Get AWS credentials
	accessKey := s.config.AccessKey
	secretKey := s.config.SecretKey

	if accessKey == "" || secretKey == "" {
		return fmt.Errorf("AWS SES requires access_key and secret_key")
	}

	// Determine region
	s.region = s.config.Region
	if s.region == "" {
		// Try to parse from host
		if strings.Contains(s.config.Host, "amazonaws.com") {
			parts := strings.Split(s.config.Host, ".")
			if len(parts) >= 2 {
				s.region = parts[1]
			}
		}

		// Default region
		if s.region == "" {
			s.region = "us-east-1"
		}
	}

	// Create AWS config
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(s.region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			accessKey,
			secretKey,
			"", // Session token (optional)
		)),
	)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create SES client
	s.client = sesv2.NewFromConfig(awsCfg)

	return nil
}

// SendVerificationEmail sends a verification email via AWS SES
func (s *sesSender) SendVerificationEmail(ctx context.Context, to, name, code string) error {
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
		s.logger.Error("failed to send verification email via AWS SES",
			"to", to,
			"error", err)
		return err
	}

	s.logger.Info("verification email sent successfully via AWS SES", "to", to)
	return nil
}

// SendInviteEmail sends an invitation email via AWS SES
func (s *sesSender) SendInviteEmail(ctx context.Context, to, role, desc string) error {
	data, err := BuildInviteEmail(s.frontendURL, to, role, desc)
	if err != nil {
		return fmt.Errorf("failed to build email data: %w", err)
	}

	htmlBody, err := s.templateManager.Render(TemplateInvite, data)
	if err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	subject := "You're invited to Passwall"
	if err := s.sendEmail(ctx, to, subject, htmlBody); err != nil {
		s.logger.Error("failed to send invite email via AWS SES",
			"to", to,
			"error", err)
		return err
	}

	s.logger.Info("invite email sent successfully via AWS SES", "to", to)
	return nil
}

// Provider returns the provider type
func (s *sesSender) Provider() Provider {
	return ProviderAWSSES
}

// Close closes the SES sender
func (s *sesSender) Close() error {
	// AWS SDK v2 doesn't require explicit cleanup
	return nil
}

// sendEmail sends an email via AWS SES
func (s *sesSender) sendEmail(ctx context.Context, to, subject, htmlBody string) error {
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

	// Construct from address
	fromAddress := fmt.Sprintf("%s <%s>", fromName, fromEmail)

	// Build destination with BCC if configured
	destination := &types.Destination{
		ToAddresses: []string{to},
	}
	if s.config.BCC != "" {
		destination.BccAddresses = []string{s.config.BCC}
	}

	// Build SES input
	input := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(fromAddress),
		Destination:      destination,
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: &types.Content{
					Data:    aws.String(subject),
					Charset: aws.String("UTF-8"),
				},
				Body: &types.Body{
					Html: &types.Content{
						Data:    aws.String(htmlBody),
						Charset: aws.String("UTF-8"),
					},
				},
			},
		},
	}

	// Send email
	result, err := s.client.SendEmail(ctx, input)
	if err != nil {
		return fmt.Errorf("AWS SES SendEmail failed: %w", err)
	}

	s.logger.Debug("AWS SES email sent",
		"messageId", aws.ToString(result.MessageId),
		"to", to)

	return nil
}
