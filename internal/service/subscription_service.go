package service

import (
	"context"
	"fmt"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/pkg/stripe"
	uuid "github.com/satori/go.uuid"
)

// SubscriptionService handles subscription operations
type SubscriptionService interface {
	Create(ctx context.Context, orgID uint, planCode string, stripeSubscriptionID string, seatsPurchased *int) (*domain.Subscription, error)
	GetByID(ctx context.Context, id uint) (*domain.Subscription, error)
	GetByOrganizationID(ctx context.Context, orgID uint) (*domain.Subscription, error)
	Update(ctx context.Context, sub *domain.Subscription) error
	UpdateSeatsPurchasedByStripeSubscriptionID(ctx context.Context, stripeSubID string, seatsPurchased *int) error
	Upgrade(ctx context.Context, orgID uint, planCode string) error
	Downgrade(ctx context.Context, orgID uint, planCode string) error
	Cancel(ctx context.Context, orgID uint) error
	Resume(ctx context.Context, orgID uint) error
	Renew(ctx context.Context, subID uint) error
	HandlePaymentSuccess(ctx context.Context, stripeSubID string) error
	HandlePaymentFailed(ctx context.Context, stripeSubID string) error
	ExpireSubscription(ctx context.Context, subID uint) error
	CheckExpiredSubscriptions(ctx context.Context) error
}

type subscriptionService struct {
	subRepo interface {
		ExpireActiveByOrganizationID(ctx context.Context, orgID uint, endedAt time.Time) error
		Create(ctx context.Context, sub *domain.Subscription) error
		GetByID(ctx context.Context, id uint) (*domain.Subscription, error)
		GetByOrganizationID(ctx context.Context, orgID uint) (*domain.Subscription, error)
		GetByStripeSubscriptionID(ctx context.Context, stripeSubID string) (*domain.Subscription, error)
		Update(ctx context.Context, sub *domain.Subscription) error
		ListPastDueExpired(ctx context.Context) ([]*domain.Subscription, error)
		ListCanceledExpired(ctx context.Context) ([]*domain.Subscription, error)
		ListManualExpired(ctx context.Context) ([]*domain.Subscription, error)
	}
	planRepo interface {
		GetByCode(ctx context.Context, code string) (*domain.Plan, error)
		GetByID(ctx context.Context, id uint) (*domain.Plan, error)
	}
	orgService OrganizationService
	// emailService interface for sending emails (optional - can be nil)
	emailService interface {
		SendPaymentFailedEmail(ctx context.Context, sub *domain.Subscription) error
		SendSubscriptionExpiredEmail(ctx context.Context, sub *domain.Subscription) error
	}
	// stripe client for cancel/reactivate operations
	stripe any
	// logger for structured logging
	logger Logger
}

// NewSubscriptionService creates a new subscription service
func NewSubscriptionService(
	subRepo interface {
		ExpireActiveByOrganizationID(ctx context.Context, orgID uint, endedAt time.Time) error
		Create(ctx context.Context, sub *domain.Subscription) error
		GetByID(ctx context.Context, id uint) (*domain.Subscription, error)
		GetByOrganizationID(ctx context.Context, orgID uint) (*domain.Subscription, error)
		GetByStripeSubscriptionID(ctx context.Context, stripeSubID string) (*domain.Subscription, error)
		Update(ctx context.Context, sub *domain.Subscription) error
		ListPastDueExpired(ctx context.Context) ([]*domain.Subscription, error)
		ListCanceledExpired(ctx context.Context) ([]*domain.Subscription, error)
		ListManualExpired(ctx context.Context) ([]*domain.Subscription, error)
	},
	planRepo interface {
		GetByCode(ctx context.Context, code string) (*domain.Plan, error)
		GetByID(ctx context.Context, id uint) (*domain.Plan, error)
	},
	orgService OrganizationService,
	emailService interface {
		SendPaymentFailedEmail(ctx context.Context, sub *domain.Subscription) error
		SendSubscriptionExpiredEmail(ctx context.Context, sub *domain.Subscription) error
	},
	stripe any,
	logger Logger,
) SubscriptionService {
	return &subscriptionService{
		subRepo:      subRepo,
		planRepo:     planRepo,
		orgService:   orgService,
		emailService: emailService,
		stripe:       stripe,
		logger:       logger,
	}
}

