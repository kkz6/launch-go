package websocket

import (
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/contrib/websocket"

	"github.com/kkz6/launch-go/internal/pkg/i18n"
)

const (
	clientWriteWait  = 10 * time.Second
	clientPongWait   = 60 * time.Second
	clientPingEvery  = 50 * time.Second
	clientSendQueue  = 64
	clientMaxMessage = 64 << 10
)

// ChannelAuthorizer verifies that a connection may subscribe to a resource
// channel. Implementations must fail closed for unknown channel scopes.
type ChannelAuthorizer interface {
	AuthorizeChannel(userID, teamID, channel string) bool
}

// Client represents a connected WebSocket client
type Client struct {
	ID         string
	UserID     string
	TeamID     string
	Locale     string
	Conn       *websocket.Conn
	Channels   map[string]bool
	Send       chan []byte
	hub        *Hub
	mu         sync.RWMutex
	sendMu     sync.RWMutex
	closing    atomic.Bool
	authorizer ChannelAuthorizer
}

// NewClient creates a new WebSocket client
func NewClient(hub *Hub, conn *websocket.Conn, userID, teamID string, authorizers ...ChannelAuthorizer) *Client {
	var authorizer ChannelAuthorizer
	if len(authorizers) > 0 {
		authorizer = authorizers[0]
	}
	return &Client{
		ID:         userID,
		UserID:     userID,
		TeamID:     teamID,
		Locale:     i18n.DefaultLocale,
		Conn:       conn,
		Channels:   make(map[string]bool),
		Send:       make(chan []byte, clientSendQueue),
		hub:        hub,
		authorizer: authorizer,
	}
}

// Close marks the client as closing to prevent send-on-closed-channel panics
func (c *Client) Close() {
	c.sendMu.Lock()
	defer c.sendMu.Unlock()
	if c.closing.Swap(true) {
		return
	}
	close(c.Send)
}

// IsClosing returns true if the client is in closing state
func (c *Client) IsClosing() bool {
	return c.closing.Load()
}

// SafeSend attempts to send a message to the client
// Returns false if the client is closing or the send buffer is full
func (c *Client) SafeSend(data []byte) bool {
	c.sendMu.RLock()
	defer c.sendMu.RUnlock()
	if c.IsClosing() {
		return false
	}

	select {
	case c.Send <- data:
		return true
	default:
		// Buffer full, client is too slow
		return false
	}
}

// ReadPump reads messages from the WebSocket connection
// This should be run in its own goroutine
func (c *Client) ReadPump() {
	defer func() {
		c.hub.Unregister(c)
		c.Conn.Close()
	}()
	c.Conn.SetReadLimit(clientMaxMessage)
	_ = c.Conn.SetReadDeadline(time.Now().Add(clientPongWait))
	c.Conn.SetPongHandler(func(string) error {
		return c.Conn.SetReadDeadline(time.Now().Add(clientPongWait))
	})

	for {
		var msg struct {
			Action  string `json:"action"`
			Channel string `json:"channel"`
		}

		if err := c.Conn.ReadJSON(&msg); err != nil {
			break
		}

		switch msg.Action {
		case "subscribe":
			channel := strings.TrimSpace(msg.Channel)
			if c.authorizer == nil || !c.authorizer.AuthorizeChannel(c.UserID, c.TeamID, channel) {
				c.sendProtocolMessage("subscription.error", channel, map[string]string{
					"message": i18n.Translate(c.Locale, "channel access denied"),
				})
				continue
			}
			c.hub.Subscribe(c, channel)
			c.sendProtocolMessage("subscription.succeeded", channel, nil)
		case "unsubscribe":
			c.hub.Unsubscribe(c, strings.TrimSpace(msg.Channel))
		}
	}
}

// WritePump writes messages to the WebSocket connection
// This should be run in its own goroutine
func (c *Client) WritePump() {
	defer c.Conn.Close()
	ticker := time.NewTicker(clientPingEvery)
	defer ticker.Stop()

	for {
		select {
		case message, ok := <-c.Send:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(clientWriteWait))
			if !ok {
				_ = c.Conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.Conn.WriteControl(
				websocket.PingMessage,
				nil,
				time.Now().Add(clientWriteWait),
			); err != nil {
				return
			}
		}
	}
}

func (c *Client) sendProtocolMessage(event, channel string, data any) {
	payload, err := json.Marshal(Message{Event: event, Channel: channel, Data: data})
	if err == nil {
		c.SafeSend(payload)
	}
}

// AddChannel adds a channel to the client's subscription list
func (c *Client) AddChannel(channel string) {
	c.mu.Lock()
	c.Channels[channel] = true
	c.mu.Unlock()
}

// RemoveChannel removes a channel from the client's subscription list
func (c *Client) RemoveChannel(channel string) {
	c.mu.Lock()
	delete(c.Channels, channel)
	c.mu.Unlock()
}

// GetChannels returns a copy of the client's subscribed channels
func (c *Client) GetChannels() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	channels := make([]string, 0, len(c.Channels))
	for channel := range c.Channels {
		channels = append(channels, channel)
	}
	return channels
}
