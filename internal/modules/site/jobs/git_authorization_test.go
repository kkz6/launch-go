package jobs

import (
	"context"
	"strconv"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/pkg/queue"
	"gorm.io/gorm"

	githubfixture "github.com/kkz6/launch-go/internal/modules/docker/testutil"
	siteaccess "github.com/kkz6/launch-go/internal/modules/site/gitaccess"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/tasks"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/stretchr/testify/require"
)

func TestWebhookDeploymentRedeliveryReusesItsPendingRecord(t *testing.T) {
	deps, _, db := deploymentReleaseJobFixture(t)
	deps.Queue = nil
	grant := githubfixture.SelectedRepository(t, db, "team-1", "org/selected", map[string]any{})
	require.NoError(t, db.Model(&models.Site{}).Where("id = ?", "site-1").Updates(map[string]any{"source_control_id": grant.SourceControl.ID, "source_control_repositories_id": grant.Repository.ID}).Error)
	var site models.Site
	require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
	snapshot, err := siteaccess.Capture(context.Background(), db, nil, &site)
	require.NoError(t, err)
	job := &CreateDeploymentJob{Deps: deps, Payload: CreateDeploymentPayload{SiteID: site.ID, GitAccess: snapshot, WebhookKey: "one-delivery:one-site"}}
	// Retrying a queue outage cannot manufacture a second deployment row.
	require.ErrorContains(t, job.Handle(context.Background()), "queue unavailable")
	require.ErrorContains(t, job.Handle(context.Background()), "queue unavailable")
	var rows []models.Deployment
	require.NoError(t, db.Where("webhook_key = ?", job.Payload.WebhookKey).Find(&rows).Error)
	require.Len(t, rows, 1)
	require.Equal(t, sitetypes.DeploymentStatusPending, rows[0].Status)
	deps.Queue = gitAuthorizationJobQueue(t)
	require.NoError(t, job.Handle(context.Background()))
	require.NoError(t, job.Handle(context.Background()))
	queued, err := deps.Queue.Inspector().ListScheduledTasks("default")
	require.NoError(t, err)
	require.Len(t, queued, 1)
}

type revokingRepositoryDispatcher struct {
	*taskrunner.FakeDispatcher
	revoke func() error
}

func (d *revokingRepositoryDispatcher) Run(ctx context.Context, task *taskrunner.PendingTask) (*taskrunner.TaskResult, error) {
	if err := d.revoke(); err != nil {
		return nil, err
	}
	if guarded, ok := task.Task.(interface{ BeforeExecution(context.Context) error }); ok {
		if err := guarded.BeforeExecution(ctx); err != nil {
			return nil, err
		}
	}
	return d.FakeDispatcher.Run(ctx, task)
}

func TestRollbackRejectsRevokedDependencyBeforeRemoteExecution(t *testing.T) {
	for _, stage := range []string{"before job", "before execution"} {
		t.Run(stage, func(t *testing.T) {
			deps, dispatcher, db := deploymentReleaseJobFixture(t)
			grant := githubfixture.SelectedRepository(t, db, "team-1", "org/selected", map[string]any{})
			dependency := githubfixture.SelectedRepository(t, db, "team-1", "org/dependency", map[string]any{})
			require.NoError(t, db.Model(&models.Site{}).Where("id = ?", "site-1").Updates(map[string]any{"source_control_id": grant.SourceControl.ID, "source_control_repositories_id": grant.Repository.ID}).Error)
			var site models.Site
			require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
			site.RepositoryDependencyIDs = []string{strconv.FormatUint(dependency.Repository.ID, 10)}
			require.NoError(t, db.Model(&site).Update("repository_dependency_ids", site.RepositoryDependencyIDs).Error)
			snapshot, err := siteaccess.Capture(context.Background(), db, nil, &site)
			require.NoError(t, err)
			release := "20261118050000-original"
			target := createReleaseDeployment(t, db, "target", &release)
			deployment := createReleaseDeployment(t, db, "rollback", nil)
			require.NoError(t, db.Model(deployment).Updates(map[string]any{"git_access": snapshot, "status": sitetypes.DeploymentStatusPending}).Error)
			revoke := func() error {
				return db.Model(dependency.Repository).Updates(map[string]any{"selected": false, "grant_version": 2}).Error
			}
			if stage == "before job" {
				require.NoError(t, revoke())
			} else {
				deps.TaskRunnerDeps.Dispatcher = &revokingRepositoryDispatcher{FakeDispatcher: dispatcher, revoke: revoke}
			}
			job := &RollbackJob{Deps: deps, Payload: RollbackPayload{SiteID: site.ID, DeploymentID: deployment.ID, TargetDeploymentID: target.ID}}
			require.Error(t, job.Handle(context.Background()))
			require.Zero(t, dispatcher.ExecutionCount())
		})
	}
}

