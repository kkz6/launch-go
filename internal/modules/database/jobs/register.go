package jobs

import (
	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/database/contracts"
	"github.com/kkz6/launch-go/internal/pkg/app"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

var deps *JobDeps

// Register initializes and registers all database job handlers.
func Register(mux *asynq.ServeMux, appDeps app.Deps, repos contracts.RepositoryRegistry) {
	deps = NewJobDeps(appDeps, repos)
	registerHandlers(mux)
}

func registerHandlers(mux *asynq.ServeMux) {
	pkgjobs.RegisterTyped(mux, TypeInstallDatabase, NewInstallDatabaseJob)
	pkgjobs.RegisterTyped(mux, TypeUninstallDatabase, NewUninstallDatabaseJob)
	pkgjobs.RegisterTyped(mux, TypeInstallDatabaseUser, NewInstallDatabaseUserJob)
	pkgjobs.RegisterTyped(mux, TypeUpdateDatabaseUser, NewUpdateDatabaseUserJob)
	pkgjobs.RegisterTyped(mux, TypeUninstallDatabaseUser, NewUninstallDatabaseUserJob)
	pkgjobs.RegisterTyped(mux, TypeSyncDatabases, NewSyncDatabasesJob)
}
