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
	gormlogger "gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/database/serializers"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	sitemodels "github.com/kkz6/launch-go/internal/modules/site/models"
	siterepos "github.com/kkz6/launch-go/internal/modules/site/repositories"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

func TestRestartAllSiteQueuesFiltersUnavailableQueues(t *testing.T) {
	jobDeps, dispatcher, db := restartQueueJobFixture(t)
	createRestartQueue(t, db, "installed", true, false)
	createRestartQueue(t, db, "pending", false, false)
	createRestartQueue(t, db, "failed", true, true)
	job := &RestartAllSiteQueuesJob{
		Deps:    jobDeps,
		Payload: RestartAllSiteQueuesPayload{SiteID: "site-1"},
	}

	err := job.Handle(context.Background())

	require.NoError(t, err)
	require.Equal(t, 1, dispatcher.ExecutionCount())
	script := dispatcher.LastExecution().Script
	require.Contains(t, script, `"installed":*`)
	require.NotContains(t, script, `"pending":*`)
	require.NotContains(t, script, `"failed":*`)
}

func TestRestartAllSiteQueuesSkipsWhenNoQueueIsInstalled(t *testing.T) {
	jobDeps, dispatcher, db := restartQueueJobFixture(t)
	createRestartQueue(t, db, "pending", false, false)
	createRestartQueue(t, db, "failed", true, true)
	job := &RestartAllSiteQueuesJob{
		Deps:    jobDeps,
		Payload: RestartAllSiteQueuesPayload{SiteID: "site-1"},
	}

	err := job.Handle(context.Background())

	require.NoError(t, err)
	require.Zero(t, dispatcher.ExecutionCount())
}

func TestRestartAllSiteQueuesReturnsNonZeroExitCode(t *testing.T) {
	jobDeps, dispatcher, db := restartQueueJobFixture(t)
	createRestartQueue(t, db, "installed", true, false)
	dispatcher.SetDefaultFailure(1, "supervisor failed")
	job := &RestartAllSiteQueuesJob{
		Deps:    jobDeps,
		Payload: RestartAllSiteQueuesPayload{SiteID: "site-1"},
	}

	err := job.Handle(context.Background())

	require.EqualError(t, err, "queue restart failed with exit code 1")
}

func TestRestartAllSiteQueuesReturnsDispatcherError(t *testing.T) {
	jobDeps, dispatcher, db := restartQueueJobFixture(t)
	createRestartQueue(t, db, "installed", true, false)
	expectedErr := errors.New("ssh unavailable")
	dispatcher.SetRunError(expectedErr)
	job := &RestartAllSiteQueuesJob{
		Deps:    jobDeps,
		Payload: RestartAllSiteQueuesPayload{SiteID: "site-1"},
	}

	err := job.Handle(context.Background())

	require.ErrorIs(t, err, expectedErr)
}

func TestRestartQueueReturnsDispatcherError(t *testing.T) {
	jobDeps, dispatcher, db := restartQueueJobFixture(t)
	createRestartQueue(t, db, "queue-1", true, false)
	expectedErr := errors.New("ssh unavailable")
	dispatcher.SetRunError(expectedErr)
	job := &RestartQueueJob{
		Deps: jobDeps,
		Payload: RestartQueuePayload{
			SiteID:  "site-1",
			QueueID: "queue-1",
		},
	}

	err := job.Handle(context.Background())

	require.ErrorIs(t, err, expectedErr)
}

func TestRestartQueueTaskConstructors(t *testing.T) {
	userID := "user-1"

	single, err := NewRestartQueueTask("site-1", "queue-1", &userID)
	require.NoError(t, err)
	require.Equal(t, TypeRestartQueue, single.Type())
	singlePayload, err := pkgjobs.UnmarshalPayload[RestartQueuePayload](single)
	require.NoError(t, err)
	require.Equal(t, "site-1", singlePayload.SiteID)
	require.Equal(t, "queue-1", singlePayload.QueueID)
	require.Equal(t, &userID, singlePayload.UserID)

	all, err := NewRestartAllSiteQueuesTask("site-1", &userID)
	require.NoError(t, err)
	require.Equal(t, TypeRestartAllSiteQueues, all.Type())
	allPayload, err := pkgjobs.UnmarshalPayload[RestartAllSiteQueuesPayload](all)
	require.NoError(t, err)
	require.Equal(t, "site-1", allPayload.SiteID)
	require.Equal(t, &userID, allPayload.UserID)
}

func restartQueueJobFixture(
	t *testing.T,
) (*JobDeps, *taskrunner.FakeDispatcher, *gorm.DB) {
	t.Helper()
	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))

	db, err := gorm.Open(
		sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"),
		&gorm.Config{
			DisableForeignKeyConstraintWhenMigrating: true,
			Logger:                                   gormlogger.Default.LogMode(gormlogger.Silent),
		},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&servermodels.Server{},
		&servermodels.InstalledService{},
		&servermodels.Task{},
		&sitemodels.Site{},
		&sitemodels.Queue{},
	))

	ip := "192.0.2.10"
	server := &servermodels.Server{
		BaseModel:  basemodels.BaseModel{ID: "server-1"},
		Name:       "Test server",
		PublicIPv4: &ip,
		PrivateKey: dbtype.EncryptedString(
			"-----BEGIN OPENSSH PRIVATE KEY-----\ntest\n-----END OPENSSH PRIVATE KEY-----",
		),
	}
	server.TeamID = "team-1"
	server.UserID = "user-1"
	require.NoError(t, db.Create(server).Error)

	site := &sitemodels.Site{
		BaseModel: basemodels.BaseModel{ID: "site-1"},
		Address:   "example.test",
		User:      "launcher",
		Path:      "/home/launcher/example.test",
		WebFolder: "public",
	}
	site.ServerID = server.ID
	site.TeamID = server.TeamID
	site.UserID = "user-1"
	require.NoError(t, db.Create(site).Error)

	dispatcher := taskrunner.NewFakeDispatcher()
	logger := zerolog.Nop()
	jobDeps := &JobDeps{
		Deps: &pkgjobs.Deps{
			DB:     db,
			Logger: &logger,
		},
		Repos:       siterepos.NewRegistry(db),
		ServerRepos: serverrepos.NewRegistry(db),
		TaskRunnerDeps: &servertasks.TaskRunnerDeps{
			DB:         db,
			Dispatcher: dispatcher,
			Logger:     &logger,
		},
	}
	return jobDeps, dispatcher, db
}

func createRestartQueue(
	t *testing.T,
	db *gorm.DB,
	id string,
	installed bool,
	failed bool,
) {
	t.Helper()
	queue := &sitemodels.Queue{
		BaseModel:       basemodels.BaseModel{ID: id},
		Command:         "php artisan queue:work",
		User:            "launcher",
		QueueName:       "default",
		StopSignal:      "TERM",
		QueueConnection: "redis",
	}
	queue.SiteID = "site-1"
	queue.ServerID = "server-1"
	queue.TeamID = "team-1"
	queue.UserID = "user-1"
	if installed {
		now := time.Now().UTC()
		queue.InstalledAt = &now
	}
	if failed {
		now := time.Now().UTC()
		queue.InstallationFailedAt = &now
	}
	require.NoError(t, db.Create(queue).Error)
}
