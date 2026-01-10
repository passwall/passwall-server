package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
)

type OrganizationItemHandler struct {
	service service.OrganizationItemService
}

func NewOrganizationItemHandler(service service.OrganizationItemService) *OrganizationItemHandler {
	return &OrganizationItemHandler{service: service}
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

	err := h.service.Delete(ctx, id, userID)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete item"})
		return
	}

	c.Status(http.StatusNoContent)
}

