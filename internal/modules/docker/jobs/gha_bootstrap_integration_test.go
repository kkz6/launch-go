package jobs

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"github.com/kkz6/launch-go/internal/database/serializers"
	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"github.com/kkz6/launch-go/internal/modules/docker/testutil"
	"github.com/kkz6/launch-go/internal/modules/docker/types"
	"github.com/kkz6/launch-go/internal/modules/git/access"
	gitmodels "github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/kkz6/launch-go/internal/modules/git/providers"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/util"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/nacl/box"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestGHABootstrapPostgresSerializesWritesAndRecordsOwnership(t *testing.T) {
	dsn := os.Getenv("DOCKER_TEST_POSTGRES_DSN")
	if dsn == "" {
		dsn = os.Getenv("LAUNCH_GITHUB_TEST_POSTGRES_DSN")
	}
	if dsn == "" {
		t.Skip("DOCKER_TEST_POSTGRES_DSN is required for PostgreSQL workflow provisioning integration")
	}
	base, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	schema := "docker_isolation_" + strings.ToLower(util.NewULID())
	require.NoError(t, base.Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() { base.Exec("DROP SCHEMA " + schema + " CASCADE"); sqlDB, _ := base.DB(); _ = sqlDB.Close() })
	parsed, err := url.Parse(dsn)
	require.NoError(t, err)
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	db, err := gorm.Open(postgres.Open(parsed.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent), DisableForeignKeyConstraintWhenMigrating: true})
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB, _ := db.DB(); _ = sqlDB.Close() })
	require.NoError(t, db.Exec("CREATE DOMAIN longtext AS text").Error)
	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))
	require.NoError(t, db.AutoMigrate(&models.Application{}, &models.ApplicationBuildSecret{}, &models.Compose{}, &models.ComposeBuildSecret{}, &gitmodels.GitHubCleanupManifest{}))
	config := dbtype.JSONMap{"branch": "main"}
	teamID := util.NewULID()
	resolved := testutil.SelectedRepository(t, db, teamID, "acme/app", config)
	snapshot := resolved.Snapshot()
	app := models.Application{Name: "app", ProjectID: util.NewULID(), SourceType: types.SourceTypeGit, SourceConfig: config, BuildLocation: types.BuildLocationGitHubActions}
	app.TeamID = teamID
	app.ServerID = util.NewULID()
	require.NoError(t, db.Create(&app).Error)
	// A second account can select the same repository for server deployments
	// without disrupting this account's GitHub Actions workflow.
	otherConfig := dbtype.JSONMap{"branch": "main"}
	otherTeamID := util.NewULID()
	otherGrant := testutil.SelectedRepository(t, db, otherTeamID, "acme/app", otherConfig)
	require.NoError(t, db.Model(otherGrant.Repository).Update("git_hub_repository_id", resolved.GitHubRepositoryID).Error)
	otherSnapshot := otherGrant.Snapshot()
	otherApp := models.Application{Name: "other", ProjectID: util.NewULID(), SourceType: types.SourceTypeGit, SourceConfig: otherConfig, BuildLocation: types.BuildLocationGitHubActions}
	otherApp.TeamID = otherTeamID
	otherApp.ServerID = util.NewULID()
	require.NoError(t, db.Create(&otherApp).Error)
	pub, _, err := box.GenerateKey(rand.Reader)
	require.NoError(t, err)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	arrived := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	var mu sync.Mutex
	var writtenWorkflow string
	var mintedScopes []map[string]any
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/access_tokens"):
			once.Do(func() { close(arrived); <-release })
			var scope map[string]any
			_ = json.NewDecoder(r.Body).Decode(&scope)
			mu.Lock()
			mintedScopes = append(mintedScopes, scope)
			mu.Unlock()
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"token": "scoped-installation-token", "expires_at": time.Now().Add(time.Hour).UTC().Format(time.RFC3339)})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/actions/secrets/public-key"):
			_ = json.NewEncoder(w).Encode(map[string]string{"key_id": "key", "key": base64.StdEncoding.EncodeToString(pub[:])})
		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/actions/secrets/"):
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/actions/variables/"):
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/actions/variables"):
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/contents/"):
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/contents/"):
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			encoded, _ := body["content"].(string)
			data, _ := base64.StdEncoding.DecodeString(encoded)
			mu.Lock()
			writtenWorkflow = string(data)
			mu.Unlock()
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"content": map[string]string{"sha": "blob-sha"}, "commit": map[string]string{"sha": "commit-sha"}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	factory := providers.NewProviderFactory()
	factory.RegisterConfig(providers.GitProviderGitHub, &providers.ProviderConfig{AppID: "app", PrivateKey: string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})), APIURL: server.URL})
	log := zerolog.New(io.Discard)
	deps := &JobDeps{Deps: &pkgjobs.Deps{DB: db, Logger: &log}, GitProviders: factory}
	first := &GHABootstrapWorkflowJob{Deps: deps, Payload: GHABootstrapWorkflowPayload{WorkloadKind: "application", WorkloadID: app.ID, LaunchBaseURL: "https://launch.example", GitAccess: &snapshot, BindingID: "first-binding"}}
	completed := make(chan error, 1)
	go func() { completed <- first.Handle(context.Background()) }()
	select {
	case <-arrived:
	case err := <-completed:
		t.Fatalf("bootstrap failed before provider: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("bootstrap did not reach provider")
	}
	second := &GHABootstrapWorkflowJob{Deps: deps, Payload: GHABootstrapWorkflowPayload{WorkloadKind: "application", WorkloadID: app.ID, LaunchBaseURL: "https://launch.example", GitAccess: &snapshot, BindingID: "second-binding"}}
	require.ErrorContains(t, second.Handle(context.Background()), "another workflow update is in progress")
	mu.Lock()
	beforeOther := requestCount
	mu.Unlock()
	otherJob := &GHABootstrapWorkflowJob{Deps: deps, Payload: GHABootstrapWorkflowPayload{WorkloadKind: "application", WorkloadID: otherApp.ID, LaunchBaseURL: "https://launch.example", GitAccess: &otherSnapshot, BindingID: "other-binding"}}
	require.Error(t, otherJob.Handle(context.Background()))
	mu.Lock()
	require.Equal(t, beforeOther, requestCount, "the other account must be rejected before issuing any remote credentials or writes")
	mu.Unlock()
	close(release)
	require.NoError(t, <-completed)
	require.NoError(t, db.First(&app, "id = ?", app.ID).Error)
	saved, err := access.SnapshotFromMap(map[string]any{"git_access": app.SourceConfig["gha_access"]})
	require.NoError(t, err)
	require.Equal(t, snapshot, *saved)
	require.NotEmpty(t, app.GHADeployTokenHash)
	require.Contains(t, app.SourceConfig["gha_workflow_path"], strings.ToLower(workflowBindingNamespace(snapshot, app.ID, "first-binding")))
	var manifest gitmodels.GitHubCleanupManifest
	require.NoError(t, db.First(&manifest).Error)
	var resources []access.Resource
	require.NoError(t, json.Unmarshal([]byte(manifest.Resources), &resources))
	require.Len(t, resources, 3)
	mu.Lock()

	digest := sha256.Sum256([]byte(writtenWorkflow))
	require.Equal(t, hex.EncodeToString(digest[:]), resources[0].ContentSHA256)
	require.NotEmpty(t, mintedScopes)
	for _, scope := range mintedScopes {
		require.Len(t, scope["repository_ids"], 1)
		require.NotEmpty(t, scope["permissions"])
	}
	require.NotContains(t, writtenWorkflow, "scoped-installation-token")
	mu.Unlock()
	composeConfig := dbtype.JSONMap{}
	for k, v := range config {
		composeConfig[k] = v
	}
	compose := models.Compose{Name: "stack", ProjectID: util.NewULID(), ComposeSourceType: "git", SourceConfig: composeConfig, BuildLocation: types.BuildLocationGitHubActions}
	compose.TeamID = teamID
	compose.ServerID = util.NewULID()
	require.NoError(t, db.Create(&compose).Error)
	composeJob := &GHABootstrapWorkflowJob{Deps: deps, Payload: GHABootstrapWorkflowPayload{WorkloadKind: "compose", WorkloadID: compose.ID, LaunchBaseURL: "https://launch.example", GitAccess: &snapshot, BindingID: "compose-binding"}}
	require.NoError(t, composeJob.Handle(context.Background()))
	require.NoError(t, db.First(&compose, "id = ?", compose.ID).Error)
	require.NotEmpty(t, compose.GHADeployTokenHash)
	require.Contains(t, compose.SourceConfig["gha_workflow_path"], "launch-compose-")
	require.NotEqual(t, app.SourceConfig["gha_deploy_secret_name"], compose.SourceConfig["gha_deploy_secret_name"])
	mu.Lock()
	require.Contains(t, writtenWorkflow, compose.SourceConfig["gha_deploy_secret_name"])
	require.NotContains(t, writtenWorkflow, app.SourceConfig["gha_deploy_secret_name"])
	mu.Unlock()
}
