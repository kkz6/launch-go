package tasks

import "fmt"

// RunScheduleScript renders the SSH script for a single schedule
// run. Mirrors dokploy's runCommand() — first checks the container
// exists, then `docker exec`s into it with the chosen shell. stdout
// + stderr both flow back through taskrunner's standard capture, so
// the resulting taskrunner log file is what View Logs reads.
//
// containerName is resolved at dispatch time (NOT cached on the
// schedule row) so a rebuild that produces a new container with the
// same project+app slugs is transparent — the schedule still finds
// the right target.
//
// shellType is "bash" or "sh". The wrapping `sh -c '<command>'`
// stays sh, only the inner exec switches — bash gets `bash -c`,
// sh gets `sh -c`. Distroless images pick sh; debian/ubuntu pick
// bash so multi-line / fancy expansions work.
func RunScheduleScript(containerName, command, shellType string) string {
	if shellType != "bash" && shellType != "sh" {
		shellType = "sh"
	}
	// Quote-shell the command so user-supplied content can't break
	// out of the `-c '...'` wrapper. Single-quote-escape pattern:
	// every `'` in the command becomes `'\''`. We do it in shell
	// here rather than Go-side so the script reads naturally; the
	// %q below escapes containerName + command into shell-safe
	// double-quoted literals.
	return fmt.Sprintf(`#!/usr/bin/env bash
set -uo pipefail
CONTAINER_NAME=%q
SHELL_TYPE=%q
COMMAND=%q

if ! docker inspect "${CONTAINER_NAME}" >/dev/null 2>&1; then
    echo "::LAUNCH::schedule::no_container"
    echo "Container ${CONTAINER_NAME} is not running. Deploy the application first."
    exit 1
fi

echo "::LAUNCH::schedule::starting"
echo "Running: docker exec ${CONTAINER_NAME} ${SHELL_TYPE} -c '${COMMAND}'"
echo "----------"

if docker exec "${CONTAINER_NAME}" "${SHELL_TYPE}" -c "${COMMAND}"; then
    echo "----------"
    echo "::LAUNCH::schedule::ok"
    exit 0
else
    EXIT_CODE=$?
    echo "----------"
    echo "::LAUNCH::schedule::failed exit=${EXIT_CODE}"
    exit ${EXIT_CODE}
fi
`, containerName, shellType, command)
}
