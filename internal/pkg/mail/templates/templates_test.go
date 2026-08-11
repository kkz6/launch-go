package templates

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmailBuilder_Build(t *testing.T) {
	// Initialize config for tests
	Initialize("Launch", "https://launch.io")

	builder := NewEmail().
		WithGreeting("Test Greeting").
		WithIntro("This is an intro paragraph.").
		WithAction("Click Me", "https://example.com", "primary").
		WithPanel("**Some panel content** with `code`.").
		WithOutro("This is an outro.").
		WithSubcopy("Fallback text here.")

	html, err := builder.Build()
	require.NoError(t, err)

	// Check that HTML contains expected elements
	assert.Contains(t, html, "Test Greeting")
	assert.Contains(t, html, "This is an intro paragraph.")
	assert.Contains(t, html, "Click Me")
	assert.Contains(t, html, "https://example.com")
	assert.Contains(t, html, "button-primary")
	assert.Contains(t, html, "panel")
	assert.Contains(t, html, "This is an outro.")
	assert.Contains(t, html, "Fallback text here.")
	assert.Contains(t, html, "<!DOCTYPE html")
	assert.Contains(t, html, "Launch")
}

func TestEmailBuilder_BuildPlainText(t *testing.T) {
	Initialize("Launch", "https://launch.io")

	builder := NewEmail().
		WithGreeting("Hello").
		WithIntro("This is a test.").
		WithAction("View", "https://example.com", "primary").
		WithOutro("Thanks!")

	text := builder.BuildPlainText()

	assert.Contains(t, text, "Hello")
	assert.Contains(t, text, "This is a test.")
	assert.Contains(t, text, "View: https://example.com")
	assert.Contains(t, text, "Thanks!")
}

func TestTeamDeletedEmail(t *testing.T) {
	html, plain, err := TeamDeletedEmail("Legacy Team", "Personal Team")
	require.NoError(t, err)
	require.Contains(t, html, "Legacy Team")
	require.Contains(t, html, "Personal Team")
	require.Contains(t, plain, "Legacy Team")
	require.Contains(t, plain, "Personal Team")
}

func TestTeamDeletedEmailReturnsRenderErrors(t *testing.T) {
	wanted := errors.New("render failed")
	originalRenderer := renderTeamDeletedEmail
	renderTeamDeletedEmail = func(*EmailBuilder) (string, error) {
		return "", wanted
	}
	t.Cleanup(func() { renderTeamDeletedEmail = originalRenderer })

	html, plain, err := TeamDeletedEmail("Legacy Team", "Personal Team")
	require.Empty(t, html)
	require.Empty(t, plain)
	require.ErrorIs(t, err, wanted)
}

func TestServerProvisionedEmail(t *testing.T) {
	Initialize("Launch", "https://launch.io")

	html, plainText, err := ServerProvisionedEmail(
		"production-server",
		"192.168.1.100",
		"launch",
		"https://launch.io/servers/123",
		"secret-password",
	)

	require.NoError(t, err)
	assert.NotEmpty(t, html)
	assert.NotEmpty(t, plainText)

	// HTML checks
	assert.Contains(t, html, "production-server")
	assert.Contains(t, html, "192.168.1.100")
	assert.Contains(t, html, "launch")
	assert.Contains(t, html, "secret-password")
	assert.Contains(t, html, "https://launch.io/servers/123")

	// Plain text checks
	assert.Contains(t, plainText, "production-server")
	assert.Contains(t, plainText, "192.168.1.100")
}

func TestDeploymentFailedEmail(t *testing.T) {
	Initialize("Launch", "https://launch.io")
	deploymentTime := time.Date(2026, 8, 7, 4, 37, 18, 0, time.UTC)
	output := "\x1b[32m✅ Repository cloned\x1b[0m\n" +
		"/home/launcher/.launch/task.sh: line 102: submodule: command not found\n" +
		"🧹 Cleaned up deployment credentials\n"

	html, plainText, err := DeploymentFailedEmail(
		"myapp.com",
		"production-server",
		"Failed",
		"abc123def456",
		"<img src=x onerror=alert(1)>\nThis commit body stays out of the summary",
		"John Doe",
		"jane@example.com",
		deploymentTime,
		output+"<script>alert('log')</script>",
		"https://launch.io/servers/123/sites/456?tab=deployments&deployment=789",
	)

	require.NoError(t, err)
	assert.NotEmpty(t, html)
	assert.NotEmpty(t, plainText)

	// Branded, ordered incident content.
	assert.Contains(t, html, ">launchctl</a>")
	assert.Contains(t, html, "Deployment failed")
	assert.Contains(t, html, "myapp.com")
	assert.Contains(t, html, "production-server")
	assert.Contains(t, html, "abc123d")
	assert.Contains(t, html, "John Doe")
	assert.Contains(t, html, "07 Aug 2026, 04:37 UTC")
	assert.Contains(t, html, "lctl / deploy")
	assert.Contains(t, html, "Last actionable error")
	assert.Contains(t, html, "submodule: command not found")
	assert.Contains(t, html, `class="deployment-trace"`)
	assert.Contains(t, html, `trace-error`)
	assert.Contains(t, html, "Open deployment")
	assert.NotContains(t, html, ">Deployment alert<")
	assert.NotContains(t, html, "Failure reason")
	assert.Less(t, strings.Index(html, "Run context"), strings.Index(html, "Open deployment"))
	assert.Less(t, strings.Index(html, "Open deployment"), strings.Index(html, "Run output"))

	// Repository-controlled metadata and output stay text, not markup.
	assert.NotContains(t, html, "<img src=x")
	assert.Contains(t, html, "&lt;img src=x onerror=alert(1)&gt;")
	assert.NotContains(t, html, "<script>alert")
	assert.Contains(t, html, "&lt;script&gt;alert(&#39;log&#39;)&lt;/script&gt;")
	assert.NotContains(t, html, "This commit body stays out of the summary")
	assert.NotContains(t, html, "\x1b[")

	assert.Contains(t, plainText, "Last actionable error:")
	assert.Contains(t, plainText, "Open deployment: https://launch.io/servers/123/sites/456?tab=deployments&deployment=789")
}

