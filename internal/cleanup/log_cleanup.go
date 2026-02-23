package cleanup

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/passwall/passwall-server/pkg/logger"
)

// LogCleanup truncates configured log files on a fixed interval.
// Files are truncated in place (not deleted/rotated) so active writers
// continue appending to the same file handles.
type LogCleanup struct {
	paths    []string
	interval time.Duration
	stopChan chan struct{}
}

func NewLogCleanup(paths []string, interval time.Duration) *LogCleanup {
	deduped := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		deduped = append(deduped, p)
	}

	return &LogCleanup{
		paths:    deduped,
		interval: interval,
		stopChan: make(chan struct{}),
	}
}

func (lc *LogCleanup) Start(ctx context.Context) {
	logger.Infof("Log cleanup started with interval: %v", lc.interval)

	ticker := time.NewTicker(lc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			lc.cleanup()
		case <-lc.stopChan:
			logger.Infof("Log cleanup stopped")
			return
		case <-ctx.Done():
			logger.Infof("Log cleanup stopped due to context cancellation")
			return
		}
	}
}

func (lc *LogCleanup) Stop() {
	close(lc.stopChan)
}

func (lc *LogCleanup) cleanup() {
	if len(lc.paths) == 0 {
		logger.Warnf("Skipping log cleanup: no log paths configured")
		return
	}

	cleared := 0
	for _, path := range lc.paths {
		if err := truncateInPlace(path); err != nil {
			logger.Errorf("Failed to truncate log file %q: %v", path, err)
			continue
		}
		cleared++
		logger.Infof("Truncated log file: %s", filepath.Base(path))
	}

	logger.Infof("Log cleanup completed: %d/%d files truncated", cleared, len(lc.paths))
}

func truncateInPlace(path string) error {
	f, err := os.OpenFile(path, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	return f.Close()
}
