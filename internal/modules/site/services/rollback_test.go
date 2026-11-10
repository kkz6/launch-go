package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/site/jobs"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	"github.com/kkz6/launch-go/internal/pkg/queue"
)

func TestRollbackQueueFailureTerminatesDeploymentAndAllowsRetry(t *testing.T) {
	for _, configured := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing queue", true: "queue rejected"}[configured], func(t *testing.T) {
			db := setupWebhookTestDB(t)
			svc := newTestDeploymentService(db)
			site := createTestSite(t, db, nil, nil)
			require.NoError(t, db.Model(site).Update("zero_downtime_deployment", true).Error)
			target := &models.Deployment{Status: sitetypes.DeploymentStatusFinished}
			target.SiteID, target.TeamID = site.ID, site.TeamID
			require.NoError(t, db.Create(target).Error)
			redis := miniredis.RunT(t)
			if configured {
				svc.Queue = queue.NewClient(config.RedisConfig{Address: redis.Addr()})
				redis.SetError("ERR queue unavailable")
			}

			deployment, err := svc.Rollback(context.Background(), site.ID, site.ServerID, site.TeamID, target.ID, "user-1")
			require.Nil(t, deployment)
			require.ErrorContains(t, err, "Rollback could not be queued")
			var failed models.Deployment
			require.NoError(t, db.Where("id != ?", target.ID).First(&failed).Error)
			require.Equal(t, sitetypes.DeploymentStatusFailed, failed.Status)
			active, err := svc.Repos().Deployment().FindActiveBySite(context.Background(), site.ID)
			require.NoError(t, err)
			require.Nil(t, active)

			redis.SetError("")
			if !configured {
				svc.Queue = queue.NewClient(config.RedisConfig{Address: redis.Addr()})
			}
			t.Cleanup(func() { require.NoError(t, svc.Queue.Close()) })
			deployment, err = svc.Rollback(context.Background(), site.ID, site.ServerID, site.TeamID, target.ID, "user-1")
			require.NoError(t, err)
			require.Equal(t, sitetypes.DeploymentStatusPending, deployment.Status)
			scheduled, err := svc.Queue.Inspector().ListScheduledTasks(queue.QueueDefault)
			require.NoError(t, err)
			require.Len(t, scheduled, 1)
			require.Equal(t, 3, scheduled[0].MaxRetry)
			var payload jobs.RollbackPayload
			require.NoError(t, json.Unmarshal(scheduled[0].Payload, &payload))
			require.Equal(t, target.ID, payload.TargetDeploymentID)
			require.Equal(t, deployment.ID, payload.DeploymentID)
		})
	}
}
