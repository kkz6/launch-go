package tasks

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/database/serializers"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	servertypes "github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner/markers"
)

type runnerAsync100DispatcherFunc func(
	context.Context,
	*taskrunner.PendingTask,
) (*taskrunner.TaskResult, error)

func (f runnerAsync100DispatcherFunc) Run(
	ctx context.Context,
	pending *taskrunner.PendingTask,
) (*taskrunner.TaskResult, error) {
	return f(ctx, pending)
}

type runnerAsync100CallbackTask struct {
	*mockTask
	events chan string
}

func newRunnerAsync100CallbackTask() *runnerAsync100CallbackTask {
	return &runnerAsync100CallbackTask{
		mockTask: createTestTask(),
		events:   make(chan string, 1),
	}
}

func (t *runnerAsync100CallbackTask) TypeName() string { return "test:runner-async-100" }

func (t *runnerAsync100CallbackTask) MarshalPayload() ([]byte, error) { return []byte(`{}`), nil }

func (t *runnerAsync100CallbackTask) OnSuccess(
	context.Context,
	*taskrunner.CallbackContext,
	string,
) error {
	t.events <- "success"
	return nil
}

func (t *runnerAsync100CallbackTask) OnFailure(
	context.Context,
	*taskrunner.CallbackContext,
	string,
	int,
) error {
	t.events <- "failure"
	return nil
}

func (t *runnerAsync100CallbackTask) OnExpired(
	context.Context,
	*taskrunner.CallbackContext,
	string,
) error {
	t.events <- "timeout"
	return nil
}

func TestTaskRunnerAsyncAndBackgroundValidateDependencies(t *testing.T) {
	t.Run("async dispatcher", func(t *testing.T) {
		_, err := NewTaskRunner(createTestServer(), createTestTask()).RunAsync(context.Background())
		require.EqualError(t, err, "no dispatcher set")
	})

	t.Run("async database", func(t *testing.T) {
		_, err := NewTaskRunner(createTestServer(), createTestTask()).
			WithDispatcher(&mockDispatcher{}).
			RunAsync(context.Background())
		require.EqualError(t, err, "database required for async tasks")
	})

	t.Run("background dispatcher", func(t *testing.T) {
		_, err := NewTaskRunner(createTestServer(), createTestTask()).RunInBackground(context.Background())
		require.EqualError(t, err, "no dispatcher set")
	})

	t.Run("background database", func(t *testing.T) {
		_, err := NewTaskRunner(createTestServer(), createTestTask()).
			WithDispatcher(&mockDispatcher{}).
			RunInBackground(context.Background())
		require.EqualError(t, err, "database required for background tasks")
	})
}

func TestTaskRunnerAsyncAndBackgroundReportCreateFailure(t *testing.T) {
	for _, test := range []struct {
		name string
		run  func(context.Context, *TaskRunner) (*models.Task, error)
	}{
		{
			name: "async",
			run: func(ctx context.Context, runner *TaskRunner) (*models.Task, error) {
				return runner.RunAsync(ctx)
			},
		},
		{
			name: "background",
			run: func(ctx context.Context, runner *TaskRunner) (*models.Task, error) {
				return runner.RunInBackground(ctx)
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			db := runnerAsync100Database(t, false)
			runner := NewTaskRunner(createTestServer(), createTestTask()).
				WithDB(db).
				WithDispatcher(&mockDispatcher{})

			tracked, err := test.run(context.Background(), runner)
			require.Nil(t, tracked)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to create task model")
		})
	}
}

func TestTaskRunnerRunAsyncHandlesPersistenceFailures(t *testing.T) {
	t.Run("mark running", func(t *testing.T) {
		db := runnerAsync100Database(t, true)
		runnerAsync100FailUpdates(t, db, func(int32) bool { return true })
		callbackTask := newRunnerAsync100CallbackTask()
		log := zerolog.New(io.Discard)
		runner := NewTaskRunner(createTestServer(), callbackTask).
			WithDB(db).
			WithDispatcher(&mockDispatcher{}).
			WithLogger(&log)

		tracked, err := runner.RunAsync(context.Background())
		require.NoError(t, err)
		require.NotNil(t, tracked)
		runnerAsync100WaitEvent(t, callbackTask, "failure")

		stored := runnerAsync100StoredTask(t, db, tracked.ID)
		assert.Equal(t, string(servertypes.TaskStatusPending), stored.Status)
	})

	t.Run("terminal result", func(t *testing.T) {
		db := runnerAsync100Database(t, true)
		runnerAsync100FailUpdates(t, db, func(call int32) bool { return call == 2 })
		callbackTask := newRunnerAsync100CallbackTask()
		log := zerolog.New(io.Discard)
		runner := NewTaskRunner(createTestServer(), callbackTask).
			WithDB(db).
			WithDispatcher(&mockDispatcher{}).
			WithLogger(&log)

		tracked, err := runner.RunAsync(context.Background())
		require.NoError(t, err)
		runnerAsync100WaitEvent(t, callbackTask, "success")

		stored := runnerAsync100StoredTask(t, db, tracked.ID)
		assert.Equal(t, string(servertypes.TaskStatusRunning), stored.Status)
	})
}

