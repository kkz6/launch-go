package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/database/serializers"
	"github.com/kkz6/launch-go/internal/modules/git/access"
	"github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/kkz6/launch-go/internal/modules/git/providers"
	sitemodels "github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/queue"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func githubEventFixture(t *testing.T) (*ProcessGitWebhookJob, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	raw, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { raw.Close() })
	require.NoError(t, db.AutoMigrate(&models.SourceControl{}, &models.SourceControlRepository{}, &models.GitHubAuthorization{}, &models.GitHubInstallation{}, &models.GitHubCredentialLease{}, &models.GitHubCleanupManifest{}, &models.GitHubWebhookDelivery{}, &models.GitHubWebhookIntent{}, &sitemodels.Site{}))
	for _, sql := range []string{"CREATE TABLE users (id text PRIMARY KEY, status text)", "INSERT INTO users VALUES ('alice','active'),('bob','active')", "CREATE TABLE teams (id text,user_id text)", "CREATE TABLE team_user (team_id text,user_id text,role text)", "INSERT INTO teams VALUES ('a','alice'),('b','bob')"} {
		require.NoError(t, db.Exec(sql).Error)
	}
	installation := models.GitHubInstallation{Host: "github.com", AppID: "7", InstallationID: "42", Status: "active", Generation: 1, Permissions: `{"contents":"read","deployments":"write"}`}
	installation.ID = "installation"
	require.NoError(t, db.Create(&installation).Error)
	now := time.Now()
	branch := "main"
	for i, team := range []string{"a", "b"} {
		user := []string{"alice", "bob"}[i]
		auth := models.GitHubAuthorization{Host: "github.com", AppID: "7", UserID: user, GitHubUserID: "99", Generation: 1}
		auth.ID = "auth-" + team
		require.NoError(t, db.Create(&auth).Error)
		sc := models.SourceControl{Provider: "github", ProviderID: "42", InstallationID: &installation.InstallationID, Status: "active", Generation: 1, GitHubAuthorizationID: &auth.ID, GitHubInstallationID: &installation.ID}
		sc.ID = "connection-" + team
		sc.TeamID = team
		sc.UserID = user
		require.NoError(t, db.Create(&sc).Error)
		// Both installations can see both repositories; account selection differs.
		for index, id := range []string{"101", "202"} {
			repo := models.SourceControlRepository{SourceControlID: sc.ID, GitHubRepositoryID: id, FullName: "owner/repo-" + id, Name: "repo-" + id, Selected: index == i, Available: true, GrantVersion: 1, VerifiedAt: &now}
			require.NoError(t, db.Create(&repo).Error)
			site := sitemodels.Site{SourceControlID: &sc.ID, SourceControlRepositoriesID: &repo.ID, RepositoryBranch: &branch, AutoDeployment: true, Address: team + id, Type: "static", User: "launcher", Path: "/tmp/test", WebFolder: "public", TLSSetting: "auto"}
			site.ID = "site-" + team + id
			site.TeamID = team
			site.ServerID = "server-" + team
			site.UserID = user
			require.NoError(t, db.Create(&site).Error)
		}
	}
	factory := providers.NewProviderFactory()
	factory.RegisterConfig(providers.GitProviderGitHub, &providers.ProviderConfig{AppID: "7", WebhookSecret: "secret"})
	log := zerolog.Nop()
	return &ProcessGitWebhookJob{Deps: &JobDeps{Deps: &pkgjobs.Deps{DB: db, Logger: &log}, ProviderFactory: factory}}, db
}

func setGitHubEvent(t *testing.T, j *ProcessGitWebhookJob, event, delivery string, data map[string]any) {
	t.Helper()
	b, err := json.Marshal(data)
	require.NoError(t, err)
	j.Payload = ProcessGitWebhookPayload{Provider: "github", Event: event, DeliveryID: delivery, Payload: string(b), Signature: githubJobSignature(b, "secret")}
}
func pushData() map[string]any {
	return map[string]any{"installation": map[string]any{"id": 42}, "repository": map[string]any{"id": 101, "full_name": "owner/repo-101"}, "ref": "refs/heads/main", "head_commit": map[string]any{"id": "abc123", "message": "update"}}
}

