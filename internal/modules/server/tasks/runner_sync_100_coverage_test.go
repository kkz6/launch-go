package tasks

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	servertypes "github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	queuepkg "github.com/kkz6/launch-go/internal/pkg/queue"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

type runnerSyncMarshalErrorTask struct {
	*mockCallbackTask
}

func (t *runnerSyncMarshalErrorTask) MarshalPayload() ([]byte, error) {
	return nil, errors.New("forced callback payload failure")
}

type runnerSyncBroadcastEvent struct {
	teamID string
	event  string
	data   any
}

type runnerSyncRecordingBroadcaster struct {
	broadcast.NopBroadcaster
	events []runnerSyncBroadcastEvent
}

func (b *runnerSyncRecordingBroadcaster) BroadcastToTeam(teamID, event string, data any) {
	b.events = append(b.events, runnerSyncBroadcastEvent{
		teamID: teamID,
		event:  event,
		data:   data,
	})
}

func TestTaskRunnerCompletionBuildersCoverIndependentAndLoggedErrors(t *testing.T) {
	logger := zerolog.New(&bytes.Buffer{})
	invalidPayload := func() {}

	for _, test := range []struct {
		name  string
		apply func(*TaskRunner) *TaskRunner
	}{
		{
			name: "complete",
			apply: func(runner *TaskRunner) *TaskRunner {
				return runner.OnComplete("invalid", invalidPayload)
			},
		},
		{
			name: "failed",
			apply: func(runner *TaskRunner) *TaskRunner {
				return runner.OnFailed("invalid", invalidPayload)
			},
		},
		{
			name: "timeout",
			apply: func(runner *TaskRunner) *TaskRunner {
				return runner.OnTimeout("invalid", invalidPayload)
			},
		},
	} {
		t.Run(test.name+" logs invalid payload", func(t *testing.T) {
			runner := NewTaskRunner(createTestServer(), createTestTask()).WithLogger(&logger)
			assert.Same(t, runner, test.apply(runner))
			assert.Nil(t, runner.completionConfig)
		})
	}

	t.Run("failed initializes config", func(t *testing.T) {
		runner := NewTaskRunner(createTestServer(), createTestTask()).
			OnFailed("coverage:failed", map[string]string{"id": "1"})
		require.NotNil(t, runner.completionConfig)
		require.NotNil(t, runner.completionConfig.OnFailed)
	})

	t.Run("timeout initializes config", func(t *testing.T) {
		runner := NewTaskRunner(createTestServer(), createTestTask()).
			OnTimeout("coverage:timeout", map[string]string{"id": "1"})
		require.NotNil(t, runner.completionConfig)
		require.NotNil(t, runner.completionConfig.OnTimeout)
	})
}

func TestTaskRunnerRunCoversTrackedPersistenceFailures(t *testing.T) {
	t.Run("task creation failure", func(t *testing.T) {
		db := asyncRunnerDatabase(t)
		registerRunnerSyncCreateFailure(t, db, 1)

		result, err := NewTaskRunner(createTestServer(), createTestTask()).
			WithDB(db).
			WithDispatcher(&mockDispatcher{}).
			TrackInDB().
			Run(context.Background())

		require.ErrorContains(t, err, "failed to create task model")
		assert.Nil(t, result)
	})

	t.Run("mark running failure terminates tracked task", func(t *testing.T) {
		db := asyncRunnerDatabase(t)
		registerRunnerSyncUpdateFailure(t, db, 1)
		callbackTask := createTestCallbackTask()

		result, err := NewTaskRunner(createTestServer(), callbackTask).
			WithDB(db).
			WithDispatcher(&mockDispatcher{}).
			TrackInDB().
			Run(context.Background())

		require.ErrorContains(t, err, "failed to mark task running")
		require.NotNil(t, result)
		require.NotNil(t, result.TaskModel)
		assert.Equal(t, string(servertypes.TaskStatusFailed), result.TaskModel.Status)
		assert.True(t, callbackTask.failureCalled())
	})

	t.Run("result save failure is logged", func(t *testing.T) {
		db := asyncRunnerDatabase(t)
		registerRunnerSyncUpdateFailure(t, db, 2)
		logger := zerolog.New(&bytes.Buffer{})
		dispatcher := &mockDispatcher{runFunc: func(
			context.Context,
			*taskrunner.PendingTask,
		) (*taskrunner.TaskResult, error) {
			return &taskrunner.TaskResult{ExitCode: 0, Output: "done"}, nil
		}}

		result, err := NewTaskRunner(createTestServer(), createTestTask()).
			WithDB(db).
			WithDispatcher(dispatcher).
			WithLogger(&logger).
			TrackInDB().
			Run(context.Background())

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "done", result.GetOutput())
	})

	t.Run("dispatcher error save failure is logged", func(t *testing.T) {
		db := asyncRunnerDatabase(t)
		registerRunnerSyncUpdateFailure(t, db, 2)
		logger := zerolog.New(&bytes.Buffer{})
		dispatcher := &mockDispatcher{runFunc: func(
			context.Context,
			*taskrunner.PendingTask,
		) (*taskrunner.TaskResult, error) {
			return nil, assert.AnError
		}}

		result, err := NewTaskRunner(createTestServer(), createTestTask()).
			WithDB(db).
			WithDispatcher(dispatcher).
			WithLogger(&logger).
			TrackInDB().
			Run(context.Background())

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.ErrorIs(t, result.Error, assert.AnError)
	})
}

