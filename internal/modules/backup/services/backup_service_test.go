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
	"github.com/kkz6/launch-go/internal/modules/backup/dto"
	"github.com/kkz6/launch-go/internal/modules/backup/models"
	"github.com/kkz6/launch-go/internal/modules/backup/repositories"
	backuptypes "github.com/kkz6/launch-go/internal/modules/backup/types"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	"github.com/kkz6/launch-go/internal/pkg/queue"
	pkgservice "github.com/kkz6/launch-go/internal/pkg/service"
)

type backupRunBroadcast struct {
	broadcast.NopModelBroadcaster
	teamID string
	event  string
	data   map[string]any
}

func (b *backupRunBroadcast) BroadcastToTeam(teamID, event string, data any) {
	b.teamID = teamID
	b.event = event
	b.data, _ = data.(map[string]any)
}

func newBackupServiceForTest(t *testing.T) (*BackupService, *gorm.DB) {
	t.Helper()
	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.StorageProvider{},
		&models.Backup{},
		&models.BackupJob{},
		&models.BackupDatabase{},
	))

	nopLogger := zerolog.Nop()
	repos := repositories.NewRegistry(db)
	deps := &ServiceDeps{
		ModuleDeps: pkgservice.ModuleDeps[*repositories.Registry]{
			Dependencies: pkgservice.Dependencies{
				DB:     db,
				Logger: &nopLogger,
			},
			Repos: repos,
		},
	}
	return NewBackupService(deps), db
}

func TestValidateS3StorageProviderIsTeamScopedAndDriverChecked(t *testing.T) {
	service, db := newBackupServiceForTest(t)
	ctx := context.Background()

	providers := []models.StorageProvider{
		{ID: 101, TeamID: "team-a", UserID: "user-a", Provider: backuptypes.StorageDriverS3, Credentials: dbtype.EncryptedJSONMap{}},
		{ID: 102, TeamID: "team-b", UserID: "user-b", Provider: backuptypes.StorageDriverS3, Credentials: dbtype.EncryptedJSONMap{}},
		{ID: 103, TeamID: "team-a", UserID: "user-a", Provider: backuptypes.StorageDriverDropbox, Credentials: dbtype.EncryptedJSONMap{}},
	}
	require.NoError(t, db.Create(&providers).Error)
	require.NoError(t, db.Exec(
		"UPDATE storage_providers SET credentials = ? WHERE id = ?",
		"not-valid-encrypted-json",
		101,
	).Error)

	require.NoError(t, service.validateS3StorageProvider(ctx, 101, "team-a"))
	require.Error(t, service.validateS3StorageProvider(ctx, 102, "team-a"))
	require.ErrorIs(t, service.validateS3StorageProvider(ctx, 103, "team-a"), ErrInvalidStorageDriver)
}

