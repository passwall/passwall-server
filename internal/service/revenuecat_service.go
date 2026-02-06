package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/passwall/passwall-server/internal/config"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/pkg/revenuecat"
	uuid "github.com/satori/go.uuid"
)

// Common errors for RevenueCat service
var (
	ErrInvalidRevenueCatSignature = errors.New("invalid_revenuecat_webhook_signature")
	ErrUserNotFoundForRevenueCat  = errors.New("user_not_found_for_revenuecat_event")
	ErrUnknownRevenueCatProduct   = errors.New("unknown_revenuecat_product")
)

// RevenueCatService handles RevenueCat webhook events for mobile subscriptions
type RevenueCatService interface {
	HandleWebhook(ctx context.Context, payload []byte, signature string) error
}

type revenueCatService struct {
	client                  *revenuecat.Client
	userRepo                repository.UserRepository
	userSubscriptionService UserSubscriptionService
	planRepo                interface {
		GetByCode(ctx context.Context, code string) (*domain.Plan, error)
	}
	activityLogger *ActivityLogger
	config         *config.Config
	logger         Logger
}

// NewRevenueCatService creates a new RevenueCat service
func NewRevenueCatService(
	userRepo repository.UserRepository,
	userSubscriptionService UserSubscriptionService,
	planRepo interface {
		GetByCode(ctx context.Context, code string) (*domain.Plan, error)
	},
	activityService UserActivityService,
	config *config.Config,
	logger Logger,
) RevenueCatService {
	// Initialize RevenueCat client
	client := revenuecat.NewClient(config.RevenueCat.WebhookSecret)

	// Register product mappings from config
	for _, product := range config.RevenueCat.Products {
		client.AddProductMapping(product.ProductID, product.PlanCode, product.BillingCycle)
	}

	return &revenueCatService{
		client:                  client,
		userRepo:                userRepo,
		userSubscriptionService: userSubscriptionService,
		planRepo:                planRepo,
		activityLogger:          NewActivityLogger(activityService),
		config:                  config,
		logger:                  logger,
	}
}

