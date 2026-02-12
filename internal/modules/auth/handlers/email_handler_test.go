package handlers_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/auth/handlers"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
)

func setupEmailHandler() (*fiber.App, *mockRepoRegistry, *handlers.Handler) {
	reg := newMockRegistry()
	cfg := testConfig()
	logger := zerolog.New(os.Stderr)
	svc, _ := services.NewService(reg, cfg, &logger, nil, newMockCache())
	handler := handlers.NewHandler(svc)
	app := newTestAppWithValidation()
	return app, reg, handler
}

// computeEmailHash mirrors the EmailVerificationService.generateEmailHash algorithm
func computeEmailHash(email string, cfg *config.Config) string {
	key := cfg.App.Key
	if key == "" {
		key = cfg.JWT.Secret
	}
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte(email))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestEmailHandler_VerifyEmail_Success(t *testing.T) {
	app, reg, handler := setupEmailHandler()
	app.Get("/verify-email/:id/:hash", handler.Email.VerifyEmail)

	user := newTestUser("user_001", "Test User", "test@example.com", "hashed")
	reg.user.users["user_001"] = user

	hash := computeEmailHash("test@example.com", testConfig())

	resp, err := app.Test(makeJSONRequest(http.MethodGet, "/verify-email/user_001/"+hash, nil), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	r := parseResponse(resp)
	assert.True(t, r.Success)
	assert.Equal(t, "Email verified successfully", r.Message)
	assert.NotNil(t, reg.user.users["user_001"].EmailVerifiedAt)
}

func TestEmailHandler_VerifyEmail_InvalidHash(t *testing.T) {
	app, reg, handler := setupEmailHandler()
	app.Get("/verify-email/:id/:hash", handler.Email.VerifyEmail)

	user := newTestUser("user_001", "Test User", "test@example.com", "hashed")
	reg.user.users["user_001"] = user

	resp, err := app.Test(makeJSONRequest(http.MethodGet, "/verify-email/user_001/invalid-hash", nil), testTimeout)
	require.NoError(t, err)
	assert.NotEqual(t, http.StatusOK, resp.StatusCode)
}

func TestEmailHandler_VerifyEmail_UserNotFound(t *testing.T) {
	app, _, handler := setupEmailHandler()
	app.Get("/verify-email/:id/:hash", handler.Email.VerifyEmail)

	resp, err := app.Test(makeJSONRequest(http.MethodGet, "/verify-email/nonexistent/somehash", nil), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestEmailHandler_VerifyEmail_AlreadyVerified(t *testing.T) {
	app, reg, handler := setupEmailHandler()
	app.Get("/verify-email/:id/:hash", handler.Email.VerifyEmail)

	now := time.Now()
	user := newTestUser("user_001", "Test User", "test@example.com", "hashed")
	user.EmailVerifiedAt = &now
	reg.user.users["user_001"] = user

	hash := computeEmailHash("test@example.com", testConfig())

	resp, err := app.Test(makeJSONRequest(http.MethodGet, "/verify-email/user_001/"+hash, nil), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestEmailHandler_ResendVerificationEmail_Success(t *testing.T) {
	app, reg, handler := setupEmailHandler()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("userID", "user_001")
		return c.Next()
	})
	app.Post("/email/verification-notification", handler.Email.ResendVerificationEmail)

	user := newTestUser("user_001", "Test User", "test@example.com", "hashed")
	reg.user.users["user_001"] = user

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/email/verification-notification", nil), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	r := parseResponse(resp)
	assert.True(t, r.Success)
}

func TestEmailHandler_ResendVerificationEmail_NoUserID(t *testing.T) {
	app, _, handler := setupEmailHandler()
	app.Post("/email/verification-notification", handler.Email.ResendVerificationEmail)

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/email/verification-notification", nil), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
