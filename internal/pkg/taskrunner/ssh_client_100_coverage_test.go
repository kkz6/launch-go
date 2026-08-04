package taskrunner

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	cryptorand "crypto/rand"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kkz6/launch-go/internal/pkg/config"
	"golang.org/x/crypto/ssh"
)

type coverageSSHServer struct {
	listener net.Listener
	signer   ssh.Signer

	mu          sync.Mutex
	connections map[*ssh.ServerConn]struct{}
	uploads     map[string][]byte
	wg          sync.WaitGroup
	closeOnce   sync.Once
}

func startCoverageSSHServer(t *testing.T) *coverageSSHServer {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(cryptorand.Reader)
	if err != nil {
		t.Fatalf("generate SSH host key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatalf("create SSH host signer: %v", err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for SSH test server: %v", err)
	}

	server := &coverageSSHServer{
		listener:    listener,
		signer:      signer,
		connections: make(map[*ssh.ServerConn]struct{}),
		uploads:     make(map[string][]byte),
	}
	server.wg.Add(1)
	go server.serve()
	t.Cleanup(server.Close)
	return server
}

func (s *coverageSSHServer) address() (string, int) {
	host, portText, _ := net.SplitHostPort(s.listener.Addr().String())
	port, _ := strconv.Atoi(portText)
	return host, port
}

func (s *coverageSSHServer) hostKey() string {
	return encodeHostKey(s.signer.PublicKey())
}

func (s *coverageSSHServer) uploaded(path string) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]byte(nil), s.uploads[path]...)
}

func (s *coverageSSHServer) Close() {
	s.closeOnce.Do(func() {
		_ = s.listener.Close()
		s.mu.Lock()
		for conn := range s.connections {
			_ = conn.Close()
		}
		s.mu.Unlock()
		s.wg.Wait()
	})
}

func (s *coverageSSHServer) serve() {
	defer s.wg.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		s.wg.Add(1)
		go s.serveConnection(conn)
	}
}

func (s *coverageSSHServer) serveConnection(raw net.Conn) {
	defer s.wg.Done()
	serverConfig := &ssh.ServerConfig{
		PasswordCallback: func(_ ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			if string(password) != "secret" {
				return nil, errors.New("invalid password")
			}
			return nil, nil
		},
		PublicKeyCallback: func(ssh.ConnMetadata, ssh.PublicKey) (*ssh.Permissions, error) {
			return nil, nil
		},
	}
	serverConfig.AddHostKey(s.signer)
	conn, channels, requests, err := ssh.NewServerConn(raw, serverConfig)
	if err != nil {
		_ = raw.Close()
		return
	}

	s.mu.Lock()
	s.connections[conn] = struct{}{}
	s.mu.Unlock()
	defer func() {
		_ = conn.Close()
		s.mu.Lock()
		delete(s.connections, conn)
		s.mu.Unlock()
	}()

	requestDone := make(chan struct{})
	go func() {
		defer close(requestDone)
		for request := range requests {
			switch request.Type {
			case "keepalive-ok":
				_ = request.Reply(true, []byte("pong"))
			case "disconnect-now":
				_ = conn.Close()
			default:
				_ = request.Reply(false, nil)
			}
		}
	}()

	for newChannel := range channels {
		if newChannel.ChannelType() != "session" {
			_ = newChannel.Reject(ssh.UnknownChannelType, "unsupported channel")
			continue
		}
		channel, channelRequests, err := newChannel.Accept()
		if err != nil {
			continue
		}
		s.wg.Add(1)
		go s.serveSession(conn, channel, channelRequests)
	}
	<-requestDone
}

func (s *coverageSSHServer) serveSession(conn *ssh.ServerConn, channel ssh.Channel, requests <-chan *ssh.Request) {
	defer s.wg.Done()
	for request := range requests {
		switch request.Type {
		case "exec":
			var payload struct{ Command string }
			if err := ssh.Unmarshal(request.Payload, &payload); err != nil {
				_ = request.Reply(false, nil)
				_ = channel.Close()
				return
			}
			if payload.Command == "reject-start" {
				_ = request.Reply(false, nil)
				_ = channel.Close()
				return
			}
			_ = request.Reply(true, nil)
			go s.execute(conn, channel, payload.Command)
		case "signal":
			_ = request.Reply(true, nil)
			s.finish(channel, 143)
			return
		default:
			_ = request.Reply(false, nil)
		}
	}
}

func (s *coverageSSHServer) execute(conn *ssh.ServerConn, channel ssh.Channel, command string) {
	switch {
	case command == "success":
		_, _ = io.WriteString(channel, "standard output\n")
		_, _ = io.WriteString(channel.Stderr(), "standard error\n")
		s.finish(channel, 0)
	case command == "exit-7":
		_, _ = io.WriteString(channel, "failed output\n")
		_, _ = io.WriteString(channel.Stderr(), "failed error\n")
		s.finish(channel, 7)
	case command == "abrupt-disconnect":
		_ = conn.Close()
	case command == "hang":
		// The session request loop releases this command when SIGTERM arrives.
	case command == "stream":
		_, _ = io.WriteString(channel, "first\nsecond\n")
		_, _ = io.WriteString(channel.Stderr(), "third\n")
		s.finish(channel, 0)
	case command == "stream-exit":
		_, _ = io.WriteString(channel, "before exit\n")
		s.finish(channel, 9)
	case strings.HasPrefix(command, "scp -tr "):
		s.receiveSCP(channel, command)
	case strings.HasPrefix(command, "bash '/tmp/launch-runscript-"):
		_, _ = io.WriteString(channel, "script ran\n")
		s.finish(channel, 0)
	case strings.HasPrefix(command, "cat -- "):
		path := unquoteCoverageShellArg(strings.TrimPrefix(command, "cat -- "))
		data := s.uploaded(path)
		if data == nil {
			s.finish(channel, 1)
			return
		}
		_, _ = channel.Write(data)
		s.finish(channel, 0)
	case strings.HasPrefix(command, "test -f "):
		path := unquoteCoverageShellArg(strings.TrimPrefix(command, "test -f "))
		if s.uploaded(path) == nil {
			s.finish(channel, 1)
			return
		}
		s.finish(channel, 0)
	case strings.HasPrefix(command, "test -d "):
		path := unquoteCoverageShellArg(strings.TrimPrefix(command, "test -d "))
		if path == "/existing-dir" {
			s.finish(channel, 0)
			return
		}
		s.finish(channel, 1)
	case strings.HasPrefix(command, "mkdir -p "):
		s.finish(channel, 0)
	default:
		_, _ = io.WriteString(channel.Stderr(), "unknown command")
		s.finish(channel, 127)
	}
}

