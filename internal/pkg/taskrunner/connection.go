package taskrunner

import (
	"time"

	"github.com/kkz6/launch-go/internal/pkg/pathutil"
	"github.com/kkz6/launch-go/internal/pkg/timeout"
)

// Connection represents SSH connection details
type Connection struct {
	Host       string
	Port       int
	User       string
	PrivateKey string
	ScriptPath string // Remote path for scripts (default: ~/.launch-tasks)
}

// GetScriptPath returns the script storage path on the remote server
func (c *Connection) GetScriptPath() string {
	if c.ScriptPath == "" {
		return pathutil.GetTaskDir(c.User)
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
// If t is not provided, timeout.SSH (30 seconds) is used.
func (c *Connection) NewSSHClient(t ...time.Duration) (*SSHClient, error) {
	sshTimeout := timeout.SSH
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
		Timeout:    sshTimeout,
	})
}

// Dial creates an SSH client and establishes the connection.
// This is a convenience method that combines NewSSHClient and Connect.
// If t is not provided, timeout.SSH (30 seconds) is used.
func (c *Connection) Dial(t ...time.Duration) (*SSHClient, error) {
	client, err := c.NewSSHClient(t...)
	if err != nil {
		return nil, err
	}

	if err := client.Connect(); err != nil {
		client.Close()
		return nil, err
	}

	return client, nil
}
