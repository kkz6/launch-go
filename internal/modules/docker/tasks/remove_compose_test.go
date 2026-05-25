package tasks

import (
	"strings"
	"testing"
)

// TestRemoveComposeScript_UsesLabelTeardown is the regression guard
// for the "deleted compose stack containers stay running" bug. The
// previous implementation called "docker compose down" without a
// compose file in cwd, which silently no-op'd. The fix is to teardown
// by the com.docker.compose.project label directly. Pin the marker
// commands so we don't accidentally revert.
func TestRemoveComposeScript_UsesLabelTeardown(t *testing.T) {
	s := RemoveComposeScript("acme-stack", "acme", "stack", false)

	// Must enumerate by label, not by compose CLI.
	mustContain(t, s, `LABEL="com.docker.compose.project=${PROJECT_NAME}"`)
	mustContain(t, s, `docker ps -a --filter "label=${LABEL}"`)
	mustContain(t, s, `docker rm -f ${CONTAINERS}`)
	mustContain(t, s, `docker network ls --filter "label=${LABEL}"`)
	mustContain(t, s, `docker network rm ${NETWORKS}`)

	// MUST NOT call "docker compose down" — that's the bug we're
	// fixing. Without the compose file in cwd, the command exits
	// non-zero and the teardown silently no-ops.
	if strings.Contains(s, "docker compose") {
		t.Fatalf("remove script must not invoke `docker compose` CLI — that's the bug we're fixing; got:\n%s", s)
	}
}

func TestRemoveComposeScript_VolumesOptIn(t *testing.T) {
	// Default (false) → script must NOT touch volumes.
	noVols := RemoveComposeScript("acme-stack", "acme", "stack", false)
	if strings.Contains(noVols, "docker volume rm") {
		t.Errorf("removeVolumes=false must NOT delete volumes:\n%s", noVols)
	}
	if strings.Contains(noVols, "docker volume ls") {
		t.Errorf("removeVolumes=false must NOT even enumerate volumes:\n%s", noVols)
	}

	// Opt-in → script MUST delete volumes by label.
	withVols := RemoveComposeScript("acme-stack", "acme", "stack", true)
	mustContain(t, withVols, `docker volume ls --filter "label=${LABEL}"`)
	mustContain(t, withVols, `docker volume rm ${VOLUMES}`)
}

func TestRemoveComposeScript_StackDirAndTraefikCleanup(t *testing.T) {
	// With both slugs the script removes the on-disk footprint.
	s := RemoveComposeScript("acme-stack", "acme", "stack", false)
	mustContain(t, s, `STACK_DIR="/var/lib/launch/projects/acme/stack"`)
	mustContain(t, s, `sudo rm -rf "${STACK_DIR}"`)
	mustContain(t, s, `TRAEFIK_CONFIG="/etc/launch/traefik/dynamic/compose-acme-stack.yml"`)
	mustContain(t, s, `sudo rm -f "${TRAEFIK_CONFIG}"`)
}

func TestRemoveComposeScript_SkipsDiskCleanupWithoutSlugs(t *testing.T) {
	// Old queued payloads (pre-fix) didn't carry slugs. The script
	// must degrade gracefully — containers/networks still cleaned,
	// but disk cleanup skipped (rather than reaching for "" paths
	// and running "sudo rm -rf /var/lib/launch/projects//").
	s := RemoveComposeScript("acme-stack", "", "", false)
	if strings.Contains(s, "STACK_DIR=") {
		t.Errorf("missing slugs must skip the STACK_DIR block:\n%s", s)
	}
	if strings.Contains(s, "TRAEFIK_CONFIG=") {
		t.Errorf("missing slugs must skip the Traefik config block:\n%s", s)
	}
	// The defence-in-depth bit: an empty path passed to "rm -rf"
	// would be a catastrophic typo. Make sure absolutely no rm -rf
	// renders when slugs are empty.
	if strings.Contains(s, "rm -rf") {
		t.Errorf("missing slugs must produce no rm -rf invocation:\n%s", s)
	}
}

func TestRemoveComposeScript_BestEffortNotFailFast(t *testing.T) {
	// The teardown is best-effort by design — one missing volume or
	// permission glitch on a single file shouldn't abort the rest
	// of the cleanup. Pin "set -uo pipefail" (not -e) and the
	// "|| true" guards on each rm.
	s := RemoveComposeScript("acme-stack", "acme", "stack", true)
	if !strings.Contains(s, "set -uo pipefail") {
		t.Errorf("remove script must use `set -uo pipefail` (no -e) so the sweep continues past per-resource failures:\n%s", s)
	}
	// Each docker rm path should be guarded.
	for _, mustHaveGuard := range []string{
		"docker rm -f ${CONTAINERS}",
		"docker network rm ${NETWORKS}",
		"docker volume rm ${VOLUMES}",
	} {
		idx := strings.Index(s, mustHaveGuard)
		if idx == -1 {
			t.Errorf("missing expected rm command %q in:\n%s", mustHaveGuard, s)
			continue
		}
		// The guard should appear within ~50 chars after the rm
		// command (on the same line or the immediate continuation).
		tail := s[idx : idx+50]
		if !strings.Contains(tail, "|| true") {
			t.Errorf("rm command %q must be guarded by `|| true` — got %q", mustHaveGuard, tail)
		}
	}
}

func TestRemoveComposeScript_EmitsCompletionMarker(t *testing.T) {
	// The job parses ::LAUNCH:: markers to surface progress in the
	// UI. Pin the terminal marker so a missing one wouldn't show as
	// a stuck "removing…" state in the deployment row.
	s := RemoveComposeScript("acme-stack", "acme", "stack", false)
	mustContain(t, s, "::LAUNCH::compose_removed::yes")
}

// mustContain is shared with the other task-test files in this
// package (see deploy_application_test.go for the canonical
// declaration). Defined once at the package level; this comment is
// here so a future "add a helper here" doesn't redeclare it.
