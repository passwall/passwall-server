package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
)

type LoginHandler struct {
	service service.LoginService
}

// NewLoginHandler creates a new login handler
func NewLoginHandler(service service.LoginService) *LoginHandler {
	return &LoginHandler{
		service: service,
	}
}

// List handles GET /api/logins
func (h *LoginHandler) List(c *gin.Context) {
	ctx := c.Request.Context()

	logins, err := h.service.List(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch logins"})
		return
	}

	c.JSON(http.StatusOK, logins)
}

// GetByID handles GET /api/logins/:id
func (h *LoginHandler) GetByID(c *gin.Context) {
	ctx := c.Request.Context()

	id, ok := GetUintParam(c, "id")
	if !ok {
		return // Error already sent by helper
	}

	login, err := h.service.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "login not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch login"})
		return
	}

	c.JSON(http.StatusOK, login)
}

// Create handles POST /api/logins
func (h *LoginHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()

	var login domain.Login
	if err := c.ShouldBindJSON(&login); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	if err := h.service.Create(ctx, &login); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create login"})
		return
	}

	c.JSON(http.StatusCreated, login)
}

// Update handles PUT /api/logins/:id
func (h *LoginHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()

	id, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	var login domain.Login
	if err := c.ShouldBindJSON(&login); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	if err := h.service.Update(ctx, id, &login); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "login not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update login"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "login updated successfully"})
}

// Delete handles DELETE /api/logins/:id
func (h *LoginHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()

	id, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	if err := h.service.Delete(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "login not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete login"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "login deleted successfully"})
}

// BulkUpdate handles PUT /api/logins/bulk-update
func (h *LoginHandler) BulkUpdate(c *gin.Context) {
	ctx := c.Request.Context()

	var logins []*domain.Login
	if err := c.ShouldBindJSON(&logins); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	if err := h.service.BulkUpdate(ctx, logins); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to bulk update logins"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "bulk update completed successfully"})
}
