package types

import (
	"database/sql/driver"
	"fmt"

	"github.com/kkz6/launch-go/internal/pkg/enumtypes"
)

// =============================================================================
// Kind: which engine the docker service runs (postgres / mysql / redis).
// =============================================================================

// Kind identifies the kind of docker service.
type Kind string

const (
	KindPostgres Kind = "postgres"
	KindMySQL    Kind = "mysql"
	KindRedis    Kind = "redis"
)

var allKinds = []Kind{KindPostgres, KindMySQL, KindRedis}

var kindLabels = map[Kind]string{
	KindPostgres: "PostgreSQL",
	KindMySQL:    "MySQL",
	KindRedis:    "Redis",
}

// String returns the wire string value.
func (k Kind) String() string { return string(k) }

// Label returns the user-facing label.
func (k Kind) Label() string { return enumtypes.Label(k, kindLabels, "Unknown") }

// IsValid reports whether the kind matches a known value.
func (k Kind) IsValid() bool { return enumtypes.IsValid(k, allKinds...) }

// DefaultImage returns the default Docker image:tag for this kind. The
// image is overridable via the install request; this is what we use
// when the caller does not supply one.
func (k Kind) DefaultImage() string {
	switch k {
	case KindPostgres:
		return "postgres:16"
	case KindMySQL:
		return "mysql:8.0"
	case KindRedis:
		return "redis:7-alpine"
	}
	return ""
}

// InternalPort is the port the engine listens on inside the container.
// Apps on launch-network reach the database at <containerName>:<this>.
func (k Kind) InternalPort() int {
	switch k {
	case KindPostgres:
		return 5432
	case KindMySQL:
		return 3306
	case KindRedis:
		return 6379
	}
	return 0
}

// RequiresDatabaseName is true for SQL engines (Postgres, MySQL) that
// initialise a named database on first start. Redis has no database
// concept at this layer (just AUTH).
func (k Kind) RequiresDatabaseName() bool {
	return k == KindPostgres || k == KindMySQL
}

// ContainerName returns the deterministic Docker container name we use
// on the host. Single-instance-per-server-per-kind, so the name is
// stable across installs.
func (k Kind) ContainerName() string {
	switch k {
	case KindPostgres:
		return "launch-postgres"
	case KindMySQL:
		return "launch-mysql"
	case KindRedis:
		return "launch-redis"
	}
	return ""
}

// VolumeName returns the named Docker volume that holds persistent data
// for this kind on a server.
func (k Kind) VolumeName() string {
	switch k {
	case KindPostgres:
		return "launch-postgres-data"
	case KindMySQL:
		return "launch-mysql-data"
	case KindRedis:
		return "launch-redis-data"
	}
	return ""
}

// RunTemplateName returns the bash template that launches a container
// of this kind. Templates live under server/tasks/templates.
func (k Kind) RunTemplateName() string {
	switch k {
	case KindPostgres:
		return "dockerservice/run_postgres.sh"
	case KindMySQL:
		return "dockerservice/run_mysql.sh"
	case KindRedis:
		return "dockerservice/run_redis.sh"
	}
	return ""
}

// Scan implements sql.Scanner for GORM.
func (k *Kind) Scan(value any) error { return enumtypes.ScanString(k, value) }

// Value implements driver.Valuer for GORM.
func (k Kind) Value() (driver.Value, error) { return enumtypes.ValueString(k) }

// ParseKind validates an input string and returns the matching Kind.
func ParseKind(s string) (Kind, error) {
	k := Kind(s)
	if !k.IsValid() {
		return "", fmt.Errorf("invalid docker service kind: %s", s)
	}
	return k, nil
}

// AllKinds returns every valid Kind.
func AllKinds() []Kind { return allKinds }

// =============================================================================
// Status: the lifecycle state of a docker service row.
// =============================================================================

// Status describes the current lifecycle phase of a docker service.
type Status string

const (
	// StatusPending — the row exists, the install job has not started.
	StatusPending Status = "pending"
	// StatusInstalling — install/uninstall job currently running.
	StatusInstalling Status = "installing"
	// StatusRunning — the container is up and accepting connections.
	StatusRunning Status = "running"
	// StatusStopped — the container is intentionally not running.
	StatusStopped Status = "stopped"
	// StatusFailed — a previous lifecycle op failed; LastError holds detail.
	StatusFailed Status = "failed"
)

var allStatuses = []Status{
	StatusPending, StatusInstalling, StatusRunning, StatusStopped, StatusFailed,
}

var statusLabels = map[Status]string{
	StatusPending:    "Pending",
	StatusInstalling: "Installing",
	StatusRunning:    "Running",
	StatusStopped:    "Stopped",
	StatusFailed:     "Failed",
}

// String returns the wire string value.
func (s Status) String() string { return string(s) }

// Label returns the user-facing label.
func (s Status) Label() string { return enumtypes.Label(s, statusLabels, "Unknown") }

// IsValid reports whether the status matches a known value.
func (s Status) IsValid() bool { return enumtypes.IsValid(s, allStatuses...) }

// IsTerminal reports whether the status represents a settled state
// (i.e. no in-progress job is currently advancing it).
func (s Status) IsTerminal() bool {
	return s == StatusRunning || s == StatusStopped || s == StatusFailed
}

// Scan implements sql.Scanner for GORM.
func (s *Status) Scan(value any) error { return enumtypes.ScanString(s, value) }

// Value implements driver.Valuer for GORM.
func (s Status) Value() (driver.Value, error) { return enumtypes.ValueString(s) }
