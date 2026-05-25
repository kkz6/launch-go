package tasks

import (
	"fmt"
	"strings"
)

// RemoveComposeScript renders the teardown bash for a compose project.
//
// History on this script (read me before "simplifying"):
//
// The previous implementation called
//
//	docker compose --project-name "${PROJECT_NAME}" down --remove-orphans
//
// without cd-ing into the stack's directory and without a "-f
// compose.yml" flag. The hypothesis at the time was that
// --project-name alone would let "docker compose down" resolve the
// running stack by label. That hypothesis is wrong. docker compose
// v2 looks for a compose file in the current directory regardless of
// --project-name; with no file present it exits non-zero, the
// script's "|| true" swallowed the error, and the teardown silently
// no-op'd while the containers kept running. The "Containers" tab on
// the host then continued to show the deleted stack's containers —
// exactly the bug we shipped.
//
// The fix here is to tear down by docker label instead of through the
// compose CLI. Compose stamps every container/network/volume it
// creates with the label com.docker.compose.project=<project-name>;
// we enumerate those resources directly and remove them. This is
// what "docker compose down" does internally — we're just skipping
// the compose-file middleman.
//
// Resources removed (in order):
//  1. Containers labelled with the project. "docker rm -f" stops and
//     removes in one call, so we don't need a separate stop pass.
//  2. Networks labelled with the project. Skips the default
//     bridge/host/none networks because they're unlabelled.
//  3. Volumes labelled with the project — ONLY when removeVolumes is
//     true. Named volumes survive otherwise so a fat-fingered Delete
//     doesn't destroy data.
//
// After resource teardown we also clean up the on-disk footprint:
//  4. The stack directory at /var/lib/launch/projects/<proj>/<comp>.
//  5. The per-compose Traefik dynamic-config file at
//     /etc/launch/traefik/dynamic/compose-<proj>-<comp>.yml.
//
// Both (4) and (5) are SKIPPED when projectSlug or composeSlug is
// empty — the historical payload didn't carry them, so older queued
// jobs fall back to "containers/networks only" cleanup.
//
// Every individual rm is best-effort ("|| true") so a missing volume
// or a permission glitch on a single file doesn't abort the rest of
// the sweep. The aggregate goal state — "no docker resources
// matching this project name" — is reachable from anywhere,
// including a partial previous teardown.
func RemoveComposeScript(
	projectName, projectSlug, composeSlug string,
	removeVolumes bool,
) string {
	var b strings.Builder
	b.WriteString("#!/usr/bin/env bash\n")
	b.WriteString("set -uo pipefail\n\n")
	fmt.Fprintf(&b, "PROJECT_NAME=%q\n", projectName)

	// Core containers + networks block. No backticks inside the raw
	// string literal — Go's raw string delimiter is also a backtick,
	// so shell-style command quoting would terminate the literal.
	b.WriteString(`LABEL="com.docker.compose.project=${PROJECT_NAME}"

# 1. Containers — stop + remove in one shot. "docker rm -f" SIGKILLs
#    after a 10s SIGTERM grace, matching the same shutdown semantics
#    the compose CLI uses for unresponsive containers.
echo "::LAUNCH::compose_remove_step::containers"
CONTAINERS=$(docker ps -a --filter "label=${LABEL}" --format '{{.ID}}' | tr '\n' ' ')
if [ -n "${CONTAINERS}" ]; then
  echo "removing containers: ${CONTAINERS}"
  # shellcheck disable=SC2086  # word splitting is what we want here
  docker rm -f ${CONTAINERS} 2>&1 || true
else
  echo "no containers matching project ${PROJECT_NAME}"
fi

# 2. Networks. Compose-created networks carry the same label as
#    containers. Default networks (bridge/host/none) don't, so
#    they're filtered out automatically.
echo "::LAUNCH::compose_remove_step::networks"
NETWORKS=$(docker network ls --filter "label=${LABEL}" --format '{{.ID}}' | tr '\n' ' ')
if [ -n "${NETWORKS}" ]; then
  echo "removing networks: ${NETWORKS}"
  # shellcheck disable=SC2086
  docker network rm ${NETWORKS} 2>&1 || true
else
  echo "no networks matching project ${PROJECT_NAME}"
fi
`)

	// 3. Volume teardown is opt-in. Keep separate from the network
	//    block so a future "always-volumes" mode is a one-line flip.
	if removeVolumes {
		b.WriteString(`
# 3. Volumes — opt-in. Same label, same approach. Volumes that are
#    still attached to a container we missed will error here; the
#    "|| true" keeps the script moving so the rest of the cleanup
#    still runs.
echo "::LAUNCH::compose_remove_step::volumes"
VOLUMES=$(docker volume ls --filter "label=${LABEL}" --format '{{.Name}}' | tr '\n' ' ')
if [ -n "${VOLUMES}" ]; then
  echo "removing volumes: ${VOLUMES}"
  # shellcheck disable=SC2086
  docker volume rm ${VOLUMES} 2>&1 || true
else
  echo "no volumes matching project ${PROJECT_NAME}"
fi
`)
	}

	// 4 + 5. Disk cleanup. Only when the caller passed the slugs;
	// without them we can't reconstruct the paths safely.
	if projectSlug != "" && composeSlug != "" {
		stackDir := fmt.Sprintf(
			"/var/lib/launch/projects/%s/%s",
			projectSlug, composeSlug,
		)
		traefikPath := fmt.Sprintf(
			"/etc/launch/traefik/dynamic/compose-%s-%s.yml",
			projectSlug, composeSlug,
		)
		fmt.Fprintf(&b, `
# 4. Stack directory on disk. Held the compose file + per-deploy
#    git clones; useless once the containers are gone.
echo "::LAUNCH::compose_remove_step::stack_dir"
STACK_DIR=%q
if [ -d "${STACK_DIR}" ]; then
  sudo rm -rf "${STACK_DIR}" 2>&1 || true
  echo "removed stack dir ${STACK_DIR}"
else
  echo "stack dir ${STACK_DIR} already gone"
fi

# 5. Traefik dynamic-config file for this stack. Leaving it behind
#    would re-introduce the (now-broken) routing rules on the next
#    Traefik reload.
echo "::LAUNCH::compose_remove_step::traefik_config"
TRAEFIK_CONFIG=%q
if [ -f "${TRAEFIK_CONFIG}" ]; then
  sudo rm -f "${TRAEFIK_CONFIG}" 2>&1 || true
  echo "removed traefik config ${TRAEFIK_CONFIG}"
else
  echo "traefik config ${TRAEFIK_CONFIG} already gone"
fi
`, stackDir, traefikPath)
	}

	b.WriteString("\necho \"::LAUNCH::compose_removed::yes\"\n")
	return b.String()
}
