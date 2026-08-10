package services

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/database/serializers"
	backupmodels "github.com/kkz6/launch-go/internal/modules/backup/models"
	backuprepos "github.com/kkz6/launch-go/internal/modules/backup/repositories"
	backuptypes "github.com/kkz6/launch-go/internal/modules/backup/types"
	"github.com/kkz6/launch-go/internal/modules/docker/jobs"
	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"github.com/kkz6/launch-go/internal/modules/docker/repositories"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/queue"
	pkgservice "github.com/kkz6/launch-go/internal/pkg/service"
)

type backupServiceRunBroadcast struct {
	broadcast.NopModelBroadcaster
	calls  int
	teamID string
	event  string
	data   map[string]any
}

func (b *backupServiceRunBroadcast) BroadcastToTeam(teamID, event string, data any) {
	b.calls++
	b.teamID = teamID
	b.event = event
	b.data, _ = data.(map[string]any)
}

func TestRunNowRequiresQueueAndTerminalizesPrecreatedRun(t *testing.T) {
	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Project{},
		&models.Database{},
		&models.DatabaseBackup{},
		&models.DatabaseBackupRun{},
	))

	const (
		teamID     = "team-01"
		serverID   = "server-01"
		projectID  = "project-01"
		databaseID = "database-01"
		backupID   = "backup-01"
	)
	project := models.Project{Name: "production"}
	project.ID = projectID
	project.TeamID = teamID
	project.ServerID = serverID
	require.NoError(t, db.Create(&project).Error)

	database := models.Database{
		ProjectID:     projectID,
		Name:          "customers",
		Engine:        "postgres",
		EngineVersion: "16",
		Credentials:   dbtype.EncryptedString(`{"Username":"launch"}`),
	}
	database.ID = databaseID
	database.TeamID = teamID
	database.ServerID = serverID
	require.NoError(t, db.Create(&database).Error)

	backup := models.DatabaseBackup{
		DatabaseID:        databaseID,
		StorageProviderID: 1,
		Enabled:           true,
	}
	backup.ID = backupID
	backup.TeamID = teamID
	require.NoError(t, db.Create(&backup).Error)

	recorder := &backupServiceRunBroadcast{}
	nopLogger := zerolog.Nop()
	repos := repositories.NewRegistry(db)
	service := NewBackupService(&ServiceDeps{
		ModuleDeps: pkgservice.ModuleDeps[*repositories.Registry]{
			Dependencies: pkgservice.Dependencies{
				DB:          db,
				Logger:      &nopLogger,
				Broadcaster: recorder,
				Queue:       nil,
			},
			Repos: repos,
		},
	})

	_, err = service.RunNow(context.Background(), databaseID, projectID, serverID, teamID, "user-01")
	require.ErrorIs(t, err, pkgservice.ErrQueueRequired)

	var runs []models.DatabaseBackupRun
	require.NoError(t, db.Where("backup_id = ?", backupID).Find(&runs).Error)
	require.Len(t, runs, 1)
	require.Equal(t, "failed", runs[0].Status)
	require.NotNil(t, runs[0].StartedAt)
	require.NotNil(t, runs[0].FinishedAt)
	require.NotNil(t, runs[0].Error)
	require.Contains(t, *runs[0].Error, "Queue not configured")

	require.Equal(t, teamID, recorder.teamID)
	require.Equal(t, "docker.database.backup.run.failed", recorder.event)
	require.Equal(t, databaseID, recorder.data["database_id"])
	require.Equal(t, projectID, recorder.data["project_id"])
	require.Equal(t, backupID, recorder.data["backup_id"])
	require.Equal(t, runs[0].ID, recorder.data["run_id"])
	require.Equal(t, serverID, recorder.data["server_id"])
	require.Equal(t, teamID, recorder.data["team_id"])
	require.Equal(t, "manual", recorder.data["source"])
	require.Equal(t, *runs[0].Error, recorder.data["error"])
}