// Create creates a new subscription
func (s *subscriptionService) Create(ctx context.Context, orgID uint, planCode string, stripeSubscriptionID string, seatsPurchased *int) (*domain.Subscription, error) {
	s.logger.Info("subscription.create called",
		"org_id", orgID,
		"plan_code", planCode,
		"stripe_subscription_id_set", stripeSubscriptionID != "",
	)
	// Idempotency for Stripe retries: if we already have this Stripe subscription, return it.
	if stripeSubscriptionID != "" {
		if existing, err := s.subRepo.GetByStripeSubscriptionID(ctx, stripeSubscriptionID); err == nil && existing != nil {
			s.logger.Info("subscription.create idempotent hit",
				"org_id", orgID,
				"stripe_subscription_id", stripeSubscriptionID,
			)
			return existing, nil
		}
	}

	plan, err := s.planRepo.GetByCode(ctx, planCode)
	if err != nil {
		s.logger.Error("subscription.create failed to get plan", "org_id", orgID, "plan_code", planCode, "error", err)
		return nil, fmt.Errorf("failed to get plan: %w", err)
	}

	if !plan.IsActive {
		s.logger.Warn("subscription.create inactive plan", "org_id", orgID, "plan_code", planCode)
		return nil, fmt.Errorf("plan is not active")
	}

	now := time.Now()

	// Hard invariant: prevent multiple active-like subscriptions per organization.
	// We expire any existing active/trialing/past_due rows before creating the new one.
	// (Old Stripe subscription should transition to canceled via webhook; this keeps DB consistent even if
	// events arrive out-of-order.)
	if err := s.subRepo.ExpireActiveByOrganizationID(ctx, orgID, now); err != nil {
		s.logger.Error("subscription.create failed to expire existing", "org_id", orgID, "error", err)
		return nil, fmt.Errorf("failed to expire existing active subscriptions: %w", err)
	}

	sub := &domain.Subscription{
		UUID:                 uuid.NewV4(),
		OrganizationID:       orgID,
		PlanID:               plan.ID,
		State:                domain.SubStateActive,
		StripeSubscriptionID: &stripeSubscriptionID,
		SeatsPurchased:       seatsPurchased,
		StartedAt:            &now,
	}

	// Handle trial period
	if plan.HasTrial() {
		sub.State = domain.SubStateTrialing
		trialEnd := time.Now().AddDate(0, 0, plan.TrialDays)
		sub.TrialEndsAt = &trialEnd
		sub.RenewAt = &trialEnd
	} else {
		// Set renew date based on billing cycle
		var renewAt time.Time
		if plan.BillingCycle == domain.BillingCycleMonthly {
			renewAt = now.AddDate(0, 1, 0)
		} else {
			renewAt = now.AddDate(1, 0, 0)
		}
		sub.RenewAt = &renewAt
	}

	if err := s.subRepo.Create(ctx, sub); err != nil {
		s.logger.Error("subscription.create failed to persist", "org_id", orgID, "error", err)
		return nil, fmt.Errorf("failed to create subscription: %w", err)
	}

	s.logger.Info("subscription.create ok",
		"org_id", orgID,
		"subscription_id", sub.ID,
		"state", sub.State,
	)
	return sub, nil
}

func (s *subscriptionService) UpdateSeatsPurchasedByStripeSubscriptionID(ctx context.Context, stripeSubID string, seatsPurchased *int) error {
	if stripeSubID == "" {
		return fmt.Errorf("stripe subscription id required")
	}
	s.logger.Info("subscription.seats sync",
		"stripe_subscription_id", stripeSubID,
		"seats_set", seatsPurchased != nil,
	)
	sub, err := s.subRepo.GetByStripeSubscriptionID(ctx, stripeSubID)
	if err != nil {
		s.logger.Error("subscription.seats sync failed to load", "stripe_subscription_id", stripeSubID, "error", err)
		return fmt.Errorf("failed to get subscription: %w", err)
	}
	sub.SeatsPurchased = seatsPurchased
	if err := s.subRepo.Update(ctx, sub); err != nil {
		s.logger.Error("subscription.seats sync failed to update", "stripe_subscription_id", stripeSubID, "error", err)
		return fmt.Errorf("failed to update subscription seats: %w", err)
	}
	return nil
}

