package stripe

import (
	"fmt"
	"strings"

	"github.com/passwall/passwall-server/pkg/logger"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/checkout/session"
	"github.com/stripe/stripe-go/v81/customer"
	"github.com/stripe/stripe-go/v81/invoice"
	"github.com/stripe/stripe-go/v81/subscription"
	"github.com/stripe/stripe-go/v81/webhook"
)

// Client wraps Stripe API client with application-specific methods
type Client struct {
	apiKey        string
	webhookSecret string
}

// NewClient creates a new Stripe client
func NewClient(apiKey, webhookSecret string) *Client {
	stripe.Key = apiKey
	mode := "unknown"
	switch {
	case strings.HasPrefix(apiKey, "sk_test_"):
		mode = "test"
	case strings.HasPrefix(apiKey, "sk_live_"):
		mode = "live"
	}
	logger.Infof("Stripe client initialized mode=%s webhook_secret_set=%t", mode, webhookSecret != "")
	return &Client{
		apiKey:        apiKey,
		webhookSecret: webhookSecret,
	}
}

// CreateCustomerParams contains parameters for creating a customer
type CreateCustomerParams struct {
	Email        string
	Name         string
	OrgID        string // Organization ID (for metadata)
	BillingEmail string
}

// CreateCustomer creates a new Stripe customer
func (c *Client) CreateCustomer(params CreateCustomerParams) (*stripe.Customer, error) {
	logger.Infof("stripe.CreateCustomer org_id=%s billing_email_set=%t", params.OrgID, params.BillingEmail != "")
	customerParams := &stripe.CustomerParams{
		Email: stripe.String(params.Email),
		Name:  stripe.String(params.Name),
	}

	// Add metadata
	customerParams.AddMetadata("organization_id", params.OrgID)
	customerParams.AddMetadata("billing_email", params.BillingEmail)

	cust, err := customer.New(customerParams)
	if err != nil {
		logger.Errorf("stripe.CreateCustomer failed org_id=%s err=%v", params.OrgID, err)
		return nil, fmt.Errorf("failed to create Stripe customer: %w", err)
	}

	logger.Infof("stripe.CreateCustomer ok org_id=%s customer_id=%s", params.OrgID, cust.ID)
	return cust, nil
}

// GetCustomer retrieves a Stripe customer by ID
func (c *Client) GetCustomer(customerID string) (*stripe.Customer, error) {
	logger.Infof("stripe.GetCustomer customer_id=%s", customerID)
	cust, err := customer.Get(customerID, nil)
	if err != nil {
		logger.Errorf("stripe.GetCustomer failed customer_id=%s err=%v", customerID, err)
		return nil, fmt.Errorf("failed to get Stripe customer: %w", err)
	}
	return cust, nil
}

// UpdateCustomer updates a Stripe customer
func (c *Client) UpdateCustomer(customerID string, params *stripe.CustomerParams) (*stripe.Customer, error) {
	logger.Infof("stripe.UpdateCustomer customer_id=%s", customerID)
	cust, err := customer.Update(customerID, params)
	if err != nil {
		logger.Errorf("stripe.UpdateCustomer failed customer_id=%s err=%v", customerID, err)
		return nil, fmt.Errorf("failed to update Stripe customer: %w", err)
	}
	return cust, nil
}

// CheckoutSessionParams contains parameters for creating a checkout session
type CheckoutSessionParams struct {
	CustomerID   string
	PriceID      string
	Quantity     int64
	SuccessURL   string
	CancelURL    string
	OrgID        string
	OrgName      string
	Plan         string
	BillingCycle string
	TrialDays    int               // Trial period in days (0 = no trial)
	Metadata     map[string]string // Additional metadata for user-level subscriptions
}

// StripeSubscription is a type alias for stripe.Subscription for external use
type StripeSubscription = stripe.Subscription

