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
// (the compose project label `com.docker.compose.project`) and the
// individual project + compose slugs so the teardown script can
// also clean up the on-disk stack dir + Traefik dynamic-config file.
// Same rationale as the app version: avoids Unscoped queries.
//
// ProjectSlug + ComposeSlug were added when the teardown was
// rewritten to use docker labels — the old script tried to invoke
// `docker compose down` without the compose file in cwd, which
// silently no-op'd and left containers running. See the comment at
// the top of tasks/remove_compose.go for the full history.
type RemoveComposePayload struct {
	ComposeID string `json:"compose_id"`
	ProjectID string `json:"project_id"`
	ServerID  string `json:"server_id"`
	TeamID    string `json:"team_id"`
	// ProjectName is `<project-slug>-<compose-slug>` — the label
	// docker compose used at deploy time.
	ProjectName string `json:"project_name"`
	// ProjectSlug + ComposeSlug let the teardown clean up the
	// per-stack directory under /var/lib/launch/projects/… and the
	// per-compose Traefik config file. Both empty for old queued
	// payloads (the script falls back to containers/networks only).
	ProjectSlug string `json:"project_slug"`
	ComposeSlug string `json:"compose_slug"`
	// RemoveVolumes flips whether named volumes are deleted along
	// with containers + networks. False by default — preserves data
	// across an accidental Delete. The UI surfaces this as an opt-in
	// checkbox on the Delete confirmation dialog.
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
		taskrunner.WithScript(tasks.RemoveComposeScript(
			j.Payload.ProjectName,
			j.Payload.ProjectSlug,
			j.Payload.ComposeSlug,
			j.Payload.RemoveVolumes,
		)),
		// Label-based teardown is fast (a few "docker rm" calls). 3
		// minutes is plenty even for a 10-service stack and gives
		// asynq's retry policy a sane bound.
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

// NewRemoveComposeTask builds the asynq Task for compose teardown.
//
// projectName is the docker compose project label
// (<projectSlug>-<composeSlug>). projectSlug and composeSlug are
// passed SEPARATELY so the script can reconstruct the on-disk paths
// for stack-dir + Traefik-config cleanup — joining them with "-"
// would be lossy (project names can contain hyphens). Either slug
// may be empty; the script falls back to containers/networks-only
// cleanup in that case.
func NewRemoveComposeTask(
	composeID, projectID, serverID, teamID,
	projectName, projectSlug, composeSlug string,
	removeVolumes bool,
) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeRemoveCompose, RemoveComposePayload{
		ComposeID:     composeID,
		ProjectID:     projectID,
		ServerID:      serverID,
		TeamID:        teamID,
		ProjectName:   projectName,
		ProjectSlug:   projectSlug,
		ComposeSlug:   composeSlug,
		RemoveVolumes: removeVolumes,
	}, pkgjobs.Dedup("docker-remove-compose", composeID))
}
