package http

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
	"github.com/passwall/passwall-server/pkg/database"
)

type ItemShareHandler struct {
	service service.ItemShareService
}

func NewItemShareHandler(svc service.ItemShareService) *ItemShareHandler {
	return &ItemShareHandler{service: svc}
}

type createItemShareRequest struct {
	ItemUUID         string     `json:"item_uuid" binding:"required"`
	SharedWithUserID *uint      `json:"shared_with_user_id,omitempty"`
	SharedWithEmail  string     `json:"shared_with_email,omitempty"`
	CanView          *bool      `json:"can_view,omitempty"`
	CanEdit          *bool      `json:"can_edit,omitempty"`
	CanShare         *bool      `json:"can_share,omitempty"`
	EncryptedKey     string     `json:"encrypted_key,omitempty"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
}

type updateSharedItemRequest struct {
	Data     string              `json:"data" binding:"required"`
	Metadata serviceItemMetadata `json:"metadata" binding:"required"`
}

type updateSharePermissionsRequest struct {
	CanEdit        *bool      `json:"can_edit,omitempty"`
	CanShare       *bool      `json:"can_share,omitempty"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	ClearExpiresAt bool       `json:"clear_expires_at,omitempty"`
}

type serviceItemMetadata struct {
	Name     string   `json:"name"`
	URIHint  string   `json:"uri_hint,omitempty"`
	Brand    string   `json:"brand,omitempty"`
	Category string   `json:"category,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	IconHint string   `json:"icon_hint,omitempty"`
}

// Create handles POST /api/item-shares
func (h *ItemShareHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)
	schema := database.GetSchema(ctx)

	var req createItemShareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	if strings.TrimSpace(req.SharedWithEmail) == "" && req.SharedWithUserID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "shared_with_user_id or shared_with_email is required"})
		return
	}

	result, err := h.service.Create(ctx, userID, schema, &service.CreateItemShareRequest{
		ItemUUID:         req.ItemUUID,
		SharedWithUserID: req.SharedWithUserID,
		SharedWithEmail:  req.SharedWithEmail,
		CanView:          req.CanView,
		CanEdit:          req.CanEdit,
		CanShare:         req.CanShare,
		EncryptedKey:     req.EncryptedKey,
		ExpiresAt:        req.ExpiresAt,
	})
	if err != nil {
		if errors.Is(err, service.ErrShareInviteSent) {
			c.JSON(http.StatusAccepted, gin.H{
				"invitation_sent": true,
				"message":         "Recipient is not registered. Signup email sent.",
			})
			return
		}
		if errors.Is(err, repository.ErrInvalidInput) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid share request"})
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "item or user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create share"})
		return
	}

	c.JSON(http.StatusCreated, buildItemShareResponse(result, true))
}

// ListOwned handles GET /api/item-shares
func (h *ItemShareHandler) ListOwned(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	shares, err := h.service.ListOwned(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list shares"})
		return
	}

	response := make([]gin.H, 0, len(shares))
	for _, share := range shares {
		response = append(response, buildItemShareResponse(share, false))
	}

	c.JSON(http.StatusOK, gin.H{"shares": response})
}

// ListReceived handles GET /api/item-shares/received
func (h *ItemShareHandler) ListReceived(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	shares, err := h.service.ListReceived(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list received shares"})
		return
	}

	response := make([]gin.H, 0, len(shares))
	for _, share := range shares {
		response = append(response, buildItemShareResponse(share, true))
	}

	c.JSON(http.StatusOK, gin.H{"shares": response})
}

// GetByUUID handles GET /api/item-shares/:uuid
func (h *ItemShareHandler) GetByUUID(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	shareUUID, ok := GetStringParam(c, "uuid")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "share uuid is required"})
		return
	}

	result, err := h.service.GetByUUID(ctx, userID, shareUUID)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "share not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get share"})
		return
	}

	// Include encrypted key only when the requester is the recipient.
	includeEncryptedKey := result.Share.SharedWithUserID != nil && *result.Share.SharedWithUserID == userID
	c.JSON(http.StatusOK, buildItemShareResponse(result, includeEncryptedKey))
}

// UpdateSharedItem handles PUT /api/item-shares/:uuid/item
func (h *ItemShareHandler) UpdateSharedItem(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	shareUUID, ok := GetStringParam(c, "uuid")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "share uuid is required"})
		return
	}

	var req updateSharedItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	item, err := h.service.UpdateSharedItem(ctx, userID, shareUUID, &service.UpdateSharedItemRequest{
		Data: req.Data,
		Metadata: domain.ItemMetadata{
			Name:     req.Metadata.Name,
			URIHint:  req.Metadata.URIHint,
			Brand:    req.Metadata.Brand,
			Category: req.Metadata.Category,
			Tags:     req.Metadata.Tags,
			IconHint: req.Metadata.IconHint,
		},
	})
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "share not found"})
			return
		}
		if errors.Is(err, repository.ErrInvalidInput) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid shared item update"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update shared item"})
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
	})
}

// UpdatePermissions handles PATCH /api/item-shares/:uuid/permissions
func (h *ItemShareHandler) UpdatePermissions(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	shareUUID, ok := GetStringParam(c, "uuid")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "share uuid is required"})
		return
	}

	var req updateSharePermissionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	result, err := h.service.UpdatePermissions(ctx, userID, shareUUID, &service.UpdateItemSharePermissionsRequest{
		CanEdit:        req.CanEdit,
		CanShare:       req.CanShare,
		ExpiresAt:      req.ExpiresAt,
		ClearExpiresAt: req.ClearExpiresAt,
	})
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "share not found"})
			return
		}
		if errors.Is(err, repository.ErrInvalidInput) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid permissions update"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update share permissions"})
		return
	}

	c.JSON(http.StatusOK, buildItemShareResponse(result, false))
}

// ReShare handles POST /api/item-shares/:uuid/re-share
func (h *ItemShareHandler) ReShare(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	shareUUID, ok := GetStringParam(c, "uuid")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "share uuid is required"})
		return
	}

	var req createItemShareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	result, err := h.service.ReShare(ctx, userID, shareUUID, &service.CreateItemShareRequest{
		ItemUUID:         req.ItemUUID,
		SharedWithUserID: req.SharedWithUserID,
		SharedWithEmail:  req.SharedWithEmail,
		CanView:          req.CanView,
		CanEdit:          req.CanEdit,
		CanShare:         req.CanShare,
		EncryptedKey:     req.EncryptedKey,
		ExpiresAt:        req.ExpiresAt,
	})
	if err != nil {
		if errors.Is(err, service.ErrShareInviteSent) {
			c.JSON(http.StatusAccepted, gin.H{
				"invitation_sent": true,
				"message":         "Recipient is not registered. Signup email sent.",
			})
			return
		}
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "share not found"})
			return
		}
		if errors.Is(err, repository.ErrInvalidInput) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid share request"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to re-share item"})
		return
	}

	c.JSON(http.StatusCreated, buildItemShareResponse(result, false))
}

// Revoke handles DELETE /api/item-shares/:id
func (h *ItemShareHandler) Revoke(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	shareID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	if err := h.service.Revoke(ctx, userID, shareID); err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "share not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke share"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "share revoked"})
}

func buildItemShareResponse(result *service.ItemShareWithItem, includeEncryptedKey bool) gin.H {
	share := result.Share
	item := result.Item

	resp := gin.H{
		"id":                 share.ID,
		"uuid":               share.UUID,
		"item_uuid":          share.ItemUUID,
		"owner_id":           share.OwnerID,
		"shared_with_user_id": share.SharedWithUserID,
		"can_view":           share.CanView,
		"can_edit":           share.CanEdit,
		"can_share":          share.CanShare,
		"expires_at":         share.ExpiresAt,
		"created_at":         share.CreatedAt,
		"item": gin.H{
			"id":                   item.ID,
			"uuid":                 item.UUID,
			"support_id":           item.SupportID,
			"support_id_formatted": item.FormatSupportID(),
			"item_type":            item.ItemType,
			"data":                 item.Data,
			"item_key_enc":         item.ItemKeyEnc,
			"metadata":             item.Metadata,
		},
	}

	if share.Owner != nil {
		resp["owner_email"] = share.Owner.Email
	}
	if share.SharedWithUser != nil {
		resp["shared_with_email"] = share.SharedWithUser.Email
		resp["shared_with_name"] = share.SharedWithUser.Name
	}
	if includeEncryptedKey {
		resp["encrypted_key"] = share.EncryptedKey
	}

	return resp
}