func TestRunNowSuppressesFailureBroadcastWhenTerminalPersistenceFails(t *testing.T) {
	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Project{},
		&models.Database{},
		&models.DatabaseBackup{},
		&models.DatabaseBackupRun{},
	))

	const (
		teamID     = "team-01"
		serverID   = "server-01"
		projectID  = "project-01"
		databaseID = "database-01"
		backupID   = "backup-01"
	)
	project := models.Project{Name: "production"}
	project.ID = projectID
	project.TeamID = teamID
	project.ServerID = serverID
	require.NoError(t, db.Create(&project).Error)

	database := models.Database{
		ProjectID:     projectID,
		Name:          "customers",
		Engine:        "postgres",
		EngineVersion: "16",
		Credentials:   dbtype.EncryptedString(`{"Username":"launch"}`),
	}
	database.ID = databaseID
	database.TeamID = teamID
	database.ServerID = serverID
	require.NoError(t, db.Create(&database).Error)

	backup := models.DatabaseBackup{
		DatabaseID:        databaseID,
		StorageProviderID: 1,
		Enabled:           true,
	}
	backup.ID = backupID
	backup.TeamID = teamID
	require.NoError(t, db.Create(&backup).Error)

	require.NoError(t, db.Exec(`
		CREATE TRIGGER fail_backup_run_terminal_update
		BEFORE UPDATE ON docker_database_backup_runs
		WHEN NEW.status = 'failed'
		BEGIN
			SELECT RAISE(ABORT, 'terminal persistence unavailable');
		END
	`).Error)

	recorder := &backupServiceRunBroadcast{}
	nopLogger := zerolog.Nop()
	service := NewBackupService(&ServiceDeps{
		ModuleDeps: pkgservice.ModuleDeps[*repositories.Registry]{
			Dependencies: pkgservice.Dependencies{
				DB:          db,
				Logger:      &nopLogger,
				Broadcaster: recorder,
				Queue:       nil,
			},
			Repos: repositories.NewRegistry(db),
		},
	})

	_, err = service.RunNow(context.Background(), databaseID, projectID, serverID, teamID, "user-01")
	require.ErrorIs(t, err, pkgservice.ErrQueueRequired)
	require.Zero(t, recorder.calls, "an unpersisted terminal state must never be broadcast")

	var runs []models.DatabaseBackupRun
	require.NoError(t, db.Where("backup_id = ?", backupID).Find(&runs).Error)
	require.Len(t, runs, 1)
	require.Equal(t, "triggered", runs[0].Status)
	require.Nil(t, runs[0].FinishedAt)
	require.Nil(t, runs[0].Error)
}

func TestRunNowBuildsTaskBeforeQueueConnectionFailure(t *testing.T) {
	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Project{},
		&models.Database{},
		&models.DatabaseBackup{},
		&models.DatabaseBackupRun{},
	))

	project := models.Project{Name: "production"}
	project.ID = "project-queue-error"
	project.TeamID = "team-queue-error"
	project.ServerID = "server-queue-error"
	require.NoError(t, db.Create(&project).Error)
	database := models.Database{
		ProjectID: project.ID, Name: "customers", Engine: "postgres", EngineVersion: "16",
		Credentials: dbtype.EncryptedString(`{"Username":"launch"}`),
	}
	database.ID = "database-queue-error"
	database.TeamID = project.TeamID
	database.ServerID = project.ServerID
	require.NoError(t, db.Create(&database).Error)
	backup := models.DatabaseBackup{DatabaseID: database.ID, StorageProviderID: 1, Enabled: true}
	backup.ID = "backup-queue-error"
	backup.TeamID = project.TeamID
	require.NoError(t, db.Create(&backup).Error)

	queueClient := queue.NewClient(config.RedisConfig{Address: "127.0.0.1:1"})
	t.Cleanup(func() { require.NoError(t, queueClient.Close()) })
	nopLogger := zerolog.Nop()
	service := NewBackupService(&ServiceDeps{
		ModuleDeps: pkgservice.ModuleDeps[*repositories.Registry]{
			Dependencies: pkgservice.Dependencies{
				DB: db, Logger: &nopLogger, Queue: queueClient,
			},
			Repos: repositories.NewRegistry(db),
		},
	})

	_, err = service.RunNow(
		context.Background(), database.ID, project.ID, project.ServerID, project.TeamID, "user-01",
	)
	require.ErrorContains(t, err, "queue database backup")
	var run models.DatabaseBackupRun
	require.NoError(t, db.Where("backup_id = ?", backup.ID).First(&run).Error)
	require.Equal(t, "failed", run.Status)
}

