package tasks

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/database/serializers"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	servertypes "github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

func TestTaskRunnerRunAsyncPersistsTerminalResults(t *testing.T) {
	for _, test := range []struct {
		name       string
		result     *taskrunner.TaskResult
		wantStatus string
		callback   func(*mockCallbackTask) bool
	}{
		{
			name:       "success",
			result:     &taskrunner.TaskResult{ExitCode: 0, Output: "finished"},
			wantStatus: string(servertypes.TaskStatusFinished),
			callback:   func(task *mockCallbackTask) bool { return task.onSuccessCalled },
		},
		{
			name:       "failure",
			result:     &taskrunner.TaskResult{ExitCode: 9, Output: "failed"},
			wantStatus: string(servertypes.TaskStatusFailed),
			callback:   func(task *mockCallbackTask) bool { return task.onFailureCalled },
		},
		{
			name:       "timeout",
			result:     &taskrunner.TaskResult{ExitCode: 124, Output: "timeout", TimedOut: true},
			wantStatus: string(servertypes.TaskStatusTimeout),
			callback:   func(task *mockCallbackTask) bool { return task.onExpiredCalled },
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			db := asyncRunnerDatabase(t)
			dispatcher := taskrunner.NewFakeDispatcher()
			dispatcher.DefaultResult = test.result
			callbackTask := createTestCallbackTask()
			runner := NewTaskRunner(createTestServer(), callbackTask).
				WithDB(db).
				WithDispatcher(dispatcher)

			tracked, err := runner.RunAsync(context.Background())
			require.NoError(t, err)
			require.NotNil(t, tracked)
			require.Eventually(t, func() bool {
				var persisted models.Task
				if err := db.First(&persisted, "id = ?", tracked.ID).Error; err != nil {
					return false
				}
				return persisted.Status == test.wantStatus
			}, time.Second, 10*time.Millisecond)
			assert.True(t, test.callback(callbackTask))
		})
	}
}

func TestTaskRunnerRunAsyncPersistsDispatcherError(t *testing.T) {
	db := asyncRunnerDatabase(t)
	dispatcher := taskrunner.NewFakeDispatcher()
	dispatcher.SetRunError(assert.AnError)
	callbackTask := createTestCallbackTask()
	runner := NewTaskRunner(createTestServer(), callbackTask).
		WithDB(db).
		WithDispatcher(dispatcher)

	tracked, err := runner.RunAsync(context.Background())
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		var persisted models.Task
		if err := db.First(&persisted, "id = ?", tracked.ID).Error; err != nil {
			return false
		}
		return persisted.Status == string(servertypes.TaskStatusFailed) &&
			strings.Contains(persisted.Output.String(), assert.AnError.Error())
	}, time.Second, 10*time.Millisecond)
	assert.True(t, callbackTask.onFailureCalled)
}

func TestTaskRunnerRunInBackgroundPersistsCompletion(t *testing.T) {
	for _, test := range []struct {
		name       string
		result     *taskrunner.TaskResult
		wantStatus string
	}{
		{
			name:       "success",
			result:     &taskrunner.TaskResult{ExitCode: 0, Output: "background finished"},
			wantStatus: string(servertypes.TaskStatusFinished),
		},
		{
			name:       "failure",
			result:     &taskrunner.TaskResult{ExitCode: 2, Output: "background failed"},
			wantStatus: string(servertypes.TaskStatusFailed),
		},
		{
			name:       "timeout",
			result:     &taskrunner.TaskResult{ExitCode: 124, Output: "background timeout", TimedOut: true},
			wantStatus: string(servertypes.TaskStatusTimeout),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			db := asyncRunnerDatabase(t)
			dispatcher := taskrunner.NewFakeDispatcher()
			dispatcher.DefaultResult = test.result
			runner := NewTaskRunner(createTestServer(), createTestCallbackTask()).
				WithDB(db).
				WithDispatcher(dispatcher)

			tracked, err := runner.RunInBackground(context.Background())
			require.NoError(t, err)
			require.Eventually(t, func() bool {
				var persisted models.Task
				if err := db.First(&persisted, "id = ?", tracked.ID).Error; err != nil {
					return false
				}
				return persisted.Status == test.wantStatus
			}, time.Second, 10*time.Millisecond)
			assert.True(t, dispatcher.LastExecution().Background)
		})
	}
}

func TestTaskRunnerDependenciesConfigureAndRun(t *testing.T) {
	dispatcher := taskrunner.NewFakeDispatcher()
	logger := zerolog.Nop()
	deps := &TaskRunnerDeps{Dispatcher: dispatcher, Logger: &logger}
	server := createTestServer()
	task := createTestTask()

	runner := deps.NewRunner(server, task)
	assert.Same(t, dispatcher, runner.dispatcher)
	assert.Same(t, &logger, runner.logger)

	result, err := deps.RunTask(context.Background(), server, task, true)
	require.NoError(t, err)
	assert.True(t, result.IsSuccessful())
	require.NotNil(t, dispatcher.LastExecution())
	assert.Equal(t, server.RootUsername(), dispatcher.LastExecution().AsUser)

	result, err = deps.RunTask(context.Background(), server, task, false)
	require.NoError(t, err)
	assert.True(t, result.IsSuccessful())
	assert.Equal(t, server.GetUsername(), dispatcher.LastExecution().AsUser)
}

func asyncRunnerDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	require.NoError(t, serializers.SetEncryptionKey(
		[]byte("0123456789abcdef0123456789abcdef"),
	))
	db, err := gorm.Open(
		sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"),
		&gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Task{}))
	return db
}
