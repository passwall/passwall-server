package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	revenuecatClient "github.com/passwall/passwall-server/pkg/revenuecat"
	stripeClient "github.com/passwall/passwall-server/pkg/stripe"
	stripeSDK "github.com/stripe/stripe-go/v81"
	stripeWebhook "github.com/stripe/stripe-go/v81/webhook"
)

type webhookEventRepoStub struct {
	events         map[string]*domain.WebhookEvent
	createCount    int
	processedCount int
}

func newWebhookEventRepoStub() *webhookEventRepoStub {
	return &webhookEventRepoStub{events: make(map[string]*domain.WebhookEvent)}
}

func (r *webhookEventRepoStub) Create(_ context.Context, event *domain.WebhookEvent) error {
	if _, exists := r.events[event.StripeEventID]; exists {
		return errors.New("duplicate webhook event")
	}
	clone := *event
	clone.CreatedAt = time.Now()
	r.events[event.StripeEventID] = &clone
	r.createCount++
	return nil
}

func (r *webhookEventRepoStub) GetByStripeEventID(
	_ context.Context,
	eventID string,
) (*domain.WebhookEvent, error) {
	event, exists := r.events[eventID]
	if !exists {
		return nil, repository.ErrNotFound
	}
	return event, nil
}

func (r *webhookEventRepoStub) MarkProcessed(_ context.Context, eventID string) error {
	event := r.events[eventID]
	event.MarkProcessed()
	r.processedCount++
	return nil
}

func (r *webhookEventRepoStub) MarkFailed(_ context.Context, eventID, message string) error {
	event := r.events[eventID]
	event.Error = &message
	return nil
}

func TestStripeWebhookReplayIsProcessedOnce(t *testing.T) {
	const secret = "whsec_test"
	payload := []byte(`{
		"id":"evt_replay",
		"object":"event",
		"api_version":"` + stripeSDK.APIVersion + `",
		"type":"passwall.test",
		"data":{"object":{}}
	}`)
	signed := stripeWebhook.GenerateTestSignedPayload(&stripeWebhook.UnsignedPayload{
		Payload: payload,
		Secret:  secret,
	})
	events := newWebhookEventRepoStub()
	service := &paymentService{
		stripe:           stripeClient.NewClient("sk_test_placeholder", secret),
		webhookEventRepo: events,
		logger:           noopLogger{},
	}

	for range 2 {
		if err := service.HandleWebhook(context.Background(), payload, signed.Header); err != nil {
			t.Fatalf("HandleWebhook() error = %v", err)
		}
	}
	if events.createCount != 1 || events.processedCount != 1 {
		t.Fatalf(
			"Stripe replay counts create=%d processed=%d, want 1/1",
			events.createCount,
			events.processedCount,
		)
	}
}

func TestRevenueCatWebhookReplayIsProcessedOnce(t *testing.T) {
	const secret = "revenuecat-test-secret"
	payload, err := json.Marshal(revenuecatClient.WebhookEvent{
		APIVersion: "1.0",
		Event: revenuecatClient.Event{
			ID:        "rc_replay",
			Type:      revenuecatClient.EventTest,
			Timestamp: time.Now().UnixMilli(),
		},
	})
	if err != nil {
		t.Fatalf("marshal RevenueCat event: %v", err)
	}
	events := newWebhookEventRepoStub()
	service := &revenueCatService{
		client:           revenuecatClient.NewClient(secret),
		webhookEventRepo: events,
		logger:           noopLogger{},
	}

	for range 2 {
		if err := service.HandleWebhook(context.Background(), payload, secret); err != nil {
			t.Fatalf("HandleWebhook() error = %v", err)
		}
	}
	if events.createCount != 1 || events.processedCount != 1 {
		t.Fatalf(
			"RevenueCat replay counts create=%d processed=%d, want 1/1",
			events.createCount,
			events.processedCount,
		)
	}
}
