package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/signedurl"
)

// TaskWebhookRepository interface for webhook handler
type TaskWebhookRepository interface {
	FindTaskByID(ctx context.Context, id string) (*models.Task, error)
	UpdateTask(ctx context.Context, task *models.Task) error
}

// TaskWebhookHandler handles task completion callbacks
type TaskWebhookHandler struct {
	repo   TaskWebhookRepository
	signer *signedurl.Signer
}

// NewTaskWebhookHandler creates a new webhook handler
func NewTaskWebhookHandler(repo TaskWebhookRepository, secretKey string) *TaskWebhookHandler {
	return &TaskWebhookHandler{
		repo:   repo,
		signer: signedurl.NewSigner(secretKey),
	}
}

// RegisterTaskWebhookRoutes registers webhook routes
func (h *TaskWebhookHandler) RegisterRoutes(router fiber.Router) {
	webhooks := router.Group("/webhooks/tasks")
	webhooks.Post("/:id/finished", h.MarkAsFinished)
	webhooks.Post("/:id/failed", h.MarkAsFailed)
	webhooks.Post("/:id/timeout", h.MarkAsTimeout)
}

// MarkAsFinished handles successful task completion
func (h *TaskWebhookHandler) MarkAsFinished(c *fiber.Ctx) error {
	taskID := c.Params("id")
	ctx := c.Context()

	if !h.verifySignature(c) {
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

	return response.OK(c, "Task marked as finished", nil)
}

// MarkAsFailed handles task failure
func (h *TaskWebhookHandler) MarkAsFailed(c *fiber.Ctx) error {
	taskID := c.Params("id")
	ctx := c.Context()

	if !h.verifySignature(c) {
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
		task.Output = &body.Output
	}

	if err := h.repo.UpdateTask(ctx, task); err != nil {
		return response.InternalError(c, "Failed to update task")
	}

	return response.OK(c, "Task marked as failed", nil)
}

// MarkAsTimeout handles task timeout
func (h *TaskWebhookHandler) MarkAsTimeout(c *fiber.Ctx) error {
	taskID := c.Params("id")
	ctx := c.Context()

	if !h.verifySignature(c) {
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

	return response.OK(c, "Task marked as timeout", nil)
}

// verifySignature validates the webhook signature using signedurl package
func (h *TaskWebhookHandler) verifySignature(c *fiber.Ctx) bool {
	return signedurl.ValidateSignedURL(c, h.signer)
}

// GenerateCallbackURLs creates signed URLs for task callbacks
func (h *TaskWebhookHandler) GenerateCallbackURLs(baseURL, taskID string, expireMinutes int) CallbackURLs {
	expireDuration := time.Duration(expireMinutes) * time.Minute

	finishedPath := fmt.Sprintf("/webhooks/tasks/%s/finished", taskID)
	failedPath := fmt.Sprintf("/webhooks/tasks/%s/failed", taskID)
	timeoutPath := fmt.Sprintf("/webhooks/tasks/%s/timeout", taskID)

	// Use the signer with the base URL for generating absolute URLs
	signerWithBase := h.signer.WithBaseURL(baseURL)

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
