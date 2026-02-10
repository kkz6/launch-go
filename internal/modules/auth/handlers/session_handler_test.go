package handlers_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/auth/handlers"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
)

func setupSessionHandlerWithAuth(userID, sessionID string) (*fiber.App, *mockRepoRegistry, *handlers.SessionHandler) {
	reg := newMockRegistry()
	handler := handlers.NewSessionHandler(reg.session)
	app := newTestApp()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("userID", userID)
		if sessionID != "" {
			c.Locals("sessionID", sessionID)
		}
		return c.Next()
	})
	return app, reg, handler
}

func TestSessionHandler_List_Success(t *testing.T) {
	app, reg, handler := setupSessionHandlerWithAuth("user_001", "session_001")
	app.Get("/sessions", handler.List)

	userID := "user_001"
	ua := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 Chrome/120.0"
	ip := "192.168.1.1"
	reg.session.sessions["session_001"] = &models.Session{
		ID:           "session_001",
		UserID:       &userID,
		UserAgent:    &ua,
		IPAddress:    &ip,
		LastActivity: int(time.Now().Unix()),
	}
	reg.session.sessions["session_002"] = &models.Session{
		ID:           "session_002",
		UserID:       &userID,
		LastActivity: int(time.Now().Unix()),
	}

	resp, err := app.Test(makeJSONRequest(http.MethodGet, "/sessions", nil))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	r := parseResponse(resp)
	assert.True(t, r.Success)
	assert.Equal(t, "Sessions retrieved", r.Message)
}

func TestSessionHandler_List_MarksCurrent(t *testing.T) {
	app, reg, handler := setupSessionHandlerWithAuth("user_001", "session_001")
	app.Get("/sessions", handler.List)

	userID := "user_001"
	reg.session.sessions["session_001"] = &models.Session{
		ID:           "session_001",
		UserID:       &userID,
		LastActivity: int(time.Now().Unix()),
	}

	resp, err := app.Test(makeJSONRequest(http.MethodGet, "/sessions", nil))
	require.NoError(t, err)

	r := parseResponse(resp)
	assert.True(t, r.Success)

	var sessions []map[string]any
	_ = json.Unmarshal(r.Data, &sessions)
	require.Len(t, sessions, 1)
	assert.True(t, sessions[0]["is_current_device"].(bool))
}

func TestSessionHandler_List_Empty(t *testing.T) {
	app, _, handler := setupSessionHandlerWithAuth("user_001", "session_001")
	app.Get("/sessions", handler.List)

	resp, err := app.Test(makeJSONRequest(http.MethodGet, "/sessions", nil))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	r := parseResponse(resp)
	assert.True(t, r.Success)
}

