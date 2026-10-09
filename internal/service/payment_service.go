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
	"github.com/passwall/passwall-server/internal/email"
	"github.com/passwall/passwall-server/internal/repository"
	stripeClient "github.com/passwall/passwall-server/pkg/stripe"
	"github.com/stripe/stripe-go/v81"
	"gorm.io/gorm"
)

var ErrInvalidStripeWebhookSignature = errors.New("invalid_stripe_webhook_signature")

type paymentService struct {
	stripe              *stripeClient.Client
	orgRepo             repository.OrganizationRepository
	orgUserRepo         repository.OrganizationUserRepository
	userRepo            repository.UserRepository
	subscriptionService SubscriptionService
	webhookEventRepo    repository.WebhookEventRepository
	planRepo            interface {
		GetByCode(ctx context.Context, code string) (*domain.Plan, error)
	}
	activityLogger *ActivityLogger
	config         *config.Config
	logger         Logger
	trialHistory   trialHistoryReader
	trialEmails    *trialEmailNotifier
}

type trialHistoryReader interface {
	HasPaidHistoryForUser(ctx context.Context, userID uint, personal bool) (bool, error)
}

type trialEmailNotifier struct {
	sender  email.Sender
	builder *email.EmailBuilder
}

// PaymentServiceOption configures optional payment service collaborators.
type PaymentServiceOption func(*paymentService)

// WithTrialHistory limits checkout trials to users without prior paid or trial
// history for the organization kind. Without it every checkout receives the plan trial.
func WithTrialHistory(reader trialHistoryReader) PaymentServiceOption {
	return func(s *paymentService) {
		s.trialHistory = reader
	}
}

// WithTrialEndingEmails sends the trial-ending reminder for Stripe trial_will_end events.
func WithTrialEndingEmails(sender email.Sender, builder *email.EmailBuilder) PaymentServiceOption {
	return func(s *paymentService) {
		if sender != nil && builder != nil {
			s.trialEmails = &trialEmailNotifier{sender: sender, builder: builder}
		}
	}
}

// NewPaymentService creates a new payment service
func NewPaymentService(
	stripe *stripeClient.Client,
	orgRepo repository.OrganizationRepository,
	orgUserRepo repository.OrganizationUserRepository,
	userRepo repository.UserRepository,
	subscriptionService SubscriptionService,
	webhookEventRepo repository.WebhookEventRepository,
	planRepo interface {
		GetByCode(ctx context.Context, code string) (*domain.Plan, error)
	},
	activityService UserActivityService,
	config *config.Config,
	logger Logger,
	opts ...PaymentServiceOption,
) PaymentService {
	service := &paymentService{
		stripe:              stripe,
		orgRepo:             orgRepo,
		orgUserRepo:         orgUserRepo,
		userRepo:            userRepo,
		subscriptionService: subscriptionService,
		webhookEventRepo:    webhookEventRepo,
		planRepo:            planRepo,
		activityLogger:      NewActivityLogger(activityService),
		config:              config,
		logger:              logger,
	}
	for _, opt := range opts {
		opt(service)
	}
	return service
}

