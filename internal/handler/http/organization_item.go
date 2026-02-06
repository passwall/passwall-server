package http

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
)

type OrganizationItemHandler struct {
	service        service.OrganizationItemService
	activityLogger *service.ActivityLogger
}

func NewOrganizationItemHandler(
	svc service.OrganizationItemService,
	activityService service.UserActivityService,
) *OrganizationItemHandler {
	return &OrganizationItemHandler{
		service:        svc,
		activityLogger: service.NewActivityLogger(activityService),
	}
}

// Create godoc
// @Summary Create organization item
// @Description Create a new item in organization (encrypted with org key)
// @Tags organization-items
// @Accept json
// @Produce json
// @Param id path int true "Organization ID"
// @Param request body service.CreateOrgItemRequest true "Item details"
// @Success 201 {object} domain.OrganizationItemDTO
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /organizations/{id}/items [post]
func (h *OrganizationItemHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	var req service.CreateOrgItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	item, err := h.service.Create(ctx, orgID, userID, &req)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to create item", "details": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, domain.ToOrganizationItemDTO(item))

	// Log activity (no secrets)
	if h.activityLogger != nil {
		ipAddress := c.ClientIP()
		userAgent := c.GetHeader("User-Agent")
		details := service.ActivityDetails{
			service.ActivityFieldOrganizationID: orgID,
			service.ActivityFieldItemID:         item.ID,
			service.ActivityFieldItemType:       strconv.FormatInt(int64(item.ItemType), 10),
		}
		if item.CollectionID != nil {
			details[service.ActivityFieldCollectionID] = *item.CollectionID
		}
		h.activityLogger.LogActivity(ctx, userID, domain.ActivityTypeItemCreated, ipAddress, userAgent, details)
	}
}

// ListByOrganization godoc
// @Summary List items in organization
// @Description Get all items in an organization (optionally filtered)
// @Tags organization-items
// @Produce json
// @Param id path int true "Organization ID"
// @Param type query int false "Item type"
// @Param collection_id query int false "Collection ID"
// @Param folder_id query int false "Folder ID"
// @Param search query string false "Search term"
// @Success 200 {array} domain.OrganizationItemDTO
// @Failure 403 {object} map[string]string
// @Router /organizations/{id}/items [get]
func (h *OrganizationItemHandler) ListByOrganization(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	filter := repository.OrganizationItemFilter{
		OrganizationID: orgID,
	}

	if typeStr := c.Query("type"); typeStr != "" {
		if typeVal, err := strconv.ParseInt(typeStr, 10, 32); err == nil {
			itemType := domain.ItemType(typeVal)
			filter.ItemType = &itemType
		}
	}

	if collectionStr := c.Query("collection_id"); collectionStr != "" {
		if collectionVal, err := strconv.ParseUint(collectionStr, 10, 32); err == nil {
			cid := uint(collectionVal)
			filter.CollectionID = &cid
		}
	}

	if folderStr := c.Query("folder_id"); folderStr != "" {
		if folderVal, err := strconv.ParseUint(folderStr, 10, 32); err == nil {
			fid := uint(folderVal)
			filter.FolderID = &fid
		}
	}

	if search := strings.TrimSpace(c.Query("search")); search != "" {
		filter.Search = search
	}

	if pageStr := c.Query("page"); pageStr != "" {
		if pageVal, err := strconv.Atoi(pageStr); err == nil {
			filter.Page = pageVal
		}
	}

	if perPageStr := c.Query("per_page"); perPageStr != "" {
		if perPageVal, err := strconv.Atoi(perPageStr); err == nil {
			filter.PerPage = perPageVal
		}
	}

	items, _, err := h.service.ListByOrganization(ctx, orgID, userID, filter)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list items"})
		return
	}

	dtos := make([]*domain.OrganizationItemDTO, len(items))
	for i, item := range items {
		dtos[i] = domain.ToOrganizationItemDTO(item)
	}

	c.JSON(http.StatusOK, dtos)
}

// ListByCollection godoc
// @Summary List items in collection
// @Description Get all items in a collection
// @Tags organization-items
// @Produce json
// @Param id path int true "Collection ID"
// @Success 200 {array} domain.OrganizationItemDTO
// @Failure 403 {object} map[string]string
// @Router /collections/{id}/items [get]
func (h *OrganizationItemHandler) ListByCollection(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	collectionID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	items, err := h.service.ListByCollection(ctx, collectionID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list items"})
		return
	}

	dtos := make([]*domain.OrganizationItemDTO, len(items))
	for i, item := range items {
		dtos[i] = domain.ToOrganizationItemDTO(item)
	}

	c.JSON(http.StatusOK, dtos)
}

// GetByID godoc
// @Summary Get organization item
// @Description Get item by ID
// @Tags organization-items
// @Produce json
// @Param id path int true "Item ID"
// @Success 200 {object} domain.OrganizationItemDTO
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /org-items/{id} [get]
func (h *OrganizationItemHandler) GetByID(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	id, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	item, err := h.service.GetByID(ctx, id, userID)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "item not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get item"})
		return
	}

	c.JSON(http.StatusOK, domain.ToOrganizationItemDTO(item))
}

// Update godoc
// @Summary Update organization item
// @Description Update item details
// @Tags organization-items
// @Accept json
// @Produce json
// @Param id path int true "Item ID"
// @Param request body service.UpdateOrgItemRequest true "Item details"
// @Success 200 {object} domain.OrganizationItemDTO
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /org-items/{id} [put]
func (h *OrganizationItemHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	id, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	var req service.UpdateOrgItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	item, err := h.service.Update(ctx, id, userID, &req)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to update item", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, domain.ToOrganizationItemDTO(item))

	// Log activity (no secrets)
	if h.activityLogger != nil {
		ipAddress := c.ClientIP()
		userAgent := c.GetHeader("User-Agent")
		details := service.ActivityDetails{
			service.ActivityFieldOrganizationID: item.OrganizationID,
			service.ActivityFieldItemID:         item.ID,
			service.ActivityFieldItemType:       strconv.FormatInt(int64(item.ItemType), 10),
		}
		if item.CollectionID != nil {
			details[service.ActivityFieldCollectionID] = *item.CollectionID
		}
		h.activityLogger.LogActivity(ctx, userID, domain.ActivityTypeItemUpdated, ipAddress, userAgent, details)
	}
}

// Delete godoc
// @Summary Delete organization item
// @Description Delete item
// @Tags organization-items
// @Param id path int true "Item ID"
// @Success 204
// @Failure 403 {object} map[string]string
// @Router /org-items/{id} [delete]
func (h *OrganizationItemHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	id, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	item, err := h.service.Delete(ctx, id, userID)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete item"})
		return
	}

	c.Status(http.StatusNoContent)

	// Log activity (no secrets)
	if h.activityLogger != nil && item != nil {
		ipAddress := c.ClientIP()
		userAgent := c.GetHeader("User-Agent")
		details := service.ActivityDetails{
			service.ActivityFieldOrganizationID: item.OrganizationID,
			service.ActivityFieldItemID:         item.ID,
			service.ActivityFieldItemType:       strconv.FormatInt(int64(item.ItemType), 10),
		}
		if item.CollectionID != nil {
			details[service.ActivityFieldCollectionID] = *item.CollectionID
		}
		h.activityLogger.LogActivity(ctx, userID, domain.ActivityTypeItemDeleted, ipAddress, userAgent, details)
	}
}
