package jobs

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/database/serializers"
	"github.com/kkz6/launch-go/internal/modules/database/models"
	"github.com/kkz6/launch-go/internal/modules/database/repositories"
	databasetemplates "github.com/kkz6/launch-go/internal/modules/database/tasks/templates"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	tasktemplates "github.com/kkz6/launch-go/internal/pkg/taskrunner/templates"
)

func TestInstallationRequiresSuccessfulRemoteTasks(t *testing.T) {
	for _, tc := range []struct {
		name, table, event, failedTask string
		job                            func(*JobDeps) pkgjobs.FailableHandler
	}{
		{"database", "databases", "database.progress", "Create MySQL Database", func(d *JobDeps) pkgjobs.FailableHandler {
			return &InstallDatabaseJob{Deps: d, Payload: InstallDatabasePayload{DatabaseID: "database-1"}}
		}},
		{"user creation", "database_users", "database_user.progress", "Create MySQL User", func(d *JobDeps) pkgjobs.FailableHandler {
			return &InstallDatabaseUserJob{Deps: d, Payload: InstallDatabaseUserPayload{DatabaseUserID: "user-1", Password: "test-password"}}
		}},
		{"user privileges", "database_users", "database_user.progress", "Grant MySQL Privileges", func(d *JobDeps) pkgjobs.FailableHandler {
			return &InstallDatabaseUserJob{Deps: d, Payload: InstallDatabaseUserPayload{DatabaseUserID: "user-1", Password: "test-password"}}
		}},
	} {
		for _, succeeds := range []bool{false, true} {
			outcome := "failure"
			if succeeds {
				outcome = "success"
			}
			t.Run(tc.name+"/"+outcome, func(t *testing.T) {
				deps, fake, db, events := executionFixture(t)
				if !succeeds {
					fake.SetFailure(tc.failedTask, 1, "database command rejected")
					if tc.name == "user privileges" {
						extra := &models.Database{BaseModel: basemodels.BaseModel{ID: "database-2"}, Name: "second_database"}
						extra.ServerID, extra.TeamID = "server-1", "team-1"
						require.NoError(t, db.Create(extra).Error)
						require.NoError(t, db.Create(&models.DatabaseDatabaseUser{DatabaseID: extra.ID, DatabaseUserID: "user-1"}).Error)
					}
				}
				job := tc.job(deps)
				err := job.Handle(context.Background())
				if succeeds {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
					job.Failed(context.Background(), err)
				}
				var saved basemodels.InstallableModel
				require.NoError(t, db.Table(tc.table).Take(&saved).Error)
				if succeeds {
					require.NotNil(t, saved.InstalledAt)
					require.Nil(t, saved.InstallationFailedAt)
					require.Contains(t, events.statuses[tc.event], "installed")
				} else {
					require.Nil(t, saved.InstalledAt)
					require.NotNil(t, saved.InstallationFailedAt)
					require.NotContains(t, events.statuses[tc.event], "installed")
					require.Contains(t, events.statuses[tc.event], "failed")
				}
				if tc.name == "user privileges" {
					require.Equal(t, 2, fake.ExecutionCount(), "creation must precede the privilege grant")
					require.Equal(t, "Create MySQL User", fake.GetExecution(0).Task.Name())
					require.Equal(t, "Grant MySQL Privileges", fake.GetExecution(1).Task.Name())
				}
			})
		}
	}
}

func TestUninstallRetainsRecordsUntilRemoteSuccess(t *testing.T) {
	for _, tc := range []struct {
		name, table, event string
		job                func(*JobDeps) pkgjobs.FailableHandler
	}{
		{"database", "databases", "database.progress", func(d *JobDeps) pkgjobs.FailableHandler {
			return &UninstallDatabaseJob{Deps: d, Payload: UninstallDatabasePayload{DatabaseID: "database-1"}}
		}},
		{"user", "database_users", "database_user.progress", func(d *JobDeps) pkgjobs.FailableHandler {
			return &UninstallDatabaseUserJob{Deps: d, Payload: UninstallDatabaseUserPayload{DatabaseUserID: "user-1"}}
		}},
	} {
		for _, outcome := range []string{"success", "command failure", "transport failure", "missing result", "record deletion failure"} {
			t.Run(tc.name+"/"+outcome, func(t *testing.T) {
				deps, fake, db, events := executionFixture(t)
				installed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
				require.NoError(t, db.Table(tc.table).Where("1 = 1").Updates(map[string]any{
					"installed_at": installed, "uninstallation_requested_at": time.Now(),
				}).Error)
				transportErr := errors.New("connection lost")
				switch outcome {
				case "command failure":
					fake.SetDefaultFailure(1, "drop rejected")
				case "transport failure":
					fake.SetRunError(transportErr)
				case "missing result":
					deps.TaskRunnerDeps.Dispatcher = emptyResultDispatcher{}
				case "record deletion failure":
					require.NoError(t, db.Callback().Delete().Before("gorm:delete").Register("reject_record_deletion", func(tx *gorm.DB) {
						if tx.Statement.Table == tc.table {
							tx.AddError(errors.New("record deletion rejected"))
						}
					}))
				}
				job := tc.job(deps)
				err := job.Handle(context.Background())
				var activities int64
				require.NoError(t, db.Model(&activity.ActivityLog{}).Where("event = ?", "uninstalled").Count(&activities).Error)
				if outcome == "success" {
					require.NoError(t, err)
					require.EqualValues(t, 1, activities)
					var count int64
					require.NoError(t, db.Table(tc.table).Count(&count).Error)
					require.Zero(t, count)
					require.Contains(t, events.statuses[tc.event], "deleted")
					return
				}
				require.Error(t, err)
				require.Zero(t, activities, "failed deletion must not record a successful uninstall")
				if outcome == "transport failure" {
					require.ErrorIs(t, err, transportErr)
				}
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				job.Failed(ctx, err)
				var saved basemodels.InstallableModel
				require.NoError(t, db.Table(tc.table).Take(&saved).Error)
				require.NotNil(t, saved.InstalledAt)
				require.True(t, installed.Equal(*saved.InstalledAt))
				require.Nil(t, saved.UninstallationRequestedAt)
				require.NotNil(t, saved.UninstallationFailedAt)
				require.NotContains(t, events.statuses[tc.event], "deleted")
				require.Contains(t, events.statuses[tc.event], "failed")
			})
		}
	}
}

