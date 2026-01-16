package domain

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// WebhookEvent represents a Stripe webhook event for idempotency
type WebhookEvent struct {
	ID        uint      `gorm:"primary_key" json:"id"`
	CreatedAt time.Time `json:"created_at"`

	StripeEventID string         `json:"stripe_event_id" gorm:"type:varchar(255);uniqueIndex;not null"`
	EventType     string         `json:"event_type" gorm:"type:varchar(100);not null"`
	Payload       WebhookPayload `json:"payload" gorm:"type:jsonb;not null"`
	ProcessedAt   *time.Time     `json:"processed_at,omitempty"`
	Error         *string        `json:"error,omitempty" gorm:"type:text"`
}

// TableName specifies the table name
func (WebhookEvent) TableName() string {
	return "webhook_events"
}

// WebhookPayload represents the webhook payload
type WebhookPayload json.RawMessage

// Scan implements sql.Scanner for WebhookPayload (JSONB)
func (p *WebhookPayload) Scan(value interface{}) error {
	if value == nil {
		*p = nil
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan WebhookPayload: expected []byte, got %T", value)
	}

	*p = bytes
	return nil
}

// Value implements driver.Valuer for WebhookPayload (JSONB)
func (p WebhookPayload) Value() (driver.Value, error) {
	if p == nil {
		return nil, nil
	}
	return json.RawMessage(p).MarshalJSON()
}

// IsProcessed checks if webhook event has been processed
func (w *WebhookEvent) IsProcessed() bool {
	return w.ProcessedAt != nil
}

// HasError checks if webhook processing failed
func (w *WebhookEvent) HasError() bool {
	return w.Error != nil
}

// MarkProcessed marks the webhook as successfully processed
func (w *WebhookEvent) MarkProcessed() {
	now := time.Now()
	w.ProcessedAt = &now
	w.Error = nil
}

// MarkFailed marks the webhook as failed with an error
func (w *WebhookEvent) MarkFailed(err error) {
	errStr := err.Error()
	w.Error = &errStr
}
