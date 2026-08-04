package taskrunner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kkz6/launch-go/internal/pkg/config"
	"github.com/oklog/ulid/v2"

	"golang.org/x/crypto/ssh"
)

// SSHClient handles SSH connections and remote command execution
type SSHClient struct {
	config  *ssh.ClientConfig
	conn    sshConnection
	host    string
	port    int
	timeout time.Duration
	deps    sshClientDependencies
}

// sshConnection and sshSession keep the client coupled to the small portion of
// x/crypto/ssh it actually uses. The adapters below are the production path;
// the boundary also lets tests exercise transport failures that a concrete
// ssh.Session cannot otherwise produce deterministically.
type sshConnection interface {
	Close() error
	NewSession() (sshSession, error)
	SendRequest(name string, wantReply bool, payload []byte) (bool, []byte, error)
}

type sshSession interface {
	Close() error
	Run(command string) error
	Signal(signal ssh.Signal) error
	StdinPipe() (io.WriteCloser, error)
	StdoutPipe() (io.Reader, error)
	StderrPipe() (io.Reader, error)
	Start(command string) error
	Wait() error
	SetStdout(writer io.Writer)
	SetStderr(writer io.Writer)
}

type cryptoSSHConnection struct {
	client *ssh.Client
}

func (c *cryptoSSHConnection) Close() error {
	return c.client.Close()
}

func (c *cryptoSSHConnection) NewSession() (sshSession, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return nil, err
	}
	return &cryptoSSHSession{session: session}, nil
}

func (c *cryptoSSHConnection) SendRequest(name string, wantReply bool, payload []byte) (bool, []byte, error) {
	return c.client.SendRequest(name, wantReply, payload)
}

type cryptoSSHSession struct {
	session *ssh.Session
}

func (s *cryptoSSHSession) Close() error                       { return s.session.Close() }
func (s *cryptoSSHSession) Run(command string) error           { return s.session.Run(command) }
func (s *cryptoSSHSession) Signal(signal ssh.Signal) error     { return s.session.Signal(signal) }
func (s *cryptoSSHSession) StdinPipe() (io.WriteCloser, error) { return s.session.StdinPipe() }
func (s *cryptoSSHSession) StdoutPipe() (io.Reader, error)     { return s.session.StdoutPipe() }
func (s *cryptoSSHSession) StderrPipe() (io.Reader, error)     { return s.session.StderrPipe() }
func (s *cryptoSSHSession) Start(command string) error         { return s.session.Start(command) }
func (s *cryptoSSHSession) Wait() error                        { return s.session.Wait() }
func (s *cryptoSSHSession) SetStdout(writer io.Writer)         { s.session.Stdout = writer }
func (s *cryptoSSHSession) SetStderr(writer io.Writer)         { s.session.Stderr = writer }

type sshRetryTimer interface {
	C() <-chan time.Time
	Stop() bool
}

type realSSHRetryTimer struct {
	timer *time.Timer
}

func (t *realSSHRetryTimer) C() <-chan time.Time { return t.timer.C }
func (t *realSSHRetryTimer) Stop() bool          { return t.timer.Stop() }

type sshClientDependencies struct {
	dialSSH  func(network, address string, config *ssh.ClientConfig) (sshConnection, error)
	dialTCP  func(network, address string, timeout time.Duration) (net.Conn, error)
	newTimer func(duration time.Duration) sshRetryTimer
}

func defaultSSHClientDependencies() sshClientDependencies {
	return sshClientDependencies{
		dialSSH: func(network, address string, config *ssh.ClientConfig) (sshConnection, error) {
			client, err := ssh.Dial(network, address, config)
			if err != nil {
				return nil, err
			}
			return &cryptoSSHConnection{client: client}, nil
		},
		dialTCP: net.DialTimeout,
		newTimer: func(duration time.Duration) sshRetryTimer {
			return &realSSHRetryTimer{timer: time.NewTimer(duration)}
		},
	}
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

	// ServerID + HostKey wire trust-on-first-use host-key
	// verification. See Connection for the semantics. If both are
	// empty the connection falls back to the legacy
	// ssh.InsecureIgnoreHostKey behaviour (used by ad-hoc CLI tools
	// that aren't tied to a Server row), but every real path that
	// talks to a Launch-managed server should populate them.
	ServerID string
	HostKey  string
}

