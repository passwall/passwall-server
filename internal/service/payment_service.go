package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/passwall/passwall-server/internal/config"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	stripeClient "github.com/passwall/passwall-server/pkg/stripe"
	"github.com/stripe/stripe-go/v81"
)

var ErrInvalidStripeWebhookSignature = errors.New("invalid_stripe_webhook_signature")

type paymentService struct {
	stripe              *stripeClient.Client
	orgRepo             repository.OrganizationRepository
	orgUserRepo         repository.OrganizationUserRepository
	subscriptionService SubscriptionService
	planRepo            interface {
		GetByCode(ctx context.Context, code string) (*domain.Plan, error)
	}
	activityLogger *ActivityLogger
	config         *config.Config
	logger         Logger
}

// #region agent log
func agentDebugLog(hypothesisId, location, message string, data map[string]any) {
	// NOTE: Do not log secrets/PII. This is debug-mode instrumentation.
	payload := map[string]any{
		"sessionId":   "debug-session",
		"runId":       "run1",
		"hypothesisId": hypothesisId,
		"location":    location,
		"message":     message,
		"data":        data,
		"timestamp":   time.Now().UnixMilli(),
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return
	}
	f, err := os.OpenFile("/Users/yakuter/projects/passwall/passwall-extension/.cursor/debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	_, _ = f.Write(append(b, '\n'))
	_ = f.Close()
}
// #endregion agent log

// NewPaymentService creates a new payment service
func NewPaymentService(
	stripe *stripeClient.Client,
	orgRepo repository.OrganizationRepository,
	orgUserRepo repository.OrganizationUserRepository,
	subscriptionService SubscriptionService,
	planRepo interface {
		GetByCode(ctx context.Context, code string) (*domain.Plan, error)
	},
	activityService UserActivityService,
	config *config.Config,
	logger Logger,
) PaymentService {
	return &paymentService{
		stripe:              stripe,
		orgRepo:             orgRepo,
		orgUserRepo:         orgUserRepo,
		subscriptionService: subscriptionService,
		planRepo:            planRepo,
		activityLogger:      NewActivityLogger(activityService),
		config:              config,
		logger:              logger,
	}
}

// CreateCheckoutSession creates a Stripe checkout session for an organization
func (s *paymentService) CreateCheckoutSession(ctx context.Context, orgID, userID uint, plan, billingCycle string, seats int, ipAddress, userAgent string) (string, error) {
	// #region agent log
	agentDebugLog("A", "payment_service.go:CreateCheckoutSession:entry", "create checkout session", map[string]any{
		"orgId":        orgID,
		"plan":         plan,
		"billingCycle": billingCycle,
		"seats":        seats,
	})
	// #endregion agent log

	// Get organization
	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		return "", fmt.Errorf("organization not found: %w", err)
	}

	// Enforce minimum seats: cannot buy fewer seats than current members.
	// This prevents under-purchasing and immediate lockouts.
	if seats < 1 {
		seats = 1
	}
	if members, err := s.orgUserRepo.ListByOrganization(ctx, orgID); err == nil {
		if len(members) > seats {
			s.logger.Info("adjusting seats to current member count",
				"org_id", orgID,
				"requested_seats", seats,
				"member_count", len(members),
			)
			seats = len(members)
		}
	} else {
		s.logger.Warn("failed to get member count for seat enforcement", "org_id", orgID, "error", err)
	}

	// Validate plan and get price ID
	priceID, err := s.getPriceID(plan, billingCycle)
	if err != nil {
		return "", err
	}
	// #region agent log
	agentDebugLog("C", "payment_service.go:CreateCheckoutSession:price", "resolved price id", map[string]any{
		"orgId":   orgID,
		"priceId": priceID,
	})
	// #endregion agent log

	// Get or create Stripe customer
	customerID := ""
	if org.StripeCustomerID != nil && *org.StripeCustomerID != "" {
		customerID = *org.StripeCustomerID

		// Verify customer exists
		_, err := s.stripe.GetCustomer(customerID)
		if err != nil {
			s.logger.Warn("stripe customer not found, creating new one", "org_id", orgID, "old_customer_id", customerID)
			customerID = "" // Force create new customer
		}
	}

	if customerID == "" {
		// Create new Stripe customer
		customer, err := s.stripe.CreateCustomer(stripeClient.CreateCustomerParams{
			Email:        org.BillingEmail,
			Name:         org.Name,
			OrgID:        fmt.Sprintf("%d", orgID),
			BillingEmail: org.BillingEmail,
		})
		if err != nil {
			return "", fmt.Errorf("failed to create Stripe customer: %w", err)
		}
		customerID = customer.ID

		// Update organization with customer ID
		org.StripeCustomerID = &customerID
		if err := s.orgRepo.Update(ctx, org); err != nil {
			s.logger.Error("failed to save stripe customer ID", "error", err)
			// Don't fail - customer is created in Stripe
		}
	}

	// Create checkout session
	successURL := fmt.Sprintf("%s/organizations/%d/billing?success=true", s.config.Server.FrontendURL, orgID)
	cancelURL := fmt.Sprintf("%s/organizations/%d/billing?canceled=true", s.config.Server.FrontendURL, orgID)

	quantity := int64(seats)
	if quantity <= 0 {
		quantity = 1
	}

	session, err := s.stripe.CreateCheckoutSession(stripeClient.CheckoutSessionParams{
		CustomerID:   customerID,
		PriceID:      priceID,
		Quantity:     quantity,
		SuccessURL:   successURL,
		CancelURL:    cancelURL,
		OrgID:        fmt.Sprintf("%d", orgID),
		OrgName:      org.Name,
		Plan:         plan,
		BillingCycle: billingCycle,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create checkout session: %w", err)
	}
	// #region agent log
	agentDebugLog("A", "payment_service.go:CreateCheckoutSession:created", "checkout session created", map[string]any{
		"orgId":      orgID,
		"sessionId":  session.ID,
		"customerId": customerID,
	})
	// #endregion agent log

	s.logger.Info("created checkout session", "org_id", orgID, "plan", plan, "billing_cycle", billingCycle, "session_id", session.ID)

	// Log activity
	s.activityLogger.LogCheckoutCreated(ctx, userID, ipAddress, userAgent, orgID, org.Name, plan, billingCycle, session.ID)

	return session.URL, nil
}

// UpdateSubscriptionSeats updates seat quantity (subscription item quantity) on Stripe.
// This is used when an org already has an active subscription and wants to add seats
// (e.g., 3 → 4). Stripe will prorate as configured.
func (s *paymentService) UpdateSubscriptionSeats(ctx context.Context, orgID, userID uint, seats int, ipAddress, userAgent string) error {
	if seats <= 0 {
		return fmt.Errorf("invalid seats")
	}

	// Basic authorization: require org membership and owner/admin role.
	// (Payment routes are sensitive; keep this check here until a dedicated middleware exists.)
	members, err := s.orgUserRepo.ListByOrganization(ctx, orgID)
	if err != nil {
		return fmt.Errorf("failed to load organization members: %w", err)
	}
	var role domain.OrganizationRole
	for _, m := range members {
		if m.UserID == userID {
			role = m.Role
			break
		}
	}
	if role == "" {
		return fmt.Errorf("forbidden")
	}
	if role != domain.OrgRoleOwner && role != domain.OrgRoleAdmin {
		return fmt.Errorf("forbidden")
	}

	// Enforce minimum seats: cannot set fewer seats than current members.
	if len(members) > seats {
		seats = len(members)
	}

	sub, err := s.subscriptionService.GetByOrganizationID(ctx, orgID)
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}
	if sub == nil || sub.Plan == nil {
		return fmt.Errorf("subscription not found")
	}
	if sub.StripeSubscriptionID == nil || *sub.StripeSubscriptionID == "" {
		return fmt.Errorf("subscription has no Stripe subscription ID")
	}

	// Only allow seat changes for seat-based plans (family/team/business).
	// Premium/free are not seat-based in our pricing model.
	base := strings.Split(sub.Plan.Code, "-")[0]
	if base != "family" && base != "team" && base != "business" {
		return fmt.Errorf("plan does not support seat changes")
	}

	// Enforce plan caps (e.g., Family max 6, Team max 10). Business is unlimited (nil).
	if sub.Plan.MaxUsers != nil && seats > *sub.Plan.MaxUsers {
		return fmt.Errorf("requested seats exceed plan limit")
	}

	// No-op if seats are unchanged (or already higher in DB).
	if sub.SeatsPurchased != nil && seats == *sub.SeatsPurchased {
		return nil
	}

	// Update Stripe subscription item quantity (prorations apply).
	if _, err := s.stripe.UpdateSubscriptionQuantity(*sub.StripeSubscriptionID, int64(seats)); err != nil {
		return err
	}

	// Webhook will sync seats_purchased into DB. Optionally trigger a manual sync path if needed.
	s.logger.Info("updated subscription seats", "org_id", orgID, "seats", seats)
	return nil
}

