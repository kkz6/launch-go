package tasks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/database/serializers"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	servertypes "github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// =============================================================================
// Harness
// =============================================================================

type recoveryBroadcastEvent struct {
	teamID string
	event  string
	data   any
}

type recoveryBroadcaster struct {
	broadcast.NopBroadcaster
	events []recoveryBroadcastEvent
}

func (b *recoveryBroadcaster) BroadcastToTeam(teamID, event string, data any) {
	b.events = append(b.events, recoveryBroadcastEvent{teamID: teamID, event: event, data: data})
}

func recoveryDB(t *testing.T) *gorm.DB {
	t.Helper()
	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))

	db, err := gorm.Open(
		sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"),
		&gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Task{}))
	return db
}

func recoveryLogger() *zerolog.Logger {
	logger := zerolog.New(&bytes.Buffer{})
	return &logger
}

func newRecoverer(t *testing.T, db *gorm.DB, broadcaster broadcast.TeamBroadcaster) *TaskRecoverer {
	t.Helper()
	return NewTaskRecoverer(db, recoveryLogger(), nil, nil, broadcaster, nil)
}

func recoveryServer() *models.Server {
	ip := "203.0.113.10"
	server := &models.Server{
		PublicIPv4: &ip,
		PrivateKey: dbtype.EncryptedString("PRIVATE KEY"),
	}
	server.ID = "srv-1"
	server.TeamID = "team-1"
	return server
}

func recoveryTask(server *models.Server) *models.Task {
	task := &models.Task{
		Name:   "Provision",
		User:   "launch",
		Type:   "server:custom",
		Status: string(servertypes.TaskStatusRunning),
		Server: server,
	}
	task.ID = "task-1"
	if server != nil {
		task.ServerID = server.ID
	}
	return task
}

// =============================================================================
// NewTaskRecoverer
// =============================================================================

func TestNewTaskRecoverer(t *testing.T) {
	db := recoveryDB(t)
	broadcaster := &recoveryBroadcaster{}

	recoverer := NewTaskRecoverer(db, recoveryLogger(), nil, nil, broadcaster, nil)
	require.NotNil(t, recoverer)

	assert.Same(t, db, recoverer.db)
	assert.Equal(t, broadcaster, recoverer.broadcaster)
}

// =============================================================================
// RecoverOrphanedTasks
// =============================================================================

func TestRecoverOrphanedTasksWithNoTasks(t *testing.T) {
	recoverer := newRecoverer(t, recoveryDB(t), &recoveryBroadcaster{})
	assert.NoError(t, recoverer.RecoverOrphanedTasks(context.Background()))
}

