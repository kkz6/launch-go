// Package jobs hosts the certificate module's asynq job handlers.
//
// Phase 6 introduces two jobs:
//
//   - certificate:warn_expiring — scheduled daily by
//     internal/schedule/kernel.go. Walks every alive stored cert,
//     finds rows whose not_after falls in the next 30 days, and emits
//     a certificate.expiring_soon broadcast per cert. Frontend
//     useCertificateAlerts composable picks these up and surfaces the
//     dashboard banner.
//
//   - certificate:fanout — dispatched from StoredCertificateService
//     on a content-change Update. Currently a stub: emits
//     certificate.fanout_required so the UI can show "N redeploys
//     pending" and prompts the user. Wiring this to actually re-run
//     install_caddyfile (for sites) and rewrite cert files on the
//     docker server + reload Traefik (for docker domains) is the next
//     follow-up — it needs cross-module job dispatch and SSH file
//     materialisation, both of which are out of Phase 6's scope.
package jobs

import (
	certrepos "github.com/kkz6/launch-go/internal/modules/certificate/repositories"
	"github.com/kkz6/launch-go/internal/pkg/app"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

// JobDeps holds shared dependencies for certificate-module asynq jobs.
// Mirrors the shape used by internal/modules/docker/jobs/deps.go.
type JobDeps struct {
	*pkgjobs.Deps
	Repos *certrepos.Registry
}

// NewJobDeps wires JobDeps from app-level dependencies.
func NewJobDeps(appDeps app.Deps, repos *certrepos.Registry) *JobDeps {
	return &JobDeps{
		Deps: &pkgjobs.Deps{
			DB:          appDeps.DB,
			Logger:      appDeps.Logger,
			Queue:       appDeps.Queue,
			Broadcaster: appDeps.WebSocket,
			Dispatcher:  appDeps.Dispatcher,
		},
		Repos: repos,
	}
}
