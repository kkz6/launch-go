package services

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kkz6/launch-go/internal/database/serializers"
	"github.com/kkz6/launch-go/internal/modules/git/access"
	"github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/kkz6/launch-go/internal/modules/git/providers"
	"github.com/kkz6/launch-go/internal/modules/git/repositories"
	"github.com/kkz6/launch-go/internal/pkg/service"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type gitFixture struct {
	db                 *gorm.DB
	service            *SourceControlService
	provider           *providers.GitHubProvider
	server             *httptest.Server
	mu                 sync.Mutex
	minted             []map[string]any
	revoked            int
	beforeToken        func()
	beforeInstallation func()
	revokeFailToken    string
	admin              bool
	repoIDs            []string
	otherInstallation  bool
	githubUserID       int64
	tokenRequests      int
}

func newGitFixture(t *testing.T) *gitFixture { return newGitFixtureDatabase(t, false) }
func newGitFixtureDatabase(t *testing.T, usePostgres bool) *gitFixture {
	t.Helper()
	require.NoError(t, serializers.SetEncryptionKey(bytes.Repeat([]byte{42}, 32)))
	var db *gorm.DB
	var err error
	config := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	if usePostgres {
		dsn := os.Getenv("LAUNCH_GITHUB_TEST_POSTGRES_DSN")
		if dsn == "" {
			t.Skip("set LAUNCH_GITHUB_TEST_POSTGRES_DSN for PostgreSQL concurrency tests")
		}
		admin, e := gorm.Open(postgres.Open(dsn), config)
		require.NoError(t, e)
		schema := fmt.Sprintf("git_core_%d", time.Now().UnixNano())
		require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
		t.Cleanup(func() { admin.Exec("DROP SCHEMA " + schema + " CASCADE"); pool, _ := admin.DB(); pool.Close() })
		parsed, e := url.Parse(dsn)
		require.NoError(t, e)
		query := parsed.Query()
		query.Set("search_path", schema)
		query.Set("client_encoding", "UTF8")
		parsed.RawQuery = query.Encode()
		db, err = gorm.Open(postgres.Open(parsed.String()), config)
	} else {
		db, err = gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), config)
	}
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	if !usePostgres {
		sqlDB.SetMaxOpenConns(1)
	}
	t.Cleanup(func() { sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&models.SourceControl{}, &models.SourceControlRepository{}, &models.GitHubAuthorization{}, &models.GitHubInstallation{}, &models.GitHubInstallationRepository{}, &models.GitHubConnectionAttempt{}, &models.GitHubCredentialLease{}, &models.GitHubCleanupManifest{}))
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX idx_source_controls_provider_providerid_teamid ON source_controls (provider,provider_id,team_id)").Error)
	for _, sql := range []string{"CREATE TABLE users (id text primary key,status text)", "INSERT INTO users VALUES ('user-a00000000000000000000','active'),('user-b00000000000000000000','active')", "CREATE TABLE teams (id text primary key, user_id text)", "CREATE TABLE team_user (team_id text,user_id text,role text)", "CREATE TABLE team_invitations (team_id text,email text,role text)", "INSERT INTO teams VALUES ('team-a00000000000000000000','user-a00000000000000000000'),('team-b00000000000000000000','user-b00000000000000000000')"} {
		require.NoError(t, db.Exec(sql).Error)
	}
	f := &gitFixture{db: db, admin: true, githubUserID: 99, repoIDs: []string{"101", "202", "303"}}
	f.server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.server.Close)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	privateKey := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	factory := providers.NewProviderFactory()
	factory.RegisterConfig(providers.GitProviderGitHub, &providers.ProviderConfig{AppID: "7", AppSlug: "launch-test", PrivateKey: string(privateKey), ClientID: "client", ClientSecret: "secret", CallbackURL: "https://launch.test/callback", APIURL: f.server.URL, BaseURL: f.server.URL})
	p, err := factory.GetProvider(providers.GitProviderGitHub)
	require.NoError(t, err)
	f.provider = p.(*providers.GitHubProvider)
	log := zerolog.Nop()
	deps := &ServiceDeps{ModuleDeps: service.ModuleDeps[*repositories.Registry]{Dependencies: service.NewDependencies(db, &log, nil, nil, nil), Repos: repositories.NewRegistry(db)}, ProviderFactory: factory}
	f.service = NewSourceControlService(deps)
	return f
}
func (f *gitFixture) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.URL.Path == "/login/oauth/access_token":
		f.mu.Lock()
		f.tokenRequests++
		f.mu.Unlock()
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["grant_type"] != "refresh_token" && len(body["code_verifier"]) < 43 {
			http.Error(w, "missing PKCE", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "user-token", "refresh_token": "refresh-token", "expires_in": 3600, "refresh_token_expires_in": 86400})
	case r.URL.Path == "/user":
		_ = json.NewEncoder(w).Encode(map[string]any{"id": f.githubUserID, "login": "sponsor"})
	case r.URL.Path == "/user/installations":
		installations := []any{map[string]any{"id": 42, "app_id": 7, "account": map[string]any{"id": 10, "login": "org", "type": "Organization"}}}
		if f.otherInstallation {
			installations = append(installations, map[string]any{"id": 43, "app_id": 7, "account": map[string]any{"id": 11, "login": "other", "type": "Organization"}})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"installations": installations})
	case r.URL.Path == "/user/installations/42/repositories" || (f.otherInstallation && r.URL.Path == "/user/installations/43/repositories"):
		repos := []any{}
		repoIDs, owner := f.repoIDs, "org"
		if r.URL.Path == "/user/installations/43/repositories" {
			repoIDs, owner = []string{"404", "505"}, "other"
		}
		for _, id := range repoIDs {
			n, _ := strconv.Atoi(id)
			repos = append(repos, map[string]any{"id": n, "name": "repo" + id, "full_name": owner + "/repo" + id, "default_branch": "main", "private": true})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"repositories": repos})
	case strings.HasSuffix(r.URL.Path, "/collaborators/sponsor/permission"):
		permission := "read"
		if f.admin {
			permission = "admin"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"permission": permission})
	case r.URL.Path == "/app/installations/42" || (f.otherInstallation && r.URL.Path == "/app/installations/43"):
		f.mu.Lock()
		hook := f.beforeInstallation
		f.beforeInstallation = nil
		f.mu.Unlock()
		if hook != nil {
			hook()
		}
		installationID, accountID, login := 42, 10, "org"
		if r.URL.Path == "/app/installations/43" {
			installationID, accountID, login = 43, 11, "other"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": installationID, "app_id": 7, "account": map[string]any{"id": accountID, "login": login, "type": "Organization"}, "permissions": map[string]any{"contents": "write", "deployments": "write", "actions": "write", "workflows": "write", "secrets": "write", "variables": "write"}, "repository_selection": "selected"})
	case r.URL.Path == "/app/installations/42/access_tokens":
		var scope map[string]any
		_ = json.NewDecoder(r.Body).Decode(&scope)
		f.mu.Lock()
		f.minted = append(f.minted, scope)
		hook := f.beforeToken
		f.beforeToken = nil
		f.mu.Unlock()
		if hook != nil {
			hook()
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "installation-token", "expires_at": time.Now().Add(time.Hour).UTC().Format(time.RFC3339)})
	case r.URL.Path == "/installation/token":
		f.mu.Lock()
		f.revoked++
		f.mu.Unlock()
		if f.revokeFailToken != "" && r.Header.Get("Authorization") == "Bearer "+f.revokeFailToken {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "unknown fake endpoint "+r.URL.Path, 404)
	}
}
func (f *gitFixture) authorize(t *testing.T, team, user, session, connection string) GitHubAttemptResponse {
	t.Helper()
	ctx := context.Background()
	a, err := f.service.BeginGitHub(ctx, team, user, session, BeginGitHubRequest{SourceControlID: connection})
	require.NoError(t, err)
	u, err := url.Parse(a.URL)
	require.NoError(t, err)
	require.Equal(t, "S256", u.Query().Get("code_challenge_method"))
	require.Equal(t, "offline_access", u.Query().Get("scope"))
	require.NotEmpty(t, u.Query().Get("code_challenge"))
	verified, err := f.service.CompleteGitHubCallback(ctx, team, user, session, u.Query().Get("state"), "code", "")
	require.NoError(t, err)
	require.Equal(t, a.AttemptID, verified.AttemptID)
	return verified
}
func (f *gitFixture) selectRepos(t *testing.T, a GitHubAttemptResponse, user, session string, ids ...string) dtoResult {
	t.Helper()
	req := GitHubSelectionRequest{InstallationID: "42", RepositoryIDs: ids}
	if a.SourceControlID != nil {
		req.ExpectedGeneration = &a.ConnectionGeneration
		req.ExpectedSelectionVersion = &a.SelectionVersion
	}
	sc, err := f.service.SelectGitHubRepositories(context.Background(), a.AttemptID, a.TeamID, user, session, req)
	require.NoError(t, err)
	return dtoResult{id: sc.ID, generation: sc.Generation}
}