func TestTaskRunnerRunAsyncLogsAndPersistsDispatcherError(t *testing.T) {
	db := runnerAsync100Database(t, true)
	callbackTask := newRunnerAsync100CallbackTask()
	log := zerolog.New(io.Discard)
	runErr := errors.New("async transport failed")
	dispatcher := runnerAsync100DispatcherFunc(func(
		context.Context,
		*taskrunner.PendingTask,
	) (*taskrunner.TaskResult, error) {
		return nil, runErr
	})
	runner := NewTaskRunner(createTestServer(), callbackTask).
		WithDB(db).
		WithDispatcher(dispatcher).
		WithLogger(&log)

	tracked, err := runner.RunAsync(context.Background())
	require.NoError(t, err)
	runnerAsync100WaitEvent(t, callbackTask, "failure")

	stored := runnerAsync100StoredTask(t, db, tracked.ID)
	assert.Equal(t, string(servertypes.TaskStatusFailed), stored.Status)
	assert.Contains(t, stored.Output.String(), runErr.Error())
}

func TestTaskRunnerRunInBackgroundHandlesTerminalResultsWithLogging(t *testing.T) {
	for _, test := range []struct {
		name       string
		result     *taskrunner.TaskResult
		wantEvent  string
		wantStatus servertypes.TaskStatus
	}{
		{
			name:       "success",
			result:     &taskrunner.TaskResult{ExitCode: 0, Output: "completed"},
			wantEvent:  "success",
			wantStatus: servertypes.TaskStatusFinished,
		},
		{
			name:       "timeout",
			result:     &taskrunner.TaskResult{ExitCode: 124, Output: "timed out", TimedOut: true},
			wantEvent:  "timeout",
			wantStatus: servertypes.TaskStatusTimeout,
		},
		{
			name:       "failure",
			result:     &taskrunner.TaskResult{ExitCode: 7, Output: "failed"},
			wantEvent:  "failure",
			wantStatus: servertypes.TaskStatusFailed,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			db := runnerAsync100Database(t, true)
			callbackTask := newRunnerAsync100CallbackTask()
			log := zerolog.New(io.Discard)
			observed := make(chan *taskrunner.PendingTask, 1)
			dispatcher := runnerAsync100DispatcherFunc(func(
				_ context.Context,
				pending *taskrunner.PendingTask,
			) (*taskrunner.TaskResult, error) {
				observed <- pending
				pending.GetCompletionChannel() <- test.result
				return nil, nil
			})
			markerHandler := taskrunner.MarkerHandlerFunc(func(
				context.Context,
				string,
				*markers.Marker,
			) error {
				return nil
			})
			runner := NewTaskRunner(createTestServer(), callbackTask).
				WithDB(db).
				WithDispatcher(dispatcher).
				WithLogger(&log).
				WithMarkerHandler(markerHandler)

			tracked, err := runner.RunInBackground(context.Background())
			require.NoError(t, err)
			runnerAsync100WaitEvent(t, callbackTask, test.wantEvent)

			pending := <-observed
			assert.True(t, pending.ShouldRunInBackground())
			assert.NotNil(t, pending.GetMarkerHandler())
			stored := runnerAsync100StoredTask(t, db, tracked.ID)
			assert.Equal(t, string(test.wantStatus), stored.Status)
		})
	}
}

