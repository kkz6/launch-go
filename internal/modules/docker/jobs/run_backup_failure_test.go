package jobs

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/database/serializers"
	backupmodels "github.com/kkz6/launch-go/internal/modules/backup/models"
	backuprepos "github.com/kkz6/launch-go/internal/modules/backup/repositories"
	backuptypes "github.com/kkz6/launch-go/internal/modules/backup/types"
	dockermodels "github.com/kkz6/launch-go/internal/modules/docker/models"
	dockerrepos "github.com/kkz6/launch-go/internal/modules/docker/repositories"
	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner/markers"
)

type dockerBackupBroadcast struct {
	event string
	data  any
}

type dockerBackupRecordingBroadcaster struct {
	broadcast.NopBroadcaster
	calls []dockerBackupBroadcast
}

func (b *dockerBackupRecordingBroadcaster) BroadcastToTeam(_ string, event string, data any) {
	b.calls = append(b.calls, dockerBackupBroadcast{event: event, data: data})
}

func TestRunBackupJobMarksPrecreatedRunFailedOnStorageConfigurationError(t *testing.T) {
	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&servermodels.Server{},
		&servermodels.InstalledService{},
		&backupmodels.StorageProvider{},
		&dockermodels.Project{},
		&dockermodels.Database{},
		&dockermodels.DatabaseBackup{},
		&dockermodels.DatabaseBackupRun{},
	))

	const (
		teamID     = "team00000000000000000000001"
		serverID   = "server000000000000000000001"
		projectID  = "project00000000000000000001"
		databaseID = "database0000000000000000001"
		backupID   = "dbbackup0000000000000000001"
		runID      = "backuprun000000000000000001"
	)

	server := servermodels.Server{Name: "docker-host"}
	server.ID = serverID
	server.TeamID = teamID
	server.UserID = "user00000000000000000000001"
	require.NoError(t, db.Create(&server).Error)

	project := dockermodels.Project{Name: "production"}
	project.ID = projectID
	project.TeamID = teamID
	project.ServerID = serverID
	require.NoError(t, db.Create(&project).Error)

	database := dockermodels.Database{
		ProjectID: projectID,
		Name:      "customers",
	}
	database.ID = databaseID
	database.TeamID = teamID
	database.ServerID = serverID
	require.NoError(t, db.Create(&database).Error)

	provider := backupmodels.StorageProvider{
		ID:          72,
		UserID:      server.UserID,
		TeamID:      teamID,
		Provider:    backuptypes.StorageDriverS3,
		Credentials: dbtype.EncryptedJSONMap{"bucket": "nightly"},
		Connected:   true,
	}
	require.NoError(t, db.Create(&provider).Error)

	backup := dockermodels.DatabaseBackup{
		DatabaseID:        databaseID,
		StorageProviderID: provider.ID,
		Enabled:           true,
	}
	backup.ID = backupID
	backup.TeamID = teamID
	require.NoError(t, db.Create(&backup).Error)
	require.NoError(t, db.Model(&backup).Update("enabled", false).Error)

	run := dockermodels.DatabaseBackupRun{
		BackupID: backupID,
		Status:   "triggered",
	}
	run.ID = runID
	require.NoError(t, db.Create(&run).Error)

	nopLogger := zerolog.Nop()
	job := &RunBackupJob{
		Deps: &JobDeps{
			Deps: &pkgjobs.Deps{
				DB:     db,
				Logger: &nopLogger,
			},
			Repos:       dockerrepos.NewRegistry(db),
			ServerRepos: serverrepos.NewRegistry(db),
			BackupRepos: backuprepos.NewRegistry(db),
		},
		Payload: RunBackupPayload{
			BackupID:   backupID,
			DatabaseID: databaseID,
			ProjectID:  projectID,
			ServerID:   serverID,
			TeamID:     teamID,
			RunID:      runID,
			Source:     "manual",
		},
	}

	require.NoError(t, job.Handle(context.Background()))

	var persisted dockermodels.DatabaseBackupRun
	require.NoError(t, db.First(&persisted, "id = ?", runID).Error)
	require.Equal(t, "failed", persisted.Status)
	require.NotNil(t, persisted.StartedAt)
	require.NotNil(t, persisted.FinishedAt)
	require.NotNil(t, persisted.Error)
	require.True(t, strings.Contains(*persisted.Error, "missing S3 credentials"))

	var count int64
	require.NoError(t, db.Model(&dockermodels.DatabaseBackupRun{}).
		Where("backup_id = ?", backupID).
		Count(&count).Error)
	require.Equal(t, int64(1), count, "the worker must update the manual run instead of creating an orphan failure row")
}

