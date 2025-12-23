package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
)

type EmailHandler struct {
	service service.EmailService
}

func NewEmailHandler(service service.EmailService) *EmailHandler {
	return &EmailHandler{service: service}
}

func (h *EmailHandler) List(c *gin.Context) {
	ctx := c.Request.Context()

	emails, err := h.service.List(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch emails"})
		return
	}

	c.JSON(http.StatusOK, emails)
}

func (h *EmailHandler) GetByID(c *gin.Context) {
	ctx := c.Request.Context()

	id, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	email, err := h.service.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "email not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch email"})
		return
	}

	c.JSON(http.StatusOK, email)
}

func (h *EmailHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()

	var email domain.Email
	if err := c.ShouldBindJSON(&email); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := h.service.Create(ctx, &email); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create email"})
		return
	}

	c.JSON(http.StatusCreated, email)
}

func (h *EmailHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()

	id, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	var email domain.Email
	if err := c.ShouldBindJSON(&email); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := h.service.Update(ctx, id, &email); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "email not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update email"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"error": "email updated successfully"})
}

func (h *EmailHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()

	id, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	if err := h.service.Delete(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "email not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete email"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "email deleted successfully"})
}

func (h *EmailHandler) BulkUpdate(c *gin.Context) {
	ctx := c.Request.Context()

	var emails []*domain.Email
	if err := c.ShouldBindJSON(&emails); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := h.service.BulkUpdate(ctx, emails); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to bulk update"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "bulk update completed successfully"})
}
