package handlers

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/jobs"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/queue"
	"github.com/kkz6/launch-go/internal/pkg/signedurl"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/pkg/util"
)

// TaskWebhookRepository interface for webhook handler
type TaskWebhookRepository interface {
	FindTaskByID(ctx context.Context, id string) (*models.Task, error)
	UpdateTask(ctx context.Context, task *models.Task) error
}

// TaskWebhookHandler handles task completion callbacks
type TaskWebhookHandler struct {
	signer   *signedurl.Signer
	logger   *zerolog.Logger
	repo     TaskWebhookRepository
	registry *taskrunner.TaskTypeRegistry
	queue    *queue.Client
	db       *gorm.DB
}

// NewTaskWebhookHandler creates a new webhook handler
func NewTaskWebhookHandler(repo TaskWebhookRepository, secretKey string, queueClient *queue.Client, logger *zerolog.Logger) *TaskWebhookHandler {
	return &TaskWebhookHandler{
		signer:   signedurl.NewSigner(secretKey),
		logger:   logger,
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

	if !signedurl.ValidateSignedURL(c, h.signer) {
		return fiberctx.RespondUnauthorized(c, "Invalid signature")
	}

	task, err := h.repo.FindTaskByID(ctx, taskID)
	if err != nil {
		return fiberctx.RespondNotFound(c, "Task not found")
	}

	if task.IsFinished() {
		return fiberctx.OK(c, "Task already finished", nil)
	}

	task.Status = "finished"
	exitCode := 0
	task.ExitCode = &exitCode

	if err := h.repo.UpdateTask(ctx, task); err != nil {
		return fiberctx.RespondInternalError(c, "Failed to update task")
	}

	// Handle callback if task has instance data
	h.handleCallback(ctx, task, taskrunner.CallbackFinished, 0)

	// Dispatch job to fetch task output from server
	h.dispatchOutputFetch(task)

	return fiberctx.OK(c, "Task marked as finished", nil)
}

// MarkAsFailed handles task failure
func (h *TaskWebhookHandler) MarkAsFailed(c *fiber.Ctx) error {
	taskID := c.Params("id")
	ctx := c.Context()

	if !signedurl.ValidateSignedURL(c, h.signer) {
		return fiberctx.RespondUnauthorized(c, "Invalid signature")
	}

	task, err := h.repo.FindTaskByID(ctx, taskID)
	if err != nil {
		return fiberctx.RespondNotFound(c, "Task not found")
	}

	if task.IsFinished() {
		return fiberctx.OK(c, "Task already finished", nil)
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
		task.Output = dbtype.EncryptedString(body.Output)
	}

	if err := h.repo.UpdateTask(ctx, task); err != nil {
		return fiberctx.RespondInternalError(c, "Failed to update task")
	}

	// Handle callback if task has instance data
	h.handleCallback(ctx, task, taskrunner.CallbackFailed, exitCode)

	// Dispatch job to fetch task output from server
	h.dispatchOutputFetch(task)

	return fiberctx.OK(c, "Task marked as failed", nil)
}

// MarkAsTimeout handles task timeout
func (h *TaskWebhookHandler) MarkAsTimeout(c *fiber.Ctx) error {
	taskID := c.Params("id")
	ctx := c.Context()

	if !signedurl.ValidateSignedURL(c, h.signer) {
		return fiberctx.RespondUnauthorized(c, "Invalid signature")
	}

	task, err := h.repo.FindTaskByID(ctx, taskID)
	if err != nil {
		return fiberctx.RespondNotFound(c, "Task not found")
	}

	if task.IsFinished() {
		return fiberctx.OK(c, "Task already finished", nil)
	}

	task.Status = "timeout"
	exitCode := 124
	task.ExitCode = &exitCode

	if err := h.repo.UpdateTask(ctx, task); err != nil {
		return fiberctx.RespondInternalError(c, "Failed to update task")
	}

	// Handle callback if task has instance data
	h.handleCallback(ctx, task, taskrunner.CallbackTimeout, 124)

	// Dispatch job to fetch task output from server
	h.dispatchOutputFetch(task)

	return fiberctx.OK(c, "Task marked as timeout", nil)
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
		if h.logger != nil {
			h.logger.Warn().
				Str("task_id", task.ID).
				Str("job_type", jobRef.Type).
				Msg("Queue client not available, cannot dispatch completion job")
		}
		return true
	}

	asynqTask := asynq.NewTask(jobRef.Type, jobRef.Payload)
	if _, err := h.queue.Enqueue(asynqTask); err != nil {
		if h.logger != nil {
			h.logger.Error().Err(err).
				Str("task_id", task.ID).
				Str("job_type", jobRef.Type).
				Msg("Failed to dispatch completion job")
		}
	} else if h.logger != nil {
		h.logger.Info().
			Str("task_id", task.ID).
			Str("job_type", jobRef.Type).
			Str("callback_type", string(callbackType)).
			Msg("Dispatched completion job")
	}

	return true
}