func TestRecoverOrphanedTasksQueryFailure(t *testing.T) {
	db := recoveryDB(t)
	require.NoError(t, db.Migrator().DropTable(&models.Task{}))

	recoverer := newRecoverer(t, db, &recoveryBroadcaster{})
	err := recoverer.RecoverOrphanedTasks(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to query orphaned tasks")
}

// A task whose server row has gone can never be recovered, so it is failed
// rather than retried forever.
func TestRecoverOrphanedTasksMarksTasksWithMissingServerAsFailed(t *testing.T) {
	db := recoveryDB(t)
	task := recoveryTask(nil)
	task.ServerID = "srv-gone"
	require.NoError(t, db.Create(task).Error)

	recoverer := newRecoverer(t, db, &recoveryBroadcaster{})
	require.NoError(t, recoverer.RecoverOrphanedTasks(context.Background()))

	var stored models.Task
	require.NoError(t, db.First(&stored, "id = ?", task.ID).Error)
	assert.Equal(t, string(servertypes.TaskStatusFailed), stored.Status)
	assert.Contains(t, stored.Output.String(), "server not found during recovery")
}

// =============================================================================
// getConnection
// =============================================================================

func TestGetConnection(t *testing.T) {
	recoverer := newRecoverer(t, recoveryDB(t), &recoveryBroadcaster{})

	t.Run("missing public IP", func(t *testing.T) {
		server := recoveryServer()
		server.PublicIPv4 = nil

		_, err := recoverer.getConnection(recoveryTask(server))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no public IP address")
	})

	t.Run("empty public IP", func(t *testing.T) {
		server := recoveryServer()
		empty := ""
		server.PublicIPv4 = &empty

		_, err := recoverer.getConnection(recoveryTask(server))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no public IP address")
	})

	t.Run("missing private key", func(t *testing.T) {
		server := recoveryServer()
		server.PrivateKey = dbtype.EncryptedString("")

		_, err := recoverer.getConnection(recoveryTask(server))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no private key")
	})

	t.Run("root task uses the root connection", func(t *testing.T) {
		server := recoveryServer()
		task := recoveryTask(server)
		task.User = server.RootUsername()

		conn, err := recoverer.getConnection(task)
		require.NoError(t, err)
		assert.Equal(t, server.RootUsername(), conn.User)
	})

	t.Run("non-root task uses that user's connection", func(t *testing.T) {
		server := recoveryServer()
		task := recoveryTask(server)
		task.User = "deployer"

		conn, err := recoverer.getConnection(task)
		require.NoError(t, err)
		assert.Equal(t, "deployer", conn.User)
	})
}

// =============================================================================
// finalizeCompletedTask
// =============================================================================

func TestFinalizeCompletedTaskMapsExitCodeToStatus(t *testing.T) {
	tests := []struct {
		name       string
		exitCode   int
		wantStatus servertypes.TaskStatus
	}{
		{name: "success", exitCode: 0, wantStatus: servertypes.TaskStatusFinished},
		{name: "timeout", exitCode: 124, wantStatus: servertypes.TaskStatusTimeout},
		{name: "failure", exitCode: 1, wantStatus: servertypes.TaskStatusFailed},
		{name: "signal", exitCode: 137, wantStatus: servertypes.TaskStatusFailed},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db := recoveryDB(t)
			broadcaster := &recoveryBroadcaster{}
			task := recoveryTask(recoveryServer())
			require.NoError(t, db.Create(task).Error)

			recoverer := newRecoverer(t, db, broadcaster)
			require.NoError(t, recoverer.finalizeCompletedTask(context.Background(), task, tc.exitCode, "some output"))

			var stored models.Task
			require.NoError(t, db.First(&stored, "id = ?", task.ID).Error)
			assert.Equal(t, string(tc.wantStatus), stored.Status)
			require.NotNil(t, stored.ExitCode)
			assert.Equal(t, tc.exitCode, *stored.ExitCode)
			assert.Equal(t, "some output", stored.Output.String())

			require.Len(t, broadcaster.events, 1)
			assert.Equal(t, "task.updated", broadcaster.events[0].event)
			assert.Equal(t, "team-1", broadcaster.events[0].teamID)
		})
	}
}

func TestFinalizeCompletedTaskSaveFailure(t *testing.T) {
	db := recoveryDB(t)
	task := recoveryTask(recoveryServer())
	require.NoError(t, db.Migrator().DropTable(&models.Task{}))

	recoverer := newRecoverer(t, db, &recoveryBroadcaster{})
	err := recoverer.finalizeCompletedTask(context.Background(), task, 0, "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update task")
}

// =============================================================================
// markTaskFailed
// =============================================================================

func TestMarkTaskFailed(t *testing.T) {
	db := recoveryDB(t)
	broadcaster := &recoveryBroadcaster{}
	task := recoveryTask(recoveryServer())
	require.NoError(t, db.Create(task).Error)

	recoverer := newRecoverer(t, db, broadcaster)
	require.NoError(t, recoverer.markTaskFailed(task, "worker restarted"))

	var stored models.Task
	require.NoError(t, db.First(&stored, "id = ?", task.ID).Error)
	assert.Equal(t, string(servertypes.TaskStatusFailed), stored.Status)
	require.NotNil(t, stored.ExitCode)
	assert.Equal(t, 1, *stored.ExitCode)
	assert.Equal(t, "[Recovery] worker restarted", stored.Output.String())
	assert.Len(t, broadcaster.events, 1)
}