func TestRunBackupJobFailedMarksPrecreatedRunFailedWithNilError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&dockermodels.DatabaseBackup{},
		&dockermodels.DatabaseBackupRun{},
	))

	backup := dockermodels.DatabaseBackup{
		DatabaseID:        "database-01",
		StorageProviderID: 1,
	}
	backup.ID = "backup-01"
	backup.TeamID = "team-01"
	require.NoError(t, db.Create(&backup).Error)

	startedAt := time.Now().UTC()
	run := dockermodels.DatabaseBackupRun{
		BackupID:  "backup-01",
		Status:    "triggered",
		StartedAt: &startedAt,
	}
	run.ID = "run-01"
	require.NoError(t, db.Create(&run).Error)

	nopLogger := zerolog.Nop()
	job := &RunBackupJob{
		Deps: &JobDeps{
			Deps:  &pkgjobs.Deps{Logger: &nopLogger},
			Repos: dockerrepos.NewRegistry(db),
		},
		Payload: RunBackupPayload{
			BackupID: "backup-01",
			RunID:    run.ID,
			TeamID:   "team-01",
		},
	}

	require.NotPanics(t, func() {
		job.Failed(context.Background(), nil)
	})

	var persisted dockermodels.DatabaseBackupRun
	require.NoError(t, db.First(&persisted, "id = ?", run.ID).Error)
	require.Equal(t, "failed", persisted.Status)
	require.NotNil(t, persisted.StartedAt)
	require.NotNil(t, persisted.FinishedAt)
	require.NotNil(t, persisted.Error)
	require.Equal(t, "backup worker failed", *persisted.Error)
}

func TestRunBackupJobRejectsCrossBackupPrecreatedRun(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&dockermodels.DatabaseBackup{},
		&dockermodels.DatabaseBackupRun{},
	))

	for _, id := range []string{"backup-a", "backup-b"} {
		backup := dockermodels.DatabaseBackup{
			DatabaseID:        "database-" + id,
			StorageProviderID: 1,
		}
		backup.ID = id
		backup.TeamID = "team-01"
		require.NoError(t, db.Create(&backup).Error)
	}
	run := dockermodels.DatabaseBackupRun{BackupID: "backup-b", Status: "triggered"}
	run.ID = "run-b"
	require.NoError(t, db.Create(&run).Error)

	nopLogger := zerolog.Nop()
	job := &RunBackupJob{
		Deps: &JobDeps{
			Deps:  &pkgjobs.Deps{Logger: &nopLogger},
			Repos: dockerrepos.NewRegistry(db),
		},
		Payload: RunBackupPayload{
			BackupID: "backup-a",
			RunID:    run.ID,
			TeamID:   "team-01",
			Source:   "manual",
		},
	}

	require.NoError(t, job.Handle(context.Background()))
	recorded, err := job.recordFailure(context.Background(), "backup-a", run.ID, "malformed replay")
	require.NoError(t, err)
	require.False(t, recorded)
	var persisted dockermodels.DatabaseBackupRun
	require.NoError(t, db.First(&persisted, "id = ?", run.ID).Error)
	require.Equal(t, "triggered", persisted.Status)
	require.Nil(t, persisted.StartedAt)
}

func TestRunBackupJobRejectsTerminalOrAlreadyClaimedRun(t *testing.T) {
	tests := []string{"success", "failed", "running"}
	for _, status := range tests {
		t.Run(status, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
				Logger: logger.Default.LogMode(logger.Silent),
			})
			require.NoError(t, err)
			require.NoError(t, db.AutoMigrate(
				&dockermodels.DatabaseBackup{},
				&dockermodels.DatabaseBackupRun{},
			))

			backup := dockermodels.DatabaseBackup{
				DatabaseID:        "database-01",
				StorageProviderID: 1,
			}
			backup.ID = "backup-01"
			backup.TeamID = "team-01"
			require.NoError(t, db.Create(&backup).Error)
			run := dockermodels.DatabaseBackupRun{BackupID: backup.ID, Status: status}
			run.ID = "run-01"
			require.NoError(t, db.Create(&run).Error)

			nopLogger := zerolog.Nop()
			job := &RunBackupJob{
				Deps: &JobDeps{
					Deps:  &pkgjobs.Deps{Logger: &nopLogger},
					Repos: dockerrepos.NewRegistry(db),
				},
				Payload: RunBackupPayload{
					BackupID: backup.ID,
					RunID:    run.ID,
					TeamID:   backup.TeamID,
					Source:   "manual",
				},
			}

			require.NoError(t, job.Handle(context.Background()))
			var persisted dockermodels.DatabaseBackupRun
			require.NoError(t, db.First(&persisted, "id = ?", run.ID).Error)
			require.Equal(t, status, persisted.Status)
			require.Nil(t, persisted.StartedAt)
		})
	}
}

