# MySQL to PostgreSQL Migration Plan

## Status: Planned

## Summary

Migrate the database layer from MySQL to PostgreSQL to leverage JSONB indexing, LISTEN/NOTIFY for WebSocket events, partial indexes for multi-tenant scoping, HOT updates for status fields, and native advanced query features (DISTINCT ON, LATERAL joins, advisory locks).

---

## Current State

- **Driver**: MySQL (configurable via `DB_DRIVER` in config)
- **ORM**: GORM with generic repository layer
- **Primary keys**: ULID (CHAR 26) across all tables
- **JSON columns**: 15+ columns storing metadata, features, provider data, credentials
- **Encrypted fields**: AES-256-GCM via custom serializer (driver-agnostic)
- **Connection pool**: 10 idle, 100 max, 1h lifetime
- **Multi-tenancy**: team_id scoping on all relevant tables

### Driver-Specific Code Locations

MySQL-specific (must be changed):
- `internal/database/migrations/migrator.go:201` — `SHOW TABLES`
- `internal/database/migrations/migrator.go:206,219` — `SET FOREIGN_KEY_CHECKS = 0/1`

PostgreSQL-specific (already present):
- `internal/modules/site/repositories/deployment_repository.go:74` — `DISTINCT ON (site_id)`

---

## Why Migrate

### 1. JSONB Indexing (High Impact)

**Affected columns across all modules:**

| Model | JSON Columns |
|-------|-------------|
| Server | `ProviderData`, `CompletedProvisionSteps` |
| Site | `TypeData`, `VcsData`, `Aliases`, `Features`, `EnabledFeatures`, `PendingFeatures`, `SharedDirectories`, `WriteableDirectories`, `SharedFiles` |
| Deployment | `CommitData`, `VcsData` |
| Backup | `IncludeFiles`, `ExcludeFiles` |
| StorageProvider | `Credentials` |
| SourceControl | `ProviderData` |
| NotificationChannel | `Data` |
| Subscription/Plan | Pricing data |

**MySQL limitation**: JSON columns cannot be directly indexed. Querying inside JSON requires `JSON_EXTRACT()` with full table scans, or generated columns with explicit indexes.

**PostgreSQL advantage**: JSONB columns support GIN indexes. Queries like `WHERE provider_data @> '{"region": "us-east-1"}'` use the index directly — O(log n) vs O(n).

**Example migration benefit:**
```sql
-- Create GIN index on server provider data
CREATE INDEX idx_servers_provider_data ON servers USING GIN (provider_data);

-- Query becomes index-backed
SELECT * FROM servers WHERE provider_data @> '{"region": "us-east-1"}' AND team_id = ?;
```

### 2. LISTEN/NOTIFY for WebSocket Events (High Impact)

**Current WebSocket events** (would need Redis pub/sub with MySQL):
- `server.status` — Server status changes
- `server.progress` — Provisioning progress
- `deployment.started` / `deployment.progress` / `deployment.log` / `deployment.finished` / `deployment.failed`
- `site.updated` — Site configuration changes

**PostgreSQL alternative**: Native pub/sub without external dependency:
```sql
-- Publisher (in task callback)
SELECT pg_notify('deployment_status', json_build_object(
    'deployment_id', '01HXYZ...',
    'status', 'finished',
    'team_id', '01HABC...'
)::text);

-- Go listener pushes to WebSocket hub
listener.Listen("deployment_status")
```

Eliminates Redis as a required dependency for real-time features.

### 3. Concurrent Status Updates — HOT Updates (High Impact)

**Frequently updated columns:**
- `tasks.status`, `tasks.output`, `tasks.exit_code` — During SSH script execution
- `deployments.status` — pending → installing → finished/failed
- `servers.connected`, `servers.last_connectivity_check` — Health checks
- `daemons.status`, `queues.status` — Background process monitoring
- `servers.progress`, `servers.progress_step` — Provisioning progress

**MySQL behavior**: Every UPDATE acquires row lock + writes undo log. Status index updates cause additional I/O.

**PostgreSQL HOT updates**: When the updated column has no index, PostgreSQL updates the row in-place on the same heap page without touching indexes. For status columns that aren't indexed (or where the index isn't on the updated column), this is significantly faster.

### 4. Partial Indexes for Multi-Tenant Scoping (Medium Impact)

**Query pattern** (every repository uses this):
```go
r.DB.Where("team_id = ? AND archived_at IS NULL", teamID)
```

**MySQL**: Full index on `(team_id, archived_at)` stores ALL rows including archived.

**PostgreSQL partial index**:
```sql
CREATE INDEX idx_servers_team_active ON servers (team_id) WHERE archived_at IS NULL;
CREATE INDEX idx_sites_team_active ON sites (team_id) WHERE archived_at IS NULL;
CREATE INDEX idx_deployments_site_active ON deployments (site_id) WHERE status IN ('pending', 'installing');
```

If 80% of servers/sites are archived over time, the partial index is 80% smaller, fits in memory better, and scans faster.

### 5. DISTINCT ON for Latest-Per-Group Queries (Medium Impact)

