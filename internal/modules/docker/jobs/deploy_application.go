package jobs

import (
	"context"
	"fmt"
	"regexp"
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
type DeployApplicationPayload struct {
	ApplicationID string `json:"application_id"`
	DeploymentID  string `json:"deployment_id"`
	ServerID      string `json:"server_id"`
	TeamID        string `json:"team_id"`
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
	if j.app.SourceType == dockertypes.SourceTypeImage {
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
