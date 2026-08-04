package jobs

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/server/types"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

func TestServiceOperationJobConsumesPHPReservation(t *testing.T) {
	patchJob, service, _, _, _ := newPatchPhpVersionTestJob(t)
	job := &ServiceOperationJob{
		Deps: patchJob.Deps,
		Payload: ServiceOperationPayload{
			ServerID:       service.ServerID,
			ServiceID:      service.ID,
			Operation:      "stop",
			PreviousStatus: types.ServiceStatusRunning,
		},
	}

	require.NoError(t, job.Handle(context.Background()))
	assert.Equal(t, types.ServiceStatusStopped, service.Status)
}

func TestServiceOperationJobRestoresPHPReservationOnFailure(t *testing.T) {
	patchJob, service, _, dispatcher, _ := newPatchPhpVersionTestJob(t)
	dispatcher.SetRunError(errors.New("ssh unavailable"))
	job := &ServiceOperationJob{
		Deps: patchJob.Deps,
		Payload: ServiceOperationPayload{
			ServerID:       service.ServerID,
			ServiceID:      service.ID,
			Operation:      "restart",
			PreviousStatus: types.ServiceStatusRunning,
		},
	}

	require.Error(t, job.Handle(context.Background()))
	job.Failed(context.Background(), errors.New("ssh unavailable"))

	assert.Equal(t, types.ServiceStatusRunning, service.Status)
}

func TestServiceOperationJobRejectsLostPHPReservation(t *testing.T) {
	patchJob, service, _, dispatcher, _ := newPatchPhpVersionTestJob(t)
	service.Status = types.ServiceStatusRunning
	job := &ServiceOperationJob{
		Deps: patchJob.Deps,
		Payload: ServiceOperationPayload{
			ServerID:       service.ServerID,
			ServiceID:      service.ID,
			Operation:      "restart",
			PreviousStatus: types.ServiceStatusRunning,
		},
	}

	err := job.Handle(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "reservation was lost")
	assert.Zero(t, dispatcher.ExecutionCount())
}

func TestRemoveServiceJobConsumesAndRestoresPHPReservation(t *testing.T) {
	patchJob, service, _, dispatcher, _ := newPatchPhpVersionTestJob(t)
	dispatcher.SetDefaultFailure(1, "remove failed")
	serviceRepo := patchJob.Deps.Repos.Service().(*patchJobServiceRepository)
	job := &RemoveServiceJob{
		Deps: patchJob.Deps,
		Payload: RemoveServicePayload{
			ServerID:       service.ServerID,
			ServiceID:      service.ID,
			PreviousStatus: types.ServiceStatusRunning,
		},
	}

	require.Error(t, job.Handle(context.Background()))
	assert.Equal(t, types.ServiceStatusUpdating, service.Status)
	job.Failed(context.Background(), errors.New("remove failed"))

	assert.Equal(t, types.ServiceStatusRunning, service.Status)
	assert.False(t, serviceRepo.removalFailed)
	assert.Empty(t, serviceRepo.deletedServiceID)
}

func TestLifecycleTaskPayloadsCarryPreviousStatus(t *testing.T) {
	operationTask, err := NewServiceOperationTask(
		"server-a",
		"service-a",
		"restart",
		types.ServiceStatusRunning,
		nil,
	)
	require.NoError(t, err)
	operationPayload, err := pkgjobs.UnmarshalPayload[ServiceOperationPayload](operationTask)
	require.NoError(t, err)
	assert.Equal(t, types.ServiceStatusRunning, operationPayload.PreviousStatus)

	removeTask, err := NewRemoveServiceTask(
		"server-a",
		"service-a",
		types.ServiceStatusInstalled,
		nil,
	)
	require.NoError(t, err)
	removePayload, err := pkgjobs.UnmarshalPayload[RemoveServicePayload](removeTask)
	require.NoError(t, err)
	assert.Equal(t, types.ServiceStatusInstalled, removePayload.PreviousStatus)
}
