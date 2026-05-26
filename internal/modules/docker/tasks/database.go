package tasks

import (
	"encoding/json"
	"fmt"
	"strings"
)

// DatabaseRunConfig is the input for the run-database SSH script.
// Pure data — keep the script renderer deterministic.
type DatabaseRunConfig struct {
	ContainerName string
	Image         string // full image ref including tag, e.g. "postgres:16"
	EnvVars       []string
	InternalPort  int
	ExternalPort  *int // when non-nil, the host:container port mapping
	// VolumeName + DataPath define the named bind that persists the
	// database's on-disk state across container recreates. When both are
	// set the script renders `-v "$VOLUME_NAME:$DATA_PATH"`.
	VolumeName string
	DataPath   string
	// WipeVolume = true makes the script `docker volume rm` the named
	// volume after stopping the container and before starting the new
	// one. That's the "Rebuild Database" Danger Zone action — wipes
	// data, container comes back fresh.
	WipeVolume bool
	// ExtraArgs are appended after the image name. Engine-specific
	// flags like `redis-server --requirepass <pw>` end up here.
	ExtraArgs []string
}

// DatabaseVolumeName composes the deterministic named-volume label we
// bind into the container at the engine's data path. The database ID is
// the only unique part that survives renames — keeping it in the volume
// name means a renamed database keeps its data automatically.
func DatabaseVolumeName(databaseID string) string {
	return "launch-db-" + strings.ToLower(databaseID) + "-data"
}

// RunDatabaseScript renders the bash that:
//  1. Stops + removes any prior container with the same name (idempotent).
//  2. Runs the new container with --restart=unless-stopped on the
//     launch-network so other workloads can reach it by DNS.
//  3. Emits the container_id via the LAUNCH marker so the worker
//     can persist it.
//
// Keeping this as a string-renderer (not a docker-API call) means we
// don't need a docker SDK dependency on the launch-go side; everything
// happens through the SSH task runner that already proxies ssh into
// the server.
func RunDatabaseScript(cfg DatabaseRunConfig) string {
	var b strings.Builder
	b.WriteString("#!/usr/bin/env bash\n")
	b.WriteString("set -euo pipefail\n\n")

	fmt.Fprintf(&b, "CONTAINER_NAME=%q\n", cfg.ContainerName)
	fmt.Fprintf(&b, "IMAGE=%q\n", cfg.Image)
	if cfg.VolumeName != "" {
		fmt.Fprintf(&b, "VOLUME_NAME=%q\n", cfg.VolumeName)
	}
	if cfg.DataPath != "" {
		fmt.Fprintf(&b, "DATA_PATH=%q\n", cfg.DataPath)
	}
	b.WriteString("\n")

	b.WriteString(`echo "::LAUNCH::db_step::pulling_image"
docker pull "${IMAGE}"

echo "::LAUNCH::db_step::stopping_old_container"
if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
  docker stop "${CONTAINER_NAME}" >/dev/null 2>&1 || true
  docker rm   "${CONTAINER_NAME}" >/dev/null 2>&1 || true
fi
`)

	// Rebuild Database wipes the persistent volume between the stop and
	// the start so the engine reinitialises from scratch. Best-effort —
	// the volume may not exist yet (first run after upgrading from the
	// pre-volume builds) and that's fine.
	if cfg.WipeVolume && cfg.VolumeName != "" {
		b.WriteString(`
echo "::LAUNCH::db_step::wiping_volume"
docker volume rm "${VOLUME_NAME}" >/dev/null 2>&1 || true
`)
	}

	b.WriteString(`
echo "::LAUNCH::db_step::starting_container"
`)

	// Build `docker run` line. Multi-line for readability; the backslash
	// continuations are honoured by bash because we're between heredocs.
	b.WriteString("CONTAINER_ID=$(docker run -d \\\n")
	b.WriteString("  --name \"${CONTAINER_NAME}\" \\\n")
	b.WriteString("  --restart=unless-stopped \\\n")
	b.WriteString("  --network launch-network \\\n")
	// Named-volume bind. Docker auto-creates the volume on first use, so
	// no separate `docker volume create` is needed. The mount means the
	// data dir survives subsequent recreates triggered by expose-toggle,
	// restart-policy changes, image-tag bumps, etc.
	if cfg.VolumeName != "" && cfg.DataPath != "" {
		b.WriteString("  -v \"${VOLUME_NAME}:${DATA_PATH}\" \\\n")
	}
	for _, env := range cfg.EnvVars {
		fmt.Fprintf(&b, "  -e %q \\\n", env)
	}
	if cfg.ExternalPort != nil {
		fmt.Fprintf(&b, "  -p %d:%d \\\n", *cfg.ExternalPort, cfg.InternalPort)
	}
	b.WriteString("  \"${IMAGE}\"")
	for _, a := range cfg.ExtraArgs {
		fmt.Fprintf(&b, " %s", shellEscapeArg(a))
	}
	b.WriteString(")\n")

	b.WriteString(`echo "::LAUNCH::container_id::${CONTAINER_ID}"
echo "::LAUNCH::image_ref::${IMAGE}"
echo "::LAUNCH::db_step::done"
`)

	return b.String()
}

