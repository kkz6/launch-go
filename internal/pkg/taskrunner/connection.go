package taskrunner

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
