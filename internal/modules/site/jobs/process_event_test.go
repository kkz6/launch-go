package jobs

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	siteevents "github.com/kkz6/launch-go/internal/modules/site/events"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	siterepos "github.com/kkz6/launch-go/internal/modules/site/repositories"
	"github.com/kkz6/launch-go/internal/pkg/app"
	pkgevents "github.com/kkz6/launch-go/internal/pkg/events"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

type eventSiteReaderStub struct {
	site *models.Site
	err  error
}

func (s eventSiteReaderStub) FindByID(_ context.Context, _ string) (*models.Site, error) {
	return s.site, s.err
}

type eventTaskQueueStub struct {
	tasks []*asynq.Task
	err   error
}

func (q *eventTaskQueueStub) Enqueue(
	task *asynq.Task,
	_ ...asynq.Option,
) (*asynq.TaskInfo, error) {
	if q.err != nil {
		return nil, q.err
	}
	q.tasks = append(q.tasks, task)
	return &asynq.TaskInfo{}, nil
}

type eventListenerStub struct {
	name  string
	calls int
	err   error
}

func (l *eventListenerStub) Name() string {
	return l.name
}

func (l *eventListenerStub) Handle(_ context.Context, _ pkgevents.Event) error {
	l.calls++
	return l.err
}

func TestRestartQueuesListenerEnqueuesRetryableSiteAction(t *testing.T) {
	taskQueue := &eventTaskQueueStub{}
	listener := &restartQueuesOnDeploymentSucceeded{
		sites: eventSiteReaderStub{site: &models.Site{
			AutoRestartQueue: true,
		}},
		queue: taskQueue,
	}
	event := deploymentSucceededEvent(t, "deployment-1")

	require.NoError(t, listener.Handle(context.Background(), event))
	require.Len(t, taskQueue.tasks, 1)
	require.Equal(t, TypeRestartAllSiteQueues, taskQueue.tasks[0].Type())

	payload, err := pkgjobs.UnmarshalPayload[RestartAllSiteQueuesPayload](taskQueue.tasks[0])
	require.NoError(t, err)
	require.Equal(t, "site-1", payload.SiteID)
}

func TestRestartQueuesListenerReadsCurrentAutoRestartSetting(t *testing.T) {
	taskQueue := &eventTaskQueueStub{}
	listener := &restartQueuesOnDeploymentSucceeded{
		sites: eventSiteReaderStub{site: &models.Site{
			AutoRestartQueue: false,
		}},
		queue: taskQueue,
	}

	require.NoError(t, listener.Handle(context.Background(), deploymentSucceededEvent(t, "deployment-1")))
	require.Empty(t, taskQueue.tasks)
}

func TestRestartQueuesListenerSurfacesFanOutFailure(t *testing.T) {
	listener := &restartQueuesOnDeploymentSucceeded{
		sites: eventSiteReaderStub{site: &models.Site{
			AutoRestartQueue: true,
		}},
		queue: &eventTaskQueueStub{err: errors.New("redis unavailable")},
	}

	err := listener.Handle(context.Background(), deploymentSucceededEvent(t, "deployment-1"))
	require.ErrorContains(t, err, "redis unavailable")
}

func TestRestartQueuesListenerTreatsExistingActionAsDelivered(t *testing.T) {
	for _, queueErr := range []error{asynq.ErrTaskIDConflict, asynq.ErrDuplicateTask} {
		listener := &restartQueuesOnDeploymentSucceeded{
			sites: eventSiteReaderStub{site: &models.Site{
				AutoRestartQueue: true,
			}},
			queue: &eventTaskQueueStub{err: queueErr},
		}

		require.NoError(t, listener.Handle(context.Background(), deploymentSucceededEvent(t, "deployment-1")))
	}
}

func TestRestartQueuesListenerValidatesAndLoadsSite(t *testing.T) {
	listener := &restartQueuesOnDeploymentSucceeded{
		sites: eventSiteReaderStub{err: errors.New("database unavailable")},
	}

	err := listener.Handle(context.Background(), pkgevents.Event{
		Name:    siteevents.DeploymentSucceededName,
		Payload: []byte(`{"site_id":`),
	})
	require.ErrorContains(t, err, "decode event")

	err = listener.Handle(context.Background(), deploymentSucceededEvent(t, "deployment-1"))
	require.ErrorContains(t, err, "find site: database unavailable")
}

func TestRestartQueuesListenerRequiresQueue(t *testing.T) {
	listener := &restartQueuesOnDeploymentSucceeded{
		sites: eventSiteReaderStub{site: &models.Site{AutoRestartQueue: true}},
	}

	err := listener.Handle(context.Background(), deploymentSucceededEvent(t, "deployment-1"))
	require.ErrorContains(t, err, "queue client is not configured")
}

