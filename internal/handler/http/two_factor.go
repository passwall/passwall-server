package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
	"github.com/passwall/passwall-server/pkg/constants"
)

type TwoFactorHandler struct {
	authService service.AuthService
}

func NewTwoFactorHandler(authService service.AuthService) *TwoFactorHandler {
	return &TwoFactorHandler{authService: authService}
}

// Setup initiates 2FA setup (returns secret + QR URL + recovery codes)
func (h *TwoFactorHandler) Setup(c *gin.Context) {
	userID, exists := c.Get(constants.ContextKeyUserID)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	resp, err := h.authService.SetupTwoFactor(c.Request.Context(), userID.(uint))
	if err != nil {
		if errors.Is(err, service.ErrTwoFactorAlreadySetup) {
			c.JSON(http.StatusConflict, gin.H{"error": "two-factor authentication is already enabled"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to set up 2FA"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// Confirm validates a TOTP code and activates 2FA
func (h *TwoFactorHandler) Confirm(c *gin.Context) {
	userID, exists := c.Get(constants.ContextKeyUserID)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req domain.TwoFactorConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := h.authService.ConfirmTwoFactor(c.Request.Context(), userID.(uint), req.TOTPCode); err != nil {
		if errors.Is(err, service.ErrInvalidTOTP) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid TOTP code"})
			return
		}
		if errors.Is(err, service.ErrTwoFactorAlreadySetup) {
			c.JSON(http.StatusConflict, gin.H{"error": "two-factor authentication is already enabled"})
			return
		}
		if errors.Is(err, service.ErrTwoFactorNotSetup) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "please call setup first"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to confirm 2FA"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "two-factor authentication enabled successfully"})
}

// Disable turns off 2FA after verifying master password + TOTP
func (h *TwoFactorHandler) Disable(c *gin.Context) {
	userID, exists := c.Get(constants.ContextKeyUserID)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req domain.TwoFactorDisableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := h.authService.DisableTwoFactor(c.Request.Context(), userID.(uint), req.MasterPasswordHash, req.TOTPCode); err != nil {
		if errors.Is(err, service.ErrUnauthorized) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid master password"})
			return
		}
		if errors.Is(err, service.ErrInvalidTOTP) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid TOTP or recovery code"})
			return
		}
		if errors.Is(err, service.ErrTwoFactorNotSetup) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "two-factor authentication is not enabled"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to disable 2FA"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "two-factor authentication disabled successfully"})
}

// Status returns the 2FA status for the current user
func (h *TwoFactorHandler) Status(c *gin.Context) {
	userID, exists := c.Get(constants.ContextKeyUserID)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	resp, err := h.authService.GetTwoFactorStatus(c.Request.Context(), userID.(uint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get 2FA status"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// Compliance returns 2FA adoption statistics for an organization (admin only)
func (h *TwoFactorHandler) Compliance(c *gin.Context) {
	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}
	userID := GetCurrentUserID(c)

	resp, err := h.authService.GetTwoFactorCompliance(c.Request.Context(), userID, orgID)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get 2FA compliance data"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// Verify completes 2FA sign-in by validating the TOTP code
func (h *TwoFactorHandler) Verify(c *gin.Context) {
	var req domain.TwoFactorVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	authResponse, err := h.authService.VerifyTwoFactorSignIn(c.Request.Context(), req.TwoFactorToken, req.TOTPCode)
	if err != nil {
		if errors.Is(err, service.ErrInvalid2FAToken) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired two-factor token"})
			return
		}
		if errors.Is(err, service.ErrInvalidTOTP) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid TOTP code"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "two-factor verification failed"})
		return
	}

	c.JSON(http.StatusOK, authResponse)
}
