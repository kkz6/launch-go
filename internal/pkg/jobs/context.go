package jobs

// Broadcaster interface for websocket broadcasting
// This breaks the import cycle by abstracting the websocket dependency
type Broadcaster interface {
	Broadcast(channel string, event string, data interface{})
	BroadcastToServer(serverID string, event string, data interface{})
	BroadcastToSite(siteID string, event string, data interface{})
	BroadcastToDeployment(deploymentID string, event string, data interface{})
}
