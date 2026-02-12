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
	"github.com/kkz6/launch-go/internal/modules/auth/services"
	"github.com/kkz6/launch-go/internal/pkg/security"
)

func setupTwoFactorHandler(userID string) (*fiber.App, *mockRepoRegistry, *handlers.Handler) {
	reg := newMockRegistry()
	cfg := testConfig()
	logger := zerolog.New(os.Stderr)
	svc, _ := services.NewService(reg, cfg, &logger, nil, newMockCache())
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

	hashed, _ := security.HashPassword("correct-password")
	user := newTestUser("user_001", "Test User", "test@example.com", hashed)
	reg.user.users["user_001"] = user

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/two-factor/enable", map[string]string{
		"password": "correct-password",
	}), testTimeout)
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
	svc, _ := services.NewService(reg, cfg, &logger, nil, newMockCache())
	handler := handlers.NewHandler(svc)
	app := newTestAppWithValidation()
	app.Post("/two-factor/enable", handler.TwoFactor.EnableTwoFactor)

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/two-factor/enable", map[string]string{
		"password": "any-password",
	}), testTimeout)
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
	}), testTimeout)
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
	}), testTimeout)
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
	}), testTimeout)
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
	}), testTimeout)
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
	}), testTimeout)
	require.NoError(t, err)
	assert.NotEqual(t, http.StatusOK, resp.StatusCode)
}

