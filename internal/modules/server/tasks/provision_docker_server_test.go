package tasks

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testDockerConfig() ProvisionDockerServerConfig {
	return ProvisionDockerServerConfig{
		ServerID:         "server-docker-123",
		TeamID:           "team-123",
		ServerName:       "Docker Test Server",
		MemoryInMB:       2048,
		PublicIPv4:       "192.168.1.200",
		Provider:         "digitalocean",
		PublicKey:        "ssh-rsa AAAA...",
		Username:         "launch",
		Password:         "secret-password",
		WorkingDirectory: "/home/launch",
		AppURL:           "https://app.example.com",
		AppName:          "launch",
		SSHPort:          22,
		NetworkName:      DockerNetworkName,
		RootDir:          DockerRootDir,
		TraefikVersion:   TraefikVersion,
		TraefikService:   TraefikServiceName,
		ACMEEmail:        "ops@example.com",
	}
}

// =============================================================================
// ProvisionDockerServer task assembly
// =============================================================================

func TestProvisionDockerServer_TaskMetadata(t *testing.T) {
	task := ProvisionDockerServer(testDockerConfig())
	require.NotNil(t, task)

	assert.Equal(t, ProvisionDockerServerTaskType, task.TypeName(), "TypeName must match the registered constant")
	assert.NotEmpty(t, task.Script(), "script must be non-empty")
	assert.Contains(t, task.Script(), "#!/bin/bash", "script must start with a bash shebang")
}

func TestProvisionDockerServer_AppliesDefaults(t *testing.T) {
	config := testDockerConfig()
	// Blank out the docker-specific knobs and verify the constructor fills them
	// with package constants.
	config.NetworkName = ""
	config.RootDir = ""
	config.TraefikVersion = ""
	config.TraefikService = ""

	task := ProvisionDockerServer(config)
	script := task.Script()

	assert.Contains(t, script, DockerNetworkName, "default network name must be applied")
	assert.Contains(t, script, DockerRootDir, "default root dir must be applied")
	assert.Contains(t, script, TraefikVersion, "default traefik version must be applied")
	assert.Contains(t, script, TraefikServiceName, "default traefik service name must be applied")
}

func TestProvisionDockerServer_CallbackPayload(t *testing.T) {
	config := testDockerConfig()
	task := ProvisionDockerServer(config)

	payload, err := task.MarshalPayload()
	require.NoError(t, err)

	body := string(payload)
	assert.Contains(t, body, config.ServerID)
	assert.Contains(t, body, config.TeamID)
	assert.Contains(t, body, config.ServerName)
	assert.Contains(t, body, config.PublicIPv4)
	assert.Contains(t, body, config.Username)
}

// =============================================================================
// Rendered script contents
// =============================================================================

func TestProvisionDockerServer_ScriptContainsAllSteps(t *testing.T) {
	script := ProvisionDockerServer(testDockerConfig()).Script()

	// Step descriptions emitted as "::LAUNCH::status::..." markers — picking a
	// stable substring from each guarantees the step actually rendered.
	expectedSubstrings := []string{
		"Configure a swap file",
		"Configure the firewall",
		"Update package lists",
		"Install essential packages",
		"Configure unattended upgrades",
		"Configure the root user",
		"Enhance SSH security",
		"Create a default user",
		"Verify ports 80 and 443",
		"Install Docker CE",
		"Initialize Docker Swarm",
		"Create the /etc/launch directory tree",
		"Deploy Traefik",
	}
	for _, want := range expectedSubstrings {
		assert.Contains(t, script, want, "script missing step: %q", want)
	}
}

func TestProvisionDockerServer_ScriptContainsDockerCommands(t *testing.T) {
	script := ProvisionDockerServer(testDockerConfig()).Script()

	// Docker install marker
	assert.Contains(t, script, "docker-ce", "must install docker-ce")
	assert.Contains(t, script, "docker-compose-plugin", "must install compose plugin")

	// Swarm + network creation
	assert.Contains(t, script, "docker swarm init", "must init Swarm")
	assert.Contains(t, script, "docker network create --driver overlay --attachable launch-network",
		"must create overlay network")

	// Traefik
	assert.Contains(t, script, "traefik:"+TraefikVersion, "must reference pinned Traefik image")
	assert.Contains(t, script, "docker service create", "must deploy Traefik as Swarm service")
	assert.Contains(t, script, "--name "+TraefikServiceName, "must use configured Traefik service name")
	assert.Contains(t, script, "/etc/launch/traefik/traefik.yml", "must write traefik.yml under RootDir")
}

func TestProvisionDockerServer_ScriptIncludesACMEWhenEmailProvided(t *testing.T) {
	config := testDockerConfig()
	config.ACMEEmail = "ops@example.com"
	script := ProvisionDockerServer(config).Script()

	assert.Contains(t, script, "ops@example.com", "ACME email must appear in script")
	assert.Contains(t, script, "certificatesResolvers:", "ACME resolver block must be emitted")
}

func TestProvisionDockerServer_DockerTemplateEscaping(t *testing.T) {
	// Docker's {{ .Name }} format strings collide with Go's template engine. The
	// template uses {{`{{`}}.Name{{`}}`}} to emit literal braces. If the escape
	// breaks, the rendered script will contain leftover backticks or a
	// pre-substituted value — both make the docker CLI fail.
	script := ProvisionDockerServer(testDockerConfig()).Script()

	assert.Contains(t, script, "--format '{{.Name}}'",
		"docker --format string must be emitted literally (template escape failure)")
	assert.False(t, strings.Contains(script, "`{{"),
		"raw escape backticks must not appear in rendered script")
}

func TestProvisionDockerServer_ProgressMarkers(t *testing.T) {
	script := ProvisionDockerServer(testDockerConfig()).Script()

	assert.Contains(t, script, "::LAUNCH::progress::100", "script must end at 100% progress")
	assert.Contains(t, script, "::LAUNCH::step_completed::install_docker", "must emit install_docker completion marker")
	assert.Contains(t, script, "::LAUNCH::step_completed::install_traefik", "must emit install_traefik completion marker")
}
