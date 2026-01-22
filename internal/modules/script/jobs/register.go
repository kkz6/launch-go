package jobs

import (
	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/script/repositories"
	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/pkg/app"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

var deps *JobDeps

// Register initializes and registers all script job handlers.
func Register(
	mux *asynq.ServeMux,
	appDeps app.Deps,
	repos *repositories.Registry,
	serverRepos *serverrepos.Registry,
) {
	deps = NewJobDeps(appDeps, repos, serverRepos)
	registerHandlers(mux)
}

func registerHandlers(mux *asynq.ServeMux) {
	pkgjobs.RegisterTyped(mux, TypeExecuteScript, NewExecuteScriptJob)
}
