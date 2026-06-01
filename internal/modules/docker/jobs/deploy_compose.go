package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"github.com/kkz6/launch-go/internal/modules/docker/tasks"
	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

// TypeDeployCompose is the asynq task type for deploying a compose stack.
const TypeDeployCompose = "docker:deploy_compose"

// DeployComposePayload travels through asynq — IDs only, except for
// the GHA-built image map which flows from the GitHub Actions webhook
// straight to the deploy script without persisting on the workload row.
//
// Override fields semantics match DeployApplicationPayload's
// Override* group: nil/empty = today's behaviour, non-empty = treat
// as the GHA path (skip on-host build, rewrite compose `build:` to
// `image:` per service, docker login GHCR with the supplied creds).
type DeployComposePayload struct {
	ComposeID    string `json:"compose_id"`
	DeploymentID string `json:"deployment_id"`
	ServerID     string `json:"server_id"`
	TeamID       string `json:"team_id"`

	// ServiceImages maps compose service name → fully-qualified image
	// reference (e.g. {"web": "ghcr.io/kkz6/test:launch-web-abc1234"}).
	// When non-empty the deploy script rewrites the compose YAML in
	// place to swap each service's `build:` block for `image: <value>`,
	// then runs `docker compose up -d` without --build.
	ServiceImages map[string]string `json:"service_images,omitempty"`

	// OverrideRegistry{URL,Username,Password} are the docker-login
	// credentials the deploy script pipes to --password-stdin BEFORE
	// `docker compose up`. For GHA, URL is ghcr.io and the password
	// is either a workflow-minted bearer (username "oauth2") or a
	// GitHub App installation token (username "x-access-token").
	//
	// NOTE: OverrideRegistryPassword is a secret. The custom String()
	// method on this struct redacts it; if you add another channel
	// that formats this struct, mirror the redaction there.
	OverrideRegistryURL      string `json:"override_registry_url,omitempty"`
	OverrideRegistryUsername string `json:"override_registry_username,omitempty"`
	OverrideRegistryPassword string `json:"override_registry_password,omitempty"`
	// OverrideRegistryPasswordMintedAtUnix mirrors the application
	// payload — see DeployApplicationPayload for the rationale.
	OverrideRegistryPasswordMintedAtUnix string `json:"override_registry_password_minted_at_unix,omitempty"`

	// Recreate switches the stack into "reload" mode: re-run
	// `docker compose up -d --remove-orphans` WITHOUT --build (ignoring
	// any RunCommand override) after rewriting the .env, so changed
	// runtime env is applied by reusing the on-host images. Used by the
	// Reload action; build-time changes still go through a full Deploy.
	Recreate bool `json:"recreate,omitempty"`
}

// String returns a redacted JSON view of the payload — same redaction
// + same rationale as DeployApplicationPayload.String(). Locked in by
// TestDeployComposePayload_StringRedaction.
func (p DeployComposePayload) String() string {
	redacted := p
	if redacted.OverrideRegistryPassword != "" {
		redacted.OverrideRegistryPassword = "[REDACTED]"
	}
	b, err := json.Marshal(redacted)
	if err != nil {
		return fmt.Sprintf("DeployComposePayload{compose=%s, deploy=%s, marshal_err=%v}",
			p.ComposeID, p.DeploymentID, err)
	}
	return string(b)
}

// DeployComposeJob mirrors DeployApplicationJob but runs the compose
// deploy script instead of single-container docker run. The deployments
// row uses target_type=compose so the polymorphic history table is
// queryable per workload kind.
type DeployComposeJob struct {
	Deps    *JobDeps
	Payload DeployComposePayload

	compose    *models.Compose
	deployment *models.Deployment
	project    *models.Project
	server     *servermodels.Server
}

// NewDeployComposeJob is the asynq constructor.
func NewDeployComposeJob(p DeployComposePayload) pkgjobs.Handler {
	return &DeployComposeJob{Deps: deps, Payload: p}
}

