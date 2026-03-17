package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
)

// BreachMonitorHandler handles breach monitoring HTTP endpoints.
type BreachMonitorHandler struct {
	service service.BreachMonitorService
}

// NewBreachMonitorHandler creates a new breach monitor handler.
func NewBreachMonitorHandler(svc service.BreachMonitorService) *BreachMonitorHandler {
	return &BreachMonitorHandler{service: svc}
}

type addEmailRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// AddEmail handles POST /api/organizations/:id/breach-monitor/emails
func (h *BreachMonitorHandler) AddEmail(c *gin.Context) {
	orgID, ok := GetResolvedOrgID(c)
	if !ok {
		return
	}
	userID := GetCurrentUserID(c)

	var req addEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid email address is required"})
		return
	}

	dto, err := h.service.AddEmail(c.Request.Context(), orgID, userID, req.Email)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, dto)
}

// ListEmails handles GET /api/organizations/:id/breach-monitor/emails
func (h *BreachMonitorHandler) ListEmails(c *gin.Context) {
	orgID, ok := GetResolvedOrgID(c)
	if !ok {
		return
	}
	userID := GetCurrentUserID(c)

	emails, err := h.service.ListEmails(c.Request.Context(), orgID, userID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, emails)
}

// RemoveEmail handles DELETE /api/organizations/:id/breach-monitor/emails/:emailId
func (h *BreachMonitorHandler) RemoveEmail(c *gin.Context) {
	orgID, ok := GetResolvedOrgID(c)
	if !ok {
		return
	}
	userID := GetCurrentUserID(c)
	emailID, ok := GetUintParam(c, "emailId")
	if !ok {
		return
	}

	if err := h.service.RemoveEmail(c.Request.Context(), orgID, userID, emailID); err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "email removed from monitoring"})
}

// CheckEmails handles POST /api/organizations/:id/breach-monitor/check
func (h *BreachMonitorHandler) CheckEmails(c *gin.Context) {
	orgID, ok := GetResolvedOrgID(c)
	if !ok {
		return
	}
	userID := GetCurrentUserID(c)

	if err := h.service.CheckEmails(c.Request.Context(), orgID, userID); err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "breach check completed"})
}

// ListBreaches handles GET /api/organizations/:id/breach-monitor/breaches
func (h *BreachMonitorHandler) ListBreaches(c *gin.Context) {
	orgID, ok := GetResolvedOrgID(c)
	if !ok {
		return
	}
	userID := GetCurrentUserID(c)

	breaches, err := h.service.ListBreaches(c.Request.Context(), orgID, userID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, breaches)
}

// DismissBreach handles PATCH /api/organizations/:id/breach-monitor/breaches/:breachId/dismiss
func (h *BreachMonitorHandler) DismissBreach(c *gin.Context) {
	orgID, ok := GetResolvedOrgID(c)
	if !ok {
		return
	}
	userID := GetCurrentUserID(c)
	breachID, ok := GetUintParam(c, "breachId")
	if !ok {
		return
	}

	if err := h.service.DismissBreach(c.Request.Context(), orgID, userID, breachID); err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "breach dismissed"})
}

// GetSummary handles GET /api/organizations/:id/breach-monitor/summary
func (h *BreachMonitorHandler) GetSummary(c *gin.Context) {
	orgID, ok := GetResolvedOrgID(c)
	if !ok {
		return
	}
	userID := GetCurrentUserID(c)

	summary, err := h.service.GetSummary(c.Request.Context(), orgID, userID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, summary)
}

func (h *BreachMonitorHandler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	case errors.Is(err, repository.ErrAlreadyExists):
		c.JSON(http.StatusConflict, gin.H{"error": "email is already being monitored"})
	case errors.Is(err, repository.ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
	case errors.Is(err, service.ErrFeatureNotAvailable):
		c.JSON(http.StatusForbidden, gin.H{"error": "breach monitoring is not available on your current plan"})
	case errors.Is(err, service.ErrSubscriptionExpired):
		c.JSON(http.StatusForbidden, gin.H{"error": "subscription expired"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
