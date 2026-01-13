package tasks

import (
	"fmt"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/site/tasks/templates"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// SiteType represents the type of site (Laravel, WordPress, etc.)
type SiteType string

const (
	SiteTypeLaravel   SiteType = "laravel"
	SiteTypeWordpress SiteType = "wordpress"
	SiteTypeStatic    SiteType = "static"
	SiteTypeGeneric   SiteType = "generic"
)

// DeploySiteConfig holds configuration for deploying a site
type DeploySiteConfig struct {
	// Site info
	SitePath             string
	SiteType             SiteType
	SiteAddress          string
	Username             string
	PHPBinary            string
	RepositoryURL        string
	RepositoryBranch     string
	InstalledAt          bool // Whether site has been installed before
	ZeroDowntimeDeployment bool

	// Directories
	RepositoryDirectory string
	LogsDirectory       string
	SharedDirectory     string
	ReleaseDirectory    string
	ReleasesDirectory   string
	CurrentDirectory    string

	// Authentication
	DeploymentID     string
	HasAppAuth       bool
	TempToken        string
	AuthURL          string
	DeployKeyPrivate string
	AppName          string

	// Hooks
	HookBeforeUpdatingRepository string
	HookAfterUpdatingRepository  string
	HookBeforeMakingCurrent      string
	HookAfterMakingCurrent       string

	// Environment
	EnvVariables map[string]string

	// Shared paths
	SharedDirectories    []string
	SharedFiles          []string
	WritableDirectories  []string

	// Cleanup
	LatestDeploymentTimestamp string
	RetentionCount            int

	// Callback
	CallbackURL string
}

// DeploySite creates a task that deploys a site (without zero-downtime)
func DeploySite(config DeploySiteConfig) *taskrunner.BaseTask {
	var scriptBuilder strings.Builder

	// Shell defaults
	scriptBuilder.WriteString("#!/bin/bash\n")
	scriptBuilder.WriteString(templates.ShellDefaults())
	scriptBuilder.WriteString("\n\n")

	// Shell variables (PHP binary)
	scriptBuilder.WriteString(templates.MustRender("deployment/shell_variables.sh", struct {
		PHPBinary string
	}{config.PHPBinary}))
	scriptBuilder.WriteString("\n")

	// Create necessary directories
	scriptBuilder.WriteString(fmt.Sprintf("# Create the necessary directories\n"))
	scriptBuilder.WriteString(fmt.Sprintf("mkdir -p %s\n", config.RepositoryDirectory))
	scriptBuilder.WriteString(fmt.Sprintf("mkdir -p %s\n", config.LogsDirectory))
	scriptBuilder.WriteString("\n")

	// Hook: before updating repository
	if config.InstalledAt && config.HookBeforeUpdatingRepository != "" {
		scriptBuilder.WriteString("echo \"Running hook before updating repository\"\n")
		scriptBuilder.WriteString(fmt.Sprintf("cd %s\n", config.RepositoryDirectory))
		scriptBuilder.WriteString(config.HookBeforeUpdatingRepository + "\n\n")
	}

	// Update repository
	if config.RepositoryURL != "" {
		scriptBuilder.WriteString(renderUpdateRepository(config))
		scriptBuilder.WriteString("\n")

		// Hook: after updating repository
		if config.HookAfterUpdatingRepository != "" {
			scriptBuilder.WriteString("echo \"Running hook after updating repository\"\n")
			scriptBuilder.WriteString(fmt.Sprintf("cd %s\n", config.RepositoryDirectory))
			scriptBuilder.WriteString(config.HookAfterUpdatingRepository + "\n\n")
		}
	}

	// Prepare fresh installation (first deploy)
	if !config.InstalledAt {
		scriptBuilder.WriteString(renderPrepareFreshInstallation(config))
		scriptBuilder.WriteString("\n")
	}

	// WordPress already installed message
	if config.InstalledAt && config.SiteType == SiteTypeWordpress {
		scriptBuilder.WriteString("echo \"Wordpress already installed!\"\n\n")
	}

	scriptBuilder.WriteString("echo \"Done!\"\n")

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Deploy Site"),
		taskrunner.WithScript(scriptBuilder.String()),
		taskrunner.WithTimeoutSeconds(600),
	)
}

