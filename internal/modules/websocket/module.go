package websocket

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/websocket/handlers"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/module"
	ws "github.com/kkz6/launch-go/internal/websocket"
)

const ModuleName = "websocket"

// Ensure Module implements required interfaces
var (
	_ app.Module            = (*Module)(nil)
	_ app.WebSocketRegistrar = (*Module)(nil)
	_ app.Shutdownable      = (*Module)(nil)
)

// Module is the WebSocket module that manages all WebSocket routes
type Module struct {
	module.Base
	hub                  *ws.Hub
	terminalHandler      *handlers.TerminalHandler
	logsHandler          *handlers.LogsHandler
	serviceStatusHandler *handlers.ServiceStatusHandler
	jwtSecret            string
}

// NewModule creates a new WebSocket module
func NewModule(b *module.Builder) *Module {
	deps := b.Deps()
	jwtSecret := deps.Config.JWT.Secret

	return &Module{
		Base:                 module.NewBase(ModuleName, b),
		hub:                  deps.WebSocket,
		terminalHandler:      handlers.NewTerminalHandler(deps.DB, jwtSecret, *deps.Logger),
		logsHandler:          handlers.NewLogsHandler(deps.DB, jwtSecret, *deps.Logger),
		serviceStatusHandler: handlers.NewServiceStatusHandler(deps.DB, jwtSecret, *deps.Logger),
		jwtSecret:            jwtSecret,
	}
}

// RegisterWebSocketRoutes registers all WebSocket routes
func (m *Module) RegisterWebSocketRoutes(router fiber.Router) {
	// Main WebSocket endpoint for pub/sub events
	router.Get("/ws", ws.Handler(m.hub, m.jwtSecret))

	// Terminal WebSocket endpoint
	router.Get("/terminal/ws", m.terminalHandler.Handler())

	// Log streaming WebSocket endpoint
	router.Get("/terminal/logs", m.logsHandler.Handler())

	// Service status monitoring WebSocket endpoint
	router.Get("/services/status", m.serviceStatusHandler.Handler())
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
