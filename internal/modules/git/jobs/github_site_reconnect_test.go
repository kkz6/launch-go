package jobs_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/database/serializers"
	"github.com/kkz6/launch-go/internal/modules/git/access"
	gitjobs "github.com/kkz6/launch-go/internal/modules/git/jobs"
	gitmodels "github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/kkz6/launch-go/internal/modules/git/providers"
	gitrepos "github.com/kkz6/launch-go/internal/modules/git/repositories"
	gitservices "github.com/kkz6/launch-go/internal/modules/git/services"
	siteaccess "github.com/kkz6/launch-go/internal/modules/site/gitaccess"
	sitejobs "github.com/kkz6/launch-go/internal/modules/site/jobs"
	sitemodels "github.com/kkz6/launch-go/internal/modules/site/models"
	siterepos "github.com/kkz6/launch-go/internal/modules/site/repositories"
	siteservices "github.com/kkz6/launch-go/internal/modules/site/services"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/queue"
	"github.com/kkz6/launch-go/internal/pkg/service"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	siteReconnectTeamA = "team-a00000000000000000000"
	siteReconnectTeamB = "team-b00000000000000000000"
	siteReconnectUserA = "user-a00000000000000000000"
	siteReconnectUserB = "user-b00000000000000000000"
)

type siteReconnectFixture struct {
	db            *gorm.DB
	connections   *gitservices.SourceControlService
	deployments   *siteservices.DeploymentService
	webhooks      *gitjobs.ProcessGitWebhookJob
	siteJobDeps   *sitejobs.JobDeps
	factory       *providers.ProviderFactory
	provider      *providers.GitHubProvider
	tokenRequests atomic.Int32
}

