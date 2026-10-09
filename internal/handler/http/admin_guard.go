package http

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/service"
	"github.com/passwall/passwall-server/pkg/constants"
)

// AdminStepUpHeader carries the short-lived token from POST /api/admin/step-up.
const AdminStepUpHeader = "X-Passwall-Step-Up"

// Stable error codes for admin step-up. All are 403: Vault treats 401 as an
// expired session and would sign the operator out.
const (
	adminCodeStepUpRequired     = "STEP_UP_REQUIRED"
	adminCodeStepUpFailed       = "STEP_UP_FAILED"
	adminCodeTwoFactorRequired  = "TWO_FACTOR_REQUIRED"
	adminAuditDetailsContextKey = "admin_audit_details"
)

func sessionUUIDFromContext(c *gin.Context) uuid.UUID {
	if value, ok := c.Get(constants.ContextKeySessionID); ok {
		if id, ok := value.(uuid.UUID); ok {
			return id
		}
	}
	return uuid.Nil
}

// RequireAdminStepUpMiddleware requires a valid step-up token for the current
// user and session. Use it after RequireSystemAdminMiddleware.
func RequireAdminStepUpMiddleware(authService service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := GetUserID(c)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "step-up required", "code": adminCodeStepUpRequired})
			return
		}
		if err := authService.VerifyAdminStepUp(c.GetHeader(AdminStepUpHeader), userID, sessionUUIDFromContext(c)); err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "step-up required", "code": adminCodeStepUpRequired})
			return
		}
		c.Next()
	}
}

// SetAdminAuditDetails lets a handler add fields to the audit entry written by
// AdminAuditMiddleware.
func SetAdminAuditDetails(c *gin.Context, details service.ActivityDetails) {
	existing, _ := c.Get(adminAuditDetailsContextKey)
	merged, _ := existing.(service.ActivityDetails)
	if merged == nil {
		merged = service.ActivityDetails{}
	}
	for key, value := range details {
		merged[key] = value
	}
	c.Set(adminAuditDetailsContextKey, merged)
}

// AdminAuditMiddleware records an admin action after the handler ran,
// including failures, with the route, result status and handler details.
func AdminAuditMiddleware(logger *service.ActivityLogger, activityType domain.ActivityType) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		actorID, err := GetUserID(c)
		if err != nil || logger == nil {
			return
		}
		details := service.ActivityDetails{
			"method": c.Request.Method,
			"route":  c.FullPath(),
			"status": c.Writer.Status(),
		}
		for _, param := range c.Params {
			details["param_"+param.Key] = param.Value
		}
		if extra, ok := c.Get(adminAuditDetailsContextKey); ok {
			if fields, ok := extra.(service.ActivityDetails); ok {
				for key, value := range fields {
					details[key] = value
				}
			}
		}
		// Detached from the request so a client disconnect cannot drop the entry.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = logger.LogActivity(ctx, actorID, activityType, GetIPAddress(c), GetUserAgent(c), details)
	}
}

// AdminStepUpHandler issues step-up tokens.
type AdminStepUpHandler struct {
	authService    service.AuthService
	activityLogger *service.ActivityLogger
}

func NewAdminStepUpHandler(authService service.AuthService, activityService service.UserActivityService) *AdminStepUpHandler {
	return &AdminStepUpHandler{authService: authService, activityLogger: service.NewActivityLogger(activityService)}
}

type adminStepUpRequest struct {
	MasterPasswordHash string `json:"master_password_hash" binding:"required"`
	Code               string `json:"code" binding:"required"`
}

// Create verifies master password + second factor and returns a step-up token.
// POST /api/admin/step-up
func (h *AdminStepUpHandler) Create(c *gin.Context) {
	userID, err := GetUserID(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "step-up failed", "code": adminCodeStepUpFailed})
		return
	}
	var req adminStepUpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "master_password_hash and code are required"})
		return
	}

	token, expiresAt, err := h.authService.IssueAdminStepUp(c.Request.Context(), userID, sessionUUIDFromContext(c), req.MasterPasswordHash, req.Code)
	SetAdminAuditDetails(c, service.ActivityDetails{"result": stepUpResult(err)})
	switch {
	case err == nil:
		c.JSON(http.StatusOK, gin.H{"step_up_token": token, "expires_at": expiresAt})
	case errors.Is(err, service.ErrTwoFactorRequired):
		c.JSON(http.StatusForbidden, gin.H{"error": "enable two-factor authentication to perform admin changes", "code": adminCodeTwoFactorRequired})
	default:
		c.JSON(http.StatusForbidden, gin.H{"error": "master password or verification code is incorrect", "code": adminCodeStepUpFailed})
	}
}

func stepUpResult(err error) string {
	switch {
	case err == nil:
		return "success"
	case errors.Is(err, service.ErrTwoFactorRequired):
		return "two_factor_required"
	default:
		return "failed"
	}
}
