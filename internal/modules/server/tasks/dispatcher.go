package tasks

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// TaskStatus represents task execution status
type TaskStatus string

const (
	TaskStatusPending      TaskStatus = "pending"
	TaskStatusRunning      TaskStatus = "running"
	TaskStatusFinished     TaskStatus = "finished"
	TaskStatusFailed       TaskStatus = "failed"
	TaskStatusTimeout      TaskStatus = "timeout"
	TaskStatusUploadFailed TaskStatus = "upload_failed"
)

// TaskRepository interface for task persistence
type TaskRepository interface {
	CreateTask(ctx context.Context, task *models.Task) error
	FindTaskByID(ctx context.Context, id string) (*models.Task, error)
	UpdateTask(ctx context.Context, task *models.Task) error
}

// ServerTaskDispatcher wraps PendingTask with server-specific behavior
// It provides a fluent API for configuring and dispatching tasks on a server
type ServerTaskDispatcher struct {
	server      *models.Server
	pendingTask *taskrunner.PendingTask
	dispatcher  *taskrunner.Dispatcher
	taskRepo    TaskRepository
	logger      *zerolog.Logger

	// Options
	keepTrack                  bool
	throwOnFail                bool
	updateLogIntervalInSeconds int
}

// NewServerTaskDispatcher creates a new ServerTaskDispatcher for the given server and task
func NewServerTaskDispatcher(
	server *models.Server,
	task taskrunner.Task,
	dispatcher *taskrunner.Dispatcher,
	taskRepo TaskRepository,
	logger *zerolog.Logger,
) *ServerTaskDispatcher {
	return &ServerTaskDispatcher{
		server:      server,
		pendingTask: taskrunner.NewPendingTask(task),
		dispatcher:  dispatcher,
		taskRepo:    taskRepo,
		logger:      logger,
	}
}

// AsRoot configures the task to run as root user on the server
func (d *ServerTaskDispatcher) AsRoot() *ServerTaskDispatcher {
	conn := d.server.ConnectionAsRoot()
	d.pendingTask.OnConnection(conn)
	return d
}

// AsUser configures the task to run as the specified user (or default server user)
func (d *ServerTaskDispatcher) AsUser(username ...string) *ServerTaskDispatcher {
	var user string
	if len(username) > 0 && username[0] != "" {
		user = username[0]
	}
	conn := d.server.ConnectionAsUser(user)
	d.pendingTask.OnConnection(conn)
	return d
}

// InBackground sets the task to run in background mode
func (d *ServerTaskDispatcher) InBackground() *ServerTaskDispatcher {
	d.pendingTask.InBackground()
	return d
}

// InForeground sets the task to run in foreground mode
func (d *ServerTaskDispatcher) InForeground() *ServerTaskDispatcher {
	d.pendingTask.InForeground()
	return d
}

// KeepTrack enables task tracking in the database
// This creates a Task model and updates it with progress/result
func (d *ServerTaskDispatcher) KeepTrack() *ServerTaskDispatcher {
	d.keepTrack = true
	d.InBackground() // Tracked tasks must run in background
	return d
}

// UpdateLogIntervalInSeconds sets the interval for updating task logs
func (d *ServerTaskDispatcher) UpdateLogIntervalInSeconds(seconds int) *ServerTaskDispatcher {
	d.updateLogIntervalInSeconds = seconds
	d.InBackground() // Log updates require background execution
	return d
}

// Throw configures the dispatcher to return an error if the task fails
func (d *ServerTaskDispatcher) Throw() *ServerTaskDispatcher {
	d.throwOnFail = true
	return d
}

// As sets a custom task ID
func (d *ServerTaskDispatcher) As(id string) *ServerTaskDispatcher {
	d.pendingTask.As(id)
	return d
}

// OnOutput sets a callback for output streaming
func (d *ServerTaskDispatcher) OnOutput(callback func(output string)) *ServerTaskDispatcher {
	d.pendingTask.OnOutput(callback)
	return d
}

