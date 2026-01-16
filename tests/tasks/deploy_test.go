package tasks_test

import (
	"strings"
	"testing"

	"github.com/kkz6/launch-go/internal/modules/site/tasks"
	"github.com/kkz6/launch-go/tests/testutil"
)

func TestDeploySite_BasicLaravel(t *testing.T) {
	config := tasks.DeploySiteConfig{
		SitePath:            "/home/launcher/example.com",
		SiteType:            tasks.SiteTypeLaravel,
		SiteAddress:         "example.com",
		Username:            "launcher",
		PHPBinary:           "php8.3",
		RepositoryURL:       "https://github.com/user/repo.git",
		RepositoryBranch:    "main",
		InstalledAt:         false,
		ZeroDowntimeDeployment: false,
		RepositoryDirectory: "/home/launcher/example.com/repository",
		LogsDirectory:       "/home/launcher/example.com/logs",
		DeploymentID:        "deploy123",
		HasAppAuth:          true,
		TempToken:           "ghs_xxxxxxxxxxxx",
		AuthURL:             "https://x-access-token:ghs_xxxxxxxxxxxx@github.com/user/repo.git",
		AppName:             "github",
	}

	task := tasks.DeploySite(config)

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
		"set -eu",
		"mkdir -p /home/launcher/example.com/repository",
		"mkdir -p /home/launcher/example.com/logs",
		"🔐 Setting up app-based authentication",
		"📦 Cloning/updating repository",
	}

	for _, section := range expectedSections {
		if !strings.Contains(script, section) {
			t.Errorf("Expected script to contain %q", section)
		}
	}

	// Match against snapshot
	testutil.MatchSnapshot(t, "tasks/deploy_basic_laravel", script)
}

func TestDeploySiteWithoutDowntime_Laravel(t *testing.T) {
	config := tasks.DeploySiteConfig{
		SitePath:               "/home/launcher/example.com",
		SiteType:               tasks.SiteTypeLaravel,
		SiteAddress:            "example.com",
		Username:               "launcher",
		PHPBinary:              "php8.3",
		RepositoryURL:          "https://github.com/user/repo.git",
		RepositoryBranch:       "main",
		InstalledAt:            false,
		ZeroDowntimeDeployment: true,
		RepositoryDirectory:    "/home/launcher/example.com/repository",
		LogsDirectory:          "/home/launcher/example.com/logs",
		SharedDirectory:        "/home/launcher/example.com/shared",
		ReleaseDirectory:       "/home/launcher/example.com/releases/20240116120000",
		ReleasesDirectory:      "/home/launcher/example.com/releases",
		CurrentDirectory:       "/home/launcher/example.com/current",
		DeploymentID:           "deploy123",
		HasAppAuth:             true,
		TempToken:              "ghs_xxxxxxxxxxxx",
		AuthURL:                "https://x-access-token:ghs_xxxxxxxxxxxx@github.com/user/repo.git",
		AppName:                "github",
		SharedDirectories:      []string{"storage"},
		SharedFiles:            []string{".env"},
		WritableDirectories: []string{
			"bootstrap/cache",
			"storage",
			"storage/app",
			"storage/framework",
			"storage/framework/cache",
			"storage/framework/sessions",
			"storage/framework/views",
			"storage/logs",
		},
		LatestDeploymentTimestamp: "20240116120000",
		RetentionCount:            5,
	}

	task := tasks.DeploySiteWithoutDowntime(config)

	if task == nil {
		t.Fatal("Expected task to be created")
	}

	script := task.Script()
	if script == "" {
		t.Fatal("Expected task script to be non-empty")
	}

	// Verify script contains expected sections for zero-downtime deployment
	expectedSections := []string{
		"#!/bin/bash",
		"set -eu",
		"mkdir -p /home/launcher/example.com/repository",
		"mkdir -p /home/launcher/example.com/shared",
		"mkdir -p /home/launcher/example.com/releases/20240116120000",
		"# Cleanup old releases",
		"# Link shared directories",
		"ln -nfs --relative",
		"storage",
		".env",
		"getfacl",
		"setfacl",
	}

	for _, section := range expectedSections {
		if !strings.Contains(script, section) {
			t.Errorf("Expected script to contain %q", section)
		}
	}

	// Verify it does NOT contain pipefail (should use set -eu only)
	if strings.Contains(script, "pipefail") {
		t.Error("Script should NOT contain pipefail (should match Laravel behavior)")
	}

	// Match against snapshot
	testutil.MatchSnapshot(t, "tasks/deploy_zero_downtime_laravel", script)
}

