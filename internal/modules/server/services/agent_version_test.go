package services

import "testing"

func TestAgentUpdateAvailable(t *testing.T) {
	cases := []struct {
		installed, latest string
		want              bool
	}{
		{"0.7.2", "0.8.0", true},    // older patch line -> update
		{"0.8.0", "0.8.0", false},   // same -> no update
		{"0.8.1", "0.8.0", false},   // newer than latest -> no update
		{"1.0.0", "0.8.0", false},   // major newer -> no update
		{"latest", "0.8.0", true},   // legacy placeholder -> prompt update
		{"master", "0.8.0", true},   // legacy placeholder -> prompt update
		{"", "0.8.0", true},         // unknown installed -> prompt update
		{"0.8.0", "", false},        // unknown latest -> nothing to offer
		{"0.8.0", "garbage", false}, // unparseable latest -> nothing to offer
		{"v0.7.0", "v0.8.0", true},  // leading-v tolerated on both
		{"0.8", "0.8.0", false},     // short form equals full
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
