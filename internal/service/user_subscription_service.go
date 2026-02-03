package service

import (
	"context"
	"fmt"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/pkg/stripe"
	uuid "github.com/satori/go.uuid"
)

// UserSubscriptionService handles user-level subscription operations
type UserSubscriptionService interface {
	Create(ctx context.Context, userID uint, planCode string, stripeSubscriptionID string) (*domain.UserSubscription, error)
	GetByID(ctx context.Context, id uint) (*domain.UserSubscription, error)
	GetByUserID(ctx context.Context, userID uint) (*domain.UserSubscription, error)
	Update(ctx context.Context, sub *domain.UserSubscription) error
	Cancel(ctx context.Context, userID uint) error
	Resume(ctx context.Context, userID uint) error
	HandlePaymentSuccess(ctx context.Context, stripeSubID string) error
	HandlePaymentFailed(ctx context.Context, stripeSubID string) error
	ExpireSubscription(ctx context.Context, subID uint) error
	CheckExpiredSubscriptions(ctx context.Context) error
}

type userSubscriptionService struct {
	subRepo  repository.UserSubscriptionRepository
	planRepo interface {
		GetByCode(ctx context.Context, code string) (*domain.Plan, error)
		GetByID(ctx context.Context, id uint) (*domain.Plan, error)
	}
	stripe any
	logger Logger
}

// NewUserSubscriptionService creates a new user subscription service
func NewUserSubscriptionService(
	subRepo repository.UserSubscriptionRepository,
	planRepo interface {
		GetByCode(ctx context.Context, code string) (*domain.Plan, error)
		GetByID(ctx context.Context, id uint) (*domain.Plan, error)
	},
	stripe any,
	logger Logger,
) UserSubscriptionService {
	return &userSubscriptionService{
		subRepo:  subRepo,
		planRepo: planRepo,
		stripe:   stripe,
		logger:   logger,
	}
}

// Create creates a new user subscription
func (s *userSubscriptionService) Create(ctx context.Context, userID uint, planCode string, stripeSubscriptionID string) (*domain.UserSubscription, error) {
	s.logger.Info("user_subscription.create called",
		"user_id", userID,
		"plan_code", planCode,
		"stripe_subscription_id_set", stripeSubscriptionID != "",
	)

	// Idempotency for Stripe retries
	if stripeSubscriptionID != "" {
		if existing, err := s.subRepo.GetByStripeSubscriptionID(ctx, stripeSubscriptionID); err == nil && existing != nil {
			s.logger.Info("user_subscription.create idempotent hit",
				"user_id", userID,
				"stripe_subscription_id", stripeSubscriptionID,
			)
			return existing, nil
		}
	}

	plan, err := s.planRepo.GetByCode(ctx, planCode)
	if err != nil {
		s.logger.Error("user_subscription.create failed to get plan", "user_id", userID, "plan_code", planCode, "error", err)
		return nil, fmt.Errorf("failed to get plan: %w", err)
	}

	if !plan.IsActive {
		s.logger.Warn("user_subscription.create inactive plan", "user_id", userID, "plan_code", planCode)
		return nil, fmt.Errorf("plan is not active")
	}

	now := time.Now()

	// Expire any existing active-like subscriptions
	if err := s.subRepo.ExpireActiveByUserID(ctx, userID, now); err != nil {
		s.logger.Error("user_subscription.create failed to expire existing", "user_id", userID, "error", err)
		return nil, fmt.Errorf("failed to expire existing active subscriptions: %w", err)
	}

	sub := &domain.UserSubscription{
		UUID:                 uuid.NewV4(),
		UserID:               userID,
		PlanID:               plan.ID,
		State:                domain.SubStateActive,
		StripeSubscriptionID: &stripeSubscriptionID,
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
		s.logger.Error("user_subscription.create failed to persist", "user_id", userID, "error", err)
		return nil, fmt.Errorf("failed to create subscription: %w", err)
	}

	s.logger.Info("user_subscription.create ok",
		"user_id", userID,
		"subscription_id", sub.ID,
		"state", sub.State,
	)
	return sub, nil
}

// GetByID retrieves a subscription by ID
func (s *userSubscriptionService) GetByID(ctx context.Context, id uint) (*domain.UserSubscription, error) {
	return s.subRepo.GetByID(ctx, id)
}

// GetByUserID retrieves the active subscription for a user
func (s *userSubscriptionService) GetByUserID(ctx context.Context, userID uint) (*domain.UserSubscription, error) {
	return s.subRepo.GetByUserID(ctx, userID)
}

// Update updates a subscription
func (s *userSubscriptionService) Update(ctx context.Context, sub *domain.UserSubscription) error {
	return s.subRepo.Update(ctx, sub)
}

