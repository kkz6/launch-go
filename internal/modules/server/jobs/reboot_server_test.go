package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

const oldBoot = "12345678-1234-1234-1234-123456789abc"
const newBoot = "87654321-4321-4321-4321-cba987654321"

func restartFixture(t *testing.T) (*RebootServerJob, *taskrunner.FakeDispatcher, *systemUpdateBroadcaster) {
	t.Helper()
	base, dispatcher, _, broadcaster := newSystemUpdateJob(t)
	require.NoError(t, base.Deps.DB.AutoMigrate(&models.ServerRestart{}))
	now := time.Now().UTC()
	deadline := now.Add(5 * time.Minute)
	require.NoError(t, base.Deps.DB.Create(&models.ServerRestart{
		ServerID: base.Payload.ServerID, OperationID: "operation-a", Status: "queued", RequestedAt: &now, DeadlineAt: &deadline,
	}).Error)
	dispatcher.SetResult("Read restart boot ID", &taskrunner.TaskResult{Output: oldBoot})
	dispatcher.SetResult("Restart server once", &taskrunner.TaskResult{})
	return &RebootServerJob{Deps: base.Deps, Payload: RebootServerPayload{ServerID: base.Payload.ServerID, OperationID: "operation-a"}}, dispatcher, broadcaster
}

func restartState(t *testing.T, job *RebootServerJob) models.ServerRestart {
	t.Helper()
	var state models.ServerRestart
	require.NoError(t, job.Deps.DB.First(&state, "server_id = ?", job.Payload.ServerID).Error)
	return state
}

func dueRestartCheck(t *testing.T, job *RebootServerJob) {
	t.Helper()
	require.NoError(t, job.Deps.DB.Model(&models.ServerRestart{}).Where("server_id = ?", job.Payload.ServerID).
		Update("check_after", time.Now().UTC().Add(-time.Second)).Error)
}

func TestRestartSuccessRequiresReachableNewBootAndNeverResendsCommand(t *testing.T) {
	job, dispatcher, broadcaster := restartFixture(t)
	ctx := context.Background()
	require.NoError(t, job.Handle(ctx))
	require.Equal(t, "verifying", restartState(t, job).Status)
	require.Len(t, dispatcher.Executions, 2)
	require.Contains(t, dispatcher.Executions[1].Script, "launchctl-system-updates-manual.service")
	require.Contains(t, dispatcher.Executions[1].Script, "flock -n 9")
	require.NoError(t, job.Handle(ctx))
	require.Len(t, dispatcher.Executions, 2, "redelivery must not issue another reboot")

	verify := &VerifyServerRestartJob{Deps: job.Deps, Payload: job.Payload}
	for _, output := range []string{oldBoot, "", "not-a-boot-id"} {
		dueRestartCheck(t, job)
		dispatcher.SetResult("Verify server restart", &taskrunner.TaskResult{Output: output})
		require.NoError(t, verify.Handle(ctx))
		require.Equal(t, "verifying", restartState(t, job).Status)
	}
	dueRestartCheck(t, job)
	dispatcher.FailOnRun = errors.New("connection refused")
	require.NoError(t, verify.Handle(ctx))
	require.Equal(t, "verifying", restartState(t, job).Status)
	dispatcher.FailOnRun = nil
	dueRestartCheck(t, job)
	dispatcher.SetResult("Verify server restart", &taskrunner.TaskResult{Output: newBoot})
	require.NoError(t, verify.Handle(ctx))
	state := restartState(t, job)
	require.Equal(t, "succeeded", state.Status)
	require.NotNil(t, state.CompletedAt)
	require.Contains(t, state.Message, "reachable")
	var server models.Server
	require.NoError(t, job.Deps.DB.First(&server, "id = ?", job.Payload.ServerID).Error)
	require.True(t, server.Connected, "verified SSH must restore connected state for the server page")
	require.Contains(t, broadcaster.events, map[string]any{
		"team": "team-a", "event": "server.updated", "data": map[string]any{"server_id": "server-a", "id": "server-a", "team_id": "team-a", "connected": true},
	})
	before := dispatcher.ExecutionCount()
	require.NoError(t, verify.Handle(ctx))
	require.Equal(t, before, dispatcher.ExecutionCount())
	var commands int
	for _, execution := range dispatcher.Executions {
		if execution.Task.Name() == "Restart server once" {
			commands++
		}
	}
	require.Equal(t, 1, commands)
	require.Contains(t, broadcaster.events, map[string]any{
		"team": "team-a", "event": "server.reboot.updated", "data": map[string]any{"server_id": "server-a", "team_id": "team-a"},
	})
}

