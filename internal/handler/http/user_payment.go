package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/service"
)

type UserPaymentHandler struct {
	service service.UserPaymentService
}

func NewUserPaymentHandler(service service.UserPaymentService) *UserPaymentHandler {
	return &UserPaymentHandler{
		service: service,
	}
}

// CreateCheckoutSession godoc
// @Summary Create Stripe checkout session for personal subscription
// @Description Create a Stripe checkout session for upgrading to Pro plan
// @Tags user-payments
// @Accept json
// @Produce json
// @Param request body UserCreateCheckoutRequest true "Checkout details"
// @Success 200 {object} UserCreateCheckoutResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/user/checkout [post]
func (h *UserPaymentHandler) CreateCheckoutSession(c *gin.Context) {
	ctx := c.Request.Context()

	userID, err := GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req UserCreateCheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	// Validate plan (only "pro" is allowed for personal subscriptions)
	if req.Plan != "pro" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plan for personal subscription (only 'pro' is allowed)"})
		return
	}

	// Validate billing cycle
	if req.BillingCycle != "monthly" && req.BillingCycle != "yearly" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid billing cycle"})
		return
	}

	ipAddress := c.ClientIP()
	userAgent := c.Request.UserAgent()

	checkoutURL, err := h.service.CreateCheckoutSession(ctx, userID, req.Plan, req.BillingCycle, ipAddress, userAgent)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create checkout session", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, UserCreateCheckoutResponse{
		URL: checkoutURL,
	})
}

// GetBillingInfo godoc
// @Summary Get personal billing information
// @Description Get billing and subscription information for the current user
// @Tags user-payments
// @Produce json
// @Success 200 {object} domain.UserBillingInfo
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/user/billing [get]
func (h *UserPaymentHandler) GetBillingInfo(c *gin.Context) {
	ctx := c.Request.Context()

	userID, err := GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	billingInfo, err := h.service.GetBillingInfo(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get billing info", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, billingInfo)
}

// CancelSubscription godoc
// @Summary Cancel personal subscription
// @Description Cancel the current user's subscription at the end of billing period
// @Tags user-payments
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/user/subscription/cancel [post]
func (h *UserPaymentHandler) CancelSubscription(c *gin.Context) {
	ctx := c.Request.Context()

	userID, err := GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	if err := h.service.Cancel(ctx, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to cancel subscription", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Subscription will be canceled at the end of the billing period. You'll keep access until then.",
	})
}

// ReactivateSubscription godoc
// @Summary Reactivate personal subscription
// @Description Reactivate a subscription that is set to cancel
// @Tags user-payments
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/user/subscription/reactivate [post]
func (h *UserPaymentHandler) ReactivateSubscription(c *gin.Context) {
	ctx := c.Request.Context()

	userID, err := GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	if err := h.service.Resume(ctx, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reactivate subscription", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Subscription reactivated successfully"})
}

// SyncSubscription godoc
// @Summary Sync personal subscription
// @Description Manually sync subscription data from Stripe
// @Tags user-payments
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/user/subscription/sync [post]
func (h *UserPaymentHandler) SyncSubscription(c *gin.Context) {
	ctx := c.Request.Context()

	userID, err := GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	if err := h.service.SyncSubscription(ctx, userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Subscription synced successfully"})
}

// Request/Response types
type UserCreateCheckoutRequest struct {
	Plan         string `json:"plan" binding:"required"`          // Only "pro" is valid for personal subscriptions
	BillingCycle string `json:"billing_cycle" binding:"required"` // monthly, yearly
}

type UserCreateCheckoutResponse struct {
	URL string `json:"url"` // Stripe checkout URL
}
