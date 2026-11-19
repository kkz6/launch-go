package jobs

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	gitaccess "github.com/kkz6/launch-go/internal/modules/git/access"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	serverservices "github.com/kkz6/launch-go/internal/modules/server/services"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	"github.com/kkz6/launch-go/internal/pkg/service"
)

func TestDeployJobPreparationFailureHasScopedDiagnostic(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prepare func(*testing.T, *gorm.DB, *models.Deployment)
		message string
	}{
		{
			name: "changed repository authorization",
			prepare: func(t *testing.T, db *gorm.DB, deployment *models.Deployment) {
				require.NoError(t, db.Model(deployment).Update("git_access", dbtype.JSONMap{"connection_id": "old-connection"}).Error)
			},
			message: "Repository access changed after this deployment was requested.",
		},
		{
			name: "incomplete repository selection",
			prepare: func(t *testing.T, db *gorm.DB, _ *models.Deployment) {
				require.NoError(t, db.Model(&models.Site{}).Where("id = ?", "site-1").Update("source_control_id", "connection-1").Error)
			},
			message: "The repository selection is incomplete or does not match its connection.",
		},
		{
			name: "missing release configuration",
			prepare: func(t *testing.T, db *gorm.DB, deployment *models.Deployment) {
				require.NoError(t, db.Model(deployment).Update("created_at", nil).Error)
			},
			message: "The deployment is missing the information needed to prepare its release.",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deps, dispatcher, db := deploymentReleaseJobFixture(t)
			deployment := pendingPreparationDeployment(t, db)
			tc.prepare(t, db, deployment)
			job := &DeployJob{Deps: deps, Payload: DeployPayload{SiteID: "site-1", DeploymentID: deployment.ID, ZeroDowntime: true}}
			err := job.Handle(context.Background())
			require.Error(t, err)
			job.Failed(context.Background(), err)

			require.Zero(t, dispatcher.ExecutionCount())
			require.NoError(t, db.First(deployment, "id = ?", deployment.ID).Error)
			require.Equal(t, sitetypes.DeploymentStatusFailed, deployment.Status)
			require.NotNil(t, deployment.TaskID)
			var site models.Site
			require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
			require.NotNil(t, site.InstallationFailedAt)
			latest, err := deps.Repos.Deployment().FindLatestBySite(context.Background(), site.ID)
			require.NoError(t, err)
			require.Equal(t, deployment.TaskID, latest.TaskID)

			api := serverservices.NewService(serverservices.ServiceDeps{
				Dependencies: service.Dependencies{DB: db, Logger: deps.Logger}, Repos: deps.ServerRepos,
			})
			response, err := api.GetTask(context.Background(), *deployment.TaskID, "server-1", "team-1")
			require.NoError(t, err)
			require.Equal(t, "Prepare site deployment", response.Name)
			require.Equal(t, "failed", response.Status)
			require.NotNil(t, response.Output)
			require.Contains(t, *response.Output, tc.message)
			_, err = api.GetTask(context.Background(), *deployment.TaskID, "server-1", "another-team")
			require.Error(t, err)
			_, err = api.GetTask(context.Background(), *deployment.TaskID, "another-server", "team-1")
			require.Error(t, err)

			var stored string
			require.NoError(t, db.Table("tasks").Select("output").Where("id = ?", *deployment.TaskID).Scan(&stored).Error)
			require.NotEmpty(t, stored)
			require.NotContains(t, stored, tc.message, "diagnostics use the existing encrypted task output")
		})
	}
}

func TestDeployJobPreparationDiagnosticNeverCopiesUnknownError(t *testing.T) {
	deps, _, db := deploymentReleaseJobFixture(t)
	deployment := pendingPreparationDeployment(t, db)
	job := &DeployJob{Deps: deps, Payload: DeployPayload{SiteID: "site-1", DeploymentID: deployment.ID}}
	job.Failed(context.Background(), errors.New("clone https://user:private-password@github.com/owner/repo failed with token private-token and DATABASE_PASSWORD=private-env"))
	var task servermodels.Task
	require.NoError(t, db.First(&task).Error)
	require.Equal(t, "Deployment preparation failed. Check the site's repository and deployment settings, then start a new deployment.", task.Output.String())
	require.Empty(t, task.Script)
	require.Empty(t, task.Instance)
	require.NotContains(t, task.Output.String(), "private-")
	require.Contains(t, deploymentPreparationFailureMessage(fmt.Errorf("upstream private-token: %w", gitaccess.ErrAccessDenied)), "Repository access is unavailable.")
	require.NotContains(t, deploymentPreparationFailureMessage(fmt.Errorf("upstream private-token: %w", gitaccess.ErrStaleAccess)), "private-token")
}

