package jobs

import (
	"context"

	"github.com/hibiken/asynq"

	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeInstallSSL = "site:install_ssl"

// InstallSSLPayload holds data for SSL installation
type InstallSSLPayload struct {
	SiteID  string `json:"site_id"`
	Address string `json:"address"`
}

// InstallSSLJob handles SSL certificate installation
type InstallSSLJob struct {
	Deps    *JobDeps
	Payload InstallSSLPayload
}

func NewInstallSSLJob(p InstallSSLPayload) pkgjobs.Handler {
	return &InstallSSLJob{Deps: deps, Payload: p}
}

// Handle executes the install SSL job
func (j *InstallSSLJob) Handle(ctx context.Context) error {
	// TODO: Implement SSL installation logic
	// 1. Get site
	// 2. Request certificate from Let's Encrypt
	// 3. Update Caddy configuration
	// 4. Reload Caddy
	// 5. Update certificate record
	j.Deps.Logger.Info().
		Str("site_id", j.Payload.SiteID).
		Str("address", j.Payload.Address).
		Msg("Install SSL job executed (not implemented)")

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