// CreateCheckoutSession creates a Stripe checkout session for an organization
func (s *paymentService) CreateCheckoutSession(ctx context.Context, orgID, userID uint, plan, billingCycle string, seats int, ipAddress, userAgent string) (string, error) {
	// Get organization
	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		return "", fmt.Errorf("organization not found: %w", err)
	}

	if org.IsPersonal && domain.IsMultiUserPlan(plan) {
		return "", fmt.Errorf("personal vaults can only be upgraded to Pro; create a separate organization for Family, Team, or Business plans")
	}
	if !org.IsPersonal && domain.IsPersonalVaultPlan(plan) && plan != string(domain.PlanFree) {
		return "", fmt.Errorf("pro plan is only available for personal vaults")
	}

	// A plan-first organization reserved its seat count at setup; reuse it when
	// the client resumes checkout without an explicit seat count.
	currentSub, _ := s.subscriptionService.GetByOrganizationID(ctx, orgID)
	isPlanFirstSetup := currentSub != nil && currentSub.State == domain.SubStateDraft
	if seats < 1 && isPlanFirstSetup && currentSub.SeatsPurchased != nil {
		seats = *currentSub.SeatsPurchased
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

	// Validate plan and get plan config (includes price ID and trial days)
	planConfig, err := s.getPlanConfig(plan, billingCycle)
	if err != nil {
		return "", err
	}

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

	// Vault routes address organizations by public ID.
	billingURL := fmt.Sprintf("%s/organizations/%s/billing", s.config.Server.FrontendURL, org.PublicID)
	successURL := billingURL + "?success=true"
	if isPlanFirstSetup {
		successURL += "&onboarding=1"
	}
	cancelURL := billingURL + "?canceled=true"

	quantity := int64(seats)
	if quantity <= 0 {
		quantity = 1
	}

	trialDays := planConfig.TrialDays
	if trialDays > 0 && !s.isTrialEligible(ctx, org, userID) {
		trialDays = 0
	}

	session, err := s.stripe.CreateCheckoutSession(stripeClient.CheckoutSessionParams{
		CustomerID:   customerID,
		PriceID:      planConfig.StripePriceID,
		Quantity:     quantity,
		SuccessURL:   successURL,
		CancelURL:    cancelURL,
		OrgID:        fmt.Sprintf("%d", orgID),
		OrgName:      org.Name,
		Plan:         plan,
		BillingCycle: billingCycle,
		TrialDays:    trialDays,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create checkout session: %w", err)
	}

	s.logger.Info("created checkout session", "org_id", orgID, "plan", plan, "billing_cycle", billingCycle, "session_id", session.ID, "trial_days", trialDays)

	// Log activity
	s.activityLogger.LogCheckoutCreated(ctx, userID, ipAddress, userAgent, orgID, org.Name, plan, billingCycle, session.ID)

	return session.URL, nil
}

// isTrialEligible grants a checkout trial only to users who never had a trial
// or paid subscription for the same organization kind (personal or shared).
func (s *paymentService) isTrialEligible(ctx context.Context, org *domain.Organization, userID uint) bool {
	if s.trialHistory == nil {
		return true
	}
	ownerID := userID
	if org.CreatedByUserID != nil {
		ownerID = *org.CreatedByUserID
	} else if org.PersonalOwnerUserID != nil {
		ownerID = *org.PersonalOwnerUserID
	}
	used, err := s.trialHistory.HasPaidHistoryForUser(ctx, ownerID, org.IsPersonal)
	if err != nil {
		s.logger.Warn("trial history lookup failed; checkout continues without trial", "org_id", org.ID, "error", err)
		return false
	}
	return !used
}

// UpdateSubscriptionSeats updates seat quantity (subscription item quantity) on Stripe.
// This is used when an org already has an active subscription and wants to add seats
// (e.g., 3 → 4). Stripe will prorate as configured.
func (s *paymentService) PreviewSeatChange(ctx context.Context, orgID, userID uint, seats int) (*domain.SeatChangePreview, error) {
	if seats <= 0 {
		return nil, fmt.Errorf("invalid seats")
	}

	// Authorization
	members, err := s.orgUserRepo.ListByOrganization(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to load organization members: %w", err)
	}
	var role domain.OrganizationRole
	for _, m := range members {
		if m.UserID == userID {
			role = m.Role
			break
		}
	}
	if role == "" || (role != domain.OrgRoleOwner && role != domain.OrgRoleAdmin) {
		return nil, fmt.Errorf("forbidden")
	}

	sub, err := s.subscriptionService.GetByOrganizationID(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}
	if sub == nil || sub.Plan == nil || sub.StripeSubscriptionID == nil || *sub.StripeSubscriptionID == "" {
		return nil, fmt.Errorf("subscription not found or has no Stripe ID")
	}

	currentSeats := 0
	if sub.SeatsPurchased != nil {
		currentSeats = *sub.SeatsPurchased
	}

	inv, err := s.stripe.PreviewSeatChange(*sub.StripeSubscriptionID, int64(seats))
	if err != nil {
		return nil, err
	}

	nextBillingDate := ""
	if sub.RenewAt != nil {
		nextBillingDate = sub.RenewAt.Format("2006-01-02")
	}

	// Prorated amount is the difference between the upcoming total and the current subscription amount
	proratedAmount := int64(0)
	for _, line := range inv.Lines.Data {
		if line.Proration {
			proratedAmount += line.Amount
		}
	}

	return &domain.SeatChangePreview{
		CurrentSeats:      currentSeats,
		RequestedSeats:    seats,
		ProratedAmount:    proratedAmount,
		Currency:          string(inv.Currency),
		NextBillingDate:   nextBillingDate,
		NextBillingAmount: inv.Total,
	}, nil
}

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
	// Pro/free are not seat-based in our pricing model.
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

	// Sync DB immediately so the frontend gets fresh data on refetch.
	// The webhook will also fire, but this avoids the stale-read window.
	if err := s.subscriptionService.UpdateSeatsPurchasedByStripeSubscriptionID(ctx, *sub.StripeSubscriptionID, &seats); err != nil {
		s.logger.Error("failed to sync seats_purchased to DB (Stripe already updated)", "org_id", orgID, "seats", seats, "err", err)
		// Don't fail the request — Stripe is the source of truth and the webhook will catch up.
	}

	s.logger.Info("updated subscription seats", "org_id", orgID, "seats", seats)
	return nil
}

