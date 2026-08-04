package jobs

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

type lifecycleCoverageServerRepository struct {
	*patchJobServerRepository
	findErr error
}

func (r *lifecycleCoverageServerRepository) FindByID(
	ctx context.Context,
	id string,
) (*models.Server, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	return r.patchJobServerRepository.FindByID(ctx, id)
}

type lifecycleCoverageServiceRepository struct {
	*patchJobServiceRepository
	findErr          error
	findByTypeErr    error
	updateStatusErr  error
	restoreErr       error
	restoreResult    *bool
	deleteErr        error
	markRemovalError error
}

func (r *lifecycleCoverageServiceRepository) FindByID(
	ctx context.Context,
	id string,
) (*models.InstalledService, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	return r.patchJobServiceRepository.FindByID(ctx, id)
}

func (r *lifecycleCoverageServiceRepository) FindByServerAndType(
	ctx context.Context,
	serverID string,
	serviceType types.ServiceType,
) ([]models.InstalledService, error) {
	if r.findByTypeErr != nil {
		return nil, r.findByTypeErr
	}
	return r.patchJobServiceRepository.FindByServerAndType(ctx, serverID, serviceType)
}

func (r *lifecycleCoverageServiceRepository) UpdateStatus(
	ctx context.Context,
	id string,
	status types.ServiceStatus,
) error {
	if r.updateStatusErr != nil {
		return r.updateStatusErr
	}
	return r.patchJobServiceRepository.UpdateStatus(ctx, id, status)
}

func (r *lifecycleCoverageServiceRepository) RestorePhpPatchStatus(
	ctx context.Context,
	id string,
	status types.ServiceStatus,
) (bool, error) {
	if r.restoreErr != nil {
		return false, r.restoreErr
	}
	if r.restoreResult != nil {
		return *r.restoreResult, nil
	}
	return r.patchJobServiceRepository.RestorePhpPatchStatus(ctx, id, status)
}

func (r *lifecycleCoverageServiceRepository) Delete(ctx context.Context, id string) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	return r.patchJobServiceRepository.Delete(ctx, id)
}

func (r *lifecycleCoverageServiceRepository) MarkRemovalFailed(
	ctx context.Context,
	id string,
) error {
	if r.markRemovalError != nil {
		return r.markRemovalError
	}
	return r.patchJobServiceRepository.MarkRemovalFailed(ctx, id)
}

func wrapLifecycleCoverageRepositories(
	t *testing.T,
	jobDeps *JobDeps,
) (*lifecycleCoverageServerRepository, *lifecycleCoverageServiceRepository) {
	t.Helper()
	registry := jobDeps.Repos.(*patchJobRegistry)
	serverRepository := &lifecycleCoverageServerRepository{
		patchJobServerRepository: registry.serverRepository.(*patchJobServerRepository),
	}
	serviceRepository := &lifecycleCoverageServiceRepository{
		patchJobServiceRepository: registry.serviceRepository.(*patchJobServiceRepository),
	}
	registry.serverRepository = serverRepository
	registry.serviceRepository = serviceRepository
	return serverRepository, serviceRepository
}

func newSetDefaultCoverageJob(t *testing.T) (
	*SetDefaultPhpJob,
	*models.InstalledService,
	*patchJobRegistry,
) {
	t.Helper()
	patchJob, service, _, _, _ := newPatchPhpVersionTestJob(t)
	return &SetDefaultPhpJob{
		Deps: patchJob.Deps,
		Payload: SetDefaultPhpPayload{
			ServerID:       service.ServerID,
			ServiceID:      service.ID,
			Version:        "8.3",
			PreviousStatus: types.ServiceStatusRunning,
		},
	}, service, patchJob.Deps.Repos.(*patchJobRegistry)
}

