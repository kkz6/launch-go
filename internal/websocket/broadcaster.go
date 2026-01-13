package websocket

// Broadcaster defines the interface for broadcasting messages to WebSocket clients.
// This interface is implemented by Hub and can be used for dependency injection.
type Broadcaster interface {
	// Broadcast sends a message to all clients subscribed to the given channel
	Broadcast(channel string, event string, data interface{})

	// BroadcastToServer sends a message to the server's channel (server.{serverID})
	BroadcastToServer(serverID string, event string, data interface{})

	// BroadcastToSite sends a message to the site's channel (site.{siteID})
	BroadcastToSite(siteID string, event string, data interface{})

	// BroadcastToDeployment sends a message to the deployment's channel (deployment.{deploymentID})
	BroadcastToDeployment(deploymentID string, event string, data interface{})

	// BroadcastToTeam sends a message to the team's channel (team.{teamID})
	BroadcastToTeam(teamID string, event string, data interface{})
}

// ModelBroadcaster extends Broadcaster with model-specific broadcasting methods.
// Use this for broadcasting model CRUD events.
type ModelBroadcaster interface {
	Broadcaster

	// BroadcastModelCreated broadcasts a model creation event to the team channel
	BroadcastModelCreated(teamID, modelName, modelID string, payload interface{})

	// BroadcastModelUpdated broadcasts a model update event to the team channel
	BroadcastModelUpdated(teamID, modelName, modelID string, payload interface{})

	// BroadcastModelDeleted broadcasts a model deletion event to the team channel
	BroadcastModelDeleted(teamID, modelName, modelID string, payload interface{})
}

// Ensure Hub implements both interfaces
var _ Broadcaster = (*Hub)(nil)
var _ ModelBroadcaster = (*Hub)(nil)
