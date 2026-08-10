package jobs

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/database/serializers"
	backupmodels "github.com/kkz6/launch-go/internal/modules/backup/models"
	backuprepos "github.com/kkz6/launch-go/internal/modules/backup/repositories"
	backuptypes "github.com/kkz6/launch-go/internal/modules/backup/types"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

type manualBackupFixture struct {
	db       *gorm.DB
	deps     *JobDeps
	teamID   string
	serverID string
	backupID string
	provider backupmodels.StorageProvider
}

func newManualBackupFixture(t *testing.T) *manualBackupFixture {
	t.Helper()
	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&servermodels.Server{},
		&servermodels.InstalledService{},
		&backupmodels.StorageProvider{},
		&backupmodels.Backup{},
		&backupmodels.BackupJob{},
		&backupmodels.BackupDatabase{},
	))

	const (
		teamID   = "team00000000000000000000001"
		serverID = "server000000000000000000001"
		backupID = "backup000000000000000000001"
	)

	server := servermodels.Server{Name: "backup-host"}
	server.ID = serverID
	server.TeamID = teamID
	server.UserID = "user00000000000000000000001"
	require.NoError(t, db.Create(&server).Error)

	provider := backupmodels.StorageProvider{
		ID:          71,
		UserID:      server.UserID,
		TeamID:      teamID,
		Provider:    backuptypes.StorageDriverS3,
		Credentials: dbtype.EncryptedJSONMap{"bucket": "nightly"},
		Connected:   true,
	}
	require.NoError(t, db.Create(&provider).Error)

	backup := backupmodels.Backup{
		StorageProviderID: provider.ID,
		CronExpression:    "0 0 * * *",
		IncludeFiles:      "[]",
		ExcludeFiles:      "[]",
		Path:              "servers",
	}
	backup.ID = backupID
	backup.TeamID = teamID
	backup.ServerID = serverID
	require.NoError(t, db.Create(&backup).Error)

	nopLogger := zerolog.Nop()
	return &manualBackupFixture{
		db:       db,
		teamID:   teamID,
		serverID: serverID,
		backupID: backupID,
		provider: provider,
		deps: &JobDeps{
			Deps: &pkgjobs.Deps{
				DB:     db,
				Logger: &nopLogger,
			},
			Repos:       backuprepos.NewRegistry(db),
			ServerRepos: serverrepos.NewRegistry(db),
		},
	}
}

func createManualBackupRun(
	t *testing.T,
	f *manualBackupFixture,
	status backuptypes.BackupJobStatus,
) *backupmodels.BackupJob {
	t.Helper()
	run := &backupmodels.BackupJob{
		Status:            status,
		BackupID:          f.backupID,
		StorageProviderID: f.provider.ID,
	}
	run.TeamID = f.teamID
	require.NoError(t, f.db.Create(run).Error)
	return run
}

func TestRunManualBackupJobRecordsStorageConfigurationFailureForLegacyPayload(t *testing.T) {
	f := newManualBackupFixture(t)
	job := &RunManualBackupJob{
		Deps: f.deps,
		Payload: RunManualBackupPayload{
			ServerID: f.serverID,
			BackupID: f.backupID,
		},
	}

	require.NoError(t, job.Handle(context.Background()))

	var jobs []backupmodels.BackupJob
	require.NoError(t, f.db.Where("backup_id = ?", f.backupID).Find(&jobs).Error)
	require.Len(t, jobs, 1)
	require.Equal(t, backuptypes.BackupJobStatusFailed, jobs[0].Status)
	require.Nil(t, jobs[0].TaskID)
	require.NotNil(t, jobs[0].Error)
	require.True(t, strings.Contains(*jobs[0].Error, "incomplete S3 credentials"))
}

func TestRunManualBackupJobAdoptsPrecreatedRun(t *testing.T) {
	f := newManualBackupFixture(t)
	run := &backupmodels.BackupJob{
		Status:            backuptypes.BackupJobStatusPending,
		BackupID:          f.backupID,
		StorageProviderID: f.provider.ID,
	}
	run.TeamID = f.teamID
	require.NoError(t, f.db.Create(run).Error)

	job := &RunManualBackupJob{
		Deps: f.deps,
		Payload: RunManualBackupPayload{
			ServerID: f.serverID,
			BackupID: f.backupID,
			TeamID:   f.teamID,
			JobID:    run.ID,
		},
	}

	require.NoError(t, job.Handle(context.Background()))

	var jobs []backupmodels.BackupJob
	require.NoError(t, f.db.Where("backup_id = ?", f.backupID).Find(&jobs).Error)
	require.Len(t, jobs, 1, "the worker must update the HTTP-created row instead of inserting another")
	require.Equal(t, run.ID, jobs[0].ID)
	require.Equal(t, backuptypes.BackupJobStatusFailed, jobs[0].Status)
	require.NotNil(t, jobs[0].Error)
	require.Contains(t, *jobs[0].Error, "incomplete S3 credentials")
}

