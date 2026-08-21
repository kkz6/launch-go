package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	notificationmodels "github.com/kkz6/launch-go/internal/modules/notification/models"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	sitemodels "github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

type syncQueueNotifier struct {
	teamID       string
	notification taskrunner.Notification
	err          error
}

func (n *syncQueueNotifier) SendToTeam(_ context.Context, teamID string, notification taskrunner.Notification) error {
	n.teamID = teamID
	n.notification = notification
	return n.err
}

func (n *syncQueueNotifier) SendToChannel(context.Context, string, taskrunner.Notification) error {
	return nil
}

func TestValidateDaemonStatusResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		result  *servertasks.TaskRunnerResult
		wantErr string
	}{
		{name: "success", result: &servertasks.TaskRunnerResult{TaskResult: &taskrunner.TaskResult{}}},
		{name: "missing result", wantErr: "returned no result"},
		{name: "timeout", result: &servertasks.TaskRunnerResult{TaskResult: &taskrunner.TaskResult{TimedOut: true}}, wantErr: "timed out after 1m0s"},
		{name: "transport failure", result: &servertasks.TaskRunnerResult{Error: context.DeadlineExceeded}, wantErr: "context deadline exceeded"},
		{name: "command failure", result: &servertasks.TaskRunnerResult{TaskResult: &taskrunner.TaskResult{ExitCode: 1}}, wantErr: "exit code 1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDaemonStatusResult(tt.result, time.Minute)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestNewSyncQueuesTask(t *testing.T) {
	t.Parallel()

	task, err := NewSyncQueuesTask("site-1", "server-1", nil)
	require.NoError(t, err)
	assert.Equal(t, TypeSyncQueues, task.Type())
	assert.Greater(t, syncQueuesJobTimeout, tasks.CheckDaemonStatus().Timeout())

	var payload SyncQueuesPayload
	require.NoError(t, json.Unmarshal(task.Payload(), &payload))
	assert.Equal(t, "site-1", payload.SiteID)
	assert.Equal(t, "server-1", payload.ServerID)
	assert.Equal(t, SyncQueuesTriggerScheduled, payload.Trigger)
}

func TestValidateDaemonStatusResultPreservesTransportError(t *testing.T) {
	t.Parallel()

	transportErr := errors.New("ssh transport failed")
	err := validateDaemonStatusResult(&servertasks.TaskRunnerResult{Error: transportErr}, time.Minute)
	require.ErrorIs(t, err, transportErr)
}

func TestIsScheduledSyncTimeout(t *testing.T) {
	t.Parallel()

	userID := "user-1"
	assert.True(t, isScheduledSyncTimeout(SyncQueuesTriggerScheduled, nil, context.DeadlineExceeded))
	assert.True(t, isScheduledSyncTimeout(SyncQueuesTriggerScheduled, nil, fmt.Errorf("status: %w", errDaemonStatusTimeout)))
	assert.False(t, isScheduledSyncTimeout(SyncQueuesTriggerManual, &userID, context.DeadlineExceeded), "manual sync failures must remain visible")
	assert.False(t, isScheduledSyncTimeout(SyncQueuesTriggerPostRestart, nil, context.DeadlineExceeded), "post-restart verification failures must remain visible")
	assert.False(t, isScheduledSyncTimeout(SyncQueuesTriggerScheduled, nil, errors.New("permission denied")))
}

func TestNotifyStoppedQueuesRunsForEverySyncTrigger(t *testing.T) {
	t.Parallel()

	for _, trigger := range []SyncQueuesTrigger{SyncQueuesTriggerManual, SyncQueuesTriggerScheduled, SyncQueuesTriggerPostRestart} {
		t.Run(string(trigger), func(t *testing.T) {
			notifier := &syncQueueNotifier{}
			logger := zerolog.Nop()
			job := &SyncQueuesJob{
				Deps: &JobDeps{
					Deps:        &pkgjobs.Deps{Logger: &logger},
					FrontendURL: "https://launch.test/",
					TaskRunnerDeps: &servertasks.TaskRunnerDeps{
						Notifier: notifier,
					},
				},
				Payload: SyncQueuesPayload{Trigger: trigger},
				site: &sitemodels.Site{
					BaseModel:    basemodels.BaseModel{ID: "site-1"},
					ServerScoped: basemodels.ServerScoped{ServerID: "server-1"},
					Address:      "example.com",
				},
				server: &servermodels.Server{
					BaseModel:  basemodels.BaseModel{ID: "server-1"},
					TeamScoped: basemodels.TeamScoped{TeamID: "team-1"},
					Name:       "Production",
				},
				failedQueueIDs: []string{"queue-1"},
			}

			job.notifyStoppedQueues(context.Background(), []sitemodels.Queue{
				{BaseModel: basemodels.BaseModel{ID: "queue-1"}, QueueName: "default"},
				{BaseModel: basemodels.BaseModel{ID: "queue-2"}, QueueName: "emails"},
			})

			assert.Equal(t, "team-1", notifier.teamID)
			require.NotNil(t, notifier.notification)
			fullNotification, ok := notifier.notification.(notificationmodels.Notification)
			require.True(t, ok)
			assert.Contains(t, fullNotification.ToEmail().Body, "default")
			assert.NotContains(t, fullNotification.ToEmail().Body, "emails")
			assert.Contains(t, fullNotification.ToEmail().Body, "?tab=queues")
		})
	}
}

func TestQueueManagementURL(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "https://launch.test/servers/server-1/sites/site-1?tab=queues", queueManagementURL("https://launch.test/", "server-1", "site-1"))
	assert.Empty(t, queueManagementURL("", "server-1", "site-1"))
}

