package auth

import (
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/auth/handlers"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
)

func TestRegisterTeamRoutesIncludesValidatedDelete(t *testing.T) {
	app := fiber.New()
	service := &services.Service{}
	module := &Module{}
	module.registerTeamRoutes(
		app.Group("/teams"),
		handlers.NewHandler(service),
		NewMiddlewareAdapter(service),
	)

	found := false
	for _, route := range app.GetRoutes() {
		if route.Method == fiber.MethodDelete && route.Path == "/teams/:teamId" {
			found = true
			break
		}
	}
	require.True(t, found)
}
