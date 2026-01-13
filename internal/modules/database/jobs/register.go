package jobs

import (
	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

// Register registers all database job handlers with the job registry.
// This uses the Handler interface pattern (same as server module).
func Register(r *jobs.Registry) {
	r.Register(TypeInstallDatabase, newInstallDatabaseJob)
	r.Register(TypeUninstallDatabase, newUninstallDatabaseJob)
	r.Register(TypeInstallDatabaseUser, newInstallDatabaseUserJob)
	r.Register(TypeUpdateDatabaseUser, newUpdateDatabaseUserJob)
	r.Register(TypeUninstallDatabaseUser, newUninstallDatabaseUserJob)
	r.Register(TypeSyncDatabases, newSyncDatabasesJob)
}

// Factory functions for creating job handlers

func newInstallDatabaseJob(t *asynq.Task) (jobs.Handler, error) {
	payload, err := jobs.ParsePayload[InstallDatabasePayload](t)
	if err != nil {
		return nil, err
	}
	job := &InstallDatabaseJob{Payload: payload}
	job.SetContext(jobContext)
	return job, nil
}

func newUninstallDatabaseJob(t *asynq.Task) (jobs.Handler, error) {
	payload, err := jobs.ParsePayload[UninstallDatabasePayload](t)
	if err != nil {
		return nil, err
	}
	job := &UninstallDatabaseJob{Payload: payload}
	job.SetContext(jobContext)
	return job, nil
}

func newInstallDatabaseUserJob(t *asynq.Task) (jobs.Handler, error) {
	payload, err := jobs.ParsePayload[InstallDatabaseUserPayload](t)
	if err != nil {
		return nil, err
	}
	job := &InstallDatabaseUserJob{Payload: payload}
	job.SetContext(jobContext)
	return job, nil
}

func newUpdateDatabaseUserJob(t *asynq.Task) (jobs.Handler, error) {
	payload, err := jobs.ParsePayload[UpdateDatabaseUserPayload](t)
	if err != nil {
		return nil, err
	}
	job := &UpdateDatabaseUserJob{Payload: payload}
	job.SetContext(jobContext)
	return job, nil
}

func newUninstallDatabaseUserJob(t *asynq.Task) (jobs.Handler, error) {
	payload, err := jobs.ParsePayload[UninstallDatabaseUserPayload](t)
	if err != nil {
		return nil, err
	}
	job := &UninstallDatabaseUserJob{Payload: payload}
	job.SetContext(jobContext)
	return job, nil
}

func newSyncDatabasesJob(t *asynq.Task) (jobs.Handler, error) {
	payload, err := jobs.ParsePayload[SyncDatabasesPayload](t)
	if err != nil {
		return nil, err
	}
	job := &SyncDatabasesJob{Payload: payload}
	job.SetContext(jobContext)
	return job, nil
}