func TestRunManualBackupJobAdoptionDoesNotPreloadProviderCredentials(t *testing.T) {
	f := newManualBackupFixture(t)
	run := &backupmodels.BackupJob{
		Status:            backuptypes.BackupJobStatusPending,
		BackupID:          f.backupID,
		StorageProviderID: f.provider.ID,
	}
	run.TeamID = f.teamID
	require.NoError(t, f.db.Create(run).Error)
	require.NoError(t, f.db.Exec(
		"UPDATE storage_providers SET credentials = ? WHERE id = ?",
		"not-valid-encrypted-json",
		f.provider.ID,
	).Error)

	job := &RunManualBackupJob{
		Deps: f.deps,
		Payload: RunManualBackupPayload{
			ServerID: f.serverID,
			BackupID: f.backupID,
			TeamID:   f.teamID,
			JobID:    run.ID,
		},
	}

	require.NoError(t, job.adoptPrecreatedJob(context.Background()))
	require.NotNil(t, job.job)
	require.Equal(t, run.ID, job.job.ID)
}

func TestRunManualBackupJobRejectsPrecreatedRunOutsidePayloadScope(t *testing.T) {
	tests := []struct {
		name        string
		jobBackupID string
		jobTeamID   string
	}{
		{name: "different backup", jobBackupID: "another-backup", jobTeamID: "same"},
		{name: "different team", jobBackupID: "same", jobTeamID: "another-team"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newManualBackupFixture(t)
			backupID := f.backupID
			if tt.jobBackupID != "same" {
				backupID = tt.jobBackupID
			}
			teamID := f.teamID
			if tt.jobTeamID != "same" {
				teamID = tt.jobTeamID
			}
			run := &backupmodels.BackupJob{
				Status:            backuptypes.BackupJobStatusPending,
				BackupID:          backupID,
				StorageProviderID: f.provider.ID,
			}
			run.TeamID = teamID
			require.NoError(t, f.db.Create(run).Error)

			job := &RunManualBackupJob{
				Deps: f.deps,
				Payload: RunManualBackupPayload{
					ServerID: f.serverID,
					BackupID: f.backupID,
					TeamID:   f.teamID,
					JobID:    run.ID,
				},
			}

			require.NoError(t, job.Handle(context.Background()))
			var persisted backupmodels.BackupJob
			require.NoError(t, f.db.First(&persisted, "id = ?", run.ID).Error)
			require.Equal(t, backuptypes.BackupJobStatusPending, persisted.Status)
		})
	}
}

func TestRunManualBackupJobFirstAttemptReplaySkipsAlreadyRunningRow(t *testing.T) {
	f := newManualBackupFixture(t)
	run := createManualBackupRun(t, f, backuptypes.BackupJobStatusRunning)
	require.NoError(t, f.db.Delete(&backupmodels.Backup{}, "id = ?", f.backupID).Error)

	job := &RunManualBackupJob{
		Deps: f.deps,
		Payload: RunManualBackupPayload{
			ServerID: f.serverID,
			BackupID: f.backupID,
			TeamID:   f.teamID,
			JobID:    run.ID,
		},
	}

	require.NoError(t, job.Handle(context.Background()))
	var persisted backupmodels.BackupJob
	require.NoError(t, f.db.First(&persisted, "id = ?", run.ID).Error)
	require.Equal(t, backuptypes.BackupJobStatusRunning, persisted.Status)
}

type manualBackupBroadcast struct {
	broadcast.NopBroadcaster
	teamID      string
	event       string
	data        map[string]any
	calls       int
	onBroadcast func(event string)
}

func (b *manualBackupBroadcast) BroadcastToTeam(teamID, event string, data any) {
	b.calls++
	if b.onBroadcast != nil {
		b.onBroadcast(event)
	}
	b.teamID = teamID
	b.event = event
	b.data, _ = data.(map[string]any)
}

func TestRunManualBackupJobPersistsLifecycleBeforeBroadcast(t *testing.T) {
	t.Run("started", func(t *testing.T) {
		f := newManualBackupFixture(t)
		run := createManualBackupRun(t, f, backuptypes.BackupJobStatusPending)
		recorder := &manualBackupBroadcast{}
		recorder.onBroadcast = func(event string) {
			require.Equal(t, "backup.run.started", event)
			var persisted backupmodels.BackupJob
			require.NoError(t, f.db.First(&persisted, "id = ?", run.ID).Error)
			require.Equal(t, backuptypes.BackupJobStatusRunning, persisted.Status)
			require.NotNil(t, persisted.TaskID)
			require.Equal(t, "task-started", *persisted.TaskID)
		}
		f.deps.Broadcaster = recorder
		job := &RunManualBackupJob{
			Deps:    f.deps,
			Payload: RunManualBackupPayload{ServerID: f.serverID, TeamID: f.teamID},
			job:     run,
		}

		job.recordStarted(context.Background(), "task-started")
		require.Equal(t, 1, recorder.calls)
	})

	t.Run("succeeded", func(t *testing.T) {
		f := newManualBackupFixture(t)
		run := createManualBackupRun(t, f, backuptypes.BackupJobStatusRunning)
		recorder := &manualBackupBroadcast{}
		recorder.onBroadcast = func(event string) {
			require.Equal(t, "backup.run.succeeded", event)
			var persisted backupmodels.BackupJob
			require.NoError(t, f.db.First(&persisted, "id = ?", run.ID).Error)
			require.Equal(t, backuptypes.BackupJobStatusFinished, persisted.Status)
			require.NotNil(t, persisted.Size)
			require.Equal(t, 8192, *persisted.Size)
			require.NotNil(t, persisted.TaskID)
			require.Equal(t, "task-succeeded", *persisted.TaskID)
		}
		f.deps.Broadcaster = recorder
		job := &RunManualBackupJob{
			Deps:    f.deps,
			Payload: RunManualBackupPayload{ServerID: f.serverID, TeamID: f.teamID},
			job:     run,
		}
		taskID := "task-succeeded"

		require.NoError(t, job.recordSuccess(context.Background(), 8192, &taskID))
		require.Equal(t, 1, recorder.calls)
	})

	t.Run("failed", func(t *testing.T) {
		f := newManualBackupFixture(t)
		run := createManualBackupRun(t, f, backuptypes.BackupJobStatusRunning)
		recorder := &manualBackupBroadcast{}
		recorder.onBroadcast = func(event string) {
			require.Equal(t, "backup.run.failed", event)
			var persisted backupmodels.BackupJob
			require.NoError(t, f.db.First(&persisted, "id = ?", run.ID).Error)
			require.Equal(t, backuptypes.BackupJobStatusFailed, persisted.Status)
			require.NotNil(t, persisted.Error)
			require.Equal(t, "upload failed", *persisted.Error)
			require.NotNil(t, persisted.TaskID)
			require.Equal(t, "task-failed", *persisted.TaskID)
		}
		f.deps.Broadcaster = recorder
		job := &RunManualBackupJob{
			Deps:    f.deps,
			Payload: RunManualBackupPayload{ServerID: f.serverID, TeamID: f.teamID},
			job:     run,
		}
		taskID := "task-failed"

		require.NoError(t, job.recordFailure(context.Background(), "upload failed", &taskID))
		require.Equal(t, 1, recorder.calls)
	})
}

