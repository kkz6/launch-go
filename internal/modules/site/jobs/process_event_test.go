package jobs

import (
	"context"
	"errors"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"

	siteevents "github.com/kkz6/launch-go/internal/modules/site/events"
	"github.com/kkz6/launch-go/internal/modules/site/models"
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
	listener := &restartQueuesOnDeploymentSucceeded{
		sites: eventSiteReaderStub{site: &models.Site{
			AutoRestartQueue: true,
		}},
		queue: &eventTaskQueueStub{err: asynq.ErrTaskIDConflict},
	}

	require.NoError(t, listener.Handle(context.Background(), deploymentSucceededEvent(t, "deployment-1")))
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
