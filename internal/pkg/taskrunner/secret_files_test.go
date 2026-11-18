package taskrunner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTaskSecretsArePrivateSeparateAndCleaned(t *testing.T) {
	for _, exit := range []string{"0", "7"} {
		t.Run(exit, func(t *testing.T) {
			t.Parallel()
			const secret = "private-repository-credential"
			task := NewBaseTask(WithSecretFiles(map[string]string{"git-credentials": secret}))
			script, cleanup, err := prepareLocalSecrets(task, `printf '%s\n' "$LAUNCH_SECRET_DIR"
test "$(cat "$LAUNCH_SECRET_DIR/git-credentials")" = "$EXPECTED"
trap ':' EXIT
exit `+exit)
			require.NoError(t, err)
			defer cleanup()
			require.NotContains(t, script, secret)
			// Inspect permissions before execution, without including credentials in output.
			pathCmd := exec.Command("bash", "-c", strings.Split(script, "\ntrap ")[0]+"\nprintf '%s' \"$LAUNCH_SECRET_DIR\"")
			pathBytes, err := pathCmd.Output()
			require.NoError(t, err)
			directory := string(pathBytes)
			info, err := os.Stat(directory)
			require.NoError(t, err)
			require.Equal(t, os.FileMode(0700), info.Mode().Perm())
			info, err = os.Stat(filepath.Join(directory, "git-credentials"))
			require.NoError(t, err)
			require.Equal(t, os.FileMode(0600), info.Mode().Perm())
			command := exec.Command("bash", "-c", script)
			command.Env = append(os.Environ(), "EXPECTED="+secret)
			output, err := command.CombinedOutput()
			if exit == "0" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			require.Equal(t, directory+"\n", string(output))
			require.NotContains(t, string(output), secret)
			_, err = os.Stat(directory)
			require.True(t, os.IsNotExist(err))
		})
	}
}

func TestTaskSecretNamesCannotEscapeDirectory(t *testing.T) {
	for _, name := range []string{"../token", "/tmp/token", "key.txt", "key\nother", ""} {
		_, cleanup, err := prepareLocalSecrets(NewBaseTask(WithSecretFiles(map[string]string{name: "secret"})), "true")
		require.Error(t, err)
		cleanup()
	}
}

func TestRemoteTaskSecretsUseSeparateRestrictedUpload(t *testing.T) {
	const secret = "private-token"
	client := &dispatcherFakeSSHClient{}
	task := NewBaseTask(WithSecretFiles(map[string]string{"git-credentials": secret}), WithTimeoutSeconds(600))
	script, cleanup, err := prepareRemoteSecrets(context.Background(), client, task, "/tmp/tasks", "true")
	require.NoError(t, err)
	require.NotContains(t, script, secret)
	require.Equal(t, secret, string(client.uploaded))
	require.Equal(t, os.FileMode(0600), client.uploadMode)
	require.Contains(t, client.runCommands[0], "mkdir -m 700")
	require.Contains(t, client.runCommands[1], "sleep 660")
	for _, command := range client.runCommands {
		require.NotContains(t, command, secret)
	}
	cleanup()
	require.Contains(t, client.runCommands[2], "rm -rf --")
}

func TestFailedSecretUploadCleansDirectory(t *testing.T) {
	client := &dispatcherFakeSSHClient{uploadErr: errors.New("upload interrupted")}
	_, _, err := prepareRemoteSecrets(context.Background(), client, NewBaseTask(WithSecretFiles(map[string]string{"git-credentials": "secret"})), "/tmp/tasks", "true")
	require.Error(t, err)
	require.Len(t, client.runCommands, 2)
	require.Contains(t, client.runCommands[1], "rm -rf --")
}

func TestSecretFilesAreCopied(t *testing.T) {
	files := map[string]string{"token": "original"}
	task := NewBaseTask(WithSecretFiles(files))
	files["token"] = "changed"
	got := task.SecretFiles()
	got["token"] = "changed"
	require.Equal(t, "original", task.SecretFiles()["token"])
}

func TestRemoteExecutionRechecksAuthorityBeforeCredentialsAndStart(t *testing.T) {
	for _, denyOn := range []int{1, 2} {
		t.Run(fmt.Sprint(denyOn), func(t *testing.T) {
			calls := 0
			denied := errors.New("repository access revoked")
			task := NewBaseTask(WithScript("echo started"), WithSecretFiles(map[string]string{"git-credentials": "private-token"}), WithBeforeExecution(func(context.Context) error {
				calls++
				if calls == denyOn {
					return denied
				}
				return nil
			}))
			client := &dispatcherFakeSSHClient{}
			dispatcher := newDispatcherCoverageHarness(client)
			_, err := dispatcher.runRemote(context.Background(), &PendingTask{Task: task, TaskID: "task", Connection: &Connection{Host: "example.test", User: "launcher"}})
			require.ErrorIs(t, err, denied)
			require.Equal(t, denyOn, calls)
			if denyOn == 1 {
				require.Empty(t, client.uploaded)
			} else {
				require.Contains(t, client.runCommands[len(client.runCommands)-1], "rm -rf --")
			}
			for _, command := range client.runCommands {
				require.NotContains(t, command, "echo started")
				require.NotContains(t, command, "timeout ")
				require.NotContains(t, command, "private-token")
			}
		})
	}
}
