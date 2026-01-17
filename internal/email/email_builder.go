package email

import "fmt"

// EmailBuilder helps build specific types of emails
// This is where business logic lives - not in the email clients
type EmailBuilder struct {
	frontendURL     string
	defaultFrom     string
	templateManager *TemplateManager
}

// NewEmailBuilder creates a new email builder
func NewEmailBuilder(frontendURL, defaultFrom string) (*EmailBuilder, error) {
	if frontendURL == "" {
		return nil, fmt.Errorf("frontend URL is required")
	}

	templateManager, err := NewTemplateManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create template manager: %w", err)
	}

	return &EmailBuilder{
		frontendURL:     frontendURL,
		defaultFrom:     defaultFrom,
		templateManager: templateManager,
	}, nil
}

// BuildVerificationEmail builds a verification email message
func (b *EmailBuilder) BuildVerificationEmail(to, name, code string) (*EmailMessage, error) {
	if to == "" {
		return nil, fmt.Errorf("recipient email is required")
	}

	if code == "" {
		return nil, fmt.Errorf("verification code is required")
	}

	// Build template data
	data, err := BuildVerificationEmail(b.frontendURL, to, name, code)
	if err != nil {
		return nil, fmt.Errorf("failed to build template data: %w", err)
	}

	// Render template
	htmlBody, err := b.templateManager.Render(TemplateVerification, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render template: %w", err)
	}

	return &EmailMessage{
		To:      to,
		From:    b.defaultFrom,
		Subject: "Verify Your Passwall Account",
		Body:    htmlBody,
	}, nil
}

// BuildInvitationEmail builds an invitation email message
func (b *EmailBuilder) BuildInvitationEmail(to, inviterName, code, role string) (*EmailMessage, error) {
	if to == "" {
		return nil, fmt.Errorf("recipient email is required")
	}

	if code == "" {
		return nil, fmt.Errorf("invitation code is required")
	}

	// Build template data
	data, err := BuildInvitationEmail(b.frontendURL, to, inviterName, code, role)
	if err != nil {
		return nil, fmt.Errorf("failed to build template data: %w", err)
	}

	// Render template
	htmlBody, err := b.templateManager.Render(TemplateInvitation, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render template: %w", err)
	}

	return &EmailMessage{
		To:      to,
		From:    b.defaultFrom,
		Subject: "You're Invited to Join Passwall!",
		Body:    htmlBody,
	}, nil
}

// BuildInvitationWithOrgEmail builds an invitation email with organization info
func (b *EmailBuilder) BuildInvitationWithOrgEmail(to, inviterName, code, role, orgName string) (*EmailMessage, error) {
	if to == "" {
		return nil, fmt.Errorf("recipient email is required")
	}

	if code == "" {
		return nil, fmt.Errorf("invitation code is required")
	}

	// Build template data
	data, err := BuildInvitationEmailWithOrg(b.frontendURL, to, inviterName, code, role, orgName)
	if err != nil {
		return nil, fmt.Errorf("failed to build template data: %w", err)
	}

	// Render template
	htmlBody, err := b.templateManager.Render(TemplateInvitation, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render template: %w", err)
	}

	subject := "You're Invited to Join Passwall"
	if orgName != "" {
		subject = fmt.Sprintf("Join %s on Passwall", orgName)
	}

	return &EmailMessage{
		To:      to,
		From:    b.defaultFrom,
		Subject: subject,
		Body:    htmlBody,
	}, nil
}

// BuildShareInviteEmail builds a share invite email for non-registered recipients
func (b *EmailBuilder) BuildShareInviteEmail(to, inviterName, itemName string) (*EmailMessage, error) {
	if to == "" {
		return nil, fmt.Errorf("recipient email is required")
	}

	data, err := BuildShareInviteEmail(b.frontendURL, to, inviterName, itemName)
	if err != nil {
		return nil, fmt.Errorf("failed to build template data: %w", err)
	}

	htmlBody, err := b.templateManager.Render(TemplateShareInvite, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render template: %w", err)
	}

	subject := "A secure password is shared with you"
	if inviterName != "" {
		subject = fmt.Sprintf("%s shared a password with you on Passwall", inviterName)
	}

	return &EmailMessage{
		To:      to,
		From:    b.defaultFrom,
		Subject: subject,
		Body:    htmlBody,
	}, nil
}

// BuildCustomEmail builds a custom email with provided subject and body
func (b *EmailBuilder) BuildCustomEmail(to, subject, htmlBody string) (*EmailMessage, error) {
	if to == "" {
		return nil, fmt.Errorf("recipient email is required")
	}

	if subject == "" {
		return nil, fmt.Errorf("subject is required")
	}

	if htmlBody == "" {
		return nil, fmt.Errorf("body is required")
	}

	return &EmailMessage{
		To:      to,
		From:    b.defaultFrom,
		Subject: subject,
		Body:    htmlBody,
	}, nil
}
