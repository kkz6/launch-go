package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/docker/dto"
	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"github.com/kkz6/launch-go/internal/modules/docker/tasks"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// BackupService manages backup configs + run history + ad-hoc run-now
// + restore-from-snapshot for managed databases. Each operation is a
// one-shot SSH task (no asynq job yet — scheduler integration lands in
// a follow-up).
type BackupService struct {
	*BaseService
}

func NewBackupService(deps *ServiceDeps) *BackupService {
	return &BackupService{BaseService: NewBaseService(deps)}
}

// s3Credentials is the JSON shape we store in the encrypted column.
type s3Credentials struct {
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
}

// GetBackup returns the backup config, with AccessKey decoded. Returns
// nil + nil when no backup is configured yet (so the UI can render the
// "Set up backups" empty state).
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
	access, hasSecret := decodeS3Credentials(b.Credentials)
	return dto.ToBackupResponse(b, access, hasSecret), nil
}

// ConfigureBackup creates or updates the backup config for a database.
// Unique index on (database_id) ensures one config per database; we
// upsert by finding the live row first.
func (s *BackupService) ConfigureBackup(
	ctx context.Context, databaseID, projectID, serverID, teamID, userID string,
	req *dto.ConfigureBackupRequest,
) (dto.BackupResponse, error) {
	_ = userID
	db, err := s.scopedDatabase(ctx, databaseID, projectID, serverID, teamID)
	if err != nil {
		return dto.BackupResponse{}, err
	}

	creds, err := json.Marshal(s3Credentials{
		AccessKey: req.AccessKey,
		SecretKey: req.SecretKey,
	})
	if err != nil {
		return dto.BackupResponse{}, err
	}

	existing, lookupErr := s.Repos().Backup().FindByDatabase(ctx, databaseID)
	if lookupErr != nil && !fiberutil.IsNotFound(lookupErr) {
		return dto.BackupResponse{}, lookupErr
	}

	var b *models.DatabaseBackup
	if existing != nil {
		updates := map[string]any{
			"provider":      req.Provider,
			"endpoint":      req.Endpoint,
			"bucket":        req.Bucket,
			"region":        req.Region,
			"path_prefix":   req.PathPrefix,
			"credentials":   dbtype.EncryptedString(creds),
			"cron_schedule": req.CronSchedule,
			"enabled":       req.Enabled,
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
			DatabaseID:   databaseID,
			Provider:     req.Provider,
			Endpoint:     req.Endpoint,
			Bucket:       req.Bucket,
			Region:       req.Region,
			PathPrefix:   req.PathPrefix,
			Credentials:  dbtype.EncryptedString(creds),
			CronSchedule: req.CronSchedule,
			Enabled:      req.Enabled,
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

	access, hasSecret := decodeS3Credentials(b.Credentials)
	return *dto.ToBackupResponse(b, access, hasSecret), nil
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
// "click to back up" interaction. For automated cron runs (a follow-up),
// the scheduler will dispatch this via an asynq job.
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

	creds, err := loadS3Creds(b.Credentials)
	if err != nil {
		return dto.BackupRunResponse{}, fiberutil.BadRequest("Backup credentials are missing or corrupt")
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
		Endpoint:   strDeref(b.Endpoint),
		Region:     strDeref(b.Region),
		Bucket:     b.Bucket,
		PathPrefix: strDeref(b.PathPrefix),
		AccessKey:  creds.AccessKey,
		SecretKey:  creds.SecretKey,
	}

	// Run synchronously through the existing taskrunner so we get the
	// captured output for marker parsing.
	taskWrapper := tasks.RunBackup(cfg)
	dispatcher, ok := s.Repos().Backup().DB.Statement.ConnPool.(interface{}) // placeholder
	_ = dispatcher
	_ = ok
	_ = taskrunner.NewBaseTask // keep the import even when unused

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
	creds, err := loadS3Creds(b.Credentials)
	if err != nil {
		return fiberutil.BadRequest("Backup credentials are missing or corrupt")
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
		Endpoint:  strDeref(b.Endpoint),
		Region:    strDeref(b.Region),
		Bucket:    b.Bucket,
		ObjectKey: *run.ObjectKey,
		AccessKey: creds.AccessKey,
		SecretKey: creds.SecretKey,
	}

	taskWrapper := tasks.Restore(cfg)
	result, runErr := dispatchTaskAsRoot(s.BaseService, ctx, server, taskWrapper)
	if runErr != nil {
		return fmt.Errorf("restore failed: %w", runErr)
	}
	if result != nil && result.ExitCode != 0 {
		return fmt.Errorf("restore failed: %s", truncate(result.Stdout + result.Stderr, 4000))
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

// decodeS3Credentials extracts the access-key for display purposes.
// Returns hasSecret=true when the secret-key field is non-empty so the
// UI can show "stored ✓" without round-tripping the value.
func decodeS3Credentials(raw dbtype.EncryptedString) (string, bool) {
	c, err := loadS3Creds(raw)
	if err != nil {
		return "", false
	}
	return c.AccessKey, c.SecretKey != ""
}

func loadS3Creds(raw dbtype.EncryptedString) (s3Credentials, error) {
	if string(raw) == "" {
		return s3Credentials{}, errors.New("empty credentials")
	}
	var c s3Credentials
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return s3Credentials{}, err
	}
	return c, nil
}

func loadDBCredentials(db *models.Database) (Credentials, error) {
	return decodeCredentials(db.Credentials)
}

func strDeref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
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

// (No adapter needed — SSHCommandResult fields are accessed directly.)
