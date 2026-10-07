package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/pkg/stripe"
)

// SubscriptionService handles subscription operations
type SubscriptionService interface {
	Create(ctx context.Context, orgID uint, planCode string, stripeSubscriptionID string, seatsPurchased *int) (*domain.Subscription, error)
	CreateFromProvider(ctx context.Context, orgID uint, planCode string, providerSubscriptionID string, seatsPurchased *int, trialEndsAt *time.Time) (*domain.Subscription, error)
	GetByID(ctx context.Context, id uint) (*domain.Subscription, error)
	GetByOrganizationID(ctx context.Context, orgID uint) (*domain.Subscription, error)
	Update(ctx context.Context, sub *domain.Subscription) error
	UpdateSeatsPurchasedByStripeSubscriptionID(ctx context.Context, stripeSubID string, seatsPurchased *int) error
	SyncProviderPeriod(ctx context.Context, stripeSubID string, periodEnd time.Time, trialEnd *time.Time) error
	Upgrade(ctx context.Context, orgID uint, planCode string) error
	Downgrade(ctx context.Context, orgID uint, planCode string) error
	Cancel(ctx context.Context, orgID uint) error
	Resume(ctx context.Context, orgID uint) error
	Renew(ctx context.Context, subID uint) error
	HandlePaymentSuccess(ctx context.Context, stripeSubID string) error
	HandlePaymentFailed(ctx context.Context, stripeSubID string) error
	HandleProviderCanceled(ctx context.Context, subID uint, endedAt *time.Time) error
	GetByStripeSubscriptionID(ctx context.Context, stripeSubID string) (*domain.Subscription, error)
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
		ListTrialEnding(ctx context.Context, before time.Time) ([]*domain.Subscription, error)
		ListManualExpired(ctx context.Context) ([]*domain.Subscription, error)
	}
	planRepo interface {
		GetByCode(ctx context.Context, code string) (*domain.Plan, error)
		GetByID(ctx context.Context, id uint) (*domain.Plan, error)
	}
	orgRepo interface {
		GetByID(ctx context.Context, id uint) (*domain.Organization, error)
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
	logger    Logger
	txManager interface {
		WithinTx(ctx context.Context, fn func(context.Context) error) error
	}
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
		ListTrialEnding(ctx context.Context, before time.Time) ([]*domain.Subscription, error)
		ListManualExpired(ctx context.Context) ([]*domain.Subscription, error)
	},
	planRepo interface {
		GetByCode(ctx context.Context, code string) (*domain.Plan, error)
		GetByID(ctx context.Context, id uint) (*domain.Plan, error)
	},
	orgRepo interface {
		GetByID(ctx context.Context, id uint) (*domain.Organization, error)
	},
	orgService OrganizationService,
	emailService interface {
		SendPaymentFailedEmail(ctx context.Context, sub *domain.Subscription) error
		SendSubscriptionExpiredEmail(ctx context.Context, sub *domain.Subscription) error
	},
	stripe any,
	logger Logger,
	txManager interface {
		WithinTx(ctx context.Context, fn func(context.Context) error) error
	},
) SubscriptionService {
	return &subscriptionService{
		subRepo:      subRepo,
		planRepo:     planRepo,
		orgRepo:      orgRepo,
		orgService:   orgService,
		emailService: emailService,
		stripe:       stripe,
		logger:       logger,
		txManager:    txManager,
	}
}

// Create creates a new subscription. A trial is derived from the plan catalog.
func (s *subscriptionService) Create(ctx context.Context, orgID uint, planCode string, stripeSubscriptionID string, seatsPurchased *int) (*domain.Subscription, error) {
	return s.create(ctx, orgID, planCode, stripeSubscriptionID, seatsPurchased, nil, true)
}

// CreateFromProvider creates a subscription whose trial is decided by the
// billing provider: trialEndsAt is the provider trial end, or nil when the
// provider started the subscription without a trial.
func (s *subscriptionService) CreateFromProvider(ctx context.Context, orgID uint, planCode string, providerSubscriptionID string, seatsPurchased *int, trialEndsAt *time.Time) (*domain.Subscription, error) {
	return s.create(ctx, orgID, planCode, providerSubscriptionID, seatsPurchased, trialEndsAt, false)
}

