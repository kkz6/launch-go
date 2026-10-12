package jobs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	backupmodels "github.com/kkz6/launch-go/internal/modules/backup/models"
	databasemodels "github.com/kkz6/launch-go/internal/modules/database/models"
	databaserepos "github.com/kkz6/launch-go/internal/modules/database/repositories"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
)

func databaseDumpFixture(t *testing.T, software string) (*manualBackupFixture, *RunManualBackupJob, databasemodels.Database) {
	t.Helper()
	f := newManualBackupFixture(t)
	require.NoError(t, f.db.AutoMigrate(&databasemodels.Database{}, &databasemodels.DatabaseUser{}, &databasemodels.DatabaseDatabaseUser{}))
	database := databasemodels.Database{Name: "karti_dev"}
	database.ServerID, database.TeamID = f.serverID, f.teamID
	require.NoError(t, f.db.Create(&database).Error)
	require.NoError(t, f.deps.Repos.Backup().SyncBackupDatabases(context.Background(), f.backupID, []string{database.ID}))
	service := servermodels.InstalledService{Name: "Database", Software: software, Type: "mysql", Status: "running"}
	service.ServerID = f.serverID
	require.NoError(t, f.db.Create(&service).Error)
	var backup backupmodels.Backup
	var server servermodels.Server
	require.NoError(t, f.db.First(&backup, "id = ?", f.backupID).Error)
	require.NoError(t, f.db.First(&server, "id = ?", f.serverID).Error)
	f.deps.DatabaseRepos = databaserepos.NewRegistry(f.db)
	return f, &RunManualBackupJob{Deps: f.deps, backup: &backup, server: &server}, database
}

func TestBackupResolvesServerAdministratorWithoutDatabaseLinks(t *testing.T) {
	for _, tc := range []struct{ software, username, engine string }{
		{"mysql80", "root", "mysql"},
		{"mariadb", "root", "mariadb"},
		{"postgresql16", "postgres", "postgres"},
	} {
		t.Run(tc.engine, func(t *testing.T) {
			f, job, _ := databaseDumpFixture(t, tc.software)
			user := databasemodels.DatabaseUser{Name: tc.username, Password: &dbtype.EncryptedNullableString{String: "test-password", Valid: true}}
			user.ServerID, user.TeamID = f.serverID, f.teamID
			require.NoError(t, f.db.Create(&user).Error)
			dumps, err := job.resolveDatabaseDumps(context.Background())
			require.NoError(t, err)
			require.Len(t, dumps, 1)
			require.Equal(t, "karti_dev", dumps[0].Name)
			require.Equal(t, tc.username, dumps[0].Username)
			require.Equal(t, tc.engine, dumps[0].Engine)
			require.Equal(t, "test-password", dumps[0].Password)
		})
	}
}

func TestBackupKeepsExplicitlyLinkedDatabaseUser(t *testing.T) {
	f, job, database := databaseDumpFixture(t, "mysql80")
	user := databasemodels.DatabaseUser{Name: "backup-user", Password: &dbtype.EncryptedNullableString{String: "test-password", Valid: true}}
	user.ServerID, user.TeamID = f.serverID, f.teamID
	require.NoError(t, f.db.Create(&user).Error)
	require.NoError(t, f.db.Create(&databasemodels.DatabaseDatabaseUser{DatabaseID: database.ID, DatabaseUserID: user.ID}).Error)
	dumps, err := job.resolveDatabaseDumps(context.Background())
	require.NoError(t, err)
	require.Len(t, dumps, 1)
	require.Equal(t, user.Name, dumps[0].Username)
}

func TestBackupDoesNotUseAdministratorFromAnotherServerOrTeam(t *testing.T) {
	for _, scope := range []string{"server", "team"} {
		t.Run(scope, func(t *testing.T) {
			f, job, _ := databaseDumpFixture(t, "mysql80")
			user := databasemodels.DatabaseUser{Name: "root"}
			user.ServerID, user.TeamID = f.serverID, f.teamID
			if scope == "server" {
				user.ServerID = "other-server"
			} else {
				user.TeamID = "other-team"
			}
			require.NoError(t, f.db.Create(&user).Error)
			_, err := job.resolveDatabaseDumps(context.Background())
			require.Error(t, err)
		})
	}
}