// HandleWebhook handles Stripe webhook events
func (s *paymentService) HandleWebhook(ctx context.Context, payload []byte, signature string) error {
	s.logger.Info("🔔 Stripe webhook received", "payload_size", len(payload))
	// #region agent log
	agentDebugLog("A", "payment_service.go:HandleWebhook:entry", "webhook received", map[string]any{
		"payloadSize":    len(payload),
		"hasSignature":   signature != "",
		"signatureBytes": len(signature),
	})
	// #endregion agent log

	// Verify webhook signature
	event, err := s.stripe.ConstructWebhookEvent(payload, signature)
	if err != nil {
		s.logger.Error("❌ Webhook signature verification failed", "error", err)
		// #region agent log
		agentDebugLog("B", "payment_service.go:HandleWebhook:verify_failed", "webhook signature verification failed", map[string]any{
			"error": err.Error(),
		})
		// #endregion agent log
		return fmt.Errorf("%w: %v", ErrInvalidStripeWebhookSignature, err)
	}

	s.logger.Info("✅ Webhook signature verified", "event_type", event.Type, "event_id", event.ID)
	// #region agent log
	agentDebugLog("A", "payment_service.go:HandleWebhook:verified", "webhook signature verified", map[string]any{
		"eventType": event.Type,
		"eventId":   event.ID,
	})
	// #endregion agent log

	// Handle different event types
	var handlerErr error
	switch event.Type {
	case "checkout.session.completed":
		s.logger.Info("🛒 Processing checkout.session.completed webhook", "event_id", event.ID)
		handlerErr = s.handleCheckoutCompleted(ctx, event)
	case "customer.subscription.created":
		s.logger.Info("➕ Processing customer.subscription.created webhook", "event_id", event.ID)
		handlerErr = s.handleSubscriptionCreated(ctx, event)
	case "customer.subscription.updated":
		s.logger.Info("🔄 Processing customer.subscription.updated webhook", "event_id", event.ID)
		handlerErr = s.handleSubscriptionUpdated(ctx, event)
	case "customer.subscription.deleted":
		s.logger.Info("🗑️  Processing customer.subscription.deleted webhook", "event_id", event.ID)
		handlerErr = s.handleSubscriptionDeleted(ctx, event)
	case "invoice.payment_succeeded":
		s.logger.Info("💰 Processing invoice.payment_succeeded webhook", "event_id", event.ID)
		handlerErr = s.handlePaymentSucceeded(ctx, event)
	case "invoice.payment_failed":
		s.logger.Info("⚠️  Processing invoice.payment_failed webhook", "event_id", event.ID)
		handlerErr = s.handlePaymentFailed(ctx, event)
	default:
		s.logger.Info("ℹ️  Unhandled webhook event type (ignored)", "event_type", event.Type, "event_id", event.ID)
		return nil // Ignore unhandled events
	}

	if handlerErr != nil {
		s.logger.Error("❌ Webhook handler failed", "event_type", event.Type, "event_id", event.ID, "error", handlerErr)
		return handlerErr
	}

	s.logger.Info("✅ Webhook processed successfully", "event_type", event.Type, "event_id", event.ID)
	return nil
}

