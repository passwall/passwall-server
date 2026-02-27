package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
)

type OrganizationSettingsHandler struct {
	service service.OrganizationSettingsService
}

func NewOrganizationSettingsHandler(service service.OrganizationSettingsService) *OrganizationSettingsHandler {
	return &OrganizationSettingsHandler{service: service}
}

// ListSettings godoc
// @Summary List organization settings
// @Description List settings for an organization, optionally filtered by section
// @Tags organization-settings
// @Produce json
// @Param id path int true "Organization ID"
// @Param section query string false "Filter by section"
// @Success 200 {object} map[string]interface{}
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /organizations/{id}/settings [get]
func (h *OrganizationSettingsHandler) ListSettings(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	section := c.Query("section")

	settings, err := h.service.ListByOrganization(ctx, orgID, userID, section)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list settings", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"settings": settings})
}

// UpsertSettings godoc
// @Summary Update organization settings
// @Description Create or update one or more organization settings
// @Tags organization-settings
// @Accept json
// @Produce json
// @Param id path int true "Organization ID"
// @Param request body domain.UpsertPreferencesRequest true "Settings to update"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /organizations/{id}/settings [put]
func (h *OrganizationSettingsHandler) UpsertSettings(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	var req domain.UpsertPreferencesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	settings, err := h.service.UpsertForOrganization(ctx, orgID, userID, &req)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		if errors.Is(err, repository.ErrInvalidInput) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid settings data"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"settings": settings})
}

// ListSettingsDefinitions godoc
// @Summary List all available organization settings definitions
// @Description Returns the catalog of all organization setting keys with metadata
// @Tags organization-settings
// @Produce json
// @Success 200 {array} domain.OrgSettingsDefinition
// @Router /settings/definitions [get]
func (h *OrganizationSettingsHandler) ListSettingsDefinitions(c *gin.Context) {
	c.JSON(http.StatusOK, h.service.GetSettingsDefinitions())
}