// CreateCheckoutSession creates a Stripe Checkout session
func (c *Client) CreateCheckoutSession(params CheckoutSessionParams) (*stripe.CheckoutSession, error) {
	quantity := params.Quantity
	if quantity <= 0 {
		quantity = 1
	}

	logger.Infof("stripe.CreateCheckoutSession org_id=%s customer_id=%s price_id=%s quantity=%d",
		params.OrgID, params.CustomerID, params.PriceID, quantity,
	)

	// Build subscription metadata
	subscriptionMetadata := map[string]string{
		"plan":          params.Plan,
		"billing_cycle": params.BillingCycle,
	}

	// Add org metadata if this is an organization subscription
	if params.OrgID != "" {
		subscriptionMetadata["organization_id"] = params.OrgID
		subscriptionMetadata["organization_name"] = params.OrgName
	}

	// Merge in any additional custom metadata (e.g., user_id for personal subscriptions)
	for k, v := range params.Metadata {
		subscriptionMetadata[k] = v
	}

	checkoutParams := &stripe.CheckoutSessionParams{
		Mode:       stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		Customer:   stripe.String(params.CustomerID),
		SuccessURL: stripe.String(params.SuccessURL),
		CancelURL:  stripe.String(params.CancelURL),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(params.PriceID),
				Quantity: stripe.Int64(quantity),
			},
		},
		SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{
			Metadata: subscriptionMetadata,
		},
	}

	// Add trial period if specified
	if params.TrialDays > 0 {
		checkoutParams.SubscriptionData.TrialPeriodDays = stripe.Int64(int64(params.TrialDays))
	}

	// Add metadata for tracking (for the checkout session itself)
	for k, v := range subscriptionMetadata {
		checkoutParams.AddMetadata(k, v)
	}
	checkoutParams.AddMetadata("seats", fmt.Sprintf("%d", quantity))

	// Allow promotion codes
	checkoutParams.AllowPromotionCodes = stripe.Bool(true)

	sess, err := session.New(checkoutParams)
	if err != nil {
		logger.Errorf("stripe.CreateCheckoutSession failed org_id=%s customer_id=%s price_id=%s err=%v",
			params.OrgID, params.CustomerID, params.PriceID, err,
		)
		return nil, fmt.Errorf("failed to create checkout session: %w", err)
	}

	logger.Infof("stripe.CreateCheckoutSession ok org_id=%s session_id=%s", params.OrgID, sess.ID)
	return sess, nil
}

// CreateSubscription creates a new subscription for a customer
func (c *Client) CreateSubscription(customerID, priceID string, metadata map[string]string) (*stripe.Subscription, error) {
	params := &stripe.SubscriptionParams{
		Customer: stripe.String(customerID),
		Items: []*stripe.SubscriptionItemsParams{
			{
				Price: stripe.String(priceID),
			},
		},
	}

	// Add metadata
	for key, value := range metadata {
		params.AddMetadata(key, value)
	}

	sub, err := subscription.New(params)
	if err != nil {
		return nil, fmt.Errorf("failed to create subscription: %w", err)
	}

	return sub, nil
}

// GetSubscription retrieves a subscription by ID
func (c *Client) GetSubscription(subscriptionID string) (*stripe.Subscription, error) {
	logger.Infof("stripe.GetSubscription subscription_id=%s", subscriptionID)
	sub, err := subscription.Get(subscriptionID, nil)
	if err != nil {
		logger.Errorf("stripe.GetSubscription failed subscription_id=%s err=%v", subscriptionID, err)
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}
	return sub, nil
}

// ListCustomerSubscriptions lists all subscriptions for a customer
func (c *Client) ListCustomerSubscriptions(customerID string) ([]*stripe.Subscription, error) {
	logger.Infof("stripe.ListCustomerSubscriptions customer_id=%s", customerID)
	params := &stripe.SubscriptionListParams{
		Customer: stripe.String(customerID),
	}
	params.AddExpand("data.default_payment_method")

	iter := subscription.List(params)
	var subscriptions []*stripe.Subscription

	for iter.Next() {
		subscriptions = append(subscriptions, iter.Subscription())
	}

	if err := iter.Err(); err != nil {
		logger.Errorf("stripe.ListCustomerSubscriptions failed customer_id=%s err=%v", customerID, err)
		return nil, fmt.Errorf("failed to list customer subscriptions: %w", err)
	}

	return subscriptions, nil
}

