package app

import (
	"github.com/spf13/viper"
	"gopkg.in/gomail.v2"

	"github.com/passwall/passwall-server/pkg/logger"
)

// SendMail is an helper to send mail all over the project
func SendMail(toName, toEmail string, subject, bodyHTML string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", m.FormatAddress(viper.GetString("email.fromemail"), viper.GetString("email.fromname")))
	m.SetHeader("To", m.FormatAddress(toEmail, toName))
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", bodyHTML)
	d := gomail.NewDialer(
		viper.GetString("email.host"),
		viper.GetInt("email.port"),
		viper.GetString("email.username"),
		viper.GetString("email.password"),
	)
	err := d.DialAndSend(m)
	if err != nil {
		logger.Errorf("Failed to send email to '%s' error: %v", toEmail, err)
	}
	return err
}