func TestDeploymentFailureSummary_PrefersActionableErrorOverCleanup(t *testing.T) {
	output := strings.Join([]string{
		"✅ Repository updated successfully",
		"Running hook after updating repository",
		"/home/launcher/.launch/task.sh: line 102: submodule: command not found",
		"🧹 Cleaned up deployment credentials",
	}, "\n")

	assert.Equal(
		t,
		"/home/launcher/.launch/task.sh: line 102: submodule: command not found",
		deploymentFailureSummary(output),
	)
}

func TestCompactDeploymentFailureSummary_RemovesShellTaskPrefix(t *testing.T) {
	assert.Equal(
		t,
		"submodule: command not found",
		compactDeploymentFailureSummary("/home/launcher/.launch/task.sh: line 102: submodule: command not found"),
	)
}

func TestDeploymentOutputLines_MarksAndWrapsFailures(t *testing.T) {
	output := strings.Repeat("x", 72) + ": command not found"
	lines := deploymentOutputLines(output)

	require.Len(t, lines, 2)
	assert.Equal(t, "01", lines[0].Number)
	assert.Equal(t, "✗", lines[0].Marker)
	assert.True(t, lines[0].IsFailure)
	assert.True(t, lines[1].Continuation)
	assert.Empty(t, lines[1].Number)
}

func TestDeploymentFailedEmail_UsesDashboardFallback(t *testing.T) {
	Initialize("Launch", "https://launch.io")

	html, plainText, err := DeploymentFailedEmail(
		"myapp.com",
		"",
		"Timed Out",
		"",
		"",
		"",
		"",
		time.Date(2026, 8, 7, 4, 37, 18, 0, time.UTC),
		"Deployment exceeded its time limit",
		"",
	)

	require.NoError(t, err)
	assert.Contains(t, html, "Deployment timed out")
	assert.NotContains(t, html, ">Server</td>")
	assert.Contains(t, html, `href="https://launch.io"`)
	assert.Contains(t, html, "Open Launch")
	assert.Contains(t, plainText, "Open Launch: https://launch.io")
}

func TestGenericFailureEmail(t *testing.T) {
	Initialize("Launch", "https://launch.io")

	html, plainText, err := GenericFailureEmail(
		"Server Error",
		"Something went wrong on the server.",
		"Last output line here",
		"Error: Process killed",
		"https://launch.io/servers/123",
		"View Server",
	)

	require.NoError(t, err)
	assert.NotEmpty(t, html)
	assert.NotEmpty(t, plainText)

	assert.Contains(t, html, "Server Error")
	assert.Contains(t, html, "Something went wrong")
	assert.Contains(t, html, "Last output line")
	assert.Contains(t, html, "Error: Process killed")
}

func TestRenderMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string
	}{
		{
			name:     "bold text",
			input:    "**bold**",
			contains: []string{"<strong>bold</strong>"},
		},
		{
			name:     "code",
			input:    "`code`",
			contains: []string{"<code>code</code>"},
		},
		{
			name:     "list",
			input:    "- item 1\n- item 2",
			contains: []string{"<li>item 1</li>", "<li>item 2</li>"},
		},
		{
			name:     "empty",
			input:    "",
			contains: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderMarkdown(tt.input)
			for _, c := range tt.contains {
				assert.Contains(t, result, c)
			}
		})
	}
}

func TestNewEmail_UsesGlobalConfig(t *testing.T) {
	Initialize("MyApp", "https://myapp.com")

	builder := NewEmail().WithGreeting("Hi")
	html, err := builder.Build()

	require.NoError(t, err)
	assert.Contains(t, html, "MyApp")
	assert.Contains(t, html, "https://myapp.com")
}

func TestConnectionTestEmail(t *testing.T) {
	Initialize("Launch", "https://launch.io")

	html, plainText, err := ConnectionTestEmail()

	require.NoError(t, err)
	assert.NotEmpty(t, html)
	assert.NotEmpty(t, plainText)

	assert.Contains(t, html, "Email Connection Successful")
	assert.Contains(t, strings.ToLower(plainText), "connection")
}
