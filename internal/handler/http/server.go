package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
)

type ServerHandler struct {
	service service.ServerService
}

func NewServerHandler(service service.ServerService) *ServerHandler {
	return &ServerHandler{service: service}
}

func (h *ServerHandler) List(c *gin.Context) {
	ctx := c.Request.Context()

	servers, err := h.service.List(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch servers"})
		return
	}

	c.JSON(http.StatusOK, servers)
}

func (h *ServerHandler) GetByID(c *gin.Context) {
	ctx := c.Request.Context()

	id, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	server, err := h.service.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch server"})
		return
	}

	c.JSON(http.StatusOK, server)
}

func (h *ServerHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()

	var server domain.Server
	if err := c.ShouldBindJSON(&server); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := h.service.Create(ctx, &server); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create server"})
		return
	}

	c.JSON(http.StatusCreated, server)
}

func (h *ServerHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()

	id, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	var server domain.Server
	if err := c.ShouldBindJSON(&server); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := h.service.Update(ctx, id, &server); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update server"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "server updated successfully"})
}

func (h *ServerHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()

	id, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	if err := h.service.Delete(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete server"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "server deleted successfully"})
}

func (h *ServerHandler) BulkUpdate(c *gin.Context) {
	ctx := c.Request.Context()

	var servers []*domain.Server
	if err := c.ShouldBindJSON(&servers); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := h.service.BulkUpdate(ctx, servers); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to bulk update"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "bulk update completed successfully"})
}
