package email

import (
	"bytes"
	"fmt"
	"html/template"
	"time"
)

// TemplateType represents different email templates
type TemplateType string

const (
	TemplateVerification TemplateType = "verification"
	TemplateInvitation   TemplateType = "invitation"
)

// TemplateData holds data for email templates
type TemplateData struct {
	Name             string
	Code             string
	ExpiryTime       string
	Year             int
	VerificationURL  string
	InviterName      string
	Role             string
	InvitationURL    string
	OrganizationName string // For org invitations
}

// TemplateManager handles email template rendering
type TemplateManager struct {
	templates map[TemplateType]*template.Template
}

// NewTemplateManager creates a new template manager
func NewTemplateManager() (*TemplateManager, error) {
	tm := &TemplateManager{
		templates: make(map[TemplateType]*template.Template),
	}

	// Parse verification template
	verifyTmpl, err := template.New("verification").Parse(verificationEmailTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse verification template: %w", err)
	}
	tm.templates[TemplateVerification] = verifyTmpl

	// Parse invitation template
	inviteTmpl, err := template.New("invitation").Parse(invitationEmailTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse invitation template: %w", err)
	}
	tm.templates[TemplateInvitation] = inviteTmpl

	return tm, nil
}

// Render renders an email template with the given data
func (tm *TemplateManager) Render(templateType TemplateType, data interface{}) (string, error) {
	tmpl, ok := tm.templates[templateType]
	if !ok {
		return "", fmt.Errorf("template not found: %s", templateType)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// BuildVerificationEmail builds a verification email with the given parameters
func BuildVerificationEmail(frontendURL, to, name, code string) (*TemplateData, error) {
	if frontendURL == "" {
		return nil, fmt.Errorf("frontend URL is required for verification emails")
	}

	verificationURL := fmt.Sprintf("%s/verify-email?email=%s&code=%s", frontendURL, to, code)

	return &TemplateData{
		Name:            name,
		Code:            code,
		ExpiryTime:      "15 minutes",
		Year:            time.Now().Year(),
		VerificationURL: verificationURL,
	}, nil
}

// verificationEmailTemplate is the HTML template for verification emails
const verificationEmailTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Verify Your Email</title>
</head>
<body style="margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; background-color: #f5f5f5;">
    <table width="100%" cellpadding="0" cellspacing="0" style="background-color: #f5f5f5; padding: 40px 20px;">
        <tr>
            <td align="center">
                <table width="600" cellpadding="0" cellspacing="0" style="background-color: #ffffff; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1);">
                    <!-- Header -->
                    <tr>
                        <td style="padding: 40px 40px 20px; text-align: center; border-bottom: 1px solid #e0e0e0;">
                            <h1 style="margin: 0; font-size: 32px; font-weight: 700; color: #1a1a1a;">
                                <span style="color: #3b82f6;">Pass</span>wall
                            </h1>
                        </td>
                    </tr>
                    
                    <!-- Content -->
                    <tr>
                        <td style="padding: 40px;">
                            <h2 style="margin: 0 0 20px; font-size: 24px; font-weight: 600; color: #1a1a1a;">
                                Welcome{{if .Name}}, {{.Name}}{{end}}!
                            </h2>
                            <p style="margin: 0 0 20px; font-size: 16px; line-height: 1.6; color: #4a5568;">
                                Thank you for signing up with Passwall. To complete your registration and secure your account, please verify your email address.
                            </p>
                            
                            <!-- Verification Code -->
                            <table width="100%" cellpadding="0" cellspacing="0" style="margin: 30px 0;">
                                <tr>
                                    <td align="center" style="background-color: #f7fafc; padding: 30px; border-radius: 8px; border: 2px dashed #cbd5e0;">
                                        <p style="margin: 0 0 10px; font-size: 14px; color: #718096; text-transform: uppercase; letter-spacing: 1px; font-weight: 600;">
                                            Your Verification Code
                                        </p>
                                        <div style="font-size: 36px; font-weight: 700; color: #3b82f6; letter-spacing: 8px; font-family: 'Courier New', monospace;">
                                            {{.Code}}
                                        </div>
                                    </td>
                                </tr>
                            </table>
                            
                            <p style="margin: 0 0 20px; font-size: 16px; line-height: 1.6; color: #4a5568;">
                                Click the button below to verify your account instantly, or enter this code on the verification page.
                            </p>
                            
                            <!-- Verify Button -->
                            <table width="100%" cellpadding="0" cellspacing="0" style="margin: 20px 0;">
                                <tr>
                                    <td align="center">
                                        <a href="{{.VerificationURL}}" style="display: inline-block; padding: 14px 32px; background-color: #3b82f6; color: #ffffff; text-decoration: none; border-radius: 6px; font-weight: 600; font-size: 16px;">
                                            Verify My Account
                                        </a>
                                    </td>
                                </tr>
                            </table>
                            
                            <p style="margin: 0 0 10px; font-size: 14px; line-height: 1.6; color: #718096; text-align: center;">
                                Or copy and paste this link into your browser:
                            </p>
                            <p style="margin: 0 0 20px; font-size: 13px; line-height: 1.6; color: #3b82f6; text-align: center; word-break: break-all;">
                                {{.VerificationURL}}
                            </p>
                            
                            <!-- Warning -->
                            <div style="background-color: #fef3c7; border-left: 4px solid #f59e0b; padding: 16px; margin: 20px 0; border-radius: 4px;">
                                <p style="margin: 0; font-size: 14px; color: #92400e;">
                                    <strong>⚠️ Important:</strong> This code will expire in <strong>{{.ExpiryTime}}</strong>. If you didn't request this verification, please ignore this email.
                                </p>
                            </div>
                            
                            <p style="margin: 20px 0 0; font-size: 14px; line-height: 1.6; color: #718096;">
                                If you're having trouble, please contact our support team.
                            </p>
                        </td>
                    </tr>
                    
                    <!-- Footer -->
                    <tr>
                        <td style="padding: 30px 40px; background-color: #f7fafc; border-top: 1px solid #e0e0e0; border-radius: 0 0 8px 8px;">
                            <p style="margin: 0 0 10px; font-size: 14px; color: #718096; text-align: center;">
                                This is an automated message, please do not reply.
                            </p>
                            <p style="margin: 0; font-size: 12px; color: #a0aec0; text-align: center;">
                                © {{.Year}} Passwall. All rights reserved.
                            </p>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>
    </table>
</body>
</html>`

// BuildInvitationEmail builds an invitation email
func BuildInvitationEmail(frontendURL, to, inviterName, code, role string) (*TemplateData, error) {
	if frontendURL == "" {
		return nil, fmt.Errorf("frontend URL is required for invitation emails")
	}

	invitationURL := fmt.Sprintf("%s/sign-up?email=%s&invitation=%s", frontendURL, to, code)

	return &TemplateData{
		InviterName:   inviterName,
		Code:          code,
		Role:          role,
		ExpiryTime:    "7 days",
		Year:          time.Now().Year(),
		InvitationURL: invitationURL,
	}, nil
}

// BuildInvitationEmailWithOrg builds an invitation email with organization info
func BuildInvitationEmailWithOrg(frontendURL, to, inviterName, code, role, orgName string) (*TemplateData, error) {
	if frontendURL == "" {
		return nil, fmt.Errorf("frontend URL is required for invitation emails")
	}

	invitationURL := fmt.Sprintf("%s/sign-up?email=%s&invitation=%s", frontendURL, to, code)

	return &TemplateData{
		InviterName:      inviterName,
		Code:             code,
		Role:             role,
		ExpiryTime:       "7 days",
		Year:             time.Now().Year(),
		InvitationURL:    invitationURL,
		OrganizationName: orgName,
	}, nil
}

// invitationEmailTemplate is the HTML template for invitation emails
const invitationEmailTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>You're Invited to Passwall</title>
</head>
<body style="margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; background-color: #f5f5f5;">
    <table width="100%" cellpadding="0" cellspacing="0" style="background-color: #f5f5f5; padding: 40px 20px;">
        <tr>
            <td align="center">
                <table width="600" cellpadding="0" cellspacing="0" style="background-color: #ffffff; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1);">
                    <!-- Header -->
                    <tr>
                        <td style="padding: 40px 40px 20px; text-align: center; border-bottom: 1px solid #e0e0e0;">
                            <h1 style="margin: 0; font-size: 32px; font-weight: 700; color: #1a1a1a;">
                                <span style="color: #3b82f6;">Pass</span>wall
                            </h1>
                        </td>
                    </tr>
                    
                    <!-- Content -->
                    <tr>
                        <td style="padding: 40px;">
                            <h2 style="margin: 0 0 20px; font-size: 24px; font-weight: 600; color: #1a1a1a;">
                                🎉 You're Invited!
                            </h2>
                            <p style="margin: 0 0 20px; font-size: 16px; line-height: 1.6; color: #4a5568;">
                                <strong>{{.InviterName}}</strong> has invited you to join{{if .OrganizationName}} <strong>{{.OrganizationName}}</strong>{{else}} their Passwall team{{end}} as a <strong>{{.Role}}</strong>.
                            </p>
                            <p style="margin: 0 0 20px; font-size: 16px; line-height: 1.6; color: #4a5568;">
                                Passwall is a secure password manager that helps teams collaborate safely.{{if .OrganizationName}} Sign up to join the organization and start collaborating!{{else}} Accept this invitation to get started!{{end}}
                            </p>
                            
                            <!-- Join Button -->
                            <table width="100%" cellpadding="0" cellspacing="0" style="margin: 30px 0;">
                                <tr>
                                    <td align="center">
                                        <a href="{{.InvitationURL}}" style="display: inline-block; padding: 16px 40px; background-color: #10b981; color: #ffffff; text-decoration: none; border-radius: 6px; font-weight: 600; font-size: 16px;">
                                            Accept Invitation
                                        </a>
                                    </td>
                                </tr>
                            </table>
                            
                            <p style="margin: 0 0 10px; font-size: 14px; line-height: 1.6; color: #718096; text-align: center;">
                                Or copy and paste this link into your browser:
                            </p>
                            <p style="margin: 0 0 20px; font-size: 13px; line-height: 1.6; color: #3b82f6; text-align: center; word-break: break-all;">
                                {{.InvitationURL}}
                            </p>
                            
                            <!-- Info Box -->
                            <div style="background-color: #dbeafe; border-left: 4px solid #3b82f6; padding: 16px; margin: 20px 0; border-radius: 4px;">
                                <p style="margin: 0 0 10px; font-size: 14px; color: #1e40af;">
                                    <strong>📧 Your invitation code:</strong>
                                </p>
                                <p style="margin: 0; font-size: 18px; font-weight: 700; color: #1e40af; font-family: 'Courier New', monospace; letter-spacing: 2px;">
                                    {{.Code}}
                                </p>
                            </div>
                            
                            <!-- Warning -->
                            <div style="background-color: #fef3c7; border-left: 4px solid #f59e0b; padding: 16px; margin: 20px 0; border-radius: 4px;">
                                <p style="margin: 0; font-size: 14px; color: #92400e;">
                                    <strong>⚠️ Important:</strong> This invitation will expire in <strong>{{.ExpiryTime}}</strong>. If you didn't expect this invitation, you can safely ignore this email.
                                </p>
                            </div>
                        </td>
                    </tr>
                    
                    <!-- Footer -->
                    <tr>
                        <td style="padding: 30px 40px; background-color: #f7fafc; border-top: 1px solid #e0e0e0; border-radius: 0 0 8px 8px;">
                            <p style="margin: 0 0 10px; font-size: 14px; color: #718096; text-align: center;">
                                This is an automated message, please do not reply.
                            </p>
                            <p style="margin: 0; font-size: 12px; color: #a0aec0; text-align: center;">
                                © {{.Year}} Passwall. All rights reserved.
                            </p>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>
    </table>
</body>
</html>`
