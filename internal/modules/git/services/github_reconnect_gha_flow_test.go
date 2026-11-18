package services

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/kkz6/launch-go/internal/config"
	dockeraccess "github.com/kkz6/launch-go/internal/modules/docker/gitaccess"
	dockerhandlers "github.com/kkz6/launch-go/internal/modules/docker/handlers"
	dockerjobs "github.com/kkz6/launch-go/internal/modules/docker/jobs"
	dockermodels "github.com/kkz6/launch-go/internal/modules/docker/models"
	dockerrepos "github.com/kkz6/launch-go/internal/modules/docker/repositories"
	dockerservices "github.com/kkz6/launch-go/internal/modules/docker/services"
	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
	"github.com/kkz6/launch-go/internal/modules/git/access"
	"github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/queue"
	"github.com/kkz6/launch-go/internal/pkg/service"
	"github.com/kkz6/launch-go/internal/pkg/util"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/nacl/box"
)

// Exercise the real authorization, selection, recovery worker, manual Deploy,
// and workflow callback together. Only GitHub HTTP and the deployment queue
// are substituted; access checks and persisted workload bindings are real.
func TestGitHubPostgresReconnectRestoresGitHubActionsEndToEnd(t *testing.T) {
	for _, mode := range []string{"legacy", "disconnected"} {
		for _, kind := range []string{"application", "compose"} {
			t.Run(mode+"/"+kind, func(t *testing.T) {
				ctx := context.Background()
				f := newGitFixtureDatabase(t, true)
				f.db.Config.DisableForeignKeyConstraintWhenMigrating = true
				require.NoError(t, f.db.Exec("CREATE DOMAIN longtext AS text").Error)
				require.NoError(t, f.db.AutoMigrate(&models.GitHubWebhookIntent{}, &dockermodels.Project{}, &dockermodels.Application{}, &dockermodels.Compose{}, &dockermodels.ApplicationBuildSecret{}, &dockermodels.ComposeBuildSecret{}, &dockermodels.Deployment{}, &dockermodels.RegistryLease{}))
				f.service.ServiceDeps().AppURL = "https://api.launch.test"
				team, user, session := "team-a00000000000000000000", "user-a00000000000000000000", "session-a"
				connection := f.selectRepos(t, f.authorize(t, team, user, session, ""), user, session, "101")
				other := f.selectRepos(t, f.authorize(t, "team-b00000000000000000000", "user-b00000000000000000000", "session-b", ""), "user-b00000000000000000000", "session-b", "303")
				var otherBefore models.SourceControl
				require.NoError(t, f.db.First(&otherBefore, "id = ?", other.id).Error)
				grant, err := access.NewResolver(f.db, f.service.ProviderFactory()).Resolve(ctx, access.Request{TeamID: team, SourceControlID: connection.id, Repository: "org/repo101", Operation: access.ActionsBootstrap})
				require.NoError(t, err)
				oldSnapshot := grant.Snapshot()
				project := dockermodels.Project{Name: "existing-project"}
				project.TeamID, project.ServerID = team, util.NewULID()
				require.NoError(t, f.db.Create(&project).Error)
				oldToken := "old-workflow-credential"
				sum := sha256.Sum256([]byte(oldToken))
				oldHash := hex.EncodeToString(sum[:])
				cfg := dbtype.JSONMap{"source_control_id": connection.id, "repo": "https://github.com/org/repo101.git", "branch": "main", "auto_deploy": true, "custom_option": "preserved", "gha_workflow_path": ".github/workflows/old.yml", "gha_workflow_sha": "old-sha", "gha_deploy_secret_name": "OLD_DEPLOY_TOKEN", "gha_image_repository": "ghcr.io/org/existing", "gha_access": oldSnapshot, "git_access": oldSnapshot}
				if mode != "legacy" {
					cfg["repository_id"] = oldSnapshot.RepositoryID
				}
				app := dockermodels.Application{Name: "existing-app", ProjectID: project.ID, SourceType: dockertypes.SourceTypeGit, SourceConfig: cfg, BuildLocation: dockertypes.BuildLocationGitHubActions, GHADeployTokenHash: &oldHash, Status: dockertypes.ApplicationStatusRunning}
				app.TeamID, app.ServerID = team, project.ServerID
				compose := dockermodels.Compose{Name: "existing-stack", ProjectID: project.ID, ComposeSourceType: "git", SourceConfig: cfg, BuildLocation: dockertypes.BuildLocationGitHubActions, GHADeployTokenHash: &oldHash, Status: dockertypes.ApplicationStatusRunning}
				compose.TeamID, compose.ServerID = team, project.ServerID
				var workloadID string
				if kind == "application" {
					require.NoError(t, f.db.Create(&app).Error)
					workloadID = app.ID
				} else {
					require.NoError(t, f.db.Create(&compose).Error)
					workloadID = compose.ID
				}
				if mode == "legacy" {
					require.NoError(t, f.db.Model(&models.SourceControl{}).Where("id = ?", connection.id).Updates(map[string]any{"status": "reconnect_required", "git_hub_authorization_id": nil, "git_hub_installation_id": nil}).Error)
					require.NoError(t, f.db.Model(&models.SourceControlRepository{}).Where("source_control_id = ?", connection.id).Updates(map[string]any{"selected": false, "available": false, "git_hub_repository_id": ""}).Error)
				} else {
					require.NoError(t, f.service.Disconnect(ctx, connection.id, team, user))
				}

				pub, priv, err := box.GenerateKey(rand.Reader)
				require.NoError(t, err)
				secrets := map[string]string{}
				workflows := map[string]string{}
				var dispatched []string
				f.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch {
					case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/actions/secrets/public-key"):
						_ = json.NewEncoder(w).Encode(map[string]string{"key_id": "test-key", "key": base64.StdEncoding.EncodeToString(pub[:])})
					case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/actions/secrets/"):
						var body struct {
							EncryptedValue string `json:"encrypted_value"`
						}
						if json.NewDecoder(r.Body).Decode(&body) != nil {
							w.WriteHeader(400)
							return
						}
						cipher, _ := base64.StdEncoding.DecodeString(body.EncryptedValue)
						plain, ok := box.OpenAnonymous(nil, cipher, pub, priv)
						if !ok {
							w.WriteHeader(400)
							return
						}
						secrets[r.URL.Path] = string(plain)
						w.WriteHeader(http.StatusCreated)
					case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/actions/variables/"):
						w.WriteHeader(http.StatusNotFound)
					case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/actions/variables"):
						w.WriteHeader(http.StatusCreated)
					case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/contents/"):
						w.WriteHeader(http.StatusNotFound)
					case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/contents/"):
						var body struct {
							Content string `json:"content"`
						}
						_ = json.NewDecoder(r.Body).Decode(&body)
						content, _ := base64.StdEncoding.DecodeString(body.Content)
						workflows[r.URL.Path] = string(content)
						w.WriteHeader(http.StatusCreated)
						_ = json.NewEncoder(w).Encode(map[string]any{"content": map[string]string{"sha": "new-blob"}, "commit": map[string]string{"sha": "new-commit"}})
					case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/dispatches"):
						dispatched = append(dispatched, r.URL.Path)
						w.WriteHeader(http.StatusNoContent)
					default:
						f.serve(w, r)
					}
				})
				redis := miniredis.RunT(t)
				q := queue.NewClient(config.RedisConfig{Address: redis.Addr()})
				t.Cleanup(func() { _ = q.Close() })
				log := zerolog.New(io.Discard)
				h := dockerhandlers.NewGHAWebhookHandler(dockerhandlers.GHAWebhookHandlerConfig{DB: f.db, Queue: q, GitProviders: f.service.ProviderFactory(), Logger: &log})
				web := fiber.New(fiber.Config{ErrorHandler: fiber.DefaultErrorHandler})
				web.Post("/application/:id", h.GHAApplicationDeploy)
				web.Post("/compose/:id", h.GHAComposeDeploy)
				callback := func(token, runID, image string) int {
					body := map[string]any{"run_id": runID, "image_tag": image, "service_images": map[string]string{"web": image}, "branch": "main", "ghcr_pull_token": "test-ghcr-pull-token", "ghcr_pull_token_minted_at": strconv.FormatInt(time.Now().Unix(), 10), "ghcr_pull_token_expires_at": strconv.FormatInt(time.Now().Add(10*time.Minute).Unix(), 10)}
					data, e := json.Marshal(body)
					require.NoError(t, e)
					req := httptest.NewRequest(http.MethodPost, "/"+kind+"/"+workloadID, bytes.NewReader(data))
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("Authorization", "Bearer "+token)
					response, e := web.Test(req, -1)
					require.NoError(t, e)
					defer response.Body.Close()
					if response.StatusCode >= http.StatusInternalServerError {
						body, _ := io.ReadAll(response.Body)
						t.Logf("callback failure: %s", body)
					}
					return response.StatusCode
				}
				require.Equal(t, http.StatusUnauthorized, callback(oldToken, "before", "ghcr.io/org/existing:old"))
				attempt := f.authorize(t, team, user, session, connection.id)
				review, err := f.service.GitHubCandidates(ctx, attempt.AttemptID, team, user, session, "42")
				require.NoError(t, err)
				saved, err := f.service.SelectGitHubRepositories(ctx, attempt.AttemptID, team, user, session, selectionFromReview(t, review, "42", "101"))
				require.NoError(t, err)
				require.Equal(t, connection.id, saved.ID)
				require.Equal(t, http.StatusUnauthorized, callback(oldToken, "before-repair", "ghcr.io/org/existing:old"))
				var intents []models.GitHubWebhookIntent
				require.NoError(t, f.db.Where("task_type = ?", dockerjobs.TypeGHABootstrapWorkflow).Find(&intents).Error)
				require.Len(t, intents, 1, "saving reconnect automatically schedules this workload")
				data, err := json.Marshal(intents[0].Payload)
				require.NoError(t, err)
				var payload dockerjobs.GHABootstrapWorkflowPayload
				require.NoError(t, json.Unmarshal(data, &payload))
				require.Equal(t, workloadID, payload.WorkloadID)
				require.NotEmpty(t, payload.BindingID)
				require.Equal(t, "https://api.launch.test", payload.LaunchBaseURL)
				require.Equal(t, oldSnapshot.RepositoryID, payload.GitAccess.RepositoryID, "existing repository ID survives reconnect")
				job := &dockerjobs.GHABootstrapWorkflowJob{Deps: &dockerjobs.JobDeps{Deps: &pkgjobs.Deps{DB: f.db, Queue: q, Logger: &log}, GitProviders: f.service.ProviderFactory()}, Payload: payload}
				require.NoError(t, job.Handle(ctx))
				var restored dbtype.JSONMap
				if kind == "application" {
					require.NoError(t, f.db.First(&app, "id = ?", workloadID).Error)
					restored = app.SourceConfig
					require.Equal(t, dockertypes.ApplicationStatusRunning, app.Status)
				} else {
					require.NoError(t, f.db.First(&compose, "id = ?", workloadID).Error)
					restored = compose.SourceConfig
					require.Equal(t, dockertypes.ApplicationStatusRunning, compose.Status)
				}
				require.Equal(t, true, restored["auto_deploy"])
				require.Equal(t, "preserved", restored["custom_option"])
				require.Equal(t, oldSnapshot.RepositoryID, restored["repository_id"])
				current, err := dockeraccess.BootstrapSnapshot(restored)
				require.NoError(t, err)
				require.Equal(t, payload.GitAccess, current)
				secretName := restored.GetString("gha_deploy_secret_name")
				newToken := secrets["/repos/org/repo101/actions/secrets/"+secretName]
				require.NotEmpty(t, newToken)
				require.NotEqual(t, oldToken, newToken)
				require.Len(t, workflows, 1)
				for path, content := range workflows {
					require.Contains(t, path, "/repos/org/repo101/")
					require.Contains(t, content, "push:")
					require.Contains(t, content, secretName)
					require.NotContains(t, content, oldToken)
				}
				deps := &dockerservices.ServiceDeps{ModuleDeps: service.ModuleDeps[*dockerrepos.Registry]{Dependencies: service.NewDependencies(f.db, &log, q, nil, nil), Repos: dockerrepos.NewRegistry(f.db)}, GitProviders: f.service.ProviderFactory()}
				var deployment *dockermodels.Deployment
				if kind == "application" {
					deployment, err = dockerservices.NewApplicationService(deps).Deploy(ctx, workloadID, project.ID, project.ServerID, team, user)
				} else {
					deployment, err = dockerservices.NewComposeService(deps).Deploy(ctx, workloadID, project.ID, project.ServerID, team, user)
				}
				require.NoError(t, err, "normal Deploy works without a manual workflow resync")
				require.NotNil(t, deployment)
				require.Len(t, dispatched, 1)
				require.Contains(t, dispatched[0], restored.GetString("gha_workflow_path")[len(".github/workflows/"):])
				image := restored.GetString("gha_image_repository") + ":new"
				require.Equal(t, http.StatusUnauthorized, callback(oldToken, "stale", image))
				require.Equal(t, http.StatusOK, callback(newToken, "restored", image), "fresh workflow can enqueue a deployment")
				before := len(workflows)
				require.NoError(t, job.Handle(ctx), "duplicate recovery delivery is safe")
				require.Len(t, workflows, before)
				require.Equal(t, http.StatusOK, callback(newToken, "after-duplicate", image))
				var otherAfter models.SourceControl
				require.NoError(t, f.db.First(&otherAfter, "id = ?", other.id).Error)
				require.Equal(t, otherBefore, otherAfter)
				require.NoError(t, f.service.Disconnect(ctx, connection.id, team, user))
				require.Error(t, job.Handle(ctx), "a delayed recovery cannot undo another disconnect")
				require.Equal(t, http.StatusUnauthorized, callback(newToken, "disconnected-again", image))
				var unauthorized int64
				require.NoError(t, f.db.Model(&dockermodels.Deployment{}).Where("gha_run_id IN ?", []string{"before", "before-repair", "stale", "disconnected-again"}).Count(&unauthorized).Error)
				require.Zero(t, unauthorized, fmt.Sprintf("%s callbacks never queued under old authority", kind))
			})
		}
	}
}
