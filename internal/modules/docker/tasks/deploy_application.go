// Package tasks contains the SSH-script tasks the docker module runs on
// docker-type servers. Each task returns a taskrunner.Task that the
// dispatcher executes over SSH.
package tasks

import (
	"fmt"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// DeployConfig holds the runtime parameters for a deploy. The job
// layer builds this from the Application + Deployment models so the
// task itself is a pure rendering function — easier to unit test, and
// no DB lookups during script generation.
type DeployConfig struct {
	DeploymentID  string
	ProjectSlug   string
	AppSlug       string
	ContainerName string

	SourceType dockertypes.SourceType
	// Image source.
	Image string
	// Git source.
	GitRepo        string
	GitBranch      string
	BuildType      dockertypes.BuildType
	DockerfilePath string
	// Dockerfile source (raw paste).
	DockerfileContents string

	// EnvVars passed to the container via `docker run --env-file`. Order
	// is preserved so the rendered script is deterministic — useful for
	// diffs in tests. The deploy script writes these to a 0600 file
	// under /run/launch (tmpfs) on the host and references the file
	// path on the `docker run` command line, instead of putting the
	// values on argv as `-e KEY=VALUE`. Avoids briefly-visible-in-`ps`
	// exposure and keeps secret values off persistent disk.
	EnvVars []EnvVar
	// BuildSecrets passed to `docker build` via BuildKit's
	// --mount=type=secret. Each entry's Name becomes the secret id
	// inside the Dockerfile (`RUN --mount=type=secret,id=<NAME>`).
	// The deploy script materialises each value to a 0600 file under
	// /run/launch/<deploy_id>.bsec/ on the host (tmpfs) and passes
	// `--secret id=<NAME>,src=<path>` for each one. Files are removed
	// as soon as the build finishes.
	//
	// Only honoured when SourceType is Git or Dockerfile — image
	// sources don't run a build step. Order is preserved so renderer
	// output is deterministic for tests.
	BuildSecrets []BuildSecret
	// Volumes attached to the container.
	Volumes []Volume

	// Runtime knobs from build_config["cpu_limit"] etc. Empty values
	// mean "skip the flag".
	CPULimit           string
	MemoryLimit        string
	RestartPolicy      string // empty → unless-stopped
	HealthcheckCommand string
	ExtraPorts         []string

	// Registry authentication for source_type=image. When username +
	// password are both non-empty, the deploy script runs
	// `docker login` BEFORE `docker pull`, then `docker logout` after
	// so the host's ambient docker config doesn't gain a stale entry.
	// RegistryURL: empty → Docker Hub (`docker login` with no host
	// argument); non-empty → that host (e.g. "ghcr.io").
	//
	// Wiring: the application service resolves whichever path is set
	// (saved credential vs inline) and hands the plaintext values to
	// the renderer. Plaintext only lives in memory; the at-rest
	// values stay encrypted via dbtype.EncryptedString.
	RegistryURL      string
	RegistryUsername string
	RegistryPassword string
}

// EnvVar is one key/value pair for the container's environment.
// Mirrors the persisted ApplicationEnvVar but stays in the tasks
// package as plain data so the renderer doesn't import models.
type EnvVar struct {
	Key   string
	Value string
}

// BuildSecret is one name/value pair mounted into `docker build` via
// BuildKit's --mount=type=secret. Different lifecycle than EnvVar:
// values land in a 0600 tmpfs file on the host BEFORE the build runs
// and are removed immediately afterwards. The Name is the id used by
// Dockerfile RUN steps (`--mount=type=secret,id=<NAME>`).
//
// Plain data here; the renderer doesn't import models. Loaded by the
// job layer from the persisted ApplicationBuildSecret table.
type BuildSecret struct {
	Name  string
	Value string
}

// Volume is one mount for the container. Type discriminates how the
// `-v` flag is rendered: named volumes use the volume name, bind
// mounts use the host path.
type Volume struct {
	Name      string
	MountPath string
	Type      string // "named" | "bind"
	HostPath  string // bind only; ignored for named
}

// DeployApplication returns a taskrunner.Task that deploys (or
// redeploys) an application according to its source type. The script is
// idempotent: it stops + removes the old container if one exists, then
// starts a new one with the same name. The new container ID is emitted
// as a `::LAUNCH::container_id::<id>` marker so the worker can persist
// it on success.
//
// Timeout is 30 minutes — plenty for a `docker build` of a typical web
// app but short enough to not pin a runaway forever.
func DeployApplication(cfg DeployConfig) taskrunner.Task {
	script := buildDeployScript(cfg)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Deploy Application"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(1800),
	)
}