func newSiteReconnectFixture(t *testing.T, usePostgres bool) *siteReconnectFixture {
	t.Helper()
	cfg := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent), DisableForeignKeyConstraintWhenMigrating: true}
	var db *gorm.DB
	var err error
	if usePostgres {
		dsn := os.Getenv("LAUNCH_GITHUB_TEST_POSTGRES_DSN")
		if dsn == "" {
			t.Skip("set LAUNCH_GITHUB_TEST_POSTGRES_DSN for PostgreSQL site reconnect integration")
		}
		admin, openErr := gorm.Open(postgres.Open(dsn), cfg)
		require.NoError(t, openErr)
		schema := fmt.Sprintf("site_reconnect_%d", time.Now().UnixNano())
		require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
		t.Cleanup(func() { admin.Exec("DROP SCHEMA " + schema + " CASCADE"); pool, _ := admin.DB(); pool.Close() })
		parsed, parseErr := url.Parse(dsn)
		require.NoError(t, parseErr)
		query := parsed.Query()
		query.Set("search_path", schema)
		query.Set("client_encoding", "UTF8")
		parsed.RawQuery = query.Encode()
		db, err = gorm.Open(postgres.Open(parsed.String()), cfg)
		require.NoError(t, err)
		require.NoError(t, db.Exec("CREATE DOMAIN longtext AS text").Error)
	} else {
		db, err = gorm.Open(sqlite.Open(":memory:"), cfg)
		require.NoError(t, err)
	}
	pool, err := db.DB()
	require.NoError(t, err)
	if !usePostgres {
		pool.SetMaxOpenConns(1)
	}
	t.Cleanup(func() { pool.Close() })
	require.NoError(t, serializers.SetEncryptionKey(bytes.Repeat([]byte{42}, 32)))
	require.NoError(t, db.AutoMigrate(&gitmodels.SourceControl{}, &gitmodels.SourceControlRepository{}, &gitmodels.GitHubAuthorization{}, &gitmodels.GitHubInstallation{}, &gitmodels.GitHubConnectionAttempt{}, &gitmodels.GitHubCredentialLease{}, &gitmodels.GitHubCleanupManifest{}, &gitmodels.GitHubWebhookDelivery{}, &gitmodels.GitHubWebhookIntent{}, &sitemodels.Site{}, &sitemodels.Deployment{}))
	for _, sql := range []string{
		"CREATE UNIQUE INDEX site_reconnect_connection ON source_controls (provider,provider_id,team_id)",
		"CREATE TABLE users (id text primary key,status text)",
		"INSERT INTO users VALUES ('user-a00000000000000000000','active'),('user-b00000000000000000000','active')",
		"CREATE TABLE teams (id text primary key,user_id text)",
		"INSERT INTO teams VALUES ('team-a00000000000000000000','user-a00000000000000000000'),('team-b00000000000000000000','user-b00000000000000000000')",
		"CREATE TABLE team_user (team_id text,user_id text,role text)",
		"CREATE TABLE team_invitations (team_id text,email text,role text)",
	} {
		require.NoError(t, db.Exec(sql).Error)
	}
	f := &siteReconnectFixture{db: db}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/login/oauth/access_token":
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "user-token", "refresh_token": "refresh-token", "expires_in": 3600, "refresh_token_expires_in": 86400})
		case r.URL.Path == "/user":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 99, "login": "sponsor"})
		case r.URL.Path == "/user/installations":
			_ = json.NewEncoder(w).Encode(map[string]any{"installations": []any{map[string]any{"id": 42, "app_id": 7, "account": map[string]any{"id": 10, "login": "org", "type": "Organization"}}}})
		case r.URL.Path == "/user/installations/42/repositories":
			repos := []any{}
			for _, id := range []int{101, 202, 303} {
				repos = append(repos, map[string]any{"id": id, "name": fmt.Sprint("repo", id), "full_name": fmt.Sprint("org/repo", id), "default_branch": "main", "private": true})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"repositories": repos})
		case strings.HasSuffix(r.URL.Path, "/collaborators/sponsor/permission"):
			_ = json.NewEncoder(w).Encode(map[string]any{"permission": "admin"})
		case r.URL.Path == "/app/installations/42":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 42, "app_id": 7, "account": map[string]any{"id": 10, "login": "org", "type": "Organization"}, "permissions": map[string]any{"contents": "read", "deployments": "write"}, "repository_selection": "selected"})
		case r.URL.Path == "/app/installations/42/access_tokens":
			f.tokenRequests.Add(1)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"token": "selected-token", "expires_at": time.Now().Add(time.Hour).UTC().Format(time.RFC3339)})
		case r.URL.Path == "/installation/token":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "unexpected fake endpoint "+r.URL.Path, http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	f.factory = providers.NewProviderFactory()
	f.factory.RegisterConfig(providers.GitProviderGitHub, &providers.ProviderConfig{AppID: "7", AppSlug: "launch-test", PrivateKey: string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})), ClientID: "client", ClientSecret: "secret", CallbackURL: "https://launch.test/callback", APIURL: server.URL, BaseURL: server.URL, WebhookSecret: "webhook-secret"})
	provider, err := f.factory.GetProvider(providers.GitProviderGitHub)
	require.NoError(t, err)
	f.provider = provider.(*providers.GitHubProvider)
	log := zerolog.Nop()
	redis := miniredis.RunT(t)
	q := queue.NewClient(config.RedisConfig{Address: redis.Addr()})
	t.Cleanup(func() { require.NoError(t, q.Close()) })
	deps := service.NewDependencies(db, &log, q, nil, nil)
	f.connections = gitservices.NewSourceControlService(&gitservices.ServiceDeps{ModuleDeps: service.ModuleDeps[*gitrepos.Registry]{Dependencies: deps, Repos: gitrepos.NewRegistry(db)}, ProviderFactory: f.factory})
	f.deployments = siteservices.NewDeploymentService(&siteservices.ServiceDeps{ModuleDeps: service.ModuleDeps[*siterepos.Registry]{Dependencies: deps, Repos: siterepos.NewRegistry(db)}})
	f.deployments.SetProviderFactory(f.factory)
	jobDeps := &pkgjobs.Deps{DB: db, Logger: &log, Queue: q}
	f.webhooks = &gitjobs.ProcessGitWebhookJob{Deps: &gitjobs.JobDeps{Deps: jobDeps, Service: f.connections, ProviderFactory: f.factory}}
	f.siteJobDeps = &sitejobs.JobDeps{Deps: jobDeps, Repos: siterepos.NewRegistry(db), ProviderFactory: f.factory}
	return f
}

