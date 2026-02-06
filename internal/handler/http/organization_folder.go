package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
)

type OrganizationFolderHandler struct {
	service service.OrganizationFolderService
}

func NewOrganizationFolderHandler(svc service.OrganizationFolderService) *OrganizationFolderHandler {
	return &OrganizationFolderHandler{service: svc}
}

// ListByOrganization godoc
// @Summary List organization folders
// @Description Get all folders for an organization
// @Tags organization-folders
// @Produce json
// @Param id path int true "Organization ID"
// @Success 200 {array} domain.OrganizationFolderDTO
// @Failure 403 {object} map[string]string
// @Router /organizations/{id}/folders [get]
func (h *OrganizationFolderHandler) ListByOrganization(c *gin.Context) {
	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}
	userID := GetCurrentUserID(c)

	folders, err := h.service.ListByOrganization(c.Request.Context(), orgID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list folders"})
		return
	}

	c.JSON(http.StatusOK, domain.ToOrganizationFolderDTOs(folders))
}

// Create godoc
// @Summary Create organization folder
// @Description Create a new folder in organization
// @Tags organization-folders
// @Accept json
// @Produce json
// @Param id path int true "Organization ID"
// @Param request body domain.CreateOrganizationFolderRequest true "Folder details"
// @Success 201 {object} domain.OrganizationFolderDTO
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /organizations/{id}/folders [post]
func (h *OrganizationFolderHandler) Create(c *gin.Context) {
	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}
	userID := GetCurrentUserID(c)

	var req domain.CreateOrganizationFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	folder, err := h.service.Create(c.Request.Context(), orgID, userID, &req)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to create folder", "details": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, domain.ToOrganizationFolderDTO(folder))
}

// Update godoc
// @Summary Update organization folder
// @Description Update folder name
// @Tags organization-folders
// @Accept json
// @Produce json
// @Param id path int true "Organization ID"
// @Param folderId path int true "Folder ID"
// @Param request body domain.UpdateOrganizationFolderRequest true "Folder details"
// @Success 200 {object} domain.OrganizationFolderDTO
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /organizations/{id}/folders/{folderId} [put]
func (h *OrganizationFolderHandler) Update(c *gin.Context) {
	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}
	folderID, ok := GetUintParam(c, "folderId")
	if !ok {
		return
	}
	userID := GetCurrentUserID(c)

	var req domain.UpdateOrganizationFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	folder, err := h.service.Update(c.Request.Context(), orgID, userID, folderID, &req)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "folder not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to update folder", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, domain.ToOrganizationFolderDTO(folder))
}

// Delete godoc
// @Summary Delete organization folder
// @Description Delete folder
// @Tags organization-folders
// @Param id path int true "Organization ID"
// @Param folderId path int true "Folder ID"
// @Success 204
// @Failure 403 {object} map[string]string
// @Router /organizations/{id}/folders/{folderId} [delete]
func (h *OrganizationFolderHandler) Delete(c *gin.Context) {
	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}
	folderID, ok := GetUintParam(c, "folderId")
	if !ok {
		return
	}
	userID := GetCurrentUserID(c)

	if err := h.service.Delete(c.Request.Context(), orgID, userID, folderID); err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "folder not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to delete folder", "details": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}
