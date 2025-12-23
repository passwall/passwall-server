package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
)

type NoteHandler struct {
	service service.NoteService
}

func NewNoteHandler(service service.NoteService) *NoteHandler {
	return &NoteHandler{service: service}
}

func (h *NoteHandler) List(c *gin.Context) {
	ctx := c.Request.Context()

	notes, err := h.service.List(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch notes"})
		return
	}

	c.JSON(http.StatusOK, notes)
}

func (h *NoteHandler) GetByID(c *gin.Context) {
	ctx := c.Request.Context()

	id, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	note, err := h.service.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "note not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch note"})
		return
	}

	c.JSON(http.StatusOK, note)
}

func (h *NoteHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()

	var note domain.Note
	if err := c.ShouldBindJSON(&note); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := h.service.Create(ctx, &note); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create note"})
		return
	}

	c.JSON(http.StatusCreated, note)
}

func (h *NoteHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()

	id, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	var note domain.Note
	if err := c.ShouldBindJSON(&note); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := h.service.Update(ctx, id, &note); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "note not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update note"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "note updated successfully"})
}

func (h *NoteHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()

	id, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	if err := h.service.Delete(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "note not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete note"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "note deleted successfully"})
}

func (h *NoteHandler) BulkUpdate(c *gin.Context) {
	ctx := c.Request.Context()

	var notes []*domain.Note
	if err := c.ShouldBindJSON(&notes); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := h.service.BulkUpdate(ctx, notes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to bulk update"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "bulk update completed successfully"})
}
