package jobs

import (
	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/docker/repositories"
	"github.com/kkz6/launch-go/internal/pkg/app"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

var deps *JobDeps

// Register initializes and registers all docker job handlers.
func Register(mux *asynq.ServeMux, appDeps app.Deps, repos *repositories.Registry) {
	deps = NewJobDeps(appDeps, repos)
	registerHandlers(mux)
}

func registerHandlers(mux *asynq.ServeMux) {
	pkgjobs.RegisterTyped(mux, TypeDeployDockerService, NewDeployDockerServiceJob)
	pkgjobs.RegisterTyped(mux, TypeManageDockerService, NewManageDockerServiceJob)
}
