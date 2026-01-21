package taskrunner

import "time"

// DefaultSSHTimeout is the default timeout for SSH connections
const DefaultSSHTimeout = 30 * time.Second

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
		if c.User == "root" {
			return "/root/.launch-tasks"
		}
		return "/home/" + c.User + "/.launch-tasks"
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
// If timeout is not provided, DefaultSSHTimeout (30 seconds) is used.
func (c *Connection) NewSSHClient(timeout ...time.Duration) (*SSHClient, error) {
	t := DefaultSSHTimeout
	if len(timeout) > 0 && timeout[0] > 0 {
		t = timeout[0]
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
		Timeout:    t,
	})
}

// Dial creates an SSH client and establishes the connection.
// This is a convenience method that combines NewSSHClient and Connect.
// If timeout is not provided, DefaultSSHTimeout (30 seconds) is used.
func (c *Connection) Dial(timeout ...time.Duration) (*SSHClient, error) {
	client, err := c.NewSSHClient(timeout...)
	if err != nil {
		return nil, err
	}

	if err := client.Connect(); err != nil {
		client.Close()
		return nil, err
	}

	return client, nil
}
