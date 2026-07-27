package handlers

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	gofiber "github.com/gofiber/fiber/v2"
	"github.com/oklog/ulid/v2"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	dockerjobs "github.com/kkz6/launch-go/internal/modules/docker/jobs"
	dockermodels "github.com/kkz6/launch-go/internal/modules/docker/models"
	dockerrepositories "github.com/kkz6/launch-go/internal/modules/docker/repositories"
	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
	gitmodels "github.com/kkz6/launch-go/internal/modules/git/models"
	gitproviders "github.com/kkz6/launch-go/internal/modules/git/providers"
	gittypes "github.com/kkz6/launch-go/internal/modules/git/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/queue"
)

// GHAWebhookHandler implements the four routes the GitHub Actions
// workflow calls back into on success/failure:
//
//	POST /api/webhooks/docker/applications/:id/deploy
//	POST /api/webhooks/docker/applications/:id/status
//	POST /api/webhooks/docker/composes/:id/deploy
//	POST /api/webhooks/docker/composes/:id/status
//
// Authentication is per-workload bearer token. We never look up the
// workload by team — the routes mount OUTSIDE team-auth middleware so
// GitHub's runner can reach them without a JWT.
//
// On a successful deploy notify, the handler:
//
//  1. Auth: sha256(bearer) ConstantTimeCompare against stored hash
//  2. Validate image_tag prefix matches configured gha_image_repository
//  3. Upsert docker_deployments by (target_id, gha_run_id) for idempotency
//  4. Mint a fresh GitHub App installation token for GHCR pull auth
//  5. Enqueue deploy_application with the override-image fields so the
//     existing image-source code path runs against the built image
//
// queue + gitProviders can be nil in tests; when nil the handler still
// records the deployment row but skips the enqueue + token mint. The
// row-level idempotency tests don't need the actual queue.
type GHAWebhookHandler struct {
	db           *gorm.DB
	queue        *queue.Client
	gitProviders *gitproviders.ProviderFactory
	logger       *zerolog.Logger
}

// GHAWebhookHandlerConfig groups the handler's collaborators so the
// constructor doesn't keep accumulating positional arguments.
type GHAWebhookHandlerConfig struct {
	DB           *gorm.DB
	Queue        *queue.Client
	GitProviders *gitproviders.ProviderFactory
	Logger       *zerolog.Logger
}

const ghaPersistenceTimeout = 30 * time.Second

// ghaContext keeps webhook persistence bounded without inheriting the
// fasthttp request context, which is canceled as soon as Fiber finishes the
// handler. The database write must be allowed to finish, but never hang
// indefinitely after the callback has been accepted.
func ghaContext(c *gofiber.Ctx) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(c.Context()), ghaPersistenceTimeout)
}

func NewGHAWebhookHandler(cfg GHAWebhookHandlerConfig) *GHAWebhookHandler {
	return &GHAWebhookHandler{
		db:           cfg.DB,
		queue:        cfg.Queue,
		gitProviders: cfg.GitProviders,
		logger:       cfg.Logger,
	}
}

// applicationDeployPayload is the success-notify body from
// gha_application.yml.tmpl.
//
// GHCRPullToken is a short-lived (~1h) bearer the workflow minted
// via GitHub's token-exchange endpoint, scoped to
// `repository:<owner>/<repo>:pull`. It lets the deploy worker pull
// a private GHCR package without the customer wiring up a long-lived
// PAT or flipping the package public. See deploy_application.go +
// the bearer-redaction Stringer for how it travels onward without
// leaking into logs.
//
// GHCRPullTokenMintedAt is the unix timestamp (seconds, UTC) at
// which the workflow minted the bearer. Used by the worker to fail
// fast with a clear error if the deploy queue was stalled long
// enough that the bearer is about to expire — rather than letting
// docker pull report an opaque 401 after the fact.
type applicationDeployPayload struct {
	ImageTag              string `json:"image_tag"`
	CommitSHA             string `json:"commit_sha"`
	Branch                string `json:"branch"`
	RunID                 string `json:"run_id"`
	RunURL                string `json:"run_url"`
	GHCRPullToken         string `json:"ghcr_pull_token,omitempty"`
	GHCRPullTokenMintedAt string `json:"ghcr_pull_token_minted_at,omitempty"`
}

