package fiber

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withAuthContext(app *fiber.App, teamID, userID string) {
	app.Use(func(c *fiber.Ctx) error {
		if teamID != "" {
			c.Locals(KeyTeamID, teamID)
		}
		if userID != "" {
			c.Locals(KeyUserID, userID)
		}
		return c.Next()
	})
}

func TestHandler_InjectsTeamAndUser(t *testing.T) {
	var gotTeam, gotUser, gotParam string
	app := fiber.New()
	withAuthContext(app, "team-1", "user-9")
	app.Post("/x/:id", Handler(func(s *Request) error {
		gotTeam = s.TeamID
		gotUser = s.UserID
		gotParam = s.Params("id") // embedded *fiber.Ctx works
		return s.SendStatus(fiber.StatusOK)
	}))

	resp, err := app.Test(httptest.NewRequest("POST", "/x/abc", nil))
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "team-1", gotTeam)
	assert.Equal(t, "user-9", gotUser)
	assert.Equal(t, "abc", gotParam)
}

func TestHandler_MissingTeamRejected(t *testing.T) {
	called := false
	app := fiber.New()
	withAuthContext(app, "", "user-9") // no team
	app.Post("/x", Handler(func(s *Request) error {
		called = true
		return s.SendStatus(fiber.StatusOK)
	}))

	resp, err := app.Test(httptest.NewRequest("POST", "/x", nil))
	require.NoError(t, err)
	assert.False(t, called, "handler must not run without team context")
	assert.NotEqual(t, fiber.StatusOK, resp.StatusCode)
}

type bindTestReq struct {
	Name string `json:"name" validate:"required"`
}

func TestBind_InjectsRequestAndBody(t *testing.T) {
	var gotTeam, gotName string
	app := fiber.New()
	withAuthContext(app, "team-2", "user-3")
	app.Post("/x", Bind(func(s *Request, req *bindTestReq) error {
		gotTeam = s.TeamID
		gotName = req.Name
		return s.SendStatus(fiber.StatusOK)
	}))

	httpReq := httptest.NewRequest("POST", "/x", strings.NewReader(`{"name":"bob"}`))
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(httpReq)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "team-2", gotTeam)
	assert.Equal(t, "bob", gotName)
}

func TestBind_RejectsInvalidBody(t *testing.T) {
	called := false
	app := fiber.New()
	withAuthContext(app, "team-2", "user-3")
	app.Post("/x", Bind(func(s *Request, req *bindTestReq) error {
		called = true
		return s.SendStatus(fiber.StatusOK)
	}))

	httpReq := httptest.NewRequest("POST", "/x", strings.NewReader(`{}`))
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(httpReq)
	require.NoError(t, err)
	assert.False(t, called)
	assert.NotEqual(t, fiber.StatusOK, resp.StatusCode)
}
