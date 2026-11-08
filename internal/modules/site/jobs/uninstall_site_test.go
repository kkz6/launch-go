package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	"github.com/kkz6/launch-go/internal/pkg/queue"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

type uninstallEvents struct {
	broadcast.NopBroadcaster
	events []string
}

func (e *uninstallEvents) BroadcastToTeam(_, event string, _ any) { e.events = append(e.events, event) }

func uninstallSiteFixture(t *testing.T) (*UninstallSiteJob, *taskrunner.FakeDispatcher, *gorm.DB, *uninstallEvents) {
	t.Helper()
	ensureSiteTaskTemplates(t)
	deps, dispatcher, db := restartQueueJobFixture(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&models.Deployment{}, &models.Certificate{}, &models.Command{}, &models.Redirect{}))
	requested := time.Now().UTC().Truncate(time.Microsecond)
	require.NoError(t, db.Model(&models.Site{}).Where("id = ?", "site-1").Update("uninstallation_requested_at", requested).Error)
	createRestartQueue(t, db, "queue-1", true, false)
	events := &uninstallEvents{}
	deps.Broadcaster = events
	actor := "staff-admin"
	job := &UninstallSiteJob{Deps: deps, Payload: UninstallSitePayload{SiteID: "site-1", ServerID: "server-1", TeamID: "team-1", UserID: &actor, RequestedAt: &requested}}
	return job, dispatcher, db, events
}

func TestUninstallSitePreservesRecordsOnEveryRemoteFailure(t *testing.T) {
	for _, step := range []string{"connection", "Delete Queue Config", "Update Caddy Site Imports", "Delete Site Files", "timeout"} {
		t.Run(step, func(t *testing.T) {
			job, dispatcher, db, events := uninstallSiteFixture(t)
			switch step {
			case "connection":
				dispatcher.SetRunError(errors.New("SSH unavailable"))
			case "timeout":
				dispatcher.SetResult("Delete Site Files", &taskrunner.TaskResult{TimedOut: true})
			default:
				dispatcher.SetFailure(step, 1, "cleanup failed")
			}
			err := job.Handle(context.Background())
			require.Error(t, err)
			require.NoError(t, db.First(&models.Queue{}, "id = ?", "queue-1").Error)
			job.Failed(context.Background(), err)
			var site models.Site
			require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
			require.Nil(t, site.UninstallationRequestedAt)
			require.NotNil(t, site.UninstallationFailedAt)
			require.NotContains(t, events.events, "site.deleted")
			require.Contains(t, events.events, "site.updated")
		})
	}
}

func TestUninstallSiteRejectsWrongScopeAndStaleRequestsBeforeRemoteWork(t *testing.T) {
	for _, tc := range []string{"wrong server", "wrong team", "server team mismatch", "stale request", "no pending request"} {
		t.Run(tc, func(t *testing.T) {
			job, dispatcher, db, events := uninstallSiteFixture(t)
			switch tc {
			case "wrong server":
				job.Payload.ServerID = "other-server"
			case "wrong team":
				job.Payload.TeamID = "other-team"
			case "server team mismatch":
				require.NoError(t, db.Exec("UPDATE servers SET team_id = 'other-team'").Error)
			case "stale request":
				old := job.Payload.RequestedAt.Add(-time.Hour)
				job.Payload.RequestedAt = &old
			case "no pending request":
				require.NoError(t, db.Exec("UPDATE sites SET uninstallation_requested_at = NULL").Error)
			}
			err := job.Handle(context.Background())
			require.Error(t, err)
			job.Failed(context.Background(), err)
			require.Zero(t, dispatcher.ExecutionCount())
			var site models.Site
			require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
			require.Nil(t, site.UninstallationFailedAt)
			require.Empty(t, events.events)
		})
	}
}

func TestUninstallSiteCompletionRemovesOnlyRequestedSiteAfterRemoteSuccess(t *testing.T) {
	job, dispatcher, db, events := uninstallSiteFixture(t)
	var other models.Site
	require.NoError(t, db.First(&other, "id = ?", "site-1").Error)
	other.ID = "other-site"
	other.Address = "other.example"
	other.Path = "/home/launcher/other.example"
	other.UninstallationRequestedAt = nil
	now := time.Now().UTC()
	other.InstalledAt = &now
	require.NoError(t, db.Create(&other).Error)
	require.NoError(t, job.Handle(context.Background()))
	require.Equal(t, 3, dispatcher.ExecutionCount())
	imports := dispatcher.GetExecution(1).Script
	require.Contains(t, imports, "/home/launcher/other.example/Caddyfile")
	require.NotContains(t, imports, "/home/launcher/example.test/Caddyfile")
	require.NoError(t, db.First(&models.Site{}, "id = ?", other.ID).Error)
	require.ErrorIs(t, db.First(&models.Site{}, "id = ?", "site-1").Error, gorm.ErrRecordNotFound)
	require.ErrorIs(t, db.First(&models.Queue{}, "id = ?", "queue-1").Error, gorm.ErrRecordNotFound)
	require.Equal(t, []string{"site.deleted"}, events.events)
	// Redelivery of a completed request must not touch a different site.
	require.NoError(t, job.Handle(context.Background()))
	require.Equal(t, 3, dispatcher.ExecutionCount())
}

