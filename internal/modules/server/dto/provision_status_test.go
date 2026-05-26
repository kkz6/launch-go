package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
)

func ptrString(s string) *string { return &s }

// countCurrent returns how many steps are marked "current" in a status response.
// At any given moment exactly one step should be "current" (or zero if failed).
func countCurrent(resp ProvisionStatusResponse) int {
	n := 0
	for _, s := range resp.Steps {
		if s.Status == "current" {
			n++
		}
	}
	return n
}

// TestBuildProvisionStatus_NoDoubleSpinner_WhenConnectingAndServiceInstalling
// pins the UX rule that the timeline shows exactly one spinner at a time.
//
// The bug: a docker server with status=starting (still waiting to connect)
// and a service row with status=installing rendered TWO concurrent spinners —
// "Waiting for server to connect" and "Installing Docker". Service current-state
// must be gated on connecting being completed.
func TestBuildProvisionStatus_NoDoubleSpinner_WhenConnectingAndServiceInstalling(t *testing.T) {
	server := &models.Server{
		Status: types.ServerStatusStarting,
		Type:   ptrString(string(types.ServerTypeDocker)),
		Services: []models.InstalledService{
			{Name: "docker", Status: types.ServiceStatusInstalling},
		},
	}

	resp := BuildProvisionStatus(server, nil)

	assert.Equal(t, 1, countCurrent(resp),
		"exactly one step must be current at a time, got %d", countCurrent(resp))
	assert.NotNil(t, resp.CurrentStep)
	assert.Equal(t, "connecting_server", resp.CurrentStep.Name,
		"while server.Status=starting and no task has run yet, the connecting step owns the spinner")
}

// TestBuildProvisionStatus_FailedServer_NoSpinner pins that a failed server
// shows zero spinners — the UI renders a failed banner instead.
func TestBuildProvisionStatus_FailedServer_NoSpinner(t *testing.T) {
	server := &models.Server{
		Status: types.ServerStatusFailed,
		Type:   ptrString(string(types.ServerTypeDocker)),
		Services: []models.InstalledService{
			{Name: "docker", Status: types.ServiceStatusInstalling},
		},
	}

	resp := BuildProvisionStatus(server, nil)

	assert.Equal(t, 0, countCurrent(resp), "failed server must have no current step")
	assert.Nil(t, resp.CurrentStep)
	assert.True(t, resp.Failed)
}

// TestBuildProvisionStatus_ServiceCurrentOnlyAfterConnectingDone confirms
// that service rows can be marked current only once connecting has finished.
func TestBuildProvisionStatus_ServiceCurrentOnlyAfterConnectingDone(t *testing.T) {
	// A task exists → connecting is "completed"; all base steps completed
	// (so the provisionSteps loop finds no current); a service is installing.
	allBaseSteps := types.ForDockerServer()
	completed := make([]string, 0, len(allBaseSteps))
	for _, s := range allBaseSteps {
		completed = append(completed, s.String())
	}

	server := &models.Server{
		Status:                  types.ServerStatusProvisioning,
		Type:                    ptrString(string(types.ServerTypeDocker)),
		CompletedProvisionSteps: completed,
		Services: []models.InstalledService{
			{Name: "docker", Status: types.ServiceStatusInstalling},
			{Name: "traefik", Status: types.ServiceStatusPending},
		},
	}

	resp := BuildProvisionStatus(server, &models.Task{})

	assert.Equal(t, 1, countCurrent(resp),
		"only one step current; second pending service must remain pending")
	assert.NotNil(t, resp.CurrentStep)
	assert.Equal(t, "docker", resp.CurrentStep.Name)
}
