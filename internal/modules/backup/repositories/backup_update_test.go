package repositories

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/kkz6/launch-go/internal/modules/backup/dto"
	"github.com/kkz6/launch-go/internal/modules/backup/models"
)

func backupUpdateFixture(t *testing.T, databases ...*gorm.DB) (*BackupRepository, *models.Backup) {
	t.Helper()
	var db *gorm.DB
	if len(databases) > 0 {
		db = databases[0]
	} else {
		db = newBackupScopeRepositoryTestDB(t)
		require.NoError(t, db.AutoMigrate(&models.BackupDatabase{}))
	}
	repo := NewBackupRepository(db)
	backup := &models.Backup{StorageProviderID: 1, DispatchToken: "backup-update-test-token", CronExpression: "0 0 * * *", IncludeFiles: "[]", ExcludeFiles: "[]", Path: "old", Enabled: true, NotificationOnFailure: true}
	backup.ServerID, backup.TeamID = "server", "team"
	require.NoError(t, repo.CreateBackupWithDatabases(context.Background(), backup, []string{"old-db"}))
	// The update service reads and passes a backup with its existing links loaded.
	require.NoError(t, db.Preload("Databases").First(backup, "id = ?", backup.ID).Error)
	require.Len(t, backup.Databases, 1)
	// SQLite accepts targetless upserts that PostgreSQL rejects. Enforce the
	// production PostgreSQL rule to reproduce the logged SQLSTATE 42601.
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("test:postgres_conflict_target", func(tx *gorm.DB) {
		if c, ok := tx.Statement.Clauses["ON CONFLICT"].Expression.(clause.OnConflict); ok && len(c.DoUpdates) > 0 && len(c.Columns) == 0 && c.OnConstraint == "" {
			tx.AddError(errors.New("ON CONFLICT DO UPDATE requires inference specification or constraint name"))
		}
	}))
	return repo, backup
}

func TestUpdateBackupDatabaseSelectionOnPostgres(t *testing.T) {
	dsn := os.Getenv("MIGRATION_TEST_DSN")
	if dsn == "" {
		t.Skip("set MIGRATION_TEST_DSN to run the PostgreSQL backup update regression")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	// Keep this test independent of the migration suite resetting public.
	schema := fmt.Sprintf("backup_update_%d", time.Now().UnixNano())
	require.NoError(t, db.Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() {
		require.NoError(t, db.Exec("DROP SCHEMA "+schema+" CASCADE").Error)
		require.NoError(t, sqlDB.Close())
	})
	require.NoError(t, db.Exec("SET search_path TO "+schema).Error)
	require.NoError(t, db.Migrator().CreateTable(&models.Backup{}, &models.BackupDatabase{}))
	repo, backup := backupUpdateFixture(t, db)
	require.NoError(t, repo.UpdateBackupWithDatabases(context.Background(), backup, []string{"new-db"}))
	ids, err := repo.GetBackupDatabaseIDs(context.Background(), backup.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"new-db"}, ids)
	require.Equal(t, ids, dto.ToBackupResponse(backup).Databases)
}

func TestUpdateBackupReplacesLoadedDatabaseSelection(t *testing.T) {
	repo, backup := backupUpdateFixture(t)
	backup.Path, backup.CronExpression = "updated", "0 3 * * *"
	backup.Enabled, backup.NotificationOnFailure = false, false
	require.NoError(t, repo.UpdateBackupWithDatabases(context.Background(), backup, []string{"new-db"}))
	ids, err := repo.GetBackupDatabaseIDs(context.Background(), backup.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"new-db"}, ids)
	require.Equal(t, ids, dto.ToBackupResponse(backup).Databases)
	var stored models.Backup
	require.NoError(t, repo.DB.First(&stored, "id = ?", backup.ID).Error)
	require.Equal(t, "updated", stored.Path)
	require.Equal(t, "0 3 * * *", stored.CronExpression)
	require.False(t, stored.Enabled)
	require.False(t, stored.NotificationOnFailure)
	// Clearing all databases must remove the links rather than resave them.
	require.NoError(t, repo.UpdateBackupWithDatabases(context.Background(), backup, []string{}))
	ids, err = repo.GetBackupDatabaseIDs(context.Background(), backup.ID)
	require.NoError(t, err)
	require.Empty(t, ids)
	require.Empty(t, dto.ToBackupResponse(backup).Databases)
}

func TestUpdateBackupRollsBackSettingsAndDatabaseSelection(t *testing.T) {
	repo, backup := backupUpdateFixture(t)
	backup.Path = "should-not-persist"
	require.NoError(t, repo.DB.Callback().Create().Before("gorm:create").Register("test:reject_new_database", func(tx *gorm.DB) {
		if tx.Statement.Table == "backup_databases" {
			tx.AddError(errors.New("database selection rejected"))
		}
	}))
	require.Error(t, repo.UpdateBackupWithDatabases(context.Background(), backup, []string{"new-db"}))
	var stored models.Backup
	require.NoError(t, repo.DB.First(&stored, "id = ?", backup.ID).Error)
	require.Equal(t, "old", stored.Path)
	ids, err := repo.GetBackupDatabaseIDs(context.Background(), backup.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"old-db"}, ids)
	require.Equal(t, ids, dto.ToBackupResponse(backup).Databases)
}
