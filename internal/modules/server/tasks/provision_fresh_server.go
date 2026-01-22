package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/kkz6/launch-go/internal/modules/notification/notifications"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks/templates"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	pkgtemplates "github.com/kkz6/launch-go/internal/pkg/taskrunner/templates"
)

const (
	// ProvisionFreshServerTaskType is the registered type name for callback reconstruction
	ProvisionFreshServerTaskType = "server:provision_fresh"
)

// ProvisionFreshServerConfig holds all configuration needed to provision a fresh server
type ProvisionFreshServerConfig struct {
	// Callback URL for progress reporting
	CallbackURL string

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
}

// provisionCallbackData holds data needed for callback handling
type provisionCallbackData struct {
	ServerID   string `json:"server_id"`
	TeamID     string `json:"team_id"`
	ServerName string `json:"server_name"`
	ServerIP   string `json:"server_ip"`
}

// provisionFreshServerTask implements Task and CallbackPayload interfaces
type provisionFreshServerTask struct {
	*taskrunner.BaseTask
	callback provisionCallbackData
}

// ProvisionFreshServer creates a task that provisions a fresh server with all
// provision steps and software installations combined into a single script.
// This matches the Laravel approach where all components are included in one task.
func ProvisionFreshServer(config ProvisionFreshServerConfig) *provisionFreshServerTask {
	var scriptBuilder strings.Builder

	// 1. Shell defaults and common functions (like Laravel's @include)
	scriptBuilder.WriteString("#!/bin/bash\n")
	scriptBuilder.WriteString(pkgtemplates.ShellDefaults())
	scriptBuilder.WriteString("\n\n")
	scriptBuilder.WriteString(pkgtemplates.CommonFunctions())
	scriptBuilder.WriteString("\n\n")
	scriptBuilder.WriteString(pkgtemplates.AptFunctions())
	scriptBuilder.WriteString("\n\n")

	// Calculate swap settings (system-level)
	swapInMB := calculateSwapInMegabytes(config.MemoryInMB)
	swappiness := calculateSwappiness(config.MemoryInMB)

	// 2. Loop through provision steps (matching Laravel's @foreach($provisionSteps()))
	provisionSteps := types.ForFreshServer()
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

	return &provisionFreshServerTask{
		BaseTask: taskrunner.NewBaseTask(
			taskrunner.WithName("Provision Fresh Server"),
			taskrunner.WithScript(scriptBuilder.String()),
			taskrunner.WithTimeoutSeconds(15*60),
		),
		callback: provisionCallbackData{
			ServerID:   config.ServerID,
			TeamID:     config.TeamID,
			ServerName: config.ServerName,
			ServerIP:   config.PublicIPv4,
		},
	}
}

// TypeName returns the registered type name for reconstruction
func (t *provisionFreshServerTask) TypeName() string {
	return ProvisionFreshServerTaskType
}

// MarshalPayload returns JSON representation of the task state needed for callbacks
func (t *provisionFreshServerTask) MarshalPayload() ([]byte, error) {
	return json.Marshal(t.callback)
}

// OnSuccess is called when the task completes successfully
func (t *provisionFreshServerTask) OnSuccess(ctx context.Context, cbCtx *taskrunner.CallbackContext, taskID string) error {
	if cbCtx.Logger != nil {
		cbCtx.Logger.Info().
			Str("task_id", taskID).
			Str("server_id", t.callback.ServerID).
			Msg("ProvisionFreshServer: onFinished callback triggered")
	}

	// Update server status to running
	now := time.Now()
	if err := cbCtx.DB.Model(&models.Server{}).
		Where("id = ?", t.callback.ServerID).
		Updates(map[string]interface{}{
			"status":       types.ServerStatusRunning,
			"installed_at": now,
		}).Error; err != nil {
		return fmt.Errorf("failed to update server status: %w", err)
	}

	// Broadcast server provisioned event
	cbCtx.BroadcastToTeam(t.callback.TeamID, "server.provisioned", map[string]interface{}{
		"server_id": t.callback.ServerID,
		"status":    "running",
	})

	// Send notification to team
	notif := notifications.NewServerProvisionedNotification(t.callback.ServerName, t.callback.ServerIP)
	if err := cbCtx.NotifyTeam(ctx, t.callback.TeamID, notif); err != nil {
		if cbCtx.Logger != nil {
			cbCtx.Logger.Warn().Err(err).Msg("Failed to send server provisioned notification")
		}
	}

	return nil
}

// OnFailure is called when the task fails with an exit code
func (t *provisionFreshServerTask) OnFailure(ctx context.Context, cbCtx *taskrunner.CallbackContext, taskID string, exitCode int) error {
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
	output := t.getTaskOutput(cbCtx, taskID)
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
func (t *provisionFreshServerTask) OnExpired(ctx context.Context, cbCtx *taskrunner.CallbackContext, taskID string) error {
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
	output := t.getTaskOutput(cbCtx, taskID)
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

// getTaskOutput retrieves the last 30 lines of task output from the database
func (t *provisionFreshServerTask) getTaskOutput(cbCtx *taskrunner.CallbackContext, taskID string) string {
	if cbCtx.DB == nil {
		return ""
	}

	var task models.Task
	if err := cbCtx.DB.Select("output").First(&task, "id = ?", taskID).Error; err != nil {
		return ""
	}

	output := task.Output.String()
	if output == "" {
		return ""
	}

	// Return last 30 lines
	lines := strings.Split(output, "\n")
	if len(lines) > 30 {
		lines = lines[len(lines)-30:]
	}
	return strings.Join(lines, "\n")
}

// dispatchCleanupJob dispatches the cleanup job for failed provisioning
func (t *provisionFreshServerTask) dispatchCleanupJob(cbCtx *taskrunner.CallbackContext) {
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
	return &provisionFreshServerTask{
		BaseTask: taskrunner.NewBaseTask(),
		callback: s,
	}
}

// renderProvisionStep renders the appropriate template for a provision step
func renderProvisionStep(step types.ProvisionStep, config ProvisionFreshServerConfig, swapInMB, swappiness int) string {
	data := buildProvisionStepData(step, config, swapInMB, swappiness)
	return templates.MustRender(step.TemplateName(), data)
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
	return templates.MustRender(software.InstallTemplateName(), data)
}

// buildSoftwareInstallData builds the template data for software installation
func buildSoftwareInstallData(software types.Software, config ProvisionFreshServerConfig) any {
	switch software {
	case types.SoftwareMySql80:
		return struct {
			RootPassword   string
			DatabaseName   string
			PublicIPv4     string
			MaxConnections int
		}{config.DatabasePassword, config.DatabaseName, config.PublicIPv4, software.MaxConnections(config.MemoryInMB)}

	case types.SoftwarePostgreSql16:
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
