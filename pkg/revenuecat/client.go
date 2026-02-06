package revenuecat

import (
	"crypto/hmac"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Common errors
var (
	ErrInvalidSignature    = errors.New("invalid_revenuecat_webhook_signature")
	ErrInvalidPayload      = errors.New("invalid_revenuecat_webhook_payload")
	ErrMissingAppUserID    = errors.New("missing_app_user_id")
	ErrUnknownProductID    = errors.New("unknown_product_id")
	ErrExpiredSubscription = errors.New("subscription_already_expired")
)

// EventType represents RevenueCat webhook event types
type EventType string

const (
	// Initial purchase events
	EventInitialPurchase       EventType = "INITIAL_PURCHASE"
	EventNonRenewingPurchase   EventType = "NON_RENEWING_PURCHASE"
	EventRenewal               EventType = "RENEWAL"
	EventProductChange         EventType = "PRODUCT_CHANGE"
	EventCancellation          EventType = "CANCELLATION"
	EventUncancellation        EventType = "UNCANCELLATION"
	EventBillingIssue          EventType = "BILLING_ISSUE"
	EventSubscriberAlias       EventType = "SUBSCRIBER_ALIAS"
	EventSubscriptionPaused    EventType = "SUBSCRIPTION_PAUSED"
	EventSubscriptionExtended  EventType = "SUBSCRIPTION_EXTENDED"
	EventExpiration            EventType = "EXPIRATION"
	EventTransfer              EventType = "TRANSFER"
	EventTest                  EventType = "TEST"
	EventTemporaryEntitlementGrant EventType = "TEMPORARY_ENTITLEMENT_GRANT"
)

// Store represents the app store
type Store string

const (
	StoreAppStore   Store = "APP_STORE"
	StorePlayStore  Store = "PLAY_STORE"
	StoreMacAppStore Store = "MAC_APP_STORE"
	StoreStripe     Store = "STRIPE"
	StoreAmazon     Store = "AMAZON"
	StorePromo      Store = "PROMOTIONAL"
)

// Environment represents the RevenueCat environment
type Environment string

const (
	EnvironmentProduction Environment = "PRODUCTION"
	EnvironmentSandbox    Environment = "SANDBOX"
)

// PeriodType represents the subscription period type
type PeriodType string

const (
	PeriodTypeNormal       PeriodType = "NORMAL"
	PeriodTypeTrial        PeriodType = "TRIAL"
	PeriodTypeIntro        PeriodType = "INTRO"
	PeriodTypePromotional  PeriodType = "PROMOTIONAL"
)

// WebhookEvent represents the top-level webhook payload from RevenueCat
type WebhookEvent struct {
	APIVersion string `json:"api_version"`
	Event      Event  `json:"event"`
}

// Event represents the event data in a webhook
type Event struct {
	// Event identification
	ID        string    `json:"id"`
	Type      EventType `json:"type"`
	Timestamp int64     `json:"event_timestamp_ms"`

	// App info
	AppID     string `json:"app_id"`
	AppUserID string `json:"app_user_id"` // This is our user.uuid

	// Aliases (alternative user IDs)
	Aliases []string `json:"aliases"`

	// Original app user ID (for transfers)
	OriginalAppUserID string `json:"original_app_user_id"`

	// Subscription info
	ProductID                 string      `json:"product_id"`
	EntitlementID             *string     `json:"entitlement_id"`
	EntitlementIDs            []string    `json:"entitlement_ids"`
	PresentedOfferingID       *string     `json:"presented_offering_id"`
	PeriodType                PeriodType  `json:"period_type"`
	PurchasedAtMs             int64       `json:"purchased_at_ms"`
	ExpirationAtMs            int64       `json:"expiration_at_ms"`
	GracePeriodExpirationAtMs int64       `json:"grace_period_expiration_at_ms"`
	Environment               Environment `json:"environment"`
	Store                     Store       `json:"store"`
	TransactionID             *string     `json:"transaction_id"`
	StoreTransactionID        *string     `json:"store_transaction_id"`
	OriginalTransactionID     *string     `json:"original_transaction_id"`

	// Pricing info
	Price                    *float64 `json:"price"`
	PriceInPurchasedCurrency *float64 `json:"price_in_purchased_currency"`
	Currency                 *string  `json:"currency"`
	TakehomePercentage       *float64 `json:"takehome_percentage"`
	TaxPercentage            *float64 `json:"tax_percentage"`
	CommissionPercentage     *float64 `json:"commission_percentage"`

	// Renewal/cancellation info
	IsTrialConversion *bool   `json:"is_trial_conversion"`
	IsFamilyShare     *bool   `json:"is_family_share"`
	CountryCode       string  `json:"country_code"`
	OfferCode         *string `json:"offer_code"`
	CancelReason      *string `json:"cancel_reason"`
	AutoResumeAt      int64   `json:"auto_resume_at_ms"`
	NewProductID      *string `json:"new_product_id"` // For PRODUCT_CHANGE
	RenewalNumber     *int    `json:"renewal_number"`

	// Subscriber attributes (custom data set via SDK)
	SubscriberAttributes map[string]SubscriberAttribute `json:"subscriber_attributes"`

	// Metadata (custom data)
	Metadata map[string]interface{} `json:"metadata"`
}

// SubscriberAttribute represents a custom attribute set on the subscriber
type SubscriberAttribute struct {
	Value     string `json:"value"`
	UpdatedAt int64  `json:"updated_at_ms"`
}

// Client is the RevenueCat webhook client
type Client struct {
	webhookSecret string
	productMappings map[string]ProductMapping // Maps RevenueCat product_id to internal plan code
}

// ProductMapping maps a RevenueCat product to internal plan
type ProductMapping struct {
	PlanCode     string
	BillingCycle string // "monthly" or "yearly"
}

// NewClient creates a new RevenueCat client
func NewClient(webhookSecret string) *Client {
	return &Client{
		webhookSecret: webhookSecret,
		productMappings: make(map[string]ProductMapping),
	}
}

// AddProductMapping adds a product ID to plan code mapping
func (c *Client) AddProductMapping(productID string, planCode, billingCycle string) {
	c.productMappings[productID] = ProductMapping{
		PlanCode:     planCode,
		BillingCycle: billingCycle,
	}
}

// GetPlanCode returns the plan code for a product ID
func (c *Client) GetPlanCode(productID string) (string, string, error) {
	mapping, ok := c.productMappings[productID]
	if !ok {
		return "", "", fmt.Errorf("%w: %s", ErrUnknownProductID, productID)
	}
	return mapping.PlanCode, mapping.BillingCycle, nil
}

// VerifyAuthorization verifies the webhook authorization from RevenueCat
// RevenueCat sends the configured secret in the Authorization header
// This is a simple string comparison, not HMAC
func (c *Client) VerifyAuthorization(authToken string) error {
	if c.webhookSecret == "" {
		// If no webhook secret is configured, skip verification (dev mode)
		return nil
	}

	// Simple string comparison using constant-time comparison
	if !hmac.Equal([]byte(authToken), []byte(c.webhookSecret)) {
		return ErrInvalidSignature
	}

	return nil
}

// ParseWebhook parses and verifies a webhook payload
func (c *Client) ParseWebhook(payload []byte, authToken string) (*WebhookEvent, error) {
	// Verify authorization first
	if err := c.VerifyAuthorization(authToken); err != nil {
		return nil, err
	}

	// Parse the webhook payload
	var event WebhookEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPayload, err)
	}

	return &event, nil
}

