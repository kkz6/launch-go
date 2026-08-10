package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	backupmodels "github.com/kkz6/launch-go/internal/modules/backup/models"
	backuptypes "github.com/kkz6/launch-go/internal/modules/backup/types"
	"github.com/kkz6/launch-go/internal/modules/docker/models"
	dockernotifications "github.com/kkz6/launch-go/internal/modules/docker/notifications"
	"github.com/kkz6/launch-go/internal/modules/docker/tasks"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner/markers"
)

// TypeRunBackup is the task type for database backups.
const TypeRunBackup = "docker:run_backup"

// RunBackupPayload identifies a database backup run.
type RunBackupPayload struct {
	BackupID   string `json:"backup_id"`
	DatabaseID string `json:"database_id"`
	ProjectID  string `json:"project_id"`
	ServerID   string `json:"server_id"`
	TeamID     string `json:"team_id"`
	RunID      string `json:"run_id,omitempty"`
	Source     string `json:"source,omitempty"`
}

// RunBackupJob is the asynq handler.
type RunBackupJob struct {
	Deps    *JobDeps
	Payload RunBackupPayload

	runID        string
	taskID       string
	finalAttempt func(context.Context, error) bool
}

func NewRunBackupJob(p RunBackupPayload) pkgjobs.Handler {
	return &RunBackupJob{Deps: deps, Payload: p}
}

