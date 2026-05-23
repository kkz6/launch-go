package repositories

import (
	"context"
	"errors"

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
