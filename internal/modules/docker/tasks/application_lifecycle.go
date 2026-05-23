package tasks

import "fmt"

// ApplicationLifecycleScript renders an SSH script that runs a single
// docker action (stop / restart) against an application's running
// container. Mirrors DatabaseLifecycleScript — kept separate because
// the application path doesn't have an associated data volume to
// recreate, so the rebuild branch lives in DeployApplicationScript
// (with force-pull), not here.
//
// Tolerates "container missing" so clicking Stop on a never-deployed
// app returns success rather than a confusing red toast.
func ApplicationLifecycleScript(containerName, action string) string {
	return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
CONTAINER_NAME=%q
ACTION=%q

if ! docker inspect "${CONTAINER_NAME}" >/dev/null 2>&1; then
    echo "::LAUNCH::app_lifecycle::no_container"
    exit 0
fi

case "${ACTION}" in
    stop)
        docker stop "${CONTAINER_NAME}" >/dev/null
        echo "::LAUNCH::app_lifecycle::stopped"
        ;;
    restart)
        docker restart "${CONTAINER_NAME}" >/dev/null
        echo "::LAUNCH::app_lifecycle::restarted"
        ;;
    start)
        docker start "${CONTAINER_NAME}" >/dev/null
        echo "::LAUNCH::app_lifecycle::started"
        ;;
    *)
        echo "Unknown action: ${ACTION}" >&2
        exit 1
        ;;
esac
`, containerName, action)
}