// Handle executes a database backup.
func (j *RunBackupJob) Handle(ctx context.Context) error {
	var run *models.DatabaseBackupRun
	if j.Payload.RunID != "" {
		j.runID = j.Payload.RunID
		retryCount, hasRetryCount := asynq.GetRetryCount(ctx)
		allowRunning := hasRetryCount && retryCount > 0
		claimedRun, claimed, err := j.Deps.Repos.BackupRun().ClaimTriggeredForBackup(
			ctx,
			j.Payload.RunID,
			j.Payload.BackupID,
			j.Payload.TeamID,
			time.Now().UTC(),
			allowRunning,
		)
		if err != nil {
			return fmt.Errorf("claim pre-created backup run: %w", err)
		}
		if !claimed {
			j.Deps.Logger.Warn().
				Str("run_id", j.Payload.RunID).
				Str("backup_id", j.Payload.BackupID).
				Msg("manual backup run is out of scope, terminal, or already claimed; skipping")
			return nil
		}
		run = claimedRun
	}

	backup, err := j.Deps.Repos.Backup().FindByDatabase(ctx, j.Payload.DatabaseID)
	if err != nil {
		return fmt.Errorf("find backup: %w", err)
	}
	if backup.ID != j.Payload.BackupID {
		j.Deps.Logger.Warn().
			Str("payload_backup_id", j.Payload.BackupID).
			Str("current_backup_id", backup.ID).
			Msg("scheduled backup superseded; skipping")
		if j.Payload.RunID != "" {
			_, persistErr := j.recordFailure(
				ctx,
				j.Payload.BackupID,
				j.Payload.RunID,
				"backup configuration changed or was removed before the worker started",
			)
			if persistErr != nil {
				return fmt.Errorf("persist superseded backup failure: %w", persistErr)
			}
		}
		return nil
	}
	if backup.TeamID != j.Payload.TeamID {
		return fmt.Errorf("backup team does not match queued run")
	}
	if !backup.Enabled && j.Payload.Source != "manual" {
		return nil
	}

	db, err := j.Deps.Repos.Database().FindByIDAndTeamServer(
		ctx, j.Payload.DatabaseID, j.Payload.TeamID, j.Payload.ServerID,
	)
	if err != nil {
		return fmt.Errorf("find database: %w", err)
	}
	project, err := j.Deps.Repos.Project().FindByIDAndTeamServer(
		ctx, j.Payload.ProjectID, j.Payload.TeamID, j.Payload.ServerID,
	)
	if err != nil {
		return fmt.Errorf("find project: %w", err)
	}
	server, err := j.Deps.ServerRepos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("find server: %w", err)
	}

	s3Creds, err := j.loadProviderS3Creds(ctx, backup.StorageProviderID, backup.TeamID)
	if err != nil {
		persisted, persistErr := j.recordFailure(ctx, backup.ID, j.runID, err.Error())
		if persistErr != nil {
			return fmt.Errorf("persist storage-configuration backup failure: %w", persistErr)
		}
		if persisted {
			j.dispatchFailureNotification(ctx, backup, db, server, err.Error())
		}
		return nil
	}
	dbCreds, err := decodeJobCredentials(db.Credentials)
	if err != nil {
		const message = "database credentials are missing or corrupt"
		persisted, persistErr := j.recordFailure(ctx, backup.ID, j.runID, message)
		if persistErr != nil {
			return fmt.Errorf("persist credential backup failure: %w", persistErr)
		}
		if persisted {
			j.dispatchFailureNotification(ctx, backup, db, server, message)
		}
		return nil
	}

	if run == nil {
		now := time.Now().UTC()
		run = &models.DatabaseBackupRun{
			BackupID:  backup.ID,
			Status:    "running",
			StartedAt: &now,
		}
		if err := j.Deps.Repos.BackupRun().Create(ctx, run); err != nil {
			return fmt.Errorf("create run row: %w", err)
		}
		j.runID = run.ID
		j.Payload.RunID = run.ID
	}

	j.Deps.BroadcastToTeam(j.Payload.TeamID, "docker.database.backup.run.started", map[string]any{
		"database_id": db.ID,
		"project_id":  j.Payload.ProjectID,
		"backup_id":   backup.ID,
		"run_id":      run.ID,
		"server_id":   server.ID,
		"team_id":     j.Payload.TeamID,
		"source":      j.Payload.Source,
	})

	cfg := tasks.BackupRunConfig{
		RunID: run.ID,
		ContainerName: tasks.DatabaseContainerName(
			tasks.SlugFromName(project.Name),
			tasks.SlugFromName(db.Name),
		),
		Engine:         db.Engine,
		Username:       dbCreds.Username,
		Password:       dbCreds.Password,
		Database:       backup.EffectiveDatabaseName(dbCreds.Database),
		Endpoint:       s3Creds.Endpoint,
		Region:         s3Creds.Region,
		Bucket:         s3Creds.Bucket,
		PathPrefix:     tasks.BackupObjectPath(backup.Path, s3Creds.Path),
		AccessKey:      s3Creds.Key,
		SecretKey:      s3Creds.Secret,
		ForcePathStyle: s3Creds.ForcePathStyle,
	}

	runID := run.ID
	markerHandler := j.progressMarkerHandler(db.ID, backup.ID, runID, server.ID)

	task := tasks.RunBackup(cfg)
	result, runErr := j.Deps.RunTask(server, task).AsRoot().
		TrackInDB().
		WithMarkerHandler(markerHandler).
		OnTaskCreated(func(taskID string) {
			j.taskID = taskID
			attached, err := j.Deps.Repos.BackupRun().AttachTaskForBackup(
				ctx,
				run.ID,
				backup.ID,
				j.Payload.TeamID,
				taskID,
			)
			if err != nil {
				j.Deps.Logger.Error().Err(err).Str("run_id", run.ID).Msg("failed to attach task to backup run")
				return
			}
			if !attached {
				j.Deps.Logger.Warn().Str("run_id", run.ID).
					Msg("backup run was no longer active when its task was created")
				return
			}
			j.Deps.BroadcastToTeam(j.Payload.TeamID, "docker.database.backup.run.started", map[string]any{
				"database_id": db.ID,
				"project_id":  j.Payload.ProjectID,
				"backup_id":   backup.ID,
				"run_id":      run.ID,
				"server_id":   server.ID,
				"team_id":     j.Payload.TeamID,
				"task_id":     taskID,
				"source":      j.Payload.Source,
			})
		}).
		Dispatch(ctx)

	finishedAt := time.Now().UTC()
	output := ""
	if result != nil {
		output = result.GetOutput()
	}

	if runErr != nil || (result != nil && !result.IsSuccessful()) {
		errMsg := ""
		if runErr != nil {
			errMsg = runErr.Error()
		}
		if output != "" {
			if errMsg != "" {
				errMsg += "\n"
			}
			errMsg += truncateForRun(output, 4000)
		}
		errMsg = truncateForRun(errMsg, 4000)
		if errMsg == "" {
			errMsg = "database backup failed without an error message"
		}
		persisted, persistErr := j.recordFailure(ctx, backup.ID, run.ID, errMsg)
		if persistErr != nil {
			return fmt.Errorf("persist backup failure state: %w", persistErr)
		}
		if persisted {
			j.dispatchFailureNotification(ctx, backup, db, server, errMsg)
		}
		return nil
	}

	objectKey, sizeBytes := tasks.ParseRunMarkers(output)
	updates := map[string]any{
		"status":      "success",
		"finished_at": finishedAt,
	}
	if objectKey != "" {
		updates["object_key"] = objectKey
	}
	if sizeBytes > 0 {
		updates["size_bytes"] = sizeBytes
	}
	if j.taskID != "" {
		updates["task_id"] = j.taskID
	}
	transitioned, err := j.Deps.Repos.BackupRun().MarkTerminalForBackup(
		ctx,
		run.ID,
		backup.ID,
		j.Payload.TeamID,
		updates,
	)
	if err != nil {
		return fmt.Errorf("persist backup success state: %w", err)
	}
	if !transitioned {
		j.Deps.Logger.Warn().Str("run_id", run.ID).
			Msg("backup run was no longer active when task succeeded; suppressing terminal event")
		return nil
	}

	run.Status = "success"
	run.FinishedAt = &finishedAt
	if objectKey != "" {
		run.ObjectKey = &objectKey
	}
	if sizeBytes > 0 {
		run.SizeBytes = &sizeBytes
	}
	if j.taskID != "" {
		run.TaskID = &j.taskID
	}

	successPayload := map[string]any{
		"database_id": db.ID,
		"project_id":  j.Payload.ProjectID,
		"backup_id":   backup.ID,
		"run_id":      run.ID,
		"server_id":   server.ID,
		"team_id":     j.Payload.TeamID,
		"object_key":  objectKey,
		"size_bytes":  sizeBytes,
		"source":      j.Payload.Source,
	}
	if j.taskID != "" {
		successPayload["task_id"] = j.taskID
	}
	j.Deps.BroadcastToTeam(j.Payload.TeamID, "docker.database.backup.run.succeeded", successPayload)

	if backup.Retention > 0 {
		if err := j.pruneRunsAndObjects(ctx, server, backup, s3Creds); err != nil {
			j.Deps.Logger.Warn().Err(err).
				Str("backup_id", backup.ID).
				Msg("scheduled retention prune failed (run still succeeded)")
		}
	}

	j.dispatchSuccessNotification(ctx, backup, db, server, run, objectKey)
	return nil
}

