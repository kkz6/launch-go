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

// --- Mongo --db override ------------------------------------------------

func TestRunBackupScript_MongoDump_NoDatabase_OmitsDbFlag(t *testing.T) {
	// Empty Database → full-instance dump. The --db flag must NOT
	// appear in the rendered command. Regression guard: an earlier
	// implementation always passed cfg.Database to mongodump, so a
	// blank value rendered "--db=" which mongodump rejects.
	script := RunBackupScript(BackupRunConfig{
		RunID:         "01HZ",
		ContainerName: "launch-db-x",
		Engine:        dockertypes.DatabaseEngineMongo,
		Username:      "u",
		Password:      "p",
		Database:      "", // un-overridden
		Bucket:        "b",
		AccessKey:     "AK",
		SecretKey:     "SK",
	})
	if !strings.Contains(script, "mongodump") {
		t.Fatalf("mongo backup must invoke mongodump, got:\n%s", script)
	}
	if strings.Contains(script, "--db=") {
		t.Errorf("mongo dump without database override must not pass --db=, got:\n%s", script)
	}
}

func TestRunBackupScript_MongoDump_WithDatabase_AddsDbFlag(t *testing.T) {
	// Database override → mongodump targets that specific DB. Without
	// this filter the full-instance archive ignores the user's choice
	// — exactly the bug the database_name field was added to fix.
	script := RunBackupScript(BackupRunConfig{
		RunID:         "01HZ",
		ContainerName: "launch-db-x",
		Engine:        dockertypes.DatabaseEngineMongo,
		Username:      "u",
		Password:      "p",
		Database:      "analytics",
		Bucket:        "b",
		AccessKey:     "AK",
		SecretKey:     "SK",
	})
	if !strings.Contains(script, "--db=analytics") {
		t.Errorf("mongo dump with database=analytics must pass --db=analytics, got:\n%s", script)
	}
}

func TestRestoreBackupScript_Mongo_WithDatabase_AddsNsInclude(t *testing.T) {
	// Restore side mirrors backup side: when the config has a database
	// override the mongorestore command filters via --nsInclude.
	script := RestoreBackupScript(RestoreBackupConfig{
		RunID:         "01HZ",
		ContainerName: "launch-db-x",
		Engine:        dockertypes.DatabaseEngineMongo,
		Username:      "u",
		Password:      "p",
		Database:      "analytics",
		Bucket:        "b",
		ObjectKey:     "analytics/2026-01-01/01HZ.bson.gz",
		AccessKey:     "AK",
		SecretKey:     "SK",
	})
	// shellEscapeArg single-quotes the value because of the dot.
	// We check substring match either way (with or without quoting)
	// so a future change to the escape policy doesn't break this test.
	if !strings.Contains(script, "--nsInclude=analytics.*") &&
		!strings.Contains(script, "--nsInclude='analytics.*'") {
		t.Errorf("mongo restore with database=analytics must pass --nsInclude=analytics.*, got:\n%s", script)
	}
}

// --- Redis restore (the docker stop / cp / start flow) -----------------

func TestRestoreBackupScript_Redis_StopCopyStart(t *testing.T) {
	// Redis restore previously was a stub. The new flow stops the
	// container, copies the unzipped dump.rdb in via `docker cp`, and
	// starts the container back up. Pin those three commands so a
	// well-meaning refactor can't silently revert to the stub.
	script := RestoreBackupScript(RestoreBackupConfig{
		RunID:         "01HZ",
		ContainerName: "launch-db-redis",
		Engine:        dockertypes.DatabaseEngineRedis,
		Bucket:        "b",
		ObjectKey:     "cache/2026-01-01/01HZ.rdb.gz",
		AccessKey:     "AK",
		SecretKey:     "SK",
	})

	mustContain := []string{
		`docker stop "${CONTAINER}"`,
		`docker cp "${TMP_UNZIP}" "${CONTAINER}:/data/dump.rdb"`,
		`docker start "${CONTAINER}"`,
		`gunzip -c "${TMP_FILE}" > "${TMP_UNZIP}"`,
	}
	for _, want := range mustContain {
		if !strings.Contains(script, want) {
			t.Errorf("redis restore script missing %q\n----\n%s", want, script)
		}
	}
	// And make sure the stub error string is GONE — otherwise the
	// restore would still fail at runtime even though the new flow
	// would also be present.
	if strings.Contains(script, "redis restore is not yet supported") {
		t.Errorf("redis restore stub string must be removed")
	}
}

// --- PruneBackupObjects ------------------------------------------------

func TestPruneBackupObjectsScript_EmptyList_RendersEmpty(t *testing.T) {
	// No-op short-circuit: zero keys → empty script (no SSH call).
	// The service caller relies on this to skip dispatch when there's
	// nothing to prune.
	got := PruneBackupObjectsScript(PruneBackupObjectsConfig{
		Bucket:    "b",
		AccessKey: "AK",
		SecretKey: "SK",
	})
	if got != "" {
		t.Errorf("empty key list must render empty script, got:\n%s", got)
	}
}

func TestPruneBackupObjectsScript_RendersOneRmPerKey(t *testing.T) {
	got := PruneBackupObjectsScript(PruneBackupObjectsConfig{
		ObjectKeys: []string{
			"acme/2026-01-01/01HA.sql.gz",
			"acme/2026-01-02/01HB.sql.gz",
		},
		Bucket:    "mybucket",
		Endpoint:  "https://eu-central.contabostorage.com",
		AccessKey: "AK",
		SecretKey: "SK",
	})
	if !strings.Contains(got, `aws s3 rm "s3://mybucket/acme/2026-01-01/01HA.sql.gz"`) {
		t.Errorf("prune script missing rm for first key, got:\n%s", got)
	}
	if !strings.Contains(got, `aws s3 rm "s3://mybucket/acme/2026-01-02/01HB.sql.gz"`) {
		t.Errorf("prune script missing rm for second key, got:\n%s", got)
	}
	if !strings.Contains(got, "--endpoint-url=") {
		t.Errorf("prune script must propagate --endpoint-url when set, got:\n%s", got)
	}
	// `set -e` is DELIBERATELY absent so one missing key doesn't halt
	// the whole sweep. This is the exact "loop continues past per-key
	// failures" property that documents the best-effort semantic.
	if strings.Contains(got, "set -euo") {
		t.Errorf("prune script must use `set -uo pipefail`, NOT `set -euo` — one missing key shouldn't abort, got:\n%s", got)
	}
}

func TestPruneBackupObjectsScript_KeysAreLoggedViaMarkers(t *testing.T) {
	// The service caller doesn't currently parse these markers, but
	// they're the only audit trail of which keys we attempted to
	// delete — invaluable when debugging "the bucket has 200 objects
	// but only 50 run rows".
	got := PruneBackupObjectsScript(PruneBackupObjectsConfig{
		ObjectKeys: []string{"acme/2026-01-01/01HA.sql.gz"},
		Bucket:     "b",
		AccessKey:  "AK",
		SecretKey:  "SK",
	})
	if !strings.Contains(got, "::LAUNCH::prune_key::") {
		t.Errorf("prune script must emit per-key markers, got:\n%s", got)
	}
	if !strings.Contains(got, "::LAUNCH::prune_step::done") {
		t.Errorf("prune script must emit done marker, got:\n%s", got)
	}
}