// HostKeyPersister captures a freshly-pinned SSH host key for the
// given server. Called exactly once per server, the first time we
// see its host key — subsequent connects use the stored value.
//
// The package-level value below is set at process boot from cmd/api
// and cmd/worker via RegisterHostKeyPersister; everything else stays
// out of the wiring path.
type HostKeyPersister func(ctx context.Context, serverID, hostKey string) error

var hostKeyPersister HostKeyPersister

// RegisterHostKeyPersister installs the function the SSH client
// calls when it discovers a server's host key for the first time
// (TOFU). Called once at process startup from cmd/api / cmd/worker.
// Safe to leave unregistered in unit tests — the SSH client just
// skips the persist step.
func RegisterHostKeyPersister(p HostKeyPersister) {
	hostKeyPersister = p
}

// encodeHostKey returns the wire-format public key as
// "<algo> <base64-blob>", matching the format OpenSSH writes into
// known_hosts. Storing in this shape lets a human eyeball the value
// in the DB or `ssh-keygen -l -f` it later for fingerprinting.
func encodeHostKey(key ssh.PublicKey) string {
	return strings.TrimSpace(key.Type() + " " + encodeBase64(key.Marshal()))
}

func encodeBase64(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

// makeHostKeyCallback returns the right HostKeyCallback for the given
// (serverID, pinnedKey) pair:
//
//   - Empty pinnedKey AND empty serverID  → legacy permissive mode.
//     Used by ad-hoc CLI tools that aren't bound to a Server row.
//     This is the only path that still skips verification, and it's
//     scoped to callers that explicitly opted out by not populating
//     the fields.
//   - Empty pinnedKey AND non-empty serverID → TOFU. Capture the
//     live key, hand it to the registered persister so subsequent
//     connections pin against it. Returns nil so the handshake
//     succeeds the first time.
//   - Non-empty pinnedKey → strict compare against the live key's
//     wire format. Mismatch fails the handshake with a clear error
//     so a MITM or IP-reuse situation surfaces immediately rather
//     than silently succeeding into the wrong host.
func makeHostKeyCallback(serverID, pinnedKey string) ssh.HostKeyCallback {
	if pinnedKey == "" && serverID == "" {
		return ssh.InsecureIgnoreHostKey()
	}
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		live := encodeHostKey(key)
		if pinnedKey == "" {
			// TOFU. A managed server must be pinned before its first
			// connection is accepted; otherwise every later connection
			// would silently trust whichever host answers the address.
			if hostKeyPersister == nil {
				return fmt.Errorf("SSH host key persister is not configured for server %s", serverID)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := hostKeyPersister(ctx, serverID, live); err != nil {
				return fmt.Errorf("persist SSH host key for server %s: %w", serverID, err)
			}
			return nil
		}
		if live == pinnedKey {
			return nil
		}
		return fmt.Errorf("ssh: host key mismatch for %s (server %s) — possible MITM. Expected %q, got %q",
			hostname, serverID, pinnedKey, live)
	}
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

	t := cfg.Timeout
	if t == 0 {
		t = config.SSH
	}

	sshConfig := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            authMethods,
		HostKeyCallback: makeHostKeyCallback(cfg.ServerID, cfg.HostKey),
		Timeout:         t,
	}

	port := cfg.Port
	if port == 0 {
		port = 22
	}

	return &SSHClient{
		config:  sshConfig,
		host:    cfg.Host,
		port:    port,
		timeout: t,
		deps:    defaultSSHClientDependencies(),
	}, nil
}

// Connect establishes the SSH connection
func (c *SSHClient) Connect() error {
	addr := fmt.Sprintf("%s:%d", c.host, c.port)

	conn, err := c.deps.dialSSH("tcp", addr, c.config)
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
	session, err := c.conn.NewSession()
	if err != nil {
		return nil, err
	}
	cryptoSession, ok := session.(*cryptoSSHSession)
	if !ok {
		return nil, fmt.Errorf("unexpected SSH session implementation %T", session)
	}
	return cryptoSession.session, nil
}

