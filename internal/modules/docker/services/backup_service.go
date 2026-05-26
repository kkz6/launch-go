package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	backupmodels "github.com/kkz6/launch-go/internal/modules/backup/models"
	backuptypes "github.com/kkz6/launch-go/internal/modules/backup/types"
	"github.com/kkz6/launch-go/internal/modules/docker/dto"
	"github.com/kkz6/launch-go/internal/modules/docker/models"
	dockernotifications "github.com/kkz6/launch-go/internal/modules/docker/notifications"
	"github.com/kkz6/launch-go/internal/modules/docker/tasks"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
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

	// Normalise the optional database-name override. Whitespace-only
	// input behaves like "empty" so the engine falls back to the row's
	// default database — the user clearing the field shouldn't trip a
	// dump on a literally-named "  " database.
	normalisedDBName := normaliseOptionalString(req.DatabaseName)

	var b *models.DatabaseBackup
	if existing != nil {
		updates := map[string]any{
			"storage_provider_id": req.StorageProviderID,
			"database_name":       normalisedDBName,
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
			DatabaseName:      normalisedDBName,
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
		Engine:   db.Engine,
		Username: dbCreds.Username,
		Password: dbCreds.Password,
		// Database-name override: if the user set one on the config row
		// we target it; otherwise fall back to the database the row was
		// provisioned with. EffectiveDatabaseName centralises the logic
		// so the scheduled path (jobs/run_backup.go) can't drift.
		Database:   b.EffectiveDatabaseName(dbCreds.Database),
		Endpoint:   s3Creds.Endpoint,
		Region:     s3Creds.Region,
		Bucket:     s3Creds.Bucket,
		PathPrefix: tasks.BackupObjectPath(b.Path, s3Creds.Path),
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
		s.dispatchBackupFailureNotification(ctx, b, db, server, errMsg)
		reloaded, _ := s.Repos().BackupRun().FindByID(ctx, run.ID)
		if reloaded != nil {
			return *dto.ToBackupRunResponse(reloaded), nil
		}
		return *dto.ToBackupRunResponse(run), nil
	}

	objectKey, sizeBytes := tasks.ParseRunMarkers(result.Stdout + result.Stderr)
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
	s.dispatchBackupSuccessNotification(ctx, b, db, server, project, objectKey, sizeBytes)

	// Honour the retention cap. Best-effort — failure to prune doesn't
	// fail the run because the snapshot itself is already safely
	// uploaded. Prunes BOTH the run rows AND the remote S3 objects;
	// the previous implementation only deleted rows, which silently
	// leaked storage cost as runs accumulated.
	if b.Retention > 0 {
		if err := s.pruneRunsAndObjects(ctx, server, b, s3Creds); err != nil {
			if s.Logger != nil {
				s.Logger.Warn().Err(err).
					Str("backup_id", b.ID).
					Msg("retention prune failed (run still succeeded)")
			}
		}
	}

	reloaded, err := s.Repos().BackupRun().FindByID(ctx, run.ID)
	if err != nil {
		return *dto.ToBackupRunResponse(run), nil
	}
	return *dto.ToBackupRunResponse(reloaded), nil
}

// Restore downloads a past snapshot and replays it into a running
// container. Identifies the run by ID; the run must have completed
// successfully and have an object_key.
//
// Target resolution:
//   - `req.TargetDatabaseID` nil/empty → restore into the source
//     database row (today's behaviour; backwards-compat).
//   - `req.TargetDatabaseID` set → restore into the named database
//     instead, after validating that target is on the same server and
//     uses the same engine. Lets users dry-run "prod → staging" or
//     copy a snapshot between same-engine workloads without a manual
//     dump/import cycle.
//
// The target's OWN container + credentials are used for ingest. We do
// not impersonate the source — the dump file is engine-format-portable
// (pg_dump → psql, mysqldump → mysql) so the target's auth is what
// the ingest CLI needs.
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
	s3Creds, err := s.loadProviderS3Creds(ctx, b.StorageProviderID, teamID)
	if err != nil {
		return err
	}

	// Resolve the target — defaults to the source DB. When the user
	// pointed at a different docker_databases row, we re-load it +
	// validate same-server + same-engine before using its container
	// + credentials.
	targetDB, targetProject, err := s.resolveRestoreTarget(ctx, db, teamID, serverID, req.TargetDatabaseID)
	if err != nil {
		return err
	}

	dbCreds, err := loadDBCredentials(targetDB)
	if err != nil {
		return fiberutil.BadRequest("Target database credentials are missing or corrupt")
	}

	cfg := tasks.RestoreBackupConfig{
		RunID: run.ID,
		ContainerName: tasks.DatabaseContainerName(
			tasks.SlugFromName(targetProject.Name),
			tasks.SlugFromName(targetDB.Name),
		),
		Engine:   targetDB.Engine,
		Username: dbCreds.Username,
		Password: dbCreds.Password,
		// Database name to ingest into. When the user is restoring
		// into a DIFFERENT row than the source, use the target's own
		// credentials.Database — the dump file is engine-format
		// portable and gets replayed under whatever DB the ingest
		// CLI is pointed at. (We DON'T re-use the source's
		// EffectiveDatabaseName here, because that would make
		// `restore prod → staging` try to write into `prod_db`
		// inside staging, which usually doesn't exist.)
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
		"database_id":        db.ID,
		"backup_id":          b.ID,
		"run_id":             run.ID,
		"server_id":          server.ID,
		"team_id":            teamID,
		"target_database_id": targetDB.ID,
	})
	return nil
}

