package tasks

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	servertypes "github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// =============================================================================
// Minimal SSH server
// =============================================================================

// recoverySSHServer answers exec requests from a table of command substring →
// stdout. recoverTask decides what to do purely from what these commands
// print, so scripting them is enough to drive every branch.
type recoverySSHServer struct {
	listener  net.Listener
	signer    ssh.Signer
	responses []recoverySSHResponse

	mu       sync.Mutex
	commands []string
	wg       sync.WaitGroup
	closeOne sync.Once
}

type recoverySSHResponse struct {
	match  string
	stdout string
}

func newRecoverySSHServer(t *testing.T, responses ...recoverySSHResponse) *recoverySSHServer {
	t.Helper()

	_, hostKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(hostKey)
	require.NoError(t, err)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	server := &recoverySSHServer{
		listener:  listener,
		signer:    signer,
		responses: responses,
	}

	server.wg.Add(1)
	go server.serve()
	t.Cleanup(server.Close)

	return server
}

func (s *recoverySSHServer) port() int {
	_, portText, _ := net.SplitHostPort(s.listener.Addr().String())
	port, _ := strconv.Atoi(portText)
	return port
}

func (s *recoverySSHServer) ranCommands() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.commands...)
}

func (s *recoverySSHServer) Close() {
	s.closeOne.Do(func() {
		_ = s.listener.Close()
		s.wg.Wait()
	})
}

func (s *recoverySSHServer) serve() {
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

func (s *recoverySSHServer) serveConnection(raw net.Conn) {
	defer s.wg.Done()

	cfg := &ssh.ServerConfig{
		PublicKeyCallback: func(ssh.ConnMetadata, ssh.PublicKey) (*ssh.Permissions, error) {
			return nil, nil
		},
	}
	cfg.AddHostKey(s.signer)

	conn, channels, requests, err := ssh.NewServerConn(raw, cfg)
	if err != nil {
		_ = raw.Close()
		return
	}
	defer conn.Close()

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

func (s *recoverySSHServer) serveSession(channel ssh.Channel, requests <-chan *ssh.Request) {
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

		for _, response := range s.responses {
			if strings.Contains(payload.Command, response.match) {
				_, _ = channel.Write([]byte(response.stdout))
				break
			}
		}

		_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
		return
	}
}

// =============================================================================
// Wiring
// =============================================================================

func recoveryPrivateKeyPEM(t *testing.T) string {
	t.Helper()

	_, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	block, err := ssh.MarshalPrivateKey(key, "")
	require.NoError(t, err)
	return string(pem.EncodeToMemory(block))
}

// sshBackedTask points a task at the in-process SSH server. The TOFU
// host-key path needs a persister registered, otherwise the first connect to
// a server with an ID and no pinned key is refused by design.
func sshBackedTask(t *testing.T, server *recoverySSHServer) *models.Task {
	t.Helper()

	taskrunner.RegisterHostKeyPersister(func(context.Context, string, string) error { return nil })
	t.Cleanup(func() { taskrunner.RegisterHostKeyPersister(nil) })

	host := "127.0.0.1"
	port := server.port()

	modelServer := &models.Server{
		PublicIPv4: &host,
		SSHPort:    &port,
		PrivateKey: dbtype.EncryptedString(recoveryPrivateKeyPEM(t)),
	}
	modelServer.ID = "srv-1"
	modelServer.TeamID = "team-1"

	task := recoveryTask(modelServer)
	task.User = "root"
	return task
}

func sshRecoverer(t *testing.T, db *gorm.DB, broadcaster *recoveryBroadcaster) *TaskRecoverer {
	t.Helper()
	recoverer := newRecoverer(t, db, broadcaster)
	recoverer.dispatcher = &taskrunner.Dispatcher{}
	return recoverer
}

// =============================================================================
// recoverTask
// =============================================================================

// An exit-code file on disk means the script finished while the worker was
// away, so the task is finalised from what the server recorded.
func TestRecoverTaskFinalizesAlreadyCompletedTask(t *testing.T) {
	server := newRecoverySSHServer(t,
		recoverySSHResponse{match: ".exit", stdout: "0\n"},
		recoverySSHResponse{match: ".log", stdout: "provisioning finished\n"},
	)

	db := recoveryDB(t)
	broadcaster := &recoveryBroadcaster{}
	task := sshBackedTask(t, server)
	require.NoError(t, db.Create(task).Error)

	recoverer := sshRecoverer(t, db, broadcaster)
	require.NoError(t, recoverer.recoverTask(context.Background(), task))

	var stored models.Task
	require.NoError(t, db.First(&stored, "id = ?", task.ID).Error)
	assert.Equal(t, string(servertypes.TaskStatusFinished), stored.Status)
	require.NotNil(t, stored.ExitCode)
	assert.Equal(t, 0, *stored.ExitCode)
	assert.Contains(t, stored.Output.String(), "provisioning finished")
	assert.NotEmpty(t, broadcaster.events)
}

func TestRecoverTaskFinalizesFailedAndTimedOutTasks(t *testing.T) {
	tests := []struct {
		name       string
		exitCode   string
		wantStatus servertypes.TaskStatus
	}{
		{name: "failure", exitCode: "1\n", wantStatus: servertypes.TaskStatusFailed},
		{name: "timeout", exitCode: "124\n", wantStatus: servertypes.TaskStatusTimeout},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := newRecoverySSHServer(t,
				recoverySSHResponse{match: ".exit", stdout: tc.exitCode},
				recoverySSHResponse{match: ".log", stdout: "output tail\n"},
			)

			db := recoveryDB(t)
			task := sshBackedTask(t, server)
			require.NoError(t, db.Create(task).Error)

			recoverer := sshRecoverer(t, db, &recoveryBroadcaster{})
			require.NoError(t, recoverer.recoverTask(context.Background(), task))

			var stored models.Task
			require.NoError(t, db.First(&stored, "id = ?", task.ID).Error)
			assert.Equal(t, string(tc.wantStatus), stored.Status)
		})
	}
}

