package tasks

import (
	"testing"

	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// buildSoftwareInstallData Tests
// =============================================================================

func testConfig() ProvisionFreshServerConfig {
	return ProvisionFreshServerConfig{
		ServerID:         "server-123",
		TeamID:           "team-123",
		ServerName:       "Test Server",
		MemoryInMB:       2048,
		PublicIPv4:       "192.168.1.100",
		Provider:         "digitalocean",
		PublicKey:        "ssh-rsa AAAA...",
		Username:         "launch",
		Password:         "secret-password",
		WorkingDirectory: "/home/launch",
		AppURL:           "https://app.example.com",
		AppName:          "launch",
		SSHPort:          22,
		DatabasePassword: "db-password",
		DatabaseName:     "launch_db",
		AgentConfigPath:  "/etc/launch-agent/config.yaml",
		AgentURL:         "https://app.example.com/agent",
	}
}

func TestBuildSoftwareInstallData_Caddy2(t *testing.T) {
	config := testConfig()
	data := buildSoftwareInstallData(types.SoftwareCaddy2, config)

	require.NotNil(t, data)

	d := data.(struct {
		Username   string
		PublicIPv4 string
	})
	assert.Equal(t, config.Username, d.Username)
	assert.Equal(t, config.PublicIPv4, d.PublicIPv4)
}

func TestBuildSoftwareInstallData_Caddy2LB(t *testing.T) {
	config := testConfig()
	data := buildSoftwareInstallData(types.SoftwareCaddy2LB, config)

	require.NotNil(t, data, "SoftwareCaddy2LB must return non-nil data (needs Username and PublicIPv4)")

	d := data.(struct {
		Username   string
		PublicIPv4 string
	})
	assert.Equal(t, config.Username, d.Username)
	assert.Equal(t, config.PublicIPv4, d.PublicIPv4)
}

func TestBuildSoftwareInstallData_MySQL80(t *testing.T) {
	config := testConfig()
	data := buildSoftwareInstallData(types.SoftwareMySQL80, config)

	require.NotNil(t, data)

	d := data.(struct {
		RootPassword   string
		DatabaseName   string
		PublicIPv4     string
		MaxConnections int
	})
	assert.Equal(t, config.DatabasePassword, d.RootPassword)
	assert.Equal(t, config.DatabaseName, d.DatabaseName)
	assert.Equal(t, config.PublicIPv4, d.PublicIPv4)
	assert.Greater(t, d.MaxConnections, 0)
}

func TestBuildSoftwareInstallData_PostgreSQL16(t *testing.T) {
	config := testConfig()
	data := buildSoftwareInstallData(types.SoftwarePostgreSQL16, config)

	require.NotNil(t, data)

	d := data.(struct {
		DatabasePassword string
		DatabaseName     string
		MaxConnections   int
	})
	assert.Equal(t, config.DatabasePassword, d.DatabasePassword)
	assert.Equal(t, config.DatabaseName, d.DatabaseName)
	assert.Greater(t, d.MaxConnections, 0)
}

func TestBuildSoftwareInstallData_Composer2(t *testing.T) {
	config := testConfig()
	data := buildSoftwareInstallData(types.SoftwareComposer2, config)

	require.NotNil(t, data)

	d := data.(struct{ Username string })
	assert.Equal(t, config.Username, d.Username)
}

func TestBuildSoftwareInstallData_LaunchAgent(t *testing.T) {
	config := testConfig()
	data := buildSoftwareInstallData(types.SoftwareLaunchAgent, config)

	require.NotNil(t, data)

	d := data.(struct {
		AgentConfigPath string
		AgentURL        string
		RootUsername    string
	})
	assert.Equal(t, config.AgentConfigPath, d.AgentConfigPath)
	assert.Equal(t, config.AgentURL, d.AgentURL)
	assert.Equal(t, "root", d.RootUsername)
}

func TestBuildSoftwareInstallData_PHP(t *testing.T) {
	config := testConfig()

	phpVersions := []types.Software{
		types.SoftwarePhp80,
		types.SoftwarePhp81,
		types.SoftwarePhp82,
		types.SoftwarePhp83,
		types.SoftwarePhp84,
	}

	for _, php := range phpVersions {
		t.Run(string(php), func(t *testing.T) {
			data := buildSoftwareInstallData(php, config)
			require.NotNil(t, data, "%s must return non-nil data", php)

			d := data.(struct {
				Version     string
				Username    string
				MaxChildren int
			})
			assert.NotEmpty(t, d.Version)
			assert.Equal(t, config.Username, d.Username)
			assert.Greater(t, d.MaxChildren, 0)
		})
	}
}

func TestBuildSoftwareInstallData_StaticTemplates(t *testing.T) {
	config := testConfig()

	staticSoftware := []types.Software{
		types.SoftwareNode21,
		types.SoftwareBun,
		types.SoftwareRedis,
		types.SoftwareSupervisor,
	}

	for _, sw := range staticSoftware {
		t.Run(string(sw), func(t *testing.T) {
			data := buildSoftwareInstallData(sw, config)
			assert.Nil(t, data, "%s should return nil (template has no variables)", sw)
		})
	}
}

// =============================================================================
// Template Rendering Integration Tests
// =============================================================================

func TestRenderSoftwareInstall_AllSoftware(t *testing.T) {
	config := testConfig()

	allSoftware := []types.Software{
		types.SoftwareCaddy2,
		types.SoftwareCaddy2LB,
		types.SoftwareMySQL80,
		types.SoftwarePostgreSQL16,
		types.SoftwareComposer2,
		types.SoftwareLaunchAgent,
		types.SoftwareNode21,
		types.SoftwareBun,
		types.SoftwareRedis,
		types.SoftwareSupervisor,
		types.SoftwarePhp82,
		types.SoftwarePhp83,
		types.SoftwarePhp84,
	}

	for _, sw := range allSoftware {
		t.Run(string(sw), func(t *testing.T) {
			assert.NotPanics(t, func() {
				script := renderSoftwareInstall(sw, config)
				assert.NotEmpty(t, script, "%s template should produce non-empty output", sw)
				assert.Contains(t, script, "#!/bin/bash", "%s template should start with bash shebang", sw)
			}, "%s template rendering should not panic", sw)
		})
	}
}

func TestRenderSoftwareInstall_Caddy2LB_ContainsExpectedValues(t *testing.T) {
	config := testConfig()
	script := renderSoftwareInstall(types.SoftwareCaddy2LB, config)

	assert.Contains(t, script, config.PublicIPv4, "LB Caddy script should contain the server IP")
	assert.Contains(t, script, config.Username, "LB Caddy script should contain the username")
	assert.NotContains(t, script, "{{ .PublicIPv4 }}", "template variables should be rendered")
	assert.NotContains(t, script, "{{ .Username }}", "template variables should be rendered")
}

func TestRenderSoftwareInstall_Caddy2_ContainsExpectedValues(t *testing.T) {
	config := testConfig()
	script := renderSoftwareInstall(types.SoftwareCaddy2, config)

	assert.Contains(t, script, config.PublicIPv4, "Caddy script should contain the server IP")
	assert.Contains(t, script, config.Username, "Caddy script should contain the username")
}

// =============================================================================
// calculateSwapInMegabytes Tests
// =============================================================================

func TestCalculateSwapInMegabytes(t *testing.T) {
	tests := []struct {
		name       string
		memoryInMB int
		expected   int
	}{
		{"512MB server", 512, 1024},
		{"1GB server", 1024, 1024},
		{"2GB server", 2048, 1024},
		{"4GB server", 4096, 2048},
		{"8GB server", 8192, 3072},
		{"16GB server", 16384, 4096},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateSwapInMegabytes(tt.memoryInMB)
			assert.Equal(t, tt.expected, result)
		})
	}
}
