package jobs

import (
	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/dockerapp/contracts"
	"github.com/kkz6/launch-go/internal/pkg/app"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

var deps *JobDeps

// Register initializes and registers all dockerapp job handlers.
func Register(mux *asynq.ServeMux, appDeps app.Deps, repos contracts.RepositoryRegistry, registry contracts.RegistryCredentialReader) {
	deps = NewJobDeps(appDeps, repos, registry)
	pkgjobs.RegisterTyped(mux, TypeDeployApp, NewDeployJob)
	pkgjobs.RegisterTyped(mux, TypeUninstallApp, NewUninstallJob)
}
