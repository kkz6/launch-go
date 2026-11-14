package tasks

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

const ReleaseInventoryBusyExitCode = 75

var releaseInventoryNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

// NewReleaseInventoryTask only reads the deployment tree. It shares the lock
// used by deploy/rollback, and never creates a lock file during an observation.
func NewReleaseInventoryTask(sitePath string) *taskrunner.BaseTask {
	script := `#!/bin/bash
set -euo pipefail
export LC_ALL=C
SITE_PATH='` + escapeShellPath(sitePath) + `'
[[ "$SITE_PATH" = /* && "$SITE_PATH" != / && -d "$SITE_PATH" && ! -L "$SITE_PATH" ]] || exit 1
SITE_ROOT=$(readlink -f -- "$SITE_PATH")
[[ -f "$SITE_ROOT/.deployment.lock" && ! -L "$SITE_ROOT/.deployment.lock" ]] || exit 1
exec 9<"$SITE_ROOT/.deployment.lock"
flock -s -n 9 || exit 75
RELEASES="$SITE_ROOT/releases"
[[ -d "$RELEASES" && ! -L "$RELEASES" ]] || exit 1
ACTIVE=""
if [[ -L "$SITE_ROOT/current" ]]; then
    CURRENT=$(readlink -f -- "$SITE_ROOT/current") || exit 1
    ACTIVE=${CURRENT##*/}
    [[ "$ACTIVE" =~ ^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$ ]] || exit 1
    [[ "$CURRENT" = "$RELEASES/$ACTIVE" && -d "$CURRENT" && ! -L "$RELEASES/$ACTIVE" ]] || exit 1
elif [[ -e "$SITE_ROOT/current" ]]; then
    exit 1
fi
printf 'LAUNCH_RELEASES_V1\nactive\t%s\n' "$ACTIVE"
shopt -s nullglob
COUNT=0
for RELEASE in "$RELEASES"/*; do
    [[ -d "$RELEASE" && ! -L "$RELEASE" ]] || continue
    NAME=${RELEASE##*/}
    [[ "$NAME" =~ ^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$ ]] || continue
    COUNT=$((COUNT + 1))
    [[ "$COUNT" -le 512 ]] || exit 1
    printf 'release\t%s\n' "$NAME"
done
printf 'END_LAUNCH_RELEASES_V1\n'
`
	return taskrunner.NewScriptTask("Inspect Site Releases", script, 10*time.Second)
}

type ReleaseInventory struct {
	ActiveRelease string
	Releases      []string
}

// ParseReleaseInventory accepts only a complete, bounded observation. Partial
// output (including a lost SSH connection after current was read) is unusable.
func ParseReleaseInventory(output string) (*ReleaseInventory, error) {
	if len(output) > 65536 {
		return nil, fmt.Errorf("release inventory too large")
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 3 || lines[0] != "LAUNCH_RELEASES_V1" || lines[len(lines)-1] != "END_LAUNCH_RELEASES_V1" || !strings.HasPrefix(lines[1], "active\t") {
		return nil, fmt.Errorf("incomplete release inventory")
	}
	inventory := &ReleaseInventory{ActiveRelease: strings.TrimPrefix(lines[1], "active\t"), Releases: []string{}}
	if inventory.ActiveRelease != "" && !releaseInventoryNamePattern.MatchString(inventory.ActiveRelease) {
		return nil, fmt.Errorf("invalid active release")
	}
	seen := make(map[string]bool)
	for _, line := range lines[2 : len(lines)-1] {
		name := strings.TrimPrefix(line, "release\t")
		if !strings.HasPrefix(line, "release\t") || !releaseInventoryNamePattern.MatchString(name) || seen[name] || len(seen) >= 512 {
			return nil, fmt.Errorf("invalid retained release")
		}
		seen[name] = true
		inventory.Releases = append(inventory.Releases, name)
	}
	if inventory.ActiveRelease != "" && !seen[inventory.ActiveRelease] {
		return nil, fmt.Errorf("active release is not retained")
	}
	return inventory, nil
}
