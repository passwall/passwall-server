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
	config *config.EmailConfig
	logger Logger
	client *sesv2.Client
	region string
}

// newSESSender creates a new AWS SES email sender
func newSESSender(cfg Config) (Sender, error) {
	if err := ValidateConfig(cfg.EmailConfig, ProviderAWSSES); err != nil {
		return nil, fmt.Errorf("invalid AWS SES config: %w", err)
	}

	sender := &sesSender{
		config: cfg.EmailConfig,
		logger: cfg.Logger,
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

// Send sends an email via AWS SES
func (s *sesSender) Send(ctx context.Context, message *EmailMessage) error {
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

	// Construct from address
	fromAddress := fmt.Sprintf("%s <%s>", fromName, fromEmail)

	// Build destination with To, CC, BCC
	destination := &types.Destination{
		ToAddresses: []string{message.To},
	}

	// Add CC if present
	if len(message.CC) > 0 {
		destination.CcAddresses = message.CC
	}

	// Build BCC list (message BCC + config BCC)
	bccList := make([]string, 0)
	bccList = append(bccList, message.BCC...)
	if s.config.BCC != "" {
		bccList = append(bccList, s.config.BCC)
	}
	if len(bccList) > 0 {
		destination.BccAddresses = bccList
	}

	// Build SES input
	input := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(fromAddress),
		Destination:      destination,
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: &types.Content{
					Data:    aws.String(message.Subject),
					Charset: aws.String("UTF-8"),
				},
				Body: &types.Body{
					Html: &types.Content{
						Data:    aws.String(message.Body),
						Charset: aws.String("UTF-8"),
					},
				},
			},
		},
	}

	// Send email
	result, err := s.client.SendEmail(ctx, input)
	if err != nil {
		s.logger.Error("failed to send email via AWS SES",
			"to", message.To,
			"subject", message.Subject,
			"error", err)
		return fmt.Errorf("AWS SES SendEmail failed: %w", err)
	}

	s.logger.Info("email sent successfully via AWS SES",
		"messageId", aws.ToString(result.MessageId),
		"to", message.To,
		"subject", message.Subject)

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

