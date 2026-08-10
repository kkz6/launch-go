package repositories

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
)

func newBackupRunScopeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:docker-backup-run-scope-%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.DatabaseBackup{}, &models.DatabaseBackupRun{}))
	return db
}

func createBackupRunScopeRows(t *testing.T, db *gorm.DB, status string) (*models.DatabaseBackup, *models.DatabaseBackupRun) {
	t.Helper()
	backup := &models.DatabaseBackup{DatabaseID: "database-01", StorageProviderID: 1}
	backup.ID = "backup-01"
	backup.TeamID = "team-01"
	require.NoError(t, db.Create(backup).Error)
	run := &models.DatabaseBackupRun{BackupID: backup.ID, Status: status}
	run.ID = "run-01"
	require.NoError(t, db.Create(run).Error)
	return backup, run
}

func TestBackupRunRepositoryClaimsAndScopesTriggeredRun(t *testing.T) {
	db := newBackupRunScopeTestDB(t)
	repo := NewBackupRunRepository(db)
	backup, run := createBackupRunScopeRows(t, db, "triggered")
	startedAt := time.Date(2026, 8, 10, 9, 10, 0, 0, time.UTC)

	claimedRun, claimed, err := repo.ClaimTriggeredForBackup(
		context.Background(), run.ID, backup.ID, backup.TeamID, startedAt, false,
	)
	require.NoError(t, err)
	require.True(t, claimed)
	require.Equal(t, "running", claimedRun.Status)
	require.NotNil(t, claimedRun.StartedAt)
	require.True(t, claimedRun.StartedAt.Equal(startedAt))

	_, claimed, err = repo.ClaimTriggeredForBackup(
		context.Background(), run.ID, backup.ID, backup.TeamID, time.Now(), false,
	)
	require.NoError(t, err)
	require.False(t, claimed)

	resumed, claimed, err := repo.ClaimTriggeredForBackup(
		context.Background(), run.ID, backup.ID, backup.TeamID, time.Now(), true,
	)
	require.NoError(t, err)
	require.True(t, claimed)
	require.Equal(t, run.ID, resumed.ID)

	_, claimed, err = repo.ClaimTriggeredForBackup(
		context.Background(), run.ID, backup.ID, "other-team", time.Now(), true,
	)
	require.NoError(t, err)
	require.False(t, claimed)
}

func TestBackupRunRepositoryAttachesAndTerminalizesOnlyActiveScopedRuns(t *testing.T) {
	db := newBackupRunScopeTestDB(t)
	repo := NewBackupRunRepository(db)
	backup, run := createBackupRunScopeRows(t, db, "triggered")

	attached, err := repo.AttachTaskForBackup(
		context.Background(), run.ID, backup.ID, backup.TeamID, "task-01",
	)
	require.NoError(t, err)
	require.True(t, attached)

	failed, err := repo.MarkFailedForBackup(
		context.Background(), run.ID, backup.ID, backup.TeamID,
		map[string]any{"status": "failed", "error": "upload failed"},
	)
	require.NoError(t, err)
	require.True(t, failed)

	attached, err = repo.AttachTaskForBackup(
		context.Background(), run.ID, backup.ID, backup.TeamID, "late-task",
	)
	require.NoError(t, err)
	require.False(t, attached)
	transitioned, err := repo.MarkTerminalForBackup(
		context.Background(), run.ID, backup.ID, backup.TeamID,
		map[string]any{"status": "success"},
	)
	require.NoError(t, err)
	require.False(t, transitioned)

	var persisted models.DatabaseBackupRun
	require.NoError(t, db.First(&persisted, "id = ?", run.ID).Error)
	require.Equal(t, "failed", persisted.Status)
	require.NotNil(t, persisted.TaskID)
	require.Equal(t, "task-01", *persisted.TaskID)
	require.NotNil(t, persisted.Error)
	require.Equal(t, "upload failed", *persisted.Error)
}

func TestBackupRunRepositoryReturnsPersistenceErrors(t *testing.T) {
	t.Run("claim update", func(t *testing.T) {
		db := newBackupRunScopeTestDB(t)
		repo := NewBackupRunRepository(db)
		backup, run := createBackupRunScopeRows(t, db, "triggered")
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())

		_, claimed, err := repo.ClaimTriggeredForBackup(
			context.Background(), run.ID, backup.ID, backup.TeamID, time.Now(), false,
		)
		require.Error(t, err)
		require.False(t, claimed)
	})

	t.Run("claimed row reload", func(t *testing.T) {
		db := newBackupRunScopeTestDB(t)
		repo := NewBackupRunRepository(db)
		backup, run := createBackupRunScopeRows(t, db, "triggered")
		require.NoError(t, db.Exec(`
			CREATE TRIGGER remove_claimed_database_backup_run
			AFTER UPDATE OF status ON docker_database_backup_runs
			WHEN NEW.status = 'running'
			BEGIN
				DELETE FROM docker_database_backup_runs WHERE id = NEW.id;
			END
		`).Error)

		claimedRun, claimed, err := repo.ClaimTriggeredForBackup(
			context.Background(), run.ID, backup.ID, backup.TeamID, time.Now(), false,
		)
		require.Error(t, err)
		require.True(t, claimed)
		require.Nil(t, claimedRun)
	})

	t.Run("retry reload", func(t *testing.T) {
		db := newBackupRunScopeTestDB(t)
		repo := NewBackupRunRepository(db)
		backup, run := createBackupRunScopeRows(t, db, "running")
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, db.Callback().Update().After("gorm:update").Register(
			"test:close_after_claim", func(*gorm.DB) { _ = sqlDB.Close() },
		))

		claimedRun, claimed, err := repo.ClaimTriggeredForBackup(
			context.Background(), run.ID, backup.ID, backup.TeamID, time.Now(), true,
		)
		require.Error(t, err)
		require.False(t, claimed)
		require.Nil(t, claimedRun)
	})

	t.Run("active transitions", func(t *testing.T) {
		db := newBackupRunScopeTestDB(t)
		repo := NewBackupRunRepository(db)
		backup, run := createBackupRunScopeRows(t, db, "running")
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())

		attached, err := repo.AttachTaskForBackup(
			context.Background(), run.ID, backup.ID, backup.TeamID, "task-01",
		)
		require.Error(t, err)
		require.False(t, attached)
		transitioned, err := repo.MarkTerminalForBackup(
			context.Background(), run.ID, backup.ID, backup.TeamID,
			map[string]any{"status": "success"},
		)
		require.Error(t, err)
		require.False(t, transitioned)
	})
}