func (f *siteReconnectFixture) connect(t *testing.T, team, user string, repos ...string) string {
	t.Helper()
	ctx := context.Background()
	attempt, err := f.connections.BeginGitHub(ctx, team, user, "session-"+user, gitservices.BeginGitHubRequest{})
	require.NoError(t, err)
	u, err := url.Parse(attempt.URL)
	require.NoError(t, err)
	_, err = f.connections.CompleteGitHubCallback(ctx, team, user, "session-"+user, u.Query().Get("state"), "code", "")
	require.NoError(t, err)
	review, err := f.connections.GitHubCandidates(ctx, attempt.AttemptID, team, user, "session-"+user, "42")
	require.NoError(t, err)
	sourceID, err := json.Marshal(review.SourceControlID)
	require.NoError(t, err)
	result, err := f.connections.SelectGitHubRepositories(ctx, attempt.AttemptID, team, user, "session-"+user, gitservices.GitHubSelectionRequest{SourceControlID: sourceID, InstallationID: "42", RepositoryIDs: repos, ExpectedGeneration: &review.ConnectionGeneration, ExpectedSelectionVersion: &review.SelectionVersion})
	require.NoError(t, err)
	return result.ID
}

func (f *siteReconnectFixture) site(t *testing.T, team, user, connection string, dependencies ...string) sitemodels.Site {
	t.Helper()
	var repo gitmodels.SourceControlRepository
	require.NoError(t, f.db.Where("source_control_id = ? AND git_hub_repository_id = ?", connection, "101").First(&repo).Error)
	branch, installed := "main", time.Now()
	site := sitemodels.Site{SourceControlID: &connection, SourceControlRepositoriesID: &repo.ID, RepositoryBranch: &branch, AutoDeployment: true, Address: team + ".example.test", Type: sitetypes.SiteTypeStatic, User: "launcher", Path: "/home/launcher/" + team, WebFolder: "public", TLSSetting: sitetypes.TLSSettingAuto, ZeroDowntimeDeployment: true, QueueDeployments: true}
	site.TeamID, site.UserID, site.ServerID, site.InstalledAt = team, user, strings.Replace(team, "team-", "host-", 1), &installed
	for _, remoteID := range dependencies {
		var dependency gitmodels.SourceControlRepository
		require.NoError(t, f.db.Where("source_control_id = ? AND git_hub_repository_id = ?", connection, remoteID).First(&dependency).Error)
		site.RepositoryDependencyIDs = append(site.RepositoryDependencyIDs, strconv.FormatUint(dependency.ID, 10))
	}
	require.NoError(t, f.db.Create(&site).Error)
	var stored sitemodels.Site
	require.NoError(t, f.db.First(&stored, "id = ?", site.ID).Error)
	return stored
}

func (f *siteReconnectFixture) push(t *testing.T, delivery string) map[string]sitejobs.CreateDeploymentPayload {
	t.Helper()
	data, err := json.Marshal(map[string]any{"installation": map[string]any{"id": 42}, "repository": map[string]any{"id": 101, "full_name": "org/repo101"}, "ref": "refs/heads/main", "head_commit": map[string]any{"id": "commit-" + delivery, "message": "update"}})
	require.NoError(t, err)
	mac := hmac.New(sha256.New, []byte("webhook-secret"))
	_, err = mac.Write(data)
	require.NoError(t, err)
	f.webhooks.Payload = gitjobs.ProcessGitWebhookPayload{Provider: "github", Event: "push", DeliveryID: delivery, Payload: string(data), Signature: "sha256=" + hex.EncodeToString(mac.Sum(nil))}
	require.NoError(t, f.webhooks.Handle(context.Background()))
	var intents []gitmodels.GitHubWebhookIntent
	require.NoError(t, f.db.Find(&intents).Error)
	result := map[string]sitejobs.CreateDeploymentPayload{}
	for _, intent := range intents {
		encoded, err := json.Marshal(intent.Payload)
		require.NoError(t, err)
		var payload sitejobs.CreateDeploymentPayload
		require.NoError(t, json.Unmarshal(encoded, &payload))
		if payload.CommitData["sha"] == "commit-"+delivery {
			require.NotNil(t, intent.DispatchedAt)
			result[intent.SiteID] = payload
		}
	}
	return result
}

