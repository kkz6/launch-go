package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/site/deployments"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	"github.com/kkz6/launch-go/internal/pkg/queue"
)

func TestCreateDeploymentJobRecordsMissingQueueFailure(t *testing.T) {
	deps, _, db := deploymentReleaseJobFixture(t)
	deps.Queue = nil
	job := &CreateDeploymentJob{Deps: deps, Payload: CreateDeploymentPayload{SiteID: "site-1"}}

	require.ErrorIs(t, job.Handle(context.Background()), deployments.ErrQueueUnavailable)
	var deployment models.Deployment
	require.NoError(t, db.First(&deployment).Error)
	require.Equal(t, sitetypes.DeploymentStatusFailed, deployment.Status)
	active, err := deps.Repos.Deployment().FindActiveBySite(context.Background(), "site-1")
	require.NoError(t, err)
	require.Nil(t, active)
}

func TestCreateDeploymentJobPreservesScheduledPayload(t *testing.T) {
	const zeroDowntime = true
	deps, _, db := deploymentReleaseJobFixture(t)
	deps.Queue = deploymentJobQueue(t)
	require.NoError(t, db.Model(&models.Site{}).Where("id = ?", "site-1").Update("zero_downtime_deployment", zeroDowntime).Error)
	userID := "user-1"
	job := &CreateDeploymentJob{Deps: deps, Payload: CreateDeploymentPayload{SiteID: "site-1", UserID: &userID}}
	before := time.Now()

	require.NoError(t, job.Handle(context.Background()))
	tasks, err := deps.Queue.Inspector().ListScheduledTasks("default")
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	var payload DeployPayload
	require.NoError(t, json.Unmarshal(tasks[0].Payload, &payload))
	require.Equal(t, "site-1", payload.SiteID)
	require.Equal(t, &userID, payload.UserID)
	require.Equal(t, zeroDowntime, payload.ZeroDowntime)
	require.Equal(t, "deploy_zd:"+payload.DeploymentID, tasks[0].ID)
	require.Equal(t, TypeDeployZeroDowntime, tasks[0].Type)
	require.WithinDuration(t, before.Add(time.Second), tasks[0].NextProcessAt, time.Second)
	assertDeploymentJobStatus(t, db, payload.DeploymentID, sitetypes.DeploymentStatusPending)
}

func TestDeployFailureHandlingAdvancesQueueOnlyOnce(t *testing.T) {
	for _, tc := range []struct {
		name           string
		queueAvailable bool
		preflight      bool
	}{
		{"queue available", true, false},
		{"queue unavailable", false, false},
		{"preflight callback with queue unavailable", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deps, dispatcher, db := deploymentReleaseJobFixture(t)
			redis := miniredis.RunT(t)
			deps.Queue = queue.NewClient(config.RedisConfig{Address: redis.Addr()})
			deps.TaskRunnerDeps.Queue = deps.Queue
			t.Cleanup(func() { require.NoError(t, deps.Queue.Close()) })
			if !tc.queueAvailable {
				redis.SetError("queue unavailable")
			}
			failure := "no dispatcher set"
			if tc.preflight {
				require.NoError(t, db.Table("servers").Where("id = ?", "server-1").Update("public_ipv4", nil).Error)
				failure = "server has no public IP address"
			} else {
				deps.TaskRunnerDeps.Dispatcher = nil
			}
			require.NoError(t, db.Model(&models.Site{}).Where("id = ?", "site-1").Update("queue_deployments", true).Error)
			current := createReleaseDeployment(t, db, "current", nil)
			queuedDeploymentForJob(t, db, "waiting-1", time.Now().Add(-time.Minute))
			queuedDeploymentForJob(t, db, "waiting-2", time.Now())
			job := &DeployJob{Deps: deps, Payload: DeployPayload{SiteID: "site-1", DeploymentID: current.ID}}

			err := job.Handle(context.Background())
			require.ErrorContains(t, err, failure)
			job.Failed(context.Background(), err)

			require.Zero(t, dispatcher.ExecutionCount())
			assertDeploymentJobStatus(t, db, current.ID, sitetypes.DeploymentStatusFailed)
			assertDeploymentJobStatus(t, db, "waiting-2", sitetypes.DeploymentStatusQueued)
			if !tc.queueAvailable {
				assertDeploymentJobStatus(t, db, "waiting-1", sitetypes.DeploymentStatusFailed)
				return
			}
			assertDeploymentJobStatus(t, db, "waiting-1", sitetypes.DeploymentStatusPending)
			tasks, err := deps.Queue.Inspector().ListPendingTasks("default")
			require.NoError(t, err)
			require.Len(t, tasks, 1)
			require.Equal(t, "deploy_zd:waiting-1", tasks[0].ID)
		})
	}
}

