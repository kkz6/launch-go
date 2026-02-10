package handlers_test

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/auth/handlers"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
	"github.com/kkz6/launch-go/internal/pkg/security"
)

// ============================================================================
// Setup helpers
// ============================================================================

func setupUserHandler(t *testing.T) (*fiber.App, *mockRepoRegistry, *handlers.UserHandler) {
	t.Helper()

	reg := newMockRegistry()
	cfg := testConfig()
	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled)
	c := &mockCache{}

	svc, err := services.NewService(reg, cfg, &logger, nil, c)
	require.NoError(t, err)

	handler := handlers.NewUserHandler(svc)
	app := newTestAppWithValidation()

	return app, reg, handler
}

func setupUserHandlerWithAuth(t *testing.T, userID string) (*fiber.App, *mockRepoRegistry, *handlers.UserHandler) {
	t.Helper()

	app, reg, handler := setupUserHandler(t)

	app.Use(func(c *fiber.Ctx) error {
		c.Locals("userID", userID)
		return c.Next()
	})

	return app, reg, handler
}

// ============================================================================
// User (GET current user)
// ============================================================================

func TestUserHandler_User_Success(t *testing.T) {
	app, reg, handler := setupUserHandlerWithAuth(t, "user_001")
	app.Get("/user", handler.User)

	teamID := "team_001"
	user := newTestUser("user_001", "John Doe", "john@example.com", "hashed")
	user.CurrentTeamID = &teamID
	reg.user.users["user_001"] = user

	resp, err := app.Test(makeJSONRequest(http.MethodGet, "/user", nil))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	r := parseResponse(resp)
	assert.True(t, r.Success)
	assert.Equal(t, "User retrieved", r.Message)

	var data map[string]any
	require.NoError(t, json.Unmarshal(r.Data, &data))
	assert.Equal(t, "user_001", data["id"])
	assert.Equal(t, "John Doe", data["name"])
	assert.Equal(t, "john@example.com", data["email"])
	assert.Equal(t, true, data["onboarded"])
}

func TestUserHandler_User_NoUserID(t *testing.T) {
	app, _, handler := setupUserHandler(t)
	app.Get("/user", handler.User)

	resp, err := app.Test(makeJSONRequest(http.MethodGet, "/user", nil))
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	r := parseResponse(resp)
	assert.False(t, r.Success)
}

// ============================================================================
// UpdateProfile
// ============================================================================

func TestUserHandler_UpdateProfile_Success(t *testing.T) {
	app, reg, handler := setupUserHandlerWithAuth(t, "user_001")
	app.Put("/user/profile", handler.UpdateProfile)

	user := newTestUser("user_001", "John Doe", "john@example.com", "hashed")
	reg.user.users["user_001"] = user

	body := map[string]string{
		"name":  "Jane Doe",
		"email": "john@example.com",
	}

	resp, err := app.Test(makeJSONRequest(http.MethodPut, "/user/profile", body))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	r := parseResponse(resp)
	assert.True(t, r.Success)
	assert.Equal(t, "Profile updated", r.Message)

	var data map[string]any
	require.NoError(t, json.Unmarshal(r.Data, &data))
	assert.Equal(t, "Jane Doe", data["name"])
}

func TestUserHandler_UpdateProfile_ValidationError_MissingName(t *testing.T) {
	app, reg, handler := setupUserHandlerWithAuth(t, "user_001")
	app.Put("/user/profile", handler.UpdateProfile)

	user := newTestUser("user_001", "John Doe", "john@example.com", "hashed")
	reg.user.users["user_001"] = user

	body := map[string]string{
		"email": "john@example.com",
	}

	resp, err := app.Test(makeJSONRequest(http.MethodPut, "/user/profile", body))
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)

	r := parseResponse(resp)
	assert.False(t, r.Success)
}

func TestUserHandler_UpdateProfile_EmailConflict(t *testing.T) {
	app, reg, handler := setupUserHandlerWithAuth(t, "user_001")
	app.Put("/user/profile", handler.UpdateProfile)

	user := newTestUser("user_001", "John Doe", "john@example.com", "hashed")
	reg.user.users["user_001"] = user

	existingUser := newTestUser("user_002", "Jane Smith", "jane@example.com", "hashed")
	reg.user.users["user_002"] = existingUser

	body := map[string]string{
		"name":  "John Doe",
		"email": "jane@example.com",
	}

	resp, err := app.Test(makeJSONRequest(http.MethodPut, "/user/profile", body))
	require.NoError(t, err)
	assert.NotEqual(t, http.StatusOK, resp.StatusCode)

	r := parseResponse(resp)
	assert.False(t, r.Success)
}

// ============================================================================
// ChangePassword
// ============================================================================

func TestUserHandler_ChangePassword_Success(t *testing.T) {
	app, reg, handler := setupUserHandlerWithAuth(t, "user_001")
	app.Put("/user/password", handler.ChangePassword)

	hashedPwd, err := security.HashPassword("password123")
	require.NoError(t, err)

	user := newTestUser("user_001", "John Doe", "john@example.com", hashedPwd)
	reg.user.users["user_001"] = user

	body := map[string]string{
		"current_password":      "password123",
		"password":              "newpassword456",
		"password_confirmation": "newpassword456",
	}

	resp, err := app.Test(makeJSONRequest(http.MethodPut, "/user/password", body))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	r := parseResponse(resp)
	assert.True(t, r.Success)
	assert.Equal(t, "Password changed successfully", r.Message)

	// Verify the password was actually updated
	updatedUser := reg.user.users["user_001"]
	assert.True(t, security.VerifyPassword(updatedUser.Password, "newpassword456"))
}