func TestGitHubPushOnlyTargetsExplicitAccountGrantAndRetriesOutbox(t *testing.T) {
	j, db := githubEventFixture(t)
	require.NoError(t, db.Model(&sitemodels.Site{}).Where("id = ?", "site-a101").Update("repository_branch", nil).Error)
	setGitHubEvent(t, j, "push", "delivery", pushData())
	// A queue outage does not lose the authorized recipient's durable intent.
	require.Error(t, j.Handle(context.Background()))
	var intents []models.GitHubWebhookIntent
	require.NoError(t, db.Find(&intents).Error)
	require.Len(t, intents, 1)
	require.Equal(t, "site-a101", intents[0].SiteID)
	require.Nil(t, intents[0].DispatchedAt)
	redis := miniredis.RunT(t)
	j.Deps.Queue = queue.NewClient(config.RedisConfig{Address: redis.Addr()})
	t.Cleanup(func() { j.Deps.Queue.Close() })
	require.NoError(t, j.Handle(context.Background()))
	require.NoError(t, j.Handle(context.Background()))
	require.NoError(t, db.Find(&intents).Error)
	require.Len(t, intents, 1)
	require.NotNil(t, intents[0].DispatchedAt)
	queued, err := j.Deps.Queue.Inspector().ListPendingTasks("default")
	require.NoError(t, err)
	require.Len(t, queued, 1)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(queued[0].Payload, &payload))
	require.Equal(t, "site-a101", payload["site_id"])
	require.NotEmpty(t, payload["git_access"])
}

func TestGitHubEventAndActionCannotConfuseRepositoryDeletionWithUninstall(t *testing.T) {
	j, db := githubEventFixture(t)
	data := pushData()
	data["action"] = "deleted"
	setGitHubEvent(t, j, "repository", "delete-repository", data)
	require.NoError(t, j.Handle(context.Background()))
	var installation models.GitHubInstallation
	require.NoError(t, db.First(&installation).Error)
	require.Equal(t, "active", installation.Status)
	var repos []models.SourceControlRepository
	require.NoError(t, db.Where("git_hub_repository_id = ?", "101").Find(&repos).Error)
	for _, repo := range repos {
		require.False(t, repo.Available)
		require.Equal(t, uint64(2), repo.GrantVersion)
	}
	var selected models.SourceControlRepository
	require.NoError(t, db.Where("source_control_id = ? AND git_hub_repository_id = ?", "connection-a", "101").First(&selected).Error)
	require.True(t, selected.Selected, "saved selection is preserved for explicit review")
	setGitHubEvent(t, j, "installation", "forged-header", data)
	require.ErrorContains(t, j.Handle(context.Background()), "unexpected repository")
}

func TestGitHubSuspendRedeliveryFencesAllConnectionsOnlyOnce(t *testing.T) {
	j, db := githubEventFixture(t)
	setGitHubEvent(t, j, "installation", "suspend", map[string]any{"action": "suspend", "installation": map[string]any{"id": 42}})
	require.NoError(t, j.Handle(context.Background()))
	require.NoError(t, j.Handle(context.Background()))
	var connections []models.SourceControl
	require.NoError(t, db.Find(&connections).Error)
	for _, sc := range connections {
		require.Equal(t, uint64(2), sc.Generation)
		require.Equal(t, "reconnect_required", sc.Status)
	}
	var installation models.GitHubInstallation
	require.NoError(t, db.First(&installation).Error)
	require.Equal(t, uint64(2), installation.Generation)
}

func TestGitHubAuthorizationRevocationAffectsMatchingIdentityAcrossLogins(t *testing.T) {
	j, db := githubEventFixture(t)
	setGitHubEvent(t, j, "github_app_authorization", "revoked", map[string]any{"action": "revoked", "sender": map[string]any{"id": 99}})
	require.NoError(t, j.Handle(context.Background()))
	var authorizations []models.GitHubAuthorization
	require.NoError(t, db.Find(&authorizations).Error)
	require.Len(t, authorizations, 2)
	for _, auth := range authorizations {
		require.NotNil(t, auth.RevokedAt)
		require.Equal(t, uint64(2), auth.Generation)
	}
}