// handleCheckoutCompleted handles checkout.session.completed event
func (s *paymentService) handleCheckoutCompleted(ctx context.Context, event stripe.Event) error {
	var session stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
		s.logger.Error("Failed to parse checkout session from webhook", "error", err)
		return fmt.Errorf("failed to parse checkout session: %w", err)
	}

	// Get organization ID from metadata
	orgIDStr := session.Metadata["organization_id"]
	if orgIDStr == "" {
		s.logger.Error("Organization ID not found in checkout session metadata", "session_id", session.ID)
		return fmt.Errorf("organization_id not found in metadata")
	}

	var orgID uint
	fmt.Sscanf(orgIDStr, "%d", &orgID)

	s.logger.Info("Processing checkout for organization", "org_id", orgID, "session_id", session.ID)

	// Get organization
	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		s.logger.Error("Organization not found", "org_id", orgID, "error", err)
		return fmt.Errorf("organization not found: %w", err)
	}

	// Update organization with subscription info
	customerID := session.Customer.ID
	org.StripeCustomerID = &customerID

	if err := s.orgRepo.Update(ctx, org); err != nil {
		s.logger.Error("Failed to update organization with Stripe customer ID", "org_id", orgID, "customer_id", customerID, "error", err)
		return fmt.Errorf("failed to update organization: %w", err)
	}

	s.logger.Info("✅ Checkout completed successfully", "org_id", orgID, "org_name", org.Name, "customer_id", customerID)

	return nil
}