func (s *subscriptionService) create(
	ctx context.Context,
	orgID uint,
	planCode string,
	stripeSubscriptionID string,
	seatsPurchased *int,
	providerTrialEnd *time.Time,
	usePlanTrial bool,
) (*domain.Subscription, error) {
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

	// Enforce plan–organization type invariant:
	// personal vaults can only have free/pro plans; multi-user plans require a non-personal org.
	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		s.logger.Error("subscription.create failed to get organization", "org_id", orgID, "error", err)
		return nil, fmt.Errorf("organization not found: %w", err)
	}
	if org.IsPersonal && domain.IsMultiUserPlan(planCode) {
		s.logger.Warn("subscription.create rejected: multi-user plan on personal vault",
			"org_id", orgID, "plan_code", planCode)
		return nil, fmt.Errorf("personal vaults can only use free or pro plans; create a separate organization for %s", planCode)
	}
	if !org.IsPersonal && domain.IsPersonalVaultPlan(planCode) && planCode != string(domain.PlanFree) {
		s.logger.Warn("subscription.create rejected: personal plan on organization",
			"org_id", orgID, "plan_code", planCode)
		return nil, fmt.Errorf("pro plan is only available for personal vaults")
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

	sub := &domain.Subscription{
		UUID:                 uuid.New(),
		OrganizationID:       orgID,
		PlanID:               plan.ID,
		State:                domain.SubStateActive,
		StripeSubscriptionID: &stripeSubscriptionID,
		SeatsPurchased:       seatsPurchased,
		StartedAt:            &now,
	}

	// Handle trial period
	var trialEnd *time.Time
	switch {
	case !usePlanTrial:
		if providerTrialEnd != nil && providerTrialEnd.After(now) {
			trialEnd = providerTrialEnd
		}
	case plan.HasTrial():
		end := now.AddDate(0, 0, plan.TrialDays)
		trialEnd = &end
	}
	if trialEnd != nil {
		sub.State = domain.SubStateTrialing
		sub.TrialEndsAt = trialEnd
		sub.RenewAt = trialEnd
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

	if err := s.withinTx(ctx, func(txCtx context.Context) error {
		// Keep the effective-subscription invariant atomic: a failed insert
		// must not leave the organization without its previous entitlement.
		if err := s.subRepo.ExpireActiveByOrganizationID(txCtx, orgID, now); err != nil {
			return fmt.Errorf("expire existing active subscriptions: %w", err)
		}
		if err := s.subRepo.Create(txCtx, sub); err != nil {
			return fmt.Errorf("create subscription: %w", err)
		}
		return nil
	}); err != nil {
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

func (s *subscriptionService) SyncProviderPeriod(
	ctx context.Context,
	stripeSubID string,
	periodEnd time.Time,
	trialEnd *time.Time,
) error {
	sub, err := s.subRepo.GetByStripeSubscriptionID(ctx, stripeSubID)
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}
	if !periodEnd.IsZero() {
		sub.RenewAt = &periodEnd
	}
	if trialEnd != nil {
		sub.TrialEndsAt = trialEnd
	}
	return s.subRepo.Update(ctx, sub)
}

// GetByID retrieves a subscription by ID
func (s *subscriptionService) GetByID(ctx context.Context, id uint) (*domain.Subscription, error) {
	return s.subRepo.GetByID(ctx, id)
}

// GetByOrganizationID retrieves the active subscription for an organization
func (s *subscriptionService) GetByOrganizationID(ctx context.Context, orgID uint) (*domain.Subscription, error) {
	return s.subRepo.GetByOrganizationID(ctx, orgID)
}

// GetByStripeSubscriptionID retrieves a subscription by its Stripe subscription ID
func (s *subscriptionService) GetByStripeSubscriptionID(ctx context.Context, stripeSubID string) (*domain.Subscription, error) {
	return s.subRepo.GetByStripeSubscriptionID(ctx, stripeSubID)
}

// Update updates a subscription
func (s *subscriptionService) Update(ctx context.Context, sub *domain.Subscription) error {
	return s.subRepo.Update(ctx, sub)
}

// Upgrade upgrades a subscription to a higher plan
func (s *subscriptionService) Upgrade(ctx context.Context, orgID uint, planCode string) error {
	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		return fmt.Errorf("organization not found: %w", err)
	}
	if org.IsPersonal && domain.IsMultiUserPlan(planCode) {
		return fmt.Errorf("personal vaults can only use free or pro plans")
	}
	if !org.IsPersonal && domain.IsPersonalVaultPlan(planCode) && planCode != string(domain.PlanFree) {
		return fmt.Errorf("pro plan is only available for personal vaults")
	}

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
	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		return fmt.Errorf("organization not found: %w", err)
	}
	if org.IsPersonal && domain.IsMultiUserPlan(planCode) {
		return fmt.Errorf("personal vaults can only use free or pro plans")
	}
	if !org.IsPersonal && domain.IsPersonalVaultPlan(planCode) && planCode != string(domain.PlanFree) {
		return fmt.Errorf("pro plan is only available for personal vaults")
	}

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

// Cancel cancels a subscription at the end of the billing period.
// The cancellation is routed to the correct payment provider (Stripe, RevenueCat, or manual).
func (s *subscriptionService) Cancel(ctx context.Context, orgID uint) error {
	// 1. Get subscription from database
	sub, err := s.subRepo.GetByOrganizationID(ctx, orgID)
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	if sub.State == domain.SubStateExpired || sub.State == domain.SubStateCanceled {
		return fmt.Errorf("subscription is already canceled or expired")
	}

	// 2. Detect payment provider and route accordingly
	provider := domain.DetectPaymentProvider(sub.StripeSubscriptionID)

	switch provider {
	case domain.PaymentProviderRevenueCat:
		// RevenueCat subscriptions are managed by App Store / Play Store.
		// They cannot be canceled from our API; the user must cancel from the store.
		store := domain.DetectStoreFromSubscriptionID(sub.StripeSubscriptionID)
		storeName := domain.StoreDisplayName(store)
		s.logger.Infof("Cancel rejected for RevenueCat subscription (org_id=%d, store=%s, subscription_id=%s)",
			orgID, storeName, *sub.StripeSubscriptionID)
		return fmt.Errorf("this subscription is managed by %s. Please cancel it from the %s directly", storeName, storeName)

	case domain.PaymentProviderStripe:
		// Cancel via Stripe API
		stripeClient, ok := s.stripe.(*stripe.Client)
		if !ok {
			return fmt.Errorf("stripe client not available")
		}

		s.logger.Infof("Canceling Stripe subscription at period end: %s (org_id=%d)",
			*sub.StripeSubscriptionID, orgID)

		stripeSub, err := stripeClient.CancelSubscription(*sub.StripeSubscriptionID, true) // true = cancel at period end
		if err != nil {
			s.logger.Error("Failed to cancel Stripe subscription",
				"stripe_subscription_id", *sub.StripeSubscriptionID,
				"org_id", orgID,
				"error", err)
			return fmt.Errorf("failed to cancel Stripe subscription: %w", err)
		}

		s.logger.Infof("Stripe subscription canceled successfully: %s (status=%s, cancel_at_period_end=true)",
			stripeSub.ID, stripeSub.Status)

	case domain.PaymentProviderManual:
		// Manual/admin-granted subscriptions have no external provider to notify.
		s.logger.Infof("Canceling manual subscription (org_id=%d)", orgID)

	default:
		return fmt.Errorf("unknown payment provider for subscription")
	}

	// 3. Update database (after external provider success, if applicable)
	now := time.Now()
	sub.CancelAt = &now
	sub.State = domain.SubStateCanceled

	s.logger.Infof("Marking subscription as CANCELED at period end (org_id=%d, provider=%s, will_expire_at=%v)",
		orgID, provider, sub.RenewAt)

	if err := s.subRepo.Update(ctx, sub); err != nil {
		s.logger.Error("Failed to update subscription in database after cancel",
			"org_id", orgID,
			"provider", provider,
			"error", err)
		return fmt.Errorf("failed to update subscription in database: %w", err)
	}

	s.logger.Infof("Subscription canceled successfully in database (org_id=%d, provider=%s)", orgID, provider)
	return nil
}

// Resume resumes a canceled subscription.
// The reactivation is routed to the correct payment provider (Stripe, RevenueCat, or manual).
func (s *subscriptionService) Resume(ctx context.Context, orgID uint) error {
	// 1. Get subscription from database
	sub, err := s.subRepo.GetByOrganizationID(ctx, orgID)
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	if sub.State != domain.SubStateCanceled {
		return fmt.Errorf("can only resume canceled subscriptions (current state: %s)", sub.State)
	}

	// 2. Detect payment provider and route accordingly
	provider := domain.DetectPaymentProvider(sub.StripeSubscriptionID)

	switch provider {
	case domain.PaymentProviderRevenueCat:
		// RevenueCat subscriptions are managed by App Store / Play Store.
		// They cannot be reactivated from our API; the user must re-subscribe from the store.
		store := domain.DetectStoreFromSubscriptionID(sub.StripeSubscriptionID)
		storeName := domain.StoreDisplayName(store)
		s.logger.Infof("Resume rejected for RevenueCat subscription (org_id=%d, store=%s, subscription_id=%s)",
			orgID, storeName, *sub.StripeSubscriptionID)
		return fmt.Errorf("this subscription is managed by %s. Please resubscribe from the %s directly", storeName, storeName)

	case domain.PaymentProviderStripe:
		// Reactivate via Stripe API
		stripeClient, ok := s.stripe.(*stripe.Client)
		if !ok {
			return fmt.Errorf("stripe client not available")
		}

		s.logger.Infof("Reactivating Stripe subscription: %s (org_id=%d)",
			*sub.StripeSubscriptionID, orgID)

		stripeSub, err := stripeClient.ReactivateSubscription(*sub.StripeSubscriptionID)
		if err != nil {
			s.logger.Error("Failed to reactivate Stripe subscription",
				"stripe_subscription_id", *sub.StripeSubscriptionID,
				"org_id", orgID,
				"error", err)
			return fmt.Errorf("failed to reactivate Stripe subscription: %w", err)
		}

		s.logger.Infof("Stripe subscription reactivated successfully: %s (status=%s)",
			stripeSub.ID, stripeSub.Status)

	case domain.PaymentProviderManual:
		// Manual/admin-granted subscriptions have no external provider to notify.
		s.logger.Infof("Reactivating manual subscription (org_id=%d)", orgID)

	default:
		return fmt.Errorf("unknown payment provider for subscription")
	}

	// 3. Update database (after external provider success, if applicable)
	sub.State = domain.SubStateActive
	sub.CancelAt = nil

	if err := s.subRepo.Update(ctx, sub); err != nil {
		s.logger.Error("Failed to update subscription in database after reactivation",
			"org_id", orgID,
			"provider", provider,
			"error", err)
		return fmt.Errorf("failed to update subscription in database: %w", err)
	}

	s.logger.Infof("Subscription reactivated successfully in database (org_id=%d, provider=%s)", orgID, provider)
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
	case domain.SubStateTrialing:
		// The $0 invoice that opens a provider trial must not end the trial early.
		if sub.TrialEndsAt != nil && sub.TrialEndsAt.After(now) {
			return nil
		}
		sub.State = domain.SubStateActive
		sub.StartedAt = &now

	case domain.SubStateDraft:
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

	// Preserve the provider-synchronized paid-through date when it is still
	// valid. Invoice-only success events may not carry a period end, so fall
	// back to the catalog billing cycle only when no future date is known.
	if sub.RenewAt == nil || !sub.RenewAt.After(now) {
		nextRenew := s.calculateNextRenewal(sub)
		sub.RenewAt = &nextRenew
	}

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
	graceDays := 14
	if sub.Plan != nil {
		graceDays = sub.Plan.GraceDays
	}
	gracePeriod := time.Now().AddDate(0, 0, graceDays)
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
	if sub.State == domain.SubStateExpired {
		return nil
	}
	if sub.Plan == nil {
		sub.Plan, err = s.planRepo.GetByID(ctx, sub.PlanID)
		if err != nil {
			return fmt.Errorf("failed to get subscription plan: %w", err)
		}
	}

	now := time.Now()
	if err := s.withinTx(ctx, func(txCtx context.Context) error {
		sub.State = domain.SubStateExpired
		sub.EndedAt = &now
		if err := s.subRepo.Update(txCtx, sub); err != nil {
			return fmt.Errorf("expire paid subscription: %w", err)
		}

		if sub.Plan.ExpiryBehavior != domain.ExpiryBehaviorDowngradeToFree || sub.Plan.IsFree() {
			return nil
		}
		freePlan, err := s.planRepo.GetByCode(txCtx, "free-monthly")
		if err != nil {
			return fmt.Errorf("load free downgrade plan: %w", err)
		}
		freeSubscription := &domain.Subscription{
			UUID:           uuid.New(),
			OrganizationID: sub.OrganizationID,
			PlanID:         freePlan.ID,
			State:          domain.SubStateActive,
			StartedAt:      &now,
		}
		if err := s.subRepo.Create(txCtx, freeSubscription); err != nil {
			return fmt.Errorf("create free downgrade subscription: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}

	if s.emailService != nil {
		go func() {
			_ = s.emailService.SendSubscriptionExpiredEmail(context.Background(), sub)
		}()
	}
	return nil
}

// HandleProviderCanceled applies a provider cancellation. endedAt is the
// provider's termination time; when it has passed (immediate cancel, refund,
// dispute) access ends now. Without it, access runs until the paid-through date.
func (s *subscriptionService) HandleProviderCanceled(ctx context.Context, subID uint, endedAt *time.Time) error {
	sub, err := s.subRepo.GetByID(ctx, subID)
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}
	now := time.Now()
	if endedAt != nil && !endedAt.After(now) {
		return s.ExpireSubscription(ctx, subID)
	}
	accessUntil := sub.RenewAt
	if endedAt != nil {
		accessUntil = endedAt
	}
	if accessUntil != nil && accessUntil.After(now) {
		until := *accessUntil
		sub.State = domain.SubStateCanceled
		sub.RenewAt = &until
		sub.CancelAt = &until
		return s.subRepo.Update(ctx, sub)
	}
	return s.ExpireSubscription(ctx, subID)
}

func (s *subscriptionService) withinTx(
	ctx context.Context,
	fn func(context.Context) error,
) error {
	if s.txManager == nil {
		return fn(ctx)
	}
	return s.txManager.WithinTx(ctx, fn)
}

// CheckExpiredSubscriptions checks and expires subscriptions that should be expired
func (s *subscriptionService) CheckExpiredSubscriptions(ctx context.Context) error {
	now := time.Now()
	batches := []struct {
		name string
		list func(context.Context) ([]*domain.Subscription, error)
	}{
		// past_due subscriptions whose grace period ended
		{"past_due", s.subRepo.ListPastDueExpired},
		// canceled subscriptions whose paid-through date passed
		{"canceled", s.subRepo.ListCanceledExpired},
		// trials follow the plan expiry behavior (Personal → Free, shared → Frozen)
		{"trial", func(ctx context.Context) ([]*domain.Subscription, error) {
			return s.subRepo.ListTrialEnding(ctx, now)
		}},
		// manual grants whose end date passed
		{"manual", s.subRepo.ListManualExpired},
	}

	var failed int
	for _, batch := range batches {
		subs, err := batch.list(ctx)
		if err != nil {
			return fmt.Errorf("failed to list %s subscriptions to expire: %w", batch.name, err)
		}
		for _, sub := range subs {
			if err := s.ExpireSubscription(ctx, sub.ID); err != nil {
				failed++
				if s.logger != nil {
					s.logger.Error("failed to expire subscription",
						"batch", batch.name,
						"subscription_id", sub.ID,
						"organization_id", sub.OrganizationID,
						"error", err,
					)
				}
			}
		}
	}
	if failed > 0 {
		return fmt.Errorf("failed to expire %d subscription(s)", failed)
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