func TestDeploySiteWithoutDowntime_AlreadyInstalled(t *testing.T) {
	config := tasks.DeploySiteConfig{
		SitePath:               "/home/launcher/example.com",
		SiteType:               tasks.SiteTypeLaravel,
		SiteAddress:            "example.com",
		Username:               "launcher",
		PHPBinary:              "php8.3",
		RepositoryURL:          "https://github.com/user/repo.git",
		RepositoryBranch:       "main",
		InstalledAt:            true, // Already installed
		ZeroDowntimeDeployment: true,
		RepositoryDirectory:    "/home/launcher/example.com/repository",
		LogsDirectory:          "/home/launcher/example.com/logs",
		SharedDirectory:        "/home/launcher/example.com/shared",
		ReleaseDirectory:       "/home/launcher/example.com/releases/20240116130000",
		ReleasesDirectory:      "/home/launcher/example.com/releases",
		CurrentDirectory:       "/home/launcher/example.com/current",
		DeploymentID:           "deploy456",
		HasAppAuth:             true,
		TempToken:              "ghs_xxxxxxxxxxxx",
		AuthURL:                "https://x-access-token:ghs_xxxxxxxxxxxx@github.com/user/repo.git",
		AppName:                "github",
		SharedDirectories:      []string{"storage"},
		SharedFiles:            []string{".env"},
		WritableDirectories:    []string{"bootstrap/cache", "storage"},
		LatestDeploymentTimestamp: "20240116130000",
		RetentionCount:            5,
		HookAfterUpdatingRepository: "composer install --no-dev",
		HookBeforeMakingCurrent:    "php artisan migrate --force",
	}

	task := tasks.DeploySiteWithoutDowntime(config)

	script := task.Script()

	// Verify hooks are included
	if !strings.Contains(script, "composer install --no-dev") {
		t.Error("Expected script to contain hook_after_updating_repository")
	}

	if !strings.Contains(script, "php artisan migrate --force") {
		t.Error("Expected script to contain hook_before_making_current")
	}

	// Should NOT contain fresh installation steps (since InstalledAt=true)
	// The prepare-fresh-installation template is only included when InstalledAt=false

	// Match against snapshot
	testutil.MatchSnapshot(t, "tasks/deploy_zero_downtime_already_installed", script)
}

func TestDeploySite_WithSSHAuth(t *testing.T) {
	config := tasks.DeploySiteConfig{
		SitePath:            "/home/launcher/example.com",
		SiteType:            tasks.SiteTypeLaravel,
		SiteAddress:         "example.com",
		Username:            "launcher",
		PHPBinary:           "php8.3",
		RepositoryURL:       "git@github.com:user/repo.git",
		RepositoryBranch:    "main",
		InstalledAt:         false,
		ZeroDowntimeDeployment: false,
		RepositoryDirectory: "/home/launcher/example.com/repository",
		LogsDirectory:       "/home/launcher/example.com/logs",
		DeploymentID:        "deploy789",
		HasAppAuth:          false,
		DeployKeyPrivate:    "-----BEGIN OPENSSH PRIVATE KEY-----\ntest-key-content\n-----END OPENSSH PRIVATE KEY-----",
	}

	task := tasks.DeploySite(config)

	script := task.Script()

	// Verify SSH authentication is set up
	if !strings.Contains(script, "SSH") || !strings.Contains(script, "deploy_key") {
		t.Error("Expected script to contain SSH deploy key setup")
	}

	// Match against snapshot
	testutil.MatchSnapshot(t, "tasks/deploy_ssh_auth", script)
}

func TestRollbackDeployment(t *testing.T) {
	config := tasks.RollbackDeploymentConfig{
		SitePath:         "/home/launcher/example.com",
		ReleaseDirectory: "/home/launcher/example.com/releases/20240115100000",
		CurrentDirectory: "/home/launcher/example.com/current",
	}

	task := tasks.RollbackDeployment(config)

	if task == nil {
		t.Fatal("Expected task to be created")
	}

	script := task.Script()

	// Verify rollback script updates symlink
	if !strings.Contains(script, "ln -nfs") {
		t.Error("Expected script to contain symlink command")
	}

	if !strings.Contains(script, "/home/launcher/example.com/releases/20240115100000") {
		t.Error("Expected script to contain release directory")
	}

	// Match against snapshot
	testutil.MatchSnapshot(t, "tasks/rollback_deployment", script)
}
