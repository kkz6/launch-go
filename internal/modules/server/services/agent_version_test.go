package services

import "testing"

func TestAgentUpdateAvailable(t *testing.T) {
	// New semantics: any installed != latest → update available.
	// Single-line release channel (the launch-util repo), so exact
	// equality is the right gate. Legacy seed builds (v1.0, "latest",
	// "master", empty) all correctly trigger the update banner now —
	// they were the original blocker because v1.0 compared as
	// semver-newer than v0.8.0 under the old logic.
	cases := []struct {
		installed, latest string
		want              bool
	}{
		{"0.7.2", "0.8.0", true},   // older patch line -> update
		{"0.8.0", "0.8.0", false},  // same -> no update
		{"0.8.1", "0.8.0", true},   // newer than latest -> update (anything ≠ latest)
		{"1.0.0", "0.8.0", true},   // legacy v1.0 -> update (this was the bug)
		{"latest", "0.8.0", true},  // legacy placeholder -> update
		{"master", "0.8.0", true},  // legacy placeholder -> update
		{"", "0.8.0", true},        // unknown installed -> update
		{"0.8.0", "", false},       // unknown latest -> nothing to offer
		{"0.8.0", "garbage", true}, // installed ≠ normalised(latest) -> update
		{"v0.7.0", "v0.8.0", true}, // leading-v tolerated on both -> still differ
		{"0.8.0", "v0.8.0", false}, // leading-v stripped on both -> equal
	}
	for _, c := range cases {
		if got := agentUpdateAvailable(c.installed, c.latest); got != c.want {
			t.Errorf("agentUpdateAvailable(%q,%q)=%v want %v", c.installed, c.latest, got, c.want)
		}
	}
}

func TestParseSemver(t *testing.T) {
	if got := parseSemver("0.8.0"); got == nil || got[0] != 0 || got[1] != 8 || got[2] != 0 {
		t.Errorf("parseSemver(0.8.0)=%v", got)
	}
	if parseSemver("latest") != nil {
		t.Errorf("parseSemver(latest) should be nil")
	}
	if got := parseSemver("1.2.3-rc1"); got == nil || got[2] != 3 {
		t.Errorf("parseSemver should drop pre-release suffix, got %v", got)
	}
}
