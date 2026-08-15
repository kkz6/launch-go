package websocket

import (
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/pkg/app"
	ws "github.com/kkz6/launch-go/internal/pkg/websocket"
)

func TestModuleRegistersWebSocketRoutesAndShutsDown(t *testing.T) {
	logger := zerolog.Nop()
	hub := ws.NewHub()
	builder := app.NewBuilder(app.Deps{
		Config: &config.Config{},
		Logger: &logger,
	})
	module := NewModule(builder, hub)

	require.Equal(t, ModuleName, module.Name())
	require.Same(t, hub, module.Hub())
	require.NotNil(t, module.channelAuthorizer)

	router := fiber.New()
	module.RegisterWebSocketRoutes(router.Group("/api"))
	routes := make(map[string]bool)
	for _, route := range router.GetRoutes() {
		routes[route.Method+" "+route.Path] = true
	}
	for _, path := range []string{
		"/api/ws",
		"/api/terminal/ws",
		"/api/terminal/logs",
		"/api/services/status",
		"/api/metrics/stream",
		"/api/scripts/execute",
		"/api/docker/applications/logs",
	} {
		require.True(t, routes[fiber.MethodGet+" "+path], path)
	}

	require.NoError(t, module.Shutdown())
}