func TestTaskRunnerFailTrackedTaskLogsSaveFailure(t *testing.T) {
	db := asyncRunnerDatabase(t)
	registerRunnerSyncUpdateFailure(t, db, 2)
	logger := zerolog.New(&bytes.Buffer{})
	server := createTestServer()
	server.PublicIPv4 = nil
	callbackTask := createTestCallbackTask()

	result, err := NewTaskRunner(server, callbackTask).
		WithDB(db).
		WithDispatcher(&mockDispatcher{}).
		WithLogger(&logger).
		TrackInDB().
		Run(context.Background())

	require.ErrorContains(t, err, "server has no public IP address")
	require.NotNil(t, result)
	assert.True(t, callbackTask.failureCalled())
}

func TestTaskRunnerCallbackErrorIsLogged(t *testing.T) {
	logger := zerolog.New(&bytes.Buffer{})
	callbackTask := createTestCallbackTask()
	callbackTask.returnError = assert.AnError
	taskModel := &models.Task{Name: "callback coverage"}

	NewTaskRunner(createTestServer(), callbackTask).
		WithLogger(&logger).
		invokeTaskCallbacks(
			context.Background(),
			taskModel,
			&taskrunner.TaskResult{ExitCode: 0},
			nil,
		)

	assert.True(t, callbackTask.successCalled())
}

func TestTaskRunnerCompletionSuccessIsLogged(t *testing.T) {
	redis := miniredis.RunT(t)
	client := queuepkg.NewClient(config.RedisConfig{Address: redis.Addr()})
	t.Cleanup(func() {
		err := client.Close()
		if err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
			require.NoError(t, err)
		}
	})
	logger := zerolog.New(&bytes.Buffer{})
	runner := NewTaskRunner(createTestServer(), createTestTask()).
		WithQueue(client).
		WithLogger(&logger).
		OnComplete("coverage:logged", map[string]string{"id": "1"})

	runner.dispatchCompletionJobs(&taskrunner.TaskResult{ExitCode: 0}, nil)

	pending, err := client.Inspector().ListPendingTasks(queuepkg.QueueDefault)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.Equal(t, "coverage:logged", pending[0].Type)
}

func TestTaskRunnerCreateTaskModelCoversUsersAndMarshalError(t *testing.T) {
	t.Run("root user", func(t *testing.T) {
		db := asyncRunnerDatabase(t)
		model, err := NewTaskRunner(createTestServer(), createTestTask()).
			WithDB(db).
			AsRoot().
			createTaskModel()
		require.NoError(t, err)
		assert.Equal(t, createTestServer().RootUsername(), model.User)
	})

	t.Run("explicit user", func(t *testing.T) {
		db := asyncRunnerDatabase(t)
		model, err := NewTaskRunner(createTestServer(), createTestTask()).
			WithDB(db).
			AsUser("deploy-user").
			createTaskModel()
		require.NoError(t, err)
		assert.Equal(t, "deploy-user", model.User)
	})

	t.Run("callback marshal failure", func(t *testing.T) {
		db := asyncRunnerDatabase(t)
		task := &runnerSyncMarshalErrorTask{mockCallbackTask: createTestCallbackTask()}
		model, err := NewTaskRunner(createTestServer(), task).
			WithDB(db).
			createTaskModel()
		require.ErrorContains(t, err, "failed to marshal callback payload")
		assert.Nil(t, model)
	})
}

func TestTaskRunnerBroadcastsTaskEvents(t *testing.T) {
	broadcaster := &runnerSyncRecordingBroadcaster{}
	server := createTestServer()
	taskModel := &models.Task{
		Name:   "broadcast coverage",
		Status: string(servertypes.TaskStatusRunning),
	}
	taskModel.ID = "task-broadcast"
	taskModel.ServerID = server.ID
	runner := NewTaskRunner(server, createTestTask()).WithBroadcaster(broadcaster)

	runner.broadcastTaskEvent("task.updated", taskModel, "output")
	runner.broadcastTaskRunning(taskModel)

	require.Len(t, broadcaster.events, 2)
	assert.Equal(t, server.TeamID, broadcaster.events[0].teamID)
	assert.Equal(t, "task.updated", broadcaster.events[0].event)
	assert.Equal(t, "task.running", broadcaster.events[1].event)
	assert.Equal(t, "task-broadcast", broadcaster.events[1].data.(map[string]any)["task_id"])
}

func registerRunnerSyncCreateFailure(t *testing.T, db *gorm.DB, nth int) {
	t.Helper()
	count := 0
	name := fmt.Sprintf("test:runner-sync:create:%s", strings.ReplaceAll(t.Name(), "/", "-"))
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register(
		name,
		func(tx *gorm.DB) {
			if tx.Statement.Table != "tasks" {
				return
			}
			count++
			if count == nth {
				tx.AddError(errors.New("forced create failure"))
			}
		},
	))
}

func registerRunnerSyncUpdateFailure(t *testing.T, db *gorm.DB, nth int) {
	t.Helper()
	count := 0
	name := fmt.Sprintf("test:runner-sync:update:%s", strings.ReplaceAll(t.Name(), "/", "-"))
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register(
		name,
		func(tx *gorm.DB) {
			if tx.Statement.Table != "tasks" {
				return
			}
			count++
			if count == nth {
				tx.AddError(errors.New("forced update failure"))
			}
		},
	))
}
