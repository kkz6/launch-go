package websocket

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/websocket/handlers"
	"github.com/kkz6/launch-go/internal/pkg/app"
	launchcache "github.com/kkz6/launch-go/internal/pkg/launch/cache"
	ws "github.com/kkz6/launch-go/internal/pkg/websocket"
)

const ModuleName = "websocket"

// Ensure Module implements required interfaces
var (
	_ app.Module             = (*Module)(nil)
	_ app.WebSocketRegistrar = (*Module)(nil)
	_ app.Shutdownable       = (*Module)(nil)
)

// Module is the WebSocket module that manages all WebSocket routes
type Module struct {
	app.Base
	hub                    *ws.Hub
	terminalHandler        *handlers.TerminalHandler
	logsHandler            *handlers.LogsHandler
	serviceStatusHandler   *handlers.ServiceStatusHandler
	metricsHandler         *handlers.MetricsHandler
	scriptExecutionHandler *handlers.ScriptExecutionHandler
	dockerLogsHandler      *handlers.DockerLogsHandler
	jwtSecret              string
	membershipCache        *launchcache.TeamMembershipCache
}

// NewModule creates a new WebSocket module. The hub is deliberately passed
// separately from app dependencies: application modules publish events via
// the shared Redis broadcaster, while this module alone owns socket clients.
func NewModule(b *app.Builder, hub *ws.Hub) *Module {
	deps := b.Deps()
	jwtSecret := deps.Config.JWT.Secret

	// Create shared base for all WebSocket handlers
	handlerBase := handlers.NewBase(deps.DB, jwtSecret, *deps.Logger, deps.MembershipCache)

	return &Module{
		Base:                   app.NewBase(ModuleName, b),
		hub:                    hub,
		terminalHandler:        handlers.NewTerminalHandler(handlerBase),
		logsHandler:            handlers.NewLogsHandler(handlerBase),
		serviceStatusHandler:   handlers.NewServiceStatusHandler(handlerBase),
		metricsHandler:         handlers.NewMetricsHandler(handlerBase),
		scriptExecutionHandler: handlers.NewScriptExecutionHandler(handlerBase),
		dockerLogsHandler:      handlers.NewDockerLogsHandler(handlerBase),
		jwtSecret:              jwtSecret,
		membershipCache:        deps.MembershipCache,
	}
}

// RegisterWebSocketRoutes registers all WebSocket routes
func (m *Module) RegisterWebSocketRoutes(router fiber.Router) {
	// Main WebSocket endpoint for pub/sub events
	// Connection URL: /api/ws?token=xxx&team_id=xxx
	router.Get("/ws", ws.Handler(m.hub, m.jwtSecret, m.membershipCache))

	// Terminal WebSocket endpoint
	router.Get("/terminal/ws", m.terminalHandler.Handler())

	// Log streaming WebSocket endpoint
	router.Get("/terminal/logs", m.logsHandler.Handler())

	// Service status monitoring WebSocket endpoint
	router.Get("/services/status", m.serviceStatusHandler.Handler())

	// Metrics streaming WebSocket endpoint
	router.Get("/metrics/stream", m.metricsHandler.Handler())

	// Script execution streaming WebSocket endpoint
	// Connection URL: /api/scripts/execute?executionId=xxx&token=xxx&team_id=xxx
	router.Get("/scripts/execute", m.scriptExecutionHandler.Handler())

	// Docker container logs streaming WebSocket endpoint.
	// Connection URL: /api/docker/applications/logs?applicationId=xxx&token=xxx
	// Sends each `docker logs -f` line as a TextMessage frame.
	router.Get("/docker/applications/logs", m.dockerLogsHandler.Handler())
}

// Shutdown gracefully shuts down the WebSocket module
func (m *Module) Shutdown() error {
	m.hub.Shutdown()
	return nil
}

// Hub returns the WebSocket hub for external access
func (m *Module) Hub() *ws.Hub {
	return m.hub
}
