package tasks

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/oklog/ulid/v2"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
	"github.com/kkz6/launch-go/internal/pkg/queue"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
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
	dispatcher       taskrunner.TaskDispatcher
	logger           *zerolog.Logger
	broadcaster      broadcast.TeamBroadcaster
	notifier         taskrunner.NotifierService
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
func (r *TaskRunner) WithDispatcher(dispatcher taskrunner.TaskDispatcher) *TaskRunner {
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

// WithBroadcaster sets the broadcaster for task events.
func (r *TaskRunner) WithBroadcaster(b broadcast.TeamBroadcaster) *TaskRunner {
	r.broadcaster = b
	return r
}

// WithNotifier sets the notifier service for sending notifications.
func (r *TaskRunner) WithNotifier(n taskrunner.NotifierService) *TaskRunner {
	r.notifier = n
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
		r.broadcastTaskRunning(taskModel)
	}

	pendingTask := taskrunner.NewPendingTask(r.task)
	pendingTask.OnConnection(conn)

	if taskModel != nil {
		pendingTask.As("task-" + taskModel.ID)
	} else {
		pendingTask.As("task-" + ulid.Make().String())
	}

	taskResult, execErr := r.dispatcher.Run(ctx, pendingTask)

	result := &TaskRunnerResult{
		TaskModel:  taskModel,
		TaskResult: taskResult,
		Error:      execErr,
	}

	if taskModel != nil && taskResult != nil {
		r.updateTaskModel(taskModel, taskResult)
	}

	// Always invoke task callbacks regardless of execution mode
	if taskModel != nil {
		r.handleTaskCompletion(ctx, taskModel, taskResult, execErr)
	}

	if execErr != nil && r.throwOnError {
		return result, execErr
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
		r.broadcastTaskRunning(taskModel)

		pendingTask := taskrunner.NewPendingTask(r.task)
		pendingTask.OnConnection(conn)
		pendingTask.As("task-" + taskModel.ID)

		bgCtx := context.Background()
		taskResult, execErr := r.dispatcher.Run(bgCtx, pendingTask)

		if execErr != nil && r.logger != nil {
			r.logger.Error().Err(execErr).
				Str("task_id", taskModel.ID).
				Str("task_name", taskModel.Name).
				Msg("Async task execution failed")
		}

		if taskResult != nil {
			r.updateTaskModel(taskModel, taskResult)
		} else if execErr != nil {
			taskModel.Status = string(TaskStatusFailed)
			taskModel.Output = basemodels.EncryptedString(execErr.Error())
			r.db.Save(taskModel)
			// Broadcast failure
			r.broadcastTaskEvent("task.updated", taskModel, execErr.Error())
		}

		// Always invoke task callbacks
		r.handleTaskCompletion(bgCtx, taskModel, taskResult, execErr)
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
		r.broadcastTaskEvent("task.updated", taskModel, err.Error())
		return taskModel, err
	}

	r.db.Model(taskModel).Update("status", string(TaskStatusRunning))
	r.broadcastTaskRunning(taskModel)

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
		r.broadcastTaskRunning(taskModel)

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
			taskModel.Status = string(TaskStatusFailed)
			taskModel.Output = basemodels.EncryptedString(execErr.Error())
			r.db.Save(taskModel)
			// Broadcast failure
			r.broadcastTaskEvent("task.updated", taskModel, execErr.Error())
		}

		// Handle task completion - invokes callbacks and dispatches completion jobs
		r.handleTaskCompletion(bgCtx, taskModel, taskResult, execErr)

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

// handleTaskCompletion handles task completion in local mode.
// It tries both approaches:
// 1. CallbackPayload: Calls OnSuccess/OnFailure/OnExpired on the task itself
// 2. CompletionConfig: Dispatches asynq jobs
func (r *TaskRunner) handleTaskCompletion(ctx context.Context, taskModel *models.Task, result *taskrunner.TaskResult, execErr error) {
	// First, try to invoke task callback methods directly (CallbackPayload approach)
	r.invokeTaskCallbacks(ctx, taskModel, result, execErr)

	// Then, dispatch completion jobs if configured (CompletionConfig approach)
	r.dispatchCompletionJobs(result, execErr)
}

