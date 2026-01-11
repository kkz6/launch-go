package taskrunner

import (
	"context"
	"encoding/json"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/taskrunner/repositories"
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
	repo       *repositories.TaskRepository
	dispatcher *Dispatcher
	ws         *websocket.Hub
	logger     *zerolog.Logger
}

// NewTaskJobHandler creates a new task job handler
func NewTaskJobHandler(db *gorm.DB, ws *websocket.Hub, logger *zerolog.Logger) *TaskJobHandler {
	return &TaskJobHandler{
		db:         db,
		repo:       repositories.NewTaskRepository(db),
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

	h.repo.UpdateStatus(payload.TaskID, "running")
	h.broadcastTaskUpdate(payload.TaskID, "running", "Task started")

	conn := &Connection{
		Host:       payload.Host,
		Port:       payload.Port,
		User:       payload.User,
		PrivateKey: payload.PrivateKey,
	}

	task := &BaseTask{
		TaskName:     "remote-task",
		TaskTimeout:  time.Duration(payload.Timeout) * time.Second,
		TemplateName: payload.Script,
		TemplateData: make(map[string]interface{}),
		OutputCallback: func(output string) {
			h.repo.UpdateOutput(payload.TaskID, output)
			h.broadcastTaskOutput(payload.TaskID, output)
		},
	}

	pt := NewPendingTask(task).
		OnConnection(conn).
		WithID(payload.TaskID)

	if payload.Background {
		pt.InBackground()
	}

	result, err := h.dispatcher.Run(ctx, pt)
	if err != nil {
		h.logger.Error().Err(err).Str("task_id", payload.TaskID).Msg("Task execution failed")
		h.repo.UpdateResult(payload.TaskID, "failed", 1, err.Error())
		h.broadcastTaskUpdate(payload.TaskID, "failed", err.Error())
		return err
	}

	status := "finished"
	if result.TimedOut {
		status = "timeout"
	} else if result.ExitCode != 0 {
		status = "failed"
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
		return nil
	}

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
		return nil
	}

	output, _ := h.dispatcher.GetTaskOutput(ctx, conn, payload.TaskID)

	status := "finished"
	if exitCode == 124 {
		status = "timeout"
	} else if exitCode != 0 {
		status = "failed"
	}

	h.repo.UpdateResult(payload.TaskID, status, exitCode, output)
	h.broadcastTaskUpdate(payload.TaskID, status, output)

	return nil
}

func (h *TaskJobHandler) broadcastTaskUpdate(taskID string, status string, message string) {
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
