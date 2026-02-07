package tasks

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner/templates"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	templates.MustRegisterAll()
	os.Exit(m.Run())
}

// =============================================================================
// UpdateUpstreamCaddyfile Tests
// =============================================================================

func TestLBUpdateUpstreamCaddyfile_ValidConfig(t *testing.T) {
	config := UpdateUpstreamCaddyfileConfig{
		UpstreamID:       "upstream-abc123",
		CaddyfilePath:    "/etc/caddy/upstreams/upstream-abc123.caddy",
		CaddyfileContent: "reverse_proxy localhost:8080",
	}

	task := UpdateUpstreamCaddyfile(config)
	require.NotNil(t, task)

	assert.Equal(t, "Update Upstream Caddyfile", task.Name())
	assert.Equal(t, 60*time.Second, task.Timeout())

	script := task.Script()
	assert.Contains(t, script, "/etc/caddy/upstreams/upstream-abc123.caddy")
	assert.Contains(t, script, "reverse_proxy localhost:8080")
}

func TestLBUpdateUpstreamCaddyfile_ScriptContainsCaddyValidate(t *testing.T) {
	config := UpdateUpstreamCaddyfileConfig{
		UpstreamID:       "upstream-validate",
		CaddyfilePath:    "/etc/caddy/upstreams/upstream-validate.caddy",
		CaddyfileContent: "reverse_proxy localhost:3000",
	}

	task := UpdateUpstreamCaddyfile(config)
	require.NotNil(t, task)

	script := task.Script()
	assert.Contains(t, script, "caddy validate")
}

func TestLBUpdateUpstreamCaddyfile_ScriptContainsCaddyReload(t *testing.T) {
	config := UpdateUpstreamCaddyfileConfig{
		UpstreamID:       "upstream-reload",
		CaddyfilePath:    "/etc/caddy/upstreams/upstream-reload.caddy",
		CaddyfileContent: "reverse_proxy localhost:9090",
	}

	task := UpdateUpstreamCaddyfile(config)
	require.NotNil(t, task)

	script := task.Script()
	assert.True(t,
		strings.Contains(script, "caddy reload") || strings.Contains(script, "service caddy reload"),
		"expected script to contain 'caddy reload' or 'service caddy reload', got:\n%s", script,
	)
}

// =============================================================================
// UpdateUpstreamsImports Tests
// =============================================================================

func TestLBUpdateUpstreamsImports_MultipleUpstreams(t *testing.T) {
	config := UpdateUpstreamsImportsConfig{
		Upstreams: []UpstreamImport{
			{ID: "upstream-001"},
			{ID: "upstream-002"},
			{ID: "upstream-003"},
		},
	}

	task := UpdateUpstreamsImports(config)
	require.NotNil(t, task)

	assert.Equal(t, "Update Upstreams Imports", task.Name())
	assert.Equal(t, 30*time.Second, task.Timeout())

	script := task.Script()
	assert.Contains(t, script, "import /etc/caddy/upstreams/upstream-001.caddy")
	assert.Contains(t, script, "import /etc/caddy/upstreams/upstream-002.caddy")
	assert.Contains(t, script, "import /etc/caddy/upstreams/upstream-003.caddy")
}

func TestLBUpdateUpstreamsImports_EmptyUpstreams(t *testing.T) {
	config := UpdateUpstreamsImportsConfig{
		Upstreams: []UpstreamImport{},
	}

	task := UpdateUpstreamsImports(config)
	require.NotNil(t, task)

	assert.Equal(t, "Update Upstreams Imports", task.Name())
	assert.Equal(t, 30*time.Second, task.Timeout())

	script := task.Script()
	assert.NotEmpty(t, script)
	assert.NotContains(t, script, "import /etc/caddy/upstreams/")
}

func TestLBUpdateUpstreamsImports_ScriptContainsUpstreamPaths(t *testing.T) {
	config := UpdateUpstreamsImportsConfig{
		Upstreams: []UpstreamImport{
			{ID: "test-upstream"},
		},
	}

	task := UpdateUpstreamsImports(config)
	require.NotNil(t, task)

	script := task.Script()
	assert.Contains(t, script, "/etc/caddy/upstreams/")
	assert.Contains(t, script, "service caddy reload")
}

// =============================================================================
// RemoveUpstreamCaddyfile Tests
// =============================================================================

func TestLBRemoveUpstreamCaddyfile_ScriptContainsCaddyfilePath(t *testing.T) {
	config := RemoveUpstreamCaddyfileConfig{
		UpstreamID:    "upstream-remove",
		CaddyfilePath: "/etc/caddy/upstreams/upstream-remove.caddy",
	}

	task := RemoveUpstreamCaddyfile(config)
	require.NotNil(t, task)

	assert.Equal(t, "Remove Upstream Caddyfile", task.Name())
	assert.Equal(t, 30*time.Second, task.Timeout())

	script := task.Script()
	assert.Contains(t, script, "/etc/caddy/upstreams/upstream-remove.caddy")
}

func TestLBRemoveUpstreamCaddyfile_ScriptContainsRmCommand(t *testing.T) {
	config := RemoveUpstreamCaddyfileConfig{
		UpstreamID:    "upstream-del",
		CaddyfilePath: "/etc/caddy/upstreams/upstream-del.caddy",
	}

	task := RemoveUpstreamCaddyfile(config)
	require.NotNil(t, task)

	script := task.Script()
	assert.Contains(t, script, "rm -f")
	assert.Contains(t, script, "/etc/caddy/upstreams/upstream-del.caddy")
}

func TestLBRemoveUpstreamCaddyfile_ScriptContainsCaddyReload(t *testing.T) {
	config := RemoveUpstreamCaddyfileConfig{
		UpstreamID:    "upstream-reload",
		CaddyfilePath: "/etc/caddy/upstreams/upstream-reload.caddy",
	}

	task := RemoveUpstreamCaddyfile(config)
	require.NotNil(t, task)

	script := task.Script()
	assert.Contains(t, script, "service caddy reload")
}

func TestLBRemoveUpstreamCaddyfile_ScriptContainsBashHeader(t *testing.T) {
	config := RemoveUpstreamCaddyfileConfig{
		UpstreamID:    "upstream-bash",
		CaddyfilePath: "/etc/caddy/upstreams/upstream-bash.caddy",
	}

	task := RemoveUpstreamCaddyfile(config)
	require.NotNil(t, task)

	script := task.Script()
	assert.True(t, strings.HasPrefix(script, "#!/bin/bash"), "expected script to start with #!/bin/bash")
}
