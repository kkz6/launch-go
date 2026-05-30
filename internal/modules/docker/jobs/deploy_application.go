package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"github.com/kkz6/launch-go/internal/modules/docker/tasks"
	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

// TypeDeployApplication is the asynq task type for deploying a docker
// application.
const TypeDeployApplication = "docker:deploy_application"

// DeployApplicationPayload travels through asynq — IDs only, never model
// pointers (per CLAUDE.md "Callback data must only contain IDs").
//
// Override* fields are used by the GitHub Actions webhook path so a
// successful CI build can hand us a freshly-built GHCR image plus a
// short-lived installation token without persisting either on the
// application row. When any Override* is set, the deploy job uses the
// overrides verbatim instead of reading source_type/RegistryCredential
// off the application. The whole shape stays IDs-only — these are
// passed by the webhook handler from the GHA notify body, not loaded
// from the DB.
type DeployApplicationPayload struct {
	ApplicationID string `json:"application_id"`
	DeploymentID  string `json:"deployment_id"`
	ServerID      string `json:"server_id"`
	TeamID        string `json:"team_id"`

	// OverrideImage forces SourceType=image with this image. Set by the
	// GHA webhook with the tag the workflow just pushed to GHCR.
	OverrideImage string `json:"override_image,omitempty"`
	// OverrideRegistryURL / Username / Password override the docker
	// login credentials the deploy script uses before `docker pull`.
	// For GHA the URL is "ghcr.io", the username is "oauth2" (when
	// the workflow minted a GHCR pull bearer) or "x-access-token"
	// (when falling back to the GitHub App installation token), and
	// the password is the corresponding token.
	//
	// NOTE: OverrideRegistryPassword is a secret. The custom String()
	// method on this struct redacts it; if you add another channel
	// that formats this struct, mirror the redaction there.
	OverrideRegistryURL      string `json:"override_registry_url,omitempty"`
	OverrideRegistryUsername string `json:"override_registry_username,omitempty"`
	OverrideRegistryPassword string `json:"override_registry_password,omitempty"`
	// OverrideRegistryPasswordMintedAtUnix carries the unix-seconds
	// timestamp at which the GHCR pull bearer was minted (workflow
	// side, UTC). Used to fail fast with a clear error if the bearer
	// is about to expire by the time the deploy job actually runs —
	// rather than letting docker pull surface an opaque 401. Empty
	// when the password isn't a workflow-minted bearer (e.g. when
	// falling back to the GitHub App installation token path).
	OverrideRegistryPasswordMintedAtUnix string `json:"override_registry_password_minted_at_unix,omitempty"`
}

// String returns a redacted JSON view of the payload. Any code path
// that prints this struct (logger.Interface, fmt.Sprintf, asynq
// retry logs, debug dumps) ends up here — making the bearer
// IMPOSSIBLE to leak via accidental formatting.
//
// This is the SINGLE place to update if the payload ever gains
// another secret. Locked in by TestDeployApplicationPayload_StringRedaction.
func (p DeployApplicationPayload) String() string {
	redacted := p
	if redacted.OverrideRegistryPassword != "" {
		redacted.OverrideRegistryPassword = "[REDACTED]"
	}
	b, err := json.Marshal(redacted)
	if err != nil {
		return fmt.Sprintf("DeployApplicationPayload{app=%s, deploy=%s, marshal_err=%v}",
			p.ApplicationID, p.DeploymentID, err)
	}
	return string(b)
}

// DeployApplicationJob runs the deploy script on the docker server,
// parses the LAUNCH markers from the output to extract container_id +
// image_ref, persists those onto the application, and broadcasts the
// terminal state.
//
// We run the SSH task synchronously (Dispatch, not RunInBackground)
// because the deploy is bounded by the script's 30-minute timeout and
// we want the container_id back inline to write a single completed
// deployment row.
type DeployApplicationJob struct {
	Deps    *JobDeps
	Payload DeployApplicationPayload

	app        *models.Application
	deployment *models.Deployment
	project    *models.Project
	server     *servermodels.Server
}

