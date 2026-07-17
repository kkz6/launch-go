package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/git/providers"
)

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

func githubWebhookSignature(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)

	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