func TestBackupRunRetryResumesExactScopedRunningRow(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&dockermodels.DatabaseBackup{},
		&dockermodels.DatabaseBackupRun{},
	))

	backup := dockermodels.DatabaseBackup{
		DatabaseID:        "database-01",
		StorageProviderID: 1,
	}
	backup.ID = "backup-01"
	backup.TeamID = "team-01"
	require.NoError(t, db.Create(&backup).Error)
	startedAt := time.Date(2026, 8, 10, 9, 0, 0, 0, time.UTC)
	run := dockermodels.DatabaseBackupRun{
		BackupID:  backup.ID,
		Status:    "running",
		StartedAt: &startedAt,
	}
	run.ID = "run-01"
	require.NoError(t, db.Create(&run).Error)

	repo := dockerrepos.NewRegistry(db).BackupRun()
	resumed, claimed, err := repo.ClaimTriggeredForBackup(
		context.Background(),
		run.ID,
		backup.ID,
		backup.TeamID,
		time.Now().UTC(),
		true,
	)
	require.NoError(t, err)
	require.True(t, claimed)
	require.NotNil(t, resumed)
	require.Equal(t, run.ID, resumed.ID)
	require.Equal(t, "running", resumed.Status)
	require.NotNil(t, resumed.StartedAt)
	require.True(t, resumed.StartedAt.Equal(startedAt), "retry must preserve the original start time")

	_, claimed, err = repo.ClaimTriggeredForBackup(
		context.Background(),
		run.ID,
		"another-backup",
		backup.TeamID,
		time.Now().UTC(),
		true,
	)
	require.NoError(t, err)
	require.False(t, claimed, "retry scope must still match the run's backup")
}

func TestRunBackupJobTerminalFailureIncludesTaskIDAndError(t *testing.T) {
	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))

	db, err := gorm.Open(
		sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"),
		&gorm.Config{
			DisableForeignKeyConstraintWhenMigrating: true,
			Logger:                                   logger.Default.LogMode(logger.Silent),
		},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&servermodels.Server{},
		&servermodels.InstalledService{},
		&servermodels.Task{},
		&backupmodels.StorageProvider{},
		&dockermodels.Project{},
		&dockermodels.Database{},
		&dockermodels.DatabaseBackup{},
		&dockermodels.DatabaseBackupRun{},
	))

	const (
		teamID     = "team00000000000000000000001"
		serverID   = "server000000000000000000001"
		projectID  = "project00000000000000000001"
		databaseID = "database0000000000000000001"
		backupID   = "dbbackup0000000000000000001"
		runID      = "backuprun000000000000000001"
	)

	ip := "192.0.2.10"
	server := &servermodels.Server{
		BaseModel:  basemodels.BaseModel{ID: serverID},
		Name:       "docker-host",
		PublicIPv4: &ip,
		PrivateKey: dbtype.EncryptedString("test-private-key"),
	}
	server.TeamID = teamID
	server.UserID = "user00000000000000000000001"
	require.NoError(t, db.Create(server).Error)

	project := &dockermodels.Project{
		BaseModel: basemodels.BaseModel{ID: projectID},
		Name:      "production",
	}
	project.TeamID = teamID
	project.ServerID = serverID
	require.NoError(t, db.Create(project).Error)

	database := &dockermodels.Database{
		BaseModel:     basemodels.BaseModel{ID: databaseID},
		ProjectID:     projectID,
		Name:          "customers",
		Engine:        dockertypes.DatabaseEnginePostgres,
		EngineVersion: "16",
		Credentials: dbtype.EncryptedString(
			`{"Username":"launch","Password":"secret","Database":"customers"}`,
		),
	}
	database.TeamID = teamID
	database.ServerID = serverID
	require.NoError(t, db.Create(database).Error)

	provider := &backupmodels.StorageProvider{
		ID:       73,
		UserID:   server.UserID,
		TeamID:   teamID,
		Provider: backuptypes.StorageDriverS3,
		Credentials: dbtype.EncryptedJSONMap{
			"endpoint": "https://eu2.contabostorage.com",
			"region":   "eu2",
			"bucket":   "nightly",
			"key":      "access-key",
			"secret":   "secret-key",
		},
		Connected: true,
	}
	require.NoError(t, db.Create(provider).Error)

	backup := &dockermodels.DatabaseBackup{
		BaseModel:         basemodels.BaseModel{ID: backupID},
		DatabaseID:        databaseID,
		StorageProviderID: provider.ID,
		Enabled:           true,
	}
	backup.TeamID = teamID
	require.NoError(t, db.Create(backup).Error)

	run := &dockermodels.DatabaseBackupRun{
		BaseModel: basemodels.BaseModel{ID: runID},
		BackupID:  backupID,
		Status:    "triggered",
	}
	require.NoError(t, db.Create(run).Error)

	nopLogger := zerolog.Nop()
	recorder := &dockerBackupRecordingBroadcaster{}
	dispatcher := taskrunner.NewFakeDispatcher()
	dispatcher.SetDefaultFailure(23, "upload permission denied")
	job := &RunBackupJob{
		Deps: &JobDeps{
			Deps: &pkgjobs.Deps{
				DB:          db,
				Logger:      &nopLogger,
				Broadcaster: recorder,
				Dispatcher:  dispatcher,
			},
			Repos:       dockerrepos.NewRegistry(db),
			ServerRepos: serverrepos.NewRegistry(db),
			BackupRepos: backuprepos.NewRegistry(db),
			TaskRunnerDeps: &servertasks.TaskRunnerDeps{
				DB:          db,
				Dispatcher:  dispatcher,
				Logger:      &nopLogger,
				Broadcaster: recorder,
			},
		},
		Payload: RunBackupPayload{
			BackupID:   backupID,
			DatabaseID: databaseID,
			ProjectID:  projectID,
			ServerID:   serverID,
			TeamID:     teamID,
			RunID:      runID,
			Source:     "manual",
		},
	}

	require.NoError(t, job.Handle(context.Background()))

	var persisted dockermodels.DatabaseBackupRun
	require.NoError(t, db.First(&persisted, "id = ?", runID).Error)
	require.Equal(t, "failed", persisted.Status)
	require.NotNil(t, persisted.TaskID)
	require.NotNil(t, persisted.Error)
	require.Contains(t, *persisted.Error, "upload permission denied")

	var failurePayload map[string]any
	for _, call := range recorder.calls {
		if call.event == "docker.database.backup.run.failed" {
			failurePayload, _ = call.data.(map[string]any)
		}
	}
	require.NotNil(t, failurePayload)
	require.Equal(t, *persisted.TaskID, failurePayload["task_id"])
	require.Contains(t, failurePayload["error"], "upload permission denied")
	require.Len(t, dispatcher.Executions, 1)
	require.Contains(t, dispatcher.Executions[0].Script, "--force-path-style")
}