func TestGitHubDeliveryCannotBeReusedForAnotherPayload(t *testing.T) {
	j, _ := githubEventFixture(t)
	setGitHubEvent(t, j, "installation_repositories", "same", map[string]any{"action": "added", "installation": map[string]any{"id": 42}})
	require.NoError(t, j.Handle(context.Background()))
	setGitHubEvent(t, j, "installation_repositories", "same", map[string]any{"action": "removed", "installation": map[string]any{"id": 42}})
	require.ErrorContains(t, j.Handle(context.Background()), "reused")
}

func TestGitHubPushDoesNotWaitForAnotherAccountsCredentialRevocation(t *testing.T) {
	j, db := githubEventFixture(t)
	require.NoError(t, serializers.SetEncryptionKey(bytes.Repeat([]byte{42}, 32)))
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)
	j.Deps.ProviderFactory = providers.NewProviderFactory()
	j.Deps.ProviderFactory.RegisterConfig(providers.GitProviderGitHub, &providers.ProviderConfig{AppID: "7", WebhookSecret: "secret", APIURL: server.URL})
	lease := models.GitHubCredentialLease{TeamID: "b", SourceControlID: "connection-b", Status: "revoke_pending", Token: "unrelated-test-token", ExpiresAt: time.Now().Add(time.Hour)}
	require.NoError(t, db.Create(&lease).Error)
	redis := miniredis.RunT(t)
	j.Deps.Queue = queue.NewClient(config.RedisConfig{Address: redis.Addr()})
	t.Cleanup(func() { j.Deps.Queue.Close() })
	setGitHubEvent(t, j, "push", "independent-push", pushData())
	require.NoError(t, j.Handle(context.Background()))
	queued, err := j.Deps.Queue.Inspector().ListPendingTasks("default")
	require.NoError(t, err)
	require.Len(t, queued, 1)
	require.Zero(t, requests.Load(), "push delivery must not wait on another account's token cleanup")
	require.Error(t, access.NewResolver(db, j.Deps.ProviderFactory).RevokePending(context.Background()))
	require.Positive(t, requests.Load())
	require.NoError(t, db.First(&lease, "id = ?", lease.ID).Error)
	require.Equal(t, "revoke_pending", lease.Status, "the recurring reconciler can retry the failed cleanup")
}

func failingGitHubAccount(t *testing.T, j *ProcessGitWebhookJob, db *gorm.DB) *atomic.Int32 {
	t.Helper()
	require.NoError(t, serializers.SetEncryptionKey(bytes.Repeat([]byte{42}, 32)))
	requests := &atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)
	j.Deps.ProviderFactory = providers.NewProviderFactory()
	j.Deps.ProviderFactory.RegisterConfig(providers.GitProviderGitHub, &providers.ProviderConfig{AppID: "7", WebhookSecret: "secret", APIURL: server.URL})
	require.NoError(t, db.Model(&models.GitHubAuthorization{}).Where("id = ?", "auth-a").Updates(map[string]any{"access_token": dbtype.EncryptedString("account-a-test-token"), "expires_at": time.Now().Add(time.Hour)}).Error)
	require.NoError(t, db.Model(&models.SourceControlRepository{}).Where("source_control_id = ?", "connection-a").Update("verified_at", nil).Error)
	return requests
}

