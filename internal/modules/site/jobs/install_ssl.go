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
	ctx     *JobContext
	Payload InstallSSLPayload
}

// NewInstallSSLJob creates a new InstallSSLJob with the given context and payload
func NewInstallSSLJob(ctx *JobContext, payload InstallSSLPayload) *InstallSSLJob {
	return &InstallSSLJob{
		ctx:     ctx,
		Payload: payload,
	}
}

// Handle executes the install SSL job
func (j *InstallSSLJob) Handle(ctx context.Context) error {
	// TODO: Implement SSL installation logic
	// 1. Get site
	// 2. Request certificate from Let's Encrypt
	// 3. Update Caddy configuration
	// 4. Reload Caddy
	// 5. Update certificate record
	j.ctx.LogInfo("Install SSL job executed (not implemented)",
		"site_id", j.Payload.SiteID,
		"address", j.Payload.Address,
	)
	return nil
}

// Failed handles job failure
func (j *InstallSSLJob) Failed(ctx context.Context, err error) {
	j.ctx.LogError(err, "Install SSL job failed",
		"site_id", j.Payload.SiteID,
		"address", j.Payload.Address,
	)
}

// NewInstallSSLTask creates an install SSL job
func NewInstallSSLTask(siteID, address string) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeInstallSSL, InstallSSLPayload{
		SiteID:  siteID,
		Address: address,
	})
}
