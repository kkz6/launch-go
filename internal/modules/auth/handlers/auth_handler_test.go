package handlers_test

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/auth/handlers"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/security"
)

// ============================================================================
// Setup helpers
// ============================================================================

const testJWTSecret = "test-jwt-secret-at-least-32-chars-long"

func setupAuthHandler(t *testing.T) (*fiber.App, *mockRepoRegistry, *handlers.AuthHandler) {
	t.Helper()

	reg := newMockRegistry()
	cfg := testConfig()
	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled)

	svc, err := services.NewService(reg, cfg, &logger, nil, newMockCache())
	require.NoError(t, err)

	handler := handlers.NewAuthHandler(svc)
	app := newTestAppWithValidation()

	return app, reg, handler
}

func setupAuthHandlerWithAuth(t *testing.T, userID, sessionID string) (*fiber.App, *mockRepoRegistry, *handlers.AuthHandler) {
	t.Helper()

	app, reg, handler := setupAuthHandler(t)
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("userID", userID)
		if sessionID != "" {
			c.Locals("sessionID", sessionID)
		}
		return c.Next()
	})

	return app, reg, handler
}

func generateRefreshToken(userID, sessionID string) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":        userID,
		"type":       "refresh",
		"session_id": sessionID,
		"iat":        time.Now().Unix(),
		"exp":        time.Now().Add(time.Hour).Unix(),
	})

	tokenStr, _ := token.SignedString([]byte(testJWTSecret))

	return tokenStr
}

// ============================================================================
// Register tests
// ============================================================================

func TestAuthHandler_Register_ValidationError_MissingFields(t *testing.T) {
	app, _, handler := setupAuthHandler(t)
	app.Post("/register", fiberutil.Validate(handler.Register))

	body := map[string]string{}
	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/register", body), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)

	r := parseResponse(resp)
	assert.False(t, r.Success)

	var validationErrors map[string][]string
	_ = json.Unmarshal(r.Errors, &validationErrors)
	assert.Contains(t, validationErrors, "name")
	assert.Contains(t, validationErrors, "email")
	assert.Contains(t, validationErrors, "password")
	assert.Contains(t, validationErrors, "password_confirmation")
}

func TestAuthHandler_Register_ValidationError_InvalidEmail(t *testing.T) {
	app, _, handler := setupAuthHandler(t)
	app.Post("/register", fiberutil.Validate(handler.Register))

	body := map[string]string{
		"name":                  "Test User",
		"email":                 "not-an-email",
		"password":              "password123",
		"password_confirmation": "password123",
	}

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/register", body), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)

	r := parseResponse(resp)
	assert.False(t, r.Success)

	var validationErrors map[string][]string
	_ = json.Unmarshal(r.Errors, &validationErrors)
	assert.Contains(t, validationErrors, "email")
}

func TestAuthHandler_Register_ValidationError_PasswordMismatch(t *testing.T) {
	app, _, handler := setupAuthHandler(t)
	app.Post("/register", fiberutil.Validate(handler.Register))

	body := map[string]string{
		"name":                  "Test User",
		"email":                 "test@example.com",
		"password":              "password123",
		"password_confirmation": "different-password",
	}

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/register", body), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)

	r := parseResponse(resp)
	assert.False(t, r.Success)

	var validationErrors map[string][]string
	_ = json.Unmarshal(r.Errors, &validationErrors)
	assert.Contains(t, validationErrors, "password_confirmation")
}

func TestAuthHandler_Register_ValidationError_ShortPassword(t *testing.T) {
	app, _, handler := setupAuthHandler(t)
	app.Post("/register", fiberutil.Validate(handler.Register))

	body := map[string]string{
		"name":                  "Test User",
		"email":                 "test@example.com",
		"password":              "short",
		"password_confirmation": "short",
	}

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/register", body), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)

	r := parseResponse(resp)
	assert.False(t, r.Success)

	var validationErrors map[string][]string
	_ = json.Unmarshal(r.Errors, &validationErrors)
	assert.Contains(t, validationErrors, "password")
}

// ============================================================================
// Login tests
// ============================================================================

func TestAuthHandler_Login_Success(t *testing.T) {
	app, reg, handler := setupAuthHandler(t)
	app.Post("/login", fiberutil.Validate(handler.Login))

	hashedPassword, err := security.HashPassword("correct-password")
	require.NoError(t, err)

	user := newTestUser("user_001", "Test User", "test@example.com", hashedPassword)
	reg.user.users["user_001"] = user

	body := map[string]string{
		"email":    "test@example.com",
		"password": "correct-password",
	}

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/login", body), testTimeout)
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
	assert.NotZero(t, data["expires_in"])

	userData, ok := data["user"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "user_001", userData["id"])
	assert.Equal(t, "Test User", userData["name"])
	assert.Equal(t, "test@example.com", userData["email"])
}