// buildDeployScript composes the bash script the SSH task runs. The
// script's `set -euo pipefail` means any failing step aborts the deploy;
// downstream `docker stop`/`docker rm` calls are tolerant of missing
// containers because that's the bootstrap case (no prior deploy).
//
// Source-specific stanzas are constructed via short helpers so this
// outer function stays readable and the branches are easy to scan.
func buildDeployScript(cfg DeployConfig) string {
	var b strings.Builder

	b.WriteString("#!/usr/bin/env bash\n")
	b.WriteString("set -euo pipefail\n\n")

	fmt.Fprintf(&b, "DEPLOY_ID=%q\n", cfg.DeploymentID)
	fmt.Fprintf(&b, "CONTAINER_NAME=%q\n", cfg.ContainerName)
	fmt.Fprintf(&b, "PROJECT_SLUG=%q\n", cfg.ProjectSlug)
	fmt.Fprintf(&b, "APP_SLUG=%q\n", cfg.AppSlug)
	b.WriteString("BUILD_ROOT=\"/var/lib/launch/projects/${PROJECT_SLUG}/${APP_SLUG}\"\n")
	b.WriteString("BUILD_DIR=\"${BUILD_ROOT}/_build/${DEPLOY_ID}\"\n")
	b.WriteString("mkdir -p \"${BUILD_ROOT}\"\n\n")

	b.WriteString("echo \"::LAUNCH::deploy_step::resolving_source\"\n")

	switch cfg.SourceType {
	case dockertypes.SourceTypeImage:
		b.WriteString(buildImageStanza(cfg))
	case dockertypes.SourceTypeGit:
		b.WriteString(buildGitStanza(cfg))
	case dockertypes.SourceTypeDockerfile:
		b.WriteString(buildDockerfileStanza(cfg))
	default:
		// Defensive: should be caught by service validation, but never
		// generate a script with no payload — that'd hang the runner.
		fmt.Fprintf(&b, "echo \"unknown source type: %s\" >&2; exit 1\n", cfg.SourceType)
	}

	// Common shutdown + start. The `|| true` after stop/rm is intentional —
	// `set -e` would otherwise fail the deploy on a fresh app that has no
	// container to stop. We tolerate the not-found case explicitly.
	b.WriteString(`
echo "::LAUNCH::deploy_step::stopping_old_container"
if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
  docker stop "${CONTAINER_NAME}" >/dev/null 2>&1 || true
  docker rm   "${CONTAINER_NAME}" >/dev/null 2>&1 || true
fi

# Materialise env vars to a 0600 file under /run/launch and pass via
# docker run --env-file. Two reasons:
#
#   1. Values never appear in ps output during container start. With
#      the old -e KEY=VALUE form they were briefly visible to any
#      local process on the host during the docker-run invocation
#      window. --env-file replaces that with just a path on argv.
#
#   2. /run/launch is tmpfs on systemd hosts (the only kind we
#      provision), so the cleartext values live in RAM only -- never
#      hit a journaled disk, never get backed up. The file is removed
#      as soon as docker run returns (Docker reads --env-file once at
#      container create and copies values into the container's env, so
#      the file is not needed afterwards).
#
# umask 077 in the subshell forces 0600 on creation -- chmod-after-
# write leaves a tiny race window. The heredoc delimiter is single-
# quoted so $ inside values is NOT shell-expanded.
mkdir -p /run/launch
chmod 700 /run/launch
ENV_FILE="/run/launch/${DEPLOY_ID}.env"
# Trap fires whether docker run succeeds, fails, or set -e aborts an
# earlier step. The "|| true" keeps the trap quiet during a fresh
# install where ENV_FILE may not have been created yet.
trap 'rm -f "${ENV_FILE:-}" 2>/dev/null || true' EXIT
(
  umask 077
  cat > "${ENV_FILE}" <<'LAUNCH_ENV_EOF'
`)

	// Emit one KEY=VALUE per line into the quoted heredoc.
	//
	// Format rules for `docker --env-file`:
	//   - one KEY=VALUE per line
	//   - no quoting (literal value, including surrounding quotes if any)
	//   - no multi-line values
	//   - lines starting with `#` are comments
	//
	// We drop keys with `=` (would yield an invalid identifier) and
	// values with newlines (would break the per-line contract). Both
	// are already rejected at write time by env_var_service; this is
	// belt-and-braces.
	for _, ev := range cfg.EnvVars {
		if strings.Contains(ev.Key, "=") {
			continue
		}
		if strings.ContainsAny(ev.Value, "\n\r") {
			// Service-layer validation should prevent this; if it
			// somehow reaches here, log a marker the deploy operator
			// can grep for and skip the row rather than producing a
			// corrupt env file.
			fmt.Fprintf(&b, "# launch: skipped %q (value contains newline)\n", ev.Key)
			continue
		}
		fmt.Fprintf(&b, "%s=%s\n", ev.Key, ev.Value)
	}

	b.WriteString(`LAUNCH_ENV_EOF
)

echo "::LAUNCH::deploy_step::starting_container"
# --network launch-network so Traefik can reach the container by DNS name.
# The network is provisioned during docker server setup (see
# internal/modules/server/tasks/docker_constants.go).
CONTAINER_ID=$(docker run -d \
  --name "${CONTAINER_NAME}" \
  --network launch-network \
  --env-file "${ENV_FILE}" \
`)

	// Restart policy. Default is unless-stopped (most apps want this);
	// override via build_config.restart_policy.
	restart := cfg.RestartPolicy
	if restart == "" {
		restart = "unless-stopped"
	}
	fmt.Fprintf(&b, "  --restart=%s \\\n", shellEscapeArg(restart))

	// Resource limits — both optional. CPU is in --cpus float format
	// (e.g. "0.5", "2"); memory is in -m byte-suffix format ("512m").
	if cfg.CPULimit != "" {
		fmt.Fprintf(&b, "  --cpus=%s \\\n", shellEscapeArg(cfg.CPULimit))
	}
	if cfg.MemoryLimit != "" {
		fmt.Fprintf(&b, "  -m %s \\\n", shellEscapeArg(cfg.MemoryLimit))
	}
	// Healthcheck — docker run takes the command as a shell string.
	if cfg.HealthcheckCommand != "" {
		fmt.Fprintf(&b, "  --health-cmd=%s \\\n", shellEscapeArg(cfg.HealthcheckCommand))
	}
	// Extra host:container ports.
	for _, p := range cfg.ExtraPorts {
		if p = strings.TrimSpace(p); p != "" {
			fmt.Fprintf(&b, "  -p %s \\\n", shellEscapeArg(p))
		}
	}

	// Env vars are NOT rendered here — they're already in the env file
	// (created before the docker run block) and referenced via the
	// `--env-file "${ENV_FILE}"` flag the parent script writes between
	// the stop-old-container step and the start-new-container line.
	// See the heredoc block above for the format + rationale.

	// Volumes: named → `-v <name>:<mount>`, bind → `-v <host>:<mount>`.
	// We assume docker create-on-demand for named volumes is fine; if
	// users want a pre-created volume with specific opts they can run
	// `docker volume create` first and reference it by name here.
	for _, v := range cfg.Volumes {
		switch v.Type {
		case "bind":
			if v.HostPath == "" {
				continue
			}
			fmt.Fprintf(&b, "  -v %s:%s \\\n", shellEscapeArg(v.HostPath), shellEscapeArg(v.MountPath))
		default: // "named"
			fmt.Fprintf(&b, "  -v %s:%s \\\n", shellEscapeArg(v.Name), shellEscapeArg(v.MountPath))
		}
	}

	b.WriteString(`  "${DOCKER_IMAGE}")
echo "::LAUNCH::container_id::${CONTAINER_ID}"
echo "::LAUNCH::image_ref::${DOCKER_IMAGE}"

# Env file has done its job (Docker copied values into the container at
# create time). Remove it immediately so it doesn't sit on tmpfs longer
# than necessary, and clear the EXIT trap so a downstream rm -rf on the
# build dir doesn't re-trigger the file-cleanup branch.
rm -f "${ENV_FILE}"
trap - EXIT

# Clean up the build directory on success. Failed builds leave it in
# place so an operator can SSH in and look at what the builder produced.
if [ -d "${BUILD_DIR}" ]; then
  rm -rf "${BUILD_DIR}"
fi

echo "::LAUNCH::deploy_step::done"
`)

	return b.String()
}