func TestUninstallSiteDatabaseCleanupIsAtomic(t *testing.T) {
	job, _, db, events := uninstallSiteFixture(t)
	require.NoError(t, db.Callback().Delete().Before("gorm:delete").Register("reject_site_deletion", func(tx *gorm.DB) {
		if tx.Statement.Table == "sites" {
			tx.AddError(errors.New("site delete failed"))
		}
	}))
	err := job.Handle(context.Background())
	require.ErrorContains(t, err, "site delete failed")
	job.Failed(context.Background(), err)
	require.NoError(t, db.First(&models.Site{}, "id = ?", "site-1").Error)
	require.NoError(t, db.First(&models.Queue{}, "id = ?", "queue-1").Error)
	require.NotContains(t, events.events, "site.deleted")
}

func TestUninstallSiteStaleFailureDoesNotClearNewRequest(t *testing.T) {
	job, dispatcher, db, events := uninstallSiteFixture(t)
	dispatcher.SetRunError(errors.New("SSH unavailable"))
	err := job.Handle(context.Background())
	require.Error(t, err)
	next := job.Payload.RequestedAt.Add(time.Minute)
	require.NoError(t, db.Model(&models.Site{}).Where("id = ?", "site-1").Update("uninstallation_requested_at", next).Error)
	job.Failed(context.Background(), err)
	var site models.Site
	require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
	require.NotNil(t, site.UninstallationRequestedAt)
	require.True(t, site.UninstallationRequestedAt.Equal(next))
	require.Nil(t, site.UninstallationFailedAt)
	require.Empty(t, events.events)
}

func TestUninstallSiteCanceledJobStillRestoresRetryableState(t *testing.T) {
	job, dispatcher, db, events := uninstallSiteFixture(t)
	dispatcher.SetRunError(context.Canceled)
	err := job.Handle(context.Background())
	require.Error(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	job.Failed(ctx, err)
	var site models.Site
	require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
	require.Nil(t, site.UninstallationRequestedAt)
	require.NotNil(t, site.UninstallationFailedAt)
	require.NoError(t, db.First(&models.Queue{}, "id = ?", "queue-1").Error)
	require.Equal(t, []string{"site.updated"}, events.events)
}

type replacingUninstallDispatcher struct {
	*taskrunner.FakeDispatcher
	replace func() error
}

func (d *replacingUninstallDispatcher) Run(ctx context.Context, task *taskrunner.PendingTask) (*taskrunner.TaskResult, error) {
	if task.Task.Name() == "Delete Site Files" {
		if err := d.replace(); err != nil {
			return nil, err
		}
	}
	return d.FakeDispatcher.Run(ctx, task)
}

func TestUninstallSiteStaleCompletionPreservesNewRequestAndRelatedRecords(t *testing.T) {
	job, dispatcher, db, events := uninstallSiteFixture(t)
	next := job.Payload.RequestedAt.Add(time.Minute)
	job.Deps.TaskRunnerDeps.Dispatcher = &replacingUninstallDispatcher{
		FakeDispatcher: dispatcher,
		replace: func() error {
			return db.Model(&models.Site{}).Where("id = ?", "site-1").Update("uninstallation_requested_at", next).Error
		},
	}
	err := job.Handle(context.Background())
	require.ErrorContains(t, err, "site deletion request changed during cleanup")
	job.Failed(context.Background(), err)
	var site models.Site
	require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
	require.NotNil(t, site.UninstallationRequestedAt)
	require.True(t, site.UninstallationRequestedAt.Equal(next))
	require.Nil(t, site.UninstallationFailedAt)
	require.NoError(t, db.First(&models.Queue{}, "id = ?", "queue-1").Error)
	require.Empty(t, events.events)
}

func TestUninstallSiteAcceptsLegacyPayloadWithStoredOwnership(t *testing.T) {
	job, _, db, _ := uninstallSiteFixture(t)
	job.Payload.TeamID = ""
	job.Payload.RequestedAt = nil
	require.NoError(t, job.Handle(context.Background()))
	require.ErrorIs(t, db.First(&models.Site{}, "id = ?", "site-1").Error, gorm.ErrRecordNotFound)
}

func TestUninstallSiteTaskPreservesActorAndUsesNewRetryID(t *testing.T) {
	actor := "staff-admin"
	now := time.Now().UTC()
	a, err := NewUninstallSiteTask("site", "server", &actor, UninstallSiteScope{TeamID: "team", RequestedAt: &now})
	require.NoError(t, err)
	b, err := NewUninstallSiteTask("site", "server", &actor, UninstallSiteScope{TeamID: "team", RequestedAt: &now})
	require.NoError(t, err)
	redis := miniredis.RunT(t)
	client := queue.NewClient(config.RedisConfig{Address: redis.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	first, err := client.EnqueueDefault(a)
	require.NoError(t, err)
	second, err := client.EnqueueDefault(b)
	require.NoError(t, err)
	require.NotEqual(t, first.ID, second.ID)
	var payload UninstallSitePayload
	require.NoError(t, json.Unmarshal(a.Payload(), &payload))
	require.Equal(t, "team", payload.TeamID)
	require.Equal(t, actor, *payload.UserID)
	require.True(t, now.Equal(*payload.RequestedAt))
}
