package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/config"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	siteevents "github.com/kkz6/launch-go/internal/modules/site/events"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	pkgevents "github.com/kkz6/launch-go/internal/pkg/events"
	"github.com/kkz6/launch-go/internal/pkg/queue"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

type rollbackCompletionEvents struct {
	uninstallEvents
	completed []map[string]any
}

func (e *rollbackCompletionEvents) BroadcastToTeam(teamID, event string, data any) {
	e.uninstallEvents.BroadcastToTeam(teamID, event, data)
	if event == "deployment.rollback.completed" {
		e.completed = append(e.completed, data.(map[string]any))
	}
}

func rollbackCompletionFixture(t *testing.T) (*RollbackJob, *taskrunner.FakeDispatcher, *gorm.DB, *rollbackCompletionEvents) {
	t.Helper()
	deps, dispatcher, db := deploymentReleaseJobFixture(t)
	release := "20261104050000"
	target := createReleaseDeployment(t, db, "original", &release)
	deployment := createReleaseDeployment(t, db, "rollback", nil)
	require.NoError(t, db.Model(deployment).Update("status", sitetypes.DeploymentStatusPending).Error)
	events := &rollbackCompletionEvents{}
	deps.Broadcaster = events
	return &RollbackJob{Deps: deps, Payload: RollbackPayload{
		SiteID: "site-1", DeploymentID: deployment.ID, TargetDeploymentID: target.ID,
	}}, dispatcher, db, events
}

func TestRollbackRejectsExecutionErrorsAndPreservesTaskLogs(t *testing.T) {
	for _, failure := range []string{"connection", "result error", "timeout", "exit code", "connection setup"} {
		t.Run(failure, func(t *testing.T) {
			job, dispatcher, db, events := rollbackCompletionFixture(t)
			switch failure {
			case "connection":
				dispatcher.SetRunError(errors.New("connection closed"))
			case "result error":
				dispatcher.SetResult("Rollback Deployment", &taskrunner.TaskResult{Error: errors.New("stream interrupted")})
			case "timeout":
				dispatcher.SetResult("Rollback Deployment", &taskrunner.TaskResult{TimedOut: true})
			case "exit code":
				dispatcher.SetFailure("Rollback Deployment", 1, "Target release does not exist")
			case "connection setup":
				require.NoError(t, db.Model(&servermodels.Server{}).Where("id = ?", "server-1").Update("public_ipv4", nil).Error)
			}
			require.ErrorIs(t, job.Handle(context.Background()), asynq.SkipRetry)
			var deployment models.Deployment
			require.NoError(t, db.First(&deployment, "id = ?", "rollback").Error)
			require.Equal(t, sitetypes.DeploymentStatusFailed, deployment.Status)
			require.NotNil(t, deployment.TaskID)
			var task servermodels.Task
			require.NoError(t, db.First(&task, "id = ?", *deployment.TaskID).Error)
			require.Equal(t, "site-1", *task.SiteID)
			require.NotContains(t, events.events, "deployment.rollback.completed")
			require.Contains(t, events.events, "deployment.rollback.failed")
		})
	}
}

func TestRollbackNeverRepeatsAnAttemptWithUnknownOrFailedOutcome(t *testing.T) {
	for _, status := range []sitetypes.DeploymentStatus{
		sitetypes.DeploymentStatusInstalling,
		sitetypes.DeploymentStatusFailed,
		sitetypes.DeploymentStatusTimeout,
	} {
		t.Run(string(status), func(t *testing.T) {
			job, dispatcher, db, events := rollbackCompletionFixture(t)
			require.NoError(t, db.Model(&models.Deployment{}).Where("id = ?", "rollback").Update("status", status).Error)
			err := job.Handle(context.Background())
			require.ErrorIs(t, err, asynq.SkipRetry)
			require.ErrorContains(t, err, "refusing to repeat the remote switch")
			require.Zero(t, dispatcher.ExecutionCount())
			job.Failed(context.Background(), err)
			var saved models.Deployment
			require.NoError(t, db.First(&saved, "id = ?", "rollback").Error)
			require.Equal(t, sitetypes.DeploymentStatusFailed, saved.Status)
			require.NotContains(t, events.events, "deployment.rollback.completed")
		})
	}
}

