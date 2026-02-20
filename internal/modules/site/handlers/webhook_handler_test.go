package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/site/services"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// testableWebhookHandler creates a Fiber handler that mimics DeployWebhook but uses a mock
func testableWebhookHandler(fn func(ctx context.Context, siteID, token string, payload map[string]any) error) fiber.Handler {
	return func(c *fiber.Ctx) error {
		siteID := c.Params("siteId")
		token := c.Params("token")

		if siteID == "" || token == "" {
			return c.SendStatus(fiber.StatusNotFound)
		}

		var payload map[string]any
		if err := c.BodyParser(&payload); err != nil {
			payload = nil
		}

		err := fn(c.Context(), siteID, token, payload)
		if err != nil {
			if errors.Is(err, services.ErrBranchMismatch) {
				return c.SendStatus(fiber.StatusOK)
			}

			var fiberErr *fiber.Error
			if errors.As(err, &fiberErr) {
				return fiberutil.Error(c, fiberErr.Code, fiberErr.Message)
			}

			if fiberutil.IsNotFound(err) {
				return c.SendStatus(fiber.StatusNotFound)
			}

			return fiberutil.RespondInternalError(c, err.Error())
		}

		return c.SendStatus(fiber.StatusNoContent)
	}
}

func newTestWebhookApp(fn func(ctx context.Context, siteID, token string, payload map[string]any) error) *fiber.App {
	app := fiber.New(fiber.Config{
		ErrorHandler: fiberutil.NewErrorHandler(),
	})

	handler := testableWebhookHandler(fn)
	app.Post("/deploy/:siteId/:token", handler)
	app.Get("/deploy/:siteId/:token", handler)

	return app
}

func TestWebhookHandler_ValidRequestReturns204(t *testing.T) {
	app := newTestWebhookApp(func(_ context.Context, siteID, token string, _ map[string]any) error {
		assert.Equal(t, "site-123", siteID)
		assert.Equal(t, "valid-token", token)
		return nil
	})

	req := httptest.NewRequest(http.MethodPost, "/deploy/site-123/valid-token", nil)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, 5000)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusNoContent, resp.StatusCode)
}

func TestWebhookHandler_InvalidTokenReturns401(t *testing.T) {
	app := newTestWebhookApp(func(_ context.Context, _, _ string, _ map[string]any) error {
		return services.ErrInvalidDeployToken
	})

	req := httptest.NewRequest(http.MethodPost, "/deploy/site-123/wrong-token", nil)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, 5000)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestWebhookHandler_SiteNotFoundReturns404(t *testing.T) {
	app := newTestWebhookApp(func(_ context.Context, _, _ string, _ map[string]any) error {
		return fiberutil.NotFound()
	})

	req := httptest.NewRequest(http.MethodPost, "/deploy/nonexistent/sometoken", nil)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, 5000)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestWebhookHandler_BranchMismatchReturns200(t *testing.T) {
	app := newTestWebhookApp(func(_ context.Context, _, _ string, _ map[string]any) error {
		return services.ErrBranchMismatch
	})

	payload := map[string]any{
		"ref": "refs/heads/develop",
		"head_commit": map[string]any{
			"id": "abc123",
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/deploy/site-123/valid-token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, 5000)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestWebhookHandler_GETMethodWorks(t *testing.T) {
	app := newTestWebhookApp(func(_ context.Context, _, _ string, _ map[string]any) error {
		return nil
	})

	req := httptest.NewRequest(http.MethodGet, "/deploy/site-123/valid-token", nil)

	resp, err := app.Test(req, 5000)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusNoContent, resp.StatusCode)
}

func TestWebhookHandler_PayloadIsParsed(t *testing.T) {
	var receivedPayload map[string]any

	app := newTestWebhookApp(func(_ context.Context, _, _ string, payload map[string]any) error {
		receivedPayload = payload
		return nil
	})

	payload := map[string]any{
		"ref": "refs/heads/main",
		"head_commit": map[string]any{
			"id":      "abc123",
			"message": "test commit",
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/deploy/site-123/valid-token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, 5000)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusNoContent, resp.StatusCode)

	require.NotNil(t, receivedPayload)
	assert.Equal(t, "refs/heads/main", receivedPayload["ref"])
}

func TestWebhookHandler_EmptyBodySetsNilPayload(t *testing.T) {
	var receivedPayload map[string]any
	called := false

	app := newTestWebhookApp(func(_ context.Context, _, _ string, payload map[string]any) error {
		receivedPayload = payload
		called = true
		return nil
	})

	req := httptest.NewRequest(http.MethodPost, "/deploy/site-123/valid-token", nil)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, 5000)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusNoContent, resp.StatusCode)
	assert.True(t, called)
	assert.Nil(t, receivedPayload)
}