// HandleWebhook processes RevenueCat webhook events
func (s *revenueCatService) HandleWebhook(ctx context.Context, payload []byte, signature string) error {
	s.logger.Info("RevenueCat webhook received", "payload_size", len(payload))

	// Parse and verify webhook
	webhook, err := s.client.ParseWebhook(payload, signature)
	if err != nil {
		if errors.Is(err, revenuecat.ErrInvalidSignature) {
			s.logger.Error("RevenueCat webhook signature verification failed", "error", err)
			return fmt.Errorf("%w: %v", ErrInvalidRevenueCatSignature, err)
		}
		s.logger.Error("RevenueCat webhook parse failed", "error", err)
		return fmt.Errorf("failed to parse webhook: %w", err)
	}

	event := webhook.Event
	s.logger.Info("RevenueCat webhook parsed",
		"event_type", event.Type,
		"event_id", event.ID,
		"app_user_id", event.AppUserID,
		"product_id", event.ProductID,
		"store", event.Store,
		"environment", event.Environment,
	)

	// Handle different event types
	var handlerErr error
	switch event.Type {
	case revenuecat.EventInitialPurchase:
		s.logger.Info("🛒 Processing INITIAL_PURCHASE webhook", "event_id", event.ID)
		handlerErr = s.handleInitialPurchase(ctx, &event)

	case revenuecat.EventRenewal:
		s.logger.Info("🔄 Processing RENEWAL webhook", "event_id", event.ID)
		handlerErr = s.handleRenewal(ctx, &event)

	case revenuecat.EventProductChange:
		s.logger.Info("🔀 Processing PRODUCT_CHANGE webhook", "event_id", event.ID)
		handlerErr = s.handleProductChange(ctx, &event)

	case revenuecat.EventCancellation:
		s.logger.Info("❌ Processing CANCELLATION webhook", "event_id", event.ID)
		handlerErr = s.handleCancellation(ctx, &event)

	case revenuecat.EventUncancellation:
		s.logger.Info("✅ Processing UNCANCELLATION webhook", "event_id", event.ID)
		handlerErr = s.handleUncancellation(ctx, &event)

	case revenuecat.EventBillingIssue:
		s.logger.Info("⚠️  Processing BILLING_ISSUE webhook", "event_id", event.ID)
		handlerErr = s.handleBillingIssue(ctx, &event)

	case revenuecat.EventExpiration:
		s.logger.Info("⏰ Processing EXPIRATION webhook", "event_id", event.ID)
		handlerErr = s.handleExpiration(ctx, &event)

	case revenuecat.EventSubscriptionPaused:
		s.logger.Info("⏸️  Processing SUBSCRIPTION_PAUSED webhook", "event_id", event.ID)
		handlerErr = s.handleSubscriptionPaused(ctx, &event)

	case revenuecat.EventSubscriptionExtended:
		s.logger.Info("📅 Processing SUBSCRIPTION_EXTENDED webhook", "event_id", event.ID)
		handlerErr = s.handleSubscriptionExtended(ctx, &event)

	case revenuecat.EventTest:
		s.logger.Info("🧪 Received TEST webhook", "event_id", event.ID)
		// Test events don't need processing
		return nil

	default:
		s.logger.Info("ℹ️  Unhandled RevenueCat event type (ignored)", "event_type", event.Type, "event_id", event.ID)
		return nil
	}

	if handlerErr != nil {
		s.logger.Error("❌ RevenueCat webhook handler failed",
			"event_type", event.Type,
			"event_id", event.ID,
			"error", handlerErr,
		)
		return handlerErr
	}

	s.logger.Info("✅ RevenueCat webhook processed successfully",
		"event_type", event.Type,
		"event_id", event.ID,
	)
	return nil
}

// handleInitialPurchase handles new subscription purchases
func (s *revenueCatService) handleInitialPurchase(ctx context.Context, event *revenuecat.Event) error {
	// Find user by app_user_id (which is user.uuid in our system)
	user, err := s.findUserByAppUserID(ctx, event.AppUserID)
	if err != nil {
		return err
	}

	// Get plan code from product ID
	planCode, _, err := s.client.GetPlanCode(event.ProductID)
	if err != nil {
		s.logger.Error("Unknown RevenueCat product",
			"product_id", event.ProductID,
			"user_id", user.ID,
		)
		return fmt.Errorf("%w: %s", ErrUnknownRevenueCatProduct, event.ProductID)
	}

	// Generate a unique subscription ID for RevenueCat purchases
	// Format: rc_{store}_{transaction_id}
	rcSubscriptionID := fmt.Sprintf("rc_%s_%s", event.Store, event.GetTransactionID())

	s.logger.Info("Creating subscription for RevenueCat purchase",
		"user_id", user.ID,
		"plan_code", planCode,
		"product_id", event.ProductID,
		"rc_subscription_id", rcSubscriptionID,
	)

	// Create subscription using UserSubscriptionService
	sub, err := s.userSubscriptionService.Create(ctx, user.ID, planCode, rcSubscriptionID)
	if err != nil {
		s.logger.Error("Failed to create subscription for RevenueCat purchase",
			"user_id", user.ID,
			"error", err,
		)
		return fmt.Errorf("failed to create subscription: %w", err)
	}

	// Update subscription with expiration date from RevenueCat
	if expiration := event.GetExpirationTime(); expiration != nil {
		sub.RenewAt = expiration
		if err := s.userSubscriptionService.Update(ctx, sub); err != nil {
			s.logger.Warn("Failed to update subscription expiration",
				"subscription_id", sub.ID,
				"error", err,
			)
		}
	}

	s.logger.Info("✅ RevenueCat subscription created successfully",
		"user_id", user.ID,
		"subscription_id", sub.ID,
		"plan_code", planCode,
		"store", event.Store,
	)

	// Log activity using existing method (org context is empty for personal subscriptions)
	s.activityLogger.LogSubscriptionCreated(ctx, user.ID, "revenuecat", string(event.Store),
		0, "", // orgID, orgName (empty for user subscriptions)
		rcSubscriptionID, planCode, string(event.PeriodType), string(event.Environment))

	return nil
}