// handleSubscriptionCreated handles customer.subscription.created event
func (s *paymentService) handleSubscriptionCreated(ctx context.Context, event stripe.Event) error {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		s.logger.Error("Failed to parse subscription from webhook", "error", err)
		return fmt.Errorf("failed to parse subscription: %w", err)
	}

	s.logger.Info("Creating new subscription", "subscription_id", sub.ID, "customer_id", sub.Customer.ID, "status", sub.Status)

	// Get organization ID from metadata
	orgIDStr := sub.Metadata["organization_id"]
	if orgIDStr == "" {
		s.logger.Error("Organization ID not found in subscription metadata", "subscription_id", sub.ID)
		// #region agent log
		agentDebugLog("D", "payment_service.go:handleSubscriptionCreated:missing_org", "missing organization_id metadata", map[string]any{
			"subscriptionId": sub.ID,
		})
		// #endregion agent log
		return fmt.Errorf("organization_id not found in metadata")
	}

	var orgID uint
	fmt.Sscanf(orgIDStr, "%d", &orgID)

	// Get plan from price ID
	priceID := stripeClient.GetPriceFromSubscription(&sub)
	planCode, _ := s.mapPriceIDToPlan(priceID)
	if planCode == "" {
		s.logger.Error("Unknown price ID in subscription", "price_id", priceID, "subscription_id", sub.ID)
		// #region agent log
		agentDebugLog("C", "payment_service.go:handleSubscriptionCreated:unknown_price", "unknown price id in subscription", map[string]any{
			"subscriptionId": sub.ID,
			"priceId":        priceID,
		})
		// #endregion agent log
		return fmt.Errorf("unknown price ID: %s", priceID)
	}
	// #region agent log
	agentDebugLog("A", "payment_service.go:handleSubscriptionCreated:mapped", "mapped subscription to plan", map[string]any{
		"subscriptionId": sub.ID,
		"orgId":          orgID,
		"priceId":        priceID,
		"planCode":       planCode,
	})
	// #endregion agent log

	// Seat-based: derive purchased seats from Stripe subscription item quantity (licensed pricing)
	var seatsPurchased *int
	if len(sub.Items.Data) > 0 {
		q := int(sub.Items.Data[0].Quantity)
		if q > 0 {
			seatsPurchased = &q
		}
	}

	s.logger.Info("Creating subscription in database", "org_id", orgID, "plan_code", planCode, "subscription_id", sub.ID)

	// Create subscription using SubscriptionService
	subscription, err := s.subscriptionService.Create(ctx, orgID, planCode, sub.ID, seatsPurchased)
	if err != nil {
		s.logger.Error("Failed to create subscription in database", "org_id", orgID, "subscription_id", sub.ID, "error", err)
		return fmt.Errorf("failed to create subscription: %w", err)
	}

	// Note: We don't update organizations table anymore
	// Plan limits are fetched from subscriptions + plans tables via JOIN

	s.logger.Info("✅ Subscription created successfully", "org_id", orgID, "subscription_id", subscription.ID, "status", sub.Status)
	return nil
}

// handleSubscriptionUpdated handles customer.subscription.updated event
func (s *paymentService) handleSubscriptionUpdated(ctx context.Context, event stripe.Event) error {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		s.logger.Error("Failed to parse subscription from webhook", "error", err)
		return fmt.Errorf("failed to parse subscription: %w", err)
	}

	s.logger.Info("Updating subscription", "subscription_id", sub.ID, "customer_id", sub.Customer.ID, "status", sub.Status)

	// Sync seats (quantity) on every subscription update
	var seatsPurchased *int
	if len(sub.Items.Data) > 0 {
		q := int(sub.Items.Data[0].Quantity)
		if q > 0 {
			seatsPurchased = &q
		}
	}
	if err := s.subscriptionService.UpdateSeatsPurchasedByStripeSubscriptionID(ctx, sub.ID, seatsPurchased); err != nil {
		s.logger.Warn("failed to sync seats purchased", "subscription_id", sub.ID, "error", err)
		// Don't fail the whole webhook for a non-critical seat sync (Stripe will retry on real failures anyway)
	}

	// Handle payment success/failure through SubscriptionService
	if sub.Status == stripe.SubscriptionStatusActive || sub.Status == stripe.SubscriptionStatusTrialing {
		err := s.subscriptionService.HandlePaymentSuccess(ctx, sub.ID)
		if err != nil {
			s.logger.Error("Failed to handle payment success", "subscription_id", sub.ID, "error", err)
			return fmt.Errorf("failed to handle payment success: %w", err)
		}
	} else if sub.Status == stripe.SubscriptionStatusPastDue {
		err := s.subscriptionService.HandlePaymentFailed(ctx, sub.ID)
		if err != nil {
			s.logger.Error("Failed to handle payment failure", "subscription_id", sub.ID, "error", err)
			return fmt.Errorf("failed to handle payment failure: %w", err)
		}
	}

	s.logger.Info("✅ Subscription updated successfully", "subscription_id", sub.ID, "status", sub.Status)
	return nil
}