// Handle runs the compose deploy. Same shape as
// DeployApplicationJob.Handle but without container_id parsing — a
// compose stack creates multiple containers and we don't track them
// individually in phase 2h.
func (j *DeployComposeJob) Handle(ctx context.Context) error {
	if err := j.loadModels(ctx); err != nil {
		return err
	}

	// Pre-flight GHCR bearer age check — same rationale as in
	// DeployApplicationJob.Handle. See ghcrBearerExpiredSoon on the
	// app job for the longer-form comment.
	if reason, expired := ghcrBearerExpiredSoon(j.Payload.OverrideRegistryPasswordMintedAtUnix, time.Now().UTC()); expired {
		j.Deps.Logger.Warn().
			Str("compose_id", j.Payload.ComposeID).
			Str("deployment_id", j.Payload.DeploymentID).
			Str("reason", reason).
			Msg("GHA deploy: GHCR pull bearer expired before compose deploy ran")
		j.markComposeDeploymentFailed(ctx,
			"GHCR pull token expired before this deploy could run "+
				"(the queue was stalled for longer than the token's ~1h "+
				"lifetime). Re-trigger the workflow on GitHub to mint a "+
				"fresh token and try again.")
		return nil
	}

	now := time.Now().UTC()
	_ = j.Deps.Repos.Deployment().UpdateFields(ctx, j.deployment.ID, map[string]any{
		"status":     dockertypes.DeploymentStatusDeploying,
		"started_at": now,
	})
	_ = j.Deps.Repos.Compose().UpdateFields(ctx, j.compose.ID, map[string]any{
		"status": dockertypes.ApplicationStatusBuilding,
	})
	j.broadcast("docker.compose.deploying", map[string]any{
		"compose_id":    j.compose.ID,
		"deployment_id": j.deployment.ID,
		"status":        "building",
	})

	cfg := tasks.ComposeDeployConfig{
		DeploymentID: j.deployment.ID,
		ProjectSlug:  tasks.SlugFromName(j.project.Name),
		ComposeSlug:  tasks.SlugFromName(j.compose.Name),
		ProjectName:  fmt.Sprintf("%s-%s", tasks.SlugFromName(j.project.Name), tasks.SlugFromName(j.compose.Name)),
	}
	hydrateComposeSource(&cfg, j.compose)
	// Same authenticated-clone treatment as application git deploys.
	cfg.GitRepo = j.Deps.resolveAuthenticatedCloneURL(ctx, map[string]any(j.compose.SourceConfig), cfg.GitRepo)

	// Reload mode: force a no-build `up -d` that reuses on-host images
	// and applies the refreshed .env. Set before the ServiceImages
	// branch so a reload never takes the GHA-rewrite path.
	cfg.RecreateOnly = j.Payload.Recreate

	// GHA path: if the webhook handed us a service_images map, thread
	// it onto the deploy config. The renderer inserts a yq rewrite
	// step that replaces each service's `build:` block with
	// `image: <ghcr image>` so the subsequent `docker compose up`
	// pulls instead of building. Override creds go into the registry
	// logins so the script logs into GHCR first.
	if len(j.Payload.ServiceImages) > 0 {
		cfg.ServiceImages = j.Payload.ServiceImages
		if j.Payload.OverrideRegistryURL != "" {
			cfg.RegistryLogins = append(cfg.RegistryLogins, tasks.ComposeRegistryLogin{
				RegistryURL: j.Payload.OverrideRegistryURL,
				Username:    j.Payload.OverrideRegistryUsername,
				Password:    j.Payload.OverrideRegistryPassword,
			})
		}
	}

	// Type=file rows attached to this stack get materialized to
	// ${STACK_DIR}/files/<path> by the deploy script. Pulling them
	// here (vs in the renderer) keeps the renderer pure and lets us
	// surface load errors as a soft warning rather than a script-time
	// failure — if the DB is unreachable, the deploy still proceeds
	// without file mounts; the script's `rm -rf ./files` keeps the
	// host clean of stale ones from a previous deploy.
	if rows, err := j.Deps.Repos.Volume().ListForCompose(ctx, j.compose.ID); err == nil {
		for i := range rows {
			r := &rows[i]
			if r.Type != "file" || r.FilePath == nil || *r.FilePath == "" {
				continue
			}
			content := ""
			if r.Content != nil {
				content = *r.Content
			}
			cfg.FileMounts = append(cfg.FileMounts, tasks.ComposeFileMount{
				RelativePath: *r.FilePath,
				Content:      content,
			})
		}
	} else {
		j.Deps.Logger.Warn().Err(err).
			Str("compose_id", j.compose.ID).
			Msg("load compose file-mounts failed; continuing without them")
	}

	// Load attached registry credentials (many-to-many) and hand
	// plaintext creds to the deploy script. The script logs into
	// each before `docker compose pull/up` and logs out after — same
	// shape the application path uses. Best-effort: a load failure
	// proceeds with no auth and surfaces the real `pull` error on
	// the deploy log if any image is private.
	if err := j.Deps.DB.Model(j.compose).
		Association("RegistryCredentials").
		Find(&j.compose.RegistryCredentials); err == nil {
		for i := range j.compose.RegistryCredentials {
			c := &j.compose.RegistryCredentials[i]
			login := tasks.ComposeRegistryLogin{
				Username: c.Username.String(),
				Password: c.Password.String(),
			}
			if c.RegistryURL != nil {
				login.RegistryURL = *c.RegistryURL
			}
			cfg.RegistryLogins = append(cfg.RegistryLogins, login)
		}
	} else {
		j.Deps.Logger.Warn().Err(err).
			Str("compose_id", j.compose.ID).
			Msg("load compose registry credentials failed; deploy will run unauthenticated")
	}

	task := tasks.DeployCompose(cfg)
	// TrackInDB() persists a server-tasks row so the frontend can
	// stream live `docker compose up` output via ServerLogViewer —
	// matches the application + database paths.
	result, runErr := j.Deps.RunTask(j.server, task).AsRoot().TrackInDB().
		OnTaskCreated(func(taskID string) {
			// Surface the task ID early so the Compose Deployments tab's
			// "View Logs" appears and streams live output during the
			// deploy, not only after it finishes.
			_ = j.Deps.Repos.Deployment().UpdateFields(ctx, j.deployment.ID, map[string]any{"task_id": taskID})
			j.deployment.TaskID = &taskID
			j.broadcast("docker.compose.deploying", map[string]any{
				"compose_id":    j.compose.ID,
				"deployment_id": j.deployment.ID,
				"task_id":       taskID,
				"status":        "building",
			})
		}).
		Dispatch(ctx)

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
	if taskID != "" {
		_ = j.Deps.Repos.Deployment().UpdateFields(ctx, j.deployment.ID, map[string]any{
			"task_id": taskID,
		})
		j.deployment.TaskID = &taskID
	}

	if runErr != nil || (result != nil && !result.IsSuccessful()) {
		j.handleFailure(ctx, output, runErr, exitCode)
		return nil
	}
	j.handleSuccess(ctx)
	return nil
}

