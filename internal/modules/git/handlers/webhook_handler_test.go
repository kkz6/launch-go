package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/git/providers"
)

type webhookQueueStub struct {
	err  error
	task *asynq.Task
}

func (q *webhookQueueStub) Enqueue(task *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
	q.task = task
	if q.err != nil {
		return nil, q.err
	}
	return &asynq.TaskInfo{}, nil
}

func TestHandleWebhook_ReturnsServerErrorWhenQueueIsUnavailable(t *testing.T) {
	const secret = "webhook-secret"
	payload := `{"ref":"refs/heads/main"}`
	signature := githubWebhookSignature([]byte(payload), secret)

	factory := providers.NewProviderFactory()
	factory.RegisterConfig(providers.GitProviderGitHub, &providers.ProviderConfig{WebhookSecret: secret})
	logger := zerolog.Nop()
	handler := NewWebhookHandler(factory, &logger)

	app := fiber.New()
	app.Post("/webhooks/git/:provider", handler.HandleWebhook)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/git/github", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", signature)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestHandleWebhookValidationAndQueueing(t *testing.T) {
	const secret = "webhook-secret"
	payload := `{"ref":"refs/heads/main"}`
	signature := githubWebhookSignature([]byte(payload), secret)

	tests := []struct {
		name           string
		provider       string
		signature      string
		configure      bool
		queue          *webhookQueueStub
		expectedStatus int
	}{
		{name: "invalid provider", provider: "unknown", expectedStatus: http.StatusBadRequest},
		{name: "missing signature", provider: "github", configure: true, expectedStatus: http.StatusBadRequest},
		{name: "provider not configured", provider: "github", signature: signature, expectedStatus: http.StatusInternalServerError},
		{name: "invalid signature", provider: "github", signature: "sha256=invalid", configure: true, expectedStatus: http.StatusUnauthorized},
		{name: "enqueue failure", provider: "github", signature: signature, configure: true, queue: &webhookQueueStub{err: errors.New("queue down")}, expectedStatus: http.StatusInternalServerError},
		{name: "success", provider: "github", signature: signature, configure: true, queue: &webhookQueueStub{}, expectedStatus: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := providers.NewProviderFactory()
			if tt.configure {
				factory.RegisterConfig(providers.GitProviderGitHub, &providers.ProviderConfig{WebhookSecret: secret})
			}
			logger := zerolog.Nop()
			handler := NewWebhookHandler(factory, &logger)
			if tt.queue != nil {
				handler.SetQueueClient(tt.queue)
			}

			app := fiber.New()
			app.Post("/webhooks/git/:provider", handler.HandleWebhook)
			req := httptest.NewRequest(http.MethodPost, "/webhooks/git/"+tt.provider, strings.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			if tt.signature != "" {
				req.Header.Set("X-Hub-Signature-256", tt.signature)
			}

			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
			if tt.expectedStatus == http.StatusOK {
				require.NotNil(t, tt.queue.task)
				assert.Equal(t, "git:process_webhook", tt.queue.task.Type())
			}
		})
	}
}

func githubWebhookSignature(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)

	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