// handleLegacyCallback reconstructs a CallbackHandler and calls its methods
func (h *TaskWebhookHandler) handleLegacyCallback(ctx context.Context, task *models.Task, callbackType taskrunner.CallbackType, exitCode int) {
	handler, err := taskrunner.ReconstructFromInstance(task.Instance.String())
	if err != nil {
		if h.logger != nil {
			h.logger.Error().Err(err).
				Str("task_id", task.ID).
				Str("task_type", task.Type).
				Msg("Failed to reconstruct task callback handler")
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
		Logger: h.logger,
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

	if callbackErr != nil && h.logger != nil {
		h.logger.Error().Err(callbackErr).
			Str("task_id", task.ID).
			Str("callback_type", string(callbackType)).
			Msg("Task callback handler failed")
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
	customPath := basePath.Clone().Path("callback").String()

	// Use the signer with the base URL for generating absolute URLs
	signerWithBase := h.signer.WithBaseURL(baseURL)

	return CallbackURLs{
		Finished: signerWithBase.SignedURL(finishedPath, nil, expireDuration),
		Failed:   signerWithBase.SignedURL(failedPath, nil, expireDuration),
		Timeout:  signerWithBase.SignedURL(timeoutPath, nil, expireDuration),
		Custom:   signerWithBase.SignedURL(customPath, nil, expireDuration),
	}
}

// CallbackURLs contains the four webhook URLs for task completion
type CallbackURLs struct {
	Finished string
	Failed   string
	Timeout  string
	Custom   string // For progress updates from script
}

// CustomCallback handles custom callbacks from running tasks (for progress updates)
// This can be called by scripts during execution to trigger output fetch or custom logic
func (h *TaskWebhookHandler) CustomCallback(c *fiber.Ctx) error {
	taskID := c.Params("id")
	ctx := c.Context()

	if !signedurl.ValidateSignedURL(c, h.signer) {
		return fiberctx.RespondUnauthorized(c, "Invalid signature")
	}

	task, err := h.repo.FindTaskByID(ctx, taskID)
	if err != nil {
		return fiberctx.RespondNotFound(c, "Task not found")
	}

	// Only allow callbacks for pending/running tasks
	if task.Status != "pending" && task.Status != "running" {
		return fiberctx.OK(c, "Task already completed", nil)
	}

	// Fetch the latest output from the server
	h.dispatchOutputFetch(task)

	// Handle callback if task has instance data
	h.handleCallback(ctx, task, taskrunner.CallbackCustom, 0)

	return fiberctx.OK(c, "Callback processed", nil)
}

// dispatchOutputFetch dispatches a job to fetch the task output from the server
func (h *TaskWebhookHandler) dispatchOutputFetch(task *models.Task) {
	if h.queue == nil {
		return
	}

	// Get server ID from task
	serverID := task.ServerID
	if serverID == "" {
		return
	}

	asynqTask, err := jobs.NewFetchTaskOutputTask(task.ID, serverID, 0) // 0 = no reschedule
	if err != nil {
		if h.logger != nil {
			h.logger.Error().Err(err).Str("task_id", task.ID).Msg("Failed to create fetch output task")
		}
		return
	}

	if _, err := h.queue.Enqueue(asynqTask); err != nil {
		if h.logger != nil {
			h.logger.Error().Err(err).Str("task_id", task.ID).Msg("Failed to dispatch fetch output job")
		}
	} else if h.logger != nil {
		h.logger.Info().Str("task_id", task.ID).Msg("Dispatched output fetch job")
	}
}
