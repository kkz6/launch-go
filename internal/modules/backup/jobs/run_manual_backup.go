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

// RunManualBackupJob executes a queued server backup.
type RunManualBackupJob struct {
	Deps    *JobDeps
	Payload RunManualBackupPayload

	server       *servermodels.Server
	backup       *models.Backup
	job          *models.BackupJob
	finalAttempt func(context.Context, error) bool
}

func NewRunManualBackupJob(p RunManualBackupPayload) pkgjobs.Handler {
	return &RunManualBackupJob{Deps: deps, Payload: p}
}

func (j *RunManualBackupJob) Handle(ctx context.Context) error {
	var err error
	if j.Payload.JobID != "" {
		if j.Payload.TeamID == "" {
			return fmt.Errorf("team id is required for a pre-created backup job")
		}
		retryCount, hasRetryCount := asynq.GetRetryCount(ctx)
		allowRunning := hasRetryCount && retryCount > 0
		claimedJob, claimed, claimErr := j.Deps.Repos.BackupJob().ClaimPendingBackupJobForRun(
			ctx,
			j.Payload.JobID,
			j.Payload.BackupID,
			j.Payload.TeamID,
			allowRunning,
		)
		if claimErr != nil {
			return fmt.Errorf("claim pre-created backup job: %w", claimErr)
		}
		if !claimed {
			j.Deps.Logger.Warn().
				Str("job_id", j.Payload.JobID).
				Str("backup_id", j.Payload.BackupID).
				Str("source", j.Payload.Source).
				Msg("backup run is out of scope, terminal, or already claimed; skipping")
			return nil
		}
		j.job = claimedJob
	}

	j.backup, err = j.Deps.Repos.Backup().FindByID(ctx, j.Payload.BackupID)
	if err != nil {
		return fmt.Errorf("find backup: %w", err)
	}
	if j.Payload.TeamID != "" && j.backup.TeamID != j.Payload.TeamID {
		return fmt.Errorf("backup team does not match queued run")
	}
	if j.backup.ServerID != j.Payload.ServerID {
		return fmt.Errorf("backup server does not match queued run")
	}
	if j.job != nil {
		if j.job.BackupID != j.backup.ID || j.job.TeamID != j.backup.TeamID {
			return fmt.Errorf("backup job scope does not match backup")
		}
		if j.job.StorageProviderID != j.backup.StorageProviderID {
			return fmt.Errorf("backup storage provider changed before the run started")
		}
	}
	j.server, err = j.Deps.ServerRepos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("find server: %w", err)
	}
	if j.server.TeamID != j.backup.TeamID {
		return fmt.Errorf("server team does not match backup")
	}

	if j.job == nil {
		j.job = &models.BackupJob{
			Status:            backuptypes.BackupJobStatusPending,
			BackupID:          j.backup.ID,
			StorageProviderID: j.backup.StorageProviderID,
		}
		j.job.TeamID = j.backup.TeamID
		if err := j.Deps.Repos.BackupJob().CreateBackupJob(ctx, j.job); err != nil {
			return fmt.Errorf("create backup job row: %w", err)
		}
	}

	s3, err := j.loadS3Creds(ctx)
	if err != nil {
		if persistErr := j.recordFailure(ctx, fmt.Sprintf("resolve storage credentials: %v", err), nil); persistErr != nil {
			return fmt.Errorf("persist storage credential failure: %w", persistErr)
		}
		return nil
	}

	includes, err := parseBackupPaths(j.backup.IncludeFiles)
	if err != nil {
		if persistErr := j.recordFailure(ctx, fmt.Sprintf("invalid include files configuration: %v", err), nil); persistErr != nil {
			return fmt.Errorf("persist include-files failure: %w", persistErr)
		}
		return nil
	}
	excludes, err := parseBackupPaths(j.backup.ExcludeFiles)
	if err != nil {
		if persistErr := j.recordFailure(ctx, fmt.Sprintf("invalid exclude files configuration: %v", err), nil); persistErr != nil {
			return fmt.Errorf("persist exclude-files failure: %w", persistErr)
		}
		return nil
	}

	dumps, dumpErr := j.resolveDatabaseDumps(ctx)
	if dumpErr != nil {
		if persistErr := j.recordFailure(ctx, fmt.Sprintf("resolve database dumps: %v", dumpErr), nil); persistErr != nil {
			return fmt.Errorf("persist database-dump failure: %w", persistErr)
		}
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
	createdTaskID := ""
	result, runErr := j.Deps.RunTask(j.server, task).AsRoot().TrackInDB().
		OnTaskCreated(func(taskID string) {
			createdTaskID = taskID
			j.recordStarted(ctx, taskID)
		}).
		Dispatch(ctx)

	output := ""
	exit := -1
	if result != nil {
		output = result.GetOutput()
		exit = result.GetExitCode()
	}

	if runErr != nil || exit != 0 {
		msg := ""
		if runErr != nil {
			msg = runErr.Error()
		}
		if output != "" {
			msg += "\n" + cleanScriptError(output)
		}
		if strings.TrimSpace(msg) == "" {
			msg = fmt.Sprintf("backup command exited with code %d without output", exit)
		}
		var taskID *string
		if createdTaskID != "" {
			taskID = &createdTaskID
		}
		if persistErr := j.recordFailure(ctx, msg, taskID); persistErr != nil {
			return fmt.Errorf("persist backup failure state: %w", persistErr)
		}
		return nil
	}

	_, size := tasks.ParseRunMarkers(output)
	var taskID *string
	if createdTaskID != "" {
		taskID = &createdTaskID
	}
	if persistErr := j.recordSuccess(ctx, size, taskID); persistErr != nil {
		return fmt.Errorf("persist backup success state: %w", persistErr)
	}
	return nil
}