// buildImageStanza handles source_type=image: optionally login to a
// private registry, pull a pre-built image, logout. No build step.
//
// `docker login` reads the password from stdin (`--password-stdin`)
// so the secret never lands in `ps` output or the script's shell
// history. The login → pull → logout sequence is wrapped in a
// subshell so a non-auth pull failure still tears the login down.
//
// docker login + docker logout both print
//
//	"WARNING! Your credentials are stored unencrypted in
//	 /root/.docker/config.json. Configure a credential helper..."
//
// to stderr unconditionally — there's no --quiet flag. Operators
// can't act on it without host-level credential-helper setup that
// Launch deliberately doesn't take over, so we filter just those
// three known lines through process substitution. docker's exit
// code is preserved (proc-sub doesn't sit in the pipeline) so a
// genuine auth failure still surfaces. Bash-only — the remote
// script always runs under bash.
const dockerCredFilter = `2> >(grep -v -E 'credentials are stored unencrypted|Configure a credential helper|credential-store' >&2)`

func buildImageStanza(cfg DeployConfig) string {
	var b strings.Builder
	if cfg.RegistryUsername != "" && cfg.RegistryPassword != "" {
		b.WriteString("\necho \"::LAUNCH::deploy_step::registry_login\"\n")
		fmt.Fprintf(&b, "DOCKER_REGISTRY_URL=%q\n", cfg.RegistryURL)
		fmt.Fprintf(&b, "DOCKER_REGISTRY_USER=%q\n", cfg.RegistryUsername)
		// Heredoc the password so it doesn't land in `ps` or the
		// script's shell history. `set +x` is defensive against a
		// future `set -x` getting flipped on for debugging — keeps
		// the password out of any shell trace regardless.
		//
		// IMPORTANT: closing this block with `set +x` (not `set -x`)
		// so we don't accidentally LEAK shell tracing into the rest
		// of the deploy. An earlier version closed with `set -x` and
		// every subsequent docker command got echoed as `+ docker...`
		// in the captured log, which buried the actually-useful
		// program output behind a wall of trace lines.
		b.WriteString("set +x\n")
		// When the password is a registry-bearer (workflow-minted GHCR
		// pull token relay), `docker login` rejects it — GHCR's login
		// validator expects PATs / install tokens, not pre-minted
		// bearers. So instead of going through docker login we write
		// the bearer DIRECTLY to ~/.docker/config.json's
		// `auths.<registry>.registrytoken` field, which makes docker
		// attach `Authorization: Bearer <bearer>` to subsequent pulls
		// without any login round-trip.
		//
		// Detection: when the username is "oauth2" the deploy was
		// initiated by the GHA bearer-relay path; anything else
		// (e.g. "x-access-token" for install-token fallback,
		// real customer usernames for Docker Hub etc.) is a normal
		// PAT/credential and goes through docker login as before.
		b.WriteString(`if [ "${DOCKER_REGISTRY_USER}" = "oauth2" ]; then
  # Bearer-relay path. Write the token directly into the docker
  # config so subsequent pulls send Authorization: Bearer <token>
  # without trying to round-trip through docker login.
  REG_HOST="${DOCKER_REGISTRY_URL:-ghcr.io}"
  mkdir -p "${HOME}/.docker"
  cat <<DOCKER_CONFIG_EOF > "${HOME}/.docker/config.json"
{"auths":{"${REG_HOST}":{"registrytoken":"$(cat <<'LAUNCH_DOCKER_PW_EOF'
`)
		b.WriteString(cfg.RegistryPassword)
		b.WriteString(`
LAUNCH_DOCKER_PW_EOF
)"}}}
DOCKER_CONFIG_EOF
else
`)
		fmt.Fprintf(&b, "  if [ -n \"${DOCKER_REGISTRY_URL}\" ]; then\n")
		fmt.Fprintf(&b, "    docker login --username \"${DOCKER_REGISTRY_USER}\" --password-stdin \"${DOCKER_REGISTRY_URL}\" %s <<'LAUNCH_DOCKER_PW_EOF'\n", dockerCredFilter)
		b.WriteString(cfg.RegistryPassword)
		b.WriteString("\nLAUNCH_DOCKER_PW_EOF\n")
		b.WriteString("  else\n")
		fmt.Fprintf(&b, "    docker login --username \"${DOCKER_REGISTRY_USER}\" --password-stdin %s <<'LAUNCH_DOCKER_PW_EOF'\n", dockerCredFilter)
		b.WriteString(cfg.RegistryPassword)
		b.WriteString("\nLAUNCH_DOCKER_PW_EOF\n")
		b.WriteString("  fi\n")
		b.WriteString("fi\n")
		// (no `set -x` here — that was the bug; see comment above)
	}
	fmt.Fprintf(&b, `
echo "::LAUNCH::deploy_step::pulling_image"
DOCKER_IMAGE=%q
docker pull "${DOCKER_IMAGE}"
`, cfg.Image)
	if cfg.RegistryUsername != "" && cfg.RegistryPassword != "" {
		// Cleanup. The bearer-relay path manages its own config.json
		// directly so the canonical `docker logout` is a no-op there;
		// just overwrite the config.json back to an empty auths map
		// so the next deploy on the same host starts from a clean
		// slate. PAT path keeps `docker logout` since it owns the
		// credential helper entry.
		b.WriteString(`if [ "${DOCKER_REGISTRY_USER}" = "oauth2" ]; then
  if [ -f "${HOME}/.docker/config.json" ]; then
    echo '{"auths":{}}' > "${HOME}/.docker/config.json"
  fi
else
`)
		fmt.Fprintf(&b, "  if [ -n \"${DOCKER_REGISTRY_URL}\" ]; then\n")
		fmt.Fprintf(&b, "    docker logout \"${DOCKER_REGISTRY_URL}\" %s || true\n", dockerCredFilter)
		b.WriteString("  else\n")
		fmt.Fprintf(&b, "    docker logout %s || true\n", dockerCredFilter)
		b.WriteString("  fi\n")
		b.WriteString("fi\n")
	}
	return b.String()
}