// Dispatch executes the task and returns the result
func (d *ServerTaskDispatcher) Dispatch(ctx context.Context) (*DispatchResult, error) {
	// Validate connection is set
	if d.pendingTask.GetConnection() == nil {
		return nil, fmt.Errorf("no connection selected: call AsRoot() or AsUser() first")
	}

	// If tracking is enabled, create task model and use tracking wrapper
	if d.keepTrack && d.taskRepo != nil {
		return d.dispatchAndKeepTrack(ctx)
	}

	// Generate task ID if not set
	if d.pendingTask.GetID() == "" {
		d.pendingTask.As("task-" + generateTaskID())
	}

	// Execute the task
	result, err := d.dispatcher.Run(ctx, d.pendingTask)
	if err != nil {
		return nil, fmt.Errorf("failed to dispatch task: %w", err)
	}

	// Check if we should throw on failure
	if d.throwOnFail && !result.IsSuccessful() {
		return nil, fmt.Errorf("task '%s' failed with exit code %d: %s",
			d.pendingTask.Task.Name(), result.ExitCode, result.Output)
	}

	return &DispatchResult{
		TaskResult: result,
	}, nil
}

// dispatchAndKeepTrack creates a task model and tracks execution
func (d *ServerTaskDispatcher) dispatchAndKeepTrack(ctx context.Context) (*DispatchResult, error) {
	// Create the task model
	taskModel, err := d.createTaskModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create task model: %w", err)
	}

	// Set the task ID from the model
	d.pendingTask.As("task-" + taskModel.ID)

	// Wrap with tracking callbacks
	originalTask := d.pendingTask.Task

	// Set callbacks on the base task if it supports them
	if baseTask, ok := originalTask.(*BaseServerTask); ok {
		baseTask.FinishedCallback = func(ctx context.Context, result *taskrunner.TaskResult) {
			d.updateTaskStatus(ctx, taskModel.ID, TaskStatusFinished, result)
		}
		baseTask.FailedCallback = func(ctx context.Context, result *taskrunner.TaskResult) {
			d.updateTaskStatus(ctx, taskModel.ID, TaskStatusFailed, result)
		}
		baseTask.TimeoutCallback = func(ctx context.Context, result *taskrunner.TaskResult) {
			d.updateTaskStatus(ctx, taskModel.ID, TaskStatusTimeout, result)
		}
	}

	// Execute the task
	result, err := d.dispatcher.Run(ctx, d.pendingTask)
	if err != nil {
		// Update task as upload failed
		d.updateTaskStatus(ctx, taskModel.ID, TaskStatusUploadFailed, nil)
		return &DispatchResult{
			TaskModel: taskModel,
		}, fmt.Errorf("failed to dispatch task: %w", err)
	}

	return &DispatchResult{
		TaskResult: result,
		TaskModel:  taskModel,
	}, nil
}

// createTaskModel creates a new task model in the database
func (d *ServerTaskDispatcher) createTaskModel(ctx context.Context) (*models.Task, error) {
	task := d.pendingTask.Task

	script, err := task.Script()
	if err != nil {
		script = ""
	}

	// Determine the user
	conn := d.pendingTask.GetConnection()
	user := "root"
	if conn != nil {
		user = conn.User
	}

	taskModel := &models.Task{
		ServerID: d.server.ID,
		Name:     task.Name(),
		User:     user,
		Type:     fmt.Sprintf("%T", task),
		Script:   script,
		Timeout:  int(task.Timeout().Seconds()),
		Status:   string(TaskStatusPending),
	}

	if err := d.taskRepo.CreateTask(ctx, taskModel); err != nil {
		return nil, err
	}

	return taskModel, nil
}

// updateTaskStatus updates the task model with the result
func (d *ServerTaskDispatcher) updateTaskStatus(ctx context.Context, taskID string, status TaskStatus, result *taskrunner.TaskResult) {
	if d.taskRepo == nil {
		return
	}

	taskModel, err := d.taskRepo.FindTaskByID(ctx, taskID)
	if err != nil {
		if d.logger != nil {
			d.logger.Error().Err(err).Str("task_id", taskID).Msg("Failed to find task for update")
		}
		return
	}

	taskModel.Status = string(status)

	if result != nil {
		taskModel.Output = &result.Output
		taskModel.ExitCode = &result.ExitCode
	}

	if err := d.taskRepo.UpdateTask(ctx, taskModel); err != nil {
		if d.logger != nil {
			d.logger.Error().Err(err).Str("task_id", taskID).Msg("Failed to update task status")
		}
	}
}

// DispatchResult contains the result of task dispatch
type DispatchResult struct {
	TaskResult *taskrunner.TaskResult
	TaskModel  *models.Task
}

// IsSuccessful returns true if the task completed successfully
func (r *DispatchResult) IsSuccessful() bool {
	if r.TaskResult != nil {
		return r.TaskResult.IsSuccessful()
	}
	if r.TaskModel != nil {
		return r.TaskModel.IsSuccessful()
	}
	return false
}

// generateTaskID generates a unique task ID
func generateTaskID() string {
	return utils.NewULID()
}