// SendRequest sends a global SSH request on the established connection.
// It is used by long-lived streams for keepalive probes while preserving
// taskrunner's managed host-key verification and connection setup.
func (c *SSHClient) SendRequest(name string, wantReply bool, payload []byte) (bool, error) {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return false, err
		}
	}
	ok, _, err := c.conn.SendRequest(name, wantReply, payload)
	return ok, err
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
	session.SetStdout(&stdout)
	session.SetStderr(&stderr)

	// Handle context cancellation
	done := make(chan error, 1)
	go func() {
		done <- session.Run(command)
	}()

	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGTERM)
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
	// Upload the script to a temp file and execute the file, rather than
	// `bash -c "<script>"`. Passing a multi-line script through bash -c is
	// unsafe: Go's %q escapes the script's newlines to the literal two
	// characters `\n`, so the remote shell receives a single broken line
	// — the `#!`-prefixed blob collapses into one comment and the script
	// silently no-ops (exit 0, no output). Writing the script to a file
	// preserves it verbatim, matching how the async dispatcher runs tasks.
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	remotePath := fmt.Sprintf("/tmp/launch-runscript-%s.sh", ulid.Make())
	if err := c.Upload(ctx, []byte(script), remotePath, 0o700); err != nil {
		return nil, fmt.Errorf("upload script: %w", err)
	}

	// Run the file, then remove it while preserving the script's exit code.
	quotedPath := quoteShellArg(remotePath)
	cmd := fmt.Sprintf("bash %s; ec=$?; rm -f %s; exit $ec", quotedPath, quotedPath)
	return c.Run(ctx, cmd)
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

	session.SetStdout(output)
	session.SetStderr(output)

	done := make(chan error, 1)
	go func() {
		done <- session.Run(command)
	}()

	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGTERM)
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

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("open SCP stdin: %w", err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("open SCP stdout: %w", err)
	}
	dir := filepath.Dir(remotePath)
	if err := session.Start("scp -tr " + quoteShellArg(dir)); err != nil {
		return fmt.Errorf("start SCP upload: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- writeSCPFile(stdin, stdout, content, filepath.Base(remotePath), mode, session.Wait)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		_ = session.Close()
		return ctx.Err()
	}
}

// writeSCPFile performs the acknowledgement-based SCP sink protocol used by
// `scp -t`. Checking every acknowledgement prevents a failed remote write from
// being reported as a successful upload.
func writeSCPFile(stdin io.WriteCloser, stdout io.Reader, content []byte, name string, mode os.FileMode, wait func() error) error {
	defer stdin.Close()
	reader := bufio.NewReader(stdout)

	if err := readSCPAck(reader); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stdin, "C%04o %d %s\n", mode.Perm(), len(content), name); err != nil {
		return fmt.Errorf("write SCP header: %w", err)
	}
	if err := readSCPAck(reader); err != nil {
		return err
	}
	if _, err := io.Copy(stdin, bytes.NewReader(content)); err != nil {
		return fmt.Errorf("write SCP content: %w", err)
	}
	if _, err := stdin.Write([]byte{0}); err != nil {
		return fmt.Errorf("finish SCP content: %w", err)
	}
	if err := readSCPAck(reader); err != nil {
		return err
	}
	// The SCP sink waits for EOF before it exits. Close stdin before waiting
	// for the remote process; deferring this close until the function returns
	// deadlocks on session.Wait until the caller's context expires.
	if err := stdin.Close(); err != nil {
		return fmt.Errorf("close SCP stdin: %w", err)
	}
	if err := wait(); err != nil {
		return fmt.Errorf("complete SCP upload: %w", err)
	}
	return nil
}

func readSCPAck(reader *bufio.Reader) error {
	code, err := reader.ReadByte()
	if err != nil {
		return fmt.Errorf("read SCP acknowledgement: %w", err)
	}
	if code == 0 {
		return nil
	}
	message, _ := reader.ReadString('\n')
	return fmt.Errorf("SCP rejected upload (%d): %s", code, strings.TrimSpace(message))
}

// Download downloads content from a remote file
func (c *SSHClient) Download(ctx context.Context, remotePath string) ([]byte, error) {
	result, err := c.Run(ctx, "cat -- "+quoteShellArg(remotePath))
	if err != nil {
		return nil, err
	}
	return []byte(result.Stdout), nil
}

