package jobs

import (
	"context"
	"errors"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	"github.com/kkz6/launch-go/internal/pkg/queue"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

type runtimeCompletionEvents struct {
	uninstallEvents
	completed map[string]any
}

func (e *runtimeCompletionEvents) BroadcastToTeam(teamID, event string, data any) {
	e.uninstallEvents.BroadcastToTeam(teamID, event, data)
	if event == "deployment.runtime.completed" {
		e.completed = data.(map[string]any)
	}
}

func TestRefreshRuntimeReportsWorkerQueueFailureSeparately(t *testing.T) {
	deps, dispatcher, db := deploymentReleaseJobFixture(t)
	deps.Queue = nil
	require.NoError(t, db.Model(&models.Site{}).Where("id = ?", "site-1").Update("auto_restart_queue", true).Error)
	events := &runtimeCompletionEvents{}
	deps.Broadcaster = events
	job := &RefreshRuntimeJob{Deps: deps, Payload: RefreshRuntimePayload{SiteID: "site-1", ServerID: "server-1", TeamID: "team-1"}}
	require.NoError(t, job.Handle(context.Background()))
	require.Equal(t, 1, dispatcher.ExecutionCount())
	require.Contains(t, events.events, "deployment.runtime.completed")
	require.NotContains(t, events.events, "deployment.runtime.failed")
	require.Equal(t, true, events.completed["post_actions_pending"])
	require.NotEmpty(t, events.completed["task_id"])
}

func TestRefreshRuntimeFailureCannotPublishSuccess(t *testing.T) {
	for _, failure := range []string{"transport", "result", "exit", "timeout"} {
		t.Run(failure, func(t *testing.T) {
			deps, dispatcher, db := deploymentReleaseJobFixture(t)
			require.NoError(t, db.Model(&models.Site{}).Where("id = ?", "site-1").Updates(map[string]any{"type": sitetypes.SiteTypeGeneric, "php_version": sitetypes.PhpVersion83}).Error)
			events := &uninstallEvents{}
			deps.Broadcaster = events
			switch failure {
			case "transport":
				dispatcher.SetRunError(errors.New("connection lost"))
			case "result":
				dispatcher.SetResult("Refresh PHP Runtime", &taskrunner.TaskResult{Error: errors.New("stream lost")})
			case "exit":
				dispatcher.SetFailure("Refresh PHP Runtime", 1, "reload denied")
			case "timeout":
				dispatcher.SetResult("Refresh PHP Runtime", &taskrunner.TaskResult{TimedOut: true})
			}
			job := &RefreshRuntimeJob{Deps: deps, Payload: RefreshRuntimePayload{SiteID: "site-1", ServerID: "server-1", TeamID: "team-1"}}
			err := job.Handle(context.Background())
			require.Error(t, err)
			job.Failed(context.Background(), err)
			require.Contains(t, events.events, "deployment.runtime.failed")
			require.NotContains(t, events.events, "deployment.runtime.completed")
			var tracked servermodels.Task
			require.NoError(t, db.First(&tracked, "id = ?", job.taskID).Error)
			require.Equal(t, "site-1", *tracked.SiteID)
		})
	}
}

func TestRefreshRuntimeHonorsWorkerPolicyWithoutSwitchingCode(t *testing.T) {
	for _, restart := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[restart], func(t *testing.T) {
			deps, dispatcher, db := deploymentReleaseJobFixture(t)
			require.NoError(t, db.Model(&models.Site{}).Where("id = ?", "site-1").Updates(map[string]any{"type": sitetypes.SiteTypeGeneric, "php_version": sitetypes.PhpVersion83, "auto_restart_queue": restart}).Error)
			events := &uninstallEvents{}
			deps.Broadcaster = events
			job := &RefreshRuntimeJob{Deps: deps, Payload: RefreshRuntimePayload{SiteID: "site-1", ServerID: "server-1", TeamID: "team-1"}}
			require.NoError(t, job.Handle(context.Background()))
			require.Equal(t, 1, dispatcher.ExecutionCount())
			require.Contains(t, events.events, "deployment.runtime.completed")
			var tracked servermodels.Task
			require.NoError(t, db.First(&tracked, "id = ?", job.taskID).Error)
			require.NotContains(t, tracked.Script, "mv -Tf")
			require.NotContains(t, tracked.Script, "artisan migrate")
			pending, err := deps.Queue.Inspector().ListPendingTasks(queue.QueueDefault)
			if restart {
				require.NoError(t, err)
				require.Len(t, pending, 1)
				require.Equal(t, TypeRestartAllSiteQueues, pending[0].Type)
			} else {
				require.ErrorIs(t, err, asynq.ErrQueueNotFound)
				require.Empty(t, pending)
			}
		})
	}
}