// handleSubscriptionDeleted handles customer.subscription.deleted event
func (s *paymentService) handleSubscriptionDeleted(ctx context.Context, event stripe.Event) error {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		s.logger.Error("Failed to parse subscription from webhook", "error", err)
		return fmt.Errorf("failed to parse subscription: %w", err)
	}

	s.logger.Info("🗑️  Subscription deleted", "subscription_id", sub.ID, "customer_id", sub.Customer.ID, "status", sub.Status)

	// Subscription deletion is now handled by SubscriptionService
	// This webhook is logged for audit purposes
	return nil
}

// handlePaymentSucceeded handles invoice.payment_succeeded event
func (s *paymentService) handlePaymentSucceeded(ctx context.Context, event stripe.Event) error {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		s.logger.Error("Failed to parse invoice from webhook", "error", err)
		return fmt.Errorf("failed to parse invoice: %w", err)
	}

	if invoice.Subscription == nil {
		s.logger.Info("ℹ️  Payment succeeded for non-subscription invoice (skipped)", "invoice_id", invoice.ID)
		return nil // Not a subscription invoice
	}

	amount := float64(invoice.AmountPaid) / 100.0
	currency := string(invoice.Currency)
	s.logger.Info("💰 Payment succeeded", "invoice_id", invoice.ID, "subscription_id", invoice.Subscription.ID,
		"amount", amount, "currency", currency, "customer_id", invoice.Customer.ID)

	// Fetch full subscription data and update organization
	sub, err := s.stripe.GetSubscription(invoice.Subscription.ID)
	if err != nil {
		s.logger.Error("Failed to get subscription after payment", "subscription_id", invoice.Subscription.ID, "error", err)
		return nil // Don't fail - invoice is paid
	}

	err = s.updateOrgFromSubscription(ctx, sub)
	if err != nil {
		s.logger.Error("Failed to update organization after payment", "subscription_id", sub.ID, "error", err)
		return err
	}

	s.logger.Info("✅ Payment processed and organization updated", "invoice_id", invoice.ID, "subscription_id", sub.ID)
	return nil
}

// handlePaymentFailed handles invoice.payment_failed event
func (s *paymentService) handlePaymentFailed(ctx context.Context, event stripe.Event) error {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		s.logger.Error("Failed to parse invoice from webhook", "error", err)
		return fmt.Errorf("failed to parse invoice: %w", err)
	}

	amount := float64(invoice.AmountDue) / 100.0
	currency := string(invoice.Currency)
	s.logger.Warn("⚠️  Payment failed", "invoice_id", invoice.ID, "customer_id", invoice.Customer.ID,
		"amount", amount, "currency", currency, "attempt_count", invoice.AttemptCount)

	// Payment failure handling is managed by SubscriptionService
	// This webhook is logged for audit purposes

	return nil
}

// updateOrgFromSubscription updates organization from Stripe subscription
func (s *paymentService) updateOrgFromSubscription(ctx context.Context, sub *stripe.Subscription) error {
	// Get organization ID from metadata
	orgIDStr := sub.Metadata["organization_id"]
	if orgIDStr == "" {
		s.logger.Warn("⚠️  Organization ID not found in subscription metadata (skipping update)", "subscription_id", sub.ID)
		return nil
	}

	var orgID uint
	fmt.Sscanf(orgIDStr, "%d", &orgID)

	s.logger.Info("Updating organization from subscription data", "org_id", orgID, "subscription_id", sub.ID)

	err := s.updateOrgFromSubscriptionWithID(ctx, sub, orgID)
	if err != nil {
		s.logger.Error("Failed to update organization from subscription", "org_id", orgID, "subscription_id", sub.ID, "error", err)
		return err
	}

	s.logger.Info("✅ Organization updated from subscription", "org_id", orgID, "subscription_id", sub.ID)
	return nil
}

