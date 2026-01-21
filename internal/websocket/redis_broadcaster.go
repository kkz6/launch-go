package websocket

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/pkg/broadcast"
)

const wsChannel = "websocket:broadcast"

// RedisBroadcaster publishes WebSocket messages to Redis for cross-process broadcasting
type RedisBroadcaster struct {
	client *redis.Client
	logger *zerolog.Logger
}

// NewRedisBroadcaster creates a new Redis-based broadcaster
func NewRedisBroadcaster(addr, password string, db int, logger *zerolog.Logger) *RedisBroadcaster {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	return &RedisBroadcaster{
		client: client,
		logger: logger,
	}
}

// BroadcastToTeam publishes a message to Redis for the team channel
func (r *RedisBroadcaster) BroadcastToTeam(teamID string, event string, data any) {
	r.publish("team."+teamID, event, data)
}

// BroadcastToServer publishes a message to Redis for the server channel
func (r *RedisBroadcaster) BroadcastToServer(serverID string, event string, data any) {
	r.publish("server."+serverID, event, data)
}

// BroadcastToSite publishes a message to Redis for the site channel
func (r *RedisBroadcaster) BroadcastToSite(siteID string, event string, data any) {
	r.publish("site."+siteID, event, data)
}

// BroadcastToDeployment publishes a message to Redis for the deployment channel
func (r *RedisBroadcaster) BroadcastToDeployment(deploymentID string, event string, data any) {
	r.publish("deployment."+deploymentID, event, data)
}

// BroadcastToUser publishes a message to Redis for the user channel
func (r *RedisBroadcaster) BroadcastToUser(userID string, event string, data any) {
	r.publish("user."+userID, event, data)
}

// Broadcast publishes a message to Redis for a specific channel
func (r *RedisBroadcaster) Broadcast(channel string, event string, data any) {
	r.publish(channel, event, data)
}

// publish sends a message to Redis Pub/Sub
func (r *RedisBroadcaster) publish(channel, event string, data any) {
	msg := &Message{
		Channel: channel,
		Event:   event,
		Data:    data,
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		if r.logger != nil {
			r.logger.Error().Err(err).Msg("Failed to marshal WebSocket message")
		}
		return
	}

	if err := r.client.Publish(context.Background(), wsChannel, payload).Err(); err != nil {
		if r.logger != nil {
			r.logger.Error().Err(err).Msg("Failed to publish WebSocket message to Redis")
		}
	}
}

// BroadcastModelCreated broadcasts a model creation event to the team channel
func (r *RedisBroadcaster) BroadcastModelCreated(teamID, modelName, modelID string, payload any) {
	r.BroadcastToTeam(
		teamID,
		broadcast.ModelEventName(modelName, "created"),
		broadcast.BuildModelEventPayload(modelName, modelID, "created", teamID, payload),
	)
}

// BroadcastModelUpdated broadcasts a model update event to the team channel
func (r *RedisBroadcaster) BroadcastModelUpdated(teamID, modelName, modelID string, payload any) {
	r.BroadcastToTeam(
		teamID,
		broadcast.ModelEventName(modelName, "updated"),
		broadcast.BuildModelEventPayload(modelName, modelID, "updated", teamID, payload),
	)
}

// BroadcastModelDeleted broadcasts a model deletion event to the team channel
func (r *RedisBroadcaster) BroadcastModelDeleted(teamID, modelName, modelID string, payload any) {
	r.BroadcastToTeam(
		teamID,
		broadcast.ModelEventName(modelName, "deleted"),
		broadcast.BuildModelEventPayload(modelName, modelID, "deleted", teamID, payload),
	)
}

// Close closes the Redis connection
func (r *RedisBroadcaster) Close() error {
	return r.client.Close()
}

// RedisSubscriber subscribes to Redis and forwards messages to the WebSocket hub
type RedisSubscriber struct {
	client *redis.Client
	hub    *Hub
	logger *zerolog.Logger
	ctx    context.Context
	cancel context.CancelFunc
}

// NewRedisSubscriber creates a new Redis subscriber that forwards to the hub
func NewRedisSubscriber(addr, password string, db int, hub *Hub, logger *zerolog.Logger) *RedisSubscriber {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx, cancel := context.WithCancel(context.Background())

	return &RedisSubscriber{
		client: client,
		hub:    hub,
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start begins listening for Redis messages and forwarding to the hub
func (s *RedisSubscriber) Start() {
	pubsub := s.client.Subscribe(s.ctx, wsChannel)

	go func() {
		defer pubsub.Close()

		ch := pubsub.Channel()
		for {
			select {
			case <-s.ctx.Done():
				return
			case msg := <-ch:
				if msg == nil {
					continue
				}

				var wsMsg Message
				if err := json.Unmarshal([]byte(msg.Payload), &wsMsg); err != nil {
					if s.logger != nil {
						s.logger.Error().Err(err).Msg("Failed to unmarshal WebSocket message from Redis")
					}
					continue
				}

				// Forward to the local WebSocket hub
				s.hub.Broadcast(wsMsg.Channel, wsMsg.Event, wsMsg.Data)
			}
		}
	}()

	if s.logger != nil {
		s.logger.Info().Msg("Redis WebSocket subscriber started")
	}
}

// Stop stops the subscriber
func (s *RedisSubscriber) Stop() {
	s.cancel()
	s.client.Close()
}