// FileExists checks if a file exists on the remote server
func (c *SSHClient) FileExists(ctx context.Context, path string) (bool, error) {
	result, err := c.Run(ctx, "test -f "+quoteShellArg(path))
	if err != nil {
		return false, fmt.Errorf("check remote file: %w", err)
	}
	return result.ExitCode == 0, nil
}

// DirExists checks if a directory exists on the remote server
func (c *SSHClient) DirExists(ctx context.Context, path string) (bool, error) {
	result, err := c.Run(ctx, "test -d "+quoteShellArg(path))
	if err != nil {
		return false, fmt.Errorf("check remote directory: %w", err)
	}
	return result.ExitCode == 0, nil
}

// MkdirAll creates a directory and all parent directories on the remote server
func (c *SSHClient) MkdirAll(ctx context.Context, path string) error {
	_, err := c.Run(ctx, "mkdir -p "+quoteShellArg(path))
	return err
}

// StreamOutput streams output from a command, calling the callback for each line
// This keeps the SSH connection open and reads output as it arrives
func (c *SSHClient) StreamOutput(ctx context.Context, command string, callback func(line string) error) error {
	if callback == nil {
		return fmt.Errorf("stream callback is required")
	}
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

	readCtx, cancelReaders := context.WithCancel(ctx)
	defer cancelReaders()
	events := make(chan streamReadEvent, 100)
	go readStreamLines(readCtx, newLineReader(stdout), events)
	go readStreamLines(readCtx, newLineReader(stderr), events)

	// Process lines until context is cancelled or stream ends
	readers := 2
	for readers > 0 {
		select {
		case <-ctx.Done():
			_ = session.Signal(ssh.SIGTERM)
			return ctx.Err()
		case event := <-events:
			if event.err != nil {
				_ = session.Signal(ssh.SIGTERM)
				return event.err
			}
			if event.done {
				readers--
				continue
			}
			if err := callback(event.line); err != nil {
				_ = session.Signal(ssh.SIGTERM)
				return err
			}
		}
	}

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- session.Wait()
	}()
	select {
	case err := <-waitDone:
		return err
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGTERM)
		return ctx.Err()
	}
}

type streamReadEvent struct {
	line string
	err  error
	done bool
}

func readStreamLines(ctx context.Context, reader *lineReader, events chan<- streamReadEvent) {
	defer sendStreamEvent(ctx, events, streamReadEvent{done: true})
	for {
		line, err := reader.readLine()
		if line != "" || err == nil {
			if !sendStreamEvent(ctx, events, streamReadEvent{line: line}) {
				return
			}
		}
		if err != nil {
			if err != io.EOF {
				_ = sendStreamEvent(ctx, events, streamReadEvent{err: err})
			}
			return
		}
	}
}

func sendStreamEvent(ctx context.Context, events chan<- streamReadEvent, event streamReadEvent) bool {
	select {
	case events <- event:
		return true
	case <-ctx.Done():
		return false
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

		conn, err := c.deps.dialTCP("tcp", fmt.Sprintf("%s:%d", c.host, c.port), config.NetworkDial)
		if err == nil {
			_ = conn.Close()
			return c.Connect()
		}

		lastErr = err
		timer := c.deps.newTimer(config.RetryDelay)
		select {
		case <-ctx.Done():
			_ = timer.Stop()
			return ctx.Err()
		case <-timer.C():
		}
	}
	if lastErr == nil {
		return fmt.Errorf("failed to connect after %d retries", maxRetries)
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
	return strings.TrimSuffix(line, "\n"), err
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

func quoteShellArg(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

// ShellQuote returns a POSIX-shell-safe single argument. Task templates use
// this when embedding paths or identifiers in remote commands; keeping the
// implementation here avoids each package inventing subtly unsafe escaping.
func ShellQuote(value string) string {
	return quoteShellArg(value)
}

// SSHClientOption is a functional option for configuring SSHClient
type SSHClientOption func(*SSHConfig)

// WithSSHTimeout sets the connection timeout for SSH clients
func WithSSHTimeout(t time.Duration) SSHClientOption {
	return func(cfg *SSHConfig) {
		cfg.Timeout = t
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
		ServerID:   conn.ServerID,
		HostKey:    conn.HostKey,
		Timeout:    config.SSH,
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
