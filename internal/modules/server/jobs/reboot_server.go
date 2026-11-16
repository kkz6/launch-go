package jobs

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

const (
	TypeRebootServer        = "server:reboot"
	TypeVerifyServerRestart = "server:verify_restart"
	TypePollServerRestarts  = "server:poll_restarts"
)

var restartBootID = regexp.MustCompile(`(?m)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

const restartGuard = `umask 077
mkdir -p /var/lib/launchctl/system-updates || exit 1
exec 9>>/var/lib/launchctl/system-updates/lock
flock -n 9 || exit 73
for unit in launchctl-system-updates.service launchctl-system-updates-manual.service; do
  state=$(systemctl show "$unit" -p ActiveState --value 2>/dev/null || true)
  case "$state" in active|activating|deactivating) exit 73;; esac
done
`

const restartTimeoutMessage = "The restart could not be confirmed within five minutes. Check the server in your provider console, then check again; another restart will not be sent."

type RebootServerPayload struct {
	ServerID    string  `json:"server_id"`
	OperationID string  `json:"operation_id"`
	UserID      *string `json:"user_id,omitempty"`
}

type RebootServerJob struct {
	Deps    *JobDeps
	Payload RebootServerPayload
}

func NewRebootServerJob(p RebootServerPayload) pkgjobs.Handler {
	return &RebootServerJob{Deps: deps, Payload: p}
}

func (j *RebootServerJob) Handle(ctx context.Context) error {
	server, state, err := readRestart(ctx, j.Deps, j.Payload)
	if err != nil || server == nil || !state.Active() {
		return err
	}
	if state.DeadlineAt == nil || time.Now().After(*state.DeadlineAt) {
		return finishRestart(ctx, j.Deps, server, state, "failed", restartTimeoutMessage)
	}
	if server.IsArchived() {
		return finishRestart(ctx, j.Deps, server, state, "failed", "The server is archived. Restore it before requesting a restart.")
	}
	// Claim before reading the host. A retry or redelivery can never resend
	// the reboot, including a crash between issuing it and persisting success.
	claimed := j.Deps.DB.WithContext(ctx).Model(&models.ServerRestart{}).
		Where("server_id = ? AND operation_id = ? AND status = ?", server.ID, state.OperationID, "queued").
		Updates(map[string]any{"status": "restarting", "message": "Preparing the server restart."})
	if claimed.Error != nil || claimed.RowsAffected == 0 {
		return claimed.Error
	}
	restartEvent(j.Deps, server)

	result, err := runRestartTask(ctx, j.Deps, server, "Read restart boot ID", restartGuard+"cat /proc/sys/kernel/random/boot_id")
	if err != nil || result == nil || !result.IsSuccessful() {
		message := "The server could not be reached before restarting. Check its connection and try again."
		if result != nil && result.GetExitCode() == 73 {
			message = "Security updates are running. Wait for them to finish, then restart the server."
		}
		return finishRestart(ctx, j.Deps, server, state, "failed", message)
	}
	bootID := restartBootID.FindString(strings.TrimSpace(result.GetOutput()))
	if bootID == "" {
		return finishRestart(ctx, j.Deps, server, state, "failed", "The server's boot identity could not be read. Check the server connection and try again.")
	}
	next := time.Now().UTC().Add(10 * time.Second)
	updated := j.Deps.DB.WithContext(ctx).Model(&models.ServerRestart{}).
		Where("server_id = ? AND operation_id = ? AND status = ?", server.ID, state.OperationID, "restarting").
		Updates(map[string]any{"boot_id": bootID, "status": "verifying", "check_after": next, "message": "Restart requested. Waiting for the server to return."})
	if updated.Error != nil || updated.RowsAffected == 0 {
		return updated.Error
	}
	state.BootID = bootID
	restartEvent(j.Deps, server)

	result, err = runRestartTask(ctx, j.Deps, server, "Restart server once", restartGuard+"systemctl reboot")
	// A transport error is ambiguous: the server may already be shutting
	// down. Keep verifying it; only a reachable, changed boot ID proves success.
	if err == nil && result != nil && !result.IsSuccessful() && result.GetExitCode() >= 0 && result.Error == nil && result.TaskResult != nil && result.TaskResult.Error == nil && !result.TaskResult.TimedOut {
		message := "The server rejected the restart command. Check the server console and try again."
		if result.GetExitCode() == 73 {
			message = "Security updates are running. Wait for them to finish, then restart the server."
		}
		state.BootID = ""
		return finishRestart(ctx, j.Deps, server, state, "failed", message)
	}
	activity.RecordWithLogPtr(ctx, "server", "restart_requested", j.Payload.UserID, server, "Server restart was requested; awaiting boot confirmation")
	j.Deps.BroadcastServerEvent(server, "server.rebooting", map[string]any{"server_id": server.ID})
	return scheduleRestartCheck(j.Deps, j.Payload, next)
}

func NewTrackedRebootServerTask(serverID, operationID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeRebootServer, RebootServerPayload{ServerID: serverID, OperationID: operationID, UserID: userID},
		pkgjobs.Dedup("reboot", serverID, operationID), asynq.MaxRetry(0), asynq.Timeout(time.Minute))
}

func runRestartTask(ctx context.Context, deps *JobDeps, server *models.Server, name, script string) (*tasks.TaskRunnerResult, error) {
	return tasks.NewTaskRunner(server, taskrunner.NewScriptTask(name, script, 15*time.Second)).
		WithDispatcher(deps.Dispatcher).WithLogger(deps.Logger).AsRoot().Run(ctx)
}

func readRestart(ctx context.Context, deps *JobDeps, payload RebootServerPayload) (*models.Server, models.ServerRestart, error) {
	var state models.ServerRestart
	err := deps.DB.WithContext(ctx).First(&state, "server_id = ? AND operation_id = ?", payload.ServerID, payload.OperationID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, state, nil
	}
	if err != nil {
		return nil, state, err
	}
	server, err := deps.Repos.Server().FindByID(ctx, payload.ServerID)
	if fiberutil.IsNotFound(err) {
		return nil, state, nil
	}
	return server, state, err
}

func restartEvent(deps *JobDeps, server *models.Server) {
	deps.BroadcastServerEvent(server, "server.reboot.updated", map[string]any{"team_id": server.TeamID})
}

func finishRestart(ctx context.Context, deps *JobDeps, server *models.Server, state models.ServerRestart, status, message string) error {
	changed := false
	err := deps.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&models.ServerRestart{}).
			Where("server_id = ? AND operation_id = ? AND status IN ?", server.ID, state.OperationID, []string{"queued", "restarting", "verifying"}).
			Updates(map[string]any{"status": status, "message": message, "completed_at": time.Now().UTC(), "check_after": nil, "boot_id": state.BootID})
		if result.Error != nil {
			return result.Error
		}
		changed = result.RowsAffected != 0
		if changed && status == "succeeded" {
			return tx.Model(&models.Server{}).Where("id = ?", server.ID).Update("connected", true).Error
		}
		return nil
	})
	if err != nil {
		return err
	}
	if changed {
		restartEvent(deps, server)
		deps.BroadcastServerEvent(server, "server.system_updates.updated", map[string]any{"team_id": server.TeamID})
		if status == "succeeded" {
			deps.BroadcastServerEvent(server, "server.updated", map[string]any{"team_id": server.TeamID, "id": server.ID, "connected": true})
			activity.RecordWithLogPtr(ctx, "server", "restarted", nil, server, "Server restart confirmed by a new boot identity and successful connection")
		}
	}
	return nil
}

func scheduleRestartCheck(deps *JobDeps, payload RebootServerPayload, when time.Time) error {
	return deps.Dispatch(TypeVerifyServerRestart, payload, asynq.ProcessAt(when), asynq.MaxRetry(0), asynq.Timeout(30*time.Second))
}

type VerifyServerRestartJob struct {
	Deps    *JobDeps
	Payload RebootServerPayload
}

func NewVerifyServerRestartJob(p RebootServerPayload) pkgjobs.Handler {
	return &VerifyServerRestartJob{Deps: deps, Payload: p}
}

func (j *VerifyServerRestartJob) Handle(ctx context.Context) error {
	server, state, err := readRestart(ctx, j.Deps, j.Payload)
	if err != nil || server == nil || !state.Active() {
		return err
	}
	if state.DeadlineAt == nil || time.Now().After(*state.DeadlineAt) {
		return finishRestart(ctx, j.Deps, server, state, "failed", restartTimeoutMessage)
	}
	if state.Status != "verifying" || state.BootID == "" {
		return nil
	}
	now := time.Now().UTC()
	// Lease probes to avoid duplicate polling chains when a delayed job and
	// the scheduler watchdog both recover the same operation.
	lease := j.Deps.DB.WithContext(ctx).Model(&models.ServerRestart{}).
		Where("server_id = ? AND operation_id = ? AND status = ? AND (check_after IS NULL OR check_after <= ?)", server.ID, state.OperationID, "verifying", now).
		Update("check_after", now.Add(30*time.Second))
	if lease.Error != nil || lease.RowsAffected == 0 {
		return lease.Error
	}
	result, probeErr := runRestartTask(ctx, j.Deps, server, "Verify server restart", "cat /proc/sys/kernel/random/boot_id")
	if probeErr == nil && result != nil && result.IsSuccessful() {
		currentBoot := restartBootID.FindString(strings.TrimSpace(result.GetOutput()))
		if currentBoot != "" && currentBoot != state.BootID {
			return finishRestart(ctx, j.Deps, server, state, "succeeded", "The server restarted successfully and is reachable.")
		}
	}
	next := time.Now().UTC().Add(10 * time.Second)
	if err := j.Deps.DB.WithContext(ctx).Model(&models.ServerRestart{}).
		Where("server_id = ? AND operation_id = ? AND status = ?", server.ID, state.OperationID, "verifying").Update("check_after", next).Error; err != nil {
		return err
	}
	return scheduleRestartCheck(j.Deps, j.Payload, next)
}

type PollServerRestartsJob struct{ Deps *JobDeps }

func (j *PollServerRestartsJob) Handle(ctx context.Context) error {
	var states []models.ServerRestart
	now := time.Now().UTC()
	if err := j.Deps.DB.WithContext(ctx).Where("status IN ?", []string{"queued", "restarting", "verifying"}).Find(&states).Error; err != nil {
		return err
	}
	for _, state := range states {
		payload := RebootServerPayload{ServerID: state.ServerID, OperationID: state.OperationID}
		if state.DeadlineAt == nil || now.After(*state.DeadlineAt) {
			server, _, err := readRestart(ctx, j.Deps, payload)
			if err != nil {
				return err
			}
			if server != nil {
				if err := finishRestart(ctx, j.Deps, server, state, "failed", restartTimeoutMessage); err != nil {
					return err
				}
			}
		} else if state.Status == "verifying" && (state.CheckAfter == nil || !state.CheckAfter.After(now)) {
			if err := scheduleRestartCheck(j.Deps, payload, now); err != nil {
				return fmt.Errorf("schedule restart confirmation: %w", err)
			}
		}
	}
	return nil
}

func NewPollServerRestartsTask() (*asynq.Task, error) {
	return pkgjobs.Task(TypePollServerRestarts, struct{}{}, asynq.Unique(time.Minute))
}

func NewVerifyServerRestartTask(serverID, operationID string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeVerifyServerRestart, RebootServerPayload{ServerID: serverID, OperationID: operationID}, asynq.MaxRetry(0), asynq.Timeout(30*time.Second))
}
