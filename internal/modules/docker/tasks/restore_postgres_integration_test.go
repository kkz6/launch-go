package tasks

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
)

// LAUNCH_TEST_POSTGRES_IMAGE=pgvector/pgvector:pg16 go test ./internal/modules/docker/tasks -run TestRestoreRealPostgres -v
// The fixture starts its own PostgreSQL inside one disposable container, with
// no network access or published ports and no connection to existing databases.
func TestRestoreRealPostgres(t *testing.T) {
	image := os.Getenv("LAUNCH_TEST_POSTGRES_IMAGE")
	if image == "" {
		t.Skip("set LAUNCH_TEST_POSTGRES_IMAGE to validate restore against an isolated PostgreSQL instance")
	}
	root := t.TempDir()
	script := RestoreBackupScript(RestoreBackupConfig{
		RunID: "postgres-fixture", ContainerName: "selected-container", Engine: dockertypes.DatabaseEnginePostgres,
		Username: "postgres", Password: "fixture-only", Database: "restore_target",
		ObjectKey: "snapshot.gz", Bucket: "fixture-bucket",
	})
	require.NoError(t, os.WriteFile(filepath.Join(root, "restore.sh"), []byte(script), 0o600))
	for name, sql := range map[string]string{
		"valid":   "CREATE TABLE restored_data (id integer PRIMARY KEY);\nINSERT INTO restored_data VALUES (42);\n",
		"invalid": "CREATE TABLE partial_changes (id integer);\nINSERT INTO partial_changes VALUES (1);\nTHIS IS INVALID SQL;\nINSERT INTO partial_changes VALUES (2);\n",
	} {
		var compressed bytes.Buffer
		writer := gzip.NewWriter(&compressed)
		_, err := writer.Write([]byte(sql))
		require.NoError(t, err)
		require.NoError(t, writer.Close())
		require.NoError(t, os.WriteFile(filepath.Join(root, name+".gz"), compressed.Bytes(), 0o600))
	}
	name := fmt.Sprintf("launch-restore-postgres-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		// Only the uniquely named fixture can be removed, including after a timeout.
		_ = exec.CommandContext(ctx, "docker", "rm", "-f", name).Run()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm", "--name", name, "--network", "none", "-i",
		"--mount", "type=bind,source="+root+",target=/fixture,readonly", image, "bash", "-s")
	cmd.Stdin = strings.NewReader(postgresRestoreFixture)
	output, err := cmd.CombinedOutput()
	require.NoError(t, ctx.Err(), "PostgreSQL fixture timed out: %s", output)
	require.NoError(t, err, "PostgreSQL restore fixture failed: %s", output)
	require.Contains(t, string(output), "POSTGRES_RESTORE_VALIDATION_PASSED")
	t.Log(string(output))
}

const postgresRestoreFixture = `set -euo pipefail
export PATH="/usr/lib/postgresql/${PG_MAJOR}/bin:$PATH"
export PGHOST=/tmp
install -d -m 700 -o postgres -g postgres /tmp/restore-pgdata
runuser -u postgres -- initdb -D /tmp/restore-pgdata --no-locale --auth=trust >/dev/null
runuser -u postgres -- pg_ctl -D /tmp/restore-pgdata -l /tmp/restore-pg.log -o "-k /tmp -h ''" -w start >/dev/null
trap 'runuser -u postgres -- pg_ctl -D /tmp/restore-pgdata -m immediate stop >/dev/null 2>&1 || true' EXIT
createdb -U postgres restore_target
createdb -U postgres unrelated
psql -X -U postgres -d unrelated -c 'CREATE TABLE untouched (id integer); INSERT INTO untouched VALUES (99);' >/dev/null
mkdir -p /tmp/restore-bin
cat > /tmp/restore-bin/launch-agent <<'SCRIPT'
#!/usr/bin/env bash
set -euo pipefail
while (($#)); do
  if [[ "$1" == --dest ]]; then dest=$2; shift 2; else shift; fi
done
cp "$RESTORE_ARCHIVE" "$dest"
SCRIPT
cat > /tmp/restore-bin/docker <<'SCRIPT'
#!/usr/bin/env bash
set -euo pipefail
test "$1" = exec
shift
while (($#)); do
  case "$1" in
    -i) shift ;;
    -e) export "$2"; shift 2 ;;
    *) test "$1" = selected-container; shift; break ;;
  esac
done
exec "$@"
SCRIPT
chmod 755 /tmp/restore-bin/*
export PATH="/tmp/restore-bin:$PATH"
# Without psql -X this startup file fails before a valid restore can run.
echo 'SELECT 1/0;' > /tmp/hostile.psqlrc
export PSQLRC=/tmp/hostile.psqlrc
export RESTORE_ARCHIVE=/fixture/valid.gz
bash /fixture/restore.sh > /tmp/valid-restore.log 2>&1
grep -q '::LAUNCH::restore_step::done' /tmp/valid-restore.log
test "$(psql -X -A -t -U postgres -d restore_target -c 'SELECT id FROM restored_data')" = 42
test -z "$(find /var/lib/launch/restore -mindepth 1 -print -quit)"
export RESTORE_ARCHIVE=/fixture/invalid.gz
if bash /fixture/restore.sh > /tmp/invalid-restore.log 2>&1; then
  cat /tmp/invalid-restore.log >&2
  echo 'Invalid SQL falsely reported successful restore' >&2
  exit 1
fi
if grep -q '::LAUNCH::restore_step::done' /tmp/invalid-restore.log; then exit 1; fi
grep -qi 'partial' /tmp/invalid-restore.log
# Fail-fast is not atomic: earlier writes remain, but later SQL must not run.
test "$(psql -X -A -t -U postgres -d restore_target -c 'SELECT string_agg(id::text, chr(44)) FROM partial_changes')" = 1
test "$(psql -X -A -t -U postgres -d unrelated -c 'SELECT id FROM untouched')" = 99
test -z "$(find /var/lib/launch/restore -mindepth 1 -print -quit)"
echo POSTGRES_RESTORE_VALIDATION_PASSED
`
