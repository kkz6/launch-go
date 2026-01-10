package server

type CreateServerRequest struct {
	Name         string `json:"name" validate:"required,min=2,max=255"`
	Provider     string `json:"provider" validate:"required,oneof=digitalocean hetzner aws linode vultr custom"`
	Region       string `json:"region" validate:"required"`
	Size         string `json:"size" validate:"required"`
	PHPVersion   string `json:"php_version" validate:"omitempty,oneof=8.1 8.2 8.3 8.4"`
	DatabaseType string `json:"database_type" validate:"omitempty,oneof=mysql postgres"`
	Credentials  string `json:"credentials" validate:"required_unless=Provider custom"`

	// For custom servers
	IPAddress  string `json:"ip_address" validate:"required_if=Provider custom,omitempty,ip"`
	SSHPort    int    `json:"ssh_port" validate:"omitempty,min=1,max=65535"`
	SSHUser    string `json:"ssh_user" validate:"omitempty,max=100"`
	PrivateKey string `json:"private_key" validate:"required_if=Provider custom"`
}

type UpdateServerRequest struct {
	Name       string `json:"name" validate:"required,min=2,max=255"`
	PHPVersion string `json:"php_version" validate:"omitempty,oneof=8.1 8.2 8.3 8.4"`
}

type CreateDatabaseRequest struct {
	Name     string `json:"name" validate:"required,min=1,max=64"`
	Type     string `json:"type" validate:"required,oneof=mysql postgres"`
	User     string `json:"user" validate:"omitempty,min=1,max=32"`
	Password string `json:"password" validate:"omitempty,min=8"`
}

type CreateSSHKeyRequest struct {
	Name      string `json:"name" validate:"required,min=1,max=255"`
	PublicKey string `json:"public_key" validate:"required"`
}

type CreateFirewallRuleRequest struct {
	Name     string `json:"name" validate:"omitempty,max=255"`
	Port     int    `json:"port" validate:"required,min=1,max=65535"`
	Protocol string `json:"protocol" validate:"omitempty,oneof=tcp udp"`
	FromIP   string `json:"from_ip" validate:"omitempty,ip|cidr"`
}

type CreateCronJobRequest struct {
	Command  string  `json:"command" validate:"required"`
	Schedule string  `json:"schedule" validate:"required"`
	User     string  `json:"user" validate:"omitempty,max=100"`
	SiteID   *string `json:"site_id" validate:"omitempty,ulid"`
}

type ServerResponse struct {
	ID               string  `json:"id"`
	TeamID           string  `json:"team_id"`
	Name             string  `json:"name"`
	Provider         string  `json:"provider"`
	IPAddress        *string `json:"ip_address,omitempty"`
	PrivateIPAddress *string `json:"private_ip_address,omitempty"`
	Region           string  `json:"region"`
	Size             string  `json:"size"`
	Status           string  `json:"status"`
	SSHPort          int     `json:"ssh_port"`
	SSHUser          string  `json:"ssh_user"`
	PHPVersion       string  `json:"php_version"`
	DatabaseType     *string `json:"database_type,omitempty"`
	WebServer        string  `json:"web_server"`
	ConnectedAt      *string `json:"connected_at,omitempty"`
	CreatedAt        string  `json:"created_at"`
	SitesCount       int     `json:"sites_count,omitempty"`
}

func ToServerResponse(server *Server) ServerResponse {
	resp := ServerResponse{
		ID:               server.ID,
		TeamID:           server.TeamID,
		Name:             server.Name,
		Provider:         string(server.Provider),
		IPAddress:        server.IPAddress,
		PrivateIPAddress: server.PrivateIPAddress,
		Region:           server.Region,
		Size:             server.Size,
		Status:           string(server.Status),
		SSHPort:          server.SSHPort,
		SSHUser:          server.SSHUser,
		PHPVersion:       server.PHPVersion,
		DatabaseType:     server.DatabaseType,
		WebServer:        server.WebServer,
		CreatedAt:        server.CreatedAt.Format("2006-01-02T15:04:05Z"),
		SitesCount:       len(server.Sites),
	}

	if server.ConnectedAt != nil {
		connected := server.ConnectedAt.Format("2006-01-02T15:04:05Z")
		resp.ConnectedAt = &connected
	}

	return resp
}