func TestRunManualBackupJobDoesNotBroadcastRejectedOrFailedPersistence(t *testing.T) {
	t.Run("terminal row", func(t *testing.T) {
		f := newManualBackupFixture(t)
		run := createManualBackupRun(t, f, backuptypes.BackupJobStatusFinished)
		recorder := &manualBackupBroadcast{}
		f.deps.Broadcaster = recorder
		job := &RunManualBackupJob{
			Deps:    f.deps,
			Payload: RunManualBackupPayload{ServerID: f.serverID, TeamID: f.teamID},
			job:     run,
		}
		taskID := "late-task"

		job.recordStarted(context.Background(), taskID)
		require.NoError(t, job.recordSuccess(context.Background(), 1, &taskID))
		require.NoError(t, job.recordFailure(context.Background(), "late failure", &taskID))

		require.Zero(t, recorder.calls)
		var persisted backupmodels.BackupJob
		require.NoError(t, f.db.First(&persisted, "id = ?", run.ID).Error)
		require.Equal(t, backuptypes.BackupJobStatusFinished, persisted.Status)
	})

	t.Run("database error", func(t *testing.T) {
		f := newManualBackupFixture(t)
		run := createManualBackupRun(t, f, backuptypes.BackupJobStatusRunning)
		recorder := &manualBackupBroadcast{}
		f.deps.Broadcaster = recorder
		job := &RunManualBackupJob{
			Deps:    f.deps,
			Payload: RunManualBackupPayload{ServerID: f.serverID, TeamID: f.teamID},
			job:     run,
		}
		sqlDB, err := f.db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())

		require.Error(t, job.recordFailure(context.Background(), "database unavailable", nil))
		require.Zero(t, recorder.calls)
	})
}

func TestRunManualBackupJobFinalFailureUpdatesAdoptedRunBeforeBackupLoad(t *testing.T) {
	f := newManualBackupFixture(t)
	run := &backupmodels.BackupJob{
		Status:            backuptypes.BackupJobStatusPending,
		BackupID:          f.backupID,
		StorageProviderID: f.provider.ID,
	}
	run.TeamID = f.teamID
	require.NoError(t, f.db.Create(run).Error)
	require.NoError(t, f.db.Delete(&backupmodels.Backup{}, "id = ?", f.backupID).Error)

	recorder := &manualBackupBroadcast{}
	f.deps.Broadcaster = recorder
	job := &RunManualBackupJob{
		Deps: f.deps,
		Payload: RunManualBackupPayload{
			ServerID: f.serverID,
			BackupID: f.backupID,
			TeamID:   f.teamID,
			JobID:    run.ID,
		},
	}

	runErr := job.Handle(context.Background())
	require.ErrorContains(t, runErr, "find backup")
	job.Failed(context.Background(), runErr)

	var persisted backupmodels.BackupJob
	require.NoError(t, f.db.First(&persisted, "id = ?", run.ID).Error)
	require.Equal(t, backuptypes.BackupJobStatusFailed, persisted.Status)
	require.NotNil(t, persisted.Error)
	require.Contains(t, *persisted.Error, "find backup")
	require.Equal(t, f.teamID, recorder.teamID)
	require.Equal(t, "backup.run.failed", recorder.event)
	require.Equal(t, run.ID, recorder.data["job_id"])
	require.Equal(t, f.backupID, recorder.data["backup_id"])
	require.Equal(t, f.serverID, recorder.data["server_id"])
	require.Equal(t, *persisted.Error, recorder.data["error"])
}

