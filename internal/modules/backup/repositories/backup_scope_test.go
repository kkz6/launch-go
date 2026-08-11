package repositories

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/database/serializers"
	"github.com/kkz6/launch-go/internal/modules/backup/models"
	backuptypes "github.com/kkz6/launch-go/internal/modules/backup/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
)

func newBackupScopeRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))
	dsn := fmt.Sprintf("file:backup-scope-repository-%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Backup{}, &models.StorageProvider{}))
	return db
}

func TestBackupRepositoryFindForRunIsServerAndTeamScoped(t *testing.T) {
	db := newBackupScopeRepositoryTestDB(t)
	repo := NewBackupRepository(db)
	backup := &models.Backup{
		StorageProviderID: 1,
		CronExpression:    "0 0 * * *",
		IncludeFiles:      "[]",
		ExcludeFiles:      "[]",
		Path:              "backups",
	}
	backup.ID = "backup-01"
	backup.ServerID = "server-01"
	backup.TeamID = "team-01"
	require.NoError(t, db.Create(backup).Error)

	found, err := repo.FindBackupForRun(
		context.Background(), backup.ID, backup.ServerID, backup.TeamID,
	)
	require.NoError(t, err)
	require.Equal(t, backup.ID, found.ID)

	for _, scope := range []struct {
		serverID string
		teamID   string
	}{
		{serverID: "other-server", teamID: backup.TeamID},
		{serverID: backup.ServerID, teamID: "other-team"},
	} {
		_, err := repo.FindBackupForRun(
			context.Background(), backup.ID, scope.serverID, scope.teamID,
		)
		require.Error(t, err)
	}
}

func TestBackupRepositoryListsOnlyEnabledBackups(t *testing.T) {
	db := newBackupScopeRepositoryTestDB(t)
	repo := NewBackupRepository(db)
	for _, backup := range []models.Backup{
		{StorageProviderID: 1, CronExpression: "0 0 * * *", IncludeFiles: "[]", ExcludeFiles: "[]", Path: "enabled", Enabled: true},
		{StorageProviderID: 1, CronExpression: "0 0 * * *", IncludeFiles: "[]", ExcludeFiles: "[]", Path: "disabled", Enabled: false},
	} {
		backup.ID = "backup-" + backup.Path
		backup.ServerID = "server-01"
		backup.TeamID = "team-01"
		require.NoError(t, db.Create(&backup).Error)
	}

	backups, err := repo.ListEnabled(context.Background())
	require.NoError(t, err)
	require.Len(t, backups, 1)
	require.Equal(t, "backup-enabled", backups[0].ID)
}

func TestStorageProviderRepositoryScopedLookups(t *testing.T) {
	db := newBackupScopeRepositoryTestDB(t)
	repo := NewStorageProviderRepository(db)
	provider := &models.StorageProvider{
		ID:          91,
		UserID:      "user-01",
		TeamID:      "team-01",
		Provider:    backuptypes.StorageDriverS3,
		Credentials: dbtype.EncryptedJSONMap{"bucket": "nightly"},
	}
	require.NoError(t, db.Create(provider).Error)

	found, err := repo.FindStorageProviderByIDAndTeam(context.Background(), provider.ID, provider.TeamID)
	require.NoError(t, err)
	require.Equal(t, provider.ID, found.ID)
	driver, err := repo.FindStorageProviderDriverByIDAndTeam(context.Background(), provider.ID, provider.TeamID)
	require.NoError(t, err)
	require.Equal(t, backuptypes.StorageDriverS3, driver)

	_, err = repo.FindStorageProviderByIDAndTeam(context.Background(), provider.ID, "other-team")
	require.Error(t, err)
	_, err = repo.FindStorageProviderDriverByIDAndTeam(context.Background(), provider.ID, "other-team")
	require.Error(t, err)
}

func TestStorageProviderRepositoryScopedLookupsReturnDatabaseErrors(t *testing.T) {
	db := newBackupScopeRepositoryTestDB(t)
	repo := NewStorageProviderRepository(db)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	_, err = repo.FindStorageProviderByIDAndTeam(context.Background(), 1, "team-01")
	require.Error(t, err)
	_, err = repo.FindStorageProviderDriverByIDAndTeam(context.Background(), 1, "team-01")
	require.Error(t, err)
}
