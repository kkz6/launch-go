package tasks

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner/markers"
)

// =============================================================================
// Swap sizing
// =============================================================================

// Smaller boxes get a lower swappiness so the kernel leans on RAM before
// paging; big boxes can afford the default-ish value.
func TestCalculateSwappiness(t *testing.T) {
	tests := []struct {
		memoryInMB int
		want       int
	}{
		{memoryInMB: 512, want: 20},
		{memoryInMB: 1024, want: 20},
		{memoryInMB: 2048, want: 35},
		{memoryInMB: 4096, want: 50},
		{memoryInMB: 8192, want: 60},
		{memoryInMB: 32768, want: 60},
	}

	for _, tc := range tests {
		assert.Equal(t, tc.want, calculateSwappiness(tc.memoryInMB), "memory %dMB", tc.memoryInMB)
	}
}

// =============================================================================
// buildProvisionStepData
// =============================================================================

// Each step gets only the fields its template references. Steps whose
// templates take no data get nil, which text/template accepts.
func TestBuildProvisionStepDataPerStep(t *testing.T) {
	config := testConfig()

	t.Run("steps carrying data", func(t *testing.T) {
		for _, step := range []types.ProvisionStep{
			types.ProvisionStepConfigureSwap,
			types.ProvisionStepConfigureFirewall,
			types.ProvisionStepSetupRoot,
			types.ProvisionStepSetupDefaultUser,
		} {
			assert.NotNil(t, buildProvisionStepData(step, config, 2048, 35), "step %s", step)
		}
	})

	t.Run("steps needing no data", func(t *testing.T) {
		for _, step := range []types.ProvisionStep{
			types.ProvisionStepDetectOS,
			types.ProvisionStepSSHSecurity,
		} {
			assert.Nil(t, buildProvisionStepData(step, config, 2048, 35), "step %s", step)
		}
	})
}

// =============================================================================
// Script assembly across configurations
// =============================================================================

func TestProvisionFreshServerAcrossConfigurations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ProvisionFreshServerConfig)
	}{
		{name: "defaults", mutate: func(*ProvisionFreshServerConfig) {}},
		{
			name: "with ssh keys",
			mutate: func(c *ProvisionFreshServerConfig) {
				c.SSHKeys = []string{"ssh-ed25519 AAAA one", "ssh-ed25519 AAAA two"}
			},
		},
		{
			name:   "small box",
			mutate: func(c *ProvisionFreshServerConfig) { c.MemoryInMB = 512 },
		},
		{
			name:   "large box",
			mutate: func(c *ProvisionFreshServerConfig) { c.MemoryInMB = 32768 },
		},
		{
			name:   "no memory reported",
			mutate: func(c *ProvisionFreshServerConfig) { c.MemoryInMB = 0 },
		},
		{
			name:   "custom ssh port",
			mutate: func(c *ProvisionFreshServerConfig) { c.SSHPort = 2222 },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config := testConfig()
			tc.mutate(&config)

			task := ProvisionFreshServer(config)
			require.NotNil(t, task)
			assert.NotEmpty(t, task.Script())
		})
	}
}

func TestProvisionDockerServerAcrossConfigurations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ProvisionDockerServerConfig)
	}{
		{name: "defaults", mutate: func(*ProvisionDockerServerConfig) {}},
		{
			name:   "with acme email",
			mutate: func(c *ProvisionDockerServerConfig) { c.ACMEEmail = "ops@example.test" },
		},
		{
			// TraefikContainer wins over the deprecated TraefikService alias.
			name: "explicit traefik container",
			mutate: func(c *ProvisionDockerServerConfig) {
				c.TraefikService = "legacy-name"
				c.TraefikContainer = "traefik-main"
			},
		},
		{
			name:   "legacy traefik service alias only",
			mutate: func(c *ProvisionDockerServerConfig) { c.TraefikService = "legacy-name" },
		},
		{
			name: "unset docker stack fields fall back to defaults",
			mutate: func(c *ProvisionDockerServerConfig) {
				c.NetworkName = ""
				c.RootDir = ""
				c.TraefikVersion = ""
			},
		},
		{
			name: "with ssh keys",
			mutate: func(c *ProvisionDockerServerConfig) {
				c.SSHKeys = []string{"ssh-ed25519 AAAA one"}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config := dockerConfig()
			tc.mutate(&config)

			task := ProvisionDockerServer(config)
			require.NotNil(t, task)
			assert.NotEmpty(t, task.Script())
		})
	}
}

// =============================================================================
// Notification failures
// =============================================================================

type failingNotifier struct{}

func (failingNotifier) SendToTeam(context.Context, string, taskrunner.Notification) error {
	return errors.New("notifier unavailable")
}

func (failingNotifier) SendToChannel(context.Context, string, taskrunner.Notification) error {
	return errors.New("notifier unavailable")
}

