# Docker server menus & workloads — design

**Status:** draft / accepted
**Date:** 2026-05-22
**Spans:** `launch-go` (backend, data model, jobs) + `launch-nuxt` (UI)
**References:** `dokploy/` repo at the workspace root, used as a UX reference
only — we are not copying code.

---

## 1. Goal

Server provisioning now supports a `docker` server type (see commits on
`feature/docker-server-provisioning`). The server detail page, however,
still falls through to the PHP-stack tabs (Sites, Databases, Daemons, …)
because those tabs are hardcoded for non-loadbalancer servers. This is
confusing — a docker server has no PHP "sites" and no system-level
"databases" the way a PHP server does.

We replace the tab set for docker servers with one that matches how docker
workloads are actually managed: projects that group applications, compose
stacks, and managed databases. Each workload gets its own detail page with
the dokploy-style subtabs (General / Deployments / Environment / Domains /
Logs / Volumes / Schedules / Advanced).

---

## 2. Scope

### In scope
- New `docker_projects`, `docker_applications`, `docker_composes`,
  `docker_databases` tables + child tables for env vars, domains, volumes,
  deployments, schedules.
- Replace the server detail tabs on a docker server with: Projects,
  Containers, Volumes, Networks, Traefik, Metrics, Advanced.
- New routes:
  - `/servers/:id/projects/:projectId` (project detail)
  - `/servers/:id/projects/:projectId/applications/:appId`
  - `/servers/:id/projects/:projectId/composes/:composeId`
  - `/servers/:id/projects/:projectId/databases/:dbId`
- Empty-state placeholders for every tab we don't implement in phase 1.
- The first basic feature, picked after this doc lands.

### Out of scope (deferred)
- **Preview deployments** (per-PR ephemeral deploys) — explicitly out.
- **Templates / one-click installs** — defer until applications work.
- **Multi-environment per project** (prod/staging split) — keep it flat
  for now; revisit once the single-environment model is in production.
- **Self-build registry / image transfer** — builds run on the target
  server.
- **Monitoring / Patches** dokploy subtabs — defer.

### Existing functionality we are NOT touching
- PHP and load-balancer server pages remain on their current tab set.
- The Sites flow and its associated routes stay intact.
- `useServerTypeRules` gets a new `tabs` field but its `showsPhp` /
  `showsDatabase` rules stay as they are.

---

## 3. Decisions captured during brainstorming

| Question | Decision |
|---|---|
| Scope of this initiative | Everything in one go (multiple PRs, phased rollout) |
| Data model hierarchy | Server → Project → Application/Compose/Database (no environments) |
| Workload types | Applications, Compose, Databases — no Templates yet |
| Server-page layout | Project-first (server has a Projects tab; drill in for workloads) |
| App subtabs | General, Deployments, Environment, Domains, Logs, Volumes, Schedules, Advanced |
| Preview deployments | Excluded |
| Build location | On the target server (dokploy-style) |
| Reverse proxy | Traefik (already installed by the docker provisioning task) |

---

## 4. Data model (launch-go)

All tables are team-scoped, soft-deletable, with the usual `created_at` /
`updated_at` audit columns. FKs cascade from `servers` → `docker_projects`
→ workload tables.

### 4.1 `docker_projects`
```
id            uuid pk
team_id       uuid not null, fk teams
server_id     uuid not null, fk servers
name          varchar(255) not null
description   text null
created_at    timestamp
updated_at    timestamp
deleted_at    timestamp null

unique (server_id, name) where deleted_at is null
```
A server has many projects. Deleting a server cascades. Project names are
unique per server (case-insensitive) so the file-system layout
(`/var/lib/launch/projects/<name>`) stays sane.

### 4.2 `docker_applications`
```
id              uuid pk
team_id         uuid
server_id       uuid
project_id      uuid fk docker_projects
name            varchar(255)              -- unique within project
source_type     enum('git','image','dockerfile')
source_config   jsonb                     -- repo url, branch, image ref, build args, …
build_type      enum('nixpacks','dockerfile','heroku-buildpack')
build_config    jsonb                     -- builder-specific
status          enum('idle','building','running','stopped','failed')
container_id    varchar(255) null         -- last known container, refreshed on deploy
last_deployed_at timestamp null
created_at / updated_at / deleted_at
```
Child tables (each with `application_id` fk, `created_at`, `updated_at`,
soft-delete where it makes sense):
- `docker_application_env_vars` (key, value, is_secret)
- `docker_application_domains` (host, path, https, certificate_id)
- `docker_application_volumes` (name, mount_path, type=`named`|`bind`, host_path)
- `docker_application_schedules` (cron, command, last_run_at, last_status)

