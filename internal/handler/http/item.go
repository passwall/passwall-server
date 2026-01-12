package http

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
	"github.com/passwall/passwall-server/pkg/database"
)

type ItemHandler struct {
	itemService service.ItemService
}

// NewItemHandler creates a new item handler
func NewItemHandler(itemService service.ItemService) *ItemHandler {
	return &ItemHandler{
		itemService: itemService,
	}
}

// Create handles POST /api/items
func (h *ItemHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()
	schema := database.GetSchema(ctx)

	var req service.CreateItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	item, err := h.itemService.Create(ctx, schema, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create item", "details": err.Error()})
		return
	}

	// Add formatted support ID
	response := gin.H{
		"id":                   item.ID,
		"uuid":                 item.UUID,
		"support_id":           item.SupportID,
		"support_id_formatted": item.FormatSupportID(),
		"item_type":            item.ItemType,
		"data":                 item.Data,
		"metadata":             item.Metadata,
		"is_favorite":          item.IsFavorite,
		"folder_id":            item.FolderID,
		"reprompt":             item.Reprompt,
		"auto_fill":            item.AutoFill,
		"auto_login":           item.AutoLogin,
		"revision":             item.Revision,
		"sync_version":         item.SyncVersion,
		"created_at":           item.CreatedAt,
		"updated_at":           item.UpdatedAt,
	}

	c.JSON(http.StatusCreated, response)
}

// List handles GET /api/items
func (h *ItemHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	schema := database.GetSchema(ctx)

	filter := parseItemFilter(c)

	response, err := h.itemService.List(ctx, schema, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list items"})
		return
	}

	// Add formatted support IDs to response
	items := make([]map[string]interface{}, len(response.Items))
	for i, item := range response.Items {
		items[i] = map[string]interface{}{
			"id":                   item.ID,
			"uuid":                 item.UUID,
			"support_id":           item.SupportID,
			"support_id_formatted": item.FormatSupportID(),
			"item_type":            item.ItemType,
			"data":                 item.Data,
			"metadata":             item.Metadata,
			"is_favorite":          item.IsFavorite,
			"folder_id":            item.FolderID,
			"reprompt":             item.Reprompt,
			"auto_fill":            item.AutoFill,
			"auto_login":           item.AutoLogin,
			"revision":             item.Revision,
			"created_at":           item.CreatedAt,
			"updated_at":           item.UpdatedAt,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"items":    items,
		"total":    response.Total,
		"page":     response.Page,
		"per_page": response.PerPage,
	})
}

// GetByID handles GET /api/items/:id
func (h *ItemHandler) GetByID(c *gin.Context) {
	ctx := c.Request.Context()
	schema := database.GetSchema(ctx)

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item ID"})
		return
	}

	item, err := h.itemService.GetByID(ctx, schema, uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "item not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":                   item.ID,
		"uuid":                 item.UUID,
		"support_id":           item.SupportID,
		"support_id_formatted": item.FormatSupportID(),
		"item_type":            item.ItemType,
		"data":                 item.Data,
		"metadata":             item.Metadata,
		"is_favorite":          item.IsFavorite,
		"folder_id":            item.FolderID,
		"reprompt":             item.Reprompt,
		"auto_fill":            item.AutoFill,
		"auto_login":           item.AutoLogin,
		"revision":             item.Revision,
		"created_at":           item.CreatedAt,
		"updated_at":           item.UpdatedAt,
	})
}

// Update handles PUT /api/items/:id
func (h *ItemHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()
	schema := database.GetSchema(ctx)

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item ID"})
		return
	}

	var req service.UpdateItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	item, err := h.itemService.Update(ctx, schema, uint(id), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update item"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":                   item.ID,
		"uuid":                 item.UUID,
		"support_id":           item.SupportID,
		"support_id_formatted": item.FormatSupportID(),
		"item_type":            item.ItemType,
		"data":                 item.Data,
		"metadata":             item.Metadata,
		"is_favorite":          item.IsFavorite,
		"reprompt":             item.Reprompt,
		"auto_fill":            item.AutoFill,
		"auto_login":           item.AutoLogin,
		"revision":             item.Revision,
		"updated_at":           item.UpdatedAt,
	})
}

// Delete handles DELETE /api/items/:id
func (h *ItemHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()
	schema := database.GetSchema(ctx)

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item ID"})
		return
	}

	if err := h.itemService.Delete(ctx, schema, uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete item"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "item deleted successfully"})
}

// parseItemFilter parses query parameters into ItemFilter
func parseItemFilter(c *gin.Context) repository.ItemFilter {
	filter := repository.ItemFilter{}

	// Item type
	if typeStr := c.Query("type"); typeStr != "" {
		if typeVal, err := strconv.Atoi(typeStr); err == nil {
			itemType := domain.ItemType(typeVal)
			filter.ItemType = &itemType
		}
	}

	// Favorite
	if favStr := c.Query("is_favorite"); favStr != "" {
		if favVal, err := strconv.ParseBool(favStr); err == nil {
			filter.IsFavorite = &favVal
		}
	}

	// Folder
	if folderStr := c.Query("folder_id"); folderStr != "" {
		if folderVal, err := strconv.ParseUint(folderStr, 10, 32); err == nil {
			fid := uint(folderVal)
			filter.FolderID = &fid
		}
	}

	// Auto-fill
	if autoFillStr := c.Query("auto_fill"); autoFillStr != "" {
		if autoFillVal, err := strconv.ParseBool(autoFillStr); err == nil {
			filter.AutoFill = &autoFillVal
		}
	}

	// Auto-login
	if autoLoginStr := c.Query("auto_login"); autoLoginStr != "" {
		if autoLoginVal, err := strconv.ParseBool(autoLoginStr); err == nil {
			filter.AutoLogin = &autoLoginVal
		}
	}

	// Tags
	if tagsStr := c.Query("tags"); tagsStr != "" {
		filter.Tags = strings.Split(tagsStr, ",")
	}

	// Search
	filter.Search = c.Query("search")

	// URI hint (domain only)
	// Supports both repeated params (?uri_hint=a&uri_hint=b) and comma-separated (?uri_hint=a,b)
	rawHints := c.QueryArray("uri_hint")
	// Fallback: some clients / frameworks send a single value where QueryArray may return empty.
	if len(rawHints) == 0 {
		if raw := c.Query("uri_hint"); raw != "" {
			rawHints = []string{raw}
		}
	}
	if len(rawHints) > 0 {
		var hints []string
		for _, raw := range rawHints {
			for _, part := range strings.Split(raw, ",") {
				hint := strings.TrimSpace(part)
				if hint == "" {
					continue
				}
				hints = append(hints, hint)
			}
		}
		filter.URIHints = hints
	}

	// Pagination
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

	return filter
}
