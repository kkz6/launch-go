package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	backupmodels "github.com/kkz6/launch-go/internal/modules/backup/models"
	backuptypes "github.com/kkz6/launch-go/internal/modules/backup/types"
	"github.com/kkz6/launch-go/internal/modules/docker/dto"
	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"github.com/kkz6/launch-go/internal/modules/docker/tasks"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// BackupService manages backup configs + run history + ad-hoc run-now
// + restore-from-snapshot for managed databases.
//
// As of the 0027 migration this service no longer stores S3 credentials
// on the docker_database_backups row. Each backup config references a
// global storage_providers entry by id; credentials live there and are
// shared across every backup that targets the same destination. The
// run-time path (RunNow + the scheduled job) loads the provider, pulls
// its S3 credentials map, and feeds the existing BackupRunConfig.
//
// Two execution paths land here:
//
//   - Synchronous (RunNow): user clicks "Run now" in the UI; we run the
//     SSH task inline so the response carries the resulting BackupRun
//     back to the browser.
//   - Asynchronous (scheduled): jobs.PollDueBackupsJob walks enabled
//     backups every minute and dispatches jobs.RunBackupJob, which
//     reproduces the same flow off the queue (and broadcasts the same
//     events). No state on the row distinguishes the two — they share
//     the same docker_database_backup_runs history.
type BackupService struct {
	*BaseService
}

func NewBackupService(deps *ServiceDeps) *BackupService {
	return &BackupService{BaseService: NewBaseService(deps)}
}

// GetBackup returns the backup config. No credentials in the response —
// they're owned by the linked storage_providers row. Returns nil + nil
// when no backup is configured yet so the UI can render the empty
// state.
func (s *BackupService) GetBackup(
	ctx context.Context, databaseID, projectID, serverID, teamID string,
) (*dto.BackupResponse, error) {
	if _, err := s.scopedDatabase(ctx, databaseID, projectID, serverID, teamID); err != nil {
		return nil, err
	}
	b, err := s.Repos().Backup().FindByDatabase(ctx, databaseID)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return dto.ToBackupResponse(b), nil
}

// ConfigureBackup creates or updates the backup config for a database.
// The storage_provider_id must reference an S3-driver provider that
// belongs to the caller's team — we 400 otherwise so the user gets a
// useful error instead of a foreign-key violation at insert time.
func (s *BackupService) ConfigureBackup(
	ctx context.Context, databaseID, projectID, serverID, teamID, userID string,
	req *dto.ConfigureBackupRequest,
) (dto.BackupResponse, error) {
	_ = userID
	db, err := s.scopedDatabase(ctx, databaseID, projectID, serverID, teamID)
	if err != nil {
		return dto.BackupResponse{}, err
	}

	// Verify the storage provider exists, belongs to this team, and is
	// an S3-flavoured driver (we don't yet know how to upload database
	// dumps to non-S3 backends).
	if _, err := s.loadTeamStorageProvider(ctx, req.StorageProviderID, teamID); err != nil {
		return dto.BackupResponse{}, err
	}

	existing, lookupErr := s.Repos().Backup().FindByDatabase(ctx, databaseID)
	if lookupErr != nil && !fiberutil.IsNotFound(lookupErr) {
		return dto.BackupResponse{}, lookupErr
	}

	var b *models.DatabaseBackup
	if existing != nil {
		updates := map[string]any{
			"storage_provider_id": req.StorageProviderID,
			"path":                req.Path,
			"retention":           req.Retention,
			"notify_on_success":   req.NotifyOnSuccess,
			"notify_on_failure":   req.NotifyOnFailure,
			"cron_schedule":       req.CronSchedule,
			"enabled":             req.Enabled,
		}
		if err := s.Repos().Backup().UpdateFields(ctx, existing.ID, updates); err != nil {
			return dto.BackupResponse{}, err
		}
		b, err = s.Repos().Backup().FindByDatabase(ctx, databaseID)
		if err != nil {
			return dto.BackupResponse{}, err
		}
	} else {
		b = &models.DatabaseBackup{
			DatabaseID:        databaseID,
			StorageProviderID: req.StorageProviderID,
			Path:              req.Path,
			Retention:         req.Retention,
			NotifyOnSuccess:   req.NotifyOnSuccess,
			NotifyOnFailure:   req.NotifyOnFailure,
			CronSchedule:      req.CronSchedule,
			Enabled:           req.Enabled,
		}
		b.TeamID = teamID
		if err := s.Repos().Backup().Create(ctx, b); err != nil {
			return dto.BackupResponse{}, err
		}
	}

	s.BroadcastToTeam(teamID, "docker.database.backup.configured", map[string]any{
		"database_id": db.ID,
		"server_id":   db.ServerID,
		"team_id":     db.TeamID,
		"backup_id":   b.ID,
	})

	return *dto.ToBackupResponse(b), nil
}

