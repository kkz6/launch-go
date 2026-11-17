package services

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net"
	"net/url"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/database/serializers"
	"github.com/kkz6/launch-go/internal/modules/server/jobs"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	"github.com/kkz6/launch-go/internal/pkg/queue"
	"github.com/kkz6/launch-go/internal/pkg/signedurl"
)

func TestCustomProvisionCommands(t *testing.T) {
	svc, db, server := serverDeleteFixture(t, types.ProviderCustom)
	svc.frontendURL = "https://app.example.com"
	server.Status = types.ServerStatusAwaitingConnection
	require.NoError(t, db.Save(&server).Error)

	signer := provisionTestSigner(t)

	ctx := context.Background()
	_, err := svc.GetProvisionCommand(ctx, server.ID, "another-team")
	require.Error(t, err)
	command, err := svc.GetProvisionCommand(ctx, server.ID, server.TeamID)
	require.NoError(t, err)
	download, err := url.Parse(strings.Split(command, "'")[1])
	require.NoError(t, err)
	require.Equal(t, "1", download.Query().Get("start"))
	require.True(t, signer.Verify(download.Path, download.Query()))
	require.NotEmpty(t, download.Query().Get("expires"))
	require.Contains(t, command, "wget -q")
	require.Contains(t, command, `printf '%s\n'`)

	legacy, err := url.Parse(server.GetProvisionScriptURL())
	require.NoError(t, err)
	tampered := legacy.Query()
	tampered.Set("start", "1")
	require.False(t, signer.Verify(legacy.Path, tampered))

	script, err := svc.GetProvisionScript(ctx, server.ID, "", true)
	require.NoError(t, err)
	callback, err := url.Parse(strings.Split(strings.Split(script, "--post-data='' '")[1], "'")[0])
	require.NoError(t, err)
	require.Equal(t, "/webhooks/servers/"+server.ID+"/provision", callback.Path)
	require.True(t, signer.Verify(callback.Path, callback.Query()))
	require.Contains(t, script, "Provisioning started. Check status:")
	require.Contains(t, script, "https://app.example.com/servers?provision="+server.ID)

	legacyScript, err := svc.GetProvisionScript(ctx, server.ID, server.TeamID, false)
	require.NoError(t, err)
	require.Contains(t, legacyScript, "Server is ready for provisioning.")
	require.NotContains(t, legacyScript, "--post-data")

	for _, shell := range []string{command, script, legacyScript} {
		check := exec.Command("bash", "-n")
		check.Stdin = strings.NewReader(shell)
		output, err := check.CombinedOutput()
		require.NoError(t, err, "%s", output)
	}
}

func TestCustomProvisionQueueFailureAllowsRetry(t *testing.T) {
	provisionTestSigner(t)
	svc, db, server := serverDeleteFixture(t, types.ProviderCustom)
	server.Status = types.ServerStatusAwaitingConnection
	require.NoError(t, db.Save(&server).Error)
	ctx := context.Background()
	require.ErrorIs(t, svc.queueCustomProvisioning(ctx, &server, ""), ErrQueueNotConfigured)

	redis := miniredis.RunT(t)
	svc.Queue = queue.NewClient(config.RedisConfig{Address: redis.Addr()})
	t.Cleanup(func() { require.NoError(t, svc.Queue.Close()) })
	require.NoError(t, db.Migrator().DropTable(&models.ServerSSHKey{}))
	require.ErrorContains(t, svc.queueCustomProvisioning(ctx, &server, ""), "failed to load server SSH keys")
	require.NoError(t, db.AutoMigrate(&models.SSHKey{}, &models.ServerSSHKey{}))
	key := &models.SSHKey{Name: "Deployment key", PublicKey: "ssh-ed25519 fixture"}
	key.TeamID = server.TeamID
	require.NoError(t, svc.repos.SSHKey().Create(ctx, key))
	require.NoError(t, svc.repos.SSHKey().AttachToServer(ctx, server.ID, key.ID))
	require.NoError(t, db.Exec(`CREATE TRIGGER provision_claim_guard BEFORE UPDATE ON servers
		WHEN NEW.status = 'starting' BEGIN SELECT RAISE(ABORT, 'cannot reserve server'); END`).Error)
	require.ErrorContains(t, svc.queueCustomProvisioning(ctx, &server, ""), "failed to persist connection state")
	require.NoError(t, db.Exec("DROP TRIGGER provision_claim_guard").Error)
	redis.SetError("ERR queue unavailable")

	require.ErrorContains(t, svc.queueCustomProvisioning(ctx, &server, ""), "failed to enqueue provision task")
	stored, err := svc.repos.Server().FindByID(ctx, server.ID)
	require.NoError(t, err)
	require.Equal(t, types.ServerStatusAwaitingConnection, stored.Status)
	require.False(t, stored.Connected)

	redis.SetError("")
	require.NoError(t, svc.queueCustomProvisioning(ctx, &server, ""))
	require.NoError(t, svc.queueCustomProvisioning(ctx, &server, ""))
	tasks, err := svc.Queue.Inspector().ListPendingTasks(queue.QueueDefault)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	var payload jobs.ProvisionServerPayload
	require.NoError(t, json.Unmarshal(tasks[0].Payload, &payload))
	require.Equal(t, []string{key.ID}, payload.SSHKeyIDs)
	stored, err = svc.repos.Server().FindByID(ctx, server.ID)
	require.NoError(t, err)
	require.Equal(t, types.ServerStatusStarting, stored.Status)
	require.True(t, stored.Connected)
	require.NoError(t, db.Model(&models.Server{}).Where("id = ?", server.ID).Update("status", types.ServerStatusFailed).Error)
	require.ErrorContains(t, svc.queueCustomProvisioning(ctx, &server, ""), "no longer awaiting connection")
}

