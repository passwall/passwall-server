package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/passwall/passwall-server/internal/config"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	stripeClient "github.com/passwall/passwall-server/pkg/stripe"
)

type userPaymentService struct {
	stripe                  *stripeClient.Client
	userRepo                repository.UserRepository
	userSubscriptionService UserSubscriptionService
	planRepo                interface {
		GetByCode(ctx context.Context, code string) (*domain.Plan, error)
		GetByID(ctx context.Context, id uint) (*domain.Plan, error)
	}
	activityLogger *ActivityLogger
	config         *config.Config
	logger         Logger
}

// NewUserPaymentService creates a new user payment service
func NewUserPaymentService(
	stripe *stripeClient.Client,
	userRepo repository.UserRepository,
	userSubscriptionService UserSubscriptionService,
	planRepo interface {
		GetByCode(ctx context.Context, code string) (*domain.Plan, error)
		GetByID(ctx context.Context, id uint) (*domain.Plan, error)
	},
	activityService UserActivityService,
	config *config.Config,
	logger Logger,
) UserPaymentService {
	return &userPaymentService{
		stripe:                  stripe,
		userRepo:                userRepo,
		userSubscriptionService: userSubscriptionService,
		planRepo:                planRepo,
		activityLogger:          NewActivityLogger(activityService),
		config:                  config,
		logger:                  logger,
	}
}