func TestAuthHandler_Login_InvalidCredentials_WrongPassword(t *testing.T) {
	app, reg, handler := setupAuthHandler(t)
	app.Post("/login", fiberutil.Validate(handler.Login))

	hashedPassword, err := security.HashPassword("correct-password")
	require.NoError(t, err)

	user := newTestUser("user_001", "Test User", "test@example.com", hashedPassword)
	reg.user.users["user_001"] = user

	body := map[string]string{
		"email":    "test@example.com",
		"password": "wrong-password",
	}

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/login", body), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	r := parseResponse(resp)
	assert.False(t, r.Success)
	assert.Equal(t, "Invalid credentials", r.Message)
}

func TestAuthHandler_Login_UserNotFound(t *testing.T) {
	app, _, handler := setupAuthHandler(t)
	app.Post("/login", fiberutil.Validate(handler.Login))

	body := map[string]string{
		"email":    "nonexistent@example.com",
		"password": "some-password",
	}

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/login", body), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	r := parseResponse(resp)
	assert.False(t, r.Success)
	assert.Equal(t, "Invalid credentials", r.Message)
}

func TestAuthHandler_Login_ValidationError_MissingEmail(t *testing.T) {
	app, _, handler := setupAuthHandler(t)
	app.Post("/login", fiberutil.Validate(handler.Login))

	body := map[string]string{
		"password": "some-password",
	}

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/login", body), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)

	r := parseResponse(resp)
	assert.False(t, r.Success)

	var validationErrors map[string][]string
	_ = json.Unmarshal(r.Errors, &validationErrors)
	assert.Contains(t, validationErrors, "email")
}

func TestAuthHandler_Login_ValidationError_MissingPassword(t *testing.T) {
	app, _, handler := setupAuthHandler(t)
	app.Post("/login", fiberutil.Validate(handler.Login))

	body := map[string]string{
		"email": "test@example.com",
	}

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/login", body), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)

	r := parseResponse(resp)
	assert.False(t, r.Success)

	var validationErrors map[string][]string
	_ = json.Unmarshal(r.Errors, &validationErrors)
	assert.Contains(t, validationErrors, "password")
}

func TestAuthHandler_Login_CreatesSession(t *testing.T) {
	app, reg, handler := setupAuthHandler(t)
	app.Post("/login", fiberutil.Validate(handler.Login))

	hashedPassword, err := security.HashPassword("correct-password")
	require.NoError(t, err)

	user := newTestUser("user_001", "Test User", "test@example.com", hashedPassword)
	reg.user.users["user_001"] = user

	body := map[string]string{
		"email":    "test@example.com",
		"password": "correct-password",
	}

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/login", body), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	assert.NotEmpty(t, reg.session.sessions, "a session should be created after login")
}

// ============================================================================
// Logout tests
// ============================================================================

func TestAuthHandler_Logout_Success(t *testing.T) {
	app, reg, handler := setupAuthHandlerWithAuth(t, "user_001", "session_001")
	app.Post("/logout", handler.Logout)

	userID := "user_001"
	reg.session.sessions["session_001"] = &models.Session{
		ID:     "session_001",
		UserID: &userID,
	}

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/logout", nil), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	r := parseResponse(resp)
	assert.True(t, r.Success)
	assert.Equal(t, "Logged out successfully", r.Message)

	_, exists := reg.session.sessions["session_001"]
	assert.False(t, exists, "session should be deleted after logout")
}

func TestAuthHandler_Logout_NoUserID(t *testing.T) {
	app, _, handler := setupAuthHandler(t)
	app.Post("/logout", handler.Logout)

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/logout", nil), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	r := parseResponse(resp)
	assert.False(t, r.Success)
}

func TestAuthHandler_Logout_NoSessionID(t *testing.T) {
	app, _, handler := setupAuthHandlerWithAuth(t, "user_001", "")
	app.Post("/logout", handler.Logout)

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/logout", nil), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	r := parseResponse(resp)
	assert.True(t, r.Success)
	assert.Equal(t, "Logged out successfully", r.Message)
}

// ============================================================================
// RefreshToken tests
// ============================================================================

func TestAuthHandler_RefreshToken_Success(t *testing.T) {
	app, reg, handler := setupAuthHandler(t)
	app.Post("/refresh", fiberutil.Validate(handler.RefreshToken))

	user := newTestUser("user_001", "Test User", "test@example.com", "hashed")
	reg.user.users["user_001"] = user

	userID := "user_001"
	reg.session.sessions["session_001"] = &models.Session{
		ID:     "session_001",
		UserID: &userID,
	}

	tokenStr := generateRefreshToken("user_001", "session_001")

	body := map[string]string{
		"refresh_token": tokenStr,
	}

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/refresh", body), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	r := parseResponse(resp)
	assert.True(t, r.Success)
	assert.Equal(t, "Token refreshed successfully", r.Message)

	var data map[string]any
	_ = json.Unmarshal(r.Data, &data)
	assert.NotEmpty(t, data["access_token"])
	assert.NotEmpty(t, data["refresh_token"])
	assert.Equal(t, "Bearer", data["token_type"])
}