func TestSessionHandler_List_NoUserID(t *testing.T) {
	app := newTestApp()
	reg := newMockRegistry()
	handler := handlers.NewSessionHandler(reg.session)
	app.Get("/sessions", handler.List)

	resp, err := app.Test(makeJSONRequest(http.MethodGet, "/sessions", nil))
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestSessionHandler_Revoke_Success(t *testing.T) {
	app, reg, handler := setupSessionHandlerWithAuth("user_001", "session_001")
	app.Delete("/sessions/:id", handler.Revoke)

	userID := "user_001"
	reg.session.sessions["session_002"] = &models.Session{
		ID:     "session_002",
		UserID: &userID,
	}

	resp, err := app.Test(makeJSONRequest(http.MethodDelete, "/sessions/session_002", nil))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	r := parseResponse(resp)
	assert.True(t, r.Success)
	assert.Equal(t, "Session revoked", r.Message)

	_, exists := reg.session.sessions["session_002"]
	assert.False(t, exists)
}

func TestSessionHandler_Revoke_NotFound(t *testing.T) {
	app, _, handler := setupSessionHandlerWithAuth("user_001", "session_001")
	app.Delete("/sessions/:id", handler.Revoke)

	resp, err := app.Test(makeJSONRequest(http.MethodDelete, "/sessions/nonexistent", nil))
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestSessionHandler_RevokeOthers_Success(t *testing.T) {
	app, reg, handler := setupSessionHandlerWithAuth("user_001", "session_001")
	app.Delete("/sessions", handler.RevokeOthers)

	userID := "user_001"
	reg.session.sessions["session_001"] = &models.Session{ID: "session_001", UserID: &userID}
	reg.session.sessions["session_002"] = &models.Session{ID: "session_002", UserID: &userID}
	reg.session.sessions["session_003"] = &models.Session{ID: "session_003", UserID: &userID}

	resp, err := app.Test(makeJSONRequest(http.MethodDelete, "/sessions", nil))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	r := parseResponse(resp)
	assert.True(t, r.Success)

	// Current session should remain
	_, exists := reg.session.sessions["session_001"]
	assert.True(t, exists)

	// Others should be deleted
	_, exists = reg.session.sessions["session_002"]
	assert.False(t, exists)
	_, exists = reg.session.sessions["session_003"]
	assert.False(t, exists)
}

func TestSessionHandler_RevokeOthers_NoCurrentSession(t *testing.T) {
	app, _, handler := setupSessionHandlerWithAuth("user_001", "")
	app.Delete("/sessions", handler.RevokeOthers)

	resp, err := app.Test(makeJSONRequest(http.MethodDelete, "/sessions", nil))
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// parseUserAgent tests
func TestParseUserAgent(t *testing.T) {
	tests := []struct {
		name            string
		ua              string
		expectBrowser   string
		expectPlatform  string
		expectIsDesktop bool
	}{
		{
			name:            "Chrome on macOS",
			ua:              "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			expectBrowser:   "Chrome",
			expectPlatform:  "macOS",
			expectIsDesktop: true,
		},
		{
			name:            "Firefox on Linux",
			ua:              "Mozilla/5.0 (X11; Linux x86_64; rv:120.0) Gecko/20100101 Firefox/120.0",
			expectBrowser:   "Firefox",
			expectPlatform:  "Linux",
			expectIsDesktop: true,
		},
		{
			name:            "Safari on iOS",
			ua:              "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
			expectBrowser:   "Safari",
			expectPlatform:  "iOS",
			expectIsDesktop: false,
		},
		{
			name:            "Chrome on Android mobile",
			ua:              "Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Mobile Safari/537.36",
			expectBrowser:   "Chrome",
			expectPlatform:  "Android",
			expectIsDesktop: false,
		},
		{
			name:            "curl",
			ua:              "curl/8.4.0",
			expectBrowser:   "curl",
			expectPlatform:  "Unknown",
			expectIsDesktop: true,
		},
		{
			name:            "empty user agent",
			ua:              "",
			expectBrowser:   "Unknown",
			expectPlatform:  "Unknown",
			expectIsDesktop: true,
		},
		{
			name:            "Edge on Windows",
			ua:              "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
			expectBrowser:   "Edge",
			expectPlatform:  "Windows",
			expectIsDesktop: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// parseUserAgent is unexported, test it indirectly via session listing
			app, reg, handler := setupSessionHandlerWithAuth("user_001", "session_001")
			app.Get("/sessions", handler.List)

			userID := "user_001"
			reg.session.sessions["session_001"] = &models.Session{
				ID:           "session_001",
				UserID:       &userID,
				UserAgent:    &tt.ua,
				LastActivity: int(time.Now().Unix()),
			}

			if tt.ua == "" {
				reg.session.sessions["session_001"].UserAgent = nil
			}

			resp, err := app.Test(makeJSONRequest(http.MethodGet, "/sessions", nil))
			require.NoError(t, err)

			r := parseResponse(resp)
			require.True(t, r.Success)

			var sessions []map[string]any
			_ = json.Unmarshal(r.Data, &sessions)
			require.Len(t, sessions, 1)

			agent := sessions[0]["agent"].(map[string]any)
			assert.Equal(t, tt.expectBrowser, agent["browser"])
			assert.Equal(t, tt.expectPlatform, agent["platform"])
			assert.Equal(t, tt.expectIsDesktop, agent["is_desktop"])
		})
	}
}