func TestRunManualBackupJobFailureUsesScopedPayloadIDFallback(t *testing.T) {
	f := newManualBackupFixture(t)
	run := &backupmodels.BackupJob{
		Status:            backuptypes.BackupJobStatusPending,
		BackupID:          f.backupID,
		StorageProviderID: f.provider.ID,
	}
	run.TeamID = f.teamID
	require.NoError(t, f.db.Create(run).Error)

	recorder := &manualBackupBroadcast{}
	f.deps.Broadcaster = recorder
	job := &RunManualBackupJob{
		Deps: f.deps,
		Payload: RunManualBackupPayload{
			ServerID: f.serverID,
			BackupID: f.backupID,
			TeamID:   f.teamID,
			JobID:    run.ID,
		},
	}

	require.NoError(t, job.recordFailure(context.Background(), "worker stopped before adoption", nil))

	var persisted backupmodels.BackupJob
	require.NoError(t, f.db.First(&persisted, "id = ?", run.ID).Error)
	require.Equal(t, backuptypes.BackupJobStatusFailed, persisted.Status)
	require.NotNil(t, persisted.Error)
	require.Equal(t, "worker stopped before adoption", *persisted.Error)
	require.Equal(t, f.teamID, recorder.teamID)
	require.Equal(t, "backup.run.failed", recorder.event)
	require.Equal(t, run.ID, recorder.data["job_id"])
}

func TestNewRunManualBackupTaskCarriesPrecreatedRunIdentity(t *testing.T) {
	userID := "user-01"
	task, err := NewRunManualBackupTask("server-01", "backup-01", "team-01", "job-01", &userID)
	require.NoError(t, err)

	payload, err := pkgjobs.UnmarshalPayload[RunManualBackupPayload](task)
	require.NoError(t, err)
	require.Equal(t, "server-01", payload.ServerID)
	require.Equal(t, "backup-01", payload.BackupID)
	require.Equal(t, "team-01", payload.TeamID)
	require.Equal(t, "job-01", payload.JobID)
	require.NotNil(t, payload.UserID)
	require.Equal(t, userID, *payload.UserID)
}

func TestRunManualBackupTaskIDIsDeterministicPerDurableRun(t *testing.T) {
	require.Equal(t, "backup-run-manual:job-01", runManualBackupTaskID("job-01"))
	require.Equal(t, runManualBackupTaskID("job-01"), runManualBackupTaskID("job-01"))
	require.NotEqual(t, runManualBackupTaskID("job-01"), runManualBackupTaskID("job-02"))
}

func TestRunManualBackupJobFailedIsNilErrorSafe(t *testing.T) {
	nopLogger := zerolog.Nop()
	job := &RunManualBackupJob{
		Deps: &JobDeps{
			Deps: &pkgjobs.Deps{Logger: &nopLogger},
		},
		Payload: RunManualBackupPayload{
			ServerID: "server-01",
			BackupID: "backup-01",
		},
	}

	require.NotPanics(t, func() {
		job.Failed(context.Background(), nil)
	})
}

func TestRunManualBackupJobRejectsAnotherTeamsStorageProvider(t *testing.T) {
	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&backupmodels.StorageProvider{}))

	provider := backupmodels.StorageProvider{
		ID:       301,
		UserID:   "user-b",
		TeamID:   "team-b",
		Provider: backuptypes.StorageDriverS3,
		Credentials: dbtype.EncryptedJSONMap{
			"bucket": "private",
			"key":    "foreign-access",
			"secret": "foreign-secret",
		},
	}
	require.NoError(t, db.Create(&provider).Error)

	job := &RunManualBackupJob{
		Deps: &JobDeps{Repos: backuprepos.NewRegistry(db)},
		backup: &backupmodels.Backup{
			StorageProviderID: provider.ID,
		},
	}
	job.backup.TeamID = "team-a"

	_, err = job.loadS3Creds(context.Background())
	require.ErrorContains(t, err, "not found")
}

func TestRunManualBackupJobCompletesTrackedTask(t *testing.T) {
	f, job, dispatcher, recorder, run := newManualBackupExecutionFixture(t)
	dispatcher.DefaultResult.Output = "::LAUNCH::object_key::servers/backup.tar.gz\n::LAUNCH::size_bytes::8192"

	require.NoError(t, job.Handle(context.Background()))

	var persisted backupmodels.BackupJob
	require.NoError(t, f.db.First(&persisted, "id = ?", run.ID).Error)
	require.Equal(t, backuptypes.BackupJobStatusFinished, persisted.Status)
	require.Equal(t, 8192, *persisted.Size)
	require.NotNil(t, persisted.TaskID)
	require.Len(t, dispatcher.Executions, 1)
	require.Contains(t, dispatcher.Executions[0].Script, "--force-path-style")
	require.Equal(t, "backup.run.succeeded", recorder.event)
	require.Equal(t, *persisted.TaskID, recorder.data["task_id"])
}

func TestRunManualBackupJobRecordsEmptyTaskFailure(t *testing.T) {
	f, job, dispatcher, recorder, run := newManualBackupExecutionFixture(t)
	dispatcher.SetDefaultFailure(17, "")

	require.NoError(t, job.Handle(context.Background()))

	var persisted backupmodels.BackupJob
	require.NoError(t, f.db.First(&persisted, "id = ?", run.ID).Error)
	require.Equal(t, backuptypes.BackupJobStatusFailed, persisted.Status)
	require.NotNil(t, persisted.TaskID)
	require.NotNil(t, persisted.Error)
	require.Equal(t, "backup command exited with code 17 without output", *persisted.Error)
	require.Equal(t, "backup.run.failed", recorder.event)
	require.Equal(t, *persisted.TaskID, recorder.data["task_id"])
}