// GetByID retrieves a subscription by ID
func (s *subscriptionService) GetByID(ctx context.Context, id uint) (*domain.Subscription, error) {
	return s.subRepo.GetByID(ctx, id)
}

// GetByOrganizationID retrieves the active subscription for an organization
func (s *subscriptionService) GetByOrganizationID(ctx context.Context, orgID uint) (*domain.Subscription, error) {
	return s.subRepo.GetByOrganizationID(ctx, orgID)
}

// Update updates a subscription
func (s *subscriptionService) Update(ctx context.Context, sub *domain.Subscription) error {
	return s.subRepo.Update(ctx, sub)
}

// Upgrade upgrades a subscription to a higher plan
func (s *subscriptionService) Upgrade(ctx context.Context, orgID uint, planCode string) error {
	sub, err := s.subRepo.GetByOrganizationID(ctx, orgID)
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	newPlan, err := s.planRepo.GetByCode(ctx, planCode)
	if err != nil {
		return fmt.Errorf("failed to get plan: %w", err)
	}

	// Validate upgrade
	currentPlan, err := s.planRepo.GetByID(ctx, sub.PlanID)
	if err != nil {
		return fmt.Errorf("failed to get current plan: %w", err)
	}

	if newPlan.PriceCents <= currentPlan.PriceCents {
		return fmt.Errorf("new plan must be higher tier than current plan")
	}

	// Update subscription
	sub.PlanID = newPlan.ID

	return s.subRepo.Update(ctx, sub)
}

// Downgrade downgrades a subscription to a lower plan
func (s *subscriptionService) Downgrade(ctx context.Context, orgID uint, planCode string) error {
	sub, err := s.subRepo.GetByOrganizationID(ctx, orgID)
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	newPlan, err := s.planRepo.GetByCode(ctx, planCode)
	if err != nil {
		return fmt.Errorf("failed to get plan: %w", err)
	}

	// Check if organization usage exceeds new plan limits
	if newPlan.MaxUsers != nil {
		memberCount, err := s.orgService.GetMemberCount(ctx, orgID)
		if err != nil {
			return fmt.Errorf("failed to get member count: %w", err)
		}
		if memberCount > *newPlan.MaxUsers {
			return fmt.Errorf("cannot downgrade: current users (%d) exceed plan limit (%d)", memberCount, *newPlan.MaxUsers)
		}
	}

	if newPlan.MaxCollections != nil {
		collectionCount, err := s.orgService.GetCollectionCount(ctx, orgID)
		if err != nil {
			return fmt.Errorf("failed to get collection count: %w", err)
		}
		if collectionCount > *newPlan.MaxCollections {
			return fmt.Errorf("cannot downgrade: current collections (%d) exceed plan limit (%d)", collectionCount, *newPlan.MaxCollections)
		}
	}

	// Schedule downgrade at end of billing period
	sub.PlanID = newPlan.ID

	return s.subRepo.Update(ctx, sub)
}