func (j *RunManualBackupJob) Failed(ctx context.Context, err error) {
	message := "backup worker failed"
	if err != nil {
		message += ": " + err.Error()
	}
	j.Deps.Logger.Error().Err(err).
		Str("server_id", j.Payload.ServerID).
		Str("backup_id", j.Payload.BackupID).
		Str("source", j.Payload.Source).
		Msg("failed to run backup")
	if !j.isFinalAttempt(ctx, err) {
		return
	}
	if j.job == nil && j.Payload.JobID != "" {
		if adoptErr := j.adoptPrecreatedJob(ctx); adoptErr != nil {
			j.Deps.Logger.Error().Err(adoptErr).Str("job_id", j.Payload.JobID).
				Msg("failed to reload backup run for final failure")
		}
	}
	if j.job != nil && (j.job.Status == backuptypes.BackupJobStatusFailed || j.job.Status == backuptypes.BackupJobStatusFinished) {
		return
	}
	var taskID *string
	if j.job != nil {
		taskID = j.job.TaskID
	}
	if persistErr := j.recordFailure(ctx, message, taskID); persistErr != nil {
		j.Deps.Logger.Error().Err(persistErr).Str("job_id", j.Payload.JobID).
			Msg("failed to persist final backup failure")
	}
}

func (j *RunManualBackupJob) isFinalAttempt(ctx context.Context, err error) bool {
	if j.finalAttempt != nil {
		return j.finalAttempt(ctx, err)
	}
	return pkgjobs.IsFinalAttempt(ctx, err)
}

func (j *RunManualBackupJob) adoptPrecreatedJob(ctx context.Context) error {
	if j.job != nil {
		return nil
	}
	if j.Payload.JobID == "" {
		return nil
	}
	if j.Payload.TeamID == "" {
		return fmt.Errorf("team id is required for a pre-created backup job")
	}
	job, err := j.Deps.Repos.BackupJob().FindBackupJobForRun(
		ctx,
		j.Payload.JobID,
		j.Payload.BackupID,
		j.Payload.TeamID,
	)
	if err != nil {
		return err
	}
	j.job = job
	return nil
}