func TestCustomProvisioningRespectsServerState(t *testing.T) {
	svc, db, server := serverDeleteFixture(t, types.ProviderCustom)
	ctx := context.Background()
	require.Error(t, svc.StartCustomProvisioning(ctx, "missing-server"))
	require.Error(t, svc.TryConnection(ctx, server.ID, "another-team", "user"))
	require.ErrorContains(t, svc.TryConnection(ctx, server.ID, server.TeamID, "user"), "not awaiting connection")
	_, err := svc.GetProvisionCommand(ctx, server.ID, server.TeamID)
	require.ErrorContains(t, err, "no longer awaiting connection")

	script, err := svc.GetProvisionScript(ctx, server.ID, "", false)
	require.NoError(t, err)
	require.Contains(t, script, "Check status:")
	require.NoError(t, db.Model(&server).Updates(map[string]any{
		"connected": true, "archived_at": time.Now(),
	}).Error)
	require.ErrorContains(t, svc.StartCustomProvisioning(ctx, server.ID), "only applies to custom servers")
	script, err = svc.GetProvisionScript(ctx, server.ID, "", true)
	require.NoError(t, err)
	require.Contains(t, script, "Launch management key authorized.")
	require.NotContains(t, script, "--post-data")
}

func TestCustomProvisioningVerifiesSSH(t *testing.T) {
	provisionTestSigner(t)
	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	block, err := ssh.MarshalPrivateKey(privateKey, "provision-test")
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(privateKey)
	require.NoError(t, err)

	for _, tt := range []struct {
		name     string
		exitCode uint32
		cancel   bool
		want     string
	}{
		{name: "verified connection"},
		{name: "remote command failed", exitCode: 1, want: "SSH whoami returned exit code 1"},
		{name: "request canceled during SSH session setup", cancel: true, want: "SSH command failed"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc, db, server := serverDeleteFixture(t, types.ProviderCustom)
			require.NoError(t, db.AutoMigrate(&models.SSHKey{}, &models.ServerSSHKey{}))
			attachTestQueue(t, svc)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var cancelProbe context.CancelFunc
			if tt.cancel {
				cancelProbe = cancel
			}
			port := serveProvisionSSHProbe(t, signer, tt.exitCode, cancelProbe)
			ip := "127.0.0.1"
			server.PublicIPv4 = &ip
			server.SSHPort = &port
			server.PrivateKey = dbtype.EncryptedString(pem.EncodeToMemory(block))
			server.HostKey = strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
			server.Status = types.ServerStatusAwaitingConnection
			require.NoError(t, db.Save(&server).Error)

			err := svc.StartCustomProvisioning(ctx, server.ID)
			if tt.want != "" {
				require.ErrorContains(t, err, tt.want)
			} else {
				require.NoError(t, err)
				require.NoError(t, svc.TryConnection(ctx, server.ID, server.TeamID, "user"))
			}
			if tt.want == "" {
				pending, err := svc.Queue.Inspector().ListPendingTasks(queue.QueueDefault)
				require.NoError(t, err)
				require.Len(t, pending, 1)
			} else {
				queues, err := svc.Queue.Inspector().Queues()
				require.NoError(t, err)
				require.Empty(t, queues)
			}
		})
	}
}