func TestSetDefaultPhpJobValidationFailures(t *testing.T) {
	t.Run("invalid previous status", func(t *testing.T) {
		job := &SetDefaultPhpJob{Payload: SetDefaultPhpPayload{
			PreviousStatus: types.ServiceStatusPending,
		}}

		err := job.Handle(context.Background())

		require.EqualError(t, err, "default PHP change has an invalid previous service status")
	})

	t.Run("server lookup", func(t *testing.T) {
		job, _, _ := newSetDefaultCoverageJob(t)
		serverRepository, _ := wrapLifecycleCoverageRepositories(t, job.Deps)
		serverRepository.findErr = errors.New("server database unavailable")

		err := job.Handle(context.Background())

		require.ErrorContains(t, err, "failed to find server")
	})

	t.Run("service lookup", func(t *testing.T) {
		job, _, _ := newSetDefaultCoverageJob(t)
		_, serviceRepository := wrapLifecycleCoverageRepositories(t, job.Deps)
		serviceRepository.findErr = errors.New("service database unavailable")

		err := job.Handle(context.Background())

		require.ErrorContains(t, err, "failed to find service")
	})

	t.Run("service ownership", func(t *testing.T) {
		job, service, _ := newSetDefaultCoverageJob(t)
		service.ServerID = "server-b"

		err := job.Handle(context.Background())

		require.ErrorContains(t, err, "does not belong to server")
	})

	t.Run("non PHP service", func(t *testing.T) {
		job, service, _ := newSetDefaultCoverageJob(t)
		service.Type = types.ServiceTypeRedis
		service.Software = types.SoftwareRedis.String()

		err := job.Handle(context.Background())

		require.ErrorContains(t, err, "not a PHP installation")
	})

	t.Run("current default lookup", func(t *testing.T) {
		job, _, _ := newSetDefaultCoverageJob(t)
		_, serviceRepository := wrapLifecycleCoverageRepositories(t, job.Deps)
		serviceRepository.findByTypeErr = errors.New("query failed")

		err := job.Handle(context.Background())

		require.ErrorContains(t, err, "find current default PHP")
	})
}

func TestSetDefaultPhpJobAlreadyDefaultReleasesReservation(t *testing.T) {
	job, service, _ := newSetDefaultCoverageJob(t)
	service.IsDefault = true

	require.NoError(t, job.Handle(context.Background()))
	assert.Equal(t, types.ServiceStatusRunning, service.Status)
}

func TestSetDefaultPhpJobAlreadyDefaultRequiresOwnedReservation(t *testing.T) {
	t.Run("target state changed", func(t *testing.T) {
		job, service, registry := newSetDefaultCoverageJob(t)
		service.IsDefault = true
		require.NoError(t, registry.db.Model(&models.InstalledService{}).
			Where("id = ?", service.ID).
			Update("status", types.ServiceStatusRunning).Error)

		err := job.Handle(context.Background())

		require.ErrorContains(t, err, "target service reservation was lost")
	})

	t.Run("database failure", func(t *testing.T) {
		job, service, registry := newSetDefaultCoverageJob(t)
		service.IsDefault = true
		sqlDB, err := registry.db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())

		err = job.Handle(context.Background())

		require.Error(t, err)
	})
}

func TestSetDefaultPhpJobRemoteFailures(t *testing.T) {
	t.Run("dispatch error", func(t *testing.T) {
		job, _, _ := newSetDefaultCoverageJob(t)
		dispatcher := job.Deps.Dispatcher
		fake := dispatcher.(*taskrunner.FakeDispatcher)
		fake.SetRunError(errors.New("ssh unavailable"))

		err := job.Handle(context.Background())

		require.ErrorContains(t, err, "failed to set default PHP")
		require.ErrorContains(t, err, "ssh unavailable")
	})

	t.Run("command failure", func(t *testing.T) {
		job, _, _ := newSetDefaultCoverageJob(t)
		fake := job.Deps.Dispatcher.(*taskrunner.FakeDispatcher)
		fake.SetDefaultFailure(1, "alternatives failed")

		err := job.Handle(context.Background())

		require.ErrorContains(t, err, "alternatives failed")
	})

	t.Run("tracked task persistence error", func(t *testing.T) {
		job, _, registry := newSetDefaultCoverageJob(t)
		sqlDB, err := registry.db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())

		err = job.Handle(context.Background())

		require.ErrorContains(t, err, "failed to set default PHP")
	})
}