// renderBuildSecretsPrelude writes each build secret to a 0600 file
// under /run/launch/${DEPLOY_ID}.bsec/ (tmpfs) and populates a bash
// array BUILD_SECRET_FLAGS the caller expands into the `docker build`
// command. Always populated (even with zero secrets) so callers can
// reference "${BUILD_SECRET_FLAGS[@]}" without conditional logic.
//
// Same security pattern as the runtime env-file:
//   - umask 077 in a subshell so files are 0600 on creation (no race)
//   - quoted heredoc ('LAUNCH_BSEC_EOF') so $ in values isn't expanded
//   - EXIT trap removes the whole directory on success or failure
//   - newline-containing values are skipped (would break the heredoc
//     format); service-layer validation already prevents this
//
// `set +u` around the trap is intentional — the deploy script runs
// under `set -euo pipefail`, and `${BUILD_SECRET_FLAGS[@]}` would
// otherwise trip on the empty case on bash 4.x. We restore `set -u`
// immediately after so the rest of the script keeps its strictness.
func renderBuildSecretsPrelude(b *strings.Builder, cfg DeployConfig) {
	b.WriteString(`
# Build-time secrets (BuildKit --mount=type=secret). Distinct from
# runtime env vars (those flow through --env-file at docker run). See
# tasks/deploy_application.go renderBuildSecretsPrelude for the
# longer-form rationale. Always emitted so the docker build line can
# safely reference "${BUILD_SECRET_FLAGS[@]}" — empty array on the
# no-secrets case is fine.
mkdir -p /run/launch
chmod 700 /run/launch
BSEC_DIR="/run/launch/${DEPLOY_ID}.bsec"
mkdir -p "${BSEC_DIR}"
chmod 700 "${BSEC_DIR}"
# Stack the trap with the env-file cleanup so docker build failures
# tear down BOTH temp surfaces. The ":-" guards handle the case where
# the script aborted before ENV_FILE or BSEC_DIR was set.
trap 'rm -f "${ENV_FILE:-}" 2>/dev/null || true; rm -rf "${BSEC_DIR:-}" 2>/dev/null || true' EXIT
declare -a BUILD_SECRET_FLAGS=()
`)
	for _, sec := range cfg.BuildSecrets {
		if !validBuildSecretFileName(sec.Name) {
			// Service-layer validation should catch this; defence in
			// depth so a corrupt row can't write to ../../etc/shadow.
			fmt.Fprintf(b, "# launch: skipped %q (invalid name)\n", sec.Name)
			continue
		}
		if strings.ContainsAny(sec.Value, "\n\r") {
			fmt.Fprintf(b, "# launch: skipped %q (value contains newline)\n", sec.Name)
			continue
		}
		// Subshell with umask 077 + quoted heredoc. The heredoc body
		// is written through `cat >` so an empty value still produces
		// a zero-byte file (BuildKit accepts these; the Dockerfile
		// gets an empty /run/secrets/<NAME>).
		fmt.Fprintf(b, `(
  umask 077
  cat > "${BSEC_DIR}/%s" <<'LAUNCH_BSEC_EOF'
%s
LAUNCH_BSEC_EOF
)
BUILD_SECRET_FLAGS+=(--secret "id=%s,src=${BSEC_DIR}/%s")
`, sec.Name, sec.Value, sec.Name, sec.Name)
	}
	// BuildKit is required for --secret to work. Existing dockerd
	// installs from our provisioning script have it bundled; this
	// just turns the buildx interpreter on for the build command.
	b.WriteString(`export DOCKER_BUILDKIT=1
`)
}

