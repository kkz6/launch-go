package taskrunner

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"golang.org/x/crypto/ssh"
)

type testPublicKey struct{}

func (testPublicKey) Type() string                        { return "ssh-ed25519" }
func (testPublicKey) Marshal() []byte                     { return []byte("test-key") }
func (testPublicKey) Verify([]byte, *ssh.Signature) error { return nil }

type closeBuffer struct{ bytes.Buffer }

func (b *closeBuffer) Close() error { return nil }

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
		stdin := &closeBuffer{}
		waited := false
		err := writeSCPFile(
			stdin,
			bytes.NewBuffer([]byte{0, 0, 0}),
			[]byte("payload"),
			"script.sh",
			0o700,
			func() error { waited = true; return nil },
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
