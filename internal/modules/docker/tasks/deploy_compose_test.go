package tasks

import (
	"strings"
	"testing"
)

func TestBuildComposeDeployScript_RawYAML(t *testing.T) {
	s := buildComposeDeployScript(ComposeDeployConfig{
		DeploymentID: "01HZ",
		ProjectSlug:  "acme",
		ComposeSlug:  "stack",
		ProjectName:  "acme-stack",
		RawYAML:      "version: \"3\"\nservices:\n  web:\n    image: nginx\n",
	})
	// Quoted heredoc — variable expansion inside the user's YAML must
	// stay in the docker-compose layer, not be evaluated by our deploy
	// script. Pin the delimiter.
	mustContain(t, s, "<<'LAUNCH_COMPOSE_EOF'")
	mustContain(t, s, "image: nginx")
	mustContain(t, s, `docker compose --project-name "${COMPOSE_PROJECT_NAME}"`)
	// Raw-YAML path doesn't clone.
	if strings.Contains(s, "git clone") {
		t.Fatalf("raw-yaml deploy shouldn't include git clone:\n%s", s)
	}
}

// TestBuildComposeDeployScript_RecreateOnly pins the compose "Reload"
// path: it forces a plain no-build `up -d`, IGNORING a RunCommand that
// would otherwise carry --build, and rewrites the .env so saved env
// applies by reusing the on-host images.
func TestBuildComposeDeployScript_RecreateOnly(t *testing.T) {
	s := buildComposeDeployScript(ComposeDeployConfig{
		DeploymentID: "01HZ",
		ProjectSlug:  "acme",
		ComposeSlug:  "stack",
		ProjectName:  "acme-stack",
		RawYAML:      "services:\n  web:\n    build: .\n",
		EnvFile:      "FOO=bar\n",
		RunCommand:   "compose -p acme-stack -f docker-compose.yml up -d --build",
		RecreateOnly: true,
	})
	// Forces the structured no-build up, not the --build RunCommand.
	mustContain(t, s, `up -d --remove-orphans`)
	if strings.Contains(s, "--build") {
		t.Fatalf("reload must not pass --build:\n%s", s)
	}
	// Refreshed .env is written so new env applies on recreate.
	mustContain(t, s, "FOO=bar")
}

func TestBuildComposeDeployScript_Git(t *testing.T) {
	s := buildComposeDeployScript(ComposeDeployConfig{
		DeploymentID:    "01HZ",
		ProjectSlug:     "acme",
		ComposeSlug:     "stack",
		ProjectName:     "acme-stack",
		GitRepo:         "https://github.com/acme/stack",
		GitBranch:       "main",
		ComposeFilePath: "deploy/docker-compose.yml",
	})
	mustContain(t, s, `git clone --depth 1 --branch "main" "https://github.com/acme/stack"`)
	mustContain(t, s, `COMPOSE_FILE_PATH="deploy/docker-compose.yml"`)
	// Empty body for raw_yaml shouldn't slip into the git path.
	if strings.Contains(s, "LAUNCH_COMPOSE_EOF") {
		t.Fatalf("git deploy shouldn't include the raw-yaml heredoc:\n%s", s)
	}
}

func TestBuildComposeDeployScript_GitDefaultsComposeFile(t *testing.T) {
	s := buildComposeDeployScript(ComposeDeployConfig{
		DeploymentID: "01HZ",
		ProjectSlug:  "acme",
		ComposeSlug:  "stack",
		ProjectName:  "acme-stack",
		GitRepo:      "https://github.com/acme/stack",
		GitBranch:    "main",
		// ComposeFilePath intentionally empty — script must default to
		// docker-compose.yml. A missing default would silently break
		// every git-source deploy.
	})
	mustContain(t, s, `COMPOSE_FILE_PATH="docker-compose.yml"`)
}

func TestBuildComposeDeployScript_NoRegistryLogins(t *testing.T) {
	// Empty RegistryLogins → no docker login block emitted. Locks
	// the invariant that public-image stacks deploy without any
	// auth machinery in the rendered script.
	s := buildComposeDeployScript(ComposeDeployConfig{
		DeploymentID: "01HZ",
		ProjectSlug:  "acme",
		ComposeSlug:  "stack",
		ProjectName:  "acme-stack",
		RawYAML:      "services:\n  web:\n    image: nginx\n",
	})
	mustNotContain(t, s, "docker login")
	mustNotContain(t, s, "docker logout")
	mustNotContain(t, s, "registry_login")
}

func TestBuildComposeDeployScript_MultipleRegistryLogins(t *testing.T) {
	// Compose stacks can pull from N registries; the renderer emits
	// one paired login/logout block per attached credential. Test
	// two different hosts to confirm the per-iteration sentinels +
	// per-iteration env-var indices both increment.
	s := buildComposeDeployScript(ComposeDeployConfig{
		DeploymentID: "01HZ",
		ProjectSlug:  "acme",
		ComposeSlug:  "stack",
		ProjectName:  "acme-stack",
		RawYAML:      "services:\n  web:\n    image: ghcr.io/acme/web\n",
		RegistryLogins: []ComposeRegistryLogin{
			{RegistryURL: "ghcr.io", Username: "kkz6", Password: "ghp_one"},
			{RegistryURL: "", Username: "dockerhubuser", Password: "dh_two"},
		},
	})
	// Per-iteration variable names so two logins don't clobber each
	// other's URL/USER values.
	mustContain(t, s, `DOCKER_REGISTRY_URL_0="ghcr.io"`)
	mustContain(t, s, `DOCKER_REGISTRY_USER_0="kkz6"`)
	mustContain(t, s, `DOCKER_REGISTRY_URL_1=""`)
	mustContain(t, s, `DOCKER_REGISTRY_USER_1="dockerhubuser"`)
	// Per-iteration heredoc sentinels — same reason.
	mustContain(t, s, "<<'LAUNCH_DOCKER_PW_EOF_0'")
	mustContain(t, s, "<<'LAUNCH_DOCKER_PW_EOF_1'")
	// Both passwords land in the script (piped via stdin, not on cmdline).
	mustContain(t, s, "ghp_one")
	mustContain(t, s, "dh_two")
	// `set +x` defensively wraps the login block.
	mustContain(t, s, "set +x")
	// REGRESSION: must NOT enable `set -x` anywhere in the rendered
	// script. An earlier version closed the login block with
	// `set -x` and every subsequent docker command leaked into the
	// log as `+ command` shell-trace. See the same assertion +
	// rationale in deploy_application_test.go.
	mustNotContain(t, s, "set -x")
	// Logout pairs run AFTER compose up — both URLs accounted for.
	// The stderr filter for the credential warning sits between the
	// command and the `|| true`.
	mustContain(t, s, `docker logout "${DOCKER_REGISTRY_URL_0}"`)
	mustContain(t, s, `docker logout "${DOCKER_REGISTRY_URL_1}"`)
	mustContain(t, s, ") || true")
}