// composeDeployPayload is the success-notify body from
// gha_compose.yml.tmpl. Multi-service: ServiceImages is the per-service
// image map collected from the matrix build's artifacts.
//
// One bearer covers every service image since they all live under
// the same `<owner>/<repo>` GHCR namespace — GHCR's token-exchange
// scopes by repository name, not per-package.
type composeDeployPayload struct {
	ServiceImages         map[string]string `json:"service_images"`
	CommitSHA             string            `json:"commit_sha"`
	Branch                string            `json:"branch"`
	RunID                 string            `json:"run_id"`
	RunURL                string            `json:"run_url"`
	GHCRPullToken         string            `json:"ghcr_pull_token,omitempty"`
	GHCRPullTokenMintedAt string            `json:"ghcr_pull_token_minted_at,omitempty"`
}

// statusPayload is the failure-notify body (same for both kinds).
type statusPayload struct {
	Status string `json:"status"` // currently always "failed"
	RunID  string `json:"run_id"`
	RunURL string `json:"run_url"`
}

// GHAApplicationDeploy handles a successful build notification from
// an application's GitHub Actions workflow. Records (or reuses) a
// docker_deployments row in `pending` status. Slice F will enqueue
// deploy_application from here.
func (h *GHAWebhookHandler) GHAApplicationDeploy(c *gofiber.Ctx) error {
	appID := c.Params("id")
	if appID == "" {
		return fiberutil.NotFound()
	}

	app, err := h.loadApplicationForGHA(appID)
	if err != nil {
		return err
	}

	if err := authenticateBearer(c, app.GHADeployTokenHash); err != nil {
		return err
	}

	var payload applicationDeployPayload
	if err := c.BodyParser(&payload); err != nil {
		return fiberutil.BadRequest("invalid JSON body")
	}
	if err := validateApplicationDeployPayload(payload); err != nil {
		return err
	}
	if err := validateImagePrefix(payload.ImageTag, app.SourceConfig); err != nil {
		return err
	}

	requestCtx, cancel := ghaContext(c)
	defer cancel()
	deployment, err := h.upsertGHADeployment(requestCtx, ghaDeploymentUpsert{
		TargetType:    "application",
		TargetID:      app.ID,
		TeamID:        app.TeamID,
		ServerID:      app.ServerID,
		RunID:         payload.RunID,
		RunURL:        payload.RunURL,
		CommitSHA:     payload.CommitSHA,
		ImageRef:      payload.ImageTag,
		TriggerSource: dockertypes.DeploymentTriggerGitHubActions,
	})
	if err != nil {
		return err
	}

	// Resolve a fresh installation token for GHCR pull auth as a
	// FALLBACK only. The workflow normally mints a per-build GHCR
	// bearer (payload.GHCRPullToken) and includes it in this
	// callback — that bearer is what actually works on private
	// packages (GitHub App install tokens can't pull private GHCR,
	// regardless of permissions; verified twice on real repos).
	// The install-token path is kept for backward-compat with
	// workflow files still on the pre-relay template + for public
	// packages where either path works.
	if err := h.enqueueApplicationDeploy(c.Context(), app, deployment, payload.ImageTag, payload.GHCRPullToken, payload.GHCRPullTokenMintedAt); err != nil {
		// Log + return 5xx so GitHub Actions retries the notify. The
		// idempotent upsert means the retry won't create a duplicate
		// deployment row — it'll find the one we just inserted and
		// re-attempt the enqueue.
		if h.logger != nil {
			h.logger.Error().Err(err).
				Str("application_id", app.ID).
				Str("deployment_id", deployment.ID).
				Msg("GHA webhook: enqueue failed")
		}
		return fiberutil.Internal("failed to queue deployment")
	}

	return fiberutil.OK(c, "Deployment queued", map[string]any{
		"deployment_id": deployment.ID,
	})
}

