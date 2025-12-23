package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
)

type BankAccountHandler struct {
	service service.BankAccountService
}

func NewBankAccountHandler(service service.BankAccountService) *BankAccountHandler {
	return &BankAccountHandler{service: service}
}

func (h *BankAccountHandler) List(c *gin.Context) {
	ctx := c.Request.Context()

	accounts, err := h.service.List(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch bank accounts"})
		return
	}

	c.JSON(http.StatusOK, accounts)
}

func (h *BankAccountHandler) GetByID(c *gin.Context) {
	ctx := c.Request.Context()

	id, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	account, err := h.service.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "bank account not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch bank account"})
		return
	}

	c.JSON(http.StatusOK, account)
}

func (h *BankAccountHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()

	var account domain.BankAccount
	if err := c.ShouldBindJSON(&account); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := h.service.Create(ctx, &account); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create bank account"})
		return
	}

	c.JSON(http.StatusCreated, account)
}

func (h *BankAccountHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()

	id, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	var account domain.BankAccount
	if err := c.ShouldBindJSON(&account); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := h.service.Update(ctx, id, &account); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "bank account not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update bank account"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "bank account updated successfully"})
}

func (h *BankAccountHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()

	id, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	if err := h.service.Delete(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "bank account not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete bank account"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "bank account deleted successfully"})
}

func (h *BankAccountHandler) BulkUpdate(c *gin.Context) {
	ctx := c.Request.Context()

	var accounts []*domain.BankAccount
	if err := c.ShouldBindJSON(&accounts); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := h.service.BulkUpdate(ctx, accounts); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to bulk update"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "bulk update completed successfully"})
}