// NewDeployApplicationJob is the constructor asynq picks up via
// pkgjobs.RegisterTyped — see register.go.
func NewDeployApplicationJob(p DeployApplicationPayload) pkgjobs.Handler {
	return &DeployApplicationJob{Deps: deps, Payload: p}
}

// Handle is the asynq entrypoint. Any error returned causes asynq to
// retry with backoff, so we only return errors that are genuinely
// transient (DB unavailable, etc.). User-error failures (image not
// found, build failure) are recorded on the deployment row and we
// return nil so asynq doesn't retry a build that will fail the same way.
func (j *DeployApplicationJob) Handle(ctx context.Context) error {
	if err := j.loadModels(ctx); err != nil {
		return err
	}

	// Fail fast if the GHCR pull bearer the workflow stamped on the
	// payload is about to expire. Without this, docker pull would
	// surface an opaque 401 with no hint why — and asynq would retry
	// the same stale-token deploy in a loop. The customer is far
	// better off seeing a clear "your queue was stalled; re-trigger
	// the workflow" message + a non-retriable failure than wasting
	// asynq retries on a token that's already dead. See
	// gha_webhook_handler.go for where mintedAtUnix gets stamped.
	if reason, expired := ghcrBearerExpiredSoon(j.Payload.OverrideRegistryPasswordMintedAtUnix, time.Now().UTC()); expired {
		j.Deps.Logger.Warn().
			Str("application_id", j.Payload.ApplicationID).
			Str("deployment_id", j.Payload.DeploymentID).
			Str("reason", reason).
			Msg("GHA deploy: GHCR pull bearer expired before deploy ran")
		j.markDeploymentFailed(ctx,
			"GHCR pull token expired before this deploy could run "+
				"(the queue was stalled for longer than the token's ~1h "+
				"lifetime). Re-trigger the workflow on GitHub to mint a "+
				"fresh token and try again.")
		return nil
	}

	now := time.Now().UTC()
	if err := j.Deps.Repos.Deployment().UpdateFields(ctx, j.deployment.ID, map[string]any{
		"status":     dockertypes.DeploymentStatusDeploying,
		"started_at": now,
	}); err != nil {
		j.Deps.Logger.Error().Err(err).Msg("failed to mark deployment as deploying")
	}

	// Flip application status to "building" so the UI shows a spinner
	// while the SSH task runs. The terminal status is set below.
	if err := j.Deps.Repos.Application().UpdateFields(ctx, j.app.ID, map[string]any{
		"status": dockertypes.ApplicationStatusBuilding,
	}); err != nil {
		j.Deps.Logger.Error().Err(err).Msg("failed to mark application as building")
	}
	j.broadcast("docker.application.deploying", map[string]any{
		"application_id": j.app.ID,
		"deployment_id":  j.deployment.ID,
		"status":         "building",
	})

	cfg := tasks.DeployConfig{
		DeploymentID:  j.deployment.ID,
		ProjectSlug:   tasks.SlugFromName(j.project.Name),
		AppSlug:       tasks.SlugFromName(j.app.Name),
		ContainerName: tasks.ContainerNameFor(j.project, j.app),
		SourceType:    j.app.SourceType,
	}
	hydrateSourceConfig(&cfg, j.app)

	// GitHub-Actions override path. Pure function so it's unit-testable
	// without standing up a worker or stubbing the DB.
	applyDeployApplicationOverrides(&cfg, j.Payload)
	// Rewrite the git URL with embedded credentials when a connected
	// source-control account was selected on the application. No-op for
	// public repos. Failures fall back to the original URL — see
	// resolveAuthenticatedCloneURL.
	cfg.GitRepo = j.Deps.resolveAuthenticatedCloneURL(ctx, map[string]any(j.app.SourceConfig), cfg.GitRepo)

	// Resolve registry authentication for image sources. Either path
	// (saved credential / inline) lands plaintext creds on the cfg
	// — the deploy script pipes them to `docker login --password-
	// stdin` so they never appear in `ps`. Best-effort: if the saved
	// credential lookup fails (deleted between create and deploy)
	// we fall through to no-auth and the pull will surface the real
	// "image not found" error on the deploy log.
	if j.Payload.OverrideImage == "" && j.app.SourceType == dockertypes.SourceTypeImage {
		if j.app.RegistryCredentialID != nil && *j.app.RegistryCredentialID != "" {
			if cred, err := j.Deps.Repos.RegistryCredential().FindByIDForTeam(
				ctx, *j.app.RegistryCredentialID, j.app.TeamID,
			); err == nil {
				if cred.RegistryURL != nil {
					cfg.RegistryURL = *cred.RegistryURL
				}
				cfg.RegistryUsername = cred.Username.String()
				cfg.RegistryPassword = cred.Password.String()
			} else {
				j.Deps.Logger.Warn().Err(err).
					Str("application_id", j.app.ID).
					Str("registry_credential_id", *j.app.RegistryCredentialID).
					Msg("registry credential lookup failed; pull will run unauthenticated")
			}
		} else if j.app.RegistryUsername != nil && *j.app.RegistryUsername != "" &&
			!j.app.RegistryPassword.IsEmpty() {
			cfg.RegistryUsername = *j.app.RegistryUsername
			cfg.RegistryPassword = j.app.RegistryPassword.String()
			// Inline credentials don't carry a URL field of their own
			// in the model — source_config holds it for that path.
			if rawURL, ok := map[string]any(j.app.SourceConfig)["registry_url"].(string); ok {
				cfg.RegistryURL = rawURL
			}
		}
	}

	// Hydrate env vars + volumes from their child tables. Best-effort:
	// transient read errors are logged but don't block the deploy — a
	// fresh app with no env vars deploys cleanly with an empty list.
	if envVars, err := j.Deps.Repos.EnvVar().ListForApplication(ctx, j.app.ID); err == nil {
		// Load project env vars once so we can resolve any
		// `${{project.<KEY>}}` references in this app's own env values.
		// Best-effort: a project-level lookup failure just leaves the
		// references literal — the user will see `${{project.X}}`
		// inside the container instead of a silently-empty value.
		projectEnv, perr := j.Deps.Repos.ProjectEnvVar().ListMapForProject(ctx, j.app.ProjectID)
		if perr != nil {
			j.Deps.Logger.Warn().Err(perr).Str("project_id", j.app.ProjectID).
				Msg("failed to load project env; ${{project.*}} references will not resolve")
			projectEnv = map[string]string{}
		}
		cfg.EnvVars = make([]tasks.EnvVar, 0, len(envVars))
		for _, e := range envVars {
			cfg.EnvVars = append(cfg.EnvVars, tasks.EnvVar{
				Key:   e.Key,
				Value: tasks.ResolveProjectRefs(string(e.Value), projectEnv),
			})
		}
	} else {
		j.Deps.Logger.Warn().Err(err).Str("application_id", j.app.ID).
			Msg("failed to load env vars; deploying without them")
	}

	// Build-time secrets ride alongside runtime env vars but flow into
	// `docker build --mount=type=secret`, not `docker run --env-file`.
	// Same best-effort posture — a read failure logs + skips; the build
	// proceeds without secrets (and any RUN steps that need them will
	// fail loudly, which is the correct failure mode).
	if buildSecrets, err := j.Deps.Repos.BuildSecret().ListForApplication(ctx, j.app.ID); err == nil {
		cfg.BuildSecrets = make([]tasks.BuildSecret, 0, len(buildSecrets))
		for _, s := range buildSecrets {
			cfg.BuildSecrets = append(cfg.BuildSecrets, tasks.BuildSecret{
				Name:  s.Name,
				Value: string(s.Value),
			})
		}
	} else {
		j.Deps.Logger.Warn().Err(err).Str("application_id", j.app.ID).
			Msg("failed to load build secrets; building without them")
	}

	// Advanced knobs travel through build_config; missing keys leave
	// the corresponding cfg field at zero (the script picks its default).
	build := map[string]any(j.app.BuildConfig)
	if v, ok := build["cpu_limit"].(string); ok {
		cfg.CPULimit = v
	}
	if v, ok := build["memory_limit"].(string); ok {
		cfg.MemoryLimit = v
	}
	if v, ok := build["restart_policy"].(string); ok {
		cfg.RestartPolicy = v
	}
	if v, ok := build["healthcheck_command"].(string); ok {
		cfg.HealthcheckCommand = v
	}
	if v, ok := build["extra_ports"].([]any); ok {
		for _, p := range v {
			if s, ok := p.(string); ok {
				cfg.ExtraPorts = append(cfg.ExtraPorts, s)
			}
		}
	}

	if vols, err := j.Deps.Repos.Volume().ListForApplication(ctx, j.app.ID); err == nil {
		cfg.Volumes = make([]tasks.Volume, 0, len(vols))
		for _, v := range vols {
			hp := ""
			if v.HostPath != nil {
				hp = *v.HostPath
			}
			cfg.Volumes = append(cfg.Volumes, tasks.Volume{
				Name:      v.Name,
				MountPath: v.MountPath,
				Type:      v.Type,
				HostPath:  hp,
			})
		}
	} else {
		j.Deps.Logger.Warn().Err(err).Str("application_id", j.app.ID).
			Msg("failed to load volumes; deploying without them")
	}

	task := tasks.DeployApplication(cfg)
	// TrackInDB() persists a server-tasks row before SSH starts so the
	// frontend can stream live build/deploy output via ServerLogViewer
	// entity="task" :entity-id="task_id" — same pattern site
	// deployments use.
	result, runErr := j.Deps.RunTask(j.server, task).AsRoot().TrackInDB().Dispatch(ctx)

	output := ""
	exitCode := -1
	taskID := ""
	if result != nil {
		output = result.GetOutput()
		exitCode = result.GetExitCode()
		if result.TaskModel != nil {
			taskID = result.TaskModel.ID
		}
	}
	// Persist the task ID onto the deployment row so the Deployments
	// tab's "View logs" can subscribe to the live log stream. Done
	// before the success/failure branch so the task_id is visible
	// whether the deploy passed or failed.
	if taskID != "" {
		_ = j.Deps.Repos.Deployment().UpdateFields(ctx, j.deployment.ID, map[string]any{
			"task_id": taskID,
		})
		j.deployment.TaskID = &taskID
	}

	if runErr != nil || (result != nil && !result.IsSuccessful()) {
		j.handleFailure(ctx, output, runErr, exitCode)
		// Returning nil so asynq doesn't retry a doomed build; the user
		// triggers a redeploy from the UI if they want another attempt.
		return nil
	}

	containerID, imageRef := parseDeployMarkers(output)
	j.handleSuccess(ctx, containerID, imageRef)
	return nil
}

