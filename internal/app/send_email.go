package app

import (
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"github.com/spf13/viper"
)

// SendMail is an helper to send mail all over the project
func SendMail(name, email string, subject, bodyHTML string) error {
	from := mail.NewEmail(viper.GetString("email.fromName"), viper.GetString("email.fromEmail"))
	to := mail.NewEmail(name, email)
	bodyText := ""
	message := mail.NewSingleEmail(from, subject, to, bodyText, bodyHTML)
	client := sendgrid.NewSendClient(viper.GetString("email.apiKey"))
	_, err := client.Send(message)
	if err != nil {
		return err
	}
	return nil
}