// PreviewPlanChange returns the prorated cost of switching to a different plan
// without actually applying the change. Only works for existing Stripe subscribers.
func (s *paymentService) PreviewPlanChange(ctx context.Context, orgID, userID uint, plan, billingCycle string, seats int) (*domain.PlanChangePreview, error) {
	if err := s.authorizeOwnerOrAdmin(ctx, orgID, userID); err != nil {
		return nil, err
	}

	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("organization not found: %w", err)
	}
	if org.IsPersonal && domain.IsMultiUserPlan(plan) {
		return nil, fmt.Errorf("personal vaults can only be upgraded to Pro; create a separate organization for Family, Team, or Business plans")
	}
	if !org.IsPersonal && domain.IsPersonalVaultPlan(plan) && plan != string(domain.PlanFree) {
		return nil, fmt.Errorf("pro plan is only available for personal vaults")
	}

	sub, err := s.subscriptionService.GetByOrganizationID(ctx, orgID)
	if err != nil || sub == nil || sub.StripeSubscriptionID == nil || *sub.StripeSubscriptionID == "" {
		return nil, fmt.Errorf("no active Stripe subscription found")
	}

	planConfig, err := s.getPlanConfig(plan, billingCycle)
	if err != nil {
		return nil, err
	}

	if seats < 1 {
		seats = 1
	}

	inv, err := s.stripe.PreviewPlanChange(*sub.StripeSubscriptionID, planConfig.StripePriceID, int64(seats))
	if err != nil {
		return nil, err
	}

	proratedAmount := int64(0)
	for _, line := range inv.Lines.Data {
		if line.Proration {
			proratedAmount += line.Amount
		}
	}

	currentPlanName := ""
	if sub.Plan != nil {
		currentPlanName = sub.Plan.Code
	}

	nextBillingDate := ""
	if sub.RenewAt != nil {
		nextBillingDate = sub.RenewAt.Format("2006-01-02")
	}

	return &domain.PlanChangePreview{
		CurrentPlan:       currentPlanName,
		NewPlan:           fmt.Sprintf("%s-%s", plan, billingCycle),
		ProratedAmount:    proratedAmount,
		Currency:          string(inv.Currency),
		NextBillingDate:   nextBillingDate,
		NextBillingAmount: inv.Total,
		ImmediateCharge:   proratedAmount > 0,
	}, nil
}

