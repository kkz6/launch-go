// Package testutil provides testing utilities
package testutil

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// SnapshotDir returns the path to the snapshots directory
func SnapshotDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "snapshots")
}

// MatchSnapshot compares actual content with a stored snapshot.
// If UPDATE_SNAPSHOTS=true, it updates the snapshot instead of comparing.
func MatchSnapshot(t *testing.T, name string, actual string) {
	t.Helper()

	snapshotPath := filepath.Join(SnapshotDir(), name+".snap")

	// Normalize line endings
	actual = normalizeLineEndings(actual)

	// Check if we should update snapshots
	if os.Getenv("UPDATE_SNAPSHOTS") == "true" {
		err := os.MkdirAll(filepath.Dir(snapshotPath), 0755)
		if err != nil {
			t.Fatalf("Failed to create snapshot directory: %v", err)
		}

		err = os.WriteFile(snapshotPath, []byte(actual), 0644)
		if err != nil {
			t.Fatalf("Failed to write snapshot: %v", err)
		}

		t.Logf("Updated snapshot: %s", snapshotPath)
		return
	}

	// Read existing snapshot
	expected, err := os.ReadFile(snapshotPath)
	if err != nil {
		if os.IsNotExist(err) {
			t.Fatalf("Snapshot does not exist: %s\nRun with UPDATE_SNAPSHOTS=true to create it.\n\nActual:\n%s", snapshotPath, actual)
		}
		t.Fatalf("Failed to read snapshot: %v", err)
	}

	expectedStr := normalizeLineEndings(string(expected))

	if actual != expectedStr {
		t.Errorf("Snapshot mismatch for %s\n\n--- Expected ---\n%s\n\n--- Actual ---\n%s\n\nRun with UPDATE_SNAPSHOTS=true to update the snapshot.",
			name, expectedStr, actual)
	}
}

// MatchSnapshotJSON compares actual JSON content with a stored snapshot.
// It normalizes JSON formatting before comparison.
func MatchSnapshotJSON(t *testing.T, name string, actual string) {
	t.Helper()
	// For now, just use the regular snapshot matching
	// In the future, we could normalize JSON formatting
	MatchSnapshot(t, name, actual)
}

// normalizeLineEndings converts all line endings to Unix-style
func normalizeLineEndings(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}

// TrimSnapshot trims leading/trailing whitespace and normalizes the snapshot
func TrimSnapshot(s string) string {
	return strings.TrimSpace(normalizeLineEndings(s))
}
