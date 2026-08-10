package repositories

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// BackupRepository handles persistence for database backup configs.
type BackupRepository struct {
	repository.Base[models.DatabaseBackup]
}

func NewBackupRepository(db *gorm.DB) *BackupRepository {
	return &BackupRepository{Base: repository.NewBase[models.DatabaseBackup](db)}
}

func (r *BackupRepository) FindByDatabase(ctx context.Context, databaseID string) (*models.DatabaseBackup, error) {
	var b models.DatabaseBackup
	err := r.DB.WithContext(ctx).Where("database_id = ?", databaseID).First(&b).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}
	return &b, nil
}

// ListEnabled returns every backup row with enabled=true and a non-empty
// cron_schedule. Used by the scheduler poller — see PollDueBackupsJob.
// Filters in SQL so we don't pull rows the scheduler would skip anyway.
func (r *BackupRepository) ListEnabled(ctx context.Context) ([]models.DatabaseBackup, error) {
	var rows []models.DatabaseBackup
	err := r.DB.WithContext(ctx).
		Where("enabled = ? AND cron_schedule IS NOT NULL AND cron_schedule <> ''", true).
		Find(&rows).Error
	return rows, err
}

// BackupRunRepository tracks past upload attempts.
type BackupRunRepository struct {
	repository.Base[models.DatabaseBackupRun]
}

func NewBackupRunRepository(db *gorm.DB) *BackupRunRepository {
	return &BackupRunRepository{Base: repository.NewBase[models.DatabaseBackupRun](db)}
}

// ClaimTriggeredForBackup atomically claims a scoped triggered run.
func (r *BackupRunRepository) ClaimTriggeredForBackup(
	ctx context.Context,
	id, backupID, teamID string,
	startedAt time.Time,
	allowRunning bool,
) (*models.DatabaseBackupRun, bool, error) {
	result := r.DB.WithContext(ctx).
		Model(&models.DatabaseBackupRun{}).
		Where("id = ? AND backup_id = ? AND status = ?", id, backupID, "triggered").
		Where(`EXISTS (
			SELECT 1 FROM docker_database_backups
			WHERE docker_database_backups.id = docker_database_backup_runs.backup_id
			  AND docker_database_backups.team_id = ?
		)`, teamID).
		Updates(map[string]any{
			"status":     "running",
			"started_at": startedAt,
		})
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected != 1 {
		if !allowRunning {
			return nil, false, nil
		}
		var running models.DatabaseBackupRun
		err := r.DB.WithContext(ctx).
			Where("id = ? AND backup_id = ? AND status = ?", id, backupID, "running").
			Where(`EXISTS (
				SELECT 1 FROM docker_database_backups
				WHERE docker_database_backups.id = docker_database_backup_runs.backup_id
				  AND docker_database_backups.team_id = ?
			)`, teamID).
			First(&running).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		if err != nil {
			return nil, false, err
		}
		return &running, true, nil
	}

	var run models.DatabaseBackupRun
	if err := r.DB.WithContext(ctx).
		Where("id = ? AND backup_id = ?", id, backupID).
		First(&run).Error; err != nil {
		return nil, true, err
	}
	return &run, true, nil
}

// AttachTaskForBackup links a task to an active scoped run.
func (r *BackupRunRepository) AttachTaskForBackup(
	ctx context.Context,
	id, backupID, teamID, taskID string,
) (bool, error) {
	result := r.DB.WithContext(ctx).
		Model(&models.DatabaseBackupRun{}).
		Where("id = ? AND backup_id = ?", id, backupID).
		Where("status IN ?", []string{"triggered", "running"}).
		Where(`EXISTS (
			SELECT 1 FROM docker_database_backups
			WHERE docker_database_backups.id = docker_database_backup_runs.backup_id
			  AND docker_database_backups.team_id = ?
		)`, teamID).
		Update("task_id", taskID)
	return result.RowsAffected == 1, result.Error
}

// MarkTerminalForBackup completes an active scoped run.
func (r *BackupRunRepository) MarkTerminalForBackup(
	ctx context.Context,
	id, backupID, teamID string,
	fields map[string]any,
) (bool, error) {
	result := r.DB.WithContext(ctx).
		Model(&models.DatabaseBackupRun{}).
		Where("id = ? AND backup_id = ?", id, backupID).
		Where("status IN ?", []string{"triggered", "running"}).
		Where(`EXISTS (
			SELECT 1 FROM docker_database_backups
			WHERE docker_database_backups.id = docker_database_backup_runs.backup_id
			  AND docker_database_backups.team_id = ?
		)`, teamID).
		Updates(fields)
	return result.RowsAffected == 1, result.Error
}

// MarkFailedForBackup fails an active scoped run.
func (r *BackupRunRepository) MarkFailedForBackup(
	ctx context.Context,
	id, backupID, teamID string,
	fields map[string]any,
) (bool, error) {
	return r.MarkTerminalForBackup(ctx, id, backupID, teamID, fields)
}

func (r *BackupRunRepository) ListForBackup(
	ctx context.Context, backupID string,
) ([]models.DatabaseBackupRun, error) {
	var rows []models.DatabaseBackupRun
	err := r.DB.WithContext(ctx).
		Where("backup_id = ?", backupID).
		Order("started_at DESC").
		Limit(50).
		Find(&rows).Error
	return rows, err
}

// ListStaleForRetention returns rows past the retention window, ordered
// most-recent-first — i.e. the runs the prune sweep should delete.
// Bounded at 500 to keep memory predictable; one sweep can therefore
// reclaim up to that many orphans per tick, and the next sweep picks up
// any remainder. This exists alongside ListForBackup because the UI
// listing is happy with the 50-row cap, but a long-lived backup config
// with Retention >= 50 would otherwise never have anything pruned.
func (r *BackupRunRepository) ListStaleForRetention(
	ctx context.Context, backupID string, keep int,
) ([]models.DatabaseBackupRun, error) {
	if keep < 0 {
		keep = 0
	}
	var rows []models.DatabaseBackupRun
	err := r.DB.WithContext(ctx).
		Where("backup_id = ?", backupID).
		Order("started_at DESC").
		Offset(keep).
		Limit(500).
		Find(&rows).Error
	return rows, err
}
