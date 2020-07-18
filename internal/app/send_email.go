package app

import (
	"fmt"
	"log"
	"net/smtp"

	"github.com/spf13/viper"
)

// SendMail is an helper to send mail all over the project
func SendMail(to []string, subject, body string) {

	// Set up authentication information.
	auth := smtp.PlainAuth(
		"",
		viper.GetString("email.username"),
		viper.GetString("email.password"),
		viper.GetString("email.host"),
	)

	header := make(map[string]string)
	header["From"] = viper.GetString("email.from")
	header["To"] = to[0]
	header["Subject"] = subject
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = "text/plain; charset=\"utf-8\""

	message := ""
	for k, v := range header {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + body

	// Connect to the server, authenticate, set the sender and recipient,
	// and send the email all in one step.
	err := smtp.SendMail(
		viper.GetString("email.host")+":"+viper.GetString("email.port"),
		auth,
		viper.GetString("email.from"),
		to,
		[]byte(message),
	)
	if err != nil {
		log.Println(err)
	}
}
