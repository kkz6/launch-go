package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/docker/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// TypeRemoveCompose tears down a `docker compose` stack the same way
// TypeRemoveApplication tears down a single app container. We split
// it from the deploy job because (a) the lifecycle is bounded
// (docker compose down rarely takes more than a few seconds), and
// (b) the row is already soft-deleted by the service so the job
// can't rely on a live compose row anymore.
const TypeRemoveCompose = "docker:remove_compose"

// RemoveComposePayload — IDs only plus the resolved project name
// (compose's `--project-name` arg). Same rationale as the app
// version: avoids Unscoped queries.
type RemoveComposePayload struct {
	ComposeID string `json:"compose_id"`
	ProjectID string `json:"project_id"`
	ServerID  string `json:"server_id"`
	TeamID    string `json:"team_id"`
	// ProjectName is `<project-slug>-<compose-slug>` — the label
	// docker compose used at deploy time.
	ProjectName string `json:"project_name"`
	// RemoveVolumes flips `docker compose down` ↔ `down -v`. False by
	// default — preserves named volumes so a fat-fingered Delete
	// doesn't destroy persistent data. The UI surfaces this as an
	// opt-in checkbox on the Delete confirmation dialog.
	RemoveVolumes bool `json:"remove_volumes"`
}

type RemoveComposeJob struct {
	Deps    *JobDeps
	Payload RemoveComposePayload
}

func NewRemoveComposeJob(p RemoveComposePayload) pkgjobs.Handler {
	return &RemoveComposeJob{Deps: deps, Payload: p}
}

func (j *RemoveComposeJob) Handle(ctx context.Context) error {
	server, err := j.Deps.ServerRepos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("find server: %w", err)
	}

	if j.Payload.ProjectName == "" {
		// Compose was never deployed — broadcast and bail.
		j.Deps.BroadcastToTeam(j.Payload.TeamID, "docker.compose.removed", map[string]any{
			"compose_id": j.Payload.ComposeID,
			"project_id": j.Payload.ProjectID,
			"server_id":  j.Payload.ServerID,
			"team_id":    j.Payload.TeamID,
		})
		return nil
	}

	task := taskrunner.NewBaseTask(
		taskrunner.WithName("Remove compose stack"),
		taskrunner.WithScript(tasks.RemoveComposeScript(j.Payload.ProjectName, j.Payload.RemoveVolumes)),
		// `docker compose down -v --remove-orphans` is bounded but a
		// large stack with slow shutdown hooks can take a minute. Use
		// 3 minutes to give it room without hanging asynq forever.
		taskrunner.WithTimeoutSeconds(180),
	)
	result, runErr := j.Deps.RunTask(server, task).AsRoot().Dispatch(ctx)
	if runErr != nil {
		// SSH-level error → asynq retry.
		return runErr
	}
	if result != nil && !result.IsSuccessful() {
		j.Deps.Logger.Warn().
			Str("compose_id", j.Payload.ComposeID).
			Str("project_name", j.Payload.ProjectName).
			Msg("compose removal exited non-zero; check the host for stragglers")
	}

	j.Deps.BroadcastToTeam(j.Payload.TeamID, "docker.compose.removed", map[string]any{
		"compose_id": j.Payload.ComposeID,
		"project_id": j.Payload.ProjectID,
		"server_id":  j.Payload.ServerID,
		"team_id":    j.Payload.TeamID,
	})
	return nil
}

func (j *RemoveComposeJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("compose_id", j.Payload.ComposeID).
		Str("project_name", j.Payload.ProjectName).
		Msg("compose removal permanently failed — stack may still be running")
}

func NewRemoveComposeTask(
	composeID, projectID, serverID, teamID, projectName string,
	removeVolumes bool,
) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeRemoveCompose, RemoveComposePayload{
		ComposeID:     composeID,
		ProjectID:     projectID,
		ServerID:      serverID,
		TeamID:        teamID,
		ProjectName:   projectName,
		RemoveVolumes: removeVolumes,
	}, pkgjobs.Dedup("docker-remove-compose", composeID))
}
