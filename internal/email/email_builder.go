package email

import (
	"fmt"
	"time"
)

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

// BuildShareNotificationEmail builds a share notification email for existing users
func (b *EmailBuilder) BuildShareNotificationEmail(to, inviterName, itemName string) (*EmailMessage, error) {
	if to == "" {
		return nil, fmt.Errorf("recipient email is required")
	}

	data, err := BuildShareNoticeEmail(b.frontendURL, to, inviterName, itemName)
	if err != nil {
		return nil, fmt.Errorf("failed to build template data: %w", err)
	}

	htmlBody, err := b.templateManager.Render(TemplateShareNotice, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render template: %w", err)
	}

	subject := "A secure item was shared with you"
	if inviterName != "" {
		subject = fmt.Sprintf("%s shared an item with you on Passwall", inviterName)
	}

	return &EmailMessage{
		To:      to,
		From:    b.defaultFrom,
		Subject: subject,
		Body:    htmlBody,
	}, nil
}

// BuildEmergencyInviteEmail builds an emergency access invitation email
func (b *EmailBuilder) BuildEmergencyInviteEmail(to, grantorName string) (*EmailMessage, error) {
	if to == "" {
		return nil, fmt.Errorf("recipient email is required")
	}

	emergencyURL := fmt.Sprintf("%s/settings/emergency-access", b.frontendURL)
	data := &TemplateData{
		GrantorName:  grantorName,
		EmergencyURL: emergencyURL,
		Year:         currentYear(),
	}

	htmlBody, err := b.templateManager.Render(TemplateEmergencyInvite, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render template: %w", err)
	}

	return &EmailMessage{
		To:      to,
		From:    b.defaultFrom,
		Subject: fmt.Sprintf("%s added you as an emergency contact on Passwall", grantorName),
		Body:    htmlBody,
	}, nil
}

// BuildEmergencyAcceptedEmail notifies grantor that grantee accepted
func (b *EmailBuilder) BuildEmergencyAcceptedEmail(to, granteeName string) (*EmailMessage, error) {
	if to == "" {
		return nil, fmt.Errorf("recipient email is required")
	}

	emergencyURL := fmt.Sprintf("%s/settings/emergency-access", b.frontendURL)
	data := &TemplateData{
		GranteeName:  granteeName,
		EmergencyURL: emergencyURL,
		Year:         currentYear(),
	}

	htmlBody, err := b.templateManager.Render(TemplateEmergencyAccepted, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render template: %w", err)
	}

	return &EmailMessage{
		To:      to,
		From:    b.defaultFrom,
		Subject: fmt.Sprintf("%s accepted your emergency access invitation", granteeName),
		Body:    htmlBody,
	}, nil
}

// BuildEmergencyRecoveryRequestEmail notifies grantor of recovery request
func (b *EmailBuilder) BuildEmergencyRecoveryRequestEmail(to, granteeName string) (*EmailMessage, error) {
	if to == "" {
		return nil, fmt.Errorf("recipient email is required")
	}

	emergencyURL := fmt.Sprintf("%s/settings/emergency-access", b.frontendURL)
	data := &TemplateData{
		GranteeName:  granteeName,
		EmergencyURL: emergencyURL,
		Year:         currentYear(),
	}

	htmlBody, err := b.templateManager.Render(TemplateEmergencyRecoveryReq, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render template: %w", err)
	}

	return &EmailMessage{
		To:      to,
		From:    b.defaultFrom,
		Subject: fmt.Sprintf("URGENT: %s is requesting emergency access to your vault", granteeName),
		Body:    htmlBody,
	}, nil
}

// BuildEmergencyRecoveryApprovedEmail notifies grantee that recovery was approved
func (b *EmailBuilder) BuildEmergencyRecoveryApprovedEmail(to string) (*EmailMessage, error) {
	if to == "" {
		return nil, fmt.Errorf("recipient email is required")
	}

	emergencyURL := fmt.Sprintf("%s/settings/emergency-access", b.frontendURL)
	data := &TemplateData{
		EmergencyURL: emergencyURL,
		Year:         currentYear(),
	}

	htmlBody, err := b.templateManager.Render(TemplateEmergencyRecoveryOK, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render template: %w", err)
	}

	return &EmailMessage{
		To:      to,
		From:    b.defaultFrom,
		Subject: "Your emergency access request has been approved",
		Body:    htmlBody,
	}, nil
}

func currentYear() int {
	return time.Now().Year()
}

// BuildSecureSendNotifyEmail builds a secure send notification email
func (b *EmailBuilder) BuildSecureSendNotifyEmail(to, senderName, sendName, sendURL string) (*EmailMessage, error) {
	if to == "" {
		return nil, fmt.Errorf("recipient email is required")
	}
	if sendURL == "" {
		return nil, fmt.Errorf("send URL is required")
	}

	data := &TemplateData{
		SendSenderName: senderName,
		SendName:       sendName,
		SendURL:        sendURL,
		Year:           currentYear(),
	}

	htmlBody, err := b.templateManager.Render(TemplateSendNotify, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render send notify template: %w", err)
	}

	subject := "You received a Secure Send"
	if senderName != "" {
		subject = fmt.Sprintf("%s sent you a secure message via Passwall", senderName)
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
