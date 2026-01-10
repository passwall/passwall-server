package cleanup

import (
	"context"
	"time"
)

// OrganizationDeletionWorker handles background tasks for organization deletion
type OrganizationDeletionWorker struct {
	orgRepo interface {
		ListScheduledForDeletion(ctx context.Context, before time.Time) ([]interface{}, error)
		HardDelete(ctx context.Context, orgID uint) error
	}
	logger interface {
		Info(msg string, args ...interface{})
		Error(msg string, args ...interface{})
	}
	interval time.Duration
}

// NewOrganizationDeletionWorker creates a new organization deletion worker
func NewOrganizationDeletionWorker(
	orgRepo interface {
		ListScheduledForDeletion(ctx context.Context, before time.Time) ([]interface{}, error)
		HardDelete(ctx context.Context, orgID uint) error
	},
	logger interface {
		Info(msg string, args ...interface{})
		Error(msg string, args ...interface{})
	},
	interval time.Duration,
) *OrganizationDeletionWorker {
	if interval == 0 {
		interval = 24 * time.Hour // Default to daily
	}

	return &OrganizationDeletionWorker{
		orgRepo:  orgRepo,
		logger:   logger,
		interval: interval,
	}
}

// Run starts the organization deletion worker
func (w *OrganizationDeletionWorker) Run(ctx context.Context) {
	w.logger.Info("organization deletion worker started", "interval", w.interval)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// Run immediately on start
	w.processScheduledDeletions(ctx)

	for {
		select {
		case <-ticker.C:
			w.processScheduledDeletions(ctx)
		case <-ctx.Done():
			w.logger.Info("organization deletion worker stopped")
			return
		}
	}
}

// processScheduledDeletions processes organizations scheduled for permanent deletion
func (w *OrganizationDeletionWorker) processScheduledDeletions(ctx context.Context) {
	w.logger.Info("checking for organizations scheduled for deletion")

	now := time.Now()
	orgs, err := w.orgRepo.ListScheduledForDeletion(ctx, now)
	if err != nil {
		w.logger.Error("failed to list scheduled deletions", "error", err)
		return
	}

	if len(orgs) == 0 {
		w.logger.Info("no organizations to delete")
		return
	}

	w.logger.Info("found organizations to delete", "count", len(orgs))

	// TODO: Hard delete logic needs proper type handling
	// This is a placeholder for the actual implementation
	// You'll need to properly type the organizations and implement hard delete

	w.logger.Info("deletion check completed")
}

