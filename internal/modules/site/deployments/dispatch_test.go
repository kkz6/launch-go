package deployments_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/modules/site/deployments"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
)

type dispatchEvent struct {
	teamID, name string
	data         any
}

type dispatchEvents struct {
	broadcast.NopBroadcaster
	events []dispatchEvent
	onSend func()
}

func (b *dispatchEvents) BroadcastToTeam(teamID, event string, data any) {
	if b.onSend != nil {
		b.onSend()
	}
	b.events = append(b.events, dispatchEvent{teamID, event, data})
}

func dispatchFixture(t *testing.T) (*gorm.DB, *models.Site, *dispatchEvents) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&models.Site{}, &models.Deployment{}))
	site := &models.Site{Address: "example.test", Type: sitetypes.SiteTypeLaravel, TLSSetting: sitetypes.TLSSettingAuto}
	site.ID, site.TeamID, site.ServerID, site.UserID = "site-1", "team-1", "server-1", "user-1"
	require.NoError(t, db.Create(site).Error)
	return db, site, &dispatchEvents{}
}

func seedDeployment(t *testing.T, db *gorm.DB, site *models.Site, id string, status sitetypes.DeploymentStatus, createdAt time.Time) *models.Deployment {
	t.Helper()
	deployment := &models.Deployment{Status: status}
	deployment.ID, deployment.SiteID, deployment.TeamID, deployment.CreatedAt = id, site.ID, site.TeamID, &createdAt
	require.NoError(t, db.Create(deployment).Error)
	return deployment
}

func requireDeploymentStatus(t *testing.T, db *gorm.DB, id string, status sitetypes.DeploymentStatus) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var deployment models.Deployment
	require.NoError(t, db.WithContext(ctx).First(&deployment, "id = ?", id).Error)
	require.Equal(t, status, deployment.Status)
}

func requireFailureEvents(t *testing.T, events *dispatchEvents, deployment *models.Deployment) {
	t.Helper()
	require.Len(t, events.events, 2)
	for i, name := range []string{"deployment.failed", "deployment.progress"} {
		event := events.events[i]
		require.Equal(t, name, event.name)
		require.Equal(t, deployment.TeamID, event.teamID)
		data, ok := event.data.(map[string]any)
		require.True(t, ok)
		require.Equal(t, deployment.ID, data["deployment_id"])
		require.Equal(t, deployment.SiteID, data["site_id"])
		require.Equal(t, "failed", data["status"])
	}
}

func TestDispatchFailuresReleasePendingDeploymentBeforeBroadcast(t *testing.T) {
	for _, failure := range []string{"queue", "factory", "missing queue", "cancelled request"} {
		t.Run(failure, func(t *testing.T) {
			db, site, events := dispatchFixture(t)
			deployment := seedDeployment(t, db, site, "attempt", sitetypes.DeploymentStatusPending, time.Now())
			events.onSend = func() { requireDeploymentStatus(t, db, deployment.ID, sitetypes.DeploymentStatusFailed) }
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cause := errors.New("dispatch failed")
			calls := 0
			dispatcher := deployments.Dispatcher{DB: db, Broadcaster: events, Enqueue: func(*asynq.Task, ...asynq.Option) (*asynq.TaskInfo, error) {
				calls++
				if failure == "cancelled request" {
					cancel()
				}
				return nil, cause
			}}
			if failure == "missing queue" {
				dispatcher.Enqueue = nil
				cause = deployments.ErrQueueUnavailable
			}
			err := dispatcher.Dispatch(ctx, deployment, func() (*asynq.Task, error) {
				if failure == "factory" {
					return nil, cause
				}
				return asynq.NewTask("test:deploy", nil), nil
			})
			require.ErrorIs(t, err, cause)
			requireDeploymentStatus(t, db, deployment.ID, sitetypes.DeploymentStatusFailed)
			require.Equal(t, sitetypes.DeploymentStatusFailed, deployment.Status)
			requireFailureEvents(t, events, deployment)
			if failure == "factory" || failure == "missing queue" {
				require.Zero(t, calls)
			} else {
				require.Equal(t, 1, calls)
			}
		})
	}
}

