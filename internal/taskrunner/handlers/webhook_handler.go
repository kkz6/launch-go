package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/pkg/response"
)

// TaskRepository interface for webhook handler
type TaskRepository interface {
	FindByID(id string) (Task, error)
	UpdateStatus(id string, status string) error
	UpdateResult(id string, status string, exitCode int, output string) error
}

// Task interface for webhook handler
type Task interface {
	IsFinished() bool
}

// WebhookHandler handles task completion callbacks
type WebhookHandler struct {
	repo      TaskRepository
	secretKey string
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(repo TaskRepository, secretKey string) *WebhookHandler {
	return &WebhookHandler{
		repo:      repo,
		secretKey: secretKey,
	}
}

// MarkAsFinished handles successful task completion
func (h *WebhookHandler) MarkAsFinished(c *fiber.Ctx) error {
	taskID := c.Params("id")

	if !h.verifySignature(c, taskID) {
		return response.Unauthorized(c, "Invalid signature")
	}

	task, err := h.repo.FindByID(taskID)
	if err != nil {
		return response.NotFound(c, "Task not found")
	}

	if task.IsFinished() {
		return response.OK(c, "Task already finished", nil)
	}

	h.repo.UpdateStatus(taskID, "finished")

	return response.OK(c, "Task marked as finished", nil)
}

// MarkAsFailed handles task failure
func (h *WebhookHandler) MarkAsFailed(c *fiber.Ctx) error {
	taskID := c.Params("id")

	if !h.verifySignature(c, taskID) {
		return response.Unauthorized(c, "Invalid signature")
	}

	task, err := h.repo.FindByID(taskID)
	if err != nil {
		return response.NotFound(c, "Task not found")
	}

	if task.IsFinished() {
		return response.OK(c, "Task already finished", nil)
	}

	var body struct {
		ExitCode int `json:"exit_code"`
	}
	c.BodyParser(&body)

	exitCode := body.ExitCode
	if exitCode == 0 {
		exitCode = 1
	}

	h.repo.UpdateResult(taskID, "failed", exitCode, "")

	return response.OK(c, "Task marked as failed", nil)
}

// MarkAsTimeout handles task timeout
func (h *WebhookHandler) MarkAsTimeout(c *fiber.Ctx) error {
	taskID := c.Params("id")

	if !h.verifySignature(c, taskID) {
		return response.Unauthorized(c, "Invalid signature")
	}

	task, err := h.repo.FindByID(taskID)
	if err != nil {
		return response.NotFound(c, "Task not found")
	}

	if task.IsFinished() {
		return response.OK(c, "Task already finished", nil)
	}

	h.repo.UpdateResult(taskID, "timeout", 124, "")

	return response.OK(c, "Task marked as timeout", nil)
}

// verifySignature validates the webhook signature
func (h *WebhookHandler) verifySignature(c *fiber.Ctx, taskID string) bool {
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
func (h *WebhookHandler) generateSignature(taskID, expires string) string {
	data := fmt.Sprintf("%s:%s", taskID, expires)
	mac := hmac.New(sha256.New, []byte(h.secretKey))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

// GenerateCallbackURLs creates signed URLs for task callbacks
func (h *WebhookHandler) GenerateCallbackURLs(baseURL, taskID string, expireMinutes int) CallbackURLs {
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
