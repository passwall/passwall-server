package cleanup

import (
	"context"
	"time"

	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/pkg/logger"
)

// SendCleanup handles periodic cleanup of expired sends
type SendCleanup struct {
	sendRepo repository.SendRepository
	interval time.Duration
	stopChan chan struct{}
}

// NewSendCleanup creates a new send cleanup service
func NewSendCleanup(sendRepo repository.SendRepository, interval time.Duration) *SendCleanup {
	return &SendCleanup{
		sendRepo: sendRepo,
		interval: interval,
		stopChan: make(chan struct{}),
	}
}

// Start begins the periodic cleanup process
func (sc *SendCleanup) Start(ctx context.Context) {
	logger.Infof("Send cleanup started with interval: %v", sc.interval)

	sc.cleanup(ctx)

	ticker := time.NewTicker(sc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sc.cleanup(ctx)
		case <-sc.stopChan:
			logger.Infof("Send cleanup stopped")
			return
		case <-ctx.Done():
			logger.Infof("Send cleanup stopped due to context cancellation")
			return
		}
	}
}

// Stop stops the cleanup process
func (sc *SendCleanup) Stop() {
	close(sc.stopChan)
}

func (sc *SendCleanup) cleanup(ctx context.Context) {
	deletedCount, err := sc.sendRepo.DeleteExpired(ctx)
	if err != nil {
		logger.Errorf("Failed to cleanup expired sends: %v", err)
		return
	}

	if deletedCount > 0 {
		logger.Infof("Successfully cleaned up %d expired sends", deletedCount)
	}
}
