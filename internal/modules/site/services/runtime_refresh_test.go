package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	"github.com/kkz6/launch-go/internal/pkg/queue"
)

func TestRuntimeRefreshScopesAndReadiness(t *testing.T) {
	for _, scenario := range []string{"wrong team", "wrong server", "static", "non atomic", "deployment active", "php changing", "php missing", "queue unavailable"} {
		t.Run(scenario, func(t *testing.T) {
			svc, db, site, redis := deploymentDispatchFixture(t)
			require.NoError(t, db.Model(site).Updates(map[string]any{"zero_downtime_deployment": true, "php_version": sitetypes.PhpVersion83}).Error)
			teamID, serverID := site.TeamID, site.ServerID
			switch scenario {
			case "wrong team":
				teamID = "another-team"
			case "wrong server":
				serverID = "another-server"
			case "static":
				require.NoError(t, db.Model(site).Update("type", sitetypes.SiteTypeStatic).Error)
			case "non atomic":
				require.NoError(t, db.Model(site).Update("zero_downtime_deployment", false).Error)
			case "deployment active":
				d := &models.Deployment{Status: sitetypes.DeploymentStatusPending}
				d.SiteID, d.TeamID = site.ID, site.TeamID
				require.NoError(t, db.Create(d).Error)
			case "php changing":
				require.NoError(t, db.Model(site).Update("pending_php_version", sitetypes.PhpVersion84).Error)
			case "php missing":
				require.NoError(t, db.Model(site).Update("php_version", nil).Error)
			case "queue unavailable":
				redis.SetError("ERR unavailable")
			}
			require.Error(t, svc.RefreshRuntime(context.Background(), site.ID, serverID, teamID, site.UserID))
		})
	}
}

func TestRuntimeRefreshDeduplicatesWithoutCreatingDeployment(t *testing.T) {
	svc, db, site, _ := deploymentDispatchFixture(t)
	require.NoError(t, db.Model(site).Updates(map[string]any{"zero_downtime_deployment": true, "type": sitetypes.SiteTypeGeneric, "php_version": sitetypes.PhpVersion83}).Error)
	for i := 0; i < 2; i++ {
		require.NoError(t, svc.RefreshRuntime(context.Background(), site.ID, site.ServerID, site.TeamID, site.UserID))
	}
	queued, err := svc.Queue.Inspector().ListPendingTasks(queue.QueueDefault)
	require.NoError(t, err)
	require.Len(t, queued, 1)
	require.Zero(t, queued[0].MaxRetry)
	var count int64
	require.NoError(t, db.Model(&models.Deployment{}).Count(&count).Error)
	require.Zero(t, count)
}
