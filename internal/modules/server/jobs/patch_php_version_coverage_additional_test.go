package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

type patchPHPBoundaryServerRepository struct {
	*patchJobServerRepository
	findErr error
}

func (r *patchPHPBoundaryServerRepository) FindByID(
	ctx context.Context,
	id string,
) (*models.Server, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	return r.patchJobServerRepository.FindByID(ctx, id)
}

type patchPHPBoundaryServiceRepository struct {
	*patchJobServiceRepository
	findErr            error
	runningStateErr    error
	finishedStateErr   error
	taskAssociationErr error
}

func (r *patchPHPBoundaryServiceRepository) FindByID(
	ctx context.Context,
	id string,
) (*models.InstalledService, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	return r.patchJobServiceRepository.FindByID(ctx, id)
}

func (r *patchPHPBoundaryServiceRepository) UpdateFields(
	ctx context.Context,
	id string,
	fields map[string]any,
) error {
	if _, exists := fields["task_id"]; exists && r.taskAssociationErr != nil {
		return r.taskAssociationErr
	}
	if typeData, ok := fields["type_data"].(dbtype.JSONMap); ok {
		switch typeData["patch_status"] {
		case "running":
			if r.runningStateErr != nil {
				return r.runningStateErr
			}
		case "finished":
			if r.finishedStateErr != nil {
				return r.finishedStateErr
			}
		}
	}
	return r.patchJobServiceRepository.UpdateFields(ctx, id, fields)
}

func wrapPatchPHPBoundaryRepositories(
	t *testing.T,
	job *PatchPhpVersionJob,
) (*patchPHPBoundaryServerRepository, *patchPHPBoundaryServiceRepository) {
	t.Helper()
	registry := job.Deps.Repos.(*patchJobRegistry)
	serverRepository := &patchPHPBoundaryServerRepository{
		patchJobServerRepository: registry.serverRepository.(*patchJobServerRepository),
	}
	serviceRepository := &patchPHPBoundaryServiceRepository{
		patchJobServiceRepository: registry.serviceRepository.(*patchJobServiceRepository),
	}
	registry.serverRepository = serverRepository
	registry.serviceRepository = serviceRepository
	return serverRepository, serviceRepository
}

func TestPatchPhpVersionJobFactoryAndTimeout(t *testing.T) {
	previousDeps := deps
	t.Cleanup(func() { deps = previousDeps })
	deps = &JobDeps{}

	handler := NewPatchPhpVersionJob(PatchPhpVersionPayload{
		ServerID:  "server-a",
		ServiceID: "service-php83",
	})
	job := handler.(*PatchPhpVersionJob)

	assert.Same(t, deps, job.Deps)
	assert.Equal(t, 20*time.Minute, job.Timeout())
}

func TestPatchPhpVersionJobValidationBoundaries(t *testing.T) {
	t.Run("service lookup", func(t *testing.T) {
		job, _, _, _, _ := newPatchPhpVersionTestJob(t)
		_, serviceRepository := wrapPatchPHPBoundaryRepositories(t, job)
		serviceRepository.findErr = errors.New("service lookup failed")

		err := job.Handle(context.Background())

		require.ErrorContains(t, err, "find PHP service")
		assert.False(t, job.validated)
	})

	t.Run("non PHP service type", func(t *testing.T) {
		job, service, _, _, _ := newPatchPhpVersionTestJob(t)
		service.Type = types.ServiceTypeRedis

		err := job.Handle(context.Background())

		require.ErrorContains(t, err, "not a PHP installation")
		assert.False(t, job.validated)
	})

	t.Run("non PHP software", func(t *testing.T) {
		job, service, _, _, _ := newPatchPhpVersionTestJob(t)
		service.Software = types.SoftwareRedis.String()

		err := job.Handle(context.Background())

		require.ErrorContains(t, err, "not a PHP installation")
	})

	t.Run("reservation missing", func(t *testing.T) {
		job, service, _, _, _ := newPatchPhpVersionTestJob(t)
		service.Status = types.ServiceStatusRunning

		err := job.Handle(context.Background())

		require.ErrorContains(t, err, "PHP patch was not reserved")
	})

	t.Run("invalid previous status", func(t *testing.T) {
		job, _, _, _, _ := newPatchPhpVersionTestJob(t)
		job.Payload.PreviousStatus = types.ServiceStatusPending

		err := job.Handle(context.Background())

		require.ErrorContains(t, err, "invalid previous service status")
		assert.False(t, job.validated)
	})
}

func TestPatchPhpVersionJobRestoresReservationWhenServerLookupFails(t *testing.T) {
	job, service, _, _, broadcaster := newPatchPhpVersionTestJob(t)
	serverRepository, _ := wrapPatchPHPBoundaryRepositories(t, job)
	serverRepository.findErr = errors.New("server lookup failed")

	err := job.Handle(context.Background())

	require.ErrorContains(t, err, "find server")
	assert.True(t, job.validated)
	assert.True(t, job.failureRecorded)
	assert.Equal(t, types.ServiceStatusRunning, service.Status)
	assert.Equal(t, "failed", service.TypeData["patch_status"])
	assert.Contains(t, service.TypeData["patch_error"], "server lookup failed")
	assert.False(t, broadcaster.hasEvent("php.patch", "failed"))
}

