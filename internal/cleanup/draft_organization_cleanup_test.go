package cleanup

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeDraftLister struct {
	ids           []uint
	createdBefore time.Time
}

func (f *fakeDraftLister) ListAbandonedDraftOrganizationIDs(_ context.Context, createdBefore time.Time) ([]uint, error) {
	f.createdBefore = createdBefore
	return f.ids, nil
}

type fakeDraftOrgs struct {
	items   map[uint]int
	failFor uint
	deleted []uint
}

func (f *fakeDraftOrgs) GetItemCount(_ context.Context, orgID uint) (int, error) {
	return f.items[orgID], nil
}

func (f *fakeDraftOrgs) Delete(_ context.Context, id uint) error {
	if id == f.failFor {
		return errors.New("delete failed")
	}
	f.deleted = append(f.deleted, id)
	return nil
}

type quietLogger struct{}

func (quietLogger) Info(string, ...interface{})  {}
func (quietLogger) Error(string, ...interface{}) {}

func TestDraftOrganizationCleanupRunOnce(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	lister := &fakeDraftLister{ids: []uint{1, 2, 3}}
	orgs := &fakeDraftOrgs{items: map[uint]int{2: 5}, failFor: 3}

	worker := NewDraftOrganizationCleanup(lister, orgs, quietLogger{}, 7*24*time.Hour, time.Hour)
	worker.now = func() time.Time { return now }

	if removed := worker.RunOnce(context.Background()); removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	if len(orgs.deleted) != 1 || orgs.deleted[0] != 1 {
		t.Fatalf("deleted = %v, want only org 1 (org 2 has items, org 3 failed)", orgs.deleted)
	}
	if want := now.Add(-7 * 24 * time.Hour); !lister.createdBefore.Equal(want) {
		t.Fatalf("cutoff = %v, want %v", lister.createdBefore, want)
	}
}
