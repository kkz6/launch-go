package jobs

import (
	"context"
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

// DeployComposePayload travels through asynq — IDs only.
type DeployComposePayload struct {
	ComposeID    string `json:"compose_id"`
	DeploymentID string `json:"deployment_id"`
	ServerID     string `json:"server_id"`
	TeamID       string `json:"team_id"`
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

	task := tasks.DeployCompose(cfg)
	// TrackInDB() persists a server-tasks row so the frontend can
	// stream live `docker compose up` output via ServerLogViewer —
	// matches the application + database paths.
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
