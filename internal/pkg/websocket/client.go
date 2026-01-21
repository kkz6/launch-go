package websocket

import (
	"sync"
	"sync/atomic"

	"github.com/gofiber/contrib/websocket"
)

// Client represents a connected WebSocket client
type Client struct {
	ID       string
	UserID   string
	TeamID   string
	Conn     *websocket.Conn
	Channels map[string]bool
	Send     chan []byte
	hub      *Hub
	mu       sync.RWMutex
	closing  atomic.Bool
}

// NewClient creates a new WebSocket client
func NewClient(hub *Hub, conn *websocket.Conn, userID, teamID string) *Client {
	return &Client{
		ID:       userID,
		UserID:   userID,
		TeamID:   teamID,
		Conn:     conn,
		Channels: make(map[string]bool),
		Send:     make(chan []byte, 256),
		hub:      hub,
	}
}

// Close marks the client as closing to prevent send-on-closed-channel panics
func (c *Client) Close() {
	c.closing.Store(true)
}

// IsClosing returns true if the client is in closing state
func (c *Client) IsClosing() bool {
	return c.closing.Load()
}

// SafeSend attempts to send a message to the client
// Returns false if the client is closing or the send buffer is full
func (c *Client) SafeSend(data []byte) bool {
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
		c.hub.unregister <- c
		c.Conn.Close()
	}()

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
			c.hub.Subscribe(c, msg.Channel)
		case "unsubscribe":
			c.hub.Unsubscribe(c, msg.Channel)
		}
	}
}

// WritePump writes messages to the WebSocket connection
// This should be run in its own goroutine
func (c *Client) WritePump() {
	defer c.Conn.Close()

	for message := range c.Send {
		if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
			break
		}
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
