# Managed Services on Docker Servers — Design

**Date:** 2026-04-27
**Branch:** `feat/docker-provisioning` (continuing)
**Status:** approved for implementation
**Predecessor:** `2026-04-27-docker-server-type-design.md`

## Goal

A Docker-typed server can host **at most one** managed Postgres or MySQL
database **and** at most one Redis instance, each running as a Docker
container on `launch-network`. Users install/manage them from the
server's index page exactly the way PHP servers expose database
management — but engine-aware and single-instance-per-kind.

## Scope (v1)

**In:**
- Three kinds: `postgres`, `mysql`, `redis`
- Lifecycle: install, show, uninstall, start, stop, restart, logs, regenerate password, toggle external port
- Install + uninstall are async via the existing job queue (broadcast progress on `managed_service.status`)
- Other ops are sync (1–3 s SSH round-trip)
- Bounded log fetch via `GET /managed/:kind/logs?tail=N`

**Out (later PRs):**
- Reset (wipe + reinstall)
- Image-tag updates (postgres:16 → :17)
- Backups (separate concern, separate PR)
- WebSocket log streaming
- MariaDB / MongoDB
- Auto-injection of connection strings into user-deployed apps

## Architecture

### Module location

New module: `internal/modules/managedservice/`. Mirrors the existing
module skeleton (`module.go`, `routes.go`, `dto/`, `services/`,
`handlers/`, `jobs/`, `tasks/`, `models/`, `repositories/`,
`contracts/`, `types/`).

### Data model

Single table `managed_services` with one row per (server, kind):

```
id                ULID, primary key
team_id           ULID, FK
server_id         ULID, FK            ← scopes to a server
kind              enum (postgres|mysql|redis)
status            enum (pending|installing|running|stopped|failed)
image             string              ← e.g. "postgres:16"
container_name    string              ← e.g. "launch-postgres"
volume_name       string              ← e.g. "launch-postgres-data"
database_name     string              ← postgres/mysql only
db_username       string              ← postgres/mysql only
db_password       encrypted_string
external_port     int (nullable)      ← null = internal-only
internal_port     int                 ← 5432 / 3306 / 6379
last_error        string (nullable)
installed_at      timestamp (nullable)
created_at        timestamp
updated_at        timestamp

UNIQUE (server_id, kind)
```

`(server_id, kind)` unique index enforces "at most one of each kind per
server." Service-level pre-check returns 409 with a helpful message
rather than letting the DB conflict bubble.

### Types (`types/types.go`)

```go
type Kind string
const (
    KindPostgres Kind = "postgres"
    KindMySQL    Kind = "mysql"
    KindRedis    Kind = "redis"
)

type Status string
const (
    StatusPending    Status = "pending"
    StatusInstalling Status = "installing"
    StatusRunning    Status = "running"
    StatusStopped    Status = "stopped"
    StatusFailed     Status = "failed"
)
```

Each `Kind` carries:
- `DefaultImage()`: "postgres:16", "mysql:8.0", "redis:7-alpine"
- `InternalPort()`: 5432, 3306, 6379
- `RequiresDatabaseName() bool`: postgres/mysql yes, redis no
- `ContainerName()`: "launch-postgres", "launch-mysql", "launch-redis"
- `VolumeName()`: "launch-postgres-data", etc.
- `RunTemplateName()`: "managedservice/run_postgres.sh", etc.

### Routes

Mounted under the server group, `MustParseEnum` validates `:kind`:

```
GET    /servers/:serverId/managed/:kind                ShowNested
POST   /servers/:serverId/managed/:kind                CreateNested (async install)
DELETE /servers/:serverId/managed/:kind                DeleteNested (async uninstall)
POST   /servers/:serverId/managed/:kind/start          ActionNested
POST   /servers/:serverId/managed/:kind/stop           ActionNested
POST   /servers/:serverId/managed/:kind/restart        ActionNested
POST   /servers/:serverId/managed/:kind/regenerate     ActionNested with body
POST   /servers/:serverId/managed/:kind/external-port  ActionNested with body (port int, null to disable)
GET    /servers/:serverId/managed/:kind/logs           bespoke (query param: tail)
```

`:kind` is read with `MustParseEnum(c, "kind", "Invalid service kind",
mstypes.ParseKind)`. Nine routes total, all team-scoped, all
provisioned-server middleware'd.

### Lifecycle

#### Install (async)
1. Service: validate request, check uniqueness, generate password,
   insert row with `status=pending`, dispatch
   `InstallManagedServiceJob`. Return 202.
2. Job: update row to `status=installing`, broadcast event, run the
   engine-specific run-script over SSH (pulls image, creates volume,
   `docker run -d` on `launch-network` with env + optional `-p`).
3. On success: mark `status=running`, set `installed_at`, broadcast.
4. On failure: mark `status=failed`, set `last_error`, broadcast.

#### Uninstall (async)
1. Service: mark `status=installing` (reused as "in-progress"),
   dispatch `UninstallManagedServiceJob`.
2. Job: `docker rm -f`, `docker volume rm`, delete row, broadcast.

