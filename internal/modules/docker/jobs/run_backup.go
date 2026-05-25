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
)

// TypeRunBackup is the asynq task type for executing a single database
// backup. Dispatched by:
//
//   - PollDueBackupsJob (the scheduler poller, every minute)
//   - One day, a "Run now (async)" button on the UI — today the UI calls
//     the synchronous BackupService.RunNow path so the response carries
//     the run row back inline.
//
// Mirrors the work BackupService.RunNow does on the HTTP path, but
// without the user-facing return DTO. Kept here (rather than calling
// into BackupService) because services → jobs is the established import
// direction in this module; reversing it would create a cycle.
const TypeRunBackup = "docker:run_backup"

// RunBackupPayload travels through asynq. IDs only — every other field
// gets re-loaded from the database when the job runs, so a backup
// that's been disabled / had its credentials rotated between dispatch
// and execution picks up the live state.
type RunBackupPayload struct {
	BackupID   string `json:"backup_id"`
	DatabaseID string `json:"database_id"`
	ProjectID  string `json:"project_id"`
	ServerID   string `json:"server_id"`
	TeamID     string `json:"team_id"`
	// Source records who/what triggered this run — "schedule" for the
	// cron poller; empty / "manual" for ad-hoc dispatches. Mostly for
	// debugging via the worker log.
	Source string `json:"source,omitempty"`
}

// RunBackupJob is the asynq handler.
type RunBackupJob struct {
	Deps    *JobDeps
	Payload RunBackupPayload
}

// NewRunBackupJob is the constructor asynq picks up via
// pkgjobs.RegisterTyped — see register.go.
func NewRunBackupJob(p RunBackupPayload) pkgjobs.Handler {
	return &RunBackupJob{Deps: deps, Payload: p}
}

