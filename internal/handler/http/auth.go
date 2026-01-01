package http

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/email"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
	"github.com/passwall/passwall-server/pkg/constants"
)

type AuthHandler struct {
	authService         service.AuthService
	verificationService service.VerificationService
	emailSender         email.Sender
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(
	authService service.AuthService,
	verificationService service.VerificationService,
	emailSender email.Sender,
) *AuthHandler {
	return &AuthHandler{
		authService:         authService,
		verificationService: verificationService,
		emailSender:         emailSender,
	}
}

// SignUp handles user registration
func (h *AuthHandler) SignUp(c *gin.Context) {
	ctx := c.Request.Context()

	var req domain.SignUpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
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

	user, err := h.authService.SignUp(ctx, &req)
	if err != nil {
		if errors.Is(err, repository.ErrAlreadyExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "user already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":     "user created successfully",
		"user_id":     user.ID,
		"email":       user.Email,
		"is_verified": user.IsVerified,
		"note":        "Please check your email for verification code",
	})
}

// SignIn handles user authentication
func (h *AuthHandler) SignIn(c *gin.Context) {
	ctx := c.Request.Context()

	var creds domain.Credentials
	if err := c.ShouldBindJSON(&creds); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	authResponse, err := h.authService.SignIn(ctx, &creds)
	if err != nil {
		if errors.Is(err, service.ErrUnauthorized) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
			return
		}
		// Check for email verification error
		if err.Error() == "email not verified" {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "email not verified",
				"message": "Please verify your email before signing in. Check your inbox for the verification code.",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "authentication failed"})
		return
	}

	c.JSON(http.StatusOK, authResponse)
}

// RefreshToken handles token refresh
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	ctx := c.Request.Context()

	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	tokenDetails, err := h.authService.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		if errors.Is(err, service.ErrExpiredToken) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired refresh token"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to refresh token"})
		return
	}

	c.JSON(http.StatusOK, tokenDetails)
}

// SignOut handles user sign out
func (h *AuthHandler) SignOut(c *gin.Context) {
	ctx := c.Request.Context()

	// Get user ID from context (set by auth middleware)
	userID, exists := c.Get(constants.ContextKeyUserID)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	if err := h.authService.SignOut(ctx, int(userID.(uint))); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to sign out"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "signed out successfully"})
}

// CheckToken validates a token
func (h *AuthHandler) CheckToken(c *gin.Context) {
	ctx := c.Request.Context()

	var req struct {
		Token string `json:"token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	claims, err := h.authService.ValidateToken(ctx, req.Token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":   true,
		"user_id": claims.UserID,
		"email":   claims.Email,
		"schema":  claims.Schema,
	})
}

// VerifyEmail handles email verification
func (h *AuthHandler) VerifyEmail(c *gin.Context) {
	ctx := c.Request.Context()

	code := c.Param("code")
	email := c.Query("email")

	if code == "" || email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "code and email are required"})
		return
	}

	err := h.verificationService.VerifyCode(ctx, email, code)
	if err != nil {
		if err.Error() == "verification code has expired" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "code expired",
				"message": "Verification code has expired. Please request a new one.",
			})
			return
		}
		if err.Error() == "verification code is invalid" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid code",
				"message": "Invalid verification code. Please check and try again.",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "verification failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Email verified successfully! You can now sign in.",
	})
}

// ResendVerificationCode resends the verification code
func (h *AuthHandler) ResendVerificationCode(c *gin.Context) {
	ctx := c.Request.Context()

	var req struct {
		Email string `json:"email" binding:"required,email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	// Generate new code
	code, err := h.verificationService.ResendCode(ctx, req.Email)
	if err != nil {
		if err.Error() == "email already verified" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "already verified",
				"message": "This email is already verified. You can sign in now.",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resend code"})
		return
	}

	// Send new verification email
	go func() {
		emailCtx := context.Background()
		if err := h.emailSender.SendVerificationEmail(emailCtx, req.Email, "", code); err != nil {
			// Log error but don't fail the request
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Verification code resent. Please check your email.",
	})
}