// enqueueApplicationDeploy picks the right GHCR pull credential and
// enqueues the deploy_application job. Skipped when queue or
// gitProviders are nil — the slice C tests use that shape.
//
// Credential selection (in order):
//
//  1. ghcrPullToken from the webhook payload — minted by the
//     workflow via token-exchange. This is the only path that works
//     for private GHCR packages. Username for the docker login is
//     hard-coded "oauth2" — GHCR ignores the value for bearer
//     auth but requires the field to be present.
//
//  2. Fresh GitHub App installation token via the source-control
//     install. Fallback for older workflow files that don't carry a
//     bearer; works fine for public packages, doesn't work for
//     private ones. Username "x-access-token" per the App
//     installation-token convention.
//
// `mintedAtUnix` is the workflow-stamped unix timestamp at which
// the bearer was minted; the deploy job checks the age before
// using it so a stalled queue surfaces a "bearer expired" error
// instead of an opaque 401.
func (h *GHAWebhookHandler) enqueueApplicationDeploy(
	ctx context.Context,
	app *dockermodels.Application,
	deployment *dockermodels.Deployment,
	imageTag string,
	ghcrPullToken string,
	mintedAtUnix string,
) error {
	if h.queue == nil {
		return nil // dry-run mode (tests)
	}

	username := "oauth2"
	password := strings.TrimSpace(ghcrPullToken)
	if password == "" {
		// Path 2: fallback to install token.
		if h.gitProviders == nil {
			return errors.New("gha webhook: git provider factory not wired")
		}
		token, err := h.resolveInstallationToken(ctx, app.SourceConfig)
		if err != nil {
			return fmt.Errorf("resolve installation token: %w", err)
		}
		username = "x-access-token"
		password = token
	}

	task, err := dockerjobs.NewDeployApplicationTaskFromGHA(
		app.ID,
		deployment.ID,
		app.ServerID,
		app.TeamID,
		imageTag,
		"ghcr.io",
		username,
		password,
		mintedAtUnix,
	)
	if err != nil {
		return fmt.Errorf("build deploy task: %w", err)
	}
	if _, err := h.queue.Enqueue(task); err != nil {
		return fmt.Errorf("enqueue deploy task: %w", err)
	}
	return nil
}

// resolveInstallationToken pulls the source_control_id from the
// workload's source_config, looks up the corresponding source_controls
// row, and asks the GitHub provider for a fresh installation token.
// Used to authenticate `docker login ghcr.io` from the deploy script.
func (h *GHAWebhookHandler) resolveInstallationToken(ctx context.Context, sourceConfig map[string]any) (string, error) {
	if sourceConfig == nil {
		return "", errors.New("source_config missing")
	}
	scID, _ := sourceConfig["source_control_id"].(string)
	if scID == "" {
		return "", errors.New("source_control_id missing from source_config")
	}

	var sc gitmodels.SourceControl
	if err := h.db.WithContext(ctx).Where("id = ?", scID).First(&sc).Error; err != nil {
		return "", fmt.Errorf("load source_control %s: %w", scID, err)
	}
	if sc.InstallationID == nil || *sc.InstallationID == "" {
		return "", errors.New("source_control has no installation_id")
	}

	provider, err := h.gitProviders.GetProvider(gitproviders.GitProviderType(gittypes.GitProviderGitHub))
	if err != nil {
		return "", fmt.Errorf("resolve github provider: %w", err)
	}
	gh, ok := provider.(*gitproviders.GitHubProvider)
	if !ok {
		return "", errors.New("github provider has unexpected type")
	}
	return gh.GetInstallationToken(ctx, *sc.InstallationID)
}

// deploymentStatusResponse is the slim status view the workflow's
// "Wait for Launch deploy to finish" poll loop reads. Error is only
// populated for failed rows so the workflow log can echo the reason
// inline rather than sending the operator hunting through Launch.
type deploymentStatusResponse struct {
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
}