func TestCreateBackupRejectsInvalidCronExpression(t *testing.T) {
	service, db := newBackupServiceForTest(t)
	_, err := service.CreateBackup(
		context.Background(),
		"server-a",
		"team-a",
		"user-a",
		&dto.CreateBackupRequest{
			CronExpression:    "0 0 * *",
			Path:              "backups",
			StorageProviderID: "1",
		},
	)
	require.ErrorIs(t, err, ErrInvalidCronExpression)

	var count int64
	require.NoError(t, db.Model(&models.Backup{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestUpdateBackupRejectsInvalidCronExpression(t *testing.T) {
	service, db := newBackupServiceForTest(t)
	backup := &models.Backup{
		StorageProviderID: 1,
		CronExpression:    "0 0 * * *",
		IncludeFiles:      "[]",
		ExcludeFiles:      "[]",
		Path:              "backups",
	}
	backup.ID = "backup-update-cron"
	backup.ServerID = "server-a"
	backup.TeamID = "team-a"
	require.NoError(t, db.Create(backup).Error)

	_, err := service.UpdateBackup(
		context.Background(),
		backup.ID,
		backup.ServerID,
		backup.TeamID,
		"user-a",
		&dto.UpdateBackupRequest{CronExpression: "invalid"},
	)
	require.ErrorIs(t, err, ErrInvalidCronExpression)

	var persisted models.Backup
	require.NoError(t, db.First(&persisted, "id = ?", backup.ID).Error)
	require.Equal(t, "0 0 * * *", persisted.CronExpression)
}

func TestRunBackupReturnsQueueDispatchFailure(t *testing.T) {
	service, db := newBackupServiceForTest(t)
	ctx := context.Background()
	recorder := &backupRunBroadcast{}
	service.SetModelBroadcaster(recorder)

	provider := models.StorageProvider{
		ID:          201,
		TeamID:      "team-a",
		UserID:      "user-a",
		Provider:    backuptypes.StorageDriverS3,
		Credentials: dbtype.EncryptedJSONMap{"bucket": "nightly", "key": "access", "secret": "hidden"},
	}
	require.NoError(t, db.Create(&provider).Error)

	backup := models.Backup{
		StorageProviderID: provider.ID,
		CronExpression:    "0 0 * * *",
		IncludeFiles:      "[]",
		ExcludeFiles:      "[]",
		Path:              "backups",
	}
	backup.ID = "backup-queue-test"
	backup.TeamID = "team-a"
	backup.ServerID = "server-a"
	require.NoError(t, db.Create(&backup).Error)
	require.NoError(t, db.Exec(
		"UPDATE storage_providers SET credentials = ? WHERE id = ?",
		"not-valid-encrypted-json",
		provider.ID,
	).Error)

	_, err := service.RunBackup(ctx, backup.ID, backup.ServerID, backup.TeamID, "user-a")
	require.ErrorIs(t, err, pkgservice.ErrQueueRequired)

	var jobs []models.BackupJob
	require.NoError(t, db.Where("backup_id = ?", backup.ID).Find(&jobs).Error)
	require.Len(t, jobs, 1)
	require.NotEmpty(t, jobs[0].ID)
	require.Equal(t, backup.TeamID, jobs[0].TeamID)
	require.Equal(t, backup.StorageProviderID, jobs[0].StorageProviderID)
	require.Equal(t, backuptypes.BackupJobStatusFailed, jobs[0].Status)
	require.NotNil(t, jobs[0].Error)
	require.ErrorContains(t, err, "queue manual backup")
	require.Contains(t, *jobs[0].Error, "Queue not configured")

	require.Equal(t, backup.TeamID, recorder.teamID)
	require.Equal(t, "backup.run.failed", recorder.event)
	require.Equal(t, backup.ID, recorder.data["backup_id"])
	require.Equal(t, backup.ServerID, recorder.data["server_id"])
	require.Equal(t, jobs[0].ID, recorder.data["job_id"])
	require.Equal(t, *jobs[0].Error, recorder.data["error"])
}

func TestRunBackupDoesNotBroadcastUnpersistedQueueFailure(t *testing.T) {
	service, db := newBackupServiceForTest(t)
	ctx := context.Background()
	recorder := &backupRunBroadcast{}
	service.SetModelBroadcaster(recorder)

	provider := models.StorageProvider{
		ID:          202,
		TeamID:      "team-a",
		UserID:      "user-a",
		Provider:    backuptypes.StorageDriverS3,
		Credentials: dbtype.EncryptedJSONMap{"bucket": "nightly", "key": "access", "secret": "hidden"},
	}
	require.NoError(t, db.Create(&provider).Error)

	backup := models.Backup{
		StorageProviderID: provider.ID,
		CronExpression:    "0 0 * * *",
		IncludeFiles:      "[]",
		ExcludeFiles:      "[]",
		Path:              "backups",
	}
	backup.ID = "backup-queue-persistence-test"
	backup.TeamID = "team-a"
	backup.ServerID = "server-a"
	require.NoError(t, db.Create(&backup).Error)
	require.NoError(t, db.Exec(`
		CREATE TRIGGER reject_backup_job_failure_update
		BEFORE UPDATE ON backup_jobs
		BEGIN
			SELECT RAISE(FAIL, 'backup job update rejected');
		END
	`).Error)

	_, err := service.RunBackup(ctx, backup.ID, backup.ServerID, backup.TeamID, "user-a")
	require.ErrorIs(t, err, pkgservice.ErrQueueRequired)

	var job models.BackupJob
	require.NoError(t, db.Where("backup_id = ?", backup.ID).First(&job).Error)
	require.Equal(t, backuptypes.BackupJobStatusPending, job.Status)
	require.Empty(t, recorder.event, "an unpersisted terminal state must not be broadcast")

	require.NoError(t, db.Exec("DROP TRIGGER reject_backup_job_failure_update").Error)
	service.recordManualBackupFailure(ctx, &job, &backup, backup.ServerID, "other-team", "out of scope")
	require.Empty(t, recorder.event)
}

func TestRunBackupRejectsUnscopedBackupAndProvider(t *testing.T) {
	service, db := newBackupServiceForTest(t)
	ctx := context.Background()
	recorder := &backupRunBroadcast{}
	service.SetModelBroadcaster(recorder)
	provider := models.StorageProvider{
		ID:          301,
		TeamID:      "team-a",
		UserID:      "user-a",
		Provider:    backuptypes.StorageDriverS3,
		Credentials: dbtype.EncryptedJSONMap{},
	}
	require.NoError(t, db.Create(&provider).Error)
	backup := models.Backup{
		StorageProviderID: provider.ID,
		CronExpression:    "0 0 * * *",
		IncludeFiles:      "[]",
		ExcludeFiles:      "[]",
		Path:              "backups",
	}
	backup.ID = "scoped-backup"
	backup.TeamID = "team-a"
	backup.ServerID = "server-a"
	require.NoError(t, db.Create(&backup).Error)

	_, err := service.RunBackup(ctx, backup.ID, "other-server", backup.TeamID, "user-a")
	require.Error(t, err)
	_, err = service.RunBackup(ctx, backup.ID, backup.ServerID, "other-team", "user-a")
	require.Error(t, err)

	require.NoError(t, db.Model(&provider).Update("team_id", "other-team").Error)
	_, err = service.RunBackup(ctx, backup.ID, backup.ServerID, backup.TeamID, "user-a")
	require.Error(t, err)
	require.ErrorContains(t, err, "validate storage provider")

	var job models.BackupJob
	require.NoError(t, db.Where("backup_id = ?", backup.ID).First(&job).Error)
	require.Equal(t, backuptypes.BackupJobStatusFailed, job.Status)
	require.NotNil(t, job.Error)
	require.Contains(t, *job.Error, "storage provider validation failed")
	require.Equal(t, backup.TeamID, recorder.teamID)
	require.Equal(t, "backup.run.failed", recorder.event)
	require.Equal(t, job.ID, recorder.data["job_id"])
	require.Equal(t, *job.Error, recorder.data["error"])
}

func TestRunBackupReturnsRunCreationFailure(t *testing.T) {
	service, db := newBackupServiceForTest(t)
	provider := models.StorageProvider{
		ID:          302,
		TeamID:      "team-a",
		UserID:      "user-a",
		Provider:    backuptypes.StorageDriverS3,
		Credentials: dbtype.EncryptedJSONMap{},
	}
	require.NoError(t, db.Create(&provider).Error)
	backup := models.Backup{
		StorageProviderID: provider.ID,
		CronExpression:    "0 0 * * *",
		IncludeFiles:      "[]",
		ExcludeFiles:      "[]",
		Path:              "backups",
	}
	backup.ID = "backup-create-run-failure"
	backup.TeamID = "team-a"
	backup.ServerID = "server-a"
	require.NoError(t, db.Create(&backup).Error)
	require.NoError(t, db.Exec(`
		CREATE TRIGGER reject_manual_backup_run
		BEFORE INSERT ON backup_jobs
		BEGIN
			SELECT RAISE(ABORT, 'run insert unavailable');
		END
	`).Error)

	_, err := service.RunBackup(
		context.Background(), backup.ID, backup.ServerID, backup.TeamID, "user-a",
	)
	require.ErrorContains(t, err, "create manual backup run")
}

func TestBackupCreateAndUpdateRejectForeignStorageProvider(t *testing.T) {
	service, db := newBackupServiceForTest(t)
	provider := models.StorageProvider{
		ID:          303,
		TeamID:      "other-team",
		UserID:      "other-user",
		Provider:    backuptypes.StorageDriverS3,
		Credentials: dbtype.EncryptedJSONMap{},
	}
	require.NoError(t, db.Create(&provider).Error)

	createRequest := &dto.CreateBackupRequest{
		CronExpression:    "0 0 * * *",
		Path:              "backups",
		StorageProviderID: "303",
	}
	_, err := service.CreateBackup(
		context.Background(), "server-a", "team-a", "user-a", createRequest,
	)
	require.Error(t, err)

	backup := models.Backup{
		StorageProviderID: provider.ID,
		CronExpression:    "0 0 * * *",
		IncludeFiles:      "[]",
		ExcludeFiles:      "[]",
		Path:              "backups",
	}
	backup.ID = "backup-update-foreign-provider"
	backup.TeamID = "team-a"
	backup.ServerID = "server-a"
	require.NoError(t, db.Create(&backup).Error)
	updateRequest := &dto.UpdateBackupRequest{
		CronExpression:    "0 1 * * *",
		Path:              "updated",
		StorageProviderID: "303",
	}
	_, err = service.UpdateBackup(
		context.Background(), backup.ID, backup.ServerID, backup.TeamID, "user-a", updateRequest,
	)
	require.Error(t, err)
}

func TestRunBackupBuildsTaskBeforeQueueConnectionFailure(t *testing.T) {
	_, db := newBackupServiceForTest(t)
	provider := models.StorageProvider{
		ID:          304,
		TeamID:      "team-a",
		UserID:      "user-a",
		Provider:    backuptypes.StorageDriverS3,
		Credentials: dbtype.EncryptedJSONMap{},
	}
	require.NoError(t, db.Create(&provider).Error)
	backup := models.Backup{
		StorageProviderID: provider.ID,
		CronExpression:    "0 0 * * *",
		IncludeFiles:      "[]",
		ExcludeFiles:      "[]",
		Path:              "backups",
	}
	backup.ID = "backup-queue-connection-error"
	backup.TeamID = "team-a"
	backup.ServerID = "server-a"
	require.NoError(t, db.Create(&backup).Error)

	queueClient := queue.NewClient(config.RedisConfig{Address: "127.0.0.1:1"})
	t.Cleanup(func() { require.NoError(t, queueClient.Close()) })
	testLogger := zerolog.Nop()
	service := NewBackupService(&ServiceDeps{
		ModuleDeps: pkgservice.ModuleDeps[*repositories.Registry]{
			Dependencies: pkgservice.Dependencies{DB: db, Logger: &testLogger, Queue: queueClient},
			Repos:        repositories.NewRegistry(db),
		},
	})

	_, err := service.RunBackup(
		context.Background(), backup.ID, backup.ServerID, backup.TeamID, "user-a",
	)
	require.ErrorContains(t, err, "queue manual backup")
}

func TestRunBackupBroadcastsQueuedRunAfterDispatch(t *testing.T) {
	service, db := newBackupServiceForTest(t)
	recorder := &backupRunBroadcast{}
	service.SetModelBroadcaster(recorder)
	provider := models.StorageProvider{
		ID: 305, TeamID: "team-a", UserID: "user-a", Provider: backuptypes.StorageDriverS3,
		Credentials: dbtype.EncryptedJSONMap{},
	}
	require.NoError(t, db.Create(&provider).Error)
	backup := models.Backup{
		StorageProviderID: provider.ID, CronExpression: "0 0 * * *",
		IncludeFiles: "[]", ExcludeFiles: "[]", Path: "backups",
	}
	backup.ID = "backup-queued"
	backup.TeamID = "team-a"
	backup.ServerID = "server-a"
	require.NoError(t, db.Create(&backup).Error)
	service.dispatchManualBackup = func(serverID, backupID, teamID, jobID string, userID *string) error {
		require.Equal(t, backup.ServerID, serverID)
		require.Equal(t, backup.ID, backupID)
		require.Equal(t, backup.TeamID, teamID)
		require.NotEmpty(t, jobID)
		require.Equal(t, "user-a", *userID)
		return nil
	}

	run, err := service.RunBackup(
		context.Background(), backup.ID, backup.ServerID, backup.TeamID, "user-a",
	)
	require.NoError(t, err)
	require.Equal(t, string(backuptypes.BackupJobStatusPending), run.Status)
	require.Equal(t, backup.TeamID, recorder.teamID)
	require.Equal(t, "backup.run.queued", recorder.event)
	require.Equal(t, run.ID, recorder.data["job_id"])
	require.Equal(t, "pending", recorder.data["status"])
}