func (j *RunManualBackupJob) recordStarted(ctx context.Context, taskID string) {
	updated, updateErr := j.Deps.Repos.BackupJob().MarkBackupJobRunningForRun(
		ctx,
		j.job.ID,
		j.job.BackupID,
		j.job.TeamID,
		taskID,
	)
	if updateErr != nil {
		j.Deps.Logger.Error().Err(updateErr).Str("job_id", j.job.ID).
			Msg("failed to persist running backup job")
		return
	}
	if !updated {
		j.Deps.Logger.Warn().Str("job_id", j.job.ID).
			Msg("backup job became terminal before its task could be attached")
		return
	}

	tid := taskID
	j.job.TaskID = &tid
	j.job.Status = backuptypes.BackupJobStatusRunning
	j.broadcast("backup.run.started", map[string]any{
		"backup_id": j.job.BackupID,
		"job_id":    j.job.ID,
		"server_id": j.Payload.ServerID,
		"task_id":   taskID,
	})
}

func (j *RunManualBackupJob) recordSuccess(ctx context.Context, size int64, taskID *string) error {
	var persistedSize *int
	if size > 0 {
		s := int(size)
		persistedSize = &s
	}
	updated, updateErr := j.Deps.Repos.BackupJob().MarkBackupJobFinishedForRun(
		ctx,
		j.job.ID,
		j.job.BackupID,
		j.job.TeamID,
		persistedSize,
		taskID,
	)
	if updateErr != nil {
		j.Deps.Logger.Error().Err(updateErr).Str("job_id", j.job.ID).
			Msg("failed to persist backup success state")
		return updateErr
	}
	if !updated {
		j.Deps.Logger.Warn().Str("job_id", j.job.ID).
			Msg("backup job was no longer running when success completed")
		return nil
	}

	j.job.Status = backuptypes.BackupJobStatusFinished
	j.job.Size = persistedSize
	if taskID != nil {
		id := *taskID
		j.job.TaskID = &id
	}
	payload := map[string]any{
		"backup_id": j.job.BackupID,
		"job_id":    j.job.ID,
		"server_id": j.Payload.ServerID,
		"size":      size,
	}
	if taskID != nil {
		payload["task_id"] = *taskID
	}
	j.broadcast("backup.run.succeeded", payload)
	return nil
}

func (j *RunManualBackupJob) recordFailure(ctx context.Context, message string, taskID *string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		message = "backup failed without an error message"
	}
	message = truncate(message, 4000)

	if j.Deps.Repos == nil {
		return nil
	}
	jobID := j.Payload.JobID
	backupID := j.Payload.BackupID
	teamID := j.Payload.TeamID
	if j.job != nil {
		jobID = j.job.ID
		backupID = j.job.BackupID
		teamID = j.job.TeamID
	}
	if jobID == "" || backupID == "" || teamID == "" {
		return nil
	}
	updated, updateErr := j.Deps.Repos.BackupJob().MarkBackupJobFailedForRun(
		ctx,
		jobID,
		backupID,
		teamID,
		message,
		taskID,
	)
	if updateErr != nil {
		j.Deps.Logger.Error().Err(updateErr).Str("job_id", jobID).
			Msg("failed to persist backup failure state")
		return updateErr
	}
	if !updated {
		return nil
	}
	if j.job != nil {
		j.job.Status = backuptypes.BackupJobStatusFailed
		errCopy := message
		j.job.Error = &errCopy
		if taskID != nil {
			id := *taskID
			j.job.TaskID = &id
		}
	}
	payload := map[string]any{
		"backup_id": backupID,
		"server_id": j.Payload.ServerID,
		"error":     message,
		"job_id":    jobID,
	}
	if taskID != nil {
		payload["task_id"] = *taskID
	}
	j.broadcast("backup.run.failed", payload)
	return nil
}

