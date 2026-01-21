package taskrunner

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHClient handles SSH connections and remote command execution
type SSHClient struct {
	config  *ssh.ClientConfig
	conn    *ssh.Client
	host    string
	port    int
	timeout time.Duration
}

// SSHConfig holds SSH connection configuration
type SSHConfig struct {
	Host           string
	Port           int
	User           string
	PrivateKey     string
	PrivateKeyPath string
	Password       string
	Timeout        time.Duration
}

// SSHCommandResult contains the result of a remote command execution
type SSHCommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// NewSSHClient creates a new SSH client with the given configuration
func NewSSHClient(cfg SSHConfig) (*SSHClient, error) {
	var authMethods []ssh.AuthMethod

	// Private key authentication
	if cfg.PrivateKey != "" {
		signer, err := ssh.ParsePrivateKey([]byte(cfg.PrivateKey))
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	} else if cfg.PrivateKeyPath != "" {
		key, err := os.ReadFile(expandPath(cfg.PrivateKeyPath))
		if err != nil {
			return nil, fmt.Errorf("failed to read private key file: %w", err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	// Password authentication
	if cfg.Password != "" {
		authMethods = append(authMethods, ssh.Password(cfg.Password))
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no authentication method provided")
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	sshConfig := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: Implement proper host key verification
		Timeout:         timeout,
	}

	port := cfg.Port
	if port == 0 {
		port = 22
	}

	return &SSHClient{
		config:  sshConfig,
		host:    cfg.Host,
		port:    port,
		timeout: timeout,
	}, nil
}

// Connect establishes the SSH connection
func (c *SSHClient) Connect() error {
	addr := fmt.Sprintf("%s:%d", c.host, c.port)

	conn, err := ssh.Dial("tcp", addr, c.config)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	c.conn = conn
	return nil
}

// Close closes the SSH connection
func (c *SSHClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// NewSession creates a new SSH session for interactive use (PTY, shell, etc.)
func (c *SSHClient) NewSession() (*ssh.Session, error) {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}
	return c.conn.NewSession()
}

// Run executes a command on the remote server
func (c *SSHClient) Run(ctx context.Context, command string) (*SSHCommandResult, error) {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	session, err := c.conn.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	// Handle context cancellation
	done := make(chan error, 1)
	go func() {
		done <- session.Run(command)
	}()

	select {
	case <-ctx.Done():
		session.Signal(ssh.SIGTERM)
		return nil, ctx.Err()
	case err := <-done:
		result := &SSHCommandResult{
			Stdout: stdout.String(),
			Stderr: stderr.String(),
		}

		if err != nil {
			exitErr, ok := err.(*ssh.ExitError)
			if !ok {
				return result, err
			}
			result.ExitCode = exitErr.ExitStatus()
		}

		return result, nil
	}
}

// RunScript executes a script on the remote server
func (c *SSHClient) RunScript(ctx context.Context, script string) (*SSHCommandResult, error) {
	// Wrap script in bash
	command := fmt.Sprintf("bash -c %q", script)
	return c.Run(ctx, command)
}

// RunWithOutput executes a command and writes output to the provided writer
func (c *SSHClient) RunWithOutput(ctx context.Context, command string, output io.Writer) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}

	session, err := c.conn.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	session.Stdout = output
	session.Stderr = output

	done := make(chan error, 1)
	go func() {
		done <- session.Run(command)
	}()

	select {
	case <-ctx.Done():
		session.Signal(ssh.SIGTERM)
		return ctx.Err()
	case err := <-done:
		return err
	}
}

// Upload uploads content to a remote file
func (c *SSHClient) Upload(ctx context.Context, content []byte, remotePath string, mode os.FileMode) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}

	session, err := c.conn.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	go func() {
		w, _ := session.StdinPipe()
		defer w.Close()

		_, _ = fmt.Fprintf(w, "C%04o %d %s\n", mode, len(content), filepath.Base(remotePath))
		_, _ = w.Write(content)
		_, _ = fmt.Fprint(w, "\x00")
	}()

	dir := filepath.Dir(remotePath)
	return session.Run(fmt.Sprintf("scp -tr %s", dir))
}

// Download downloads content from a remote file
func (c *SSHClient) Download(ctx context.Context, remotePath string) ([]byte, error) {
	result, err := c.Run(ctx, fmt.Sprintf("cat %s", remotePath))
	if err != nil {
		return nil, err
	}
	return []byte(result.Stdout), nil
}

// FileExists checks if a file exists on the remote server
func (c *SSHClient) FileExists(ctx context.Context, path string) (bool, error) {
	result, err := c.Run(ctx, fmt.Sprintf("test -f %s && echo 'exists'", path))
	if err != nil {
		return false, nil // File doesn't exist
	}
	return strings.TrimSpace(result.Stdout) == "exists", nil
}

