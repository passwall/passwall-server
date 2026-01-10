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
	var sub domain.Subscription
	err := r.db.WithContext(ctx).
		Preload("Plan").
		Where("organization_id = ?", orgID).
		Order("created_at DESC").
		First(&sub).Error
	if err != nil {
		return nil, err
	}
	return &sub, nil
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

