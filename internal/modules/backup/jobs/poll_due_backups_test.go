package jobs

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	backupmodels "github.com/kkz6/launch-go/internal/modules/backup/models"
	backuprepos "github.com/kkz6/launch-go/internal/modules/backup/repositories"
	backuptypes "github.com/kkz6/launch-go/internal/modules/backup/types"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

type scheduledBackupBroadcast struct {
	broadcast.NopBroadcaster
	event string
	data  map[string]any
}

func (b *scheduledBackupBroadcast) BroadcastToTeam(_ string, event string, data any) {
	b.event = event
	b.data, _ = data.(map[string]any)
}

func newScheduledBackupPollerFixture(t *testing.T) (*gorm.DB, *JobDeps) {
	t.Helper()
	dsn := fmt.Sprintf("file:scheduled-backup-poller-%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&backupmodels.StorageProvider{},
		&backupmodels.Backup{},
		&backupmodels.BackupJob{},
		&backupmodels.BackupDatabase{},
	))
	nopLogger := zerolog.Nop()
	return db, &JobDeps{
		Deps:  &pkgjobs.Deps{DB: db, Logger: &nopLogger},
		Repos: backuprepos.NewRegistry(db),
	}
}

func createScheduledBackup(
	t *testing.T,
	db *gorm.DB,
	id, expression string,
	enabled bool,
) *backupmodels.Backup {
	t.Helper()
	backup := &backupmodels.Backup{
		StorageProviderID: 71,
		CronExpression:    expression,
		IncludeFiles:      "[]",
		ExcludeFiles:      "[]",
		Enabled:           enabled,
		Path:              "servers",
	}
	backup.ID = id
	backup.TeamID = "team-01"
	backup.ServerID = "server-01"
	require.NoError(t, db.Create(backup).Error)
	return backup
}

func TestPollDueBackupsDispatchesOnlyEnabledDueRows(t *testing.T) {
	db, deps := newScheduledBackupPollerFixture(t)
	recorder := &scheduledBackupBroadcast{}
	deps.Broadcaster = recorder
	due := createScheduledBackup(t, db, "backup-due", "0 0 * * *", true)
	createScheduledBackup(t, db, "backup-disabled", "0 0 * * *", false)
	createScheduledBackup(t, db, "backup-later", "0 1 * * *", true)
	createScheduledBackup(t, db, "backup-invalid", "invalid", true)

	var tasks []*asynq.Task
	job := &PollDueBackupsJob{
		Deps: deps,
		now: func() time.Time {
			return time.Date(2026, 8, 11, 0, 0, 31, 0, time.UTC)
		},
		enqueue: func(task *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
			tasks = append(tasks, task)
			return &asynq.TaskInfo{ID: "queued"}, nil
		},
	}

	require.NoError(t, job.Handle(context.Background()))
	require.Len(t, tasks, 1)
	payload, err := pkgjobs.UnmarshalPayload[RunManualBackupPayload](tasks[0])
	require.NoError(t, err)
	require.Equal(t, due.ID, payload.BackupID)
	require.Equal(t, due.TeamID, payload.TeamID)
	require.Equal(t, due.ServerID, payload.ServerID)
	require.Equal(t, "schedule", payload.Source)
	require.NotEmpty(t, payload.JobID)
	require.Nil(t, payload.UserID)

	var runs []backupmodels.BackupJob
	require.NoError(t, db.Find(&runs).Error)
	require.Len(t, runs, 1)
	require.Equal(t, backuptypes.BackupJobStatusPending, runs[0].Status)
	require.Equal(t, payload.JobID, runs[0].ID)
	require.Equal(t, "backup.run.queued", recorder.event)
	require.Equal(t, payload.JobID, recorder.data["job_id"])
	require.Equal(t, "schedule", recorder.data["source"])
}

func TestPollDueBackupsCleansUpDuplicateDispatch(t *testing.T) {
	db, deps := newScheduledBackupPollerFixture(t)
	createScheduledBackup(t, db, "backup-due", "0 0 * * *", true)
	job := &PollDueBackupsJob{
		Deps: deps,
		now:  func() time.Time { return time.Date(2026, 8, 11, 0, 0, 0, 0, time.UTC) },
		enqueue: func(*asynq.Task, ...asynq.Option) (*asynq.TaskInfo, error) {
			return nil, asynq.ErrTaskIDConflict
		},
	}

	require.NoError(t, job.Handle(context.Background()))
	var count int64
	require.NoError(t, db.Model(&backupmodels.BackupJob{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestPollDueBackupsTracksEnqueueFailure(t *testing.T) {
	db, deps := newScheduledBackupPollerFixture(t)
	backup := createScheduledBackup(t, db, "backup-due", "0 0 * * *", true)
	recorder := &scheduledBackupBroadcast{}
	deps.Broadcaster = recorder
	job := &PollDueBackupsJob{
		Deps: deps,
		now:  func() time.Time { return time.Date(2026, 8, 11, 0, 0, 0, 0, time.UTC) },
		enqueue: func(*asynq.Task, ...asynq.Option) (*asynq.TaskInfo, error) {
			return nil, errors.New("redis unavailable")
		},
	}

	require.NoError(t, job.Handle(context.Background()))
	var run backupmodels.BackupJob
	require.NoError(t, db.First(&run).Error)
	require.Equal(t, backuptypes.BackupJobStatusFailed, run.Status)
	require.NotNil(t, run.Error)
	require.Contains(t, *run.Error, "redis unavailable")
	require.Equal(t, "backup.run.failed", recorder.event)
	require.Equal(t, backup.ID, recorder.data["backup_id"])
	require.Equal(t, "schedule", recorder.data["source"])
}

func TestPollDueBackupsReturnsRepositoryFailure(t *testing.T) {
	db, deps := newScheduledBackupPollerFixture(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	job := &PollDueBackupsJob{Deps: deps}
	require.ErrorContains(t, job.Handle(context.Background()), "list enabled backups")
}

func TestScheduledBackupTaskIDUsesBackupAndUTCMinute(t *testing.T) {
	instant := time.Date(2026, 8, 11, 5, 30, 42, 0, time.FixedZone("IST", 5*60*60+30*60))
	require.Equal(
		t,
		"backup-run-scheduled:backup-01:2026-08-11T00:00",
		scheduledBackupTaskID("backup-01", instant),
	)
	require.NotEqual(
		t,
		scheduledBackupTaskID("backup-01", instant),
		scheduledBackupTaskID("backup-02", instant),
	)
}