// Handle dumps the database via SSH and uploads to S3. Persistence
// (BackupRun row + broadcast) is identical to BackupService.RunNow on
// the HTTP path.
func (j *RunBackupJob) Handle(ctx context.Context) error {
	backup, err := j.Deps.Repos.Backup().FindByDatabase(ctx, j.Payload.DatabaseID)
	if err != nil {
		return fmt.Errorf("find backup: %w", err)
	}
	if backup.ID != j.Payload.BackupID {
		// Backup was deleted + re-created between dispatch and execution.
		// Treat as a no-op rather than running a backup against a row
		// the user already removed.
		j.Deps.Logger.Warn().
			Str("payload_backup_id", j.Payload.BackupID).
			Str("current_backup_id", backup.ID).
			Msg("scheduled backup superseded; skipping")
		return nil
	}
	if !backup.Enabled {
		// Race: the poller saw enabled=true, user disabled before this
		// job ran. Skip without recording a failed run.
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

	// Resolve credentials from the linked storage_providers row (the
	// docker_database_backups row only carries the FK now). If the
	// provider was deleted out from under us, fail the run with a
	// clean error instead of retrying forever.
	s3Creds, err := j.loadProviderS3Creds(ctx, backup.StorageProviderID, backup.TeamID)
	if err != nil {
		j.recordFailure(ctx, backup.ID, "", err.Error())
		j.dispatchFailureNotification(ctx, backup, db, server, err.Error())
		return nil
	}
	dbCreds, err := decodeJobCredentials(db.Credentials)
	if err != nil {
		j.recordFailure(ctx, backup.ID, "", "database credentials are missing or corrupt")
		j.dispatchFailureNotification(ctx, backup, db, server, "database credentials are missing or corrupt")
		return nil
	}

	now := time.Now().UTC()
	run := &models.DatabaseBackupRun{
		BackupID:  backup.ID,
		Status:    "running",
		StartedAt: &now,
	}
	if err := j.Deps.Repos.BackupRun().Create(ctx, run); err != nil {
		return fmt.Errorf("create run row: %w", err)
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
		Engine:   db.Engine,
		Username: dbCreds.Username,
		Password: dbCreds.Password,
		// Database-name override mirrors the synchronous RunNow path
		// in services/backup_service.go — if the user pointed this
		// backup config at a specific database inside the engine
		// (e.g. one they created manually after provisioning), the
		// scheduled run targets the same name. Empty override falls
		// back to the row's default database. EffectiveDatabaseName
		// centralises the picker so the two paths can't drift.
		Database: backup.EffectiveDatabaseName(dbCreds.Database),
		// All S3 destination fields come from the storage_providers
		// row now. The backup row only owns the optional sub-folder
		// (backup.Path) which we join onto the provider's default
		// path.
		Endpoint:   s3Creds.Endpoint,
		Region:     s3Creds.Region,
		Bucket:     s3Creds.Bucket,
		PathPrefix: tasks.BackupObjectPath(backup.Path, s3Creds.Path),
		AccessKey:  s3Creds.Key,
		SecretKey:  s3Creds.Secret,
	}

	task := tasks.RunBackup(cfg)
	result, runErr := j.Deps.RunTask(server, task).AsRoot().Dispatch(ctx)

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
		_ = j.Deps.Repos.BackupRun().UpdateFields(ctx, run.ID, map[string]any{
			"status":      "failed",
			"finished_at": finishedAt,
			"error":       truncateForRun(errMsg, 4000),
		})
		j.Deps.BroadcastToTeam(j.Payload.TeamID, "docker.database.backup.run.failed", map[string]any{
			"database_id": db.ID,
			"project_id":  j.Payload.ProjectID,
			"backup_id":   backup.ID,
			"run_id":      run.ID,
			"server_id":   server.ID,
			"team_id":     j.Payload.TeamID,
			"source":      j.Payload.Source,
		})
		// Fire the per-config notification (email/slack/etc.) if the
		// user opted in via NotifyOnFailure. Best-effort — a failed
		// notify shouldn't override the recorded failure status.
		j.dispatchFailureNotification(ctx, backup, db, server, errMsg)
		// Return nil so asynq doesn't auto-retry a backup that will fail
		// the same way (bad creds, missing CLI tool). The next cron tick
		// will re-attempt on its natural cadence.
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
	_ = j.Deps.Repos.BackupRun().UpdateFields(ctx, run.ID, updates)

	j.Deps.BroadcastToTeam(j.Payload.TeamID, "docker.database.backup.run.succeeded", map[string]any{
		"database_id": db.ID,
		"project_id":  j.Payload.ProjectID,
		"backup_id":   backup.ID,
		"run_id":      run.ID,
		"server_id":   server.ID,
		"team_id":     j.Payload.TeamID,
		"object_key":  objectKey,
		"size_bytes":  sizeBytes,
		"source":      j.Payload.Source,
	})

	// Honour the retention cap — prune both old run rows AND their
	// remote S3 objects. Previously only the synchronous "Run now"
	// path enforced retention; scheduled runs leaked rows + storage
	// forever. Best-effort: failure here doesn't fail the run because
	// the backup itself already succeeded.
	if backup.Retention > 0 {
		if err := j.pruneRunsAndObjects(ctx, server, backup, s3Creds); err != nil {
			j.Deps.Logger.Warn().Err(err).
				Str("backup_id", backup.ID).
				Msg("scheduled retention prune failed (run still succeeded)")
		}
	}

	// Notifications — only fired AFTER the prune so a user inspecting
	// the bucket from the email link sees the final state, not a
	// transient one with stale objects.
	j.dispatchSuccessNotification(ctx, backup, db, server, run, objectKey)
	return nil
}

// Failed is asynq's framework-level callback (Handle panicked or returned
// an error). User-data failures are recorded inside Handle and return
// nil, so reaching this point means something genuinely transient.
func (j *RunBackupJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("backup_id", j.Payload.BackupID).
		Str("database_id", j.Payload.DatabaseID).
		Str("source", j.Payload.Source).
		Msg("run backup job failed at the framework level")
}

// recordFailure persists a failed run row when we abort before
// dispatching the SSH task — e.g. credentials missing on load.
func (j *RunBackupJob) recordFailure(ctx context.Context, backupID, runID, msg string) {
	now := time.Now().UTC()
	if runID == "" {
		run := &models.DatabaseBackupRun{
			BackupID:   backupID,
			Status:     "failed",
			StartedAt:  &now,
			FinishedAt: &now,
			Error:      strPtr(msg),
		}
		_ = j.Deps.Repos.BackupRun().Create(ctx, run)
		runID = run.ID
	} else {
		_ = j.Deps.Repos.BackupRun().UpdateFields(ctx, runID, map[string]any{
			"status":      "failed",
			"finished_at": now,
			"error":       msg,
		})
	}
	j.Deps.BroadcastToTeam(j.Payload.TeamID, "docker.database.backup.run.failed", map[string]any{
		"database_id": j.Payload.DatabaseID,
		"project_id":  j.Payload.ProjectID,
		"backup_id":   backupID,
		"run_id":      runID,
		"server_id":   j.Payload.ServerID,
		"team_id":     j.Payload.TeamID,
		"source":      j.Payload.Source,
		"error":       msg,
	})
}

// NewRunBackupTask packages the asynq task. Includes the backup ID +
// the dispatch minute in the dedup key so two scheduler ticks in the
// same minute (e.g. on restart) don't double-fire, but distinct minutes
// each get their own run.
func NewRunBackupTask(
	backupID, databaseID, projectID, serverID, teamID, source string,
) (*asynq.Task, error) {
	minuteKey := time.Now().UTC().Format("2006-01-02T15:04")
	return pkgjobs.TaskWithID(TypeRunBackup, RunBackupPayload{
		BackupID:   backupID,
		DatabaseID: databaseID,
		ProjectID:  projectID,
		ServerID:   serverID,
		TeamID:     teamID,
		Source:     source,
	}, pkgjobs.Dedup("docker-run-backup", backupID, minuteKey))
}

// loadProviderS3Creds resolves the storage_provider FK on a backup row
// into a usable S3Credentials struct. Validates the provider belongs
// to the team (defence-in-depth — the configure-time path already
// checks) and that the provider is S3-flavoured.
//
// Kept on the job receiver so we don't have to thread BackupRepos
// through every helper signature.
func (j *RunBackupJob) loadProviderS3Creds(
	ctx context.Context, providerID uint64, teamID string,
) (backupmodels.S3Credentials, error) {
	if j.Deps.BackupRepos == nil {
		return backupmodels.S3Credentials{},
			fmt.Errorf("storage providers registry is not wired into docker worker")
	}
	p, err := j.Deps.BackupRepos.StorageProvider().FindStorageProviderByID(ctx, providerID)
	if err != nil {
		return backupmodels.S3Credentials{}, fmt.Errorf(
			"storage provider %d not found", providerID,
		)
	}
	if p.TeamID != teamID {
		return backupmodels.S3Credentials{}, fmt.Errorf(
			"storage provider %d does not belong to team %s", providerID, teamID,
		)
	}
	if p.Provider != backuptypes.StorageDriverS3 {
		return backupmodels.S3Credentials{}, fmt.Errorf(
			"storage provider %d is not an S3 driver", providerID,
		)
	}
	raw, err := json.Marshal(p.GetCredentials())
	if err != nil {
		return backupmodels.S3Credentials{}, fmt.Errorf("encode provider credentials: %w", err)
	}
	var c backupmodels.S3Credentials
	if err := json.Unmarshal(raw, &c); err != nil {
		return backupmodels.S3Credentials{}, fmt.Errorf("decode S3 credentials: %w", err)
	}
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

// pruneRunsAndObjects mirrors the service-side helper in
// services/backup_service.go — the prune contract has to hold on
// both the manual and scheduled paths, and the jobs package can't
// import services (cycle). Best-effort: failure here doesn't fail
// the surrounding successful run.
//
// Deletes BOTH the run rows past Retention AND the corresponding
// remote S3 objects so the bucket doesn't accumulate stale dumps.
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
			ObjectKeys: objectKeys,
			Endpoint:   s3Creds.Endpoint,
			Region:     s3Creds.Region,
			Bucket:     s3Creds.Bucket,
			AccessKey:  s3Creds.Key,
			SecretKey:  s3Creds.Secret,
		}
		task := tasks.PruneBackupObjects(cfg)
		if _, err := j.Deps.RunTask(server, task).AsRoot().Dispatch(ctx); err != nil {
			return fmt.Errorf("prune remote objects: %w", err)
		}
	}

	for i := range stale {
		_ = j.Deps.Repos.BackupRun().Delete(ctx, stale[i].ID)
	}
	return nil
}