func parseBackupPaths(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var paths []string
	if err := json.Unmarshal([]byte(raw), &paths); err != nil {
		return nil, err
	}
	return paths, nil
}

func (j *RunManualBackupJob) broadcast(event string, data map[string]any) {
	if data == nil {
		data = map[string]any{}
	}
	teamID := j.Payload.TeamID
	if j.job != nil && j.job.TeamID != "" {
		teamID = j.job.TeamID
	} else if j.backup != nil && j.backup.TeamID != "" {
		teamID = j.backup.TeamID
	}
	if teamID == "" {
		return
	}
	if _, ok := data["team_id"]; !ok {
		data["team_id"] = teamID
	}
	j.Deps.BroadcastToTeam(teamID, event, data)
}

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

func (j *RunManualBackupJob) detectServerDBEngine(ctx context.Context) (string, error) {
	svcs, err := j.Deps.ServerRepos.Service().FindByServer(ctx, j.server.ID)
	if err != nil {
		return "", err
	}
	for _, s := range svcs {
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

func (j *RunManualBackupJob) loadS3Creds(ctx context.Context) (models.S3Credentials, error) {
	p, err := j.Deps.Repos.StorageProvider().FindStorageProviderByIDAndTeam(
		ctx,
		j.backup.StorageProviderID,
		j.backup.TeamID,
	)
	if err != nil {
		return models.S3Credentials{}, fmt.Errorf("provider %d not found", j.backup.StorageProviderID)
	}
	if p.Provider != backuptypes.StorageDriverS3 {
		return models.S3Credentials{}, fmt.Errorf("provider %d is not an S3 driver", j.backup.StorageProviderID)
	}
	credentials := p.GetCredentials()
	raw, _ := json.Marshal(credentials)
	var c models.S3Credentials
	if err := json.Unmarshal(raw, &c); err != nil {
		return models.S3Credentials{}, fmt.Errorf("decode S3 credentials: %w", err)
	}
	c.ApplyLegacyDefaults(credentials)
	if c.Bucket == "" || c.Key == "" || c.Secret == "" {
		return models.S3Credentials{}, fmt.Errorf("provider %d has incomplete S3 credentials", j.backup.StorageProviderID)
	}
	return c, nil
}

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
	if len(keep) > 5 {
		keep = keep[len(keep)-5:]
	}
	return strings.Join(keep, "\n")
}

func NewRunManualBackupTask(
	serverID, backupID, teamID, jobID string,
	userID *string,
) (*asynq.Task, error) {
	payload := RunManualBackupPayload{
		ServerID: serverID,
		BackupID: backupID,
		TeamID:   teamID,
		JobID:    jobID,
		Source:   "manual",
		UserID:   userID,
	}
	if jobID == "" {
		return pkgjobs.Task(TypeRunManualBackup, payload)
	}
	return pkgjobs.TaskWithID(TypeRunManualBackup, payload, runManualBackupTaskID(jobID))
}

func NewRunScheduledBackupTask(
	serverID, backupID, teamID, jobID string,
	scheduledAt time.Time,
) (*asynq.Task, error) {
	payload := RunManualBackupPayload{
		ServerID: serverID,
		BackupID: backupID,
		TeamID:   teamID,
		JobID:    jobID,
		Source:   "schedule",
	}
	return pkgjobs.TaskWithID(
		TypeRunManualBackup,
		payload,
		scheduledBackupTaskID(backupID, scheduledAt),
	)
}

func runManualBackupTaskID(jobID string) string {
	return pkgjobs.Dedup("backup-run-manual", jobID)
}

func scheduledBackupTaskID(backupID string, scheduledAt time.Time) string {
	minute := scheduledAt.UTC().Truncate(time.Minute).Format("2006-01-02T15:04")
	return pkgjobs.Dedup("backup-run-scheduled", backupID, minute)
}