func (j *RunBackupJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("backup_id", j.Payload.BackupID).
		Str("database_id", j.Payload.DatabaseID).
		Str("source", j.Payload.Source).
		Msg("run backup job failed at the framework level")
	if !j.isFinalAttempt(ctx, err) {
		return
	}

	message := "backup worker failed"
	if err != nil {
		message += ": " + err.Error()
	}
	runID := j.runID
	if runID == "" {
		runID = j.Payload.RunID
	}
	if _, persistErr := j.recordFailure(ctx, j.Payload.BackupID, runID, message); persistErr != nil {
		j.Deps.Logger.Error().Err(persistErr).
			Str("backup_id", j.Payload.BackupID).
			Str("run_id", runID).
			Msg("failed to persist framework-level backup failure")
	}
}

func (j *RunBackupJob) isFinalAttempt(ctx context.Context, err error) bool {
	if j.finalAttempt != nil {
		return j.finalAttempt(ctx, err)
	}
	return pkgjobs.IsFinalAttempt(ctx, err)
}

func (j *RunBackupJob) recordFailure(
	ctx context.Context,
	backupID, runID, msg string,
) (bool, error) {
	now := time.Now().UTC()
	msg = truncateForRun(msg, 4000)
	if msg == "" {
		msg = "database backup failed without an error message"
	}
	if runID == "" {
		runID = j.runID
	}
	shouldBroadcast := false
	if runID == "" {
		run := &models.DatabaseBackupRun{
			BackupID:   backupID,
			Status:     "failed",
			StartedAt:  &now,
			FinishedAt: &now,
			Error:      strPtr(msg),
		}
		if err := j.Deps.Repos.BackupRun().Create(ctx, run); err != nil {
			return false, err
		}
		runID = run.ID
		j.runID = run.ID
		j.Payload.RunID = run.ID
		shouldBroadcast = true
	} else {
		updates := map[string]any{
			"status":      "failed",
			"finished_at": now,
			"error":       msg,
		}
		if j.taskID != "" {
			updates["task_id"] = j.taskID
		}
		updated, err := j.Deps.Repos.BackupRun().MarkTerminalForBackup(
			ctx,
			runID,
			backupID,
			j.Payload.TeamID,
			updates,
		)
		if err != nil {
			return false, err
		}
		shouldBroadcast = updated
	}
	if !shouldBroadcast {
		return false, nil
	}
	failurePayload := map[string]any{
		"database_id": j.Payload.DatabaseID,
		"project_id":  j.Payload.ProjectID,
		"backup_id":   backupID,
		"run_id":      runID,
		"server_id":   j.Payload.ServerID,
		"team_id":     j.Payload.TeamID,
		"source":      j.Payload.Source,
		"error":       msg,
	}
	if j.taskID != "" {
		failurePayload["task_id"] = j.taskID
	}
	j.Deps.BroadcastToTeam(j.Payload.TeamID, "docker.database.backup.run.failed", failurePayload)
	return true, nil
}