func TestRollbackClaimCannotOverwriteAnotherWorkersClaim(t *testing.T) {
	job, dispatcher, db, events := rollbackCompletionFixture(t)
	claimed := false
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("competing_rollback_claim", func(tx *gorm.DB) {
		if tx.Statement.Table == "deployments" && !claimed {
			claimed = true
			// A competing worker wins after this delivery read pending, before
			// its conditional claim reaches the database.
			tx.AddError(tx.Exec("UPDATE deployments SET status = ? WHERE id = ?", sitetypes.DeploymentStatusInstalling, "rollback").Error)
		}
	}))
	err := job.Handle(context.Background())
	require.ErrorIs(t, err, errRollbackAlreadyClaimed)
	require.ErrorIs(t, err, asynq.SkipRetry)
	require.Zero(t, dispatcher.ExecutionCount())
	job.Failed(context.Background(), err)
	var saved models.Deployment
	require.NoError(t, db.First(&saved, "id = ?", "rollback").Error)
	require.Equal(t, sitetypes.DeploymentStatusInstalling, saved.Status)
	require.Empty(t, events.events)
}

func TestRollbackRetriesEventPublicationWithoutSwitchingAgain(t *testing.T) {
	job, dispatcher, db, events := rollbackCompletionFixture(t)
	redis := miniredis.RunT(t)
	job.Deps.Queue = queue.NewClient(config.RedisConfig{Address: redis.Addr()})
	t.Cleanup(func() { require.NoError(t, job.Deps.Queue.Close()) })
	redis.SetError("ERR queue unavailable")

	err := job.Handle(context.Background())
	require.ErrorIs(t, err, errRollbackPostActions)
	require.NotErrorIs(t, err, asynq.SkipRetry)
	job.Failed(context.Background(), err)
	var deployment models.Deployment
	require.NoError(t, db.First(&deployment, "id = ?", "rollback").Error)
	require.Equal(t, sitetypes.DeploymentStatusFinished, deployment.Status)
	require.NotContains(t, events.events, "deployment.rollback.failed")
	require.Contains(t, events.events, "deployment.rollback.completed")
	require.Len(t, events.completed, 1)
	require.Equal(t, "finished", events.completed[0]["status"])
	require.Equal(t, true, events.completed[0]["post_actions_pending"])
	require.Equal(t, "Code restored, but worker restart could not be queued.", events.completed[0]["warning"])
	require.Equal(t, 1, dispatcher.ExecutionCount())

	redis.SetError("")
	// A transient read failure during publication-only recovery must not
	// downgrade the completed switch or turn off the remaining retries.
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("fail_recovery_site_read", func(tx *gorm.DB) {
		if tx.Statement.Table == "sites" {
			tx.AddError(errors.New("site read unavailable"))
		}
	}))
	retry := &RollbackJob{Deps: job.Deps, Payload: job.Payload}
	err = retry.Handle(context.Background())
	require.ErrorIs(t, err, errRollbackPostActions)
	require.NotErrorIs(t, err, asynq.SkipRetry)
	retry.Failed(context.Background(), err)
	require.NoError(t, db.First(&deployment, "id = ?", "rollback").Error)
	require.Equal(t, sitetypes.DeploymentStatusFinished, deployment.Status)
	require.NoError(t, db.Callback().Query().Remove("fail_recovery_site_read"))
	// Even a non-publication error delivered to Failed must preserve a
	// durably completed code switch.
	retry.Failed(context.Background(), errors.New("worker interrupted"))
	require.NoError(t, db.First(&deployment, "id = ?", "rollback").Error)
	require.Equal(t, sitetypes.DeploymentStatusFinished, deployment.Status)
	// Event recovery does not depend on the historical target remaining in
	// the deployment list, and must not re-activate it after a newer deploy.
	require.NoError(t, db.Delete(&models.Deployment{}, "id = ?", "original").Error)
	retry = &RollbackJob{Deps: job.Deps, Payload: job.Payload}
	require.NoError(t, retry.Handle(context.Background()))
	require.Equal(t, 1, dispatcher.ExecutionCount())
	require.Contains(t, events.events, "deployment.rollback.completed")
	require.NotContains(t, events.completed[len(events.completed)-1], "post_actions_pending")
	enqueued, err := job.Deps.Queue.Inspector().ListPendingTasks(queue.QueueDefault)
	require.NoError(t, err)
	require.Len(t, enqueued, 1)
	// Another delivery accepts the existing event without adding a second.
	require.NoError(t, retry.Handle(context.Background()))
	require.Equal(t, 1, dispatcher.ExecutionCount())
	enqueued, err = job.Deps.Queue.Inspector().ListPendingTasks(queue.QueueDefault)
	require.NoError(t, err)
	require.Len(t, enqueued, 1)
}

