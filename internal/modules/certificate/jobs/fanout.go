package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"

	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

// TypeFanoutCertificate is the asynq task type dispatched by
// StoredCertificateService.Update whenever the cert/key bytes change.
//
// The job walks every site / docker-app / docker-compose that
// references the cert and enqueues the appropriate downstream task
// (site:install_ssl for sites, docker:sync_traefik_config for docker
// applications, docker:sync_compose_traefik_config for compose
// stacks). The downstream Traefik sync jobs already know how to
// materialise stored cert files on the server thanks to Phase 6.5's
// resolveStoredCertMaterials helper — so a single fanout dispatch
// re-pushes the new cert to every referencing resource.
//
// We don't run the SSH writes inline here: each downstream task has
// its own retry semantics and broadcast contract, and fanning out via
// the queue keeps a slow re-deploy on one resource from blocking the
// others.
const TypeFanoutCertificate = "certificate:fanout"

// FanoutCertificatePayload carries the cert id + team id. The job
// re-fetches the usage list each tick — a domain that was added /
// removed between the Update call and the job firing is correctly
// included / excluded.
type FanoutCertificatePayload struct {
	CertificateID string `json:"certificate_id"`
	TeamID        string `json:"team_id"`
}

// FanoutCertificateJob implements the fanout logic.
type FanoutCertificateJob struct {
	Deps    *JobDeps
	Payload FanoutCertificatePayload
}

// NewFanoutCertificateJob is the asynq constructor (see register.go).
func NewFanoutCertificateJob(p FanoutCertificatePayload) pkgjobs.Handler {
	return &FanoutCertificateJob{Deps: deps, Payload: p}
}

// downstream payload shapes — duplicated here (not imported) to avoid
// a circular dep on the docker / site modules. The task types live in
// their respective modules' const decls; we publish here using the
// raw string values + identical JSON field tags.
//
// If either downstream changes its payload, this code will silently
// dispatch malformed tasks and the worker will fail them — keep this
// list in lockstep when touching the downstream signatures.
type dockerSyncTraefikPayload struct {
	ApplicationID string `json:"application_id"`
	ServerID      string `json:"server_id"`
	TeamID        string `json:"team_id"`
}

type dockerSyncComposeTraefikPayload struct {
	ComposeID string `json:"compose_id"`
	ServerID  string `json:"server_id"`
	TeamID    string `json:"team_id"`
}

type siteInstallSSLPayload struct {
	SiteID  string `json:"site_id"`
	Address string `json:"address"`
}

// Handle walks UsageDispatchRefs and enqueues a downstream task per
// resource. Errors on a single dispatch are logged but don't abort
// the loop — one stale server shouldn't block the fanout for the
// rest of the team.
func (j *FanoutCertificateJob) Handle(ctx context.Context) error {
	refs, err := j.Deps.Repos.StoredCertificates.UsageDispatchRefs(
		ctx, j.Payload.TeamID, j.Payload.CertificateID,
	)
	if err != nil {
		return fmt.Errorf("usage refs: %w", err)
	}

	dispatched := 0
	for _, r := range refs {
		var (
			taskType string
			payload  any
		)
		switch r.Kind {
		case "site":
			taskType = "site:install_ssl"
			payload = siteInstallSSLPayload{SiteID: r.SiteID, Address: r.Address}
		case "docker_application":
			taskType = "docker:sync_traefik_config"
			payload = dockerSyncTraefikPayload{
				ApplicationID: r.ApplicationID,
				ServerID:      r.ServerID,
				TeamID:        r.TeamID,
			}
		case "docker_compose":
			taskType = "docker:sync_compose_traefik_config"
			payload = dockerSyncComposeTraefikPayload{
				ComposeID: r.ComposeID,
				ServerID:  r.ServerID,
				TeamID:    r.TeamID,
			}
		default:
			j.Deps.Logger.Warn().
				Str("kind", r.Kind).
				Str("certificate_id", j.Payload.CertificateID).
				Msg("certificate fanout: unknown ref kind; skipping")
			continue
		}

		body, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			j.Deps.Logger.Error().Err(marshalErr).
				Str("task_type", taskType).
				Msg("certificate fanout: payload marshal failed")
			continue
		}
		task := asynq.NewTask(taskType, body)
		if _, enqueueErr := j.Deps.Queue.Enqueue(task); enqueueErr != nil {
			j.Deps.Logger.Error().Err(enqueueErr).
				Str("task_type", taskType).
				Str("certificate_id", j.Payload.CertificateID).
				Msg("certificate fanout: enqueue failed")
			continue
		}
		dispatched++
	}

	j.Deps.Logger.Info().
		Str("certificate_id", j.Payload.CertificateID).
		Str("team_id", j.Payload.TeamID).
		Int("dispatched", dispatched).
		Int("ref_count", len(refs)).
		Msg("certificate fanout complete")

	j.Deps.Broadcaster.BroadcastToTeam(j.Payload.TeamID, "certificate.fanout_complete", map[string]any{
		"team_id":        j.Payload.TeamID,
		"certificate_id": j.Payload.CertificateID,
		"dispatched":     dispatched,
		"ref_count":      len(refs),
	})
	return nil
}

// NewFanoutCertificateTask packages the fanout task for the service
// layer. Dedup by certificate id so a burst of consecutive Updates
// only triggers one downstream wave.
func NewFanoutCertificateTask(certificateID, teamID string) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeFanoutCertificate, FanoutCertificatePayload{
		CertificateID: certificateID,
		TeamID:        teamID,
	}, pkgjobs.Dedup("certificate-fanout", certificateID))
}
