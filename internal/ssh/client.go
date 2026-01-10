package ssh

import (
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

type Client struct {
	config  *ssh.ClientConfig
	conn    *ssh.Client
	host    string
	port    int
	timeout time.Duration
}

type Config struct {
	Host           string
	Port           int
	User           string
	PrivateKey     string
	PrivateKeyPath string
	Password       string
	Timeout        time.Duration
}

type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

func NewClient(cfg Config) (*Client, error) {
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

	return &Client{
		config:  sshConfig,
		host:    cfg.Host,
		port:    port,
		timeout: timeout,
	}, nil
}

func (c *Client) Connect() error {
	addr := fmt.Sprintf("%s:%d", c.host, c.port)

	conn, err := ssh.Dial("tcp", addr, c.config)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	c.conn = conn
	return nil
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *Client) Run(ctx context.Context, command string) (*CommandResult, error) {
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
		result := &CommandResult{
			Stdout: stdout.String(),
			Stderr: stderr.String(),
		}

		if err != nil {
			if exitErr, ok := err.(*ssh.ExitError); ok {
				result.ExitCode = exitErr.ExitStatus()
			} else {
				return result, err
			}
		}

		return result, nil
	}
}

func (c *Client) RunScript(ctx context.Context, script string) (*CommandResult, error) {
	// Wrap script in bash
	command := fmt.Sprintf("bash -c %q", script)
	return c.Run(ctx, command)
}

func (c *Client) RunWithOutput(ctx context.Context, command string, output io.Writer) error {
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

func (c *Client) Upload(ctx context.Context, content []byte, remotePath string, mode os.FileMode) error {
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

		fmt.Fprintf(w, "C%04o %d %s\n", mode, len(content), filepath.Base(remotePath))
		w.Write(content)
		fmt.Fprint(w, "\x00")
	}()

	dir := filepath.Dir(remotePath)
	return session.Run(fmt.Sprintf("scp -tr %s", dir))
}

func (c *Client) Download(ctx context.Context, remotePath string) ([]byte, error) {
	result, err := c.Run(ctx, fmt.Sprintf("cat %s", remotePath))
	if err != nil {
		return nil, err
	}
	return []byte(result.Stdout), nil
}

func (c *Client) FileExists(ctx context.Context, path string) (bool, error) {
	result, err := c.Run(ctx, fmt.Sprintf("test -f %s && echo 'exists'", path))
	if err != nil {
		return false, nil // File doesn't exist
	}
	return strings.TrimSpace(result.Stdout) == "exists", nil
}

func (c *Client) DirExists(ctx context.Context, path string) (bool, error) {
	result, err := c.Run(ctx, fmt.Sprintf("test -d %s && echo 'exists'", path))
	if err != nil {
		return false, nil
	}
	return strings.TrimSpace(result.Stdout) == "exists", nil
}

func (c *Client) MkdirAll(ctx context.Context, path string) error {
	_, err := c.Run(ctx, fmt.Sprintf("mkdir -p %s", path))
	return err
}

func (c *Client) WaitForConnection(ctx context.Context, maxRetries int) error {
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", c.host, c.port), 5*time.Second)
		if err == nil {
			conn.Close()
			return c.Connect()
		}

		lastErr = err
		time.Sleep(10 * time.Second)
	}

	return fmt.Errorf("failed to connect after %d retries: %w", maxRetries, lastErr)
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}
