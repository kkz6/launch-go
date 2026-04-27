package tasks_test

import (
	"testing"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/tests/testutil"
)

func TestProvisionFreshServer_BasicConfig(t *testing.T) {
	config := tasks.ProvisionFreshServerConfig{
		MemoryInMB:       2048,
		PublicIPv4:       "192.168.1.100",
		Provider:         "custom",
		PublicKey:        "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ... test@example.com",
		Username:         "launcher",
		Password:         "securepassword123",
		WorkingDirectory: ".launch",
		SSHKeys:          []string{"ssh-rsa KEY1...", "ssh-rsa KEY2..."},
		SSHPort:          22,
		SoftwareStack:    []types.Software{types.SoftwareCaddy2},
		DatabasePassword: "dbpassword123",
		DatabaseName:     "launch",
	}

	task := tasks.ProvisionFreshServer(config)

	if task == nil {
		t.Fatal("Expected task to be created")
	}

	script := task.Script()
	if script == "" {
		t.Fatal("Expected task script to be non-empty")
	}

	// Verify script contains expected sections
	expectedSections := []string{
		"#!/bin/bash",
		"Provision Step:",
		"set -euo pipefail",
	}

	for _, section := range expectedSections {
		if !containsString(script, section) {
			t.Errorf("Expected script to contain %q", section)
		}
	}

	// Match against snapshot
	testutil.MatchSnapshot(t, "tasks/provision_basic", script)
}

func TestProvisionFreshServer_WithMySQLAndPHP(t *testing.T) {
	config := tasks.ProvisionFreshServerConfig{
		MemoryInMB:       4096,
		PublicIPv4:       "10.0.0.1",
		Provider:         "digitalocean",
		PublicKey:        "ssh-rsa TESTKEY... user@host",
		Username:         "launcher",
		Password:         "password123",
		WorkingDirectory: ".launch",
		SSHPort:          22,
		SoftwareStack: []types.Software{
			types.SoftwareCaddy2,
			types.SoftwarePhp83,
			types.SoftwareComposer2,
			types.SoftwareMySQL80,
		},
		DatabasePassword: "mysqlpassword",
		DatabaseName:     "app_db",
	}

	task := tasks.ProvisionFreshServer(config)

	script := task.Script()

	// Verify PHP and MySQL are included
	if !containsString(script, "php") {
		t.Error("Expected script to contain PHP installation")
	}

	if !containsString(script, "mysql") || !containsString(script, "MySQL") {
		t.Error("Expected script to contain MySQL installation")
	}

	testutil.MatchSnapshot(t, "tasks/provision_mysql_php", script)
}

func TestProvisionFreshServer_WithPostgreSQL(t *testing.T) {
	config := tasks.ProvisionFreshServerConfig{
		MemoryInMB:       2048,
		PublicIPv4:       "10.0.0.2",
		Provider:         "aws",
		PublicKey:        "ssh-rsa TESTKEY... user@host",
		Username:         "launcher",
		Password:         "password123",
		WorkingDirectory: ".launch",
		SSHPort:          2222,
		SoftwareStack: []types.Software{
			types.SoftwareCaddy2,
			types.SoftwarePhp83,
			types.SoftwarePostgreSQL16,
		},
		DatabasePassword: "pgpassword",
		DatabaseName:     "app_db",
	}

	task := tasks.ProvisionFreshServer(config)

	script := task.Script()

	// Verify PostgreSQL is included
	if !containsString(script, "postgresql") || !containsString(script, "PostgreSQL") {
		t.Error("Expected script to contain PostgreSQL installation")
	}

	testutil.MatchSnapshot(t, "tasks/provision_postgresql", script)
}

func TestProvisionFreshServer_DockerType(t *testing.T) {
	config := tasks.ProvisionFreshServerConfig{
		MemoryInMB:       2048,
		PublicIPv4:       "203.0.113.10",
		Provider:         "custom",
		PublicKey:        "ssh-rsa DOCKERKEY... user@host",
		Username:         "launcher",
		Password:         "dockerpassword",
		WorkingDirectory: ".launch",
		SSHPort:          22,
		SoftwareStack: []types.Software{
			types.SoftwareDocker,
			types.SoftwareTraefik,
			types.SoftwareLaunchAgent,
		},
	}

	task := tasks.ProvisionFreshServer(config)
	if task == nil {
		t.Fatal("Expected task to be created")
	}
	script := task.Script()

	// Both install steps must show up in the combined provisioning script.
	expectedSnippets := []string{
		"Install Docker Engine + Compose plugin",
		"containerd.io",
		"docker-compose-plugin",
		"usermod -aG docker launcher",
		"docker network inspect launch-network",
		"Configure Traefik",
		"--name launch-traefik",
		"traefik:v3.6",
	}
	for _, snippet := range expectedSnippets {
		if !containsString(script, snippet) {
			t.Errorf("docker provisioning script missing %q", snippet)
		}
	}

	// Without TraefikAdminEmail, the ACME resolver must NOT be present.
	if containsString(script, "certificatesResolvers") {
		t.Error("docker provisioning without TraefikAdminEmail must omit ACME resolver")
	}

	testutil.MatchSnapshot(t, "tasks/provision_docker_server", script)
}

func containsString(haystack, needle string) bool {
	return len(haystack) > 0 && len(needle) > 0 &&
		(haystack == needle || len(haystack) > len(needle) &&
			(haystack[:len(needle)] == needle ||
				haystack[len(haystack)-len(needle):] == needle ||
				findSubstring(haystack, needle)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
