package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
	"github.com/passwall/passwall-server/pkg/hibp"
)

// CompromisedCheckHandler handles batch password compromise checks via HIBP Pwned Passwords.
type CompromisedCheckHandler struct {
	pwnedClient  *hibp.PwnedPasswordsClient
	userRepo     repository.UserRepository
	orgUserRepo  repository.OrganizationUserRepository
	entitlements service.OrganizationEntitlementService
}

// NewCompromisedCheckHandler creates a new handler.
func NewCompromisedCheckHandler(
	pwnedClient *hibp.PwnedPasswordsClient,
	userRepo repository.UserRepository,
	orgUserRepo repository.OrganizationUserRepository,
	entitlements service.OrganizationEntitlementService,
) *CompromisedCheckHandler {
	return &CompromisedCheckHandler{
		pwnedClient:  pwnedClient,
		userRepo:     userRepo,
		orgUserRepo:  orgUserRepo,
		entitlements: entitlements,
	}
}

type batchCheckRequest struct {
	Hashes         []string `json:"hashes" binding:"required"`
	OrganizationID uint     `json:"organization_id,omitempty"`
}

type batchCheckResponse struct {
	Results []hibp.PwnedResult `json:"results"`
}

// BatchCheck handles POST /api/compromised-check
// Accepts a list of uppercase SHA-1 hashes and returns breach counts for each.
func (h *CompromisedCheckHandler) BatchCheck(c *gin.Context) {
	var req batchCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "hashes array is required"})
		return
	}

	if len(req.Hashes) == 0 {
		c.JSON(http.StatusOK, batchCheckResponse{Results: []hibp.PwnedResult{}})
		return
	}

	const maxBatchSize = 1000
	if len(req.Hashes) > maxBatchSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "maximum 1000 hashes per request",
		})
		return
	}
	orgID := req.OrganizationID
	userID := GetCurrentUserID(c)
	if orgID == 0 {
		user, err := h.userRepo.GetByID(c.Request.Context(), userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve organization"})
			return
		}
		orgID = user.DefaultOrganizationID
	}
	if _, err := h.orgUserRepo.GetActiveByOrgAndUser(c.Request.Context(), orgID, userID); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, repository.ErrNotFound) {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": "access denied"})
		return
	}
	if err := h.entitlements.Authorize(c.Request.Context(), orgID, domain.CapabilitySecurityInsightsRead); err != nil {
		if respondEntitlementError(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check security insights entitlement"})
		return
	}

	results, err := h.pwnedClient.CheckBatch(req.Hashes)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "compromised check temporarily unavailable",
		})
		return
	}

	c.JSON(http.StatusOK, batchCheckResponse{Results: results})
}
