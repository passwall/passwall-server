package cleanup

import (
	"context"
	"time"
)

// DraftOrganizationCleanup removes plan-first organizations whose owner never
// completed checkout. Organizations that already hold vault items are kept so
// no encrypted data is discarded.
type DraftOrganizationCleanup struct {
	drafts interface {
		ListAbandonedDraftOrganizationIDs(ctx context.Context, createdBefore time.Time) ([]uint, error)
	}
	orgs interface {
		GetItemCount(ctx context.Context, orgID uint) (int, error)
		Delete(ctx context.Context, id uint) error
	}
	logger interface {
		Info(msg string, args ...interface{})
		Error(msg string, args ...interface{})
	}
	maxAge   time.Duration
	interval time.Duration
	now      func() time.Time
}

// NewDraftOrganizationCleanup creates the abandoned draft organization worker.
func NewDraftOrganizationCleanup(
	drafts interface {
		ListAbandonedDraftOrganizationIDs(ctx context.Context, createdBefore time.Time) ([]uint, error)
	},
	orgs interface {
		GetItemCount(ctx context.Context, orgID uint) (int, error)
		Delete(ctx context.Context, id uint) error
	},
	logger interface {
		Info(msg string, args ...interface{})
		Error(msg string, args ...interface{})
	},
	maxAge time.Duration,
	interval time.Duration,
) *DraftOrganizationCleanup {
	if maxAge <= 0 {
		maxAge = 7 * 24 * time.Hour
	}
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	return &DraftOrganizationCleanup{
		drafts:   drafts,
		orgs:     orgs,
		logger:   logger,
		maxAge:   maxAge,
		interval: interval,
		now:      time.Now,
	}
}

// Run starts the worker and blocks until ctx is canceled.
func (w *DraftOrganizationCleanup) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.RunOnce(ctx)
	for {
		select {
		case <-ticker.C:
			w.RunOnce(ctx)
		case <-ctx.Done():
			return
		}
	}
}

// RunOnce deletes abandoned draft organizations and returns how many were removed.
func (w *DraftOrganizationCleanup) RunOnce(ctx context.Context) int {
	ids, err := w.drafts.ListAbandonedDraftOrganizationIDs(ctx, w.now().Add(-w.maxAge))
	if err != nil {
		w.logger.Error("failed to list abandoned draft organizations", "error", err)
		return 0
	}

	removed := 0
	for _, id := range ids {
		items, err := w.orgs.GetItemCount(ctx, id)
		if err != nil {
			w.logger.Error("failed to count items for draft organization", "org_id", id, "error", err)
			continue
		}
		if items > 0 {
			continue
		}
		if err := w.orgs.Delete(ctx, id); err != nil {
			w.logger.Error("failed to delete abandoned draft organization", "org_id", id, "error", err)
			continue
		}
		removed++
	}
	if removed > 0 {
		w.logger.Info("deleted abandoned draft organizations", "count", removed)
	}
	return removed
}