// GHAApplicationDeploymentStatus returns the current status of a single
// deployment row so the GitHub Actions workflow can BLOCK until the
// deploy reaches a terminal state.
//
// Why this exists: the workflow mints a short-lived GHCR pull bearer and
// relays it to Launch, and the worker uses it to `docker pull` the
// private image on the target host. That bearer is derived from the
// run's GITHUB_TOKEN, which GitHub REVOKES the instant the job ends — so
// if the workflow finishes before the async worker pulls, the relayed
// token is already dead and the pull dies with
// "unauthenticated: User cannot be authenticated with the token
// provided." By polling this endpoint, the workflow keeps its job (and
// therefore the token) alive until the deploy is genuinely done.
// See gha_application.yml.tmpl "Wait for Launch deploy to finish".
func (h *GHAWebhookHandler) GHAApplicationDeploymentStatus(c *gofiber.Ctx) error {
	return h.deploymentStatus(c, "application")
}

// GHAComposeDeploymentStatus mirrors GHAApplicationDeploymentStatus for
// compose stacks.
func (h *GHAWebhookHandler) GHAComposeDeploymentStatus(c *gofiber.Ctx) error {
	return h.deploymentStatus(c, "compose")
}

// deploymentStatus is the shared body for the application/compose poll
// endpoints. Auth is the same per-workload bearer the deploy/status
// notifies use; we load the workload to get the stored token hash, then
// fetch the deployment scoped to (target_type, target_id, id) so a token
// for one workload can't read another workload's deployment rows.
func (h *GHAWebhookHandler) deploymentStatus(c *gofiber.Ctx, targetType string) error {
	id := c.Params("id")
	deploymentID := c.Params("deploymentId")
	if id == "" || deploymentID == "" {
		return fiberutil.NotFound()
	}

	var tokenHash *string
	var targetID string
	switch targetType {
	case "application":
		app, err := h.loadApplicationForGHA(id)
		if err != nil {
			return err
		}
		tokenHash, targetID = app.GHADeployTokenHash, app.ID
	case "compose":
		compose, err := h.loadComposeForGHA(id)
		if err != nil {
			return err
		}
		tokenHash, targetID = compose.GHADeployTokenHash, compose.ID
	default:
		return fiberutil.NotFound()
	}

	if err := authenticateBearer(c, tokenHash); err != nil {
		return err
	}

	var dep dockermodels.Deployment
	err := h.db.Where(
		"id = ? AND target_type = ? AND target_id = ?",
		deploymentID, targetType, targetID,
	).First(&dep).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fiberutil.NotFound()
		}
		return err
	}

	resp := deploymentStatusResponse{Status: dep.Status.String()}
	if dep.Error != nil {
		resp.Error = *dep.Error
	}
	if dep.FinishedAt != nil {
		resp.FinishedAt = dep.FinishedAt.UTC().Format(time.RFC3339)
	}
	return fiberutil.OK(c, "Deployment status", resp)
}

// GHAApplicationStatus handles a failure-only notification (the
// `if: failure()` step in the workflow). Marks the corresponding
// deployment row as failed.
func (h *GHAWebhookHandler) GHAApplicationStatus(c *gofiber.Ctx) error {
	appID := c.Params("id")
	if appID == "" {
		return fiberutil.NotFound()
	}

	app, err := h.loadApplicationForGHA(appID)
	if err != nil {
		return err
	}
	if err := authenticateBearer(c, app.GHADeployTokenHash); err != nil {
		return err
	}

	var payload statusPayload
	if err := c.BodyParser(&payload); err != nil {
		return fiberutil.BadRequest("invalid JSON body")
	}
	if payload.RunID == "" {
		return fiberutil.Validation("run_id is required")
	}

	requestCtx, cancel := ghaContext(c)
	defer cancel()
	if err := h.markGHADeploymentFailed(requestCtx, "application", app.ID, app.TeamID, app.ServerID, payload); err != nil {
		return err
	}

	return fiberutil.OK(c, "Status recorded", nil)
}

