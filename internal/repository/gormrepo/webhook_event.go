package gormrepo

import (
	"context"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"gorm.io/gorm"
)

type webhookEventRepository struct {
	db *gorm.DB
}

// NewWebhookEventRepository creates a new webhook event repository
func NewWebhookEventRepository(db *gorm.DB) *webhookEventRepository {
	return &webhookEventRepository{db: db}
}

// Create creates a new webhook event
func (r *webhookEventRepository) Create(ctx context.Context, event *domain.WebhookEvent) error {
	return r.db.WithContext(ctx).Create(event).Error
}

// GetByID retrieves a webhook event by ID
func (r *webhookEventRepository) GetByID(ctx context.Context, id uint) (*domain.WebhookEvent, error) {
	var event domain.WebhookEvent
	err := r.db.WithContext(ctx).First(&event, id).Error
	if err != nil {
		return nil, err
	}
	return &event, nil
}

// GetByStripeEventID retrieves a webhook event by Stripe event ID
func (r *webhookEventRepository) GetByStripeEventID(ctx context.Context, stripeEventID string) (*domain.WebhookEvent, error) {
	var event domain.WebhookEvent
	err := r.db.WithContext(ctx).Where("stripe_event_id = ?", stripeEventID).First(&event).Error
	if err != nil {
		return nil, err
	}
	return &event, nil
}

// Exists checks if a webhook event exists by Stripe event ID
func (r *webhookEventRepository) Exists(ctx context.Context, stripeEventID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&domain.WebhookEvent{}).
		Where("stripe_event_id = ?", stripeEventID).
		Count(&count).Error
	return count > 0, err
}

// MarkProcessed marks a webhook event as processed
func (r *webhookEventRepository) MarkProcessed(ctx context.Context, stripeEventID string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&domain.WebhookEvent{}).
		Where("stripe_event_id = ?", stripeEventID).
		Updates(map[string]interface{}{
			"processed_at": now,
			"error":        nil,
		}).Error
}

// MarkFailed marks a webhook event as failed with an error message
func (r *webhookEventRepository) MarkFailed(ctx context.Context, stripeEventID string, errMsg string) error {
	return r.db.WithContext(ctx).
		Model(&domain.WebhookEvent{}).
		Where("stripe_event_id = ?", stripeEventID).
		Update("error", errMsg).Error
}

// Update updates a webhook event
func (r *webhookEventRepository) Update(ctx context.Context, event *domain.WebhookEvent) error {
	return r.db.WithContext(ctx).Save(event).Error
}

// ListUnprocessed retrieves unprocessed webhook events
func (r *webhookEventRepository) ListUnprocessed(ctx context.Context) ([]*domain.WebhookEvent, error) {
	var events []*domain.WebhookEvent
	err := r.db.WithContext(ctx).
		Where("processed_at IS NULL AND error IS NULL").
		Order("created_at ASC").
		Find(&events).Error
	return events, err
}

// ListFailed retrieves failed webhook events
func (r *webhookEventRepository) ListFailed(ctx context.Context) ([]*domain.WebhookEvent, error) {
	var events []*domain.WebhookEvent
	err := r.db.WithContext(ctx).
		Where("error IS NOT NULL").
		Order("created_at DESC").
		Find(&events).Error
	return events, err
}

// DeleteOld deletes webhook events older than the specified duration
func (r *webhookEventRepository) DeleteOld(ctx context.Context, olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)
	return r.db.WithContext(ctx).
		Where("created_at < ? AND processed_at IS NOT NULL", cutoff).
		Delete(&domain.WebhookEvent{}).Error
}

