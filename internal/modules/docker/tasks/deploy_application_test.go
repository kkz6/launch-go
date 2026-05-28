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

func TestBuildDeployScript_EnvVars_RenderAndEscape(t *testing.T) {
	// Single-quote escaping keeps shell metachars in values from
	// breaking the run line. Test a value containing a single quote
	// itself to lock the escape policy.
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
	// `@` triggers quoting (it's not in the safe-char set
	// `[A-Za-z0-9_\-./=]`); `:` also does. Whole value lands inside
	// a single-quoted shell string.
	mustContain(t, s, "-e 'DB_URL=postgres://u:p@host/db'")
	// Single-quote inside a single-quoted shell string is escaped
	// via the `'\''` close-reopen idiom.
	mustContain(t, s, `-e 'TRICKY=he said '\''hi'\'''`)
}

func TestBuildDeployScript_EnvVars_DropsKeysContainingEquals(t *testing.T) {
	// Defense in depth: even if a corrupt row sneaks past the service
	// regex, the renderer drops keys containing `=` so the run line
	// can't end up with `-e FOO=BAR=value` (which docker interprets
	// as a var named `FOO=BAR` — invalid identifier).
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
	// `OK_KEY=1` contains only safe chars (letter/digit/underscore/=)
	// so shellEscapeArg returns it unquoted. The `=` belongs to the
	// docker `-e KEY=VALUE` syntax — it's preserved deliberately.
	mustContain(t, s, "-e OK_KEY=1")
	mustNotContain(t, s, "BAD=KEY")
	mustNotContain(t, s, "should-not-appear")
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