func TestTaskRunnerRunInBackgroundHandlesPersistenceFailures(t *testing.T) {
	t.Run("mark running", func(t *testing.T) {
		db := runnerAsync100Database(t, true)
		runnerAsync100FailUpdates(t, db, func(int32) bool { return true })
		callbackTask := newRunnerAsync100CallbackTask()
		log := zerolog.New(io.Discard)
		runner := NewTaskRunner(createTestServer(), callbackTask).
			WithDB(db).
			WithDispatcher(&mockDispatcher{}).
			WithLogger(&log)

		tracked, err := runner.RunInBackground(context.Background())
		require.NoError(t, err)
		runnerAsync100WaitEvent(t, callbackTask, "failure")

		stored := runnerAsync100StoredTask(t, db, tracked.ID)
		assert.Equal(t, string(servertypes.TaskStatusPending), stored.Status)
	})

	t.Run("terminal result", func(t *testing.T) {
		db := runnerAsync100Database(t, true)
		runnerAsync100FailUpdates(t, db, func(call int32) bool { return call == 2 })
		callbackTask := newRunnerAsync100CallbackTask()
		log := zerolog.New(io.Discard)
		dispatcher := runnerAsync100DispatcherFunc(func(
			_ context.Context,
			pending *taskrunner.PendingTask,
		) (*taskrunner.TaskResult, error) {
			pending.GetCompletionChannel() <- &taskrunner.TaskResult{ExitCode: 0, Output: "done"}
			return nil, nil
		})
		runner := NewTaskRunner(createTestServer(), callbackTask).
			WithDB(db).
			WithDispatcher(dispatcher).
			WithLogger(&log)

		tracked, err := runner.RunInBackground(context.Background())
		require.NoError(t, err)
		runnerAsync100WaitEvent(t, callbackTask, "success")

		stored := runnerAsync100StoredTask(t, db, tracked.ID)
		assert.Equal(t, string(servertypes.TaskStatusRunning), stored.Status)
	})
}

func TestTaskRunnerRunInBackgroundHandlesStartMonitorFailures(t *testing.T) {
	for _, test := range []struct {
		name       string
		timeout    time.Duration
		dispatcher runnerAsync100DispatcherFunc
		wantOutput string
	}{
		{
			name: "start error",
			dispatcher: func(
				context.Context,
				*taskrunner.PendingTask,
			) (*taskrunner.TaskResult, error) {
				return nil, errors.New("background start failed")
			},
			wantOutput: "background start failed",
		},
		{
			name:    "monitor timeout",
			timeout: -time.Minute,
			dispatcher: func(
				ctx context.Context,
				_ *taskrunner.PendingTask,
			) (*taskrunner.TaskResult, error) {
				<-ctx.Done()
				return nil, nil
			},
			wantOutput: context.DeadlineExceeded.Error(),
		},
		{
			name: "nil monitor result",
			dispatcher: func(
				_ context.Context,
				pending *taskrunner.PendingTask,
			) (*taskrunner.TaskResult, error) {
				pending.GetCompletionChannel() <- nil
				return nil, nil
			},
			wantOutput: "background task monitor returned no result",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			db := runnerAsync100Database(t, true)
			callbackTask := newRunnerAsync100CallbackTask()
			if test.timeout != 0 {
				callbackTask.timeout = test.timeout
			}
			log := zerolog.New(io.Discard)
			runner := NewTaskRunner(createTestServer(), callbackTask).
				WithDB(db).
				WithDispatcher(test.dispatcher).
				WithLogger(&log)

			tracked, err := runner.RunInBackground(context.Background())
			require.NoError(t, err)
			runnerAsync100WaitEvent(t, callbackTask, "failure")

			stored := runnerAsync100StoredTask(t, db, tracked.ID)
			assert.Equal(t, string(servertypes.TaskStatusFailed), stored.Status)
			assert.Contains(t, stored.Output.String(), test.wantOutput)
		})
	}
}

func runnerAsync100Database(t *testing.T, migrate bool) *gorm.DB {
	t.Helper()
	require.NoError(t, serializers.SetEncryptionKey(
		[]byte("0123456789abcdef0123456789abcdef"),
	))
	dsn := "file:" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()) +
		"?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	if migrate {
		require.NoError(t, db.AutoMigrate(&models.Task{}))
	}
	return db
}

func runnerAsync100FailUpdates(
	t *testing.T,
	db *gorm.DB,
	fail func(call int32) bool,
) {
	t.Helper()
	var calls atomic.Int32
	name := "runner_async_100_fail_update"
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register(
		name,
		func(tx *gorm.DB) {
			if fail(calls.Add(1)) {
				tx.AddError(errors.New("forced update failure"))
			}
		},
	))
	t.Cleanup(func() {
		require.NoError(t, db.Callback().Update().Remove(name))
	})
}

func runnerAsync100StoredTask(t *testing.T, db *gorm.DB, id string) models.Task {
	t.Helper()
	var stored models.Task
	require.NoError(t, db.First(&stored, "id = ?", id).Error)
	return stored
}

func runnerAsync100WaitEvent(
	t *testing.T,
	task *runnerAsync100CallbackTask,
	want string,
) {
	t.Helper()
	select {
	case got := <-task.events:
		assert.Equal(t, want, got)
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %q callback", want)
	}
}