// Cancel cancels a subscription at the end of the billing period
func (s *subscriptionService) Cancel(ctx context.Context, orgID uint) error {
	// 1. Get subscription from database
	sub, err := s.subRepo.GetByOrganizationID(ctx, orgID)
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	if sub.State == domain.SubStateExpired || sub.State == domain.SubStateCanceled {
		return fmt.Errorf("subscription is already canceled or expired")
	}

	// 2. Cancel subscription in Stripe FIRST (always at period end)
	if sub.StripeSubscriptionID == nil || *sub.StripeSubscriptionID == "" {
		return fmt.Errorf("subscription has no Stripe subscription ID")
	}

	stripeClient, ok := s.stripe.(*stripe.Client)
	if !ok {
		return fmt.Errorf("stripe client not available")
	}

	s.logger.Infof("🔄 Canceling Stripe subscription at period end: %s (org_id=%d)",
		*sub.StripeSubscriptionID, orgID)

	stripeSub, err := stripeClient.CancelSubscription(*sub.StripeSubscriptionID, true) // true = cancel at period end
	if err != nil {
		s.logger.Error("Failed to cancel Stripe subscription",
			"stripe_subscription_id", *sub.StripeSubscriptionID,
			"org_id", orgID,
			"error", err)
		return fmt.Errorf("failed to cancel Stripe subscription: %w", err)
	}

	s.logger.Infof("✅ Stripe subscription canceled successfully: %s (status=%s, cancel_at_period_end=true)",
		stripeSub.ID, stripeSub.Status)

	// 3. Only update database AFTER Stripe success
	now := time.Now()
	sub.CancelAt = &now
	sub.State = domain.SubStateCanceled
	// RenewAt will be used to determine when to expire

	s.logger.Infof("📝 Marking subscription as CANCELED at period end (org_id=%d, will_expire_at=%v)",
		orgID, sub.RenewAt)

	if err := s.subRepo.Update(ctx, sub); err != nil {
		s.logger.Error("Failed to update subscription in database after Stripe cancel",
			"org_id", orgID,
			"error", err)
		return fmt.Errorf("failed to update subscription in database: %w", err)
	}

	s.logger.Infof("✅ Subscription canceled successfully in database (org_id=%d)", orgID)
	return nil
}

// Resume resumes a canceled subscription
func (s *subscriptionService) Resume(ctx context.Context, orgID uint) error {
	// 1. Get subscription from database
	sub, err := s.subRepo.GetByOrganizationID(ctx, orgID)
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	if sub.State != domain.SubStateCanceled {
		return fmt.Errorf("can only resume canceled subscriptions (current state: %s)", sub.State)
	}

	// 2. Reactivate subscription in Stripe FIRST
	if sub.StripeSubscriptionID == nil || *sub.StripeSubscriptionID == "" {
		return fmt.Errorf("subscription has no Stripe subscription ID")
	}

	stripeClient, ok := s.stripe.(*stripe.Client)
	if !ok {
		return fmt.Errorf("stripe client not available")
	}

	s.logger.Infof("🔄 Reactivating Stripe subscription: %s (org_id=%d)",
		*sub.StripeSubscriptionID, orgID)

	stripeSub, err := stripeClient.ReactivateSubscription(*sub.StripeSubscriptionID)
	if err != nil {
		s.logger.Error("Failed to reactivate Stripe subscription",
			"stripe_subscription_id", *sub.StripeSubscriptionID,
			"org_id", orgID,
			"error", err)
		return fmt.Errorf("failed to reactivate Stripe subscription: %w", err)
	}

	s.logger.Infof("✅ Stripe subscription reactivated successfully: %s (status=%s)",
		stripeSub.ID, stripeSub.Status)

	// 3. Only update database AFTER Stripe success
	sub.State = domain.SubStateActive
	sub.CancelAt = nil

	if err := s.subRepo.Update(ctx, sub); err != nil {
		s.logger.Error("Failed to update subscription in database after Stripe reactivation",
			"org_id", orgID,
			"error", err)
		return fmt.Errorf("failed to update subscription in database: %w", err)
	}

	s.logger.Infof("✅ Subscription reactivated successfully in database (org_id=%d)", orgID)
	return nil
}

// Renew renews a subscription
func (s *subscriptionService) Renew(ctx context.Context, subID uint) error {
	sub, err := s.subRepo.GetByID(ctx, subID)
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	// Calculate next renewal date
	nextRenew := s.calculateNextRenewal(sub)
	sub.RenewAt = &nextRenew

	return s.subRepo.Update(ctx, sub)
}