// GetAppUserID returns the app user ID from the event (user's UUID in our system)
func (e *Event) GetAppUserID() string {
	return e.AppUserID
}

// GetExpirationTime returns the expiration time as time.Time
func (e *Event) GetExpirationTime() *time.Time {
	if e.ExpirationAtMs == 0 {
		return nil
	}
	t := time.UnixMilli(e.ExpirationAtMs)
	return &t
}

// GetPurchaseTime returns the purchase time as time.Time
func (e *Event) GetPurchaseTime() *time.Time {
	if e.PurchasedAtMs == 0 {
		return nil
	}
	t := time.UnixMilli(e.PurchasedAtMs)
	return &t
}

// GetGracePeriodExpirationTime returns the grace period expiration time
func (e *Event) GetGracePeriodExpirationTime() *time.Time {
	if e.GracePeriodExpirationAtMs == 0 {
		return nil
	}
	t := time.UnixMilli(e.GracePeriodExpirationAtMs)
	return &t
}

// GetAutoResumeTime returns the auto-resume time for paused subscriptions
func (e *Event) GetAutoResumeTime() *time.Time {
	if e.AutoResumeAt == 0 {
		return nil
	}
	t := time.UnixMilli(e.AutoResumeAt)
	return &t
}

// GetEventTime returns the event timestamp as time.Time
func (e *Event) GetEventTime() time.Time {
	return time.UnixMilli(e.Timestamp)
}

// GetTransactionID returns the best available transaction ID
// Priority: original_transaction_id > transaction_id > event ID
func (e *Event) GetTransactionID() string {
	if e.OriginalTransactionID != nil && *e.OriginalTransactionID != "" {
		return *e.OriginalTransactionID
	}
	if e.TransactionID != nil && *e.TransactionID != "" {
		return *e.TransactionID
	}
	// Fallback to event ID if no transaction ID available
	return e.ID
}

// IsSandbox returns true if the event is from a sandbox environment
func (e *Event) IsSandbox() bool {
	return e.Environment == EnvironmentSandbox
}

// IsAppStore returns true if the purchase is from Apple App Store
func (e *Event) IsAppStore() bool {
	return e.Store == StoreAppStore || e.Store == StoreMacAppStore
}

// IsPlayStore returns true if the purchase is from Google Play Store
func (e *Event) IsPlayStore() bool {
	return e.Store == StorePlayStore
}

// IsSubscriptionEvent returns true if this is a subscription-related event
func (e *Event) IsSubscriptionEvent() bool {
	switch e.Type {
	case EventInitialPurchase, EventRenewal, EventProductChange,
		EventCancellation, EventUncancellation, EventBillingIssue,
		EventSubscriptionPaused, EventSubscriptionExtended, EventExpiration:
		return true
	default:
		return false
	}
}

// RequiresSubscriptionUpdate returns true if this event should update subscription state
func (e *Event) RequiresSubscriptionUpdate() bool {
	switch e.Type {
	case EventInitialPurchase, EventRenewal, EventProductChange,
		EventCancellation, EventUncancellation, EventBillingIssue, EventExpiration:
		return true
	default:
		return false
	}
}

// GetSubscriberAttributeValue returns the value of a subscriber attribute
func (e *Event) GetSubscriberAttributeValue(key string) (string, bool) {
	attr, ok := e.SubscriberAttributes[key]
	if !ok {
		return "", false
	}
	return attr.Value, true
}
