package tasks

import (
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
)

// These tests execute the generated script, replacing only its temporary root
// and external download/container commands. They never connect to a database.
func TestRestoreRejectsInvalidArchiveBeforeAnyContainerMutation(t *testing.T) {
	for _, engine := range []dockertypes.DatabaseEngine{dockertypes.DatabaseEnginePostgres, dockertypes.DatabaseEngineRedis} {
		for _, mode := range []string{"missing", "empty", "empty-payload", "unreadable", "truncated", "corrupt", "download-failure"} {
			t.Run(string(engine)+"/"+mode, func(t *testing.T) {
				if mode == "unreadable" && os.Geteuid() == 0 {
					t.Skip("root can read files regardless of permission bits")
				}
				fixture := newRestoreShellFixture(t, mode)
				output, err := fixture.run(t, engine)
				require.Error(t, err, "invalid archive reported success: %s", output)
				require.NotContains(t, output, "::LAUNCH::restore_step::done")
				require.NoFileExists(t, filepath.Join(fixture.root, "container-calls"), "preflight must finish before ingest or Redis stop")
				fixture.assertCleanup(t)
			})
		}
	}
}

func TestRestoreValidArchiveReachesOnlySelectedEngine(t *testing.T) {
	for _, engine := range []dockertypes.DatabaseEngine{
		dockertypes.DatabaseEnginePostgres, dockertypes.DatabaseEngineMySQL,
		dockertypes.DatabaseEngineMariaDB, dockertypes.DatabaseEngineRedis,
	} {
		t.Run(string(engine), func(t *testing.T) {
			fixture := newRestoreShellFixture(t, "valid")
			output, err := fixture.run(t, engine)
			require.NoError(t, err, "%s", output)
			require.Contains(t, output, "::LAUNCH::restore_step::done")
			payload, err := os.ReadFile(filepath.Join(fixture.root, "ingested"))
			require.NoError(t, err)
			require.Equal(t, restoreFixturePayload, string(payload))
			calls, err := os.ReadFile(filepath.Join(fixture.root, "container-calls"))
			require.NoError(t, err)
			require.Contains(t, string(calls), "selected-container")
			if engine == dockertypes.DatabaseEngineRedis {
				require.Equal(t, []string{"stop", "cp", "start"}, restoreCallKinds(string(calls)))
			} else {
				require.Equal(t, []string{"exec"}, restoreCallKinds(string(calls)))
				require.Contains(t, string(calls), "selected_db")
			}
			fixture.assertCleanup(t)
		})
	}
}

func TestRestoreIngestFailureDoesNotReportDone(t *testing.T) {
	fixture := newRestoreShellFixture(t, "ingest-failure")
	output, err := fixture.run(t, dockertypes.DatabaseEnginePostgres)
	require.Error(t, err)
	require.NotContains(t, output, "::LAUNCH::restore_step::done")
	require.Contains(t, strings.ToLower(output), "partial", "ingest failure should explain possible partial changes")
	fixture.assertCleanup(t)
}

func TestRestoreTemporaryCleanupPreservesOtherAttempts(t *testing.T) {
	fixture := newRestoreShellFixture(t, "valid")
	oldAttempt := filepath.Join(fixture.root, "restore", "run-1")
	require.NoError(t, os.MkdirAll(oldAttempt, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(oldAttempt, "keep"), []byte("another attempt"), 0o600))
	output, err := fixture.run(t, dockertypes.DatabaseEnginePostgres)
	require.NoError(t, err, "%s", output)
	contents, err := os.ReadFile(filepath.Join(oldAttempt, "keep"))
	require.NoError(t, err)
	require.Equal(t, "another attempt", string(contents))
	entries, err := os.ReadDir(filepath.Join(fixture.root, "restore"))
	require.NoError(t, err)
	require.Len(t, entries, 1, "only the unrelated attempt should remain")
}

const restoreFixturePayload = "CREATE TABLE restored_data (id integer);\n"

type restoreShellFixture struct {
	root string
	mode string
}

func newRestoreShellFixture(t *testing.T, mode string) restoreShellFixture {
	t.Helper()
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	require.NoError(t, os.MkdirAll(bin, 0o700))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "restore"), 0o700))
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if mode != "empty-payload" {
		_, err := writer.Write([]byte(restoreFixturePayload))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	archive := compressed.Bytes()
	if mode == "truncated" {
		// The complete SQL payload is recoverable, but the gzip trailer is
		// missing. Streaming directly into the database would already mutate it.
		archive = archive[:len(archive)-8]
	} else if mode == "corrupt" {
		archive = []byte("this is not a gzip archive")
	}
	require.NoError(t, os.WriteFile(filepath.Join(root, "archive.gz"), archive, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(bin, "launch-agent"), []byte(restoreDownloadStub), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(bin, "docker"), []byte(restoreDockerStub), 0o700))
	return restoreShellFixture{root: root, mode: mode}
}

func (f restoreShellFixture) run(t *testing.T, engine dockertypes.DatabaseEngine) (string, error) {
	t.Helper()
	script := RestoreBackupScript(RestoreBackupConfig{
		RunID: "run-1", ContainerName: "selected-container", Engine: engine,
		Username: "fixture_user", Password: "fixture_password", Database: "selected_db",
		ObjectKey: "snapshot.gz", Bucket: "fixture-bucket",
	})
	script = strings.ReplaceAll(script, "/var/lib/launch/restore", filepath.Join(f.root, "restore"))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "-s")
	cmd.Env = append(os.Environ(), "PATH="+filepath.Join(f.root, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"), "RESTORE_FIXTURE_ROOT="+f.root, "RESTORE_FIXTURE_MODE="+f.mode)
	cmd.Stdin = strings.NewReader(script)
	output, err := cmd.CombinedOutput()
	require.NoError(t, ctx.Err(), "restore script timed out: %s", output)
	return string(output), err
}

func (f restoreShellFixture) assertCleanup(t *testing.T) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(f.root, "restore"))
	require.NoError(t, err)
	require.Empty(t, entries, "restore temporary files must be removed on success and failure")
}

func restoreCallKinds(calls string) []string {
	var kinds []string
	for _, line := range strings.Split(strings.TrimSpace(calls), "\n") {
		kinds = append(kinds, strings.Fields(line)[0])
	}
	return kinds
}

const restoreDownloadStub = `#!/usr/bin/env bash
set -euo pipefail
while (($#)); do
  if [[ "$1" == --dest ]]; then dest=$2; shift 2; else shift; fi
done
case "$RESTORE_FIXTURE_MODE" in
  missing) exit 0 ;;
  empty) : > "$dest" ;;
  download-failure) echo 'fixture download failed' >&2; exit 17 ;;
  *) cp "$RESTORE_FIXTURE_ROOT/archive.gz" "$dest" ;;
esac
if [[ "$RESTORE_FIXTURE_MODE" == unreadable ]]; then chmod 000 "$dest"; fi
`

const restoreDockerStub = `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$RESTORE_FIXTURE_ROOT/container-calls"
case "$1" in
  exec) cat > "$RESTORE_FIXTURE_ROOT/ingested" ;;
  cp) cp "$2" "$RESTORE_FIXTURE_ROOT/ingested" ;;
  stop|start) ;;
  *) exit 64 ;;
esac
if [[ "$RESTORE_FIXTURE_MODE" == ingest-failure ]]; then
  echo 'fixture database rejected a statement' >&2
  exit 23
fi
`