func TestRunManualBackupJobReturnsTerminalPersistenceFailures(t *testing.T) {
	t.Run("started", func(t *testing.T) {
		f, job, _, recorder, run := newManualBackupExecutionFixture(t)
		require.NoError(t, f.db.Exec(`
			CREATE TRIGGER reject_backup_job_started
			BEFORE UPDATE ON backup_jobs
			WHEN NEW.status = 'running' AND NEW.task_id IS NOT NULL
			BEGIN
				SELECT RAISE(ABORT, 'running persistence unavailable');
			END
		`).Error)

		require.NoError(t, job.Handle(context.Background()))
		var persisted backupmodels.BackupJob
		require.NoError(t, f.db.First(&persisted, "id = ?", run.ID).Error)
		require.Equal(t, backuptypes.BackupJobStatusFinished, persisted.Status)
		for _, event := range []string{"backup.run.started"} {
			require.NotEqual(t, event, recorder.event)
		}
	})

	t.Run("succeeded", func(t *testing.T) {
		f, job, _, recorder, run := newManualBackupExecutionFixture(t)
		require.NoError(t, f.db.Exec(`
			CREATE TRIGGER reject_backup_job_success
			BEFORE UPDATE ON backup_jobs
			WHEN NEW.status = 'finished'
			BEGIN
				SELECT RAISE(ABORT, 'success persistence unavailable');
			END
		`).Error)

		err := job.Handle(context.Background())
		require.ErrorContains(t, err, "persist backup success state")
		var persisted backupmodels.BackupJob
		require.NoError(t, f.db.First(&persisted, "id = ?", run.ID).Error)
		require.Equal(t, backuptypes.BackupJobStatusRunning, persisted.Status)
		require.NotEqual(t, "backup.run.succeeded", recorder.event)
	})

	t.Run("failed", func(t *testing.T) {
		f, job, dispatcher, recorder, run := newManualBackupExecutionFixture(t)
		dispatcher.SetDefaultFailure(17, "upload failed")
		require.NoError(t, f.db.Exec(`
			CREATE TRIGGER reject_backup_job_failure
			BEFORE UPDATE ON backup_jobs
			WHEN NEW.status = 'failed'
			BEGIN
				SELECT RAISE(ABORT, 'failure persistence unavailable');
			END
		`).Error)

		err := job.Handle(context.Background())
		require.ErrorContains(t, err, "persist backup failure state")
		var persisted backupmodels.BackupJob
		require.NoError(t, f.db.First(&persisted, "id = ?", run.ID).Error)
		require.Equal(t, backuptypes.BackupJobStatusRunning, persisted.Status)
		require.NotEqual(t, "backup.run.failed", recorder.event)
	})
}

func TestRunManualBackupJobRejectsInvalidQueuedScope(t *testing.T) {
	t.Run("missing team", func(t *testing.T) {
		f := newManualBackupFixture(t)
		job := &RunManualBackupJob{
			Deps: f.deps,
			Payload: RunManualBackupPayload{
				ServerID: f.serverID,
				BackupID: f.backupID,
				JobID:    "job-01",
			},
		}
		require.ErrorContains(t, job.Handle(context.Background()), "team id is required")
	})

	t.Run("backup team", func(t *testing.T) {
		f := newManualBackupFixture(t)
		run := createManualBackupRun(t, f, backuptypes.BackupJobStatusPending)
		require.NoError(t, f.db.Model(run).Update("team_id", "other-team").Error)
		job := &RunManualBackupJob{
			Deps: f.deps,
			Payload: RunManualBackupPayload{
				ServerID: f.serverID,
				BackupID: f.backupID,
				TeamID:   "other-team",
				JobID:    run.ID,
			},
		}
		require.ErrorContains(t, job.Handle(context.Background()), "backup team does not match")
	})

	t.Run("backup server", func(t *testing.T) {
		f := newManualBackupFixture(t)
		run := createManualBackupRun(t, f, backuptypes.BackupJobStatusPending)
		job := &RunManualBackupJob{
			Deps: f.deps,
			Payload: RunManualBackupPayload{
				ServerID: "other-server",
				BackupID: f.backupID,
				TeamID:   f.teamID,
				JobID:    run.ID,
			},
		}
		require.ErrorContains(t, job.Handle(context.Background()), "backup server does not match")
	})

	t.Run("storage provider changed", func(t *testing.T) {
		f := newManualBackupFixture(t)
		run := createManualBackupRun(t, f, backuptypes.BackupJobStatusPending)
		require.NoError(t, f.db.Model(run).Update("storage_provider_id", f.provider.ID+1).Error)
		job := &RunManualBackupJob{
			Deps: f.deps,
			Payload: RunManualBackupPayload{
				ServerID: f.serverID,
				BackupID: f.backupID,
				TeamID:   f.teamID,
				JobID:    run.ID,
			},
		}
		require.ErrorContains(t, job.Handle(context.Background()), "storage provider changed")
	})

	t.Run("preset job scope", func(t *testing.T) {
		f := newManualBackupFixture(t)
		run := createManualBackupRun(t, f, backuptypes.BackupJobStatusPending)
		run.BackupID = "other-backup"
		job := &RunManualBackupJob{
			Deps: f.deps,
			Payload: RunManualBackupPayload{
				ServerID: f.serverID,
				BackupID: f.backupID,
				TeamID:   f.teamID,
			},
			job: run,
		}
		require.ErrorContains(t, job.Handle(context.Background()), "backup job scope does not match")
	})

	t.Run("server team", func(t *testing.T) {
		f := newManualBackupFixture(t)
		run := createManualBackupRun(t, f, backuptypes.BackupJobStatusPending)
		require.NoError(t, f.db.Model(&servermodels.Server{}).
			Where("id = ?", f.serverID).Update("team_id", "other-team").Error)
		job := &RunManualBackupJob{
			Deps: f.deps,
			Payload: RunManualBackupPayload{
				ServerID: f.serverID,
				BackupID: f.backupID,
				TeamID:   f.teamID,
				JobID:    run.ID,
			},
		}
		require.ErrorContains(t, job.Handle(context.Background()), "server team does not match")
	})
}