// HandlePaymentSuccess handles successful payment webhook
func (s *subscriptionService) HandlePaymentSuccess(ctx context.Context, stripeSubID string) error {
	sub, err := s.subRepo.GetByStripeSubscriptionID(ctx, stripeSubID)
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	// Transition state based on current state
	now := time.Now()

	switch sub.State {
	case domain.SubStateDraft, domain.SubStateTrialing:
		// First payment succeeded - activate subscription
		sub.State = domain.SubStateActive
		sub.StartedAt = &now

	case domain.SubStatePastDue:
		// Payment retry succeeded - restore to active
		sub.State = domain.SubStateActive
		sub.GracePeriodEndsAt = nil

	case domain.SubStateExpired:
		// Reactivation payment succeeded
		sub.State = domain.SubStateActive
		sub.StartedAt = &now
		sub.EndedAt = nil
	}

	// Update renewal date
	nextRenew := s.calculateNextRenewal(sub)
	sub.RenewAt = &nextRenew

	return s.subRepo.Update(ctx, sub)
}

// HandlePaymentFailed handles failed payment webhook
func (s *subscriptionService) HandlePaymentFailed(ctx context.Context, stripeSubID string) error {
	sub, err := s.subRepo.GetByStripeSubscriptionID(ctx, stripeSubID)
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	// Move to past_due state with grace period
	sub.State = domain.SubStatePastDue
	gracePeriod := time.Now().AddDate(0, 0, 14) // 14 days grace period
	sub.GracePeriodEndsAt = &gracePeriod

	// Send notification email
	if s.emailService != nil {
		go func() {
			_ = s.emailService.SendPaymentFailedEmail(context.Background(), sub)
		}()
	}

	return s.subRepo.Update(ctx, sub)
}

// ExpireSubscription expires a subscription
func (s *subscriptionService) ExpireSubscription(ctx context.Context, subID uint) error {
	sub, err := s.subRepo.GetByID(ctx, subID)
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	now := time.Now()
	sub.State = domain.SubStateExpired
	sub.EndedAt = &now

	// Send notification email
	if s.emailService != nil {
		go func() {
			_ = s.emailService.SendSubscriptionExpiredEmail(context.Background(), sub)
		}()
	}

	return s.subRepo.Update(ctx, sub)
}

// CheckExpiredSubscriptions checks and expires subscriptions that should be expired
func (s *subscriptionService) CheckExpiredSubscriptions(ctx context.Context) error {
	// Find past_due subscriptions with expired grace periods
	pastDueSubs, err := s.subRepo.ListPastDueExpired(ctx)
	if err != nil {
		return fmt.Errorf("failed to list past due subscriptions: %w", err)
	}

	for _, sub := range pastDueSubs {
		if err := s.ExpireSubscription(ctx, sub.ID); err != nil {
			// Log error but continue processing others
			continue
		}
	}

	// Find canceled subscriptions with expired periods
	canceledSubs, err := s.subRepo.ListCanceledExpired(ctx)
	if err != nil {
		return fmt.Errorf("failed to list canceled subscriptions: %w", err)
	}

	for _, sub := range canceledSubs {
		if err := s.ExpireSubscription(ctx, sub.ID); err != nil {
			// Log error but continue processing others
			continue
		}
	}

	// Find manual active/trialing subscriptions where end date passed
	manualSubs, err := s.subRepo.ListManualExpired(ctx)
	if err != nil {
		return fmt.Errorf("failed to list manual expired subscriptions: %w", err)
	}
	for _, sub := range manualSubs {
		if err := s.ExpireSubscription(ctx, sub.ID); err != nil {
			continue
		}
	}

	return nil
}

// calculateNextRenewal calculates the next renewal date based on billing cycle
func (s *subscriptionService) calculateNextRenewal(sub *domain.Subscription) time.Time {
	if sub.Plan == nil {
		return time.Now().AddDate(0, 1, 0) // Default to 1 month
	}

	now := time.Now()

	switch sub.Plan.BillingCycle {
	case domain.BillingCycleMonthly:
		return now.AddDate(0, 1, 0)
	case domain.BillingCycleYearly:
		return now.AddDate(1, 0, 0)
	default:
		return now.AddDate(0, 1, 0)
	}
}