func TestDeployJobFailurePreservesAlreadyTerminalDeployment(t *testing.T) {
	for _, status := range []sitetypes.DeploymentStatus{
		sitetypes.DeploymentStatusFinished, sitetypes.DeploymentStatusFailed, sitetypes.DeploymentStatusTimeout,
	} {
		t.Run(string(status), func(t *testing.T) {
			deps, _, db := deploymentReleaseJobFixture(t)
			current := createReleaseDeployment(t, db, "current", nil)
			require.NoError(t, db.Model(current).Update("status", status).Error)
			queuedDeploymentForJob(t, db, "waiting-1", time.Now())
			job := &DeployJob{Deps: deps, Payload: DeployPayload{SiteID: "site-1", DeploymentID: current.ID}}

			job.Failed(context.Background(), errors.New("already handled"))

			assertDeploymentJobStatus(t, db, current.ID, status)
			assertDeploymentJobStatus(t, db, "waiting-1", sitetypes.DeploymentStatusQueued)
		})
	}
}

func TestDeploymentCleanupPreservesTimeoutWhenNextDispatchFails(t *testing.T) {
	deps, _, db := deploymentReleaseJobFixture(t)
	deps.Queue = nil
	current := createReleaseDeployment(t, db, "current", nil)
	require.NoError(t, db.Model(current).Update("status", sitetypes.DeploymentStatusPending).Error)
	queuedDeploymentForJob(t, db, "waiting-1", time.Now().Add(-time.Minute))
	queuedDeploymentForJob(t, db, "waiting-2", time.Now())
	job := &CleanupPendingSiteDeploymentJob{Deps: deps, Payload: CleanupPendingSiteDeploymentPayload{SiteID: "site-1", DeploymentID: current.ID}}

	require.NoError(t, job.Handle(context.Background()))

	assertDeploymentJobStatus(t, db, current.ID, sitetypes.DeploymentStatusTimeout)
	assertDeploymentJobStatus(t, db, "waiting-1", sitetypes.DeploymentStatusFailed)
	assertDeploymentJobStatus(t, db, "waiting-2", sitetypes.DeploymentStatusQueued)
}

func deploymentJobQueue(t *testing.T) *queue.Client {
	t.Helper()
	redis := miniredis.RunT(t)
	client := queue.NewClient(config.RedisConfig{Address: redis.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	return client
}

func queuedDeploymentForJob(t *testing.T, db *gorm.DB, id string, createdAt time.Time) {
	t.Helper()
	deployment := createReleaseDeployment(t, db, id, nil)
	require.NoError(t, db.Model(deployment).Updates(map[string]any{
		"status": sitetypes.DeploymentStatusQueued, "created_at": createdAt,
	}).Error)
}

func assertDeploymentJobStatus(t *testing.T, db *gorm.DB, id string, expected sitetypes.DeploymentStatus) {
	t.Helper()
	var deployment models.Deployment
	require.NoError(t, db.First(&deployment, "id = ?", id).Error)
	require.Equal(t, expected, deployment.Status, "deployment %s", id)
}
