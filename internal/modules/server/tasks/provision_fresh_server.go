package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/kkz6/launch-go/internal/modules/notification/notifications"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner/markers"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner/templates"
)

const (
	// ProvisionFreshServerTaskType is the registered type name for callback reconstruction
	ProvisionFreshServerTaskType = "server:provision_fresh"
)

// ProvisionFreshServerConfig holds all configuration needed to provision a fresh server
type ProvisionFreshServerConfig struct {
	// Server and team info for callbacks
	ServerID   string
	TeamID     string
	ServerName string // For notifications

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
	SoftwareStack []types.Software

	// Database config (for MySQL or PostgreSQL)
	DatabasePassword string
	DatabaseName     string

	// Launch Agent
	AgentConfigPath string
	AgentURL        string

	// Docker/Traefik config
	AdminEmail string
}

// provisionCallbackData holds data needed for callback handling
type provisionCallbackData struct {
	ServerID       string `json:"server_id"`
	TeamID         string `json:"team_id"`
	ServerName     string `json:"server_name"`
	ServerIP       string `json:"server_ip"`
	ServerUsername string `json:"server_username"`
}

// ProvisionFreshServerTask implements Task and CallbackPayload interfaces
type ProvisionFreshServerTask struct {
	*taskrunner.BaseTask
	callback provisionCallbackData
}

// ProvisionFreshServer creates a task that provisions a fresh server with all
// provision steps and software installations combined into a single script.
// This matches the Laravel approach where all components are included in one task.
func ProvisionFreshServer(config ProvisionFreshServerConfig) *ProvisionFreshServerTask {
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

	// Calculate total steps for progress tracking
	provisionSteps := types.ForFreshServer()
	totalSteps := len(provisionSteps) + len(config.SoftwareStack)
	currentStep := 0

	// Helper to calculate progress percentage
	calcProgress := func() int {
		if totalSteps == 0 {
			return 0
		}
		// Reserve 0-90% for steps, 90-100% for cleanup
		return (currentStep * 90) / totalSteps
	}

	// 2. Loop through provision steps (matching Laravel's @foreach($provisionSteps()))
	for _, step := range provisionSteps {
		script := renderProvisionStep(step, config, swapInMB, swappiness)
		scriptBuilder.WriteString(fmt.Sprintf("\n# === Provision Step: %s ===\n", step.Description()))

		// Emit status marker before step
		scriptBuilder.WriteString(markers.BashEchoStatus(step.Description()) + "\n")

		scriptBuilder.WriteString(script)
		scriptBuilder.WriteString("\n")

		// Emit step completed and progress markers
		currentStep++
		scriptBuilder.WriteString(markers.BashEchoStepCompleted(step.String()) + "\n")
		scriptBuilder.WriteString(markers.BashEchoProgress(calcProgress()) + "\n")
	}

	// 3. Loop through software stack (matching Laravel's @foreach($softwareStack()))
	for _, software := range config.SoftwareStack {
		script := renderSoftwareInstall(software, config)
		scriptBuilder.WriteString(fmt.Sprintf("\n# === Install Software: %s ===\n", software.Label()))

		// Emit status marker before installation
		scriptBuilder.WriteString(markers.BashEchoStatus("Installing "+software.Label()) + "\n")

		scriptBuilder.WriteString(script)
		scriptBuilder.WriteString("\n")

		// Emit software installed and progress markers
		currentStep++
		scriptBuilder.WriteString(markers.BashEchoSoftwareInstalled(software.String()) + "\n")
		scriptBuilder.WriteString(markers.BashEchoProgress(calcProgress()) + "\n")
	}

	// 4. Final cleanup (from Laravel template)
	scriptBuilder.WriteString("\n# === Final Cleanup ===\n")
	scriptBuilder.WriteString(markers.BashEchoStatus("Running final cleanup") + "\n")
	scriptBuilder.WriteString("# See 'apt-update-upgrade'\n")
	scriptBuilder.WriteString("waitForAptUnlock\n")
	scriptBuilder.WriteString("sudo apt-mark unhold cloud-init\n")
	scriptBuilder.WriteString(markers.BashEchoProgress(100) + "\n")

	return &ProvisionFreshServerTask{
		BaseTask: taskrunner.NewBaseTask(
			taskrunner.WithName("Provision Fresh Server"),
			taskrunner.WithScript(scriptBuilder.String()),
			taskrunner.WithTimeoutSeconds(15*60),
		),
		callback: provisionCallbackData{
			ServerID:       config.ServerID,
			TeamID:         config.TeamID,
			ServerName:     config.ServerName,
			ServerIP:       config.PublicIPv4,
			ServerUsername: config.Username,
		},
	}
}

