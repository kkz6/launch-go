package repositories

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/modules/backup/models"
	backuptypes "github.com/kkz6/launch-go/internal/modules/backup/types"
)

func newBackupJobRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:backup-job-repository-%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.BackupJob{}))
	return db
}

func createBackupJobRepositoryTestRow(
	t *testing.T,
	db *gorm.DB,
	status backuptypes.BackupJobStatus,
) *models.BackupJob {
	t.Helper()
	job := &models.BackupJob{
		Status:            status,
		BackupID:          "backup000000000000000000001",
		StorageProviderID: 71,
	}
	job.TeamID = "team00000000000000000000001"
	require.NoError(t, db.Create(job).Error)
	return job
}

func TestBackupJobRepositoryClaimsPendingOnceAndResumesOnlyRetry(t *testing.T) {
	db := newBackupJobRepositoryTestDB(t)
	repo := NewBackupJobRepository(db)
	job := createBackupJobRepositoryTestRow(t, db, backuptypes.BackupJobStatusPending)

	claimedJob, claimed, err := repo.ClaimPendingBackupJobForRun(
		context.Background(), job.ID, job.BackupID, job.TeamID, false,
	)
	require.NoError(t, err)
	require.True(t, claimed)
	require.Equal(t, backuptypes.BackupJobStatusRunning, claimedJob.Status)

	_, claimed, err = repo.ClaimPendingBackupJobForRun(
		context.Background(), job.ID, job.BackupID, job.TeamID, false,
	)
	require.NoError(t, err)
	require.False(t, claimed, "a duplicate first delivery must not resume a running row")

	resumedJob, claimed, err := repo.ClaimPendingBackupJobForRun(
		context.Background(), job.ID, job.BackupID, job.TeamID, true,
	)
	require.NoError(t, err)
	require.True(t, claimed, "an identified retry may resume its exact running row")
	require.Equal(t, job.ID, resumedJob.ID)
	require.Equal(t, backuptypes.BackupJobStatusRunning, resumedJob.Status)
}

func TestBackupJobRepositoryFindsRunOnlyWithinScope(t *testing.T) {
	db := newBackupJobRepositoryTestDB(t)
	repo := NewBackupJobRepository(db)
	job := createBackupJobRepositoryTestRow(t, db, backuptypes.BackupJobStatusPending)

	found, err := repo.FindBackupJobForRun(
		context.Background(), job.ID, job.BackupID, job.TeamID,
	)
	require.NoError(t, err)
	require.Equal(t, job.ID, found.ID)

	for _, scope := range []struct {
		backupID string
		teamID   string
	}{
		{backupID: "other-backup", teamID: job.TeamID},
		{backupID: job.BackupID, teamID: "other-team"},
	} {
		_, err := repo.FindBackupJobForRun(
			context.Background(), job.ID, scope.backupID, scope.teamID,
		)
		require.Error(t, err)
	}
}

func TestBackupJobRepositoryClaimRejectsWrongScopeAndTerminalRows(t *testing.T) {
	tests := []struct {
		name     string
		status   backuptypes.BackupJobStatus
		backupID string
		teamID   string
	}{
		{
			name:     "wrong backup",
			status:   backuptypes.BackupJobStatusPending,
			backupID: "another-backup",
			teamID:   "team00000000000000000000001",
		},
		{
			name:     "wrong team",
			status:   backuptypes.BackupJobStatusPending,
			backupID: "backup000000000000000000001",
			teamID:   "another-team",
		},
		{
			name:     "finished",
			status:   backuptypes.BackupJobStatusFinished,
			backupID: "backup000000000000000000001",
			teamID:   "team00000000000000000000001",
		},
		{
			name:     "failed",
			status:   backuptypes.BackupJobStatusFailed,
			backupID: "backup000000000000000000001",
			teamID:   "team00000000000000000000001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newBackupJobRepositoryTestDB(t)
			repo := NewBackupJobRepository(db)
			job := createBackupJobRepositoryTestRow(t, db, tt.status)

			_, claimed, err := repo.ClaimPendingBackupJobForRun(
				context.Background(), job.ID, tt.backupID, tt.teamID, true,
			)
			require.NoError(t, err)
			require.False(t, claimed)

			var persisted models.BackupJob
			require.NoError(t, db.First(&persisted, "id = ?", job.ID).Error)
			require.Equal(t, tt.status, persisted.Status)
		})
	}
}

