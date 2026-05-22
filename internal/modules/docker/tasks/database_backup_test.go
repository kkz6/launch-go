package tasks

import (
	"strings"
	"testing"

	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
)

func TestComposeObjectKey_Layout(t *testing.T) {
	// The format ":path/<database>/<yyyy-mm-dd>/<run>.<ext>.gz" is what
	// the UI promises in the docs. Lock the slash count + ext per engine.
	cases := []struct {
		engine  dockertypes.DatabaseEngine
		extWant string
	}{
		{dockertypes.DatabaseEnginePostgres, ".sql.gz"},
		{dockertypes.DatabaseEngineMySQL, ".sql.gz"},
		{dockertypes.DatabaseEngineMariaDB, ".sql.gz"},
		{dockertypes.DatabaseEngineMongo, ".bson.gz"},
		{dockertypes.DatabaseEngineRedis, ".rdb.gz"},
	}
	for _, c := range cases {
		key := composeObjectKey(BackupRunConfig{
			RunID:    "01HZ",
			Engine:   c.engine,
			Database: "acme",
		})
		if !strings.HasSuffix(key, c.extWant) {
			t.Errorf("%s key %q doesn't end with %q", c.engine, key, c.extWant)
		}
		// With no path prefix the key starts with the database name.
		if !strings.HasPrefix(key, "acme/") {
			t.Errorf("%s key %q must lead with database name when prefix is empty", c.engine, key)
		}
		if !strings.Contains(key, "/01HZ.") {
			t.Errorf("%s key %q missing run id", c.engine, key)
		}
	}
}

func TestComposeObjectKey_RespectsPrefix(t *testing.T) {
	// Both with and without trailing slashes should produce the same
	// canonical path — easy footgun to leave dangling slashes.
	cfg := BackupRunConfig{
		RunID:      "01HZ",
		Engine:     dockertypes.DatabaseEnginePostgres,
		Database:   "acme",
		PathPrefix: "/prod/databases/",
	}
	got := composeObjectKey(cfg)
	if !strings.HasPrefix(got, "prod/databases/") {
		t.Errorf("expected key prefix prod/databases/, got %q", got)
	}
	if strings.HasPrefix(got, "/") {
		t.Errorf("S3 keys should not start with a slash, got %q", got)
	}
}

func TestRunBackupScript_AWSCLISanityCheck(t *testing.T) {
	// We exit early if the aws CLI isn't installed. Pin the marker
	// since the worker's failure handler keys off it.
	script := RunBackupScript(BackupRunConfig{
		RunID:         "01HZ",
		ContainerName: "launch-db-x",
		Engine:        dockertypes.DatabaseEnginePostgres,
		Database:      "acme",
		Bucket:        "test-bucket",
		AccessKey:     "AK",
		SecretKey:     "SK",
	})
	if !strings.Contains(script, `command -v aws`) {
		t.Errorf("backup script must guard on aws CLI presence")
	}
	if !strings.Contains(script, "::LAUNCH::object_key::") {
		t.Errorf("backup script must emit object_key marker")
	}
	if !strings.Contains(script, "::LAUNCH::size_bytes::") {
		t.Errorf("backup script must emit size_bytes marker")
	}
}

func TestRunBackupScript_PgDumpUsesPGPassword(t *testing.T) {
	// PGPASSWORD has to be set on the `docker exec` invocation, not the
	// containing shell — otherwise other commands in the script could
	// inherit it.
	script := RunBackupScript(BackupRunConfig{
		RunID:         "01HZ",
		ContainerName: "launch-db-x",
		Engine:        dockertypes.DatabaseEnginePostgres,
		Username:      "u",
		Password:      "p",
		Database:      "d",
		Bucket:        "b",
		AccessKey:     "AK",
		SecretKey:     "SK",
	})
	if !strings.Contains(script, "docker exec -e PGPASSWORD=") {
		t.Errorf("pg backup must inject PGPASSWORD via docker exec, got:\n%s", script)
	}
}

func TestRestoreBackupScript_PerEngineIngest(t *testing.T) {
	cases := []struct {
		engine dockertypes.DatabaseEngine
		expect string
	}{
		{dockertypes.DatabaseEnginePostgres, "psql"},
		{dockertypes.DatabaseEngineMySQL, "mysql -u"},
		{dockertypes.DatabaseEngineMariaDB, "mysql -u"},
		{dockertypes.DatabaseEngineMongo, "mongorestore"},
	}
	for _, c := range cases {
		script := RestoreBackupScript(RestoreBackupConfig{
			RunID:         "01HZ",
			ContainerName: "launch-db-x",
			Engine:        c.engine,
			Username:      "u",
			Password:      "p",
			Database:      "d",
			Bucket:        "b",
			ObjectKey:     "acme/2026-01-01/01HZ.sql.gz",
			AccessKey:     "AK",
			SecretKey:     "SK",
		})
		if !strings.Contains(script, c.expect) {
			t.Errorf("%s restore script missing %q\n----\n%s", c.engine, c.expect, script)
		}
		if !strings.Contains(script, "gunzip -c") {
			t.Errorf("%s restore script must pipe through gunzip", c.engine)
		}
	}
}

func TestRestoreBackupScript_RedisRejected(t *testing.T) {
	// Redis restore via mongorestore-equivalent doesn't exist — the
	// .rdb file swap needs a container stop+start dance we haven't
	// shipped. Confirm the script explicitly fails rather than running
	// some "best-effort" cargo-culted command.
	script := RestoreBackupScript(RestoreBackupConfig{
		Engine: dockertypes.DatabaseEngineRedis,
	})
	if !strings.Contains(script, "redis restore is not yet supported") {
		t.Errorf("redis restore must explicitly say it's unsupported, got:\n%s", script)
	}
	if !strings.Contains(script, "exit 1") {
		t.Errorf("redis restore script must exit 1")
	}
}
