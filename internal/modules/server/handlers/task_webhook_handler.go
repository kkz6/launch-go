package handlers

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/pkg/util"
	"github.com/kkz6/launch-go/internal/pkg/webhook"
	"github.com/kkz6/launch-go/internal/pkg/queue"
)

// TaskWebhookRepository interface for webhook handler
type TaskWebhookRepository interface {
	FindTaskByID(ctx context.Context, id string) (*models.Task, error)
	UpdateTask(ctx context.Context, task *models.Task) error
}

// TaskWebhookHandler handles task completion callbacks
type TaskWebhookHandler struct {
	webhook.Base
	repo     TaskWebhookRepository
	registry *taskrunner.TaskTypeRegistry
	queue    *queue.Client
	db       *gorm.DB
}

// NewTaskWebhookHandler creates a new webhook handler
func NewTaskWebhookHandler(repo TaskWebhookRepository, secretKey string, queueClient *queue.Client, logger *zerolog.Logger) *TaskWebhookHandler {
	return &TaskWebhookHandler{
		Base:     webhook.NewBase(secretKey, logger),
		repo:     repo,
		registry: taskrunner.DefaultRegistry,
		queue:    queueClient,
	}
}

// WithRegistry sets a custom task type registry
func (h *TaskWebhookHandler) WithRegistry(registry *taskrunner.TaskTypeRegistry) *TaskWebhookHandler {
	h.registry = registry
	return h
}

// WithDB sets the database connection for callback context
func (h *TaskWebhookHandler) WithDB(db *gorm.DB) *TaskWebhookHandler {
	h.db = db
	return h
}

// MarkAsFinished handles successful task completion
func (h *TaskWebhookHandler) MarkAsFinished(c *fiber.Ctx) error {
	taskID := c.Params("id")
	ctx := c.Context()

	if !h.VerifySignature(c) {
		return response.Unauthorized(c, "Invalid signature")
	}

	task, err := h.repo.FindTaskByID(ctx, taskID)
	if err != nil {
		return response.NotFound(c, "Task not found")
	}

	if task.IsFinished() {
		return response.OK(c, "Task already finished", nil)
	}

	task.Status = "finished"
	exitCode := 0
	task.ExitCode = &exitCode

	if err := h.repo.UpdateTask(ctx, task); err != nil {
		return response.InternalError(c, "Failed to update task")
	}

	// Handle callback if task has instance data
	h.handleCallback(ctx, task, taskrunner.CallbackFinished, 0)

	return response.OK(c, "Task marked as finished", nil)
}

// MarkAsFailed handles task failure
func (h *TaskWebhookHandler) MarkAsFailed(c *fiber.Ctx) error {
	taskID := c.Params("id")
	ctx := c.Context()

	if !h.VerifySignature(c) {
		return response.Unauthorized(c, "Invalid signature")
	}

	task, err := h.repo.FindTaskByID(ctx, taskID)
	if err != nil {
		return response.NotFound(c, "Task not found")
	}

	if task.IsFinished() {
		return response.OK(c, "Task already finished", nil)
	}

	var body struct {
		ExitCode int    `json:"exit_code"`
		Output   string `json:"output"`
	}
	c.BodyParser(&body)

	exitCode := body.ExitCode
	if exitCode == 0 {
		exitCode = 1
	}

	task.Status = "failed"
	task.ExitCode = &exitCode
	if body.Output != "" {
		task.Output = basemodels.EncryptedString(body.Output)
	}

	if err := h.repo.UpdateTask(ctx, task); err != nil {
		return response.InternalError(c, "Failed to update task")
	}

	// Handle callback if task has instance data
	h.handleCallback(ctx, task, taskrunner.CallbackFailed, exitCode)

	return response.OK(c, "Task marked as failed", nil)
}

// MarkAsTimeout handles task timeout
func (h *TaskWebhookHandler) MarkAsTimeout(c *fiber.Ctx) error {
	taskID := c.Params("id")
	ctx := c.Context()

	if !h.VerifySignature(c) {
		return response.Unauthorized(c, "Invalid signature")
	}

	task, err := h.repo.FindTaskByID(ctx, taskID)
	if err != nil {
		return response.NotFound(c, "Task not found")
	}

	if task.IsFinished() {
		return response.OK(c, "Task already finished", nil)
	}

	task.Status = "timeout"
	exitCode := 124
	task.ExitCode = &exitCode

	if err := h.repo.UpdateTask(ctx, task); err != nil {
		return response.InternalError(c, "Failed to update task")
	}

	// Handle callback if task has instance data
	h.handleCallback(ctx, task, taskrunner.CallbackTimeout, 124)

	return response.OK(c, "Task marked as timeout", nil)
}