// handleRenewal handles subscription renewals
func (s *revenueCatService) handleRenewal(ctx context.Context, event *revenuecat.Event) error {
	user, err := s.findUserByAppUserID(ctx, event.AppUserID)
	if err != nil {
		return err
	}

	rcSubscriptionID := fmt.Sprintf("rc_%s_%s", event.Store, event.GetTransactionID())

	s.logger.Info("Processing subscription renewal",
		"user_id", user.ID,
		"rc_subscription_id", rcSubscriptionID,
	)

	// Handle payment success (reactivates if past_due, updates state)
	if err := s.userSubscriptionService.HandlePaymentSuccess(ctx, rcSubscriptionID); err != nil {
		// If subscription not found, try to create it (late webhook scenario)
		if isNotFoundError(err) {
			s.logger.Warn("Subscription not found for renewal, creating new one",
				"user_id", user.ID,
				"rc_subscription_id", rcSubscriptionID,
			)
			return s.handleInitialPurchase(ctx, event)
		}
		return err
	}

	// Update renewal date
	sub, err := s.userSubscriptionService.GetByUserID(ctx, user.ID)
	if err == nil && sub != nil {
		if expiration := event.GetExpirationTime(); expiration != nil {
			sub.RenewAt = expiration
			sub.GracePeriodEndsAt = nil // Clear grace period on successful renewal
			if err := s.userSubscriptionService.Update(ctx, sub); err != nil {
				s.logger.Warn("Failed to update renewal date",
					"subscription_id", sub.ID,
					"error", err,
				)
			}
		}
	}

	s.logger.Info("✅ RevenueCat subscription renewed successfully",
		"user_id", user.ID,
		"rc_subscription_id", rcSubscriptionID,
	)

	return nil
}

// handleProductChange handles subscription plan changes
func (s *revenueCatService) handleProductChange(ctx context.Context, event *revenuecat.Event) error {
	user, err := s.findUserByAppUserID(ctx, event.AppUserID)
	if err != nil {
		return err
	}

	// Check if new product ID exists
	if event.NewProductID == nil || *event.NewProductID == "" {
		s.logger.Error("Missing new_product_id in product change event",
			"user_id", user.ID,
		)
		return fmt.Errorf("missing new_product_id in product change event")
	}

	// Get new plan code from new product ID
	newPlanCode, _, err := s.client.GetPlanCode(*event.NewProductID)
	if err != nil {
		s.logger.Error("Unknown new product in product change",
			"new_product_id", *event.NewProductID,
			"user_id", user.ID,
		)
		return fmt.Errorf("%w: %s", ErrUnknownRevenueCatProduct, *event.NewProductID)
	}

	// Get current subscription
	sub, err := s.userSubscriptionService.GetByUserID(ctx, user.ID)
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	// Get the new plan
	plan, err := s.planRepo.GetByCode(ctx, newPlanCode)
	if err != nil {
		return fmt.Errorf("plan not found: %w", err)
	}

	// Update subscription with new plan
	sub.PlanID = plan.ID
	if expiration := event.GetExpirationTime(); expiration != nil {
		sub.RenewAt = expiration
	}

	if err := s.userSubscriptionService.Update(ctx, sub); err != nil {
		return fmt.Errorf("failed to update subscription: %w", err)
	}

	s.logger.Info("✅ RevenueCat subscription plan changed",
		"user_id", user.ID,
		"old_product_id", event.ProductID,
		"new_product_id", event.NewProductID,
		"new_plan_code", newPlanCode,
	)

	return nil
}

