package http

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
)

type SendHandler struct {
	service service.SendService
}

func NewSendHandler(svc service.SendService) *SendHandler {
	return &SendHandler{service: svc}
}

type createSendRequest struct {
	Name           string     `json:"name" binding:"required"`
	OrganizationID uint       `json:"organization_id" binding:"required"`
	Type           string     `json:"type" binding:"required"`
	Data           string     `json:"data" binding:"required"`
	Notes          *string    `json:"notes,omitempty"`
	Password       *string    `json:"password,omitempty"`
	MaxAccessCount *int       `json:"max_access_count,omitempty"`
	ExpirationDate *time.Time `json:"expiration_date,omitempty"`
	DeletionDate   *time.Time `json:"deletion_date,omitempty"`
	HideEmail      bool       `json:"hide_email"`
}

type updateSendRequest struct {
	Name           *string    `json:"name,omitempty"`
	Data           *string    `json:"data,omitempty"`
	Notes          *string    `json:"notes,omitempty"`
	Password       *string    `json:"password,omitempty"`
	MaxAccessCount *int       `json:"max_access_count,omitempty"`
	ExpirationDate *time.Time `json:"expiration_date,omitempty"`
	DeletionDate   *time.Time `json:"deletion_date,omitempty"`
	Disabled       *bool      `json:"disabled,omitempty"`
	HideEmail      *bool      `json:"hide_email,omitempty"`
}

// Create handles POST /api/sends
func (h *SendHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	var req createSendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	send, err := h.service.Create(ctx, userID, &domain.CreateSendRequest{
		Name:           req.Name,
		OrganizationID: req.OrganizationID,
		Type:           domain.SendType(req.Type),
		Data:           req.Data,
		Notes:          req.Notes,
		Password:       req.Password,
		MaxAccessCount: req.MaxAccessCount,
		ExpirationDate: req.ExpirationDate,
		DeletionDate:   req.DeletionDate,
		HideEmail:      req.HideEmail,
	})
	if err != nil {
		if errors.Is(err, repository.ErrInvalidInput) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid send data"})
			return
		}
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "send creation is disabled by organization policy"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create send"})
		return
	}

	c.JSON(http.StatusCreated, domain.ToSendDTO(send))
}

// List handles GET /api/sends
func (h *SendHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	sends, err := h.service.List(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list sends"})
		return
	}

	dtos := make([]*domain.SendDTO, 0, len(sends))
	for _, s := range sends {
		dtos = append(dtos, domain.ToSendDTO(s))
	}

	c.JSON(http.StatusOK, gin.H{"sends": dtos})
}

// GetByUUID handles GET /api/sends/:uuid
func (h *SendHandler) GetByUUID(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	sendUUID, ok := GetStringParam(c, "uuid")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uuid is required"})
		return
	}

	send, err := h.service.GetByUUID(ctx, userID, sendUUID)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "send not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get send"})
		return
	}

	c.JSON(http.StatusOK, domain.ToSendDTO(send))
}

// Update handles PUT /api/sends/:uuid
func (h *SendHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	sendUUID, ok := GetStringParam(c, "uuid")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uuid is required"})
		return
	}

	var req updateSendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	send, err := h.service.Update(ctx, userID, sendUUID, &domain.UpdateSendRequest{
		Name:           req.Name,
		Data:           req.Data,
		Notes:          req.Notes,
		Password:       req.Password,
		MaxAccessCount: req.MaxAccessCount,
		ExpirationDate: req.ExpirationDate,
		DeletionDate:   req.DeletionDate,
		Disabled:       req.Disabled,
		HideEmail:      req.HideEmail,
	})
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "send not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update send"})
		return
	}

	c.JSON(http.StatusOK, domain.ToSendDTO(send))
}

// Delete handles DELETE /api/sends/:uuid
func (h *SendHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	sendUUID, ok := GetStringParam(c, "uuid")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uuid is required"})
		return
	}

	if err := h.service.Delete(ctx, userID, sendUUID); err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "send not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete send"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "send deleted"})
}

// Access handles GET /api/sends/access/:access_id (public — no auth required)
func (h *SendHandler) Access(c *gin.Context) {
	ctx := c.Request.Context()

	accessID, ok := GetStringParam(c, "access_id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "access_id is required"})
		return
	}

	dto, err := h.service.GetByAccessID(ctx, accessID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "send not found or expired"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to access send"})
		return
	}

	c.JSON(http.StatusOK, dto)
}

// VerifyPassword handles POST /api/sends/access/:access_id/password
func (h *SendHandler) VerifyPassword(c *gin.Context) {
	ctx := c.Request.Context()

	accessID, ok := GetStringParam(c, "access_id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "access_id is required"})
		return
	}

	var req struct {
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password is required"})
		return
	}

	dto, err := h.service.VerifySendPassword(ctx, accessID, req.Password)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "incorrect password"})
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "send not found or expired"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify password"})
		return
	}

	c.JSON(http.StatusOK, dto)
}