// Output captured before the restart is worth keeping — the recovery note is
// appended to it rather than replacing it.
func TestMarkTaskFailedAppendsToExistingOutput(t *testing.T) {
	db := recoveryDB(t)
	task := recoveryTask(recoveryServer())
	task.Output = dbtype.EncryptedString("partial progress")
	require.NoError(t, db.Create(task).Error)

	recoverer := newRecoverer(t, db, &recoveryBroadcaster{})
	require.NoError(t, recoverer.markTaskFailed(task, "worker restarted"))

	var stored models.Task
	require.NoError(t, db.First(&stored, "id = ?", task.ID).Error)
	assert.Equal(t, "partial progress\n[Recovery] worker restarted", stored.Output.String())
}

func TestMarkTaskFailedSaveFailure(t *testing.T) {
	db := recoveryDB(t)
	task := recoveryTask(recoveryServer())
	require.NoError(t, db.Migrator().DropTable(&models.Task{}))

	recoverer := newRecoverer(t, db, &recoveryBroadcaster{})
	assert.Error(t, recoverer.markTaskFailed(task, "worker restarted"))
}

// =============================================================================
// broadcastTaskUpdate
// =============================================================================

func TestBroadcastTaskUpdateSkipsWhenItCannotRoute(t *testing.T) {
	db := recoveryDB(t)

	t.Run("no broadcaster", func(t *testing.T) {
		recoverer := NewTaskRecoverer(db, recoveryLogger(), nil, nil, nil, nil)
		assert.NotPanics(t, func() {
			recoverer.broadcastTaskUpdate(recoveryTask(recoveryServer()), "output")
		})
	})

	t.Run("no server means no team to route to", func(t *testing.T) {
		broadcaster := &recoveryBroadcaster{}
		recoverer := newRecoverer(t, db, broadcaster)

		recoverer.broadcastTaskUpdate(recoveryTask(nil), "output")
		assert.Empty(t, broadcaster.events)
	})
}

// =============================================================================
// invokeCallbacks
// =============================================================================

type recoveryCallbackHandler struct {
	successes int
	failures  int
	expiries  int
	lastExit  int
	returnErr error
}

func (h *recoveryCallbackHandler) OnSuccess(context.Context, *taskrunner.CallbackContext, string) error {
	h.successes++
	return h.returnErr
}

func (h *recoveryCallbackHandler) OnFailure(_ context.Context, _ *taskrunner.CallbackContext, _ string, exitCode int) error {
	h.failures++
	h.lastExit = exitCode
	return h.returnErr
}

func (h *recoveryCallbackHandler) OnExpired(context.Context, *taskrunner.CallbackContext, string) error {
	h.expiries++
	return h.returnErr
}

// Registered once so invokeCallbacks can reconstruct a handler from a task's
// stored Instance payload the way the real recovery path does.
type recoveryCallbackState struct {
	Marker string `json:"marker"`
}

var recoveryHandlerSingleton = &recoveryCallbackHandler{}

func (recoveryCallbackState) NewTask() taskrunner.CallbackHandler {
	return recoveryHandlerSingleton
}

const recoveryCallbackType = "server:recovery_test_task"

func recoveryInstancePayload(t *testing.T) string {
	t.Helper()
	instance := map[string]any{
		"type":    recoveryCallbackType,
		"payload": json.RawMessage(`{"marker":"m"}`),
	}
	encoded, err := json.Marshal(instance)
	require.NoError(t, err)
	return string(encoded)
}

func TestInvokeCallbacksWithoutInstanceIsANoop(t *testing.T) {
	recoverer := newRecoverer(t, recoveryDB(t), &recoveryBroadcaster{})
	task := recoveryTask(recoveryServer())

	assert.NoError(t, recoverer.invokeCallbacks(context.Background(), task, 0))
}

