package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
)

type UserAppearancePreferencesHandler struct {
	service service.UserAppearancePreferencesService
}

func NewUserAppearancePreferencesHandler(service service.UserAppearancePreferencesService) *UserAppearancePreferencesHandler {
	return &UserAppearancePreferencesHandler{service: service}
}

// Get returns appearance preferences for the authenticated user
// GET /api/users/me/appearance-preferences
func (h *UserAppearancePreferencesHandler) Get(c *gin.Context) {
	userID, err := GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	prefs, err := h.service.GetForUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get appearance preferences"})
		return
	}

	c.JSON(http.StatusOK, domain.ToUserAppearancePreferencesDTO(prefs))
}

// Update updates appearance preferences for the authenticated user
// PUT /api/users/me/appearance-preferences
func (h *UserAppearancePreferencesHandler) Update(c *gin.Context) {
	userID, err := GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req domain.UpdateUserAppearancePreferencesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	prefs, err := h.service.UpdateForUser(c.Request.Context(), userID, &req)
	if err != nil {
		if err == repository.ErrInvalidInput {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update appearance preferences"})
		return
	}

	c.JSON(http.StatusOK, domain.ToUserAppearancePreferencesDTO(prefs))
}

