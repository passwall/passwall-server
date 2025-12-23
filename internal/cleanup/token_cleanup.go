package cleanup

import (
	"context"
	"time"

	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/pkg/logger"
)

// TokenCleanup handles periodic cleanup of expired tokens
type TokenCleanup struct {
	tokenRepo repository.TokenRepository
	interval  time.Duration
	stopChan  chan struct{}
}

// NewTokenCleanup creates a new token cleanup service
func NewTokenCleanup(tokenRepo repository.TokenRepository, interval time.Duration) *TokenCleanup {
	return &TokenCleanup{
		tokenRepo: tokenRepo,
		interval:  interval,
		stopChan:  make(chan struct{}),
	}
}

// Start begins the periodic cleanup process
func (tc *TokenCleanup) Start(ctx context.Context) {
	logger.Infof("Token cleanup started with interval: %v", tc.interval)

	// Run cleanup immediately on start
	tc.cleanup(ctx)

	ticker := time.NewTicker(tc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tc.cleanup(ctx)
		case <-tc.stopChan:
			logger.Infof("Token cleanup stopped")
			return
		case <-ctx.Done():
			logger.Infof("Token cleanup stopped due to context cancellation")
			return
		}
	}
}

// Stop stops the cleanup process
func (tc *TokenCleanup) Stop() {
	close(tc.stopChan)
}

// cleanup performs the actual cleanup operation
func (tc *TokenCleanup) cleanup(ctx context.Context) {
	logger.Infof("Running token cleanup...")

	deletedCount, err := tc.tokenRepo.DeleteExpired(ctx)
	if err != nil {
		logger.Errorf("Failed to delete expired tokens: %v", err)
		return
	}

	if deletedCount > 0 {
		logger.Infof("Successfully deleted %d expired tokens", deletedCount)
	} else {
		logger.Debugf("No expired tokens found")
	}
}

