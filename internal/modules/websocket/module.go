package websocket

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/websocket/handlers"
	"github.com/kkz6/launch-go/internal/pkg/app"
	ws "github.com/kkz6/launch-go/internal/websocket"
)

// Module is the WebSocket module that manages all WebSocket routes
type Module struct {
	hub             *ws.Hub
	terminalHandler *handlers.TerminalHandler
	logsHandler     *handlers.LogsHandler
	jwtSecret       string
}

// Ensure Module implements required interfaces
var _ app.Module = (*Module)(nil)
var _ app.WebSocketRegistrar = (*Module)(nil)
var _ app.Shutdownable = (*Module)(nil)

// NewModule creates a new WebSocket module
func NewModule(hub *ws.Hub, db *gorm.DB, jwtSecret string, logger *zerolog.Logger) *Module {
	return &Module{
		hub:             hub,
		terminalHandler: handlers.NewTerminalHandler(db, jwtSecret, *logger),
		logsHandler:     handlers.NewLogsHandler(db, jwtSecret, *logger),
		jwtSecret:       jwtSecret,
	}
}

// NewModuleFromContext creates a WebSocket module from app context
func NewModuleFromContext(ctx *app.Context) *Module {
	return NewModule(ctx.WebSocket, ctx.DB, ctx.Config.JWT.Secret, ctx.Logger)
}

// Name returns the module name
func (m *Module) Name() string {
	return "websocket"
}

// RegisterWebSocketRoutes registers all WebSocket routes
func (m *Module) RegisterWebSocketRoutes(router fiber.Router) {
	// Main WebSocket endpoint for pub/sub events
	router.Get("/ws", ws.Handler(m.hub, m.jwtSecret))

	// Terminal WebSocket endpoint
	router.Get("/terminal/ws", m.terminalHandler.Handler())

	// Log streaming WebSocket endpoint
	router.Get("/terminal/logs", m.logsHandler.Handler())
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
