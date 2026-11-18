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
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

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
)

type recoveryGitHub struct {
	mu                       sync.Mutex
	files                    map[string]string
	secrets                  map[string]string
	requests, workflowWrites int
	failWorkflowResponse     bool
	failSecret               bool
	secretStatus             int
	afterWorkflow            func()
	pub, priv                *[32]byte
}

func (f *recoveryGitHub) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests++
	switch {
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/access_tokens"):
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"token": "scoped-token", "expires_at": time.Now().Add(time.Hour).UTC().Format(time.RFC3339)})
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/actions/secrets/public-key"):
		_ = json.NewEncoder(w).Encode(map[string]string{"key_id": "key", "key": base64.StdEncoding.EncodeToString(f.pub[:])})
	case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/actions/secrets/"):
		if f.secretStatus != 0 {
			w.WriteHeader(f.secretStatus)
			_, _ = w.Write([]byte(`{"message":"Resource not accessible by integration"}`))
			return
		}
		if f.failSecret && strings.Contains(r.URL.Path, "LAUNCH_BUILD_") {
			f.failSecret = false
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		sealed, err := base64.StdEncoding.DecodeString(body["encrypted_value"])
		opened, ok := box.OpenAnonymous(nil, sealed, f.pub, f.priv)
		if err != nil || !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		f.secrets[r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]] = string(opened)
		w.WriteHeader(http.StatusCreated)
	case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/actions/variables/"):
		w.WriteHeader(http.StatusNotFound)
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/actions/variables"):
		w.WriteHeader(http.StatusCreated)
	case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/contents/"):
		content, exists := f.files[r.URL.Path]
		if !exists {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"sha": "blob-sha", "encoding": "base64", "content": base64.StdEncoding.EncodeToString([]byte(content))})
	case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/contents/"):
		if _, exists := f.files[r.URL.Path]; exists {
			w.WriteHeader(http.StatusConflict)
			return
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		content, _ := base64.StdEncoding.DecodeString(body["content"])
		f.files[r.URL.Path] = string(content)
		f.workflowWrites++
		if f.afterWorkflow != nil {
			f.afterWorkflow()
		}
		if f.failWorkflowResponse {
			f.failWorkflowResponse = false
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"content": map[string]string{"sha": "blob-sha"}})
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

type recoveryFixture struct {
	db                    *gorm.DB
	job                   *GHABootstrapWorkflowJob
	remote                *recoveryGitHub
	config                dbtype.JSONMap
	snapshot, oldSnapshot access.Snapshot
	teamID, id, kind      string
}

