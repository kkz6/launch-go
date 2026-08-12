package handlers

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"io"
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	serverModels "github.com/kkz6/launch-go/internal/modules/server/models"
	serverTypes "github.com/kkz6/launch-go/internal/modules/server/types"
	launchstatus "github.com/kkz6/launch-go/internal/pkg/launch/status"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

type serviceStatusSSHServer struct {
	listener net.Listener
	signer   ssh.Signer

	mu       sync.Mutex
	commands []string
	wg       sync.WaitGroup
	close    sync.Once
}

func newServiceStatusSSHServer(t *testing.T) *serviceStatusSSHServer {
	t.Helper()

	_, hostKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(hostKey)
	require.NoError(t, err)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	server := &serviceStatusSSHServer{listener: listener, signer: signer}
	server.wg.Add(1)
	go server.serve()
	t.Cleanup(server.Close)

	return server
}

func (s *serviceStatusSSHServer) Close() {
	s.close.Do(func() {
		_ = s.listener.Close()
		s.wg.Wait()
	})
}

func (s *serviceStatusSSHServer) serve() {
	defer s.wg.Done()

	for {
		raw, err := s.listener.Accept()
		if err != nil {
			return
		}

		s.wg.Add(1)
		go s.serveConnection(raw)
	}
}

func (s *serviceStatusSSHServer) serveConnection(raw net.Conn) {
	defer s.wg.Done()

	config := &ssh.ServerConfig{NoClientAuth: true}
	config.AddHostKey(s.signer)
	connection, channels, requests, err := ssh.NewServerConn(raw, config)
	if err != nil {
		_ = raw.Close()
		return
	}
	defer connection.Close()
	go ssh.DiscardRequests(requests)

	for newChannel := range channels {
		if newChannel.ChannelType() != "session" {
			_ = newChannel.Reject(ssh.UnknownChannelType, "unsupported")
			continue
		}

		channel, channelRequests, err := newChannel.Accept()
		if err != nil {
			continue
		}
		s.wg.Add(1)
		go s.serveSession(channel, channelRequests)
	}
}

func (s *serviceStatusSSHServer) serveSession(channel ssh.Channel, requests <-chan *ssh.Request) {
	defer s.wg.Done()
	defer channel.Close()

	for request := range requests {
		if request.Type != "exec" {
			_ = request.Reply(false, nil)
			continue
		}

		var payload struct{ Command string }
		if err := ssh.Unmarshal(request.Payload, &payload); err != nil {
			_ = request.Reply(false, nil)
			return
		}
		_ = request.Reply(true, nil)

		s.mu.Lock()
		s.commands = append(s.commands, payload.Command)
		s.mu.Unlock()

		if strings.Contains(payload.Command, "systemctl show redis-server") {
			_, _ = channel.Write([]byte("inactive\n---\nLoadState=not-found\nActiveState=inactive\nSubState=dead\n"))
		}
		_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
		return
	}
}

func (s *serviceStatusSSHServer) connection(t *testing.T) *taskrunner.Connection {
	t.Helper()

	_, clientKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	keyBlock, err := ssh.MarshalPrivateKey(clientKey, "")
	require.NoError(t, err)
	port := s.listener.Addr().(*net.TCPAddr).Port

	return &taskrunner.Connection{
		Host:       "127.0.0.1",
		Port:       port,
		User:       "root",
		PrivateKey: string(pem.EncodeToMemory(keyBlock)),
	}
}

func (s *serviceStatusSSHServer) ranSystemdProbe() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, command := range s.commands {
		if strings.Contains(command, "systemctl show redis-server") &&
			strings.Contains(command, "--property=LoadState,ActiveState") {
			return true
		}
	}
	return false
}