func TestSetDefaultPhpJobPersistenceFailureOutcomes(t *testing.T) {
	t.Run("unknown previous runtime", func(t *testing.T) {
		job, service, registry := newSetDefaultCoverageJob(t)
		require.NoError(t, registry.db.Delete(&models.InstalledService{}, "id = ?", service.ID).Error)

		err := job.Handle(context.Background())

		require.ErrorContains(t, err, "previous default runtime is unknown")
	})

	t.Run("runtime already target version", func(t *testing.T) {
		job, service, registry := newSetDefaultCoverageJob(t)
		job.Deps.Dispatcher.(*taskrunner.FakeDispatcher).DefaultResult.Output =
			"LAUNCH_PREVIOUS_DEFAULT_PHP_VERSION=8.3\n"
		require.NoError(t, registry.db.Delete(&models.InstalledService{}, "id = ?", service.ID).Error)

		err := job.Handle(context.Background())

		require.ErrorContains(t, err, "runtime was already PHP 8.3")
	})

	t.Run("runtime rollback failure", func(t *testing.T) {
		job, service, registry := newSetDefaultCoverageJob(t)
		previous := models.InstalledService{
			BaseModel:    basemodels.BaseModel{ID: "service-php82"},
			ServerScoped: basemodels.ServerScoped{ServerID: service.ServerID},
			Type:         types.ServiceTypePhp,
			Name:         "PHP 8.2",
			Version:      "8.2.20",
			Status:       types.ServiceStatusRunning,
			Software:     types.SoftwarePhp82.String(),
			IsDefault:    true,
		}
		require.NoError(t, registry.db.Create(&previous).Error)
		registry.serviceRepository.(*patchJobServiceRepository).services =
			[]models.InstalledService{previous, *service}
		require.NoError(t, registry.db.Delete(&models.InstalledService{}, "id = ?", service.ID).Error)
		job.Deps.Dispatcher.(*taskrunner.FakeDispatcher).SetFailure(
			"Rollback Default PHP to 8.2",
			1,
			"rollback command failed",
		)

		err := job.Handle(context.Background())

		require.ErrorContains(t, err, "runtime rollback failed")
		require.ErrorContains(t, err, "rollback command failed")
	})
}

func TestSetDefaultPhpHelpersCoverDefensivePaths(t *testing.T) {
	t.Run("persistence requires a database", func(t *testing.T) {
		job := &SetDefaultPhpJob{
			Deps: &JobDeps{Deps: &pkgjobs.Deps{}, Repos: &patchJobRegistry{}},
		}

		err := job.persistDefault(context.Background())

		require.EqualError(t, err, "database is not configured")
	})

	t.Run("reservation must still exist when persisting", func(t *testing.T) {
		job, _, registry := newSetDefaultCoverageJob(t)
		require.NoError(t, registry.db.Model(&models.Server{}).
			Where("id = ?", job.Payload.ServerID).
			Update("pending_default_php_service_id", nil).Error)

		err := job.persistDefault(context.Background())

		require.ErrorContains(t, err, "reservation was lost")
	})

	t.Run("rollback dispatch error", func(t *testing.T) {
		job, _, registry := newSetDefaultCoverageJob(t)
		job.server = registry.serverRepository.(*patchJobServerRepository).server
		job.Deps.Dispatcher.(*taskrunner.FakeDispatcher).SetRunError(errors.New("connection lost"))

		err := job.rollbackDefault(context.Background(), "8.2")

		require.ErrorContains(t, err, "connection lost")
	})

	t.Run("rollback command failure", func(t *testing.T) {
		job, _, registry := newSetDefaultCoverageJob(t)
		job.server = registry.serverRepository.(*patchJobServerRepository).server
		job.Deps.Dispatcher.(*taskrunner.FakeDispatcher).SetDefaultFailure(1, "bad alternatives")

		err := job.rollbackDefault(context.Background(), "8.2")

		require.ErrorContains(t, err, "rollback default PHP: bad alternatives")
	})

	t.Run("release without database", func(t *testing.T) {
		job := &SetDefaultPhpJob{
			Deps: &JobDeps{Deps: &pkgjobs.Deps{}, Repos: &patchJobRegistry{}},
		}

		restored, err := job.releaseReservation(context.Background())

		require.NoError(t, err)
		assert.False(t, restored)
	})

	t.Run("release missing reservation", func(t *testing.T) {
		job, _, registry := newSetDefaultCoverageJob(t)
		require.NoError(t, registry.db.Model(&models.Server{}).
			Where("id = ?", job.Payload.ServerID).
			Update("pending_default_php_service_id", nil).Error)

		restored, err := job.releaseReservation(context.Background())

		require.NoError(t, err)
		assert.False(t, restored)
	})

	t.Run("release inactive previous state", func(t *testing.T) {
		job, _, _ := newSetDefaultCoverageJob(t)
		job.Payload.PreviousStatus = types.ServiceStatusPending

		restored, err := job.releaseReservation(context.Background())

		require.NoError(t, err)
		assert.False(t, restored)
	})
}