// Failed is asynq's job-level failure callback (called when Handle
// returns an error or panics). We use this only for the "loadModels
// failed" / DB-unavailable case since Handle traps user-error failures
// itself.
func (j *DeployApplicationJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("application_id", j.Payload.ApplicationID).
		Str("deployment_id", j.Payload.DeploymentID).
		Msg("deploy application job failed at the framework level")

	// Best-effort: mark the deployment as failed if we can.
	finishedAt := time.Now().UTC()
	errMsg := err.Error()
	_ = j.Deps.Repos.Deployment().UpdateFields(ctx, j.Payload.DeploymentID, map[string]any{
		"status":      dockertypes.DeploymentStatusFailed,
		"finished_at": finishedAt,
		"error":       errMsg,
	})
}

// loadModels hydrates j.{app, deployment, project, server} or returns
// a wrapped error indicating which lookup failed.
func (j *DeployApplicationJob) loadModels(ctx context.Context) error {
	var err error
	j.deployment, err = j.Deps.Repos.Deployment().FindByID(ctx, j.Payload.DeploymentID)
	if err != nil {
		return fmt.Errorf("find deployment: %w", err)
	}
	j.app, err = j.Deps.Repos.Application().FindByIDAndTeamServer(
		ctx, j.Payload.ApplicationID, j.Payload.TeamID, j.Payload.ServerID,
	)
	if err != nil {
		return fmt.Errorf("find application: %w", err)
	}
	j.project, err = j.Deps.Repos.Project().FindByIDAndTeamServer(
		ctx, j.app.ProjectID, j.Payload.TeamID, j.Payload.ServerID,
	)
	if err != nil {
		return fmt.Errorf("find project: %w", err)
	}
	j.server, err = j.Deps.ServerRepos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("find server: %w", err)
	}
	return nil
}

