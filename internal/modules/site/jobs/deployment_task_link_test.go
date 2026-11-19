package jobs

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

func TestDeployJobPreflightFailureRetainsCurrentInstallationLog(t *testing.T) {
	for _, zeroDowntime := range []bool{false, true} {
		t.Run(fmt.Sprintf("zero_downtime_%t", zeroDowntime), func(t *testing.T) {
			deps, dispatcher, db := deploymentReleaseJobFixture(t)
			deps.TaskRunnerDeps.Queue = deps.Queue
			events := taskLinkEvents{failed: make(chan struct{}, 1)}
			deps.Broadcaster, deps.TaskRunnerDeps.Broadcaster = events, events
			require.NoError(t, db.Model(&models.Site{}).Where("id = ?", "site-1").Updates(map[string]any{"installed_at": nil, "zero_downtime_deployment": zeroDowntime}).Error)
			require.NoError(t, db.Table("servers").Where("id = ?", "server-1").Update("public_ipv4", nil).Error)
			var oldTaskID string
			for attempt := 1; attempt <= 2; attempt++ {
				deployment := createReleaseDeployment(t, db, fmt.Sprintf("install-attempt-%d", attempt), nil)
				job := &DeployJob{Deps: deps, Payload: DeployPayload{SiteID: "site-1", DeploymentID: deployment.ID, ZeroDowntime: zeroDowntime}}
				require.ErrorContains(t, job.Handle(context.Background()), "server has no public IP address")
				require.NoError(t, db.First(deployment, "id = ?", deployment.ID).Error)
				require.Equal(t, sitetypes.DeploymentStatusFailed, deployment.Status)
				require.NotNil(t, deployment.TaskID)
				require.NotEqual(t, oldTaskID, *deployment.TaskID)
				var task servermodels.Task
				require.NoError(t, db.First(&task, "id = ?", *deployment.TaskID).Error)
				require.Equal(t, "server-1", task.ServerID)
				require.Equal(t, "site-1", *task.SiteID)
				require.Equal(t, "failed", task.Status)
				require.Contains(t, task.Output.String(), "server has no public IP address")
				latest, err := deps.Repos.Deployment().FindLatestBySite(context.Background(), "site-1")
				require.NoError(t, err)
				require.Equal(t, deployment.ID, latest.ID)
				require.Equal(t, deployment.TaskID, latest.TaskID, "the existing site response must point to this attempt's log")
				oldTaskID = *deployment.TaskID
			}
			require.Zero(t, dispatcher.ExecutionCount())
		})
	}
}

type taskLinkDispatcher func(context.Context, *taskrunner.PendingTask) (*taskrunner.TaskResult, error)

func (f taskLinkDispatcher) Run(ctx context.Context, pending *taskrunner.PendingTask) (*taskrunner.TaskResult, error) {
	return f(ctx, pending)
}

type taskLinkEvents struct {
	broadcast.NopBroadcaster
	failed chan struct{}
}

func (e taskLinkEvents) BroadcastToTeam(_ string, event string, _ any) {
	if event == "deployment.failed" {
		select {
		case e.failed <- struct{}{}:
		default:
		}
	}
}

func waitTaskLinkFailure(t *testing.T, events taskLinkEvents) {
	t.Helper()
	select {
	case <-events.failed:
	case <-time.After(5 * time.Second):
		t.Fatal("deployment failure callback did not finish")
	}
}

