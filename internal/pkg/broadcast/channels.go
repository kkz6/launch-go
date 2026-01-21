package broadcast

// Channel name builders for WebSocket broadcasting

// TeamChannel returns the channel name for a team
func TeamChannel(teamID string) string {
	return "team." + teamID
}

// ServerChannel returns the channel name for a server
func ServerChannel(serverID string) string {
	return "server." + serverID
}

// SiteChannel returns the channel name for a site
func SiteChannel(siteID string) string {
	return "site." + siteID
}

// DeploymentChannel returns the channel name for a deployment
func DeploymentChannel(deploymentID string) string {
	return "deployment." + deploymentID
}

// UserChannel returns the channel name for a user
func UserChannel(userID string) string {
	return "user." + userID
}

// TaskChannel returns the channel name for a task
func TaskChannel(taskID string) string {
	return "task." + taskID
}