// TypeName returns the registered type name for reconstruction
func (t *ProvisionFreshServerTask) TypeName() string {
	return ProvisionFreshServerTaskType
}

// MarshalPayload returns JSON representation of the task state needed for callbacks
func (t *ProvisionFreshServerTask) MarshalPayload() ([]byte, error) {
	return json.Marshal(t.callback)
}

// OnSuccess is called when the task completes successfully
func (t *ProvisionFreshServerTask) OnSuccess(ctx context.Context, cbCtx *taskrunner.CallbackContext, taskID string) error {
	if cbCtx.Logger != nil {
		cbCtx.Logger.Info().
			Str("task_id", taskID).
			Str("server_id", t.callback.ServerID).
			Msg("ProvisionFreshServer: onFinished callback triggered")
	}

	now := time.Now()

	// Update server status to running
	if err := cbCtx.DB.Model(&models.Server{}).
		Where("id = ?", t.callback.ServerID).
		Updates(map[string]interface{}{
			"status":         types.ServerStatusRunning,
			"provisioned_at": now,
			"progress":       100,
		}).Error; err != nil {
		return fmt.Errorf("failed to update server status: %w", err)
	}

	// Mark all firewall rules as installed (they were configured during provisioning)
	if err := cbCtx.DB.Model(&models.FirewallRule{}).
		Where("server_id = ?", t.callback.ServerID).
		Update("installed_at", now).Error; err != nil {
		if cbCtx.Logger != nil {
			cbCtx.Logger.Warn().Err(err).Msg("Failed to mark firewall rules as installed")
		}
	}

	// Mark all services as running (Laravel uses status field, not installed_at)
	if err := cbCtx.DB.Model(&models.InstalledService{}).
		Where("server_id = ?", t.callback.ServerID).
		Update("status", types.ServiceStatusRunning).Error; err != nil {
		if cbCtx.Logger != nil {
			cbCtx.Logger.Warn().Err(err).Msg("Failed to mark services as running")
		}
	}

	// Broadcast server provisioned event
	cbCtx.BroadcastToTeam(t.callback.TeamID, "server.provisioned", map[string]interface{}{
		"server_id": t.callback.ServerID,
		"status":    "running",
	})

	// Send notification to team
	notif := notifications.NewServerProvisionedNotification(t.callback.ServerName, t.callback.ServerIP, t.callback.ServerUsername)
	if err := cbCtx.NotifyTeam(ctx, t.callback.TeamID, notif); err != nil {
		if cbCtx.Logger != nil {
			cbCtx.Logger.Warn().Err(err).Msg("Failed to send server provisioned notification")
		}
	}

	return nil
}

// OnFailure is called when the task fails with an exit code
func (t *ProvisionFreshServerTask) OnFailure(ctx context.Context, cbCtx *taskrunner.CallbackContext, taskID string, exitCode int) error {
	if cbCtx.Logger != nil {
		cbCtx.Logger.Error().
			Str("task_id", taskID).
			Str("server_id", t.callback.ServerID).
			Int("exit_code", exitCode).
			Msg("ProvisionFreshServer: onFailed callback triggered")
	}

	// Update server status to failed
	if err := cbCtx.DB.Model(&models.Server{}).
		Where("id = ?", t.callback.ServerID).
		Update("status", types.ServerStatusFailed).Error; err != nil {
		return fmt.Errorf("failed to update server status: %w", err)
	}

	// Broadcast server provision failed event
	cbCtx.BroadcastToTeam(t.callback.TeamID, "server.provision_failed", map[string]interface{}{
		"server_id": t.callback.ServerID,
		"status":    "failed",
		"exit_code": exitCode,
	})

	// Get task output for notification
	output := cbCtx.GetTaskOutputTail(taskID, 30)
	errorMessage := fmt.Sprintf("Task failed with exit code %d", exitCode)

	// Send notification to team
	notif := notifications.NewServerProvisioningFailedNotification(t.callback.ServerName, output, errorMessage)
	if err := cbCtx.NotifyTeam(ctx, t.callback.TeamID, notif); err != nil {
		if cbCtx.Logger != nil {
			cbCtx.Logger.Warn().Err(err).Msg("Failed to send server provisioning failed notification")
		}
	}

	// Dispatch cleanup job
	t.dispatchCleanupJob(cbCtx)

	return nil
}

