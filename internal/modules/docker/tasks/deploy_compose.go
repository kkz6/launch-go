package tasks

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// ComposeDeployConfig holds the runtime parameters for a compose deploy.
// Same shape rationale as DeployConfig: pure data so the script renderer
// is a deterministic function.
type ComposeDeployConfig struct {
	DeploymentID string
	ProjectSlug  string
	ComposeSlug  string

	// ProjectName is what we pass to `docker compose --project-name`.
	// Generated from project + compose slugs so containers across two
	// different compose stacks in the same project don't collide.
	ProjectName string

	// One of these is set depending on the compose source type.
	GitRepo         string
	GitBranch       string
	ComposeFilePath string

	RawYAML string

	// EnvFile is the body the Environment subtab persists into
	// `docker_composes.env_file`. When non-empty the deploy script
	// writes it to `${STACK_DIR}/.env` BEFORE `docker compose up`.
	// Compose reads .env from the project dir automatically for
	// ${VAR} substitution in the YAML and propagates matching keys
	// to services. Empty string = no .env file is written (and any
	// existing one is removed so a cleared Environment tab actually
	// takes effect on the next deploy).
	EnvFile string

	// RunCommand overrides the docker command suffix the deploy
	// script runs. When set, the script runs `docker <run_command>`
	// verbatim instead of the default
	// `docker compose -p NAME -f FILE up -d --build --remove-orphans`.
	// Use ComposeDefaultRunCommand to render the default for
	// preview / "default command" UI hints so the script + the hint
	// can never disagree.
	RunCommand string

	// FileMounts are the type=file rows attached to this compose stack.
	// Each is materialized to `${STACK_DIR}/files/<RelativePath>` on
	// the host BEFORE `docker compose up` so the YAML can reference
	// them via `./files/<RelativePath>:<container_path>:ro`. The deploy
	// script wipes the `files/` directory at the top of each run so
	// stale rows from previous deploys don't linger.
	//
	// Bind- and volume-type rows are NOT included here — those are
	// informational on the compose surface; the operator wires them
	// into the YAML themselves and we don't rewrite docker-compose.yml.
	FileMounts []ComposeFileMount

	// RegistryLogins are the 0..N saved-credential rows the stack
	// attached. The deploy script runs `docker login` for each
	// before `docker compose pull/up` so private images on any of
	// these registries resolve. We do a paired `docker logout` after
	// the compose command finishes to avoid leaking ambient creds in
	// the host's `~/.docker/config.json`.
	//
	// Plaintext only lives in this in-memory config struct; at-rest
	// stays encrypted via dbtype.EncryptedString on the saved row.
	RegistryLogins []ComposeRegistryLogin

	// ServiceImages, when non-empty, switches the compose deploy into
	// "GHA-built images" mode. For each (service → image) pair the
	// deploy script rewrites the compose file in place to replace the
	// service's `build:` block with `image: <image>`, then runs
	// `docker compose up -d` WITHOUT --build. Used by the GitHub
	// Actions webhook path — GHA built + pushed each image to GHCR
	// in a matrix job, and we just deploy them.
	//
	// Empty / nil = today's behaviour: build on the host with
	// `docker compose up --build`.
	ServiceImages map[string]string
}

// ComposeRegistryLogin is one resolved registry login. RegistryURL
// is empty for Docker Hub (the script omits the host arg to
// `docker login` in that case).
type ComposeRegistryLogin struct {
	RegistryURL string
	Username    string
	Password    string
}

// ComposeFileMount is a single type=file row materialized for a
// compose deploy. Pure data — the deploy script does the writing so
// the rendering function stays deterministic and side-effect-free.
type ComposeFileMount struct {
	// RelativePath is the on-host filename written under
	// `${STACK_DIR}/files/`. Subdirectories are honored (e.g.
	// "nginx/site.conf") — the script `mkdir -p`s the parent before
	// the write.
	RelativePath string
	// Content is the body written verbatim. Heredoc-quoted so shell
	// expansion can't corrupt config files that embed `$`-sigils.
	Content string
}

// ComposeDefaultRunCommand renders the docker-suffix the deploy
// script falls back to when no per-stack RunCommand override is set.
// Exposed so the frontend Advanced subtab can show the literal
// default in its "Default Command (...)" hint via an API endpoint —
// the hint and the actual deploy can never drift because both call
// this function.
func ComposeDefaultRunCommand(projectName, composeFilePath string) string {
	if composeFilePath == "" {
		composeFilePath = "docker-compose.yml"
	}
	return fmt.Sprintf(
		"compose -p %s -f %s up -d --build --remove-orphans",
		projectName, composeFilePath,
	)
}

