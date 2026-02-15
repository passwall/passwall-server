package http

import (
	"net/http"
	"strings"

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

	// Validate user count
	if req.Seats <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user count"})
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
		// Return 400 for externally managed subscriptions (RevenueCat/App Store/Play Store)
		if isExternalProviderError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
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
		// Return 400 for externally managed subscriptions (RevenueCat/App Store/Play Store)
		if isExternalProviderError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
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

// PreviewSeatChange godoc
// @Summary Preview user license change cost
// @Description Returns the prorated cost impact of changing user count without applying changes
// @Tags payments
// @Accept json
// @Produce json
// @Param id path int true "Organization ID"
// @Param request body UpdateSubscriptionSeatsRequest true "User count preview"
// @Success 200 {object} domain.SeatChangePreview
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /organizations/{id}/subscription/seats/preview [post]
func (h *PaymentHandler) PreviewSeatChange(c *gin.Context) {
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user count"})
		return
	}

	userID, err := GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	preview, err := h.service.PreviewSeatChange(ctx, orgID, userID, req.Seats)
	if err != nil {
		if strings.Contains(err.Error(), "forbidden") {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, preview)
}

// UpdateSubscriptionSeats godoc
// @Summary Update subscription user count
// @Description Increase/decrease user count for an organization's active subscription (Stripe proration applies)
// @Tags payments
// @Accept json
// @Produce json
// @Param id path int true "Organization ID"
// @Param request body UpdateSubscriptionSeatsRequest true "User count update"
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user count"})
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

	c.JSON(http.StatusOK, gin.H{"message": "User licenses updated successfully"})
}

// PreviewPlanChange godoc
// @Summary Preview plan change cost
// @Description Returns the prorated cost impact of switching to a different plan without applying changes
// @Tags payments
// @Accept json
// @Produce json
// @Param id path int true "Organization ID"
// @Param request body CreateCheckoutRequest true "Plan change preview"
// @Success 200 {object} domain.PlanChangePreview
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /organizations/{id}/subscription/change/preview [post]
func (h *PaymentHandler) PreviewPlanChange(c *gin.Context) {
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

	userID, err := GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	preview, err := h.service.PreviewPlanChange(ctx, orgID, userID, req.Plan, req.BillingCycle, req.Seats)
	if err != nil {
		if strings.Contains(err.Error(), "forbidden") {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, preview)
}

// ChangePlan godoc
// @Summary Change subscription plan inline
// @Description Switches an existing subscription to a different plan with prorated billing. No Stripe redirect needed.
// @Tags payments
// @Accept json
// @Produce json
// @Param id path int true "Organization ID"
// @Param request body CreateCheckoutRequest true "Plan change request"
// @Success 200 {object} domain.PlanChangeResult
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /organizations/{id}/subscription/change [post]
func (h *PaymentHandler) ChangePlan(c *gin.Context) {
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
	if req.Seats <= 0 {
		req.Seats = 1
	}

	userID, err := GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	ipAddress := c.ClientIP()
	userAgent := c.Request.UserAgent()

	result, err := h.service.ChangePlan(ctx, orgID, userID, req.Plan, req.BillingCycle, req.Seats, ipAddress, userAgent)
	if err != nil {
		if strings.Contains(err.Error(), "forbidden") {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		if isExternalProviderError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// Request/Response types
type CreateCheckoutRequest struct {
	Plan         string `json:"plan" binding:"required"`          // premium, family, team, business
	BillingCycle string `json:"billing_cycle" binding:"required"` // monthly, yearly
	Seats        int    `json:"seats" binding:"required"`         // user count (quantity). Use 1 for non-per-user plans.
}

type CreateCheckoutResponse struct {
	URL string `json:"url"` // Stripe checkout URL
}

type UpdateSubscriptionSeatsRequest struct {
	Seats int `json:"seats" binding:"required"`
}

// CancelSubscriptionRequest is no longer needed - we always cancel at period end

// isExternalProviderError checks if the error is about a subscription managed by an external provider
// (App Store, Play Store via RevenueCat). These errors should be returned as 400 (client error)
// rather than 500 (server error) since the user needs to take action in the external store.
func isExternalProviderError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "managed by") && (strings.Contains(msg, "App Store") ||
		strings.Contains(msg, "Play Store") || strings.Contains(msg, "directly"))
}
