package jobs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypePatchPhpVersion = "server:patch_php_version"

type PatchPhpVersionPayload struct {
	ServerID       string              `json:"server_id"`
	ServiceID      string              `json:"service_id"`
	PreviousStatus types.ServiceStatus `json:"previous_status"`
	UserID         *string             `json:"user_id,omitempty"`
	TaskID         string              `json:"task_id,omitempty"`
}

type PatchPhpVersionJob struct {
	Deps    *JobDeps
	Payload PatchPhpVersionPayload

	server          *models.Server
	service         *models.InstalledService
	taskID          string
	versionSeries   string
	previousStatus  types.ServiceStatus
	validated       bool
	failureRecorded bool
}

func NewPatchPhpVersionJob(payload PatchPhpVersionPayload) pkgjobs.Handler {
	return &PatchPhpVersionJob{Deps: deps, Payload: payload}
}

func (j *PatchPhpVersionJob) Timeout() time.Duration {
	return 20 * time.Minute
}

func (j *PatchPhpVersionJob) Handle(ctx context.Context) error {
	var err error
	j.taskID = j.Payload.TaskID

	j.service, err = j.Deps.Repos.Service().FindByID(ctx, j.Payload.ServiceID)
	if err != nil {
		return fmt.Errorf("find PHP service: %w", err)
	}

	if j.service.ServerID != j.Payload.ServerID {
		return fmt.Errorf("PHP service does not belong to server")
	}

	software := j.service.GetSoftware()
	if j.service.Type != types.ServiceTypePhp || !software.IsPhp() {
		return fmt.Errorf("service is not a PHP installation")
	}

	if j.service.Status != types.ServiceStatusUpdating {
		return fmt.Errorf("PHP patch was not reserved")
	}
	if !j.Payload.PreviousStatus.IsActive() {
		return fmt.Errorf("PHP patch has an invalid previous service status")
	}
	j.previousStatus = j.Payload.PreviousStatus
	j.validated = true

	j.server, err = j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return j.recordFailure(ctx, fmt.Errorf("find server: %w", err))
	}

	j.versionSeries = software.GetVersion()
	if err := j.setPatchState(ctx, types.ServiceStatusUpdating, "running", "", ""); err != nil {
		return fmt.Errorf("mark PHP service updating: %w", err)
	}
	j.broadcastStatus(types.ServiceStatusUpdating)

	runner := j.Deps.RunTask(j.server, tasks.PatchPHP(software)).
		AsRoot().
		TrackInDB().
		OnTaskCreated(func(taskID string) {
			j.taskID = taskID
			j.service.TaskID = &taskID
			if updateErr := j.Deps.Repos.Service().UpdateFields(
				context.WithoutCancel(ctx),
				j.service.ID,
				map[string]any{"task_id": taskID},
			); updateErr != nil {
				j.Deps.Logger.Error().
					Err(updateErr).
					Str("service_id", j.service.ID).
					Str("task_id", taskID).
					Msg("failed to associate PHP patch task")
			}
			j.broadcastPatch("running", "", "")
		})
	if j.Payload.TaskID != "" {
		runner.UsePreparedTask(j.Payload.TaskID)
		j.broadcastPatch("running", "", "")
	}
	result, err := runner.Dispatch(ctx)
	if err != nil {
		return j.recordFailure(ctx, fmt.Errorf("run PHP patch: %w", err))
	}
	if result == nil {
		return j.recordFailure(ctx, fmt.Errorf("run PHP patch: task returned no result"))
	}
	if !result.IsSuccessful() {
		return j.recordFailure(ctx, fmt.Errorf("patch PHP %s: %s", j.versionSeries, result.GetOutput()))
	}

	detectedVersion := tasks.ParsePatchedPHPVersion(result.GetOutput())
	if !isVersionInSeries(detectedVersion, j.versionSeries) {
		return j.recordFailure(ctx, fmt.Errorf(
			"verify patched PHP version: expected %s.x, detected %q",
			j.versionSeries,
			detectedVersion,
		))
	}

	terminalCtx, cancelTerminal := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancelTerminal()
	if err := j.setPatchState(
		terminalCtx,
		types.ServiceStatusRunning,
		"finished",
		"",
		detectedVersion,
	); err != nil {
		return j.recordFailure(ctx, fmt.Errorf("persist patched PHP version: %w", err))
	}

	j.service.Version = detectedVersion
	j.service.Status = types.ServiceStatusRunning
	j.broadcastStatus(types.ServiceStatusRunning)
	j.broadcastPatch("finished", "", detectedVersion)

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Str("service_id", j.service.ID).
		Str("version", detectedVersion).
		Msg("PHP patch completed")

	return nil
}

