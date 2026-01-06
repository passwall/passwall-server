package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/service"
)

type ExcludedDomainHandler struct {
	service service.ExcludedDomainService
}

func NewExcludedDomainHandler(service service.ExcludedDomainService) *ExcludedDomainHandler {
	return &ExcludedDomainHandler{service: service}
}

// List returns all excluded domains for the authenticated user
// GET /api/excluded-domains
func (h *ExcludedDomainHandler) List(c *gin.Context) {
	userID, err := GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	domains, err := h.service.GetByUserID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get excluded domains"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"excluded_domains": domain.ToExcludedDomainDTOs(domains),
	})
}

// Create adds a new excluded domain
// POST /api/excluded-domains
func (h *ExcludedDomainHandler) Create(c *gin.Context) {
	userID, err := GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req domain.CreateExcludedDomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	excludedDomain, err := h.service.Create(c.Request.Context(), userID, &req)
	if err != nil {
		if err.Error() == "domain already excluded" {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, domain.ToExcludedDomainDTO(excludedDomain))
}

// Delete removes an excluded domain by ID
// DELETE /api/excluded-domains/:id
func (h *ExcludedDomainHandler) Delete(c *gin.Context) {
	userID, err := GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.service.Delete(c.Request.Context(), uint(id), userID); err != nil {
		if err.Error() == "excluded domain not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete excluded domain"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "excluded domain deleted"})
}

// DeleteByDomain removes an excluded domain by domain name
// DELETE /api/excluded-domains/by-domain/:domain
func (h *ExcludedDomainHandler) DeleteByDomain(c *gin.Context) {
	userID, err := GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	domain := c.Param("domain")
	if domain == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "domain is required"})
		return
	}

	if err := h.service.DeleteByDomain(c.Request.Context(), userID, domain); err != nil {
		if err.Error() == "excluded domain not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "excluded domain deleted"})
}

// Check if a domain is excluded
// GET /api/excluded-domains/check/:domain
func (h *ExcludedDomainHandler) Check(c *gin.Context) {
	userID, err := GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	domain := c.Param("domain")
	if domain == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "domain is required"})
		return
	}

	isExcluded, err := h.service.IsExcluded(c.Request.Context(), userID, domain)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check domain"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"is_excluded": isExcluded})
}