func TestUserHandler_ChangePassword_WrongCurrentPassword(t *testing.T) {
	app, reg, handler := setupUserHandlerWithAuth(t, "user_001")
	app.Put("/user/password", handler.ChangePassword)

	hashedPwd, err := security.HashPassword("password123")
	require.NoError(t, err)

	user := newTestUser("user_001", "John Doe", "john@example.com", hashedPwd)
	reg.user.users["user_001"] = user

	body := map[string]string{
		"current_password":      "wrongpassword",
		"password":              "newpassword456",
		"password_confirmation": "newpassword456",
	}

	resp, err := app.Test(makeJSONRequest(http.MethodPut, "/user/password", body))
	require.NoError(t, err)
	assert.NotEqual(t, http.StatusOK, resp.StatusCode)

	r := parseResponse(resp)
	assert.False(t, r.Success)
}

func TestUserHandler_ChangePassword_ValidationError_PasswordTooShort(t *testing.T) {
	app, reg, handler := setupUserHandlerWithAuth(t, "user_001")
	app.Put("/user/password", handler.ChangePassword)

	hashedPwd, err := security.HashPassword("password123")
	require.NoError(t, err)

	user := newTestUser("user_001", "John Doe", "john@example.com", hashedPwd)
	reg.user.users["user_001"] = user

	body := map[string]string{
		"current_password":      "password123",
		"password":              "short",
		"password_confirmation": "short",
	}

	resp, err := app.Test(makeJSONRequest(http.MethodPut, "/user/password", body))
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)

	r := parseResponse(resp)
	assert.False(t, r.Success)
}

// ============================================================================
// DeleteAccount
// ============================================================================

func TestUserHandler_DeleteAccount_ValidationError_MissingPassword(t *testing.T) {
	app, reg, handler := setupUserHandlerWithAuth(t, "user_001")
	app.Delete("/user/account", handler.DeleteAccount)

	hashedPwd, err := security.HashPassword("password123")
	require.NoError(t, err)

	user := newTestUser("user_001", "John Doe", "john@example.com", hashedPwd)
	reg.user.users["user_001"] = user

	body := map[string]string{}

	resp, err := app.Test(makeJSONRequest(http.MethodDelete, "/user/account", body))
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)

	r := parseResponse(resp)
	assert.False(t, r.Success)
}

// ============================================================================
// CheckUserStatus (public endpoint - no auth required)
// ============================================================================

func TestUserHandler_CheckUserStatus_ExistingUser(t *testing.T) {
	app, reg, handler := setupUserHandler(t)
	app.Post("/user/status", handler.CheckUserStatus)

	now := time.Now()
	user := newTestUser("user_001", "John Doe", "john@example.com", "hashed")
	user.EmailVerifiedAt = &now
	reg.user.users["user_001"] = user

	body := map[string]string{
		"email": "john@example.com",
	}

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/user/status", body))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	r := parseResponse(resp)
	assert.True(t, r.Success)
	assert.Equal(t, "User status retrieved", r.Message)

	var data map[string]any
	require.NoError(t, json.Unmarshal(r.Data, &data))
	assert.Equal(t, true, data["user_exists"])
	assert.Equal(t, false, data["requires_verification"])
	assert.Equal(t, false, data["has_two_factor"])
	assert.Equal(t, false, data["has_passkeys"])
	assert.Equal(t, float64(0), data["passkey_count"])
}

func TestUserHandler_CheckUserStatus_NonExistentUser(t *testing.T) {
	app, _, handler := setupUserHandler(t)
	app.Post("/user/status", handler.CheckUserStatus)

	body := map[string]string{
		"email": "nobody@example.com",
	}

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/user/status", body))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	r := parseResponse(resp)
	assert.True(t, r.Success)
	assert.Equal(t, "User status retrieved", r.Message)

	var data map[string]any
	require.NoError(t, json.Unmarshal(r.Data, &data))
	assert.Equal(t, false, data["user_exists"])
	assert.Equal(t, false, data["requires_verification"])
	assert.Equal(t, false, data["has_two_factor"])
	assert.Equal(t, false, data["has_passkeys"])
	assert.Equal(t, float64(0), data["passkey_count"])
}

// ============================================================================
// ResetOnboarding
// ============================================================================

func TestUserHandler_ResetOnboarding_Success(t *testing.T) {
	app, reg, handler := setupUserHandlerWithAuth(t, "user_001")
	app.Post("/user/reset-onboarding", handler.ResetOnboarding)

	user := newTestUser("user_001", "John Doe", "john@example.com", "hashed")
	user.Onboarded = true
	reg.user.users["user_001"] = user

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/user/reset-onboarding", nil))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	r := parseResponse(resp)
	assert.True(t, r.Success)
	assert.Equal(t, "Onboarding reset successfully", r.Message)

	var data map[string]any
	require.NoError(t, json.Unmarshal(r.Data, &data))
	assert.Equal(t, false, data["onboarded"])

	// Verify the user's onboarded flag was actually set to false in the repo
	assert.False(t, reg.user.users["user_001"].Onboarded)
}
