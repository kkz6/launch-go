package services

import "github.com/kkz6/launch-go/internal/pkg/dbtype"

// ghaPendingChangesKey is the source_config key holding the count of
// build-secret mutations made since the last successful workflow
// re-sync. The dto layer reads the same key to derive gha_out_of_sync.
const ghaPendingChangesKey = "gha_pending_changes"

// ghaPendingFromConfig reads the pending-changes counter, tolerating
// the float64 that JSON decoding produces for stored numbers.
func ghaPendingFromConfig(cfg dbtype.JSONMap) int {
	if cfg == nil {
		return 0
	}
	switch v := cfg[ghaPendingChangesKey].(type) {
	case int:
		return v
	case float64:
		return int(v)
	default:
		return 0
	}
}

// bumpGHAPendingChanges increments the "build-secret changes since
// last sync" counter on an application's source_config, returning the
// (possibly newly-allocated) map and the new count. Called on every
// build-secret create/update/delete for a GitHub Actions app so the
// UI shows a "re-sync workflow" banner instead of committing the
// workflow YAML on every change.
func bumpGHAPendingChanges(cfg dbtype.JSONMap) (dbtype.JSONMap, int) {
	if cfg == nil {
		cfg = dbtype.JSONMap{}
	}
	n := ghaPendingFromConfig(cfg) + 1
	cfg[ghaPendingChangesKey] = n
	return cfg, n
}

// resetGHAPendingChanges zeroes the counter after a successful
// workflow re-sync so the banner clears.
func resetGHAPendingChanges(cfg dbtype.JSONMap) dbtype.JSONMap {
	if cfg == nil {
		cfg = dbtype.JSONMap{}
	}
	cfg[ghaPendingChangesKey] = 0
	return cfg
}
