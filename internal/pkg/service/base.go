package service

import (
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	"github.com/kkz6/launch-go/internal/pkg/models"
	"github.com/kkz6/launch-go/internal/queue"
)

// Base provides common service dependencies.
// Embed this in your service to get common functionality.
//
// Usage:
//
//	type DatabaseService struct {
//	    service.Base
//	    repo *repositories.Repository
//	}
type Base struct {
	Queue  *queue.Client
	WS     broadcast.ModelBroadcaster
	Logger *zerolog.Logger
}

// NewBase creates a new Base service
func NewBase(q *queue.Client, ws broadcast.ModelBroadcaster, logger *zerolog.Logger) Base {
	return Base{
		Queue:  q,
		WS:     ws,
		Logger: logger,
	}
}

// EnqueueTask enqueues a task to the default queue
func (s *Base) EnqueueTask(task *asynq.Task) error {
	if s.Queue == nil {
		return nil // Silently succeed in test mode
	}

	_, err := s.Queue.EnqueueDefault(task)
	return err
}

// EnqueueTaskWithOptions enqueues a task with custom options
func (s *Base) EnqueueTaskWithOptions(task *asynq.Task, opts ...asynq.Option) error {
	if s.Queue == nil {
		return nil
	}

	_, err := s.Queue.Enqueue(task, opts...)
	return err
}

// BroadcastToServer broadcasts a message to all clients subscribed to a server
func (s *Base) BroadcastToServer(serverID, event string, data interface{}) {
	if s.WS == nil {
		return
	}
	s.WS.BroadcastToServer(serverID, event, data)
}

// BroadcastToTeam broadcasts a message to all clients subscribed to a team
func (s *Base) BroadcastToTeam(teamID, event string, data interface{}) {
	if s.WS == nil {
		return
	}
	s.WS.BroadcastToTeam(teamID, event, data)
}

// BroadcastToSite broadcasts a message to all clients subscribed to a site
func (s *Base) BroadcastToSite(siteID, event string, data interface{}) {
	if s.WS == nil {
		return
	}
	s.WS.BroadcastToSite(siteID, event, data)
}

// BroadcastToDeployment broadcasts a message to all clients subscribed to a deployment
func (s *Base) BroadcastToDeployment(deploymentID, event string, data interface{}) {
	if s.WS == nil {
		return
	}
	s.WS.BroadcastToDeployment(deploymentID, event, data)
}

// Broadcast broadcasts a message to a channel
func (s *Base) Broadcast(channel, event string, data interface{}) {
	if s.WS == nil {
		return
	}
	s.WS.Broadcast(channel, event, data)
}

// LogError logs an error with context
func (s *Base) LogError(err error, msg string, fields ...interface{}) {
	if s.Logger == nil {
		return
	}
	event := s.Logger.Error().Err(err)
	for i := 0; i < len(fields)-1; i += 2 {
		if key, ok := fields[i].(string); ok {
			event = event.Interface(key, fields[i+1])
		}
	}
	event.Msg(msg)
}

// LogWarn logs a warning with context
func (s *Base) LogWarn(msg string, fields ...interface{}) {
	if s.Logger == nil {
		return
	}
	event := s.Logger.Warn()
	for i := 0; i < len(fields)-1; i += 2 {
		if key, ok := fields[i].(string); ok {
			event = event.Interface(key, fields[i+1])
		}
	}
	event.Msg(msg)
}

// LogInfo logs an info message with context
func (s *Base) LogInfo(msg string, fields ...interface{}) {
	if s.Logger == nil {
		return
	}
	event := s.Logger.Info()
	for i := 0; i < len(fields)-1; i += 2 {
		if key, ok := fields[i].(string); ok {
			event = event.Interface(key, fields[i+1])
		}
	}
	event.Msg(msg)
}

// HasQueue returns true if a queue client is configured
func (s *Base) HasQueue() bool {
	return s.Queue != nil
}

// HasWebsocket returns true if a websocket hub is configured
func (s *Base) HasWebsocket() bool {
	return s.WS != nil
}

// BroadcastModelCreated broadcasts a model creation event to the team channel.
// The model must implement the Broadcastable interface.
func (s *Base) BroadcastModelCreated(model models.Broadcastable) {
	if s.WS == nil {
		return
	}
	s.WS.BroadcastModelCreated(
		model.GetTeamID(),
		model.BroadcastName(),
		model.BroadcastPayload()["id"].(string),
		model.BroadcastPayload(),
	)
}

// BroadcastModelUpdated broadcasts a model update event to the team channel.
// The model must implement the Broadcastable interface.
func (s *Base) BroadcastModelUpdated(model models.Broadcastable) {
	if s.WS == nil {
		return
	}
	s.WS.BroadcastModelUpdated(
		model.GetTeamID(),
		model.BroadcastName(),
		model.BroadcastPayload()["id"].(string),
		model.BroadcastPayload(),
	)
}

// BroadcastModelDeleted broadcasts a model deletion event to the team channel.
// The model must implement the Broadcastable interface.
func (s *Base) BroadcastModelDeleted(model models.Broadcastable) {
	if s.WS == nil {
		return
	}
	s.WS.BroadcastModelDeleted(
		model.GetTeamID(),
		model.BroadcastName(),
		model.BroadcastPayload()["id"].(string),
		model.BroadcastPayload(),
	)
}