### 4.3 `docker_composes`
Same shape as `docker_applications` minus `build_type`/`build_config`, plus
`compose_file_path` and `compose_source_type` (`git`|`raw_yaml`). Shares
the `docker_application_env_vars` semantics via a separate
`docker_compose_env_vars` table (avoid polymorphism in env vars — keeps
indexes simple).

### 4.4 `docker_databases`
```
id              uuid pk
team_id, server_id, project_id
name
engine          enum('postgres','mysql','mariadb','redis','mongo')
engine_version  varchar(32)             -- e.g. "16", "8.0"
image_tag       varchar(255)            -- resolved tag we actually pulled
external_port   int null                -- host-side; null = internal only
credentials     jsonb                   -- encrypted at rest (use existing
                                        --   credential encryption helpers)
status          enum('starting','running','stopped','failed')
created_at / updated_at / deleted_at
```
Child: `docker_database_backups` (s3 bucket/path/schedule/last_run_at).

### 4.5 `docker_deployments` (polymorphic)
```
id           uuid pk
team_id, server_id
target_type  enum('application','compose')
target_id    uuid                       -- application or compose id
status       enum('pending','building','deploying','success','failed','cancelled')
commit_sha   varchar(64) null           -- git sources only
commit_msg   text null
image_ref    varchar(512) null          -- image sources only
log_path     varchar(512)               -- where the deployment log lives
started_at   timestamp null
finished_at  timestamp null
error        text null
```
Polymorphism is fine here because every read goes via
`(target_type, target_id)` and we never join across types. Index on
`(target_type, target_id, started_at desc)` for the per-workload history
view.

Databases don't deploy — they restart, snapshot, restore. They get their
own `docker_database_events` table instead of sharing this one.

---

## 5. Server detail page — top-level tabs (launch-nuxt)

Source of truth is `useServerTypeRules`. Add a `tabs` field returning the
type-specific list, drop the hardcoded `isLoadBalancerServer` switch in
`Navbar.vue` and `pages/servers/[id]/index.vue` in favour of reading from
the composable.

| Tab | Component | Icon | Phase 1 state |
|---|---|---|---|
| Projects | `ServerDockerProjects.vue` | `lucide:folder-tree` | List + create works (basic feature) |
| Containers | `ServerDockerContainers.vue` | `lucide:container` | Empty-state placeholder |
| Volumes | `ServerDockerVolumes.vue` | `lucide:hard-drive` | Empty-state placeholder |
| Networks | `ServerDockerNetworks.vue` | `lucide:network` | Empty-state placeholder |
| Traefik | `ServerDockerTraefik.vue` | `simple-icons:traefikproxy` | Empty-state placeholder |
| Metrics | existing `ServerShowMetrics` | `lucide:activity` | Works as-is |
| Advanced | existing `ServerAdvancedSettings` | `lucide:sliders-horizontal` | Works as-is |

Docker-specific tab content lives under `components/server/docker/` so a
grep for "docker" lands on every relevant component.

---

## 6. Project detail page (launch-nuxt)

Route: `/servers/:id/projects/:projectId`. Breadcrumb updates to
`Servers / {server} / {project}`.

| Subtab | Component | Phase 1 state |
|---|---|---|
| Overview | `Project/Overview.vue` | Totals + empty-state placeholder |
| Applications | `Project/Applications.vue` | List + Create sheet (basic feature) |
| Compose | `Project/Composes.vue` | List view (Create sheet placeholder) |
| Databases | `Project/Databases.vue` | List view (Create sheet placeholder) |
| Settings | `Project/Settings.vue` | Rename + delete |

Create flows are right-side `<Sheet>` slide-ins (matches AddDomain,
CreateScript). Three components: `CreateApplicationSheet.vue`,
`CreateComposeSheet.vue`, `CreateDatabaseSheet.vue`. Only Application gets
the full form in phase 1; the others render a "Coming soon" body.

---

## 7. Application detail page (launch-nuxt)

Route:
`/servers/:id/projects/:projectId/applications/:applicationId`.

| Subtab | Phase 1 |
|---|---|
| General | Show source + build settings (read-only is fine if create flow already captured them) |
| Deployments | Empty list + "deploy" CTA placeholder |
| Environment | Empty placeholder |
| Domains | Empty placeholder |
| Logs | Empty placeholder |
| Volumes | Empty placeholder |
| Schedules | Empty placeholder |
| Advanced | Empty placeholder |

Tab-visibility rule for after phase 1: subtabs other than General require
`last_deployed_at != null`; otherwise show "Available after first
deployment". Helps users understand why Logs is empty before they deploy.

**Compose detail** and **Database detail** pages share a chrome component
(`DockerWorkloadShell.vue`) so the breadcrumb, status badge, and action
buttons (Deploy / Restart / Stop) are consistent. Their tab sets:

