package tasks

import (
	"fmt"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/tasks/templates"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// ProvisionFreshServerConfig holds all configuration needed to provision a fresh server
type ProvisionFreshServerConfig struct {
	// Callback URL for progress reporting
	CallbackURL string

	// Server info
	MemoryInMB int
	PublicIPv4 string
	Provider   string // aws, digitalocean, etc.

	// Root setup
	PublicKey string

	// Default user setup
	Username         string
	Password         string
	WorkingDirectory string
	SSHKeys          []string
	AppURL           string
	AppName          string

	// Firewall
	SSHPort int

	// Software stack to install
	SoftwareStack []enums.Software

	// Database config (for MySQL or PostgreSQL)
	DatabasePassword string
	DatabaseName     string

	// Launch Agent
	AgentConfigPath string
	AgentURL        string
}

// ProvisionFreshServer creates a task that provisions a fresh server with all
// provision steps and software installations combined into a single script.
// This matches the Laravel approach where all components are included in one task.
func ProvisionFreshServer(config ProvisionFreshServerConfig) *taskrunner.BaseTask {
	var scriptBuilder strings.Builder

	// 1. Shell defaults and common functions (like Laravel's @include)
	scriptBuilder.WriteString("#!/bin/bash\n")
	scriptBuilder.WriteString(templates.ShellDefaults())
	scriptBuilder.WriteString("\n\n")
	scriptBuilder.WriteString(templates.CommonFunctions())
	scriptBuilder.WriteString("\n\n")
	scriptBuilder.WriteString(templates.AptFunctions())
	scriptBuilder.WriteString("\n\n")

	// Calculate swap settings (system-level)
	swapInMB := calculateSwapInMegabytes(config.MemoryInMB)
	swappiness := calculateSwappiness(config.MemoryInMB)

	// 2. Loop through provision steps (matching Laravel's @foreach($provisionSteps()))
	provisionSteps := enums.ForFreshServer()
	for _, step := range provisionSteps {
		script := renderProvisionStep(step, config, swapInMB, swappiness)
		scriptBuilder.WriteString(fmt.Sprintf("\n# === Provision Step: %s ===\n", step.Description()))
		scriptBuilder.WriteString(script)
		scriptBuilder.WriteString("\n")

		// Add callback after each step (like Laravel's <x-server::task-callback>)
		if config.CallbackURL != "" {
			scriptBuilder.WriteString(fmt.Sprintf(
				`httpPostSilently "%s" '{"provision_step_completed":"%s"}'`+"\n",
				config.CallbackURL, step.String(),
			))
		}
	}

	// 3. Loop through software stack (matching Laravel's @foreach($softwareStack()))
	for _, software := range config.SoftwareStack {
		script := renderSoftwareInstall(software, config)
		scriptBuilder.WriteString(fmt.Sprintf("\n# === Install Software: %s ===\n", software.Label()))
		scriptBuilder.WriteString(script)
		scriptBuilder.WriteString("\n")

		// Add callback after each software install
		if config.CallbackURL != "" {
			scriptBuilder.WriteString(fmt.Sprintf(
				`httpPostSilently "%s" '{"software_installed":"%s"}'`+"\n",
				config.CallbackURL, software.String(),
			))
		}
	}

	// 4. Final cleanup (from Laravel template)
	scriptBuilder.WriteString("\n# === Final Cleanup ===\n")
	scriptBuilder.WriteString("# See 'apt-update-upgrade'\n")
	scriptBuilder.WriteString("waitForAptUnlock\n")
	scriptBuilder.WriteString("sudo apt-mark unhold cloud-init\n")

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Provision Fresh Server"),
		taskrunner.WithScript(scriptBuilder.String()),
		taskrunner.WithTimeout(15*60), // 15 minutes
	)
}

// renderProvisionStep renders the appropriate template for a provision step
func renderProvisionStep(step enums.ProvisionStep, config ProvisionFreshServerConfig, swapInMB, swappiness int) string {
	data := buildProvisionStepData(step, config, swapInMB, swappiness)
	return templates.MustRender(step.TemplateName(), data)
}

// buildProvisionStepData builds the template data for a provision step
func buildProvisionStepData(step enums.ProvisionStep, config ProvisionFreshServerConfig, swapInMB, swappiness int) any {
	switch step {
	case enums.ProvisionStepConfigureSwap:
		return struct {
			SwapInMegabytes int
			Swappiness      int
		}{swapInMB, swappiness}

	case enums.ProvisionStepConfigureFirewall:
		sshPort := config.SSHPort
		if sshPort == 0 {
			sshPort = 22
		}
		return struct{ SSHPort int }{sshPort}

	case enums.ProvisionStepSetupRoot:
		return struct {
			PublicKey string
			Provider  string
		}{config.PublicKey, config.Provider}

	case enums.ProvisionStepSetupDefaultUser:
		return struct {
			Username         string
			Password         string
			WorkingDirectory string
			SSHKeys          []string
			AppURL           string
			AppName          string
		}{
			config.Username,
			config.Password,
			config.WorkingDirectory,
			config.SSHKeys,
			config.AppURL,
			config.AppName,
		}

	default:
		return nil
	}
}

// renderSoftwareInstall renders the appropriate template for software installation
func renderSoftwareInstall(software enums.Software, config ProvisionFreshServerConfig) string {
	data := buildSoftwareInstallData(software, config)
	return templates.MustRender(software.InstallTemplateName(), data)
}

// buildSoftwareInstallData builds the template data for software installation
func buildSoftwareInstallData(software enums.Software, config ProvisionFreshServerConfig) any {
	switch software {
	case enums.SoftwareMySql80:
		return struct {
			RootPassword   string
			DatabaseName   string
			PublicIPv4     string
			MaxConnections int
		}{config.DatabasePassword, config.DatabaseName, config.PublicIPv4, software.MaxConnections(config.MemoryInMB)}

	case enums.SoftwarePostgreSql16:
		return struct {
			DatabasePassword string
			DatabaseName     string
			MaxConnections   int
		}{config.DatabasePassword, config.DatabaseName, software.MaxConnections(config.MemoryInMB)}

	case enums.SoftwareCaddy2:
		return struct {
			Username   string
			PublicIPv4 string
		}{config.Username, config.PublicIPv4}

	case enums.SoftwareComposer2:
		return struct{ Username string }{config.Username}

	case enums.SoftwareLaunchAgent:
		return struct {
			AgentConfigPath string
			AgentURL        string
			RootUsername    string
		}{config.AgentConfigPath, config.AgentURL, "root"}

	default:
		// Handle PHP versions
		if software.IsPhp() {
			return struct {
				Version     string
				Username    string
				MaxChildren int
			}{software.GetVersion(), config.Username, software.MaxChildren(config.MemoryInMB)}
		}
		return nil
	}
}

// calculateSwapInMegabytes returns swap size based on server memory
func calculateSwapInMegabytes(memoryInMB int) int {
	switch {
	case memoryInMB <= 2048:
		return 1024
	case memoryInMB <= 4096:
		return 2048
	case memoryInMB <= 8192:
		return 3072
	default:
		return 4096
	}
}

// calculateSwappiness returns swappiness value based on server memory
func calculateSwappiness(memoryInMB int) int {
	switch {
	case memoryInMB <= 1024:
		return 20
	case memoryInMB <= 2048:
		return 35
	case memoryInMB <= 4096:
		return 50
	default:
		return 60
	}
}