func TestPasswordUpdateReportsRemoteResult(t *testing.T) {
	for _, succeeds := range []bool{false, true} {
		name := "failure"
		if succeeds {
			name = "success"
		}
		t.Run(name, func(t *testing.T) {
			deps, fake, db, events := executionFixture(t)
			if !succeeds {
				fake.SetDefaultFailure(1, "password update rejected")
			}
			password := "replacement-password"
			job := &UpdateDatabaseUserJob{Deps: deps, Payload: UpdateDatabaseUserPayload{DatabaseUserID: "user-1", Password: &password}}
			err := job.Handle(context.Background())
			if succeeds {
				require.NoError(t, err)
				require.Contains(t, events.statuses["database_user.progress"], "updated")
			} else {
				require.Error(t, err)
				job.Failed(context.Background(), err)
				require.NotContains(t, events.statuses["database_user.progress"], "updated")
				require.Contains(t, events.statuses["database_user.progress"], "failed")
			}
			require.NoError(t, db.First(&models.DatabaseUser{}, "id = ?", "user-1").Error)
			require.Equal(t, 1, fake.ExecutionCount())
			require.Equal(t, "Update MySQL Password", fake.LastExecution().Task.Name())
		})
	}
}

type executionEvents struct {
	broadcast.NopBroadcaster
	statuses map[string][]string
}

type emptyResultDispatcher struct{}

func (emptyResultDispatcher) Run(context.Context, *taskrunner.PendingTask) (*taskrunner.TaskResult, error) {
	return nil, nil
}

func (e *executionEvents) BroadcastToTeam(_ string, event string, data any) {
	if payload, ok := data.(map[string]any); ok {
		if status, ok := payload["status"].(string); ok {
			e.statuses[event] = append(e.statuses[event], status)
		}
	}
}

var executionTemplatesOnce sync.Once
var executionTemplatesErr error

func executionFixture(t *testing.T) (*JobDeps, *taskrunner.FakeDispatcher, *gorm.DB, *executionEvents) {
	t.Helper()
	executionTemplatesOnce.Do(func() {
		executionTemplatesErr = tasktemplates.Register("database", databasetemplates.FS, nil)
	})
	require.NoError(t, executionTemplatesErr)
	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "jobs.sqlite")), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true, Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&servermodels.Server{}, &servermodels.InstalledService{}, &servermodels.Task{},
		&models.Database{}, &models.DatabaseUser{}, &models.DatabaseDatabaseUser{}, &activity.ActivityLog{}))
	activity.SetDB(db)
	t.Cleanup(func() { activity.SetDB(nil) })
	ip := "192.0.2.10"
	server := &servermodels.Server{BaseModel: basemodels.BaseModel{ID: "server-1"}, PublicIPv4: &ip,
		PrivateKey: dbtype.EncryptedString("fixture-private-key"), DatabasePassword: dbtype.EncryptedString("admin-password")}
	server.TeamID = "team-1"
	require.NoError(t, db.Create(server).Error)
	database := &models.Database{BaseModel: basemodels.BaseModel{ID: "database-1"}, Name: "app_database"}
	database.ServerID, database.TeamID = server.ID, server.TeamID
	user := &models.DatabaseUser{BaseModel: basemodels.BaseModel{ID: "user-1"}, Name: "app_user"}
	user.ServerID, user.TeamID = server.ID, server.TeamID
	require.NoError(t, db.Create(database).Error)
	require.NoError(t, db.Create(user).Error)
	require.NoError(t, db.Create(&models.DatabaseDatabaseUser{DatabaseID: database.ID, DatabaseUserID: user.ID}).Error)
	fake := taskrunner.NewFakeDispatcher()
	events := &executionEvents{statuses: make(map[string][]string)}
	log := zerolog.Nop()
	return NewJobDepsWithParams(db, repositories.NewRegistry(db), &log, events, fake, nil), fake, db, events
}
