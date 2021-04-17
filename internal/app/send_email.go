package app

import (
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"github.com/spf13/viper"
)

// SendMail is an helper to send mail all over the project
func SendMail(name, email string, subject, bodyHTML string) error {
	message := mail.NewSingleEmail(
		mail.NewEmail(viper.GetString("email.fromName"), viper.GetString("email.fromEmail")), // From.
		subject,
		mail.NewEmail(name, email), // To.
		"",                         // Body text.
		bodyHTML)
	if _, err := sendgrid.NewSendClient(viper.GetString("email.apiKey")).Send(message); err != nil {
		return err
	}
	return nil
}
