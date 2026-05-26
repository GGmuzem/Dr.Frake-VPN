package agent

import (
	"context"
	"testing"
)

type fakeUpdater struct {
	applied   []string
	rollback []string
}

func (u *fakeUpdater) Apply(_ context.Context, digest string) error {
	u.applied = append(u.applied, digest)
	return nil
}

func (u *fakeUpdater) Rollback(_ context.Context, digest string) error {
	u.rollback = append(u.rollback, digest)
	return nil
}

func TestUpdateManagerAppliesImmutableDigest(t *testing.T) {
	updater := &fakeUpdater{}
	manager := NewUpdateManager(updater)

	result := manager.Apply(context.Background(), "ghcr.io/ggmuzem/fblink-node-agent@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if result.Status != UpdateStatusSucceeded {
		t.Fatalf("expected success, got %#v", result)
	}
	if len(updater.applied) != 1 {
		t.Fatalf("expected updater call, got %d", len(updater.applied))
	}
}

func TestUpdateManagerRejectsMutableTag(t *testing.T) {
	updater := &fakeUpdater{}
	manager := NewUpdateManager(updater)

	result := manager.Apply(context.Background(), "ghcr.io/ggmuzem/fblink-node-agent:latest")
	if result.Status != UpdateStatusFailed {
		t.Fatalf("expected failure, got %#v", result)
	}
	if len(updater.applied) != 0 {
		t.Fatal("mutable tag must not be applied")
	}
}

func TestUpdateManagerRollsBackToPreviousDigest(t *testing.T) {
	updater := &fakeUpdater{}
	manager := NewUpdateManager(updater)
	manager.state.PreviousDigest = "ghcr.io/ggmuzem/fblink-node-agent@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	result := manager.Rollback(context.Background())
	if result.Status != UpdateStatusRolledBack {
		t.Fatalf("expected rollback, got %#v", result)
	}
	if len(updater.rollback) != 1 {
		t.Fatalf("expected rollback call, got %d", len(updater.rollback))
	}
}