func TestSetDefaultPhpFailedCleanupOutcomes(t *testing.T) {
	t.Run("reservation already gone", func(t *testing.T) {
		job, _, registry := newSetDefaultCoverageJob(t)
		require.NoError(t, registry.db.Model(&models.Server{}).
			Where("id = ?", job.Payload.ServerID).
			Update("pending_default_php_service_id", nil).Error)

		job.Failed(context.Background(), errors.New("job failed"))
	})

	t.Run("database failure", func(t *testing.T) {
		job, _, registry := newSetDefaultCoverageJob(t)
		sqlDB, err := registry.db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())

		job.Failed(context.Background(), errors.New("job failed"))
	})
}

func TestSetDefaultPhpJobFactories(t *testing.T) {
	previousDeps := deps
	t.Cleanup(func() { deps = previousDeps })
	deps = &JobDeps{}

	handler := NewSetDefaultPhpJob(SetDefaultPhpPayload{ServerID: "server-a"})
	require.IsType(t, &SetDefaultPhpJob{}, handler)

	task, err := NewSetDefaultPhpTask(
		"server-a",
		"service-php83",
		"8.3",
		types.ServiceStatusRunning,
		nil,
	)
	require.NoError(t, err)
	assert.Equal(t, TypeSetDefaultPhp, task.Type())
	payload, err := pkgjobs.UnmarshalPayload[SetDefaultPhpPayload](task)
	require.NoError(t, err)
	assert.Equal(t, "8.3", payload.Version)
}

func newServiceOperationCoverageJob(t *testing.T, operation string) (
	*ServiceOperationJob,
	*models.InstalledService,
	*patchJobRegistry,
) {
	t.Helper()
	patchJob, service, _, _, _ := newPatchPhpVersionTestJob(t)
	service.Type = types.ServiceTypeRedis
	service.Software = types.SoftwareRedis.String()
	service.Name = "Redis"
	service.Status = types.ServiceStatusRunning
	return &ServiceOperationJob{
		Deps: patchJob.Deps,
		Payload: ServiceOperationPayload{
			ServerID:  service.ServerID,
			ServiceID: service.ID,
			Operation: operation,
		},
	}, service, patchJob.Deps.Repos.(*patchJobRegistry)
}

func TestServiceOperationJobSupportsLifecycleCommands(t *testing.T) {
	for _, operation := range []string{"start", "reload"} {
		t.Run(operation, func(t *testing.T) {
			job, _, _ := newServiceOperationCoverageJob(t, operation)

			require.NoError(t, job.Handle(context.Background()))
			assert.True(t, job.Deps.Broadcaster.(*patchRecordingBroadcaster).
				hasEvent("service.operation", "finished"))
		})
	}

	t.Run("launch agent update", func(t *testing.T) {
		job, service, _ := newServiceOperationCoverageJob(t, "update")
		service.Type = types.ServiceTypeLaunchAgent
		service.Software = types.SoftwareLaunchAgent.String()
		service.Name = "Launch Agent"

		require.NoError(t, job.Handle(context.Background()))
		assert.Equal(t, "Update Launch Agent", job.Deps.Dispatcher.(*taskrunner.FakeDispatcher).
			LastExecution().Task.Name())
	})
}

