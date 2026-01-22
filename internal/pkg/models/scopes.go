package models

// TeamScoped provides a TeamID field for models that belong to a team.
// This is the primary multi-tenancy scope - most resources belong to a team.
//
// Usage:
//
//	type MyModel struct {
//	    models.BaseModel
//	    models.TeamScoped
//	}
type TeamScoped struct {
	TeamID string `gorm:"column:team_id;type:char(26);not null;index" json:"team_id"`
}

// GetTeamID returns the team ID
func (t *TeamScoped) GetTeamID() string {
	return t.TeamID
}

// SetTeamID sets the team ID
func (t *TeamScoped) SetTeamID(teamID string) {
	t.TeamID = teamID
}

// ServerScoped provides a ServerID field for models that belong to a server.
// Use this for resources like crons, daemons, firewall rules, etc.
//
// Usage:
//
//	type MyModel struct {
//	    models.BaseModel
//	    models.ServerScoped
//	}
type ServerScoped struct {
	ServerID string `gorm:"column:server_id;type:char(26);not null;index" json:"server_id"`
}

// GetServerID returns the server ID
func (s *ServerScoped) GetServerID() string {
	return s.ServerID
}

// SetServerID sets the server ID
func (s *ServerScoped) SetServerID(serverID string) {
	s.ServerID = serverID
}

// SiteScoped provides a SiteID field for models that belong to a site.
// Use this for resources like deployments, certificates, queues, etc.
//
// Usage:
//
//	type MyModel struct {
//	    models.BaseModel
//	    models.SiteScoped
//	}
type SiteScoped struct {
	SiteID string `gorm:"column:site_id;type:char(26);not null;index" json:"site_id"`
}

// GetSiteID returns the site ID
func (s *SiteScoped) GetSiteID() string {
	return s.SiteID
}

// SetSiteID sets the site ID
func (s *SiteScoped) SetSiteID(siteID string) {
	s.SiteID = siteID
}

// UserScoped provides a UserID field for models that belong to a user.
// Use this for resources like personal access tokens, passkeys, etc.
//
// Usage:
//
//	type MyModel struct {
//	    models.BaseModel
//	    models.UserScoped
//	}
type UserScoped struct {
	UserID string `gorm:"column:user_id;type:char(26);not null;index" json:"user_id"`
}

// GetUserID returns the user ID
func (u *UserScoped) GetUserID() string {
	return u.UserID
}

// SetUserID sets the user ID
func (u *UserScoped) SetUserID(userID string) {
	u.UserID = userID
}

// TeamServerScoped combines TeamScoped and ServerScoped for models that need both.
// This is useful for resources that belong to a server but also need direct team access.
//
// Usage:
//
//	type MyModel struct {
//	    models.BaseModel
//	    models.TeamServerScoped
//	}
type TeamServerScoped struct {
	TeamScoped
	ServerScoped
}

// TeamSiteScoped combines TeamScoped and SiteScoped for models that need both.
// This is useful for resources that belong to a site but also need direct team access.
//
// Usage:
//
//	type MyModel struct {
//	    models.BaseModel
//	    models.TeamSiteScoped
//	}
type TeamSiteScoped struct {
	TeamScoped
	SiteScoped
}

// ServerSiteScoped combines ServerScoped and SiteScoped for models that need both.
// This is useful for resources that belong to both a server and a specific site.
//
// Usage:
//
//	type MyModel struct {
//	    models.BaseModel
//	    models.ServerSiteScoped
//	}
type ServerSiteScoped struct {
	ServerScoped
	SiteScoped
}

// FullScoped combines TeamScoped, ServerScoped, and SiteScoped for maximum context.
// Use sparingly - most models don't need all three scopes.
//
// Usage:
//
//	type MyModel struct {
//	    models.BaseModel
//	    models.FullScoped
//	}
type FullScoped struct {
	TeamScoped
	ServerScoped
	SiteScoped
}