func TestExistingSitesRestoreDeploymentsAfterGitHubReconnect(t *testing.T) {
	for _, postgresDB := range []bool{false, true} {
		for _, previous := range []string{"disconnected", "legacy"} {
			t.Run(fmt.Sprintf("postgres=%t/%s", postgresDB, previous), func(t *testing.T) {
				f := newSiteReconnectFixture(t, postgresDB)
				ctx := context.Background()
				connectionA := f.connect(t, siteReconnectTeamA, siteReconnectUserA, "101", "303")
				connectionB := f.connect(t, siteReconnectTeamB, siteReconnectUserB, "101")
				siteA, siteB := f.site(t, siteReconnectTeamA, siteReconnectUserA, connectionA, "303"), f.site(t, siteReconnectTeamB, siteReconnectUserB, connectionB)
				var beforeB gitmodels.SourceControl
				require.NoError(t, f.db.First(&beforeB, "id = ?", connectionB).Error)
				oldPush := f.push(t, "before-reconnect")[siteA.ID]
				release := "running-release"
				live := sitemodels.Deployment{Status: sitetypes.DeploymentStatusFinished, ReleaseName: &release}
				live.SiteID, live.TeamID = siteA.ID, siteA.TeamID
				require.NoError(t, f.db.Create(&live).Error)
				var liveBefore sitemodels.Deployment
				require.NoError(t, f.db.First(&liveBefore, "id = ?", live.ID).Error)
				oldAuthorization, err := siteaccess.Capture(ctx, f.db, f.factory, &siteA)
				require.NoError(t, err)
				oldDeployment := &sitemodels.Deployment{Status: sitetypes.DeploymentStatusQueued, GitAccess: oldAuthorization}
				oldDeployment.SiteID, oldDeployment.TeamID = siteA.ID, siteA.TeamID
				require.NoError(t, f.db.Create(oldDeployment).Error)
				oldGrant, err := siteaccess.ForDeployment(ctx, f.db, f.factory, &siteA, oldDeployment, access.Clone)
				require.NoError(t, err)
				if previous == "disconnected" {
					require.NoError(t, access.NewResolver(f.db, f.factory).Disconnect(ctx, connectionA, siteReconnectTeamA))
				} else {
					// Migration leaves the same legacy rows and site references, but no grants.
					require.NoError(t, f.db.Model(&gitmodels.SourceControl{}).Where("id = ?", connectionA).Updates(map[string]any{"status": "reconnect_required", "git_hub_authorization_id": nil, "git_hub_installation_id": nil}).Error)
					require.NoError(t, f.db.Model(&gitmodels.SourceControlRepository{}).Where("source_control_id = ?", connectionA).Updates(map[string]any{"selected": false, "available": false, "verified_at": nil, "git_hub_repository_id": ""}).Error)
				}
				_, err = f.deployments.Deploy(ctx, siteA.ID, siteA.ServerID, siteA.TeamID, siteA.UserID)
				require.Error(t, err)
				_, err = f.provider.GetInstallationToken(oldGrant.Context(ctx), "42")
				require.Error(t, err)
				require.Zero(t, f.tokenRequests.Load(), "inactive grants cannot request GitHub credentials")
				blocked := f.push(t, "blocked")
				require.NotContains(t, blocked, siteA.ID)
				require.Contains(t, blocked, siteB.ID, "the other account continues receiving pushes")
				require.Equal(t, connectionA, f.connect(t, siteReconnectTeamA, siteReconnectUserA, "101", "303"))
				var restored sitemodels.Site
				require.NoError(t, f.db.First(&restored, "id = ?", siteA.ID).Error)
				require.Equal(t, siteA, restored, "reconnect preserves site configuration, installed status and repository IDs")
				var liveAfter sitemodels.Deployment
				require.NoError(t, f.db.First(&liveAfter, "id = ?", live.ID).Error)
				require.Equal(t, liveBefore, liveAfter, "reconnect must not delete or change the running release record")
				require.Error(t, (&sitejobs.CreateDeploymentJob{Deps: f.siteJobDeps, Payload: oldPush}).Handle(ctx), "old queued push authorization must stay stale")
				manual, err := f.deployments.Deploy(ctx, siteA.ID, siteA.ServerID, siteA.TeamID, siteA.UserID)
				require.NoError(t, err)
				grant, err := siteaccess.ForDeployment(ctx, f.db, f.factory, &restored, manual, access.Clone)
				require.NoError(t, err)
				_, err = f.provider.GetInstallationToken(grant.Context(ctx), "42")
				require.NoError(t, err)
				require.EqualValues(t, 1, f.tokenRequests.Load())
				_, err = siteaccess.ForDeployment(ctx, f.db, f.factory, &restored, oldDeployment, access.Clone)
				require.Error(t, err, "restoring fresh deployments must not revive previously queued authorization")
				_, err = f.provider.GetInstallationToken(oldGrant.Context(ctx), "42")
				require.Error(t, err)
				require.EqualValues(t, 1, f.tokenRequests.Load())
				payloads := f.push(t, "restored")
				require.Len(t, payloads, 2)
				for _, site := range []sitemodels.Site{siteA, siteB} {
					payload := payloads[site.ID]
					job := &sitejobs.CreateDeploymentJob{Deps: f.siteJobDeps, Payload: payload}
					require.NoError(t, job.Handle(ctx))
					var auto sitemodels.Deployment
					require.NoError(t, f.db.Where("webhook_key = ?", payload.WebhookKey).First(&auto).Error)
					_, err = siteaccess.ForDeployment(ctx, f.db, f.factory, &site, &auto, access.Clone)
					require.NoError(t, err)
				}
				var afterB gitmodels.SourceControl
				require.NoError(t, f.db.First(&afterB, "id = ?", connectionB).Error)
				require.Equal(t, beforeB, afterB, "reconnecting account A must not alter account B's connection")
				var preservedB sitemodels.Site
				require.NoError(t, f.db.First(&preservedB, "id = ?", siteB.ID).Error)
				require.Equal(t, siteB, preservedB)
			})
		}
	}
}

