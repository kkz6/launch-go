package services

import (
	"testing"

	"github.com/kkz6/launch-go/internal/pkg/dbtype"
)

// First build-secret change on a freshly-synced app starts the
// pending counter at 1.
func TestBumpGHAPendingChanges_StartsAtOne(t *testing.T) {
	cfg, n := bumpGHAPendingChanges(dbtype.JSONMap{})
	if n != 1 {
		t.Fatalf("expected count 1, got %d", n)
	}
	if got := ghaPendingFromConfig(cfg); got != 1 {
		t.Errorf("config counter should be 1, got %d", got)
	}
}

// The counter comes back from the DB as a JSON float64; bumping it
// must read that and increment.
func TestBumpGHAPendingChanges_IncrementsExistingFloat(t *testing.T) {
	cfg, n := bumpGHAPendingChanges(dbtype.JSONMap{"gha_pending_changes": float64(2)})
	if n != 3 {
		t.Fatalf("expected count 3, got %d", n)
	}
	if got := ghaPendingFromConfig(cfg); got != 3 {
		t.Errorf("config counter should be 3, got %d", got)
	}
}

// A nil source_config (legacy rows) must still produce a usable map.
func TestBumpGHAPendingChanges_NilMap(t *testing.T) {
	cfg, n := bumpGHAPendingChanges(nil)
	if n != 1 {
		t.Fatalf("expected count 1, got %d", n)
	}
	if cfg == nil {
		t.Fatal("must return a non-nil map")
	}
}

// Re-sync clears the counter so the banner disappears.
func TestResetGHAPendingChanges_ZeroesCounter(t *testing.T) {
	cfg := resetGHAPendingChanges(dbtype.JSONMap{"gha_pending_changes": float64(5)})
	if got := ghaPendingFromConfig(cfg); got != 0 {
		t.Errorf("config counter should be 0 after reset, got %d", got)
	}
}
