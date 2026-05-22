package dto

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestClassifyTaskFailure_DpkgLock pins the most common failure mode (the
// one that prompted this classifier) — cloud-init holds the apt lock when
// our provision script tries to install packages.
func TestClassifyTaskFailure_DpkgLock(t *testing.T) {
	rawOutput := `Rules updated (v6)
Firewall is active and enabled on system startup
::LAUNCH::step_completed::configure_firewall
::LAUNCH::progress::15
::LAUNCH::status::Install essential packages (curl, git, wget, etc.)
Install essential packages
E: Could not get lock /var/lib/dpkg/lock-frontend. It is held by process 1355 (apt-get)
E: Unable to acquire the dpkg frontend lock (/var/lib/dpkg/lock-frontend), is another process using it?`

	got := classifyTaskFailure(rawOutput)

	assert.NotContains(t, got, "::LAUNCH::", "launch markers must be stripped from the user-facing message")
	assert.NotContains(t, got, "dpkg", "raw dpkg error must not leak to the customer")
	assert.NotContains(t, got, "process 1355", "raw process IDs must not leak")
	assert.Contains(t, strings.ToLower(got), "package manager", "should explain in plain language")
}

func TestClassifyTaskFailure_OutOfDiskSpace(t *testing.T) {
	got := classifyTaskFailure("E: You don't have enough free space in /var/cache/apt/archives/.\nNo space left on device")
	assert.Contains(t, strings.ToLower(got), "disk space")
	assert.NotContains(t, got, "::LAUNCH::")
}

func TestClassifyTaskFailure_DnsResolution(t *testing.T) {
	got := classifyTaskFailure("Err:1 http://archive.ubuntu.com jammy InRelease\n  Temporary failure resolving 'archive.ubuntu.com'")
	assert.Contains(t, strings.ToLower(got), "dns", "DNS failure should mention DNS")
}

func TestClassifyTaskFailure_GenericFallback(t *testing.T) {
	// Unknown error → generic message, no raw bash leaks.
	got := classifyTaskFailure("some unexpected error nobody has seen before\nfoo bar baz")
	assert.NotContains(t, got, "some unexpected error", "raw output must not be echoed for unknown failures")
	assert.NotContains(t, got, "::LAUNCH::")
	assert.Contains(t, strings.ToLower(got), "setup didn't complete")
}

func TestStripLaunchMarkers(t *testing.T) {
	in := "Configure swap\n::LAUNCH::progress::5\n::LAUNCH::step_completed::configure_swap\nSwap configured"
	out := stripLaunchMarkers(in)
	assert.NotContains(t, out, "::LAUNCH::")
	assert.Contains(t, out, "Configure swap")
	assert.Contains(t, out, "Swap configured")
}
