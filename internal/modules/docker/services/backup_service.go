package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	backupmodels "github.com/kkz6/launch-go/internal/modules/backup/models"
	backuptypes "github.com/kkz6/launch-go/internal/modules/backup/types"
	"github.com/kkz6/launch-go/internal/modules/docker/dto"
	"github.com/kkz6/launch-go/internal/modules/docker/jobs"
	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"github.com/kkz6/launch-go/internal/modules/docker/tasks"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	pkgservice "github.com/kkz6/launch-go/internal/pkg/service"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// BackupService manages database backup configuration and runs.
type BackupService struct {
	*BaseService
	dispatchBackup func(pkgservice.TaskFactory) error
}

func NewBackupService(deps *ServiceDeps) *BackupService {
	service := &BackupService{BaseService: NewBaseService(deps)}
	service.dispatchBackup = service.MustDispatch
	return service
}

// GetBackup returns a database's backup configuration.
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

// ConfigureBackup creates or updates a database backup configuration.
func (s *BackupService) ConfigureBackup(
	ctx context.Context, databaseID, projectID, serverID, teamID, userID string,
	req *dto.ConfigureBackupRequest,
) (dto.BackupResponse, error) {
	_ = userID
	db, err := s.scopedDatabase(ctx, databaseID, projectID, serverID, teamID)
	if err != nil {
		return dto.BackupResponse{}, err
	}

	if _, err := s.loadTeamStorageProvider(ctx, req.StorageProviderID, teamID); err != nil {
		return dto.BackupResponse{}, err
	}

	existing, lookupErr := s.Repos().Backup().FindByDatabase(ctx, databaseID)
	if lookupErr != nil && !fiberutil.IsNotFound(lookupErr) {
		return dto.BackupResponse{}, lookupErr
	}

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

// DeleteBackup removes a database backup configuration.
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
	return mapResponseValues(rows, dto.ToBackupRunResponse), nil
}

// RunNow persists and queues an immediate backup.
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

	now := time.Now().UTC()
	run := &models.DatabaseBackupRun{
		BackupID:  b.ID,
		Status:    "triggered",
		StartedAt: &now,
	}
	if err := s.Repos().BackupRun().Create(ctx, run); err != nil {
		return dto.BackupRunResponse{}, err
	}

	dispatchErr := s.dispatchBackup(func() (*asynq.Task, error) {
		return jobs.NewRunBackupTask(b.ID, db.ID, db.ProjectID, serverID, teamID, run.ID, "manual")
	})
	if dispatchErr != nil {
		finishedAt := time.Now().UTC()
		message := "failed to enqueue database backup job: " + dispatchErr.Error()
		persisted, updateErr := s.Repos().BackupRun().MarkTerminalForBackup(ctx, run.ID, b.ID, teamID, map[string]any{
			"status":      "failed",
			"finished_at": finishedAt,
			"error":       message,
		})
		if updateErr != nil {
			s.LogError(updateErr, "failed to mark backup run enqueue failure", "run_id", run.ID)
		}
		if updateErr == nil && persisted {
			s.BroadcastToTeam(teamID, "docker.database.backup.run.failed", map[string]any{
				"database_id": db.ID,
				"project_id":  db.ProjectID,
				"backup_id":   b.ID,
				"run_id":      run.ID,
				"server_id":   serverID,
				"team_id":     teamID,
				"source":      "manual",
				"error":       message,
			})
		}
		return dto.BackupRunResponse{}, fmt.Errorf("queue database backup: %w", dispatchErr)
	}

	s.BroadcastToTeam(teamID, "docker.database.backup.run.queued", map[string]any{
		"database_id": db.ID,
		"project_id":  db.ProjectID,
		"backup_id":   b.ID,
		"run_id":      run.ID,
		"server_id":   serverID,
		"team_id":     teamID,
		"source":      "manual",
		"status":      "pending",
	})

	return *dto.ToBackupRunResponse(run), nil
}

// Restore replays a completed backup into a compatible database.
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
		// Non-AWS S3 (Contabo/MinIO/Wasabi) needs path-style addressing.
		ForcePathStyle: s3Creds.ForcePathStyle,
	}

	taskWrapper := tasks.Restore(cfg)
	result, runErr := dispatchTaskAsRoot(ctx, s.BaseService, server, taskWrapper)
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

func (s *BackupService) resolveRestoreTarget(
	ctx context.Context,
	source *models.Database,
	teamID, serverID string,
	targetID *string,
) (*models.Database, *models.Project, error) {
	if targetID == nil || *targetID == "" || *targetID == source.ID {
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

func (s *BackupService) loadTeamStorageProvider(
	ctx context.Context, id uint64, teamID string,
) (*backupmodels.StorageProvider, error) {
	if s.BackupRepos() == nil {
		return nil, fmt.Errorf("storage providers registry is not wired into docker module")
	}
	p, err := s.BackupRepos().StorageProvider().FindStorageProviderByIDAndTeam(ctx, id, teamID)
	if err != nil {
		return nil, fiberutil.BadRequest("Storage provider not found")
	}
	if p.Provider != backuptypes.StorageDriverS3 {
		return nil, fiberutil.BadRequest("Only S3-compatible storage providers can host database backups")
	}
	return p, nil
}

func (s *BackupService) loadProviderS3Creds(
	ctx context.Context, providerID uint64, teamID string,
) (backupmodels.S3Credentials, error) {
	p, err := s.loadTeamStorageProvider(ctx, providerID, teamID)
	if err != nil {
		return backupmodels.S3Credentials{}, err
	}
	credentials := p.GetCredentials()
	raw, _ := json.Marshal(credentials)
	var c backupmodels.S3Credentials
	if err := json.Unmarshal(raw, &c); err != nil {
		return backupmodels.S3Credentials{}, fmt.Errorf("decode S3 credentials: %w", err)
	}
	c.ApplyLegacyDefaults(credentials)
	if c.Bucket == "" || c.Key == "" || c.Secret == "" {
		return backupmodels.S3Credentials{}, fiberutil.BadRequest("Storage provider is missing S3 credentials")
	}
	return c, nil
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

func dispatchTaskAsRoot(
	ctx context.Context, base *BaseService,
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
