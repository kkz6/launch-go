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

	// Branch is referenced in the on.push.branches list AND nowhere
	// else — assert presence in the expected literal.
	assert.Contains(t, got, `branches: ["deploy-branch"]`)
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
