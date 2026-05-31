package taskrunner

import (
	"time"

	"github.com/kkz6/launch-go/internal/pkg/config"
	"github.com/kkz6/launch-go/internal/pkg/launch/paths"
)

// Connection represents SSH connection details
type Connection struct {
	Host       string
	Port       int
	User       string
	PrivateKey string
	ScriptPath string // Remote path for scripts (default: ~/.launch)

	// ServerID + HostKey power TOFU host-key verification.
	//
	//   HostKey == "" → first connect on a server that's never been
	//                   pinned. The callback captures the live key
	//                   and persists it via the package-level
	//                   persister registered at boot.
	//   HostKey != "" → every subsequent connect compares against
	//                   the stored value and refuses on mismatch.
	//
	// ServerID is needed only so the persister knows which row to
	// update on the TOFU path; callers connecting to one-off boxes
	// (e.g. fresh provisions where the row already exists but the
	// model hasn't been refetched) can leave it empty and the
	// persister silently no-ops.
	ServerID string
	HostKey  string
}

// GetScriptPath returns the script storage path on the remote server
func (c *Connection) GetScriptPath() string {
	if c.ScriptPath == "" {
		return paths.GetTaskDir(c.User)
	}
	return c.ScriptPath
}

// Is checks if this connection matches another connection
func (c *Connection) Is(other *Connection) bool {
	if c == nil || other == nil {
		return c == other
	}
	return c.Host == other.Host &&
		c.Port == other.Port &&
		c.User == other.User
}

// NewSSHClient creates an SSH client from this connection configuration.
// If t is not provided, config.SSH (30 seconds) is used.
func (c *Connection) NewSSHClient(t ...time.Duration) (*SSHClient, error) {
	sshTimeout := config.SSH
	if len(t) > 0 && t[0] > 0 {
		sshTimeout = t[0]
	}

	port := c.Port
	if port == 0 {
		port = 22
	}

	return NewSSHClient(SSHConfig{
		Host:       c.Host,
		Port:       port,
		User:       c.User,
		PrivateKey: c.PrivateKey,
		ServerID:   c.ServerID,
		HostKey:    c.HostKey,
		Timeout:    sshTimeout,
	})
}

// Dial creates an SSH client and establishes the connection.
// This is a convenience method that combines NewSSHClient and Connect.
// If t is not provided, config.SSH (30 seconds) is used.
func (c *Connection) Dial(t ...time.Duration) (*SSHClient, error) {
	client, err := c.NewSSHClient(t...)
	if err != nil {
		return nil, err
	}

	if err := client.Connect(); err != nil {
		_ = client.Close()
		return nil, err
	}

	return client, nil
}