func (s *coverageSSHServer) receiveSCP(channel ssh.Channel, command string) {
	directory := unquoteCoverageShellArg(strings.TrimPrefix(command, "scp -tr "))
	if directory == "/upload-hang" {
		return
	}
	reader := bufio.NewReader(channel)
	_, _ = channel.Write([]byte{0})
	header, err := reader.ReadString('\n')
	if err != nil {
		_ = channel.Close()
		return
	}
	parts := strings.SplitN(strings.TrimSpace(header), " ", 3)
	if len(parts) != 3 {
		_, _ = channel.Write([]byte("\x01invalid header\n"))
		_ = channel.Close()
		return
	}
	size, err := strconv.Atoi(parts[1])
	if err != nil {
		_, _ = channel.Write([]byte("\x01invalid size\n"))
		_ = channel.Close()
		return
	}
	_, _ = channel.Write([]byte{0})
	data := make([]byte, size)
	if _, err := io.ReadFull(reader, data); err != nil {
		_ = channel.Close()
		return
	}
	if marker, err := reader.ReadByte(); err != nil || marker != 0 {
		_ = channel.Close()
		return
	}
	path := filepath.Join(directory, parts[2])
	s.mu.Lock()
	s.uploads[path] = append([]byte(nil), data...)
	s.mu.Unlock()
	_, _ = channel.Write([]byte{0})
	// The real scp sink exits only after the source closes stdin. Waiting for
	// EOF keeps the server from racing the client's explicit stdin close.
	_, _ = io.Copy(io.Discard, reader)
	s.finish(channel, 0)
}

func (s *coverageSSHServer) finish(channel ssh.Channel, status uint32) {
	_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{Status: status}))
	_ = channel.Close()
}

func unquoteCoverageShellArg(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'' {
		value = value[1 : len(value)-1]
	}
	return strings.ReplaceAll(value, "'\"'\"'", "'")
}