func newRecoveryFixture(t *testing.T, kind string) *recoveryFixture {
	t.Helper()
	dsn := os.Getenv("DOCKER_TEST_POSTGRES_DSN")
	if dsn == "" {
		dsn = os.Getenv("LAUNCH_GITHUB_TEST_POSTGRES_DSN")
	}
	if dsn == "" {
		t.Skip("PostgreSQL DSN required for GHA recovery integration")
	}
	base, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	schema := "gha_recovery_" + strings.ToLower(util.NewULID())
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
	require.NoError(t, db.AutoMigrate(&models.Application{}, &models.ApplicationBuildSecret{}, &models.Compose{}, &models.ComposeBuildSecret{}))
	config := dbtype.JSONMap{"branch": "main", "auto_deploy": true, "custom_option": "preserved", "compose_file_path": "deploy/compose.yml", "gha_workflow_sha": "old-blob", "gha_workflow_path": ".github/workflows/old.yml", "gha_deploy_secret_name": "OLD_TOKEN", "gha_binding_id": "old-binding", "gha_install_status": "installation_gone"}
	teamID := util.NewULID()
	selected := testutil.SelectedRepository(t, db, teamID, "acme/app", config)
	oldSnapshot := selected.Snapshot()
	snapshot := oldSnapshot
	snapshot.ConnectionGeneration++
	require.NoError(t, db.Model(selected.SourceControl).Update("generation", snapshot.ConnectionGeneration).Error)
	// Pre-isolation workloads lacked these fields; recovery must upgrade them.
	delete(config, "git_access")
	delete(config, "repository_id")
	oldHash := "old-token-hash"
	var id string
	if kind == "application" {
		app := models.Application{Name: "web", ProjectID: util.NewULID(), SourceType: types.SourceTypeGit, SourceConfig: config, BuildConfig: dbtype.JSONMap{"dockerfile_path": "deploy/Dockerfile"}, BuildLocation: types.BuildLocationGitHubActions, GHADeployTokenHash: &oldHash}
		app.TeamID = teamID
		app.ServerID = util.NewULID()
		require.NoError(t, db.Create(&app).Error)
		id = app.ID
		require.NoError(t, db.Create(&models.ApplicationBuildSecret{ApplicationID: id, Name: "NPM_TOKEN", Value: dbtype.EncryptedString("build-secret")}).Error)
	} else {
		compose := models.Compose{Name: "stack", ProjectID: util.NewULID(), ComposeSourceType: "git", SourceConfig: config, BuildLocation: types.BuildLocationGitHubActions, GHADeployTokenHash: &oldHash}
		compose.TeamID = teamID
		compose.ServerID = util.NewULID()
		require.NoError(t, db.Create(&compose).Error)
		id = compose.ID
		require.NoError(t, db.Create(&models.ComposeBuildSecret{ComposeID: id, Name: "NPM_TOKEN", Value: dbtype.EncryptedString("build-secret")}).Error)
	}
	pub, priv, err := box.GenerateKey(rand.Reader)
	require.NoError(t, err)
	remote := &recoveryGitHub{files: map[string]string{"/repos/acme/app/contents/.github/workflows/old.yml": "user-edited old workflow"}, secrets: map[string]string{}, pub: pub, priv: priv}
	server := httptest.NewServer(http.HandlerFunc(remote.serve))
	t.Cleanup(server.Close)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	factory := providers.NewProviderFactory()
	factory.RegisterConfig(providers.GitProviderGitHub, &providers.ProviderConfig{AppID: "app", PrivateKey: string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})), APIURL: server.URL})
	log := zerolog.New(io.Discard)
	job := &GHABootstrapWorkflowJob{Deps: &JobDeps{Deps: &pkgjobs.Deps{DB: db, Logger: &log}, GitProviders: factory}, Payload: GHABootstrapWorkflowPayload{WorkloadKind: kind, WorkloadID: id, GitAccess: &snapshot, BindingID: "recovery-binding", Recovery: true, RotateToken: true, LaunchBaseURL: "https://launch.example"}}
	return &recoveryFixture{db: db, job: job, remote: remote, config: config, snapshot: snapshot, oldSnapshot: oldSnapshot, teamID: teamID, id: id, kind: kind}
}

func (f *recoveryFixture) table() string {
	if f.kind == "application" {
		return "docker_applications"
	}
	return "docker_composes"
}
func (f *recoveryFixture) saved(t *testing.T) (dbtype.JSONMap, string) {
	t.Helper()
	if f.kind == "application" {
		var row models.Application
		require.NoError(t, f.db.First(&row, "id = ?", f.id).Error)
		return row.SourceConfig, *row.GHADeployTokenHash
	}
	var row models.Compose
	require.NoError(t, f.db.First(&row, "id = ?", f.id).Error)
	return row.SourceConfig, *row.GHADeployTokenHash
}
func (f *recoveryFixture) saveConfig(t *testing.T) {
	t.Helper()
	require.NoError(t, f.db.Table(f.table()).Where("id = ?", f.id).Update("source_config", f.config).Error)
}

func TestGHABootstrapRecoveryRestoresLegacyAndDeduplicates(t *testing.T) {
	for _, kind := range []string{"application", "compose"} {
		t.Run(kind, func(t *testing.T) {
			f := newRecoveryFixture(t, kind)
			require.NoError(t, f.job.Handle(context.Background()))
			config, hash := f.saved(t)
			primary, err := access.SnapshotFromMap(config)
			require.NoError(t, err)
			require.Equal(t, f.snapshot, *primary)
			installed, err := access.SnapshotFromMap(map[string]any{"git_access": config["gha_access"]})
			require.NoError(t, err)
			require.Equal(t, f.snapshot, *installed)
			require.Equal(t, f.snapshot.RepositoryID, config["repository_id"])
			require.Equal(t, "https://github.com/acme/app.git", config["repo"])
			require.Equal(t, true, config["auto_deploy"])
			require.Equal(t, "preserved", config["custom_option"])
			require.Equal(t, "recovery-binding", config["gha_binding_id"])
			require.NotEqual(t, "old-token-hash", hash)
			namespace := workflowBindingNamespace(f.snapshot, f.id, "recovery-binding")
			f.remote.mu.Lock()
			raw := f.remote.secrets[stringAt(config, "gha_deploy_secret_name")]
			require.NotEmpty(t, raw)
			digest := sha256.Sum256([]byte(raw))
			require.Equal(t, hex.EncodeToString(digest[:]), hash)
			require.Equal(t, "build-secret", f.remote.secrets["LAUNCH_BUILD_"+namespace+"_NPM_TOKEN"])
			workflow := f.remote.files["/repos/acme/app/contents/"+stringAt(config, "gha_workflow_path")]
			require.Contains(t, workflow, "LAUNCH_BUILD_"+namespace+"_NPM_TOKEN")
			require.Contains(t, workflow, "push:")
			if kind == "application" {
				require.Contains(t, workflow, "deploy/Dockerfile")
			} else {
				require.Contains(t, workflow, "deploy/compose.yml")
			}
			require.Equal(t, "user-edited old workflow", f.remote.files["/repos/acme/app/contents/.github/workflows/old.yml"])
			requests := f.remote.requests
			f.remote.mu.Unlock()
			require.NoError(t, f.job.Handle(context.Background()), "same queued intent redelivery")
			f.job.Payload.BindingID = "older-delayed-recovery"
			require.NoError(t, f.job.Handle(context.Background()), "healthy manual/recovery binding wins over delayed recovery")
			delete(config, "gha_binding_id")
			f.config = config
			f.saveConfig(t)
			require.NoError(t, f.job.Handle(context.Background()), "healthy older deployments without a stored binding ID must remain unchanged")
			_, afterHash := f.saved(t)
			require.Equal(t, hash, afterHash)
			f.remote.mu.Lock()
			require.Equal(t, requests, f.remote.requests)
			f.remote.mu.Unlock()
		})
	}
}