func TestGitHubPushFailureInOneAccountDoesNotRollbackOtherAccountDelivery(t *testing.T) {
	j, db := githubEventFixture(t)
	require.NoError(t, db.Model(&models.SourceControlRepository{}).Where("source_control_id = ? AND git_hub_repository_id = ?", "connection-b", "101").Update("selected", true).Error)
	requests := failingGitHubAccount(t, j, db)
	redis := miniredis.RunT(t)
	j.Deps.Queue = queue.NewClient(config.RedisConfig{Address: redis.Addr()})
	t.Cleanup(func() { j.Deps.Queue.Close() })
	setGitHubEvent(t, j, "push", "partial-push", pushData())
	for range 2 {
		require.ErrorContains(t, j.Handle(context.Background()), "status 401")
		var intents []models.GitHubWebhookIntent
		require.NoError(t, db.Find(&intents).Error)
		require.Len(t, intents, 1)
		require.Equal(t, "site-b101", intents[0].SiteID)
		require.NotNil(t, intents[0].DispatchedAt)
		var receipt models.GitHubWebhookDelivery
		require.NoError(t, db.First(&receipt).Error)
		require.Nil(t, receipt.ProcessedAt, "failed account remains retryable")
	}
	require.Positive(t, requests.Load())
	// A's later recovery fills only its missing intent; B is never dispatched twice.
	require.NoError(t, db.Model(&models.SourceControlRepository{}).Where("source_control_id = ?", "connection-a").Update("verified_at", time.Now()).Error)
	require.NoError(t, j.Handle(context.Background()))
	require.NoError(t, j.Handle(context.Background()))
	queued, err := j.Deps.Queue.Inspector().ListPendingTasks("default")
	require.NoError(t, err)
	require.Len(t, queued, 2)
	var receipt models.GitHubWebhookDelivery
	require.NoError(t, db.First(&receipt).Error)
	require.NotNil(t, receipt.ProcessedAt)
}

func TestGitHubPermissionRecoveryIgnoresOtherInstallationsBeforeProviderChecks(t *testing.T) {
	j, db := githubEventFixture(t)
	j.Deps.AppURL = "https://launch.test"
	permissions := `{"contents":"write","workflows":"write","actions":"write","secrets":"write","variables":"write"}`
	require.NoError(t, db.Model(&models.GitHubInstallation{}).Where("id = ?", "installation").Update("permissions", permissions).Error)
	other := models.GitHubInstallation{Host: "github.com", AppID: "7", InstallationID: "84", Status: "active", Generation: 1, Permissions: permissions}
	other.ID = "other-installation"
	require.NoError(t, db.Create(&other).Error)
	require.NoError(t, db.Model(&models.SourceControl{}).Where("id = ?", "connection-a").Updates(map[string]any{"installation_id": "84", "git_hub_installation_id": other.ID}).Error)
	require.NoError(t, db.Exec("CREATE TABLE docker_applications (id TEXT PRIMARY KEY, team_id TEXT, source_config TEXT, gha_deploy_token_hash TEXT, build_location TEXT, deleted_at datetime)").Error)
	for _, team := range []string{"a", "b"} {
		var repo models.SourceControlRepository
		require.NoError(t, db.Where("source_control_id = ? AND selected = ?", "connection-"+team, true).First(&repo).Error)
		grant, err := access.NewResolver(db, j.Deps.ProviderFactory).Resolve(context.Background(), access.Request{TeamID: team, SourceControlID: repo.SourceControlID, RepositoryID: fmt.Sprint(repo.ID), Operation: access.ActionsBootstrap})
		require.NoError(t, err)
		source := dbtype.JSONMap{"source_control_id": repo.SourceControlID, "repository_id": fmt.Sprint(repo.ID), "gha_install_status": "permissions_missing"}
		access.StoreSnapshot(source, grant.Snapshot())
		require.NoError(t, db.Table("docker_applications").Create(map[string]any{"id": "app-" + team, "team_id": team, "source_config": source, "build_location": "github_actions"}).Error)
	}
	requests := failingGitHubAccount(t, j, db)
	require.NoError(t, j.capturePermissionRecovery(context.Background(), db, "permission-receipt", "42"))
	require.Zero(t, requests.Load(), "another installation's unavailable authorization must not be consulted")
	var intents []models.GitHubWebhookIntent
	require.NoError(t, db.Find(&intents).Error)
	require.Len(t, intents, 1)
	require.Equal(t, "app-b", intents[0].Payload.GetString("workload_id"))
	require.Len(t, intents[0].Payload.GetString("binding_id"), 26)
	require.Equal(t, "https://launch.test", intents[0].Payload.GetString("launch_base_url"))
	require.Equal(t, true, intents[0].Payload["recovery"])
}

