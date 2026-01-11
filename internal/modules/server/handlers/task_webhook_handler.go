package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

// TaskWebhookRepository interface for webhook handler
type TaskWebhookRepository interface {
	FindTaskByID(ctx context.Context, id string) (*models.Task, error)
	UpdateTask(ctx context.Context, task *models.Task) error
}

// TaskWebhookHandler handles task completion callbacks
type TaskWebhookHandler struct {
	repo      TaskWebhookRepository
	secretKey string
}

// NewTaskWebhookHandler creates a new webhook handler
func NewTaskWebhookHandler(repo TaskWebhookRepository, secretKey string) *TaskWebhookHandler {
	return &TaskWebhookHandler{
		repo:      repo,
		secretKey: secretKey,
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

	if !h.verifySignature(c, taskID) {
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

	if !h.verifySignature(c, taskID) {
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

	if !h.verifySignature(c, taskID) {
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

// verifySignature validates the webhook signature
func (h *TaskWebhookHandler) verifySignature(c *fiber.Ctx, taskID string) bool {
	signature := c.Query("signature")
	expires := c.Query("expires")

	if signature == "" || expires == "" {
		return false
	}

	expiresInt, err := strconv.ParseInt(expires, 10, 64)
	if err != nil {
		return false
	}

	if time.Now().Unix() > expiresInt {
		return false
	}

	expected := h.generateSignature(taskID, expires)
	return hmac.Equal([]byte(signature), []byte(expected))
}

// generateSignature creates HMAC signature for webhook URL
func (h *TaskWebhookHandler) generateSignature(taskID, expires string) string {
	data := fmt.Sprintf("%s:%s", taskID, expires)
	mac := hmac.New(sha256.New, []byte(h.secretKey))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

// GenerateCallbackURLs creates signed URLs for task callbacks
func (h *TaskWebhookHandler) GenerateCallbackURLs(baseURL, taskID string, expireMinutes int) CallbackURLs {
	expires := strconv.FormatInt(time.Now().Add(time.Duration(expireMinutes)*time.Minute).Unix(), 10)
	signature := h.generateSignature(taskID, expires)

	params := url.Values{}
	params.Set("signature", signature)
	params.Set("expires", expires)
	query := params.Encode()

	return CallbackURLs{
		Finished: fmt.Sprintf("%s/webhooks/tasks/%s/finished?%s", baseURL, taskID, query),
		Failed:   fmt.Sprintf("%s/webhooks/tasks/%s/failed?%s", baseURL, taskID, query),
		Timeout:  fmt.Sprintf("%s/webhooks/tasks/%s/timeout?%s", baseURL, taskID, query),
	}
}

// CallbackURLs contains the three webhook URLs for task completion
type CallbackURLs struct {
	Finished string
	Failed   string
	Timeout  string
}
