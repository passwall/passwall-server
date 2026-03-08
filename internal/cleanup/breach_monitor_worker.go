package cleanup

import (
	"context"
	"time"

	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
	"github.com/passwall/passwall-server/pkg/logger"
)

// BreachMonitorWorker periodically rechecks monitored emails against HIBP.
type BreachMonitorWorker struct {
	breachMonitorRepo repository.BreachMonitorRepository
	breachMonitorSvc  service.BreachMonitorService
	featureSvc        service.FeatureService
	interval          time.Duration
	stopChan          chan struct{}
}

// NewBreachMonitorWorker creates a new breach monitor background worker.
func NewBreachMonitorWorker(
	repo repository.BreachMonitorRepository,
	svc service.BreachMonitorService,
	featureSvc service.FeatureService,
	interval time.Duration,
) *BreachMonitorWorker {
	return &BreachMonitorWorker{
		breachMonitorRepo: repo,
		breachMonitorSvc:  svc,
		featureSvc:        featureSvc,
		interval:          interval,
		stopChan:          make(chan struct{}),
	}
}

// Start begins the periodic breach check process.
func (w *BreachMonitorWorker) Start(ctx context.Context) {
	logger.Infof("Breach monitor worker started with interval: %v", w.interval)

	w.check(ctx)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.check(ctx)
		case <-w.stopChan:
			logger.Infof("Breach monitor worker stopped")
			return
		case <-ctx.Done():
			logger.Infof("Breach monitor worker stopped due to context cancellation")
			return
		}
	}
}

// Stop stops the worker.
func (w *BreachMonitorWorker) Stop() {
	close(w.stopChan)
}

func (w *BreachMonitorWorker) check(ctx context.Context) {
	emails, err := w.breachMonitorRepo.ListAllMonitoredEmails(ctx)
	if err != nil {
		logger.Errorf("Breach monitor: failed to list emails: %v", err)
		return
	}

	if len(emails) == 0 {
		return
	}

	totalNew := 0
	for _, email := range emails {
		canUse, err := w.featureSvc.CanUseBreachMonitoring(ctx, email.OrganizationID)
		if err != nil || !canUse {
			continue
		}

		newBreaches, err := w.breachMonitorSvc.CheckSingleEmail(ctx, email)
		if err != nil {
			logger.Errorf("Breach monitor: check failed for email ID %d: %v", email.ID, err)
			continue
		}
		totalNew += newBreaches
	}

	if totalNew > 0 {
		logger.Infof("Breach monitor: found %d new breaches across %d emails", totalNew, len(emails))
	}
}