#### Sync ops
- **Show**: row → DTO with computed `dsn` field
- **Start/Stop/Restart**: `docker {start,stop,restart} <container>`
- **Logs**: `docker logs --tail=<N> <container>` over SSH; returned as
  `{logs: "..."}` JSON
- **Regenerate password**: `ALTER USER ... PASSWORD '...'` for SQL,
  `redis-cli CONFIG SET requirepass '...'` for Redis, then update DB
- **External port toggle**: stop + remove + re-run with/without `-p`

### Run scripts

Three templates under
`internal/modules/managedservice/tasks/templates/managedservice/`:

```
run_postgres.sh   → docker pull + docker run -d --name launch-postgres
                    --network launch-network -v launch-postgres-data:/var/lib/postgresql/data
                    -e POSTGRES_DB/USER/PASSWORD [-p :5432]
run_mysql.sh      → similar with mysql:8.0 + MYSQL_* env
run_redis.sh      → docker run -d --name launch-redis
                    --network launch-network -v launch-redis-data:/data
                    [-p :6379] redis:7-alpine redis-server --requirepass <pwd>
```

Each starts with `docker rm -f <name> 2>/dev/null || true` so re-running
is idempotent.

### DTO

```go
type CreateManagedServiceRequest struct {
    DatabaseName string `json:"database_name,omitempty"`  // postgres/mysql only
    Username     string `json:"username,omitempty"`       // postgres/mysql only
    Image        string `json:"image,omitempty"`          // override default
    ExternalPort *int   `json:"external_port,omitempty"`  // null = internal-only
}

type ManagedServiceResponse struct {
    ID            string  `json:"id"`
    Kind          string  `json:"kind"`
    Status        string  `json:"status"`
    Image         string  `json:"image"`
    ContainerName string  `json:"container_name"`
    InternalPort  int     `json:"internal_port"`
    ExternalPort  *int    `json:"external_port,omitempty"`
    DatabaseName  string  `json:"database_name,omitempty"`
    Username      string  `json:"username,omitempty"`
    Password      string  `json:"password"`         // decrypted on Show
    DSN           string  `json:"dsn"`              // pre-built connection string
    InstalledAt   *string `json:"installed_at,omitempty"`
    LastError     string  `json:"last_error,omitempty"`
}

type ToggleExternalPortRequest struct {
    Port *int `json:"port"` // null = disable, otherwise the host port
}
```

DSN computation:
- Postgres: `postgres://user:pass@<host>:<port>/db` (host = server IP if
  external port set, else `launch-postgres` for in-cluster apps)
- MySQL: `mysql://user:pass@<host>:<port>/db`
- Redis: `redis://:pass@<host>:<port>` (no user)

### Tests

Layer 1 — types unit: `Kind` validity, `DefaultImage`, `InternalPort`,
`RequiresDatabaseName`. `Status` validity. `ParseKind`.

Layer 2 — model + repo: in-memory recorder for Create/Find/Update/
Delete with the unique-by-(server,kind) constraint. Sqlite-backed test
proves the migration's unique index works.

Layer 3 — service unit: in-memory repo + recorder for the SSH runner.
Tests:
- Install rejects duplicate (server, kind)
- Install dispatches job and inserts row with correct defaults
- Install of postgres without database_name uses default `launch`
- Install of redis ignores database_name field
- Show wraps row + DSN
- Start/Stop/Restart issue the right `docker` command
- Logs uses correct `--tail` value
- RegeneratePassword runs the right engine-specific exec
- ToggleExternalPort restarts container with the right port flag
- Uninstall dispatches job

Layer 4 — task templates (snapshot tests in `tests/tasks/`):
- `RunPostgres` script contains the right env vars, mounts, volume
  flag, no published port when externalPort=0, `-p` when externalPort > 0
- `RunMySQL` and `RunRedis` analogues

Layer 5 — handler/route: integration test through `app.Test()` for one
end-to-end happy path (install postgres on a fake server, returns 202).

### Module wiring

Register in `cmd/api/main.go` and `cmd/worker/main.go`. Cross-module
dep on the server module is read-only (we need to know server IP for
DSN computation): use a small `ServerReader` adapter analogous to the
existing one.

## Files (rough estimate)

```
internal/modules/managedservice/
  module.go
  routes.go
  contracts/services.go
  contracts/repositories.go
  dto/requests.go
  dto/responses.go
  models/managed_service.go
  repositories/managed_service_repository.go
  repositories/registry.go
  services/base.go
  services/managed_service_service.go
  services/managed_service_service_test.go
  jobs/install.go
  jobs/uninstall.go
  jobs/register.go
  jobs/payloads.go
  tasks/run.go
  tasks/templates/managedservice/run_postgres.sh
  tasks/templates/managedservice/run_mysql.sh
  tasks/templates/managedservice/run_redis.sh
  types/types.go
  types/types_test.go

internal/migrations/
  20260427_managed_services.sql

tests/tasks/
  managedservice_run_test.go

cmd/api/main.go                  ← register module
cmd/worker/main.go               ← register jobs
```

Roughly 25 files, 1500–2000 LOC including tests.
