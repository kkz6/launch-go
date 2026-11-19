package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/stretchr/testify/require"
)

func TestGitHubCandidatesRejectInvalidPageBeforeCallingService(t *testing.T) {
	app := fiber.New()
	handler := NewSourceControlHandler(nil)
	app.Get("/candidates", func(c *fiber.Ctx) error {
		return handler.GitHubCandidates(&fiberctx.Request{Ctx: c})
	})
	for _, value := range []string{"", "0", "-1", "10001", "abc", "1.5", "9999999999999999999999"} {
		t.Run(value, func(t *testing.T) {
			response, err := app.Test(httptest.NewRequest(http.MethodGet, "/candidates?page="+value, nil))
			require.NoError(t, err)
			defer response.Body.Close()
			require.Equal(t, http.StatusBadRequest, response.StatusCode)
		})
	}
}
