package tasks

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
	"github.com/stretchr/testify/require"
)

// Run with LAUNCH_TEST_MONGO_IMAGE=mongo:8.0. The disposable container has no
// network or exposed ports; only synthetic data and generated restore scripts
// are used. The docker shim invokes the real mongorestore inside that container.
func TestMongoRestoreGeneratedScriptIntegration(t *testing.T) {
	image := os.Getenv("LAUNCH_TEST_MONGO_IMAGE")
	if image == "" {
		t.Skip("set LAUNCH_TEST_MONGO_IMAGE to run isolated MongoDB restore validation")
	}
	container := fmt.Sprintf("launchctl-mongo-restore-%d", time.Now().UnixNano())
	run := func(input string, args ...string) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "docker", args...)
		cmd.Stdin = strings.NewReader(input)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	output, err := run("", "run", "--rm", "-d", "--network", "none", "--name", container, image, "--bind_ip", "127.0.0.1")
	require.NoError(t, err, output)
	t.Cleanup(func() { _, _ = run("", "rm", "-f", container) })
	ready := false
	for range 100 {
		if _, err := run("", "exec", container, "mongosh", "--quiet", "--eval", "db.runCommand({ping:1})"); err == nil {
			ready = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	require.True(t, ready, "local MongoDB did not become ready")
	seed := `db.getSiblingDB("admin").createUser({user:"fixture",pwd:"fixture-password",roles:["root"]});
db.getSiblingDB("production").items.insertOne({marker:"backup"});
db.getSiblingDB("production").createCollection("empty");
db.getSiblingDB("staging").items.insertOne({marker:"old-target"});
db.getSiblingDB("staging").extra.insertOne({marker:"keep-extra"});
db.getSiblingDB("unrelated").items.insertOne({marker:"keep-unrelated"});`
	output, err = run("", "exec", container, "mongosh", "--quiet", "--eval", seed)
	require.NoError(t, err, output)
	setup := `set -eu
mongodump --db=production --archive=/tmp/source.archive >/dev/null
gzip -c /tmp/source.archive > /tmp/source.archive.gz
mkdir -p /tmp/restore-bin
cat > /tmp/restore-bin/docker <<'SH'
#!/bin/sh
set -eu
[ "$1" = exec ]; shift
[ "$1" = -i ]; shift
[ "$1" = mongo-fixture ]; shift
exec "$@"
SH
cat > /tmp/restore-bin/launch-agent <<'SH'
#!/bin/sh
set -eu
while [ "$#" -gt 0 ]; do
  case "$1" in --dest) cp /tmp/source.archive.gz "$2"; exit 0;; esac
  shift
done
exit 1
SH
chmod +x /tmp/restore-bin/docker /tmp/restore-bin/launch-agent
`
	output, err = run(setup, "exec", "-i", container, "bash", "-s")
	require.NoError(t, err, output)
	check := func(js string) {
		t.Helper()
		out, err := run("", "exec", container, "mongosh", "--quiet", "--eval", js)
		require.NoError(t, err, out)
	}
	restore := func(source, target string) (string, error) {
		cfg := RestoreBackupConfig{Engine: dockertypes.DatabaseEngineMongo, ContainerName: "mongo-fixture", SourceDatabase: source, Database: target, Username: "fixture", Password: "fixture-password", ObjectKey: "synthetic.gz", Bucket: "fixture"}
		return run(RestoreBackupScript(cfg), "exec", "-i", "-e", "PATH=/tmp/restore-bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin", container, "bash", "-s")
	}
	output, err = restore("production", "staging")
	require.NoError(t, err, output)
	require.Contains(t, output, "::LAUNCH::restore_step::done")
	check(`if (db.getSiblingDB("staging").items.findOne().marker !== "backup" || db.getSiblingDB("staging").extra.countDocuments({}) !== 1 || !db.getSiblingDB("staging").getCollectionNames().includes("empty") || db.getSiblingDB("unrelated").items.findOne().marker !== "keep-unrelated") quit(1)`)
	check(`db.getSiblingDB("production").items.updateOne({},{$set:{marker:"changed-source"}})`)
	output, err = restore("production", "production")
	require.NoError(t, err, output)
	check(`if (db.getSiblingDB("production").items.findOne().marker !== "backup") quit(1)`)
	for _, source := range []string{"absent", ""} {
		output, err = restore(source, "staging")
		require.Error(t, err, output)
		require.NotContains(t, output, "::LAUNCH::restore_step::done")
		require.Contains(t, output, "before database ingestion")
		check(`if (db.getSiblingDB("staging").items.findOne().marker !== "backup" || db.getSiblingDB("unrelated").items.findOne().marker !== "keep-unrelated") quit(1)`)
	}
}
