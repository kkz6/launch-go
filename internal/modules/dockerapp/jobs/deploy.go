package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/dockerapp/models"
	dockerapptasks "github.com/kkz6/launch-go/internal/modules/dockerapp/tasks"
	"github.com/kkz6/launch-go/internal/modules/dockerapp/types"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

const TypeDeployApp = "dockerapp:deploy"

// DeployPayload carries the IDs needed to deploy an app.
type DeployPayload struct {
	AppID  string  `json:"app_id"`
	UserID *string `json:"user_id,omitempty"`
}

// DeployJob handles application deployment.
type DeployJob struct {
	Deps    *JobDeps
	Payload DeployPayload

	app    *models.App
	server *servermodels.Server
}

// NewDeployJob constructs the job handler.
func NewDeployJob(p DeployPayload) pkgjobs.Handler {
	return &DeployJob{Deps: deps, Payload: p}
}

// Handle pulls the image (or writes the compose project) and runs the
// container with all configured env / ports / volumes / Traefik labels.
func (j *DeployJob) Handle(ctx context.Context) error {
	if err := j.load(ctx); err != nil {
		return err
	}

	if err := j.markStatus(ctx, types.StatusDeploying); err != nil {
		return err
	}

	j.Deps.BroadcastAppEvent(j.server, "app.progress", j.app.ID, "deploying", fmt.Sprintf("Deploying %s", j.app.Name))

	var task *taskrunner.BaseTask
	switch j.app.Source {
	case types.SourceCompose:
		opts, err := j.buildComposeDeployOptions(ctx)
		if err != nil {
			return err
		}
		task = dockerapptasks.ComposeDeploy(opts)
	default:
		opts, err := j.buildDeployOptions(ctx)
		if err != nil {
			return err
		}
		task = dockerapptasks.Deploy(opts)
	}

	result, err := j.Deps.RunTask(j.server, task).AsRoot().Dispatch(ctx)
	if err != nil {
		return fmt.Errorf("failed to dispatch deploy task: %w", err)
	}
	if !result.IsSuccessful() {
		return fmt.Errorf("deploy script failed: %s", result.GetOutput())
	}

	now := time.Now()
	updates := map[string]any{
		"status":      types.StatusRunning,
		"last_error":  nil,
		"deployed_at": &now,
	}
	if result.TaskModel != nil {
		updates["task_id"] = result.TaskModel.ID
	}
	if err := j.Deps.Repos.App().Update(ctx, j.app.ID, updates); err != nil {
		return fmt.Errorf("failed to mark app running: %w", err)
	}

	j.Deps.BroadcastAppEvent(j.server, "app.progress", j.app.ID, "running", fmt.Sprintf("%s deployed", j.app.Name))
	return nil
}

// Failed marks the app failed and records the error.
func (j *DeployJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("app_id", j.Payload.AppID).
		Msg("docker app deploy failed")

	msg := err.Error()
	_ = j.Deps.Repos.App().Update(ctx, j.Payload.AppID, map[string]any{
		"status":     types.StatusFailed,
		"last_error": &msg,
	})
	if j.server != nil {
		j.Deps.BroadcastAppEvent(j.server, "app.progress", j.Payload.AppID, "failed", err.Error())
	}
}

func (j *DeployJob) load(ctx context.Context) error {
	app, err := j.Deps.Repos.App().FindByIDWithRelations(ctx, j.Payload.AppID)
	if err != nil {
		return fmt.Errorf("failed to find app: %w", err)
	}
	j.app = app

	server, err := j.Deps.GetServer(ctx, app.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}
	j.server = server
	return nil
}

func (j *DeployJob) markStatus(ctx context.Context, status types.Status) error {
	return j.Deps.Repos.App().Update(ctx, j.app.ID, map[string]any{"status": status})
}

func (j *DeployJob) buildDeployOptions(ctx context.Context) (dockerapptasks.DeployOptions, error) {
	opts := dockerapptasks.DeployOptions{
		AppName:       j.app.Name,
		Container:     j.app.Container(),
		ImageRef:      j.app.ImageRef(),
		RestartPolicy: string(j.app.RestartPolicy),
	}

	for _, e := range j.app.EnvVars {
		opts.EnvVars = append(opts.EnvVars, dockerapptasks.EnvVar{
			Key:   e.Key,
			Value: e.Value.String(),
		})
	}
	for _, p := range j.app.Ports {
		proto := p.Protocol
		if proto == "" {
			proto = "tcp"
		}
		opts.Ports = append(opts.Ports, dockerapptasks.Port{
			HostPort:      p.HostPort,
			ContainerPort: p.ContainerPort,
			Protocol:      proto,
		})
	}
	for _, v := range j.app.Volumes {
		opts.Volumes = append(opts.Volumes, dockerapptasks.Volume{
			HostName:  types.DefaultVolumeName(j.app.Name, v.Name),
			MountPath: v.MountPath,
		})
	}

	domains := make([]dockerapptasks.DomainSpec, 0, len(j.app.Domains))
	for _, d := range j.app.Domains {
		domains = append(domains, dockerapptasks.DomainSpec{
			Domain:        d.Domain,
			ContainerPort: d.ContainerPort,
			TLS:           d.TLS,
		})
	}
	opts.Labels = dockerapptasks.TraefikLabels(j.app.Name, domains)

	if j.app.RegistryCredentialID != nil && j.Deps.Registry != nil {
		cred, err := j.Deps.Registry.GetForApp(ctx, *j.app.RegistryCredentialID, j.app.TeamID)
		if err != nil {
			return opts, fmt.Errorf("failed to load registry credential: %w", err)
		}
		if cred != nil {
			opts.RegistryURL = cred.URL
			opts.RegistryUsername = cred.Username
			opts.RegistryPassword = cred.Password
		}
	}

	return opts, nil
}

func (j *DeployJob) buildComposeDeployOptions(ctx context.Context) (dockerapptasks.ComposeDeployOptions, error) {
	opts := dockerapptasks.ComposeDeployOptions{
		AppName:     j.app.Name,
		Project:     j.app.ComposeProject(),
		ComposeYAML: j.app.ComposeYAMLValue(),
		ComposeEnv:  j.app.ComposeEnvValue(),
	}

	if j.app.RegistryCredentialID != nil && j.Deps.Registry != nil {
		cred, err := j.Deps.Registry.GetForApp(ctx, *j.app.RegistryCredentialID, j.app.TeamID)
		if err != nil {
			return opts, fmt.Errorf("failed to load registry credential: %w", err)
		}
		if cred != nil {
			opts.RegistryURL = cred.URL
			opts.RegistryUsername = cred.Username
			opts.RegistryPassword = cred.Password
		}
	}

	return opts, nil
}

// NewDeployTask builds the asynq task.
func NewDeployTask(appID string, userID *string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeDeployApp, DeployPayload{
		AppID:  appID,
		UserID: userID,
	}, asynq.TaskID(pkgjobs.Dedup("deploy_dockerapp", appID)))
}