// resolveRestoreTarget picks which database row receives the restored
// snapshot. If no override is provided we fall back to the source DB
// (today's behaviour). Otherwise the target is validated against
// three rules — fail-loud, not silent — so a typo'd ID or a
// cross-engine attempt aborts cleanly:
//
//  1. The target must exist + belong to the caller's team.
//  2. It must live on the SAME server as the source (cross-server
//     restore is out of scope: the SSH session ties us to one host).
//  3. It must use the SAME ENGINE family — Postgres dumps don't
//     replay into MySQL. Mongo / Redis don't even share dump format.
//
// Returns the resolved database + its project (the project is needed
// to compute the container name `launch-db-<project>-<db>`).
func (s *BackupService) resolveRestoreTarget(
	ctx context.Context,
	source *models.Database,
	teamID, serverID string,
	targetID *string,
) (*models.Database, *models.Project, error) {
	if targetID == nil || *targetID == "" || *targetID == source.ID {
		// Default path — same-database restore.
		sourceProject, err := s.Repos().Project().FindByIDAndTeamServer(
			ctx, source.ProjectID, teamID, serverID,
		)
		if err != nil {
			return nil, nil, err
		}
		return source, sourceProject, nil
	}

	target, err := s.Repos().Database().FindByIDAndTeamServer(ctx, *targetID, teamID, serverID)
	if err != nil {
		return nil, nil, fiberutil.BadRequest("Target database not found on this server")
	}
	if target.ServerID != source.ServerID {
		// Defence-in-depth — FindByIDAndTeamServer already scoped to
		// serverID, but re-checking against the source's server makes
		// the intent obvious if the scope helper ever changes.
		return nil, nil, fiberutil.BadRequest("Cross-server restore is not supported")
	}
	if target.Engine != source.Engine {
		return nil, nil, fiberutil.BadRequest(fmt.Sprintf(
			"Cannot restore a %s backup into a %s database",
			source.Engine, target.Engine,
		))
	}

	targetProject, err := s.Repos().Project().FindByIDAndTeamServer(
		ctx, target.ProjectID, teamID, serverID,
	)
	if err != nil {
		return nil, nil, err
	}
	return target, targetProject, nil
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

// pruneRunsAndObjects enforces the retention cap on a backup config.
// Deletes BOTH the oldest run rows (in MySQL) AND the corresponding
// remote S3 objects (via `aws s3 rm` on the docker server). The
// previous implementation only deleted rows, which silently leaked
// storage cost as runs accumulated.
//
// Best-effort by design:
//   - If S3 deletion fails for a key the row stays in the DB so we
//     don't orphan the object; next prune retries.
//   - If row deletion fails after a successful S3 delete the run row
//     is harmless (its object_key already points at a 404). The next
//     prune cycle re-evaluates retention from scratch.
//
// Mirrors the inline duplicate in jobs/run_backup.go — the prune
// invariant has to be enforced on both the manual ("Run now") and
// scheduled paths, and the jobs package can't import services. The
// duplication is small (~30 lines) and the alternative is a third
// helper package that nothing else would use.
func (s *BackupService) pruneRunsAndObjects(
	ctx context.Context,
	server interface {
		ConnectionAsRoot() *taskrunner.Connection
	},
	b *models.DatabaseBackup,
	s3Creds backupmodels.S3Credentials,
) error {
	if b.Retention <= 0 {
		return nil
	}
	stale, err := s.Repos().BackupRun().ListStaleForRetention(ctx, b.ID, b.Retention)
	if err != nil {
		return err
	}
	if len(stale) == 0 {
		return nil
	}

	// Collect the object keys we plan to delete. Skip rows that never
	// uploaded an object (failed runs); those have no S3 footprint.
	objectKeys := make([]string, 0, len(stale))
	for i := range stale {
		if stale[i].ObjectKey != nil && *stale[i].ObjectKey != "" {
			objectKeys = append(objectKeys, *stale[i].ObjectKey)
		}
	}

	// 1. Delete remote objects. If this fails we bail BEFORE deleting
	//    DB rows so the next prune retries — losing the row but not the
	//    object orphans the storage.
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
		if _, err := dispatchTaskAsRoot(s.BaseService, ctx, server, task); err != nil {
			return fmt.Errorf("prune remote objects: %w", err)
		}
	}

	// 2. Delete the DB rows. Per-row delete (vs. a bulk WHERE …) so
	//    one bad row doesn't take down the whole sweep.
	for i := range stale {
		_ = s.Repos().BackupRun().Delete(ctx, stale[i].ID)
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

// normaliseOptionalString trims a *string and returns nil when the
// result is empty. Used for the optional DatabaseName override so we
// don't persist " " or "" as a non-null override that would later
// cause the dump command to target a literally-named "" database.
func normaliseOptionalString(s *string) *string {
	if s == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*s)
	if trimmed == "" {
		return nil
	}
	return &trimmed
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

// dispatchBackupSuccessNotification fires the success notification for
// a synchronous "Run now" path when the user opted in via
// NotifyOnSuccess. Mirrors the worker-side helper in
// jobs/run_backup.go so the manual + scheduled paths produce
// identical emails / Slack messages. Best-effort: a missing notifier
// or a notification failure doesn't roll back the successful run.
func (s *BackupService) dispatchBackupSuccessNotification(
	ctx context.Context,
	b *models.DatabaseBackup,
	db *models.Database,
	server *servermodels.Server,
	project *models.Project,
	objectKey string,
	sizeBytes int64,
) {
	if !b.NotifyOnSuccess {
		return
	}
	notifier := s.Notifier()
	if notifier == nil {
		return
	}

	projectName := ""
	if project != nil {
		projectName = project.Name
	}

	notif := dockernotifications.
		NewDatabaseBackupSucceededNotification(db.Name, projectName, server.Name, string(db.Engine)).
		WithObjectKey(objectKey).
		WithStorageProvider(s.lookupStorageProviderLabel(ctx, b.StorageProviderID, b.TeamID)).
		WithSource("manual")
	if sizeBytes > 0 {
		notif.WithSizeBytes(sizeBytes)
	}

	if err := notifier.SendToTeam(ctx, b.TeamID, notif); err != nil && s.Logger != nil {
		s.Logger.Warn().Err(err).
			Str("backup_id", b.ID).
			Msg("failed to send backup success notification (manual run)")
	}
}

// dispatchBackupFailureNotification is the failure twin — same
// routing, same channels, fired when NotifyOnFailure is set (default
// true). The error output is truncated by the notification's
// WithError helper so an OOM dump doesn't blow up Slack message
// limits.
func (s *BackupService) dispatchBackupFailureNotification(
	ctx context.Context,
	b *models.DatabaseBackup,
	db *models.Database,
	server *servermodels.Server,
	errOutput string,
) {
	if !b.NotifyOnFailure {
		return
	}
	notifier := s.Notifier()
	if notifier == nil {
		return
	}

	notif := dockernotifications.
		NewDatabaseBackupFailedNotification(db.Name, "", server.Name, string(db.Engine)).
		WithError(errOutput).
		WithStorageProvider(s.lookupStorageProviderLabel(ctx, b.StorageProviderID, b.TeamID)).
		WithSource("manual")

	if err := notifier.SendToTeam(ctx, b.TeamID, notif); err != nil && s.Logger != nil {
		s.Logger.Warn().Err(err).
			Str("backup_id", b.ID).
			Msg("failed to send backup failure notification (manual run)")
	}
}

// lookupStorageProviderLabel resolves the human label for a storage
// provider so the notification body reads "uploaded to <Contabo>"
// instead of a numeric ID. Same logic as the worker-side helper;
// duplicated rather than shared because the service and job packages
// can't import each other (cycle).
func (s *BackupService) lookupStorageProviderLabel(ctx context.Context, providerID uint64, teamID string) string {
	if s.BackupRepos() == nil {
		return ""
	}
	p, err := s.BackupRepos().StorageProvider().FindStorageProviderByID(ctx, providerID)
	if err != nil || p == nil || p.TeamID != teamID {
		return ""
	}
	if p.Label == nil {
		return ""
	}
	return *p.Label
}
