package handlers

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	serverModels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

func TestBuildTailCommandDoesNotWaitForExistingLogs(t *testing.T) {
	command := buildTailCommand("/var/log/app.log", 100, false)

	if command != "tail -n 100 -f '/var/log/app.log' 2>&1" {
		t.Fatalf("unexpected command: %s", command)
	}
}

func TestBuildTailCommandWaitsForTaskLog(t *testing.T) {
	path := "/home/ubuntu/.launch/task-it's-safe.log"
	command := buildTailCommand(path, 250, true)

	quotedPath := shellQuote(path)
	if count := strings.Count(command, quotedPath); count != 3 {
		t.Fatalf("expected the safely quoted path three times, got %d in %q", count, command)
	}
	if !strings.Contains(command, `while [ "$attempt" -lt 60 ]`) {
		t.Fatalf("expected a bounded wait before tailing: %s", command)
	}
	if !strings.Contains(command, "sleep 0.5") {
		t.Fatalf("expected the wait loop to avoid busy polling: %s", command)
	}
	if !strings.Contains(command, "exit 42") {
		t.Fatalf("expected a distinct missing-task-log exit status: %s", command)
	}
	if !strings.HasSuffix(command, "tail -n 250 -f "+quotedPath+" 2>&1") {
		t.Fatalf("expected the command to tail the task log after waiting: %s", command)
	}
}

func TestBuildLogStreamCommand(t *testing.T) {
	require.Equal(
		t,
		"tail -n 20 -f '/var/log/app.log' 2>&1",
		buildLogStreamCommand("/var/log/app.log", 20, "", false),
	)
	require.Equal(
		t,
		"tail -n 20 -f '/var/log/app.log' 2>&1 | grep --line-buffered -iF 'it'\\''s ready'",
		buildLogStreamCommand("/var/log/app.log", 20, "it's ready", false),
	)
}

func TestDefaultLogStreamDialer(t *testing.T) {
	t.Run("dial error", func(t *testing.T) {
		client, err := defaultLogStreamDialer(&taskrunner.Connection{
			Host:       "127.0.0.1",
			PrivateKey: "invalid",
		})

		require.Nil(t, client)
		require.ErrorContains(t, err, "failed to parse private key")
	})

	t.Run("session", func(t *testing.T) {
		connection, stop := startLogSSHServer(t)
		defer stop()

		client, err := defaultLogStreamDialer(connection)
		require.NoError(t, err)
		defer func() { _ = client.Close() }()
		session, err := client.NewSession()
		require.NoError(t, err)
		require.NoError(t, session.Close())
	})
}

func TestStreamLogsRejectsIncompleteServerConnections(t *testing.T) {
	ip := "127.0.0.1"
	tests := []struct {
		name    string
		server  *serverModels.Server
		message string
	}{
		{
			name:    "missing host",
			server:  &serverModels.Server{},
			message: "Server has no public IP",
		},
		{
			name:    "missing private key",
			server:  &serverModels.Server{PublicIPv4: &ip},
			message: "No SSH key configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := newTestLogsHandler(nil)
			socket := newFakeLogStreamSocket()

			handler.streamLogs(socket, tt.server, "/var/log/app.log", 10, "")

			requireLogStreamError(t, <-socket.messageCh, tt.message)
		})
	}
}

func TestStreamLogsReportsDialAndSessionErrors(t *testing.T) {
	t.Run("dial", func(t *testing.T) {
		handler := newTestLogsHandler(func(*taskrunner.Connection) (logStreamClient, error) {
			return nil, errors.New("dial failed")
		})
		socket := newFakeLogStreamSocket()

		handler.streamLogs(socket, validLogStreamServer(), "/var/log/app.log", 10, "")

		requireLogStreamError(t, <-socket.messageCh, "SSH connection failed: dial failed")
	})

	t.Run("session", func(t *testing.T) {
		client := newFakeLogStreamClient(nil)
		client.newSessionErr = errors.New("session failed")
		handler := newTestLogsHandler(func(*taskrunner.Connection) (logStreamClient, error) {
			return client, nil
		})
		socket := newFakeLogStreamSocket()

		handler.streamLogs(socket, validLogStreamServer(), "/var/log/app.log", 10, "")

		requireLogStreamError(t, <-socket.messageCh, "Failed to create session")
		require.Equal(t, int32(1), client.closeCalls.Load())
	})
}

