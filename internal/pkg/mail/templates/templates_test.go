package templates

import (
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

	html, plainText, err := DeploymentFailedEmail(
		"myapp.com",
		"production-server",
		"Failed",
		"abc123def456",
		"Fix critical bug",
		"John Doe",
		"jane@example.com",
		time.Now(),
		"Error: Connection refused",
		"https://launch.io/sites/123",
	)

	require.NoError(t, err)
	assert.NotEmpty(t, html)
	assert.NotEmpty(t, plainText)

	// HTML checks
	assert.Contains(t, html, "myapp.com")
	assert.Contains(t, html, "production-server")
	assert.Contains(t, html, "abc123d") // Short hash
	assert.Contains(t, html, "Fix critical bug")
	assert.Contains(t, html, "John Doe")
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
