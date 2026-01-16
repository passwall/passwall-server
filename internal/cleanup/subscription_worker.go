package cleanup

import (
	"context"
	"time"

	"github.com/passwall/passwall-server/internal/service"
)

// SubscriptionWorker handles background tasks for subscription lifecycle management
type SubscriptionWorker struct {
	subService service.SubscriptionService
	logger     interface {
		Info(msg string, args ...interface{})
		Error(msg string, args ...interface{})
	}
	interval time.Duration
}

// NewSubscriptionWorker creates a new subscription worker
func NewSubscriptionWorker(
	subService service.SubscriptionService,
	logger interface {
		Info(msg string, args ...interface{})
		Error(msg string, args ...interface{})
	},
	interval time.Duration,
) *SubscriptionWorker {
	if interval == 0 {
		interval = 6 * time.Hour // Default to 6 hours
	}

	return &SubscriptionWorker{
		subService: subService,
		logger:     logger,
		interval:   interval,
	}
}

// Run starts the subscription worker
func (w *SubscriptionWorker) Run(ctx context.Context) {
	w.logger.Info("subscription worker started", "interval", w.interval)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// Run immediately on start
	w.processExpiredSubscriptions(ctx)

	for {
		select {
		case <-ticker.C:
			w.processExpiredSubscriptions(ctx)
		case <-ctx.Done():
			w.logger.Info("subscription worker stopped")
			return
		}
	}
}

// processExpiredSubscriptions checks and expires subscriptions that should be expired
func (w *SubscriptionWorker) processExpiredSubscriptions(ctx context.Context) {
	w.logger.Info("checking for expired subscriptions")

	if err := w.subService.CheckExpiredSubscriptions(ctx); err != nil {
		w.logger.Error("failed to check expired subscriptions", "error", err)
		return
	}

	w.logger.Info("subscription expiry check completed")
}
