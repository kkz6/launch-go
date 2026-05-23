package tasks

import "fmt"

// RemoveApplicationScript renders a shell script that stops and
// removes a docker application's container. Tolerates "container
// doesn't exist" so deleting an app that never deployed is a no-op
// rather than a hard failure.
//
// Mirrors the rm branch of DatabaseLifecycleScript — kept separate
// because the application task needs to handle the case where the
// container name was never reserved (zero-deploy delete).
func RemoveApplicationScript(containerName string) string {
	return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
CONTAINER_NAME=%q

# Container may not exist (e.g. app was never deployed, or already
# torn down out-of-band). Treat both stop and rm as best-effort —
# success of the overall script means "the container is gone".
if docker inspect "${CONTAINER_NAME}" >/dev/null 2>&1; then
    docker stop "${CONTAINER_NAME}" >/dev/null 2>&1 || true
    docker rm   "${CONTAINER_NAME}" >/dev/null 2>&1 || true
    echo "::LAUNCH::app_removed::yes"
else
    echo "::LAUNCH::app_removed::no_container"
fi
`, containerName)
}
