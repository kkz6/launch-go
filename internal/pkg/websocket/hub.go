package websocket

import (
	"encoding/json"
	"sync"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"

	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	"github.com/kkz6/launch-go/internal/pkg/i18n"
	launchcache "github.com/kkz6/launch-go/internal/pkg/launch/cache"
)

// Message represents a WebSocket message
type Message struct {
	Event   string      `json:"event"`
	Channel string      `json:"channel"`
	Data    interface{} `json:"data"`
}

type outboundMessage struct {
	channel string
	payload []byte
}

// Hub manages WebSocket clients and message broadcasting
type Hub struct {
	clients    map[*Client]bool
	channels   map[string]map[*Client]bool
	broadcast  chan outboundMessage
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
	done       chan struct{}
	shutdown   sync.Once
}

// NewHub creates a new WebSocket hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		channels:   make(map[string]map[*Client]bool),
		broadcast:  make(chan outboundMessage, 256),
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
	client.Close()
	channels := client.GetChannels()

	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[client]; !ok {
		return
	}

	delete(h.clients, client)

	// Remove from all channels
	for _, channel := range channels {
		if clients, ok := h.channels[channel]; ok {
			delete(clients, client)
			if len(clients) == 0 {
				delete(h.channels, channel)
			}
		}
	}
}

// broadcastToChannel sends a message to all clients subscribed to the channel
func (h *Hub) broadcastToChannel(message outboundMessage) {
	h.mu.RLock()
	clients, ok := h.channels[message.channel]
	if !ok {
		h.mu.RUnlock()
		// No clients subscribed — normal when nobody is viewing the
		// page. Caller has already paid for the marshal but that's
		// cheap; staying silent here avoids log spam during idle
		// background broadcasts.
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
		if !client.SafeSend(message.payload) {
			// Client buffer full or closing, schedule for removal
			go h.Unregister(client)
		}
	}
}

// Subscribe adds a client to a channel
func (h *Hub) Subscribe(client *Client, channel string) {
	if channel == "" || client.IsClosing() {
		return
	}
	client.AddChannel(channel)

	h.mu.Lock()
	defer h.mu.Unlock()
	if client.IsClosing() {
		client.RemoveChannel(channel)
		return
	}

	if _, ok := h.channels[channel]; !ok {
		h.channels[channel] = make(map[*Client]bool)
	}

	h.channels[channel][client] = true
}

// Unsubscribe removes a client from a channel
func (h *Hub) Unsubscribe(client *Client, channel string) {
	client.RemoveChannel(channel)

	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.channels[channel]; ok {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.channels, channel)
		}
	}
}

// Broadcast sends a message to all clients subscribed to a channel
func (h *Hub) Broadcast(channel string, event string, data interface{}) {
	payload, err := json.Marshal(Message{
		Event:   event,
		Channel: channel,
		Data:    data,
	})
	if err != nil {
		log.Warn().Err(err).Str("event", event).Msg("Hub: failed to marshal broadcast message")
		return
	}
	h.enqueueSerialized(channel, payload)
}

// BroadcastSerialized forwards an already-encoded websocket message without
// decoding and encoding its data again. Redis subscribers use this fast path.
func (h *Hub) BroadcastSerialized(channel string, payload []byte) {
	h.enqueueSerialized(channel, payload)
}

func (h *Hub) enqueueSerialized(channel string, payload []byte) {
	select {
	case h.broadcast <- outboundMessage{channel: channel, payload: payload}:
	case <-h.done:
	}
}

// BroadcastToServer sends a message to the server's channel
func (h *Hub) BroadcastToServer(serverID string, event string, data interface{}) {
	h.Broadcast(broadcast.ServerChannel(serverID), event, broadcast.EnsureRoutingField(data, "server_id", serverID))
}

// BroadcastToSite sends a message to the site's channel
func (h *Hub) BroadcastToSite(siteID string, event string, data interface{}) {
	h.Broadcast(broadcast.SiteChannel(siteID), event, broadcast.EnsureRoutingField(data, "site_id", siteID))
}