func TestInvokeCallbacksRoutesByExitCode(t *testing.T) {
	taskrunner.RegisterCallbackState[recoveryCallbackState](recoveryCallbackType)

	tests := []struct {
		name     string
		exitCode int
		status   servertypes.TaskStatus
		assert   func(*testing.T, *recoveryCallbackHandler)
	}{
		{
			name:     "success",
			exitCode: 0,
			assert: func(t *testing.T, h *recoveryCallbackHandler) {
				assert.Equal(t, 1, h.successes)
			},
		},
		{
			name:     "timeout by exit code",
			exitCode: 124,
			assert: func(t *testing.T, h *recoveryCallbackHandler) {
				assert.Equal(t, 1, h.expiries)
			},
		},
		{
			// A task the recoverer already stamped as timed out routes to
			// OnExpired even though its exit code looks like a plain failure.
			name:     "timeout by stored status",
			exitCode: 1,
			status:   servertypes.TaskStatusTimeout,
			assert: func(t *testing.T, h *recoveryCallbackHandler) {
				assert.Equal(t, 1, h.expiries)
			},
		},
		{
			name:     "failure",
			exitCode: 3,
			assert: func(t *testing.T, h *recoveryCallbackHandler) {
				assert.Equal(t, 1, h.failures)
				assert.Equal(t, 3, h.lastExit)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			*recoveryHandlerSingleton = recoveryCallbackHandler{}

			task := recoveryTask(recoveryServer())
			task.Instance = dbtype.EncryptedString(recoveryInstancePayload(t))
			if tc.status != "" {
				task.Status = string(tc.status)
			}

			recoverer := newRecoverer(t, recoveryDB(t), &recoveryBroadcaster{})
			require.NoError(t, recoverer.invokeCallbacks(context.Background(), task, tc.exitCode))
			tc.assert(t, recoveryHandlerSingleton)
		})
	}
}

func TestInvokeCallbacksPropagatesHandlerErrors(t *testing.T) {
	taskrunner.RegisterCallbackState[recoveryCallbackState](recoveryCallbackType)

	for _, exitCode := range []int{0, 124, 5} {
		*recoveryHandlerSingleton = recoveryCallbackHandler{returnErr: errors.New("callback exploded")}

		task := recoveryTask(recoveryServer())
		task.Instance = dbtype.EncryptedString(recoveryInstancePayload(t))

		recoverer := newRecoverer(t, recoveryDB(t), &recoveryBroadcaster{})
		err := recoverer.invokeCallbacks(context.Background(), task, exitCode)

		require.Error(t, err, "exit code %d", exitCode)
		assert.Contains(t, err.Error(), "callback exploded")
	}
}

// =============================================================================
// dispatchCompletionJobs
// =============================================================================

// An Instance that isn't a callback payload may still be a CompletionConfig,
// so invokeCallbacks falls through to the completion-job path rather than
// erroring.
func TestInvokeCallbacksFallsBackToCompletionJobs(t *testing.T) {
	recoverer := newRecoverer(t, recoveryDB(t), &recoveryBroadcaster{})
	task := recoveryTask(recoveryServer())
	task.Instance = dbtype.EncryptedString(`{"not":"a callback"}`)

	assert.NoError(t, recoverer.invokeCallbacks(context.Background(), task, 0))
}

func TestDispatchCompletionJobsWithoutQueueClient(t *testing.T) {
	recoverer := newRecoverer(t, recoveryDB(t), &recoveryBroadcaster{})
	task := recoveryTask(recoveryServer())

	config := taskrunner.CompletionConfig{
		OnFinished: &taskrunner.JobRef{Type: "site:deploy", Payload: []byte(`{}`)},
	}
	encoded, err := json.Marshal(config)
	require.NoError(t, err)

	assert.NotPanics(t, func() {
		recoverer.dispatchCompletionJobs(task, 0, string(encoded))
	})
}

