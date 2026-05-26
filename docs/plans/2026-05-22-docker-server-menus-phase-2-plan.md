# Docker server menus — phase 2 implementation plan

**Status:** active
**Date:** 2026-05-22
**Predecessor:** docs/plans/2026-05-22-docker-server-menus-design.md
**Branch:** continues on `feature/docker-server-menus` (both repos)

## Goals

Phase 2 makes the docker server actually deploy things. By the end:

- Users can register Applications inside a Project, with source =
  `image` | `git` | `dockerfile`.
- Pushing "Deploy" runs an `asynq` job on the worker that SSHs into the
  docker server, clones+builds (or just pulls), and `docker run`s the
  container with the configured env vars + volumes + restart policy.
- The Application's Logs subtab streams live container stdout/stderr
  over WebSocket.
- Domains + Traefik wiring lets users attach hostnames; Traefik serves
  HTTPS with Let's Encrypt certs.
- Private git repos work via the existing `git` source-control
  connections.
- Compose stacks + managed Databases get their CRUD + deploy paths.

That's most of the design doc's phase-2/3 work. We'll land it as
sequential PR-sized slices on this branch so each slice is reviewable
in isolation.

## Slicing

| Slice | What ships | Verification gate |
|---|---|---|
| **2a** | Application CRUD: model already exists; add repo, service, handlers, routes, broadcast events. Frontend create dialog with source-type selector; General subtab becomes real. | `POST /docker/projects/:id/applications` creates a row; UI list + General tab render it. |
| **2b** | Deploy job for `source_type=image` (simplest path: `docker pull` + `docker run`). Deployment history table writes; lifecycle broadcasts. Deployments subtab renders the history. | Click Deploy on a `nginx:latest` app → row appears in `docker_deployments` with status=success, container is running on the server. |
| **2c** | Deploy job for `source_type=git` (public): clone on server, detect Dockerfile, build with `docker build` (Nixpacks deferred to 2c.2 if needed), `docker run`. | Deploy of `github.com/dokploy/example-app` succeeds end-to-end with built image. |
| **2d** | Logs subtab: WS endpoint `/api/applications/:id/logs/stream` that runs `docker logs -f <container>` via SSH and forwards lines to the WS. Frontend wires LogViewer.vue. | Visible log lines update live in the UI when the container writes. |
| **2e** | Domains subtab: CRUD for `docker_application_domains` + Traefik dynamic-config writer (HTTP only first). | Adding `app.example.com` makes the container reachable at that host. |
| **2f** | TLS for domains: enable Traefik's Let's Encrypt resolver with HTTP-01 challenge. Cert lifecycle tracked. | `https://app.example.com` serves a valid cert. |
| **2g** | Private git: extend 2c's clone step to use a per-app deploy key uploaded to the user's source-control account. | Deploy of a private GitHub repo succeeds. |
| **2h** | `source_type=dockerfile`: user pastes a Dockerfile, we write it to a temp dir on the server, `docker build`, run. | Pasted Dockerfile builds + runs. |
| **2i** | Compose CRUD + `docker compose up -d` job. | `docker-compose.yml` from git or raw deploys all services. |
| **2j** | Database CRUD + lifecycle (`docker run` of postgres/mysql/redis/mariadb/mongo + credential generation + start/stop/restart actions + backups). | Creating a postgres database brings up a healthy container with credentials visible in the UI. |

Each slice ends with `go test ./... && go vet ./... && npm run test`
green and a manual smoke test against the running stack
(`localhost:3000` + `localhost:8080`).

## Cross-cutting infrastructure (built in 2a but used by all later slices)

### Backend file system layout on the docker server
```
/var/lib/launch/
  projects/
    <project-slug>/
      <app-slug>/
        _build/            # ephemeral clone + build dir, deleted on success
          <deploy_id>/
        deployments/
          <deploy_id>.log  # deploy stdout/stderr, served to UI
        compose.yml        # for compose workloads
  traefik/
    dynamic/
      <project>-<app>.yml  # traefik router config per app
```

Slug = ULID-tail or sanitised name; pick once in slice 2a.

### TaskRunner pattern for the deploy job

We already have `taskrunner` with callback support (see CLAUDE.md
"Task Definition with Callbacks"). The deploy task follows the same
shape as `DeploySiteTask`:

- Payload struct carries IDs only (`application_id`, `deployment_id`,
  `server_id`).
- `OnSuccess` updates `docker_applications.status=running`,
  `last_deployed_at=now`, persists `container_id`, broadcasts
  `application.deployed`.
- `OnFailure` sets `deployments.status=failed`, writes the captured
  output tail to `deployments.error`, broadcasts `application.failed`.
- `OnExpired` is `OnFailure` with status="timeout".

### WebSocket events (added to CLAUDE.md alongside server.* events)

Each broadcast must apply `EnsureRoutingField` per the architecture
rules in CLAUDE.md.

- **Application channel** `application.{applicationId}`:
  `application.updated`, `application.deploying`, `application.deployed`,
  `application.failed`, `application.stopped`, `application.log` (each
  log line during deploy).
- **Deployment channel** `deployment.{deploymentId}`:
  `deployment.started`, `deployment.progress`, `deployment.log`,
  `deployment.finished`, `deployment.failed`. Names match site
  deployments so the frontend listener can be a generic
  `useDeploymentEvents`.

### Frontend changes per slice

| Slice | Frontend |
|---|---|
| 2a | Create-application Sheet, list cards on Project's Applications tab, General subtab content |
| 2b | Deploy button (header action), Deployments subtab table |
| 2c | "Detect from repo" branch picker, build-type indicator |
| 2d | LogViewer in Logs subtab (reuse server/LogViewer.vue) |
| 2e | Domains table + AddDomain dialog (host, port, https toggle off for now) |
| 2f | Cert status badge in domains table, "renewing" / "issued" states |
| 2g | Source-control connection picker in create form when source_type=git |
| 2h | Paste-Dockerfile editor (CodeMirror) |
| 2i | Compose create flow (file or git), Compose subtab — almost identical UI to Apps |
| 2j | Database create flow (engine + version), Database subtab |

## Phase-1 fix to land first

Move the `Compose` and `Database` model definitions out of phase-1
status (they were placeholders). We need to:

- Define repo registries for compose + database (parallel to project repo).
- Add cross-service constructors in the docker module.

Doing this in slice 2a keeps the schema work contained.

## Decision log (from brainstorming on 2026-05-22)

| Question | Decision |
|---|---|
| Phase-2 scope | Land all sub-features sequentially on the same branch |
| Source types in 2a-2c | image + git (public) first; dockerfile in 2h; private git in 2g |
| TLS | Let's Encrypt via Traefik in 2f (HTTP first in 2e) |
| Git auth strategy | Reuse existing source-control connections in the `git` module |

## Open questions (deferred until they bite)

- Where do deploy logs live long-term? In-DB row + log file on the
  server? Decide in 2d.
- How do we handle the cert-renewal lifecycle? Traefik handles
  in-memory but on container restart it needs persistent storage —
  validate the provisioning step set this up.
- Build cache reuse: rebuilds of the same git ref should reuse layers.
  Easy with BuildKit; flag for later if it noticeably slows demos.

## How to run during development

1. `make migrate-up` (apply 0021 + 0022 if not already)
2. `make run` (API on :8080)
3. `make worker` (asynq worker)
4. From `launch-nuxt`: `npm run dev` (Nuxt on :3000)
5. Create a docker server, then a project under it, then an
   application — each slice extends what's clickable.