// shellEscapeArg single-quotes a CLI arg for safe interpolation into a
// bash command. We use this for engine-specific ExtraArgs (e.g. the
// redis-server --requirepass flag) where the value comes from a
// password we generated. Same shape as shellQuote in docker_logs.go.
func shellEscapeArg(s string) string {
	if s == "" {
		return "''"
	}
	// Fast-path for the common case of a-z 0-9 etc.
	safe := true
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' || c == '.' || c == '/' || c == '=') {
			safe = false
			break
		}
	}
	if safe {
		return s
	}
	out := make([]byte, 0, len(s)+2)
	out = append(out, '\'')
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			out = append(out, '\'', '\\', '\'', '\'')
			continue
		}
		out = append(out, s[i])
	}
	out = append(out, '\'')
	return string(out)
}

// DatabaseAdvancedUpdate carries the Advanced subtab knobs that map
// to `docker update` flags. Empty strings mean "leave unchanged" —
// docker update needs the flag to be omitted in that case, so the
// script generator below conditionally appends each one.
type DatabaseAdvancedUpdate struct {
	RestartPolicy     string // "no" | "on-failure" | "always" | "unless-stopped"
	CPULimit          string // docker --cpus, e.g. "0.5"
	MemoryLimit       string // docker -m, e.g. "512m" / "1g"
	CPUReservation    string // docker --cpu-shares-equivalent (we use --cpus reservation)
	MemoryReservation string // docker --memory-reservation
}

// DatabaseLifecycleScript renders a script for a managed database
// container action. Supported actions:
//
//   - start | stop | restart  →  docker <action> <container>
//   - rm                       →  docker stop (best-effort) + docker rm
//   - update-restart:<policy>  →  docker update --restart=<policy>
//   - update-advanced:<json>   →  docker update with the full knob set
//     (restart policy + CPU + memory + reservations). JSON-encoded
//     DatabaseAdvancedUpdate marshalled by the service.
//
// Everything else returns an error script that exits non-zero, so a
// caller bug surfaces as a failed task rather than executing an
// attacker-shaped docker subcommand.
// `volumeToRemove`, when non-empty AND action=="rm", appends a
// best-effort `docker volume rm <name>` step after the container is
// removed. Used by DeleteDatabase when the user opts into volume
// cleanup via the Delete confirmation checkbox. Empty (the default)
// preserves the named data volume so a recovered database keeps its
// state on the next create.
func DatabaseLifecycleScript(containerName, action, volumeToRemove string) string {
	if strings.HasPrefix(action, "update-advanced:") {
		raw := action[len("update-advanced:"):]
		var cfg DatabaseAdvancedUpdate
		if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
			return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
echo "invalid advanced-update payload: %s" >&2
exit 1
`, err.Error())
		}
		return databaseAdvancedUpdateScript(containerName, cfg)
	}
	if strings.HasPrefix(action, "update-restart:") {
		policy := action[len("update-restart:"):]
		switch policy {
		case "no", "on-failure", "always", "unless-stopped":
		default:
			return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
echo "unsupported restart policy: %s" >&2
exit 1
`, policy)
		}
		return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