func (j *DeployApplicationJob) handleSuccess(ctx context.Context, containerID, imageRef string) {
	finishedAt := time.Now().UTC()

	deploymentUpdates := map[string]any{
		"status":      dockertypes.DeploymentStatusSuccess,
		"finished_at": finishedAt,
	}
	if imageRef != "" {
		deploymentUpdates["image_ref"] = imageRef
	}
	_ = j.Deps.Repos.Deployment().UpdateFields(ctx, j.deployment.ID, deploymentUpdates)

	appUpdates := map[string]any{
		"status":           dockertypes.ApplicationStatusRunning,
		"last_deployed_at": finishedAt,
	}
	if containerID != "" {
		appUpdates["container_id"] = containerID
	}
	_ = j.Deps.Repos.Application().UpdateFields(ctx, j.app.ID, appUpdates)

	// Re-sync Traefik so routing rules reflect the just-deployed
	// container name. Same path that domain mutations take — keeps the
	// behaviour consistent so an operator can reason about one resync
	// path, not two.
	if syncTask, err := NewSyncTraefikConfigTask(j.app.ID, j.Payload.ServerID, j.Payload.TeamID); err == nil {
		// Queue.Enqueue returns (taskID, error). We don't need the id here.
		if _, enqErr := j.Deps.Queue.Enqueue(syncTask); enqErr != nil {
			j.Deps.Logger.Error().Err(enqErr).Msg("failed to enqueue traefik resync after deploy")
		}
	}

	j.broadcast("docker.application.deployed", map[string]any{
		"application_id": j.app.ID,
		"deployment_id":  j.deployment.ID,
		"container_id":   containerID,
		"image_ref":      imageRef,
		"status":         "running",
	})
}

