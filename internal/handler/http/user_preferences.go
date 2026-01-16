package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
)

type UserPreferencesHandler struct {
	service service.PreferencesService
}

func NewUserPreferencesHandler(service service.PreferencesService) *UserPreferencesHandler {
	return &UserPreferencesHandler{service: service}
}

// List returns preferences for the authenticated user.
// GET /api/users/me/preferences?section=localization
func (h *UserPreferencesHandler) List(c *gin.Context) {
	userID, err := GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	section := c.Query("section")
	prefs, err := h.service.ListForUser(c.Request.Context(), userID, section)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get preferences"})
		return
	}

	out := make([]*domain.PreferenceDTO, 0, len(prefs))
	for _, p := range prefs {
		out = append(out, domain.ToPreferenceDTO(p))
	}

	c.JSON(http.StatusOK, gin.H{"preferences": out})
}

// Upsert updates one or more preferences for the authenticated user.
// PUT /api/users/me/preferences
func (h *UserPreferencesHandler) Upsert(c *gin.Context) {
	userID, err := GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req domain.UpsertPreferencesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	prefs, err := h.service.UpsertForUser(c.Request.Context(), userID, &req)
	if err != nil {
		if err == repository.ErrInvalidInput {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update preferences"})
		return
	}

	out := make([]*domain.PreferenceDTO, 0, len(prefs))
	for _, p := range prefs {
		out = append(out, domain.ToPreferenceDTO(p))
	}

	c.JSON(http.StatusOK, gin.H{"preferences": out})
}