func TestAuthHandler_RefreshToken_InvalidToken(t *testing.T) {
	app, _, handler := setupAuthHandler(t)
	app.Post("/refresh", fiberutil.Validate(handler.RefreshToken))

	body := map[string]string{
		"refresh_token": "invalid-jwt-token",
	}

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/refresh", body), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	r := parseResponse(resp)
	assert.False(t, r.Success)
	assert.Equal(t, "Invalid token", r.Message)
}

func TestAuthHandler_RefreshToken_ValidationError_EmptyBody(t *testing.T) {
	app, _, handler := setupAuthHandler(t)
	app.Post("/refresh", fiberutil.Validate(handler.RefreshToken))

	body := map[string]string{}

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/refresh", body), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)

	r := parseResponse(resp)
	assert.False(t, r.Success)

	var validationErrors map[string][]string
	_ = json.Unmarshal(r.Errors, &validationErrors)
	assert.Contains(t, validationErrors, "refresh_token")
}

func TestAuthHandler_RefreshToken_ExpiredToken(t *testing.T) {
	app, _, handler := setupAuthHandler(t)
	app.Post("/refresh", fiberutil.Validate(handler.RefreshToken))

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":        "user_001",
		"type":       "refresh",
		"session_id": "session_001",
		"iat":        time.Now().Add(-2 * time.Hour).Unix(),
		"exp":        time.Now().Add(-1 * time.Hour).Unix(),
	})
	expiredToken, _ := token.SignedString([]byte(testJWTSecret))

	body := map[string]string{
		"refresh_token": expiredToken,
	}

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/refresh", body), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	r := parseResponse(resp)
	assert.False(t, r.Success)
	assert.Equal(t, "Invalid token", r.Message)
}

func TestAuthHandler_RefreshToken_WrongTokenType(t *testing.T) {
	app, _, handler := setupAuthHandler(t)
	app.Post("/refresh", fiberutil.Validate(handler.RefreshToken))

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":        "user_001",
		"type":       "access",
		"session_id": "session_001",
		"iat":        time.Now().Unix(),
		"exp":        time.Now().Add(time.Hour).Unix(),
	})
	accessToken, _ := token.SignedString([]byte(testJWTSecret))

	body := map[string]string{
		"refresh_token": accessToken,
	}

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/refresh", body), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	r := parseResponse(resp)
	assert.False(t, r.Success)
	assert.Equal(t, "Invalid token", r.Message)
}

func TestAuthHandler_RefreshToken_WrongSecret(t *testing.T) {
	app, _, handler := setupAuthHandler(t)
	app.Post("/refresh", fiberutil.Validate(handler.RefreshToken))

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":        "user_001",
		"type":       "refresh",
		"session_id": "session_001",
		"iat":        time.Now().Unix(),
		"exp":        time.Now().Add(time.Hour).Unix(),
	})
	tokenStr, _ := token.SignedString([]byte("wrong-secret-key-that-is-different"))

	body := map[string]string{
		"refresh_token": tokenStr,
	}

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/refresh", body), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	r := parseResponse(resp)
	assert.False(t, r.Success)
}

func TestAuthHandler_RefreshToken_RevokedSession(t *testing.T) {
	app, reg, handler := setupAuthHandler(t)
	app.Post("/refresh", fiberutil.Validate(handler.RefreshToken))

	user := newTestUser("user_001", "Test User", "test@example.com", "hashed")
	reg.user.users["user_001"] = user

	// Do NOT add session_001 to the mock -- it has been revoked

	tokenStr := generateRefreshToken("user_001", "session_001")

	body := map[string]string{
		"refresh_token": tokenStr,
	}

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/refresh", body), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	r := parseResponse(resp)
	assert.False(t, r.Success)
	assert.Equal(t, "Invalid token", r.Message)
}

func TestAuthHandler_RefreshToken_UserNotFound(t *testing.T) {
	app, reg, handler := setupAuthHandler(t)
	app.Post("/refresh", fiberutil.Validate(handler.RefreshToken))

	// Do NOT add the user to the mock -- user has been deleted

	userID := "user_001"
	reg.session.sessions["session_001"] = &models.Session{
		ID:     "session_001",
		UserID: &userID,
	}

	tokenStr := generateRefreshToken("user_001", "session_001")

	body := map[string]string{
		"refresh_token": tokenStr,
	}

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/refresh", body), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	r := parseResponse(resp)
	assert.False(t, r.Success)
	assert.Equal(t, "Invalid token", r.Message)
}
