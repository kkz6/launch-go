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