func newCoveragePasswordClient(t *testing.T, server *coverageSSHServer) *SSHClient {
	t.Helper()
	host, port := server.address()
	client, err := NewSSHClient(SSHConfig{
		Host:     host,
		Port:     port,
		User:     "deploy",
		Password: "secret",
		HostKey:  server.hostKey(),
		ServerID: "server-test",
		Timeout:  time.Second,
	})
	if err != nil {
		t.Fatalf("create SSH client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func coveragePrivateKey(t *testing.T) string {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(cryptorand.Reader)
	if err != nil {
		t.Fatalf("generate client key: %v", err)
	}
	block, err := ssh.MarshalPrivateKey(privateKey, "")
	if err != nil {
		t.Fatalf("marshal client key: %v", err)
	}
	return string(pem.EncodeToMemory(block))
}

type coverageLockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *coverageLockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *coverageLockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

func TestSSHClient100HostKeyAndConstructorPaths(t *testing.T) {
	key := testPublicKey{}
	live := encodeHostKey(key)

	legacy := makeHostKeyCallback("", "")
	if err := legacy("legacy", nil, key); err != nil {
		t.Fatalf("legacy host-key callback: %v", err)
	}

	originalPersister := hostKeyPersister
	t.Cleanup(func() { RegisterHostKeyPersister(originalPersister) })
	var persistedID, persistedKey string
	RegisterHostKeyPersister(func(ctx context.Context, serverID, hostKey string) error {
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("TOFU persistence context should have a deadline")
		}
		persistedID, persistedKey = serverID, hostKey
		return nil
	})
	if err := makeHostKeyCallback("server-1", "")("host", nil, key); err != nil {
		t.Fatalf("TOFU callback: %v", err)
	}
	if persistedID != "server-1" || persistedKey != live {
		t.Fatalf("persisted host key = (%q, %q), want (%q, %q)", persistedID, persistedKey, "server-1", live)
	}
	if err := makeHostKeyCallback("server-1", live)("host", nil, key); err != nil {
		t.Fatalf("matching pinned key: %v", err)
	}
	if err := makeHostKeyCallback("server-1", "ssh-ed25519 wrong")("host", nil, key); err == nil || !strings.Contains(err.Error(), "possible MITM") {
		t.Fatalf("mismatched pinned key error = %v", err)
	}
	RegisterHostKeyPersister(nil)
	if err := makeHostKeyCallback("server-1", "")("host", nil, key); err == nil || !strings.Contains(err.Error(), "persister is not configured") {
		t.Fatalf("missing persister error = %v", err)
	}

	if _, err := NewSSHClient(SSHConfig{PrivateKey: "not a key"}); err == nil || !strings.Contains(err.Error(), "parse private key") {
		t.Fatalf("inline private-key error = %v", err)
	}
	if _, err := NewSSHClient(SSHConfig{PrivateKeyPath: filepath.Join(t.TempDir(), "missing")}); err == nil || !strings.Contains(err.Error(), "read private key file") {
		t.Fatalf("missing private-key file error = %v", err)
	}
	invalidPath := filepath.Join(t.TempDir(), "invalid-key")
	if err := os.WriteFile(invalidPath, []byte("not a key"), 0o600); err != nil {
		t.Fatalf("write invalid key: %v", err)
	}
	if _, err := NewSSHClient(SSHConfig{PrivateKeyPath: invalidPath}); err == nil || !strings.Contains(err.Error(), "parse private key") {
		t.Fatalf("file private-key parse error = %v", err)
	}
	validPath := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(validPath, []byte(coveragePrivateKey(t)), 0o600); err != nil {
		t.Fatalf("write valid key: %v", err)
	}
	client, err := NewSSHClient(SSHConfig{
		Host:           "example.test",
		Port:           2202,
		User:           "deploy",
		PrivateKeyPath: validPath,
		Password:       "fallback",
		Timeout:        2 * time.Second,
	})
	if err != nil {
		t.Fatalf("file-key client: %v", err)
	}
	if client.port != 2202 || client.timeout != 2*time.Second || len(client.config.Auth) != 2 {
		t.Fatalf("unexpected explicit configuration: port=%d timeout=%s auth=%d", client.port, client.timeout, len(client.config.Auth))
	}
}

func TestSSHClient100RealTransportLifecycle(t *testing.T) {
	server := startCoverageSSHServer(t)
	client := newCoveragePasswordClient(t, server)

	if err := client.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("new public session: %v", err)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("close public session: %v", err)
	}
	ok, err := client.SendRequest("keepalive-ok", true, []byte("ping"))
	if err != nil || !ok {
		t.Fatalf("accepted global request = %v, %v", ok, err)
	}
	ok, err = client.SendRequest("keepalive-no", true, nil)
	if err != nil || ok {
		t.Fatalf("rejected global request = %v, %v", ok, err)
	}

	result, err := client.Run(context.Background(), "success")
	if err != nil {
		t.Fatalf("run success: %v", err)
	}
	if result.Stdout != "standard output\n" || result.Stderr != "standard error\n" || result.ExitCode != 0 {
		t.Fatalf("unexpected successful result: %+v", result)
	}
	result, err = client.Run(context.Background(), "exit-7")
	if err != nil {
		t.Fatalf("run exit status: %v", err)
	}
	if result.ExitCode != 7 || result.Stdout != "failed output\n" || result.Stderr != "failed error\n" {
		t.Fatalf("unexpected failed result: %+v", result)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	if _, err := client.Run(ctx, "hang"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("canceled run error = %v", err)
	}

	var output coverageLockedBuffer
	if err := client.RunWithOutput(context.Background(), "success", &output); err != nil {
		t.Fatalf("run with output: %v", err)
	}
	if !strings.Contains(output.String(), "standard output") || !strings.Contains(output.String(), "standard error") {
		t.Fatalf("combined output = %q", output.String())
	}
	if err := client.RunWithOutput(context.Background(), "exit-7", io.Discard); err == nil {
		t.Fatal("expected RunWithOutput to return remote exit error")
	}
	ctx, cancel = context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	if err := client.RunWithOutput(ctx, "hang", io.Discard); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("canceled RunWithOutput error = %v", err)
	}

	path := "/tmp/a path/it's-safe.txt"
	content := []byte("uploaded content\n")
	if err := client.Upload(context.Background(), content, path, 0o640); err != nil {
		t.Fatalf("upload: %v", err)
	}
	if got := server.uploaded(path); !bytes.Equal(got, content) {
		t.Fatalf("uploaded content = %q, want %q", got, content)
	}
	downloaded, err := client.Download(context.Background(), path)
	if err != nil || !bytes.Equal(downloaded, content) {
		t.Fatalf("download = %q, %v", downloaded, err)
	}
	exists, err := client.FileExists(context.Background(), path)
	if err != nil || !exists {
		t.Fatalf("existing file = %v, %v", exists, err)
	}
	exists, err = client.FileExists(context.Background(), "/tmp/missing")
	if err != nil || exists {
		t.Fatalf("missing file = %v, %v", exists, err)
	}
	exists, err = client.DirExists(context.Background(), "/existing-dir")
	if err != nil || !exists {
		t.Fatalf("existing directory = %v, %v", exists, err)
	}
	exists, err = client.DirExists(context.Background(), "/missing-dir")
	if err != nil || exists {
		t.Fatalf("missing directory = %v, %v", exists, err)
	}
	if err := client.MkdirAll(context.Background(), "/new dir/child"); err != nil {
		t.Fatalf("mkdir all: %v", err)
	}

	var lines []string
	if err := client.StreamOutput(context.Background(), "stream", func(line string) error {
		lines = append(lines, line)
		return nil
	}); err != nil {
		t.Fatalf("stream output: %v", err)
	}
	lineCounts := make(map[string]int, len(lines))
	firstIndex, secondIndex := -1, -1
	for index, line := range lines {
		lineCounts[line]++
		if line == "first" {
			firstIndex = index
		}
		if line == "second" {
			secondIndex = index
		}
	}
	if len(lines) != 3 || lineCounts["first"] != 1 || lineCounts["second"] != 1 || lineCounts["third"] != 1 || firstIndex >= secondIndex {
		t.Fatalf("stream lines = %q", lines)
	}
	callbackErr := errors.New("stop callback")
	if err := client.StreamOutput(context.Background(), "stream", func(string) error { return callbackErr }); !errors.Is(err, callbackErr) {
		t.Fatalf("stream callback error = %v", err)
	}
	if err := client.StreamOutput(context.Background(), "stream-exit", func(string) error { return nil }); err == nil {
		t.Fatal("expected stream exit error")
	}
	ctx, cancel = context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	if err := client.StreamOutput(ctx, "hang", func(string) error { return nil }); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("canceled stream error = %v", err)
	}

	scriptResult, err := client.RunScript(context.Background(), "#!/bin/bash\necho script ran\n")
	if err != nil || scriptResult.Stdout != "script ran\n" {
		t.Fatalf("run script = %+v, %v", scriptResult, err)
	}
}