func TestRunBackupJobSuppressesTerminalBroadcastWhenPersistenceFails(t *testing.T) {
	tests := []struct {
		name       string
		failedTask bool
		errorText  string
	}{
		{name: "success", errorText: "persist backup success state"},
		{name: "failure", failedTask: true, errorText: "persist backup failure state"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newBackupExecutionFixture(t, true)
			if tt.failedTask {
				fixture.dispatcher.SetDefaultFailure(23, "upload permission denied")
			}
			require.NoError(t, fixture.db.Exec(`
				CREATE TRIGGER fail_backup_run_terminal_update
				BEFORE UPDATE ON docker_database_backup_runs
				WHEN NEW.status IN ('success', 'failed')
				BEGIN
					SELECT RAISE(ABORT, 'terminal persistence unavailable');
				END
			`).Error)

			err := fixture.job.Handle(context.Background())
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.errorText)

			for _, call := range fixture.recorder.calls {
				require.NotEqual(t, "docker.database.backup.run.succeeded", call.event)
				require.NotEqual(t, "docker.database.backup.run.failed", call.event)
			}

			var persisted dockermodels.DatabaseBackupRun
			require.NoError(t, fixture.db.First(&persisted, "id = ?", fixture.runID).Error)
			require.Equal(t, "running", persisted.Status)
			require.Nil(t, persisted.FinishedAt)
		})
	}
}

func TestRunBackupJobRepairsTaskIDInTerminalWriteAfterAttachmentFailure(t *testing.T) {
	fixture := newBackupExecutionFixture(t, true)
	fixture.dispatcher.SetDefaultFailure(23, "upload permission denied")

	require.NoError(t, fixture.db.Exec(`
		CREATE TRIGGER fail_backup_run_task_attachment
		BEFORE UPDATE ON docker_database_backup_runs
		WHEN OLD.status = 'running'
		  AND NEW.status = 'running'
		  AND NEW.task_id IS NOT NULL
		BEGIN
			SELECT RAISE(ABORT, 'task attachment unavailable');
		END
	`).Error)

	require.NoError(t, fixture.job.Handle(context.Background()))

	var persisted dockermodels.DatabaseBackupRun
	require.NoError(t, fixture.db.First(&persisted, "id = ?", fixture.runID).Error)
	require.Equal(t, "failed", persisted.Status)
	require.NotNil(t, persisted.TaskID)

	startedWithTask := 0
	terminalWithTask := 0
	for _, call := range fixture.recorder.calls {
		payload, _ := call.data.(map[string]any)
		if call.event == "docker.database.backup.run.started" && payload["task_id"] != nil {
			startedWithTask++
		}
		if call.event == "docker.database.backup.run.failed" && payload["task_id"] == *persisted.TaskID {
			terminalWithTask++
		}
	}
	require.Zero(t, startedWithTask, "failed task attachment must suppress the started-with-task event")
	require.Equal(t, 1, terminalWithTask, "the durable terminal update must carry the task identity")
}

func TestRunBackupJobPersistsSuccessfulRunAndTask(t *testing.T) {
	fixture := newBackupExecutionFixture(t, true)
	fixture.dispatcher.DefaultResult.Output = "::LAUNCH::object_key::database/customers.sql.gz\n::LAUNCH::size_bytes::16384"

	require.NoError(t, fixture.job.Handle(context.Background()))

	var persisted dockermodels.DatabaseBackupRun
	require.NoError(t, fixture.db.First(&persisted, "id = ?", fixture.runID).Error)
	require.Equal(t, "success", persisted.Status)
	require.NotNil(t, persisted.FinishedAt)
	require.NotNil(t, persisted.ObjectKey)
	require.Equal(t, "database/customers.sql.gz", *persisted.ObjectKey)
	require.NotNil(t, persisted.SizeBytes)
	require.Equal(t, int64(16384), *persisted.SizeBytes)
	require.NotNil(t, persisted.TaskID)

	var successPayload map[string]any
	for _, call := range fixture.recorder.calls {
		if call.event == "docker.database.backup.run.succeeded" {
			successPayload, _ = call.data.(map[string]any)
		}
	}
	require.NotNil(t, successPayload)
	require.Equal(t, *persisted.TaskID, successPayload["task_id"])
	require.Equal(t, int64(16384), successPayload["size_bytes"])
}

