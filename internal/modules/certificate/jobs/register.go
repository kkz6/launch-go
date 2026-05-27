package jobs

import (
	"github.com/hibiken/asynq"

	certrepos "github.com/kkz6/launch-go/internal/modules/certificate/repositories"
	"github.com/kkz6/launch-go/internal/pkg/app"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

// deps is the package-private dependency bundle shared by every asynq
// job constructor in this package. Set once by Register at boot. Same
// shape the docker / server modules use.
var deps *JobDeps

// Register wires JobDeps and binds every asynq task type this module
// publishes to its handler.
func Register(
	mux *asynq.ServeMux,
	appDeps app.Deps,
	repos *certrepos.Registry,
) {
	deps = NewJobDeps(appDeps, repos)
	pkgjobs.RegisterTyped(mux, TypeWarnExpiringCertificates, NewWarnExpiringCertificatesJob)
}
