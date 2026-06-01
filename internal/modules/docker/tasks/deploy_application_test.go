package tasks

import (
	"strings"
	"testing"

	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
)

func TestSlugFromName(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"api", "api"},
		{"My API!", "my-api"},
		{"acme-prod", "acme-prod"},
		{"under_score", "under-score"},
		{"   spaces   ", "spaces"},
		// Collapsed dashes — multiple non-alphanumeric runs become one.
		{"foo/bar/baz", "foo-bar-baz"},
		// Trailing punctuation shouldn't leave a dangling dash.
		{"trailing-", "trailing"},
		// Empty input gets a default.
		{"", "app"},
		{"!!!", "app"},
	}
	for _, c := range cases {
		got := SlugFromName(c.in)
		if got != c.want {
			t.Errorf("SlugFromName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestBuildDeployScript_Image(t *testing.T) {
	s := buildDeployScript(DeployConfig{
		DeploymentID:  "01HZ",
		ProjectSlug:   "acme",
		AppSlug:       "api",
		ContainerName: "launch-acme-api",
		SourceType:    dockertypes.SourceTypeImage,
		Image:         "nginx:1.27",
	})
	mustContain(t, s, "set -euo pipefail")
	mustContain(t, s, `docker pull "${DOCKER_IMAGE}"`)
	// We intentionally don't pull :latest implicitly — the user passes a tag.
	mustContain(t, s, `DOCKER_IMAGE="nginx:1.27"`)
	mustContain(t, s, `::LAUNCH::container_id::`)
	// No git stanza should leak into an image deploy.
	mustNotContain(t, s, "git clone")
	mustNotContain(t, s, "docker build")
}

// TestBuildDeployScript_RecreateOnly pins the "Reload" path: even for a
// git/dockerfile app (which would normally build), RecreateOnly skips
// the build/clone/pull and re-runs the existing image with the fresh
// env, preferring the running container's actual image.
func TestBuildDeployScript_RecreateOnly(t *testing.T) {
	s := buildDeployScript(DeployConfig{
		DeploymentID:  "01HZ",
		ProjectSlug:   "acme",
		AppSlug:       "api",
		ContainerName: "launch-acme-api",
		SourceType:    dockertypes.SourceTypeGit, // would normally build…
		RecreateOnly:  true,                      // …but reload skips it
		Image:         "ghcr.io/acme/api:launch-abc1234",
		EnvVars:       []EnvVar{{Key: "FOO", Value: "bar"}},
	})
	// Fallback image is the latest deployment's image_ref…
	mustContain(t, s, `DOCKER_IMAGE="ghcr.io/acme/api:launch-abc1234"`)
	// …but it prefers the running container's actual image.
	mustContain(t, s, `docker inspect --format '{{.Config.Image}}' "${CONTAINER_NAME}"`)
	// Guards the image is present locally — no pull on the reload path.
	mustContain(t, s, `docker image inspect "${DOCKER_IMAGE}"`)
	// Fresh env is written and the container is recreated.
	mustContain(t, s, "FOO=bar")
	mustContain(t, s, "docker run -d")
	// The whole point: no build / clone / pull.
	mustNotContain(t, s, "git clone")
	mustNotContain(t, s, "docker build")
	mustNotContain(t, s, `docker pull "${DOCKER_IMAGE}"`)
}

func TestBuildDeployScript_Git_DefaultDockerfilePath(t *testing.T) {
	s := buildDeployScript(DeployConfig{
		DeploymentID:  "01HZ",
		ProjectSlug:   "acme",
		AppSlug:       "api",
		ContainerName: "launch-acme-api",
		SourceType:    dockertypes.SourceTypeGit,
		GitRepo:       "https://github.com/acme/api",
		GitBranch:     "main",
		BuildType:     dockertypes.BuildTypeDockerfile,
		// DockerfilePath intentionally empty — script should default to Dockerfile.
	})
	mustContain(t, s, `git clone --depth 1 --branch "main" "https://github.com/acme/api"`)
	mustContain(t, s, `DOCKERFILE_PATH="Dockerfile"`)
	// Auto-upgrade rule: nixpacks with a Dockerfile present must flip to dockerfile.
	mustContain(t, s, `BUILD_TYPE="dockerfile"`)
}

func TestBuildDeployScript_Dockerfile_QuotedHeredoc(t *testing.T) {
	contents := "FROM alpine:3.20\nENV FOO=$BAR\nCMD [\"echo\", \"hi\"]\n"
	s := buildDeployScript(DeployConfig{
		DeploymentID:       "01HZ",
		ProjectSlug:        "acme",
		AppSlug:            "api",
		ContainerName:      "launch-acme-api",
		SourceType:         dockertypes.SourceTypeDockerfile,
		DockerfileContents: contents,
	})
	// The quoted heredoc delimiter is what keeps $BAR un-expanded — if
	// someone changes the delimiter without quotes, env vars inside the
	// user's Dockerfile would leak through the shell. Lock it.
	mustContain(t, s, "<<'LAUNCH_DOCKERFILE_EOF'")
	mustContain(t, s, "FROM alpine:3.20")
	mustContain(t, s, `ENV FOO=$BAR`)
}

// --- Image source: registry authentication -----------------------

func TestBuildDeployScript_Image_NoRegistryAuth(t *testing.T) {
	// Public image — no docker login block should appear. Locks the
	// invariant that an empty username silently skips the auth path.
	s := buildDeployScript(DeployConfig{
		DeploymentID:  "01HZ",
		ProjectSlug:   "acme",
		AppSlug:       "api",
		ContainerName: "launch-acme-api",
		SourceType:    dockertypes.SourceTypeImage,
		Image:         "nginx:1.27",
	})
	mustNotContain(t, s, "docker login")
	mustNotContain(t, s, "docker logout")
	mustNotContain(t, s, "registry_login")
}

func TestBuildDeployScript_Image_RegistryAuth_NamedHost(t *testing.T) {
	// Saved-credential / inline-with-URL path. RegistryURL non-empty
	// → docker login receives the host arg; logout pairs with it.
	s := buildDeployScript(DeployConfig{
		DeploymentID:     "01HZ",
		ProjectSlug:      "acme",
		AppSlug:          "api",
		ContainerName:    "launch-acme-api",
		SourceType:       dockertypes.SourceTypeImage,
		Image:            "ghcr.io/acme/api:v3",
		RegistryURL:      "ghcr.io",
		RegistryUsername: "kkz6",
		RegistryPassword: "ghp_secret_token_value",
	})
	mustContain(t, s, "registry_login")
	mustContain(t, s, `DOCKER_REGISTRY_URL="ghcr.io"`)
	mustContain(t, s, `DOCKER_REGISTRY_USER="kkz6"`)
	// Password piped via stdin heredoc — not on the cmdline.
	mustContain(t, s, "docker login --username")
	mustContain(t, s, "--password-stdin")
	mustContain(t, s, "<<'LAUNCH_DOCKER_PW_EOF'")
	mustContain(t, s, "ghp_secret_token_value")
	// `set +x` wraps the login block defensively even though the
	// outer script doesn't enable -x — keeps the toggle visible if
	// debug tracing gets added later.
	mustContain(t, s, "set +x")
	// REGRESSION: an earlier version closed the login block with
	// `set -x`, which LEAKED shell tracing across the rest of the
	// deploy and buried the captured log under `+ command` echoes.
	// Must NOT appear — there's no scenario where we want tracing
	// on for the whole script. (`set +x` is fine; `set -x` is not.)
	mustNotContain(t, s, "set -x")
	// docker login's "credentials stored unencrypted" warning gets
	// filtered out via a stderr process substitution. Same filter
	// is applied to docker logout — it touches the same config.json
	// and emits the same warning. Operators can't act on that
	// warning without host-level credential helper setup that
	// Launch deliberately doesn't take over.
	mustContain(t, s, "credentials are stored unencrypted")
	mustContain(t, s, "Configure a credential helper")
	// Logout pairs with the login AND carries the stderr filter
	// (the credential warning fires on logout too).
	mustContain(t, s, `docker logout "${DOCKER_REGISTRY_URL}"`)
	// Count of stderr-filter occurrences: 2x login (URL set / not set)
	// + 2x logout (URL set / not set) = 4. Catches a future
	// regression where someone drops the filter from logout (only)
	// without touching login — the warning would silently return.
	if got := strings.Count(s, "credentials are stored unencrypted"); got != 4 {
		t.Fatalf("stderr filter must wrap BOTH docker login AND docker logout — got %d occurrences, want 4", got)
	}
}

func TestBuildDeployScript_Image_RegistryAuth_DockerHub(t *testing.T) {
	// Docker Hub path — RegistryURL empty. The `if -z` branch in the
	// renderer fires `docker login` (and logout) with NO host arg.
	s := buildDeployScript(DeployConfig{
		DeploymentID:     "01HZ",
		ProjectSlug:      "acme",
		AppSlug:          "api",
		ContainerName:    "launch-acme-api",
		SourceType:       dockertypes.SourceTypeImage,
		Image:            "acme/api:v3",
		RegistryURL:      "",
		RegistryUsername: "kkz6",
		RegistryPassword: "secret",
	})
	mustContain(t, s, "registry_login")
	// Branch with no host — same heredoc sentinel, no URL argument.
	mustContain(t, s, `if [ -n "${DOCKER_REGISTRY_URL}" ]; then`)
	mustContain(t, s, "else")
	mustContain(t, s, "fi")
	// Logout also conditional on URL presence. The stderr filter
	// for the credential warning sits between the command and the
	// `|| true`, so the literal-string assertion would over-tighten;
	// just confirm both halves are present in order.
	mustContain(t, s, "docker logout 2> >(")
	mustContain(t, s, ") || true")
}

// --- Run-line: env vars, volumes, build_config knobs --------------

func TestBuildDeployScript_RunLine_AppliesBuildConfig(t *testing.T) {
	// Restart policy / CPU / memory / healthcheck / extra ports all
	// flow from cfg into shell-escaped flags on `docker run`. Locks
	// the contract so a future refactor that drops one is caught.
	s := buildDeployScript(DeployConfig{
		DeploymentID:       "01HZ",
		ProjectSlug:        "acme",
		AppSlug:            "api",
		ContainerName:      "launch-acme-api",
		SourceType:         dockertypes.SourceTypeImage,
		Image:              "nginx:1.27",
		RestartPolicy:      "on-failure",
		CPULimit:           "0.5",
		MemoryLimit:        "512m",
		HealthcheckCommand: "curl -f http://localhost",
		ExtraPorts:         []string{"8081:80", "9000:9000/tcp"},
	})
	mustContain(t, s, "--restart=on-failure")
	mustContain(t, s, "--cpus=0.5")
	mustContain(t, s, "-m 512m")
	mustContain(t, s, "--health-cmd=")
	// Both extra-port mappings emitted.
	mustContain(t, s, "-p '8081:80'")
	mustContain(t, s, "-p '9000:9000/tcp'")
}

func TestBuildDeployScript_RunLine_DefaultRestartPolicy(t *testing.T) {
	// Empty RestartPolicy → defaults to unless-stopped (no surprising
	// "no" default, that would silently break crash recovery).
	s := buildDeployScript(DeployConfig{
		DeploymentID:  "01HZ",
		ProjectSlug:   "acme",
		AppSlug:       "api",
		ContainerName: "launch-acme-api",
		SourceType:    dockertypes.SourceTypeImage,
		Image:         "nginx:1.27",
	})
	mustContain(t, s, "--restart=unless-stopped")
}

func TestBuildDeployScript_EnvVars_RenderedInQuotedHeredoc(t *testing.T) {
	// Env vars no longer go on the `docker run` command line as
	// `-e KEY=VALUE`. They're written to a 0600 file under
	// /run/launch (tmpfs) via a quoted heredoc and passed to docker
	// run via `--env-file`. This test pins the new format so a future
	// regression can't silently revert to argv-based env passing —
	// which would put values back into `ps` output during start.
	s := buildDeployScript(DeployConfig{
		DeploymentID:  "01HZ",
		ProjectSlug:   "acme",
		AppSlug:       "api",
		ContainerName: "launch-acme-api",
		SourceType:    dockertypes.SourceTypeImage,
		Image:         "nginx:1.27",
		EnvVars: []EnvVar{
			{Key: "DB_URL", Value: "postgres://u:p@host/db"},
			{Key: "TRICKY", Value: "he said 'hi'"},
		},
	})

	// The env-file write block is present.
	mustContain(t, s, `ENV_FILE="/run/launch/${DEPLOY_ID}.env"`)
	mustContain(t, s, "mkdir -p /run/launch")
	mustContain(t, s, "chmod 700 /run/launch")
	// umask 077 in the subshell forces 0600 on creation (no chmod
	// race). The single-quoted heredoc keeps $ from being expanded.
	mustContain(t, s, "umask 077")
	mustContain(t, s, "<<'LAUNCH_ENV_EOF'")
	// EXIT trap so a docker run failure still cleans up the file.
	mustContain(t, s, `trap 'rm -f "${ENV_FILE:-}"`)

	// Values land in the heredoc verbatim — no shell escaping, no
	// surrounding quotes (docker --env-file treats those literally).
	// The single quote in TRICKY's value stays a single quote.
	mustContain(t, s, "DB_URL=postgres://u:p@host/db")
	mustContain(t, s, "TRICKY=he said 'hi'")

	// docker run references the file via --env-file.
	mustContain(t, s, `--env-file "${ENV_FILE}"`)

	// And the old per-var argv form is GONE — guarding against a
	// regression where both paths emit and the values reappear in ps.
	mustNotContain(t, s, "-e DB_URL=")
	mustNotContain(t, s, "-e 'DB_URL=")

	// Post-run cleanup: rm + clear the trap so the build-dir cleanup
	// below doesn't bounce off a stale handler.
	mustContain(t, s, `rm -f "${ENV_FILE}"`)
	mustContain(t, s, "trap - EXIT")
}

func TestBuildDeployScript_EnvVars_DropsKeysContainingEquals(t *testing.T) {
	// Defense in depth: even if a corrupt row sneaks past the service
	// regex, the renderer drops keys containing `=`. In the old form
	// this was about `docker run -e FOO=BAR=value` parsing as a var
	// named `FOO=BAR` (invalid identifier). In the --env-file form
	// it'd corrupt the file's per-line KEY=VALUE contract. The drop
	// rule is the same; the assertion target moves to the heredoc.
	s := buildDeployScript(DeployConfig{
		DeploymentID:  "01HZ",
		ProjectSlug:   "acme",
		AppSlug:       "api",
		ContainerName: "launch-acme-api",
		SourceType:    dockertypes.SourceTypeImage,
		Image:         "nginx:1.27",
		EnvVars: []EnvVar{
			{Key: "OK_KEY", Value: "1"},
			{Key: "BAD=KEY", Value: "should-not-appear"},
		},
	})
	// OK row lands in the env file as a plain KEY=VALUE line.
	mustContain(t, s, "OK_KEY=1")
	// Bad key never makes it into the file.
	mustNotContain(t, s, "BAD=KEY")
	mustNotContain(t, s, "should-not-appear")
}

func TestBuildDeployScript_EnvVars_DropsValuesWithNewlines(t *testing.T) {
	// docker --env-file is line-oriented; a value containing \n would
	// split into two corrupt lines. Service-layer validation should
	// reject these at write time, but the renderer enforces it again
	// as belt-and-braces.
	s := buildDeployScript(DeployConfig{
		DeploymentID:  "01HZ",
		ProjectSlug:   "acme",
		AppSlug:       "api",
		ContainerName: "launch-acme-api",
		SourceType:    dockertypes.SourceTypeImage,
		Image:         "nginx:1.27",
		EnvVars: []EnvVar{
			{Key: "OK_KEY", Value: "single line"},
			{Key: "MULTI", Value: "line1\nline2"},
		},
	})
	mustContain(t, s, "OK_KEY=single line")
	// Skipped row emits a `# launch: skipped` comment marker so a
	// deploy operator grepping the script can see what was dropped.
	mustContain(t, s, `# launch: skipped "MULTI"`)
	// The literal value must NOT appear (would mean it leaked into the
	// heredoc and broke the per-line format).
	mustNotContain(t, s, "line1")
	mustNotContain(t, s, "line2")
}

func TestBuildDeployScript_BuildSecrets_GitStanzaRendersFlags(t *testing.T) {
	// Build secrets travel through `docker build --secret`, NOT through
	// the runtime env-file. Pin the rendered shape so a future change
	// can't silently move them onto -e or argv.
	s := buildDeployScript(DeployConfig{
		DeploymentID:  "01HZ",
		ProjectSlug:   "acme",
		AppSlug:       "api",
		ContainerName: "launch-acme-api",
		SourceType:    dockertypes.SourceTypeGit,
		GitRepo:       "https://github.com/acme/api.git",
		GitBranch:     "main",
		BuildType:     dockertypes.BuildTypeDockerfile,
		BuildSecrets: []BuildSecret{
			{Name: "NPM_TOKEN", Value: "npm_secret"},
			{Name: "GH_PAT", Value: "ghp_xxx"},
		},
	})

	// Prelude block — tmpfs dir creation + array bootstrap.
	mustContain(t, s, `BSEC_DIR="/run/launch/${DEPLOY_ID}.bsec"`)
	mustContain(t, s, `mkdir -p "${BSEC_DIR}"`)
	mustContain(t, s, "declare -a BUILD_SECRET_FLAGS=()")

	// Each secret writes to a 0600 file via quoted heredoc + appends to
	// the flag array. The heredoc delimiter prevents shell expansion of
	// $ characters in the value (guards against bash injecting from a
	// value like "$(rm -rf /)").
	mustContain(t, s, `cat > "${BSEC_DIR}/NPM_TOKEN" <<'LAUNCH_BSEC_EOF'
npm_secret`)
	mustContain(t, s, `cat > "${BSEC_DIR}/GH_PAT" <<'LAUNCH_BSEC_EOF'
ghp_xxx`)
	mustContain(t, s, `BUILD_SECRET_FLAGS+=(--secret "id=NPM_TOKEN,src=${BSEC_DIR}/NPM_TOKEN")`)
	mustContain(t, s, `BUILD_SECRET_FLAGS+=(--secret "id=GH_PAT,src=${BSEC_DIR}/GH_PAT")`)

	// umask 077 in the subshell forces 0600 on creation.
	mustContain(t, s, "umask 077")

	// BuildKit is required for --secret to be honored.
	mustContain(t, s, "export DOCKER_BUILDKIT=1")

	// EXIT trap covers both ENV_FILE (set later, in the runtime block)
	// and BSEC_DIR — same trap line handles both.
	mustContain(t, s, `trap 'rm -f "${ENV_FILE:-}"`)
	mustContain(t, s, `rm -rf "${BSEC_DIR:-}"`)

	// docker build line includes the array expansion BEFORE -t so the
	// flags land before image-tag and context positional args.
	mustContain(t, s, `docker build "${BUILD_SECRET_FLAGS[@]}" -t "${DOCKER_IMAGE}" -f "${DOCKERFILE_PATH}" .`)

	// Post-build cleanup removes BSEC_DIR explicitly (belt-and-braces;
	// the trap would also catch a later failure).
	mustContain(t, s, `rm -rf "${BSEC_DIR:-}"`)

	// And the values must NOT appear anywhere outside the heredoc —
	// guard against an accidental echo or expansion. Substring match
	// over the body where they shouldn't be.
	mustNotContain(t, s, "-e NPM_TOKEN=")
	mustNotContain(t, s, "--build-arg NPM_TOKEN")
}

func TestBuildDeployScript_BuildSecrets_DockerfileStanzaRendersFlags(t *testing.T) {
	// Same checks for the Dockerfile-paste source variant. Pasted
	// Dockerfile lands in the build dir via its own quoted heredoc;
	// build secrets follow the same shape as in the git stanza.
	s := buildDeployScript(DeployConfig{
		DeploymentID:       "01HZ",
		ProjectSlug:        "acme",
		AppSlug:            "api",
		ContainerName:      "launch-acme-api",
		SourceType:         dockertypes.SourceTypeDockerfile,
		DockerfileContents: "FROM alpine:3.20\nRUN echo hi",
		BuildSecrets: []BuildSecret{
			{Name: "PIP_INDEX_URL", Value: "https://user:pass@example.com/pypi"},
		},
	})
	mustContain(t, s, `BSEC_DIR="/run/launch/${DEPLOY_ID}.bsec"`)
	mustContain(t, s, `BUILD_SECRET_FLAGS+=(--secret "id=PIP_INDEX_URL,src=${BSEC_DIR}/PIP_INDEX_URL")`)
	mustContain(t, s, "export DOCKER_BUILDKIT=1")
	mustContain(t, s, `docker build "${BUILD_SECRET_FLAGS[@]}" -t "${DOCKER_IMAGE}" .`)
}

func TestBuildDeployScript_BuildSecrets_NoSecretsStillEmitsArray(t *testing.T) {
	// Empty BuildSecrets list must still emit the array declaration —
	// otherwise `"${BUILD_SECRET_FLAGS[@]}"` in the docker build line
	// would expand to an unbound-var error under `set -u`. The empty
	// array branch is the common case (most apps don't use secrets).
	s := buildDeployScript(DeployConfig{
		DeploymentID:  "01HZ",
		ProjectSlug:   "acme",
		AppSlug:       "api",
		ContainerName: "launch-acme-api",
		SourceType:    dockertypes.SourceTypeGit,
		GitRepo:       "https://github.com/acme/api.git",
		GitBranch:     "main",
		BuildType:     dockertypes.BuildTypeDockerfile,
	})
	mustContain(t, s, "declare -a BUILD_SECRET_FLAGS=()")
	// No BUILD_SECRET_FLAGS+= lines on the no-secrets path.
	mustNotContain(t, s, "BUILD_SECRET_FLAGS+=(--secret")
	// docker build still references the array — expansion is fine on
	// an empty array.
	mustContain(t, s, `docker build "${BUILD_SECRET_FLAGS[@]}"`)
}

func TestBuildDeployScript_BuildSecrets_RejectsInvalidNames(t *testing.T) {
	// Service-layer validation already enforces the env-name regex;
	// the renderer reinforces it as defence-in-depth so a corrupt row
	// (or a future bypass) can't traverse out of BSEC_DIR via "../".
	s := buildDeployScript(DeployConfig{
		DeploymentID:  "01HZ",
		ProjectSlug:   "acme",
		AppSlug:       "api",
		ContainerName: "launch-acme-api",
		SourceType:    dockertypes.SourceTypeGit,
		GitRepo:       "https://github.com/acme/api.git",
		GitBranch:     "main",
		BuildType:     dockertypes.BuildTypeDockerfile,
		BuildSecrets: []BuildSecret{
			{Name: "../etc/shadow", Value: "stolen"},
			{Name: "OK_ONE", Value: "ok"},
		},
	})
	// Bad name gets a comment marker + is skipped; good name still
	// makes it through.
	mustContain(t, s, "# launch: skipped")
	mustContain(t, s, "OK_ONE")
	// The bad name's value must NOT appear in the script.
	mustNotContain(t, s, "stolen")
}

func TestBuildDeployScript_BuildSecrets_DropsValuesWithNewlines(t *testing.T) {
	// Newlines would break the per-line heredoc format. Same drop +
	// marker pattern as env-vars (TestBuildDeployScript_EnvVars_DropsValuesWithNewlines).
	s := buildDeployScript(DeployConfig{
		DeploymentID:  "01HZ",
		ProjectSlug:   "acme",
		AppSlug:       "api",
		ContainerName: "launch-acme-api",
		SourceType:    dockertypes.SourceTypeGit,
		GitRepo:       "https://github.com/acme/api.git",
		GitBranch:     "main",
		BuildType:     dockertypes.BuildTypeDockerfile,
		BuildSecrets: []BuildSecret{
			{Name: "MULTI", Value: "line1\nline2"},
			{Name: "OK", Value: "single"},
		},
	})
	mustContain(t, s, `# launch: skipped "MULTI"`)
	mustContain(t, s, "OK")
	// The literal value must NOT appear (would mean it leaked into the
	// heredoc and broke the per-line format).
	mustNotContain(t, s, "line1")
	mustNotContain(t, s, "line2")
}

func TestBuildDeployScript_BuildSecrets_NotRenderedOnImageSource(t *testing.T) {
	// `source_type=image` skips the build step entirely (docker pull
	// only). The build-secrets prelude lives inside the git/dockerfile
	// stanzas, not the image stanza, so an image-source app with
	// nominally-set build secrets must not emit the prelude. Locks
	// that contract.
	s := buildDeployScript(DeployConfig{
		DeploymentID:  "01HZ",
		ProjectSlug:   "acme",
		AppSlug:       "api",
		ContainerName: "launch-acme-api",
		SourceType:    dockertypes.SourceTypeImage,
		Image:         "nginx:1.27",
		BuildSecrets: []BuildSecret{
			{Name: "SHOULD_NOT_APPEAR", Value: "ignored"},
		},
	})
	mustNotContain(t, s, "BSEC_DIR=")
	mustNotContain(t, s, "BUILD_SECRET_FLAGS")
	mustNotContain(t, s, "SHOULD_NOT_APPEAR")
}

func TestBuildDeployScript_Volumes_BindAndNamed(t *testing.T) {
	// Two mount kinds emit different `-v` shapes. The renderer picks
	// host_path for bind and volume name for named.
	s := buildDeployScript(DeployConfig{
		DeploymentID:  "01HZ",
		ProjectSlug:   "acme",
		AppSlug:       "api",
		ContainerName: "launch-acme-api",
		SourceType:    dockertypes.SourceTypeImage,
		Image:         "nginx:1.27",
		Volumes: []Volume{
			{Name: "data", MountPath: "/var/lib/data", Type: "named"},
			{Name: "logs", MountPath: "/var/log/app", Type: "bind", HostPath: "/opt/logs"},
		},
	})
	// shellEscapeArg quotes each HALF of the mount spec separately,
	// then the script joins them with `:`. Both halves here contain
	// only safe chars (letter/digit/_/-/./= plus `/`) so they render
	// unquoted.
	mustContain(t, s, "-v data:/var/lib/data")
	mustContain(t, s, "-v /opt/logs:/var/log/app")
	// Bind mount missing host_path is skipped (no empty :mount entry).
	mustNotContain(t, s, "-v :/var/log/app")
}

func mustContain(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("script missing %q\n----\n%s\n----", needle, haystack)
	}
}

func mustNotContain(t *testing.T, haystack, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Errorf("script unexpectedly contains %q", needle)
	}
}