func (j *DeployComposeJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("compose_id", j.Payload.ComposeID).
		Str("deployment_id", j.Payload.DeploymentID).
		Msg("deploy compose job failed at the framework level")

	finishedAt := time.Now().UTC()
	errMsg := err.Error()
	_ = j.Deps.Repos.Deployment().UpdateFields(ctx, j.Payload.DeploymentID, map[string]any{
		"status":      dockertypes.DeploymentStatusFailed,
		"finished_at": finishedAt,
		"error":       errMsg,
	})
}

func (j *DeployComposeJob) loadModels(ctx context.Context) error {
	var err error
	j.deployment, err = j.Deps.Repos.Deployment().FindByID(ctx, j.Payload.DeploymentID)
	if err != nil {
		return fmt.Errorf("find deployment: %w", err)
	}
	j.compose, err = j.Deps.Repos.Compose().FindByIDAndTeamServer(
		ctx, j.Payload.ComposeID, j.Payload.TeamID, j.Payload.ServerID,
	)
	if err != nil {
		return fmt.Errorf("find compose: %w", err)
	}
	j.project, err = j.Deps.Repos.Project().FindByIDAndTeamServer(
		ctx, j.compose.ProjectID, j.Payload.TeamID, j.Payload.ServerID,
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

func (j *DeployComposeJob) handleSuccess(ctx context.Context) {
	finishedAt := time.Now().UTC()
	_ = j.Deps.Repos.Deployment().UpdateFields(ctx, j.deployment.ID, map[string]any{
		"status":      dockertypes.DeploymentStatusSuccess,
		"finished_at": finishedAt,
	})
	_ = j.Deps.Repos.Compose().UpdateFields(ctx, j.compose.ID, map[string]any{
		"status":           dockertypes.ApplicationStatusRunning,
		"last_deployed_at": finishedAt,
	})

	// Re-render the per-compose Traefik file after a successful
	// deploy. Container names depend on the compose project name
	// (which is stable across deploys), so the YAML only changes when
	// the user mutates a domain — but firing on deploy keeps the file
	// in sync if Traefik's dynamic dir was wiped, the file was
	// manually edited, or the very first deploy is happening before
	// any domain mutation has run. Same idempotency rationale as the
	// application path.
	if task, err := NewSyncComposeTraefikConfigTask(
		j.compose.ID, j.Payload.ServerID, j.Payload.TeamID,
	); err == nil {
		if _, err := j.Deps.Queue.Enqueue(task); err != nil {
			j.Deps.Logger.Warn().Err(err).
				Str("compose_id", j.compose.ID).
				Msg("enqueue post-deploy compose traefik sync failed")
		}
	}

	j.broadcast("docker.compose.deployed", map[string]any{
		"compose_id":    j.compose.ID,
		"deployment_id": j.deployment.ID,
		"status":        "running",
	})
}

func (j *DeployComposeJob) handleFailure(ctx context.Context, output string, runErr error, exitCode int) {
	finishedAt := time.Now().UTC()
	errMsg := ""
	if runErr != nil {
		errMsg = runErr.Error()
	}
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
	_ = j.Deps.Repos.Compose().UpdateFields(ctx, j.compose.ID, map[string]any{
		"status": dockertypes.ApplicationStatusFailed,
	})
	j.broadcast("docker.compose.failed", map[string]any{
		"compose_id":    j.compose.ID,
		"deployment_id": j.deployment.ID,
		"status":        "failed",
		"exit_code":     exitCode,
		"error":         summarise(errMsg),
	})
}

// markComposeDeploymentFailed mirrors markDeploymentFailed on the
// application job — flips deployment + compose rows to failed +
// broadcasts the WS event. Used by the pre-flight bearer-age guard
// so we surface a clean, non-retriable failure instead of letting
// docker pull eventually error with an opaque 401.
func (j *DeployComposeJob) markComposeDeploymentFailed(ctx context.Context, errMsg string) {
	finishedAt := time.Now().UTC()
	_ = j.Deps.Repos.Deployment().UpdateFields(ctx, j.deployment.ID, map[string]any{
		"status":      dockertypes.DeploymentStatusFailed,
		"finished_at": finishedAt,
		"error":       errMsg,
	})
	_ = j.Deps.Repos.Compose().UpdateFields(ctx, j.compose.ID, map[string]any{
		"status": dockertypes.ApplicationStatusFailed,
	})
	j.broadcast("docker.compose.failed", map[string]any{
		"compose_id":    j.compose.ID,
		"deployment_id": j.deployment.ID,
		"status":        "failed",
		"error":         errMsg,
	})
}

func (j *DeployComposeJob) broadcast(event string, data map[string]any) {
	if data == nil {
		data = map[string]any{}
	}
	data["team_id"] = j.Payload.TeamID
	data["server_id"] = j.Payload.ServerID
	if _, ok := data["compose_id"]; !ok {
		data["compose_id"] = j.Payload.ComposeID
	}
	j.Deps.BroadcastToTeam(j.Payload.TeamID, event, data)
}

// hydrateComposeSource maps the stored source_config + raw_yaml fields
// into the ComposeDeployConfig the script renderer expects.
func hydrateComposeSource(cfg *tasks.ComposeDeployConfig, c *models.Compose) {
	switch c.ComposeSourceType {
	case "git":
		src := map[string]any(c.SourceConfig)
		if v, ok := src["repo"].(string); ok {
			cfg.GitRepo = v
		}
		if v, ok := src["branch"].(string); ok {
			cfg.GitBranch = v
		}
		if c.ComposeFilePath != nil {
			cfg.ComposeFilePath = *c.ComposeFilePath
		}
	case "raw_yaml":
		if c.RawYAML != nil {
			cfg.RawYAML = *c.RawYAML
		}
	}
	// EnvFile is source-agnostic — the Environment subtab applies to
	// both git and raw_yaml stacks. Empty/missing → deploy script
	// removes any stale .env on the host.
	if c.EnvFile != nil {
		cfg.EnvFile = *c.EnvFile
	}
	// RunCommand is also source-agnostic — the operator's override
	// runs verbatim regardless of git vs raw_yaml.
	if c.RunCommand != nil {
		cfg.RunCommand = *c.RunCommand
	}
}

// NewDeployComposeTask packages the asynq task for the service.
func NewDeployComposeTask(composeID, deploymentID, serverID, teamID string) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeDeployCompose, DeployComposePayload{
		ComposeID:    composeID,
		DeploymentID: deploymentID,
		ServerID:     serverID,
		TeamID:       teamID,
	}, pkgjobs.Dedup("docker-deploy-compose", deploymentID))
}