// DeployCompose returns a taskrunner.Task that brings a compose stack
// up via `docker compose up -d`. Idempotent: `up` with no changes is a
// no-op; with changes, docker recreates only the services whose hashes
// changed.
//
// Timeout matches DeployApplication (30 minutes) — compose stacks with
// builds can take longer than a single app, but 30 minutes is a hard
// ceiling that catches truly stuck deploys.
func DeployCompose(cfg ComposeDeployConfig) taskrunner.Task {
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Deploy Compose Stack"),
		taskrunner.WithScript(buildComposeDeployScript(cfg)),
		taskrunner.WithTimeoutSeconds(1800),
	)
}

// buildComposeDeployScript composes the bash that the SSH runner
// executes. Source-specific stanzas mirror buildDeployScript for
// applications — keep them parallel so an operator who's read one can
// read the other.
func buildComposeDeployScript(cfg ComposeDeployConfig) string {
	var b strings.Builder
	b.WriteString("#!/usr/bin/env bash\n")
	b.WriteString("set -euo pipefail\n\n")

	fmt.Fprintf(&b, "DEPLOY_ID=%q\n", cfg.DeploymentID)
	fmt.Fprintf(&b, "PROJECT_SLUG=%q\n", cfg.ProjectSlug)
	fmt.Fprintf(&b, "COMPOSE_SLUG=%q\n", cfg.ComposeSlug)
	fmt.Fprintf(&b, "COMPOSE_PROJECT_NAME=%q\n", cfg.ProjectName)
	b.WriteString("STACK_DIR=\"/var/lib/launch/projects/${PROJECT_SLUG}/${COMPOSE_SLUG}\"\n\n")

	b.WriteString("mkdir -p \"${STACK_DIR}\"\n")
	b.WriteString("echo \"::LAUNCH::deploy_step::resolving_source\"\n")

	if cfg.RawYAML != "" {
		// Inline-YAML path: write the user-pasted content to a fresh
		// compose file at the stack dir. Quoted heredoc keeps shell
		// variables from being expanded by us — they belong to docker
		// compose's own variable substitution.
		b.WriteString(`echo "::LAUNCH::deploy_step::writing_compose"
cd "${STACK_DIR}"
cat > docker-compose.yml <<'LAUNCH_COMPOSE_EOF'
`)
		b.WriteString(cfg.RawYAML)
		// Make sure the heredoc body ends with a newline.
		if !strings.HasSuffix(cfg.RawYAML, "\n") {
			b.WriteString("\n")
		}
		b.WriteString("LAUNCH_COMPOSE_EOF\n")
		b.WriteString("COMPOSE_FILE_PATH=\"docker-compose.yml\"\n")
	} else if cfg.GitRepo != "" {
		// Git-clone path. Each deploy clones into a per-deploy dir
		// under _build so concurrent deploys can't race; on success we
		// rsync the working tree to STACK_DIR for stable file paths.
		fmt.Fprintf(&b, `echo "::LAUNCH::deploy_step::cloning_repository"
BUILD_DIR="${STACK_DIR}/_build/${DEPLOY_ID}"
mkdir -p "${BUILD_DIR}"
git clone --depth 1 --branch %q %q "${BUILD_DIR}"
cd "${BUILD_DIR}"
`, cfg.GitBranch, cfg.GitRepo)
		// Default to docker-compose.yml; user can override per-stack.
		composeFile := cfg.ComposeFilePath
		if composeFile == "" {
			composeFile = "docker-compose.yml"
		}
		fmt.Fprintf(&b, "COMPOSE_FILE_PATH=%q\n", composeFile)
		b.WriteString(`if [ ! -f "${COMPOSE_FILE_PATH}" ]; then
  echo "compose file not found at ${COMPOSE_FILE_PATH}" >&2
  exit 1
fi
`)
	} else {
		// Should be unreachable due to service validation — defensive.
		b.WriteString("echo \"compose deploy missing source\" >&2; exit 1\n")
	}

	// GHA-built images path: rewrite each named service's `build:` block
	// with `image: <ghcr image>` so the subsequent `docker compose up`
	// pulls instead of building. The customer's repo already has a
	// compose file with `build:` directives — GHA matrix-built each
	// service into GHCR, and we're just retargeting the YAML.
	//
	// We use yq (mikefarah's Go yq, which is widely available via snap
	// or a single-binary download). When yq isn't present we install
	// it; the download is ~5MB and idempotent across deploys.
	if len(cfg.ServiceImages) > 0 {
		b.WriteString(`echo "::LAUNCH::deploy_step::rewriting_compose_for_gha"
if ! command -v yq >/dev/null 2>&1; then
  echo "Installing yq for compose image rewriting"
  sudo curl -fsSL https://github.com/mikefarah/yq/releases/latest/download/yq_linux_amd64 -o /usr/local/bin/yq
  sudo chmod +x /usr/local/bin/yq
fi
`)
		// Sort for deterministic script output — golden tests would
		// otherwise see different bytes on each render because Go map
		// iteration order is randomised.
		serviceNames := make([]string, 0, len(cfg.ServiceImages))
		for name := range cfg.ServiceImages {
			serviceNames = append(serviceNames, name)
		}
		sort.Strings(serviceNames)
		for _, name := range serviceNames {
			image := cfg.ServiceImages[name]
			// Set image, remove build. Quoting the service name keeps
			// hyphenated or numeric-only service keys from breaking the
			// yq expression. We pass the image value as an env var so
			// special characters in tags (rare but possible) can't
			// terminate the yq string literal.
			fmt.Fprintf(&b,
				"LAUNCH_GHA_IMAGE=%q yq -i '.services.\"%s\".image = strenv(LAUNCH_GHA_IMAGE) | del(.services.\"%s\".build)' \"${COMPOSE_FILE_PATH}\"\n",
				image, name, name,
			)
		}
	}

	// Write or remove the `.env` file based on EnvFile content. For
	// raw_yaml the `cd "${STACK_DIR}"` already happened above; for
	// git deploys we `cd "${BUILD_DIR}"` so the .env writes there.
	// Either way relative paths resolve from PWD, so .env lives next
	// to the compose file like docker compose expects.
	if cfg.EnvFile != "" {
		b.WriteString(`echo "::LAUNCH::deploy_step::writing_env_file"
cat > .env <<'LAUNCH_COMPOSE_ENV_EOF'
`)
		b.WriteString(cfg.EnvFile)
		if !strings.HasSuffix(cfg.EnvFile, "\n") {
			b.WriteString("\n")
		}
		b.WriteString("LAUNCH_COMPOSE_ENV_EOF\n")
	} else {
		// Explicitly clear the file when EnvFile is empty so a
		// previously-set environment doesn't silently linger on the
		// host after the user clears the Environment tab. `rm -f`
		// tolerates the "no such file" case for first deploys.
		b.WriteString("rm -f .env\n")
	}

	// Materialize type=file volume rows to `${STACK_DIR}/files/`. We
	// wipe `files/` at the top of each deploy so rows removed from
	// the UI actually disappear on the host — otherwise stale files
	// would haunt the deploy directory forever. The `mkdir -p` on
	// each parent dir lets operators use subpaths like
	// "nginx/site.conf" without an extra round-trip.
	//
	// Heredoc tag uses a per-iteration sentinel so a payload that
	// itself contains "LAUNCH_FILE_EOF" can't terminate the heredoc
	// early. The tag varies per index; collisions on a content body
	// that happens to repeat the tag are still possible but vastly
	// less likely than a single shared sentinel.
	if len(cfg.FileMounts) > 0 {
		b.WriteString("\necho \"::LAUNCH::deploy_step::writing_files\"\n")
		// Resolve `files/` against PWD — same logic the .env block
		// uses, so raw-YAML deploys land at ${STACK_DIR}/files/ and
		// git deploys land at ${BUILD_DIR}/files/. The compose YAML's
		// `./files/<path>` relative paths line up either way.
		b.WriteString("rm -rf ./files && mkdir -p ./files\n")
		for i, fm := range cfg.FileMounts {
			tag := fmt.Sprintf("LAUNCH_FILE_EOF_%d", i)
			fmt.Fprintf(&b, "mkdir -p \"$(dirname \"./files/%s\")\"\n", fm.RelativePath)
			fmt.Fprintf(&b, "cat > ./files/%s <<'%s'\n", fm.RelativePath, tag)
			b.WriteString(fm.Content)
			if !strings.HasSuffix(fm.Content, "\n") {
				b.WriteString("\n")
			}
			fmt.Fprintf(&b, "%s\n", tag)
		}
	}

	// Registry logins — one `docker login` per attached saved
	// credential. Password piped via `--password-stdin` so it
	// doesn't appear in `ps`. Each login uses a per-iteration
	// heredoc sentinel so a password containing the sentinel can't
	// terminate it early.
	if len(cfg.RegistryLogins) > 0 {
		b.WriteString("\necho \"::LAUNCH::deploy_step::registry_login\"\n")
		b.WriteString("set +x\n")
		// Same stderr-filter applied to every login: docker emits the
		// "credentials stored unencrypted" warning unconditionally and
		// it's noise an operator can't act on without host-level
		// credential-helper setup that Launch doesn't take over. See
		// deploy_application.go#buildImageStanza for the rationale.
		const stderrFilter = `2> >(grep -v -E 'credentials are stored unencrypted|Configure a credential helper|credential-store' >&2)`
		for i, l := range cfg.RegistryLogins {
			tag := fmt.Sprintf("LAUNCH_DOCKER_PW_EOF_%d", i)
			fmt.Fprintf(&b, "DOCKER_REGISTRY_URL_%d=%q\n", i, l.RegistryURL)
			fmt.Fprintf(&b, "DOCKER_REGISTRY_USER_%d=%q\n", i, l.Username)
			fmt.Fprintf(&b, "if [ -n \"${DOCKER_REGISTRY_URL_%d}\" ]; then\n", i)
			fmt.Fprintf(&b,
				"  docker login --username \"${DOCKER_REGISTRY_USER_%d}\" --password-stdin \"${DOCKER_REGISTRY_URL_%d}\" %s <<'%s'\n",
				i, i, stderrFilter, tag,
			)
			b.WriteString(l.Password)
			if !strings.HasSuffix(l.Password, "\n") {
				b.WriteString("\n")
			}
			fmt.Fprintf(&b, "%s\n", tag)
			b.WriteString("else\n")
			fmt.Fprintf(&b,
				"  docker login --username \"${DOCKER_REGISTRY_USER_%d}\" --password-stdin %s <<'%s'\n",
				i, stderrFilter, tag,
			)
			b.WriteString(l.Password)
			if !strings.HasSuffix(l.Password, "\n") {
				b.WriteString("\n")
			}
			fmt.Fprintf(&b, "%s\n", tag)
			b.WriteString("fi\n")
		}
		b.WriteString("set -x\n")
	}

	// `--network launch-network` happens inside the compose file (each
	// service declares the network); the platform pre-creates it on the
	// server, but compose doesn't take a top-level --network flag the
	// way `docker run` does. Operators routing through Traefik must
	// declare external: true on the launch-network in their compose
	// file. This is the documented expectation.
	//
	// RunCommand override path: when the operator sets a custom
	// command via the Advanced subtab, we trust their string and run
	// `docker <run_command>` verbatim. This is the same shape dokploy
	// uses — the cost is the operator owns the whole tail including
	// `compose -p NAME -f FILE`, but the gain is no-cache / no-build /
	// bring-your-own-tail flexibility that's otherwise impossible to
	// express through structured flags.
	b.WriteString("\necho \"::LAUNCH::deploy_step::compose_up\"\n")
	if cfg.RunCommand != "" {
		// Single-line invocation so the user's command goes through
		// the shell exactly as written. They're responsible for any
		// quoting / escaping — same trust model dokploy applies.
		fmt.Fprintf(&b, "docker %s\n", cfg.RunCommand)
	} else {
		b.WriteString(`docker compose --project-name "${COMPOSE_PROJECT_NAME}" \
  -f "${COMPOSE_FILE_PATH}" \
  up -d --remove-orphans
`)
	}
	// Pair the logins with logouts so the host's docker config
	// doesn't gain stale credential entries. `|| true` keeps `set -e`
	// from failing the deploy if the logout itself errors — the
	// containers are already up, the deploy succeeded.
	if len(cfg.RegistryLogins) > 0 {
		for i := range cfg.RegistryLogins {
			fmt.Fprintf(&b, "if [ -n \"${DOCKER_REGISTRY_URL_%d}\" ]; then\n", i)
			fmt.Fprintf(&b, "  docker logout \"${DOCKER_REGISTRY_URL_%d}\" || true\n", i)
			b.WriteString("else\n")
			b.WriteString("  docker logout || true\n")
			b.WriteString("fi\n")
		}
	}
	b.WriteString("\necho \"::LAUNCH::deploy_step::done\"\n")

	return b.String()
}