// CancelSubscription cancels a subscription
func (c *Client) CancelSubscription(subscriptionID string, cancelAtPeriodEnd bool) (*stripe.Subscription, error) {
	logger.Infof("stripe.CancelSubscription subscription_id=%s cancel_at_period_end=%t", subscriptionID, cancelAtPeriodEnd)
	if cancelAtPeriodEnd {
		// Cancel at end of billing period
		params := &stripe.SubscriptionParams{
			CancelAtPeriodEnd: stripe.Bool(true),
		}
		sub, err := subscription.Update(subscriptionID, params)
		if err != nil {
			logger.Errorf("stripe.CancelSubscription failed subscription_id=%s err=%v", subscriptionID, err)
			return nil, fmt.Errorf("failed to cancel subscription at period end: %w", err)
		}
		return sub, nil
	}

	// Cancel immediately
	sub, err := subscription.Cancel(subscriptionID, nil)
	if err != nil {
		logger.Errorf("stripe.CancelSubscription failed subscription_id=%s err=%v", subscriptionID, err)
		return nil, fmt.Errorf("failed to cancel subscription immediately: %w", err)
	}
	return sub, nil
}

// ReactivateSubscription reactivates a subscription set to cancel
func (c *Client) ReactivateSubscription(subscriptionID string) (*stripe.Subscription, error) {
	logger.Infof("stripe.ReactivateSubscription subscription_id=%s", subscriptionID)
	params := &stripe.SubscriptionParams{
		CancelAtPeriodEnd: stripe.Bool(false),
	}
	sub, err := subscription.Update(subscriptionID, params)
	if err != nil {
		logger.Errorf("stripe.ReactivateSubscription failed subscription_id=%s err=%v", subscriptionID, err)
		return nil, fmt.Errorf("failed to reactivate subscription: %w", err)
	}
	return sub, nil
}

// UpdateSubscriptionQuantity updates the quantity (seats) for the first subscription item.
// This is used for seat-based (per-user) billing. Stripe will apply proration according
// to your account settings and the subscription's collection method.
func (c *Client) UpdateSubscriptionQuantity(subscriptionID string, quantity int64) (*stripe.Subscription, error) {
	if subscriptionID == "" {
		return nil, fmt.Errorf("subscriptionID is required")
	}
	if quantity <= 0 {
		return nil, fmt.Errorf("quantity must be > 0")
	}

	logger.Infof("stripe.UpdateSubscriptionQuantity subscription_id=%s quantity=%d", subscriptionID, quantity)
	// Fetch current subscription to get subscription item ID.
	sub, err := subscription.Get(subscriptionID, nil)
	if err != nil {
		logger.Errorf("stripe.UpdateSubscriptionQuantity failed subscription_id=%s err=%v", subscriptionID, err)
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}
	if sub == nil || len(sub.Items.Data) == 0 {
		return nil, fmt.Errorf("subscription has no items to update")
	}

	itemID := sub.Items.Data[0].ID
	if itemID == "" {
		return nil, fmt.Errorf("subscription item id is empty")
	}

	params := &stripe.SubscriptionParams{
		Items: []*stripe.SubscriptionItemsParams{
			{
				ID:       stripe.String(itemID),
				Quantity: stripe.Int64(quantity),
			},
		},
		// For seat increases we want Stripe to invoice & attempt payment immediately.
		// See: https://docs.stripe.com/billing/subscriptions/prorations
		ProrationBehavior: stripe.String("always_invoice"),
	}

	updated, err := subscription.Update(subscriptionID, params)
	if err != nil {
		logger.Errorf("stripe.UpdateSubscriptionQuantity failed subscription_id=%s err=%v", subscriptionID, err)
		return nil, fmt.Errorf("failed to update subscription quantity: %w", err)
	}

	logger.Infof("stripe.UpdateSubscriptionQuantity ok subscription_id=%s", subscriptionID)
	return updated, nil
}

