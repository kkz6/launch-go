package tasks

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	"github.com/kkz6/launch-go/internal/pkg/queue"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// The software stack is appended after the fixed provision steps, each
// wrapped in status/installed/progress markers so the UI can follow along.
func TestProvisionFreshServerRendersSoftwareStack(t *testing.T) {
	provisionConfig := testConfig()
	provisionConfig.SoftwareStack = []types.Software{
		types.SoftwarePhp83,
		types.SoftwareRedis,
		types.SoftwareCaddy2,
	}

	task := ProvisionFreshServer(provisionConfig)
	require.NotNil(t, task)

	script := task.Script()
	assert.Contains(t, script, "=== Install Software: PHP 8.3 ===")
	assert.Contains(t, script, "=== Install Software: Redis ===")
	assert.Contains(t, script, "::LAUNCH::software_installed::php83")
	assert.Contains(t, script, "::LAUNCH::status::Installing Redis")
	assert.Contains(t, script, "::LAUNCH::progress::")
}

// An unset SSH port means the customer never overrode it, so the firewall
// template has to fall back to 22 or the rule would open port 0.
func TestBuildProvisionStepDataDefaultsSSHPort(t *testing.T) {
	provisionConfig := testConfig()
	provisionConfig.SSHPort = 0

	data := buildProvisionStepData(types.ProvisionStepConfigureFirewall, provisionConfig, 2048, 35)
	require.NotNil(t, data)

	port, ok := data.(struct{ SSHPort int })
	require.True(t, ok)
	assert.Equal(t, 22, port.SSHPort)
}

// Kept renderable so a retried callback from a pre-v2 swarm provision still
// produces a script instead of panicking.
func TestRenderDockerProvisionStepLegacySwarmNetwork(t *testing.T) {
	assert.NotPanics(t, func() {
		script := renderDockerProvisionStep(types.ProvisionStepSetupSwarmNetwork, dockerConfig(), 2048, 35)
		assert.NotEmpty(t, script)
	})
}

// Same best-effort contract as the fresh-server callback: the server is
// provisioned even if the bookkeeping updates fail.
func TestProvisionDockerServerOnSuccessToleratesBookkeepingFailures(t *testing.T) {
	db := provisionCallbackDB(t)
	seedProvisioningServer(t, db)
	require.NoError(t, db.Migrator().DropTable(&models.FirewallRule{}))
	require.NoError(t, db.Migrator().DropTable(&models.InstalledService{}))

	broadcaster := &recoveryBroadcaster{}
	task := ProvisionDockerServer(dockerConfig())

	require.NoError(t, task.OnSuccess(context.Background(),
		provisionCallbackContext(t, db, broadcaster), "task-1"))

	assert.Equal(t, types.ServerStatusRunning, storedServer(t, db).Status)
	assert.NotEmpty(t, broadcaster.events)
}

// =============================================================================
// dispatchCompletionJobs with a queue client
// =============================================================================

// Points at a port nothing is listening on: Enqueue fails, which is the
// branch that logs rather than propagating — a completion job that can't be
// queued must not take down recovery.
func unreachableQueueClient() *queue.Client {
	return queue.NewClient(config.RedisConfig{Address: "127.0.0.1:1"})
}

func TestDispatchCompletionJobsEnqueuesSelectedJob(t *testing.T) {
	tests := []struct {
		name     string
		exitCode int
	}{
		{name: "finished", exitCode: 0},
		{name: "timeout", exitCode: 124},
		{name: "failed", exitCode: 9},
	}

	completion := taskrunner.CompletionConfig{
		OnFinished: &taskrunner.JobRef{Type: "job:finished", Payload: []byte(`{}`)},
		OnTimeout:  &taskrunner.JobRef{Type: "job:timeout", Payload: []byte(`{}`)},
		OnFailed:   &taskrunner.JobRef{Type: "job:failed", Payload: []byte(`{}`)},
	}
	encoded, err := json.Marshal(completion)
	require.NoError(t, err)

	client := unreachableQueueClient()
	defer client.Close()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recoverer := newRecoverer(t, recoveryDB(t), &recoveryBroadcaster{})
			recoverer.queueClient = client

			assert.NotPanics(t, func() {
				recoverer.dispatchCompletionJobs(recoveryTask(recoveryServer()), tc.exitCode, string(encoded))
			})
		})
	}
}

// =============================================================================
// invokeCallbacks with a handler-less instance
// =============================================================================

// An Instance that reconstructs to no handler is a no-op rather than an
// error — nothing is registered for that type any more.
func TestInvokeCallbacksWithUnregisteredType(t *testing.T) {
	recoverer := newRecoverer(t, recoveryDB(t), &recoveryBroadcaster{})
	task := recoveryTask(recoveryServer())

	instance, err := json.Marshal(map[string]any{
		"type":    "server:not_registered_anywhere",
		"payload": json.RawMessage(`{}`),
	})
	require.NoError(t, err)
	task.Instance = dbtype.EncryptedString(string(instance))

	assert.NoError(t, recoverer.invokeCallbacks(context.Background(), task, 0))
}

// A registered type whose factory yields no handler: nothing to invoke, and
// nothing to complain about either.
type nilHandlerState struct{}

func (nilHandlerState) NewTask() taskrunner.CallbackHandler { return nil }

func TestInvokeCallbacksWithNilHandler(t *testing.T) {
	const typeName = "server:nil_handler_state"
	taskrunner.RegisterCallbackState[nilHandlerState](typeName)

	recoverer := newRecoverer(t, recoveryDB(t), &recoveryBroadcaster{})
	task := recoveryTask(recoveryServer())

	instance, err := json.Marshal(map[string]any{
		"type":    typeName,
		"payload": json.RawMessage(`{}`),
	})
	require.NoError(t, err)
	task.Instance = dbtype.EncryptedString(string(instance))

	assert.NoError(t, recoverer.invokeCallbacks(context.Background(), task, 0))
}

// A completion config with no job for the outcome that occurred is a no-op,
// even with a queue available.
func TestDispatchCompletionJobsWithNoJobForOutcome(t *testing.T) {
	completion := taskrunner.CompletionConfig{
		OnFinished: &taskrunner.JobRef{Type: "job:finished", Payload: []byte(`{}`)},
	}
	encoded, err := json.Marshal(completion)
	require.NoError(t, err)

	client := unreachableQueueClient()
	defer client.Close()

	recoverer := newRecoverer(t, recoveryDB(t), &recoveryBroadcaster{})
	recoverer.queueClient = client

	assert.NotPanics(t, func() {
		// Exit code 9 selects OnFailed, which this config leaves unset.
		recoverer.dispatchCompletionJobs(recoveryTask(recoveryServer()), 9, string(encoded))
	})
}