func TestPatchPhpVersionJobInitialStateFailureCanBeReleased(t *testing.T) {
	job, service, _, _, _ := newPatchPhpVersionTestJob(t)
	_, serviceRepository := wrapPatchPHPBoundaryRepositories(t, job)
	serviceRepository.runningStateErr = errors.New("state write failed")

	err := job.Handle(context.Background())

	require.ErrorContains(t, err, "mark PHP service updating")
	assert.True(t, job.validated)
	assert.False(t, job.failureRecorded)
	serviceRepository.runningStateErr = nil
	job.Failed(context.Background(), err)
	assert.True(t, job.failureRecorded)
	assert.Equal(t, types.ServiceStatusRunning, service.Status)
}

func TestPatchPhpVersionJobTracksTaskEvenWhenAssociationWriteFails(t *testing.T) {
	job, service, _, dispatcher, broadcaster := newPatchPhpVersionTestJob(t)
	_, serviceRepository := wrapPatchPHPBoundaryRepositories(t, job)
	serviceRepository.taskAssociationErr = errors.New("task association failed")
	dispatcher.DefaultResult.Output = "LAUNCH_PHP_PATCH_VERSION=8.3.12\n"

	require.NoError(t, job.Handle(context.Background()))

	assert.NotEmpty(t, job.taskID)
	require.NotNil(t, service.TaskID)
	assert.Equal(t, job.taskID, *service.TaskID)
	assert.True(t, broadcaster.hasEvent("php.patch", "running"))
	assert.True(t, broadcaster.hasEvent("php.patch", "finished"))
}

func TestPatchPhpVersionJobDispatchPersistenceFailure(t *testing.T) {
	job, service, db, _, broadcaster := newPatchPhpVersionTestJob(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	err = job.Handle(context.Background())

	require.ErrorContains(t, err, "run PHP patch")
	assert.True(t, job.failureRecorded)
	assert.Equal(t, types.ServiceStatusRunning, service.Status)
	assert.True(t, broadcaster.hasEvent("php.patch", "failed"))
}

func TestPatchPhpVersionJobRejectsUnexpectedDetectedVersion(t *testing.T) {
	job, service, _, dispatcher, broadcaster := newPatchPhpVersionTestJob(t)
	dispatcher.DefaultResult.Output = "LAUNCH_PHP_PATCH_VERSION=8.2.25\n"

	err := job.Handle(context.Background())

	require.ErrorContains(t, err, "expected 8.3.x")
	require.ErrorContains(t, err, `detected "8.2.25"`)
	assert.Equal(t, types.ServiceStatusRunning, service.Status)
	assert.Equal(t, "failed", service.TypeData["patch_status"])
	assert.True(t, broadcaster.hasEvent("php.patch", "failed"))
}

func TestPatchPhpVersionJobRestoresStateWhenFinalPersistenceFails(t *testing.T) {
	job, service, _, dispatcher, broadcaster := newPatchPhpVersionTestJob(t)
	_, serviceRepository := wrapPatchPHPBoundaryRepositories(t, job)
	serviceRepository.finishedStateErr = errors.New("final write failed")
	dispatcher.DefaultResult.Output = "LAUNCH_PHP_PATCH_VERSION=8.3.13\n"

	err := job.Handle(context.Background())

	require.ErrorContains(t, err, "persist patched PHP version")
	assert.True(t, job.failureRecorded)
	assert.Equal(t, types.ServiceStatusRunning, service.Status)
	assert.Equal(t, "8.3.6", service.Version)
	assert.Equal(t, "failed", service.TypeData["patch_status"])
	assert.Contains(t, service.TypeData["patch_error"], "final write failed")
	assert.True(t, broadcaster.hasEvent("php.patch", "failed"))
}

func TestPatchPhpVersionFailureAndBroadcastNilBoundaries(t *testing.T) {
	job, _, _, _, broadcaster := newPatchPhpVersionTestJob(t)

	job.broadcastStatus(types.ServiceStatusRunning)
	job.broadcastPatch("failed", "message", "")
	assert.Empty(t, broadcaster.events)

	patchErr := errors.New("validation failed")
	assert.ErrorIs(t, job.recordFailure(context.Background(), patchErr), patchErr)
	assert.False(t, job.failureRecorded)

	job.Failed(context.Background(), patchErr)
	assert.False(t, job.failureRecorded)
}

func TestPatchPhpVersionHelpersHandleEmptyDataAndLimits(t *testing.T) {
	assert.Empty(t, cloneTypeData(nil))
	assert.Equal(t, "message", boundedPatchError("message", 0))
	assert.True(t, isVersionInSeries("8.3", "8.3"))
	assert.False(t, isVersionInSeries("8.30.1", "8.3"))
}

func TestPatchPhpVersionTaskIncludesUserAndTimeout(t *testing.T) {
	userID := "user-a"
	task, err := NewPatchPhpVersionTask(
		"server-a",
		"service-php83",
		types.ServiceStatusInstalled,
		&userID,
	)
	require.NoError(t, err)
	require.NotNil(t, task)

	payload, err := pkgjobs.UnmarshalPayload[PatchPhpVersionPayload](task)
	require.NoError(t, err)
	assert.Equal(t, "server-a", payload.ServerID)
	assert.Equal(t, "service-php83", payload.ServiceID)
	assert.Equal(t, types.ServiceStatusInstalled, payload.PreviousStatus)
	require.NotNil(t, payload.UserID)
	assert.Equal(t, userID, *payload.UserID)
	assert.Equal(t, TypePatchPhpVersion, task.Type())
}