**Already needed** in `deployment_repository.go:74`:
```go
// FindLatestBySiteIDs returns the latest deployment for each site
// Currently uses PostgreSQL-specific DISTINCT ON
r.DB.Raw(`SELECT DISTINCT ON (site_id) * FROM deployments
    WHERE site_id IN ? ORDER BY site_id, created_at DESC`, siteIDs)
```

**MySQL equivalent** requires a subquery + join (slower, more complex):
```sql
SELECT d.* FROM deployments d
INNER JOIN (
    SELECT site_id, MAX(created_at) as max_created
    FROM deployments WHERE site_id IN (?) GROUP BY site_id
) latest ON d.site_id = latest.site_id AND d.created_at = latest.max_created
```

This pattern is needed anywhere you want "latest X per group" — latest backup job per backup, latest metric per server, etc.

### 6. Advisory Locks for Deployment Serialization (Medium Impact)

**Problem**: Two deployments for the same site should not run concurrently.

**MySQL**: `SELECT GET_LOCK('deploy:site:01HXYZ', 30)` — string-based, single namespace, limited scalability.

**PostgreSQL**:
```sql
SELECT pg_advisory_lock(hashtext('deploy:' || site_id));
-- or use the ULID hash as bigint for faster numeric locks
```

Advisory locks in PostgreSQL are faster (integer-based), support try-lock semantics, and auto-release on disconnect.

### 7. LATERAL Joins (Medium Impact)

**Use case**: Get all sites with their latest deployment:

**PostgreSQL:**
```sql
SELECT s.*, d.* FROM sites s
LEFT JOIN LATERAL (
    SELECT * FROM deployments WHERE site_id = s.id ORDER BY created_at DESC LIMIT 1
) d ON true
WHERE s.team_id = ?;
```

**MySQL**: Requires subquery in SELECT or two-step join. LATERAL is supported in MySQL 8.0.14+ but less commonly used.

### 8. Native Array Columns (Low Impact, Optional)

**Columns that could use PostgreSQL arrays instead of JSON:**
- `sites.aliases` — `text[]` instead of `JSONStringSlice`
- `sites.features` — `text[]`
- `sites.shared_directories` — `text[]`
- `sites.writeable_directories` — `text[]`
- `sites.shared_files` — `text[]`
- `servers.completed_provision_steps` — `text[]`

**Benefit**: `ANY()` operator, GIN index on array contains, no JSON parse overhead.

---

## Performance Comparison for Key Patterns

### Pattern 1: Dashboard — All Servers with Status

```
MySQL:   SELECT * FROM servers WHERE team_id = ? AND archived_at IS NULL ORDER BY created_at DESC
PgSQL:   Same query, but partial index (team_id WHERE archived_at IS NULL) = smaller index scan
```
**Expected gain**: 20-40% faster on tables with many archived rows.

### Pattern 2: Deployment List with Commit Data Query

```
MySQL:   Full scan on JSON column for any filter on commit_data
PgSQL:   GIN-indexed JSONB query
```
**Expected gain**: 10-100x faster for JSON-filtered queries (depends on table size).

### Pattern 3: Concurrent Deployment Status Updates

```
MySQL:   Row lock + undo log + index update on status column
PgSQL:   HOT update (no index maintenance if status not indexed separately)
```
**Expected gain**: 30-50% less I/O per status update under concurrent load.

### Pattern 4: Task Output Streaming (longtext append)

```
MySQL:   Full row rewrite for LONGTEXT update + undo log
PgSQL:   TOAST storage for large text, only changed portion written
```
**Expected gain**: Significant for large outputs (10KB+ per update). TOAST avoids rewriting the full row.

### Pattern 5: Metric Inserts (time-series pattern)

```
MySQL:   INSERT with ULID PK (somewhat sequential due to time prefix)
PgSQL:   Same, but can use BRIN index for time-range queries (much smaller than B-tree)
```
**Expected gain**: BRIN indexes for metrics table = 100x smaller than B-tree for range queries.

---

## Where MySQL is Equivalent

| Pattern | Notes |
|---------|-------|
| Simple CRUD (single row by PK) | Both equal |
| Team-scoped reads with B-tree index | Both equal for simple lookups |
| Pagination (LIMIT/OFFSET) | Both equal |
| Foreign key enforcement | Both equal |
| Connection pooling (100 conns) | Both handle fine |
| Transaction isolation (READ COMMITTED) | Both equal |
| Encrypted field read/write | Custom serializer works with both |

---

## Migration Steps

### Phase 1: Infrastructure (Low Risk)

1. **Provision PostgreSQL instance** (same specs as current MySQL)
2. **Update `DB_DRIVER=postgres`** in environment config
3. **Fix MySQL-specific code in migrator**:
   - `internal/database/migrations/migrator.go:201` — Replace `SHOW TABLES` with `SELECT tablename FROM pg_tables WHERE schemaname = 'public'`
   - `internal/database/migrations/migrator.go:206,219` — Replace `SET FOREIGN_KEY_CHECKS` with `SET session_replication_role = 'replica'` / `'origin'`