- Compose: General, Deployments, Environment, Domains, Logs, Schedules
- Database: General, Environment, Backups, Logs, Advanced

---

## 8. Backend API surface (launch-go)

New module: `internal/modules/docker/`. Mirror the structure of
`internal/modules/server/` (config, dto, jobs, models, providers if needed,
repos, services, tasks). Routes under `/api/servers/{serverId}/docker/...`
keep tenancy enforcement aligned with the existing server middleware.

Phase-1 endpoints (minimum to ship the basic feature):

```
GET    /api/servers/{serverId}/docker/projects
POST   /api/servers/{serverId}/docker/projects
GET    /api/servers/{serverId}/docker/projects/{projectId}
PATCH  /api/servers/{serverId}/docker/projects/{projectId}
DELETE /api/servers/{serverId}/docker/projects/{projectId}

GET    /api/servers/{serverId}/docker/projects/{projectId}/applications
POST   /api/servers/{serverId}/docker/projects/{projectId}/applications
GET    .../applications/{applicationId}
PATCH  .../applications/{applicationId}
DELETE .../applications/{applicationId}
```

Read-only endpoints (`GET /containers`, `GET /volumes`, `GET /networks`,
`GET /traefik`) get stubs that return empty arrays in phase 1 so the
frontend can be wired without blocking on the worker pipeline.

---

## 9. Worker pipeline (later phases — sketched here so we don't paint
ourselves into a corner)

Deploys run as `asynq` tasks dispatched from the API. Pipeline for an
application git-source deploy:

1. **prepare** — write a deployment row (`status=pending`), broadcast
   `deployment.started`.
2. **clone** — `git clone` on the docker server via SSH into
   `/var/lib/launch/projects/<project>/<app>/_build/<deploy_id>`.
3. **build** — invoke chosen builder (nixpacks/dockerfile) producing
   `launch/<project>-<app>:<short_sha>`.
4. **start** — `docker run -d` with computed flags from env vars,
   volumes, networks, restart policy; update `container_id`.
5. **wire Traefik** — write/refresh the dynamic config file for the
   app's domains; reload Traefik (`docker exec` reload signal).
6. **finalise** — broadcast `deployment.finished` (or `.failed`),
   update `applications.status` and `last_deployed_at`.

The broadcast contract from
[CLAUDE.md](launch-go/CLAUDE.md#websocket-broadcasting--architecture--rules)
applies: every state transition broadcasts with the routing field, both on
worker path (via `RedisBroadcaster`) and API path (via Mixin).

Channels we add:
- `application.{applicationId}` — for log streaming + status updates.
- `project.{projectId}` — for project-level dashboard updates.

---

## 10. Rollout phases

1. **Phase 1 (this initiative, what we're starting now)**
   - Migrations for all four tables.
   - Backend: projects CRUD + applications CRUD (no deploy logic).
   - Frontend:
     - `useServerTypeRules.tabs` extension.
     - Docker server tabs with empty-state placeholders for non-Projects.
     - Project list + create.
     - Application list + create sheet (form only, no deploy yet).
     - All subtabs scaffolded with "Coming soon" placeholders.

2. **Phase 2 — First deploy works end-to-end**
   - Deploy pipeline for `source_type=git` only.
   - Logs subtab (read-only, last deployment).
   - Domains subtab + Traefik wiring.

3. **Phase 3 — Compose + databases**
   - Compose CRUD + deploy.
   - Database CRUD + lifecycle (start/stop/restart/backup).

4. **Phase 4 — Polish**
   - Containers / Volumes / Networks server-level tabs become real.
   - Schedules subtab.
   - Advanced subtab (resources, ports, healthcheck).

---

## 11. Open questions to revisit before phase 2

- Are application names unique per project, or per server? (Going with
  per-project for now; revisit when collision happens in QA.)
- Where do deploy logs live — same `provision_logs` table or a dedicated
  one? (Likely dedicated, but resolve once Logs subtab is built.)
- How do we surface build errors in the UI distinctly from runtime
  errors? (Defer — affects Deployments subtab design.)
- Registry credentials for private images: per-team or per-application?

---

## 12. Notes on dokploy parity

Things we are deliberately doing differently from dokploy:

| Dokploy | Us | Why |
|---|---|---|
| Project → Environment → Service | Project → Workload (no env layer) | Dual-environment model is a big abstraction; we don't have customer demand yet |
| Preview deployments | Not building | Out of scope per brainstorming |
| Subtabs visible on undeployed app | Hidden until first deploy | Reduces "why is this empty?" support tickets |
| Generic "services" tab | Type-specific (Applications/Compose/Databases) tabs | Easier to wire create flows, easier to scan |

