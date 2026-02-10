package handlers_test

import (
	"net/http"
	"os"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/auth/handlers"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
)

func setupPasswordHandler() (*fiber.App, *mockRepoRegistry, *handlers.Handler) {
	reg := newMockRegistry()
	cfg := testConfig()
	logger := zerolog.New(os.Stderr)
	svc, _ := services.NewService(reg, cfg, &logger, nil, &mockCache{})
	handler := handlers.NewHandler(svc)
	app := newTestAppWithValidation()
	return app, reg, handler
}

func TestPasswordHandler_ForgotPassword_Success(t *testing.T) {
	app, reg, handler := setupPasswordHandler()
	app.Post("/forgot-password", handler.Password.ForgotPassword)

	user := newTestUser("user_001", "Test User", "test@example.com", "hashed")
	reg.user.users["user_001"] = user

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/forgot-password", map[string]string{
		"email": "test@example.com",
	}))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	r := parseResponse(resp)
	assert.True(t, r.Success)
	assert.Contains(t, r.Message, "password reset link")
}

func TestPasswordHandler_ForgotPassword_AlwaysReturnsSuccess(t *testing.T) {
	app, _, handler := setupPasswordHandler()
	app.Post("/forgot-password", handler.Password.ForgotPassword)

	// Non-existent email should still return success to prevent enumeration
	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/forgot-password", map[string]string{
		"email": "nonexistent@example.com",
	}))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	r := parseResponse(resp)
	assert.True(t, r.Success)
}

func TestPasswordHandler_ForgotPassword_ValidationError(t *testing.T) {
	app, _, handler := setupPasswordHandler()
	app.Post("/forgot-password", handler.Password.ForgotPassword)

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/forgot-password", map[string]string{
		"email": "not-an-email",
	}))
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

func TestPasswordHandler_ResetPassword_ValidationError(t *testing.T) {
	app, _, handler := setupPasswordHandler()
	app.Post("/reset-password", handler.Password.ResetPassword)

	// Missing required fields
	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/reset-password", map[string]string{}))
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

func TestPasswordHandler_ResetPassword_InvalidToken(t *testing.T) {
	app, reg, handler := setupPasswordHandler()
	app.Post("/reset-password", handler.Password.ResetPassword)

	user := newTestUser("user_001", "Test User", "test@example.com", "hashed")
	reg.user.users["user_001"] = user

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/reset-password", map[string]string{
		"email":                 "test@example.com",
		"token":                 "invalid-token",
		"password":              "newpassword123",
		"password_confirmation": "newpassword123",
	}))
	require.NoError(t, err)
	// No reset token exists, so this should fail
	assert.NotEqual(t, http.StatusOK, resp.StatusCode)
}