func TestNotifyStoppedQueuesSkipsUnavailableNotifierAndUnknownQueues(t *testing.T) {
	t.Parallel()

	job := &SyncQueuesJob{Deps: &JobDeps{}}
	assert.NotPanics(t, func() { job.notifyStoppedQueues(context.Background(), nil) })

	notifier := &syncQueueNotifier{}
	logger := zerolog.Nop()
	job = &SyncQueuesJob{
		Deps: &JobDeps{
			Deps:           &pkgjobs.Deps{Logger: &logger},
			TaskRunnerDeps: &servertasks.TaskRunnerDeps{Notifier: notifier},
		},
		site:           &sitemodels.Site{},
		server:         &servermodels.Server{},
		failedQueueIDs: []string{"missing"},
	}
	job.notifyStoppedQueues(context.Background(), []sitemodels.Queue{{BaseModel: basemodels.BaseModel{ID: "queue-1"}}})
	assert.Nil(t, notifier.notification)
}

func TestNotifyStoppedQueuesDoesNotFailSyncWhenNotificationDeliveryFails(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	job := &SyncQueuesJob{
		Deps: &JobDeps{
			Deps:           &pkgjobs.Deps{Logger: &logger},
			TaskRunnerDeps: &servertasks.TaskRunnerDeps{Notifier: &syncQueueNotifier{err: errors.New("email unavailable")}},
		},
		site:           &sitemodels.Site{BaseModel: basemodels.BaseModel{ID: "site-1"}},
		server:         &servermodels.Server{TeamScoped: basemodels.TeamScoped{TeamID: "team-1"}},
		failedQueueIDs: []string{"queue-1"},
	}

	assert.NotPanics(t, func() {
		job.notifyStoppedQueues(context.Background(), []sitemodels.Queue{{BaseModel: basemodels.BaseModel{ID: "queue-1"}, QueueName: "default"}})
	})
}

func TestScheduledSyncIgnoresDispatcherDeadline(t *testing.T) {
	jobDeps, dispatcher, db := restartQueueJobFixture(t)
	createRestartQueue(t, db, "queue-1", true, false)
	dispatcher.SetRunError(context.DeadlineExceeded)
	job := &SyncQueuesJob{Deps: jobDeps, Payload: SyncQueuesPayload{
		SiteID: "site-1", ServerID: "server-1", Trigger: SyncQueuesTriggerScheduled,
	}}

	require.NoError(t, job.Handle(context.Background()))
}

func TestSyncQueuesReturnsQueueUpdateErrors(t *testing.T) {
	jobDeps, _, db := restartQueueJobFixture(t)
	createRestartQueue(t, db, "queue-1", true, false)
	require.NoError(t, db.Exec(`
		CREATE TRIGGER reject_queue_update BEFORE UPDATE ON queues
		BEGIN SELECT RAISE(FAIL, 'update rejected'); END;
	`).Error)
	job := &SyncQueuesJob{Deps: jobDeps, Payload: SyncQueuesPayload{
		SiteID: "site-1", ServerID: "server-1", Trigger: SyncQueuesTriggerManual,
	}}

	err := job.Handle(context.Background())
	require.ErrorContains(t, err, "update queue statuses")
	require.ErrorContains(t, err, "update rejected")
}
