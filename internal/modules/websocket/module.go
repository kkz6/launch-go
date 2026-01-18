package websocket

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/websocket/handlers"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/cache"
	"github.com/kkz6/launch-go/internal/pkg/module"
	ws "github.com/kkz6/launch-go/internal/websocket"
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
	module.Base
	hub                  *ws.Hub
	terminalHandler      *handlers.TerminalHandler
	logsHandler          *handlers.LogsHandler
	serviceStatusHandler *handlers.ServiceStatusHandler
	metricsHandler       *handlers.MetricsHandler
	jwtSecret            string
	membershipCache      *cache.TeamMembershipCache
}

// NewModule creates a new WebSocket module
func NewModule(b *module.Builder) *Module {
	deps := b.Deps()
	jwtSecret := deps.Config.JWT.Secret

	// Type assert to get the concrete Hub (only API server uses this module)
	hub, _ := deps.WebSocket.(*ws.Hub)

	return &Module{
		Base:                 module.NewBase(ModuleName, b),
		hub:                  hub,
		terminalHandler:      handlers.NewTerminalHandler(deps.DB, jwtSecret, *deps.Logger, deps.MembershipCache),
		logsHandler:          handlers.NewLogsHandler(deps.DB, jwtSecret, *deps.Logger, deps.MembershipCache),
		serviceStatusHandler: handlers.NewServiceStatusHandler(deps.DB, jwtSecret, *deps.Logger, deps.MembershipCache),
		metricsHandler:       handlers.NewMetricsHandler(deps.DB, jwtSecret, *deps.Logger, deps.MembershipCache),
		jwtSecret:            jwtSecret,
		membershipCache:      deps.MembershipCache,
	}
}

// RegisterWebSocketRoutes registers all WebSocket routes
func (m *Module) RegisterWebSocketRoutes(router fiber.Router) {
	// Main WebSocket endpoint for pub/sub events
	// Connection URL: /ws?token=xxx&team_id=xxx
	router.Get("/ws", ws.Handler(m.hub, m.jwtSecret, m.membershipCache))

	// Terminal WebSocket endpoint
	router.Get("/terminal/ws", m.terminalHandler.Handler())

	// Log streaming WebSocket endpoint
	router.Get("/terminal/logs", m.logsHandler.Handler())

	// Service status monitoring WebSocket endpoint
	router.Get("/services/status", m.serviceStatusHandler.Handler())

	// Metrics streaming WebSocket endpoint
	router.Get("/metrics/stream", m.metricsHandler.Handler())
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
