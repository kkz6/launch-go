package taskrunner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

var secretFileName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,79}$`)

type secretFileTask interface{ SecretFiles() map[string]string }

func taskSecretFiles(task Task) (map[string]string, error) {
	source, ok := task.(secretFileTask)
	if !ok {
		return nil, nil
	}
	files := source.SecretFiles()
	for name := range files {
		if !secretFileName.MatchString(name) {
			return nil, fmt.Errorf("invalid secret file name")
		}
	}
	return files, nil
}

// The parent shell owns cleanup so a task's own EXIT trap cannot replace it.
// Neither the script nor its wrapper contains secret values.
func wrapSecretScript(script, directory string) string {
	delimiter := "LAUNCH_BODY_" + ulid.Make().String()
	for strings.Contains(script, delimiter) {
		delimiter = "LAUNCH_BODY_" + ulid.Make().String()
	}
	return "#!/usr/bin/env bash\nset +x\nexport LAUNCH_SECRET_DIR=" + ShellQuote(directory) +
		"\ntrap 'rm -rf -- \"$LAUNCH_SECRET_DIR\"' EXIT\ntrap 'exit 143' TERM\ntrap 'exit 130' INT\n" +
		"bash <<'" + delimiter + "'\n" + script + "\n" + delimiter + "\nexit $?\n"
}

func prepareLocalSecrets(task Task, script string) (string, func(), error) {
	files, err := taskSecretFiles(task)
	noop := func() {}
	if err != nil || len(files) == 0 {
		return script, noop, err
	}
	directory, err := os.MkdirTemp("", "launch-secrets-")
	if err != nil {
		return "", noop, err
	}
	cleanup := func() { _ = os.RemoveAll(directory) }
	for name, value := range files {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(value), 0600); err != nil {
			cleanup()
			return "", noop, err
		}
	}
	return wrapSecretScript(script, directory), cleanup, nil
}

func prepareRemoteSecrets(ctx context.Context, client dispatcherSSHClient, task Task, taskDir, script string) (string, func(), error) {
	files, err := taskSecretFiles(task)
	noop := func() {}
	if err != nil || len(files) == 0 {
		return script, noop, err
	}
	directory := filepath.Join(taskDir, "secrets-"+ulid.Make().String())
	cleanup := func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = client.Run(cleanupCtx, "rm -rf -- "+ShellQuote(directory))
	}
	result, err := client.Run(ctx, "umask 077; mkdir -m 700 -- "+ShellQuote(directory))
	if err != nil || (result != nil && result.ExitCode != 0) {
		return "", noop, fmt.Errorf("create private task directory")
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := client.Upload(ctx, []byte(files[name]), filepath.Join(directory, name), 0600); err != nil {
			cleanup()
			return "", noop, fmt.Errorf("upload task secret: %w", err)
		}
	}
	// The watchdog also removes credentials after a killed/disconnected task.
	seconds := int(task.Timeout().Seconds()) + 60
	if seconds < 60 {
		seconds = 60
	}
	watchdog := fmt.Sprintf("sleep %d; rm -rf -- %s", seconds, ShellQuote(directory))
	result, err = client.Run(ctx, "nohup sh -c "+ShellQuote(watchdog)+" </dev/null >/dev/null 2>&1 &")
	if err != nil || (result != nil && result.ExitCode != 0) {
		cleanup()
		return "", noop, fmt.Errorf("schedule task secret cleanup")
	}
	return wrapSecretScript(script, directory), cleanup, nil
}
