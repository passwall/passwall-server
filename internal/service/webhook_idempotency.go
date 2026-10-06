package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
)

// ErrWebhookInProgress is returned while another delivery of the same event
// is still being processed. Handlers must answer with a retryable status so
// the provider redelivers if that first attempt never completes.
var ErrWebhookInProgress = errors.New("webhook event is already being processed")

const webhookReservationTTL = 5 * time.Minute

// reserveWebhookEvent claims an event for processing. It returns skip=true
// when the event was already processed successfully.
func reserveWebhookEvent(
	ctx context.Context,
	repo repository.WebhookEventRepository,
	eventID string,
	eventType string,
) (bool, error) {
	if repo == nil {
		return false, nil
	}
	stored, err := repo.GetByStripeEventID(ctx, eventID)
	switch {
	case err == nil:
		return webhookReservationState(stored)
	case !errors.Is(err, repository.ErrNotFound):
		return false, fmt.Errorf("failed to check webhook idempotency: %w", err)
	}

	reservation := &domain.WebhookEvent{
		StripeEventID: eventID,
		EventType:     eventType,
		Payload:       domain.WebhookPayload(`{}`),
	}
	if createErr := repo.Create(ctx, reservation); createErr != nil {
		existing, lookupErr := repo.GetByStripeEventID(ctx, eventID)
		if lookupErr != nil {
			return false, fmt.Errorf("failed to reserve webhook event: %w", createErr)
		}
		return webhookReservationState(existing)
	}
	return false, nil
}

func webhookReservationState(event *domain.WebhookEvent) (bool, error) {
	if event.IsProcessed() {
		return true, nil
	}
	if !event.HasError() && time.Since(event.CreatedAt) < webhookReservationTTL {
		return false, ErrWebhookInProgress
	}
	return false, nil
}