func TestGitHubLifecycleBeforeFirstConnectionPreservesRevocationEvidence(t *testing.T) {
	for _, action := range []string{"removed", "suspend", "deleted", "unsuspend"} {
		t.Run(action, func(t *testing.T) {
			j, db := githubEventFixture(t)
			event := "installation"
			data := map[string]any{"action": action, "installation": map[string]any{"id": 777}}
			if action == "removed" {
				event = "installation_repositories"
				data["repositories_removed"] = []any{map[string]any{"id": 303}}
			}
			setGitHubEvent(t, j, event, "before-first-connection", data)
			require.NoError(t, j.Handle(context.Background()))
			require.NoError(t, j.Handle(context.Background()))
			var installation models.GitHubInstallation
			require.NoError(t, db.Where("installation_id = ?", "777").First(&installation).Error)
			if action == "removed" {
				require.Equal(t, uint64(2), installation.RepositoryRevision)
				require.Equal(t, uint64(1), installation.Generation)
			} else {
				require.Equal(t, uint64(2), installation.Generation)
				status := map[string]string{"suspend": "suspended", "deleted": "deleted", "unsuspend": "active"}[action]
				require.Equal(t, status, installation.Status)
			}
			var other models.GitHubInstallation
			require.NoError(t, db.First(&other, "id = ?", "installation").Error)
			require.Equal(t, uint64(1), other.Generation)
			require.Equal(t, uint64(1), other.RepositoryRevision)
			var connections int64
			require.NoError(t, db.Model(&models.SourceControl{}).Where("installation_id = ?", "777").Count(&connections).Error)
			require.Zero(t, connections, "a lifecycle receipt never creates local access")
		})
	}
}

func TestGitHubReconcileRestoresAlreadyReconnectedWorkflowsOnce(t *testing.T) {
	j, db := githubEventFixture(t)
	ctx := context.Background()
	j.Deps.AppURL = "https://launch.test"
	require.NoError(t, db.Model(&models.GitHubInstallation{}).Where("id = ?", "installation").Update("permissions", `{"contents":"write","workflows":"write","actions":"write","secrets":"write","variables":"write"}`).Error)
	require.NoError(t, db.Exec("CREATE TABLE docker_applications (id TEXT PRIMARY KEY, team_id TEXT, source_config TEXT, gha_deploy_token_hash TEXT, build_location TEXT, deleted_at datetime)").Error)
	var repo models.SourceControlRepository
	require.NoError(t, db.Where("source_control_id = ? AND selected = ?", "connection-a", true).First(&repo).Error)
	source := dbtype.JSONMap{"source_control_id": "connection-a", "repository_id": fmt.Sprint(repo.ID), "gha_workflow_sha": "old-sha", "gha_install_status": "ok"}
	require.NoError(t, db.Table("docker_applications").Create(map[string]any{"id": "existing-app", "team_id": "a", "source_config": source, "gha_deploy_token_hash": "old-token", "build_location": "github_actions"}).Error)
	reconcile := ReconcileGitHubAccessJob{Deps: j.Deps}
	require.ErrorContains(t, reconcile.Handle(ctx), "queue unavailable")
	var intents []models.GitHubWebhookIntent
	require.NoError(t, db.Find(&intents).Error)
	require.Len(t, intents, 1)
	redis := miniredis.RunT(t)
	j.Deps.Queue = queue.NewClient(config.RedisConfig{Address: redis.Addr()})
	t.Cleanup(func() { _ = j.Deps.Queue.Close() })
	require.NoError(t, reconcile.Handle(ctx))
	queued, err := j.Deps.Queue.Inspector().ListPendingTasks("default")
	require.NoError(t, err)
	require.Len(t, queued, 1)
	source["gha_recovery_status"] = "failed"
	source["gha_recovery_binding_id"] = intents[0].Payload.GetString("binding_id")
	require.NoError(t, db.Table("docker_applications").Where("id = ?", "existing-app").Update("source_config", source).Error)
	require.NoError(t, reconcile.Handle(ctx))
	require.NoError(t, db.Find(&intents).Error)
	require.Len(t, intents, 1, "terminal failure does not create new bindings every minute")
	require.NoError(t, db.Model(&models.SourceControlRepository{}).Where("id = ?", repo.ID).Update("grant_version", gorm.Expr("grant_version + 1")).Error)
	require.NoError(t, reconcile.Handle(ctx))
	require.NoError(t, db.Find(&intents).Error)
	require.Len(t, intents, 2, "a new grant may recover automatically")
}