// ChangePlan performs an inline plan change on an existing Stripe subscription.
// This swaps the price item in-place with prorated billing — no new checkout
// session is created. This is the industry-standard approach used by 1Password,
// Bitwarden, and other SaaS products.
func (s *paymentService) ChangePlan(ctx context.Context, orgID, userID uint, plan, billingCycle string, seats int, ipAddress, userAgent string) (*domain.PlanChangeResult, error) {
	if err := s.authorizeOwnerOrAdmin(ctx, orgID, userID); err != nil {
		return nil, err
	}

	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("organization not found: %w", err)
	}
	if org.IsPersonal && domain.IsMultiUserPlan(plan) {
		return nil, fmt.Errorf("personal vaults can only be upgraded to Pro; create a separate organization for Family, Team, or Business plans")
	}
	if !org.IsPersonal && domain.IsPersonalVaultPlan(plan) && plan != string(domain.PlanFree) {
		return nil, fmt.Errorf("pro plan is only available for personal vaults")
	}

	sub, err := s.subscriptionService.GetByOrganizationID(ctx, orgID)
	if err != nil || sub == nil || sub.StripeSubscriptionID == nil || *sub.StripeSubscriptionID == "" {
		return nil, fmt.Errorf("no active Stripe subscription found")
	}

	planConfig, err := s.getPlanConfig(plan, billingCycle)
	if err != nil {
		return nil, err
	}

	if seats < 1 {
		seats = 1
	}

	// Enforce minimum seats against current member count
	members, _ := s.orgUserRepo.ListByOrganization(ctx, orgID)
	if len(members) > seats {
		seats = len(members)
	}

	// Enforce plan max users
	if planConfig.MaxUsers != nil && seats > *planConfig.MaxUsers {
		return nil, fmt.Errorf("requested seats (%d) exceed plan maximum (%d)", seats, *planConfig.MaxUsers)
	}

	metadata := map[string]string{
		"plan":          plan,
		"billing_cycle": billingCycle,
	}

	_, err = s.stripe.UpdateSubscriptionPlan(*sub.StripeSubscriptionID, planConfig.StripePriceID, int64(seats), metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to change plan: %w", err)
	}

	// Sync DB immediately so the frontend gets fresh data on refetch.
	// Look up the new plan record and update PlanID + seats in the subscription.
	newPlanCode := plan + "-" + billingCycle
	newPlan, planErr := s.planRepo.GetByCode(ctx, newPlanCode)
	if planErr == nil && newPlan != nil {
		sub.PlanID = newPlan.ID
		sub.Plan = newPlan
		sub.SeatsPurchased = &seats
		if err := s.subscriptionService.Update(ctx, sub); err != nil {
			s.logger.Error("failed to sync plan change to DB (Stripe already updated)", "org_id", orgID, "plan", newPlanCode, "err", err)
			// Don't fail — Stripe is the source of truth and the webhook will catch up.
		}
	} else {
		s.logger.Error("failed to look up new plan for DB sync", "plan_code", newPlanCode, "err", planErr)
	}

	s.logger.Info("plan changed via inline update", "org_id", orgID, "new_plan", plan, "billing_cycle", billingCycle, "seats", seats)
	s.activityLogger.LogCheckoutCreated(ctx, userID, ipAddress, userAgent, orgID, "", plan, billingCycle, "inline-plan-change")

	return &domain.PlanChangeResult{
		Success: true,
		Message: "Plan changed successfully.",
	}, nil
}

// authorizeOwnerOrAdmin checks if the user is an owner or admin of the organization.
func (s *paymentService) authorizeOwnerOrAdmin(ctx context.Context, orgID, userID uint) error {
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
	if role == "" || (role != domain.OrgRoleOwner && role != domain.OrgRoleAdmin) {
		return fmt.Errorf("forbidden")
	}
	return nil
}

// HandleWebhook handles Stripe webhook events
func (s *paymentService) HandleWebhook(ctx context.Context, payload []byte, signature string) error {
	s.logger.Info("Stripe webhook received", "payload_size", len(payload))

	event, err := s.stripe.ConstructWebhookEvent(payload, signature)
	if err != nil {
		s.logger.Error("Webhook signature verification failed", "error", err)
		return fmt.Errorf("%w: %v", ErrInvalidStripeWebhookSignature, err)
	}

	s.logger.Info("Webhook signature verified", "event_type", event.Type, "event_id", event.ID)
	skip, err := reserveWebhookEvent(ctx, s.webhookEventRepo, event.ID, string(event.Type))
	if err != nil {
		return err
	}
	if skip {
		s.logger.Info("Skipping already processed Stripe webhook", "event_type", event.Type, "event_id", event.ID)
		return nil
	}

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
	case "customer.subscription.trial_will_end":
		s.logger.Info("Processing customer.subscription.trial_will_end webhook", "event_id", event.ID)
		handlerErr = s.handleTrialWillEnd(ctx, event)
	default:
		s.logger.Info("ℹ️  Unhandled webhook event type (ignored)", "event_type", event.Type, "event_id", event.ID)
	}

	if handlerErr != nil {
		if s.webhookEventRepo != nil {
			if markErr := s.webhookEventRepo.MarkFailed(ctx, event.ID, handlerErr.Error()); markErr != nil {
				s.logger.Error("Failed to record Stripe webhook failure", "event_id", event.ID, "error", markErr)
			}
		}
		s.logger.Error("❌ Webhook handler failed", "event_type", event.Type, "event_id", event.ID, "error", handlerErr)
		return handlerErr
	}

	if s.webhookEventRepo != nil {
		if err := s.webhookEventRepo.MarkProcessed(ctx, event.ID); err != nil {
			return fmt.Errorf("failed to mark Stripe webhook processed: %w", err)
		}
	}
	s.logger.Info("✅ Webhook processed successfully", "event_type", event.Type, "event_id", event.ID)
	return nil
}

