package handlers_test

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/pquerna/otp/totp"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/auth/handlers"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
	"github.com/kkz6/launch-go/internal/pkg/security"
)

func setupTwoFactorHandler(userID string) (*fiber.App, *mockRepoRegistry, *handlers.Handler) {
	reg := newMockRegistry()
	cfg := testConfig()
	logger := zerolog.New(os.Stderr)
	svc, _ := services.NewService(reg, cfg, &logger, nil, &mockCache{})
	handler := handlers.NewHandler(svc)
	app := newTestAppWithValidation()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("userID", userID)
		return c.Next()
	})
	return app, reg, handler
}

func TestTwoFactorHandler_EnableTwoFactor_Success(t *testing.T) {
	app, reg, handler := setupTwoFactorHandler("user_001")
	app.Post("/two-factor/enable", handler.TwoFactor.EnableTwoFactor)

	user := newTestUser("user_001", "Test User", "test@example.com", "hashed")
	reg.user.users["user_001"] = user

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/two-factor/enable", nil))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	r := parseResponse(resp)
	assert.True(t, r.Success)

	var data map[string]any
	_ = json.Unmarshal(r.Data, &data)
	assert.NotEmpty(t, data["qr_code_url"])
	assert.NotEmpty(t, data["secret_key"])
}

func TestTwoFactorHandler_EnableTwoFactor_NoUserID(t *testing.T) {
	reg := newMockRegistry()
	cfg := testConfig()
	logger := zerolog.New(os.Stderr)
	svc, _ := services.NewService(reg, cfg, &logger, nil, &mockCache{})
	handler := handlers.NewHandler(svc)
	app := newTestAppWithValidation()
	app.Post("/two-factor/enable", handler.TwoFactor.EnableTwoFactor)

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/two-factor/enable", nil))
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestTwoFactorHandler_ConfirmTwoFactor_Success(t *testing.T) {
	app, reg, handler := setupTwoFactorHandler("user_001")
	app.Post("/two-factor/confirm", handler.TwoFactor.ConfirmTwoFactor)

	secret := "JBSWY3DPEHPK3PXP"
	user := newTestUser("user_001", "Test User", "test@example.com", "hashed")
	user.TwoFactorSecret = &secret
	reg.user.users["user_001"] = user

	code, err := totp.GenerateCode(secret, time.Now())
	require.NoError(t, err)

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/two-factor/confirm", map[string]string{
		"code": code,
	}))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	r := parseResponse(resp)
	assert.True(t, r.Success)

	var data map[string]any
	_ = json.Unmarshal(r.Data, &data)
	assert.NotNil(t, data["recovery_codes"])

	// Verify 2FA is confirmed
	assert.NotNil(t, reg.user.users["user_001"].TwoFactorConfirmedAt)
}

func TestTwoFactorHandler_ConfirmTwoFactor_InvalidCode(t *testing.T) {
	app, reg, handler := setupTwoFactorHandler("user_001")
	app.Post("/two-factor/confirm", handler.TwoFactor.ConfirmTwoFactor)

	secret := "JBSWY3DPEHPK3PXP"
	user := newTestUser("user_001", "Test User", "test@example.com", "hashed")
	user.TwoFactorSecret = &secret
	reg.user.users["user_001"] = user

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/two-factor/confirm", map[string]string{
		"code": "000000",
	}))
	require.NoError(t, err)
	assert.NotEqual(t, http.StatusOK, resp.StatusCode)
}

func TestTwoFactorHandler_ConfirmTwoFactor_ValidationError(t *testing.T) {
	app, reg, handler := setupTwoFactorHandler("user_001")
	app.Post("/two-factor/confirm", handler.TwoFactor.ConfirmTwoFactor)

	user := newTestUser("user_001", "Test User", "test@example.com", "hashed")
	reg.user.users["user_001"] = user

	// Code must be exactly 6 chars
	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/two-factor/confirm", map[string]string{
		"code": "12",
	}))
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

func TestTwoFactorHandler_DisableTwoFactor_Success(t *testing.T) {
	app, reg, handler := setupTwoFactorHandler("user_001")
	app.Delete("/two-factor/disable", handler.TwoFactor.DisableTwoFactor)

	secret := "JBSWY3DPEHPK3PXP"
	now := time.Now()
	hashed, _ := security.HashPassword("correct-password")
	user := newTestUser("user_001", "Test User", "test@example.com", hashed)
	user.TwoFactorSecret = &secret
	user.TwoFactorConfirmedAt = &now
	reg.user.users["user_001"] = user

	resp, err := app.Test(makeJSONRequest(http.MethodDelete, "/two-factor/disable", map[string]string{
		"password": "correct-password",
	}))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	r := parseResponse(resp)
	assert.True(t, r.Success)

	// Verify 2FA is disabled
	assert.Nil(t, reg.user.users["user_001"].TwoFactorSecret)
	assert.Nil(t, reg.user.users["user_001"].TwoFactorConfirmedAt)
}