// GHAComposeDeploy mirrors GHAApplicationDeploy for compose stacks.
// The deployment row stores the first service's image as ImageRef so
// the existing deployment-list UI has something to render; the full
// service_images map is persisted on docker_deployments via a new JSON
// column in a future slice (or applied by deploy_compose from the
// webhook payload directly).
func (h *GHAWebhookHandler) GHAComposeDeploy(c *gofiber.Ctx) error {
	composeID := c.Params("id")
	if composeID == "" {
		return fiberutil.NotFound()
	}

	compose, err := h.loadComposeForGHA(composeID)
	if err != nil {
		return err
	}
	if err := authenticateBearer(c, compose.GHADeployTokenHash); err != nil {
		return err
	}

	var payload composeDeployPayload
	if err := c.BodyParser(&payload); err != nil {
		return fiberutil.BadRequest("invalid JSON body")
	}
	if err := validateComposeDeployPayload(payload); err != nil {
		return err
	}
	// Every service image must point at the same configured repository
	// so we don't accidentally pull from an attacker-controlled tag.
	for svc, img := range payload.ServiceImages {
		if err := validateImagePrefix(img, compose.SourceConfig); err != nil {
			return fiberutil.Validation(fmt.Sprintf("service %q: %s", svc, err.Error()))
		}
	}

	// Pick a deterministic "primary" image for the deployment-list
	// preview. Sorting by service name keeps two runs of the same
	// service set picking the same primary, so the UI doesn't flicker
	// between deploys.
	primaryImage := primaryServiceImage(payload.ServiceImages)

	requestCtx, cancel := ghaContext(c)
	defer cancel()
	deployment, err := h.upsertGHADeployment(requestCtx, ghaDeploymentUpsert{
		TargetType:    "compose",
		TargetID:      compose.ID,
		TeamID:        compose.TeamID,
		ServerID:      compose.ServerID,
		RunID:         payload.RunID,
		RunURL:        payload.RunURL,
		CommitSHA:     payload.CommitSHA,
		ImageRef:      primaryImage,
		TriggerSource: dockertypes.DeploymentTriggerGitHubActions,
	})
	if err != nil {
		return err
	}

	if err := h.enqueueComposeDeploy(c.Context(), compose, deployment, payload.ServiceImages, payload.GHCRPullToken, payload.GHCRPullTokenMintedAt); err != nil {
		if h.logger != nil {
			h.logger.Error().Err(err).
				Str("compose_id", compose.ID).
				Str("deployment_id", deployment.ID).
				Msg("GHA compose webhook: enqueue failed")
		}
		return fiberutil.Internal("failed to queue deployment")
	}

	return fiberutil.OK(c, "Deployment queued", map[string]any{
		"deployment_id": deployment.ID,
	})
}

// enqueueComposeDeploy mirrors enqueueApplicationDeploy for compose
// stacks. Credential selection rules + redaction semantics are
// identical — see the application path for the longer comment.
func (h *GHAWebhookHandler) enqueueComposeDeploy(
	ctx context.Context,
	compose *dockermodels.Compose,
	deployment *dockermodels.Deployment,
	serviceImages map[string]string,
	ghcrPullToken string,
	mintedAtUnix string,
) error {
	if h.queue == nil {
		return nil
	}
	username := "oauth2"
	password := strings.TrimSpace(ghcrPullToken)
	if password == "" {
		if h.gitProviders == nil {
			return errors.New("gha webhook: git provider factory not wired")
		}
		token, err := h.resolveInstallationToken(ctx, compose.SourceConfig)
		if err != nil {
			return fmt.Errorf("resolve installation token: %w", err)
		}
		username = "x-access-token"
		password = token
	}
	task, err := dockerjobs.NewDeployComposeTaskFromGHA(
		compose.ID,
		deployment.ID,
		compose.ServerID,
		compose.TeamID,
		serviceImages,
		"ghcr.io",
		username,
		password,
		mintedAtUnix,
	)
	if err != nil {
		return fmt.Errorf("build deploy task: %w", err)
	}
	if _, err := h.queue.Enqueue(task); err != nil {
		return fmt.Errorf("enqueue deploy task: %w", err)
	}
	return nil
}