func TestRunBackupJobUsesFallbackForEmptyTaskFailure(t *testing.T) {
	fixture := newBackupExecutionFixture(t, true)
	fixture.dispatcher.SetDefaultFailure(17, "")

	require.NoError(t, fixture.job.Handle(context.Background()))

	var persisted dockermodels.DatabaseBackupRun
	require.NoError(t, fixture.db.First(&persisted, "id = ?", fixture.runID).Error)
	require.Equal(t, "failed", persisted.Status)
	require.NotNil(t, persisted.Error)
	require.Equal(t, "database backup failed without an error message", *persisted.Error)
}

func TestRunBackupJobRecordsCorruptDatabaseCredentials(t *testing.T) {
	fixture := newBackupExecutionFixture(t, true)
	require.NoError(t, fixture.db.Exec(
		"UPDATE docker_databases SET credentials = '' WHERE id = ?",
		fixture.job.Payload.DatabaseID,
	).Error)

	require.NoError(t, fixture.job.Handle(context.Background()))

	var persisted dockermodels.DatabaseBackupRun
	require.NoError(t, fixture.db.First(&persisted, "id = ?", fixture.runID).Error)
	require.Equal(t, "failed", persisted.Status)
	require.NotNil(t, persisted.Error)
	require.Equal(t, "database credentials are missing or corrupt", *persisted.Error)
	require.Empty(t, fixture.dispatcher.Executions)
}

func TestRunBackupJobMarksSupersededPrecreatedRunFailed(t *testing.T) {
	fixture := newBackupExecutionFixture(t, true)
	require.NoError(t, fixture.db.Model(&dockermodels.DatabaseBackup{}).
		Where("id = ?", fixture.backupID).
		Update("database_id", "old-database").Error)
	replacement := &dockermodels.DatabaseBackup{
		DatabaseID:        fixture.job.Payload.DatabaseID,
		StorageProviderID: 73,
		Enabled:           true,
	}
	replacement.ID = "replacement-backup"
	replacement.TeamID = fixture.job.Payload.TeamID
	require.NoError(t, fixture.db.Create(replacement).Error)

	require.NoError(t, fixture.job.Handle(context.Background()))

	var persisted dockermodels.DatabaseBackupRun
	require.NoError(t, fixture.db.First(&persisted, "id = ?", fixture.runID).Error)
	require.Equal(t, "failed", persisted.Status)
	require.NotNil(t, persisted.Error)
	require.Contains(t, *persisted.Error, "backup configuration changed")
}

func TestRunBackupJobReturnsSupersededRunPersistenceError(t *testing.T) {
	fixture := newBackupExecutionFixture(t, true)
	require.NoError(t, fixture.db.Model(&dockermodels.DatabaseBackup{}).
		Where("id = ?", fixture.backupID).
		Update("database_id", "old-database").Error)
	replacement := &dockermodels.DatabaseBackup{
		DatabaseID: fixture.job.Payload.DatabaseID, StorageProviderID: 73, Enabled: true,
	}
	replacement.ID = "replacement-backup"
	replacement.TeamID = fixture.job.Payload.TeamID
	require.NoError(t, fixture.db.Create(replacement).Error)
	require.NoError(t, fixture.db.Exec(`
		CREATE TRIGGER reject_superseded_failure_update
		BEFORE UPDATE ON docker_database_backup_runs
		WHEN NEW.status = 'failed'
		BEGIN
			SELECT RAISE(ABORT, 'failure update unavailable');
		END
	`).Error)

	require.ErrorContains(t, fixture.job.Handle(context.Background()), "persist superseded backup failure")
}

func TestRunBackupJobSuppressesTaskAndTerminalEventsAfterConcurrentTerminalState(t *testing.T) {
	fixture := newBackupExecutionFixture(t, true)
	require.NoError(t, fixture.db.Exec(`
		CREATE TRIGGER terminalize_database_backup_after_claim
		AFTER UPDATE OF status ON docker_database_backup_runs
		WHEN OLD.status = 'triggered' AND NEW.status = 'running'
		BEGIN
			UPDATE docker_database_backup_runs SET status = 'failed' WHERE id = NEW.id;
		END
	`).Error)

	require.NoError(t, fixture.job.Handle(context.Background()))

	var persisted dockermodels.DatabaseBackupRun
	require.NoError(t, fixture.db.First(&persisted, "id = ?", fixture.runID).Error)
	require.Equal(t, "failed", persisted.Status)
	for _, call := range fixture.recorder.calls {
		payload, _ := call.data.(map[string]any)
		if call.event == "docker.database.backup.run.started" {
			require.Nil(t, payload["task_id"])
		}
		require.NotEqual(t, "docker.database.backup.run.succeeded", call.event)
	}
}

