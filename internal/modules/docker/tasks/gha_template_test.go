package tasks

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// updateGolden lets us refresh the golden files when an intentional
// template change lands:
//
//	go test ./internal/modules/docker/tasks/ -run Workflow -update
//
// Default (no flag) compares rendered output against the on-disk
// golden file byte-for-byte, which is the CI-time invariant we care
// about: any unintended template drift breaks the build.
var updateGolden = flag.Bool("update", false, "rewrite the golden files instead of asserting against them")

const (
	goldenApplication = "testdata/gha_application.golden.yml"
	goldenCompose     = "testdata/gha_compose.golden.yml"
)

func TestRenderApplicationWorkflow_GoldenStable(t *testing.T) {
	got, err := RenderApplicationWorkflow(ApplicationWorkflowData{
		Branch:         "main",
		DockerfilePath: "Dockerfile",
		LaunchBaseURL:  "https://launchctl.io",
		AppID:          "01HJXVHGRGTQRX4P0G3Y8R6CK7",
	})
	require.NoError(t, err)
	assertGolden(t, goldenApplication, got)
}

func TestRenderComposeWorkflow_GoldenStable(t *testing.T) {
	got, err := RenderComposeWorkflow(ComposeWorkflowData{
		Branch:          "main",
		ComposeFilePath: "docker-compose.yml",
		LaunchBaseURL:   "https://launchctl.io",
		ComposeID:       "01HJXVHGRGTQRX4P0G3Y8R6CK8",
	})
	require.NoError(t, err)
	assertGolden(t, goldenCompose, got)
}

// TestRenderApplicationWorkflow_RespectsBuildType pins that an explicit
// build method is honoured in the Detect-builder step rather than
// auto-detected: "dockerfile" fails loudly if the Dockerfile is missing,
// "nixpacks" forces nixpacks, and "" (auto) falls back to detection.
func TestRenderApplicationWorkflow_RespectsBuildType(t *testing.T) {
	base := func(bt string) ApplicationWorkflowData {
		return ApplicationWorkflowData{
			Branch:         "main",
			DockerfilePath: "Dockerfile",
			BuildType:      bt,
			LaunchBaseURL:  "https://launchctl.io",
			AppID:          "01HJX",
		}
	}

	df, err := RenderApplicationWorkflow(base("dockerfile"))
	require.NoError(t, err)
	// Explicit dockerfile: guards the file and errors clearly if absent;
	// no silent nixpacks fallback in the case body.
	assert.Contains(t, df, `case "dockerfile" in`)
	assert.Contains(t, df, `if [ ! -f "Dockerfile" ]`)
	assert.Contains(t, df, "Build method is 'dockerfile' but Dockerfile was not found")

	np, err := RenderApplicationWorkflow(base("nixpacks"))
	require.NoError(t, err)
	assert.Contains(t, np, `case "nixpacks" in`)

	auto, err := RenderApplicationWorkflow(base(""))
	require.NoError(t, err)
	// Auto (empty): detect by Dockerfile presence in the fallback arm.
	assert.Contains(t, auto, `case "" in`)
	assert.Contains(t, auto, `if [ -f "Dockerfile" ]`)
}

// TestRenderApplicationWorkflow_AutoDeployTrigger pins the auto-deploy
// behaviour: ON adds an `on: push: branches: [<branch>]` trigger (so a push
// auto-deploys) while keeping workflow_dispatch; OFF is manual-only and emits
// NO push trigger (committing the workflow must never auto-deploy).
func TestRenderApplicationWorkflow_AutoDeployTrigger(t *testing.T) {
	base := func(auto bool) ApplicationWorkflowData {
		return ApplicationWorkflowData{
			Branch:         "release",
			DockerfilePath: "Dockerfile",
			LaunchBaseURL:  "https://launchctl.io",
			AppID:          "01HJX",
			AutoDeploy:     auto,
		}
	}

	on, err := RenderApplicationWorkflow(base(true))
	require.NoError(t, err)
	// The push *trigger* is identified by `branches: [<branch>]` — note the
	// build step always has `push: true`, so we key off `branches:` here.
	assert.Contains(t, on, "branches: [release]", "auto-deploy ON must add a push trigger scoped to the deploy branch")
	assert.Contains(t, on, "workflow_dispatch:", "manual dispatch must remain available")

	off, err := RenderApplicationWorkflow(base(false))
	require.NoError(t, err)
	assert.NotContains(t, off, "branches:", "auto-deploy OFF must NOT emit a push trigger")
	assert.Contains(t, off, "workflow_dispatch:", "manual dispatch is the only trigger when off")
}

