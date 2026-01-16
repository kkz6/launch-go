package jobs

import (
	"github.com/hibiken/asynq"

	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

// RegisterHandlers registers all database job handlers with the asynq mux.
func RegisterHandlers(mux *asynq.ServeMux) {
	pkgjobs.RegisterHandler(mux, TypeInstallDatabase, jobContext, NewInstallDatabaseJob)
	pkgjobs.RegisterHandler(mux, TypeUninstallDatabase, jobContext, NewUninstallDatabaseJob)
	pkgjobs.RegisterHandler(mux, TypeInstallDatabaseUser, jobContext, NewInstallDatabaseUserJob)
	pkgjobs.RegisterHandler(mux, TypeUpdateDatabaseUser, jobContext, NewUpdateDatabaseUserJob)
	pkgjobs.RegisterHandler(mux, TypeUninstallDatabaseUser, jobContext, NewUninstallDatabaseUserJob)
	pkgjobs.RegisterHandler(mux, TypeSyncDatabases, jobContext, NewSyncDatabasesJob)
}
