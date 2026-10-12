package jobs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
)

func TestServiceInstallationRetainsTaskLogs(t *testing.T) {
	for _, fails := range []bool{false, true} {
		name := "success"
		if fails {
			name = "failure"
		}
		t.Run(name, func(t *testing.T) {
			fixture, service, db, dispatcher, _ := newPatchPhpVersionTestJob(t)
			service.Status = types.ServiceStatusPending
			service.Software = "bun"
			if fails {
				dispatcher.SetDefaultFailure(1, "installation failed")
			}
			job := &AddServiceJob{Deps: fixture.Deps, Payload: AddServicePayload{ServerID: service.ServerID, ServiceID: service.ID, Software: "bun"}}
			err := job.Handle(context.Background())
			if fails {
				require.Error(t, err)
				job.Failed(context.Background(), err)
				require.Equal(t, types.ServiceStatusFailed, service.Status)
			} else {
				require.NoError(t, err)
				require.Equal(t, types.ServiceStatusRunning, service.Status)
			}
			response := dto.ToServiceResponse(service)
			require.NotNil(t, response.InstallTaskID)
			var tracked models.Task
			require.NoError(t, db.First(&tracked, "id = ?", *response.InstallTaskID).Error)
			require.Equal(t, service.ServerID, tracked.ServerID)
		})
	}
}