// Cancel cancels a subscription at the end of the billing period
func (s *userSubscriptionService) Cancel(ctx context.Context, userID uint) error {
	sub, err := s.subRepo.GetByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	if sub.State == domain.SubStateExpired || sub.State == domain.SubStateCanceled {
		return fmt.Errorf("subscription is already canceled or expired")
	}

	if sub.StripeSubscriptionID == nil || *sub.StripeSubscriptionID == "" {
		return fmt.Errorf("subscription has no Stripe subscription ID")
	}

	stripeClient, ok := s.stripe.(*stripe.Client)
	if !ok {
		return fmt.Errorf("stripe client not available")
	}

	s.logger.Infof("🔄 Canceling user Stripe subscription at period end: %s (user_id=%d)",
		*sub.StripeSubscriptionID, userID)

	stripeSub, err := stripeClient.CancelSubscription(*sub.StripeSubscriptionID, true)
	if err != nil {
		s.logger.Error("Failed to cancel Stripe subscription",
			"stripe_subscription_id", *sub.StripeSubscriptionID,
			"user_id", userID,
			"error", err)
		return fmt.Errorf("failed to cancel Stripe subscription: %w", err)
	}

	s.logger.Infof("✅ Stripe subscription canceled successfully: %s (status=%s)",
		stripeSub.ID, stripeSub.Status)

	now := time.Now()
	sub.CancelAt = &now
	sub.State = domain.SubStateCanceled

	if err := s.subRepo.Update(ctx, sub); err != nil {
		s.logger.Error("Failed to update subscription in database after Stripe cancel",
			"user_id", userID,
			"error", err)
		return fmt.Errorf("failed to update subscription in database: %w", err)
	}

	s.logger.Infof("✅ User subscription canceled successfully in database (user_id=%d)", userID)
	return nil
}

// Resume resumes a canceled subscription
func (s *userSubscriptionService) Resume(ctx context.Context, userID uint) error {
	sub, err := s.subRepo.GetByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	if sub.State != domain.SubStateCanceled {
		return fmt.Errorf("can only resume canceled subscriptions (current state: %s)", sub.State)
	}

	if sub.StripeSubscriptionID == nil || *sub.StripeSubscriptionID == "" {
		return fmt.Errorf("subscription has no Stripe subscription ID")
	}

	stripeClient, ok := s.stripe.(*stripe.Client)
	if !ok {
		return fmt.Errorf("stripe client not available")
	}

	s.logger.Infof("🔄 Reactivating user Stripe subscription: %s (user_id=%d)",
		*sub.StripeSubscriptionID, userID)

	stripeSub, err := stripeClient.ReactivateSubscription(*sub.StripeSubscriptionID)
	if err != nil {
		s.logger.Error("Failed to reactivate Stripe subscription",
			"stripe_subscription_id", *sub.StripeSubscriptionID,
			"user_id", userID,
			"error", err)
		return fmt.Errorf("failed to reactivate Stripe subscription: %w", err)
	}

	s.logger.Infof("✅ Stripe subscription reactivated successfully: %s (status=%s)",
		stripeSub.ID, stripeSub.Status)

	sub.State = domain.SubStateActive
	sub.CancelAt = nil

	if err := s.subRepo.Update(ctx, sub); err != nil {
		s.logger.Error("Failed to update subscription in database after Stripe reactivation",
			"user_id", userID,
			"error", err)
		return fmt.Errorf("failed to update subscription in database: %w", err)
	}

	s.logger.Infof("✅ User subscription reactivated successfully in database (user_id=%d)", userID)
	return nil
}

// HandlePaymentSuccess handles successful payment webhook
func (s *userSubscriptionService) HandlePaymentSuccess(ctx context.Context, stripeSubID string) error {
	sub, err := s.subRepo.GetByStripeSubscriptionID(ctx, stripeSubID)
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	now := time.Now()

	switch sub.State {
	case domain.SubStateDraft, domain.SubStateTrialing:
		sub.State = domain.SubStateActive
		sub.StartedAt = &now
	case domain.SubStatePastDue:
		sub.State = domain.SubStateActive
		sub.GracePeriodEndsAt = nil
	case domain.SubStateExpired:
		sub.State = domain.SubStateActive
		sub.StartedAt = &now
		sub.EndedAt = nil
	}

	nextRenew := s.calculateNextRenewal(sub)
	sub.RenewAt = &nextRenew

	return s.subRepo.Update(ctx, sub)
}

// HandlePaymentFailed handles failed payment webhook
func (s *userSubscriptionService) HandlePaymentFailed(ctx context.Context, stripeSubID string) error {
	sub, err := s.subRepo.GetByStripeSubscriptionID(ctx, stripeSubID)
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	sub.State = domain.SubStatePastDue
	gracePeriod := time.Now().AddDate(0, 0, 14)
	sub.GracePeriodEndsAt = &gracePeriod

	return s.subRepo.Update(ctx, sub)
}

// ExpireSubscription expires a subscription
func (s *userSubscriptionService) ExpireSubscription(ctx context.Context, subID uint) error {
	sub, err := s.subRepo.GetByID(ctx, subID)
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	now := time.Now()
	sub.State = domain.SubStateExpired
	sub.EndedAt = &now

	return s.subRepo.Update(ctx, sub)
}

// CheckExpiredSubscriptions checks and expires subscriptions that should be expired
func (s *userSubscriptionService) CheckExpiredSubscriptions(ctx context.Context) error {
	// Find past_due subscriptions with expired grace periods
	pastDueSubs, err := s.subRepo.ListPastDueExpired(ctx)
	if err != nil {
		return fmt.Errorf("failed to list past due subscriptions: %w", err)
	}

	for _, sub := range pastDueSubs {
		if err := s.ExpireSubscription(ctx, sub.ID); err != nil {
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
			continue
		}
	}

	// Find manual subscriptions
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

func (s *userSubscriptionService) calculateNextRenewal(sub *domain.UserSubscription) time.Time {
	if sub.Plan == nil {
		return time.Now().AddDate(0, 1, 0)
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