// handleCheckoutCompleted handles checkout.session.completed event
// Supports both organization-level and user-level subscriptions
func (s *paymentService) handleCheckoutCompleted(ctx context.Context, event stripe.Event) error {
	var session stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
		s.logger.Error("Failed to parse checkout session from webhook", "error", err)
		return fmt.Errorf("failed to parse checkout session: %w", err)
	}

	// Handle as organization subscription
	orgIDStr := session.Metadata["organization_id"]
	if orgIDStr == "" {
		s.logger.Error("organization_id not found in checkout session metadata", "session_id", session.ID)
		return fmt.Errorf("no organization_id in metadata")
	}

	var orgID uint
	if _, err := fmt.Sscanf(orgIDStr, "%d", &orgID); err != nil {
		s.logger.Error("invalid organization_id in checkout session metadata", "session_id", session.ID, "organization_id", orgIDStr, "error", err)
		return fmt.Errorf("invalid organization_id in metadata: %w", err)
	}

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

	// Handle as organization subscription
	orgIDStr := sub.Metadata["organization_id"]
	if orgIDStr == "" {
		s.logger.Error("organization_id not found in subscription metadata", "subscription_id", sub.ID)
		return fmt.Errorf("no organization_id in metadata")
	}

	var orgID uint
	if _, err := fmt.Sscanf(orgIDStr, "%d", &orgID); err != nil {
		s.logger.Error("invalid organization_id in subscription metadata", "subscription_id", sub.ID, "organization_id", orgIDStr, "error", err)
		return fmt.Errorf("invalid organization_id in metadata: %w", err)
	}

	// Get plan from price ID
	priceID := stripeClient.GetPriceFromSubscription(&sub)
	planCode, _ := s.mapPriceIDToPlan(priceID)
	if planCode == "" {
		s.logger.Error("Unknown price ID in subscription", "price_id", priceID, "subscription_id", sub.ID)
		return fmt.Errorf("unknown price ID: %s", priceID)
	}

	// Seat-based: derive purchased seats from Stripe subscription item quantity (licensed pricing)
	var seatsPurchased *int
	if len(sub.Items.Data) > 0 {
		q := int(sub.Items.Data[0].Quantity)
		if q > 0 {
			seatsPurchased = &q
		}
	}

	s.logger.Info("Creating subscription in database", "org_id", orgID, "plan_code", planCode, "subscription_id", sub.ID)

	if sub.Status != stripe.SubscriptionStatusActive && sub.Status != stripe.SubscriptionStatusTrialing && sub.Status != stripe.SubscriptionStatusPastDue {
		s.logger.Info("skipping non-access Stripe subscription", "subscription_id", sub.ID, "status", sub.Status)
		return nil
	}
	subscription, err := s.subscriptionService.CreateFromProvider(ctx, orgID, planCode, sub.ID, seatsPurchased, stripeTrialEnd(&sub))
	if err != nil {
		s.logger.Error("Failed to create subscription in database", "org_id", orgID, "subscription_id", sub.ID, "error", err)
		return fmt.Errorf("failed to create subscription: %w", err)
	}
	if err := s.syncProviderPeriod(ctx, &sub); err != nil {
		return err
	}
	if subscription.State == domain.SubStateTrialing {
		s.logTrialEvent(ctx, domain.ActivityTypeTrialStarted, orgID, sub.ID, planCode)
	}
	if sub.Status == stripe.SubscriptionStatusPastDue {
		if err := s.subscriptionService.HandlePaymentFailed(ctx, sub.ID); err != nil {
			return err
		}
	}

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

	// Handle organization subscription update
	if err := s.handleOrgSubscriptionUpdate(ctx, sub); err != nil {
		if isRecordNotFound(err) {
			s.logger.Warn("Subscription not found for update",
				"subscription_id", sub.ID, "status", sub.Status)
			return nil
		}
		s.logger.Error("Failed to handle subscription update", "subscription_id", sub.ID, "error", err)
		return err
	}

	s.logger.Info("✅ Subscription updated successfully", "subscription_id", sub.ID, "status", sub.Status)
	return nil
}

