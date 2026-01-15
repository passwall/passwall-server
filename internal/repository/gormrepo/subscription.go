package gormrepo

import (
	"context"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"gorm.io/gorm"
)

type subscriptionRepository struct {
	db *gorm.DB
}

// NewSubscriptionRepository creates a new subscription repository
func NewSubscriptionRepository(db *gorm.DB) *subscriptionRepository {
	return &subscriptionRepository{db: db}
}

// ExpireActiveByOrganizationID expires any active/trialing/past_due subscriptions for the org.
// This enforces the invariant: at most one "active-like" subscription per organization.
func (r *subscriptionRepository) ExpireActiveByOrganizationID(ctx context.Context, orgID uint, endedAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&domain.Subscription{}).
		Where("organization_id = ? AND state IN ?", orgID, []domain.SubscriptionState{
			domain.SubStateActive,
			domain.SubStateTrialing,
			domain.SubStatePastDue,
		}).
		Updates(map[string]any{
			"state":    domain.SubStateExpired,
			"ended_at": &endedAt,
		}).Error
}

// selectEffectiveSubscription picks the subscription that should be treated as the
// current source of truth for entitlements.
//
// Priority:
// 1) active / trialing / past_due
// 2) canceled but still within access window (renew_at in the future)
// 3) otherwise the most recent record
func selectEffectiveSubscription(subs []*domain.Subscription, now time.Time) *domain.Subscription {
	if len(subs) == 0 {
		return nil
	}

	// subs are expected to be ordered by created_at DESC
	for _, s := range subs {
		if s == nil {
			continue
		}
		switch s.State {
		case domain.SubStateActive, domain.SubStateTrialing, domain.SubStatePastDue:
			return s
		}
	}

	for _, s := range subs {
		if s == nil {
			continue
		}
		if s.State == domain.SubStateCanceled && s.RenewAt != nil && s.RenewAt.After(now) {
			return s
		}
	}

	return subs[0]
}

// Create creates a new subscription
func (r *subscriptionRepository) Create(ctx context.Context, sub *domain.Subscription) error {
	return r.db.WithContext(ctx).Create(sub).Error
}

// GetByID retrieves a subscription by ID
func (r *subscriptionRepository) GetByID(ctx context.Context, id uint) (*domain.Subscription, error) {
	var sub domain.Subscription
	err := r.db.WithContext(ctx).Preload("Plan").First(&sub, id).Error
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

// GetByUUID retrieves a subscription by UUID
func (r *subscriptionRepository) GetByUUID(ctx context.Context, uuid string) (*domain.Subscription, error) {
	var sub domain.Subscription
	err := r.db.WithContext(ctx).Preload("Plan").Where("uuid = ?", uuid).First(&sub).Error
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

// GetByOrganizationID retrieves the active subscription for an organization
func (r *subscriptionRepository) GetByOrganizationID(ctx context.Context, orgID uint) (*domain.Subscription, error) {
	var subs []*domain.Subscription
	err := r.db.WithContext(ctx).
		Preload("Plan").
		Where("organization_id = ?", orgID).
		Order("created_at DESC").
		Find(&subs).Error
	if err != nil {
		return nil, err
	}

	effective := selectEffectiveSubscription(subs, time.Now())
	if effective == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return effective, nil
}

// GetByStripeSubscriptionID retrieves a subscription by Stripe subscription ID
func (r *subscriptionRepository) GetByStripeSubscriptionID(ctx context.Context, stripeSubID string) (*domain.Subscription, error) {
	var sub domain.Subscription
	err := r.db.WithContext(ctx).
		Preload("Plan").
		Where("stripe_subscription_id = ?", stripeSubID).
		First(&sub).Error
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

// Update updates a subscription
func (r *subscriptionRepository) Update(ctx context.Context, sub *domain.Subscription) error {
	return r.db.WithContext(ctx).Save(sub).Error
}

// Delete soft deletes a subscription
func (r *subscriptionRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&domain.Subscription{}, id).Error
}

// ListByState retrieves subscriptions by state
func (r *subscriptionRepository) ListByState(ctx context.Context, state domain.SubscriptionState) ([]*domain.Subscription, error) {
	var subs []*domain.Subscription
	err := r.db.WithContext(ctx).
		Preload("Plan").
		Preload("Organization").
		Where("state = ?", state).
		Find(&subs).Error
	return subs, err
}

// ListPastDueExpired retrieves past_due subscriptions where grace period has ended
func (r *subscriptionRepository) ListPastDueExpired(ctx context.Context) ([]*domain.Subscription, error) {
	var subs []*domain.Subscription
	now := time.Now()
	err := r.db.WithContext(ctx).
		Preload("Plan").
		Preload("Organization").
		Where("state = ? AND grace_period_ends_at IS NOT NULL AND grace_period_ends_at < ?", domain.SubStatePastDue, now).
		Find(&subs).Error
	return subs, err
}

// ListCanceledExpired retrieves canceled subscriptions where period has ended
func (r *subscriptionRepository) ListCanceledExpired(ctx context.Context) ([]*domain.Subscription, error) {
	var subs []*domain.Subscription
	now := time.Now()
	err := r.db.WithContext(ctx).
		Preload("Plan").
		Preload("Organization").
		Where("state = ? AND renew_at IS NOT NULL AND renew_at < ?", domain.SubStateCanceled, now).
		Find(&subs).Error
	return subs, err
}

// ListManualExpired retrieves manual (non-Stripe) subscriptions that should be expired.
// Manual grants use renew_at as an end date.
func (r *subscriptionRepository) ListManualExpired(ctx context.Context) ([]*domain.Subscription, error) {
	var subs []*domain.Subscription
	now := time.Now()
	err := r.db.WithContext(ctx).
		Preload("Plan").
		Preload("Organization").
		Where("stripe_subscription_id IS NULL AND renew_at IS NOT NULL AND renew_at < ? AND state IN ?", now, []domain.SubscriptionState{
			domain.SubStateActive,
			domain.SubStateTrialing,
		}).
		Find(&subs).Error
	return subs, err
}

// ListExpiring retrieves subscriptions expiring before a given date
func (r *subscriptionRepository) ListExpiring(ctx context.Context, before time.Time) ([]*domain.Subscription, error) {
	var subs []*domain.Subscription
	err := r.db.WithContext(ctx).
		Preload("Plan").
		Preload("Organization").
		Where("renew_at IS NOT NULL AND renew_at < ? AND state IN (?)", before, []domain.SubscriptionState{
			domain.SubStateActive,
			domain.SubStateTrialing,
		}).
		Find(&subs).Error
	return subs, err
}

// ListTrialEnding retrieves trial subscriptions ending before a given date
func (r *subscriptionRepository) ListTrialEnding(ctx context.Context, before time.Time) ([]*domain.Subscription, error) {
	var subs []*domain.Subscription
	err := r.db.WithContext(ctx).
		Preload("Plan").
		Preload("Organization").
		Where("state = ? AND trial_ends_at IS NOT NULL AND trial_ends_at < ?", domain.SubStateTrialing, before).
		Find(&subs).Error
	return subs, err
}

