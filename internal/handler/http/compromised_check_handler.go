package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/pkg/hibp"
)

// CompromisedCheckHandler handles batch password compromise checks via HIBP Pwned Passwords.
type CompromisedCheckHandler struct {
	pwnedClient *hibp.PwnedPasswordsClient
}

// NewCompromisedCheckHandler creates a new handler.
func NewCompromisedCheckHandler(pwnedClient *hibp.PwnedPasswordsClient) *CompromisedCheckHandler {
	return &CompromisedCheckHandler{pwnedClient: pwnedClient}
}

type batchCheckRequest struct {
	Hashes []string `json:"hashes" binding:"required"`
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

	results, err := h.pwnedClient.CheckBatch(req.Hashes)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "compromised check temporarily unavailable",
		})
		return
	}

	c.JSON(http.StatusOK, batchCheckResponse{Results: results})
}
