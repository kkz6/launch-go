package services

import (
	"fmt"

	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
)

// engineSpec captures everything we need to know to spin up a database
// container for a given engine. Pure data — kept here so the deploy
// task script can render itself without touching DB models.
//
// Image is parameterised by version: the renderer composes
// "<image>:<engineVersion>" so a future engine change (e.g. moving to
// pg-bouncer-fronted Postgres) is a single struct edit.
type engineSpec struct {
	Image           string   // image name (no tag)
	DefaultVersion  string   // sensible LTS-ish tag
	SupportedVersions []string
	InternalPort    int      // the canonical port the daemon listens on
	// DataPath is the canonical in-container directory where the engine
	// keeps its on-disk state. The run-database script bind-mounts a
	// named volume here so the data survives container recreates
	// (expose-toggle, restart-policy change, image bump, rebuild).
	DataPath string
	// EnvFn renders the env-var args for `docker run`. Takes the
	// generated credentials so we never embed defaults in the catalogue
	// itself.
	EnvFn func(creds Credentials) []string
}

// Credentials are the auto-generated secrets that travel from the
// service layer into the run task. Mirrored into the encrypted DB
// column on the database row so we can show them in the UI later.
type Credentials struct {
	Username string
	Password string
	Database string
}

// engineCatalogue is the source of truth for which engines we support.
// Adding a new engine = one entry; the rest of the service layer keys
// off it via engineSpecFor.
var engineCatalogue = map[dockertypes.DatabaseEngine]engineSpec{
	dockertypes.DatabaseEnginePostgres: {
		Image:             "postgres",
		DefaultVersion:    "16",
		SupportedVersions: []string{"16", "15", "14"},
		InternalPort:      5432,
		DataPath:          "/var/lib/postgresql/data",
		EnvFn: func(c Credentials) []string {
			return []string{
				fmt.Sprintf("POSTGRES_USER=%s", c.Username),
				fmt.Sprintf("POSTGRES_PASSWORD=%s", c.Password),
				fmt.Sprintf("POSTGRES_DB=%s", c.Database),
			}
		},
	},
	dockertypes.DatabaseEngineMySQL: {
		Image:             "mysql",
		DefaultVersion:    "8.0",
		SupportedVersions: []string{"8.0", "8.4"},
		InternalPort:      3306,
		DataPath:          "/var/lib/mysql",
		EnvFn: func(c Credentials) []string {
			return []string{
				// MYSQL_ROOT_PASSWORD == app password keeps things simple
				// for a single-tenant managed DB. If we expose multi-user
				// management later, split these.
				fmt.Sprintf("MYSQL_ROOT_PASSWORD=%s", c.Password),
				fmt.Sprintf("MYSQL_DATABASE=%s", c.Database),
				fmt.Sprintf("MYSQL_USER=%s", c.Username),
				fmt.Sprintf("MYSQL_PASSWORD=%s", c.Password),
			}
		},
	},
	dockertypes.DatabaseEngineMariaDB: {
		Image:             "mariadb",
		DefaultVersion:    "11",
		SupportedVersions: []string{"11", "10.11"},
		InternalPort:      3306,
		DataPath:          "/var/lib/mysql",
		EnvFn: func(c Credentials) []string {
			return []string{
				fmt.Sprintf("MARIADB_ROOT_PASSWORD=%s", c.Password),
				fmt.Sprintf("MARIADB_DATABASE=%s", c.Database),
				fmt.Sprintf("MARIADB_USER=%s", c.Username),
				fmt.Sprintf("MARIADB_PASSWORD=%s", c.Password),
			}
		},
	},
	dockertypes.DatabaseEngineRedis: {
		Image:             "redis",
		DefaultVersion:    "7",
		SupportedVersions: []string{"7", "6"},
		InternalPort:      6379,
		DataPath:          "/data",
		EnvFn: func(_ Credentials) []string {
			// Redis doesn't honour env vars for auth; the run task adds
			// `redis-server --requirepass <password>` instead. EnvFn
			// stays empty so the deploy script's env loop is empty.
			return nil
		},
	},
	dockertypes.DatabaseEngineMongo: {
		Image:             "mongo",
		DefaultVersion:    "7",
		SupportedVersions: []string{"7", "6"},
		InternalPort:      27017,
		DataPath:          "/data/db",
		EnvFn: func(c Credentials) []string {
			return []string{
				fmt.Sprintf("MONGO_INITDB_ROOT_USERNAME=%s", c.Username),
				fmt.Sprintf("MONGO_INITDB_ROOT_PASSWORD=%s", c.Password),
			}
		},
	},
}

// engineSpecFor returns the spec or an error. Service callers branch on
// the error to return a 400 to the user rather than a 500 — the engine
// string is validated at the request layer too, but defence-in-depth.
func engineSpecFor(engine dockertypes.DatabaseEngine) (engineSpec, bool) {
	s, ok := engineCatalogue[engine]
	return s, ok
}

// SupportedDatabaseEngines returns the engines + versions the UI can
// offer. Helps the create-dialog form stay in sync with the backend
// without an extra config endpoint.
func SupportedDatabaseEngines() map[string][]string {
	out := make(map[string][]string, len(engineCatalogue))
	for engine, spec := range engineCatalogue {
		out[string(engine)] = append([]string(nil), spec.SupportedVersions...)
	}
	return out
}
