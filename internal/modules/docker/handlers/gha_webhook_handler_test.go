package handlers

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	gofiber "github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	dockermodels "github.com/kkz6/launch-go/internal/modules/docker/models"
	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	"github.com/kkz6/launch-go/internal/pkg/util"
)

// rawToken is the cleartext bearer the workflow would send; the DB
// stores its sha256. Used across cases so we don't repeat the hash
// fixture in every test setup.
const rawToken = "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

func hashOf(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// setupHandler returns a wired-up handler + fiber app + an application
// row with build_location=github_actions and the configured GHCR
// repository. The tests can then post arbitrary bodies against the
// returned URL prefixes. Compose tests build their own via the
// embedded helper.
func setupHandler(t *testing.T) (*gofiber.App, *gorm.DB, *dockermodels.Application) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&dockermodels.Application{}, &dockermodels.Deployment{}))

	tokenHash := hashOf(rawToken)
	app := &dockermodels.Application{
		ProjectID:          util.NewULID(),
		Name:               "test-app",
		SourceType:         dockertypes.SourceTypeGit,
		Status:             dockertypes.ApplicationStatusIdle,
		InternalPort:       80,
		BuildLocation:      dockertypes.BuildLocationGitHubActions,
		GHADeployTokenHash: &tokenHash,
		SourceConfig: dbtype.JSONMap{
			"gha_image_repository": "ghcr.io/kkz6/test-repo",
			"repository":           "kkz6/test-repo",
			"branch":               "main",
		},
	}
	app.ID = util.NewULID()
	app.TeamID = util.NewULID()
	app.ServerID = util.NewULID()
	require.NoError(t, db.Create(app).Error)

	h := NewGHAWebhookHandler(GHAWebhookHandlerConfig{DB: db})
	fapp := gofiber.New(gofiber.Config{
		ErrorHandler: func(c *gofiber.Ctx, err error) error {
			// Use Fiber's stock error mapping so our fiberutil.* helpers
			// translate to the right status codes.
			return gofiber.DefaultErrorHandler(c, err)
		},
	})
	fapp.Post("/api/webhooks/docker/applications/:id/deploy", h.GHAApplicationDeploy)
	fapp.Post("/api/webhooks/docker/applications/:id/status", h.GHAApplicationStatus)
	return fapp, db, app
}

func setupComposeHandler(t *testing.T) (*gofiber.App, *gorm.DB, *dockermodels.Compose) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&dockermodels.Compose{}, &dockermodels.Deployment{}))

	tokenHash := hashOf(rawToken)
	compose := &dockermodels.Compose{
		ProjectID:          util.NewULID(),
		Name:               "test-stack",
		ComposeSourceType:  "git",
		Status:             dockertypes.ApplicationStatusIdle,
		BuildLocation:      dockertypes.BuildLocationGitHubActions,
		GHADeployTokenHash: &tokenHash,
		SourceConfig: dbtype.JSONMap{
			"gha_image_repository": "ghcr.io/kkz6/test-stack",
		},
	}
	compose.ID = util.NewULID()
	compose.TeamID = util.NewULID()
	compose.ServerID = util.NewULID()
	require.NoError(t, db.Create(compose).Error)

	h := NewGHAWebhookHandler(GHAWebhookHandlerConfig{DB: db})
	fapp := gofiber.New(gofiber.Config{
		ErrorHandler: gofiber.DefaultErrorHandler,
	})
	fapp.Post("/api/webhooks/docker/composes/:id/deploy", h.GHAComposeDeploy)
	fapp.Post("/api/webhooks/docker/composes/:id/status", h.GHAComposeStatus)
	return fapp, db, compose
}

func post(t *testing.T, app *gofiber.App, path, bearer string, body any) (*http.Response, []byte) {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	out, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, out
}

