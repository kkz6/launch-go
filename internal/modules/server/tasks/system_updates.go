package tasks

import (
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

//go:embed system_updates.py
var systemUpdatesProgram string

type SystemUpdateSettings struct {
	Enabled       bool   `json:"enabled"`
	Schedule      string `json:"schedule" validate:"required"`
	Notifications bool   `json:"notifications"`
}

type SystemUpdatePackage struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type SystemUpdateReport struct {
	ID             string                `json:"id"`
	FinishedAt     time.Time             `json:"finished_at"`
	Status         string                `json:"status"`
	Packages       []SystemUpdatePackage `json:"packages"`
	RebootRequired bool                  `json:"reboot_required"`
	Notifications  *bool                 `json:"notifications,omitempty"`
}

type SystemUpdateStatus struct {
	SystemUpdateSettings
	Supported        bool                  `json:"supported"`
	Managed          bool                  `json:"managed"`
	UpdaterInstalled bool                  `json:"updater_installed"`
	TimerActive      bool                  `json:"timer_active"`
	Running          bool                  `json:"running"`
	NativeEnabled    bool                  `json:"native_enabled"`
	RebootRequired   bool                  `json:"reboot_required"`
	MonthlyPatches   int                   `json:"security_patches_this_month"`
	LastRun          *SystemUpdateReport   `json:"last_run"`
	RecentlyUpdated  []SystemUpdatePackage `json:"recently_updated"`
}

type SystemUpdateResult struct {
	SystemUpdateStatus
	RebootID string               `json:"reboot_id"`
	Reports  []SystemUpdateReport `json:"reports"`
	Error    string               `json:"error"`
	Logs     string               `json:"logs"`
}

func SystemUpdateAction(ctx context.Context, server *models.Server, dispatcher taskrunner.TaskDispatcher, logger *zerolog.Logger, action string) (SystemUpdateResult, error) {
	switch action {
	case "logs":
	case "retry", "repair":
		action += " " + base64.StdEncoding.EncodeToString([]byte(systemUpdatesProgram))
	default:
		return SystemUpdateResult{}, fmt.Errorf("unsupported system update action")
	}
	return runSystemUpdates(ctx, server, dispatcher, logger, action, nil)
}

func SystemUpdates(ctx context.Context, server *models.Server, dispatcher taskrunner.TaskDispatcher, logger *zerolog.Logger, settings *SystemUpdateSettings) (SystemUpdateResult, error) {
	return runSystemUpdates(ctx, server, dispatcher, logger, "status", settings)
}

func MonitorSystemUpdates(ctx context.Context, server *models.Server, dispatcher taskrunner.TaskDispatcher, logger *zerolog.Logger) (SystemUpdateResult, error) {
	return runSystemUpdates(ctx, server, dispatcher, logger, "monitor", nil)
}

func runSystemUpdates(ctx context.Context, server *models.Server, dispatcher taskrunner.TaskDispatcher, logger *zerolog.Logger, args string, settings *SystemUpdateSettings) (SystemUpdateResult, error) {
	if settings != nil {
		args = fmt.Sprintf("configure %t %s %t %s", settings.Enabled, taskrunner.ShellQuote(settings.Schedule), settings.Notifications,
			base64.StdEncoding.EncodeToString([]byte(systemUpdatesProgram)))
	}
	script := "if ! command -v python3 >/dev/null; then\n" +
		"  echo '{\"supported\":false}'\n  exit 0\nfi\n" +
		"/usr/bin/python3 - " + args + " <<'LAUNCHCTL_SYSTEM_UPDATES'\n" +
		systemUpdatesProgram + "LAUNCHCTL_SYSTEM_UPDATES\n"
	task := taskrunner.NewScriptTask("System updates", script, 20*time.Second)
	result, err := NewTaskRunner(server, task).WithDispatcher(dispatcher).WithLogger(logger).AsRoot().Run(ctx)
	if err != nil {
		return SystemUpdateResult{}, err
	}
	if result.Error != nil {
		return SystemUpdateResult{}, result.Error
	}
	if !result.IsSuccessful() {
		return SystemUpdateResult{}, fmt.Errorf("system updates task failed (exit %d): %s", result.GetExitCode(), result.GetOutput())
	}
	// SSH combines stdout and stderr. The result is the last JSON line;
	// systemctl may also print service status or symlink diagnostics.
	lines := strings.Split(strings.TrimSpace(result.GetOutput()), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		var status SystemUpdateResult
		if strings.HasPrefix(lines[i], "{") && json.Unmarshal([]byte(lines[i]), &status) == nil {
			return status, nil
		}
	}
	return SystemUpdateResult{}, fmt.Errorf("invalid system updates task response")
}
