package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/docker/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

// TypeSyncComposeTraefikConfig is the asynq task type for writing the
// per-compose Traefik dynamic-config file. Triggered on any compose
// domain mutation (create/update/delete) and on successful deploy.
// Parallel to TypeSyncTraefikConfig but scoped to compose stacks —
// the renderer + on-disk filename both differ enough that one shared
// job would be a worse fit than two siblings.
const TypeSyncComposeTraefikConfig = "docker:sync_compose_traefik_config"

// SyncComposeTraefikConfigPayload carries IDs only — the job
// re-fetches the compose row + domains so a stale payload can't
// write outdated config (same shape as SyncTraefikConfigPayload).
type SyncComposeTraefikConfigPayload struct {
	ComposeID             string `json:"compose_id"`
	ServerID              string `json:"server_id"`
	TeamID                string `json:"team_id"`
	ForceCertificateRetry bool   `json:"force_certificate_retry,omitempty"`
}

// SyncComposeTraefikConfigJob renders the per-compose YAML and
// uploads it via SSH. Idempotent: same inputs produce identical
// output, so dispatching on every domain change is safe with no
// ordering coordination.
type SyncComposeTraefikConfigJob struct {
	Deps    *JobDeps
	Payload SyncComposeTraefikConfigPayload
}

// NewSyncComposeTraefikConfigJob is the asynq constructor (see
// register.go).
func NewSyncComposeTraefikConfigJob(p SyncComposeTraefikConfigPayload) pkgjobs.Handler {
	return &SyncComposeTraefikConfigJob{Deps: deps, Payload: p}
}

// Handle re-fetches the compose + project + domains, renders the
// YAML, and writes it via SSH. Returning an error makes asynq retry
// — typically transient SSH failures.
func (j *SyncComposeTraefikConfigJob) Handle(ctx context.Context) error {
	compose, err := j.Deps.Repos.Compose().FindByIDAndTeamServer(
		ctx, j.Payload.ComposeID, j.Payload.TeamID, j.Payload.ServerID,
	)
	if err != nil {
		return fmt.Errorf("find compose: %w", err)
	}
	project, err := j.Deps.Repos.Project().FindByIDAndTeamServer(
		ctx, compose.ProjectID, j.Payload.TeamID, j.Payload.ServerID,
	)
	if err != nil {
		return fmt.Errorf("find project: %w", err)
	}
	server, err := j.Deps.ServerRepos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("find server: %w", err)
	}

	domains, err := j.Deps.Repos.Domain().ListForCompose(ctx, compose.ID)
	if err != nil {
		return fmt.Errorf("list compose domains: %w", err)
	}

	projectSlug := tasks.SlugFromName(project.Name)
	composeSlug := tasks.SlugFromName(compose.Name)
	// The compose project name we pass to `docker compose -p` is
	// `<project>-<compose>` (see jobs/deploy_compose.go). Containers
	// then resolve as `<project>-<compose>-<service>-1`. Keep these
	// in lockstep — if the deploy script changes the -p value the
	// renderer must follow.
	projectName := fmt.Sprintf("%s-%s", projectSlug, composeSlug)
	routerSuffix := ""
	if j.Payload.ForceCertificateRetry {
		routerSuffix = fmt.Sprintf("-retry-%d", time.Now().UnixNano())
	}

	yaml := tasks.RenderComposeTraefikConfig(tasks.ComposeTraefikConfigArgs{
		ProjectSlug:  projectSlug,
		ComposeSlug:  composeSlug,
		ProjectName:  projectName,
		Domains:      domains,
		RouterSuffix: routerSuffix,
	})

	// Same stored-cert materialisation as the application sync —
	// resolve PEMs for any domain that references a stored cert and
	// upload them under /var/lib/launch/traefik/certs/<id>/.
	// The application job has the canonical resolver (this module
	// shares the certificate repo via JobDeps.CertRepos).
	if certMaterials, mErr := resolveStoredCertMaterials(ctx, j.Deps, j.Payload.TeamID, domains); mErr != nil {
		return fmt.Errorf("resolve stored certs: %w", mErr)
	} else if len(certMaterials) > 0 {
		certTask := tasks.WriteStoredCertificatesTask(certMaterials)
		certResult, certErr := j.Deps.RunTask(server, certTask).AsRoot().Dispatch(ctx)
		if certErr != nil {
			return fmt.Errorf("ssh write stored certs: %w", certErr)
		}
		if certResult != nil && !certResult.IsSuccessful() {
			return fmt.Errorf("stored cert write failed: %s", certResult.GetOutput())
		}
	}

	task := tasks.WriteComposeTraefikConfigTask(projectSlug, composeSlug, yaml)
	result, runErr := j.Deps.RunTask(server, task).AsRoot().Dispatch(ctx)
	if runErr != nil {
		return fmt.Errorf("ssh write: %w", runErr)
	}
	if result != nil && !result.IsSuccessful() {
		return fmt.Errorf("compose traefik config write failed: %s", result.GetOutput())
	}

	j.Deps.BroadcastToTeam(j.Payload.TeamID, "docker.compose.traefik_synced", map[string]any{
		"compose_id":   compose.ID,
		"server_id":    server.ID,
		"team_id":      j.Payload.TeamID,
		"domain_count": len(domains),
	})
	return nil
}

// Failed mirrors SyncTraefikConfigJob.Failed — log + give up; the
// next domain change re-triggers the job, so we don't surface a hard
// user-visible error.
func (j *SyncComposeTraefikConfigJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("compose_id", j.Payload.ComposeID).
		Str("server_id", j.Payload.ServerID).
		Msg("sync compose traefik config job gave up")
}

// NewSyncComposeTraefikConfigTask packages the asynq task for the
// service layer. Dedup key namespaced under docker-compose-traefik-
// sync so it can't collide with the application-side job.
func NewSyncComposeTraefikConfigTask(composeID, serverID, teamID string) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeSyncComposeTraefikConfig, SyncComposeTraefikConfigPayload{
		ComposeID: composeID,
		ServerID:  serverID,
		TeamID:    teamID,
	}, pkgjobs.Dedup("docker-compose-traefik-sync", composeID))
}

func NewRetryComposeTraefikConfigTask(composeID, serverID, teamID string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeSyncComposeTraefikConfig, SyncComposeTraefikConfigPayload{
		ComposeID:             composeID,
		ServerID:              serverID,
		TeamID:                teamID,
		ForceCertificateRetry: true,
	}, asynq.Unique(time.Minute))
}
