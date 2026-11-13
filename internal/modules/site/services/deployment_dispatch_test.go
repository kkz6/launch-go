package services

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/site/jobs"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	"github.com/kkz6/launch-go/internal/pkg/queue"
	"github.com/kkz6/launch-go/internal/pkg/service"
)

func TestDeployQueueFailurePreservesFailedAttemptAndAllowsRetry(t *testing.T) {
	for _, zeroDowntime := range []bool{false, true} {
		name, taskType := "standard", jobs.TypeDeploy
		if zeroDowntime {
			name, taskType = "zero downtime", jobs.TypeDeployZeroDowntime
		}
		t.Run(name, func(t *testing.T) {
			svc, db, site, redis := deploymentDispatchFixture(t)
			require.NoError(t, db.Model(site).Update("zero_downtime_deployment", zeroDowntime).Error)
			redis.SetError("ERR deployment queue unavailable")
			result, err := svc.Deploy(context.Background(), site.ID, site.ServerID, site.TeamID, site.UserID)
			require.Error(t, err)
			require.Nil(t, result)
			var attempted []models.Deployment
			require.NoError(t, db.Where("site_id = ?", site.ID).Find(&attempted).Error)
			require.Len(t, attempted, 1, "failed dispatch must retain deployment history")
			requireFailedDispatch(t, svc, &attempted[0])

			redis.SetError("")
			retried, err := svc.Deploy(context.Background(), site.ID, site.ServerID, site.TeamID, site.UserID)
			require.NoError(t, err)
			require.NotEqual(t, attempted[0].ID, retried.ID)
			require.Equal(t, sitetypes.DeploymentStatusPending, retried.Status)
			queued, err := svc.Queue.Inspector().ListScheduledTasks(queue.QueueDefault)
			require.NoError(t, err)
			require.Len(t, queued, 1)
			require.Equal(t, taskType, queued[0].Type)
			var payload jobs.DeployPayload
			require.NoError(t, json.Unmarshal(queued[0].Payload, &payload))
			require.Equal(t, retried.ID, payload.DeploymentID)
			require.Equal(t, site.ID, payload.SiteID)
			require.Equal(t, zeroDowntime, payload.ZeroDowntime)
		})
	}
}

func TestRollbackQueueFailurePreservesTargetAndAllowsRetry(t *testing.T) {
	svc, db, site, redis := deploymentDispatchFixture(t)
	require.NoError(t, db.Model(site).Update("zero_downtime_deployment", true).Error)
	target := createDispatchDeployment(t, db, site, sitetypes.DeploymentStatusFinished, time.Now().Add(-time.Hour))
	redis.SetError("ERR deployment queue unavailable")
	result, err := svc.Rollback(context.Background(), site.ID, site.ServerID, site.TeamID, target.ID, site.UserID)
	require.Error(t, err)
	require.Nil(t, result)
	var attempted []models.Deployment
	require.NoError(t, db.Where("site_id = ? AND id <> ?", site.ID, target.ID).Find(&attempted).Error)
	require.Len(t, attempted, 1)
	requireFailedDispatch(t, svc, &attempted[0])
	require.Equal(t, target.ID, attempted[0].CommitData["rollback_to"])
	var preserved models.Deployment
	require.NoError(t, db.First(&preserved, "id = ?", target.ID).Error)
	require.Equal(t, sitetypes.DeploymentStatusFinished, preserved.Status)

	redis.SetError("")
	retried, err := svc.Rollback(context.Background(), site.ID, site.ServerID, site.TeamID, target.ID, site.UserID)
	require.NoError(t, err)
	require.NotEqual(t, attempted[0].ID, retried.ID)
	queued, err := svc.Queue.Inspector().ListScheduledTasks(queue.QueueDefault)
	require.NoError(t, err)
	require.Len(t, queued, 1)
	require.Equal(t, jobs.TypeRollback, queued[0].Type)
	var payload jobs.RollbackPayload
	require.NoError(t, json.Unmarshal(queued[0].Payload, &payload))
	require.Equal(t, retried.ID, payload.DeploymentID)
	require.Equal(t, target.ID, payload.TargetDeploymentID)
}