func TestDispatchCompletionJobsIgnoresUnusableConfigs(t *testing.T) {
	recoverer := newRecoverer(t, recoveryDB(t), &recoveryBroadcaster{})
	task := recoveryTask(recoveryServer())

	for _, instance := range []string{
		"not json at all",
		`{"on_finished":null}`,
		`{"on_finished":{"type":""}}`,
	} {
		assert.NotPanics(t, func() {
			recoverer.dispatchCompletionJobs(task, 0, instance)
		}, "instance %q", instance)
	}
}

func TestDispatchCompletionJobsSelectsJobByExitCode(t *testing.T) {
	config := taskrunner.CompletionConfig{
		OnFinished: &taskrunner.JobRef{Type: "job:finished", Payload: []byte(`{}`)},
		OnTimeout:  &taskrunner.JobRef{Type: "job:timeout", Payload: []byte(`{}`)},
		OnFailed:   &taskrunner.JobRef{Type: "job:failed", Payload: []byte(`{}`)},
	}
	encoded, err := json.Marshal(config)
	require.NoError(t, err)

	tests := []struct {
		exitCode int
		wantType string
	}{
		{exitCode: 0, wantType: "job:finished"},
		{exitCode: 124, wantType: "job:timeout"},
		{exitCode: 7, wantType: "job:failed"},
	}

	for _, tc := range tests {
		parsed, err := taskrunner.UnmarshalCompletionConfig(string(encoded))
		require.NoError(t, err)

		var jobRef *taskrunner.JobRef
		switch tc.exitCode {
		case 0:
			jobRef = parsed.OnFinished
		case 124:
			jobRef = parsed.OnTimeout
		default:
			jobRef = parsed.OnFailed
		}

		require.NotNil(t, jobRef)
		assert.Equal(t, tc.wantType, jobRef.Type)
		assert.Equal(t, tc.wantType, asynq.NewTask(jobRef.Type, jobRef.Payload).Type())
	}
}

// =============================================================================
// remonitorRunningTask
// =============================================================================

// The dispatcher is constructed without a stream monitor in tests, which is
// the same state a worker is in when monitoring is disabled.
func TestRemonitorRunningTaskWithoutStreamMonitor(t *testing.T) {
	recoverer := newRecoverer(t, recoveryDB(t), &recoveryBroadcaster{})
	// A zero-value dispatcher has no stream monitor, matching a worker
	// where monitoring was never wired up.
	recoverer.dispatcher = &taskrunner.Dispatcher{}

	task := recoveryTask(recoveryServer())
	conn, err := recoverer.getConnection(task)
	require.NoError(t, err)

	err = recoverer.remonitorRunningTask(context.Background(), task, conn)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stream monitor not available")
}

// =============================================================================
// markerHandlerForTask
// =============================================================================

// Provision tasks get a marker handler so the progress bar and step list keep
// updating after a worker restart; everything else broadcasts via callbacks.
func TestMarkerHandlerForTask(t *testing.T) {
	recoverer := newRecoverer(t, recoveryDB(t), &recoveryBroadcaster{})

	t.Run("nil task", func(t *testing.T) {
		assert.Nil(t, recoverer.markerHandlerForTask(nil))
	})

	t.Run("task without a server", func(t *testing.T) {
		assert.Nil(t, recoverer.markerHandlerForTask(recoveryTask(nil)))
	})

	t.Run("non-provision task", func(t *testing.T) {
		task := recoveryTask(recoveryServer())
		task.Type = "site:deploy"
		assert.Nil(t, recoverer.markerHandlerForTask(task))
	})

	for _, taskType := range []string{ProvisionFreshServerTaskType, ProvisionDockerServerTaskType} {
		t.Run(taskType, func(t *testing.T) {
			task := recoveryTask(recoveryServer())
			task.Type = taskType
			assert.NotNil(t, recoverer.markerHandlerForTask(task))
		})
	}
}