func TestDeployJobPreparationDiagnosticIsAtomicAndIdempotent(t *testing.T) {
	for _, emptyTaskID := range []bool{false, true} {
		t.Run(fmt.Sprintf("empty_task_id_%t", emptyTaskID), func(t *testing.T) {
			deps, _, db := deploymentReleaseJobFixture(t)
			deployment := pendingPreparationDeployment(t, db)
			if emptyTaskID {
				require.NoError(t, db.Model(deployment).Update("task_id", "").Error)
			}
			// SQLite has no row locks; serial connections model the production
			// row lock while concurrent callbacks exercise the idempotence guard.
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			var callbacks sync.WaitGroup
			for attempt := 0; attempt < 6; attempt++ {
				callbacks.Go(func() {
					job := &DeployJob{Deps: deps, Payload: DeployPayload{SiteID: "site-1", DeploymentID: deployment.ID}}
					job.Failed(context.Background(), gitaccess.ErrStaleAccess)
				})
			}
			callbacks.Wait()
			var tasks []servermodels.Task
			require.NoError(t, db.Find(&tasks).Error)
			require.Len(t, tasks, 1)
			require.NoError(t, db.First(deployment, "id = ?", deployment.ID).Error)
			require.Equal(t, &tasks[0].ID, deployment.TaskID)
			require.Equal(t, sitetypes.DeploymentStatusFailed, deployment.Status)
		})
	}

	t.Run("link failure rolls back diagnostic and status", func(t *testing.T) {
		deps, _, db := deploymentReleaseJobFixture(t)
		deployment := pendingPreparationDeployment(t, db)
		require.NoError(t, db.Callback().Update().Before("gorm:update").Register("test:reject_diagnostic_link", func(tx *gorm.DB) {
			if tx.Statement.Table == "deployments" {
				tx.AddError(errors.New("deployment update unavailable"))
			}
		}))
		defer db.Callback().Update().Remove("test:reject_diagnostic_link")
		job := &DeployJob{Deps: deps, Payload: DeployPayload{SiteID: "site-1", DeploymentID: deployment.ID}}
		job.Failed(context.Background(), gitaccess.ErrStaleAccess)
		assertNoPreparationDiagnostic(t, db, deployment.ID)
	})
}

func TestDeployJobPreparationFailurePreservesExistingTaskAndHistory(t *testing.T) {
	deps, _, db := deploymentReleaseJobFixture(t)
	deployment := pendingPreparationDeployment(t, db)
	task := servermodels.Task{Name: "Deploy site", Type: "site:deploy", User: "launcher", Status: "failed", Output: "original deployment output"}
	task.ServerID = "server-1"
	require.NoError(t, db.Create(&task).Error)
	require.NoError(t, db.Model(deployment).Update("task_id", task.ID).Error)
	job := &DeployJob{Deps: deps, Payload: DeployPayload{SiteID: "site-1", DeploymentID: deployment.ID}}
	job.Failed(context.Background(), gitaccess.ErrStaleAccess)
	require.NoError(t, db.First(deployment, "id = ?", deployment.ID).Error)
	require.Equal(t, sitetypes.DeploymentStatusFailed, deployment.Status)
	require.Equal(t, &task.ID, deployment.TaskID)
	var saved servermodels.Task
	require.NoError(t, db.First(&saved, "id = ?", task.ID).Error)
	require.Equal(t, task.Output, saved.Output)

	historical := createReleaseDeployment(t, db, "old-failure", nil)
	require.NoError(t, db.Model(historical).Update("status", sitetypes.DeploymentStatusFailed).Error)
	job.Payload.DeploymentID = historical.ID
	job.Failed(context.Background(), gitaccess.ErrStaleAccess)
	require.NoError(t, db.First(historical, "id = ?", historical.ID).Error)
	require.Nil(t, historical.TaskID, "do not invent diagnostic logs for terminal history")
	var count int64
	require.NoError(t, db.Model(&servermodels.Task{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestDeployJobPreparationFailureRejectsMismatchedScope(t *testing.T) {
	for _, scope := range []string{"site", "deployment team", "server team", "changed ownership"} {
		t.Run(scope, func(t *testing.T) {
			deps, dispatcher, db := deploymentReleaseJobFixture(t)
			deployment := pendingPreparationDeployment(t, db)
			job := &DeployJob{Deps: deps, Payload: DeployPayload{SiteID: "site-1", DeploymentID: deployment.ID}}
			switch scope {
			case "site":
				require.NoError(t, db.Model(deployment).Update("site_id", "another-site").Error)
			case "deployment team":
				require.NoError(t, db.Model(deployment).Update("team_id", "another-team").Error)
			case "server team":
				require.NoError(t, db.Model(&servermodels.Server{}).Where("id = ?", "server-1").Update("team_id", "another-team").Error)
			case "changed ownership":
				// Capture a genuine preparation failure before an account transfer.
				require.NoError(t, db.Model(deployment).Update("git_access", dbtype.JSONMap{"connection_id": "old"}).Error)
				require.ErrorIs(t, job.Handle(context.Background()), gitaccess.ErrStaleAccess)
				for _, table := range []string{"servers", "sites", "deployments"} {
					require.NoError(t, db.Table(table).Where("team_id = ?", "team-1").Update("team_id", "another-team").Error)
				}
			}
			if scope != "changed ownership" {
				require.Error(t, job.Handle(context.Background()))
			}
			job.Failed(context.Background(), errors.New("scope denied"))
			require.Zero(t, dispatcher.ExecutionCount())
			assertNoPreparationDiagnostic(t, db, deployment.ID)
		})
	}
}

func pendingPreparationDeployment(t *testing.T, db *gorm.DB) *models.Deployment {
	t.Helper()
	deployment := createReleaseDeployment(t, db, "preparation", nil)
	require.NoError(t, db.Model(deployment).Update("status", sitetypes.DeploymentStatusPending).Error)
	require.NoError(t, db.Model(&models.Site{}).Where("id = ?", "site-1").Update("installed_at", nil).Error)
	return deployment
}

func assertNoPreparationDiagnostic(t *testing.T, db *gorm.DB, deploymentID string) {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(&servermodels.Task{}).Count(&count).Error)
	require.Zero(t, count)
	var deployment models.Deployment
	require.NoError(t, db.First(&deployment, "id = ?", deploymentID).Error)
	require.Nil(t, deployment.TaskID)
	require.Equal(t, sitetypes.DeploymentStatusPending, deployment.Status)
	var site models.Site
	require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
	require.Nil(t, site.InstallationFailedAt)
}