func TestGHABootstrapRecoveryRetriesPartialWrites(t *testing.T) {
	for _, kind := range []string{"application", "compose"} {
		for _, failure := range []string{"secret", "workflow_response"} {
			t.Run(kind+"/"+failure, func(t *testing.T) {
				f := newRecoveryFixture(t, kind)
				f.remote.failSecret = failure == "secret"
				f.remote.failWorkflowResponse = failure == "workflow_response"
				require.Error(t, f.job.Handle(context.Background()))
				config, hash := f.saved(t)
				require.Equal(t, "old-token-hash", hash)
				require.Equal(t, "old-binding", config["gha_binding_id"])
				require.NoError(t, f.job.Handle(context.Background()))
				config, hash = f.saved(t)
				require.Equal(t, "recovery-binding", config["gha_binding_id"])
				f.remote.mu.Lock()
				require.Equal(t, 1, f.remote.workflowWrites, "partial-response retry must reuse identical committed workflow")
				token := f.remote.secrets[stringAt(config, "gha_deploy_secret_name")]
				digest := sha256.Sum256([]byte(token))
				require.Equal(t, hex.EncodeToString(digest[:]), hash)
				f.remote.mu.Unlock()
			})
		}
	}
}

func TestGHABootstrapRecoveryFailsBeforeWritesWithoutAuthorityOrSecrets(t *testing.T) {
	for _, kind := range []string{"application", "compose"} {
		for _, failure := range []string{"stale_snapshot", "unselected", "cross_account", "dependency", "secret_read", "missing_url"} {
			t.Run(kind+"/"+failure, func(t *testing.T) {
				f := newRecoveryFixture(t, kind)
				switch failure {
				case "stale_snapshot":
					f.job.Payload.GitAccess = &f.oldSnapshot
				case "unselected":
					require.NoError(t, f.db.Model(&gitmodels.SourceControlRepository{}).Where("id = ?", f.snapshot.RepositoryID).Update("selected", false).Error)
				case "cross_account":
					require.NoError(t, f.db.Table(f.table()).Where("id = ?", f.id).Update("team_id", util.NewULID()).Error)
				case "dependency":
					f.config["dependency_repository_ids"] = []string{util.NewULID()}
					f.saveConfig(t)
				case "missing_url":
					f.job.Payload.LaunchBaseURL = ""
				case "secret_read":
					callbackName := "test:secret_read_failure"
					require.NoError(t, f.db.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
						if strings.HasSuffix(tx.Statement.Table, "_build_secrets") {
							tx.AddError(errors.New("secret inventory unavailable"))
						}
					}))
					defer f.db.Callback().Query().Remove(callbackName)
				}
				require.Error(t, f.job.Handle(context.Background()))
				_, hash := f.saved(t)
				require.Equal(t, "old-token-hash", hash)
				f.remote.mu.Lock()
				require.Zero(t, f.remote.requests)
				f.remote.mu.Unlock()
			})
		}
	}
}

