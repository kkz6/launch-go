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

	// Resolve linked databases → dump specs. Best-effort: skip any that
	// can't be resolved and log instead of failing the whole run, but if
	// the operator selected databases and NONE can be resolved, fail
	// loudly (better than silently uploading an empty archive).
	dumps, dumpErr := j.resolveDatabaseDumps(ctx)
	if dumpErr != nil {
		j.recordFailure(ctx, fmt.Sprintf("resolve database dumps: %v", dumpErr), nil)
		return nil
	}

	cfg := tasks.RunBackupConfig{
		JobID:          j.job.ID,
		BackupID:       j.backup.ID,
		Databases:      dumps,
		IncludeFiles:   includes,
		ExcludeFiles:   excludes,
		Endpoint:       s3.Endpoint,
		Region:         s3.Region,
		Bucket:         s3.Bucket,
		PathPrefix:     joinPath(j.backup.Path, s3.Path),
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

// resolveDatabaseDumps loads the databases linked to this backup
// (backup_databases → databases → users) and pairs them with the
// server's installed DB engine to produce one DatabaseDump per
// selected database, each with an authenticated dump command shape.
//
// Returns nil if the backup has no linked databases (the run is then
// files-only). Returns an error if linked DBs exist but none can be
// resolved — better to fail loudly than silently skip a backup the
// operator explicitly asked for.
func (j *RunManualBackupJob) resolveDatabaseDumps(ctx context.Context) ([]tasks.DatabaseDump, error) {
	databaseIDs, err := j.Deps.Repos.Backup().GetBackupDatabaseIDs(ctx, j.backup.ID)
	if err != nil {
		return nil, fmt.Errorf("load backup_databases: %w", err)
	}
	if len(databaseIDs) == 0 {
		return nil, nil
	}
	if j.Deps.DatabaseRepos == nil {
		return nil, fmt.Errorf(
			"backup links %d database(s) but the database module is not wired into the worker", len(databaseIDs),
		)
	}

	// Engine is per-server, not per-database — look it up once. The
	// services table records what was installed via the provision flow.
	engine, err := j.detectServerDBEngine(ctx)
	if err != nil {
		return nil, fmt.Errorf("detect server database engine: %w", err)
	}

	out := make([]tasks.DatabaseDump, 0, len(databaseIDs))
	for _, dbID := range databaseIDs {
		db, err := j.Deps.DatabaseRepos.Database().FindByIDsAndServer(ctx, []string{dbID}, j.server.ID)
		if err != nil || len(db) == 0 {
			j.Deps.Logger.Warn().Str("database_id", dbID).
				Msg("backup: linked database not found — skipping")
			continue
		}
		// Pick the first user that has access — a backup just needs
		// read perms, so any linked user works; root is preferred.
		users, err := j.Deps.DatabaseRepos.User().FindByDatabase(ctx, dbID)
		if err != nil || len(users) == 0 {
			j.Deps.Logger.Warn().Str("database_id", dbID).
				Msg("backup: no database_users linked — skipping")
			continue
		}
		user := users[0]
		for _, u := range users {
			if u.Name == "root" {
				user = u
				break
			}
		}
		password := ""
		if user.Password != nil && user.Password.Valid {
			// EncryptedNullableString stores the plaintext on .String
			// after GORM Scan decrypts it; .Valid distinguishes NULL.
			password = user.Password.String
		}
		out = append(out, tasks.DatabaseDump{
			Name:     db[0].Name,
			Engine:   engine,
			Username: user.Name,
			Password: password,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf(
			"all %d linked database(s) were unresolvable (missing rows / no users)", len(databaseIDs),
		)
	}
	return out, nil
}

// detectServerDBEngine returns "mysql" / "postgres" / "mariadb" based
// on which database-server service is installed on this server. We
// pick the first hit; servers running multiple engines simultaneously
// aren't a supported configuration today.
func (j *RunManualBackupJob) detectServerDBEngine(ctx context.Context) (string, error) {
	svcs, err := j.Deps.ServerRepos.Service().FindByServer(ctx, j.server.ID)
	if err != nil {
		return "", err
	}
	for _, s := range svcs {
		// Match by prefix so future mysql/postgres versions don't need
		// a code change here.
		switch {
		case strings.HasPrefix(s.Software, "mysql"):
			return "mysql", nil
		case strings.HasPrefix(s.Software, "postgres") || strings.HasPrefix(s.Software, "postgresql"):
			return "postgres", nil
		case strings.HasPrefix(s.Software, "mariadb"):
			return "mariadb", nil
		}
	}
	return "", fmt.Errorf("no database engine (mysql/postgres/mariadb) installed on this server")
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

// joinPath stitches the backup-row sub-path with the provider's default
// prefix — neither is required, both are stripped of stray slashes.
func joinPath(backupPath, providerPath string) string {
	a := trimSlash(providerPath)
	b := trimSlash(backupPath)
	switch {
	case a != "" && b != "":
		return a + "/" + b
	case a != "":
		return a
	default:
		return b
	}
}

func trimSlash(s string) string {
	for len(s) > 0 && (s[0] == '/' || s[len(s)-1] == '/') {
		if s[0] == '/' {
			s = s[1:]
		} else {
			s = s[:len(s)-1]
		}
	}
	return s
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