func TestGHAApplicationDeploy_HappyPath_CreatesPendingDeployment(t *testing.T) {
	app, db, application := setupHandler(t)

	body := applicationDeployPayload{
		ImageTag:  "ghcr.io/kkz6/test-repo:launch-abc1234",
		CommitSHA: "abc1234567890",
		Branch:    "main",
		RunID:     "11111111",
		RunURL:    "https://github.com/kkz6/test-repo/actions/runs/11111111",
	}
	resp, _ := post(t, app, "/api/webhooks/docker/applications/"+application.ID+"/deploy", rawToken, body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var rows []dockermodels.Deployment
	require.NoError(t, db.Find(&rows).Error)
	require.Len(t, rows, 1)
	assert.Equal(t, "application", rows[0].TargetType)
	assert.Equal(t, application.ID, rows[0].TargetID)
	assert.Equal(t, dockertypes.DeploymentStatusPending, rows[0].Status)
	assert.Equal(t, dockertypes.DeploymentTriggerGitHubActions, rows[0].TriggerSource)
	require.NotNil(t, rows[0].GHARunID)
	assert.Equal(t, "11111111", *rows[0].GHARunID)
}

func TestGHAApplicationDeploy_BadToken_Returns401_NoRow(t *testing.T) {
	app, db, application := setupHandler(t)

	body := applicationDeployPayload{
		ImageTag: "ghcr.io/kkz6/test-repo:launch-x", RunID: "22",
	}
	resp, _ := post(t, app, "/api/webhooks/docker/applications/"+application.ID+"/deploy", "totally-wrong-token", body)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	var count int64
	require.NoError(t, db.Model(&dockermodels.Deployment{}).Count(&count).Error)
	assert.Equal(t, int64(0), count, "no deployment should be created when auth fails")
}

func TestGHAApplicationDeploy_MissingBearer_Returns401(t *testing.T) {
	app, _, application := setupHandler(t)
	body := applicationDeployPayload{ImageTag: "ghcr.io/kkz6/test-repo:x", RunID: "1"}
	resp, _ := post(t, app, "/api/webhooks/docker/applications/"+application.ID+"/deploy", "", body)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestGHAApplicationDeploy_IdempotentRetry_SameRunReusesRow(t *testing.T) {
	app, db, application := setupHandler(t)

	body := applicationDeployPayload{
		ImageTag:  "ghcr.io/kkz6/test-repo:launch-xyz",
		CommitSHA: "deadbeef",
		Branch:    "main",
		RunID:     "55555555",
		RunURL:    "https://github.com/kkz6/test-repo/actions/runs/55555555",
	}
	// First call.
	resp1, _ := post(t, app, "/api/webhooks/docker/applications/"+application.ID+"/deploy", rawToken, body)
	assert.Equal(t, http.StatusOK, resp1.StatusCode)

	// Same run_id retried — should NOT create a second row.
	resp2, _ := post(t, app, "/api/webhooks/docker/applications/"+application.ID+"/deploy", rawToken, body)
	assert.Equal(t, http.StatusOK, resp2.StatusCode)

	var count int64
	require.NoError(t, db.Model(&dockermodels.Deployment{}).Count(&count).Error)
	assert.Equal(t, int64(1), count, "two notifies with the same run_id must reuse the existing row")

	// A different run_id should produce a second row.
	body.RunID = "66666666"
	resp3, _ := post(t, app, "/api/webhooks/docker/applications/"+application.ID+"/deploy", rawToken, body)
	assert.Equal(t, http.StatusOK, resp3.StatusCode)
	require.NoError(t, db.Model(&dockermodels.Deployment{}).Count(&count).Error)
	assert.Equal(t, int64(2), count)
}

// The workflow's "Mint GHCR pull token" step adds two new fields to
// the success payload — ghcr_pull_token + ghcr_pull_token_minted_at.
// Older workflows (pre-bearer-relay) don't send them. Both shapes
// must continue to produce a clean 200 + deployment row.
func TestGHAApplicationDeploy_AcceptsGHCRBearerPayload(t *testing.T) {
	app, db, application := setupHandler(t)

	body := applicationDeployPayload{
		ImageTag:              "ghcr.io/kkz6/test-repo:launch-bearer",
		CommitSHA:             "abc1234567890",
		Branch:                "main",
		RunID:                 "with-bearer-001",
		RunURL:                "https://github.com/kkz6/test-repo/actions/runs/with-bearer-001",
		GHCRPullToken:         "ghs_VeryDefinitelyFakeBearerForTestingOnly_xxxxxxxxxxx",
		GHCRPullTokenMintedAt: "1717000000",
	}
	resp, raw := post(t, app, "/api/webhooks/docker/applications/"+application.ID+"/deploy", rawToken, body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	// The token itself MUST NOT appear in the response body —
	// nothing the customer's UI receives should ever contain it.
	assert.NotContains(t, string(raw), body.GHCRPullToken,
		"the bearer must never echo back to the caller")

	var rows []dockermodels.Deployment
	require.NoError(t, db.Find(&rows).Error)
	require.Len(t, rows, 1)
	assert.Equal(t, dockertypes.DeploymentStatusPending, rows[0].Status)
	// The bearer must also NOT land on the deployment row anywhere
	// (error / image_ref / log_path) — the row gets persisted long
	// before docker login runs, and operators view this through the
	// UI's View Logs sheet.
	if rows[0].ImageRef != nil {
		assert.NotContains(t, *rows[0].ImageRef, body.GHCRPullToken)
	}
	if rows[0].Error != nil {
		assert.NotContains(t, *rows[0].Error, body.GHCRPullToken)
	}
}

func TestGHAApplicationDeploy_RejectsImageOutsideConfiguredRepository(t *testing.T) {
	app, db, application := setupHandler(t)

	body := applicationDeployPayload{
		ImageTag: "ghcr.io/attacker/evil:latest", // not in gha_image_repository
		RunID:    "77",
	}
	resp, raw := post(t, app, "/api/webhooks/docker/applications/"+application.ID+"/deploy", rawToken, body)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	assert.Contains(t, string(raw), "not in the configured repository")

	var count int64
	require.NoError(t, db.Model(&dockermodels.Deployment{}).Count(&count).Error)
	assert.Equal(t, int64(0), count, "rejecting a bad image must not create a deployment row")
}

func TestGHAApplicationDeploy_NonGHAApp_Returns404(t *testing.T) {
	app, db, application := setupHandler(t)

	// Flip the app off GHA mode — webhook should now behave as if the
	// row doesn't exist.
	require.NoError(t, db.Model(application).Update("build_location", "server").Error)

	body := applicationDeployPayload{
		ImageTag: "ghcr.io/kkz6/test-repo:launch-z", RunID: "1",
	}
	resp, _ := post(t, app, "/api/webhooks/docker/applications/"+application.ID+"/deploy", rawToken, body)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestGHAApplicationStatus_MarksDeploymentFailed(t *testing.T) {
	app, db, application := setupHandler(t)

	// Seed a successful-deploy row first.
	deploy := applicationDeployPayload{
		ImageTag: "ghcr.io/kkz6/test-repo:launch-a", RunID: "200",
		RunURL: "https://x/runs/200",
	}
	post(t, app, "/api/webhooks/docker/applications/"+application.ID+"/deploy", rawToken, deploy)

	// Now a failure notify for the same run_id.
	fail := statusPayload{Status: "failed", RunID: "200", RunURL: "https://x/runs/200"}
	resp, _ := post(t, app, "/api/webhooks/docker/applications/"+application.ID+"/status", rawToken, fail)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var rows []dockermodels.Deployment
	require.NoError(t, db.Find(&rows).Error)
	require.Len(t, rows, 1)
	assert.Equal(t, dockertypes.DeploymentStatusFailed, rows[0].Status)
	require.NotNil(t, rows[0].FinishedAt)
}

func TestGHAApplicationStatus_FailureWithoutPriorDeployCreatesFailedRow(t *testing.T) {
	app, db, application := setupHandler(t)

	fail := statusPayload{Status: "failed", RunID: "999", RunURL: "https://x/runs/999"}
	resp, _ := post(t, app, "/api/webhooks/docker/applications/"+application.ID+"/status", rawToken, fail)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var rows []dockermodels.Deployment
	require.NoError(t, db.Find(&rows).Error)
	require.Len(t, rows, 1)
	assert.Equal(t, dockertypes.DeploymentStatusFailed, rows[0].Status)
}

func TestGHAComposeDeploy_AcceptsServiceImagesMap(t *testing.T) {
	app, db, compose := setupComposeHandler(t)

	body := composeDeployPayload{
		ServiceImages: map[string]string{
			"web":    "ghcr.io/kkz6/test-stack:launch-web-abc1234",
			"worker": "ghcr.io/kkz6/test-stack:launch-worker-abc1234",
		},
		CommitSHA: "deadbeef",
		Branch:    "main",
		RunID:     "1",
		RunURL:    "https://x/runs/1",
	}
	resp, _ := post(t, app, "/api/webhooks/docker/composes/"+compose.ID+"/deploy", rawToken, body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var rows []dockermodels.Deployment
	require.NoError(t, db.Find(&rows).Error)
	require.Len(t, rows, 1)
	assert.Equal(t, "compose", rows[0].TargetType)
	require.NotNil(t, rows[0].ImageRef)
	// Primary image is the lexicographically-first service ("web" < "worker").
	assert.Equal(t, "ghcr.io/kkz6/test-stack:launch-web-abc1234", *rows[0].ImageRef)
}

func TestGHAComposeDeploy_RejectsImageOutsideConfiguredRepository(t *testing.T) {
	app, _, compose := setupComposeHandler(t)

	body := composeDeployPayload{
		ServiceImages: map[string]string{
			"web": "ghcr.io/attacker/evil:latest",
		},
		RunID: "1",
	}
	resp, raw := post(t, app, "/api/webhooks/docker/composes/"+compose.ID+"/deploy", rawToken, body)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	assert.Contains(t, string(raw), "not in the configured repository")
}

func TestPrimaryServiceImage_IsDeterministic(t *testing.T) {
	m := map[string]string{
		"worker": "img-worker",
		"web":    "img-web",
		"queue":  "img-queue",
	}
	// "queue" is the lexicographically smallest key.
	for i := 0; i < 50; i++ {
		assert.Equal(t, "img-queue", primaryServiceImage(m))
	}
}
