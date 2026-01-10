package websocket

import (
	"encoding/json"
	"sync"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

type Message struct {
	Event   string      `json:"event"`
	Channel string      `json:"channel"`
	Data    interface{} `json:"data"`
}

type Client struct {
	ID       string
	UserID   string
	TeamID   string
	Conn     *websocket.Conn
	Channels map[string]bool
	Send     chan []byte
	mu       sync.RWMutex
}

type Hub struct {
	clients    map[*Client]bool
	channels   map[string]map[*Client]bool
	broadcast  chan *Message
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
	done       chan struct{}
}

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
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
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
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.broadcastToChannel(message)
		}
	}
}

func (h *Hub) broadcastToChannel(message *Message) {
	h.mu.RLock()
	clients, ok := h.channels[message.Channel]
	h.mu.RUnlock()

	if !ok {
		return
	}

	data, err := json.Marshal(message)
	if err != nil {
		return
	}

	for client := range clients {
		select {
		case client.Send <- data:
		default:
			h.unregister <- client
		}
	}
}

func (h *Hub) Subscribe(client *Client, channel string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.channels[channel]; !ok {
		h.channels[channel] = make(map[*Client]bool)
	}

	h.channels[channel][client] = true
	client.mu.Lock()
	client.Channels[channel] = true
	client.mu.Unlock()
}

func (h *Hub) Unsubscribe(client *Client, channel string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.channels[channel]; ok {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.channels, channel)
		}
	}

	client.mu.Lock()
	delete(client.Channels, channel)
	client.mu.Unlock()
}

func (h *Hub) Broadcast(channel string, event string, data interface{}) {
	h.broadcast <- &Message{
		Event:   event,
		Channel: channel,
		Data:    data,
	}
}

func (h *Hub) BroadcastToServer(serverID string, event string, data interface{}) {
	h.Broadcast("server."+serverID, event, data)
}

func (h *Hub) BroadcastToSite(siteID string, event string, data interface{}) {
	h.Broadcast("site."+siteID, event, data)
}

func (h *Hub) BroadcastToDeployment(deploymentID string, event string, data interface{}) {
	h.Broadcast("deployment."+deploymentID, event, data)
}

func (h *Hub) BroadcastToTeam(teamID string, event string, data interface{}) {
	h.Broadcast("team."+teamID, event, data)
}

func (h *Hub) Shutdown() {
	close(h.done)
}

func Handler(hub *Hub, jwtSecret string) fiber.Handler {
	return websocket.New(func(c *websocket.Conn) {
		// Get token from query params
		token := c.Query("token")
		if token == "" {
			c.Close()
			return
		}

		// Validate JWT
		claims, err := validateToken(token, jwtSecret)
		if err != nil {
			c.Close()
			return
		}

		client := &Client{
			ID:       claims["sub"].(string),
			UserID:   claims["sub"].(string),
			TeamID:   claims["team_id"].(string),
			Conn:     c,
			Channels: make(map[string]bool),
			Send:     make(chan []byte, 256),
		}

		hub.register <- client

		// Auto-subscribe to team channel
		hub.Subscribe(client, "team."+client.TeamID)

		go writePump(client)
		readPump(hub, client)
	})
}

func validateToken(tokenString, secret string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})

	if err != nil || !token.Valid {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, jwt.ErrTokenInvalidClaims
	}

	return claims, nil
}

func readPump(hub *Hub, client *Client) {
	defer func() {
		hub.unregister <- client
		client.Conn.Close()
	}()

	for {
		var msg struct {
			Action  string `json:"action"`
			Channel string `json:"channel"`
		}

		if err := client.Conn.ReadJSON(&msg); err != nil {
			break
		}

		switch msg.Action {
		case "subscribe":
			hub.Subscribe(client, msg.Channel)
		case "unsubscribe":
			hub.Unsubscribe(client, msg.Channel)
		}
	}
}

func writePump(client *Client) {
	defer client.Conn.Close()

	for message := range client.Send {
		if err := client.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
			break
		}
	}
}