4. **Run migrations against PostgreSQL** — GORM handles dialect differences for CREATE TABLE
5. **Verify all encrypted field serialization works** — The `encrypted.go` serializer uses `driver.Valuer`/`sql.Scanner` which is driver-agnostic

### Phase 2: Schema Optimization (Medium Risk)

1. **Convert JSON columns to JSONB** in migrations:
   ```go
   migrator.AlterColumn(&Server{}, "provider_data") // type: jsonb
   ```
2. **Add GIN indexes on JSONB columns**:
   ```sql
   CREATE INDEX idx_servers_provider_data ON servers USING GIN (provider_data);
   CREATE INDEX idx_sites_vcs_data ON sites USING GIN (vcs_data);
   CREATE INDEX idx_sites_features ON sites USING GIN (features);
   ```
3. **Add partial indexes**:
   ```sql
   CREATE INDEX idx_servers_team_active ON servers (team_id) WHERE archived_at IS NULL;
   CREATE INDEX idx_sites_team_active ON sites (team_id) WHERE archived_at IS NULL;
   CREATE INDEX idx_sites_server_active ON sites (server_id) WHERE archived_at IS NULL;
   CREATE INDEX idx_deployments_active ON deployments (site_id) WHERE status IN ('pending', 'installing');
   ```
4. **Add BRIN index on metrics** (time-series):
   ```sql
   CREATE INDEX idx_metrics_created ON metrics USING BRIN (created_at);
   ```

### Phase 3: Feature Adoption (Low-Medium Risk)

1. **LISTEN/NOTIFY** for WebSocket events — Add PostgreSQL listener in WebSocket hub
2. **Advisory locks** for deployment serialization per site
3. **LATERAL joins** where "latest per group" pattern is needed
4. **Convert array columns** (optional) from JSONStringSlice to native `text[]`

### Phase 4: Data Migration (High Risk — Do Last)

1. **pg_dump/restore** or use a migration tool (pgloader, DMS)
2. **Verify encrypted fields** decrypt correctly after migration
3. **Verify JSON data** round-trips correctly (MySQL JSON → PostgreSQL JSONB)
4. **Verify ULID ordering** is preserved
5. **Run full test suite** against PostgreSQL

---

## Files Requiring Changes

| File | Change | Risk |
|------|--------|------|
| `.env` / config | `DB_DRIVER=postgres` | Trivial |
| `internal/database/database.go` | Already supports postgres (line 36-38) | None |
| `internal/database/migrations/migrator.go` | Replace MySQL-specific SQL | Low |
| `internal/database/migrations/*.go` | Add JSONB type hints, GIN indexes | Low |
| `internal/modules/site/repositories/deployment_repository.go` | `DISTINCT ON` already works | None |
| `internal/pkg/dbtype/json.go` | Verify JSONB scanning | Low |
| `internal/pkg/dbtype/encrypted.go` | Verify bytea storage | Low |
| `internal/modules/websocket/` | Add LISTEN/NOTIFY listener (new feature) | Medium |
| `internal/modules/site/services/deployment_service.go` | Add advisory lock before deploy | Medium |

---

## Risks and Mitigations

| Risk | Mitigation |
|------|-----------|
| VACUUM overhead | Configure `autovacuum_vacuum_scale_factor = 0.05` for high-churn tables (tasks, deployments) |
| Connection limit | PostgreSQL processes are heavier than MySQL threads — keep MaxOpenConns at 100, use PgBouncer if scaling beyond |
| Data migration corruption | Run checksums on encrypted fields before/after migration |
| JSONB vs JSON differences | JSONB doesn't preserve key order or duplicate keys — verify no code depends on these |
| Long-running transactions | PostgreSQL bloats more than MySQL with long transactions — ensure all transactions are short |
| Learning curve | PostgreSQL tooling (psql, pg_stat_statements, EXPLAIN ANALYZE) differs from MySQL |

---

## Decision Criteria

Migrate if:
- You plan to query inside JSON columns (filtering by provider, features, etc.)
- You need real-time events without Redis (LISTEN/NOTIFY)
- Your deployment concurrency will grow (10+ concurrent deploys)
- You want smaller indexes via partial indexing (cost savings on large tables)

Stay on MySQL if:
- Your JSON columns are purely opaque storage (never queried)
- You have existing MySQL operational expertise and don't want to retrain
- Your dataset is small enough that performance differences are negligible (<100K rows per table)

---

## Estimated Impact

| Metric | Current (MySQL) | Expected (PostgreSQL) | Improvement |
|--------|-----------------|----------------------|-------------|
| JSON-filtered queries | Full scan | GIN index | 10-100x |
| Status update latency (concurrent) | Row lock + undo | HOT update | 30-50% |
| Index memory (with archives) | Full index | Partial index | 60-80% smaller |
| Latest-per-group queries | Subquery + join | DISTINCT ON / LATERAL | 40-60% |
| Real-time events | Requires Redis | Native LISTEN/NOTIFY | Removes dependency |
| Metric range queries | B-tree | BRIN | 100x smaller index |