// BroadcastToDeployment sends a message to the deployment's channel
func (h *Hub) BroadcastToDeployment(deploymentID string, event string, data interface{}) {
	h.Broadcast(broadcast.DeploymentChannel(deploymentID), event, broadcast.EnsureRoutingField(data, "deployment_id", deploymentID))
}

// BroadcastToTeam sends a message to the team's channel.
//
// Injects team_id into the payload so the frontend's useChannelEvents
// filter (eventData.team_id === channelParts[1]) can route the event to
// the right handler. Mirrors the same injection in RedisBroadcaster.
func (h *Hub) BroadcastToTeam(teamID string, event string, data interface{}) {
	h.Broadcast(broadcast.TeamChannel(teamID), event, broadcast.EnsureRoutingField(data, "team_id", teamID))
}

// BroadcastToUser sends a message to the user's channel
func (h *Hub) BroadcastToUser(userID string, event string, data interface{}) {
	h.Broadcast(broadcast.UserChannel(userID), event, broadcast.EnsureRoutingField(data, "user_id", userID))
}

// BroadcastModelCreated broadcasts a model creation event to the team channel
func (h *Hub) BroadcastModelCreated(teamID, modelName, modelID string, payload interface{}) {
	h.BroadcastToTeam(
		teamID,
		broadcast.ModelEventName(modelName, "created"),
		broadcast.BuildModelEventPayload(modelName, modelID, "created", teamID, payload),
	)
}

// BroadcastModelUpdated broadcasts a model update event to the team channel
func (h *Hub) BroadcastModelUpdated(teamID, modelName, modelID string, payload interface{}) {
	h.BroadcastToTeam(
		teamID,
		broadcast.ModelEventName(modelName, "updated"),
		broadcast.BuildModelEventPayload(modelName, modelID, "updated", teamID, payload),
	)
}

// BroadcastModelDeleted broadcasts a model deletion event to the team channel
func (h *Hub) BroadcastModelDeleted(teamID, modelName, modelID string, payload interface{}) {
	h.BroadcastToTeam(
		teamID,
		broadcast.ModelEventName(modelName, "deleted"),
		broadcast.BuildModelEventPayload(modelName, modelID, "deleted", teamID, payload),
	)
}

// Shutdown gracefully shuts down the hub
func (h *Hub) Shutdown() {
	h.shutdown.Do(func() {
		close(h.done)
		h.mu.RLock()
		clients := make([]*Client, 0, len(h.clients))
		for client := range h.clients {
			clients = append(clients, client)
		}
		h.mu.RUnlock()
		for _, client := range clients {
			client.Close()
		}
	})
}

// Register registers a client with the hub
func (h *Hub) Register(client *Client) {
	select {
	case h.register <- client:
	case <-h.done:
		client.Close()
	}
}

// Unregister removes a client unless the hub has already shut down.
func (h *Hub) Unregister(client *Client) {
	select {
	case h.unregister <- client:
	case <-h.done:
		client.Close()
	}
}

// Handler returns a Fiber handler for the main WebSocket endpoint
// Connection URL: /ws?token=xxx&team_id=xxx
func Handler(hub *Hub, jwtSecret string, membershipCache *launchcache.TeamMembershipCache, authorizer ChannelAuthorizer) fiber.Handler {
	return websocket.New(func(c *websocket.Conn) {
		// Authenticate and validate team membership
		claims, err := AuthenticateWebSocket(c, jwtSecret, membershipCache)
		if err != nil {
			c.Close()
			return
		}

		// Create client
		client := NewClient(hub, c, claims.UserID, claims.TeamID, authorizer)
		if locale, ok := i18n.Normalize(c.Query("locale")); ok {
			client.Locale = locale
		}

		// Register with hub
		hub.Register(client)

		// Auto-subscribe to team channel
		teamChannel := broadcast.TeamChannel(client.TeamID)
		hub.Subscribe(client, teamChannel)

		// Keep the Fiber connection wrapper alive until both pumps have
		// stopped. Fiber returns the wrapper to a pool when this handler
		// exits, so letting WritePump outlive the handler races with that
		// pool cleanup and can make the writer observe a recycled socket.
		writerDone := make(chan struct{})
		go func() {
			defer close(writerDone)
			client.WritePump()
		}()
		client.ReadPump()
		client.Close()
		<-writerDone
	})
}