func (j *DeployApplicationJob) handleFailure(ctx context.Context, output string, runErr error, exitCode int) {
	finishedAt := time.Now().UTC()

	errMsg := ""
	if runErr != nil {
		errMsg = runErr.Error()
	}
	// Append the tail of the script output. Cap the length so a runaway
	// build log doesn't blow up the deployments row.
	if output != "" {
		tail := output
		if len(tail) > 4000 {
			tail = "…" + tail[len(tail)-4000:]
		}
		if errMsg != "" {
			errMsg += "\n\n--- output tail ---\n" + tail
		} else {
			errMsg = tail
		}
	}

	_ = j.Deps.Repos.Deployment().UpdateFields(ctx, j.deployment.ID, map[string]any{
		"status":      dockertypes.DeploymentStatusFailed,
		"finished_at": finishedAt,
		"error":       errMsg,
	})
	_ = j.Deps.Repos.Application().UpdateFields(ctx, j.app.ID, map[string]any{
		"status": dockertypes.ApplicationStatusFailed,
	})

	j.broadcast("docker.application.failed", map[string]any{
		"application_id": j.app.ID,
		"deployment_id":  j.deployment.ID,
		"status":         "failed",
		"exit_code":      exitCode,
		// Send a short error summary to the UI; the full output sits on
		// the deployment row for the user to inspect.
		"error": summarise(errMsg),
	})
}

// broadcast sends an event to the team channel with the routing fields
// the frontend expects. Mirrors the contract documented in CLAUDE.md
// ("WebSocket Broadcasting — Architecture & Rules").
func (j *DeployApplicationJob) broadcast(event string, data map[string]any) {
	if data == nil {
		data = map[string]any{}
	}
	data["team_id"] = j.Payload.TeamID
	data["server_id"] = j.Payload.ServerID
	if _, ok := data["application_id"]; !ok {
		data["application_id"] = j.Payload.ApplicationID
	}
	j.Deps.BroadcastToTeam(j.Payload.TeamID, event, data)
}