// updateOrgFromSubscriptionWithID updates organization from subscription with explicit orgID
func (s *paymentService) updateOrgFromSubscriptionWithID(ctx context.Context, sub *stripe.Subscription, orgID uint) error {
	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		return fmt.Errorf("organization not found: %w", err)
	}

	// Map subscription to organization plan
	priceID := stripeClient.GetPriceFromSubscription(sub)
	planCode, billingCycle := s.mapPriceIDToPlan(priceID)
	if planCode == "" {
		return fmt.Errorf("unknown price ID: %s", priceID)
	}

	// Persist subscription in DB (critical for "paid but still free" cases when webhooks are not delivered).
	// Seat-based: derive purchased seats from Stripe subscription item quantity (licensed pricing)
	var seatsPurchased *int
	if len(sub.Items.Data) > 0 {
		q := int(sub.Items.Data[0].Quantity)
		if q > 0 {
			seatsPurchased = &q
		}
	}
	// #region agent log
	agentDebugLog("F", "payment_service.go:updateOrgFromSubscriptionWithID:dbUpsert:attempt", "upserting subscription from Stripe subscription", map[string]any{
		"orgId":        orgID,
		"planCode":     planCode,
		"stripeSubId":  sub.ID,
		"status":       string(sub.Status),
		"seatsPresent": seatsPurchased != nil,
	})
	// #endregion agent log
	if _, err := s.subscriptionService.Create(ctx, orgID, planCode, sub.ID, seatsPurchased); err != nil {
		// #region agent log
		agentDebugLog("F", "payment_service.go:updateOrgFromSubscriptionWithID:dbUpsert:failed", "failed to upsert subscription", map[string]any{
			"orgId":       orgID,
			"planCode":    planCode,
			"stripeSubId": sub.ID,
		})
		// #endregion agent log
		return fmt.Errorf("failed to upsert subscription: %w", err)
	}
	// #region agent log
	agentDebugLog("F", "payment_service.go:updateOrgFromSubscriptionWithID:dbUpsert:ok", "upserted subscription", map[string]any{
		"orgId":       orgID,
		"planCode":    planCode,
		"stripeSubId": sub.ID,
	})
	// #endregion agent log

	// Update organization (only Stripe customer ID)
	// Note: Plan limits come from subscriptions table, not organizations table
	subscriptionID := sub.ID
	status := string(sub.Status)
	customerID := sub.Customer.ID
	org.StripeCustomerID = &customerID

	if err := s.orgRepo.Update(ctx, org); err != nil {
		return fmt.Errorf("failed to update organization: %w", err)
	}

	s.logger.Info("updated organization from subscription",
		"org_id", orgID,
		"plan", planCode,
		"status", status,
		"billing_cycle", billingCycle,
	)

	// Log activity (find organization owner for activity logging)
	ownerID := s.getOrganizationOwnerID(ctx, orgID)
	if ownerID > 0 {
		if status == "active" && !strings.HasPrefix(planCode, "free") {
			// This is an upgrade
			s.activityLogger.LogOrganizationUpgraded(ctx, ownerID, "webhook", "Stripe Webhook",
				orgID, org.Name, subscriptionID, "", planCode, string(billingCycle), status)
		} else {
			// General subscription update
			s.activityLogger.LogSubscriptionUpdated(ctx, ownerID, "webhook", "Stripe Webhook",
				orgID, org.Name, subscriptionID, planCode, string(billingCycle), status)
		}
	}

	return nil
}

// getOrganizationOwnerID returns the first owner user ID for an organization
func (s *paymentService) getOrganizationOwnerID(ctx context.Context, orgID uint) uint {
	members, err := s.orgUserRepo.ListByOrganization(ctx, orgID)
	if err != nil {
		s.logger.Error("failed to get organization members for activity logging", "org_id", orgID, "error", err)
		return 0
	}

	for _, member := range members {
		if member.Role == domain.OrgRoleOwner {
			return member.UserID
		}
	}

	s.logger.Warn("no owner found for organization", "org_id", orgID)
	return 0
}

