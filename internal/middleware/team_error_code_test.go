package middleware

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/i18n"
	launchcache "github.com/kkz6/launch-go/internal/pkg/launch/cache"
)

func TestTeamScopeReturnsStableLocalizedErrorCodes(t *testing.T) {
	store := newMemCache()
	require.NoError(t, store.Set(
		context.Background(),
		"team_membership:user-1:team-1",
		`{"team_id":"team-1","user_id":"user-1","is_member":false}`,
		time.Minute,
	))
	InitTeamMiddleware(launchcache.NewTeamMembershipCache(store, nil))
	t.Cleanup(func() { InitTeamMiddleware(nil) })

	app := fiber.New()
	app.Use(Locale(i18n.LocaleEnglish))
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(fiberutil.KeyUserID, "user-1")
		return c.Next()
	})
	app.Get("/guarded", TeamScope(), func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	tests := []struct {
		name    string
		teamID  string
		status  int
		code    string
		english string
	}{
		{name: "missing context", status: fiber.StatusBadRequest, code: "team.context_required", english: "X-Team-ID header is required"},
		{name: "not a member", teamID: "team-1", status: fiber.StatusForbidden, code: "team.not_member", english: "You are not a member of this team"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/guarded", nil)
			req.Header.Set("Accept-Language", "ja")
			if tt.teamID != "" {
				req.Header.Set("X-Team-ID", tt.teamID)
			}
			resp, err := app.Test(req)
			require.NoError(t, err)
			assert.Equal(t, tt.status, resp.StatusCode)

			var body fiberutil.ErrorResponse
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
			assert.Equal(t, tt.code, body.Code)
			assert.NotEqual(t, tt.english, body.Message)
		})
	}
}
