package services

import (
	"strings"
	"testing"

	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
)

// TestEngineCatalogue_AllEnginesPresent guards against an
// accidental removal in the engine map — the UI's create dialog
// depends on this returning the full set.
func TestEngineCatalogue_AllEnginesPresent(t *testing.T) {
	want := []dockertypes.DatabaseEngine{
		dockertypes.DatabaseEnginePostgres,
		dockertypes.DatabaseEngineMySQL,
		dockertypes.DatabaseEngineMariaDB,
		dockertypes.DatabaseEngineRedis,
		dockertypes.DatabaseEngineMongo,
	}
	for _, engine := range want {
		if _, ok := engineSpecFor(engine); !ok {
			t.Errorf("engine catalogue missing %q", engine)
		}
	}
}

// TestEngineCatalogue_CanonicalPorts pins the canonical service ports.
// These match the docker images' defaults — silently changing them
// would break every "internal-only" database the UI advertises.
func TestEngineCatalogue_CanonicalPorts(t *testing.T) {
	cases := map[dockertypes.DatabaseEngine]int{
		dockertypes.DatabaseEnginePostgres: 5432,
		dockertypes.DatabaseEngineMySQL:    3306,
		dockertypes.DatabaseEngineMariaDB:  3306,
		dockertypes.DatabaseEngineRedis:    6379,
		dockertypes.DatabaseEngineMongo:    27017,
	}
	for engine, port := range cases {
		spec, _ := engineSpecFor(engine)
		if spec.InternalPort != port {
			t.Errorf("%s internal port = %d, want %d", engine, spec.InternalPort, port)
		}
	}
}

// TestEngineCatalogue_EnvVarsIncludeCredentials makes sure each
// engine's EnvFn actually wires the credentials into the env list —
// a regression that left these empty would deploy an unauthenticated
// container, which is exactly the wrong direction for a "managed"
// database.
func TestEngineCatalogue_EnvVarsIncludeCredentials(t *testing.T) {
	creds := Credentials{Username: "u", Password: "secret-pw", Database: "appdb"}
	cases := map[dockertypes.DatabaseEngine][]string{
		dockertypes.DatabaseEnginePostgres: {"POSTGRES_USER=u", "POSTGRES_PASSWORD=secret-pw", "POSTGRES_DB=appdb"},
		dockertypes.DatabaseEngineMySQL:    {"MYSQL_ROOT_PASSWORD=secret-pw", "MYSQL_DATABASE=appdb", "MYSQL_USER=u", "MYSQL_PASSWORD=secret-pw"},
		dockertypes.DatabaseEngineMariaDB:  {"MARIADB_ROOT_PASSWORD=secret-pw", "MARIADB_USER=u"},
		dockertypes.DatabaseEngineMongo:    {"MONGO_INITDB_ROOT_USERNAME=u", "MONGO_INITDB_ROOT_PASSWORD=secret-pw"},
	}
	for engine, mustContain := range cases {
		spec, _ := engineSpecFor(engine)
		envs := spec.EnvFn(creds)
		joined := strings.Join(envs, "\n")
		for _, want := range mustContain {
			if !strings.Contains(joined, want) {
				t.Errorf("%s env vars missing %q; got %v", engine, want, envs)
			}
		}
	}
}

// Redis is the odd one out — auth doesn't ride in env vars. The job
// layer adds a --requirepass flag instead; the env list should be
// empty here so we don't accidentally double up.
func TestEngineCatalogue_RedisEnvIsEmpty(t *testing.T) {
	spec, _ := engineSpecFor(dockertypes.DatabaseEngineRedis)
	if got := spec.EnvFn(Credentials{Password: "x"}); len(got) != 0 {
		t.Errorf("redis EnvFn should return no env vars, got %v", got)
	}
}

// SupportedDatabaseEngines feeds the create-form catalogue. Each
// entry must have at least one supported version — an empty list
// would make the UI render an unselectable engine.
func TestSupportedDatabaseEngines_NonEmptyVersions(t *testing.T) {
	for engine, versions := range SupportedDatabaseEngines() {
		if len(versions) == 0 {
			t.Errorf("engine %s has zero supported versions", engine)
		}
	}
}
