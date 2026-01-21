package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/backup/enums"
	"github.com/kkz6/launch-go/internal/modules/backup/models"
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
	var job models.BackupJob
	err := r.DB.WithContext(ctx).
		Preload("Backup").
		Preload("StorageProvider").
		First(&job, "id = ?", id).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrBackupJobNotFound
	}

	if err != nil {
		return nil, err
	}

	return &job, nil
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
		Where("backup_id = ? AND status = ?", backupID, enums.BackupJobStatusFinished).
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
		Where("backup_id = ? AND status = ?", backupID, enums.BackupJobStatusFinished).
		Select("COALESCE(SUM(size), 0)").
		Scan(&totalSize).Error

	return totalSize, err
}
