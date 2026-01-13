package jobs

import (
	"context"
)

// InstallSSLJob handles SSL certificate installation
type InstallSSLJob struct {
	SiteJobBase
	Payload InstallSSLPayload
}

// Type returns the job type
func (j *InstallSSLJob) Type() string {
	return TypeInstallSSL
}

// Handle executes the install SSL job
func (j *InstallSSLJob) Handle(ctx context.Context) error {
	// TODO: Implement SSL installation logic
	// 1. Get site
	// 2. Request certificate from Let's Encrypt
	// 3. Update Caddy configuration
	// 4. Reload Caddy
	// 5. Update certificate record
	j.LogInfo("Install SSL job executed (not implemented)",
		"site_id", j.Payload.SiteID,
		"address", j.Payload.Address,
	)
	return nil
}

// Failed handles job failure
func (j *InstallSSLJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Install SSL job failed",
		"site_id", j.Payload.SiteID,
		"address", j.Payload.Address,
	)
}