func TestRestartQueuesListenerLogsSuccessfulFanOut(t *testing.T) {
	var output bytes.Buffer
	logger := zerolog.New(&output)
	listener := &restartQueuesOnDeploymentSucceeded{
		sites:  eventSiteReaderStub{site: &models.Site{AutoRestartQueue: true}},
		queue:  &eventTaskQueueStub{},
		logger: &logger,
	}

	require.NoError(t, listener.Handle(context.Background(), deploymentSucceededEvent(t, "deployment-1")))
	require.Contains(t, output.String(), "queued post-deployment queue restart")
}

func TestProcessSiteEventJobRequiresDispatcher(t *testing.T) {
	job := &ProcessSiteEventJob{}

	err := job.Handle(context.Background())

	require.ErrorContains(t, err, "site event dispatcher is not configured")
}

func TestProcessSiteEventJobDispatchesAndReportsListenerError(t *testing.T) {
	dispatcher := pkgevents.NewDispatcher()
	listener := &eventListenerStub{name: "listener", err: errors.New("listener failed")}
	dispatcher.MustSubscribe(siteevents.DeploymentSucceededName, listener)
	job := &ProcessSiteEventJob{
		Dispatcher: dispatcher,
		Event:      deploymentSucceededEvent(t, "deployment-1"),
	}

	err := job.Handle(context.Background())

	require.ErrorContains(t, err, "listener failed")
	require.Equal(t, 1, listener.calls)
}

func TestProcessSiteEventJobFailedLogsEventContext(t *testing.T) {
	var output bytes.Buffer
	logger := zerolog.New(&output)
	job := &ProcessSiteEventJob{
		Deps:  &JobDeps{Deps: &pkgjobs.Deps{Logger: &logger}},
		Event: deploymentSucceededEvent(t, "deployment-1"),
	}

	job.Failed(context.Background(), errors.New("dispatch failed"))

	require.Contains(t, output.String(), "site event dispatch failed")
	require.Contains(t, output.String(), "deployment-1")
	require.Contains(t, output.String(), siteevents.DeploymentSucceededName)
}

func TestNewProcessSiteEventJobUsesConfiguredDependencies(t *testing.T) {
	originalDeps := deps
	originalDispatcher := siteEventDispatcher
	t.Cleanup(func() {
		deps = originalDeps
		siteEventDispatcher = originalDispatcher
	})

	configuredDeps := &JobDeps{}
	configuredDispatcher := pkgevents.NewDispatcher()
	deps = configuredDeps
	siteEventDispatcher = configuredDispatcher
	event := deploymentSucceededEvent(t, "deployment-1")

	handler := NewProcessSiteEventJob(event)
	job, ok := handler.(*ProcessSiteEventJob)

	require.True(t, ok)
	require.Same(t, configuredDeps, job.Deps)
	require.Same(t, configuredDispatcher, job.Dispatcher)
	require.Equal(t, event, job.Event)
}

func TestRegisterWiresSiteEventWorker(t *testing.T) {
	originalDeps := deps
	originalDispatcher := siteEventDispatcher
	t.Cleanup(func() {
		deps = originalDeps
		siteEventDispatcher = originalDispatcher
	})

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Site{}))

	site := &models.Site{AutoRestartQueue: false}
	site.ID = "site-1"
	site.ServerID = "server-1"
	site.TeamID = "team-1"
	site.UserID = "user-1"
	require.NoError(t, db.Create(site).Error)

	logger := zerolog.Nop()
	mux := asynq.NewServeMux()
	Register(
		mux,
		app.Deps{DB: db, Logger: &logger},
		siterepos.NewRegistry(db),
		serverrepos.NewRegistry(db),
		nil,
		nil,
	)

	event := deploymentSucceededEvent(t, "deployment-1")
	task, err := pkgjobs.Task(siteevents.TypeProcessEvent, event)
	require.NoError(t, err)
	require.NoError(t, mux.ProcessTask(context.Background(), task))
	require.NotNil(t, siteEventDispatcher)
}

func deploymentSucceededEvent(t *testing.T, deploymentID string) pkgevents.Event {
	t.Helper()
	event, err := siteevents.NewDeploymentSucceeded(siteevents.DeploymentSucceeded{
		DeploymentID: deploymentID,
		SiteID:       "site-1",
		ServerID:     "server-1",
		TeamID:       "team-1",
		TaskID:       "task-1",
	})
	require.NoError(t, err)
	return event
}