func TestRollbackPublishesWorkerRestartEventAfterCompletion(t *testing.T) {
	for _, autoRestart := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[autoRestart], func(t *testing.T) {
			job, _, db, _ := rollbackCompletionFixture(t)
			require.NoError(t, db.Model(&models.Site{}).Where("id = ?", "site-1").Update("auto_restart_queue", autoRestart).Error)
			redis := miniredis.RunT(t)
			job.Deps.Queue = queue.NewClient(config.RedisConfig{Address: redis.Addr()})
			t.Cleanup(func() { require.NoError(t, job.Deps.Queue.Close()) })

			require.NoError(t, job.Handle(context.Background()))
			enqueued, err := job.Deps.Queue.Inspector().ListPendingTasks(queue.QueueDefault)
			require.NoError(t, err)
			require.Len(t, enqueued, 1)
			require.Equal(t, siteevents.TypeProcessEvent, enqueued[0].Type)
			require.Equal(t, postDeployMaxRetries, enqueued[0].MaxRetry)
			var event pkgevents.Event
			require.NoError(t, json.Unmarshal(enqueued[0].Payload, &event))
			payload, err := pkgevents.Decode[siteevents.DeploymentSucceeded](event)
			require.NoError(t, err)
			require.Equal(t, "rollback", payload.DeploymentID)
			require.Equal(t, *job.deployment.TaskID, payload.TaskID)
			require.Equal(t, "site-1", payload.SiteID)
			require.Equal(t, "server-1", payload.ServerID)
			require.Equal(t, "team-1", payload.TeamID)

			// The same listener used for normal deployments honors the site's
			// restart setting and schedules all installed workers, including Octane.
			require.NoError(t, configureSiteEventDispatcher(job.Deps).Dispatch(context.Background(), event))
			enqueued, err = job.Deps.Queue.Inspector().ListPendingTasks(queue.QueueDefault)
			require.NoError(t, err)
			if autoRestart {
				require.Len(t, enqueued, 2)
				types := []string{enqueued[0].Type, enqueued[1].Type}
				require.Contains(t, types, TypeRestartAllSiteQueues)
			} else {
				require.Len(t, enqueued, 1)
			}
		})
	}
}

func TestRollbackDoesNotReportSuccessBeforeStatusIsSaved(t *testing.T) {
	job, _, db, events := rollbackCompletionFixture(t)
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("reject_finished_rollback", func(tx *gorm.DB) {
		if deployment, ok := tx.Statement.Dest.(*models.Deployment); ok && deployment.ID == "rollback" && deployment.Status == sitetypes.DeploymentStatusFinished {
			tx.AddError(errors.New("database unavailable"))
		}
	}))
	require.ErrorContains(t, job.Handle(context.Background()), "failed to save completed rollback")
	require.NotContains(t, events.events, "deployment.rollback.completed")
}