func TestStreamLogsBuildsImmediateAndWaitingCommands(t *testing.T) {
	tests := []struct {
		name     string
		run      func(*LogsHandler, logStreamSocket, *serverModels.Server)
		contains string
		excludes string
	}{
		{
			name: "standard logs",
			run: func(handler *LogsHandler, socket logStreamSocket, server *serverModels.Server) {
				handler.streamLogs(socket, server, "/var/log/app.log", 15, "ready")
			},
			contains: "grep --line-buffered -iF 'ready'",
			excludes: `while [ "$attempt" -lt 60 ]`,
		},
		{
			name: "task logs",
			run: func(handler *LogsHandler, socket logStreamSocket, server *serverModels.Server) {
				handler.streamTaskLogs(socket, server, "/root/.launch/task-1.log", 15)
			},
			contains: `while [ "$attempt" -lt 60 ]`,
			excludes: "grep --line-buffered",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := newFakeLogStreamSession(strings.NewReader(""))
			client := newFakeLogStreamClient(session)
			handler := newTestLogsHandler(func(*taskrunner.Connection) (logStreamClient, error) {
				return client, nil
			})
			socket := newFakeLogStreamSocket()
			done := make(chan struct{})

			go func() {
				defer close(done)
				tt.run(handler, socket, validLogStreamServer())
			}()

			command := receiveWithTimeout(t, session.startCh)
			require.Contains(t, command, tt.contains)
			require.NotContains(t, command, tt.excludes)
			session.waitCh <- nil
			waitForDone(t, done)
			require.Equal(t, int32(1), session.closeCalls.Load())
			require.Equal(t, int32(1), client.closeCalls.Load())
		})
	}
}

func TestStreamLogSessionSetupErrors(t *testing.T) {
	t.Run("stdout", func(t *testing.T) {
		handler := newTestLogsHandler(nil)
		socket := newFakeLogStreamSocket()
		session := newFakeLogStreamSession(nil)
		session.stdoutErr = errors.New("stdout failed")

		handler.streamLogSession(socket, session, "tail")

		require.Empty(t, socket.messages())
	})

	t.Run("start", func(t *testing.T) {
		handler := newTestLogsHandler(nil)
		socket := newFakeLogStreamSocket()
		session := newFakeLogStreamSession(strings.NewReader(""))
		session.startErr = errors.New("start failed")

		handler.streamLogSession(socket, session, "tail")

		requireLogStreamError(t, <-socket.messageCh, "Failed to start log streaming")
	})
}

func TestStreamLogSessionRelaysOutputAndCompletes(t *testing.T) {
	handler := newTestLogsHandler(nil)
	socket := newFakeLogStreamSocket()
	session := newFakeLogStreamSession(strings.NewReader("first line\n"))
	done := make(chan struct{})

	go func() {
		defer close(done)
		handler.streamLogSession(socket, session, "tail")
	}()

	require.Equal(t, "tail", receiveWithTimeout(t, session.startCh))
	require.Equal(t, "first line\n", string(receiveWithTimeout(t, socket.messageCh)))
	session.waitCh <- nil
	waitForDone(t, done)
	require.True(t, socket.isClosed())
	require.Equal(t, int32(1), session.closeCalls.Load())
}

func TestStreamLogSessionStopsReaderWhenCommandCompletes(t *testing.T) {
	handler := newTestLogsHandler(nil)
	socket := newFakeLogStreamSocket()
	session := newFakeLogStreamSession(nil)
	reader := &zeroAfterCloseLogReader{
		closed:  session.closed,
		entered: make(chan struct{}),
	}
	session.stdout = reader
	done := make(chan struct{})

	go func() {
		defer close(done)
		handler.streamLogSession(socket, session, "tail")
	}()

	receiveWithTimeout(t, session.startCh)
	receiveWithTimeout(t, reader.entered)
	session.waitCh <- nil
	waitForDone(t, done)
	require.True(t, socket.isClosed())
}

