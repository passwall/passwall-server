package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
)

// FailedLoginTracker enforces the failed_login_limit policy. Attempts are
// counted per organization, account and client IP: one person guessing at
// one account is blocked, without letting an attacker lock out a whole
// organization or clear the counter by signing in to another account.
type FailedLoginTracker interface {
	RecordFailedAttempt(ctx context.Context, orgID, userID uint, ip string)
	IsBlocked(ctx context.Context, orgID, userID uint, ip string) (bool, string)
	RecordSuccess(ctx context.Context, orgID, userID uint, ip string)
}

type failedLoginEntry struct {
	Attempts  int
	FirstFail time.Time
	BlockedAt *time.Time
	// KeepUntil is when the entry may be dropped: the end of its block or
	// counting window (blocks can last up to 24 hours).
	KeepUntil time.Time
}

type failedLoginTracker struct {
	policyService OrganizationPolicyService
	mu            sync.RWMutex
	entries       map[string]*failedLoginEntry // key: "orgID:userID:ip"
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

func (t *failedLoginTracker) key(orgID, userID uint, ip string) string {
	return fmt.Sprintf("%d:%d:%s", orgID, userID, ip)
}

func (t *failedLoginTracker) RecordFailedAttempt(ctx context.Context, orgID, userID uint, ip string) {
	config := t.getConfig(ctx, orgID)
	if config == nil {
		return
	}

	k := t.key(orgID, userID, ip)
	t.mu.Lock()
	defer t.mu.Unlock()

	entry, ok := t.entries[k]
	if !ok {
		entry = &failedLoginEntry{FirstFail: time.Now()}
		t.entries[k] = entry
	}

	// A served block starts a fresh count, so repeated guessing is blocked
	// again; otherwise reset once the window has passed.
	windowDuration := time.Duration(config.WindowMinutes) * time.Minute
	blockDuration := time.Duration(config.BlockDurationMinutes) * time.Minute
	blockServed := entry.BlockedAt != nil && time.Since(*entry.BlockedAt) > blockDuration
	if blockServed || (entry.BlockedAt == nil && time.Since(entry.FirstFail) > windowDuration) {
		entry.Attempts = 0
		entry.FirstFail = time.Now()
		entry.BlockedAt = nil
	}

	entry.Attempts++

	entry.KeepUntil = entry.FirstFail.Add(windowDuration)
	if entry.Attempts >= config.MaxAttempts && entry.BlockedAt == nil {
		now := time.Now()
		entry.BlockedAt = &now
	}
	if entry.BlockedAt != nil {
		entry.KeepUntil = entry.BlockedAt.Add(blockDuration)
	}
}

func (t *failedLoginTracker) IsBlocked(ctx context.Context, orgID, userID uint, ip string) (bool, string) {
	config := t.getConfig(ctx, orgID)
	if config == nil {
		return false, ""
	}

	k := t.key(orgID, userID, ip)
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

func (t *failedLoginTracker) RecordSuccess(ctx context.Context, orgID, userID uint, ip string) {
	k := t.key(orgID, userID, ip)
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
	config := &failedLoginConfig{
		MaxAttempts:          5,
		WindowMinutes:        15,
		BlockDurationMinutes: 30,
	}
	data, err := t.policyService.GetPolicyData(ctx, orgID, domain.PolicyFailedLoginLimit)
	if err != nil {
		// Fail closed: without the policy we cannot tell whether limits apply,
		// so the strictest defaults do.
		return config
	}
	if data == nil {
		return nil
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
		t.prune(time.Now())
	}
}

func (t *failedLoginTracker) prune(now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for k, entry := range t.entries {
		if now.After(entry.KeepUntil) {
			delete(t.entries, k)
		}
	}
}