// renderBuildSecretsTeardown removes the BSEC_DIR explicitly on the
// success path. The EXIT trap from the prelude catches failure paths;
// this is a belt-and-braces immediate cleanup so the values don't sit
// on tmpfs longer than necessary. Trap is NOT cleared here because
// the env-file teardown still needs it later in the script.
func renderBuildSecretsTeardown(b *strings.Builder) {
	b.WriteString(`
# Build done — bsec files are no longer needed. Trap stays armed for
# the runtime env-file cleanup later in the script.
rm -rf "${BSEC_DIR:-}" 2>/dev/null || true
`)
}

// validBuildSecretFileName rejects path-traversal and shell-special
// characters in the secret name before we use it as a filename. The
// service layer already enforces the env-name regex; this is the
// last-line check before touching the filesystem.
func validBuildSecretFileName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '_':
		default:
			return false
		}
	}
	return true
}

// buildGitStanza handles source_type=git: clone the repo into the build
// dir, run docker build, then run. Public-repo only in phase 2 — private
// repos via source-control creds land in slice 2g (see plan).
//
// BuildType determines the actual build command. Nixpacks is the default
// when no Dockerfile is present at the repo root; the script auto-detects.
func buildGitStanza(cfg DeployConfig) string {
	dockerfilePath := cfg.DockerfilePath
	if dockerfilePath == "" {
		dockerfilePath = "Dockerfile"
	}
	var b strings.Builder
	fmt.Fprintf(&b, `
echo "::LAUNCH::deploy_step::cloning_repository"
mkdir -p "${BUILD_DIR}"
git clone --depth 1 --branch %q %q "${BUILD_DIR}"
cd "${BUILD_DIR}"

echo "::LAUNCH::deploy_step::building_image"
DOCKER_IMAGE="launch/${PROJECT_SLUG}-${APP_SLUG}:${DEPLOY_ID}"
BUILD_TYPE=%q
DOCKERFILE_PATH=%q

# Auto-detect: if the user picked nixpacks but a Dockerfile sits at the
# given path, prefer it. Saves a footgun for repos that have a Dockerfile
# but were created with the default builder.
if [ "${BUILD_TYPE}" = "nixpacks" ] && [ -f "${DOCKERFILE_PATH}" ]; then
  BUILD_TYPE="dockerfile"
fi
`, cfg.GitBranch, cfg.GitRepo, cfg.BuildType, dockerfilePath)

	// Build secrets prelude must come BEFORE the build invocation so
	// BUILD_SECRET_FLAGS is in scope when docker build runs.
	renderBuildSecretsPrelude(&b, cfg)

	// Nixpacks doesn't support BuildKit --secret. If the user has
	// build secrets configured but is on nixpacks we surface that
	// loudly rather than silently dropping them.
	b.WriteString(`
case "${BUILD_TYPE}" in
  dockerfile)
    docker build "${BUILD_SECRET_FLAGS[@]}" -t "${DOCKER_IMAGE}" -f "${DOCKERFILE_PATH}" .
    ;;
  nixpacks)
    if [ "${#BUILD_SECRET_FLAGS[@]}" -gt 0 ]; then
      echo "::LAUNCH::warning::nixpacks does not support --mount=type=secret; build secrets will not be available during this build" >&2
    fi
    if ! command -v nixpacks >/dev/null 2>&1; then
      echo "nixpacks not installed on this server" >&2
      exit 1
    fi
    nixpacks build . --name "${DOCKER_IMAGE}"
    ;;
  *)
    echo "unsupported build type: ${BUILD_TYPE}" >&2
    exit 1
    ;;
esac
`)
	renderBuildSecretsTeardown(&b)
	return b.String()
}

