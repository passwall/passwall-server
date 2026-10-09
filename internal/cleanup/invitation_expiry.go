package cleanup

import (
	"context"
	"time"
)

// InvitationExpiry marks pending organization invitations past their expiry
// as expired so admin listings and seat counts stay accurate. Reads already
// treat overdue rows as expired; this keeps the stored status in sync.
type InvitationExpiry struct {
	invitations interface {
		ExpireOverdueInvitations(ctx context.Context) (int64, error)
	}
	logger interface {
		Info(msg string, args ...interface{})
		Error(msg string, args ...interface{})
	}
	interval time.Duration
}

// NewInvitationExpiry creates the invitation expiry worker.
func NewInvitationExpiry(
	invitations interface {
		ExpireOverdueInvitations(ctx context.Context) (int64, error)
	},
	logger interface {
		Info(msg string, args ...interface{})
		Error(msg string, args ...interface{})
	},
	interval time.Duration,
) *InvitationExpiry {
	if interval <= 0 {
		interval = time.Hour
	}
	return &InvitationExpiry{invitations: invitations, logger: logger, interval: interval}
}

// Run starts the worker and blocks until ctx is canceled.
func (w *InvitationExpiry) Run(ctx context.Context) {
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

// RunOnce expires overdue invitations.
func (w *InvitationExpiry) RunOnce(ctx context.Context) {
	expired, err := w.invitations.ExpireOverdueInvitations(ctx)
	if err != nil {
		w.logger.Error("failed to expire organization invitations", "error", err)
		return
	}
	if expired > 0 {
		w.logger.Info("expired organization invitations", "count", expired)
	}
}