func TestRunManualBackupJobRecordsConfigurationFailures(t *testing.T) {
	tests := []struct {
		name      string
		include   string
		exclude   string
		linkDB    bool
		wantError string
	}{
		{name: "include files", include: "{", exclude: "[]", wantError: "invalid include files configuration"},
		{name: "exclude files", include: "[]", exclude: "{", wantError: "invalid exclude files configuration"},
		{name: "database wiring", include: "[]", exclude: "[]", linkDB: true, wantError: "database module is not wired"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newManualBackupFixture(t)
			f.provider.Credentials = dbtype.EncryptedJSONMap{
				"endpoint": "https://objects.example.com",
				"region":   "eu-1",
				"bucket":   "nightly",
				"key":      "access",
				"secret":   "secret",
			}
			require.NoError(t, f.db.Save(&f.provider).Error)
			require.NoError(t, f.db.Model(&backupmodels.Backup{}).
				Where("id = ?", f.backupID).
				Updates(map[string]any{"include_files": tt.include, "exclude_files": tt.exclude}).Error)
			if tt.linkDB {
				require.NoError(t, f.db.Create(&backupmodels.BackupDatabase{
					BackupID: f.backupID, DatabaseID: "database-01",
				}).Error)
			}
			run := createManualBackupRun(t, f, backuptypes.BackupJobStatusPending)
			job := &RunManualBackupJob{
				Deps: f.deps,
				Payload: RunManualBackupPayload{
					ServerID: f.serverID,
					BackupID: f.backupID,
					TeamID:   f.teamID,
					JobID:    run.ID,
				},
			}

			require.NoError(t, job.Handle(context.Background()))
			var persisted backupmodels.BackupJob
			require.NoError(t, f.db.First(&persisted, "id = ?", run.ID).Error)
			require.Equal(t, backuptypes.BackupJobStatusFailed, persisted.Status)
			require.NotNil(t, persisted.Error)
			require.Contains(t, *persisted.Error, tt.wantError)
		})
	}
}

func TestRunManualBackupJobReturnsEarlyPersistenceErrors(t *testing.T) {
	t.Run("legacy job creation", func(t *testing.T) {
		f := newManualBackupFixture(t)
		require.NoError(t, f.db.Exec(`
			CREATE TRIGGER reject_backup_job_insert
			BEFORE INSERT ON backup_jobs
			BEGIN
				SELECT RAISE(ABORT, 'job insert unavailable');
			END
		`).Error)
		job := &RunManualBackupJob{
			Deps: f.deps,
			Payload: RunManualBackupPayload{
				ServerID: f.serverID,
				BackupID: f.backupID,
			},
		}
		require.ErrorContains(t, job.Handle(context.Background()), "create backup job row")
	})

	t.Run("credential failure state", func(t *testing.T) {
		f := newManualBackupFixture(t)
		run := createManualBackupRun(t, f, backuptypes.BackupJobStatusPending)
		require.NoError(t, f.db.Exec(`
			CREATE TRIGGER reject_credential_failure
			BEFORE UPDATE ON backup_jobs
			WHEN NEW.status = 'failed'
			BEGIN
				SELECT RAISE(ABORT, 'failure update unavailable');
			END
		`).Error)
		job := &RunManualBackupJob{
			Deps: f.deps,
			Payload: RunManualBackupPayload{
				ServerID: f.serverID,
				BackupID: f.backupID,
				TeamID:   f.teamID,
				JobID:    run.ID,
			},
		}
		require.ErrorContains(t, job.Handle(context.Background()), "persist storage credential failure")
	})

	t.Run("claim", func(t *testing.T) {
		f := newManualBackupFixture(t)
		run := createManualBackupRun(t, f, backuptypes.BackupJobStatusPending)
		sqlDB, err := f.db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
		job := &RunManualBackupJob{
			Deps: f.deps,
			Payload: RunManualBackupPayload{
				ServerID: f.serverID,
				BackupID: f.backupID,
				TeamID:   f.teamID,
				JobID:    run.ID,
			},
		}
		require.ErrorContains(t, job.Handle(context.Background()), "claim pre-created backup job")
	})
}