type coverageFakeConnection struct {
	session    sshSession
	newErr     error
	closeErr   error
	sendOK     bool
	sendBody   []byte
	sendErr    error
	closeCalls int
}

func (c *coverageFakeConnection) Close() error {
	c.closeCalls++
	return c.closeErr
}

func (c *coverageFakeConnection) NewSession() (sshSession, error) {
	return c.session, c.newErr
}

func (c *coverageFakeConnection) SendRequest(string, bool, []byte) (bool, []byte, error) {
	return c.sendOK, c.sendBody, c.sendErr
}

type coverageFakeSession struct {
	stdin       io.WriteCloser
	stdout      io.Reader
	stderr      io.Reader
	stdinErr    error
	stdoutErr   error
	stderrErr   error
	startErr    error
	runErr      error
	waitErr     error
	closeErr    error
	signalErr   error
	runBlock    chan struct{}
	waitBlock   chan struct{}
	waitStarted chan struct{}
	stdoutDest  io.Writer
	stderrDest  io.Writer
	stdoutWrite string
	stderrWrite string
	signaled    bool
	closed      bool
}

func (s *coverageFakeSession) Close() error {
	s.closed = true
	if s.waitBlock != nil {
		select {
		case <-s.waitBlock:
		default:
			close(s.waitBlock)
		}
	}
	return s.closeErr
}

func (s *coverageFakeSession) Run(string) error {
	if s.runBlock != nil {
		<-s.runBlock
	}
	if s.stdoutDest != nil {
		_, _ = io.WriteString(s.stdoutDest, s.stdoutWrite)
	}
	if s.stderrDest != nil {
		_, _ = io.WriteString(s.stderrDest, s.stderrWrite)
	}
	return s.runErr
}

func (s *coverageFakeSession) Signal(ssh.Signal) error {
	s.signaled = true
	if s.runBlock != nil {
		select {
		case <-s.runBlock:
		default:
			close(s.runBlock)
		}
	}
	return s.signalErr
}

func (s *coverageFakeSession) StdinPipe() (io.WriteCloser, error) { return s.stdin, s.stdinErr }
func (s *coverageFakeSession) StdoutPipe() (io.Reader, error)     { return s.stdout, s.stdoutErr }
func (s *coverageFakeSession) StderrPipe() (io.Reader, error)     { return s.stderr, s.stderrErr }
func (s *coverageFakeSession) Start(string) error                 { return s.startErr }
func (s *coverageFakeSession) Wait() error {
	if s.waitStarted != nil {
		close(s.waitStarted)
	}
	if s.waitBlock != nil {
		<-s.waitBlock
	}
	return s.waitErr
}
func (s *coverageFakeSession) SetStdout(writer io.Writer) { s.stdoutDest = writer }
func (s *coverageFakeSession) SetStderr(writer io.Writer) { s.stderrDest = writer }

func clientWithCoverageFake(session sshSession) (*SSHClient, *coverageFakeConnection) {
	connection := &coverageFakeConnection{session: session}
	return &SSHClient{conn: connection, deps: defaultSSHClientDependencies()}, connection
}

type coverageErrorReader struct{ err error }

func (r coverageErrorReader) Read([]byte) (int, error) { return 0, r.err }

func TestSSHClient100StreamOutputPreservesTerminalDataAndReadErrors(t *testing.T) {
	t.Run("unterminated final line", func(t *testing.T) {
		session := &coverageFakeSession{
			stdout: strings.NewReader("final line"),
			stderr: strings.NewReader(""),
		}
		client, _ := clientWithCoverageFake(session)
		var lines []string
		if err := client.StreamOutput(context.Background(), "command", func(line string) error {
			lines = append(lines, line)
			return nil
		}); err != nil {
			t.Fatalf("StreamOutput() error = %v", err)
		}
		if len(lines) != 1 || lines[0] != "final line" {
			t.Fatalf("stream lines = %q", lines)
		}
	})

	t.Run("read error wins over clean peer EOF", func(t *testing.T) {
		readErr := errors.New("stream read failed")
		for i := 0; i < 20; i++ {
			session := &coverageFakeSession{
				stdout: coverageErrorReader{err: readErr},
				stderr: strings.NewReader(""),
			}
			client, _ := clientWithCoverageFake(session)
			if err := client.StreamOutput(context.Background(), "command", func(string) error { return nil }); !errors.Is(err, readErr) {
				t.Fatalf("iteration %d: StreamOutput() error = %v", i, err)
			}
		}
	})
}

