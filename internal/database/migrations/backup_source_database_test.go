package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBackupSourceDatabaseMigrationPreservesUnknownLegacySource(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.Exec("CREATE TABLE docker_database_backup_runs (id TEXT PRIMARY KEY, status TEXT)").Error)
	require.NoError(t, db.Exec("INSERT INTO docker_database_backup_runs VALUES ('legacy', 'success')").Error)
	require.NoError(t, addBackupSourceDatabaseUp(db))
	require.NoError(t, addBackupSourceDatabaseUp(db))
	var legacy struct {
		Status         string
		SourceDatabase *string
	}
	require.NoError(t, db.Table("docker_database_backup_runs").Where("id = ?", "legacy").Take(&legacy).Error)
	require.Equal(t, "success", legacy.Status)
	require.Nil(t, legacy.SourceDatabase)
	require.NoError(t, db.Table("docker_database_backup_runs").Where("id = ?", "legacy").Update("source_database", "production").Error)
	require.NoError(t, db.Table("docker_database_backup_runs").Where("id = ?", "legacy").Take(&legacy).Error)
	require.Equal(t, "production", *legacy.SourceDatabase)
}
