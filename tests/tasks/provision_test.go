package tasks_test

import (
	"testing"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
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
		SoftwareStack:    []enums.Software{enums.SoftwareCaddy2},
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
		SoftwareStack: []enums.Software{
			enums.SoftwareCaddy2,
			enums.SoftwarePhp83,
			enums.SoftwareComposer2,
			enums.SoftwareMySql80,
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
		SoftwareStack: []enums.Software{
			enums.SoftwareCaddy2,
			enums.SoftwarePhp83,
			enums.SoftwarePostgreSql16,
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