// GHAComposeStatus mirrors GHAApplicationStatus for composes.
func (h *GHAWebhookHandler) GHAComposeStatus(c *gofiber.Ctx) error {
	composeID := c.Params("id")
	if composeID == "" {
		return fiberutil.NotFound()
	}

	compose, err := h.loadComposeForGHA(composeID)
	if err != nil {
		return err
	}
	if err := authenticateBearer(c, compose.GHADeployTokenHash); err != nil {
		return err
	}

	var payload statusPayload
	if err := c.BodyParser(&payload); err != nil {
		return fiberutil.BadRequest("invalid JSON body")
	}
	if payload.RunID == "" {
		return fiberutil.Validation("run_id is required")
	}

	requestCtx, cancel := ghaContext(c)
	defer cancel()
	if err := h.markGHADeploymentFailed(requestCtx, "compose", compose.ID, compose.TeamID, compose.ServerID, payload); err != nil {
		return err
	}
	return fiberutil.OK(c, "Status recorded", nil)
}

// --- loaders ---------------------------------------------------------

func (h *GHAWebhookHandler) loadApplicationForGHA(id string) (*dockermodels.Application, error) {
	var app dockermodels.Application
	if err := h.db.Where("id = ?", id).First(&app).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}
	if app.BuildLocation != dockertypes.BuildLocationGitHubActions {
		// Don't leak whether the ID exists for non-GHA apps — same 404
		// shape as a missing row so probing can't enumerate IDs.
		return nil, fiberutil.NotFound()
	}
	if app.GHADeployTokenHash == nil || *app.GHADeployTokenHash == "" {
		// Bootstrap hasn't run yet — auth would fail anyway, but 404
		// keeps the response surface uniform.
		return nil, fiberutil.NotFound()
	}
	return &app, nil
}

func (h *GHAWebhookHandler) loadComposeForGHA(id string) (*dockermodels.Compose, error) {
	var compose dockermodels.Compose
	if err := h.db.Where("id = ?", id).First(&compose).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}
	if compose.BuildLocation != dockertypes.BuildLocationGitHubActions {
		return nil, fiberutil.NotFound()
	}
	if compose.GHADeployTokenHash == nil || *compose.GHADeployTokenHash == "" {
		return nil, fiberutil.NotFound()
	}
	return &compose, nil
}

// --- auth ------------------------------------------------------------

// authenticateBearer parses Authorization: Bearer <token>, sha256s
// the raw token, and constant-time-compares against the stored hash.
// Returns nil on match, 401 on mismatch / missing header — no body
// detail (defense against enumeration).
func authenticateBearer(c *gofiber.Ctx, storedHash *string) error {
	if storedHash == nil || *storedHash == "" {
		return fiberutil.Unauthorized()
	}
	header := c.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return fiberutil.Unauthorized()
	}
	raw := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	if raw == "" {
		return fiberutil.Unauthorized()
	}
	got := sha256.Sum256([]byte(raw))
	gotHex := hex.EncodeToString(got[:])
	if subtle.ConstantTimeCompare([]byte(gotHex), []byte(*storedHash)) != 1 {
		return fiberutil.Unauthorized()
	}
	return nil
}

// --- payload validation ---------------------------------------------

func validateApplicationDeployPayload(p applicationDeployPayload) error {
	if p.ImageTag == "" {
		return fiberutil.Validation("image_tag is required")
	}
	if p.RunID == "" {
		return fiberutil.Validation("run_id is required")
	}
	return nil
}

func validateComposeDeployPayload(p composeDeployPayload) error {
	if len(p.ServiceImages) == 0 {
		return fiberutil.Validation("service_images must contain at least one entry")
	}
	if p.RunID == "" {
		return fiberutil.Validation("run_id is required")
	}
	return nil
}