func TestServiceOperationJobRejectsInvalidTargets(t *testing.T) {
	t.Run("service lookup", func(t *testing.T) {
		job, _, _ := newServiceOperationCoverageJob(t, "start")
		_, serviceRepository := wrapLifecycleCoverageRepositories(t, job.Deps)
		serviceRepository.findErr = errors.New("lookup failed")

		err := job.Handle(context.Background())

		require.ErrorContains(t, err, "failed to find service")
	})

	t.Run("ownership", func(t *testing.T) {
		job, service, _ := newServiceOperationCoverageJob(t, "start")
		service.ServerID = "server-b"

		err := job.Handle(context.Background())

		require.ErrorContains(t, err, "does not belong to server")
	})

	t.Run("non reserved updating service", func(t *testing.T) {
		job, service, _ := newServiceOperationCoverageJob(t, "start")
		service.Status = types.ServiceStatusUpdating

		err := job.Handle(context.Background())

		require.ErrorContains(t, err, "service is being updated")
	})

	t.Run("server lookup", func(t *testing.T) {
		job, _, _ := newServiceOperationCoverageJob(t, "start")
		serverRepository, _ := wrapLifecycleCoverageRepositories(t, job.Deps)
		serverRepository.findErr = errors.New("lookup failed")

		err := job.Handle(context.Background())

		require.ErrorContains(t, err, "failed to find server")
	})

	t.Run("unsupported update", func(t *testing.T) {
		job, _, _ := newServiceOperationCoverageJob(t, "update")

		err := job.Handle(context.Background())

		require.ErrorContains(t, err, "only supported for the launch agent")
	})

	t.Run("unknown operation", func(t *testing.T) {
		job, _, _ := newServiceOperationCoverageJob(t, "dance")

		err := job.Handle(context.Background())

		require.EqualError(t, err, "unknown operation: dance")
	})
}

func TestServiceOperationJobReportsExecutionAndPersistenceFailures(t *testing.T) {
	t.Run("command failure", func(t *testing.T) {
		job, _, _ := newServiceOperationCoverageJob(t, "start")
		job.Deps.Dispatcher.(*taskrunner.FakeDispatcher).SetDefaultFailure(1, "systemd failed")

		err := job.Handle(context.Background())

		require.ErrorContains(t, err, "systemd failed")
		assert.True(t, job.Deps.Broadcaster.(*patchRecordingBroadcaster).
			hasEvent("service.operation", "failed"))
	})

	t.Run("reserved PHP status persistence", func(t *testing.T) {
		patchJob, service, _, _, _ := newPatchPhpVersionTestJob(t)
		_, serviceRepository := wrapLifecycleCoverageRepositories(t, patchJob.Deps)
		serviceRepository.updateStatusErr = errors.New("write failed")
		job := &ServiceOperationJob{
			Deps: patchJob.Deps,
			Payload: ServiceOperationPayload{
				ServerID:       service.ServerID,
				ServiceID:      service.ID,
				Operation:      "stop",
				PreviousStatus: types.ServiceStatusRunning,
			},
		}

		err := job.Handle(context.Background())

		require.ErrorContains(t, err, "persist PHP service operation status")
		require.NotNil(t, job.remoteStatus)
		job.Failed(context.Background(), err)
	})

	t.Run("tracked task persistence", func(t *testing.T) {
		job, _, registry := newServiceOperationCoverageJob(t, "start")
		sqlDB, err := registry.db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())

		err = job.Handle(context.Background())

		require.ErrorContains(t, err, "failed to start service")
	})
}