// handleOrgSubscriptionUpdate handles subscription update for organization subscriptions
func (s *paymentService) handleOrgSubscriptionUpdate(ctx context.Context, sub stripe.Subscription) error {
	if err := s.syncProviderPeriod(ctx, &sub); err != nil {
		if isRecordNotFound(err) {
			return err
		}
		s.logger.Warn("failed to sync subscription period", "subscription_id", sub.ID, "error", err)
	}

	// Sync seats (quantity) on every subscription update
	var seatsPurchased *int
	if len(sub.Items.Data) > 0 {
		q := int(sub.Items.Data[0].Quantity)
		if q > 0 {
			seatsPurchased = &q
		}
	}
	if err := s.subscriptionService.UpdateSeatsPurchasedByStripeSubscriptionID(ctx, sub.ID, seatsPurchased); err != nil {
		// If subscription not found, return the error to try user subscription
		if isRecordNotFound(err) {
			return err
		}
		s.logger.Warn("failed to sync seats purchased for org subscription", "subscription_id", sub.ID, "error", err)
	}

	// Handle payment success/failure through SubscriptionService
	switch sub.Status {
	case stripe.SubscriptionStatusActive, stripe.SubscriptionStatusTrialing:
		if sub.Status == stripe.SubscriptionStatusActive {
			if dbSub, err := s.subscriptionService.GetByStripeSubscriptionID(ctx, sub.ID); err == nil && dbSub.State == domain.SubStateTrialing {
				planCode := ""
				if dbSub.Plan != nil {
					planCode = dbSub.Plan.Code
				}
				s.logTrialEvent(ctx, domain.ActivityTypeTrialConverted, dbSub.OrganizationID, sub.ID, planCode)
			}
		}
		return s.subscriptionService.HandlePaymentSuccess(ctx, sub.ID)
	case stripe.SubscriptionStatusPastDue:
		return s.subscriptionService.HandlePaymentFailed(ctx, sub.ID)
	case stripe.SubscriptionStatusCanceled:
		dbSub, err := s.subscriptionService.GetByStripeSubscriptionID(ctx, sub.ID)
		if err != nil {
			s.logger.Warn("Subscription not found for canceled update", "stripe_subscription_id", sub.ID)
			return nil
		}
		if dbSub.State != domain.SubStateExpired {
			return s.subscriptionService.HandleProviderCanceled(ctx, dbSub.ID, stripeEndedAt(&sub))
		}
	}

	return nil
}

func (s *paymentService) syncProviderPeriod(ctx context.Context, sub *stripe.Subscription) error {
	var periodEnd time.Time
	if sub.CurrentPeriodEnd > 0 {
		periodEnd = time.Unix(sub.CurrentPeriodEnd, 0)
	}
	var trialEnd *time.Time
	if sub.TrialEnd > 0 {
		value := time.Unix(sub.TrialEnd, 0)
		trialEnd = &value
	}
	return s.subscriptionService.SyncProviderPeriod(ctx, sub.ID, periodEnd, trialEnd)
}

func isRecordNotFound(err error) bool {
	return errors.Is(err, repository.ErrNotFound) ||
		errors.Is(err, gorm.ErrRecordNotFound) ||
		strings.Contains(err.Error(), "record not found")
}

// stripeEndedAt returns when Stripe terminated the subscription. A deleted or
// canceled Stripe subscription without ended_at is treated as ended now.
func stripeEndedAt(sub *stripe.Subscription) *time.Time {
	endedAt := time.Now()
	if sub.EndedAt > 0 {
		endedAt = time.Unix(sub.EndedAt, 0)
	}
	return &endedAt
}