func TestBackupJobRepositoryTerminalTransitionsCannotBeOverwritten(t *testing.T) {
	db := newBackupJobRepositoryTestDB(t)
	repo := NewBackupJobRepository(db)
	job := createBackupJobRepositoryTestRow(t, db, backuptypes.BackupJobStatusRunning)
	taskID := "task00000000000000000000001"
	size := 4096

	updated, err := repo.MarkBackupJobFinishedForRun(
		context.Background(), job.ID, job.BackupID, job.TeamID, &size, &taskID,
	)
	require.NoError(t, err)
	require.True(t, updated)

	updated, err = repo.MarkBackupJobFailedForRun(
		context.Background(), job.ID, job.BackupID, job.TeamID, "late failure", nil,
	)
	require.NoError(t, err)
	require.False(t, updated)
	updated, err = repo.MarkBackupJobRunningForRun(
		context.Background(), job.ID, job.BackupID, job.TeamID, "late-task",
	)
	require.NoError(t, err)
	require.False(t, updated)

	var persisted models.BackupJob
	require.NoError(t, db.First(&persisted, "id = ?", job.ID).Error)
	require.Equal(t, backuptypes.BackupJobStatusFinished, persisted.Status)
	require.Equal(t, &size, persisted.Size)
	require.Equal(t, &taskID, persisted.TaskID)
	require.Nil(t, persisted.Error)
}

func TestBackupJobRepositoryFailureTransitionPersistsTask(t *testing.T) {
	db := newBackupJobRepositoryTestDB(t)
	repo := NewBackupJobRepository(db)
	job := createBackupJobRepositoryTestRow(t, db, backuptypes.BackupJobStatusPending)
	taskID := "task-failed"

	updated, err := repo.MarkBackupJobFailedForRun(
		context.Background(), job.ID, job.BackupID, job.TeamID, "upload failed", &taskID,
	)
	require.NoError(t, err)
	require.True(t, updated)

	var persisted models.BackupJob
	require.NoError(t, db.First(&persisted, "id = ?", job.ID).Error)
	require.Equal(t, backuptypes.BackupJobStatusFailed, persisted.Status)
	require.Equal(t, &taskID, persisted.TaskID)
	require.NotNil(t, persisted.Error)
	require.Equal(t, "upload failed", *persisted.Error)
}

func TestBackupJobRepositoryReturnsClaimPersistenceErrors(t *testing.T) {
	t.Run("claim update", func(t *testing.T) {
		db := newBackupJobRepositoryTestDB(t)
		repo := NewBackupJobRepository(db)
		job := createBackupJobRepositoryTestRow(t, db, backuptypes.BackupJobStatusPending)
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())

		_, claimed, err := repo.ClaimPendingBackupJobForRun(
			context.Background(), job.ID, job.BackupID, job.TeamID, false,
		)
		require.Error(t, err)
		require.False(t, claimed)
	})

	t.Run("claimed row reload", func(t *testing.T) {
		db := newBackupJobRepositoryTestDB(t)
		repo := NewBackupJobRepository(db)
		job := createBackupJobRepositoryTestRow(t, db, backuptypes.BackupJobStatusPending)
		require.NoError(t, db.Exec(`
			CREATE TRIGGER remove_claimed_backup_job
			AFTER UPDATE OF status ON backup_jobs
			WHEN NEW.status = 'running'
			BEGIN
				DELETE FROM backup_jobs WHERE id = NEW.id;
			END
		`).Error)

		claimedJob, claimed, err := repo.ClaimPendingBackupJobForRun(
			context.Background(), job.ID, job.BackupID, job.TeamID, false,
		)
		require.Error(t, err)
		require.True(t, claimed)
		require.Nil(t, claimedJob)
	})

	t.Run("retry reload", func(t *testing.T) {
		db := newBackupJobRepositoryTestDB(t)
		repo := NewBackupJobRepository(db)
		job := createBackupJobRepositoryTestRow(t, db, backuptypes.BackupJobStatusRunning)
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, db.Callback().Update().After("gorm:update").Register(
			"test:close_after_claim", func(*gorm.DB) { _ = sqlDB.Close() },
		))

		claimedJob, claimed, err := repo.ClaimPendingBackupJobForRun(
			context.Background(), job.ID, job.BackupID, job.TeamID, true,
		)
		require.Error(t, err)
		require.False(t, claimed)
		require.Nil(t, claimedJob)
	})
}
