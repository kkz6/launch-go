package jobs

type Broadcaster interface {
	Broadcast(channel string, event string, data any)
	BroadcastToServer(serverID string, event string, data any)
	BroadcastToSite(siteID string, event string, data any)
	BroadcastToDeployment(deploymentID string, event string, data any)
	BroadcastToTeam(teamID string, event string, data any)
}