// validateImagePrefix confirms the image points at the GHCR repository
// we configured on enable. Without this, a leaked deploy token would
// let an attacker swap our deploy target to their own image — the
// token only proves "I can talk to your webhook," it doesn't bind the
// image source.
func validateImagePrefix(image string, sourceConfig map[string]any) error {
	if sourceConfig == nil {
		return fiberutil.Validation("workload has no source configuration recorded")
	}
	prefix, _ := sourceConfig["gha_image_repository"].(string)
	if prefix == "" {
		return fiberutil.Validation("workload has no GHA image repository configured")
	}
	// Allow either "<prefix>:tag" or "<prefix>" exactly. Splitting on
	// ":" is sufficient because GHCR registry hosts include no colon
	// outside the optional `:port` we never use.
	host := image
	if idx := strings.IndexByte(image, ':'); idx >= 0 {
		host = image[:idx]
	}
	if host != prefix {
		return fiberutil.Validation(fmt.Sprintf("image %q is not in the configured repository %q", image, prefix))
	}
	return nil
}

// --- deployment upsert ----------------------------------------------

type ghaDeploymentUpsert struct {
	TargetType    string
	TargetID      string
	TeamID        string
	ServerID      string
	RunID         string
	RunURL        string
	CommitSHA     string
	ImageRef      string
	TriggerSource dockertypes.DeploymentTriggerSource
}