// CreateCheckoutSession creates a Stripe checkout session for a user's personal subscription
func (s *userPaymentService) CreateCheckoutSession(ctx context.Context, userID uint, plan, billingCycle string, ipAddress, userAgent string) (string, error) {
	s.logger.Info("user_payment.create_checkout called",
		"user_id", userID,
		"plan", plan,
		"billing_cycle", billingCycle,
	)

	// Get user
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("user not found: %w", err)
	}

	// Validate plan (only "pro" is valid for personal subscriptions)
	if plan != "pro" {
		return "", fmt.Errorf("invalid plan for personal subscription: %s (only 'pro' is allowed)", plan)
	}

	// Validate billing cycle
	if billingCycle != "monthly" && billingCycle != "yearly" {
		return "", fmt.Errorf("invalid billing cycle: %s", billingCycle)
	}

	// Get plan config (includes price ID and trial days)
	planConfig, err := s.getPlanConfig(plan, billingCycle)
	if err != nil {
		return "", err
	}

	// Get or create Stripe customer for user
	customerID := ""
	if user.StripeCustomerID != nil && *user.StripeCustomerID != "" {
		customerID = *user.StripeCustomerID

		// Verify customer exists
		_, err := s.stripe.GetCustomer(customerID)
		if err != nil {
			s.logger.Warn("stripe customer not found, creating new one", "user_id", userID, "old_customer_id", customerID)
			customerID = ""
		}
	}

	if customerID == "" {
		// Create new Stripe customer for user
		customer, err := s.stripe.CreateCustomer(stripeClient.CreateCustomerParams{
			Email:        user.Email,
			Name:         user.Name,
			OrgID:        "", // No org for personal subscriptions
			BillingEmail: user.Email,
		})
		if err != nil {
			return "", fmt.Errorf("failed to create Stripe customer: %w", err)
		}
		customerID = customer.ID

		// Update user with customer ID
		user.StripeCustomerID = &customerID
		if err := s.userRepo.Update(ctx, user); err != nil {
			s.logger.Error("failed to save stripe customer ID to user", "error", err)
		}
	}

	// Create checkout session
	successURL := fmt.Sprintf("%s/billing?success=true", s.config.Server.FrontendURL)
	cancelURL := fmt.Sprintf("%s/billing?canceled=true", s.config.Server.FrontendURL)

	session, err := s.stripe.CreateCheckoutSession(stripeClient.CheckoutSessionParams{
		CustomerID:   customerID,
		PriceID:      planConfig.StripePriceID,
		Quantity:     1, // Personal subscriptions are always 1 seat
		SuccessURL:   successURL,
		CancelURL:    cancelURL,
		OrgID:        "", // No org - use user_id in metadata
		OrgName:      "",
		Plan:         plan,
		BillingCycle: billingCycle,
		TrialDays:    planConfig.TrialDays, // Trial period from config
		Metadata: map[string]string{
			"user_id":       fmt.Sprintf("%d", userID),
			"type":          "personal",
			"plan":          plan,
			"billing_cycle": billingCycle,
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create checkout session: %w", err)
	}

	s.logger.Info("created user checkout session",
		"user_id", userID,
		"plan", plan,
		"billing_cycle", billingCycle,
		"session_id", session.ID,
	)

	return session.URL, nil
}

// GetBillingInfo retrieves billing information for a user
func (s *userPaymentService) GetBillingInfo(ctx context.Context, userID uint) (*domain.UserBillingInfo, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// Get item count
	itemCount, _ := s.userRepo.GetItemCount(ctx, user.Schema)

	billingInfo := &domain.UserBillingInfo{
		UserID:       user.ID,
		Email:        user.Email,
		Name:         user.Name,
		IsPro:        false,
		CurrentPlan:  "free",
		CurrentItems: itemCount,
	}

	// Get subscription from database
	subscription, err := s.userSubscriptionService.GetByUserID(ctx, userID)
	if err != nil {
		s.logger.Info("No subscription found for user", "user_id", userID)
		// Return free plan billing info
		return billingInfo, nil
	}

	if subscription != nil && subscription.Plan != nil {
		// Subscription has plan loaded - continue
	} else if subscription != nil && subscription.Plan == nil {
		// Fallback: load plan by ID if preload failed or plan was not included
		plan, err := s.planRepo.GetByID(ctx, subscription.PlanID)
		if err != nil {
			s.logger.Warn("Failed to load plan for subscription",
				"user_id", userID,
				"subscription_id", subscription.ID,
				"plan_id", subscription.PlanID,
				"error", err,
			)
		} else {
			subscription.Plan = plan
		}
	}

	if subscription != nil && subscription.Plan != nil {
		// Only set plan code if subscription is active (active, trialing, past_due, or canceled but still in access period)
		isActiveSubscription := subscription.State == domain.SubStateActive ||
			subscription.State == domain.SubStateTrialing ||
			subscription.State == domain.SubStatePastDue ||
			(subscription.State == domain.SubStateCanceled && subscription.RenewAt != nil && subscription.RenewAt.After(time.Now()))

		if isActiveSubscription {
			billingInfo.CurrentPlan = subscription.Plan.Code
			// Check if it's a Pro plan (any plan that starts with "pro-")
			billingInfo.IsPro = strings.HasPrefix(subscription.Plan.Code, "pro-")
		}
		// Always include subscription info so UI can show history/status
		billingInfo.Subscription = domain.ToUserSubscriptionDTO(subscription)
	}

	// Fetch invoices from Stripe only for Stripe-backed subscriptions.
	// RevenueCat/App Store/Google Play purchases do not have Stripe invoices.
	isRevenueCatSubscription := subscription != nil &&
		subscription.StripeSubscriptionID != nil &&
		strings.HasPrefix(*subscription.StripeSubscriptionID, "rc_")

	if !isRevenueCatSubscription && user.StripeCustomerID != nil && *user.StripeCustomerID != "" {
		stripeInvoices, err := s.stripe.ListInvoices(*user.StripeCustomerID, 10)
		if err != nil {
			s.logger.Warn("Failed to fetch invoices from Stripe", "user_id", userID, "error", err)
		} else if len(stripeInvoices) > 0 {
			invoiceDTOs := make([]*domain.InvoiceDTO, 0, len(stripeInvoices))
			for _, inv := range stripeInvoices {
				status := domain.InvoiceStatus(inv.Status)
				issuedAt := time.Unix(inv.Created, 0)
				amountDisplay := fmt.Sprintf("$%.2f", float64(inv.AmountPaid)/100)

				invoiceDTO := &domain.InvoiceDTO{
					Status:        status,
					AmountCents:   int(inv.AmountPaid),
					AmountDisplay: amountDisplay,
					Currency:      string(inv.Currency),
					IssuedAt:      issuedAt,
				}

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
		}
	}

	return billingInfo, nil
}

// Cancel cancels the user's subscription
func (s *userPaymentService) Cancel(ctx context.Context, userID uint) error {
	return s.userSubscriptionService.Cancel(ctx, userID)
}

// Resume resumes the user's canceled subscription
func (s *userPaymentService) Resume(ctx context.Context, userID uint) error {
	return s.userSubscriptionService.Resume(ctx, userID)
}

// SyncSubscription manually syncs subscription data from Stripe
func (s *userPaymentService) SyncSubscription(ctx context.Context, userID uint) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	if user.StripeCustomerID == nil || *user.StripeCustomerID == "" {
		return fmt.Errorf("no Stripe customer ID found for user")
	}

	// List all subscriptions for this customer
	subscriptions, err := s.stripe.ListCustomerSubscriptions(*user.StripeCustomerID)
	if err != nil {
		return fmt.Errorf("failed to list subscriptions: %w", err)
	}

	if len(subscriptions) == 0 {
		return fmt.Errorf("no subscriptions found for customer")
	}

	// Find active subscription
	var activeSub *stripeClient.StripeSubscription
	for _, sub := range subscriptions {
		// Check metadata to ensure it's a personal subscription
		if sub.Metadata["type"] != "personal" {
			continue
		}
		if sub.Status == "active" || sub.Status == "trialing" {
			activeSub = sub
			break
		}
	}

	if activeSub == nil {
		return fmt.Errorf("no active personal subscription found")
	}

	// Get plan code from Stripe subscription
	priceID := stripeClient.GetPriceFromSubscription(activeSub)
	planCode, _ := s.mapPriceIDToPlan(priceID)
	if planCode == "" {
		return fmt.Errorf("unknown price ID: %s", priceID)
	}

	// Create/update subscription in database
	_, err = s.userSubscriptionService.Create(ctx, userID, planCode, activeSub.ID)
	if err != nil {
		return fmt.Errorf("failed to sync subscription: %w", err)
	}

	s.logger.Info("manually synced user subscription",
		"user_id", userID,
		"subscription_id", activeSub.ID,
		"status", activeSub.Status,
	)

	return nil
}

// getPlanConfig returns the plan config for a plan and billing cycle
func (s *userPaymentService) getPlanConfig(plan, billingCycle string) (*config.PlanConfig, error) {
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
func (s *userPaymentService) mapPriceIDToPlan(priceID string) (string, domain.BillingCycle) {
	cfg := s.config.Stripe

	for _, plan := range cfg.Plans {
		if plan.StripePriceID == priceID {
			return plan.Code, domain.BillingCycle(plan.BillingCycle)
		}
	}

	return "", ""
}

// IsProPlan checks if a plan code represents a Pro plan
func IsProPlan(planCode string) bool {
	return strings.HasPrefix(planCode, "pro-")
}