func TestRunBackupJobReturnsClaimPersistenceError(t *testing.T) {
	fixture := newBackupExecutionFixture(t, true)
	sqlDB, err := fixture.db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	require.ErrorContains(t, fixture.job.Handle(context.Background()), "claim pre-created backup run")
}

func TestRunBackupJobRejectsBackupFromAnotherTeam(t *testing.T) {
	fixture := newBackupExecutionFixture(t, false)
	fixture.job.Payload.TeamID = "another-team"

	err := fixture.job.Handle(context.Background())

	require.ErrorContains(t, err, "backup team does not match queued run")
	require.Empty(t, fixture.recorder.calls)
	var runCount int64
	require.NoError(t, fixture.db.Model(&dockermodels.DatabaseBackupRun{}).Count(&runCount).Error)
	require.Zero(t, runCount)
}

func TestRunBackupJobScopesStorageProviderToTeam(t *testing.T) {
	fixture := newBackupExecutionFixture(t, false)
	providerLabel := "Nightly storage"
	require.NoError(t, fixture.db.Model(&backupmodels.StorageProvider{}).
		Where("id = ?", 73).
		Update("label", providerLabel).Error)

	_, err := fixture.job.loadProviderS3Creds(context.Background(), 73, "another-team")
	require.ErrorContains(t, err, "storage provider 73 not found")
	require.Empty(t, fixture.job.lookupStorageProviderLabel(context.Background(), 73, "another-team"))
	require.Equal(t, providerLabel, fixture.job.lookupStorageProviderLabel(
		context.Background(),
		73,
		fixture.job.Payload.TeamID,
	))
}

func TestRunBackupJobRecordFailureCreatesAndReusesRun(t *testing.T) {
	fixture := newBackupExecutionFixture(t, false)

	persisted, err := fixture.job.recordFailure(context.Background(), fixture.backupID, "", "")
	require.NoError(t, err)
	require.True(t, persisted)
	require.NotEmpty(t, fixture.job.runID)
	require.Equal(t, fixture.job.runID, fixture.job.Payload.RunID)

	var run dockermodels.DatabaseBackupRun
	require.NoError(t, fixture.db.First(&run, "id = ?", fixture.job.runID).Error)
	require.Equal(t, "failed", run.Status)
	require.NotNil(t, run.Error)
	require.Equal(t, "database backup failed without an error message", *run.Error)

	reusedRun := &dockermodels.DatabaseBackupRun{BackupID: fixture.backupID, Status: "triggered"}
	reusedRun.ID = "reused-run"
	require.NoError(t, fixture.db.Create(reusedRun).Error)
	fixture.job.runID = reusedRun.ID
	persisted, err = fixture.job.recordFailure(
		context.Background(), fixture.backupID, "", "reused run failed",
	)
	require.NoError(t, err)
	require.True(t, persisted)
}

func TestRunBackupJobRecordFailureReturnsCreateError(t *testing.T) {
	fixture := newBackupExecutionFixture(t, false)
	require.NoError(t, fixture.db.Exec(`
		CREATE TRIGGER reject_database_backup_run_insert
		BEFORE INSERT ON docker_database_backup_runs
		BEGIN
			SELECT RAISE(ABORT, 'run insert unavailable');
		END
	`).Error)

	persisted, err := fixture.job.recordFailure(
		context.Background(), fixture.backupID, "", "pre-dispatch failed",
	)
	require.Error(t, err)
	require.False(t, persisted)
}

func TestRunBackupJobReturnsPreflightFailurePersistenceErrors(t *testing.T) {
	t.Run("storage provider", func(t *testing.T) {
		fixture := newBackupExecutionFixture(t, true)
		var provider backupmodels.StorageProvider
		require.NoError(t, fixture.db.First(&provider, 73).Error)
		provider.Credentials = dbtype.EncryptedJSONMap{"bucket": "nightly"}
		require.NoError(t, fixture.db.Save(&provider).Error)
		require.NoError(t, fixture.db.Exec(`
			CREATE TRIGGER reject_storage_failure_update
			BEFORE UPDATE ON docker_database_backup_runs
			WHEN NEW.status = 'failed'
			BEGIN
				SELECT RAISE(ABORT, 'failure update unavailable');
			END
		`).Error)

		require.ErrorContains(t, fixture.job.Handle(context.Background()), "persist storage-configuration backup failure")
	})

	t.Run("database credentials", func(t *testing.T) {
		fixture := newBackupExecutionFixture(t, true)
		require.NoError(t, fixture.db.Exec(
			"UPDATE docker_databases SET credentials = '' WHERE id = ?",
			fixture.job.Payload.DatabaseID,
		).Error)
		require.NoError(t, fixture.db.Exec(`
			CREATE TRIGGER reject_credential_failure_update
			BEFORE UPDATE ON docker_database_backup_runs
			WHEN NEW.status = 'failed'
			BEGIN
				SELECT RAISE(ABORT, 'failure update unavailable');
			END
		`).Error)

		require.ErrorContains(t, fixture.job.Handle(context.Background()), "persist credential backup failure")
	})
}

