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
