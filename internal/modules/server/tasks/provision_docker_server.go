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
	// ProvisionDockerServerTaskType is the registered type name for callback reconstruction
	ProvisionDockerServerTaskType = "server:provision_docker"
)

// ProvisionDockerServerConfig holds all configuration needed to provision a Docker server.
// Mirrors ProvisionFreshServerConfig but omits the PHP-stack fields (software stack,
// database credentials, agent URL) and adds Docker/Traefik specifics.
type ProvisionDockerServerConfig struct {
	// Server and team info for callbacks
	ServerID   string
	TeamID     string
	ServerName string

	// Server info
	MemoryInMB int
	PublicIPv4 string
	Provider   string

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

	// Docker-stack
	NetworkName    string // e.g. "launch-network"
	RootDir        string // e.g. "/etc/launch"
	TraefikVersion string // e.g. "v3.1"
	TraefikService string // e.g. "launch-traefik"
	ACMEEmail      string // optional; if empty, the ACME resolver block is omitted
}

// provisionDockerCallbackData holds data needed for callback handling.
// Only IDs and primitives — never models or pointers.
type provisionDockerCallbackData struct {
	ServerID       string `json:"server_id"`
	TeamID         string `json:"team_id"`
	ServerName     string `json:"server_name"`
	ServerIP       string `json:"server_ip"`
	ServerUsername string `json:"server_username"`
}

// ProvisionDockerServerTask implements Task and CallbackPayload interfaces.
type ProvisionDockerServerTask struct {
	*taskrunner.BaseTask
	callback provisionDockerCallbackData
}

// ProvisionDockerServer creates a task that provisions a fresh server with the
// Docker stack: base hardening (swap/firewall/apt/packages/user/ssh) followed by
// Docker CE install, Swarm init + overlay network, /etc/launch directory tree,
// and a Traefik service deployed onto that overlay.
func ProvisionDockerServer(config ProvisionDockerServerConfig) *ProvisionDockerServerTask {
	if config.NetworkName == "" {
		config.NetworkName = DockerNetworkName
	}
	if config.RootDir == "" {
		config.RootDir = DockerRootDir
	}
	if config.TraefikVersion == "" {
		config.TraefikVersion = TraefikVersion
	}
	if config.TraefikService == "" {
		config.TraefikService = TraefikServiceName
	}

	var scriptBuilder strings.Builder

	// 1. Shell defaults and common functions (matches ProvisionFreshServer).
	scriptBuilder.WriteString("#!/bin/bash\n")
	scriptBuilder.WriteString(templates.ShellDefaults())
	scriptBuilder.WriteString("\n\n")
	scriptBuilder.WriteString(templates.CommonFunctions())
	scriptBuilder.WriteString("\n\n")
	scriptBuilder.WriteString(templates.AptFunctions())
	scriptBuilder.WriteString("\n\n")

	// Make the box safe for apt-get before doing anything. See
	// quiesceAptForProvisioning's doc in functions.go — this combines
	// cloud-init waiting, stopping/masking unattended-upgrades, killing
	// stragglers, and a final waitForAptUnlock. Removes the race where the
	// apt-daily systemd timer fires between our check and our install.
	scriptBuilder.WriteString("# Stop apt/dpkg interference before any provision step runs.\n")
	scriptBuilder.WriteString("quiesceAptForProvisioning\n\n")

	swapInMB := calculateSwapInMegabytes(config.MemoryInMB)
	swappiness := calculateSwappiness(config.MemoryInMB)

	provisionSteps := types.ForDockerServer()
	totalSteps := len(provisionSteps)
	currentStep := 0

	calcProgress := func() int {
		if totalSteps == 0 {
			return 0
		}
		return (currentStep * 90) / totalSteps
	}

	// 2. Loop through provision steps in order.
	for _, step := range provisionSteps {
		script := renderDockerProvisionStep(step, config, swapInMB, swappiness)
		scriptBuilder.WriteString(fmt.Sprintf("\n# === Provision Step: %s ===\n", step.Description()))

		scriptBuilder.WriteString(markers.BashEchoStatus(step.Description()) + "\n")

		scriptBuilder.WriteString(script)
		scriptBuilder.WriteString("\n")

		currentStep++
		scriptBuilder.WriteString(markers.BashEchoStepCompleted(step.String()) + "\n")
		scriptBuilder.WriteString(markers.BashEchoProgress(calcProgress()) + "\n")
	}

	// 3. Final cleanup. Restore the systemd units we masked at the start
	// so unattended-upgrades resumes on the live server.
	scriptBuilder.WriteString("\n# === Final Cleanup ===\n")
	scriptBuilder.WriteString(markers.BashEchoStatus("Running final cleanup") + "\n")
	scriptBuilder.WriteString("waitForAptUnlock\n")
	scriptBuilder.WriteString("sudo apt-mark unhold cloud-init || true\n")
	scriptBuilder.WriteString("restoreUnattendedUpgrades\n")
	scriptBuilder.WriteString(markers.BashEchoProgress(100) + "\n")

	return &ProvisionDockerServerTask{
		BaseTask: taskrunner.NewBaseTask(
			taskrunner.WithName("Provision Docker Server"),
			taskrunner.WithScript(scriptBuilder.String()),
			taskrunner.WithTimeoutSeconds(15*60),
		),
		callback: provisionDockerCallbackData{
			ServerID:       config.ServerID,
			TeamID:         config.TeamID,
			ServerName:     config.ServerName,
			ServerIP:       config.PublicIPv4,
			ServerUsername: config.Username,
		},
	}
}