func (j *PatchPhpVersionJob) setPatchState(
	ctx context.Context,
	serviceStatus types.ServiceStatus,
	patchStatus,
	patchError,
	version string,
) error {
	typeData := cloneTypeData(j.service.TypeData)
	typeData["patch_status"] = patchStatus

	now := time.Now().UTC().Format(time.RFC3339)
	if patchStatus == "running" {
		typeData["patch_started_at"] = now
		delete(typeData, "patch_finished_at")
		delete(typeData, "patch_error")
	} else {
		typeData["patch_finished_at"] = now
	}

	if patchError != "" {
		typeData["patch_error"] = patchError
	}

	fields := map[string]any{
		"status":    serviceStatus,
		"type_data": typeData,
	}
	if version != "" {
		fields["version"] = version
	}

	if err := j.Deps.Repos.Service().UpdateFields(ctx, j.service.ID, fields); err != nil {
		return err
	}

	j.service.TypeData = typeData
	j.service.Status = serviceStatus
	return nil
}

func (j *PatchPhpVersionJob) recordFailure(ctx context.Context, patchErr error) error {
	failureMessage := boundedPatchError(patchErr.Error(), 4096)
	cleanupCtx, cancelCleanup := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancelCleanup()
	if j.Deps.TaskRunnerDeps != nil {
		if err := j.Deps.TaskRunnerDeps.FailPreparedTask(cleanupCtx, j.Payload.TaskID, patchErr); err != nil {
			j.Deps.Logger.Error().Err(err).
				Str("task_id", j.Payload.TaskID).
				Msg("failed to persist PHP patch action failure")
		}
	}
	if j.service != nil {
		if err := j.setPatchState(
			cleanupCtx,
			j.previousStatus,
			"failed",
			failureMessage,
			"",
		); err != nil {
			j.Deps.Logger.Error().Err(err).
				Str("service_id", j.Payload.ServiceID).
				Msg("failed to persist PHP patch failure")
		} else {
			j.failureRecorded = true
		}
	}

	if j.failureRecorded {
		j.broadcastStatus(j.previousStatus)
	}
	j.broadcastPatch("failed", failureMessage, "")
	return patchErr
}

func (j *PatchPhpVersionJob) broadcastStatus(serviceStatus types.ServiceStatus) {
	if j.server == nil || j.service == nil {
		return
	}

	j.Deps.BroadcastServerEvent(j.server, "service.status_changed", map[string]any{
		"server_id":  j.server.ID,
		"service_id": j.service.ID,
		"status":     serviceStatus.String(),
	})
}

func (j *PatchPhpVersionJob) broadcastPatch(patchStatus, output, version string) {
	if j.server == nil || j.service == nil {
		return
	}

	data := map[string]any{
		"server_id":  j.server.ID,
		"service_id": j.service.ID,
		"task_id":    j.taskID,
		"status":     patchStatus,
		"version":    j.versionSeries,
	}
	if output != "" {
		data["output"] = output
	}
	if version != "" {
		data["installed_version"] = version
	}

	j.Deps.BroadcastServerEvent(j.server, "php.patch", data)
}

func (j *PatchPhpVersionJob) Failed(ctx context.Context, err error) {
	if j.validated && !j.failureRecorded {
		_ = j.recordFailure(ctx, err)
	} else if j.Deps.TaskRunnerDeps != nil {
		cleanupCtx, cancelCleanup := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
		defer cancelCleanup()
		_ = j.Deps.TaskRunnerDeps.FailPreparedTask(cleanupCtx, j.Payload.TaskID, err)
	}

	j.Deps.Logger.Error().Err(err).
		Str("server_id", j.Payload.ServerID).
		Str("service_id", j.Payload.ServiceID).
		Msg("failed to patch PHP")
}

func NewPatchPhpVersionTask(
	serverID,
	serviceID string,
	previousStatus types.ServiceStatus,
	userID *string,
	taskIDs ...string,
) (*asynq.Task, error) {
	taskID := ""
	if len(taskIDs) > 0 {
		taskID = taskIDs[0]
	}
	return pkgjobs.Task(
		TypePatchPhpVersion,
		PatchPhpVersionPayload{
			ServerID:       serverID,
			ServiceID:      serviceID,
			PreviousStatus: previousStatus,
			UserID:         userID,
			TaskID:         taskID,
		},
		asynq.Timeout(20*time.Minute),
	)
}

func isVersionInSeries(version, series string) bool {
	return version == series || strings.HasPrefix(version, series+".")
}

func cloneTypeData(source dbtype.JSONMap) dbtype.JSONMap {
	cloned := make(dbtype.JSONMap, len(source)+3)
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func boundedPatchError(message string, limit int) string {
	if limit <= 0 || len(message) <= limit {
		return message
	}
	return message[:limit-1] + "…"
}
