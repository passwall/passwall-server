package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
)

// FailedLoginTracker tracks failed login attempts per IP per organization and
// temporarily blocks IPs that exceed the configured threshold.
type FailedLoginTracker interface {
	RecordFailedAttempt(ctx context.Context, orgID uint, ip string)
	IsBlocked(ctx context.Context, orgID uint, ip string) (bool, string)
	RecordSuccess(ctx context.Context, orgID uint, ip string)
}

type failedLoginEntry struct {
	Attempts  int
	FirstFail time.Time
	BlockedAt *time.Time
}

type failedLoginTracker struct {
	policyService OrganizationPolicyService
	mu            sync.RWMutex
	entries       map[string]*failedLoginEntry // key: "orgID:ip"
}

// NewFailedLoginTracker creates a new failed login tracker
func NewFailedLoginTracker(policyService OrganizationPolicyService) FailedLoginTracker {
	t := &failedLoginTracker{
		policyService: policyService,
		entries:       make(map[string]*failedLoginEntry),
	}
	go t.cleanup()
	return t
}

func (t *failedLoginTracker) key(orgID uint, ip string) string {
	return fmt.Sprintf("%d:%s", orgID, ip)
}

func (t *failedLoginTracker) RecordFailedAttempt(ctx context.Context, orgID uint, ip string) {
	config := t.getConfig(ctx, orgID)
	if config == nil {
		return
	}

	k := t.key(orgID, ip)
	t.mu.Lock()
	defer t.mu.Unlock()

	entry, ok := t.entries[k]
	if !ok {
		entry = &failedLoginEntry{FirstFail: time.Now()}
		t.entries[k] = entry
	}

	// Reset window if outside the time window
	windowDuration := time.Duration(config.WindowMinutes) * time.Minute
	if time.Since(entry.FirstFail) > windowDuration {
		entry.Attempts = 0
		entry.FirstFail = time.Now()
		entry.BlockedAt = nil
	}

	entry.Attempts++

	if entry.Attempts >= config.MaxAttempts && entry.BlockedAt == nil {
		now := time.Now()
		entry.BlockedAt = &now
	}
}

func (t *failedLoginTracker) IsBlocked(ctx context.Context, orgID uint, ip string) (bool, string) {
	config := t.getConfig(ctx, orgID)
	if config == nil {
		return false, ""
	}

	k := t.key(orgID, ip)
	t.mu.RLock()
	defer t.mu.RUnlock()

	entry, ok := t.entries[k]
	if !ok {
		return false, ""
	}

	if entry.BlockedAt == nil {
		return false, ""
	}

	blockDuration := time.Duration(config.BlockDurationMinutes) * time.Minute
	if time.Since(*entry.BlockedAt) > blockDuration {
		return false, ""
	}

	remaining := blockDuration - time.Since(*entry.BlockedAt)
	return true, fmt.Sprintf("too many failed login attempts, try again in %d minutes", int(remaining.Minutes())+1)
}

func (t *failedLoginTracker) RecordSuccess(ctx context.Context, orgID uint, ip string) {
	k := t.key(orgID, ip)
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.entries, k)
}

type failedLoginConfig struct {
	MaxAttempts          int
	WindowMinutes        int
	BlockDurationMinutes int
}

func (t *failedLoginTracker) getConfig(ctx context.Context, orgID uint) *failedLoginConfig {
	data, err := t.policyService.GetPolicyData(ctx, orgID, domain.PolicyFailedLoginLimit)
	if err != nil || data == nil {
		return nil
	}

	config := &failedLoginConfig{
		MaxAttempts:          5,
		WindowMinutes:        15,
		BlockDurationMinutes: 30,
	}

	if v, ok := data["max_attempts"].(float64); ok && v > 0 {
		config.MaxAttempts = int(v)
	}
	if v, ok := data["window_minutes"].(float64); ok && v > 0 {
		config.WindowMinutes = int(v)
	}
	if v, ok := data["block_duration_minutes"].(float64); ok && v > 0 {
		config.BlockDurationMinutes = int(v)
	}

	return config
}

// cleanup periodically removes expired entries to prevent memory leaks
func (t *failedLoginTracker) cleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		t.mu.Lock()
		now := time.Now()
		for k, entry := range t.entries {
			maxAge := 2 * time.Hour
			if entry.BlockedAt != nil {
				if now.Sub(*entry.BlockedAt) > maxAge {
					delete(t.entries, k)
				}
			} else if now.Sub(entry.FirstFail) > maxAge {
				delete(t.entries, k)
			}
		}
		t.mu.Unlock()
	}
}
