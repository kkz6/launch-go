package tasks

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMongoRestoreMapsOnlyRecordedSourceToSelectedTarget(t *testing.T) {
	cfg := RestoreBackupConfig{ContainerName: "mongo-test", Username: "user'quoted", Password: "password'quoted", SourceDatabase: "production", Database: "staging"}
	command := mongoRestoreIngestCommand(cfg)
	require.Contains(t, command, "--nsInclude='production.*'")
	require.Contains(t, command, "--nsFrom='production.*'")
	require.Contains(t, command, "--nsTo='staging.*'")
	require.Contains(t, command, "--stopOnError --drop")
	require.Contains(t, command, shellEscapeArg(cfg.Password))
	require.NotContains(t, mongoRestorePreflight(cfg), "--drop")
	require.Contains(t, mongoRestorePreflight(cfg), "--dryRun --verbose")
	for _, invalid := range []string{"", "*", "prod.*", "prod$target", "admin", "config", "local", "prod\nother", strings.Repeat("a", 64)} {
		cfg.SourceDatabase = invalid
		require.Error(t, ValidateMongoRestoreDatabases(cfg.SourceDatabase, cfg.Database), invalid)
		require.NotContains(t, mongoRestorePreflight(cfg), "docker exec", invalid)
	}
	cfg.SourceDatabase = "production"
	cfg.Database = "staging.*"
	require.Error(t, ValidateMongoRestoreDatabases(cfg.SourceDatabase, cfg.Database))
	require.NotContains(t, mongoRestoreIngestCommand(cfg), "docker exec")
}