func backupProgressPayload(
	payload RunBackupPayload,
	databaseID, backupID, runID, serverID string,
	marker *markers.Marker,
) map[string]any {
	return map[string]any{
		"database_id": databaseID,
		"project_id":  payload.ProjectID,
		"backup_id":   backupID,
		"run_id":      runID,
		"server_id":   serverID,
		"team_id":     payload.TeamID,
		"source":      payload.Source,
		"type":        marker.Type,
		"value":       marker.Value,
	}
}

func (j *RunBackupJob) progressMarkerHandler(
	databaseID, backupID, runID, serverID string,
) taskrunner.MarkerHandler {
	return taskrunner.MarkerHandlerFunc(func(_ context.Context, _ string, marker *markers.Marker) error {
		payload := backupProgressPayload(j.Payload, databaseID, backupID, runID, serverID, marker)
		j.Deps.BroadcastToTeam(j.Payload.TeamID, "docker.database.backup.run.progress", payload)
		return nil
	})
}

func NewRunBackupTask(
	backupID, databaseID, projectID, serverID, teamID, runID, source string,
) (*asynq.Task, error) {
	dedupKey := runID
	if dedupKey == "" {
		dedupKey = time.Now().UTC().Format("2006-01-02T15:04")
	}
	return pkgjobs.TaskWithID(TypeRunBackup, RunBackupPayload{
		BackupID:   backupID,
		DatabaseID: databaseID,
		ProjectID:  projectID,
		ServerID:   serverID,
		TeamID:     teamID,
		RunID:      runID,
		Source:     source,
	}, pkgjobs.Dedup("docker-run-backup", backupID, dedupKey))
}

func (j *RunBackupJob) loadProviderS3Creds(
	ctx context.Context, providerID uint64, teamID string,
) (backupmodels.S3Credentials, error) {
	if j.Deps.BackupRepos == nil {
		return backupmodels.S3Credentials{},
			fmt.Errorf("storage providers registry is not wired into docker worker")
	}
	p, err := j.Deps.BackupRepos.StorageProvider().FindStorageProviderByIDAndTeam(
		ctx,
		providerID,
		teamID,
	)
	if err != nil {
		return backupmodels.S3Credentials{}, fmt.Errorf(
			"storage provider %d not found", providerID,
		)
	}
	if p.Provider != backuptypes.StorageDriverS3 {
		return backupmodels.S3Credentials{}, fmt.Errorf(
			"storage provider %d is not an S3 driver", providerID,
		)
	}
	credentials := p.GetCredentials()
	raw, _ := json.Marshal(credentials)
	var c backupmodels.S3Credentials
	if err := json.Unmarshal(raw, &c); err != nil {
		return backupmodels.S3Credentials{}, fmt.Errorf("decode S3 credentials: %w", err)
	}
	c.ApplyLegacyDefaults(credentials)
	if c.Bucket == "" || c.Key == "" || c.Secret == "" {
		return backupmodels.S3Credentials{},
			fmt.Errorf("storage provider %d is missing S3 credentials", providerID)
	}
	return c, nil
}

func strPtr(s string) *string { return &s }

