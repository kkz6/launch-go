package tasks

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	queuepkg "github.com/kkz6/launch-go/internal/pkg/queue"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

type completionCoverageTask struct {
	*mockTask
	jobType string
	payload any
}

func (t *completionCoverageTask) CompletionJobType() string { return t.jobType }
func (t *completionCoverageTask) CompletionPayload() any    { return t.payload }

func TestTaskRunnerDispatchesConfiguredCompletionJobs(t *testing.T) {
	redis := miniredis.RunT(t)
	client := queuepkg.NewClient(config.RedisConfig{Address: redis.Addr()})
	t.Cleanup(func() {
		err := client.Close()
		if err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
			require.NoError(t, err)
		}
	})

	runner := NewTaskRunner(createTestServer(), createTestTask()).
		WithQueue(client).
		OnComplete("coverage:finished", map[string]string{"state": "finished"}).
		OnFailed("coverage:failed", map[string]string{"state": "failed"}).
		OnTimeout("coverage:timeout", map[string]string{"state": "timeout"})

	runner.dispatchCompletionJobs(&taskrunner.TaskResult{ExitCode: 0}, nil)
	runner.dispatchCompletionJobs(&taskrunner.TaskResult{ExitCode: 2}, nil)
	runner.dispatchCompletionJobs(&taskrunner.TaskResult{ExitCode: 124, TimedOut: true}, nil)
	runner.dispatchCompletionJobs(nil, assert.AnError)
	runner.dispatchCompletionJobs(nil, nil)

	pending, err := client.Inspector().ListPendingTasks(queuepkg.QueueDefault)
	require.NoError(t, err)
	require.Len(t, pending, 4)
	assert.ElementsMatch(t, []string{
		"coverage:finished",
		"coverage:failed",
		"coverage:timeout",
		"coverage:failed",
	}, []string{pending[0].Type, pending[1].Type, pending[2].Type, pending[3].Type})
}

func TestTaskRunnerExtractsCompletionJobAndHandlesUnavailableQueue(t *testing.T) {
	logger := zerolog.New(io.Discard)
	task := &completionCoverageTask{
		mockTask: createTestTask(),
		jobType:  "coverage:extracted",
		payload:  map[string]string{"id": "task-1"},
	}
	runner := NewTaskRunner(createTestServer(), task).WithLogger(&logger)

	// The extracted configuration is valid, but a missing queue must be a safe no-op.
	runner.dispatchCompletionJobs(&taskrunner.TaskResult{ExitCode: 0}, nil)

	// Invalid extracted payloads and empty completion types also remain safe no-ops.
	task.payload = func() {}
	runner.dispatchCompletionJobs(&taskrunner.TaskResult{ExitCode: 0}, nil)
	task.jobType = ""
	runner.dispatchCompletionJobs(&taskrunner.TaskResult{ExitCode: 0}, nil)
}

func TestTaskRunnerLogsCompletionEnqueueFailure(t *testing.T) {
	redis := miniredis.RunT(t)
	client := queuepkg.NewClient(config.RedisConfig{Address: redis.Addr()})
	require.NoError(t, client.Close())
	logger := zerolog.New(io.Discard)
	runner := NewTaskRunner(createTestServer(), createTestTask()).
		WithQueue(client).
		WithLogger(&logger).
		OnComplete("coverage:closed", nil)

	runner.dispatchCompletionJobs(&taskrunner.TaskResult{ExitCode: 0}, nil)
}

func TestTaskRunnerCreateTaskModelCompletionSerialization(t *testing.T) {
	db := asyncRunnerDatabase(t)
	server := createTestServer()

	t.Run("explicit completion configuration", func(t *testing.T) {
		runner := NewTaskRunner(server, createTestTask()).
			WithDB(db).
			OnComplete("coverage:stored", map[string]string{"id": "1"})

		model, err := runner.createTaskModel()
		require.NoError(t, err)
		assert.Contains(t, model.Instance.String(), "coverage:stored")
	})

	t.Run("completion configuration from task", func(t *testing.T) {
		task := &completionCoverageTask{
			mockTask: createTestTask(),
			jobType:  "coverage:task-stored",
			payload:  map[string]string{"id": "2"},
		}
		runner := NewTaskRunner(server, task).WithDB(db)

		model, err := runner.createTaskModel()
		require.NoError(t, err)
		assert.Contains(t, model.Instance.String(), "coverage:task-stored")
	})

	t.Run("invalid explicit completion configuration", func(t *testing.T) {
		runner := NewTaskRunner(server, createTestTask()).WithDB(db)
		runner.completionConfig = &taskrunner.CompletionConfig{
			OnFinished: &taskrunner.JobRef{
				Type:    "coverage:invalid",
				Payload: json.RawMessage("{"),
			},
		}

		model, err := runner.createTaskModel()
		require.Error(t, err)
		assert.Nil(t, model)
		assert.Contains(t, err.Error(), "failed to marshal completion config")
	})

	t.Run("database create failure", func(t *testing.T) {
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())

		model, err := NewTaskRunner(server, createTestTask()).WithDB(db).createTaskModel()
		require.Error(t, err)
		assert.Nil(t, model)
	})
}

func TestTaskRunnerHandleCompletionWithoutCallbacksOrConfig(t *testing.T) {
	runner := NewTaskRunner(createTestServer(), createTestTask())
	runner.handleTaskCompletion(
		context.Background(),
		&models.Task{},
		&taskrunner.TaskResult{ExitCode: 0},
		nil,
	)
}
