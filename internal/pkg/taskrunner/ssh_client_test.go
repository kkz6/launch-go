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
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

type testPublicKey struct{}

func (testPublicKey) Type() string                        { return "ssh-ed25519" }
func (testPublicKey) Marshal() []byte                     { return []byte("test-key") }
func (testPublicKey) Verify([]byte, *ssh.Signature) error { return nil }

type closeBuffer struct{ bytes.Buffer }

func (b *closeBuffer) Close() error { return nil }

type trackedCloseBuffer struct {
	bytes.Buffer
	closed *bool
}

func (b *trackedCloseBuffer) Close() error {
	*b.closed = true
	return nil
}

func TestShellQuote(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "plain path", input: "path with spaces", want: "'path with spaces'"},
		{name: "single quote", input: "it's safe", want: "'it'\"'\"'s safe'"},
		{name: "shell metacharacter", input: "$(not executed)", want: "'$(not executed)'"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ShellQuote(tc.input); got != tc.want {
				t.Fatalf("ShellQuote(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestCompleteBackgroundTaskRoutesFailureToCallback(t *testing.T) {
	var failed bool
	task := NewBaseTask()
	task.FailedCallback = func(_ context.Context, result *TaskResult) {
		failed = errors.Is(result.Error, context.DeadlineExceeded)
	}

	pending := NewPendingTask(task).WithCompletionChannel(make(chan *TaskResult, 1))
	result := &TaskResult{TaskID: pending.TaskID, ExitCode: 1, Error: context.DeadlineExceeded}
	NewDispatcher(nil, nil).completeBackgroundTask(context.Background(), pending, result)

	if !failed {
		t.Fatal("expected the failed callback to receive the monitoring error")
	}
	if got := <-pending.GetCompletionChannel(); got != result {
		t.Fatal("expected the completion channel to receive the task result")
	}
}

func TestReadSCPAck(t *testing.T) {
	t.Run("accepts success", func(t *testing.T) {
		if err := readSCPAck(bufio.NewReader(bytes.NewBuffer([]byte{0}))); err != nil {
			t.Fatalf("expected success acknowledgement: %v", err)
		}
	})

	t.Run("returns remote rejection", func(t *testing.T) {
		err := readSCPAck(bufio.NewReader(bytes.NewBufferString("\x01permission denied\n")))
		if err == nil || err.Error() != "SCP rejected upload (1): permission denied" {
			t.Fatalf("unexpected rejection error: %v", err)
		}
	})

	t.Run("returns read failure", func(t *testing.T) {
		if err := readSCPAck(bufio.NewReader(bytes.NewBuffer(nil))); !errors.Is(err, io.EOF) {
			t.Fatalf("expected EOF, got %v", err)
		}
	})
}

func TestWriteSCPFile(t *testing.T) {
	t.Run("writes a complete acknowledged transfer", func(t *testing.T) {
		closed := false
		stdin := &trackedCloseBuffer{closed: &closed}
		waited := false
		err := writeSCPFile(
			stdin,
			bytes.NewBuffer([]byte{0, 0, 0}),
			[]byte("payload"),
			"script.sh",
			0o700,
			func() error {
				if !closed {
					t.Fatal("expected SCP stdin to close before waiting")
				}
				waited = true
				return nil
			},
		)
		if err != nil {
			t.Fatalf("writeSCPFile returned error: %v", err)
		}
		if !waited {
			t.Fatal("expected upload completion to wait for SCP")
		}
		want := "C0700 7 script.sh\npayload\x00"
		if got := stdin.String(); got != want {
			t.Fatalf("SCP payload = %q, want %q", got, want)
		}
	})

	t.Run("propagates completion failure", func(t *testing.T) {
		err := writeSCPFile(
			&closeBuffer{}, bytes.NewBuffer([]byte{0, 0, 0}), []byte("x"), "x", os.FileMode(0o600),
			func() error { return errors.New("remote SCP failed") },
		)
		if err == nil || err.Error() != "complete SCP upload: remote SCP failed" {
			t.Fatalf("unexpected completion error: %v", err)
		}
	})
}

func TestManagedHostKeyPersistenceFailureRejectsConnection(t *testing.T) {
	original := hostKeyPersister
	t.Cleanup(func() { hostKeyPersister = original })
	hostKeyPersister = func(context.Context, string, string) error { return errors.New("database unavailable") }

	err := makeHostKeyCallback("server-1", "")("server", nil, testPublicKey{})
	if err == nil || err.Error() != "persist SSH host key for server server-1: database unavailable" {
		t.Fatalf("unexpected host-key persistence error: %v", err)
	}
}

func TestNewSSHClientValidatesAuthenticationAndDefaults(t *testing.T) {
	if _, err := NewSSHClient(SSHConfig{Host: "example.test"}); err == nil || err.Error() != "no authentication method provided" {
		t.Fatalf("expected missing authentication error, got %v", err)
	}

	client, err := NewSSHClient(SSHConfig{Host: "example.test", User: "deploy", Password: "secret"})
	if err != nil {
		t.Fatalf("NewSSHClient returned error: %v", err)
	}
	if client.port != 22 || client.config.User != "deploy" {
		t.Fatalf("unexpected client defaults: port=%d user=%q", client.port, client.config.User)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("Close returned error without connection: %v", err)
	}
}

func TestConnectionHelpersAndClientOptions(t *testing.T) {
	conn := &Connection{Host: "example.test", User: "deploy", PrivateKey: "key", ScriptPath: "/tmp/scripts", ServerID: "server-1", HostKey: "ssh-ed25519 AAAA"}
	if got := conn.GetScriptPath(); got != "/tmp/scripts" {
		t.Fatalf("GetScriptPath() = %q", got)
	}
	if !conn.Is(&Connection{Host: "example.test", User: "deploy"}) {
		t.Fatal("expected matching host/user connection")
	}
	if conn.Is(&Connection{Host: "other.test", User: "deploy"}) {
		t.Fatal("unexpected match for different host")
	}
	if !(*Connection)(nil).Is(nil) {
		t.Fatal("nil connections should match")
	}

	_, privateKey, err := ed25519.GenerateKey(cryptorand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}
	privateKeyPEM, err := ssh.MarshalPrivateKey(privateKey, "")
	if err != nil {
		t.Fatalf("MarshalPrivateKey returned error: %v", err)
	}
	client, err := NewSSHClientFromConnection(
		&Connection{Host: "example.test", User: "deploy", PrivateKey: string(pem.EncodeToMemory(privateKeyPEM))},
		WithSSHPort(2200), WithSSHTimeout(time.Second),
	)
	if err != nil {
		t.Fatalf("NewSSHClientFromConnection returned error: %v", err)
	}
	if client.port != 2200 || client.timeout != time.Second {
		t.Fatalf("options not applied: port=%d timeout=%s", client.port, client.timeout)
	}
	if _, err := NewSSHClientFromConnection(nil); err == nil {
		t.Fatal("expected nil connection error")
	}
}

func TestLineReaderAndExpandPath(t *testing.T) {
	r := newLineReader(strings.NewReader("first\nsecond"))
	line, err := r.readLine()
	if err != nil || line != "first" {
		t.Fatalf("first line = %q, %v", line, err)
	}
	line, err = r.readLine()
	if !errors.Is(err, io.EOF) || line != "second" {
		t.Fatalf("second line = %q, %v", line, err)
	}
	if got := expandPath("/tmp/key"); got != "/tmp/key" {
		t.Fatalf("expandPath changed absolute path: %q", got)
	}
	if got := expandPath("~/key"); !strings.HasSuffix(got, "/key") {
		t.Fatalf("expandPath did not expand home path: %q", got)
	}
}

func TestWaitForConnectionHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client, err := NewSSHClient(SSHConfig{Host: "127.0.0.1", Password: "secret"})
	if err != nil {
		t.Fatalf("NewSSHClient returned error: %v", err)
	}
	if err := client.WaitForConnection(ctx, 3); !errors.Is(err, context.Canceled) {
		t.Fatalf("WaitForConnection error = %v, want context.Canceled", err)
	}
}
