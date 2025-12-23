package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
)

type CreditCardHandler struct {
	service service.CreditCardService
}

func NewCreditCardHandler(service service.CreditCardService) *CreditCardHandler {
	return &CreditCardHandler{service: service}
}

func (h *CreditCardHandler) List(c *gin.Context) {
	ctx := c.Request.Context()

	cards, err := h.service.List(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch credit cards"})
		return
	}

	c.JSON(http.StatusOK, cards)
}

func (h *CreditCardHandler) GetByID(c *gin.Context) {
	ctx := c.Request.Context()

	id, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	card, err := h.service.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "credit card not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch credit card"})
		return
	}

	c.JSON(http.StatusOK, card)
}

func (h *CreditCardHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()

	var card domain.CreditCard
	if err := c.ShouldBindJSON(&card); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := h.service.Create(ctx, &card); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create credit card"})
		return
	}

	c.JSON(http.StatusCreated, card)
}

func (h *CreditCardHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()

	id, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	var card domain.CreditCard
	if err := c.ShouldBindJSON(&card); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := h.service.Update(ctx, id, &card); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "credit card not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update credit card"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "credit card updated successfully"})
}

func (h *CreditCardHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()

	id, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	if err := h.service.Delete(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "credit card not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete credit card"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "credit card deleted successfully"})
}

func (h *CreditCardHandler) BulkUpdate(c *gin.Context) {
	ctx := c.Request.Context()

	var cards []*domain.CreditCard
	if err := c.ShouldBindJSON(&cards); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := h.service.BulkUpdate(ctx, cards); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to bulk update"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "bulk update completed successfully"})
}
