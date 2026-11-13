package tasks

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/site/deployments"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

func TestDeploymentCompletionPreservesResultWhenNextDispatchFails(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status sitetypes.DeploymentStatus
		finish func(*deploySiteTask, *taskrunner.CallbackContext) error
	}{
		{"success", sitetypes.DeploymentStatusFinished, func(task *deploySiteTask, cb *taskrunner.CallbackContext) error {
			return task.OnSuccess(context.Background(), cb, "task-1")
		}},
		{"failure", sitetypes.DeploymentStatusFailed, func(task *deploySiteTask, cb *taskrunner.CallbackContext) error {
			return task.OnFailure(context.Background(), cb, "task-1", 1)
		}},
		{"timeout", sitetypes.DeploymentStatusTimeout, func(task *deploySiteTask, cb *taskrunner.CallbackContext) error {
			return task.OnExpired(context.Background(), cb, "task-1")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, task := queuedDeploymentCallback(t, false)
			queue := &callbackQueueStub{errorsByType: map[string]error{
				deployments.TypeDeploy: errors.New("queue unavailable"),
			}}
			var output bytes.Buffer
			logger := zerolog.New(&output)
			cb := &taskrunner.CallbackContext{DB: db, Queue: queue, Logger: &logger}

			require.NoError(t, tc.finish(task, cb))

			assertCallbackDeploymentStatus(t, db, "deployment-1", tc.status)
			assertCallbackDeploymentStatus(t, db, "waiting-1", sitetypes.DeploymentStatusFailed)
			assertCallbackDeploymentStatus(t, db, "waiting-2", sitetypes.DeploymentStatusQueued)
			require.Contains(t, output.String(), "Failed to enqueue next deployment")
		})
	}
}

func TestDeploymentCompletionAdvancesZeroDowntimeQueueOnlyOnce(t *testing.T) {
	db, task := queuedDeploymentCallback(t, true)
	queue := &callbackQueueStub{}
	cb := &taskrunner.CallbackContext{DB: db, Queue: queue}

	require.NoError(t, task.OnSuccess(context.Background(), cb, "task-1"))
	require.NoError(t, task.OnSuccess(context.Background(), cb, "task-1"))

	assertCallbackDeploymentStatus(t, db, "deployment-1", sitetypes.DeploymentStatusFinished)
	assertCallbackDeploymentStatus(t, db, "waiting-1", sitetypes.DeploymentStatusPending)
	assertCallbackDeploymentStatus(t, db, "waiting-2", sitetypes.DeploymentStatusQueued)
	deployCount := 0
	for _, queued := range queue.tasks {
		if queued.Type() != deployments.TypeDeployZeroDowntime {
			continue
		}
		deployCount++
		payload, err := pkgjobs.UnmarshalPayload[deployments.Payload](queued)
		require.NoError(t, err)
		require.Equal(t, "waiting-1", payload.DeploymentID)
		require.Equal(t, "site-1", payload.SiteID)
		require.True(t, payload.ZeroDowntime)
	}
	require.Equal(t, 1, deployCount)
}

func queuedDeploymentCallback(t *testing.T, zeroDowntime bool) (*gorm.DB, *deploySiteTask) {
	t.Helper()
	db := deploymentCallbackDB(t)
	require.NoError(t, db.Exec("ALTER TABLE deployments ADD COLUMN team_id TEXT").Error)
	require.NoError(t, db.Exec("CREATE TABLE sites (id TEXT PRIMARY KEY, zero_downtime_deployment BOOLEAN)").Error)
	require.NoError(t, db.Exec("CREATE TABLE tasks (id TEXT PRIMARY KEY, output TEXT)").Error)
	require.NoError(t, db.Exec("INSERT INTO sites VALUES (?, ?)", "site-1", zeroDowntime).Error)
	for i, id := range []string{"waiting-1", "waiting-2"} {
		require.NoError(t, db.Exec(
			"INSERT INTO deployments (id, site_id, team_id, status, created_at) VALUES (?, ?, ?, ?, ?)",
			id, "site-1", "team-1", sitetypes.DeploymentStatusQueued, time.Now().Add(time.Duration(i)*time.Minute),
		).Error)
	}
	task := deploymentEventTask()
	task.callback.QueueDeployments = true
	task.callback.ZeroDowntimeDeployment = zeroDowntime
	return db, task
}

func assertCallbackDeploymentStatus(t *testing.T, db *gorm.DB, id string, expected sitetypes.DeploymentStatus) {
	t.Helper()
	var deployment models.Deployment
	require.NoError(t, db.First(&deployment, "id = ?", id).Error)
	require.Equal(t, expected, deployment.Status, "deployment %s", id)
}