func TestStreamLogSessionHandlesWriteFailure(t *testing.T) {
	tests := []struct {
		name        string
		deadlineErr error
		writeErr    error
	}{
		{name: "deadline", deadlineErr: errors.New("deadline failed")},
		{name: "write", writeErr: errors.New("write failed")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := newTestLogsHandler(nil)
			socket := newFakeLogStreamSocket()
			socket.deadlineErr = tt.deadlineErr
			socket.writeErr = tt.writeErr
			session := newFakeLogStreamSession(bytes.NewBufferString("output"))
			done := make(chan struct{})

			go func() {
				defer close(done)
				handler.streamLogSession(socket, session, "tail")
			}()

			receiveWithTimeout(t, session.startCh)
			if tt.deadlineErr == nil {
				receiveWithTimeout(t, socket.writeAttemptCh)
			}
			session.waitCh <- nil
			waitForDone(t, done)
		})
	}
}

func TestWriteLogStreamMessageRejectsDeadlineFailure(t *testing.T) {
	handler := newTestLogsHandler(nil)
	socket := newFakeLogStreamSocket()
	expected := errors.New("deadline failed")
	socket.deadlineErr = expected

	err := handler.writeLogStreamMessage(socket, []byte("output"))

	require.ErrorIs(t, err, expected)
	require.Empty(t, socket.messages())
}

func TestStreamLogSessionBoundsBlockedWrite(t *testing.T) {
	handler := newTestLogsHandler(nil)
	handler.logStreamWriteTimeout = 20 * time.Millisecond
	handler.logStreamShutdownTimeout = 100 * time.Millisecond
	socket := newFakeLogStreamSocket()
	socket.blockWrites = true
	session := newFakeLogStreamSession(bytes.NewBufferString("output"))
	done := make(chan struct{})

	go func() {
		defer close(done)
		handler.streamLogSession(socket, session, "tail")
	}()

	receiveWithTimeout(t, session.startCh)
	receiveWithTimeout(t, socket.writeAttemptCh)
	session.waitCh <- nil
	waitForDone(t, done)
	require.True(t, socket.isClosed())
}

func TestStreamLogSessionCancelsDelayedReader(t *testing.T) {
	handler := newTestLogsHandler(nil)
	handler.logStreamShutdownTimeout = 20 * time.Millisecond
	socket := newFakeLogStreamSocket()
	reader := &socketCloseLogReader{
		closed:  socket.closed,
		entered: make(chan struct{}),
	}
	session := newFakeLogStreamSession(reader)
	done := make(chan struct{})

	go func() {
		defer close(done)
		handler.streamLogSession(socket, session, "tail")
	}()

	receiveWithTimeout(t, session.startCh)
	receiveWithTimeout(t, reader.entered)
	session.waitCh <- nil
	waitForDone(t, done)
	require.True(t, socket.isClosed())
}

func TestStreamLogSessionReportsCommandFailures(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		message string
	}{
		{name: "unexpected", err: errors.New("command failed"), message: "Log stream ended unexpectedly"},
		{name: "missing task log", err: fakeExitStatusError(42), message: "Task log file is not available on the server"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := newTestLogsHandler(nil)
			socket := newFakeLogStreamSocket()
			session := newFakeLogStreamSession(strings.NewReader(""))
			done := make(chan struct{})

			go func() {
				defer close(done)
				handler.streamLogSession(socket, session, "tail")
			}()

			receiveWithTimeout(t, session.startCh)
			session.waitCh <- tt.err
			requireLogStreamError(t, receiveWithTimeout(t, socket.messageCh), tt.message)
			waitForDone(t, done)
		})
	}
}

func TestStreamLogSessionTerminatesOnClientDisconnect(t *testing.T) {
	handler := newTestLogsHandler(nil)
	socket := newFakeLogStreamSocket()
	session := newFakeLogStreamSession(strings.NewReader(""))
	done := make(chan struct{})

	go func() {
		defer close(done)
		handler.streamLogSession(socket, session, "tail")
	}()

	receiveWithTimeout(t, session.startCh)
	socket.readCh <- nil
	socket.readCh <- io.EOF
	waitForDone(t, done)
	require.Equal(t, ssh.SIGTERM, receiveWithTimeout(t, session.signalCh))
}