func TestRenderComposeWorkflow_AutoDeployTrigger(t *testing.T) {
	on, err := RenderComposeWorkflow(ComposeWorkflowData{
		Branch: "main", ComposeFilePath: "docker-compose.yml",
		LaunchBaseURL: "https://launchctl.io", ComposeID: "01HJX", AutoDeploy: true,
	})
	require.NoError(t, err)
	assert.Contains(t, on, "push:")
	assert.Contains(t, on, "branches: [main]")
	assert.Contains(t, on, "workflow_dispatch:")
}

func TestRenderApplicationWorkflow_RejectsMissingFields(t *testing.T) {
	cases := []struct {
		name string
		data ApplicationWorkflowData
	}{
		{"no branch", ApplicationWorkflowData{DockerfilePath: "Dockerfile", LaunchBaseURL: "https://x", AppID: "A"}},
		{"no dockerfile", ApplicationWorkflowData{Branch: "main", LaunchBaseURL: "https://x", AppID: "A"}},
		{"no base url", ApplicationWorkflowData{Branch: "main", DockerfilePath: "Dockerfile", AppID: "A"}},
		{"no app id", ApplicationWorkflowData{Branch: "main", DockerfilePath: "Dockerfile", LaunchBaseURL: "https://x"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := RenderApplicationWorkflow(tc.data)
			require.Error(t, err, "should reject incomplete input")
			assert.Contains(t, err.Error(), "missing required field")
		})
	}
}

