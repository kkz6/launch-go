package tasks

import (
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
	// ExtraArgs are appended after the image name. Engine-specific
	// flags like `redis-server --requirepass <pw>` end up here.
	ExtraArgs []string
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
	fmt.Fprintf(&b, "IMAGE=%q\n\n", cfg.Image)

	b.WriteString(`echo "::LAUNCH::db_step::pulling_image"
docker pull "${IMAGE}"

echo "::LAUNCH::db_step::stopping_old_container"
if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
  docker stop "${CONTAINER_NAME}" >/dev/null 2>&1 || true
  docker rm   "${CONTAINER_NAME}" >/dev/null 2>&1 || true
fi

echo "::LAUNCH::db_step::starting_container"
`)

	// Build `docker run` line. Multi-line for readability; the backslash
	// continuations are honoured by bash because we're between heredocs.
	b.WriteString("CONTAINER_ID=$(docker run -d \\\n")
	b.WriteString("  --name \"${CONTAINER_NAME}\" \\\n")
	b.WriteString("  --restart=unless-stopped \\\n")
	b.WriteString("  --network launch-network \\\n")
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

// DatabaseLifecycleScript renders a script for a managed database
// container action. Supported actions:
//
//   - start | stop | restart  →  docker <action> <container>
//   - rm                       →  docker stop (best-effort) + docker rm
//   - update-restart:<policy>  →  docker update --restart=<policy>
//
// Everything else returns an error script that exits non-zero, so a
// caller bug surfaces as a failed task rather than executing an
// attacker-shaped docker subcommand.
func DatabaseLifecycleScript(containerName, action string) string {
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
	if action == "rm" {
		// `docker rm` won't remove a running container by default; force
		// the stop so the operator can delete a database without two
		// clicks.
		stopFirst = "docker stop \"${CONTAINER_NAME}\" >/dev/null 2>&1 || true\n"
	}
	return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
CONTAINER_NAME=%q
%sdocker %s "${CONTAINER_NAME}"
echo "::LAUNCH::db_lifecycle::%s"
`, containerName, stopFirst, action, action)
}

// DatabaseContainerName composes the on-server container name for a
// managed database. Same shape as ContainerNameFor for applications,
// with a distinct prefix so app/db names can't collide.
func DatabaseContainerName(projectSlug, dbSlug string) string {
	return fmt.Sprintf("launch-db-%s-%s", projectSlug, dbSlug)
}
