package websocket

import (
	"encoding/json"
	"sync"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/pkg/cache"
)

// Message represents a WebSocket message
type Message struct {
	Event   string      `json:"event"`
	Channel string      `json:"channel"`
	Data    interface{} `json:"data"`
}

// Hub manages WebSocket clients and message broadcasting
type Hub struct {
	clients    map[*Client]bool
	channels   map[string]map[*Client]bool
	broadcast  chan *Message
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
	done       chan struct{}
}

// NewHub creates a new WebSocket hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		channels:   make(map[string]map[*Client]bool),
		broadcast:  make(chan *Message, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		done:       make(chan struct{}),
	}
}

// Run starts the hub's main event loop
func (h *Hub) Run() {
	for {
		select {
		case <-h.done:
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.removeClient(client)

		case message := <-h.broadcast:
			h.broadcastToChannel(message)
		}
	}
}

// removeClient removes a client from the hub and all its channels
func (h *Hub) removeClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[client]; !ok {
		return
	}

	// Mark client as closing before closing the channel
	client.Close()

	delete(h.clients, client)
	close(client.Send)

	// Remove from all channels
	for channel := range client.Channels {
		if clients, ok := h.channels[channel]; ok {
			delete(clients, client)
			if len(clients) == 0 {
				delete(h.channels, channel)
			}
		}
	}
}

// broadcastToChannel sends a message to all clients subscribed to the channel
func (h *Hub) broadcastToChannel(message *Message) {
	data, err := json.Marshal(message)
	if err != nil {
		return
	}

	h.mu.RLock()
	clients, ok := h.channels[message.Channel]
	if !ok {
		h.mu.RUnlock()
		return
	}

	// Copy clients to avoid holding lock during send
	clientsCopy := make([]*Client, 0, len(clients))
	for client := range clients {
		clientsCopy = append(clientsCopy, client)
	}
	h.mu.RUnlock()

	// Send to all clients
	for _, client := range clientsCopy {
		if !client.SafeSend(data) {
			// Client buffer full or closing, schedule for removal
			go func(c *Client) {
				h.unregister <- c
			}(client)
		}
	}
}

// Subscribe adds a client to a channel
func (h *Hub) Subscribe(client *Client, channel string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.channels[channel]; !ok {
		h.channels[channel] = make(map[*Client]bool)
	}

	h.channels[channel][client] = true
	client.AddChannel(channel)
}

// Unsubscribe removes a client from a channel
func (h *Hub) Unsubscribe(client *Client, channel string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.channels[channel]; ok {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.channels, channel)
		}
	}

	client.RemoveChannel(channel)
}

// Broadcast sends a message to all clients subscribed to a channel
func (h *Hub) Broadcast(channel string, event string, data interface{}) {
	h.broadcast <- &Message{
		Event:   event,
		Channel: channel,
		Data:    data,
	}
}

// BroadcastToServer sends a message to the server's channel
func (h *Hub) BroadcastToServer(serverID string, event string, data interface{}) {
	h.Broadcast("server."+serverID, event, data)
}

// BroadcastToSite sends a message to the site's channel
func (h *Hub) BroadcastToSite(siteID string, event string, data interface{}) {
	h.Broadcast("site."+siteID, event, data)
}

// BroadcastToDeployment sends a message to the deployment's channel
func (h *Hub) BroadcastToDeployment(deploymentID string, event string, data interface{}) {
	h.Broadcast("deployment."+deploymentID, event, data)
}

// BroadcastToTeam sends a message to the team's channel
func (h *Hub) BroadcastToTeam(teamID string, event string, data interface{}) {
	h.Broadcast("team."+teamID, event, data)
}

// BroadcastModelCreated broadcasts a model creation event to the team channel
func (h *Hub) BroadcastModelCreated(teamID, modelName, modelID string, payload interface{}) {
	h.BroadcastToTeam(teamID, modelName+".created", map[string]interface{}{
		"id":      modelID,
		"model":   modelName,
		"action":  "created",
		"team_id": teamID,
		"data":    payload,
	})
}

// BroadcastModelUpdated broadcasts a model update event to the team channel
func (h *Hub) BroadcastModelUpdated(teamID, modelName, modelID string, payload interface{}) {
	h.BroadcastToTeam(teamID, modelName+".updated", map[string]interface{}{
		"id":      modelID,
		"model":   modelName,
		"action":  "updated",
		"team_id": teamID,
		"data":    payload,
	})
}

// BroadcastModelDeleted broadcasts a model deletion event to the team channel
func (h *Hub) BroadcastModelDeleted(teamID, modelName, modelID string, payload interface{}) {
	h.BroadcastToTeam(teamID, modelName+".deleted", map[string]interface{}{
		"id":      modelID,
		"model":   modelName,
		"action":  "deleted",
		"team_id": teamID,
		"data":    payload,
	})
}

// Shutdown gracefully shuts down the hub
func (h *Hub) Shutdown() {
	close(h.done)
}

// Register registers a client with the hub
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Handler returns a Fiber handler for the main WebSocket endpoint
// Connection URL: /ws?token=xxx&team_id=xxx
func Handler(hub *Hub, jwtSecret string, membershipCache *cache.TeamMembershipCache) fiber.Handler {
	return websocket.New(func(c *websocket.Conn) {
		// Authenticate and validate team membership
		claims, err := AuthenticateWebSocket(c, jwtSecret, membershipCache)
		if err != nil {
			c.Close()
			return
		}

		// Create client
		client := NewClient(hub, c, claims.UserID, claims.TeamID)

		// Register with hub
		hub.Register(client)

		// Auto-subscribe to team channel
		hub.Subscribe(client, "team."+client.TeamID)

		// Start pumps
		go client.WritePump()
		client.ReadPump()
	})
}
