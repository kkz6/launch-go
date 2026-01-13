package tasks

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/oklog/ulid/v2"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/queue"
)

// TaskStatus represents the status of a task execution.
type TaskStatus string

const (
	TaskStatusPending  TaskStatus = "pending"
	TaskStatusRunning  TaskStatus = "running"
	TaskStatusFinished TaskStatus = "finished"
	TaskStatusFailed   TaskStatus = "failed"
	TaskStatusTimeout  TaskStatus = "timeout"
)

// CallbackURLs holds the webhook URLs for task status updates.
type CallbackURLs struct {
	FinishedURL string
	FailedURL   string
	TimeoutURL  string
}

// TaskRunnerResult holds the result of a task execution.
type TaskRunnerResult struct {
	TaskModel  *models.Task
	TaskResult *taskrunner.TaskResult
	Error      error
}

// IsSuccessful returns true if the task completed successfully.
func (r *TaskRunnerResult) IsSuccessful() bool {
	if r.TaskResult != nil {
		return r.TaskResult.IsSuccessful()
	}
	if r.TaskModel != nil {
		return r.TaskModel.IsSuccessful()
	}
	return false
}

// GetOutput returns the task output.
func (r *TaskRunnerResult) GetOutput() string {
	if r.TaskResult != nil {
		return r.TaskResult.Output
	}
	if r.TaskModel != nil && !r.TaskModel.Output.IsEmpty() {
		return r.TaskModel.Output.String()
	}
	return ""
}

// GetExitCode returns the task exit code.
func (r *TaskRunnerResult) GetExitCode() int {
	if r.TaskResult != nil {
		return r.TaskResult.ExitCode
	}
	if r.TaskModel != nil && r.TaskModel.ExitCode != nil {
		return *r.TaskModel.ExitCode
	}
	return -1
}

// TaskRunner handles task execution on servers.
// Use the builder pattern to configure and execute tasks.
type TaskRunner struct {
	server           *models.Server
	task             taskrunner.Task
	db               *gorm.DB
	queue            *queue.Client
	dispatcher       *taskrunner.Dispatcher
	logger           *zerolog.Logger
	asRoot           bool
	username         string
	trackInDB        bool
	throwOnError     bool
	callbackURLs     *CallbackURLs
	completionConfig *taskrunner.CompletionConfig
}

// NewTaskRunner creates a new TaskRunner for a server.
func NewTaskRunner(server *models.Server, task taskrunner.Task) *TaskRunner {
	return &TaskRunner{
		server: server,
		task:   task,
	}
}

// WithDB sets the database connection for task tracking.
func (r *TaskRunner) WithDB(db *gorm.DB) *TaskRunner {
	r.db = db
	return r
}

// WithDispatcher sets the task dispatcher.
func (r *TaskRunner) WithDispatcher(dispatcher *taskrunner.Dispatcher) *TaskRunner {
	r.dispatcher = dispatcher
	return r
}

// WithQueue sets the queue client for dispatching completion jobs.
func (r *TaskRunner) WithQueue(q *queue.Client) *TaskRunner {
	r.queue = q
	return r
}

// WithLogger sets the logger.
func (r *TaskRunner) WithLogger(logger *zerolog.Logger) *TaskRunner {
	r.logger = logger
	return r
}

// AsRoot sets the task to run as root.
func (r *TaskRunner) AsRoot() *TaskRunner {
	r.asRoot = true
	r.username = ""
	return r
}

// AsUser sets the task to run as a specific user.
func (r *TaskRunner) AsUser(username ...string) *TaskRunner {
	r.asRoot = false
	if len(username) > 0 && username[0] != "" {
		r.username = username[0]
	} else {
		r.username = r.server.GetUsername()
	}
	return r
}

// TrackInDB enables database tracking for the task.
func (r *TaskRunner) TrackInDB() *TaskRunner {
	r.trackInDB = true
	return r
}

// ThrowOnError enables throwing errors on task failure.
func (r *TaskRunner) ThrowOnError() *TaskRunner {
	r.throwOnError = true
	return r
}

// Throw is an alias for ThrowOnError.
func (r *TaskRunner) Throw() *TaskRunner {
	return r.ThrowOnError()
}