CONTAINER_NAME=%q
docker update --restart=%s "${CONTAINER_NAME}"
echo "::LAUNCH::db_lifecycle::update-restart"
`, containerName, shellEscapeArg(policy))
	}
	if action != "start" && action != "stop" && action != "restart" && action != "rm" {
		// Defensive — the caller is supposed to validate, but never
		// generate a script with an unknown command.
		return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
echo "unknown lifecycle action: %s" >&2
exit 1
`, action)
	}
	var stopFirst string
	var volumeRm string
	if action == "rm" {
		// `docker rm` won't remove a running container by default; force
		// the stop so the operator can delete a database without two
		// clicks.
		stopFirst = "docker stop \"${CONTAINER_NAME}\" >/dev/null 2>&1 || true\n"
		if volumeToRemove != "" {
			// Best-effort: the volume may not exist (database never
			// successfully ran) and `docker volume rm` errors on
			// missing volumes. The container is already gone, so a
			// missing volume isn't a failure — the goal state is
			// "no container, no data" and missing data already
			// satisfies the latter.
			volumeRm = fmt.Sprintf(
				"docker volume rm %q >/dev/null 2>&1 || true\n",
				volumeToRemove,
			)
		}
	}
	return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
CONTAINER_NAME=%q
%sdocker %s "${CONTAINER_NAME}"
%secho "::LAUNCH::db_lifecycle::%s"
`, containerName, stopFirst, action, volumeRm, action)
}

// DatabaseContainerName composes the on-server container name for a
// managed database. Same shape as ContainerNameFor for applications,
// with a distinct prefix so app/db names can't collide.
func DatabaseContainerName(projectSlug, dbSlug string) string {
	return fmt.Sprintf("launch-db-%s-%s", projectSlug, dbSlug)
}

// databaseAdvancedUpdateScript renders `docker update` with whichever
// knobs the caller set. Each empty field is silently skipped so a
// partial update (just CPU, say) doesn't accidentally clear memory
// limits — docker would treat any flag absence as "no change".
func databaseAdvancedUpdateScript(
	containerName string, cfg DatabaseAdvancedUpdate,
) string {
	flags := []string{}
	if cfg.RestartPolicy != "" {
		switch cfg.RestartPolicy {
		case "no", "on-failure", "always", "unless-stopped":
			flags = append(flags, "--restart="+shellEscapeArg(cfg.RestartPolicy))
		default:
			return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
echo "unsupported restart policy: %s" >&2
exit 1
`, cfg.RestartPolicy)
		}
	}
	if cfg.CPULimit != "" {
		flags = append(flags, "--cpus="+shellEscapeArg(cfg.CPULimit))
	}
	if cfg.MemoryLimit != "" {
		flags = append(flags, "--memory="+shellEscapeArg(cfg.MemoryLimit))
	}
	if cfg.MemoryReservation != "" {
		flags = append(flags, "--memory-reservation="+shellEscapeArg(cfg.MemoryReservation))
	}
	// CPU reservation maps to --cpu-shares; docker doesn't have a hard
	// CPU reservation flag the way memory does. shares is relative
	// (1024 = baseline) so we let the user type a raw shares number
	// here. Skip if empty.
	if cfg.CPUReservation != "" {
		flags = append(flags, "--cpu-shares="+shellEscapeArg(cfg.CPUReservation))
	}

	if len(flags) == 0 {
		// No-op — still emit the marker so the caller's success path
		// sees output even when the form was submitted without changes.
		return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
CONTAINER_NAME=%q
echo "::LAUNCH::db_lifecycle::update-advanced-noop"
`, containerName)
	}

	return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
CONTAINER_NAME=%q
docker update %s "${CONTAINER_NAME}"
echo "::LAUNCH::db_lifecycle::update-advanced"
`, containerName, strings.Join(flags, " "))
}