func TestRunManualBackupJobReturnsInvalidConfigurationPersistenceErrors(t *testing.T) {
	tests := []struct {
		name    string
		include string
		exclude string
		linkDB  bool
		want    string
	}{
		{name: "include", include: "{", exclude: "[]", want: "persist include-files failure"},
		{name: "exclude", include: "[]", exclude: "{", want: "persist exclude-files failure"},
		{name: "database", include: "[]", exclude: "[]", linkDB: true, want: "persist database-dump failure"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newManualBackupFixture(t)
			f.provider.Credentials = dbtype.EncryptedJSONMap{
				"endpoint": "https://objects.example.com", "region": "eu-1",
				"bucket": "nightly", "key": "access", "secret": "secret",
			}
			require.NoError(t, f.db.Save(&f.provider).Error)
			require.NoError(t, f.db.Model(&backupmodels.Backup{}).
				Where("id = ?", f.backupID).
				Updates(map[string]any{"include_files": tt.include, "exclude_files": tt.exclude}).Error)
			if tt.linkDB {
				require.NoError(t, f.db.Create(&backupmodels.BackupDatabase{
					BackupID: f.backupID, DatabaseID: "database-01",
				}).Error)
			}
			run := createManualBackupRun(t, f, backuptypes.BackupJobStatusPending)
			require.NoError(t, f.db.Exec(`
				CREATE TRIGGER reject_configuration_failure
				BEFORE UPDATE ON backup_jobs
				WHEN NEW.status = 'failed'
				BEGIN
					SELECT RAISE(ABORT, 'failure update unavailable');
				END
			`).Error)
			job := &RunManualBackupJob{
				Deps: f.deps,
				Payload: RunManualBackupPayload{
					ServerID: f.serverID, BackupID: f.backupID, TeamID: f.teamID, JobID: run.ID,
				},
			}
			require.ErrorContains(t, job.Handle(context.Background()), tt.want)
		})
	}
}

func TestRunManualBackupJobHelperBranches(t *testing.T) {
	t.Run("paths", func(t *testing.T) {
		paths, err := parseBackupPaths("  ")
		require.NoError(t, err)
		require.Nil(t, paths)
		paths, err = parseBackupPaths(`["/var/www","/etc"]`)
		require.NoError(t, err)
		require.Equal(t, []string{"/var/www", "/etc"}, paths)
		_, err = parseBackupPaths("{")
		require.Error(t, err)
	})

	t.Run("empty success fields", func(t *testing.T) {
		f := newManualBackupFixture(t)
		run := createManualBackupRun(t, f, backuptypes.BackupJobStatusRunning)
		recorder := &manualBackupBroadcast{}
		f.deps.Broadcaster = recorder
		job := &RunManualBackupJob{
			Deps: f.deps, Payload: RunManualBackupPayload{ServerID: f.serverID}, job: run,
		}
		require.NoError(t, job.recordSuccess(context.Background(), 0, nil))
		require.Nil(t, job.job.Size)
		require.Nil(t, job.job.TaskID)
		_, hasTaskID := recorder.data["task_id"]
		require.False(t, hasTaskID)
	})

	t.Run("empty failure message", func(t *testing.T) {
		f := newManualBackupFixture(t)
		run := createManualBackupRun(t, f, backuptypes.BackupJobStatusPending)
		job := &RunManualBackupJob{
			Deps: f.deps, Payload: RunManualBackupPayload{ServerID: f.serverID}, job: run,
		}
		require.NoError(t, job.recordFailure(context.Background(), "  ", nil))
		require.Equal(t, "backup failed without an error message", *job.job.Error)
	})

	t.Run("missing failure persistence context", func(t *testing.T) {
		logger := zerolog.Nop()
		job := &RunManualBackupJob{Deps: &JobDeps{Deps: &pkgjobs.Deps{Logger: &logger}}}
		require.NoError(t, job.recordFailure(context.Background(), "failed", nil))

		f := newManualBackupFixture(t)
		job = &RunManualBackupJob{Deps: f.deps}
		require.NoError(t, job.recordFailure(context.Background(), "failed", nil))
	})

	t.Run("broadcast fallbacks", func(t *testing.T) {
		logger := zerolog.Nop()
		recorder := &manualBackupBroadcast{}
		deps := &JobDeps{Deps: &pkgjobs.Deps{Logger: &logger, Broadcaster: recorder}}
		job := &RunManualBackupJob{Deps: deps, Payload: RunManualBackupPayload{TeamID: "payload-team"}}
		job.broadcast("payload", nil)
		require.Equal(t, "payload-team", recorder.teamID)
		require.Equal(t, "payload-team", recorder.data["team_id"])

		job = &RunManualBackupJob{Deps: deps, backup: &backupmodels.Backup{}}
		job.backup.TeamID = "backup-team"
		job.broadcast("backup", map[string]any{"team_id": "explicit-team"})
		require.Equal(t, "backup-team", recorder.teamID)
		require.Equal(t, "explicit-team", recorder.data["team_id"])

		calls := recorder.calls
		job = &RunManualBackupJob{Deps: deps}
		job.broadcast("ignored", nil)
		require.Equal(t, calls, recorder.calls)
	})

	t.Run("adoption guards", func(t *testing.T) {
		f := newManualBackupFixture(t)
		run := createManualBackupRun(t, f, backuptypes.BackupJobStatusPending)
		job := &RunManualBackupJob{Deps: f.deps, job: run}
		require.NoError(t, job.adoptPrecreatedJob(context.Background()))

		job = &RunManualBackupJob{Deps: f.deps}
		require.NoError(t, job.adoptPrecreatedJob(context.Background()))

		job.Payload.JobID = run.ID
		require.ErrorContains(t, job.adoptPrecreatedJob(context.Background()), "team id is required")

		job.Payload.TeamID = f.teamID
		job.Payload.BackupID = "wrong-backup"
		require.Error(t, job.adoptPrecreatedJob(context.Background()))
	})

	t.Run("storage driver", func(t *testing.T) {
		f := newManualBackupFixture(t)
		require.NoError(t, f.db.Model(&backupmodels.StorageProvider{}).
			Where("id = ?", f.provider.ID).
			Update("provider", backuptypes.StorageDriverDropbox).Error)
		job := &RunManualBackupJob{Deps: f.deps, backup: &backupmodels.Backup{StorageProviderID: f.provider.ID}}
		job.backup.TeamID = f.teamID
		_, err := job.loadS3Creds(context.Background())
		require.ErrorContains(t, err, "not an S3 driver")
	})

	t.Run("legacy task", func(t *testing.T) {
		task, err := NewRunManualBackupTask("server", "backup", "", "", nil)
		require.NoError(t, err)
		payload, err := pkgjobs.UnmarshalPayload[RunManualBackupPayload](task)
		require.NoError(t, err)
		require.Empty(t, payload.JobID)
	})
}