// TypeName returns the registered type name for reconstruction.
func (t *ProvisionDockerServerTask) TypeName() string {
	return ProvisionDockerServerTaskType
}

// MarshalPayload returns JSON representation of the task state needed for callbacks.
func (t *ProvisionDockerServerTask) MarshalPayload() ([]byte, error) {
	return json.Marshal(t.callback)
}

// OnSuccess is called when the task completes successfully.
func (t *ProvisionDockerServerTask) OnSuccess(ctx context.Context, cbCtx *taskrunner.CallbackContext, taskID string) error {
	if cbCtx.Logger != nil {
		cbCtx.Logger.Info().
			Str("task_id", taskID).
			Str("server_id", t.callback.ServerID).
			Msg("ProvisionDockerServer: onFinished callback triggered")
	}

	now := time.Now()

	if err := cbCtx.DB.Model(&models.Server{}).
		Where("id = ?", t.callback.ServerID).
		Updates(map[string]interface{}{
			"status":         types.ServerStatusRunning,
			"provisioned_at": now,
			"progress":       100,
		}).Error; err != nil {
		return fmt.Errorf("failed to update server status: %w", err)
	}

	if err := cbCtx.DB.Model(&models.FirewallRule{}).
		Where("server_id = ?", t.callback.ServerID).
		Update("installed_at", now).Error; err != nil {
		if cbCtx.Logger != nil {
			cbCtx.Logger.Warn().Err(err).Msg("Failed to mark firewall rules as installed")
		}
	}

	if err := cbCtx.DB.Model(&models.InstalledService{}).
		Where("server_id = ?", t.callback.ServerID).
		Update("status", types.ServiceStatusRunning).Error; err != nil {
		if cbCtx.Logger != nil {
			cbCtx.Logger.Warn().Err(err).Msg("Failed to mark services as running")
		}
	}

	cbCtx.BroadcastToTeam(t.callback.TeamID, "server.provisioned", map[string]interface{}{
		"server_id": t.callback.ServerID,
		"status":    "running",
	})

	notif := notifications.NewServerProvisionedNotification(t.callback.ServerName, t.callback.ServerIP, t.callback.ServerUsername)
	if err := cbCtx.NotifyTeam(ctx, t.callback.TeamID, notif); err != nil {
		if cbCtx.Logger != nil {
			cbCtx.Logger.Warn().Err(err).Msg("Failed to send server provisioned notification")
		}
	}

	return nil
}

// OnFailure is called when the task fails with a non-zero exit code.
func (t *ProvisionDockerServerTask) OnFailure(ctx context.Context, cbCtx *taskrunner.CallbackContext, taskID string, exitCode int) error {
	if cbCtx.Logger != nil {
		cbCtx.Logger.Error().
			Str("task_id", taskID).
			Str("server_id", t.callback.ServerID).
			Int("exit_code", exitCode).
			Msg("ProvisionDockerServer: onFailed callback triggered")
	}

	if err := cbCtx.DB.Model(&models.Server{}).
		Where("id = ?", t.callback.ServerID).
		Update("status", types.ServerStatusFailed).Error; err != nil {
		return fmt.Errorf("failed to update server status: %w", err)
	}

	cbCtx.BroadcastToTeam(t.callback.TeamID, "server.provision_failed", map[string]interface{}{
		"server_id": t.callback.ServerID,
		"status":    "failed",
		"exit_code": exitCode,
	})

	output := cbCtx.GetTaskOutputTail(taskID, 30)
	errorMessage := fmt.Sprintf("Task failed with exit code %d", exitCode)

	notif := notifications.NewServerProvisioningFailedNotification(t.callback.ServerName, output, errorMessage)
	if err := cbCtx.NotifyTeam(ctx, t.callback.TeamID, notif); err != nil {
		if cbCtx.Logger != nil {
			cbCtx.Logger.Warn().Err(err).Msg("Failed to send server provisioning failed notification")
		}
	}

	t.dispatchCleanupJob(cbCtx)
	return nil
}