// DeleteBackup turns off backups for a database. Existing run rows are
// kept for audit.
func (s *BackupService) DeleteBackup(
	ctx context.Context, databaseID, projectID, serverID, teamID, userID string,
) error {
	_ = userID
	if _, err := s.scopedDatabase(ctx, databaseID, projectID, serverID, teamID); err != nil {
		return err
	}
	b, err := s.Repos().Backup().FindByDatabase(ctx, databaseID)
	if err != nil {
		return err
	}
	if err := s.Repos().Backup().Delete(ctx, b.ID); err != nil {
		return err
	}
	s.BroadcastToTeam(teamID, "docker.database.backup.deleted", map[string]any{
		"database_id": databaseID,
		"server_id":   serverID,
		"team_id":     teamID,
		"backup_id":   b.ID,
	})
	return nil
}

// ListRuns returns the recent backup history for a database (max 50).
func (s *BackupService) ListRuns(
	ctx context.Context, databaseID, projectID, serverID, teamID string,
) ([]dto.BackupRunResponse, error) {
	if _, err := s.scopedDatabase(ctx, databaseID, projectID, serverID, teamID); err != nil {
		return nil, err
	}
	b, err := s.Repos().Backup().FindByDatabase(ctx, databaseID)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return []dto.BackupRunResponse{}, nil
		}
		return nil, err
	}
	rows, err := s.Repos().BackupRun().ListForBackup(ctx, b.ID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.BackupRunResponse, 0, len(rows))
	for i := range rows {
		out = append(out, *dto.ToBackupRunResponse(&rows[i]))
	}
	return out, nil
}

// RunNow kicks off an immediate backup. Synchronous SSH call — fits a
// "click to back up" interaction so the response carries the run row
// back inline. The scheduled-cron path lives in jobs.RunBackupJob,
// dispatched once per minute by jobs.PollDueBackupsJob.
func (s *BackupService) RunNow(
	ctx context.Context, databaseID, projectID, serverID, teamID, userID string,
) (dto.BackupRunResponse, error) {
	_ = userID
	db, err := s.scopedDatabase(ctx, databaseID, projectID, serverID, teamID)
	if err != nil {
		return dto.BackupRunResponse{}, err
	}
	b, err := s.Repos().Backup().FindByDatabase(ctx, databaseID)
	if err != nil {
		return dto.BackupRunResponse{}, err
	}

	server, err := s.ServerRepos().Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return dto.BackupRunResponse{}, err
	}
	project, err := s.Repos().Project().FindByIDAndTeamServer(ctx, db.ProjectID, teamID, serverID)
	if err != nil {
		return dto.BackupRunResponse{}, err
	}

	s3Creds, err := s.loadProviderS3Creds(ctx, b.StorageProviderID, teamID)
	if err != nil {
		return dto.BackupRunResponse{}, err
	}
	dbCreds, err := loadDBCredentials(db)
	if err != nil {
		return dto.BackupRunResponse{}, fiberutil.BadRequest("Database credentials are missing or corrupt")
	}

	now := time.Now().UTC()
	run := &models.DatabaseBackupRun{
		BackupID:  b.ID,
		Status:    "running",
		StartedAt: &now,
	}
	if err := s.Repos().BackupRun().Create(ctx, run); err != nil {
		return dto.BackupRunResponse{}, err
	}

	cfg := tasks.BackupRunConfig{
		RunID: run.ID,
		ContainerName: tasks.DatabaseContainerName(
			tasks.SlugFromName(project.Name),
			tasks.SlugFromName(db.Name),
		),
		Engine:     db.Engine,
		Username:   dbCreds.Username,
		Password:   dbCreds.Password,
		Database:   dbCreds.Database,
		Endpoint:   s3Creds.Endpoint,
		Region:     s3Creds.Region,
		Bucket:     s3Creds.Bucket,
		PathPrefix: backupObjectPath(b.Path, s3Creds.Path),
		AccessKey:  s3Creds.Key,
		SecretKey:  s3Creds.Secret,
	}

	// Run synchronously through the existing taskrunner so we get the
	// captured output for marker parsing.
	taskWrapper := tasks.RunBackup(cfg)
	result, runErr := dispatchTaskAsRoot(s.BaseService, ctx, server, taskWrapper)

	finishedAt := time.Now().UTC()
	if runErr != nil || (result != nil && result.ExitCode != 0) {
		errMsg := ""
		if runErr != nil {
			errMsg = runErr.Error()
		}
		if result != nil {
			errMsg += "\n" + result.Stdout + result.Stderr
		}
		_ = s.Repos().BackupRun().UpdateFields(ctx, run.ID, map[string]any{
			"status":      "failed",
			"finished_at": finishedAt,
			"error":       truncate(errMsg, 4000),
		})
		s.BroadcastToTeam(teamID, "docker.database.backup.run.failed", map[string]any{
			"database_id": db.ID,
			"backup_id":   b.ID,
			"run_id":      run.ID,
			"server_id":   server.ID,
			"team_id":     teamID,
		})
		reloaded, _ := s.Repos().BackupRun().FindByID(ctx, run.ID)
		if reloaded != nil {
			return *dto.ToBackupRunResponse(reloaded), nil
		}
		return *dto.ToBackupRunResponse(run), nil
	}

	objectKey, sizeBytes := parseBackupMarkers(result.Stdout + result.Stderr)
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
	_ = s.Repos().BackupRun().UpdateFields(ctx, run.ID, updates)
	s.BroadcastToTeam(teamID, "docker.database.backup.run.succeeded", map[string]any{
		"database_id": db.ID,
		"backup_id":   b.ID,
		"run_id":      run.ID,
		"server_id":   server.ID,
		"team_id":     teamID,
		"object_key":  objectKey,
		"size_bytes":  sizeBytes,
	})

	// Honour the retention cap. Best-effort — failure to prune doesn't
	// fail the run because the snapshot itself is already safely
	// uploaded.
	if b.Retention > 0 {
		_ = s.pruneOldRuns(ctx, b.ID, b.Retention)
	}

	reloaded, err := s.Repos().BackupRun().FindByID(ctx, run.ID)
	if err != nil {
		return *dto.ToBackupRunResponse(run), nil
	}
	return *dto.ToBackupRunResponse(reloaded), nil
}

