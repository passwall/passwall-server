package http

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/email"
)

type SupportHandler struct {
	emailSender email.Sender
	logger      email.Logger
}

// NewSupportHandler creates a new support handler
func NewSupportHandler(emailSender email.Sender, logger email.Logger) *SupportHandler {
	return &SupportHandler{
		emailSender: emailSender,
		logger:      logger,
	}
}

// SupportRequest represents a support/contact form submission
type SupportRequest struct {
	Name    string `json:"name" binding:"required,min=2,max=100"`
	Email   string `json:"email" binding:"required,email"`
	Subject string `json:"subject" binding:"required,min=3,max=200"`
	Message string `json:"message" binding:"required,min=10,max=2000"`
}

// SendSupportEmail handles support form submissions
// @Summary Send support request
// @Description Sends a support request email to support@passwall.io
// @Tags support
// @Accept json
// @Produce json
// @Param request body SupportRequest true "Support request details"
// @Success 200 {object} map[string]interface{} "message: support request sent successfully"
// @Failure 400 {object} map[string]interface{} "error: invalid request"
// @Failure 500 {object} map[string]interface{} "error: failed to send support request"
// @Router /api/support [post]
func (h *SupportHandler) SendSupportEmail(c *gin.Context) {
	ctx := c.Request.Context()

	var req SupportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"details": err.Error(),
		})
		return
	}

	// Check for disposable email
	if IsDisposableEmail(req.Email) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid email domain",
			"message": "Disposable email addresses are not allowed. Please use a permanent email address.",
		})
		return
	}

	// Get user info from context if authenticated
	var userEmail string
	if email, exists := c.Get("user_email"); exists {
		userEmail = email.(string)
	}

	// Log the support request
	h.logger.Info("Support request received",
		"from", req.Email,
		"name", req.Name,
		"subject", req.Subject,
		"authenticated_user", userEmail,
	)

	// Send support email
	if err := h.sendSupportNotification(ctx, &req, userEmail); err != nil {
		h.logger.Error("Failed to send support email",
			"error", err,
			"from", req.Email,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to send support request",
			"details": "Please try again later or contact us directly at support@passwall.io",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "support request sent successfully",
		"note":    "We will get back to you as soon as possible",
	})
}

// sendSupportNotification sends the support request to support@passwall.io
func (h *SupportHandler) sendSupportNotification(ctx context.Context, req *SupportRequest, authenticatedUser string) error {
	// Build email body
	body := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>Support Request</title>
</head>
<body style="margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; background-color: #f5f5f5;">
	<table width="100%%" cellpadding="0" cellspacing="0" style="background-color: #f5f5f5; padding: 40px 20px;">
		<tr>
			<td align="center">
				<table width="600" cellpadding="0" cellspacing="0" style="background-color: #ffffff; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1);">
					<!-- Header -->
					<tr>
						<td style="padding: 40px 40px 20px; text-align: center; border-bottom: 1px solid #e0e0e0;">
							<h1 style="margin: 0; font-size: 32px; font-weight: 700; color: #1a1a1a;">
								<span style="color: #3b82f6;">Pass</span>wall Support
							</h1>
						</td>
					</tr>
					
					<!-- Content -->
					<tr>
						<td style="padding: 40px;">
							<h2 style="margin: 0 0 20px; font-size: 24px; font-weight: 600; color: #1a1a1a;">
								New Support Request
							</h2>
							
							<div style="background-color: #f7fafc; padding: 20px; border-radius: 8px; margin: 20px 0; border-left: 4px solid #3b82f6;">
								<table width="100%%" cellpadding="0" cellspacing="0">
									<tr>
										<td style="padding: 8px 0;">
											<strong style="color: #4a5568;">From:</strong>
											<span style="color: #1a1a1a;">%s</span>
										</td>
									</tr>
									<tr>
										<td style="padding: 8px 0;">
											<strong style="color: #4a5568;">Email:</strong>
											<span style="color: #1a1a1a;">%s</span>
										</td>
									</tr>
									<tr>
										<td style="padding: 8px 0;">
											<strong style="color: #4a5568;">Subject:</strong>
											<span style="color: #1a1a1a;">%s</span>
										</td>
									</tr>
									%s
								</table>
							</div>

							<div style="background-color: #fff; padding: 20px; border: 1px solid #e5e7eb; border-radius: 8px; margin: 20px 0;">
								<h3 style="color: #374151; margin: 0 0 15px;">Message:</h3>
								<p style="white-space: pre-wrap; margin: 0; line-height: 1.6; color: #1a1a1a;">%s</p>
							</div>

							<div style="background-color: #dbeafe; border-left: 4px solid #3b82f6; padding: 16px; margin: 20px 0; border-radius: 4px;">
								<p style="margin: 0; font-size: 14px; color: #1e40af;">
									<strong>Reply to:</strong> %s
								</p>
							</div>
						</td>
					</tr>
					
					<!-- Footer -->
					<tr>
						<td style="padding: 30px 40px; background-color: #f7fafc; border-top: 1px solid #e0e0e0; border-radius: 0 0 8px 8px;">
							<p style="margin: 0 0 10px; font-size: 14px; color: #718096; text-align: center;">
								This message was sent via the Passwall Admin Panel Support Form
							</p>
							<p style="margin: 0; font-size: 12px; color: #a0aec0; text-align: center;">
								© %d Passwall. All rights reserved.
							</p>
						</td>
					</tr>
				</table>
			</td>
		</tr>
	</table>
</body>
</html>`, req.Name, req.Email, req.Subject,
		func() string {
			if authenticatedUser != "" {
				return fmt.Sprintf(`<tr>
										<td style="padding: 8px 0;">
											<strong style="color: #4a5568;">Authenticated User:</strong>
											<span style="color: #1a1a1a;">%s</span>
										</td>
									</tr>`, authenticatedUser)
			}
			return `<tr>
										<td style="padding: 8px 0;">
											<em style="color: #718096;">Sent by: Guest (not authenticated)</em>
										</td>
									</tr>`
		}(),
		req.Message, req.Email, time.Now().Year())

	// Send support email directly using EmailMessage
	subject := fmt.Sprintf("Support Request: %s", req.Subject)
	message := &email.EmailMessage{
		To:      "support@passwall.io",
		Subject: subject,
		Body:    body,
	}
	
	return h.emailSender.Send(ctx, message)
}