// PreviewSeatChange uses Stripe's upcoming invoice API to show what the user
// will be charged (or credited) if they change seat count. No mutation occurs.
func (c *Client) PreviewSeatChange(subscriptionID string, newQuantity int64) (*stripe.Invoice, error) {
	if subscriptionID == "" {
		return nil, fmt.Errorf("subscriptionID is required")
	}
	if newQuantity <= 0 {
		return nil, fmt.Errorf("quantity must be > 0")
	}

	logger.Infof("stripe.PreviewSeatChange subscription_id=%s quantity=%d", subscriptionID, newQuantity)

	sub, err := subscription.Get(subscriptionID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}
	if sub == nil || len(sub.Items.Data) == 0 {
		return nil, fmt.Errorf("subscription has no items")
	}

	itemID := sub.Items.Data[0].ID

	params := &stripe.InvoiceUpcomingParams{
		Subscription: stripe.String(subscriptionID),
		SubscriptionItems: []*stripe.SubscriptionItemsParams{
			{
				ID:       stripe.String(itemID),
				Quantity: stripe.Int64(newQuantity),
			},
		},
		SubscriptionProrationBehavior: stripe.String("always_invoice"),
	}

	inv, err := invoice.Upcoming(params)
	if err != nil {
		logger.Errorf("stripe.PreviewSeatChange failed subscription_id=%s err=%v", subscriptionID, err)
		return nil, fmt.Errorf("failed to preview seat change: %w", err)
	}

	return inv, nil
}

// UpdateSubscriptionPlan replaces the price on an existing subscription (plan change).
// This is the industry-standard approach for changing plans when the customer already
// has an active subscription with a payment method on file. Stripe handles proration
// automatically — the customer is NOT redirected to a new checkout page.
func (c *Client) UpdateSubscriptionPlan(subscriptionID string, newPriceID string, newQuantity int64, metadata map[string]string) (*stripe.Subscription, error) {
	if subscriptionID == "" {
		return nil, fmt.Errorf("subscriptionID is required")
	}
	if newPriceID == "" {
		return nil, fmt.Errorf("newPriceID is required")
	}
	if newQuantity <= 0 {
		newQuantity = 1
	}

	logger.Infof("stripe.UpdateSubscriptionPlan subscription_id=%s new_price=%s quantity=%d", subscriptionID, newPriceID, newQuantity)

	// Fetch current subscription to get the subscription item ID
	sub, err := subscription.Get(subscriptionID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}
	if sub == nil || len(sub.Items.Data) == 0 {
		return nil, fmt.Errorf("subscription has no items to update")
	}

	oldItemID := sub.Items.Data[0].ID

	params := &stripe.SubscriptionParams{
		Items: []*stripe.SubscriptionItemsParams{
			{
				ID:       stripe.String(oldItemID),
				Price:    stripe.String(newPriceID),
				Quantity: stripe.Int64(newQuantity),
			},
		},
		// Immediately invoice the prorated amount for plan changes
		ProrationBehavior: stripe.String("always_invoice"),
	}

	// Update metadata (plan, billing_cycle, etc.)
	for k, v := range metadata {
		params.AddMetadata(k, v)
	}

	updated, err := subscription.Update(subscriptionID, params)
	if err != nil {
		logger.Errorf("stripe.UpdateSubscriptionPlan failed subscription_id=%s err=%v", subscriptionID, err)
		return nil, fmt.Errorf("failed to update subscription plan: %w", err)
	}

	logger.Infof("stripe.UpdateSubscriptionPlan ok subscription_id=%s new_price=%s", subscriptionID, newPriceID)
	return updated, nil
}