func TestProcessNextQueuedFailurePreservesPredecessorAndRemainingQueue(t *testing.T) {
	for _, zeroDowntime := range []bool{false, true} {
		name, taskType := "standard", jobs.TypeDeploy
		if zeroDowntime {
			name, taskType = "zero downtime", jobs.TypeDeployZeroDowntime
		}
		t.Run(name, func(t *testing.T) {
			svc, db, site, redis := deploymentDispatchFixture(t)
			require.NoError(t, db.Model(site).Update("zero_downtime_deployment", zeroDowntime).Error)
			predecessor := createDispatchDeployment(t, db, site, sitetypes.DeploymentStatusFinished, time.Now().Add(-2*time.Hour))
			first := createDispatchDeployment(t, db, site, sitetypes.DeploymentStatusQueued, time.Now().Add(-time.Hour))
			second := createDispatchDeployment(t, db, site, sitetypes.DeploymentStatusQueued, time.Now())
			redis.SetError("ERR deployment queue unavailable")
			result, err := svc.ProcessNextQueued(context.Background(), site.ID)
			require.Error(t, err)
			require.Nil(t, result)
			require.NoError(t, db.First(first, "id = ?", first.ID).Error)
			requireFailedDispatch(t, svc, first)
			require.NoError(t, db.First(second, "id = ?", second.ID).Error)
			require.Equal(t, sitetypes.DeploymentStatusQueued, second.Status, "an outage must not consume every queued deployment")
			require.NoError(t, db.First(predecessor, "id = ?", predecessor.ID).Error)
			require.Equal(t, sitetypes.DeploymentStatusFinished, predecessor.Status)

			redis.SetError("")
			next, err := svc.ProcessNextQueued(context.Background(), site.ID)
			require.NoError(t, err)
			require.Equal(t, second.ID, next.ID)
			require.Equal(t, sitetypes.DeploymentStatusPending, next.Status)
			queued, err := svc.Queue.Inspector().ListPendingTasks(queue.QueueDefault)
			require.NoError(t, err)
			require.Len(t, queued, 1)
			require.Equal(t, taskType, queued[0].Type)
		})
	}
}

func TestDeployWithoutQueueFailsInsteadOfReportingSuccess(t *testing.T) {
	svc, db, site, _ := deploymentDispatchFixture(t)
	svc.Queue = nil
	result, err := svc.Deploy(context.Background(), site.ID, site.ServerID, site.TeamID, site.UserID)
	require.Error(t, err)
	require.Nil(t, result)
	var attempted []models.Deployment
	require.NoError(t, db.Where("site_id = ?", site.ID).Find(&attempted).Error)
	require.Len(t, attempted, 1)
	requireFailedDispatch(t, svc, &attempted[0])
}

func deploymentDispatchFixture(t *testing.T) (*DeploymentService, *gorm.DB, *models.Site, *miniredis.Miniredis) {
	t.Helper()
	db := setupWebhookTestDB(t)
	redis := miniredis.RunT(t)
	client := queue.NewClient(config.RedisConfig{Address: redis.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	logger := zerolog.Nop()
	svc := NewDeploymentService(&ServiceDeps{ModuleDeps: service.ModuleDeps[*repositories.Registry]{
		Dependencies: service.Dependencies{DB: db, Logger: &logger, Queue: client, Broadcaster: &deploymentDispatchEvents{}},
		Repos:        repositories.NewRegistry(db),
	}})
	return svc, db, createTestSite(t, db, nil, nil), redis
}

func createDispatchDeployment(t *testing.T, db *gorm.DB, site *models.Site, status sitetypes.DeploymentStatus, createdAt time.Time) *models.Deployment {
	t.Helper()
	deployment := &models.Deployment{Status: status}
	deployment.SiteID, deployment.TeamID, deployment.CreatedAt = site.ID, site.TeamID, &createdAt
	require.NoError(t, db.Create(deployment).Error)
	return deployment
}

func requireFailedDispatch(t *testing.T, svc *DeploymentService, deployment *models.Deployment) {
	t.Helper()
	require.Equal(t, sitetypes.DeploymentStatusFailed, deployment.Status)
	require.Nil(t, deployment.TaskID)
	active, err := svc.Repos().Deployment().FindActiveBySite(context.Background(), deployment.SiteID)
	require.NoError(t, err)
	require.Nil(t, active, "failed enqueue must not block subsequent deployments")
	site, err := svc.Repos().Site().FindByID(context.Background(), deployment.SiteID)
	require.NoError(t, err)
	require.Nil(t, site.InstallationFailedAt, "dispatch repair must not alter the site's installation result")
	events := svc.GetModelBroadcaster().(*deploymentDispatchEvents).events
	require.Len(t, events, 2, "failed dispatch must emit terminal events without reporting a start")
	require.ElementsMatch(t, []string{"deployment.failed", "deployment.progress"}, []string{events[0].name, events[1].name})
	for _, event := range events {
		require.Equal(t, deployment.TeamID, event.teamID)
		payload, ok := event.data.(map[string]any)
		require.True(t, ok)
		require.Equal(t, deployment.TeamID, payload["team_id"])
		require.Equal(t, deployment.SiteID, payload["site_id"])
		require.Equal(t, deployment.ID, payload["deployment_id"])
		require.Equal(t, "failed", payload["status"])
		require.Equal(t, "Failed to queue deployment", payload["message"])
	}
}

type deploymentDispatchEvents struct {
	broadcast.NopModelBroadcaster
	events []deploymentDispatchEvent
}

type deploymentDispatchEvent struct {
	teamID string
	name   string
	data   any
}

func (b *deploymentDispatchEvents) BroadcastToTeam(teamID, event string, data any) {
	b.events = append(b.events, deploymentDispatchEvent{teamID: teamID, name: event, data: data})
}