// WithCallbacks sets callback URLs for async tasks.
func (r *TaskRunner) WithCallbacks(urls *CallbackURLs) *TaskRunner {
	r.callbackURLs = urls
	return r
}

// OnComplete sets a job to dispatch when the task completes successfully.
func (r *TaskRunner) OnComplete(jobType string, payload any) *TaskRunner {
	jobRef, err := taskrunner.NewJobRef(jobType, payload)
	if err != nil {
		if r.logger != nil {
			r.logger.Error().Err(err).Msg("Failed to create job ref for OnComplete")
		}
		return r
	}

	if r.completionConfig == nil {
		r.completionConfig = &taskrunner.CompletionConfig{}
	}
	r.completionConfig.OnFinished = jobRef
	return r
}

// OnFailed sets a job to dispatch when the task fails.
func (r *TaskRunner) OnFailed(jobType string, payload any) *TaskRunner {
	jobRef, err := taskrunner.NewJobRef(jobType, payload)
	if err != nil {
		if r.logger != nil {
			r.logger.Error().Err(err).Msg("Failed to create job ref for OnFailed")
		}
		return r
	}

	if r.completionConfig == nil {
		r.completionConfig = &taskrunner.CompletionConfig{}
	}
	r.completionConfig.OnFailed = jobRef
	return r
}

// OnTimeout sets a job to dispatch when the task times out.
func (r *TaskRunner) OnTimeout(jobType string, payload any) *TaskRunner {
	jobRef, err := taskrunner.NewJobRef(jobType, payload)
	if err != nil {
		if r.logger != nil {
			r.logger.Error().Err(err).Msg("Failed to create job ref for OnTimeout")
		}
		return r
	}

	if r.completionConfig == nil {
		r.completionConfig = &taskrunner.CompletionConfig{}
	}
	r.completionConfig.OnTimeout = jobRef
	return r
}

// Run executes the task synchronously and returns the result.
func (r *TaskRunner) Run(ctx context.Context) (*TaskRunnerResult, error) {
	conn, err := r.getConnection()
	if err != nil {
		return nil, err
	}

	if r.dispatcher == nil {
		return nil, fmt.Errorf("no dispatcher set: call WithDispatcher() first")
	}

	var taskModel *models.Task
	if r.trackInDB && r.db != nil {
		taskModel, err = r.createTaskModel()
		if err != nil {
			return nil, fmt.Errorf("failed to create task model: %w", err)
		}
		r.db.Model(taskModel).Update("status", string(TaskStatusRunning))
	}

	pendingTask := taskrunner.NewPendingTask(r.task)
	pendingTask.OnConnection(conn)

	if taskModel != nil {
		pendingTask.As("task-" + taskModel.ID)
	} else {
		pendingTask.As("task-" + ulid.Make().String())
	}

	taskResult, err := r.dispatcher.Run(ctx, pendingTask)

	result := &TaskRunnerResult{
		TaskModel:  taskModel,
		TaskResult: taskResult,
		Error:      err,
	}

	if taskModel != nil && taskResult != nil {
		r.updateTaskModel(taskModel, taskResult)
	}

	if err != nil && r.throwOnError {
		return result, err
	}

	if taskResult != nil && !taskResult.IsSuccessful() && r.throwOnError {
		return result, fmt.Errorf("task '%s' failed with exit code %d: %s",
			r.task.Name(), taskResult.ExitCode, taskResult.Output)
	}

	return result, nil
}

// Dispatch is an alias for Run (for compatibility).
func (r *TaskRunner) Dispatch(ctx context.Context) (*TaskRunnerResult, error) {
	return r.Run(ctx)
}