func TestGetServiceStatusPersistsMissingRedisUnit(t *testing.T) {
	server := newServiceStatusSSHServer(t)
	connection, err := server.connection(t).Dial()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, connection.Close()) })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE services (
		id TEXT PRIMARY KEY,
		status TEXT NOT NULL,
		version TEXT,
		updated_at DATETIME
	)`).Error)
	require.NoError(t, db.Exec(
		"INSERT INTO services (id, status, version) VALUES (?, ?, ?)",
		"service-redis",
		serverTypes.ServiceStatusStopped,
		"latest",
	).Error)

	service := &serverModels.InstalledService{
		Software: "redis",
		Name:     "Redis",
		Status:   serverTypes.ServiceStatusStopped,
		Version:  "latest",
	}
	service.ID = "service-redis"
	handler := &ServiceStatusHandler{Base: Base{DB: db, Logger: zerolog.New(io.Discard)}}

	serviceStatus := handler.getServiceStatus(connection, service)

	assert.Equal(t, launchstatus.StateMissing, serviceStatus.Status)
	assert.False(t, serviceStatus.IsActive)
	assert.True(t, server.ranSystemdProbe())
	assert.Equal(t, serverTypes.ServiceStatusMissing, service.Status)

	var persistedStatus string
	require.NoError(t, db.Raw(
		"SELECT status FROM services WHERE id = ?",
		service.ID,
	).Scan(&persistedStatus).Error)
	assert.Equal(t, serverTypes.ServiceStatusMissing.String(), persistedStatus)
}

func TestParseServiceOutputReportsMissingSystemdUnit(t *testing.T) {
	handler := &ServiceStatusHandler{}
	serviceStatus := &launchstatus.ServiceStatus{}

	handler.parseServiceOutput("inactive\n---\nLoadState=not-found\nActiveState=inactive\nSubState=dead\nMainPID=1234\n", serviceStatus)

	assert.Equal(t, launchstatus.StateMissing, serviceStatus.Status)
	assert.False(t, serviceStatus.IsActive)
	assert.Equal(t, 1234, serviceStatus.PID)
}

func TestParseServiceOutputReportsLoadedInactiveUnitAsStopped(t *testing.T) {
	handler := &ServiceStatusHandler{}
	serviceStatus := &launchstatus.ServiceStatus{}

	handler.parseServiceOutput("inactive\n---\nLoadState=loaded\nActiveState=inactive\nSubState=dead\n", serviceStatus)

	assert.Equal(t, launchstatus.StateStopped, serviceStatus.Status)
	assert.False(t, serviceStatus.IsActive)
}

func TestPersistMissingStatusUpdatesTheStoredService(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE services (
		id TEXT PRIMARY KEY,
		status TEXT NOT NULL,
		updated_at DATETIME
	)`).Error)
	require.NoError(t, db.Exec(
		"INSERT INTO services (id, status) VALUES (?, ?)",
		"service-redis",
		serverTypes.ServiceStatusStopped,
	).Error)

	service := &serverModels.InstalledService{Status: serverTypes.ServiceStatusStopped}
	service.ID = "service-redis"
	handler := &ServiceStatusHandler{Base: Base{DB: db, Logger: zerolog.New(io.Discard)}}

	handler.persistMissingStatus(service, launchstatus.StateMissing)

	var persistedStatus string
	require.NoError(t, db.Raw(
		"SELECT status FROM services WHERE id = ?",
		service.ID,
	).Scan(&persistedStatus).Error)
	assert.Equal(t, serverTypes.ServiceStatusMissing.String(), persistedStatus)
	assert.Equal(t, serverTypes.ServiceStatusMissing, service.Status)

	// Both no-op paths remain safe and avoid unnecessary database writes.
	handler.persistMissingStatus(service, launchstatus.StateMissing)
	handler.persistMissingStatus(service, launchstatus.StateRunning)
}

func TestPersistMissingStatusKeepsCachedStateWhenPersistenceFails(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	service := &serverModels.InstalledService{Status: serverTypes.ServiceStatusStopped}
	service.ID = "service-redis"
	handler := &ServiceStatusHandler{Base: Base{DB: db, Logger: zerolog.New(io.Discard)}}

	handler.persistMissingStatus(service, launchstatus.StateMissing)

	assert.Equal(t, serverTypes.ServiceStatusStopped, service.Status)
}
