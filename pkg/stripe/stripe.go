package stripe

import (
	"fmt"

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
	customerParams := &stripe.CustomerParams{
		Email: stripe.String(params.Email),
		Name:  stripe.String(params.Name),
	}

	// Add metadata
	customerParams.AddMetadata("organization_id", params.OrgID)
	customerParams.AddMetadata("billing_email", params.BillingEmail)

	cust, err := customer.New(customerParams)
	if err != nil {
		return nil, fmt.Errorf("failed to create Stripe customer: %w", err)
	}

	return cust, nil
}

// GetCustomer retrieves a Stripe customer by ID
func (c *Client) GetCustomer(customerID string) (*stripe.Customer, error) {
	cust, err := customer.Get(customerID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get Stripe customer: %w", err)
	}
	return cust, nil
}

// UpdateCustomer updates a Stripe customer
func (c *Client) UpdateCustomer(customerID string, params *stripe.CustomerParams) (*stripe.Customer, error) {
	cust, err := customer.Update(customerID, params)
	if err != nil {
		return nil, fmt.Errorf("failed to update Stripe customer: %w", err)
	}
	return cust, nil
}

// CheckoutSessionParams contains parameters for creating a checkout session
type CheckoutSessionParams struct {
	CustomerID   string
	PriceID      string
	SuccessURL   string
	CancelURL    string
	OrgID        string
	OrgName      string
	Plan         string
	BillingCycle string
}

// CreateCheckoutSession creates a Stripe Checkout session
func (c *Client) CreateCheckoutSession(params CheckoutSessionParams) (*stripe.CheckoutSession, error) {
	checkoutParams := &stripe.CheckoutSessionParams{
		Mode:       stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		Customer:   stripe.String(params.CustomerID),
		SuccessURL: stripe.String(params.SuccessURL),
		CancelURL:  stripe.String(params.CancelURL),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(params.PriceID),
				Quantity: stripe.Int64(1),
			},
		},
		SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{
			Metadata: map[string]string{
				"organization_id":   params.OrgID,
				"organization_name": params.OrgName,
				"plan":              params.Plan,
				"billing_cycle":     params.BillingCycle,
			},
		},
	}

	// Add metadata for tracking (for the checkout session itself)
	checkoutParams.AddMetadata("organization_id", params.OrgID)
	checkoutParams.AddMetadata("organization_name", params.OrgName)
	checkoutParams.AddMetadata("plan", params.Plan)
	checkoutParams.AddMetadata("billing_cycle", params.BillingCycle)

	// Allow promotion codes
	checkoutParams.AllowPromotionCodes = stripe.Bool(true)

	sess, err := session.New(checkoutParams)
	if err != nil {
		return nil, fmt.Errorf("failed to create checkout session: %w", err)
	}

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
	sub, err := subscription.Get(subscriptionID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}
	return sub, nil
}

// ListCustomerSubscriptions lists all subscriptions for a customer
func (c *Client) ListCustomerSubscriptions(customerID string) ([]*stripe.Subscription, error) {
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
		return nil, fmt.Errorf("failed to list customer subscriptions: %w", err)
	}

	return subscriptions, nil
}

// CancelSubscription cancels a subscription
func (c *Client) CancelSubscription(subscriptionID string, cancelAtPeriodEnd bool) (*stripe.Subscription, error) {
	if cancelAtPeriodEnd {
		// Cancel at end of billing period
		params := &stripe.SubscriptionParams{
			CancelAtPeriodEnd: stripe.Bool(true),
		}
		sub, err := subscription.Update(subscriptionID, params)
		if err != nil {
			return nil, fmt.Errorf("failed to cancel subscription at period end: %w", err)
		}
		return sub, nil
	}

	// Cancel immediately
	sub, err := subscription.Cancel(subscriptionID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to cancel subscription immediately: %w", err)
	}
	return sub, nil
}

// ReactivateSubscription reactivates a subscription set to cancel
func (c *Client) ReactivateSubscription(subscriptionID string) (*stripe.Subscription, error) {
	params := &stripe.SubscriptionParams{
		CancelAtPeriodEnd: stripe.Bool(false),
	}
	sub, err := subscription.Update(subscriptionID, params)
	if err != nil {
		return nil, fmt.Errorf("failed to reactivate subscription: %w", err)
	}
	return sub, nil
}

// ConstructWebhookEvent constructs and verifies a webhook event
func (c *Client) ConstructWebhookEvent(payload []byte, signature string) (stripe.Event, error) {
	event, err := webhook.ConstructEventWithOptions(
		payload,
		signature,
		c.webhookSecret,
		webhook.ConstructEventOptions{
			IgnoreAPIVersionMismatch: true,
		},
	)
	if err != nil {
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