func TestSelectedSiteDeployJobDeniesRetiredAuthorizationBeforeRemoteExecution(t *testing.T) {
	for _, change := range []string{"reconnect", "deselect", "connection-from-another-account"} {
		t.Run(change, func(t *testing.T) {
			deps, dispatcher, db := deploymentReleaseJobFixture(t)
			grant := githubfixture.SelectedRepository(t, db, "team-1", "org/selected", map[string]any{})
			require.NoError(t, db.Model(&models.Site{}).Where("id = ?", "site-1").Updates(map[string]any{"source_control_id": grant.SourceControl.ID, "source_control_repositories_id": grant.Repository.ID}).Error)
			var site models.Site
			require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
			snapshot, err := siteaccess.Capture(context.Background(), db, nil, &site)
			require.NoError(t, err)
			deployment := createReleaseDeployment(t, db, "git-deployment", nil)
			require.NoError(t, db.Model(deployment).Updates(map[string]any{"git_access": snapshot, "status": sitetypes.DeploymentStatusPending}).Error)
			switch change {
			case "reconnect":
				require.NoError(t, db.Model(grant.SourceControl).Update("generation", 2).Error)
			case "deselect":
				require.NoError(t, db.Model(grant.Repository).Updates(map[string]any{"selected": false, "grant_version": 2}).Error)
			case "connection-from-another-account":
				require.NoError(t, db.Model(grant.SourceControl).Update("team_id", "team-2").Error)
			}
			job := &DeployJob{Deps: deps, Payload: DeployPayload{SiteID: site.ID, DeploymentID: deployment.ID}}
			err = job.Handle(context.Background())
			require.Error(t, err)
			require.Zero(t, dispatcher.ExecutionCount())
			job.Failed(context.Background(), err)
			assertGitAuthorizationDeploymentStatus(t, db, deployment.ID, sitetypes.DeploymentStatusFailed)
		})
	}
}

func TestPostDeploymentJobsRejectRevokedOriginBeforeSideEffects(t *testing.T) {
	for _, kind := range []string{"caddy", "features", "install-queue", "restart-queues"} {
		t.Run(kind, func(t *testing.T) {
			deps, dispatcher, db := deploymentReleaseJobFixture(t)
			grant := githubfixture.SelectedRepository(t, db, "team-1", "org/selected", map[string]any{})
			require.NoError(t, db.Model(&models.Site{}).Where("id = ?", "site-1").Updates(map[string]any{"source_control_id": grant.SourceControl.ID, "source_control_repositories_id": grant.Repository.ID, "type": "laravel"}).Error)
			var site models.Site
			require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
			snapshot, err := siteaccess.Capture(context.Background(), db, nil, &site)
			require.NoError(t, err)
			deployment := createReleaseDeployment(t, db, "post-deployment", nil)
			require.NoError(t, db.Model(deployment).Update("git_access", snapshot).Error)
			createRestartQueue(t, db, "queue-1", true, false)
			require.NoError(t, db.Model(grant.Repository).Updates(map[string]any{"selected": false, "grant_version": 2}).Error)
			switch kind {
			case "caddy":
				job := &InstallCaddyfileJob{Deps: deps, Payload: CaddyfilePayload{SiteID: site.ID, DeploymentID: deployment.ID}}
				err = job.Handle(context.Background())
				require.Error(t, err)
				job.Failed(context.Background(), err)
				var saved models.Site
				require.NoError(t, db.First(&saved, "id = ?", site.ID).Error)
				require.NotNil(t, saved.InstalledAt, "stale first-deploy callback cannot clear a live site's state")
			case "features":
				err = (&AnalyzeLaravelFeaturesJob{Deps: deps, Payload: AnalyzeLaravelFeaturesPayload{SiteID: site.ID, ServerID: site.ServerID, DeploymentID: deployment.ID}}).Handle(context.Background())
			case "install-queue":
				job := &InstallQueueJob{Deps: deps, Payload: InstallQueuePayload{SiteID: site.ID, QueueID: "queue-1", DeploymentID: deployment.ID}}
				err = job.Handle(context.Background())
				require.Error(t, err)
				job.Failed(context.Background(), err)
				var saved models.Queue
				require.NoError(t, db.First(&saved, "id = ?", "queue-1").Error)
				require.NotNil(t, saved.InstalledAt)
			case "restart-queues":
				err = (&RestartAllSiteQueuesJob{Deps: deps, Payload: RestartAllSiteQueuesPayload{SiteID: site.ID, DeploymentID: deployment.ID}}).Handle(context.Background())
			}
			require.Error(t, err)
			require.Zero(t, dispatcher.ExecutionCount())
		})
	}
}