func TestStreamLogSessionCancelsDelayedReaderOnDisconnect(t *testing.T) {
	handler := newTestLogsHandler(nil)
	handler.logStreamShutdownTimeout = 20 * time.Millisecond
	socket := newFakeLogStreamSocket()
	reader := &socketCloseLogReader{
		closed:  socket.closed,
		entered: make(chan struct{}),
	}
	session := newFakeLogStreamSession(reader)
	done := make(chan struct{})

	go func() {
		defer close(done)
		handler.streamLogSession(socket, session, "tail")
	}()

	receiveWithTimeout(t, session.startCh)
	receiveWithTimeout(t, reader.entered)
	socket.readCh <- io.EOF
	waitForDone(t, done)
	require.True(t, socket.isClosed())
	require.Equal(t, ssh.SIGTERM, receiveWithTimeout(t, session.signalCh))
}

func TestStreamTaskOutput(t *testing.T) {
	t.Run("missing task", func(t *testing.T) {
		handler := newTestLogsHandler(nil)
		handler.DB = newTaskLogDB(t)
		socket := newFakeLogStreamSocket()

		handler.streamTaskOutput(socket, "missing", "server-1", 20)

		requireLogStreamError(t, <-socket.messageCh, "Task not found")
		require.True(t, socket.isClosed())
	})

	t.Run("finished task", func(t *testing.T) {
		db := newTaskLogDB(t)
		insertTaskLogRows(t, db, "finished", "stored output")
		handler := newTestLogsHandler(nil)
		handler.DB = db
		socket := newFakeLogStreamSocket()

		handler.streamTaskOutput(socket, "task-1", "server-1", 20)

		require.Equal(t, "stored output", string(<-socket.messageCh))
		require.False(t, socket.currentWriteDeadline().IsZero())
		require.True(t, socket.isClosed())
	})

	t.Run("running task", func(t *testing.T) {
		db := newTaskLogDB(t)
		insertTaskLogRows(t, db, "running", "")
		session := newFakeLogStreamSession(strings.NewReader(""))
		client := newFakeLogStreamClient(session)
		handler := newTestLogsHandler(func(*taskrunner.Connection) (logStreamClient, error) {
			return client, nil
		})
		handler.DB = db
		socket := newFakeLogStreamSocket()
		done := make(chan struct{})

		go func() {
			defer close(done)
			handler.streamTaskOutput(socket, "task-1", "server-1", 20)
		}()

		command := receiveWithTimeout(t, session.startCh)
		require.Contains(t, command, "/root/.launch/task-task-1.log")
		require.Contains(t, command, `while [ "$attempt" -lt 60 ]`)
		session.waitCh <- nil
		waitForDone(t, done)
	})
}

type fakeExitStatusError int

func (e fakeExitStatusError) Error() string {
	return "exit status"
}

func (e fakeExitStatusError) ExitStatus() int {
	return int(e)
}

type zeroAfterCloseLogReader struct {
	closed    <-chan struct{}
	entered   chan struct{}
	enterOnce sync.Once
}

func (r *zeroAfterCloseLogReader) Read([]byte) (int, error) {
	r.enterOnce.Do(func() { close(r.entered) })
	<-r.closed
	return 0, nil
}

type socketCloseLogReader struct {
	closed    <-chan struct{}
	entered   chan struct{}
	enterOnce sync.Once
}

func (r *socketCloseLogReader) Read([]byte) (int, error) {
	r.enterOnce.Do(func() { close(r.entered) })
	<-r.closed
	return 0, io.EOF
}

type fakeLogStreamSocket struct {
	mu             sync.Mutex
	writes         [][]byte
	messageCh      chan []byte
	writeAttemptCh chan []byte
	readCh         chan error
	closed         chan struct{}
	closeOnce      sync.Once
	writeErr       error
	deadlineErr    error
	deadlineMu     sync.Mutex
	writeDeadline  time.Time
	blockWrites    bool
}