// upsertGHADeployment is the idempotency hinge — the partial UNIQUE
// index on (target_type, target_id, gha_run_id) WHERE gha_run_id IS
// NOT NULL guarantees two notifies for the same run reuse the row
// rather than spawning a duplicate. We do the lookup ourselves
// (rather than ON CONFLICT) because GORM's OnConflict-ignore returns
// no useful "did we insert or hit conflict" signal across drivers.
func (h *GHAWebhookHandler) upsertGHADeployment(ctx context.Context, input ghaDeploymentUpsert) (*dockermodels.Deployment, error) {
	var existing dockermodels.Deployment
	err := h.db.Where(
		"target_type = ? AND target_id = ? AND gha_run_id = ?",
		input.TargetType, input.TargetID, input.RunID,
	).First(&existing).Error
	if err == nil {
		// Update in place — keep the row id, refresh the volatile
		// fields. Status stays at whatever the existing row says,
		// because the second notify is most likely a retry of the
		// success message we already accepted.
		updates := map[string]any{
			"gha_run_url": input.RunURL,
			"image_ref":   input.ImageRef,
			"commit_sha":  input.CommitSHA,
		}
		if err := h.db.Model(&existing).Updates(updates).Error; err != nil {
			return nil, err
		}
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// No row for this run yet. Adopt the dispatch-time placeholder (pending,
	// github_actions, no run id) if one exists, so a single trigger doesn't
	// leave an orphaned Pending row beside the webhook-created one (#92).
	if ph, ok := h.claimGHAPlaceholder(input.TargetType, input.TargetID); ok {
		runID := input.RunID
		updates := map[string]any{
			"gha_run_id":     runID,
			"gha_run_url":    input.RunURL,
			"image_ref":      input.ImageRef,
			"commit_sha":     input.CommitSHA,
			"trigger_source": input.TriggerSource,
		}
		if err := h.db.Model(ph).Updates(updates).Error; err != nil {
			return nil, err
		}
		return ph, nil
	}

	commitSHA := input.CommitSHA
	imageRef := input.ImageRef
	runID := input.RunID
	runURL := input.RunURL
	deployment := &dockermodels.Deployment{
		TargetType:    input.TargetType,
		TargetID:      input.TargetID,
		Status:        dockertypes.DeploymentStatusPending,
		TriggerSource: input.TriggerSource,
		CommitSHA:     &commitSHA,
		ImageRef:      &imageRef,
		GHARunID:      &runID,
		GHARunURL:     &runURL,
	}
	// Hand-set the IDs that BaseModel + TeamScoped + ServerScoped
	// expect — we're a webhook with no team context, the workload row
	// has already told us which team owns it.
	deployment.ID = newULID()
	deployment.TeamID = input.TeamID
	deployment.ServerID = input.ServerID

	if err := dockerrepositories.NewDeploymentRepository(h.db).Create(ctx, deployment); err != nil {
		return nil, err
	}
	return deployment, nil
}

func (h *GHAWebhookHandler) markGHADeploymentFailed(ctx context.Context, targetType, targetID, teamID, serverID string, payload statusPayload) error {
	var existing dockermodels.Deployment
	err := h.db.Where(
		"target_type = ? AND target_id = ? AND gha_run_id = ?",
		targetType, targetID, payload.RunID,
	).First(&existing).Error

	now := time.Now()
	if err == nil {
		updates := map[string]any{
			"status":      dockertypes.DeploymentStatusFailed,
			"gha_run_url": payload.RunURL,
			"finished_at": now,
		}
		return h.db.Model(&existing).Updates(updates).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	// No prior deploy row for this run. Adopt the dispatch-time placeholder
	// (pending, github_actions, no run id) if one exists, so a failed run
	// doesn't leave an orphaned Pending row beside it (#92).
	if ph, ok := h.claimGHAPlaceholder(targetType, targetID); ok {
		runID := payload.RunID
		updates := map[string]any{
			"status":      dockertypes.DeploymentStatusFailed,
			"gha_run_id":  runID,
			"gha_run_url": payload.RunURL,
			"finished_at": now,
		}
		return h.db.Model(ph).Updates(updates).Error
	}
	// Otherwise materialise a failed row so the deployment list still shows
	// "Run #N failed" with a link to the GitHub Actions page.
	runID := payload.RunID
	runURL := payload.RunURL
	deployment := &dockermodels.Deployment{
		TargetType:    targetType,
		TargetID:      targetID,
		Status:        dockertypes.DeploymentStatusFailed,
		TriggerSource: dockertypes.DeploymentTriggerGitHubActions,
		FinishedAt:    &now,
		GHARunID:      &runID,
		GHARunURL:     &runURL,
	}
	deployment.ID = newULID()
	deployment.TeamID = teamID
	deployment.ServerID = serverID
	return dockerrepositories.NewDeploymentRepository(h.db).Create(ctx, deployment)
}

// claimGHAPlaceholder returns the most recent dispatch-time placeholder
// deployment for a target — a github_actions row still in Pending with no
// gha_run_id assigned — so the first webhook notify for a run can adopt it
// instead of spawning a duplicate row. Returns (nil, false) when none exists.
func (h *GHAWebhookHandler) claimGHAPlaceholder(targetType, targetID string) (*dockermodels.Deployment, bool) {
	var ph dockermodels.Deployment
	err := h.db.Where(
		"target_type = ? AND target_id = ? AND status = ? AND trigger_source = ? AND (gha_run_id IS NULL OR gha_run_id = '')",
		targetType, targetID,
		dockertypes.DeploymentStatusPending,
		dockertypes.DeploymentTriggerGitHubActions,
	).Order("created_at DESC").First(&ph).Error
	if err != nil {
		return nil, false
	}
	return &ph, true
}

// --- helpers --------------------------------------------------------

// primaryServiceImage picks a stable "primary" image for ImageRef on
// compose deployments. Sort service names lexicographically, return
// the first. Empty map returns "".
func primaryServiceImage(svcImages map[string]string) string {
	if len(svcImages) == 0 {
		return ""
	}
	names := make([]string, 0, len(svcImages))
	for name := range svcImages {
		names = append(names, name)
	}
	sortStrings(names)
	return svcImages[names[0]]
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

// newULID is a thin wrapper so tests can stub the deployment id
// generator if needed — but right now we just call straight through
// to the project's standard generator.
func newULID() string {
	return ulid.Make().String()
}
