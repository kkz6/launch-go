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
	"github.com/kkz6/launch-go/internal/modules/docker/tasks"
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
		return nil
	}
	dbCreds, err := decodeJobCredentials(db.Credentials)
	if err != nil {
		j.recordFailure(ctx, backup.ID, "", "database credentials are missing or corrupt")
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
		Database: dbCreds.Database,
		// All S3 destination fields come from the storage_providers
		// row now. The backup row only owns the optional sub-folder
		// (backup.Path) which we join onto the provider's default
		// path.
		Endpoint:   s3Creds.Endpoint,
		Region:     s3Creds.Region,
		Bucket:     s3Creds.Bucket,
		PathPrefix: composeBackupPath(backup.Path, s3Creds.Path),
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
		// Return nil so asynq doesn't auto-retry a backup that will fail
		// the same way (bad creds, missing CLI tool). The next cron tick
		// will re-attempt on its natural cadence.
		return nil
	}

	objectKey, sizeBytes := parseRunMarkers(output)
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

// composeBackupPath joins the storage provider's default path with the
// per-backup sub-folder. Either may be empty; we trim slashes so the
// upload script's "<prefix>/<file>" concat doesn't double up. Same
// rules as services.backupObjectPath.
func composeBackupPath(perBackup *string, providerDefault string) string {
	trim := func(s string) string {
		for len(s) > 0 && (s[0] == '/' || s[0] == ' ') {
			s = s[1:]
		}
		for len(s) > 0 && (s[len(s)-1] == '/' || s[len(s)-1] == ' ') {
			s = s[:len(s)-1]
		}
		return s
	}
	out := trim(providerDefault)
	if perBackup != nil {
		seg := trim(*perBackup)
		if seg != "" {
			if out == "" {
				out = seg
			} else {
				out = out + "/" + seg
			}
		}
	}
	return out
}

// parseRunMarkers walks the SSH output for `::LAUNCH::object_key::<k>`
// and `::LAUNCH::size_bytes::<n>` lines. Missing values mean the script
// aborted before emitting them — the caller treats those as failures.
func parseRunMarkers(output string) (string, int64) {
	const okPrefix = "::LAUNCH::object_key::"
	const szPrefix = "::LAUNCH::size_bytes::"
	var key string
	var size int64
	for _, line := range splitLines(output) {
		line = trimSpace(line)
		if len(line) > len(okPrefix) && line[:len(okPrefix)] == okPrefix {
			key = line[len(okPrefix):]
		}
		if len(line) > len(szPrefix) && line[:len(szPrefix)] == szPrefix {
			n := int64(0)
			for _, c := range line[len(szPrefix):] {
				if c < '0' || c > '9' {
					break
				}
				n = n*10 + int64(c-'0')
			}
			size = n
		}
	}
	return key, size
}

func strPtr(s string) *string { return &s }

func truncateForRun(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
