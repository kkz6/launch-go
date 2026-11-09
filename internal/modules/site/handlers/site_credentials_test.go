package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/site/models"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

func TestSiteDeploymentCredentialsRespectRequestPermissions(t *testing.T) {
	for _, tc := range []struct {
		name, userID, teamID, role string
		readOnly, allowed          bool
	}{
		{name: "member", userID: "user", teamID: "team", role: "member"},
		{name: "editor", userID: "user", teamID: "team", role: "editor", allowed: true},
		{name: "admin", userID: "user", teamID: "team", role: "admin", allowed: true},
		{name: "owner", userID: "user", teamID: "team", role: "owner", allowed: true},
		{name: "spectating owner", userID: "user", teamID: "team", role: "owner", readOnly: true},
		{name: "unknown role", userID: "user", teamID: "team", role: "unknown"},
		{name: "missing role", userID: "user", teamID: "team"},
		{name: "other team", userID: "user", teamID: "other", role: "owner"},
		{name: "missing team", userID: "user", role: "owner"},
		{name: "missing user", teamID: "team", role: "owner"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			token := "test-deployment-secret"
			site := &models.Site{DeployToken: &token}
			site.ID, site.TeamID = "site", "team"
			app := fiber.New()
			app.Get("/site", func(c *fiber.Ctx) error {
				c.Locals(fiberctx.KeyUserID, tc.userID)
				fiberctx.SetTeamContext(c, tc.teamID, tc.role)
				c.Locals(fiberctx.KeyImpersonationReadOnly, tc.readOnly)
				return c.JSON(siteResponseForRequest(&fiberctx.Request{Ctx: c}, site))
			})
			response, err := app.Test(httptest.NewRequest("GET", "/site", nil))
			require.NoError(t, err)
			defer response.Body.Close()
			var body map[string]any
			require.NoError(t, json.NewDecoder(response.Body).Decode(&body))
			require.Equal(t, "site", body["id"])
			if tc.allowed {
				require.Equal(t, token, body["deploy_token"])
				require.Contains(t, body["deploy_webhook_url"], token)
			} else {
				require.NotContains(t, body, "deploy_token")
				require.NotContains(t, body, "deploy_webhook_url")
			}
		})
	}
}
