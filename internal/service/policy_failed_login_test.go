package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/stretchr/testify/assert"
)

type stubPolicyData struct {
	OrganizationPolicyService
	data domain.PolicyData
	err  error
}

func (s stubPolicyData) GetPolicyData(context.Context, uint, domain.PolicyType) (domain.PolicyData, error) {
	return s.data, s.err
}

func newTestTracker(policy OrganizationPolicyService) *failedLoginTracker {
	return &failedLoginTracker{policyService: policy, entries: make(map[string]*failedLoginEntry)}
}

func TestFailedLoginBlocksOneAccountFromOneIP(t *testing.T) {
	ctx := context.Background()
	tracker := newTestTracker(stubPolicyData{data: domain.PolicyData{"max_attempts": float64(3), "window_minutes": float64(15), "block_duration_minutes": float64(30)}})
	const org, victim, other = 1, 10, 11

	for i := 0; i < 3; i++ {
		tracker.RecordFailedAttempt(ctx, org, victim, "198.51.100.1")
	}
	blocked, _ := tracker.IsBlocked(ctx, org, victim, "198.51.100.1")
	assert.True(t, blocked)

	// Other members (and the same member elsewhere) are not locked out.
	blocked, _ = tracker.IsBlocked(ctx, org, other, "198.51.100.1")
	assert.False(t, blocked)
	blocked, _ = tracker.IsBlocked(ctx, org, victim, "203.0.113.5")
	assert.False(t, blocked)

	// Another account signing in from the same IP does not clear the block.
	tracker.RecordSuccess(ctx, org, other, "198.51.100.1")
	blocked, _ = tracker.IsBlocked(ctx, org, victim, "198.51.100.1")
	assert.True(t, blocked)
}

func TestFailedLoginBlocksAgainAfterBlockExpires(t *testing.T) {
	ctx := context.Background()
	tracker := newTestTracker(stubPolicyData{data: domain.PolicyData{"max_attempts": float64(3), "window_minutes": float64(60), "block_duration_minutes": float64(5)}})
	for i := 0; i < 3; i++ {
		tracker.RecordFailedAttempt(ctx, 1, 10, "ip")
	}
	// The block was served long ago, but the window is still open.
	served := time.Now().Add(-10 * time.Minute)
	tracker.entries[tracker.key(1, 10, "ip")].BlockedAt = &served

	blocked, _ := tracker.IsBlocked(ctx, 1, 10, "ip")
	assert.False(t, blocked)
	for i := 0; i < 3; i++ {
		tracker.RecordFailedAttempt(ctx, 1, 10, "ip")
	}
	blocked, _ = tracker.IsBlocked(ctx, 1, 10, "ip")
	assert.True(t, blocked, "repeated guessing after a block is blocked again")
}

func TestFailedLoginFailsClosedOnPolicyErrors(t *testing.T) {
	ctx := context.Background()
	tracker := newTestTracker(stubPolicyData{err: errors.New("db down")})
	for i := 0; i < 5; i++ {
		tracker.RecordFailedAttempt(ctx, 1, 10, "ip")
	}
	blocked, _ := tracker.IsBlocked(ctx, 1, 10, "ip")
	assert.True(t, blocked)

	// Without the policy there are no limits.
	disabled := newTestTracker(stubPolicyData{})
	for i := 0; i < 10; i++ {
		disabled.RecordFailedAttempt(ctx, 1, 10, "ip")
	}
	blocked, _ = disabled.IsBlocked(ctx, 1, 10, "ip")
	assert.False(t, blocked)
}

func TestFailedLoginLongBlockSurvivesCleanup(t *testing.T) {
	ctx := context.Background()
	tracker := newTestTracker(stubPolicyData{data: domain.PolicyData{"max_attempts": float64(3), "window_minutes": float64(15), "block_duration_minutes": float64(1440)}})
	for i := 0; i < 3; i++ {
		tracker.RecordFailedAttempt(ctx, 1, 10, "198.51.100.1")
	}
	tracker.prune(time.Now().Add(3 * time.Hour))
	blocked, _ := tracker.IsBlocked(ctx, 1, 10, "198.51.100.1")
	assert.True(t, blocked, "a 24h block outlives the old 2h cleanup")

	tracker.prune(time.Now().Add(25 * time.Hour))
	assert.Empty(t, tracker.entries)
}
