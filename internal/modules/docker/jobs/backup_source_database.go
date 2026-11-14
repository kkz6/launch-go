package jobs

import (
	"context"
	"errors"
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
)

var errBackupRunNotRunning = errors.New("backup run is no longer running")

// Capture once, before dispatch. Retries use the recorded name even if the
// backup override or database credentials have changed since the first attempt.
func (j *RunBackupJob) captureSourceDatabase(ctx context.Context, run *models.DatabaseBackupRun, selected string) (string, error) {
	if run.SourceDatabase != nil {
		return *run.SourceDatabase, nil
	}
	result := j.Deps.DB.WithContext(ctx).Model(&models.DatabaseBackupRun{}).
		Where("id = ? AND backup_id = ? AND status = ? AND source_database IS NULL", run.ID, run.BackupID, "running").
		Update("source_database", selected)
	if result.Error != nil {
		return "", result.Error
	}
	if result.RowsAffected == 1 {
		run.SourceDatabase = &selected
		return selected, nil
	}
	var saved models.DatabaseBackupRun
	if err := j.Deps.DB.WithContext(ctx).Where("id = ? AND backup_id = ?", run.ID, run.BackupID).First(&saved).Error; err != nil {
		return "", err
	}
	if saved.Status != "running" {
		return "", errBackupRunNotRunning
	}
	if saved.SourceDatabase == nil {
		return "", fmt.Errorf("backup source could not be recorded")
	}
	run.SourceDatabase = saved.SourceDatabase
	return *saved.SourceDatabase, nil
}
