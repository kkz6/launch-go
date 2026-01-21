// Package broadcast provides interfaces and helpers for broadcasting events
// to WebSocket clients. This package serves as the central location for all
// broadcast-related types, ensuring consistency across services, jobs, and tasks.
//
// Architecture:
//   - Broadcaster interface: Core broadcasting capability (team/server/site/deployment channels)
//   - ModelBroadcaster interface: Extends Broadcaster with model CRUD event helpers
//   - Event struct: Standard event structure with type, data, and timestamp
//   - Event constants: Predefined event types for common operations
//
// Usage in services:
//
//	type MyService struct {
//	    ws broadcast.Broadcaster
//	}
//
//	func (s *MyService) DoSomething() {
//	    s.ws.BroadcastToTeam(teamID, broadcast.ServerUpdated, data)
//	}
package broadcast

import "time"

// Broadcaster defines the interface for broadcasting messages to WebSocket clients.
// This is the primary interface for sending real-time updates to connected clients.
//
// Channel naming conventions:
//   - team.{teamID} - Team-wide broadcasts
//   - server.{serverID} - Server-specific broadcasts
//   - site.{siteID} - Site-specific broadcasts
//   - deployment.{deploymentID} - Deployment-specific broadcasts
//   - user.{userID} - User-specific broadcasts (for private notifications)
type Broadcaster interface {
	// Broadcast sends an event to a specific channel
	Broadcast(channel string, event string, data any)

	// BroadcastToTeam sends an event to the team channel (team.{teamID})
	BroadcastToTeam(teamID string, event string, data any)

	// BroadcastToServer sends an event to the server channel (server.{serverID})
	BroadcastToServer(serverID string, event string, data any)

	// BroadcastToSite sends an event to the site channel (site.{siteID})
	BroadcastToSite(siteID string, event string, data any)

	// BroadcastToDeployment sends an event to the deployment channel (deployment.{deploymentID})
	BroadcastToDeployment(deploymentID string, event string, data any)

	// BroadcastToUser sends an event to the user channel (user.{userID})
	BroadcastToUser(userID string, event string, data any)
}

// ModelBroadcaster extends Broadcaster with model-specific broadcasting methods.
// Use this for broadcasting model CRUD events with standardized payloads.
//
// Event payloads include:
//   - id: The model ID
//   - model: The model name (e.g., "server", "site")
//   - action: The action type ("created", "updated", "deleted")
//   - team_id: The team ID for filtering
//   - data: The model payload
type ModelBroadcaster interface {
	Broadcaster

	// BroadcastModelCreated broadcasts a model creation event to the team channel
	// Event name: {modelName}.created
	BroadcastModelCreated(teamID, modelName, modelID string, payload any)

	// BroadcastModelUpdated broadcasts a model update event to the team channel
	// Event name: {modelName}.updated
	BroadcastModelUpdated(teamID, modelName, modelID string, payload any)

	// BroadcastModelDeleted broadcasts a model deletion event to the team channel
	// Event name: {modelName}.deleted
	BroadcastModelDeleted(teamID, modelName, modelID string, payload any)
}

// TeamBroadcaster is an alias for Broadcaster for backward compatibility.
// Deprecated: Use Broadcaster instead.
type TeamBroadcaster = Broadcaster

// Event represents a WebSocket event with type, data, and metadata.
// Use NewEvent or the event builder methods to create events.
type Event struct {
	// Type is the event type (e.g., "server.updated", "deployment.finished")
	Type string `json:"type"`

	// Data contains the event payload
	Data any `json:"data"`

	// Timestamp is when the event was created
	Timestamp time.Time `json:"timestamp"`

	// Meta contains optional metadata about the event
	Meta map[string]any `json:"meta,omitempty"`
}

// NewEvent creates a new Event with the given type and data.
// The timestamp is automatically set to the current time.
//
// Example:
//
//	event := broadcast.NewEvent(broadcast.ServerUpdated, serverData)
func NewEvent(eventType string, data any) *Event {
	return &Event{
		Type:      eventType,
		Data:      data,
		Timestamp: time.Now().UTC(),
	}
}

// NewEventWithMeta creates a new Event with metadata.
//
// Example:
//
//	event := broadcast.NewEventWithMeta(broadcast.DeploymentProgress, data, map[string]any{
//	    "step": 3,
//	    "total_steps": 10,
//	})
func NewEventWithMeta(eventType string, data any, meta map[string]any) *Event {
	return &Event{
		Type:      eventType,
		Data:      data,
		Timestamp: time.Now().UTC(),
		Meta:      meta,
	}
}

// WithMeta adds metadata to the event and returns the event for chaining.
func (e *Event) WithMeta(key string, value any) *Event {
	if e.Meta == nil {
		e.Meta = make(map[string]any)
	}
	e.Meta[key] = value
	return e
}

// ToMap converts the event to a map suitable for broadcasting.
func (e *Event) ToMap() map[string]any {
	result := map[string]any{
		"type":      e.Type,
		"data":      e.Data,
		"timestamp": e.Timestamp,
	}
	if len(e.Meta) > 0 {
		result["meta"] = e.Meta
	}
	return result
}

// NopBroadcaster is a no-op implementation of Broadcaster for testing.
type NopBroadcaster struct{}

func (NopBroadcaster) Broadcast(channel string, event string, data any)          {}
func (NopBroadcaster) BroadcastToTeam(teamID string, event string, data any)     {}
func (NopBroadcaster) BroadcastToServer(serverID string, event string, data any) {}
func (NopBroadcaster) BroadcastToSite(siteID string, event string, data any)     {}
func (NopBroadcaster) BroadcastToDeployment(deploymentID string, event string, data any) {
}
func (NopBroadcaster) BroadcastToUser(userID string, event string, data any) {}

// NopModelBroadcaster is a no-op implementation of ModelBroadcaster for testing.
type NopModelBroadcaster struct {
	NopBroadcaster
}

func (NopModelBroadcaster) BroadcastModelCreated(teamID, modelName, modelID string, payload any) {
}
func (NopModelBroadcaster) BroadcastModelUpdated(teamID, modelName, modelID string, payload any) {
}
func (NopModelBroadcaster) BroadcastModelDeleted(teamID, modelName, modelID string, payload any) {
}

// Ensure NopBroadcaster implements Broadcaster
var _ Broadcaster = NopBroadcaster{}

// Ensure NopModelBroadcaster implements ModelBroadcaster
var _ ModelBroadcaster = NopModelBroadcaster{}