func TestTwoFactorHandler_DisableTwoFactor_WrongPassword(t *testing.T) {
	app, reg, handler := setupTwoFactorHandler("user_001")
	app.Delete("/two-factor/disable", handler.TwoFactor.DisableTwoFactor)

	secret := "JBSWY3DPEHPK3PXP"
	now := time.Now()
	hashed, _ := security.HashPassword("correct-password")
	user := newTestUser("user_001", "Test User", "test@example.com", hashed)
	user.TwoFactorSecret = &secret
	user.TwoFactorConfirmedAt = &now
	reg.user.users["user_001"] = user

	resp, err := app.Test(makeJSONRequest(http.MethodDelete, "/two-factor/disable", map[string]string{
		"password": "wrong-password",
	}))
	require.NoError(t, err)
	assert.NotEqual(t, http.StatusOK, resp.StatusCode)
}

func TestTwoFactorHandler_DisableTwoFactor_ValidationError(t *testing.T) {
	app, reg, handler := setupTwoFactorHandler("user_001")
	app.Delete("/two-factor/disable", handler.TwoFactor.DisableTwoFactor)

	user := newTestUser("user_001", "Test User", "test@example.com", "hashed")
	reg.user.users["user_001"] = user

	resp, err := app.Test(makeJSONRequest(http.MethodDelete, "/two-factor/disable", map[string]string{}))
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

func TestTwoFactorHandler_TwoFactorChallenge_Success(t *testing.T) {
	reg := newMockRegistry()
	cfg := testConfig()
	logger := zerolog.New(os.Stderr)
	svc, _ := services.NewService(reg, cfg, &logger, nil, &mockCache{})
	handler := handlers.NewHandler(svc)
	app := newTestAppWithValidation()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("userID", "user_001")
		c.Locals("sessionID", "session_001")
		return c.Next()
	})
	app.Post("/two-factor/challenge", handler.TwoFactor.TwoFactorChallenge)

	secret := "JBSWY3DPEHPK3PXP"
	now := time.Now()
	user := newTestUser("user_001", "Test User", "test@example.com", "hashed")
	user.TwoFactorSecret = &secret
	user.TwoFactorConfirmedAt = &now
	reg.user.users["user_001"] = user

	userID := "user_001"
	reg.session.sessions["session_001"] = &models.Session{ID: "session_001", UserID: &userID}

	code, err := totp.GenerateCode(secret, time.Now())
	require.NoError(t, err)

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/two-factor/challenge", map[string]string{
		"code": code,
	}))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	r := parseResponse(resp)
	assert.True(t, r.Success)
	assert.Equal(t, "Two-factor authentication verified", r.Message)
}

func TestTwoFactorHandler_TwoFactorChallenge_InvalidCode(t *testing.T) {
	app, reg, handler := setupTwoFactorHandler("user_001")
	app.Post("/two-factor/challenge", handler.TwoFactor.TwoFactorChallenge)

	secret := "JBSWY3DPEHPK3PXP"
	now := time.Now()
	user := newTestUser("user_001", "Test User", "test@example.com", "hashed")
	user.TwoFactorSecret = &secret
	user.TwoFactorConfirmedAt = &now
	reg.user.users["user_001"] = user

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/two-factor/challenge", map[string]string{
		"code": "000000",
	}))
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestTwoFactorHandler_GetRecoveryCodes_Success(t *testing.T) {
	app, reg, handler := setupTwoFactorHandler("user_001")
	app.Get("/two-factor/recovery-codes", handler.TwoFactor.GetRecoveryCodes)

	secret := "JBSWY3DPEHPK3PXP"
	now := time.Now()
	codes := "hash1,hash2,hash3"
	user := newTestUser("user_001", "Test User", "test@example.com", "hashed")
	user.TwoFactorSecret = &secret
	user.TwoFactorConfirmedAt = &now
	user.TwoFactorRecoveryCodes = &codes
	reg.user.users["user_001"] = user

	resp, err := app.Test(makeJSONRequest(http.MethodGet, "/two-factor/recovery-codes", nil))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	r := parseResponse(resp)
	assert.True(t, r.Success)

	var data map[string]any
	_ = json.Unmarshal(r.Data, &data)
	assert.Equal(t, float64(3), data["remaining_count"])
}

func TestTwoFactorHandler_RegenerateRecoveryCodes_Success(t *testing.T) {
	app, reg, handler := setupTwoFactorHandler("user_001")
	app.Post("/two-factor/recovery-codes", handler.TwoFactor.RegenerateRecoveryCodes)

	secret := "JBSWY3DPEHPK3PXP"
	now := time.Now()
	user := newTestUser("user_001", "Test User", "test@example.com", "hashed")
	user.TwoFactorSecret = &secret
	user.TwoFactorConfirmedAt = &now
	reg.user.users["user_001"] = user

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/two-factor/recovery-codes", nil))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	r := parseResponse(resp)
	assert.True(t, r.Success)

	var data map[string]any
	_ = json.Unmarshal(r.Data, &data)
	codes := data["recovery_codes"].([]any)
	assert.Len(t, codes, 8)
}

func TestTwoFactorHandler_RegenerateRecoveryCodes_NoUserID(t *testing.T) {
	reg := newMockRegistry()
	cfg := testConfig()
	logger := zerolog.New(os.Stderr)
	svc, _ := services.NewService(reg, cfg, &logger, nil, &mockCache{})
	handler := handlers.NewHandler(svc)
	app := newTestAppWithValidation()
	app.Post("/two-factor/recovery-codes", handler.TwoFactor.RegenerateRecoveryCodes)

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/two-factor/recovery-codes", nil))
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
