package handlers

import (
	"encoding/json"
	"fmt"

	"github.com/gofiber/contrib/websocket"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/i18n"
	launchcache "github.com/kkz6/launch-go/internal/pkg/launch/cache"
	ws "github.com/kkz6/launch-go/internal/pkg/websocket"
)

// WSMessage is a standard WebSocket message format for consistent JSON structure.
type WSMessage struct {
	Event   string `json:"event"`
	Data    any    `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
}

// Base provides common dependencies and functionality for all WebSocket handlers.
// Embed this struct in specific handlers to get access to shared resources.
type Base struct {
	DB              *gorm.DB
	JWTSecret       string
	Logger          zerolog.Logger
	MembershipCache *launchcache.TeamMembershipCache
}

// NewBase creates a new Base handler with all dependencies.
func NewBase(db *gorm.DB, jwtSecret string, logger zerolog.Logger, membershipCache *launchcache.TeamMembershipCache) Base {
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

// SendError sends a structured JSON error message to the WebSocket client and logs it.
// The message is formatted as {"event":"error","message":"..."}.
func (b *Base) SendError(c *websocket.Conn, msg string, args ...any) {
	logMessage := msg
	if len(args) > 0 {
		logMessage = fmt.Sprintf(msg, args...)
	}
	b.Logger.Warn().Msg(logMessage)
	_ = SendEvent(c, "error", map[string]string{"message": b.Translate(c, msg, args...)})
}

// Translate localizes dedicated WebSocket control messages using the
// canonical locale supplied during the handshake. Raw terminal, log, and
// command output is intentionally never passed through this method.
func (b *Base) Translate(c *websocket.Conn, message string, args ...any) string {
	locale := i18n.DefaultLocale
	if c != nil {
		if requested, ok := i18n.Normalize(c.Query("locale")); ok {
			locale = requested
		}
	}
	return i18n.Translate(locale, message, args...)
}

// SendJSON sends a JSON message to the WebSocket client.
// Returns an error if marshaling or sending fails.
func (b *Base) SendJSON(c *websocket.Conn, v any) error {
	return c.WriteJSON(v)
}

// SendEvent sends a structured WebSocket message with an event name and data.
// This ensures consistent JSON formatting across all handlers.
func SendEvent(c *websocket.Conn, event string, data any) error {
	msg := WSMessage{Event: event, Data: data}
	bytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return c.WriteMessage(websocket.TextMessage, bytes)
}

// SendEventWithMessage sends a structured WebSocket message with an event and a message string.
// Use this for simple event notifications without complex data structures.
func SendEventWithMessage(c *websocket.Conn, event, message string) error {
	msg := WSMessage{Event: event, Message: message}
	bytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return c.WriteMessage(websocket.TextMessage, bytes)
}

// SendErrorEvent sends a structured error event to the WebSocket client.
// This is a standalone function for use without a Base handler instance.
func SendErrorEvent(c *websocket.Conn, message string) error {
	return SendEvent(c, "error", map[string]string{"message": message})
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