// buildDockerfileStanza handles source_type=dockerfile: write the user-
// pasted Dockerfile contents into the build dir and `docker build` it.
//
// We use a heredoc with a quoted delimiter ('LAUNCH_DOCKERFILE_EOF') so
// shell variables inside the pasted Dockerfile aren't expanded by the
// deploy script — they belong to the docker build context, not us.
func buildDockerfileStanza(cfg DeployConfig) string {
	var b strings.Builder
	fmt.Fprintf(&b, `
echo "::LAUNCH::deploy_step::writing_dockerfile"
mkdir -p "${BUILD_DIR}"
cd "${BUILD_DIR}"
cat > Dockerfile <<'LAUNCH_DOCKERFILE_EOF'
%s
LAUNCH_DOCKERFILE_EOF

echo "::LAUNCH::deploy_step::building_image"
DOCKER_IMAGE="launch/${PROJECT_SLUG}-${APP_SLUG}:${DEPLOY_ID}"
`, cfg.DockerfileContents)
	renderBuildSecretsPrelude(&b, cfg)
	b.WriteString(`docker build "${BUILD_SECRET_FLAGS[@]}" -t "${DOCKER_IMAGE}" .
`)
	renderBuildSecretsTeardown(&b)
	return b.String()
}

// SlugFromName converts a free-form name into a safe filesystem/path
// segment. Lower-cased ASCII letters/digits/dashes only; collapses
// other runs to single dashes; trims leading/trailing dashes.
//
// Exposed because the job layer needs it for both Application.Name and
// Project.Name when building the deploy config — keep one definition.
func SlugFromName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	prevDash := false
	for _, r := range name {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevDash = false
		case r == '-' || r == '_':
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	out := strings.TrimRight(b.String(), "-")
	if out == "" {
		out = "app"
	}
	return out
}

// ContainerNameFor produces the docker container name a deploy will use.
// Pattern: launch-<project>-<app>. We don't include the deploy ID so the
// container can be safely replaced in-place by the next deploy.
func ContainerNameFor(project *models.Project, app *models.Application) string {
	return fmt.Sprintf("launch-%s-%s", SlugFromName(project.Name), SlugFromName(app.Name))
}