func TestDeployJobLinksTaskBeforeBackgroundExecution(t *testing.T) {
	deps, _, db := deploymentReleaseJobFixture(t)
	deps.TaskRunnerDeps.Queue = deps.Queue
	events := taskLinkEvents{failed: make(chan struct{}, 1)}
	deps.Broadcaster, deps.TaskRunnerDeps.Broadcaster = events, events
	deployment := createReleaseDeployment(t, db, "deployment", nil)
	observed := make(chan error, 1)
	deps.TaskRunnerDeps.Dispatcher = taskLinkDispatcher(func(ctx context.Context, pending *taskrunner.PendingTask) (*taskrunner.TaskResult, error) {
		var saved models.Deployment
		err := db.WithContext(ctx).First(&saved, "id = ?", deployment.ID).Error
		if err == nil && (saved.TaskID == nil || *saved.TaskID != pending.TaskID) {
			err = errors.New("deployment was dispatched without its task log identity")
		}
		if err == nil && saved.Status != sitetypes.DeploymentStatusInstalling {
			err = errors.New("current installation status was lost before dispatch")
		}
		if err == nil {
			err = pending.Task.(interface{ BeforeExecution(context.Context) error }).BeforeExecution(ctx)
		}
		observed <- err
		pending.GetCompletionChannel() <- &taskrunner.TaskResult{TaskID: pending.TaskID, ExitCode: 1, Output: "deployment script failed"}
		return nil, nil
	})
	job := &DeployJob{Deps: deps, Payload: DeployPayload{SiteID: "site-1", DeploymentID: deployment.ID}}
	require.NoError(t, job.Handle(context.Background()))
	select {
	case err := <-observed:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("background task did not start")
	}
	waitTaskLinkFailure(t, events)
	var saved models.Deployment
	require.NoError(t, db.First(&saved, "id = ?", deployment.ID).Error)
	require.NotNil(t, saved.TaskID)
	var task servermodels.Task
	require.NoError(t, db.First(&task, "id = ?", *saved.TaskID).Error)
	require.Equal(t, "site-1", *task.SiteID)
	require.Equal(t, "failed", task.Status)
	require.Equal(t, "deployment script failed", task.Output.String())
}

func TestDeployJobAssociationFailurePreventsRemoteExecution(t *testing.T) {
	for _, preflightFails := range []bool{false, true} {
		t.Run(fmt.Sprintf("connection_failure_%t", preflightFails), func(t *testing.T) {
			deps, _, db := deploymentReleaseJobFixture(t)
			deps.TaskRunnerDeps.Queue = deps.Queue
			events := taskLinkEvents{failed: make(chan struct{}, 1)}
			deps.Broadcaster, deps.TaskRunnerDeps.Broadcaster = events, events
			deployment := createReleaseDeployment(t, db, "deployment", nil)
			if preflightFails {
				require.NoError(t, db.Table("servers").Where("id = ?", "server-1").Update("public_ipv4", nil).Error)
			}
			require.NoError(t, db.Callback().Update().Before("gorm:update").Register("test:reject_task_link", func(tx *gorm.DB) {
				if tx.Statement.Table != "deployments" {
					return
				}
				if fields, ok := tx.Statement.Dest.(map[string]any); ok {
					if _, exists := fields["task_id"]; exists {
						tx.AddError(errors.New("task link unavailable"))
					}
				}
			}))
			defer db.Callback().Update().Remove("test:reject_task_link")
			checked := make(chan error, 1)
			deps.TaskRunnerDeps.Dispatcher = taskLinkDispatcher(func(ctx context.Context, pending *taskrunner.PendingTask) (*taskrunner.TaskResult, error) {
				// The production dispatcher calls the same guard before connecting/uploading
				// any task script or credentials. A failed log association must stop here.
				err := pending.Task.(interface{ BeforeExecution(context.Context) error }).BeforeExecution(ctx)
				checked <- err
				if err == nil {
					pending.GetCompletionChannel() <- &taskrunner.TaskResult{TaskID: pending.TaskID, ExitCode: 0}
				}
				return nil, err
			})
			job := &DeployJob{Deps: deps, Payload: DeployPayload{SiteID: "site-1", DeploymentID: deployment.ID}}
			err := job.Handle(context.Background())
			if preflightFails {
				require.ErrorContains(t, err, "task link unavailable")
				require.ErrorContains(t, err, "server has no public IP address")
			} else {
				require.NoError(t, err, "background failures are reported through the tracked task")
				select {
				case err := <-checked:
					require.ErrorContains(t, err, "task link unavailable")
				case <-time.After(5 * time.Second):
					t.Fatal("task execution guard did not run")
				}
			}
			waitTaskLinkFailure(t, events)
			var saved models.Deployment
			require.NoError(t, db.First(&saved, "id = ?", deployment.ID).Error)
			require.Nil(t, saved.TaskID)
			require.Equal(t, sitetypes.DeploymentStatusFailed, saved.Status)
			var task servermodels.Task
			require.NoError(t, db.Where("site_id = ?", "site-1").First(&task).Error)
			require.Equal(t, "failed", task.Status)
			if !preflightFails {
				require.Contains(t, task.Output.String(), "task link unavailable")
			}
		})
	}
}