// NewRecreateComposeTask enqueues a compose "reload": re-up the stack
// with the current .env and no rebuild (reuse on-host images). Same job
// + dedup slot as a deploy, with Recreate set.
func NewRecreateComposeTask(composeID, deploymentID, serverID, teamID string) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeDeployCompose, DeployComposePayload{
		ComposeID:    composeID,
		DeploymentID: deploymentID,
		ServerID:     serverID,
		TeamID:       teamID,
		Recreate:     true,
	}, pkgjobs.Dedup("docker-deploy-compose", deploymentID))
}

// NewDeployComposeTaskFromGHA wraps NewDeployComposeTask with the
// GHA-built image map + GHCR pull credentials the webhook handler
// resolved. Dedup key still keys off deployment_id so two retried
// notifies for the same run reuse the same dedup slot.
func NewDeployComposeTaskFromGHA(
	composeID, deploymentID, serverID, teamID string,
	serviceImages map[string]string,
	registryURL, registryUsername, registryPassword string,
	mintedAtUnix string,
) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeDeployCompose, DeployComposePayload{
		ComposeID:                            composeID,
		DeploymentID:                         deploymentID,
		ServerID:                             serverID,
		TeamID:                               teamID,
		ServiceImages:                        serviceImages,
		OverrideRegistryURL:                  registryURL,
		OverrideRegistryUsername:             registryUsername,
		OverrideRegistryPassword:             registryPassword,
		OverrideRegistryPasswordMintedAtUnix: mintedAtUnix,
	}, pkgjobs.Dedup("docker-deploy-compose", deploymentID))
}