func TestSiteReconnectDoesNotGrantUnselectedRepositoryOrDependency(t *testing.T) {
	for _, selected := range [][]string{{"303"}, {"101"}} {
		t.Run(strings.Join(selected, ","), func(t *testing.T) {
			f := newSiteReconnectFixture(t, false)
			ctx := context.Background()
			connection := f.connect(t, siteReconnectTeamA, siteReconnectUserA, "101", "303")
			site := f.site(t, siteReconnectTeamA, siteReconnectUserA, connection, "303")
			otherConnection := f.connect(t, siteReconnectTeamB, siteReconnectUserB, "101")
			otherSite := f.site(t, siteReconnectTeamB, siteReconnectUserB, otherConnection)
			require.NoError(t, access.NewResolver(f.db, f.factory).Disconnect(ctx, connection, siteReconnectTeamA))
			require.Equal(t, connection, f.connect(t, siteReconnectTeamA, siteReconnectUserA, selected...))
			_, err := f.deployments.Deploy(ctx, site.ID, site.ServerID, site.TeamID, site.UserID)
			require.Error(t, err, "both the main repository and declared dependency must be selected")
			payloads := f.push(t, "incomplete")
			require.NotContains(t, payloads, site.ID)
			require.Contains(t, payloads, otherSite.ID)
			var count int64
			require.NoError(t, f.db.Model(&sitemodels.Deployment{}).Where("site_id = ?", site.ID).Count(&count).Error)
			require.Zero(t, count)
			var unchanged sitemodels.Site
			require.NoError(t, f.db.First(&unchanged, "id = ?", site.ID).Error)
			require.Equal(t, site, unchanged)
			require.Zero(t, f.tokenRequests.Load())
		})
	}
}