// DirExists checks if a directory exists on the remote server
func (c *SSHClient) DirExists(ctx context.Context, path string) (bool, error) {
	result, err := c.Run(ctx, fmt.Sprintf("test -d %s && echo 'exists'", path))
	if err != nil {
		return false, nil
	}
	return strings.TrimSpace(result.Stdout) == "exists", nil
}

// MkdirAll creates a directory and all parent directories on the remote server
func (c *SSHClient) MkdirAll(ctx context.Context, path string) error {
	_, err := c.Run(ctx, fmt.Sprintf("mkdir -p %s", path))
	return err
}

// StreamOutput streams output from a command, calling the callback for each line
// This keeps the SSH connection open and reads output as it arrives
func (c *SSHClient) StreamOutput(ctx context.Context, command string, callback func(line string) error) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}

	session, err := c.conn.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := session.Start(command); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	// Create a combined reader
	combined := io.MultiReader(stdout, stderr)
	reader := newLineReader(combined)

	// Read lines in a goroutine
	lineChan := make(chan string, 100)
	errChan := make(chan error, 1)

	go func() {
		for {
			line, err := reader.readLine()
			if err != nil {
				if err != io.EOF {
					errChan <- err
				}
				close(lineChan)
				return
			}
			lineChan <- line
		}
	}()

	// Process lines until context is cancelled or stream ends
	for {
		select {
		case <-ctx.Done():
			session.Signal(ssh.SIGTERM)
			return ctx.Err()
		case err := <-errChan:
			return err
		case line, ok := <-lineChan:
			if !ok {
				// Stream ended
				return session.Wait()
			}
			if err := callback(line); err != nil {
				session.Signal(ssh.SIGTERM)
				return err
			}
		}
	}
}

// WaitForConnection waits for the SSH connection to become available
func (c *SSHClient) WaitForConnection(ctx context.Context, maxRetries int) error {
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", c.host, c.port), 5*time.Second)
		if err == nil {
			_ = conn.Close()
			return c.Connect()
		}

		lastErr = err
		time.Sleep(10 * time.Second)
	}

	return fmt.Errorf("failed to connect after %d retries: %w", maxRetries, lastErr)
}

// lineReader reads lines from an io.Reader
type lineReader struct {
	reader *bufio.Reader
}

// newLineReader creates a new lineReader
func newLineReader(r io.Reader) *lineReader {
	return &lineReader{reader: bufio.NewReader(r)}
}

// readLine reads a single line
func (r *lineReader) readLine() (string, error) {
	line, err := r.reader.ReadString('\n')
	if err != nil {
		return line, err
	}
	return strings.TrimSuffix(line, "\n"), nil
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

// SSHClientOption is a functional option for configuring SSHClient
type SSHClientOption func(*SSHConfig)

// WithSSHTimeout sets the connection timeout for SSH clients
func WithSSHTimeout(timeout time.Duration) SSHClientOption {
	return func(cfg *SSHConfig) {
		cfg.Timeout = timeout
	}
}

// WithSSHPort sets the SSH port
func WithSSHPort(port int) SSHClientOption {
	return func(cfg *SSHConfig) {
		cfg.Port = port
	}
}

// NewSSHClientFromConnection creates an SSH client from a Connection struct.
// This is the preferred way to create SSH clients when you have a Connection.
func NewSSHClientFromConnection(conn *Connection, opts ...SSHClientOption) (*SSHClient, error) {
	if conn == nil {
		return nil, fmt.Errorf("connection cannot be nil")
	}

	cfg := SSHConfig{
		Host:       conn.Host,
		Port:       conn.Port,
		User:       conn.User,
		PrivateKey: conn.PrivateKey,
		Timeout:    30 * time.Second,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	return NewSSHClient(cfg)
}

// NewSSHClientFromServer creates an SSH client from a ServerConnection.
// By default, connects as the server's default user.
func NewSSHClientFromServer(server ServerConnection, opts ...SSHClientOption) (*SSHClient, error) {
	if server == nil {
		return nil, fmt.Errorf("server cannot be nil")
	}

	return NewSSHClientFromConnection(server.ConnectionAsUser(), opts...)
}

// NewSSHClientFromServerAsRoot creates an SSH client from a ServerConnection,
// configured to connect as the root user.
func NewSSHClientFromServerAsRoot(server ServerConnection, opts ...SSHClientOption) (*SSHClient, error) {
	if server == nil {
		return nil, fmt.Errorf("server cannot be nil")
	}

	return NewSSHClientFromConnection(server.ConnectionAsRoot(), opts...)
}

// NewSSHClientFromServerAsUser creates an SSH client from a ServerConnection,
// configured to connect as the specified user.
func NewSSHClientFromServerAsUser(server ServerConnection, username string, opts ...SSHClientOption) (*SSHClient, error) {
	if server == nil {
		return nil, fmt.Errorf("server cannot be nil")
	}

	return NewSSHClientFromConnection(server.ConnectionAsUser(username), opts...)
}