// DeploySiteWithoutDowntime creates a task that deploys a site with zero-downtime
func DeploySiteWithoutDowntime(config DeploySiteConfig) *taskrunner.BaseTask {
	config.ZeroDowntimeDeployment = true
	var scriptBuilder strings.Builder

	// Shell defaults
	scriptBuilder.WriteString("#!/bin/bash\n")
	scriptBuilder.WriteString(templates.ShellDefaults())
	scriptBuilder.WriteString("\n\n")

	// Shell variables
	scriptBuilder.WriteString(templates.MustRender("deployment/shell_variables.sh", struct {
		PHPBinary string
	}{config.PHPBinary}))
	scriptBuilder.WriteString("\n")

	// Create necessary directories
	scriptBuilder.WriteString("# Create the necessary directories\n")
	scriptBuilder.WriteString(fmt.Sprintf("mkdir -p %s\n", config.RepositoryDirectory))
	scriptBuilder.WriteString(fmt.Sprintf("mkdir -p %s\n", config.SharedDirectory))
	scriptBuilder.WriteString(fmt.Sprintf("mkdir -p %s\n", config.ReleaseDirectory))
	scriptBuilder.WriteString(fmt.Sprintf("mkdir -p %s\n", config.LogsDirectory))
	scriptBuilder.WriteString("\n")

	// Cleanup old releases
	scriptBuilder.WriteString("# Cleanup old releases\n")
	scriptBuilder.WriteString(templates.MustRender("deployment/cleanup_old_releases.sh", struct {
		LatestDeploymentTimestamp string
		ReleasesDirectory         string
		RetentionCount            int
	}{
		LatestDeploymentTimestamp: config.LatestDeploymentTimestamp,
		ReleasesDirectory:         config.ReleasesDirectory,
		RetentionCount:            config.RetentionCount,
	}))
	scriptBuilder.WriteString("\n")

	// Hook: before updating repository
	if config.HookBeforeUpdatingRepository != "" {
		scriptBuilder.WriteString("echo \"Running hook before updating repository\"\n")
		scriptBuilder.WriteString(fmt.Sprintf("cd %s\n", config.ReleaseDirectory))
		scriptBuilder.WriteString(config.HookBeforeUpdatingRepository + "\n\n")
	}

	// Update repository
	if config.RepositoryURL != "" {
		scriptBuilder.WriteString(renderUpdateRepository(config))
		scriptBuilder.WriteString("\n")

		// Hook: after updating repository
		if config.HookAfterUpdatingRepository != "" {
			scriptBuilder.WriteString("echo \"Running hook after updating repository\"\n")
			scriptBuilder.WriteString(fmt.Sprintf("cd %s\n", config.ReleaseDirectory))
			scriptBuilder.WriteString(config.HookAfterUpdatingRepository + "\n\n")
		}
	}

	// Prepare fresh installation
	if !config.InstalledAt {
		scriptBuilder.WriteString(renderPrepareFreshInstallation(config))
		scriptBuilder.WriteString("\n")
	}

	// Link shared directories
	scriptBuilder.WriteString("# Link shared directories\n")
	scriptBuilder.WriteString(templates.MustRender("deployment/link_shared_directories.sh", struct {
		SharedDirectories []string
		SharedDirectory   string
		ReleaseDirectory  string
	}{
		SharedDirectories: config.SharedDirectories,
		SharedDirectory:   config.SharedDirectory,
		ReleaseDirectory:  config.ReleaseDirectory,
	}))
	scriptBuilder.WriteString("\n")

	// Link shared files
	scriptBuilder.WriteString("# Link shared files\n")
	scriptBuilder.WriteString(templates.MustRender("deployment/link_shared_files.sh", struct {
		SharedFiles      []string
		SharedDirectory  string
		ReleaseDirectory string
	}{
		SharedFiles:      config.SharedFiles,
		SharedDirectory:  config.SharedDirectory,
		ReleaseDirectory: config.ReleaseDirectory,
	}))
	scriptBuilder.WriteString("\n")

	// Make directories writable
	scriptBuilder.WriteString("# Make directories writable\n")
	scriptBuilder.WriteString(templates.MustRender("deployment/make_directories_writable.sh", struct {
		WritableDirectories []string
		ReleaseDirectory    string
		Username            string
	}{
		WritableDirectories: config.WritableDirectories,
		ReleaseDirectory:    config.ReleaseDirectory,
		Username:            config.Username,
	}))
	scriptBuilder.WriteString("\n")

	// Hook: before making current
	if config.HookBeforeMakingCurrent != "" {
		scriptBuilder.WriteString("echo \"Running hook before putting the site live\"\n")
		scriptBuilder.WriteString(fmt.Sprintf("cd %s\n", config.ReleaseDirectory))
		scriptBuilder.WriteString(config.HookBeforeMakingCurrent + "\n\n")
	}

	// Make deployment current
	scriptBuilder.WriteString("# Make deployment current\n")
	scriptBuilder.WriteString(templates.MustRender("deployment/make_deployment_current.sh", struct {
		SitePath         string
		ReleaseDirectory string
		CurrentDirectory string
	}{
		SitePath:         config.SitePath,
		ReleaseDirectory: config.ReleaseDirectory,
		CurrentDirectory: config.CurrentDirectory,
	}))
	scriptBuilder.WriteString("\n")

	// Hook: after making current
	if config.HookAfterMakingCurrent != "" {
		scriptBuilder.WriteString("echo \"Running hook after putting the site live\"\n")
		scriptBuilder.WriteString(fmt.Sprintf("cd %s\n", config.ReleaseDirectory))
		scriptBuilder.WriteString(config.HookAfterMakingCurrent + "\n\n")
	}

	scriptBuilder.WriteString("echo \"Done!\"\n")

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Deploy Site (Zero Downtime)"),
		taskrunner.WithScript(scriptBuilder.String()),
		taskrunner.WithTimeoutSeconds(600),
	)
}

