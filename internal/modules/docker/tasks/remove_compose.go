package tasks

import "fmt"

// RemoveComposeScript runs `docker compose down -v` for the given
// project name. The `-v` removes anonymous volumes the stack created;
// named external volumes are preserved (compose down --volumes does
// not touch volumes declared external: true).
//
// The compose file no longer needs to exist on disk: we use
// `docker compose --project-name <name>` against the running stack,
// which only reads the project label from the existing containers.
// `2>/dev/null || true` swallows the "no resources" case when the
// stack was never up.
func RemoveComposeScript(projectName string) string {
	return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
PROJECT_NAME=%q

# Best-effort teardown — if nothing matches the label the command
# exits non-zero, which we treat as success because the goal state
# ("no containers for this project") is already true.
docker compose --project-name "${PROJECT_NAME}" down -v --remove-orphans 2>&1 || true
echo "::LAUNCH::compose_removed::yes"
`, projectName)
}
