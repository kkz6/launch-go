package server

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/pkg/app"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/signedurl"
)

func TestCustomProvisionWebhookRequiresSignature(t *testing.T) {
	module := NewModule(app.NewBuilder(app.Deps{Config: &config.Config{
		App: config.AppConfig{FrontendURL: "https://app.example.test"},
	}}))
	router := fiber.New(fiber.Config{ErrorHandler: fiberutil.NewErrorHandler()})
	module.RegisterWebhookRoutes(router)
	previous := signedurl.GetDefaultSigner()
	signedurl.SetDefaultSigner(signedurl.NewSigner("route-signing-test"))
	t.Cleanup(func() { signedurl.SetDefaultSigner(previous) })

	path := "/webhooks/servers/invalid/provision"
	for _, tt := range []struct {
		url  string
		want int
	}{
		{path, fiber.StatusForbidden},
		{signedurl.PermanentSign(path, nil), fiber.StatusBadRequest},
	} {
		response, err := router.Test(httptest.NewRequest("POST", tt.url, nil))
		require.NoError(t, err)
		require.Equal(t, tt.want, response.StatusCode)
		require.NoError(t, response.Body.Close())
	}
}
