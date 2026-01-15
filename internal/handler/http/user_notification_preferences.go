package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/service"
)

type UserNotificationPreferencesHandler struct {
	service service.UserNotificationPreferencesService
}

func NewUserNotificationPreferencesHandler(service service.UserNotificationPreferencesService) *UserNotificationPreferencesHandler {
	return &UserNotificationPreferencesHandler{service: service}
}

// Get returns notification preferences for the authenticated user
// GET /api/users/me/notification-preferences
func (h *UserNotificationPreferencesHandler) Get(c *gin.Context) {
	userID, err := GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	prefs, err := h.service.GetForUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get notification preferences"})
		return
	}

	c.JSON(http.StatusOK, domain.ToUserNotificationPreferencesDTO(prefs))
}

// Update updates notification preferences for the authenticated user
// PUT /api/users/me/notification-preferences
func (h *UserNotificationPreferencesHandler) Update(c *gin.Context) {
	userID, err := GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req domain.UpdateUserNotificationPreferencesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	prefs, err := h.service.UpdateForUser(c.Request.Context(), userID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update notification preferences"})
		return
	}

	c.JSON(http.StatusOK, domain.ToUserNotificationPreferencesDTO(prefs))
}

