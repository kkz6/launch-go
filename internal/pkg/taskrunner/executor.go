package taskrunner

// ServerConnection provides SSH connection details for a server.
// This interface allows the task runner to work without importing server models.
type ServerConnection interface {
	// GetID returns the server's unique identifier
	GetID() string
	// GetTeamID returns the team ID for broadcasting
	GetTeamID() string
	// GetIPAddress returns the server's public IP address
	GetIPAddress() string
	// GetSSHPort returns the SSH port (default 22)
	GetSSHPort() int
	// GetPrivateKey returns the SSH private key
	GetPrivateKey() string
	// GetSudoPassword returns the password for sudo operations
	GetSudoPassword() string
	// GetUsername returns the default username for the server
	GetUsername() string
	// ConnectionAsRoot returns an SSH connection as root
	ConnectionAsRoot() *Connection
	// ConnectionAsUser returns an SSH connection as the given user
	ConnectionAsUser(username ...string) *Connection
}