func TestRenderComposeWorkflow_RejectsMissingFields(t *testing.T) {
	_, err := RenderComposeWorkflow(ComposeWorkflowData{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required field")
}

// TestRenderApplicationWorkflow_StableOnRepeat protects against
// non-determinism creeping into the template (e.g. map iteration
// order if we ever pass a map in). Two consecutive renders with the
// same input must produce byte-identical output.
func TestRenderApplicationWorkflow_StableOnRepeat(t *testing.T) {
	data := ApplicationWorkflowData{
		Branch:         "main",
		DockerfilePath: "Dockerfile",
		LaunchBaseURL:  "https://launchctl.io",
		AppID:          "01HJXVHGRGTQRX4P0G3Y8R6CK7",
	}
	a, errA := RenderApplicationWorkflow(data)
	b, errB := RenderApplicationWorkflow(data)
	require.NoError(t, errA)
	require.NoError(t, errB)
	assert.Equal(t, a, b, "renderer must be deterministic")
}

// TestRenderedYAMLContainsExpectedAnchors is a sanity check that the
// rendered output references the customer-supplied values where we
// expect — defends against a future template edit that accidentally
// removes a {{.Field}} interpolation. A more thorough YAML-parsing
// check would be nice but we'd need a YAML library; substring is
// sufficient for "did the input land in the right places."
func TestRenderedYAMLContainsExpectedAnchors(t *testing.T) {
	got, err := RenderApplicationWorkflow(ApplicationWorkflowData{
		Branch:         "deploy-branch",
		DockerfilePath: "deploy/Dockerfile",
		LaunchBaseURL:  "https://my-launch.example",
		AppID:          "01TESTAPP",
	})
	require.NoError(t, err)

	// Deploys are manual-only: the workflow triggers on workflow_dispatch
	// and must NOT auto-deploy on push (committing/syncing the workflow
	// file, or pushing code, shouldn't kick off a deploy).
	assert.Contains(t, got, "workflow_dispatch:")
	assert.NotContains(t, got, "branches:")
	// DockerfilePath shows up in two places: the existence check + the
	// build-push-action input.
	assert.Contains(t, got, `if [ -f "deploy/Dockerfile" ]`)
	assert.Contains(t, got, `file: deploy/Dockerfile`)
	// The webhook URL is built at workflow-RUN time from the
	// LAUNCH_WEBHOOK_URL Actions variable (so customers can move
	// Launch without re-syncing the workflow file). The rendered
	// YAML carries the GHA-expression placeholder, NOT the literal
	// LaunchBaseURL value — but the bootstrap job still pushes
	// LaunchBaseURL into the variable, so the round-trip works.
	assert.Contains(t, got, "${{ vars.LAUNCH_WEBHOOK_URL }}/api/webhooks/docker/applications/01TESTAPP/deploy")
	assert.Contains(t, got, "${{ vars.LAUNCH_WEBHOOK_URL }}/api/webhooks/docker/applications/01TESTAPP/status")
	// LaunchBaseURL is intentionally NOT baked into the YAML body
	// — the variable indirection is the point. Locked in to catch a
	// future regression that re-bakes it.
	assert.NotContains(t, got, "https://my-launch.example")

	// Negative: there should be no Go-template residue. If our
	// {{ "{{" }} escaping is wrong, "<no value>" or "{{...}}" pieces
	// referencing our struct fields would leak.
	assert.NotContains(t, got, "<no value>")
	assert.NotContains(t, got, "{{.")
}

// TestRenderApplicationWorkflow_BuildSecretsRenderSecretsBlock exercises
// the optional `secrets:` block on docker/build-push-action that gets
// populated when BuildSecretNames is non-empty. Each name maps to a
// repo secret named LAUNCH_BUILD_<NAME> — the bootstrap job is
// responsible for pushing those alongside the workflow file. Empty
// list (covered by the golden test above) produces NO secrets block,
// keeping the no-secrets workflow clean.
func TestRenderApplicationWorkflow_BuildSecretsRenderSecretsBlock(t *testing.T) {
	got, err := RenderApplicationWorkflow(ApplicationWorkflowData{
		Branch:           "main",
		DockerfilePath:   "Dockerfile",
		LaunchBaseURL:    "https://launchctl.io",
		AppID:            "01HJX",
		BuildSecretNames: []string{"NPM_TOKEN", "GH_PAT"},
	})
	require.NoError(t, err)

	// secrets: block is present on the Dockerfile build step.
	assert.Contains(t, got, "secrets: |")
	// Each name maps to its LAUNCH_BUILD_<NAME> repo secret.
	assert.Contains(t, got, "NPM_TOKEN=${{ secrets.LAUNCH_BUILD_NPM_TOKEN }}")
	assert.Contains(t, got, "GH_PAT=${{ secrets.LAUNCH_BUILD_GH_PAT }}")
}

func TestRenderApplicationWorkflow_NoBuildSecretsOmitsBlock(t *testing.T) {
	// Symmetric assertion: with no BuildSecretNames, the YAML must
	// not contain a stray `secrets:` line. Otherwise GitHub Actions
	// would parse the empty multiline as "no secrets" but customers
	// would see a confusing dangling YAML key.
	got, err := RenderApplicationWorkflow(ApplicationWorkflowData{
		Branch:         "main",
		DockerfilePath: "Dockerfile",
		LaunchBaseURL:  "https://launchctl.io",
		AppID:          "01HJX",
	})
	require.NoError(t, err)
	assert.NotContains(t, got, "secrets: |")
	assert.NotContains(t, got, "LAUNCH_BUILD_")
}

func TestRenderComposeWorkflow_BuildSecretsRenderSecretsBlock(t *testing.T) {
	got, err := RenderComposeWorkflow(ComposeWorkflowData{
		Branch:           "main",
		ComposeFilePath:  "docker-compose.yml",
		LaunchBaseURL:    "https://launchctl.io",
		ComposeID:        "01HJX",
		BuildSecretNames: []string{"PIP_INDEX_URL"},
	})
	require.NoError(t, err)
	assert.Contains(t, got, "secrets: |")
	assert.Contains(t, got, "PIP_INDEX_URL=${{ secrets.LAUNCH_BUILD_PIP_INDEX_URL }}")
}

func assertGolden(t *testing.T, path, actual string) {
	t.Helper()
	if *updateGolden {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(actual), 0o644))
		t.Logf("golden updated: %s", path)
		return
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err, "golden file missing — run `go test -run Workflow -update`")
	if got := strings.ReplaceAll(actual, "\r\n", "\n"); got != string(want) {
		t.Fatalf("rendered output diverged from golden %s.\n--- want (first 500) ---\n%s\n--- got (first 500) ---\n%s",
			path, truncate(string(want), 500), truncate(got, 500))
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
