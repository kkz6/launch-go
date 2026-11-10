package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
	"github.com/kkz6/launch-go/internal/pkg/queue"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

func TestDeployJobPersistsReleaseBeforeDispatchAndReusesIt(t *testing.T) {
	deps, _, db := deploymentReleaseJobFixture(t)
	// Stop at the dispatch boundary, after the identity must be durable.
	deps.TaskRunnerDeps.Dispatcher = nil
	deployment := createReleaseDeployment(t, db, "01arz3ndektsv4rrffq69g5fav", nil)
	oldCreation := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, db.Model(deployment).Update("created_at", oldCreation).Error)
	payload := DeployPayload{SiteID: "site-1", DeploymentID: deployment.ID, ZeroDowntime: true}
	job := &DeployJob{Deps: deps, Payload: payload}
	require.ErrorContains(t, job.Handle(context.Background()), "no dispatcher set")

	var saved models.Deployment
	require.NoError(t, db.First(&saved, "id = ?", deployment.ID).Error)
	require.NotNil(t, saved.ReleaseName)
	name := *saved.ReleaseName
	// Delayed jobs and simultaneous deliveries choose the same unique name.
	require.Equal(t, "20250101000000-"+deployment.ID, name)

	retry := &DeployJob{Deps: deps, Payload: payload}
	require.ErrorContains(t, retry.Handle(context.Background()), "no dispatcher set")
	require.NoError(t, db.First(&saved, "id = ?", deployment.ID).Error)
	require.Equal(t, name, *saved.ReleaseName)
	config := retry.buildDeployConfig(context.Background(), retry.site, &saved, "team-1")
	require.Equal(t, name, config.ReleaseTimestamp)
}

func TestDeploymentJobsAbortWhenReleaseCannotBeSaved(t *testing.T) {
	for _, rollback := range []bool{false, true} {
		name := "deploy"
		if rollback {
			name = "rollback"
		}
		t.Run(name, func(t *testing.T) {
			deps, dispatcher, db := deploymentReleaseJobFixture(t)
			release := "20261104050000"
			target := createReleaseDeployment(t, db, "target", &release)
			deployment := createReleaseDeployment(t, db, "operation", nil)
			require.NoError(t, db.Model(deployment).Update("status", sitetypes.DeploymentStatusPending).Error)
			require.NoError(t, db.Callback().Update().Before("gorm:update").Register("reject_release_save", func(tx *gorm.DB) {
				if tx.Statement.Table == "deployments" {
					tx.AddError(errors.New("release write failed"))
				}
			}))
			var err error
			if rollback {
				job := &RollbackJob{Deps: deps, Payload: RollbackPayload{
					SiteID: "site-1", DeploymentID: deployment.ID, TargetDeploymentID: target.ID,
				}}
				err = job.Handle(context.Background())
			} else {
				job := &DeployJob{Deps: deps, Payload: DeployPayload{
					SiteID: "site-1", DeploymentID: deployment.ID, ZeroDowntime: true,
				}}
				err = job.Handle(context.Background())
			}
			require.ErrorContains(t, err, "release write failed")
			require.Zero(t, dispatcher.ExecutionCount())
		})
	}
}

func TestRollbackJobUsesRecordedReleaseIncludingRepeatedRollback(t *testing.T) {
	deps, dispatcher, db := deploymentReleaseJobFixture(t)
	release := "20261104050000-01arz3ndektsv4rrffq69g5fav"
	target := createReleaseDeployment(t, db, "original", &release)
	for _, id := range []string{"rollback-1", "rollback-2"} {
		deployment := createReleaseDeployment(t, db, id, nil)
		require.NoError(t, db.Model(deployment).Update("status", sitetypes.DeploymentStatusPending).Error)
		job := &RollbackJob{Deps: deps, Payload: RollbackPayload{
			SiteID: "site-1", DeploymentID: deployment.ID, TargetDeploymentID: target.ID,
		}}
		require.NoError(t, job.Handle(context.Background()))
		require.NoError(t, db.First(deployment, "id = ?", deployment.ID).Error)
		require.Equal(t, sitetypes.DeploymentStatusFinished, deployment.Status)
		require.NotNil(t, deployment.ReleaseName)
		require.Equal(t, release, *deployment.ReleaseName)
		script := dispatcher.GetExecution(dispatcher.ExecutionCount() - 1).Script
		require.Contains(t, script, "/home/launcher/example.test/releases/"+release)
		target = deployment
	}
	require.Equal(t, 2, dispatcher.ExecutionCount())
}

func TestRollbackJobDoesNotDispatchWithoutReleaseInformation(t *testing.T) {
	deps, dispatcher, db := deploymentReleaseJobFixture(t)
	target := createReleaseDeployment(t, db, "legacy-without-task", nil)
	deployment := createReleaseDeployment(t, db, "rollback", nil)
	require.NoError(t, db.Model(deployment).Update("status", sitetypes.DeploymentStatusPending).Error)
	job := &RollbackJob{Deps: deps, Payload: RollbackPayload{
		SiteID: "site-1", DeploymentID: deployment.ID, TargetDeploymentID: target.ID,
	}}
	require.ErrorContains(t, job.Handle(context.Background()), "release information unavailable")
	require.Zero(t, dispatcher.ExecutionCount())
}

func deploymentReleaseJobFixture(t *testing.T) (*JobDeps, *taskrunner.FakeDispatcher, *gorm.DB) {
	t.Helper()
	ensureSiteTaskTemplates(t)
	deps, dispatcher, db := restartQueueJobFixture(t)
	redis := miniredis.RunT(t)
	client := queue.NewClient(config.RedisConfig{Address: redis.Addr()})
	deps.Queue = client
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	require.NoError(t, db.AutoMigrate(&models.Deployment{}))
	require.NoError(t, db.Model(&models.Site{}).Where("id = ?", "site-1").Updates(map[string]any{
		"zero_downtime_deployment": true,
		"installed_at":             time.Now(),
	}).Error)
	return deps, dispatcher, db
}

func createReleaseDeployment(t *testing.T, db *gorm.DB, id string, release *string) *models.Deployment {
	t.Helper()
	deployment := &models.Deployment{
		BaseModel:   basemodels.BaseModel{ID: id},
		Status:      sitetypes.DeploymentStatusFinished,
		ReleaseName: release,
	}
	deployment.SiteID = "site-1"
	deployment.TeamID = "team-1"
	require.NoError(t, db.Create(deployment).Error)
	return deployment
}
