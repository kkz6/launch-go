package handlers

import (
	"github.com/gofiber/contrib/websocket"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/cache"
	ws "github.com/kkz6/launch-go/internal/websocket"
)

// Base provides common dependencies and functionality for all WebSocket handlers.
// Embed this struct in specific handlers to get access to shared resources.
type Base struct {
	DB              *gorm.DB
	JWTSecret       string
	Logger          zerolog.Logger
	MembershipCache *cache.TeamMembershipCache
}

// NewBase creates a new Base handler with all dependencies.
func NewBase(db *gorm.DB, jwtSecret string, logger zerolog.Logger, membershipCache *cache.TeamMembershipCache) Base {
	return Base{
		DB:              db,
		JWTSecret:       jwtSecret,
		Logger:          logger,
		MembershipCache: membershipCache,
	}
}

// WithComponent returns a copy of the base with a component-specific logger.
func (b Base) WithComponent(component string) Base {
	return Base{
		DB:              b.DB,
		JWTSecret:       b.JWTSecret,
		Logger:          b.Logger.With().Str("component", component).Logger(),
		MembershipCache: b.MembershipCache,
	}
}

// Authenticate validates the WebSocket connection using JWT and team membership.
// Returns the claims on success, or an error on failure.
func (b *Base) Authenticate(c *websocket.Conn) (*ws.Claims, error) {
	return ws.AuthenticateWebSocket(c, b.JWTSecret, b.MembershipCache)
}

// SendError sends an error message to the WebSocket client and logs it.
func (b *Base) SendError(c *websocket.Conn, msg string) {
	b.Logger.Warn().Msg(msg)
	_ = c.WriteMessage(websocket.TextMessage, []byte(msg))
}

// SendJSON sends a JSON message to the WebSocket client.
// Returns an error if marshaling or sending fails.
func (b *Base) SendJSON(c *websocket.Conn, v any) error {
	return c.WriteJSON(v)
}

// LogInfo logs an info message with connection context.
func (b *Base) LogInfo(msg string, fields ...any) {
	event := b.Logger.Info()
	for i := 0; i < len(fields)-1; i += 2 {
		if key, ok := fields[i].(string); ok {
			event = event.Interface(key, fields[i+1])
		}
	}
	event.Msg(msg)
}

// LogWarn logs a warning message with connection context.
func (b *Base) LogWarn(msg string, fields ...any) {
	event := b.Logger.Warn()
	for i := 0; i < len(fields)-1; i += 2 {
		if key, ok := fields[i].(string); ok {
			event = event.Interface(key, fields[i+1])
		}
	}
	event.Msg(msg)
}

// LogError logs an error message with connection context.
func (b *Base) LogError(err error, msg string, fields ...any) {
	event := b.Logger.Error().Err(err)
	for i := 0; i < len(fields)-1; i += 2 {
		if key, ok := fields[i].(string); ok {
			event = event.Interface(key, fields[i+1])
		}
	}
	event.Msg(msg)
}

// LogDebug logs a debug message with connection context.
func (b *Base) LogDebug(msg string, fields ...any) {
	event := b.Logger.Debug()
	for i := 0; i < len(fields)-1; i += 2 {
		if key, ok := fields[i].(string); ok {
			event = event.Interface(key, fields[i+1])
		}
	}
	event.Msg(msg)
}
