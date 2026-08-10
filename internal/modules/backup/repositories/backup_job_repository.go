package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/backup/models"
	backuptypes "github.com/kkz6/launch-go/internal/modules/backup/types"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// BackupJobRepository handles database operations for backup jobs
type BackupJobRepository struct {
	repository.Base[models.BackupJob]
}

// NewBackupJobRepository creates a new backup job repository
func NewBackupJobRepository(db *gorm.DB) *BackupJobRepository {
	return &BackupJobRepository{
		Base: repository.NewBase[models.BackupJob](db),
	}
}

// CreateBackupJob creates a new backup job
func (r *BackupJobRepository) CreateBackupJob(ctx context.Context, job *models.BackupJob) error {
	return r.DB.WithContext(ctx).Create(job).Error
}

// FindBackupJobByID finds a backup job by ID
func (r *BackupJobRepository) FindBackupJobByID(ctx context.Context, id string) (*models.BackupJob, error) {
	return repository.FindOne[models.BackupJob](ctx, r.DB,
		repository.WithID(id),
		repository.Preload("Backup"),
		repository.Preload("StorageProvider"),
	)
}

// FindBackupJobForRun finds a scoped backup job without preloads.
func (r *BackupJobRepository) FindBackupJobForRun(
	ctx context.Context,
	id, backupID, teamID string,
) (*models.BackupJob, error) {
	return repository.FindOne[models.BackupJob](ctx, r.DB,
		repository.WithID(id),
		repository.WithTeamID(teamID),
		func(db *gorm.DB) *gorm.DB {
			return db.Where("backup_id = ?", backupID)
		},
	)
}

// ClaimPendingBackupJobForRun atomically claims a scoped pending job.
func (r *BackupJobRepository) ClaimPendingBackupJobForRun(
	ctx context.Context,
	id, backupID, teamID string,
	allowRunning bool,
) (*models.BackupJob, bool, error) {
	result := r.DB.WithContext(ctx).
		Model(&models.BackupJob{}).
		Where("id = ? AND backup_id = ? AND team_id = ?", id, backupID, teamID).
		Where("status = ?", backuptypes.BackupJobStatusPending).
		Update("status", backuptypes.BackupJobStatusRunning)
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected != 1 {
		if !allowRunning {
			return nil, false, nil
		}

		var running models.BackupJob
		err := r.DB.WithContext(ctx).
			Where("id = ? AND backup_id = ? AND team_id = ?", id, backupID, teamID).
			Where("status = ?", backuptypes.BackupJobStatusRunning).
			First(&running).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		if err != nil {
			return nil, false, err
		}
		return &running, true, nil
	}

	var job models.BackupJob
	if err := r.DB.WithContext(ctx).
		Where("id = ? AND backup_id = ? AND team_id = ?", id, backupID, teamID).
		First(&job).Error; err != nil {
		return nil, true, err
	}
	return &job, true, nil
}

// MarkBackupJobRunningForRun attaches a task to an active job.
func (r *BackupJobRepository) MarkBackupJobRunningForRun(
	ctx context.Context,
	id, backupID, teamID, taskID string,
) (bool, error) {
	result := r.DB.WithContext(ctx).
		Model(&models.BackupJob{}).
		Where("id = ? AND backup_id = ? AND team_id = ?", id, backupID, teamID).
		Where("status IN ?", []backuptypes.BackupJobStatus{
			backuptypes.BackupJobStatusPending,
			backuptypes.BackupJobStatusRunning,
		}).
		Updates(map[string]any{
			"status":  backuptypes.BackupJobStatusRunning,
			"task_id": taskID,
		})
	return result.RowsAffected == 1, result.Error
}

// MarkBackupJobFinishedForRun completes a running job.
func (r *BackupJobRepository) MarkBackupJobFinishedForRun(
	ctx context.Context,
	id, backupID, teamID string,
	size *int,
	taskID *string,
) (bool, error) {
	updates := map[string]any{"status": backuptypes.BackupJobStatusFinished}
	if size != nil {
		updates["size"] = *size
	}
	if taskID != nil {
		updates["task_id"] = *taskID
	}
	result := r.DB.WithContext(ctx).
		Model(&models.BackupJob{}).
		Where("id = ? AND backup_id = ? AND team_id = ?", id, backupID, teamID).
		Where("status = ?", backuptypes.BackupJobStatusRunning).
		Updates(updates)
	return result.RowsAffected == 1, result.Error
}

// MarkBackupJobFailedForRun fails an active scoped job.
func (r *BackupJobRepository) MarkBackupJobFailedForRun(
	ctx context.Context,
	id, backupID, teamID, message string,
	taskID *string,
) (bool, error) {
	updates := map[string]any{
		"status": backuptypes.BackupJobStatusFailed,
		"error":  message,
	}
	if taskID != nil {
		updates["task_id"] = *taskID
	}
	result := r.DB.WithContext(ctx).
		Model(&models.BackupJob{}).
		Where("id = ? AND backup_id = ? AND team_id = ?", id, backupID, teamID).
		Where("status IN ?", []backuptypes.BackupJobStatus{
			backuptypes.BackupJobStatusPending,
			backuptypes.BackupJobStatusRunning,
		}).
		Updates(updates)
	return result.RowsAffected == 1, result.Error
}

// FindBackupJobsByBackupID finds all jobs for a backup
func (r *BackupJobRepository) FindBackupJobsByBackupID(ctx context.Context, backupID string) ([]models.BackupJob, error) {
	var jobs []models.BackupJob
	err := r.DB.WithContext(ctx).
		Where("backup_id = ?", backupID).
		Order("created_at DESC").
		Find(&jobs).Error

	return jobs, err
}

// FindFinishedBackupJobs finds finished jobs for a backup
func (r *BackupJobRepository) FindFinishedBackupJobs(ctx context.Context, backupID string) ([]models.BackupJob, error) {
	var jobs []models.BackupJob
	err := r.DB.WithContext(ctx).
		Where("backup_id = ? AND status = ?", backupID, backuptypes.BackupJobStatusFinished).
		Order("created_at DESC").
		Find(&jobs).Error

	return jobs, err
}

// UpdateBackupJob updates a backup job
func (r *BackupJobRepository) UpdateBackupJob(ctx context.Context, job *models.BackupJob) error {
	return r.DB.WithContext(ctx).Save(job).Error
}

// DeleteBackupJob deletes a backup job
func (r *BackupJobRepository) DeleteBackupJob(ctx context.Context, id string) error {
	return r.DB.WithContext(ctx).Delete(&models.BackupJob{}, "id = ?", id).Error
}

// GetBackupJobsTotalSize gets the total size of all finished jobs for a backup
func (r *BackupJobRepository) GetBackupJobsTotalSize(ctx context.Context, backupID string) (int64, error) {
	var totalSize int64
	err := r.DB.WithContext(ctx).
		Model(&models.BackupJob{}).
		Where("backup_id = ? AND status = ?", backupID, backuptypes.BackupJobStatusFinished).
		Select("COALESCE(SUM(size), 0)").
		Scan(&totalSize).Error

	return totalSize, err
}