func TestDockerBackupServiceLoadsLegacyS3CompatibilityDefaults(t *testing.T) {
	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&backupmodels.StorageProvider{}))
	provider := &backupmodels.StorageProvider{
		ID:       501,
		TeamID:   "team-a",
		UserID:   "user-a",
		Provider: backuptypes.StorageDriverS3,
		Credentials: dbtype.EncryptedJSONMap{
			"endpoint": "https://objects.example.com",
			"region":   "eu-1",
			"bucket":   "nightly",
			"key":      "access",
			"secret":   "secret",
		},
	}
	require.NoError(t, db.Create(provider).Error)
	nopLogger := zerolog.Nop()
	service := NewBackupService(&ServiceDeps{
		ModuleDeps: pkgservice.ModuleDeps[*repositories.Registry]{
			Dependencies: pkgservice.Dependencies{DB: db, Logger: &nopLogger},
			Repos:        repositories.NewRegistry(db),
		},
		BackupRepos: backuprepos.NewRegistry(db),
	})

	credentials, err := service.loadProviderS3Creds(context.Background(), provider.ID, provider.TeamID)
	require.NoError(t, err)
	require.True(t, credentials.ForcePathStyle)
	require.Equal(t, "nightly", credentials.Bucket)

	_, err = service.loadProviderS3Creds(context.Background(), provider.ID, "another-team")
	require.ErrorContains(t, err, "Storage provider not found")
}

func TestRunNowBroadcastsQueuedRunAfterDispatch(t *testing.T) {
	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Project{}, &models.Database{}, &models.DatabaseBackup{}, &models.DatabaseBackupRun{},
	))
	project := models.Project{Name: "production"}
	project.ID = "project-queued"
	project.TeamID = "team-queued"
	project.ServerID = "server-queued"
	require.NoError(t, db.Create(&project).Error)
	database := models.Database{
		ProjectID: project.ID, Name: "customers", Engine: "postgres", EngineVersion: "16",
		Credentials: dbtype.EncryptedString(`{"Username":"launch"}`),
	}
	database.ID = "database-queued"
	database.TeamID = project.TeamID
	database.ServerID = project.ServerID
	require.NoError(t, db.Create(&database).Error)
	backup := models.DatabaseBackup{DatabaseID: database.ID, StorageProviderID: 1, Enabled: true}
	backup.ID = "backup-queued"
	backup.TeamID = project.TeamID
	require.NoError(t, db.Create(&backup).Error)
	recorder := &backupServiceRunBroadcast{}
	testLogger := zerolog.Nop()
	service := NewBackupService(&ServiceDeps{
		ModuleDeps: pkgservice.ModuleDeps[*repositories.Registry]{
			Dependencies: pkgservice.Dependencies{DB: db, Logger: &testLogger, Broadcaster: recorder},
			Repos:        repositories.NewRegistry(db),
		},
	})
	service.dispatchBackup = func(factory pkgservice.TaskFactory) error {
		task, err := factory()
		require.NoError(t, err)
		payload, err := pkgjobs.UnmarshalPayload[jobs.RunBackupPayload](task)
		require.NoError(t, err)
		require.Equal(t, backup.ID, payload.BackupID)
		require.Equal(t, "manual", payload.Source)
		return nil
	}

	run, err := service.RunNow(
		context.Background(), database.ID, project.ID, project.ServerID, project.TeamID, "user-01",
	)
	require.NoError(t, err)
	require.Equal(t, "triggered", run.Status)
	require.Equal(t, project.TeamID, recorder.teamID)
	require.Equal(t, "docker.database.backup.run.queued", recorder.event)
	require.Equal(t, run.ID, recorder.data["run_id"])
	require.Equal(t, "pending", recorder.data["status"])
}
