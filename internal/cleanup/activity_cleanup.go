package cleanup

import (
	"context"
	"time"

	"github.com/passwall/passwall-server/internal/service"
	"github.com/passwall/passwall-server/pkg/logger"
)

// ActivityCleanup handles periodic cleanup of old user activities
type ActivityCleanup struct {
	activityService service.UserActivityService
	interval        time.Duration
	retentionPeriod time.Duration
	stopChan        chan struct{}
}

// NewActivityCleanup creates a new activity cleanup service
// Recommended: Run every 24 hours, keep activities for 90 days
func NewActivityCleanup(
	activityService service.UserActivityService,
	interval time.Duration,
	retentionPeriod time.Duration,
) *ActivityCleanup {
	return &ActivityCleanup{
		activityService: activityService,
		interval:        interval,
		retentionPeriod: retentionPeriod,
		stopChan:        make(chan struct{}),
	}
}

// Start begins the periodic cleanup process
func (ac *ActivityCleanup) Start(ctx context.Context) {
	logger.Infof("Activity cleanup started (interval: %v, retention: %v)", ac.interval, ac.retentionPeriod)

	// Run cleanup immediately on start
	ac.cleanup(ctx)

	ticker := time.NewTicker(ac.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ac.cleanup(ctx)
		case <-ac.stopChan:
			logger.Infof("Activity cleanup stopped")
			return
		case <-ctx.Done():
			logger.Infof("Activity cleanup stopped due to context cancellation")
			return
		}
	}
}

// Stop stops the cleanup process
func (ac *ActivityCleanup) Stop() {
	close(ac.stopChan)
}

// cleanup performs the actual cleanup operation
func (ac *ActivityCleanup) cleanup(ctx context.Context) {
	logger.Debugf("Running activity cleanup (older than %v)...", ac.retentionPeriod)

	deletedCount, err := ac.activityService.CleanupOldActivities(ctx, ac.retentionPeriod)
	if err != nil {
		logger.Errorf("Failed to cleanup old activities: %v", err)
		return
	}

	if deletedCount > 0 {
		logger.Infof("Successfully cleaned up %d old activities", deletedCount)
	} else {
		logger.Debugf("No old activities to cleanup")
	}
}
