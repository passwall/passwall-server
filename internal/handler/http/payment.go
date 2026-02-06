package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
)

type PaymentHandler struct {
	service             service.PaymentService
	subscriptionService service.SubscriptionService
	orgRepo             repository.OrganizationRepository
}

func NewPaymentHandler(service service.PaymentService, subscriptionService service.SubscriptionService, orgRepo repository.OrganizationRepository) *PaymentHandler {
	return &PaymentHandler{
		service:             service,
		subscriptionService: subscriptionService,
		orgRepo:             orgRepo,
	}
}

// CreateCheckoutSession godoc
// @Summary Create Stripe checkout session
// @Description Create a Stripe checkout session for upgrading organization
// @Tags payments
// @Accept json
// @Produce json
// @Param id path int true "Organization ID"
// @Param request body CreateCheckoutRequest true "Checkout details"
// @Success 200 {object} CreateCheckoutResponse
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /organizations/{id}/checkout [post]
func (h *PaymentHandler) CreateCheckoutSession(c *gin.Context) {
	ctx := c.Request.Context()

	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	var req CreateCheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	// Validate plan
	validPlans := []string{"premium", "family", "team", "business"}
	isValid := false
	for _, plan := range validPlans {
		if req.Plan == plan {
			isValid = true
			break
		}
	}
	if !isValid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plan"})
		return
	}

	// Validate billing cycle
	if req.BillingCycle != "monthly" && req.BillingCycle != "yearly" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid billing cycle"})
		return
	}

	// Validate seats
	if req.Seats <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid seats"})
		return
	}

	// Get user ID for activity logging
	userID, err := GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// Get IP and User-Agent for activity logging
	ipAddress := c.ClientIP()
	userAgent := c.Request.UserAgent()

	// Create checkout session
	checkoutURL, err := h.service.CreateCheckoutSession(ctx, orgID, userID, req.Plan, req.BillingCycle, req.Seats, ipAddress, userAgent)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create checkout session", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, CreateCheckoutResponse{
		URL: checkoutURL,
	})
}

// GetBillingInfo godoc
// @Summary Get billing information
// @Description Get billing and subscription information for an organization
// @Tags payments
// @Produce json
// @Param id path int true "Organization ID"
// @Success 200 {object} domain.BillingInfo
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /organizations/{id}/billing [get]
func (h *PaymentHandler) GetBillingInfo(c *gin.Context) {
	ctx := c.Request.Context()

	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	billingInfo, err := h.service.GetBillingInfo(ctx, orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get billing info", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, billingInfo)
}

// GetMyBillingInfo godoc
// @Summary Get billing information for current user's default organization
// @Description Get billing and subscription information for the authenticated user's default (personal) organization.
// @Description This is a convenience endpoint for mobile apps that don't manage organizations directly.
// @Tags payments
// @Produce json
// @Success 200 {object} domain.BillingInfo
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /me/billing [get]
func (h *PaymentHandler) GetMyBillingInfo(c *gin.Context) {
	ctx := c.Request.Context()

	// Get authenticated user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	uid, ok := userID.(uint)
	if !ok {
		// Try int conversion (some auth middleware sets int)
		if intID, ok := userID.(int); ok {
			uid = uint(intID)
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user_id in context"})
			return
		}
	}

	// Find user's default organization
	org, err := h.orgRepo.GetDefaultByOwnerID(ctx, uid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "default organization not found"})
		return
	}

	// Get billing info for the default org
	billingInfo, err := h.service.GetBillingInfo(ctx, org.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get billing info", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, billingInfo)
}

// CancelSubscription godoc
// @Summary Cancel subscription
// @Description Cancel an organization's subscription
// @Tags payments
// @Accept json
// @Produce json
// @Param id path int true "Organization ID"
// @Param request body CancelSubscriptionRequest true "Cancel options"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /organizations/{id}/subscription/cancel [post]
func (h *PaymentHandler) CancelSubscription(c *gin.Context) {
	ctx := c.Request.Context()

	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	// Cancel subscription at period end using SubscriptionService
	if err := h.subscriptionService.Cancel(ctx, orgID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to cancel subscription", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Subscription will be canceled at the end of the billing period. You'll keep access until then.",
	})
}

// ReactivateSubscription godoc
// @Summary Reactivate subscription
// @Description Reactivate a subscription that is set to cancel
// @Tags payments
// @Produce json
// @Param id path int true "Organization ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /organizations/{id}/subscription/reactivate [post]
func (h *PaymentHandler) ReactivateSubscription(c *gin.Context) {
	ctx := c.Request.Context()

	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	// Resume subscription using SubscriptionService
	if err := h.subscriptionService.Resume(ctx, orgID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reactivate subscription", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Subscription reactivated successfully"})
}

// SyncSubscription godoc
// @Summary Sync subscription
// @Description Manually sync subscription data from Stripe
// @Tags payments
// @Produce json
// @Param id path int true "Organization ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /organizations/{id}/subscription/sync [post]
func (h *PaymentHandler) SyncSubscription(c *gin.Context) {
	ctx := c.Request.Context()

	orgID, ok := GetUintParam(c, "id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	if err := h.service.SyncSubscription(ctx, orgID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Subscription synced successfully"})
}

// UpdateSubscriptionSeats godoc
// @Summary Update subscription seats
// @Description Increase/decrease seat quantity for an organization's active subscription (Stripe proration applies)
// @Tags payments
// @Accept json
// @Produce json
// @Param id path int true "Organization ID"
// @Param request body UpdateSubscriptionSeatsRequest true "Seat update"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /organizations/{id}/subscription/seats [post]
func (h *PaymentHandler) UpdateSubscriptionSeats(c *gin.Context) {
	ctx := c.Request.Context()

	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	var req UpdateSubscriptionSeatsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}
	if req.Seats <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid seats"})
		return
	}

	userID, err := GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	ipAddress := c.ClientIP()
	userAgent := c.Request.UserAgent()

	if err := h.service.UpdateSubscriptionSeats(ctx, orgID, userID, req.Seats, ipAddress, userAgent); err != nil {
		// Keep error mapping simple for now
		if err.Error() == "forbidden" {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Subscription seats updated successfully"})
}

// Request/Response types
type CreateCheckoutRequest struct {
	Plan         string `json:"plan" binding:"required"`          // premium, family, team, business
	BillingCycle string `json:"billing_cycle" binding:"required"` // monthly, yearly
	Seats        int    `json:"seats" binding:"required"`         // seat count (quantity). Use 1 for non-seat plans.
}

type CreateCheckoutResponse struct {
	URL string `json:"url"` // Stripe checkout URL
}

type UpdateSubscriptionSeatsRequest struct {
	Seats int `json:"seats" binding:"required"`
}

// CancelSubscriptionRequest is no longer needed - we always cancel at period end