type dtoResult struct {
	id         string
	generation uint64
}

func TestGitHubTwoAccountsSelectOnlyTheirRepositories(t *testing.T) {
	f := newGitFixture(t)
	ctx := context.Background()
	a := f.authorize(t, "team-a00000000000000000000", "user-a00000000000000000000", "session-a", "")
	candidates, err := f.service.GitHubCandidates(ctx, a.AttemptID, "team-a00000000000000000000", "user-a00000000000000000000", "session-a", "42")
	require.NoError(t, err)
	require.Len(t, candidates.Repositories, 3)
	var count int64
	require.NoError(t, f.db.Model(&models.SourceControlRepository{}).Count(&count).Error)
	require.Zero(t, count, "listing candidates must not grant access")
	first := f.selectRepos(t, a, "user-a00000000000000000000", "session-a", "101")
	b := f.authorize(t, "team-b00000000000000000000", "user-b00000000000000000000", "session-b", "")
	second := f.selectRepos(t, b, "user-b00000000000000000000", "session-b", "202")
	r := access.NewResolver(f.db, f.service.ProviderFactory())
	grant, err := r.Resolve(ctx, access.Request{TeamID: "team-a00000000000000000000", SourceControlID: first.id, Repository: "org/repo101", Operation: access.Clone})
	require.NoError(t, err)
	token, err := f.provider.GetInstallationToken(grant.Context(ctx), "42")
	require.NoError(t, err)
	require.Equal(t, "installation-token", token)
	require.Equal(t, []any{float64(101)}, f.minted[0]["repository_ids"])
	require.Equal(t, map[string]any{"contents": "read"}, f.minted[0]["permissions"])
	for _, req := range []access.Request{{TeamID: "team-b00000000000000000000", SourceControlID: first.id, Repository: "org/repo101", Operation: access.Clone}, {TeamID: "team-a00000000000000000000", SourceControlID: first.id, Repository: "org/repo202", Operation: access.Clone}, {TeamID: "team-b00000000000000000000", SourceControlID: second.id, Repository: "org/repo101", Operation: access.Clone}} {
		_, err = r.Resolve(ctx, req)
		require.Error(t, err)
	}
	_, err = f.provider.GetInstallationToken(ctx, "42")
	require.Error(t, err)
	repos, err := f.service.GetRepositoriesBySourceControlID(ctx, first.id, "team-a00000000000000000000")
	require.NoError(t, err)
	require.Len(t, repos, 1)
	require.Equal(t, "101", repos[0].GitHubRepositoryID)
	// Independent authorization permits the same repository only after explicit selection.
	review := f.authorize(t, "team-b00000000000000000000", "user-b00000000000000000000", "session-b", second.id)
	f.selectRepos(t, review, "user-b00000000000000000000", "session-b", "101", "202")
	_, err = r.Resolve(ctx, access.Request{TeamID: "team-b00000000000000000000", SourceControlID: second.id, Repository: "org/repo101", Operation: access.Clone})
	require.NoError(t, err)
}
func TestGitHubOAuthBindingReplayAndMembership(t *testing.T) {
	f := newGitFixture(t)
	ctx := context.Background()
	a, err := f.service.BeginGitHub(ctx, "team-a00000000000000000000", "user-a00000000000000000000", "session-a", BeginGitHubRequest{})
	require.NoError(t, err)
	u, _ := url.Parse(a.URL)
	state := u.Query().Get("state")
	for _, x := range [][3]string{{"team-b00000000000000000000", "user-b00000000000000000000", "session-b"}, {"team-a00000000000000000000", "user-a00000000000000000000", "other-session"}, {"team-b00000000000000000000", "user-a00000000000000000000", "session-a"}} {
		_, err = f.service.CompleteGitHubCallback(ctx, x[0], x[1], x[2], state, "code", "")
		require.Error(t, err)
	}
	require.Zero(t, f.tokenRequests)
	_, err = f.service.CompleteGitHubCallback(ctx, "team-a00000000000000000000", "user-a00000000000000000000", "session-a", state, "code", "")
	require.NoError(t, err)
	_, err = f.service.CompleteGitHubCallback(ctx, "team-a00000000000000000000", "user-a00000000000000000000", "session-a", state, "code", "")
	require.Error(t, err)
	require.Equal(t, 1, f.tokenRequests)
	require.NoError(t, f.db.Exec("INSERT INTO team_invitations VALUES ('team-a00000000000000000000','new@example.test','member')").Error)
	_, err = f.service.GitHubAttempt(ctx, a.AttemptID, "team-a00000000000000000000", "user-a00000000000000000000", "session-a")
	require.Error(t, err)
	_, err = f.service.BeginGitHub(ctx, "team-a00000000000000000000", "user-a00000000000000000000", "session-a", BeginGitHubRequest{})
	require.ErrorContains(t, err, "shared_workspace_confirmation_required")
	_, err = f.service.BeginGitHub(ctx, "team-a00000000000000000000", "user-a00000000000000000000", "session-a", BeginGitHubRequest{AcknowledgeShared: true})
	require.NoError(t, err)
}
func TestGitHubZeroSelectionInventoryAndRemovalDoNotGrant(t *testing.T) {
	f := newGitFixture(t)
	ctx := context.Background()
	a := f.authorize(t, "team-a00000000000000000000", "user-a00000000000000000000", "session-a", "")
	connection := f.selectRepos(t, a, "user-a00000000000000000000", "session-a")
	var sc models.SourceControl
	require.NoError(t, f.db.First(&sc, "id = ?", connection.id).Error)
	inventory := []map[string]any{{"id": json.Number("101"), "name": "repo101", "full_name": "org/repo101", "private": true}}
	require.NoError(t, f.service.SyncInstallationRepositories(ctx, &sc, inventory))
	var count int64
	require.NoError(t, f.db.Model(&models.SourceControlRepository{}).Count(&count).Error)
	require.Zero(t, count)
	a = f.authorize(t, "team-a00000000000000000000", "user-a00000000000000000000", "session-a", connection.id)
	f.selectRepos(t, a, "user-a00000000000000000000", "session-a", "101")
	require.NoError(t, f.db.First(&sc, "id = ?", connection.id).Error)
	require.NoError(t, f.service.SyncInstallationRepositories(ctx, &sc, nil))
	require.NoError(t, f.service.SyncInstallationRepositories(ctx, &sc, inventory))
	var repo models.SourceControlRepository
	require.NoError(t, f.db.Where("source_control_id = ?", sc.ID).First(&repo).Error)
	require.True(t, repo.Selected, "saved selection survives upstream unavailability")
	require.False(t, repo.Available)
	_, err := access.NewResolver(f.db, f.service.ProviderFactory()).Resolve(ctx, access.Request{TeamID: "team-a00000000000000000000", SourceControlID: sc.ID, Repository: "org/repo101", Operation: access.Clone})
	require.Error(t, err)
}
func TestGitHubRevocationDuringTokenIssuanceNeverReturnsCredential(t *testing.T) {
	f := newGitFixture(t)
	ctx := context.Background()
	a := f.authorize(t, "team-a00000000000000000000", "user-a00000000000000000000", "session-a", "")
	sc := f.selectRepos(t, a, "user-a00000000000000000000", "session-a", "101")
	resolver := access.NewResolver(f.db, f.service.ProviderFactory())
	grant, err := resolver.Resolve(ctx, access.Request{TeamID: "team-a00000000000000000000", SourceControlID: sc.id, Repository: "org/repo101", Operation: access.Clone})
	require.NoError(t, err)
	f.beforeToken = func() { require.NoError(t, resolver.Disconnect(ctx, sc.id, "team-a00000000000000000000")) }
	token, err := f.provider.GetInstallationToken(grant.Context(ctx), "42")
	require.Error(t, err)
	require.Empty(t, token)
	require.Equal(t, 1, f.revoked)
	require.Error(t, access.ValidateSnapshot(ctx, f.db, "team-a00000000000000000000", grant.Snapshot()))
	require.NoError(t, resolver.RevokePending(ctx))
	var row models.SourceControl
	require.NoError(t, f.db.First(&row, "id = ?", sc.id).Error)
	require.Equal(t, "disconnected", row.Status)
}
func TestGitHubStaleSelectionAndSponsorLoss(t *testing.T) {
	f := newGitFixture(t)
	ctx := context.Background()
	a := f.authorize(t, "team-a00000000000000000000", "user-a00000000000000000000", "session-a", "")
	sc := f.selectRepos(t, a, "user-a00000000000000000000", "session-a", "101")
	one := f.authorize(t, "team-a00000000000000000000", "user-a00000000000000000000", "session-a", sc.id)
	two := f.authorize(t, "team-a00000000000000000000", "user-a00000000000000000000", "session-a", sc.id)
	f.selectRepos(t, one, "user-a00000000000000000000", "session-a", "202")
	_, err := f.service.SelectGitHubRepositories(ctx, two.AttemptID, "team-a00000000000000000000", "user-a00000000000000000000", "session-a", GitHubSelectionRequest{InstallationID: "42", RepositoryIDs: []string{"101"}, ExpectedGeneration: &two.ConnectionGeneration, ExpectedSelectionVersion: &two.SelectionVersion})
	require.Error(t, err)
	past := time.Now().Add(-6 * time.Minute)
	require.NoError(t, f.db.Model(&models.SourceControlRepository{}).Where("source_control_id = ?", sc.id).Update("verified_at", past).Error)
	f.admin = false
	_, err = access.NewResolver(f.db, f.service.ProviderFactory()).Resolve(ctx, access.Request{TeamID: "team-a00000000000000000000", SourceControlID: sc.id, Repository: "org/repo202", Operation: access.Clone})
	require.Error(t, err)
}
func TestGitHubUnknownInstallationAndRepositoryRejected(t *testing.T) {
	f := newGitFixture(t)
	a := f.authorize(t, "team-a00000000000000000000", "user-a00000000000000000000", "session-a", "")
	for _, req := range []GitHubSelectionRequest{{InstallationID: "77", RepositoryIDs: []string{"101"}}, {InstallationID: "42", RepositoryIDs: []string{"999"}}, {InstallationID: "42", RepositoryIDs: []string{"101", "101"}}} {
		_, err := f.service.SelectGitHubRepositories(context.Background(), a.AttemptID, "team-a00000000000000000000", "user-a00000000000000000000", "session-a", req)
		require.Error(t, err)
	}
}