func TestTwoFactorHandler_DisableTwoFactor_ValidationError(t *testing.T) {
	app, reg, handler := setupTwoFactorHandler("user_001")
	app.Delete("/two-factor/disable", handler.TwoFactor.DisableTwoFactor)

	user := newTestUser("user_001", "Test User", "test@example.com", "hashed")
	reg.user.users["user_001"] = user

	resp, err := app.Test(makeJSONRequest(http.MethodDelete, "/two-factor/disable", map[string]string{}), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

func TestTwoFactorHandler_TwoFactorChallenge_Success(t *testing.T) {
	reg := newMockRegistry()
	cfg := testConfig()
	logger := zerolog.New(os.Stderr)
	mc := newMockCache()
	svc, _ := services.NewService(reg, cfg, &logger, nil, mc)
	handler := handlers.NewHandler(svc)

	// Set up the user with 2FA enabled
	secret := "JBSWY3DPEHPK3PXP"
	now := time.Now()
	hashed, _ := security.HashPassword("correct-password")
	user := newTestUser("user_001", "Test User", "test@example.com", hashed)
	user.TwoFactorSecret = &secret
	user.TwoFactorConfirmedAt = &now
	reg.user.users["user_001"] = user

	// Step 1: Login to get a challenge token
	loginApp := newTestAppWithValidation()
	loginApp.Post("/login", handler.Auth.Login)

	loginResp, err := loginApp.Test(makeJSONRequest(http.MethodPost, "/login", map[string]string{
		"email":    "test@example.com",
		"password": "correct-password",
	}), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, loginResp.StatusCode)

	loginResult := parseResponse(loginResp)
	assert.True(t, loginResult.Success)

	var loginData map[string]any
	_ = json.Unmarshal(loginResult.Data, &loginData)
	assert.True(t, loginData["two_factor_required"].(bool))
	challengeToken := loginData["challenge_token"].(string)
	assert.NotEmpty(t, challengeToken)
	// No auth tokens should be returned
	assert.Nil(t, loginData["access_token"])

	// Step 2: Complete the challenge
	challengeApp := newTestAppWithValidation()
	challengeApp.Post("/two-factor/challenge", handler.TwoFactor.TwoFactorChallenge)

	code, err := totp.GenerateCode(secret, time.Now())
	require.NoError(t, err)

	resp, err := challengeApp.Test(makeJSONRequest(http.MethodPost, "/two-factor/challenge", map[string]string{
		"challenge_token": challengeToken,
		"code":            code,
	}), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	r := parseResponse(resp)
	assert.True(t, r.Success)
	assert.Equal(t, "Login successful", r.Message)

	var data map[string]any
	_ = json.Unmarshal(r.Data, &data)
	assert.NotEmpty(t, data["access_token"])
	assert.NotEmpty(t, data["refresh_token"])
	assert.Equal(t, "Bearer", data["token_type"])
}

func TestTwoFactorHandler_TwoFactorChallenge_InvalidCode(t *testing.T) {
	reg := newMockRegistry()
	cfg := testConfig()
	logger := zerolog.New(os.Stderr)
	mc := newMockCache()
	svc, _ := services.NewService(reg, cfg, &logger, nil, mc)
	handler := handlers.NewHandler(svc)

	// Set up the user with 2FA enabled
	secret := "JBSWY3DPEHPK3PXP"
	now := time.Now()
	hashed, _ := security.HashPassword("correct-password")
	user := newTestUser("user_001", "Test User", "test@example.com", hashed)
	user.TwoFactorSecret = &secret
	user.TwoFactorConfirmedAt = &now
	reg.user.users["user_001"] = user

	// Login to get a challenge token
	loginApp := newTestAppWithValidation()
	loginApp.Post("/login", handler.Auth.Login)

	loginResp, err := loginApp.Test(makeJSONRequest(http.MethodPost, "/login", map[string]string{
		"email":    "test@example.com",
		"password": "correct-password",
	}), testTimeout)
	require.NoError(t, err)

	loginResult := parseResponse(loginResp)
	var loginData map[string]any
	_ = json.Unmarshal(loginResult.Data, &loginData)
	challengeToken := loginData["challenge_token"].(string)

	// Try with invalid code
	challengeApp := newTestAppWithValidation()
	challengeApp.Post("/two-factor/challenge", handler.TwoFactor.TwoFactorChallenge)

	resp, err := challengeApp.Test(makeJSONRequest(http.MethodPost, "/two-factor/challenge", map[string]string{
		"challenge_token": challengeToken,
		"code":            "000000",
	}), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestTwoFactorHandler_TwoFactorChallenge_InvalidToken(t *testing.T) {
	reg := newMockRegistry()
	cfg := testConfig()
	logger := zerolog.New(os.Stderr)
	svc, _ := services.NewService(reg, cfg, &logger, nil, newMockCache())
	handler := handlers.NewHandler(svc)

	app := newTestAppWithValidation()
	app.Post("/two-factor/challenge", handler.TwoFactor.TwoFactorChallenge)

	user := newTestUser("user_001", "Test User", "test@example.com", "hashed")
	reg.user.users["user_001"] = user

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/two-factor/challenge", map[string]string{
		"challenge_token": "invalid-token",
		"code":            "123456",
	}), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestTwoFactorHandler_TwoFactorChallenge_TokenCannotBeReused(t *testing.T) {
	reg := newMockRegistry()
	cfg := testConfig()
	logger := zerolog.New(os.Stderr)
	mc := newMockCache()
	svc, _ := services.NewService(reg, cfg, &logger, nil, mc)
	handler := handlers.NewHandler(svc)

	secret := "JBSWY3DPEHPK3PXP"
	now := time.Now()
	hashed, _ := security.HashPassword("correct-password")
	user := newTestUser("user_001", "Test User", "test@example.com", hashed)
	user.TwoFactorSecret = &secret
	user.TwoFactorConfirmedAt = &now
	reg.user.users["user_001"] = user

	// Login to get challenge token
	loginApp := newTestAppWithValidation()
	loginApp.Post("/login", handler.Auth.Login)

	loginResp, err := loginApp.Test(makeJSONRequest(http.MethodPost, "/login", map[string]string{
		"email":    "test@example.com",
		"password": "correct-password",
	}), testTimeout)
	require.NoError(t, err)

	loginResult := parseResponse(loginResp)
	var loginData map[string]any
	_ = json.Unmarshal(loginResult.Data, &loginData)
	challengeToken := loginData["challenge_token"].(string)

	// First use — succeeds
	challengeApp := newTestAppWithValidation()
	challengeApp.Post("/two-factor/challenge", handler.TwoFactor.TwoFactorChallenge)

	code, _ := totp.GenerateCode(secret, time.Now())
	resp, err := challengeApp.Test(makeJSONRequest(http.MethodPost, "/two-factor/challenge", map[string]string{
		"challenge_token": challengeToken,
		"code":            code,
	}), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Second use — fails (token consumed)
	code2, _ := totp.GenerateCode(secret, time.Now())
	resp2, err := challengeApp.Test(makeJSONRequest(http.MethodPost, "/two-factor/challenge", map[string]string{
		"challenge_token": challengeToken,
		"code":            code2,
	}), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp2.StatusCode)
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

	resp, err := app.Test(makeJSONRequest(http.MethodGet, "/two-factor/recovery-codes", nil), testTimeout)
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

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/two-factor/recovery-codes", nil), testTimeout)
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
	svc, _ := services.NewService(reg, cfg, &logger, nil, newMockCache())
	handler := handlers.NewHandler(svc)
	app := newTestAppWithValidation()
	app.Post("/two-factor/recovery-codes", handler.TwoFactor.RegenerateRecoveryCodes)

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/two-factor/recovery-codes", nil), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// Test that login with 2FA user returns challenge token, not auth tokens
func TestAuthHandler_Login_TwoFactorEnabled_ReturnsChallengeToken(t *testing.T) {
	reg := newMockRegistry()
	cfg := testConfig()
	logger := zerolog.New(os.Stderr)
	svc, _ := services.NewService(reg, cfg, &logger, nil, newMockCache())
	handler := handlers.NewHandler(svc)
	app := newTestAppWithValidation()
	app.Post("/login", handler.Auth.Login)

	secret := "JBSWY3DPEHPK3PXP"
	now := time.Now()
	hashed, _ := security.HashPassword("correct-password")
	user := newTestUser("user_001", "Test User", "test@example.com", hashed)
	user.TwoFactorSecret = &secret
	user.TwoFactorConfirmedAt = &now
	reg.user.users["user_001"] = user

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/login", map[string]string{
		"email":    "test@example.com",
		"password": "correct-password",
	}), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	r := parseResponse(resp)
	assert.True(t, r.Success)

	var data map[string]any
	_ = json.Unmarshal(r.Data, &data)
	assert.True(t, data["two_factor_required"].(bool))
	assert.NotEmpty(t, data["challenge_token"])
	// No auth tokens should be present
	assert.Nil(t, data["access_token"])
	assert.Nil(t, data["refresh_token"])

	// No session should be created
	assert.Empty(t, reg.session.sessions)
}