func TestGHABootstrapRecoveryCannotCompleteAfterRevocation(t *testing.T) {
	for _, kind := range []string{"application", "compose"} {
		t.Run(kind, func(t *testing.T) {
			f := newRecoveryFixture(t, kind)
			var mutationErr error
			f.remote.afterWorkflow = func() {
				mutationErr = f.db.Model(&gitmodels.SourceControl{}).Where("id = ?", f.snapshot.ConnectionID).Update("generation", gorm.Expr("generation + 1")).Error
			}
			require.Error(t, f.job.Handle(context.Background()))
			require.NoError(t, mutationErr)
			config, hash := f.saved(t)
			require.Equal(t, "old-token-hash", hash)
			require.Equal(t, "old-binding", config["gha_binding_id"])
			require.NotContains(t, config, "git_access")
		})
	}
}

func TestGHABootstrapRecoveryPreservesModifiedPartialWorkflow(t *testing.T) {
	f := newRecoveryFixture(t, "application")
	f.remote.failWorkflowResponse = true
	require.Error(t, f.job.Handle(context.Background()))
	f.remote.mu.Lock()
	var path string
	for key := range f.remote.files {
		if !strings.HasSuffix(key, "old.yml") {
			path = key
		}
	}
	require.NotEmpty(t, path)
	f.remote.files[path] = "user edited recovery workflow"
	f.remote.mu.Unlock()
	require.ErrorContains(t, f.job.Handle(context.Background()), "workflow was modified")
	_, hash := f.saved(t)
	require.Equal(t, "old-token-hash", hash)
	f.remote.mu.Lock()
	defer f.remote.mu.Unlock()
	require.Equal(t, "user edited recovery workflow", f.remote.files[path])
	require.Equal(t, 1, f.remote.workflowWrites)
}

func TestGHABootstrapRecoveryTerminalFailureRecordsRetryBinding(t *testing.T) {
	f := newRecoveryFixture(t, "application")
	f.remote.secretStatus = http.StatusForbidden
	require.NoError(t, f.job.Handle(context.Background()), "terminal permission failure ends queue retries")
	config, hash := f.saved(t)
	require.Equal(t, "failed", config["gha_recovery_status"])
	require.Equal(t, "recovery-binding", config["gha_recovery_binding_id"])
	require.Equal(t, "old-token-hash", hash)
	require.Equal(t, "old-binding", config["gha_binding_id"])
}

func TestGHABootstrapRecoveryFailureMarkerIsFenced(t *testing.T) {
	for _, kind := range []string{"application", "compose"} {
		t.Run(kind, func(t *testing.T) {
			f := newRecoveryFixture(t, kind)
			f.remote.failSecret = true
			err := f.job.Handle(context.Background())
			require.Error(t, err)
			f.job.Failed(context.Background(), err)
			config, hash := f.saved(t)
			require.Equal(t, "failed", config["gha_recovery_status"])
			require.Equal(t, "recovery-binding", config["gha_recovery_binding_id"])
			require.Equal(t, "old-token-hash", hash)
			require.NoError(t, f.job.Handle(context.Background()))
			config, hash = f.saved(t)
			require.Empty(t, config["gha_recovery_status"])
			f.job.Payload.BindingID = "late-failure"
			f.job.Failed(context.Background(), errors.New("delayed exhausted retry"))
			after, afterHash := f.saved(t)
			require.Empty(t, after["gha_recovery_status"])
			require.Equal(t, hash, afterHash)
			// A failure from a revoked generation must not mutate the newly connected workload.
			require.NoError(t, f.db.Model(&gitmodels.SourceControl{}).Where("id = ?", f.snapshot.ConnectionID).Update("generation", gorm.Expr("generation + 1")).Error)
			f.job.Payload.GitAccess = &f.oldSnapshot
			f.job.Failed(context.Background(), errors.New("stale failure"))
			after, afterHash = f.saved(t)
			require.Empty(t, after["gha_recovery_status"])
			require.Equal(t, hash, afterHash)
		})
	}
}

func TestGHABootstrapRecoveryCannotCompleteAfterBranchChange(t *testing.T) {
	for _, kind := range []string{"application", "compose"} {
		t.Run(kind, func(t *testing.T) {
			f := newRecoveryFixture(t, kind)
			mutationResult := make(chan error, 1)
			f.remote.afterWorkflow = func() {
				merge, err := sourceConfigMergeExpr(map[string]any{"branch": "new-release"})
				if err == nil {
					err = f.db.Table(f.table()).Where("id = ?", f.id).Update("source_config", merge).Error
				}
				mutationResult <- err
			}
			require.Error(t, f.job.Handle(context.Background()))
			require.NoError(t, <-mutationResult)
			config, hash := f.saved(t)
			require.Equal(t, "new-release", config["branch"])
			require.Equal(t, "old-token-hash", hash)
			require.Equal(t, "old-binding", config["gha_binding_id"])
			require.NotContains(t, config, "git_access")
		})
	}
}