// RunAsync executes the task asynchronously in a goroutine.
func (r *TaskRunner) RunAsync(ctx context.Context) (*models.Task, error) {
	r.trackInDB = true

	conn, err := r.getConnection()
	if err != nil {
		return nil, err
	}

	if r.dispatcher == nil {
		return nil, fmt.Errorf("no dispatcher set")
	}

	if r.db == nil {
		return nil, fmt.Errorf("database required for async tasks")
	}

	taskModel, err := r.createTaskModel()
	if err != nil {
		return nil, fmt.Errorf("failed to create task model: %w", err)
	}

	go func() {
		r.db.Model(taskModel).Update("status", string(TaskStatusRunning))

		pendingTask := taskrunner.NewPendingTask(r.task)
		pendingTask.OnConnection(conn)
		pendingTask.As("task-" + taskModel.ID)

		bgCtx := context.Background()
		taskResult, err := r.dispatcher.Run(bgCtx, pendingTask)

		if err != nil && r.logger != nil {
			r.logger.Error().Err(err).
				Str("task_id", taskModel.ID).
				Str("task_name", taskModel.Name).
				Msg("Async task execution failed")
		}

		if taskResult != nil {
			r.updateTaskModel(taskModel, taskResult)
		} else if err != nil {
			errStr := err.Error()
			r.db.Model(taskModel).Updates(map[string]any{
				"status": string(TaskStatusFailed),
				"output": errStr,
			})
		}

		r.sendCallbacks(taskModel, taskResult, err)
	}()

	return taskModel, nil
}

// RunInBackground executes the task in the background.
// The execution mode is automatically determined:
//   - If callback URLs are configured: Uses HTTP callbacks (production mode)
//   - If no callback URLs: Uses long-running SSH connection (local/dev mode)
func (r *TaskRunner) RunInBackground(ctx context.Context) (*models.Task, error) {
	r.trackInDB = true

	if r.dispatcher == nil {
		return nil, fmt.Errorf("no dispatcher set")
	}

	if r.db == nil {
		return nil, fmt.Errorf("database required for background tasks")
	}

	if r.hasCallbackURLs() {
		return r.runWithCallbacks(ctx)
	}

	return r.runLongRunning(ctx)
}

func (r *TaskRunner) hasCallbackURLs() bool {
	return r.callbackURLs != nil && r.callbackURLs.FinishedURL != ""
}

func (r *TaskRunner) runWithCallbacks(ctx context.Context) (*models.Task, error) {
	conn, err := r.getConnection()
	if err != nil {
		return nil, err
	}

	taskModel, err := r.createTaskModel()
	if err != nil {
		return nil, fmt.Errorf("failed to create task model: %w", err)
	}

	wrappedScript := r.wrapTaskForBackground(taskModel)
	wrappedTask := taskrunner.NewBaseTask(
		taskrunner.WithName(r.task.Name()+" (Background)"),
		taskrunner.WithScript(wrappedScript),
		taskrunner.WithTimeout(r.task.Timeout()+30*time.Second),
	)

	pendingTask := taskrunner.NewPendingTask(wrappedTask)
	pendingTask.OnConnection(conn)
	pendingTask.InBackground()
	pendingTask.As("task-" + taskModel.ID)

	_, err = r.dispatcher.Run(ctx, pendingTask)
	if err != nil {
		r.db.Model(taskModel).Update("status", string(TaskStatusFailed))
		return taskModel, err
	}

	r.db.Model(taskModel).Update("status", string(TaskStatusRunning))

	if r.logger != nil {
		r.logger.Info().
			Str("task_id", taskModel.ID).
			Str("task_name", taskModel.Name).
			Str("mode", "callback").
			Msg("Task started in background with callbacks")
	}

	return taskModel, nil
}