func serveProvisionSSHProbe(t *testing.T, signer ssh.Signer, exitCode uint32, cancel context.CancelFunc) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	done := make(chan struct{})
	t.Cleanup(func() {
		_ = listener.Close()
		<-done
	})
	config := &ssh.ServerConfig{
		PublicKeyCallback: func(metadata ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if metadata.User() != "root" || string(key.Marshal()) != string(signer.PublicKey().Marshal()) {
				return nil, errors.New("unexpected SSH identity")
			}
			return nil, nil
		},
	}
	config.AddHostKey(signer)
	go func() {
		defer close(done)
		raw, err := listener.Accept()
		if err != nil {
			return
		}
		defer raw.Close()
		_ = raw.SetDeadline(time.Now().Add(5 * time.Second))
		conn, channels, requests, err := ssh.NewServerConn(raw, config)
		if err != nil {
			return
		}
		defer conn.Close()
		go ssh.DiscardRequests(requests)
		session, ok := <-channels
		if !ok {
			return
		}
		if cancel != nil {
			cancel()
			_ = conn.Wait()
			return
		}
		channel, reqs, err := session.Accept()
		if err != nil {
			return
		}
		defer channel.Close()
		request, ok := <-reqs
		var execRequest struct{ Command string }
		if !ok || request.Type != "exec" || ssh.Unmarshal(request.Payload, &execRequest) != nil || execRequest.Command != "whoami" {
			return
		}
		_ = request.Reply(true, nil)
		_, _ = channel.Write([]byte("root\n"))
		_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{exitCode}))
	}()
	return listener.Addr().(*net.TCPAddr).Port
}
func TestCustomProvisionScriptReportsCallbackResult(t *testing.T) {
	provisionTestSigner(t)
	svc := &Service{frontendURL: "https://app.example.com"}
	server := &models.Server{
		Provider:  types.ProviderCustom,
		Status:    types.ServerStatusAwaitingConnection,
		PublicKey: "ssh-ed25519 test-key\n",
	}
	server.ID = "test-server"
	statusURL := "https://app.example.com/servers?provision=test-server"
	script := svc.generateAuthorizeKeyScript(server, true)

	// Exercise the shell with a temporary authorized_keys file and a stub callback.
	script = strings.NewReplacer("/root/.ssh", `"$TEST_SSH_DIR"`, `"$EUID"`, "0").Replace(script)
	script = `
chown() { :; }
wget() {
    grep -qxF 'ssh-ed25519 test-key' "$TEST_SSH_DIR/authorized_keys" || return 2
    return "$TEST_CALLBACK_EXIT"
}
` + script

	sshDir := t.TempDir()
	for _, code := range []string{"0", "1"} {
		check := exec.Command("bash")
		check.Env = append(check.Environ(), "TEST_SSH_DIR="+sshDir, "TEST_CALLBACK_EXIT="+code)
		check.Stdin = strings.NewReader(script)
		output, err := check.CombinedOutput()
		if code == "0" {
			require.NoError(t, err)
			require.Equal(t, "Provisioning started. Check status: "+statusURL+"\n", string(output))
		} else {
			require.Error(t, err)
			require.Equal(t, "Server is ready for provisioning. Open Launch to try the connection: "+statusURL+"\n", string(output))
		}
	}
}

func provisionTestSigner(t *testing.T) *signedurl.Signer {
	t.Helper()
	previous := signedurl.GetDefaultSigner()
	signer := signedurl.NewSigner("test-key").WithBaseURL("https://api.example.com")
	signedurl.SetDefaultSigner(signer)
	t.Cleanup(func() { signedurl.SetDefaultSigner(previous) })
	return signer
}