// invokeTaskCallbacks calls the task's callback methods if it implements CallbackPayload.
// This is used in local mode where we don't have HTTP callbacks.
func (r *TaskRunner) invokeTaskCallbacks(ctx context.Context, taskModel *models.Task, result *taskrunner.TaskResult, execErr error) {
	// Check if task implements CallbackPayload
	callbackTask, ok := r.task.(taskrunner.CallbackPayload)
	if !ok {
		return
	}

	// Create callback context with dependencies
	cbCtx := &taskrunner.CallbackContext{
		DB:       r.db,
		Queue:    r.queue,
		Logger:   r.logger,
		Notifier: r.notifier,
	}
	cbCtx.SetBroadcaster(r.broadcaster)

	var err error
	if result != nil {
		if result.IsSuccessful() {
			err = callbackTask.OnSuccess(ctx, cbCtx, taskModel.ID)
		} else if result.TimedOut {
			err = callbackTask.OnExpired(ctx, cbCtx, taskModel.ID)
		} else {
			err = callbackTask.OnFailure(ctx, cbCtx, taskModel.ID, result.ExitCode)
		}
	} else if execErr != nil {
		err = callbackTask.OnFailure(ctx, cbCtx, taskModel.ID, 1)
	}

	if err != nil && r.logger != nil {
		r.logger.Error().Err(err).
			Str("task_id", taskModel.ID).
			Str("task_name", taskModel.Name).
			Msg("Task callback handler failed")
	}
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
		ServerScopedModel: basemodels.ServerScopedModel{ServerID: r.server.ID},
		Name:              r.task.Name(),
		User:              user,
		Type:              taskType,
		Script:            basemodels.EncryptedString(script),
		Timeout:           int(r.task.Timeout().Seconds()),
		Status:            string(TaskStatusPending),
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
		taskModel.Instance = basemodels.EncryptedString(instance)
	} else if callbackTask, ok := r.task.(taskrunner.CallbackPayload); ok {
		instance, err := taskrunner.MarshalInstance(callbackTask)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal callback payload: %w", err)
		}
		taskModel.Instance = basemodels.EncryptedString(instance)
	}

	if err := r.db.Create(taskModel).Error; err != nil {
		return nil, err
	}

	// Broadcast task created event
	r.broadcastTaskEvent("task.created", taskModel, "")

	return taskModel, nil
}

func (r *TaskRunner) updateTaskModel(taskModel *models.Task, result *taskrunner.TaskResult) {
	// Update model fields directly to ensure EncryptedString Valuer is used
	taskModel.Output = basemodels.EncryptedString(result.Output)
	taskModel.ExitCode = &result.ExitCode

	if result.TimedOut {
		taskModel.Status = string(TaskStatusTimeout)
	} else if result.IsSuccessful() {
		taskModel.Status = string(TaskStatusFinished)
	} else {
		taskModel.Status = string(TaskStatusFailed)
	}

	r.db.Save(taskModel)

	// Broadcast task updated event with output
	r.broadcastTaskEvent("task.updated", taskModel, result.Output)
}

// broadcastTaskEvent broadcasts a task event to the server's team channel.
func (r *TaskRunner) broadcastTaskEvent(event string, taskModel *models.Task, output string) {
	if r.broadcaster == nil || r.server == nil {
		return
	}

	data := map[string]interface{}{
		"task_id":   taskModel.ID,
		"server_id": taskModel.ServerID,
		"name":      taskModel.Name,
		"status":    taskModel.Status,
		"user":      taskModel.User,
	}

	// Include exit code if available
	if taskModel.ExitCode != nil {
		data["exit_code"] = *taskModel.ExitCode
	}

	// Include output for updated events
	if output != "" {
		data["output"] = output
	}

	r.broadcaster.BroadcastToTeam(r.server.TeamID, event, data)
}

// broadcastTaskRunning broadcasts that a task has started running.
func (r *TaskRunner) broadcastTaskRunning(taskModel *models.Task) {
	if r.broadcaster == nil || r.server == nil {
		return
	}

	r.broadcaster.BroadcastToTeam(r.server.TeamID, "task.running", map[string]interface{}{
		"task_id":   taskModel.ID,
		"server_id": taskModel.ServerID,
		"name":      taskModel.Name,
		"status":    string(TaskStatusRunning),
	})
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
	DB          *gorm.DB
	Queue       *queue.Client
	Dispatcher  taskrunner.TaskDispatcher
	Logger      *zerolog.Logger
	Broadcaster broadcast.TeamBroadcaster
	Notifier    taskrunner.NotifierService
	LocalMode   bool
}

// IsLocalMode returns true if running in local development mode
func (d *TaskRunnerDeps) IsLocalMode() bool {
	return d.LocalMode
}

// NewRunner creates a new TaskRunner with dependencies pre-configured.
func (d *TaskRunnerDeps) NewRunner(server *models.Server, task taskrunner.Task) *TaskRunner {
	return NewTaskRunner(server, task).
		WithDB(d.DB).
		WithQueue(d.Queue).
		WithDispatcher(d.Dispatcher).
		WithLogger(d.Logger).
		WithBroadcaster(d.Broadcaster).
		WithNotifier(d.Notifier)
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