// OnExpired is called when the task times out
func (t *ProvisionFreshServerTask) OnExpired(ctx context.Context, cbCtx *taskrunner.CallbackContext, taskID string) error {
	if cbCtx.Logger != nil {
		cbCtx.Logger.Error().
			Str("task_id", taskID).
			Str("server_id", t.callback.ServerID).
			Msg("ProvisionFreshServer: onTimeout callback triggered")
	}

	// Update server status to failed
	if err := cbCtx.DB.Model(&models.Server{}).
		Where("id = ?", t.callback.ServerID).
		Update("status", types.ServerStatusFailed).Error; err != nil {
		return fmt.Errorf("failed to update server status: %w", err)
	}

	// Broadcast server provision timeout event
	cbCtx.BroadcastToTeam(t.callback.TeamID, "server.provision_timeout", map[string]interface{}{
		"server_id": t.callback.ServerID,
		"status":    "failed",
	})

	// Get task output for notification
	output := cbCtx.GetTaskOutputTail(taskID, 30)
	errorMessage := "Task timed out"

	// Send notification to team
	notif := notifications.NewServerProvisioningFailedNotification(t.callback.ServerName, output, errorMessage)
	if err := cbCtx.NotifyTeam(ctx, t.callback.TeamID, notif); err != nil {
		if cbCtx.Logger != nil {
			cbCtx.Logger.Warn().Err(err).Msg("Failed to send server provisioning failed notification")
		}
	}

	// Dispatch cleanup job
	t.dispatchCleanupJob(cbCtx)

	return nil
}

// dispatchCleanupJob dispatches the cleanup job for failed provisioning
func (t *ProvisionFreshServerTask) dispatchCleanupJob(cbCtx *taskrunner.CallbackContext) {
	// Dispatch cleanup job via queue
	if err := cbCtx.DispatchJob("server:cleanup_failed_provisioning", map[string]interface{}{
		"server_id": t.callback.ServerID,
		"team_id":   t.callback.TeamID,
		"reason":    "Provisioning task failed or timed out",
	}); err != nil && cbCtx.Logger != nil {
		cbCtx.Logger.Error().Err(err).Msg("Failed to dispatch cleanup job")
	}
}

// NewTask implements taskrunner.CallbackStateFactory
func (s provisionCallbackData) NewTask() taskrunner.CallbackHandler {
	return &ProvisionFreshServerTask{
		BaseTask: taskrunner.NewBaseTask(),
		callback: s,
	}
}

// renderProvisionStep renders the appropriate template for a provision step
func renderProvisionStep(step types.ProvisionStep, config ProvisionFreshServerConfig, swapInMB, swappiness int) string {
	data := buildProvisionStepData(step, config, swapInMB, swappiness)
	return templates.MustRender("server", step.TemplateName(), data)
}

// buildProvisionStepData builds the template data for a provision step
func buildProvisionStepData(step types.ProvisionStep, config ProvisionFreshServerConfig, swapInMB, swappiness int) any {
	switch step {
	case types.ProvisionStepConfigureSwap:
		return struct {
			SwapInMegabytes int
			Swappiness      int
		}{swapInMB, swappiness}

	case types.ProvisionStepConfigureFirewall:
		sshPort := config.SSHPort
		if sshPort == 0 {
			sshPort = 22
		}
		return struct{ SSHPort int }{sshPort}

	case types.ProvisionStepSetupRoot:
		return struct {
			PublicKey string
			Provider  string
		}{config.PublicKey, config.Provider}

	case types.ProvisionStepSetupDefaultUser:
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
func renderSoftwareInstall(software types.Software, config ProvisionFreshServerConfig) string {
	data := buildSoftwareInstallData(software, config)
	return templates.MustRender("server", software.InstallTemplateName(), data)
}

// buildSoftwareInstallData builds the template data for software installation
func buildSoftwareInstallData(software types.Software, config ProvisionFreshServerConfig) any {
	switch software {
	case types.SoftwareMySQL80:
		return struct {
			RootPassword   string
			DatabaseName   string
			PublicIPv4     string
			MaxConnections int
		}{config.DatabasePassword, config.DatabaseName, config.PublicIPv4, software.MaxConnections(config.MemoryInMB)}

	case types.SoftwarePostgreSQL16:
		return struct {
			DatabasePassword string
			DatabaseName     string
			MaxConnections   int
		}{config.DatabasePassword, config.DatabaseName, software.MaxConnections(config.MemoryInMB)}

	case types.SoftwareCaddy2:
		return struct {
			Username   string
			PublicIPv4 string
		}{config.Username, config.PublicIPv4}

	case types.SoftwareComposer2:
		return struct{ Username string }{config.Username}

	case types.SoftwareLaunchAgent:
		return struct {
			AgentConfigPath string
			AgentURL        string
			RootUsername    string
		}{config.AgentConfigPath, config.AgentURL, "root"}

	case types.SoftwareDocker:
		return struct{ Username string }{config.Username}

	case types.SoftwareTraefik:
		email := config.AdminEmail
		if email == "" {
			email = "admin@" + config.PublicIPv4
		}
		return struct{ AdminEmail string }{email}

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