func (r *TaskRunner) runLongRunning(ctx context.Context) (*models.Task, error) {
	conn, err := r.getConnection()
	if err != nil {
		return nil, err
	}

	taskModel, err := r.createTaskModel()
	if err != nil {
		return nil, fmt.Errorf("failed to create task model: %w", err)
	}

	if r.logger != nil {
		r.logger.Info().
			Str("task_id", taskModel.ID).
			Str("task_name", taskModel.Name).
			Str("mode", "long_running").
			Msg("Task started with long-running SSH connection")
	}

	go func() {
		r.db.Model(taskModel).Update("status", string(TaskStatusRunning))

		pendingTask := taskrunner.NewPendingTask(r.task)
		pendingTask.OnConnection(conn)
		pendingTask.As("task-" + taskModel.ID)

		bgCtx := context.Background()
		taskResult, execErr := r.dispatcher.Run(bgCtx, pendingTask)

		if execErr != nil && r.logger != nil {
			r.logger.Error().Err(execErr).
				Str("task_id", taskModel.ID).
				Str("task_name", taskModel.Name).
				Msg("Long-running task execution failed")
		}

		if taskResult != nil {
			r.updateTaskModel(taskModel, taskResult)
		} else if execErr != nil {
			errStr := execErr.Error()
			r.db.Model(taskModel).Updates(map[string]any{
				"status": string(TaskStatusFailed),
				"output": errStr,
			})
		}

		r.dispatchCompletionJobs(taskResult, execErr)

		if r.logger != nil {
			status := "unknown"
			if taskResult != nil {
				if taskResult.IsSuccessful() {
					status = "finished"
				} else if taskResult.TimedOut {
					status = "timeout"
				} else {
					status = "failed"
				}
			}
			r.logger.Info().
				Str("task_id", taskModel.ID).
				Str("task_name", taskModel.Name).
				Str("status", status).
				Msg("Long-running task completed")
		}
	}()

	return taskModel, nil
}

func (r *TaskRunner) dispatchCompletionJobs(result *taskrunner.TaskResult, execErr error) {
	config := r.completionConfig
	if config == nil {
		if extracted, err := taskrunner.ExtractCompletionConfig(r.task); err == nil {
			config = extracted
		}
	}

	if config == nil {
		return
	}

	if r.queue == nil {
		if r.logger != nil {
			r.logger.Warn().Msg("Queue client not available, cannot dispatch completion job")
		}
		return
	}

	var jobRef *taskrunner.JobRef
	if result != nil {
		if result.IsSuccessful() {
			jobRef = config.OnFinished
		} else if result.TimedOut {
			jobRef = config.OnTimeout
		} else {
			jobRef = config.OnFailed
		}
	} else if execErr != nil {
		jobRef = config.OnFailed
	}

	if jobRef == nil || jobRef.Type == "" {
		return
	}

	asynqTask := asynq.NewTask(jobRef.Type, jobRef.Payload)
	if _, err := r.queue.Enqueue(asynqTask); err != nil {
		if r.logger != nil {
			r.logger.Error().Err(err).
				Str("job_type", jobRef.Type).
				Msg("Failed to dispatch completion job")
		}
	} else if r.logger != nil {
		r.logger.Info().
			Str("job_type", jobRef.Type).
			Msg("Dispatched completion job")
	}
}

func (r *TaskRunner) getConnection() (*taskrunner.Connection, error) {
	if r.server.PublicIPv4 == nil || *r.server.PublicIPv4 == "" {
		return nil, fmt.Errorf("server has no public IP address")
	}

	if r.server.PrivateKey.IsEmpty() {
		return nil, fmt.Errorf("server has no private key")
	}

	if r.asRoot {
		return r.server.ConnectionAsRoot(), nil
	}

	if r.username != "" {
		return r.server.ConnectionAsUser(r.username), nil
	}

	return r.server.ConnectionAsUser(), nil
}

func (r *TaskRunner) createTaskModel() (*models.Task, error) {
	script := r.task.Script()
	taskType := getTaskTypeName(r.task)

	user := r.server.GetUsername()
	if r.asRoot {
		user = r.server.RootUsername()
	} else if r.username != "" {
		user = r.username
	}

	taskModel := &models.Task{
		ServerID: r.server.ID,
		Name:     r.task.Name(),
		User:     user,
		Type:     taskType,
		Script:   script,
		Timeout:  int(r.task.Timeout().Seconds()),
		Status:   string(TaskStatusPending),
	}

	completionConfig := r.completionConfig

	if completionConfig == nil {
		if extracted, err := taskrunner.ExtractCompletionConfig(r.task); err == nil && extracted != nil {
			completionConfig = extracted
		}
	}

	if completionConfig != nil {
		instance, err := taskrunner.MarshalCompletionConfig(completionConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal completion config: %w", err)
		}
		taskModel.Instance = &instance
	} else if callbackTask, ok := r.task.(taskrunner.CallbackPayload); ok {
		instance, err := taskrunner.MarshalInstance(callbackTask)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal callback payload: %w", err)
		}
		taskModel.Instance = &instance
	}

	if err := r.db.Create(taskModel).Error; err != nil {
		return nil, err
	}

	return taskModel, nil
}

