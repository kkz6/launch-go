package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/site/tasks"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

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
