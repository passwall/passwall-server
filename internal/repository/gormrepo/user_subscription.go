package gormrepo

import (
	"context"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"gorm.io/gorm"
)

type userSubscriptionRepository struct {
	db *gorm.DB
}

// NewUserSubscriptionRepository creates a new user subscription repository
func NewUserSubscriptionRepository(db *gorm.DB) *userSubscriptionRepository {
	return &userSubscriptionRepository{db: db}
}

// ExpireActiveByUserID expires any active/trialing/past_due subscriptions for the user.
// This enforces the invariant: at most one "active-like" subscription per user.
func (r *userSubscriptionRepository) ExpireActiveByUserID(ctx context.Context, userID uint, endedAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&domain.UserSubscription{}).
		Where("user_id = ? AND state IN ?", userID, []domain.SubscriptionState{
			domain.SubStateActive,
			domain.SubStateTrialing,
			domain.SubStatePastDue,
		}).
		Updates(map[string]any{
			"state":    domain.SubStateExpired,
			"ended_at": &endedAt,
		}).Error
}

// selectEffectiveUserSubscription picks the subscription that should be treated as the
// current source of truth for entitlements.
//
// Priority:
// 1) active / trialing / past_due
// 2) canceled but still within access window (renew_at in the future)
// 3) otherwise the most recent record
func selectEffectiveUserSubscription(subs []*domain.UserSubscription, now time.Time) *domain.UserSubscription {
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
func (r *userSubscriptionRepository) Create(ctx context.Context, sub *domain.UserSubscription) error {
	return r.db.WithContext(ctx).Create(sub).Error
}

// GetByID retrieves a subscription by ID
func (r *userSubscriptionRepository) GetByID(ctx context.Context, id uint) (*domain.UserSubscription, error) {
	var sub domain.UserSubscription
	err := r.db.WithContext(ctx).Preload("Plan").First(&sub, id).Error
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

// GetByUUID retrieves a subscription by UUID
func (r *userSubscriptionRepository) GetByUUID(ctx context.Context, uuid string) (*domain.UserSubscription, error) {
	var sub domain.UserSubscription
	err := r.db.WithContext(ctx).Preload("Plan").Where("uuid = ?", uuid).First(&sub).Error
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

// GetByUserID retrieves the active subscription for a user
func (r *userSubscriptionRepository) GetByUserID(ctx context.Context, userID uint) (*domain.UserSubscription, error) {
	var subs []*domain.UserSubscription
	err := r.db.WithContext(ctx).
		Preload("Plan").
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&subs).Error
	if err != nil {
		return nil, err
	}

	effective := selectEffectiveUserSubscription(subs, time.Now())
	if effective == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return effective, nil
}

// GetByStripeSubscriptionID retrieves a subscription by Stripe subscription ID
func (r *userSubscriptionRepository) GetByStripeSubscriptionID(ctx context.Context, stripeSubID string) (*domain.UserSubscription, error) {
	var sub domain.UserSubscription
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
func (r *userSubscriptionRepository) Update(ctx context.Context, sub *domain.UserSubscription) error {
	return r.db.WithContext(ctx).Save(sub).Error
}

// Delete soft deletes a subscription
func (r *userSubscriptionRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&domain.UserSubscription{}, id).Error
}

// ListPastDueExpired retrieves past_due subscriptions where grace period has ended
func (r *userSubscriptionRepository) ListPastDueExpired(ctx context.Context) ([]*domain.UserSubscription, error) {
	var subs []*domain.UserSubscription
	now := time.Now()
	err := r.db.WithContext(ctx).
		Preload("Plan").
		Preload("User").
		Where("state = ? AND grace_period_ends_at IS NOT NULL AND grace_period_ends_at < ?", domain.SubStatePastDue, now).
		Find(&subs).Error
	return subs, err
}

// ListCanceledExpired retrieves canceled subscriptions where period has ended
func (r *userSubscriptionRepository) ListCanceledExpired(ctx context.Context) ([]*domain.UserSubscription, error) {
	var subs []*domain.UserSubscription
	now := time.Now()
	err := r.db.WithContext(ctx).
		Preload("Plan").
		Preload("User").
		Where("state = ? AND renew_at IS NOT NULL AND renew_at < ?", domain.SubStateCanceled, now).
		Find(&subs).Error
	return subs, err
}

// ListManualExpired retrieves manual (non-Stripe) subscriptions that should be expired.
// Manual grants use renew_at as an end date.
func (r *userSubscriptionRepository) ListManualExpired(ctx context.Context) ([]*domain.UserSubscription, error) {
	var subs []*domain.UserSubscription
	now := time.Now()
	err := r.db.WithContext(ctx).
		Preload("Plan").
		Preload("User").
		Where("stripe_subscription_id IS NULL AND renew_at IS NOT NULL AND renew_at < ? AND state IN ?", now, []domain.SubscriptionState{
			domain.SubStateActive,
			domain.SubStateTrialing,
		}).
		Find(&subs).Error
	return subs, err
}