// renderUpdateRepository renders the update repository template
func renderUpdateRepository(config DeploySiteConfig) string {
	return templates.MustRender("deployment/update_repository.sh", struct {
		RepositoryDirectory    string
		RepositoryURL          string
		RepositoryBranch       string
		SitePath               string
		ZeroDowntimeDeployment bool
		DeploymentID           string
		HasAppAuth             bool
		TempToken              string
		AuthURL                string
		DeployKeyPrivate       string
		AppName                string
		ReleaseDirectory       string
	}{
		RepositoryDirectory:    config.RepositoryDirectory,
		RepositoryURL:          config.RepositoryURL,
		RepositoryBranch:       config.RepositoryBranch,
		SitePath:               config.SitePath,
		ZeroDowntimeDeployment: config.ZeroDowntimeDeployment,
		DeploymentID:           config.DeploymentID,
		HasAppAuth:             config.HasAppAuth,
		TempToken:              config.TempToken,
		AuthURL:                config.AuthURL,
		DeployKeyPrivate:       config.DeployKeyPrivate,
		AppName:                config.AppName,
		ReleaseDirectory:       config.ReleaseDirectory,
	})
}

// renderPrepareFreshInstallation renders the prepare fresh installation template
func renderPrepareFreshInstallation(config DeploySiteConfig) string {
	var scriptBuilder strings.Builder
	scriptBuilder.WriteString(fmt.Sprintf("cd %s\n\n", config.SitePath))

	switch config.SiteType {
	case SiteTypeLaravel:
		scriptBuilder.WriteString(templates.MustRender("deployment/prepare_fresh_installation/laravel.sh", struct {
			SitePath               string
			ZeroDowntimeDeployment bool
			SharedDirectory        string
			ReleaseDirectory       string
			RepositoryDirectory    string
			EnvVariables           map[string]string
		}{
			SitePath:               config.SitePath,
			ZeroDowntimeDeployment: config.ZeroDowntimeDeployment,
			SharedDirectory:        config.SharedDirectory,
			ReleaseDirectory:       config.ReleaseDirectory,
			RepositoryDirectory:    config.RepositoryDirectory,
			EnvVariables:           config.EnvVariables,
		}))

	case SiteTypeWordpress:
		scriptBuilder.WriteString(templates.MustRender("deployment/prepare_fresh_installation/wordpress.sh", struct {
			SitePath            string
			RepositoryDirectory string
			EnvVariables        map[string]string
		}{
			SitePath:            config.SitePath,
			RepositoryDirectory: config.RepositoryDirectory,
			EnvVariables:        config.EnvVariables,
		}))
	}

	scriptBuilder.WriteString(fmt.Sprintf("\ncd %s\n", config.SitePath))
	return scriptBuilder.String()
}

// RollbackDeploymentConfig holds configuration for rolling back a deployment
type RollbackDeploymentConfig struct {
	SitePath         string
	ReleaseDirectory string
	CurrentDirectory string
}

// RollbackDeployment creates a task to rollback to a previous deployment
func RollbackDeployment(config RollbackDeploymentConfig) *taskrunner.BaseTask {
	script := templates.MustRender("rollback_deployment.sh", struct {
		SitePath         string
		ReleaseDirectory string
		CurrentDirectory string
	}{
		SitePath:         config.SitePath,
		ReleaseDirectory: config.ReleaseDirectory,
		CurrentDirectory: config.CurrentDirectory,
	})

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Rollback Deployment"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(60),
	)
}
