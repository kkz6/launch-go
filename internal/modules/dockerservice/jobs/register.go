package jobs

import (
	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/dockerservice/contracts"
	"github.com/kkz6/launch-go/internal/pkg/app"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

var deps *JobDeps

// Register initializes and registers all docker-service job handlers.
func Register(mux *asynq.ServeMux, appDeps app.Deps, repos contracts.RepositoryRegistry) {
	deps = NewJobDeps(appDeps, repos)
	pkgjobs.RegisterTyped(mux, TypeInstallDockerService, NewInstallJob)
	pkgjobs.RegisterTyped(mux, TypeUninstallDockerService, NewUninstallJob)
}
