package handlers_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/auth/handlers"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

func setupPATHandler(userID string) (*fiber.App, *mockRepoRegistry, *handlers.PATHandler) {
	reg := newMockRegistry()
	cfg := &config.Config{
		JWT: config.JWTConfig{Secret: "test-secret"},
		App: config.AppConfig{Name: "TestApp"},
	}
	authSvc := services.NewAuthService(reg, cfg, nil, newMockCache())
	twoFactorSvc := services.NewTwoFactorService(reg, cfg, authSvc)
	handler := handlers.NewPATHandler(reg.pat, twoFactorSvc, reg.user)

	app := fiber.New(fiber.Config{
		ErrorHandler: fiberutil.NewErrorHandler(),
	})
	app.Use(func(c *fiber.Ctx) error {
		if userID != "" {
			c.Locals("userID", userID)
		}
		return c.Next()
	})

	return app, reg, handler
}

func TestPATHandler_List_Success(t *testing.T) {
	app, reg, handler := setupPATHandler("user_001")
	app.Get("/tokens", handler.List)

	now := time.Now()
	abilities := `["read","write"]`
	reg.pat.tokens["pat_001"] = &models.PersonalAccessToken{
		TokenableType: "User",
		TokenableID:   "user_001",
		Name:          "My Token",
		Token:         "hashed_token_1",
		Abilities:     &abilities,
		ExpiresAt:     &now,
	}
	reg.pat.tokens["pat_001"].ID = "pat_001"
	reg.pat.tokens["pat_001"].CreatedAt = &now

	reg.pat.tokens["pat_002"] = &models.PersonalAccessToken{
		TokenableType: "User",
		TokenableID:   "user_001",
		Name:          "Another Token",
		Token:         "hashed_token_2",
	}
	reg.pat.tokens["pat_002"].ID = "pat_002"

	resp, err := app.Test(makeJSONRequest(http.MethodGet, "/tokens", nil), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	r := parseResponse(resp)
	assert.True(t, r.Success)
	assert.Equal(t, "Tokens retrieved", r.Message)

	var tokens []map[string]any
	require.NoError(t, json.Unmarshal(r.Data, &tokens))
	assert.Len(t, tokens, 2)
}

func TestPATHandler_List_Empty(t *testing.T) {
	app, _, handler := setupPATHandler("user_001")
	app.Get("/tokens", handler.List)

	resp, err := app.Test(makeJSONRequest(http.MethodGet, "/tokens", nil), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	r := parseResponse(resp)
	assert.True(t, r.Success)
	assert.Equal(t, "Tokens retrieved", r.Message)

	var tokens []map[string]any
	require.NoError(t, json.Unmarshal(r.Data, &tokens))
	assert.Empty(t, tokens)
}

func TestPATHandler_List_NoUserID(t *testing.T) {
	app, _, handler := setupPATHandler("")
	app.Get("/tokens", handler.List)

	resp, err := app.Test(makeJSONRequest(http.MethodGet, "/tokens", nil), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestPATHandler_Create_Success_No2FA(t *testing.T) {
	app, reg, handler := setupPATHandler("user_001")
	app.Post("/tokens", handler.Create)

	reg.user.users["user_001"] = newTestUser("user_001", "Test User", "test@example.com", "hashed")

	body := map[string]any{
		"name": "My API Token",
	}

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/tokens", body), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	r := parseResponse(resp)
	assert.True(t, r.Success)
	assert.Equal(t, "Token created", r.Message)

	var data map[string]any
	require.NoError(t, json.Unmarshal(r.Data, &data))
	assert.NotEmpty(t, data["plain_text_token"])
	assert.Equal(t, "My API Token", data["name"])

	abilities, ok := data["abilities"].([]any)
	require.True(t, ok)
	assert.Equal(t, []any{"*"}, abilities)
}

func TestPATHandler_Create_WithScopes(t *testing.T) {
	app, reg, handler := setupPATHandler("user_001")
	app.Post("/tokens", handler.Create)

	reg.user.users["user_001"] = newTestUser("user_001", "Test User", "test@example.com", "hashed")

	body := map[string]any{
		"name":   "Scoped Token",
		"scopes": []string{"read", "write"},
	}

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/tokens", body), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	r := parseResponse(resp)
	assert.True(t, r.Success)

	var data map[string]any
	require.NoError(t, json.Unmarshal(r.Data, &data))

	abilities, ok := data["abilities"].([]any)
	require.True(t, ok)
	assert.Len(t, abilities, 2)
	assert.Equal(t, "read", abilities[0])
	assert.Equal(t, "write", abilities[1])
}

func TestPATHandler_Create_WithExpiresAt(t *testing.T) {
	app, reg, handler := setupPATHandler("user_001")
	app.Post("/tokens", handler.Create)

	reg.user.users["user_001"] = newTestUser("user_001", "Test User", "test@example.com", "hashed")

	expiresAt := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	body := map[string]any{
		"name":       "Expiring Token",
		"expires_at": expiresAt,
	}

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/tokens", body), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	r := parseResponse(resp)
	assert.True(t, r.Success)

	var data map[string]any
	require.NoError(t, json.Unmarshal(r.Data, &data))
	assert.NotNil(t, data["expires_at"])
}

func TestPATHandler_Create_ValidationError_MissingName(t *testing.T) {
	app, reg, handler := setupPATHandler("user_001")
	app.Post("/tokens", handler.Create)

	reg.user.users["user_001"] = newTestUser("user_001", "Test User", "test@example.com", "hashed")

	body := map[string]any{
		"scopes": []string{"read"},
	}

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/tokens", body), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)

	r := parseResponse(resp)
	assert.False(t, r.Success)

	var errs map[string][]string
	require.NoError(t, json.Unmarshal(r.Errors, &errs))
	assert.Contains(t, errs, "name")
}

func TestPATHandler_Create_InvalidExpiresAtFormat(t *testing.T) {
	app, reg, handler := setupPATHandler("user_001")
	app.Post("/tokens", handler.Create)

	reg.user.users["user_001"] = newTestUser("user_001", "Test User", "test@example.com", "hashed")

	body := map[string]any{
		"name":       "Bad Date Token",
		"expires_at": "not-a-date",
	}

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/tokens", body), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	r := parseResponse(resp)
	assert.False(t, r.Success)
	assert.Contains(t, r.Message, "Invalid expires_at format")
}

func TestPATHandler_Create_2FA_RequiredNoCode(t *testing.T) {
	app, reg, handler := setupPATHandler("user_001")
	app.Post("/tokens", handler.Create)

	now := time.Now()
	secret := "JBSWY3DPEHPK3PXP"
	user := newTestUser("user_001", "2FA User", "2fa@example.com", "hashed")
	user.TwoFactorSecret = &secret
	user.TwoFactorConfirmedAt = &now
	reg.user.users["user_001"] = user

	body := map[string]any{
		"name": "Token Without Code",
	}

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/tokens", body), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)

	r := parseResponse(resp)
	assert.False(t, r.Success)

	var errs map[string][]string
	require.NoError(t, json.Unmarshal(r.Errors, &errs))
	assert.Contains(t, errs, "code")
	assert.Contains(t, errs["code"][0], "Two-factor authentication code is required")
}

func TestPATHandler_Create_2FA_InvalidCode(t *testing.T) {
	app, reg, handler := setupPATHandler("user_001")
	app.Post("/tokens", handler.Create)

	now := time.Now()
	secret := "JBSWY3DPEHPK3PXP"
	user := newTestUser("user_001", "2FA User", "2fa@example.com", "hashed")
	user.TwoFactorSecret = &secret
	user.TwoFactorConfirmedAt = &now
	reg.user.users["user_001"] = user

	body := map[string]any{
		"name": "Token With Bad Code",
		"code": "000000",
	}

	resp, err := app.Test(makeJSONRequest(http.MethodPost, "/tokens", body), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)

	r := parseResponse(resp)
	assert.False(t, r.Success)

	var errs map[string][]string
	require.NoError(t, json.Unmarshal(r.Errors, &errs))
	assert.Contains(t, errs, "code")
	assert.Contains(t, errs["code"][0], "Invalid two-factor authentication code")
}

func TestPATHandler_Delete_Success(t *testing.T) {
	app, reg, handler := setupPATHandler("user_001")
	app.Delete("/tokens/:id", handler.Delete)

	reg.pat.tokens["pat_001"] = &models.PersonalAccessToken{
		TokenableType: "User",
		TokenableID:   "user_001",
		Name:          "To Delete",
		Token:         "hashed_token",
	}
	reg.pat.tokens["pat_001"].ID = "pat_001"

	resp, err := app.Test(makeJSONRequest(http.MethodDelete, "/tokens/pat_001", nil), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	_, exists := reg.pat.tokens["pat_001"]
	assert.False(t, exists)
}

func TestPATHandler_Delete_NotFound(t *testing.T) {
	app, _, handler := setupPATHandler("user_001")
	app.Delete("/tokens/:id", handler.Delete)

	resp, err := app.Test(makeJSONRequest(http.MethodDelete, "/tokens/nonexistent", nil), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	r := parseResponse(resp)
	assert.False(t, r.Success)
	assert.Equal(t, "Token not found", r.Message)
}

func TestPATHandler_Delete_MissingID(t *testing.T) {
	app, _, handler := setupPATHandler("user_001")
	// Register without :id param to simulate missing ID
	app.Delete("/tokens/", handler.Delete)

	resp, err := app.Test(makeJSONRequest(http.MethodDelete, "/tokens/", nil), testTimeout)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	r := parseResponse(resp)
	assert.False(t, r.Success)
	assert.Contains(t, r.Message, "Token ID is required")
}