func newFakeLogStreamSocket() *fakeLogStreamSocket {
	return &fakeLogStreamSocket{
		messageCh:      make(chan []byte, 8),
		writeAttemptCh: make(chan []byte, 8),
		readCh:         make(chan error, 8),
		closed:         make(chan struct{}),
	}
}

func (s *fakeLogStreamSocket) WriteMessage(_ int, payload []byte) error {
	copyOfPayload := append([]byte(nil), payload...)
	s.writeAttemptCh <- copyOfPayload
	if s.writeErr != nil {
		return s.writeErr
	}
	if s.blockWrites {
		s.deadlineMu.Lock()
		deadline := s.writeDeadline
		s.deadlineMu.Unlock()
		timer := time.NewTimer(time.Until(deadline))
		defer timer.Stop()
		select {
		case <-timer.C:
			return errors.New("write deadline exceeded")
		case <-s.closed:
			return io.EOF
		}
	}
	s.mu.Lock()
	s.writes = append(s.writes, copyOfPayload)
	s.mu.Unlock()
	s.messageCh <- copyOfPayload
	return nil
}

func (s *fakeLogStreamSocket) SetWriteDeadline(deadline time.Time) error {
	if s.deadlineErr != nil {
		return s.deadlineErr
	}
	s.deadlineMu.Lock()
	s.writeDeadline = deadline
	s.deadlineMu.Unlock()
	return nil
}

func (s *fakeLogStreamSocket) ReadMessage() (int, []byte, error) {
	select {
	case err := <-s.readCh:
		return 0, nil, err
	case <-s.closed:
		return 0, nil, io.EOF
	}
}

func (s *fakeLogStreamSocket) Close() error {
	s.closeOnce.Do(func() { close(s.closed) })
	return nil
}

func (s *fakeLogStreamSocket) messages() [][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([][]byte(nil), s.writes...)
}

func (s *fakeLogStreamSocket) currentWriteDeadline() time.Time {
	s.deadlineMu.Lock()
	defer s.deadlineMu.Unlock()
	return s.writeDeadline
}

func (s *fakeLogStreamSocket) isClosed() bool {
	select {
	case <-s.closed:
		return true
	default:
		return false
	}
}

type fakeLogStreamSession struct {
	stdout     io.Reader
	stdoutErr  error
	startErr   error
	waitCh     chan error
	startCh    chan string
	signalCh   chan ssh.Signal
	closed     chan struct{}
	closeOnce  sync.Once
	closeCalls atomic.Int32
}

func newFakeLogStreamSession(stdout io.Reader) *fakeLogStreamSession {
	return &fakeLogStreamSession{
		stdout:   stdout,
		waitCh:   make(chan error, 1),
		startCh:  make(chan string, 1),
		signalCh: make(chan ssh.Signal, 1),
		closed:   make(chan struct{}),
	}
}

func (s *fakeLogStreamSession) StdoutPipe() (io.Reader, error) {
	return s.stdout, s.stdoutErr
}

func (s *fakeLogStreamSession) Start(command string) error {
	if s.startErr != nil {
		return s.startErr
	}
	s.startCh <- command
	return nil
}

func (s *fakeLogStreamSession) Wait() error {
	select {
	case err := <-s.waitCh:
		return err
	case <-s.closed:
		return io.EOF
	}
}

func (s *fakeLogStreamSession) Signal(signal ssh.Signal) error {
	s.signalCh <- signal
	return nil
}

func (s *fakeLogStreamSession) Close() error {
	s.closeCalls.Add(1)
	s.closeOnce.Do(func() { close(s.closed) })
	return nil
}

type fakeLogStreamClient struct {
	session       logStreamSession
	newSessionErr error
	closeCalls    atomic.Int32
}

func newFakeLogStreamClient(session logStreamSession) *fakeLogStreamClient {
	return &fakeLogStreamClient{session: session}
}

func (c *fakeLogStreamClient) NewSession() (logStreamSession, error) {
	return c.session, c.newSessionErr
}

func (c *fakeLogStreamClient) Close() error {
	c.closeCalls.Add(1)
	return nil
}