// A notification that can't be delivered must not roll back a provision that
// actually succeeded, nor mask a failure that already happened.
func TestProvisionCallbacksToleratesNotifierFailures(t *testing.T) {
	tests := []struct {
		name   string
		invoke func(*testing.T, *taskrunner.CallbackContext) error
	}{
		{
			name: "fresh success",
			invoke: func(t *testing.T, cbCtx *taskrunner.CallbackContext) error {
				return ProvisionFreshServer(testConfig()).OnSuccess(context.Background(), cbCtx, "task-1")
			},
		},
		{
			name: "fresh failure",
			invoke: func(t *testing.T, cbCtx *taskrunner.CallbackContext) error {
				return ProvisionFreshServer(testConfig()).OnFailure(context.Background(), cbCtx, "task-1", 3)
			},
		},
		{
			name: "fresh timeout",
			invoke: func(t *testing.T, cbCtx *taskrunner.CallbackContext) error {
				return ProvisionFreshServer(testConfig()).OnExpired(context.Background(), cbCtx, "task-1")
			},
		},
		{
			name: "docker success",
			invoke: func(t *testing.T, cbCtx *taskrunner.CallbackContext) error {
				return ProvisionDockerServer(dockerConfig()).OnSuccess(context.Background(), cbCtx, "task-1")
			},
		},
		{
			name: "docker failure",
			invoke: func(t *testing.T, cbCtx *taskrunner.CallbackContext) error {
				return ProvisionDockerServer(dockerConfig()).OnFailure(context.Background(), cbCtx, "task-1", 3)
			},
		},
		{
			name: "docker timeout",
			invoke: func(t *testing.T, cbCtx *taskrunner.CallbackContext) error {
				return ProvisionDockerServer(dockerConfig()).OnExpired(context.Background(), cbCtx, "task-1")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db := provisionCallbackDB(t)
			seedProvisioningServer(t, db)

			cbCtx := provisionCallbackContext(t, db, &recoveryBroadcaster{})
			cbCtx.Notifier = failingNotifier{}

			assert.NoError(t, tc.invoke(t, cbCtx))
		})
	}
}

// The callbacks also run with no logger wired up, so every logger guard has
// to hold.
func TestProvisionCallbacksWithoutLogger(t *testing.T) {
	db := provisionCallbackDB(t)
	seedProvisioningServer(t, db)

	cbCtx := &taskrunner.CallbackContext{DB: db}
	cbCtx.SetBroadcaster(&recoveryBroadcaster{})
	cbCtx.Notifier = failingNotifier{}

	fresh := ProvisionFreshServer(testConfig())
	assert.NoError(t, fresh.OnSuccess(context.Background(), cbCtx, "task-1"))
	assert.NoError(t, fresh.OnFailure(context.Background(), cbCtx, "task-1", 3))
	assert.NoError(t, fresh.OnExpired(context.Background(), cbCtx, "task-1"))

	docker := ProvisionDockerServer(dockerConfig())
	assert.NoError(t, docker.OnSuccess(context.Background(), cbCtx, "task-1"))
	assert.NoError(t, docker.OnFailure(context.Background(), cbCtx, "task-1", 3))
	assert.NoError(t, docker.OnExpired(context.Background(), cbCtx, "task-1"))
}

// =============================================================================
// handleStepCompleted DB failure
// =============================================================================

// The step read succeeds but the write fails — provisioning continues and the
// event still reaches the UI.
func TestHandleStepCompletedToleratesUpdateFailure(t *testing.T) {
	db := markerDB(t)
	markerServer(t, db)
	broadcaster := &recoveryBroadcaster{}
	handler := newMarkerHandler(t, db, broadcaster)

	require.NoError(t, db.Exec("ALTER TABLE servers RENAME COLUMN progress_step TO progress_step_old").Error)

	require.NoError(t, handler.OnMarker(context.Background(), "task-1",
		&markers.Marker{Type: markers.StepCompleted, Value: "configure_swap"}))
	assert.NotEmpty(t, broadcaster.events)
}

// =============================================================================
// GetTaskOutputTail
// =============================================================================

// The failure notifications include the tail of the task log; a task row that
// isn't there yields an empty tail rather than an error.
func TestProvisionFailureWithStoredTaskOutput(t *testing.T) {
	db := provisionCallbackDB(t)
	seedProvisioningServer(t, db)

	task := &models.Task{Name: "Provision", User: "root", Type: ProvisionFreshServerTaskType}
	task.ID = "task-1"
	task.ServerID = "server-123"
	task.Output = dbtype.EncryptedString("line one\nline two\nline three")
	require.NoError(t, db.Create(task).Error)

	cbCtx := provisionCallbackContext(t, db, &recoveryBroadcaster{})
	assert.NoError(t, ProvisionFreshServer(testConfig()).
		OnFailure(context.Background(), cbCtx, "task-1", 3))

	assert.Equal(t, types.ServerStatusFailed, storedServer(t, db).Status)
}