// GetBillingInfo retrieves billing information for an organization
func (s *paymentService) GetBillingInfo(ctx context.Context, orgID uint) (*domain.BillingInfo, error) {
	// Get organization
	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("organization not found: %w", err)
	}

	// Count current members
	members, err := s.orgUserRepo.ListByOrganization(ctx, orgID)
	if err != nil {
		s.logger.Error("failed to count members", "error", err)
		// Don't fail - return 0
	}

	// Collection count would require collection repository
	currentCollections := 0

	// Convert org to DTO (plan/limits derived from subscription when available)
	orgDTO := domain.ToOrganizationDTO(org)

	billingInfo := &domain.BillingInfo{
		Organization:       orgDTO,
		CurrentUsers:       len(members),
		CurrentCollections: currentCollections,
		CurrentItems:       0,
	}

	// Get subscription from database (with Plan preloaded)
	subscription, err := s.subscriptionService.GetByOrganizationID(ctx, orgID)
	if err != nil {
		s.logger.Warn("No subscription found in database", "org_id", orgID, "error", err)
	}

	hasStripeCustomer := org.StripeCustomerID != nil && *org.StripeCustomerID != ""
	subNil := subscription == nil
	subPlanNil := subscription == nil || subscription.Plan == nil
	subPlanCode := ""
	if subscription != nil && subscription.Plan != nil {
		subPlanCode = subscription.Plan.Code
	}

	// Auto-sync is a local-dev fallback when Stripe webhooks aren't delivered.
	// Keep it OFF by default (webhooks should be the primary mechanism).
	autoSyncEnv := strings.ToLower(strings.TrimSpace(os.Getenv("PW_STRIPE_BILLING_AUTOSYNC")))
	autoSyncEnabled := autoSyncEnv == "1" || autoSyncEnv == "true" || autoSyncEnv == "yes"
	shouldAutoSync := autoSyncEnabled && hasStripeCustomer && (err != nil || subNil || subPlanNil || strings.HasPrefix(subPlanCode, "free"))
	// #region agent log
	agentDebugLog("E", "payment_service.go:GetBillingInfo:autoSync:decision", "billing autosync decision", map[string]any{
		"orgId":            orgID,
		"hasStripeCustomer": hasStripeCustomer,
		"subNil":           subNil,
		"subPlanNil":       subPlanNil,
		"subPlanCode":      subPlanCode,
		"autoSyncEnabled":  autoSyncEnabled,
		"shouldAutoSync":   shouldAutoSync,
	})
	// #endregion agent log

	// If webhooks are not delivered (common in local dev), try a best-effort sync from Stripe
	// when we don't have a meaningful subscription in DB. This prevents "paid but still free" UX after redirect.
	if shouldAutoSync {
		// #region agent log
		agentDebugLog("E", "payment_service.go:GetBillingInfo:autoSync:attempt", "attempting Stripe sync", map[string]any{
			"orgId": orgID,
		})
		// #endregion agent log

		if syncErr := s.SyncSubscription(ctx, orgID); syncErr != nil {
			s.logger.Warn("auto sync subscription failed", "org_id", orgID, "error", syncErr)
			// #region agent log
			agentDebugLog("E", "payment_service.go:GetBillingInfo:autoSync:failed", "Stripe sync failed", map[string]any{
				"orgId": orgID,
			})
			// #endregion agent log
			// Continue without subscription (don't fail billing endpoint)
			subscription = nil
		} else {
			// Re-fetch subscription after successful sync.
			subscription, err = s.subscriptionService.GetByOrganizationID(ctx, orgID)
			if err != nil || subscription == nil || subscription.Plan == nil {
				s.logger.Warn("auto sync succeeded but subscription still missing", "org_id", orgID, "error", err)
				// #region agent log
				agentDebugLog("E", "payment_service.go:GetBillingInfo:autoSync:missing", "sync ok but subscription still missing", map[string]any{
					"orgId": orgID,
				})
				// #endregion agent log
				subscription = nil
			} else {
				// #region agent log
				agentDebugLog("E", "payment_service.go:GetBillingInfo:autoSync:ok", "Stripe sync applied; returning subscription", map[string]any{
					"orgId":       orgID,
					"subPlanCode": subscription.Plan.Code,
				})
				// #endregion agent log
			}
		}
	}

	if subscription == nil {
		// Return billing info without subscription
		return billingInfo, nil
	}

	// Single source of truth: derive org plan/limits from subscription+plan.
	orgDTO = domain.ToOrganizationDTOWithSubscription(org, subscription)
	billingInfo.Organization = orgDTO
	billingInfo.Subscription = domain.ToSubscriptionDTO(subscription)

	// Fetch invoices from Stripe if organization has Stripe customer ID
	if org.StripeCustomerID != nil && *org.StripeCustomerID != "" {
		stripeInvoices, err := s.stripe.ListInvoices(*org.StripeCustomerID, 10)
		if err != nil {
			s.logger.Warn("Failed to fetch invoices from Stripe", "org_id", orgID, "customer_id", *org.StripeCustomerID, "error", err)
			// Don't fail - return billing info without invoices
			return billingInfo, nil
		}

		// Convert Stripe invoices to domain InvoiceDTOs
		if len(stripeInvoices) > 0 {
			invoiceDTOs := make([]*domain.InvoiceDTO, 0, len(stripeInvoices))
			for _, inv := range stripeInvoices {
				// Convert status to domain InvoiceStatus
				status := domain.InvoiceStatus(inv.Status)

				// Convert timestamps to time.Time
				issuedAt := time.Unix(inv.Created, 0)

				// Format amount for display
				amountDisplay := fmt.Sprintf("$%.2f", float64(inv.AmountPaid)/100)

				invoiceDTO := &domain.InvoiceDTO{
					Status:        status,
					AmountCents:   int(inv.AmountPaid),
					AmountDisplay: amountDisplay,
					Currency:      string(inv.Currency),
					IssuedAt:      issuedAt,
				}

				// Add optional fields
				if inv.ID != "" {
					invoiceDTO.StripeInvoiceID = &inv.ID
				}
				if inv.InvoicePDF != "" {
					invoiceDTO.InvoicePDFURL = &inv.InvoicePDF
				}
				if inv.HostedInvoiceURL != "" {
					invoiceDTO.HostedInvoiceURL = &inv.HostedInvoiceURL
				}
				if inv.StatusTransitions != nil && inv.StatusTransitions.PaidAt > 0 {
					paidAt := time.Unix(inv.StatusTransitions.PaidAt, 0)
					invoiceDTO.PaidAt = &paidAt
				}

				invoiceDTOs = append(invoiceDTOs, invoiceDTO)
			}
			billingInfo.Invoices = invoiceDTOs
			s.logger.Info("📄 Fetched invoices from Stripe", "org_id", orgID, "count", len(invoiceDTOs))
		}
	}

	return billingInfo, nil
}