// A non-numeric exit code means the file is corrupt; that's an error rather
// than a silent "finished".
func TestRecoverTaskRejectsUnparseableExitCode(t *testing.T) {
	server := newRecoverySSHServer(t,
		recoverySSHResponse{match: ".exit", stdout: "not-a-number\n"},
	)

	db := recoveryDB(t)
	task := sshBackedTask(t, server)
	require.NoError(t, db.Create(task).Error)

	recoverer := sshRecoverer(t, db, &recoveryBroadcaster{})
	err := recoverer.recoverTask(context.Background(), task)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse exit code")
}

// No exit-code file but a log file touched in the last minute means the
// script is still going, so the task is re-monitored rather than failed.
func TestRecoverTaskRemonitorsStillRunningTask(t *testing.T) {
	server := newRecoverySSHServer(t,
		recoverySSHResponse{match: "find", stdout: "/home/root/.launch/task-task-1.log\n"},
	)

	db := recoveryDB(t)
	task := sshBackedTask(t, server)
	require.NoError(t, db.Create(task).Error)

	recoverer := sshRecoverer(t, db, &recoveryBroadcaster{})
	err := recoverer.recoverTask(context.Background(), task)

	// The zero-value dispatcher has no stream monitor, so re-monitoring
	// reports that rather than silently dropping the task.
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stream monitor not available")

	assert.Condition(t, func() bool {
		for _, command := range server.ranCommands() {
			if strings.Contains(command, "find") {
				return true
			}
		}
		return false
	}, "should have probed the log file for recent writes")
}

// Neither an exit code nor a recently-written log: the worker cannot tell
// what happened, so the task is failed rather than left running forever.
func TestRecoverTaskMarksUnknownStateAsFailed(t *testing.T) {
	server := newRecoverySSHServer(t)

	db := recoveryDB(t)
	broadcaster := &recoveryBroadcaster{}
	task := sshBackedTask(t, server)
	require.NoError(t, db.Create(task).Error)

	recoverer := sshRecoverer(t, db, broadcaster)
	require.NoError(t, recoverer.recoverTask(context.Background(), task))

	var stored models.Task
	require.NoError(t, db.First(&stored, "id = ?", task.ID).Error)
	assert.Equal(t, string(servertypes.TaskStatusFailed), stored.Status)
	assert.Contains(t, stored.Output.String(), "task lost during worker restart")
}

func TestRecoverTaskSurfacesConnectionFailures(t *testing.T) {
	db := recoveryDB(t)
	task := recoveryTask(recoveryServer())
	task.Server.PublicIPv4 = nil

	recoverer := sshRecoverer(t, db, &recoveryBroadcaster{})
	assert.Error(t, recoverer.recoverTask(context.Background(), task))
}

// A host that refuses the connection fails the recovery for that task; the
// caller marks it failed and moves on to the next one.
func TestRecoverTaskSurfacesDialFailures(t *testing.T) {
	server := newRecoverySSHServer(t)
	db := recoveryDB(t)
	task := sshBackedTask(t, server)
	server.Close()

	recoverer := sshRecoverer(t, db, &recoveryBroadcaster{})
	err := recoverer.recoverTask(context.Background(), task)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "SSH connection failed")
}

// =============================================================================
// RecoverOrphanedTasks end to end
// =============================================================================

func TestRecoverOrphanedTasksRecoversAndReportsFailures(t *testing.T) {
	server := newRecoverySSHServer(t,
		recoverySSHResponse{match: ".exit", stdout: "0\n"},
		recoverySSHResponse{match: ".log", stdout: "done\n"},
	)

	db := recoveryDB(t)
	require.NoError(t, db.AutoMigrate(&models.Server{}))

	// Preload("Server") needs the row to exist, or the task is
	// short-circuited as "server not found" instead of being recovered.
	// Creating the task cascades to its Server association.
	task := sshBackedTask(t, server)
	require.NoError(t, db.Create(task).Error)

	recoverer := sshRecoverer(t, db, &recoveryBroadcaster{})
	require.NoError(t, recoverer.RecoverOrphanedTasks(context.Background()))

	var stored models.Task
	require.NoError(t, db.First(&stored, "id = ?", task.ID).Error)
	assert.Equal(t, string(servertypes.TaskStatusFinished), stored.Status)
}

// A task whose recovery fails is marked failed and the sweep continues to the
// next one, rather than aborting the whole startup pass.
func TestRecoverOrphanedTasksMarksUnrecoverableTasksFailed(t *testing.T) {
	server := newRecoverySSHServer(t)

	db := recoveryDB(t)
	require.NoError(t, db.AutoMigrate(&models.Server{}))

	task := sshBackedTask(t, server)
	require.NoError(t, db.Create(task).Error)
	server.Close()

	recoverer := sshRecoverer(t, db, &recoveryBroadcaster{})
	require.NoError(t, recoverer.RecoverOrphanedTasks(context.Background()))

	var stored models.Task
	require.NoError(t, db.First(&stored, "id = ?", task.ID).Error)
	assert.Equal(t, string(servertypes.TaskStatusFailed), stored.Status)
	assert.Contains(t, stored.Output.String(), "recovery failed")
}