// handleCancellation handles subscription cancellations
func (s *revenueCatService) handleCancellation(ctx context.Context, event *revenuecat.Event) error {
	user, err := s.findUserByAppUserID(ctx, event.AppUserID)
	if err != nil {
		return err
	}

	sub, err := s.userSubscriptionService.GetByUserID(ctx, user.ID)
	if err != nil {
		// If subscription not found, log and return (idempotent)
		if isNotFoundError(err) {
			s.logger.Warn("Subscription not found for cancellation (already canceled?)",
				"user_id", user.ID,
			)
			return nil
		}
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	// Set subscription to canceled state
	now := time.Now()
	sub.State = domain.SubStateCanceled
	sub.CancelAt = &now

	// Keep access until expiration
	if expiration := event.GetExpirationTime(); expiration != nil {
		sub.RenewAt = expiration
	}

	if err := s.userSubscriptionService.Update(ctx, sub); err != nil {
		return fmt.Errorf("failed to cancel subscription: %w", err)
	}

	s.logger.Info("✅ RevenueCat subscription canceled",
		"user_id", user.ID,
		"subscription_id", sub.ID,
		"cancel_reason", event.CancelReason,
	)

	return nil
}

// handleUncancellation handles subscription reactivations
func (s *revenueCatService) handleUncancellation(ctx context.Context, event *revenuecat.Event) error {
	user, err := s.findUserByAppUserID(ctx, event.AppUserID)
	if err != nil {
		return err
	}

	sub, err := s.userSubscriptionService.GetByUserID(ctx, user.ID)
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	// Reactivate subscription
	sub.State = domain.SubStateActive
	sub.CancelAt = nil

	if expiration := event.GetExpirationTime(); expiration != nil {
		sub.RenewAt = expiration
	}

	if err := s.userSubscriptionService.Update(ctx, sub); err != nil {
		return fmt.Errorf("failed to reactivate subscription: %w", err)
	}

	s.logger.Info("✅ RevenueCat subscription uncanceled",
		"user_id", user.ID,
		"subscription_id", sub.ID,
	)

	return nil
}

// handleBillingIssue handles payment failures
func (s *revenueCatService) handleBillingIssue(ctx context.Context, event *revenuecat.Event) error {
	user, err := s.findUserByAppUserID(ctx, event.AppUserID)
	if err != nil {
		return err
	}

	sub, err := s.userSubscriptionService.GetByUserID(ctx, user.ID)
	if err != nil {
		// If subscription not found, log and return
		if isNotFoundError(err) {
			s.logger.Warn("Subscription not found for billing issue",
				"user_id", user.ID,
			)
			return nil
		}
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	// Set subscription to past_due state
	sub.State = domain.SubStatePastDue

	// Set grace period from RevenueCat or default to 14 days
	if gracePeriod := event.GetGracePeriodExpirationTime(); gracePeriod != nil {
		sub.GracePeriodEndsAt = gracePeriod
	} else {
		gracePeriod := time.Now().AddDate(0, 0, 14)
		sub.GracePeriodEndsAt = &gracePeriod
	}

	if err := s.userSubscriptionService.Update(ctx, sub); err != nil {
		return fmt.Errorf("failed to update subscription: %w", err)
	}

	s.logger.Info("⚠️  RevenueCat billing issue recorded",
		"user_id", user.ID,
		"subscription_id", sub.ID,
		"grace_period_ends_at", sub.GracePeriodEndsAt,
	)

	return nil
}

// handleExpiration handles subscription expirations
func (s *revenueCatService) handleExpiration(ctx context.Context, event *revenuecat.Event) error {
	user, err := s.findUserByAppUserID(ctx, event.AppUserID)
	if err != nil {
		return err
	}

	sub, err := s.userSubscriptionService.GetByUserID(ctx, user.ID)
	if err != nil {
		// If subscription not found, log and return (idempotent)
		if isNotFoundError(err) {
			s.logger.Warn("Subscription not found for expiration (already expired?)",
				"user_id", user.ID,
			)
			return nil
		}
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	// Expire the subscription
	if err := s.userSubscriptionService.ExpireSubscription(ctx, sub.ID); err != nil {
		return fmt.Errorf("failed to expire subscription: %w", err)
	}

	s.logger.Info("⏰ RevenueCat subscription expired",
		"user_id", user.ID,
		"subscription_id", sub.ID,
	)

	return nil
}

// handleSubscriptionPaused handles subscription pauses (Android only)
func (s *revenueCatService) handleSubscriptionPaused(ctx context.Context, event *revenuecat.Event) error {
	user, err := s.findUserByAppUserID(ctx, event.AppUserID)
	if err != nil {
		return err
	}

	sub, err := s.userSubscriptionService.GetByUserID(ctx, user.ID)
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	// Set to canceled state (paused subscriptions don't renew)
	sub.State = domain.SubStateCanceled
	now := time.Now()
	sub.CancelAt = &now

	// Track when it will auto-resume
	if autoResume := event.GetAutoResumeTime(); autoResume != nil {
		// Store auto-resume date in RenewAt for reference
		sub.RenewAt = autoResume
	}

	if err := s.userSubscriptionService.Update(ctx, sub); err != nil {
		return fmt.Errorf("failed to pause subscription: %w", err)
	}

	s.logger.Info("⏸️  RevenueCat subscription paused",
		"user_id", user.ID,
		"subscription_id", sub.ID,
		"auto_resume_at", event.GetAutoResumeTime(),
	)

	return nil
}

// handleSubscriptionExtended handles subscription extensions (promotional)
func (s *revenueCatService) handleSubscriptionExtended(ctx context.Context, event *revenuecat.Event) error {
	user, err := s.findUserByAppUserID(ctx, event.AppUserID)
	if err != nil {
		return err
	}

	sub, err := s.userSubscriptionService.GetByUserID(ctx, user.ID)
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	// Update expiration date
	if expiration := event.GetExpirationTime(); expiration != nil {
		sub.RenewAt = expiration
	}

	// Ensure subscription is active
	if sub.State == domain.SubStateExpired || sub.State == domain.SubStateCanceled {
		sub.State = domain.SubStateActive
		now := time.Now()
		sub.StartedAt = &now
		sub.CancelAt = nil
		sub.EndedAt = nil
	}

	if err := s.userSubscriptionService.Update(ctx, sub); err != nil {
		return fmt.Errorf("failed to extend subscription: %w", err)
	}

	s.logger.Info("📅 RevenueCat subscription extended",
		"user_id", user.ID,
		"subscription_id", sub.ID,
		"new_expiration", event.GetExpirationTime(),
	)

	return nil
}

// findUserByAppUserID finds a user by their RevenueCat app_user_id (which is user.uuid)
func (s *revenueCatService) findUserByAppUserID(ctx context.Context, appUserID string) (*domain.User, error) {
	// Validate UUID format
	_, err := uuid.FromString(appUserID)
	if err != nil {
		s.logger.Error("Invalid app_user_id format (not a valid UUID)",
			"app_user_id", appUserID,
			"error", err,
		)
		return nil, fmt.Errorf("%w: invalid UUID format", ErrUserNotFoundForRevenueCat)
	}

	// Find user by UUID (repository takes string)
	user, err := s.userRepo.GetByUUID(ctx, appUserID)
	if err != nil {
		s.logger.Error("User not found for RevenueCat app_user_id",
			"app_user_id", appUserID,
			"error", err,
		)
		return nil, fmt.Errorf("%w: %s", ErrUserNotFoundForRevenueCat, appUserID)
	}

	return user, nil
}

// isNotFoundError checks if an error is a "not found" error
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return errStr == "record not found" ||
		errStr == "subscription not found" ||
		errStr == "failed to get subscription: record not found"
}