func TestRunManualBackupJobFailedHandlesTerminalAndAdoptionErrors(t *testing.T) {
	t.Run("retry", func(t *testing.T) {
		f := newManualBackupFixture(t)
		run := createManualBackupRun(t, f, backuptypes.BackupJobStatusPending)
		job := &RunManualBackupJob{
			Deps: f.deps,
			Payload: RunManualBackupPayload{
				ServerID: f.serverID, BackupID: f.backupID, TeamID: f.teamID, JobID: run.ID,
			},
			job:          run,
			finalAttempt: func(context.Context, error) bool { return false },
		}
		job.Failed(context.Background(), errors.New("retryable"))
		var persisted backupmodels.BackupJob
		require.NoError(t, f.db.First(&persisted, "id = ?", run.ID).Error)
		require.Equal(t, backuptypes.BackupJobStatusPending, persisted.Status)
	})

	t.Run("terminal", func(t *testing.T) {
		f := newManualBackupFixture(t)
		run := createManualBackupRun(t, f, backuptypes.BackupJobStatusFinished)
		job := &RunManualBackupJob{
			Deps: f.deps,
			Payload: RunManualBackupPayload{
				ServerID: f.serverID, BackupID: f.backupID, TeamID: f.teamID, JobID: run.ID,
			},
			job: run,
		}
		job.Failed(context.Background(), errors.New("late failure"))
		var persisted backupmodels.BackupJob
		require.NoError(t, f.db.First(&persisted, "id = ?", run.ID).Error)
		require.Equal(t, backuptypes.BackupJobStatusFinished, persisted.Status)
	})

	t.Run("adoption", func(t *testing.T) {
		f := newManualBackupFixture(t)
		job := &RunManualBackupJob{
			Deps: f.deps,
			Payload: RunManualBackupPayload{
				ServerID: f.serverID, BackupID: f.backupID, TeamID: f.teamID, JobID: "missing-job",
			},
		}
		require.NotPanics(t, func() {
			job.Failed(context.Background(), errors.New("worker stopped"))
		})
	})

	t.Run("failure persistence", func(t *testing.T) {
		f := newManualBackupFixture(t)
		run := createManualBackupRun(t, f, backuptypes.BackupJobStatusPending)
		require.NoError(t, f.db.Exec(`
			CREATE TRIGGER reject_final_failure
			BEFORE UPDATE ON backup_jobs
			WHEN NEW.status = 'failed'
			BEGIN
				SELECT RAISE(ABORT, 'failure update unavailable');
			END
		`).Error)
		job := &RunManualBackupJob{
			Deps: f.deps,
			Payload: RunManualBackupPayload{
				ServerID: f.serverID, BackupID: f.backupID, TeamID: f.teamID, JobID: run.ID,
			},
			job: run,
		}
		require.NotPanics(t, func() {
			job.Failed(context.Background(), errors.New("worker stopped"))
		})
	})
}

func newManualBackupExecutionFixture(
	t *testing.T,
) (*manualBackupFixture, *RunManualBackupJob, *taskrunner.FakeDispatcher, *manualBackupBroadcast, *backupmodels.BackupJob) {
	t.Helper()
	f := newManualBackupFixture(t)
	require.NoError(t, f.db.AutoMigrate(&servermodels.Task{}))

	var server servermodels.Server
	require.NoError(t, f.db.First(&server, "id = ?", f.serverID).Error)
	ip := "192.0.2.20"
	server.PublicIPv4 = &ip
	server.PrivateKey = dbtype.EncryptedString("test-private-key")
	require.NoError(t, f.db.Save(&server).Error)

	f.provider.Credentials = dbtype.EncryptedJSONMap{
		"endpoint": "https://objects.example.com",
		"region":   "eu-1",
		"bucket":   "nightly",
		"key":      "access-key",
		"secret":   "secret-key",
	}
	require.NoError(t, f.db.Save(&f.provider).Error)
	require.NoError(t, f.db.Model(&backupmodels.Backup{}).
		Where("id = ?", f.backupID).
		Update("include_files", `["/var/www"]`).Error)

	run := createManualBackupRun(t, f, backuptypes.BackupJobStatusPending)
	recorder := &manualBackupBroadcast{}
	dispatcher := taskrunner.NewFakeDispatcher()
	testLogger := zerolog.Nop()
	f.deps.Broadcaster = recorder
	f.deps.Dispatcher = dispatcher
	f.deps.TaskRunnerDeps = &servertasks.TaskRunnerDeps{
		DB:          f.db,
		Dispatcher:  dispatcher,
		Logger:      &testLogger,
		Broadcaster: recorder,
	}
	job := &RunManualBackupJob{
		Deps: f.deps,
		Payload: RunManualBackupPayload{
			ServerID: f.serverID,
			BackupID: f.backupID,
			TeamID:   f.teamID,
			JobID:    run.ID,
		},
	}
	return f, job, dispatcher, recorder, run
}