func TestGitHubPostgresConcurrentOAuthAndSelection(t *testing.T) {
	f := newGitFixtureDatabase(t, true)
	ctx := context.Background()
	a, err := f.service.BeginGitHub(ctx, "team-a00000000000000000000", "user-a00000000000000000000", "session-a", BeginGitHubRequest{})
	require.NoError(t, err)
	u, _ := url.Parse(a.URL)
	results := make(chan error, 2)
	for range 2 {
		go func() {
			_, err := f.service.CompleteGitHubCallback(ctx, "team-a00000000000000000000", "user-a00000000000000000000", "session-a", u.Query().Get("state"), "code", "")
			results <- err
		}()
	}
	successes := 0
	for range 2 {
		if <-results == nil {
			successes++
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, f.tokenRequests)
	sc := f.selectRepos(t, a, "user-a00000000000000000000", "session-a", "101")
	one := f.authorize(t, "team-a00000000000000000000", "user-a00000000000000000000", "session-a", sc.id)
	two := f.authorize(t, "team-a00000000000000000000", "user-a00000000000000000000", "session-a", sc.id)
	for _, attempt := range []GitHubAttemptResponse{one, two} {
		go func(attempt GitHubAttemptResponse) {
			_, err := f.service.SelectGitHubRepositories(ctx, attempt.AttemptID, "team-a00000000000000000000", "user-a00000000000000000000", "session-a", GitHubSelectionRequest{InstallationID: "42", RepositoryIDs: []string{"202"}, ExpectedGeneration: &attempt.ConnectionGeneration, ExpectedSelectionVersion: &attempt.SelectionVersion})
			results <- err
		}(attempt)
	}
	successes = 0
	for range 2 {
		if <-results == nil {
			successes++
		}
	}
	require.Equal(t, 1, successes)
	var authorization models.GitHubAuthorization
	require.NoError(t, f.db.First(&authorization).Error)
	require.NoError(t, f.db.Model(&authorization).Update("expires_at", time.Now().Add(-time.Minute)).Error)
	before := f.tokenRequests
	for range 2 {
		go func() { _, _, err := access.UserToken(ctx, f.db, f.provider, authorization.ID); results <- err }()
	}
	for range 2 {
		require.NoError(t, <-results)
	}
	require.Equal(t, before+1, f.tokenRequests, "only one rotation may consume the refresh token")
}

func TestGitHubCleanupOnlyDeletesRecordedResourcesAndCannotSurviveReconnect(t *testing.T) {
	f := newGitFixture(t)
	ctx := context.Background()
	team, user := "team-a00000000000000000000", "user-a00000000000000000000"
	a := f.authorize(t, team, user, "session-a", "")
	sc := f.selectRepos(t, a, user, "session-a", "101")
	resolver := access.NewResolver(f.db, f.service.ProviderFactory())
	grant, err := resolver.Resolve(ctx, access.Request{TeamID: team, SourceControlID: sc.id, Repository: "org/repo101", Operation: access.ActionsBootstrap})
	require.NoError(t, err)
	resources := []access.Resource{{Kind: "workflow", Name: ".github/workflows/launch-one.yml", Branch: "main", ContentSHA256: strings.Repeat("a", 64)}, {Kind: "secret", Name: "LAUNCH_TOKEN_ONE"}, {Kind: "variable", Name: "LAUNCH_HOOK_ONE"}}
	id, err := access.RegisterCleanup(ctx, f.db, sc.id, grant.Snapshot().RepositoryID, grant.Snapshot(), resources)
	require.NoError(t, err)
	_, err = access.ResolveCleanup(ctx, f.db, f.service.ProviderFactory(), id)
	require.Error(t, err, "registered cleanup cannot yet issue tokens")
	require.NoError(t, resolver.Disconnect(ctx, sc.id, team))
	cleanup, err := access.ResolveCleanup(ctx, f.db, f.service.ProviderFactory(), id)
	require.NoError(t, err)
	require.NoError(t, cleanup.ValidateRequest(ctx, http.MethodDelete, "/repos/org/repo101/actions/secrets/LAUNCH_TOKEN_ONE", nil))
	require.NoError(t, cleanup.ValidateRequest(ctx, http.MethodGet, "/repos/org/repo101/contents/.github/workflows/launch-one.yml?ref=main", nil))
	for _, request := range [][2]string{{http.MethodGet, "/repos/org/repo101/contents/package.json"}, {http.MethodPut, "/repos/org/repo101/contents/.github/workflows/launch-one.yml"}, {http.MethodDelete, "/repos/org/repo101/actions/secrets/OTHER"}, {http.MethodDelete, "/repos/org/repo202/actions/secrets/LAUNCH_TOKEN_ONE"}, {http.MethodGet, "/repos/org/repo101/contents/.github/workflows/launch-one.yml?ref=other"}} {
		require.Error(t, cleanup.ValidateRequest(ctx, request[0], request[1], nil))
	}
	_, err = f.provider.GetInstallationToken(cleanup.Context(ctx), "42")
	require.NoError(t, err)
	require.NoError(t, cleanup.Complete(ctx))
	require.NoError(t, cleanup.Fail(ctx))
	var completed models.GitHubCleanupManifest
	require.NoError(t, f.db.First(&completed, "id = ?", id).Error)
	require.Equal(t, "completed", completed.Status, "a stale retry cannot reopen completed cleanup")
	require.GreaterOrEqual(t, f.revoked, 1)
	_, err = access.ResolveCleanup(ctx, f.db, f.service.ProviderFactory(), id)
	require.Error(t, err)
	// Pending cleanup from the old generation must not operate after a reconnect.
	review := f.authorize(t, team, user, "session-a", sc.id)
	f.selectRepos(t, review, user, "session-a", "101")
	require.Error(t, cleanup.Validate(ctx))
}
func TestGitHubOAuthExpiredAttemptAndSetupStateReplay(t *testing.T) {
	f := newGitFixture(t)
	ctx := context.Background()
	team, user := "team-a00000000000000000000", "user-a00000000000000000000"
	attempt, err := f.service.BeginGitHub(ctx, team, user, "session-a", BeginGitHubRequest{})
	require.NoError(t, err)
	u, _ := url.Parse(attempt.URL)
	require.NoError(t, f.db.Model(&models.GitHubConnectionAttempt{}).Where("id = ?", attempt.AttemptID).Update("expires_at", time.Now().Add(-time.Second)).Error)
	_, err = f.service.CompleteGitHubCallback(ctx, team, user, "session-a", u.Query().Get("state"), "code", "")
	require.Error(t, err)
	a := f.authorize(t, team, user, "session-a", "")
	status, err := f.service.GitHubAttempt(ctx, a.AttemptID, team, user, "session-a")
	require.NoError(t, err)
	install, _ := url.Parse(status.InstallURL)
	_, err = f.service.CompleteGitHubCallback(ctx, team, user, "session-a", install.Query().Get("state"), "", "42")
	require.NoError(t, err)
	_, err = f.service.CompleteGitHubCallback(ctx, team, user, "session-a", install.Query().Get("state"), "", "42")
	require.Error(t, err)
	var count int64
	require.NoError(t, f.db.Model(&models.SourceControl{}).Count(&count).Error)
	require.Zero(t, count, "installation callbacks cannot grant account access")
}
func TestGitHubPostgresConcurrentNewConnectionIsUnique(t *testing.T) {
	f := newGitFixtureDatabase(t, true)
	ctx := context.Background()
	team, user := "team-a00000000000000000000", "user-a00000000000000000000"
	one := f.authorize(t, team, user, "session-a", "")
	two := f.authorize(t, team, user, "session-a", "")
	results := make(chan error, 2)
	for _, a := range []GitHubAttemptResponse{one, two} {
		go func(a GitHubAttemptResponse) {
			_, err := f.service.SelectGitHubRepositories(ctx, a.AttemptID, team, user, "session-a", GitHubSelectionRequest{InstallationID: "42", RepositoryIDs: []string{"101"}})
			results <- err
		}(a)
	}
	successes := 0
	for range 2 {
		if <-results == nil {
			successes++
		}
	}
	require.Equal(t, 1, successes)
	var count int64
	require.NoError(t, f.db.Model(&models.SourceControl{}).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestGitHubSelectionEditPreservesUnchangedGrantsAndWorkflows(t *testing.T) {
	f := newGitFixture(t)
	ctx := context.Background()
	team, user := "team-a00000000000000000000", "user-a00000000000000000000"
	a := f.authorize(t, team, user, "session-a", "")
	sc := f.selectRepos(t, a, user, "session-a", "101")
	resolver := access.NewResolver(f.db, f.service.ProviderFactory())
	req := access.Request{TeamID: team, SourceControlID: sc.id, Repository: "org/repo101", Operation: access.Clone}
	grant, err := resolver.Resolve(ctx, req)
	require.NoError(t, err)
	id, err := access.RegisterCleanup(ctx, f.db, sc.id, grant.Snapshot().RepositoryID, grant.Snapshot(), []access.Resource{{Kind: "secret", Name: "LAUNCH_TOKEN_KEEP"}})
	require.NoError(t, err)
	_, err = f.provider.GetInstallationToken(grant.Context(ctx), "42")
	require.NoError(t, err)
	for _, selected := range [][]string{{"101", "202"}, {"101", "202"}, {"101"}} {
		review := f.authorize(t, team, user, "session-a", sc.id)
		updated := f.selectRepos(t, review, user, "session-a", selected...)
		require.Equal(t, sc.generation, updated.generation)
		expected := grant.Snapshot()
		req.Expected = &expected
		_, err = resolver.Resolve(ctx, req)
		require.NoError(t, err, "editing another repository must preserve this grant")
		var manifest models.GitHubCleanupManifest
		require.NoError(t, f.db.First(&manifest, "id = ?", id).Error)
		require.Equal(t, "registered", manifest.Status)
		var lease models.GitHubCredentialLease
		require.NoError(t, f.db.First(&lease).Error)
		require.Equal(t, "released", lease.Status)
	}
}
func TestGitHubCandidatesRetainUnavailableSavedSelectionForReview(t *testing.T) {
	f := newGitFixture(t)
	ctx := context.Background()
	team, user := "team-a00000000000000000000", "user-a00000000000000000000"
	a := f.authorize(t, team, user, "session-a", "")
	sc := f.selectRepos(t, a, user, "session-a", "101")
	f.repoIDs = []string{"202"}
	review := f.authorize(t, team, user, "session-a", sc.id)
	reviewed, err := f.service.GitHubCandidates(ctx, review.AttemptID, team, user, "session-a", "42")
	require.NoError(t, err)
	require.Equal(t, review.SelectionVersion, reviewed.SelectionVersion)
	require.Len(t, reviewed.Repositories, 2)
	require.True(t, reviewed.Repositories[0].Selected)
	require.True(t, reviewed.Repositories[0].Unavailable)
	require.Equal(t, "101", reviewed.Repositories[0].GitHubRepositoryID)
}

func TestGitHubPostgresSelectionRechecksMembershipAfterTeamLock(t *testing.T) {
	f := newGitFixtureDatabase(t, true)
	ctx := context.Background()
	team, user := "team-a00000000000000000000", "user-a00000000000000000000"
	a := f.authorize(t, team, user, "session-a", "")
	mutation := f.db.Begin()
	require.NoError(t, mutation.Error)
	require.NoError(t, mutation.Exec("SELECT id FROM teams WHERE id = ? FOR UPDATE", team).Error)
	started := make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		close(started)
		_, err := f.service.SelectGitHubRepositories(ctx, a.AttemptID, team, user, "session-a", GitHubSelectionRequest{InstallationID: "42", RepositoryIDs: []string{"101"}})
		finished <- err
	}()
	<-started
	select {
	case err := <-finished:
		t.Fatalf("selection did not wait for the membership lock: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	require.NoError(t, mutation.Exec("INSERT INTO team_invitations VALUES (?,?,'member')", team, "new@example.test").Error)
	require.NoError(t, mutation.Commit().Error)
	require.Error(t, <-finished)
	var count int64
	require.NoError(t, f.db.Model(&models.SourceControlRepository{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestGitHubDeletedOrSuspendedLocalSponsorCannotUseSavedGrant(t *testing.T) {
	for _, status := range []string{"suspended", "deleted"} {
		t.Run(status, func(t *testing.T) {
			f := newGitFixture(t)
			ctx := context.Background()
			team, user := "team-a00000000000000000000", "user-a00000000000000000000"
			a := f.authorize(t, team, user, "session-a", "")
			sc := f.selectRepos(t, a, user, "session-a", "101")
			resolver := access.NewResolver(f.db, f.service.ProviderFactory())
			req := access.Request{TeamID: team, SourceControlID: sc.id, Repository: "org/repo101", Operation: access.Clone}
			grant, err := resolver.Resolve(ctx, req)
			require.NoError(t, err)
			if status == "deleted" {
				require.NoError(t, f.db.Exec("DELETE FROM users WHERE id = ?", user).Error)
			} else {
				require.NoError(t, f.db.Table("users").Where("id = ?", user).Update("status", status).Error)
			}
			_, err = resolver.Resolve(ctx, req)
			require.Error(t, err)
			token, err := f.provider.GetInstallationToken(grant.Context(ctx), "42")
			require.Error(t, err)
			require.Empty(t, token)
		})
	}
}