// stripeTrialEnd returns the provider trial end, or nil when the subscription
// has no trial.
func stripeTrialEnd(sub *stripe.Subscription) *time.Time {
	if sub == nil || sub.TrialEnd <= 0 {
		return nil
	}
	end := time.Unix(sub.TrialEnd, 0)
	return &end
}

// logTrialEvent records a trial funnel event against the organization owner.
func (s *paymentService) logTrialEvent(ctx context.Context, activityType domain.ActivityType, orgID uint, subscriptionID, planCode string) {
	ownerID := s.getOrganizationOwnerID(ctx, orgID)
	if ownerID == 0 {
		return
	}
	orgName := ""
	if org, err := s.orgRepo.GetByID(ctx, orgID); err == nil {
		orgName = org.Name
	}
	s.activityLogger.LogTrialEvent(ctx, activityType, ownerID, orgID, orgName, subscriptionID, planCode)
}

// handleTrialWillEnd reminds the billing contact before a Stripe trial converts
// to a paid subscription.
func (s *paymentService) handleTrialWillEnd(ctx context.Context, event stripe.Event) error {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		return fmt.Errorf("failed to parse subscription: %w", err)
	}

	var orgID uint
	if _, err := fmt.Sscanf(sub.Metadata["organization_id"], "%d", &orgID); err != nil || orgID == 0 {
		s.logger.Warn("trial_will_end without organization metadata (skipped)", "subscription_id", sub.ID)
		return nil
	}
	trialEnd := stripeTrialEnd(&sub)
	if trialEnd == nil {
		return nil
	}
	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		s.logger.Warn("trial_will_end organization not found (skipped)", "org_id", orgID, "subscription_id", sub.ID)
		return nil
	}

	if s.trialEmails != nil && org.BillingEmail != "" {
		billingURL := fmt.Sprintf("%s/organizations/%s/billing", s.config.Server.FrontendURL, org.PublicID)
		message, err := s.trialEmails.builder.BuildTrialEndingEmail(org.BillingEmail, org.Name, *trialEnd, billingURL)
		if err != nil {
			return fmt.Errorf("failed to build trial ending email: %w", err)
		}
		if err := s.trialEmails.sender.Send(ctx, message); err != nil {
			return fmt.Errorf("failed to send trial ending email: %w", err)
		}
	}

	planCode, _ := s.mapPriceIDToPlan(stripeClient.GetPriceFromSubscription(&sub))
	s.logTrialEvent(ctx, domain.ActivityTypeTrialEndingNotified, orgID, sub.ID, planCode)
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

	dbSub, err := s.subscriptionService.GetByStripeSubscriptionID(ctx, sub.ID)
	if err != nil {
		s.logger.Warn("Subscription not found in DB for deleted webhook, skipping", "stripe_subscription_id", sub.ID)
		return nil
	}

	if dbSub.State == domain.SubStateExpired {
		return nil
	}
	if dbSub.State == domain.SubStateTrialing {
		planCode := ""
		if dbSub.Plan != nil {
			planCode = dbSub.Plan.Code
		}
		s.logTrialEvent(ctx, domain.ActivityTypeTrialCanceled, dbSub.OrganizationID, sub.ID, planCode)
	}

	if err := s.subscriptionService.HandleProviderCanceled(ctx, dbSub.ID, stripeEndedAt(&sub)); err != nil {
		s.logger.Error("Failed to expire subscription on Stripe deletion", "subscription_id", dbSub.ID, "error", err)
		return fmt.Errorf("failed to expire subscription: %w", err)
	}

	s.logger.Info("✅ Subscription expired after Stripe deletion", "subscription_id", dbSub.ID)
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
	if _, err := fmt.Sscanf(orgIDStr, "%d", &orgID); err != nil {
		s.logger.Warn("invalid organization_id in subscription metadata (skipping update)", "subscription_id", sub.ID, "organization_id", orgIDStr, "error", err)
		return nil
	}

	// Webhooks also fire for subscriptions without current access (e.g. the
	// final invoice of a canceled subscription). Nothing to apply; returning an
	// error would only make Stripe retry the event.
	if sub.Status != stripe.SubscriptionStatusActive && sub.Status != stripe.SubscriptionStatusTrialing {
		s.logger.Info("skipping organization update for Stripe subscription without access", "org_id", orgID, "subscription_id", sub.ID, "status", sub.Status)
		return nil
	}

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
	if sub.Status != stripe.SubscriptionStatusActive && sub.Status != stripe.SubscriptionStatusTrialing {
		return fmt.Errorf("stripe subscription has no current access (status: %s)", sub.Status)
	}
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

	if _, err := s.subscriptionService.CreateFromProvider(ctx, orgID, planCode, sub.ID, seatsPurchased, stripeTrialEnd(sub)); err != nil {
		return fmt.Errorf("failed to upsert subscription: %w", err)
	}

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
		return nil, fmt.Errorf("failed to count members: %w", err)
	}

	currentCollections, err := s.orgRepo.GetCollectionCount(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to count collections: %w", err)
	}
	currentItems, err := s.orgRepo.GetItemCount(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to count items: %w", err)
	}

	// Convert org to DTO (plan/limits derived from subscription when available)
	orgDTO := domain.ToOrganizationDTO(org)

	billingInfo := &domain.BillingInfo{
		Organization:       orgDTO,
		CurrentUsers:       len(members),
		CurrentCollections: currentCollections,
		CurrentItems:       currentItems,
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

	// If webhooks are not delivered (common in local dev), try a best-effort sync from Stripe
	// when we don't have a meaningful subscription in DB. This prevents "paid but still free" UX after redirect.
	if shouldAutoSync {
		if syncErr := s.SyncSubscription(ctx, orgID); syncErr != nil {
			s.logger.Warn("auto sync subscription failed", "org_id", orgID, "error", syncErr)
			// Continue without subscription (don't fail billing endpoint)
			subscription = nil
		} else {
			// Re-fetch subscription after successful sync.
			subscription, err = s.subscriptionService.GetByOrganizationID(ctx, orgID)
			if err != nil || subscription == nil || subscription.Plan == nil {
				s.logger.Warn("auto sync succeeded but subscription still missing", "org_id", orgID, "error", err)
				subscription = nil
			}
		}
	}

	if subscription == nil {
		// Return billing info without subscription
		return billingInfo, nil
	}

	// Check if subscription is active (grants entitlements)
	isActiveSubscription := subscription.State == domain.SubStateActive ||
		subscription.State == domain.SubStateTrialing ||
		subscription.State == domain.SubStatePastDue ||
		(subscription.State == domain.SubStateCanceled && subscription.RenewAt != nil && subscription.RenewAt.After(time.Now()))

	// Only derive org plan/limits from subscription if it's active
	if isActiveSubscription {
		orgDTO = domain.ToOrganizationDTOWithSubscription(org, subscription)
		billingInfo.Organization = orgDTO
	}
	// Always include subscription info so UI can show history/status
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
	if current, err := s.subscriptionService.GetByOrganizationID(ctx, orgID); err == nil &&
		isActiveManual(current, time.Now()) {
		return fmt.Errorf("active manual subscription cannot be overwritten")
	}
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
		return fmt.Errorf("no active or trialing subscription found")
	}

	// Update organization from subscription
	// For manual sync, we pass the orgID directly instead of relying on metadata
	if err := s.updateOrgFromSubscriptionWithID(ctx, activeSub, orgID); err != nil {
		return fmt.Errorf("failed to update organization: %w", err)
	}

	s.logger.Info("manually synced subscription", "org_id", orgID, "subscription_id", activeSub.ID, "status", activeSub.Status)

	return nil
}

// getPlanConfig returns the plan config for a plan and billing cycle
func (s *paymentService) getPlanConfig(plan, billingCycle string) (*config.PlanConfig, error) {
	// Get plan config from config
	planCode := fmt.Sprintf("%s-%s", plan, billingCycle)

	for i := range s.config.Stripe.Plans {
		if s.config.Stripe.Plans[i].Code == planCode {
			if s.config.Stripe.Plans[i].StripePriceID == "" {
				return nil, fmt.Errorf("stripe price ID not configured for plan: %s", planCode)
			}
			return &s.config.Stripe.Plans[i], nil
		}
	}

	return nil, fmt.Errorf("plan not found in config: %s (billing cycle: %s)", plan, billingCycle)
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