func TestServiceOperationFailedCleanupBranches(t *testing.T) {
	t.Run("persists known remote state", func(t *testing.T) {
		patchJob, service, _, _, _ := newPatchPhpVersionTestJob(t)
		status := types.ServiceStatusStopped
		job := &ServiceOperationJob{
			Deps:         patchJob.Deps,
			Payload:      ServiceOperationPayload{ServiceID: service.ID, PreviousStatus: types.ServiceStatusRunning},
			remoteStatus: &status,
		}

		job.Failed(context.Background(), errors.New("database retry"))

		assert.Equal(t, types.ServiceStatusStopped, service.Status)
	})

	t.Run("restore error", func(t *testing.T) {
		patchJob, service, _, _, _ := newPatchPhpVersionTestJob(t)
		_, serviceRepository := wrapLifecycleCoverageRepositories(t, patchJob.Deps)
		serviceRepository.restoreErr = errors.New("restore failed")
		job := &ServiceOperationJob{
			Deps: patchJob.Deps,
			Payload: ServiceOperationPayload{
				ServerID:       service.ServerID,
				ServiceID:      service.ID,
				PreviousStatus: types.ServiceStatusRunning,
			},
		}

		job.Failed(context.Background(), errors.New("operation failed"))
	})

	t.Run("restore broadcasts status", func(t *testing.T) {
		patchJob, service, _, _, broadcaster := newPatchPhpVersionTestJob(t)
		server := patchJob.Deps.Repos.Server().(*patchJobServerRepository).server
		job := &ServiceOperationJob{
			Deps:   patchJob.Deps,
			server: server,
			Payload: ServiceOperationPayload{
				ServerID:       service.ServerID,
				ServiceID:      service.ID,
				PreviousStatus: types.ServiceStatusRunning,
			},
		}

		job.Failed(context.Background(), errors.New("operation failed"))

		assert.True(t, broadcaster.hasEvent("service.status_changed", "running"))
	})

	t.Run("broadcast failure ignores incomplete job", func(t *testing.T) {
		job := &ServiceOperationJob{}
		job.broadcastFailure("task-a", "failed")
	})
}

func TestServiceOperationJobFactory(t *testing.T) {
	previousDeps := deps
	t.Cleanup(func() { deps = previousDeps })
	deps = &JobDeps{}

	handler := NewServiceOperationJob(ServiceOperationPayload{Operation: "restart"})
	require.IsType(t, &ServiceOperationJob{}, handler)
}

func newRemoveServiceCoverageJob(t *testing.T) (
	*RemoveServiceJob,
	*models.InstalledService,
	*patchJobRegistry,
) {
	t.Helper()
	patchJob, service, _, _, _ := newPatchPhpVersionTestJob(t)
	service.Type = types.ServiceTypeRedis
	service.Software = types.SoftwareRedis.String()
	service.Name = "Redis"
	service.Status = types.ServiceStatusRunning
	return &RemoveServiceJob{
		Deps: patchJob.Deps,
		Payload: RemoveServicePayload{
			ServerID:  service.ServerID,
			ServiceID: service.ID,
		},
	}, service, patchJob.Deps.Repos.(*patchJobRegistry)
}

func TestRemoveServiceJobSuccess(t *testing.T) {
	job, service, registry := newRemoveServiceCoverageJob(t)
	serviceRepository := registry.serviceRepository.(*patchJobServiceRepository)

	require.NoError(t, job.Handle(context.Background()))

	assert.True(t, job.remoteRemoved)
	assert.Equal(t, service.ID, serviceRepository.deletedServiceID)
	assert.True(t, job.Deps.Broadcaster.(*patchRecordingBroadcaster).
		hasEvent("service.status_changed", string(types.ServiceStatusUninstalling)))
}

func TestRemoveServiceJobValidationFailures(t *testing.T) {
	t.Run("service lookup", func(t *testing.T) {
		job, _, _ := newRemoveServiceCoverageJob(t)
		_, serviceRepository := wrapLifecycleCoverageRepositories(t, job.Deps)
		serviceRepository.findErr = errors.New("lookup failed")

		err := job.Handle(context.Background())

		require.ErrorContains(t, err, "failed to find service")
	})

	t.Run("ownership", func(t *testing.T) {
		job, service, _ := newRemoveServiceCoverageJob(t)
		service.ServerID = "server-b"

		err := job.Handle(context.Background())

		require.ErrorContains(t, err, "does not belong to server")
	})

	t.Run("lost PHP reservation", func(t *testing.T) {
		patchJob, service, _, _, _ := newPatchPhpVersionTestJob(t)
		service.Status = types.ServiceStatusRunning
		job := &RemoveServiceJob{
			Deps: patchJob.Deps,
			Payload: RemoveServicePayload{
				ServerID:       service.ServerID,
				ServiceID:      service.ID,
				PreviousStatus: types.ServiceStatusRunning,
			},
		}

		err := job.Handle(context.Background())

		require.ErrorContains(t, err, "reservation was lost")
	})

	t.Run("non reserved updating service", func(t *testing.T) {
		job, service, _ := newRemoveServiceCoverageJob(t)
		service.Status = types.ServiceStatusUpdating

		err := job.Handle(context.Background())

		require.ErrorContains(t, err, "service is being updated")
	})

	t.Run("server lookup", func(t *testing.T) {
		job, _, _ := newRemoveServiceCoverageJob(t)
		serverRepository, _ := wrapLifecycleCoverageRepositories(t, job.Deps)
		serverRepository.findErr = errors.New("lookup failed")

		err := job.Handle(context.Background())

		require.ErrorContains(t, err, "failed to find server")
	})

	t.Run("status update", func(t *testing.T) {
		job, _, _ := newRemoveServiceCoverageJob(t)
		_, serviceRepository := wrapLifecycleCoverageRepositories(t, job.Deps)
		serviceRepository.updateStatusErr = errors.New("write failed")

		err := job.Handle(context.Background())

		require.ErrorContains(t, err, "failed to update service status")
	})
}

