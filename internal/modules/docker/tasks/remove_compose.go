package tasks

import "fmt"

// RemoveComposeScript renders the teardown bash for a compose project.
//
// When `removeVolumes` is true the script runs `docker compose down -v
// --remove-orphans`, which deletes anonymous AND named volumes the
// stack declared (external: true volumes are left alone). When false
// (the default exposed in the UI), it runs plain `docker compose down
// --remove-orphans`, which only stops + removes containers and
// networks — named volumes survive so the data is still there if the
// user redeploys.
//
// Pre-v2 launchctl always used `down -v`, which silently destroyed
// data on a misclick. The flag flipped the default to "keep" so a
// fat-fingered Delete doesn't lose state.
//
// The compose file no longer needs to exist on disk: we use
// `docker compose --project-name <name>` against the running stack,
// which only reads the project label from the existing containers.
// `2>/dev/null || true` swallows the "no resources" case when the
// stack was never up.
func RemoveComposeScript(projectName string, removeVolumes bool) string {
	flag := "--remove-orphans"
	if removeVolumes {
		flag = "-v --remove-orphans"
	}
	return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
PROJECT_NAME=%q

# Best-effort teardown — if nothing matches the label the command
# exits non-zero, which we treat as success because the goal state
# ("no containers for this project") is already true.
docker compose --project-name "${PROJECT_NAME}" down %s 2>&1 || true
echo "::LAUNCH::compose_removed::yes"
`, projectName, flag)
}
