package email

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/passwall/passwall-server/internal/config"
)

// smtpSender implements email sending via SMTP
type smtpSender struct {
	config *config.EmailConfig
	logger Logger
}

// newSMTPSender creates a new SMTP email sender
func newSMTPSender(cfg Config) (Sender, error) {
	if err := ValidateConfig(cfg.EmailConfig, ProviderSMTP); err != nil {
		return nil, fmt.Errorf("invalid SMTP config: %w", err)
	}

	sender := &smtpSender{
		config: cfg.EmailConfig,
		logger: cfg.Logger,
	}

	cfg.Logger.Info("SMTP email sender initialized",
		"host", cfg.EmailConfig.Host,
		"port", cfg.EmailConfig.Port)

	return sender, nil
}

// Send sends an email via SMTP
func (s *smtpSender) Send(ctx context.Context, message *EmailMessage) error {
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

	// Determine sender email
	from := message.From
	if from == "" {
		from = s.config.FromEmail
		if from == "" {
			from = s.config.Username
		}
	}

	if from == "" {
		return fmt.Errorf("sender email (From) is required")
	}

	fromName := s.config.FromName
	if fromName == "" {
		fromName = "Passwall"
	}

	// Build recipients list (To + CC + BCC + config BCC)
	recipients := []string{message.To}
	recipients = append(recipients, message.CC...)
	recipients = append(recipients, message.BCC...)
	if s.config.BCC != "" {
		recipients = append(recipients, s.config.BCC)
	}

	// Build SMTP message
	msg := s.buildMessage(from, fromName, message)

	// SMTP authentication
	auth := smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)

	// Server address
	addr := fmt.Sprintf("%s:%s", s.config.Host, s.config.Port)

	// Retry with exponential backoff
	maxRetries := 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Send with TLS if port is 587 (STARTTLS) or 465 (TLS)
		var err error
		if s.config.Port == "587" || s.config.Port == "465" {
			err = s.sendWithTLS(addr, auth, from, recipients, msg)
		} else {
			err = smtp.SendMail(addr, auth, from, recipients, msg)
		}

		if err == nil {
			s.logger.Info("email sent successfully via SMTP",
				"to", message.To,
				"subject", message.Subject)
			return nil
		}

		lastErr = err
		s.logger.Warn("SMTP send failed, retrying",
			"attempt", attempt,
			"max_retries", maxRetries,
			"error", err)

		// Exponential backoff
		if attempt < maxRetries {
			backoff := time.Duration(attempt) * time.Second
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	s.logger.Error("failed to send email via SMTP",
		"to", message.To,
		"subject", message.Subject,
		"error", lastErr)

	return fmt.Errorf("SMTP send failed after %d attempts: %w", maxRetries, lastErr)
}

// Provider returns the provider type
func (s *smtpSender) Provider() Provider {
	return ProviderSMTP
}

// Close closes the SMTP sender
func (s *smtpSender) Close() error {
	// SMTP doesn't maintain persistent connections
	return nil
}

// sendWithTLS sends email with TLS support
func (s *smtpSender) sendWithTLS(addr string, auth smtp.Auth, from string, recipients []string, msg []byte) error {
	tlsConfig := &tls.Config{
		ServerName: s.config.Host,
		MinVersion: tls.VersionTLS12,
	}

	// Try direct TLS connection first (port 465)
	if s.config.Port == "465" {
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("TLS dial failed: %w", err)
		}
		defer conn.Close()

		return s.sendViaClient(conn, auth, from, recipients, msg)
	}

	// Use STARTTLS (port 587)
	return s.sendWithSTARTTLS(addr, auth, from, recipients, msg)
}

// sendWithSTARTTLS sends email using STARTTLS
func (s *smtpSender) sendWithSTARTTLS(addr string, auth smtp.Auth, from string, recipients []string, msg []byte) error {
	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("SMTP dial failed: %w", err)
	}
	defer client.Quit()

	// Use STARTTLS if available
	if ok, _ := client.Extension("STARTTLS"); ok {
		tlsConfig := &tls.Config{
			ServerName: s.config.Host,
			MinVersion: tls.VersionTLS12,
		}
		if err = client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("STARTTLS failed: %w", err)
		}
	}

	// Authenticate
	if auth != nil {
		if err = client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth failed: %w", err)
		}
	}

	// Set sender
	if err = client.Mail(from); err != nil {
		return fmt.Errorf("MAIL command failed: %w", err)
	}

	// Set recipients (To + BCC)
	for _, recipient := range recipients {
		if err = client.Rcpt(recipient); err != nil {
			return fmt.Errorf("RCPT command failed for %s: %w", recipient, err)
		}
	}

	// Send data
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA command failed: %w", err)
	}

	if _, err = w.Write(msg); err != nil {
		return fmt.Errorf("write message failed: %w", err)
	}

	if err = w.Close(); err != nil {
		return fmt.Errorf("close message failed: %w", err)
	}

	return nil
}

// sendViaClient sends email via an existing connection
func (s *smtpSender) sendViaClient(conn net.Conn, auth smtp.Auth, from string, recipients []string, msg []byte) error {
	client, err := smtp.NewClient(conn, s.config.Host)
	if err != nil {
		return fmt.Errorf("create SMTP client failed: %w", err)
	}
	defer client.Quit()

	if auth != nil {
		if err = client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth failed: %w", err)
		}
	}

	if err = client.Mail(from); err != nil {
		return fmt.Errorf("MAIL command failed: %w", err)
	}

	// Set all recipients (To + BCC)
	for _, recipient := range recipients {
		if err = client.Rcpt(recipient); err != nil {
			return fmt.Errorf("RCPT command failed for %s: %w", recipient, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA command failed: %w", err)
	}

	if _, err = w.Write(msg); err != nil {
		return fmt.Errorf("write message failed: %w", err)
	}

	if err = w.Close(); err != nil {
		return fmt.Errorf("close message failed: %w", err)
	}

	return nil
}

// buildMessage builds the email message with headers
func (s *smtpSender) buildMessage(from, fromName string, message *EmailMessage) []byte {
	var buf bytes.Buffer

	// Headers
	buf.WriteString(fmt.Sprintf("From: %s <%s>\r\n", fromName, from))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", message.To))

	// Add CC if present
	if len(message.CC) > 0 {
		buf.WriteString(fmt.Sprintf("Cc: %s\r\n", strings.Join(message.CC, ", ")))
	}

	// Add BCC if present (including config BCC)
	bccList := make([]string, 0)
	bccList = append(bccList, message.BCC...)
	if s.config.BCC != "" {
		bccList = append(bccList, s.config.BCC)
	}
	if len(bccList) > 0 {
		buf.WriteString(fmt.Sprintf("Bcc: %s\r\n", strings.Join(bccList, ", ")))
	}

	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", message.Subject))
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	buf.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	buf.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))
	buf.WriteString("\r\n")

	// Body
	buf.WriteString(strings.TrimSpace(message.Body))

	return buf.Bytes()
}