// Restore downloads a past snapshot and replays it into the running
// container. Identifies the run by ID; the run must have completed
// successfully and have an object_key.
func (s *BackupService) Restore(
	ctx context.Context, databaseID, projectID, serverID, teamID, userID string,
	req *dto.RestoreBackupRequest,
) error {
	_ = userID
	db, err := s.scopedDatabase(ctx, databaseID, projectID, serverID, teamID)
	if err != nil {
		return err
	}
	b, err := s.Repos().Backup().FindByDatabase(ctx, databaseID)
	if err != nil {
		return err
	}
	run, err := s.Repos().BackupRun().FindByID(ctx, req.RunID)
	if err != nil {
		return err
	}
	if run.BackupID != b.ID || run.Status != "success" || run.ObjectKey == nil {
		return fiberutil.BadRequest("Selected run is not a restorable snapshot")
	}

	server, err := s.ServerRepos().Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}
	project, err := s.Repos().Project().FindByIDAndTeamServer(ctx, db.ProjectID, teamID, serverID)
	if err != nil {
		return err
	}
	s3Creds, err := s.loadProviderS3Creds(ctx, b.StorageProviderID, teamID)
	if err != nil {
		return err
	}
	dbCreds, err := loadDBCredentials(db)
	if err != nil {
		return fiberutil.BadRequest("Database credentials are missing or corrupt")
	}

	cfg := tasks.RestoreBackupConfig{
		RunID: run.ID,
		ContainerName: tasks.DatabaseContainerName(
			tasks.SlugFromName(project.Name),
			tasks.SlugFromName(db.Name),
		),
		Engine:    db.Engine,
		Username:  dbCreds.Username,
		Password:  dbCreds.Password,
		Database:  dbCreds.Database,
		Endpoint:  s3Creds.Endpoint,
		Region:    s3Creds.Region,
		Bucket:    s3Creds.Bucket,
		ObjectKey: *run.ObjectKey,
		AccessKey: s3Creds.Key,
		SecretKey: s3Creds.Secret,
	}

	taskWrapper := tasks.Restore(cfg)
	result, runErr := dispatchTaskAsRoot(s.BaseService, ctx, server, taskWrapper)
	if runErr != nil {
		return fmt.Errorf("restore failed: %w", runErr)
	}
	if result != nil && result.ExitCode != 0 {
		return fmt.Errorf("restore failed: %s", truncate(result.Stdout+result.Stderr, 4000))
	}

	s.BroadcastToTeam(teamID, "docker.database.backup.restored", map[string]any{
		"database_id": db.ID,
		"backup_id":   b.ID,
		"run_id":      run.ID,
		"server_id":   server.ID,
		"team_id":     teamID,
	})
	return nil
}

// scopedDatabase mirrors the chain check used by DatabaseService.
func (s *BackupService) scopedDatabase(
	ctx context.Context, databaseID, projectID, serverID, teamID string,
) (*models.Database, error) {
	if _, err := s.Repos().Project().FindByIDAndTeamServer(ctx, projectID, teamID, serverID); err != nil {
		return nil, err
	}
	db, err := s.Repos().Database().FindByIDAndTeamServer(ctx, databaseID, teamID, serverID)
	if err != nil {
		return nil, err
	}
	if db.ProjectID != projectID {
		return nil, fiberutil.NotFound()
	}
	return db, nil
}