// hydrateSourceConfig copies the source-type-specific fields from the
// application's stored config into the script-template config. Keeping
// this isolated means the script template can stay a pure-data struct.
func hydrateSourceConfig(cfg *tasks.DeployConfig, app *models.Application) {
	src := map[string]any(app.SourceConfig)
	build := map[string]any(app.BuildConfig)

	switch app.SourceType {
	case dockertypes.SourceTypeImage:
		if v, ok := src["image"].(string); ok {
			cfg.Image = v
		}

	case dockertypes.SourceTypeGit:
		if v, ok := src["repo"].(string); ok {
			cfg.GitRepo = v
		}
		if v, ok := src["branch"].(string); ok {
			cfg.GitBranch = v
		}
		if app.BuildType != nil {
			cfg.BuildType = *app.BuildType
		} else {
			cfg.BuildType = dockertypes.BuildTypeNixpacks
		}
		if v, ok := build["dockerfile_path"].(string); ok {
			cfg.DockerfilePath = v
		}

	case dockertypes.SourceTypeDockerfile:
		if v, ok := src["contents"].(string); ok {
			cfg.DockerfileContents = v
		}
	}
}

// parseDeployMarkers walks the SSH-task output for our `::LAUNCH::key::value`
// markers and extracts container_id + image_ref. Both are optional —
// missing values just don't get persisted, no error.
//
// Multiple matches for the same key keep the last occurrence (the script
// emits the marker once at the end so this is robust to repeated runs).
func parseDeployMarkers(output string) (containerID, imageRef string) {
	for _, line := range strings.Split(output, "\n") {
		if !strings.Contains(line, "::LAUNCH::") {
			continue
		}
		if m := containerIDRe.FindStringSubmatch(line); len(m) == 2 {
			containerID = strings.TrimSpace(m[1])
		}
		if m := imageRefRe.FindStringSubmatch(line); len(m) == 2 {
			imageRef = strings.TrimSpace(m[1])
		}
	}
	return
}

var (
	containerIDRe = regexp.MustCompile(`::LAUNCH::container_id::([^\s]+)`)
	imageRefRe    = regexp.MustCompile(`::LAUNCH::image_ref::([^\s]+)`)
)

// summarise returns a short single-line summary for the WS payload — the
// full error stays on the deployment row.
func summarise(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if idx := strings.Index(s, "\n"); idx > 0 && idx < 200 {
		return s[:idx]
	}
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}

// ghcrBearerMaxAge is how long after the workflow minted a GHCR pull
// bearer we still consider it usable. GHCR docs say bearers live ~1h;
// we use a 50-minute cap to give the deploy a few minutes' headroom
// between this check and the actual `docker pull` on the remote host.
const ghcrBearerMaxAge = 50 * time.Minute

// ghcrBearerExpiredSoon reports whether the workflow-stamped GHCR pull
// bearer is close enough to its TTL that we shouldn't hand it to
// docker pull. Returns (humanReadableReason, true) when the bearer is
// too old; ("", false) when it's fresh OR when no workflow-stamped
// bearer accompanied this deploy (install-token fallback path, or
// non-GHA deploy).
//
// Package-level (not a method) so the compose job + tests can call it
// without pulling in DeployApplicationJob.
func ghcrBearerExpiredSoon(mintedAtUnix string, now time.Time) (string, bool) {
	stamp := strings.TrimSpace(mintedAtUnix)
	if stamp == "" {
		return "", false
	}
	mintedUnix, err := strconv.ParseInt(stamp, 10, 64)
	if err != nil || mintedUnix == 0 {
		return "", false
	}
	mintedAt := time.Unix(mintedUnix, 0).UTC()
	age := now.Sub(mintedAt)
	if age < 0 {
		// Stamp is in the future — almost certainly clock drift
		// between the GHA runner and our host; tolerate small skew
		// silently. (Anything beyond a couple of minutes here is
		// pathological enough we don't want to silently proceed.)
		if age < -5*time.Minute {
			return fmt.Sprintf("token minted in the future (%v ahead)", -age), true
		}
		return "", false
	}
	if age > ghcrBearerMaxAge {
		return fmt.Sprintf("token age %v exceeds max %v", age.Round(time.Second), ghcrBearerMaxAge), true
	}
	return "", false
}

