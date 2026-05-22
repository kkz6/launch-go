package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/docker/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

// TypeSyncTraefikConfig is the asynq task type for writing the Traefik
// dynamic-config file for an application. Triggered on any domain
// mutation (create/update/delete) and on successful deploy.
const TypeSyncTraefikConfig = "docker:sync_traefik_config"

// SyncTraefikConfigPayload carries IDs only, per the CLAUDE.md callback
// rules — the job re-fetches the application + domains so a stale
// payload can't write outdated config.
type SyncTraefikConfigPayload struct {
	ApplicationID string `json:"application_id"`
	ServerID      string `json:"server_id"`
	TeamID        string `json:"team_id"`
}

// SyncTraefikConfigJob renders the Traefik YAML for an application and
// uploads it to the docker server via SSH. Idempotent — every run
// produces the same content for the same inputs, so it's safe to
// dispatch on every domain change without coordinating concurrency.
type SyncTraefikConfigJob struct {
	Deps    *JobDeps
	Payload SyncTraefikConfigPayload
}

// NewSyncTraefikConfigJob is the asynq constructor (see register.go).
func NewSyncTraefikConfigJob(p SyncTraefikConfigPayload) pkgjobs.Handler {
	return &SyncTraefikConfigJob{Deps: deps, Payload: p}
}

// Handle re-fetches the application + project + domains and writes the
// rendered YAML. Returning an error makes asynq retry — these failures
// are typically transient (SSH temporarily unreachable) so a retry is
// appropriate.
func (j *SyncTraefikConfigJob) Handle(ctx context.Context) error {
	app, err := j.Deps.Repos.Application().FindByIDAndTeamServer(
		ctx, j.Payload.ApplicationID, j.Payload.TeamID, j.Payload.ServerID,
	)
	if err != nil {
		return fmt.Errorf("find application: %w", err)
	}
	project, err := j.Deps.Repos.Project().FindByIDAndTeamServer(
		ctx, app.ProjectID, j.Payload.TeamID, j.Payload.ServerID,
	)
	if err != nil {
		return fmt.Errorf("find project: %w", err)
	}
	server, err := j.Deps.ServerRepos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("find server: %w", err)
	}

	domains, err := j.Deps.Repos.Domain().ListForApplication(ctx, app.ID)
	if err != nil {
		return fmt.Errorf("list domains: %w", err)
	}

	projectSlug := tasks.SlugFromName(project.Name)
	appSlug := tasks.SlugFromName(app.Name)
	containerName := tasks.ContainerNameFor(project, app)

	yaml := tasks.RenderTraefikConfig(tasks.TraefikConfigArgs{
		ProjectSlug:   projectSlug,
		AppSlug:       appSlug,
		ContainerName: containerName,
		InternalPort:  app.InternalPort,
		Domains:       domains,
	})

	task := tasks.WriteTraefikConfigTask(projectSlug, appSlug, yaml)
	result, runErr := j.Deps.RunTask(server, task).AsRoot().Dispatch(ctx)
	if runErr != nil {
		return fmt.Errorf("ssh write: %w", runErr)
	}
	if result != nil && !result.IsSuccessful() {
		return fmt.Errorf("traefik config write failed: %s", result.GetOutput())
	}

	j.Deps.BroadcastToTeam(j.Payload.TeamID, "docker.application.traefik_synced", map[string]any{
		"application_id": app.ID,
		"server_id":      server.ID,
		"team_id":        j.Payload.TeamID,
		"domain_count":   len(domains),
	})
	return nil
}

// Failed lets asynq report a giving-up event. We don't surface it to the
// user as a hard failure — the domain row is already persisted, the
// re-sync just couldn't reach the server. Next domain change retriggers
// the job; an operator can also POST a manual sync if we expose one.
func (j *SyncTraefikConfigJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("application_id", j.Payload.ApplicationID).
		Str("server_id", j.Payload.ServerID).
		Msg("sync traefik config job gave up")
}

// NewSyncTraefikConfigTask packages the asynq task for the service layer.
func NewSyncTraefikConfigTask(applicationID, serverID, teamID string) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeSyncTraefikConfig, SyncTraefikConfigPayload{
		ApplicationID: applicationID,
		ServerID:      serverID,
		TeamID:        teamID,
	}, pkgjobs.Dedup("docker-traefik-sync", applicationID))
}