// OnExpired is called when the task times out.
func (t *ProvisionDockerServerTask) OnExpired(ctx context.Context, cbCtx *taskrunner.CallbackContext, taskID string) error {
	if cbCtx.Logger != nil {
		cbCtx.Logger.Error().
			Str("task_id", taskID).
			Str("server_id", t.callback.ServerID).
			Msg("ProvisionDockerServer: onTimeout callback triggered")
	}

	if err := cbCtx.DB.Model(&models.Server{}).
		Where("id = ?", t.callback.ServerID).
		Update("status", types.ServerStatusFailed).Error; err != nil {
		return fmt.Errorf("failed to update server status: %w", err)
	}

	cbCtx.BroadcastToTeam(t.callback.TeamID, "server.provision_timeout", map[string]interface{}{
		"server_id": t.callback.ServerID,
		"status":    "failed",
	})

	output := cbCtx.GetTaskOutputTail(taskID, 30)
	notif := notifications.NewServerProvisioningFailedNotification(t.callback.ServerName, output, "Task timed out")
	if err := cbCtx.NotifyTeam(ctx, t.callback.TeamID, notif); err != nil {
		if cbCtx.Logger != nil {
			cbCtx.Logger.Warn().Err(err).Msg("Failed to send server provisioning failed notification")
		}
	}

	t.dispatchCleanupJob(cbCtx)
	return nil
}

func (t *ProvisionDockerServerTask) dispatchCleanupJob(cbCtx *taskrunner.CallbackContext) {
	if err := cbCtx.DispatchJob("server:cleanup_failed_provisioning", map[string]interface{}{
		"server_id": t.callback.ServerID,
		"team_id":   t.callback.TeamID,
		"reason":    "Docker provisioning task failed or timed out",
	}); err != nil && cbCtx.Logger != nil {
		cbCtx.Logger.Error().Err(err).Msg("Failed to dispatch cleanup job")
	}
}

// NewTask implements taskrunner.CallbackStateFactory for reconstruction from DB.
func (s provisionDockerCallbackData) NewTask() taskrunner.CallbackHandler {
	return &ProvisionDockerServerTask{
		BaseTask: taskrunner.NewBaseTask(),
		callback: s,
	}
}

// renderDockerProvisionStep renders the appropriate template for a docker-stack step.
// Base hardening steps reuse buildProvisionStepData (sharing logic with the PHP path);
// the docker-specific steps have their own data builders.
func renderDockerProvisionStep(step types.ProvisionStep, config ProvisionDockerServerConfig, swapInMB, swappiness int) string {
	switch step {
	case types.ProvisionStepInstallDocker:
		return templates.MustRender("server", step.TemplateName(), struct {
			Username string
		}{config.Username})

	case types.ProvisionStepSetupSwarmNetwork:
		return templates.MustRender("server", step.TemplateName(), struct {
			NetworkName string
			PublicIPv4  string
		}{config.NetworkName, config.PublicIPv4})

	case types.ProvisionStepSetupLaunchDirs:
		return templates.MustRender("server", step.TemplateName(), struct {
			RootDir  string
			Username string
		}{config.RootDir, config.Username})

	case types.ProvisionStepInstallTraefik:
		return templates.MustRender("server", step.TemplateName(), struct {
			RootDir        string
			NetworkName    string
			ServiceName    string
			TraefikVersion string
			ACMEEmail      string
		}{config.RootDir, config.NetworkName, config.TraefikService, config.TraefikVersion, config.ACMEEmail})
	}

	// Fall through to the shared base-hardening renderer.
	fresh := ProvisionFreshServerConfig{
		ServerID:         config.ServerID,
		TeamID:           config.TeamID,
		ServerName:       config.ServerName,
		MemoryInMB:       config.MemoryInMB,
		PublicIPv4:       config.PublicIPv4,
		Provider:         config.Provider,
		PublicKey:        config.PublicKey,
		Username:         config.Username,
		Password:         config.Password,
		WorkingDirectory: config.WorkingDirectory,
		SSHKeys:          config.SSHKeys,
		AppURL:           config.AppURL,
		AppName:          config.AppName,
		SSHPort:          config.SSHPort,
	}
	return renderProvisionStep(step, fresh, swapInMB, swappiness)
}