// markDeploymentFailed flips the deployment row to status=failed +
// the application row to status=failed + broadcasts the failure
// event. Used by the pre-flight bearer-age guard so we surface a
// clean, non-retriable failure to the UI rather than letting docker
// pull eventually error with an opaque 401.
func (j *DeployApplicationJob) markDeploymentFailed(ctx context.Context, errMsg string) {
	finishedAt := time.Now().UTC()
	_ = j.Deps.Repos.Deployment().UpdateFields(ctx, j.deployment.ID, map[string]any{
		"status":      dockertypes.DeploymentStatusFailed,
		"finished_at": finishedAt,
		"error":       errMsg,
	})
	_ = j.Deps.Repos.Application().UpdateFields(ctx, j.app.ID, map[string]any{
		"status": dockertypes.ApplicationStatusFailed,
	})
	j.broadcast("docker.application.failed", map[string]any{
		"application_id": j.app.ID,
		"deployment_id":  j.deployment.ID,
		"status":         "failed",
		"error":          errMsg,
	})
}

// applyDeployApplicationOverrides flips the deploy config to the
// image-source path when the webhook handler supplied an override
// image + creds. Idempotent + a no-op when OverrideImage is empty —
// the existing non-GHA flow runs unchanged.
//
// Extracted as a pure function (no DB, no SSH) so the GHA override
// branch can be unit-tested without standing up the full job. The
// caller is the deploy job's Handle; tests exercise it directly with
// a synthetic config + payload.
//
// Mutation contract:
//   - OverrideImage == "" → returns without touching cfg
//   - OverrideImage set   → SourceType flipped to image, Image set,
//     each Registry* field that's non-empty in the payload overwrites
//     the corresponding cfg field (empty in payload = keep cfg as-is)
func applyDeployApplicationOverrides(cfg *tasks.DeployConfig, payload DeployApplicationPayload) {
	if payload.OverrideImage == "" {
		return
	}
	cfg.SourceType = dockertypes.SourceTypeImage
	cfg.Image = payload.OverrideImage
	if payload.OverrideRegistryURL != "" {
		cfg.RegistryURL = payload.OverrideRegistryURL
	}
	if payload.OverrideRegistryUsername != "" {
		cfg.RegistryUsername = payload.OverrideRegistryUsername
	}
	if payload.OverrideRegistryPassword != "" {
		cfg.RegistryPassword = payload.OverrideRegistryPassword
	}
}

// NewDeployApplicationTask packages the asynq task for enqueueing from
// the service layer.
func NewDeployApplicationTask(
	applicationID, deploymentID, serverID, teamID string,
) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeDeployApplication, DeployApplicationPayload{
		ApplicationID: applicationID,
		DeploymentID:  deploymentID,
		ServerID:      serverID,
		TeamID:        teamID,
	}, pkgjobs.Dedup("docker-deploy-app", deploymentID))
}

// NewDeployApplicationTaskFromGHA mirrors NewDeployApplicationTask but
// carries the image + registry credentials the GHA workflow built and
// the webhook handler resolved. Used exclusively from the GHA webhook
// path; everywhere else continues to call NewDeployApplicationTask.
//
// Dedup key still keys off deployment_id, so two webhook retries for
// the same run_id (which reuse the existing deployment row) reuse the
// same dedup slot and asynq drops the duplicate enqueue.
func NewDeployApplicationTaskFromGHA(
	applicationID, deploymentID, serverID, teamID string,
	image, registryURL, registryUsername, registryPassword string,
	mintedAtUnix string,
) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeDeployApplication, DeployApplicationPayload{
		ApplicationID:                        applicationID,
		DeploymentID:                         deploymentID,
		ServerID:                             serverID,
		TeamID:                               teamID,
		OverrideImage:                        image,
		OverrideRegistryURL:                  registryURL,
		OverrideRegistryUsername:             registryUsername,
		OverrideRegistryPassword:             registryPassword,
		OverrideRegistryPasswordMintedAtUnix: mintedAtUnix,
	}, pkgjobs.Dedup("docker-deploy-app", deploymentID))
}
