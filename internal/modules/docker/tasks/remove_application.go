package tasks

import (
	"fmt"
	"strings"
)

// RemoveApplicationScript renders a shell script that stops + removes
// a docker application's container and (optionally) wipes the named
// volumes the application declared.
//
// Tolerates "container doesn't exist" so deleting an app that never
// deployed is a no-op rather than a hard failure.
//
// Mirrors the rm branch of DatabaseLifecycleScript — kept separate
// because the application task needs to handle the case where the
// container name was never reserved (zero-deploy delete).
//
// When `volumeNames` is non-empty, each entry is passed to
// `docker volume rm` AFTER the container is gone. Docker won't remove
// a volume that's still in use, so the order matters: stop → rm
// container → rm volumes. Each `volume rm` is best-effort (`|| true`)
// so a missing volume doesn't fail the whole script.
func RemoveApplicationScript(containerName string, volumeNames []string) string {
	var volumeBlock string
	if len(volumeNames) > 0 {
		var b strings.Builder
		b.WriteString("\n# Remove named volumes the application declared. Best-effort —\n")
		b.WriteString("# `docker volume rm` exits non-zero if the volume doesn't exist\n")
		b.WriteString("# or is still attached to something we missed; neither should\n")
		b.WriteString("# fail the overall delete since the container is already gone.\n")
		for _, v := range volumeNames {
			fmt.Fprintf(&b, "docker volume rm %q >/dev/null 2>&1 || true\n", v)
		}
		volumeBlock = b.String()
	}

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
%s`, containerName, volumeBlock)
}