// SyncSubscription manually syncs subscription data from Stripe
func (s *paymentService) SyncSubscription(ctx context.Context, orgID uint) error {
	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		return fmt.Errorf("organization not found: %w", err)
	}

	if org.StripeCustomerID == nil || *org.StripeCustomerID == "" {
		return fmt.Errorf("no Stripe customer ID found")
	}

	// List all subscriptions for this customer
	subscriptions, err := s.stripe.ListCustomerSubscriptions(*org.StripeCustomerID)
	if err != nil {
		return fmt.Errorf("failed to list subscriptions: %w", err)
	}

	if len(subscriptions) == 0 {
		return fmt.Errorf("no subscriptions found for customer")
	}

	// Use the first active subscription
	var activeSub *stripe.Subscription
	for _, sub := range subscriptions {
		if sub.Status == stripe.SubscriptionStatusActive || sub.Status == stripe.SubscriptionStatusTrialing {
			activeSub = sub
			break
		}
	}

	if activeSub == nil {
		// No active subscription, check for canceled/past_due
		if len(subscriptions) > 0 {
			activeSub = subscriptions[0] // Use the most recent one
		} else {
			return fmt.Errorf("no active subscription found")
		}
	}

	// Update organization from subscription
	// For manual sync, we pass the orgID directly instead of relying on metadata
	if err := s.updateOrgFromSubscriptionWithID(ctx, activeSub, orgID); err != nil {
		return fmt.Errorf("failed to update organization: %w", err)
	}

	s.logger.Info("manually synced subscription", "org_id", orgID, "subscription_id", activeSub.ID, "status", activeSub.Status)

	return nil
}

// getPriceID returns the Stripe price ID for a plan and billing cycle
func (s *paymentService) getPriceID(plan, billingCycle string) (string, error) {
	// Get price ID from config
	planCode := fmt.Sprintf("%s-%s", plan, billingCycle)

	for _, configPlan := range s.config.Stripe.Plans {
		if configPlan.Code == planCode {
			if configPlan.StripePriceID == "" {
				return "", fmt.Errorf("stripe price ID not configured for plan: %s", planCode)
			}
			return configPlan.StripePriceID, nil
		}
	}

	return "", fmt.Errorf("plan not found in config: %s (billing cycle: %s)", plan, billingCycle)
}

// mapPriceIDToPlan maps a Stripe price ID to plan name and billing cycle
func (s *paymentService) mapPriceIDToPlan(priceID string) (string, domain.BillingCycle) {
	cfg := s.config.Stripe

	for _, plan := range cfg.Plans {
		if plan.StripePriceID == priceID {
			return plan.Code, domain.BillingCycle(plan.BillingCycle)
		}
	}

	// Fallback for unknown price IDs
	{
		return "", ""
	}
}