func TestDispatchPreservesQueueAndRepairErrorsWithoutFalseEvents(t *testing.T) {
	db, site, events := dispatchFixture(t)
	deployment := seedDeployment(t, db, site, "attempt", sitetypes.DeploymentStatusPending, time.Now())
	queueErr, repairErr := errors.New("queue unavailable"), errors.New("database unavailable")
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("test:repair_error", func(tx *gorm.DB) { tx.AddError(repairErr) }))
	dispatcher := deployments.Dispatcher{DB: db, Broadcaster: events, Enqueue: func(*asynq.Task, ...asynq.Option) (*asynq.TaskInfo, error) {
		return nil, queueErr
	}}
	err := dispatcher.Dispatch(context.Background(), deployment, func() (*asynq.Task, error) { return asynq.NewTask("test:deploy", nil), nil })
	require.ErrorIs(t, err, queueErr)
	require.ErrorIs(t, err, repairErr)
	requireDeploymentStatus(t, db, deployment.ID, sitetypes.DeploymentStatusPending)
	require.Empty(t, events.events)
}

func TestDispatchFailureDoesNotOverwriteWorkerProgress(t *testing.T) {
	for _, status := range []sitetypes.DeploymentStatus{sitetypes.DeploymentStatusInstalling, sitetypes.DeploymentStatusFinished} {
		t.Run(string(status), func(t *testing.T) {
			db, site, events := dispatchFixture(t)
			deployment := seedDeployment(t, db, site, "attempt", sitetypes.DeploymentStatusPending, time.Now())
			cause := errors.New("queue reply lost")
			dispatcher := deployments.Dispatcher{DB: db, Broadcaster: events, Enqueue: func(*asynq.Task, ...asynq.Option) (*asynq.TaskInfo, error) {
				// Simulate a worker accepting the task before the producer sees an error.
				require.NoError(t, db.Model(&models.Deployment{}).Where("id = ?", deployment.ID).Update("status", status).Error)
				return nil, cause
			}}
			err := dispatcher.Dispatch(context.Background(), deployment, func() (*asynq.Task, error) { return asynq.NewTask("test:deploy", nil), nil })
			require.ErrorIs(t, err, cause)
			requireDeploymentStatus(t, db, deployment.ID, status)
			require.Empty(t, events.events)
		})
	}
}

func TestDispatchSuccessPreservesOptionsWithoutStartingDeployment(t *testing.T) {
	db, site, events := dispatchFixture(t)
	deployment := seedDeployment(t, db, site, "attempt", sitetypes.DeploymentStatusPending, time.Now())
	task := asynq.NewTask("test:deploy", nil)
	options := []asynq.Option{asynq.ProcessIn(time.Second), asynq.Queue("critical")}
	dispatcher := deployments.Dispatcher{DB: db, Broadcaster: events, Enqueue: func(got *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
		require.Same(t, task, got)
		require.Equal(t, options, opts)
		return &asynq.TaskInfo{}, nil
	}}
	require.NoError(t, dispatcher.Dispatch(context.Background(), deployment, func() (*asynq.Task, error) { return task, nil }, options...))
	requireDeploymentStatus(t, db, deployment.ID, sitetypes.DeploymentStatusPending)
	require.Empty(t, events.events)
}