func TestRunBackupJobFailedHandlesPersistenceError(t *testing.T) {
	fixture := newBackupExecutionFixture(t, true)
	require.NoError(t, fixture.db.Exec(`
		CREATE TRIGGER reject_framework_failure_update
		BEFORE UPDATE ON docker_database_backup_runs
		WHEN NEW.status = 'failed'
		BEGIN
			SELECT RAISE(ABORT, 'failure update unavailable');
		END
	`).Error)

	require.NotPanics(t, func() {
		fixture.job.Failed(context.Background(), errors.New("worker stopped"))
	})
}

func TestRunBackupJobFailedSkipsRetryableAttempt(t *testing.T) {
	fixture := newBackupExecutionFixture(t, true)
	fixture.job.finalAttempt = func(context.Context, error) bool { return false }

	fixture.job.Failed(context.Background(), errors.New("retryable"))

	var persisted dockermodels.DatabaseBackupRun
	require.NoError(t, fixture.db.First(&persisted, "id = ?", fixture.runID).Error)
	require.Equal(t, "triggered", persisted.Status)
}

func TestRunBackupJobTaskAndNilNotificationBranches(t *testing.T) {
	payload := RunBackupPayload{
		BackupID: "backup", DatabaseID: "database", ProjectID: "project",
		ServerID: "server", TeamID: "team", RunID: "run", Source: "manual",
	}
	task, err := NewRunBackupTask(
		payload.BackupID, payload.DatabaseID, payload.ProjectID, payload.ServerID,
		payload.TeamID, payload.RunID, payload.Source,
	)
	require.NoError(t, err)
	decoded, err := pkgjobs.UnmarshalPayload[RunBackupPayload](task)
	require.NoError(t, err)
	require.Equal(t, payload, decoded)

	task, err = NewRunBackupTask("backup", "database", "project", "server", "team", "", "schedule")
	require.NoError(t, err)
	decoded, err = pkgjobs.UnmarshalPayload[RunBackupPayload](task)
	require.NoError(t, err)
	require.Empty(t, decoded.RunID)

	testLogger := zerolog.Nop()
	recorder := &dockerBackupRecordingBroadcaster{}
	job := &RunBackupJob{Deps: &JobDeps{Deps: &pkgjobs.Deps{Logger: &testLogger, Broadcaster: recorder}}}
	job.Payload = payload
	job.dispatchSuccessNotification(
		context.Background(), &dockermodels.DatabaseBackup{NotifyOnSuccess: true}, nil, nil, nil, "",
	)
	job.dispatchFailureNotification(
		context.Background(), &dockermodels.DatabaseBackup{NotifyOnFailure: true}, nil, nil, "failed",
	)

	handler := NewRunBackupJob(payload)
	require.IsType(t, &RunBackupJob{}, handler)

	progress := backupProgressPayload(
		payload, payload.DatabaseID, payload.BackupID, payload.RunID, payload.ServerID,
		&markers.Marker{Type: "backup_step", Value: "uploading"},
	)
	require.Equal(t, payload.Source, progress["source"])
	require.Equal(t, "backup_step", progress["type"])
	require.Equal(t, "uploading", progress["value"])

	markerHandler := job.progressMarkerHandler(
		payload.DatabaseID, payload.BackupID, payload.RunID, payload.ServerID,
	)
	require.NoError(t, markerHandler.OnMarker(
		context.Background(), "task", &markers.Marker{Type: "backup_step", Value: "uploading"},
	))
	require.Equal(t, "docker.database.backup.run.progress", recorder.calls[0].event)
	require.Equal(t, progress, recorder.calls[0].data)
}

func TestRunBackupJobFailedReusesCreatedScheduledRun(t *testing.T) {
	fixture := newBackupExecutionFixture(t, false)

	require.NoError(t, fixture.db.Exec(`
		CREATE TRIGGER fail_backup_run_success_update
		BEFORE UPDATE ON docker_database_backup_runs
		WHEN NEW.status = 'success'
		BEGIN
			SELECT RAISE(ABORT, 'success persistence unavailable');
		END
	`).Error)

	handleErr := fixture.job.Handle(context.Background())
	require.Error(t, handleErr)
	require.Contains(t, handleErr.Error(), "persist backup success state")
	require.NotEmpty(t, fixture.job.runID)

	createdRunID := fixture.job.runID
	fixture.job.Failed(context.Background(), handleErr)

	var runs []dockermodels.DatabaseBackupRun
	require.NoError(t, fixture.db.Where("backup_id = ?", fixture.backupID).Find(&runs).Error)
	require.Len(t, runs, 1, "framework fallback must not create a duplicate scheduled failure row")
	require.Equal(t, createdRunID, runs[0].ID)
	require.Equal(t, "failed", runs[0].Status)
	require.NotNil(t, runs[0].FinishedAt)
	require.NotNil(t, runs[0].Error)
}