func TestSSHClient100StreamOutputReadsStderrWhileStdoutIsOpen(t *testing.T) {
	stdoutReader, stdoutWriter := io.Pipe()
	stderrReader, stderrWriter := io.Pipe()
	t.Cleanup(func() {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
		_ = stderrReader.Close()
		_ = stderrWriter.Close()
	})

	session := &coverageFakeSession{stdout: stdoutReader, stderr: stderrReader}
	client, _ := clientWithCoverageFake(session)
	lines := make(chan string, 1)
	result := make(chan error, 1)
	go func() {
		result <- client.StreamOutput(context.Background(), "command", func(line string) error {
			lines <- line
			return nil
		})
	}()

	written := make(chan error, 1)
	go func() {
		_, err := io.WriteString(stderrWriter, "live stderr\n")
		written <- err
	}()
	select {
	case err := <-written:
		if err != nil {
			t.Fatalf("write stderr: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("stderr was not consumed while stdout remained open")
	}
	select {
	case line := <-lines:
		if line != "live stderr" {
			t.Fatalf("stream line = %q", line)
		}
	case <-time.After(time.Second):
		t.Fatal("live stderr did not reach the callback")
	}

	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("StreamOutput() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("StreamOutput did not finish after both streams closed")
	}
}

func TestSSHClient100StreamOutputCancelsBlockedSessionWait(t *testing.T) {
	waitBlock := make(chan struct{})
	waitStarted := make(chan struct{})
	session := &coverageFakeSession{
		stdout:      strings.NewReader(""),
		stderr:      strings.NewReader(""),
		waitBlock:   waitBlock,
		waitStarted: waitStarted,
	}
	client, _ := clientWithCoverageFake(session)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		result <- client.StreamOutput(ctx, "command", func(string) error { return nil })
	}()

	select {
	case <-waitStarted:
	case <-time.After(time.Second):
		t.Fatal("StreamOutput did not begin waiting for session completion")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("StreamOutput() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("context cancellation did not interrupt session wait")
	}
	select {
	case <-waitBlock:
	default:
		t.Fatal("session close did not release the blocked wait")
	}
	if !session.signaled || !session.closed {
		t.Fatalf("session cleanup = signaled:%v closed:%v", session.signaled, session.closed)
	}
}

func TestSSHClient100DeterministicFailurePaths(t *testing.T) {
	dialErr := errors.New("dial failed")
	connectClient := &SSHClient{deps: defaultSSHClientDependencies(), host: "host", port: 22, config: &ssh.ClientConfig{}}
	connectClient.deps.dialSSH = func(string, string, *ssh.ClientConfig) (sshConnection, error) {
		return nil, dialErr
	}
	if err := connectClient.Connect(); err == nil || !strings.Contains(err.Error(), "failed to connect") {
		t.Fatalf("connect failure = %v", err)
	}

	client := &SSHClient{host: "host", port: 22, config: &ssh.ClientConfig{}, deps: defaultSSHClientDependencies()}
	client.deps.dialSSH = func(string, string, *ssh.ClientConfig) (sshConnection, error) { return nil, dialErr }
	if _, err := client.NewSession(); !errors.Is(err, dialErr) {
		t.Fatalf("NewSession connect error = %v", err)
	}
	if _, err := client.SendRequest("request", true, nil); !errors.Is(err, dialErr) {
		t.Fatalf("SendRequest connect error = %v", err)
	}
	if _, err := client.Run(context.Background(), "command"); !errors.Is(err, dialErr) {
		t.Fatalf("Run connect error = %v", err)
	}
	if _, err := client.RunScript(context.Background(), "script"); !errors.Is(err, dialErr) {
		t.Fatalf("RunScript connect error = %v", err)
	}
	if err := client.RunWithOutput(context.Background(), "command", io.Discard); !errors.Is(err, dialErr) {
		t.Fatalf("RunWithOutput connect error = %v", err)
	}
	if err := client.Upload(context.Background(), nil, "/tmp/file", 0o600); !errors.Is(err, dialErr) {
		t.Fatalf("Upload connect error = %v", err)
	}
	if err := client.StreamOutput(context.Background(), "command", func(string) error { return nil }); !errors.Is(err, dialErr) {
		t.Fatalf("StreamOutput connect error = %v", err)
	}

	newSessionErr := errors.New("new session failed")
	for name, run := range map[string]func(*SSHClient) error{
		"run":        func(client *SSHClient) error { _, err := client.Run(context.Background(), "x"); return err },
		"run output": func(client *SSHClient) error { return client.RunWithOutput(context.Background(), "x", io.Discard) },
		"upload":     func(client *SSHClient) error { return client.Upload(context.Background(), nil, "/tmp/x", 0o600) },
		"stream": func(client *SSHClient) error {
			return client.StreamOutput(context.Background(), "x", func(string) error { return nil })
		},
	} {
		t.Run(name+" session creation", func(t *testing.T) {
			fakeConn := &coverageFakeConnection{newErr: newSessionErr}
			if err := run(&SSHClient{conn: fakeConn, deps: defaultSSHClientDependencies()}); !errors.Is(err, newSessionErr) {
				t.Fatalf("session creation error = %v", err)
			}
		})
	}

	fakeConn := &coverageFakeConnection{session: &coverageFakeSession{}}
	publicClient := &SSHClient{conn: fakeConn, deps: defaultSSHClientDependencies()}
	if _, err := publicClient.NewSession(); err == nil || !strings.Contains(err.Error(), "unexpected SSH session implementation") {
		t.Fatalf("unexpected public session implementation error = %v", err)
	}
	fakeConn.sendErr = errors.New("send failed")
	if _, err := publicClient.SendRequest("request", true, nil); !errors.Is(err, fakeConn.sendErr) {
		t.Fatalf("send request failure = %v", err)
	}
	fakeConn.closeErr = errors.New("close failed")
	if err := publicClient.Close(); !errors.Is(err, fakeConn.closeErr) {
		t.Fatalf("close failure = %v", err)
	}

	transportErr := errors.New("transport failed")
	fakeSession := &coverageFakeSession{runErr: transportErr, stdoutWrite: "partial"}
	client, _ = clientWithCoverageFake(fakeSession)
	result, err := client.Run(context.Background(), "x")
	if !errors.Is(err, transportErr) || result.Stdout != "partial" {
		t.Fatalf("non-exit Run result = %+v, %v", result, err)
	}

	for name, session := range map[string]*coverageFakeSession{
		"stdin":  {stdinErr: errors.New("stdin failed")},
		"stdout": {stdin: &closeBuffer{}, stdoutErr: errors.New("stdout failed")},
		"start":  {stdin: &closeBuffer{}, stdout: bytes.NewReader(nil), startErr: errors.New("start failed")},
	} {
		t.Run("upload "+name+" failure", func(t *testing.T) {
			client, _ := clientWithCoverageFake(session)
			if err := client.Upload(context.Background(), nil, "/tmp/x", 0o600); err == nil {
				t.Fatal("expected upload setup error")
			}
		})
	}

	for name, session := range map[string]*coverageFakeSession{
		"stdout": {stdoutErr: errors.New("stdout failed")},
		"stderr": {stdout: bytes.NewReader(nil), stderrErr: errors.New("stderr failed")},
		"start":  {stdout: bytes.NewReader(nil), stderr: bytes.NewReader(nil), startErr: errors.New("start failed")},
		"read":   {stdout: coverageErrorReader{err: errors.New("read failed")}, stderr: bytes.NewReader(nil)},
	} {
		t.Run("stream "+name+" failure", func(t *testing.T) {
			client, _ := clientWithCoverageFake(session)
			if err := client.StreamOutput(context.Background(), "x", func(string) error { return nil }); err == nil {
				t.Fatal("expected stream setup/read error")
			}
		})
	}
	if err := publicClient.StreamOutput(context.Background(), "x", nil); err == nil || err.Error() != "stream callback is required" {
		t.Fatalf("nil callback error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancelSession := &coverageFakeSession{runBlock: make(chan struct{})}
	client, _ = clientWithCoverageFake(cancelSession)
	cancel()
	if _, err := client.Run(ctx, "x"); !errors.Is(err, context.Canceled) || !cancelSession.signaled {
		t.Fatalf("fake canceled Run = %v, signaled=%v", err, cancelSession.signaled)
	}

	ctx, cancel = context.WithCancel(context.Background())
	cancelSession = &coverageFakeSession{runBlock: make(chan struct{})}
	client, _ = clientWithCoverageFake(cancelSession)
	cancel()
	if err := client.RunWithOutput(ctx, "x", io.Discard); !errors.Is(err, context.Canceled) || !cancelSession.signaled {
		t.Fatalf("fake canceled RunWithOutput = %v, signaled=%v", err, cancelSession.signaled)
	}
}

type coverageFailWriter struct {
	writes    int
	failWrite int
	closeErr  error
}

func (w *coverageFailWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes == w.failWrite {
		return 0, errors.New("write failed")
	}
	return len(p), nil
}

func (w *coverageFailWriter) Close() error { return w.closeErr }

func TestSSHClient100SCPProtocolFailures(t *testing.T) {
	tests := []struct {
		name   string
		stdin  io.WriteCloser
		acks   []byte
		wait   func() error
		needle string
	}{
		{name: "initial acknowledgement", stdin: &coverageFailWriter{}, acks: nil, wait: func() error { return nil }, needle: "read SCP acknowledgement"},
		{name: "header write", stdin: &coverageFailWriter{failWrite: 1}, acks: []byte{0}, wait: func() error { return nil }, needle: "write SCP header"},
		{name: "header acknowledgement", stdin: &coverageFailWriter{}, acks: []byte{0, 1, 'n', 'o', '\n'}, wait: func() error { return nil }, needle: "SCP rejected upload"},
		{name: "content write", stdin: &coverageFailWriter{failWrite: 2}, acks: []byte{0, 0}, wait: func() error { return nil }, needle: "write SCP content"},
		{name: "content terminator", stdin: &coverageFailWriter{failWrite: 3}, acks: []byte{0, 0}, wait: func() error { return nil }, needle: "finish SCP content"},
		{name: "content acknowledgement", stdin: &coverageFailWriter{}, acks: []byte{0, 0, 2, 'b', 'a', 'd', '\n'}, wait: func() error { return nil }, needle: "SCP rejected upload"},
		{name: "stdin close", stdin: &coverageFailWriter{closeErr: errors.New("close failed")}, acks: []byte{0, 0, 0}, wait: func() error { return nil }, needle: "close SCP stdin"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := writeSCPFile(tc.stdin, bytes.NewReader(tc.acks), []byte("payload"), "file", 0o640, tc.wait)
			if err == nil || !strings.Contains(err.Error(), tc.needle) {
				t.Fatalf("writeSCPFile error = %v, want %q", err, tc.needle)
			}
		})
	}
}

type coverageRetryTimer struct {
	channel chan time.Time
	stop    bool
}

func (t *coverageRetryTimer) C() <-chan time.Time { return t.channel }
func (t *coverageRetryTimer) Stop() bool {
	return t.stop
}

func TestSSHClient100WaitForConnectionPaths(t *testing.T) {
	server := startCoverageSSHServer(t)
	client := newCoveragePasswordClient(t, server)
	if err := client.WaitForConnection(context.Background(), 1); err != nil {
		t.Fatalf("wait for available connection: %v", err)
	}
	if client.conn == nil {
		t.Fatal("WaitForConnection should establish SSH connection")
	}

	client = &SSHClient{deps: defaultSSHClientDependencies()}
	if err := client.WaitForConnection(context.Background(), 0); err == nil || err.Error() != "failed to connect after 0 retries" {
		t.Fatalf("zero retries error = %v", err)
	}

	client = &SSHClient{host: "host", port: 22, deps: defaultSSHClientDependencies()}
	dialErr := errors.New("not ready")
	client.deps.dialTCP = func(string, string, time.Duration) (net.Conn, error) { return nil, dialErr }
	timer := &coverageRetryTimer{channel: make(chan time.Time, 1)}
	timer.channel <- time.Now()
	client.deps.newTimer = func(time.Duration) sshRetryTimer { return timer }
	if err := client.WaitForConnection(context.Background(), 1); err == nil || !errors.Is(err, dialErr) {
		t.Fatalf("exhausted retries error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	timer = &coverageRetryTimer{channel: make(chan time.Time, 1), stop: false}
	client.deps.dialTCP = func(string, string, time.Duration) (net.Conn, error) {
		cancel()
		return nil, dialErr
	}
	client.deps.newTimer = func(time.Duration) sshRetryTimer { return timer }
	waitResult := make(chan error, 1)
	go func() { waitResult <- client.WaitForConnection(ctx, 1) }()
	select {
	case err := <-waitResult:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled retry error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled retry blocked while stopping its timer")
	}

	ctx, cancel = context.WithCancel(context.Background())
	cancel()
	if err := client.WaitForConnection(ctx, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-canceled retry error = %v", err)
	}

	ctx, cancel = context.WithCancel(context.Background())
	timer = &coverageRetryTimer{channel: make(chan time.Time), stop: true}
	client.deps.dialTCP = func(string, string, time.Duration) (net.Conn, error) {
		cancel()
		return nil, dialErr
	}
	client.deps.newTimer = func(time.Duration) sshRetryTimer { return timer }
	if err := client.WaitForConnection(ctx, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled stopped-timer error = %v", err)
	}
}

type coverageServerConnection struct {
	base *Connection
}

func (s *coverageServerConnection) GetID() string           { return "server-id" }
func (s *coverageServerConnection) GetTeamID() string       { return "team-id" }
func (s *coverageServerConnection) GetIPAddress() string    { return s.base.Host }
func (s *coverageServerConnection) GetSSHPort() int         { return s.base.Port }
func (s *coverageServerConnection) GetPrivateKey() string   { return s.base.PrivateKey }
func (s *coverageServerConnection) GetSudoPassword() string { return "" }
func (s *coverageServerConnection) GetUsername() string     { return s.base.User }
func (s *coverageServerConnection) ConnectionAsRoot() *Connection {
	cloned := *s.base
	cloned.User = "root"
	return &cloned
}
func (s *coverageServerConnection) ConnectionAsUser(username ...string) *Connection {
	cloned := *s.base
	if len(username) > 0 {
		cloned.User = username[0]
	}
	return &cloned
}

func TestSSHClient100ConnectionAndServerConstructors(t *testing.T) {
	privateKey := coveragePrivateKey(t)
	connection := &Connection{Host: "example.test", User: "deploy", PrivateKey: privateKey}
	if got := connection.GetScriptPath(); !strings.HasSuffix(got, "/.launch") {
		t.Fatalf("default script path = %q", got)
	}

	client, err := connection.NewSSHClient()
	if err != nil || client.port != 22 || client.timeout != config.SSH {
		t.Fatalf("default connection client = %+v, %v", client, err)
	}
	client, err = connection.NewSSHClient(-time.Second)
	if err != nil || client.timeout != config.SSH {
		t.Fatalf("negative timeout client = %+v, %v", client, err)
	}
	client, err = connection.NewSSHClient(123 * time.Millisecond)
	if err != nil || client.timeout != 123*time.Millisecond {
		t.Fatalf("custom timeout client = %+v, %v", client, err)
	}
	if _, err := (&Connection{PrivateKey: "invalid"}).NewSSHClient(); err == nil {
		t.Fatal("expected connection constructor key error")
	}

	server := startCoverageSSHServer(t)
	host, port := server.address()
	dialConnection := &Connection{Host: host, Port: port, User: "deploy", PrivateKey: privateKey, HostKey: server.hostKey(), ServerID: "server-test"}
	dialed, err := dialConnection.Dial(time.Second)
	if err != nil {
		t.Fatalf("connection dial: %v", err)
	}
	_ = dialed.Close()
	if _, err := (&Connection{PrivateKey: "invalid"}).Dial(); err == nil {
		t.Fatal("expected Dial constructor error")
	}
	closedListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for closed port: %v", err)
	}
	_, portText, _ := net.SplitHostPort(closedListener.Addr().String())
	closedPort, _ := strconv.Atoi(portText)
	_ = closedListener.Close()
	if _, err := (&Connection{Host: "127.0.0.1", Port: closedPort, User: "deploy", PrivateKey: privateKey}).Dial(50 * time.Millisecond); err == nil {
		t.Fatal("expected Dial connection error")
	}

	serverConnection := &coverageServerConnection{base: &Connection{Host: "host", Port: 2222, User: "deploy", PrivateKey: privateKey}}
	if _, err := NewSSHClientFromServer(nil); err == nil {
		t.Fatal("expected nil default-server error")
	}
	if _, err := NewSSHClientFromServerAsRoot(nil); err == nil {
		t.Fatal("expected nil root-server error")
	}
	if _, err := NewSSHClientFromServerAsUser(nil, "alice"); err == nil {
		t.Fatal("expected nil named-user server error")
	}
	client, err = NewSSHClientFromServer(serverConnection, WithSSHTimeout(time.Second))
	if err != nil || client.config.User != "deploy" {
		t.Fatalf("default-user server client = %+v, %v", client, err)
	}
	client, err = NewSSHClientFromServerAsRoot(serverConnection, WithSSHPort(2200))
	if err != nil || client.config.User != "root" || client.port != 2200 {
		t.Fatalf("root server client = %+v, %v", client, err)
	}
	client, err = NewSSHClientFromServerAsUser(serverConnection, "alice")
	if err != nil || client.config.User != "alice" {
		t.Fatalf("named-user server client = %+v, %v", client, err)
	}
}

func TestSSHClient100RunScriptAndRemoteHelperErrors(t *testing.T) {
	dialErr := errors.New("unavailable")
	client := &SSHClient{conn: &coverageFakeConnection{newErr: dialErr}, deps: defaultSSHClientDependencies()}
	if _, err := client.RunScript(context.Background(), "script"); err == nil || !strings.Contains(err.Error(), "upload script") {
		t.Fatalf("RunScript upload error = %v", err)
	}
	if _, err := client.Download(context.Background(), "/tmp/file"); !errors.Is(err, dialErr) {
		t.Fatalf("Download error = %v", err)
	}
	if _, err := client.FileExists(context.Background(), "/tmp/file"); err == nil || !strings.Contains(err.Error(), "check remote file") {
		t.Fatalf("FileExists error = %v", err)
	}
	if _, err := client.DirExists(context.Background(), "/tmp/dir"); err == nil || !strings.Contains(err.Error(), "check remote directory") {
		t.Fatalf("DirExists error = %v", err)
	}
	if err := client.MkdirAll(context.Background(), "/tmp/dir"); !errors.Is(err, dialErr) {
		t.Fatalf("MkdirAll error = %v", err)
	}
}

func TestSSHClient100UploadCancellation(t *testing.T) {
	server := startCoverageSSHServer(t)
	client := newCoveragePasswordClient(t, server)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	if err := client.Upload(ctx, []byte("payload"), "/upload-hang/file", 0o600); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("canceled upload error = %v", err)
	}
}

func TestSSHClient100AutoConnectAndAbruptErrors(t *testing.T) {
	server := startCoverageSSHServer(t)
	client := newCoveragePasswordClient(t, server)
	if _, err := client.Run(context.Background(), "success"); err != nil {
		t.Fatalf("auto-connected Run: %v", err)
	}
	_ = client.Close()

	client = newCoveragePasswordClient(t, server)
	if _, err := client.SendRequest("keepalive-ok", true, nil); err != nil {
		t.Fatalf("auto-connected SendRequest: %v", err)
	}
	_ = client.Close()

	client = newCoveragePasswordClient(t, server)
	if _, err := client.NewSession(); err != nil {
		t.Fatalf("auto-connected NewSession: %v", err)
	}
	_ = client.Close()

	client = newCoveragePasswordClient(t, server)
	if err := client.RunWithOutput(context.Background(), "success", io.Discard); err != nil {
		t.Fatalf("auto-connected RunWithOutput: %v", err)
	}
	_ = client.Close()

	client = newCoveragePasswordClient(t, server)
	if err := client.Upload(context.Background(), []byte("x"), "/tmp/auto", 0o600); err != nil {
		t.Fatalf("auto-connected Upload: %v", err)
	}
	_ = client.Close()

	client = newCoveragePasswordClient(t, server)
	if err := client.StreamOutput(context.Background(), "stream", func(string) error { return nil }); err != nil {
		t.Fatalf("auto-connected StreamOutput: %v", err)
	}
	_ = client.Close()

	client = newCoveragePasswordClient(t, server)
	if _, err := client.RunScript(context.Background(), "echo x"); err != nil {
		t.Fatalf("auto-connected RunScript: %v", err)
	}
	_ = client.Close()

	client = newCoveragePasswordClient(t, server)
	result, err := client.Run(context.Background(), "abrupt-disconnect")
	if err == nil || result == nil {
		t.Fatalf("abrupt Run result = %+v, %v", result, err)
	}

	client = newCoveragePasswordClient(t, server)
	if err := client.Connect(); err != nil {
		t.Fatalf("connect before closed-session test: %v", err)
	}
	if err := client.conn.Close(); err != nil {
		t.Fatalf("close connection before NewSession: %v", err)
	}
	if _, err := client.NewSession(); err == nil {
		t.Fatal("expected NewSession to propagate the closed transport error")
	}
}

func TestSSHClient100SendStreamEvent(t *testing.T) {
	events := make(chan streamReadEvent, 1)
	event := streamReadEvent{line: "line"}
	if !sendStreamEvent(context.Background(), events, event) {
		t.Fatal("expected stream event delivery")
	}
	if got := <-events; got != event {
		t.Fatalf("stream event = %#v", got)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if sendStreamEvent(ctx, make(chan streamReadEvent), streamReadEvent{line: "ignored"}) {
		t.Fatal("canceled stream event should not be delivered")
	}

	readEvents := make(chan streamReadEvent)
	readStreamLines(ctx, newLineReader(strings.NewReader("ignored\n")), readEvents)
	if len(readEvents) != 0 {
		t.Fatal("canceled stream reader should not emit events")
	}
}

func TestSSHClient100RealRetryTimerAdapter(t *testing.T) {
	timer := (&SSHClient{deps: defaultSSHClientDependencies()}).deps.newTimer(time.Hour)
	if timer.C() == nil {
		t.Fatal("real retry timer channel is nil")
	}
	if !timer.Stop() {
		t.Fatal("fresh real retry timer should stop")
	}
}

func TestSSHClient100FormattingHelpers(t *testing.T) {
	if got := encodeBase64(nil); got != "" {
		t.Fatalf("empty base64 = %q", got)
	}
	if got := unquoteCoverageShellArg("'it'\"'\"'s'"); got != "it's" {
		t.Fatalf("test shell unquote = %q", got)
	}
	reader := newLineReader(strings.NewReader("line\r\n"))
	line, err := reader.readLine()
	if err != nil || line != "line\r" {
		t.Fatalf("CRLF read = %q, %v", line, err)
	}
	if got := ShellQuote(""); got != "''" {
		t.Fatalf("empty shell quote = %q", got)
	}
}
