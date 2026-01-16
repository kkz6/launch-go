// Package broadcast provides interfaces for broadcasting events to team channels.
package broadcast

// TeamBroadcaster broadcasts events to team channels.
// All events are sent to team.{teamID} channel for simplicity.
type TeamBroadcaster interface {
	// BroadcastToTeam sends an event to the team channel
	BroadcastToTeam(teamID string, event string, data any)

	// BroadcastToServer sends an event to the server channel
	BroadcastToServer(serverID string, event string, data any)

	// BroadcastToSite sends an event to the site channel
	BroadcastToSite(siteID string, event string, data any)

	// BroadcastToDeployment sends an event to the deployment channel
	BroadcastToDeployment(deploymentID string, event string, data any)

	// Broadcast sends an event to a specific channel
	Broadcast(channel string, event string, data any)
}

// ModelBroadcaster extends TeamBroadcaster with model-specific broadcasting methods.
// Use this for broadcasting model CRUD events.
type ModelBroadcaster interface {
	TeamBroadcaster

	// BroadcastModelCreated broadcasts a model creation event to the team channel
	BroadcastModelCreated(teamID, modelName, modelID string, payload any)

	// BroadcastModelUpdated broadcasts a model update event to the team channel
	BroadcastModelUpdated(teamID, modelName, modelID string, payload any)

	// BroadcastModelDeleted broadcasts a model deletion event to the team channel
	BroadcastModelDeleted(teamID, modelName, modelID string, payload any)
}