// dispatchSuccessNotification fires the per-config success
// notification when the user opted in via NotifyOnSuccess. The
// notification routes via the TaskRunnerDeps.Notifier (email + Slack
// + Discord + Telegram per the team's configured channels). No-op if
// the flag is off or no Notifier is wired (test runs, partial bring-
// up). The team's enabled channels decide which transports fire.
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

// dispatchFailureNotification fires the per-config failure
// notification when the user opted in via NotifyOnFailure (default
// true). Same routing as success; the error output is truncated for
// readability by the notification's WithError helper.
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

// lookupProjectName resolves a project name for notification copy.
// Falls back to empty string on any error — the notification renders
// "—" in that slot rather than crashing. Cheap query (PK on
// projectID); we don't bother caching across notifications.
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

// lookupStorageProviderLabel resolves a human label for the linked
// storage_providers row. The notification body shows it so users can
// see "uploaded to Contabo Storage" instead of a numeric ID.
func (j *RunBackupJob) lookupStorageProviderLabel(ctx context.Context, providerID uint64, teamID string) string {
	if j.Deps.BackupRepos == nil {
		return ""
	}
	p, err := j.Deps.BackupRepos.StorageProvider().FindStorageProviderByID(ctx, providerID)
	if err != nil || p == nil || p.TeamID != teamID {
		return ""
	}
	if p.Label == nil {
		return ""
	}
	return *p.Label
}

// notificationSource normalises the asynq payload Source field into
// a human-friendly tag for the notification body. Empty defaults to
// "schedule" because manual runs are dispatched via the service
// (which doesn't currently hit this notification path); future
// async-manual runs can pass "manual".
func notificationSource(s string) string {
	if s == "" {
		return "schedule"
	}
	return s
}