func TestAdvanceNextFailureStopsAfterOneCandidate(t *testing.T) {
	db, site, events := dispatchFixture(t)
	now := time.Now()
	seedDeployment(t, db, site, "predecessor", sitetypes.DeploymentStatusFinished, now.Add(-3*time.Hour))
	first := seedDeployment(t, db, site, "first", sitetypes.DeploymentStatusQueued, now.Add(-2*time.Hour))
	seedDeployment(t, db, site, "second", sitetypes.DeploymentStatusQueued, now.Add(-time.Hour))
	cause := errors.New("queue unavailable")
	calls := 0
	dispatcher := deployments.Dispatcher{DB: db, Broadcaster: events, Enqueue: func(*asynq.Task, ...asynq.Option) (*asynq.TaskInfo, error) {
		calls++
		requireDeploymentStatus(t, db, first.ID, sitetypes.DeploymentStatusPending)
		return nil, cause
	}}
	next, err := dispatcher.AdvanceNext(context.Background(), site.ID)
	require.Nil(t, next)
	require.ErrorIs(t, err, cause)
	require.Equal(t, 1, calls)
	requireDeploymentStatus(t, db, first.ID, sitetypes.DeploymentStatusFailed)
	requireDeploymentStatus(t, db, "second", sitetypes.DeploymentStatusQueued)
	requireDeploymentStatus(t, db, "predecessor", sitetypes.DeploymentStatusFinished)
	requireFailureEvents(t, events, first)
}

func TestAdvanceNextDoesNotClaimWhileDeploymentIsActive(t *testing.T) {
	for _, status := range []sitetypes.DeploymentStatus{sitetypes.DeploymentStatusPending, sitetypes.DeploymentStatusInstalling} {
		t.Run(string(status), func(t *testing.T) {
			db, site, events := dispatchFixture(t)
			seedDeployment(t, db, site, "active", status, time.Now().Add(-time.Hour))
			seedDeployment(t, db, site, "waiting", sitetypes.DeploymentStatusQueued, time.Now())
			dispatcher := deployments.Dispatcher{DB: db, Broadcaster: events, Enqueue: func(*asynq.Task, ...asynq.Option) (*asynq.TaskInfo, error) {
				t.Error("active deployment must prevent queue advancement")
				return nil, nil
			}}
			next, err := dispatcher.AdvanceNext(context.Background(), site.ID)
			require.NoError(t, err)
			require.Nil(t, next)
			requireDeploymentStatus(t, db, "waiting", sitetypes.DeploymentStatusQueued)
			require.Empty(t, events.events)
		})
	}
}

func TestAdvanceNextCommitsOneClaimBeforeEnqueue(t *testing.T) {
	db, site, events := dispatchFixture(t)
	require.NoError(t, db.Model(site).Update("zero_downtime_deployment", true).Error)
	seedDeployment(t, db, site, "later", sitetypes.DeploymentStatusQueued, time.Now())
	seedDeployment(t, db, site, "oldest", sitetypes.DeploymentStatusQueued, time.Now().Add(-time.Hour))
	dispatcher := deployments.Dispatcher{DB: db, Broadcaster: events}
	calls := 0
	dispatcher.Enqueue = func(task *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
		calls++
		require.Equal(t, 1, calls, "another advancement must observe the pending claim")
		require.Equal(t, "site:deploy_zero_downtime", task.Type())
		var payload map[string]any
		require.NoError(t, json.Unmarshal(task.Payload(), &payload))
		require.Equal(t, "oldest", payload["deployment_id"])
		require.Equal(t, site.ID, payload["site_id"])
		require.Equal(t, true, payload["zero_downtime"])
		// With one database connection, these reads also prove the transaction
		// released its connection before invoking the external queue.
		requireDeploymentStatus(t, db, "oldest", sitetypes.DeploymentStatusPending)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		next, err := dispatcher.AdvanceNext(ctx, site.ID)
		require.NoError(t, err)
		require.Nil(t, next)
		return &asynq.TaskInfo{}, nil
	}
	next, err := dispatcher.AdvanceNext(context.Background(), site.ID)
	require.NoError(t, err)
	require.NotNil(t, next)
	require.Equal(t, "oldest", next.ID)
	require.Equal(t, sitetypes.DeploymentStatusPending, next.Status)
	require.Equal(t, 1, calls)
	requireDeploymentStatus(t, db, "later", sitetypes.DeploymentStatusQueued)
	require.Empty(t, events.events)
}
