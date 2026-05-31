package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/backup/models"
	"github.com/kkz6/launch-go/internal/modules/backup/tasks"
	backuptypes "github.com/kkz6/launch-go/internal/modules/backup/types"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeRunManualBackup = "backup:run_manual"

// RunManualBackupJob triggers a manual backup run on a server.
//
// Lifecycle:
//
//  1. Create a backup_jobs row (status=pending) immediately so the UI
//     has something to render.
//  2. Resolve the linked storage provider's S3 credentials (the global
//     storage_providers row, decrypted by the GORM serializer).
//  3. Build + run the backup script over SSH via the shared TaskRunner
//     with TrackInDB + OnTaskCreated, so the server-tasks row exists
//     before SSH even starts. The OnTaskCreated hook stamps the
//     backup_jobs row with task_id + flips it to "running" and
//     broadcasts run.started — that's what the live log console
//     subscribes to.
//  4. On finish, parse object_key + size markers from the captured
//     output and update the backup_jobs row to finished/failed +
//     broadcast the terminal event.
type RunManualBackupJob struct {
	Deps    *JobDeps
	Payload RunManualBackupPayload

	server *servermodels.Server
	backup *models.Backup
	job    *models.BackupJob
}

func NewRunManualBackupJob(p RunManualBackupPayload) pkgjobs.Handler {
	return &RunManualBackupJob{Deps: deps, Payload: p}
}

func (j *RunManualBackupJob) Handle(ctx context.Context) error {
	var err error

	j.backup, err = j.Deps.Repos.Backup().FindBackupByID(ctx, j.Payload.BackupID)
	if err != nil {
		return fmt.Errorf("find backup: %w", err)
	}
	j.server, err = j.Deps.ServerRepos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("find server: %w", err)
	}

	// Resolve S3 creds from the linked storage provider. Same pattern as
	// docker DB backups — the GORM EncryptedJSONMap serializer decrypts
	// the credentials column on Find; we just shape it into S3Credentials.
	s3, err := j.loadS3Creds(ctx)
	if err != nil {
		j.recordFailure(ctx, fmt.Sprintf("resolve storage credentials: %v", err), nil)
		return nil
	}

	// Create the backup_jobs row up-front so the history list shows it
	// immediately. Status starts pending; OnTaskCreated flips to running.
	now := time.Now().UTC()
	_ = now
	j.job = &models.BackupJob{
		Status:            backuptypes.BackupJobStatusPending,
		BackupID:          j.backup.ID,
		StorageProviderID: j.backup.StorageProviderID,
	}
	j.job.TeamID = j.backup.TeamID
	if err := j.Deps.Repos.BackupJob().CreateBackupJob(ctx, j.job); err != nil {
		return fmt.Errorf("create backup job row: %w", err)
	}

	// Parse the JSON-encoded include/exclude paths off the backup row.
	var includes, excludes []string
	_ = json.Unmarshal([]byte(j.backup.IncludeFiles), &includes)
	_ = json.Unmarshal([]byte(j.backup.ExcludeFiles), &excludes)

	// Backup row's `Path` field — the form labels it "Backup Path"
	// with helper "The path on the server to backup". Treat it as an
	// implicit include so an operator who fills in just that field
	// (the common case before the include/exclude inputs landed)
	// still gets a working backup. Explicit includes are additive.
	if rootPath := strings.TrimSpace(j.backup.Path); rootPath != "" && rootPath != "/" {
		includes = append([]string{rootPath}, includes...)
	}

	cfg := tasks.RunBackupConfig{
		JobID:        j.job.ID,
		BackupID:     j.backup.ID,
		IncludeFiles: includes,
		ExcludeFiles: excludes,
		Endpoint:     s3.Endpoint,
		Region:       s3.Region,
		Bucket:       s3.Bucket,
		// S3 sub-prefix comes from the provider's default Path (the
		// per-team "where do uploads go in the bucket"), NOT from
		// the backup row's Path (that's a source path on the box).
		PathPrefix:     strings.Trim(s3.Path, "/"),
		AccessKey:      s3.Key,
		SecretKey:      s3.Secret,
		ForcePathStyle: s3.ForcePathStyle,
	}

	task := tasks.RunBackup(cfg)
	result, runErr := j.Deps.RunTask(j.server, task).AsRoot().TrackInDB().
		OnTaskCreated(func(taskID string) {
			// Stamp the run row with the task id + flip to running
			// before the script starts, so the UI can open the live
			// log console immediately on the run.started event.
			tid := taskID
			j.job.TaskID = &tid
			j.job.Status = backuptypes.BackupJobStatusRunning
			_ = j.Deps.Repos.BackupJob().UpdateBackupJob(ctx, j.job)
			j.broadcast("backup.run.started", map[string]any{
				"backup_id": j.backup.ID,
				"job_id":    j.job.ID,
				"server_id": j.server.ID,
				"task_id":   tid,
			})
		}).
		Dispatch(ctx)

	// Capture the script's output (stdout+stderr concatenated) and parse
	// the final markers if the script reached the upload step.
	output := ""
	exit := -1
	if result != nil {
		output = result.GetOutput()
		exit = result.GetExitCode()
	}

	finishedAt := time.Now().UTC()
	_ = finishedAt
	if runErr != nil || exit != 0 {
		msg := ""
		if runErr != nil {
			msg = runErr.Error()
		}
		if output != "" {
			// The full transcript still lives behind View Logs; the
			// error column only needs a clean one-liner. Strip the
			// `::LAUNCH::` control markers + the verbose `==>` step
			// echoes so the row tooltip shows just the actionable
			// failure message (e.g. "no include paths configured").
			msg += "\n" + cleanScriptError(output)
		}
		j.recordFailure(ctx, msg, j.job.TaskID)
		return nil
	}

	_, size := tasks.ParseRunMarkers(output)
	j.job.Status = backuptypes.BackupJobStatusFinished
	if size > 0 {
		s := int(size)
		j.job.Size = &s
	}
	if err := j.Deps.Repos.BackupJob().UpdateBackupJob(ctx, j.job); err != nil {
		j.Deps.Logger.Warn().Err(err).Str("job_id", j.job.ID).
			Msg("failed to mark backup job as finished")
	}
	j.broadcast("backup.run.succeeded", map[string]any{
		"backup_id": j.backup.ID,
		"job_id":    j.job.ID,
		"server_id": j.server.ID,
		"task_id":   j.job.TaskID,
		"size":      size,
	})
	return nil
}

