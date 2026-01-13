package jobs

import (
	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

func getContext() any {
	return jobContext
}

// Register registers all database jobs with the registry.
func Register(r *jobs.Registry) {
	r.Register(TypeInstallDatabase, jobs.MakeFactory(
		func(p InstallDatabasePayload) *InstallDatabaseJob { return &InstallDatabaseJob{Payload: p} },
		getContext,
	))
	r.Register(TypeUninstallDatabase, jobs.MakeFactory(
		func(p UninstallDatabasePayload) *UninstallDatabaseJob { return &UninstallDatabaseJob{Payload: p} },
		getContext,
	))
	r.Register(TypeInstallDatabaseUser, jobs.MakeFactory(
		func(p InstallDatabaseUserPayload) *InstallDatabaseUserJob { return &InstallDatabaseUserJob{Payload: p} },
		getContext,
	))
	r.Register(TypeUpdateDatabaseUser, jobs.MakeFactory(
		func(p UpdateDatabaseUserPayload) *UpdateDatabaseUserJob { return &UpdateDatabaseUserJob{Payload: p} },
		getContext,
	))
	r.Register(TypeUninstallDatabaseUser, jobs.MakeFactory(
		func(p UninstallDatabaseUserPayload) *UninstallDatabaseUserJob { return &UninstallDatabaseUserJob{Payload: p} },
		getContext,
	))
	r.Register(TypeSyncDatabases, jobs.MakeFactory(
		func(p SyncDatabasesPayload) *SyncDatabasesJob { return &SyncDatabasesJob{Payload: p} },
		getContext,
	))
}