func TestRemoveServiceJobRemoteAndPersistenceFailures(t *testing.T) {
	t.Run("dispatch error", func(t *testing.T) {
		job, _, _ := newRemoveServiceCoverageJob(t)
		job.Deps.Dispatcher.(*taskrunner.FakeDispatcher).SetRunError(errors.New("ssh failed"))

		err := job.Handle(context.Background())

		require.ErrorContains(t, err, "failed to remove service")
	})

	t.Run("command failure", func(t *testing.T) {
		job, _, _ := newRemoveServiceCoverageJob(t)
		job.Deps.Dispatcher.(*taskrunner.FakeDispatcher).SetDefaultFailure(1, "apt failed")

		err := job.Handle(context.Background())

		require.ErrorContains(t, err, "apt failed")
	})

	t.Run("delete failure", func(t *testing.T) {
		job, _, _ := newRemoveServiceCoverageJob(t)
		_, serviceRepository := wrapLifecycleCoverageRepositories(t, job.Deps)
		serviceRepository.deleteErr = errors.New("delete failed")

		err := job.Handle(context.Background())

		require.ErrorContains(t, err, "failed to delete service record")
		assert.True(t, job.remoteRemoved)
		job.Failed(context.Background(), err)
		assert.True(t, serviceRepository.removalFailed)
	})

	t.Run("tracked task persistence", func(t *testing.T) {
		job, _, registry := newRemoveServiceCoverageJob(t)
		sqlDB, err := registry.db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())

		err = job.Handle(context.Background())

		require.ErrorContains(t, err, "failed to remove service")
	})
}

func TestRemoveServiceFailedCleanupBranches(t *testing.T) {
	t.Run("remote removal finishes local deletion", func(t *testing.T) {
		job, service, registry := newRemoveServiceCoverageJob(t)
		job.remoteRemoved = true

		job.Failed(context.Background(), errors.New("database retry"))

		assert.Equal(t, service.ID,
			registry.serviceRepository.(*patchJobServiceRepository).deletedServiceID)
	})

	t.Run("restore error falls back to failed state", func(t *testing.T) {
		patchJob, service, _, _, _ := newPatchPhpVersionTestJob(t)
		_, serviceRepository := wrapLifecycleCoverageRepositories(t, patchJob.Deps)
		serviceRepository.restoreErr = errors.New("restore failed")
		job := &RemoveServiceJob{
			Deps: patchJob.Deps,
			Payload: RemoveServicePayload{
				ServerID:       service.ServerID,
				ServiceID:      service.ID,
				PreviousStatus: types.ServiceStatusRunning,
			},
		}

		job.Failed(context.Background(), errors.New("remove failed"))

		assert.True(t, serviceRepository.removalFailed)
	})

	t.Run("failed marker error", func(t *testing.T) {
		job, _, _ := newRemoveServiceCoverageJob(t)
		_, serviceRepository := wrapLifecycleCoverageRepositories(t, job.Deps)
		serviceRepository.markRemovalError = errors.New("mark failed")

		job.Failed(context.Background(), errors.New("remove failed"))
	})
}

func TestRemoveServiceJobFactory(t *testing.T) {
	previousDeps := deps
	t.Cleanup(func() { deps = previousDeps })
	deps = &JobDeps{}

	handler := NewRemoveServiceJob(RemoveServicePayload{ServiceID: "service-a"})
	require.IsType(t, &RemoveServiceJob{}, handler)
}