// handleCallback processes the task completion by either:
// 1. Dispatching a job from CompletionConfig (new simple approach)
// 2. Reconstructing and calling a CallbackHandler (legacy approach)
func (h *TaskWebhookHandler) handleCallback(ctx context.Context, task *models.Task, callbackType taskrunner.CallbackType, exitCode int) {
	if task.Instance.IsEmpty() {
		return
	}

	// Try the new CompletionConfig approach first (dispatches asynq jobs)
	if h.handleCompletionConfig(task, callbackType) {
		return
	}

	// Fall back to legacy CallbackPayload approach
	h.handleLegacyCallback(ctx, task, callbackType, exitCode)
}

// handleCompletionConfig dispatches an asynq job based on the completion config
func (h *TaskWebhookHandler) handleCompletionConfig(task *models.Task, callbackType taskrunner.CallbackType) bool {
	config, err := taskrunner.UnmarshalCompletionConfig(task.Instance.String())
	if err != nil || config == nil {
		return false // Not a CompletionConfig, try legacy approach
	}

	// Get the appropriate job ref based on callback type
	var jobRef *taskrunner.JobRef
	switch callbackType {
	case taskrunner.CallbackFinished:
		jobRef = config.OnFinished
	case taskrunner.CallbackFailed:
		jobRef = config.OnFailed
	case taskrunner.CallbackTimeout:
		jobRef = config.OnTimeout
	}

	if jobRef == nil || jobRef.Type == "" {
		return true // Valid config but no job for this callback type
	}

	// Dispatch the asynq job
	if h.queue == nil {
		if h.Logger != nil {
			h.LogWarn("Queue client not available, cannot dispatch completion job",
				"task_id", task.ID,
				"job_type", jobRef.Type,
			)
		}
		return true
	}

	asynqTask := asynq.NewTask(jobRef.Type, jobRef.Payload)
	if _, err := h.queue.Enqueue(asynqTask); err != nil {
		if h.Logger != nil {
			h.LogError(err, "Failed to dispatch completion job",
				"task_id", task.ID,
				"job_type", jobRef.Type,
			)
		}
	} else if h.Logger != nil {
		h.LogInfo("Dispatched completion job",
			"task_id", task.ID,
			"job_type", jobRef.Type,
			"callback_type", string(callbackType),
		)
	}

	return true
}

// handleLegacyCallback reconstructs a CallbackHandler and calls its methods
func (h *TaskWebhookHandler) handleLegacyCallback(ctx context.Context, task *models.Task, callbackType taskrunner.CallbackType, exitCode int) {
	handler, err := taskrunner.ReconstructFromInstance(task.Instance.String())
	if err != nil {
		if h.Logger != nil {
			h.LogError(err, "Failed to reconstruct task callback handler",
				"task_id", task.ID,
				"task_type", task.Type,
			)
		}
		return
	}

	if handler == nil {
		return
	}

	// Create callback context with dependencies
	cbCtx := &taskrunner.CallbackContext{
		DB:     h.db,
		Queue:  h.queue,
		Logger: h.Logger,
	}

	// Execute callback based on type
	var callbackErr error
	switch callbackType {
	case taskrunner.CallbackFinished:
		callbackErr = handler.OnSuccess(ctx, cbCtx, task.ID)
	case taskrunner.CallbackFailed:
		callbackErr = handler.OnFailure(ctx, cbCtx, task.ID, exitCode)
	case taskrunner.CallbackTimeout:
		callbackErr = handler.OnExpired(ctx, cbCtx, task.ID)
	}

	if callbackErr != nil && h.Logger != nil {
		h.LogError(callbackErr, "Task callback handler failed",
			"task_id", task.ID,
			"callback_type", string(callbackType),
		)
	}
}

// GenerateCallbackURLs creates signed URLs for task callbacks
func (h *TaskWebhookHandler) GenerateCallbackURLs(baseURL, taskID string, expireMinutes int) CallbackURLs {
	expireDuration := time.Duration(expireMinutes) * time.Minute

	// Build paths using util for consistent URL construction
	basePath := util.New("").Path("webhooks", "tasks", taskID)
	finishedPath := basePath.Clone().Path("finished").String()
	failedPath := basePath.Clone().Path("failed").String()
	timeoutPath := basePath.Clone().Path("timeout").String()

	// Use the signer with the base URL for generating absolute URLs
	signerWithBase := h.Signer.WithBaseURL(baseURL)

	return CallbackURLs{
		Finished: signerWithBase.SignedURL(finishedPath, nil, expireDuration),
		Failed:   signerWithBase.SignedURL(failedPath, nil, expireDuration),
		Timeout:  signerWithBase.SignedURL(timeoutPath, nil, expireDuration),
	}
}

// CallbackURLs contains the three webhook URLs for task completion
type CallbackURLs struct {
	Finished string
	Failed   string
	Timeout  string
}
