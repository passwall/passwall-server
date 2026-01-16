package http

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/service"
)

type WebhookHandler struct {
	paymentService service.PaymentService
}

func NewWebhookHandler(paymentService service.PaymentService) *WebhookHandler {
	return &WebhookHandler{
		paymentService: paymentService,
	}
}

// HandleStripeWebhook godoc
// @Summary Handle Stripe webhooks
// @Description Receive and process Stripe webhook events
// @Tags webhooks
// @Accept json
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /webhooks/stripe [post]
func (h *WebhookHandler) HandleStripeWebhook(c *gin.Context) {
	ctx := c.Request.Context()

	// Read raw body (needed for signature verification)
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	// Get Stripe signature from header
	signature := c.GetHeader("Stripe-Signature")
	if signature == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing Stripe-Signature header"})
		return
	}

	// Process webhook
	if err := h.paymentService.HandleWebhook(ctx, payload, signature); err != nil {
		// Log the error for debugging
		// Return 200 even on error to prevent Stripe retries (signature already verified)
		// Only return error in response body for debugging
		c.JSON(http.StatusOK, gin.H{"error": err.Error(), "received": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{"received": true})
}
