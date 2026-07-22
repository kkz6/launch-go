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
	assert.True(t, isScheduledSyncTimeout(nil, context.DeadlineExceeded))
	assert.True(t, isScheduledSyncTimeout(nil, fmt.Errorf("status: %w", errDaemonStatusTimeout)))
	assert.False(t, isScheduledSyncTimeout(&userID, context.DeadlineExceeded), "manual sync failures must remain visible")
	assert.False(t, isScheduledSyncTimeout(nil, errors.New("permission denied")))
}