func (j *RunManualBackupJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("server_id", j.Payload.ServerID).
		Str("backup_id", j.Payload.BackupID).
		Msg("failed to run manual backup")
	// If the job row was created, mark it failed too — Failed() runs on
	// asynq exhaustion / panic, not on the normal failure path which is
	// already handled by recordFailure().
	if j.job != nil && j.job.Status != backuptypes.BackupJobStatusFailed && j.job.Status != backuptypes.BackupJobStatusFinished {
		msg := err.Error()
		_ = j.Deps.Repos.BackupJob().UpdateBackupJob(ctx, &models.BackupJob{
			BaseModel:         j.job.BaseModel,
			TeamScoped:        j.job.TeamScoped,
			Status:            backuptypes.BackupJobStatusFailed,
			BackupID:          j.job.BackupID,
			StorageProviderID: j.job.StorageProviderID,
			TaskID:            j.job.TaskID,
			Error:             &msg,
		})
	}
}

// recordFailure updates the BackupJob row + broadcasts the failure
// event. taskID is optional — set when the task was created before the
// failure, NULL when we failed earlier (e.g. resolving credentials).
func (j *RunManualBackupJob) recordFailure(ctx context.Context, message string, taskID *string) {
	if j.job != nil {
		j.job.Status = backuptypes.BackupJobStatusFailed
		errCopy := truncate(message, 4000)
		j.job.Error = &errCopy
		j.job.TaskID = taskID
		_ = j.Deps.Repos.BackupJob().UpdateBackupJob(ctx, j.job)
	}
	payload := map[string]any{
		"backup_id": j.Payload.BackupID,
		"server_id": j.Payload.ServerID,
	}
	if j.job != nil {
		payload["job_id"] = j.job.ID
	}
	if taskID != nil {
		payload["task_id"] = *taskID
	}
	j.broadcast("backup.run.failed", payload)
}

// broadcast emits a WS event scoped to the backup's team channel. The
// frontend's `useChannelEvents` filter requires the routing field
// (team_id) on every payload — added here so callers don't have to
// remember.
func (j *RunManualBackupJob) broadcast(event string, data map[string]any) {
	if data == nil {
		data = map[string]any{}
	}
	if _, ok := data["team_id"]; !ok && j.backup != nil {
		data["team_id"] = j.backup.TeamID
	}
	j.Deps.BroadcastToTeam(j.backup.TeamID, event, data)
}

// loadS3Creds materialises the linked storage provider's S3Credentials.
// The provider's `credentials` column is an EncryptedJSONMap that GORM
// decrypts on Find; we just JSON-re-encode/decode it to coerce the
// map[string]any into the typed S3Credentials shape. Same pattern as
// the docker module's loadProviderS3Creds.
func (j *RunManualBackupJob) loadS3Creds(ctx context.Context) (models.S3Credentials, error) {
	p, err := j.Deps.Repos.StorageProvider().FindStorageProviderByID(ctx, j.backup.StorageProviderID)
	if err != nil {
		return models.S3Credentials{}, fmt.Errorf("provider %d not found", j.backup.StorageProviderID)
	}
	raw, err := json.Marshal(p.GetCredentials())
	if err != nil {
		return models.S3Credentials{}, fmt.Errorf("encode credentials: %w", err)
	}
	var c models.S3Credentials
	if err := json.Unmarshal(raw, &c); err != nil {
		return models.S3Credentials{}, fmt.Errorf("decode S3 credentials: %w", err)
	}
	if c.Bucket == "" || c.Key == "" {
		return models.S3Credentials{}, fmt.Errorf("provider %d has incomplete S3 credentials", j.backup.StorageProviderID)
	}
	return c, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// cleanScriptError reduces the raw script transcript to just the lines
// worth surfacing on the row tooltip: drop `::LAUNCH::` control markers
// (they're for the marker handler, not humans) and the verbose `==>`
// step echoes. The full transcript still lives behind View Logs.
func cleanScriptError(output string) string {
	var keep []string
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "::LAUNCH::") {
			continue
		}
		if strings.HasPrefix(trimmed, "==>") {
			continue
		}
		keep = append(keep, trimmed)
	}
	// Last few lines are the most diagnostic (the actual failure tail);
	// cap to avoid dumping a multi-page script into a tooltip.
	if len(keep) > 5 {
		keep = keep[len(keep)-5:]
	}
	return strings.Join(keep, "\n")
}

func NewRunManualBackupTask(serverID, backupID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeRunManualBackup,
		RunManualBackupPayload{ServerID: serverID, BackupID: backupID, UserID: userID},
	)
}
