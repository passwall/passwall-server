package app

import (
	"context"

	brevo "github.com/getbrevo/brevo-go/lib"
	"github.com/spf13/viper"

	"github.com/passwall/passwall-server/pkg/logger"
)

// SendMail is a helper to send mail all over the project using Brevo
func SendMail(toName, toEmail string, subject, bodyHTML string) error {
	// Get Brevo API key from config
	apiKey := viper.GetString("email.apikey")
	if apiKey == "" {
		logger.Errorf("Brevo API key not configured")
		return nil // Return nil to not break the flow if email is not configured
	}

	// Create context
	ctx := context.Background()

	// Configure Brevo client
	cfg := brevo.NewConfiguration()
	cfg.AddDefaultHeader("api-key", apiKey)

	// Create API client
	client := brevo.NewAPIClient(cfg)

	// Prepare email
	sender := &brevo.SendSmtpEmailSender{
		Name:  viper.GetString("email.fromname"),
		Email: viper.GetString("email.fromemail"),
	}

	to := []brevo.SendSmtpEmailTo{
		{
			Email: toEmail,
			Name:  toName,
		},
	}

	email := brevo.SendSmtpEmail{
		Sender:      sender,
		To:          to,
		Subject:     subject,
		HtmlContent: bodyHTML,
	}

	// Send email
	result, _, err := client.TransactionalEmailsApi.SendTransacEmail(ctx, email)
	if err != nil {
		logger.Errorf("Failed to send email to '%s' error: %v", toEmail, err)
		return err
	}

	logger.Infof("Email sent successfully to '%s', MessageID: %s", toEmail, result.MessageId)
	return nil
}