type backupExecutionFixture struct {
	db         *gorm.DB
	job        *RunBackupJob
	recorder   *dockerBackupRecordingBroadcaster
	dispatcher *taskrunner.FakeDispatcher
	backupID   string
	runID      string
}

func newBackupExecutionFixture(t *testing.T, precreateRun bool) backupExecutionFixture {
	t.Helper()
	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))

	db, err := gorm.Open(
		sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"),
		&gorm.Config{
			DisableForeignKeyConstraintWhenMigrating: true,
			Logger:                                   logger.Default.LogMode(logger.Silent),
		},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&servermodels.Server{},
		&servermodels.InstalledService{},
		&servermodels.Task{},
		&backupmodels.StorageProvider{},
		&dockermodels.Project{},
		&dockermodels.Database{},
		&dockermodels.DatabaseBackup{},
		&dockermodels.DatabaseBackupRun{},
	))

	const (
		teamID     = "team00000000000000000000001"
		serverID   = "server000000000000000000001"
		projectID  = "project00000000000000000001"
		databaseID = "database0000000000000000001"
		backupID   = "dbbackup0000000000000000001"
		runID      = "backuprun000000000000000001"
	)

	ip := "192.0.2.10"
	server := &servermodels.Server{
		BaseModel:  basemodels.BaseModel{ID: serverID},
		Name:       "docker-host",
		PublicIPv4: &ip,
		PrivateKey: dbtype.EncryptedString("test-private-key"),
	}
	server.TeamID = teamID
	server.UserID = "user00000000000000000000001"
	require.NoError(t, db.Create(server).Error)

	project := &dockermodels.Project{
		BaseModel: basemodels.BaseModel{ID: projectID},
		Name:      "production",
	}
	project.TeamID = teamID
	project.ServerID = serverID
	require.NoError(t, db.Create(project).Error)

	database := &dockermodels.Database{
		BaseModel:     basemodels.BaseModel{ID: databaseID},
		ProjectID:     projectID,
		Name:          "customers",
		Engine:        dockertypes.DatabaseEnginePostgres,
		EngineVersion: "16",
		Credentials: dbtype.EncryptedString(
			`{"Username":"launch","Password":"secret","Database":"customers"}`,
		),
	}
	database.TeamID = teamID
	database.ServerID = serverID
	require.NoError(t, db.Create(database).Error)

	provider := &backupmodels.StorageProvider{
		ID:       73,
		UserID:   server.UserID,
		TeamID:   teamID,
		Provider: backuptypes.StorageDriverS3,
		Credentials: dbtype.EncryptedJSONMap{
			"endpoint": "https://eu2.contabostorage.com",
			"region":   "eu2",
			"bucket":   "nightly",
			"key":      "access-key",
			"secret":   "secret-key",
		},
		Connected: true,
	}
	require.NoError(t, db.Create(provider).Error)

	backup := &dockermodels.DatabaseBackup{
		BaseModel:         basemodels.BaseModel{ID: backupID},
		DatabaseID:        databaseID,
		StorageProviderID: provider.ID,
		Enabled:           true,
	}
	backup.TeamID = teamID
	require.NoError(t, db.Create(backup).Error)

	payloadRunID := ""
	if precreateRun {
		run := &dockermodels.DatabaseBackupRun{
			BaseModel: basemodels.BaseModel{ID: runID},
			BackupID:  backupID,
			Status:    "triggered",
		}
		require.NoError(t, db.Create(run).Error)
		payloadRunID = runID
	}

	nopLogger := zerolog.Nop()
	recorder := &dockerBackupRecordingBroadcaster{}
	dispatcher := taskrunner.NewFakeDispatcher()
	job := &RunBackupJob{
		Deps: &JobDeps{
			Deps: &pkgjobs.Deps{
				DB:          db,
				Logger:      &nopLogger,
				Broadcaster: recorder,
				Dispatcher:  dispatcher,
			},
			Repos:       dockerrepos.NewRegistry(db),
			ServerRepos: serverrepos.NewRegistry(db),
			BackupRepos: backuprepos.NewRegistry(db),
			TaskRunnerDeps: &servertasks.TaskRunnerDeps{
				DB:          db,
				Dispatcher:  dispatcher,
				Logger:      &nopLogger,
				Broadcaster: recorder,
			},
		},
		Payload: RunBackupPayload{
			BackupID:   backupID,
			DatabaseID: databaseID,
			ProjectID:  projectID,
			ServerID:   serverID,
			TeamID:     teamID,
			RunID:      payloadRunID,
			Source:     map[bool]string{true: "manual", false: "schedule"}[precreateRun],
		},
	}

	return backupExecutionFixture{
		db:         db,
		job:        job,
		recorder:   recorder,
		dispatcher: dispatcher,
		backupID:   backupID,
		runID:      payloadRunID,
	}
}
