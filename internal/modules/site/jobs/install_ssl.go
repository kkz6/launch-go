package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeInstallSSL = "site:install_ssl"

// InstallSSLPayload holds data for SSL installation
type InstallSSLPayload struct {
	SiteID  string `json:"site_id"`
	Address string `json:"address"`
}

// InstallSSLJob handles SSL certificate installation by delegating to
// the Caddyfile install pipeline. Caddy's auto-TLS handles Let's
// Encrypt provisioning when site.TLSSetting == "auto"; the custom and
// internal paths are configured via the TLS snippet that
// InstallCaddyfileJob renders. The site-cert PEM materialisation is
// also handled there (via WriteSiteCertificatesTask), so SSL install
// boils down to "reload the Caddyfile with the current settings."
//
// Callers (the SSL update handler, the certificate fanout job) get a
// single asynq task type to enqueue; the worker takes care of
// dispatching the downstream Caddyfile job.
type InstallSSLJob struct {
	Deps    *JobDeps
	Payload InstallSSLPayload
}

func NewInstallSSLJob(p InstallSSLPayload) pkgjobs.Handler {
	return &InstallSSLJob{Deps: deps, Payload: p}
}

// Handle dispatches a downstream Caddyfile install for the site —
// install_caddyfile re-resolves the active certificate, writes the
// PEM bytes to disk, and reloads Caddy with the updated config.
func (j *InstallSSLJob) Handle(ctx context.Context) error {
	j.Deps.Logger.Info().
		Str("site_id", j.Payload.SiteID).
		Str("address", j.Payload.Address).
		Msg("install_ssl: delegating to install_caddyfile")

	task, err := NewInstallCaddyfileTask(j.Payload.SiteID, nil)
	if err != nil {
		return fmt.Errorf("build install_caddyfile task: %w", err)
	}
	if _, err := j.Deps.Queue.Enqueue(task); err != nil {
		return fmt.Errorf("enqueue install_caddyfile: %w", err)
	}
	return nil
}

// Failed handles job failure
func (j *InstallSSLJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("site_id", j.Payload.SiteID).
		Str("address", j.Payload.Address).
		Msg("Install SSL job failed")
}

// NewInstallSSLTask creates an install SSL job
func NewInstallSSLTask(siteID, address string) (*asynq.Task, error) {
	return pkgjobs.Task(TypeInstallSSL, InstallSSLPayload{
		SiteID:  siteID,
		Address: address,
	})
}