// PreviewPlanChange previews the cost of switching to a different plan/price.
// Uses the same upcoming invoice API as seat changes but with a different price ID.
func (c *Client) PreviewPlanChange(subscriptionID string, newPriceID string, newQuantity int64) (*stripe.Invoice, error) {
	if subscriptionID == "" {
		return nil, fmt.Errorf("subscriptionID is required")
	}

	logger.Infof("stripe.PreviewPlanChange subscription_id=%s new_price=%s quantity=%d", subscriptionID, newPriceID, newQuantity)

	sub, err := subscription.Get(subscriptionID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}
	if sub == nil || len(sub.Items.Data) == 0 {
		return nil, fmt.Errorf("subscription has no items")
	}

	itemID := sub.Items.Data[0].ID

	params := &stripe.InvoiceUpcomingParams{
		Subscription: stripe.String(subscriptionID),
		SubscriptionItems: []*stripe.SubscriptionItemsParams{
			{
				ID:       stripe.String(itemID),
				Price:    stripe.String(newPriceID),
				Quantity: stripe.Int64(newQuantity),
			},
		},
		SubscriptionProrationBehavior: stripe.String("always_invoice"),
	}

	inv, err := invoice.Upcoming(params)
	if err != nil {
		logger.Errorf("stripe.PreviewPlanChange failed subscription_id=%s err=%v", subscriptionID, err)
		return nil, fmt.Errorf("failed to preview plan change: %w", err)
	}

	return inv, nil
}

// ConstructWebhookEvent constructs and verifies a webhook event
func (c *Client) ConstructWebhookEvent(payload []byte, signature string) (stripe.Event, error) {
	logger.Infof("stripe.ConstructWebhookEvent payload_size=%d signature_present=%t", len(payload), signature != "")
	event, err := webhook.ConstructEventWithOptions(
		payload,
		signature,
		c.webhookSecret,
		webhook.ConstructEventOptions{
			IgnoreAPIVersionMismatch: true,
		},
	)
	if err != nil {
		logger.Errorf("stripe.ConstructWebhookEvent failed err=%v", err)
		return stripe.Event{}, fmt.Errorf("failed to verify webhook signature: %w", err)
	}
	return event, nil
}

// GetPriceFromSubscription extracts the price ID from a subscription
func GetPriceFromSubscription(sub *stripe.Subscription) string {
	if len(sub.Items.Data) > 0 {
		return sub.Items.Data[0].Price.ID
	}
	return ""
}

// ListInvoices lists invoices for a customer
func (c *Client) ListInvoices(customerID string, limit int64) ([]*stripe.Invoice, error) {
	logger.Infof("stripe.ListInvoices customer_id=%s limit=%d", customerID, limit)
	params := &stripe.InvoiceListParams{
		Customer: stripe.String(customerID),
	}
	params.Limit = stripe.Int64(limit)

	iter := invoice.List(params)
	var invoices []*stripe.Invoice

	for iter.Next() {
		invoices = append(invoices, iter.Invoice())
	}

	if err := iter.Err(); err != nil {
		logger.Errorf("stripe.ListInvoices failed customer_id=%s err=%v", customerID, err)
		return nil, fmt.Errorf("failed to list invoices: %w", err)
	}

	return invoices, nil
}

// GetBillingCycleFromPrice determines billing cycle from price interval
func GetBillingCycleFromPrice(priceInterval stripe.PriceRecurringInterval) string {
	switch priceInterval {
	case stripe.PriceRecurringIntervalMonth:
		return "monthly"
	case stripe.PriceRecurringIntervalYear:
		return "yearly"
	default:
		return "monthly"
	}
}

// CancelSubscription cancels a Stripe subscription
// If cancelAtPeriodEnd is true, subscription will remain active until end of billing period
// If false, subscription is canceled immediately