func (r *TaskRunner) updateTaskModel(taskModel *models.Task, result *taskrunner.TaskResult) {
	updates := map[string]any{}

	updates["output"] = result.Output
	updates["exit_code"] = result.ExitCode

	if result.TimedOut {
		updates["status"] = string(TaskStatusTimeout)
	} else if result.IsSuccessful() {
		updates["status"] = string(TaskStatusFinished)
	} else {
		updates["status"] = string(TaskStatusFailed)
	}

	r.db.Model(taskModel).Updates(updates)
}

func (r *TaskRunner) sendCallbacks(taskModel *models.Task, result *taskrunner.TaskResult, err error) {
	if r.callbackURLs == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var url string
	if result != nil {
		if result.TimedOut {
			url = r.callbackURLs.TimeoutURL
		} else if result.IsSuccessful() {
			url = r.callbackURLs.FinishedURL
		} else {
			url = r.callbackURLs.FailedURL
		}
	} else if err != nil {
		url = r.callbackURLs.FailedURL
	}

	if url == "" {
		return
	}

	req, _ := http.NewRequestWithContext(ctx, "POST", url, nil)
	client := &http.Client{Timeout: 15 * time.Second}
	client.Do(req)
}

func (r *TaskRunner) wrapTaskForBackground(taskModel *models.Task) string {
	actualScript := r.task.Script()
	timeout := r.task.Timeout()

	var timeoutCmd string
	if timeout > 0 {
		timeoutCmd = fmt.Sprintf("timeout %ds ", int(timeout.Seconds()))
	}

	finishedURL := ""
	failedURL := ""
	timeoutURL := ""
	if r.callbackURLs != nil {
		finishedURL = r.callbackURLs.FinishedURL
		failedURL = r.callbackURLs.FailedURL
		timeoutURL = r.callbackURLs.TimeoutURL
	}

	return fmt.Sprintf(`#!/bin/bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

%s

DIRECTORY=$(dirname "$0")
FILENAME=$(basename "$0")
EXT="${FILENAME##*.}"
PATH_ACTUAL_SCRIPT="$DIRECTORY/${FILENAME%%.*}-original.$EXT"

cat > $PATH_ACTUAL_SCRIPT << 'TASK_EOF'
%s
TASK_EOF

%sbash $PATH_ACTUAL_SCRIPT
EXIT_CODE=$?

if [[ $EXIT_CODE -eq 0 ]]; then
    httpPostSilently "%s"
elif [[ $EXIT_CODE -eq 124 ]]; then
    httpPostSilently "%s"
else
    httpPostSilently "%s" "{\"exit_code\":$EXIT_CODE}"
fi

exit $EXIT_CODE
`, taskrunner.CommonFunctions(), strings.TrimSpace(actualScript), timeoutCmd, finishedURL, timeoutURL, failedURL)
}

func getTaskTypeName(task taskrunner.Task) string {
	t := reflect.TypeOf(task)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.PkgPath() + "." + t.Name()
}

// TaskRunnerDeps holds dependencies for creating TaskRunners.
type TaskRunnerDeps struct {
	DB         *gorm.DB
	Queue      *queue.Client
	Dispatcher *taskrunner.Dispatcher
	Logger     *zerolog.Logger
}

// NewRunner creates a new TaskRunner with dependencies pre-configured.
func (d *TaskRunnerDeps) NewRunner(server *models.Server, task taskrunner.Task) *TaskRunner {
	return NewTaskRunner(server, task).
		WithDB(d.DB).
		WithQueue(d.Queue).
		WithDispatcher(d.Dispatcher).
		WithLogger(d.Logger)
}

// RunTask is a convenience function to run a task on a server synchronously.
func (d *TaskRunnerDeps) RunTask(ctx context.Context, server *models.Server, task taskrunner.Task, asRoot bool) (*TaskRunnerResult, error) {
	runner := d.NewRunner(server, task)
	if asRoot {
		runner.AsRoot()
	} else {
		runner.AsUser()
	}
	return runner.Run(ctx)
}
