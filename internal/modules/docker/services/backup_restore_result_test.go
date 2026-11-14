package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/database/serializers"
	backupmodels "github.com/kkz6/launch-go/internal/modules/backup/models"
	backuprepos "github.com/kkz6/launch-go/internal/modules/backup/repositories"
	"github.com/kkz6/launch-go/internal/modules/docker/dto"
	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"github.com/kkz6/launch-go/internal/modules/docker/repositories"
	"github.com/kkz6/launch-go/internal/modules/docker/tasks"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	pkgservice "github.com/kkz6/launch-go/internal/pkg/service"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/tests/testutil"
)

type restoreServiceFixture struct {
	service        *BackupService
	db             *gorm.DB
	source, target *models.Database
	backup         *models.DatabaseBackup
	run            *models.DatabaseBackupRun
	events         *backupServiceRunBroadcast
}

func newRestoreServiceFixture(t *testing.T) restoreServiceFixture {
	t.Helper()
	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))
	db := testutil.SQLite(t, &models.Project{}, &models.Database{}, &models.DatabaseBackup{}, &models.DatabaseBackupRun{}, &servermodels.Server{}, &servermodels.InstalledService{}, &servermodels.FirewallRule{}, &servermodels.Cron{}, &servermodels.Daemon{}, &servermodels.SSHKey{}, &backupmodels.StorageProvider{})
	server := &servermodels.Server{Name: "fixture"}
	server.ID, server.TeamID = "server-1", "team-1"
	require.NoError(t, db.Create(server).Error)
	project := &models.Project{Name: "fixture"}
	project.ID, project.ServerID, project.TeamID = "project-1", server.ID, server.TeamID
	require.NoError(t, db.Create(project).Error)
	newDatabase := func(id, name string) *models.Database {
		database := &models.Database{ProjectID: project.ID, Name: name, Engine: "postgres", EngineVersion: "16",
			Credentials: dbtype.EncryptedString(`{"Username":"fixture","Password":"database-secret","Database":"` + name + `"}`)}
		database.ID, database.ServerID, database.TeamID = id, server.ID, server.TeamID
		require.NoError(t, db.Create(database).Error)
		return database
	}
	source, target := newDatabase("source-1", "production"), newDatabase("target-1", "staging")
	provider := &backupmodels.StorageProvider{ID: 1, UserID: "user-1", TeamID: server.TeamID, Provider: "s3",
		Credentials: dbtype.EncryptedJSONMap{"key": "storage-access-secret", "secret": "storage-private-secret", "bucket": "fixture"}}
	require.NoError(t, db.Create(provider).Error)
	backup := &models.DatabaseBackup{DatabaseID: source.ID, StorageProviderID: provider.ID, Enabled: true}
	backup.ID, backup.TeamID = "backup-1", server.TeamID
	require.NoError(t, db.Create(backup).Error)
	key := "snapshot.gz"
	run := &models.DatabaseBackupRun{BackupID: backup.ID, Status: "success", ObjectKey: &key}
	run.ID = "run-1"
	require.NoError(t, db.Create(run).Error)
	events := &backupServiceRunBroadcast{}
	logger := zerolog.Nop()
	svc := NewBackupService(&ServiceDeps{
		ModuleDeps:  pkgservice.ModuleDeps[*repositories.Registry]{Dependencies: pkgservice.Dependencies{DB: db, Logger: &logger, Broadcaster: events}, Repos: repositories.NewRegistry(db)},
		ServerRepos: serverrepos.NewRegistry(db), BackupRepos: backuprepos.NewRegistry(db),
	})
	svc.restoreTask = func(context.Context, *servermodels.Server, taskrunner.Task) (*taskrunner.SSHCommandResult, error) {
		return &taskrunner.SSHCommandResult{Stdout: "::LAUNCH::restore_step::done\n"}, nil
	}
	return restoreServiceFixture{svc, db, source, target, backup, run, events}
}

func TestRestoreFailureNeverBroadcastsSuccess(t *testing.T) {
	for _, scenario := range []string{"missing result", "missing completion", "client failure", "SSH failure", "SSH failure after output"} {
		t.Run(scenario, func(t *testing.T) {
			f := newRestoreServiceFixture(t)
			f.service.restoreTask = func(context.Context, *servermodels.Server, taskrunner.Task) (*taskrunner.SSHCommandResult, error) {
				switch scenario {
				case "missing result":
					return nil, nil
				case "missing completion":
					return &taskrunner.SSHCommandResult{}, nil
				case "client failure":
					return &taskrunner.SSHCommandResult{ExitCode: 3, Stderr: "invalid SQL database-secret storage-access-secret storage-private-secret"}, nil
				case "SSH failure":
					return nil, errors.New("connection lost database-secret")
				default:
					return &taskrunner.SSHCommandResult{Stdout: "::LAUNCH::restore_step::done\n"}, errors.New("connection lost")
				}
			}
			err := f.service.Restore(context.Background(), f.source.ID, f.source.ProjectID, f.source.ServerID, f.source.TeamID, "user-1", &dto.RestoreBackupRequest{RunID: f.run.ID, TargetDatabaseID: &f.target.ID})
			require.Error(t, err)
			require.Contains(t, err.Error(), "partial changes")
			for _, secret := range []string{"database-secret", "storage-access-secret", "storage-private-secret"} {
				require.NotContains(t, err.Error(), secret)
			}
			require.Zero(t, f.events.calls)
			app := fiber.New(fiber.Config{ErrorHandler: fiberutil.NewErrorHandler()})
			app.Post("/restore", func(*fiber.Ctx) error { return err })
			response, testErr := app.Test(httptest.NewRequest("POST", "/restore", nil))
			require.NoError(t, testErr)
			defer response.Body.Close()
			require.Equal(t, fiber.StatusInternalServerError, response.StatusCode)
			var body fiberutil.ErrorResponse
			require.NoError(t, json.NewDecoder(response.Body).Decode(&body))
			require.False(t, body.Success)
			require.Equal(t, err.Error(), body.Message)
		})
	}
}

func TestRestoreSuccessIdentifiesSelectedTarget(t *testing.T) {
	f := newRestoreServiceFixture(t)
	require.NoError(t, f.service.Restore(context.Background(), f.source.ID, f.source.ProjectID, f.source.ServerID, f.source.TeamID, "user-1", &dto.RestoreBackupRequest{RunID: f.run.ID, TargetDatabaseID: &f.target.ID}))
	require.Equal(t, "docker.database.backup.restored", f.events.event)
	require.Equal(t, f.target.ID, f.events.data["target_database_id"])
	require.Equal(t, 1, f.events.calls)
}

func TestRestoreErrorRedactsBeforeTruncating(t *testing.T) {
	secret := "very-long-'secret&value"
	for _, form := range []string{secret, taskrunner.ShellQuote(secret), url.QueryEscape(secret), url.PathEscape(secret)} {
		err := restoreResultError(&taskrunner.SSHCommandResult{ExitCode: 1, Stderr: strings.Repeat("x", 3985) + form}, nil, tasks.RestoreBackupConfig{Password: secret})
		require.NotContains(t, err.Error(), "very-long-")
	}
}
