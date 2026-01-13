package models

// Broadcastable is an interface for models that can broadcast their changes
// to WebSocket clients. Models opt-in to broadcasting by implementing this interface.
//
// When a model implements Broadcastable, the service layer can call broadcast
// helpers after Create/Update/Delete operations to notify connected clients.
//
// Example implementation:
//
//	func (s *Server) GetTeamID() string {
//	    return s.TeamID
//	}
//
//	func (s *Server) BroadcastName() string {
//	    return "server"
//	}
//
//	func (s *Server) BroadcastPayload() map[string]interface{} {
//	    return map[string]interface{}{
//	        "id":   s.ID,
//	        "name": s.Name,
//	    }
//	}
type Broadcastable interface {
	// GetTeamID returns the team ID that this model belongs to.
	// This is used to determine which WebSocket channel to broadcast to.
	// All broadcasts go to team.{teamID} channel.
	GetTeamID() string

	// BroadcastName returns the model name used in the event name.
	// For example: "server", "cron", "site", "deployment"
	// The event will be: {name}.created, {name}.updated, {name}.deleted
	BroadcastName() string

	// BroadcastPayload returns the data to include in the broadcast.
	// This should contain enough information for the frontend to identify
	// which resource changed and optionally refetch the relevant data.
	BroadcastPayload() map[string]interface{}
}

// BroadcastableWithServer is implemented by models that belong to a server
// and need to include server_id in the broadcast payload.
type BroadcastableWithServer interface {
	Broadcastable
	GetServerID() string
}

// BroadcastableWithSite is implemented by models that belong to a site
// and need to include site_id in the broadcast payload.
type BroadcastableWithSite interface {
	Broadcastable
	GetSiteID() string
}
