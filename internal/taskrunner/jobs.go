package taskrunner

import (
	"context"
	"encoding/json"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/websocket"
)

// Job types
const (
	TypeRunTask          = "task:run"
	TypeUpdateTaskOutput = "task:update_output"
	TypeCheckTaskStatus  = "task:check_status"
)

// TaskJobHandler handles task-related background jobs
type TaskJobHandler struct {
	db         *gorm.DB
	repo       *TaskRepository
	dispatcher *Dispatcher
	ws         *websocket.Hub
	logger     *zerolog.Logger
}

func NewTaskJobHandler(db *gorm.DB, ws *websocket.Hub, logger *zerolog.Logger) *TaskJobHandler {
	return &TaskJobHandler{
		db:         db,
		repo:       NewTaskRepository(db),
		dispatcher: NewDispatcher(logger, ws),
		ws:         ws,
		logger:     logger,
	}
}

// RunTaskPayload contains data for running a task
type RunTaskPayload struct {
	TaskID     string `json:"task_id"`
	ServerID   string `json:"server_id"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	User       string `json:"user"`
	PrivateKey string `json:"private_key"`
	Script     string `json:"script"`
	Timeout    int    `json:"timeout"`
	Background bool   `json:"background"`
}

// NewRunTaskJob creates a new run task job
func NewRunTaskJob(payload RunTaskPayload) (*asynq.Task, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeRunTask, data), nil
}

// HandleRunTask executes a task on a remote server
func (h *TaskJobHandler) HandleRunTask(ctx context.Context, t *asynq.Task) error {
	var payload RunTaskPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return err
	}

	h.logger.Info().
		Str("task_id", payload.TaskID).
		Str("server_id", payload.ServerID).
		Msg("Running task")

	// Update task status to running
	h.repo.UpdateStatus(payload.TaskID, TaskStatusRunning)
	h.broadcastTaskUpdate(payload.TaskID, TaskStatusRunning, "Task started")

	// Create connection
	conn := &Connection{
		Host:       payload.Host,
		Port:       payload.Port,
		User:       payload.User,
		PrivateKey: payload.PrivateKey,
	}

	// Create task wrapper
	task := &BaseTask{
		TaskName:    "remote-task",
		TaskTimeout: time.Duration(payload.Timeout) * time.Second,
		Template:    payload.Script,
		Data:        make(map[string]interface{}),
		OutputCallback: func(output string) {
			h.repo.UpdateOutput(payload.TaskID, output)
			h.broadcastTaskOutput(payload.TaskID, output)
		},
	}

	// Create pending task
	pt := NewPendingTask(task).
		OnConnection(conn).
		WithID(payload.TaskID)

	if payload.Background {
		pt.InBackground()
	}

	// Execute
	result, err := h.dispatcher.Run(ctx, pt)
	if err != nil {
		h.logger.Error().Err(err).Str("task_id", payload.TaskID).Msg("Task execution failed")
		h.repo.UpdateResult(payload.TaskID, TaskStatusFailed, 1, err.Error())
		h.broadcastTaskUpdate(payload.TaskID, TaskStatusFailed, err.Error())
		return err
	}

	// Update result
	status := TaskStatusFinished
	if result.TimedOut {
		status = TaskStatusTimeout
	} else if result.ExitCode != 0 {
		status = TaskStatusFailed
	}

	h.repo.UpdateResult(payload.TaskID, status, result.ExitCode, result.Output)
	h.broadcastTaskUpdate(payload.TaskID, status, result.Output)

	return nil
}

// UpdateTaskOutputPayload contains data for updating task output
type UpdateTaskOutputPayload struct {
	TaskID     string `json:"task_id"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	User       string `json:"user"`
	PrivateKey string `json:"private_key"`
	RetryCount int    `json:"retry_count"`
}

// NewUpdateTaskOutputJob creates a job to fetch and update task output
func NewUpdateTaskOutputJob(payload UpdateTaskOutputPayload) (*asynq.Task, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeUpdateTaskOutput, data), nil
}

// HandleUpdateTaskOutput fetches output from remote server and updates database
func (h *TaskJobHandler) HandleUpdateTaskOutput(ctx context.Context, t *asynq.Task) error {
	var payload UpdateTaskOutputPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return err
	}

	conn := &Connection{
		Host:       payload.Host,
		Port:       payload.Port,
		User:       payload.User,
		PrivateKey: payload.PrivateKey,
	}

	output, err := h.dispatcher.GetTaskOutput(ctx, conn, payload.TaskID)
	if err != nil {
		h.logger.Warn().Err(err).Str("task_id", payload.TaskID).Msg("Failed to get task output")
		// Don't fail the job - output might not be ready yet
		return nil
	}

	// Update output in database
	h.repo.UpdateOutput(payload.TaskID, output)
	h.broadcastTaskOutput(payload.TaskID, output)

	return nil
}

// CheckTaskStatusPayload contains data for checking background task status
type CheckTaskStatusPayload struct {
	TaskID     string `json:"task_id"`
	PID        string `json:"pid"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	User       string `json:"user"`
	PrivateKey string `json:"private_key"`
}

// NewCheckTaskStatusJob creates a job to check if a background task is complete
func NewCheckTaskStatusJob(payload CheckTaskStatusPayload) (*asynq.Task, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeCheckTaskStatus, data), nil
}

// HandleCheckTaskStatus checks if a background task has completed
func (h *TaskJobHandler) HandleCheckTaskStatus(ctx context.Context, t *asynq.Task) error {
	var payload CheckTaskStatusPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return err
	}

	conn := &Connection{
		Host:       payload.Host,
		Port:       payload.Port,
		User:       payload.User,
		PrivateKey: payload.PrivateKey,
	}

	isRunning, exitCode, err := h.dispatcher.CheckTaskStatus(ctx, conn, payload.PID)
	if err != nil {
		return err
	}

	if isRunning {
		// Still running - this job will be retried
		return nil
	}

	// Task completed - fetch final output and update status
	output, _ := h.dispatcher.GetTaskOutput(ctx, conn, payload.TaskID)

	status := TaskStatusFinished
	if exitCode == 124 {
		status = TaskStatusTimeout
	} else if exitCode != 0 {
		status = TaskStatusFailed
	}

	h.repo.UpdateResult(payload.TaskID, status, exitCode, output)
	h.broadcastTaskUpdate(payload.TaskID, status, output)

	return nil
}

// Helper methods
func (h *TaskJobHandler) broadcastTaskUpdate(taskID string, status TaskStatus, message string) {
	h.ws.Broadcast("task."+taskID, "task.updated", map[string]interface{}{
		"task_id": taskID,
		"status":  status,
		"message": message,
	})
}

func (h *TaskJobHandler) broadcastTaskOutput(taskID string, output string) {
	h.ws.Broadcast("task."+taskID, "task.output", map[string]interface{}{
		"task_id": taskID,
		"output":  output,
	})
}