func TestPostDeploymentTaskRechecksBeforeRemoteExecution(t *testing.T) {
	deps, _, db := deploymentReleaseJobFixture(t)
	grant := githubfixture.SelectedRepository(t, db, "team-1", "org/selected", map[string]any{})
	require.NoError(t, db.Model(&models.Site{}).Where("id = ?", "site-1").Updates(map[string]any{"source_control_id": grant.SourceControl.ID, "source_control_repositories_id": grant.Repository.ID}).Error)
	var site models.Site
	require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
	snapshot, err := siteaccess.Capture(context.Background(), db, nil, &site)
	require.NoError(t, err)
	deployment := createReleaseDeployment(t, db, "post-deployment", nil)
	require.NoError(t, db.Model(deployment).Update("git_access", snapshot).Error)
	check, err := newPostDeploymentCheck(context.Background(), deps, &site, deployment.ID)
	require.NoError(t, err)
	task := tasks.RestartAllQueues([]string{"queue-1"})
	require.NoError(t, check.attach(task))
	guarded := task
	require.NoError(t, guarded.BeforeExecution(context.Background()))
	require.NoError(t, db.Model(grant.SourceControl).Update("generation", 2).Error)
	require.Error(t, guarded.BeforeExecution(context.Background()), "revocation after job start must stop the uploaded task")
	manual, err := newPostDeploymentCheck(context.Background(), deps, &site, "")
	require.NoError(t, err)
	require.NoError(t, manual.validate(context.Background()), "manual queue operations retain their existing policy")
}

func TestPostDeploymentEventPreservesDeploymentAuthorization(t *testing.T) {
	queue := &eventTaskQueueStub{}
	listener := &restartQueuesOnDeploymentSucceeded{sites: eventSiteReaderStub{site: &models.Site{AutoRestartQueue: true}}, queue: queue}
	require.NoError(t, listener.Handle(context.Background(), deploymentSucceededEvent(t, "deployment-1")))
	require.Len(t, queue.tasks, 1)
	payload, err := pkgjobs.UnmarshalPayload[RestartAllSiteQueuesPayload](queue.tasks[0])
	require.NoError(t, err)
	require.Equal(t, "deployment-1", payload.DeploymentID)
}

func gitAuthorizationJobQueue(t *testing.T) *queue.Client {
	t.Helper()
	redis := miniredis.RunT(t)
	client := queue.NewClient(config.RedisConfig{Address: redis.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	return client
}

func assertGitAuthorizationDeploymentStatus(t *testing.T, db *gorm.DB, id string, expected sitetypes.DeploymentStatus) {
	t.Helper()
	var deployment models.Deployment
	require.NoError(t, db.First(&deployment, "id = ?", id).Error)
	require.Equal(t, expected, deployment.Status, "deployment %s", id)
}