func newTestLogsHandler(dialer logStreamDialer) *LogsHandler {
	handler := NewLogsHandler(NewBase(nil, "", zerolog.Nop(), nil))
	if dialer != nil {
		handler.dialLogStream = dialer
	}
	return handler
}

func validLogStreamServer() *serverModels.Server {
	ip := "127.0.0.1"
	return &serverModels.Server{
		PublicIPv4: &ip,
		PrivateKey: dbtype.EncryptedString("private-key"),
	}
}

func requireLogStreamError(t *testing.T, payload []byte, message string) {
	t.Helper()
	require.JSONEq(t, `{"event":"error","data":{"message":`+string(mustJSON(t, message))+`}}`, string(payload))
}

func mustJSON(t *testing.T, value string) []byte {
	t.Helper()
	payload, err := json.Marshal(value)
	require.NoError(t, err)
	return payload
}

func receiveWithTimeout[T any](t *testing.T, values <-chan T) T {
	t.Helper()
	select {
	case value := <-values:
		return value
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for value")
		var zero T
		return zero
	}
}

func waitForDone(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for completion")
	}
}

func newTaskLogDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "-") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	require.NoError(t, db.Exec(`
		CREATE TABLE servers (
			id TEXT PRIMARY KEY,
			public_ipv4 TEXT,
			private_key TEXT,
			provider TEXT,
			working_directory TEXT,
			ssh_port INTEGER,
			operating_system TEXT,
			host_key TEXT,
			deleted_at DATETIME
		)
	`).Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE tasks (
			id TEXT PRIMARY KEY,
			server_id TEXT,
			user TEXT,
			status TEXT,
			output TEXT,
			deleted_at DATETIME
		)
	`).Error)
	return db
}

func insertTaskLogRows(t *testing.T, db *gorm.DB, status, output string) {
	t.Helper()
	require.NoError(t, db.Exec(
		`INSERT INTO servers (id, public_ipv4, private_key, provider) VALUES (?, ?, ?, ?)`,
		"server-1", "127.0.0.1", "private-key", "custom",
	).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO tasks (id, server_id, user, status, output) VALUES (?, ?, ?, ?, ?)`,
		"task-1", "server-1", "root", status, output,
	).Error)
}

func startLogSSHServer(t *testing.T) (*taskrunner.Connection, func()) {
	t.Helper()
	_, clientPrivateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	clientPEM, err := ssh.MarshalPrivateKey(clientPrivateKey, "")
	require.NoError(t, err)

	_, hostPrivateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	hostSigner, err := ssh.NewSignerFromKey(hostPrivateKey)
	require.NoError(t, err)
	serverConfig := &ssh.ServerConfig{NoClientAuth: true}
	serverConfig.AddHostKey(hostSigner)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	done := make(chan struct{})
	accepted := make(chan net.Conn, 1)
	go func() {
		defer close(done)
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		accepted <- connection
		serverConnection, channels, requests, handshakeErr := ssh.NewServerConn(connection, serverConfig)
		if handshakeErr != nil {
			_ = connection.Close()
			return
		}
		go ssh.DiscardRequests(requests)
		var openChannels []ssh.Channel
		for channelRequest := range channels {
			if channelRequest.ChannelType() != "session" {
				_ = channelRequest.Reject(ssh.UnknownChannelType, "unsupported")
				continue
			}
			channel, channelRequests, channelErr := channelRequest.Accept()
			if channelErr != nil {
				continue
			}
			openChannels = append(openChannels, channel)
			go ssh.DiscardRequests(channelRequests)
		}
		for _, channel := range openChannels {
			_ = channel.Close()
		}
		_ = serverConnection.Close()
	}()

	address := listener.Addr().(*net.TCPAddr)
	connection := &taskrunner.Connection{
		Host:       "127.0.0.1",
		Port:       address.Port,
		User:       "root",
		PrivateKey: string(pem.EncodeToMemory(clientPEM)),
	}
	stop := func() {
		_ = listener.Close()
		select {
		case connection := <-accepted:
			_ = connection.Close()
		case <-done:
			return
		case <-time.After(2 * time.Second):
			t.Error("timed out cancelling SSH server")
			return
		}
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("timed out stopping SSH server")
		}
	}
	return connection, stop
}
