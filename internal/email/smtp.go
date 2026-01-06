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
	config          *config.EmailConfig
	frontendURL     string
	logger          Logger
	templateManager *TemplateManager
}

// newSMTPSender creates a new SMTP email sender
func newSMTPSender(cfg Config) (Sender, error) {
	if err := ValidateConfig(cfg.EmailConfig, ProviderSMTP); err != nil {
		return nil, fmt.Errorf("invalid SMTP config: %w", err)
	}

	templateManager, err := NewTemplateManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create template manager: %w", err)
	}

	sender := &smtpSender{
		config:          cfg.EmailConfig,
		frontendURL:     cfg.FrontendURL,
		logger:          cfg.Logger,
		templateManager: templateManager,
	}

	cfg.Logger.Info("SMTP email sender initialized",
		"host", cfg.EmailConfig.Host,
		"port", cfg.EmailConfig.Port)

	return sender, nil
}

// SendVerificationEmail sends a verification email via SMTP
func (s *smtpSender) SendVerificationEmail(ctx context.Context, to, name, code string) error {
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
		s.logger.Error("failed to send verification email via SMTP",
			"to", to,
			"error", err)
		return err
	}

	s.logger.Info("verification email sent successfully via SMTP", "to", to)
	return nil
}

// SendInviteEmail sends an invitation email via SMTP
func (s *smtpSender) SendInviteEmail(ctx context.Context, to, role, desc string) error {
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
		s.logger.Error("failed to send invite email via SMTP",
			"to", to,
			"error", err)
		return err
	}

	s.logger.Info("invite email sent successfully via SMTP", "to", to)
	return nil
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

// sendEmail sends an email via SMTP
func (s *smtpSender) sendEmail(ctx context.Context, to, subject, htmlBody string) error {
	from := s.config.FromEmail
	if from == "" {
		from = s.config.Username
	}

	fromName := s.config.FromName
	if fromName == "" {
		fromName = "Passwall"
	}

	// Build message
	msg := s.buildMessage(from, fromName, to, subject, htmlBody)

	// SMTP authentication
	auth := smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)

	// Server address
	addr := fmt.Sprintf("%s:%s", s.config.Host, s.config.Port)

	// Recipients list (To + BCC)
	recipients := []string{to}
	if s.config.BCC != "" {
		recipients = append(recipients, s.config.BCC)
	}

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

	return fmt.Errorf("SMTP send failed after %d attempts: %w", maxRetries, lastErr)
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
func (s *smtpSender) buildMessage(from, fromName, to, subject, htmlBody string) []byte {
	var buf bytes.Buffer

	// Headers
	buf.WriteString(fmt.Sprintf("From: %s <%s>\r\n", fromName, from))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", to))

	// Add BCC if configured
	if s.config.BCC != "" {
		buf.WriteString(fmt.Sprintf("Bcc: %s\r\n", s.config.BCC))
	}

	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	buf.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	buf.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))
	buf.WriteString("\r\n")

	// Body
	buf.WriteString(strings.TrimSpace(htmlBody))

	return buf.Bytes()
}