func truncateForRun(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func (j *RunBackupJob) pruneRunsAndObjects(
	ctx context.Context,
	server *servermodels.Server,
	backup *models.DatabaseBackup,
	s3Creds backupmodels.S3Credentials,
) error {
	if backup.Retention <= 0 {
		return nil
	}
	stale, err := j.Deps.Repos.BackupRun().ListStaleForRetention(ctx, backup.ID, backup.Retention)
	if err != nil {
		return err
	}
	if len(stale) == 0 {
		return nil
	}

	objectKeys := make([]string, 0, len(stale))
	for i := range stale {
		if stale[i].ObjectKey != nil && *stale[i].ObjectKey != "" {
			objectKeys = append(objectKeys, *stale[i].ObjectKey)
		}
	}

	if len(objectKeys) > 0 {
		cfg := tasks.PruneBackupObjectsConfig{
			ObjectKeys:     objectKeys,
			Endpoint:       s3Creds.Endpoint,
			Region:         s3Creds.Region,
			Bucket:         s3Creds.Bucket,
			AccessKey:      s3Creds.Key,
			SecretKey:      s3Creds.Secret,
			ForcePathStyle: s3Creds.ForcePathStyle,
		}
		task := tasks.PruneBackupObjects(cfg)
		if _, err := j.Deps.RunTask(server, task).AsRoot().Dispatch(ctx); err != nil {
			return fmt.Errorf("prune remote objects: %w", err)
		}
	}

	for i := range stale {
		if err := j.Deps.Repos.BackupRun().Delete(ctx, stale[i].ID); err != nil {
			j.Deps.Logger.Warn().Err(err).Str("run_id", stale[i].ID).Msg("failed to prune stale backup run")
		}
	}
	return nil
}

func (j *RunBackupJob) dispatchSuccessNotification(
	ctx context.Context,
	backup *models.DatabaseBackup,
	db *models.Database,
	server *servermodels.Server,
	run *models.DatabaseBackupRun,
	objectKey string,
) {
	if !backup.NotifyOnSuccess {
		return
	}
	if j.Deps.TaskRunnerDeps == nil {
		return
	}
	notifier := j.Deps.TaskRunnerDeps.Notifier
	if notifier == nil {
		return
	}

	projectName := j.lookupProjectName(ctx, backup.TeamID, j.Payload.ProjectID, j.Payload.ServerID)
	providerLabel := j.lookupStorageProviderLabel(ctx, backup.StorageProviderID, backup.TeamID)

	notif := dockernotifications.
		NewDatabaseBackupSucceededNotification(db.Name, projectName, server.Name, string(db.Engine)).
		WithObjectKey(objectKey).
		WithStorageProvider(providerLabel).
		WithSource(notificationSource(j.Payload.Source))
	if run.SizeBytes != nil {
		notif.WithSizeBytes(*run.SizeBytes)
	}

	if err := notifier.SendToTeam(ctx, backup.TeamID, notif); err != nil {
		j.Deps.Logger.Warn().Err(err).
			Str("backup_id", backup.ID).
			Msg("failed to send backup success notification")
	}
}

func (j *RunBackupJob) dispatchFailureNotification(
	ctx context.Context,
	backup *models.DatabaseBackup,
	db *models.Database,
	server *servermodels.Server,
	errOutput string,
) {
	if !backup.NotifyOnFailure {
		return
	}
	if j.Deps.TaskRunnerDeps == nil {
		return
	}
	notifier := j.Deps.TaskRunnerDeps.Notifier
	if notifier == nil {
		return
	}

	projectName := j.lookupProjectName(ctx, backup.TeamID, j.Payload.ProjectID, j.Payload.ServerID)
	providerLabel := j.lookupStorageProviderLabel(ctx, backup.StorageProviderID, backup.TeamID)

	notif := dockernotifications.
		NewDatabaseBackupFailedNotification(db.Name, projectName, server.Name, string(db.Engine)).
		WithError(errOutput).
		WithStorageProvider(providerLabel).
		WithSource(notificationSource(j.Payload.Source))

	if err := notifier.SendToTeam(ctx, backup.TeamID, notif); err != nil {
		j.Deps.Logger.Warn().Err(err).
			Str("backup_id", backup.ID).
			Msg("failed to send backup failure notification")
	}
}

func (j *RunBackupJob) lookupProjectName(ctx context.Context, teamID, projectID, serverID string) string {
	if projectID == "" {
		return ""
	}
	p, err := j.Deps.Repos.Project().FindByIDAndTeamServer(ctx, projectID, teamID, serverID)
	if err != nil || p == nil {
		return ""
	}
	return p.Name
}

func (j *RunBackupJob) lookupStorageProviderLabel(ctx context.Context, providerID uint64, teamID string) string {
	if j.Deps.BackupRepos == nil {
		return ""
	}
	p, err := j.Deps.BackupRepos.StorageProvider().FindStorageProviderByIDAndTeam(
		ctx,
		providerID,
		teamID,
	)
	if err != nil || p == nil {
		return ""
	}
	if p.Label == nil {
		return ""
	}
	return *p.Label
}

func notificationSource(s string) string {
	if s == "" {
		return "schedule"
	}
	return s
}