// loadTeamStorageProvider verifies a storage_provider id, team match,
// and S3-driver-ness. Used at configure time so the user sees a clean
// 400 instead of a FK error from MySQL.
func (s *BackupService) loadTeamStorageProvider(
	ctx context.Context, id uint64, teamID string,
) (*backupmodels.StorageProvider, error) {
	if s.BackupRepos() == nil {
		return nil, fmt.Errorf("storage providers registry is not wired into docker module")
	}
	p, err := s.BackupRepos().StorageProvider().FindStorageProviderByID(ctx, id)
	if err != nil {
		return nil, fiberutil.BadRequest("Storage provider not found")
	}
	if p.TeamID != teamID {
		return nil, fiberutil.BadRequest("Storage provider does not belong to your team")
	}
	if p.Provider != backuptypes.StorageDriverS3 {
		return nil, fiberutil.BadRequest("Only S3-compatible storage providers can host database backups")
	}
	return p, nil
}

// loadProviderS3Creds is the runtime cousin of loadTeamStorageProvider —
// called by RunNow/Restore to materialise the S3Credentials struct from
// the provider's encrypted JSON map.
func (s *BackupService) loadProviderS3Creds(
	ctx context.Context, providerID uint64, teamID string,
) (backupmodels.S3Credentials, error) {
	p, err := s.loadTeamStorageProvider(ctx, providerID, teamID)
	if err != nil {
		return backupmodels.S3Credentials{}, err
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
		return backupmodels.S3Credentials{}, fiberutil.BadRequest("Storage provider is missing S3 credentials")
	}
	return c, nil
}

// pruneOldRuns deletes the oldest run rows once the count exceeds the
// retention cap. We only delete the run rows here — actually deleting
// the remote S3 objects is a future enhancement (needs the storage
// driver layer).
func (s *BackupService) pruneOldRuns(ctx context.Context, backupID string, retention int) error {
	rows, err := s.Repos().BackupRun().ListForBackup(ctx, backupID)
	if err != nil {
		return err
	}
	if len(rows) <= retention {
		return nil
	}
	// ListForBackup returns most-recent-first; trim from the tail.
	for i := retention; i < len(rows); i++ {
		_ = s.Repos().BackupRun().Delete(ctx, rows[i].ID)
	}
	return nil
}

func loadDBCredentials(db *models.Database) (Credentials, error) {
	return decodeCredentials(db.Credentials)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// backupObjectPath composes the final bucket-prefix used by the upload
// script — the storage provider's "default" path joined with the
// per-backup sub-folder. Either may be empty; we strip leading/trailing
// slashes so the script's "<prefix>/<file>" concat doesn't double up.
func backupObjectPath(perBackup *string, providerDefault string) string {
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

// parseBackupMarkers reads `::LAUNCH::object_key::<k>` and `::LAUNCH::
// size_bytes::<n>` from the script output. Missing values mean the
// script aborted before emitting them.
func parseBackupMarkers(output string) (string, int64) {
	var key string
	var size int64
	for _, line := range splitLinesBackup(output) {
		const okPrefix = "::LAUNCH::object_key::"
		const szPrefix = "::LAUNCH::size_bytes::"
		if i := indexOf(line, okPrefix); i >= 0 {
			key = line[i+len(okPrefix):]
		}
		if i := indexOf(line, szPrefix); i >= 0 {
			n := int64(0)
			for _, c := range line[i+len(szPrefix):] {
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

// Tiny non-stdlib helpers to avoid a strings import cycle for the
// small handful of operations we do here.
func splitLinesBackup(s string) []string {
	out := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func indexOf(haystack, needle string) int {
	if len(needle) == 0 {
		return 0
	}
	for i := 0; i <= len(haystack)-len(needle); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

// dispatchTaskAsRoot is a tiny shim that runs a task via the SSH client
// directly. We avoid going through the asynq machinery because backup
// + restore are user-initiated and benefit from the synchronous
// "wait for result" semantics.
func dispatchTaskAsRoot(
	base *BaseService, ctx context.Context,
	server interface {
		ConnectionAsRoot() *taskrunner.Connection
	},
	task taskrunner.Task,
) (*taskrunner.SSHCommandResult, error) {
	_ = base
	client, err := taskrunner.NewSSHClientFromConnection(server.ConnectionAsRoot())
	if err != nil {
		return nil, err
	}
	defer client.Close()
	if err := client.Connect(); err != nil {
		return nil, err
	}
	return client.RunScript(ctx, task.Script())
}
