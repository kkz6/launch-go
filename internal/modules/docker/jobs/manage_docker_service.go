package jobs

import (
	"context"
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/docker/tasks"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

const TypeManageDockerService = "docker:manage_service"

// ManageDockerServicePayload holds the data for a manage docker service job
type ManageDockerServicePayload struct {
	DockerServiceID string `json:"docker_service_id"`
	TeamID          string `json:"team_id"`
	ServerID        string `json:"server_id"`
	Action          string `json:"action"` // "stop", "start", "restart"
}

// ManageDockerServiceJob manages a docker service (stop/start/restart)
type ManageDockerServiceJob struct {
	Deps    *JobDeps
	Payload ManageDockerServicePayload
}

// NewManageDockerServiceJob creates a new ManageDockerServiceJob
func NewManageDockerServiceJob(p ManageDockerServicePayload) pkgjobs.Handler {
	return &ManageDockerServiceJob{Deps: deps, Payload: p}
}

// Handle executes the manage docker service job
func (j *ManageDockerServiceJob) Handle(ctx context.Context) error {
	svc, err := j.Deps.Repos.DockerService().FindByID(ctx, j.Payload.DockerServiceID)
	if err != nil {
		return fmt.Errorf("failed to find docker service: %w", err)
	}

	var server servermodels.Server
	if err := j.Deps.DB.WithContext(ctx).First(&server, "id = ?", j.Payload.ServerID).Error; err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	config := tasks.ManageConfig{ContainerName: svc.ContainerName}

	var task *taskrunner.BaseTask

	switch j.Payload.Action {
	case "stop":
		task = tasks.StopDockerService(config)
	case "start":
		task = tasks.StartDockerService(config)
	case "restart":
		task = tasks.RestartDockerService(config)
	default:
		return fmt.Errorf("unknown action: %s", j.Payload.Action)
	}

	if _, err := j.Deps.TaskRunnerDeps.NewRunner(&server, task).AsRoot().RunInBackground(ctx); err != nil {
		return fmt.Errorf("failed to execute %s task: %w", j.Payload.Action, err)
	}

	j.Deps.BroadcastToTeam(j.Payload.TeamID, fmt.Sprintf("docker_service.%s", j.Payload.Action), map[string]any{
		"docker_service_id": svc.ID,
		"action":            j.Payload.Action,
	})

	return nil
}

// Failed handles job failure
func (j *ManageDockerServiceJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("docker_service_id", j.Payload.DockerServiceID).
		Str("action", j.Payload.Action).
		Msg("failed to manage docker service")
}

// NewManageDockerServiceTask creates and dispatches a manage docker service job
func NewManageDockerServiceTask(dockerServiceID, teamID, serverID, action string) error {
	return deps.Dispatch(TypeManageDockerService, ManageDockerServicePayload{
		DockerServiceID: dockerServiceID,
		TeamID:          teamID,
		ServerID:        serverID,
		Action:          action,
	})
}
