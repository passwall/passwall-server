package http

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/service"
)

type WebhookHandler struct {
	paymentService    service.PaymentService
	revenueCatService service.RevenueCatService
}

func NewWebhookHandler(paymentService service.PaymentService, revenueCatService service.RevenueCatService) *WebhookHandler {
	return &WebhookHandler{
		paymentService:    paymentService,
		revenueCatService: revenueCatService,
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
		// Important:
		// - Signature is verified inside HandleWebhook.
		// - If processing fails (DB/Stripe transient), return 500 so Stripe can retry.
		if errors.Is(err, service.ErrInvalidStripeWebhookSignature) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid Stripe signature", "received": false})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "received": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{"received": true})
}

// HandleRevenueCatWebhook godoc
// @Summary Handle RevenueCat webhooks
// @Description Receive and process RevenueCat webhook events for mobile in-app purchases
// @Tags webhooks
// @Accept json
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /webhooks/revenuecat [post]
func (h *WebhookHandler) HandleRevenueCatWebhook(c *gin.Context) {
	ctx := c.Request.Context()

	// Read raw body (needed for signature verification)
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	// Get RevenueCat authorization from header
	// RevenueCat sends: "Authorization: Bearer <your-secret>"
	authHeader := c.GetHeader("Authorization")
	var authToken string
	if authHeader != "" {
		// Strip "Bearer " prefix if present
		if strings.HasPrefix(authHeader, "Bearer ") {
			authToken = strings.TrimPrefix(authHeader, "Bearer ")
		} else {
			authToken = authHeader
		}
	}

	// Process webhook
	if err := h.revenueCatService.HandleWebhook(ctx, payload, authToken); err != nil {
		// Check for signature verification failure
		if errors.Is(err, service.ErrInvalidRevenueCatSignature) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid RevenueCat signature", "received": false})
			return
		}
		// Return 500 for transient errors so RevenueCat will retry
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "received": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{"received": true})
}