func TestRestartPreflightFailureDoesNotRestart(t *testing.T) {
	for _, tc := range []struct {
		name, output string
		exit         int
		want         string
	}{
		{"missing identity", "", 0, "identity"},
		{"invalid identity", "garbage", 0, "identity"},
		{"updates running", "", 73, "Security updates"},
		{"unreachable", "", 255, "connection"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			job, dispatcher, _ := restartFixture(t)
			dispatcher.SetResult("Read restart boot ID", &taskrunner.TaskResult{Output: tc.output, ExitCode: tc.exit})
			require.NoError(t, job.Handle(context.Background()))
			state := restartState(t, job)
			require.Equal(t, "failed", state.Status)
			require.Empty(t, state.BootID)
			require.Contains(t, state.Message, tc.want)
			require.Len(t, dispatcher.Executions, 1)
		})
	}
}

func TestRestartCommandTransportFailureKeepsVerifyingButRejectionFails(t *testing.T) {
	for _, tc := range []struct {
		name        string
		result      *taskrunner.TaskResult
		expected    string
		hasBaseline bool
	}{
		{"transport dropped", &taskrunner.TaskResult{ExitCode: -1, Error: errors.New("EOF")}, "verifying", true},
		{"transport timeout", &taskrunner.TaskResult{ExitCode: 124, TimedOut: true}, "verifying", true},
		{"explicit rejection", &taskrunner.TaskResult{ExitCode: 1}, "failed", false},
		{"updates started", &taskrunner.TaskResult{ExitCode: 73}, "failed", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			job, dispatcher, _ := restartFixture(t)
			dispatcher.SetResult("Restart server once", tc.result)
			require.NoError(t, job.Handle(context.Background()))
			state := restartState(t, job)
			require.Equal(t, tc.expected, state.Status)
			require.Equal(t, tc.hasBaseline, state.BootID != "")
			require.NoError(t, job.Handle(context.Background()))
			require.Len(t, dispatcher.Executions, 2)
		})
	}
}

func TestRestartDeadlineAndWatchdogRecoverAbandonedStates(t *testing.T) {
	for _, phase := range []string{"queued", "restarting", "verifying"} {
		t.Run(phase, func(t *testing.T) {
			job, dispatcher, _ := restartFixture(t)
			require.NoError(t, job.Deps.DB.Model(&models.ServerRestart{}).Where("server_id = ?", job.Payload.ServerID).
				Updates(map[string]any{"status": phase, "deadline_at": time.Now().Add(-time.Second), "boot_id": oldBoot}).Error)
			require.NoError(t, (&PollServerRestartsJob{Deps: job.Deps}).Handle(context.Background()))
			state := restartState(t, job)
			require.Equal(t, "failed", state.Status)
			require.NotNil(t, state.CompletedAt)
			require.Contains(t, state.Message, "provider console")
			require.Empty(t, dispatcher.Executions)
		})
	}
}

func TestRestartProbeLeaseAndStaleOperationCannotInterfere(t *testing.T) {
	job, dispatcher, _ := restartFixture(t)
	require.NoError(t, job.Handle(context.Background()))
	before := dispatcher.ExecutionCount()
	verify := &VerifyServerRestartJob{Deps: job.Deps, Payload: job.Payload}
	require.NoError(t, verify.Handle(context.Background()))
	require.Equal(t, before, dispatcher.ExecutionCount(), "future check lease blocks an early duplicate")
	job.Payload.OperationID = "old-operation"
	require.NoError(t, job.Handle(context.Background()))
	verify.Payload = job.Payload
	require.NoError(t, verify.Handle(context.Background()))
	require.Equal(t, before, dispatcher.ExecutionCount())
	require.Equal(t, "verifying", restartState(t, job).Status)
}

func TestRestartRecheckCannotBeFinishedByOldTimeoutOrInFlightProbe(t *testing.T) {
	job, dispatcher, broadcaster := restartFixture(t)
	ctx := context.Background()
	require.NoError(t, job.Handle(ctx))
	previous := restartState(t, job)
	server, _, err := readRestart(ctx, job.Deps, job.Payload)
	require.NoError(t, err)
	// A safe recheck replaces the operation generation after a timeout.
	deadline := time.Now().UTC().Add(5 * time.Minute)
	require.NoError(t, job.Deps.DB.Model(&models.ServerRestart{}).Where("server_id = ?", job.Payload.ServerID).
		Updates(map[string]any{"operation_id": "recheck-b", "status": "verifying", "deadline_at": deadline}).Error)
	eventCount := len(broadcaster.events)
	for _, outcome := range []string{"failed", "succeeded"} {
		require.NoError(t, finishRestart(ctx, job.Deps, server, previous, outcome, "stale result"))
		current := restartState(t, job)
		require.Equal(t, "recheck-b", current.OperationID)
		require.Equal(t, "verifying", current.Status)
		require.Equal(t, oldBoot, current.BootID)
		require.Nil(t, current.CompletedAt)
	}
	require.Len(t, broadcaster.events, eventCount)
	before := dispatcher.ExecutionCount()
	require.NoError(t, (&VerifyServerRestartJob{Deps: job.Deps, Payload: job.Payload}).Handle(ctx))
	require.Equal(t, before, dispatcher.ExecutionCount(), "an old queued probe must not run for the recheck generation")
}
