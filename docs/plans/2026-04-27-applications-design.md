# Applications on Docker Servers — Phased Plan

**Date:** 2026-04-27
**Branch (target):** `feat/docker-provisioning`
**Constraint reminder:** No Docker Swarm, no replicas — every application is exactly one container (or one Compose project) on exactly one server.

---

## Goal

Let users deploy and manage applications on Docker servers, the way "Sites" work on PHP servers but for containers. Sources span:

- **Image** (pull `ghcr.io/foo/bar:tag` and run) — covers ~90% of usage
- **Compose** (paste a YAML, run `docker compose up -d`) — covers multi-service stacks (n8n, Postgres+app, etc.)
- **Git** (later) — clone + build + run

Each application gets a detail page with: Overview, Environment, Volumes, Domains, Logs, Terminal, Settings. The global credentials manager grows a "Docker registries" section so private images (GHCR, Docker Hub, generic) work.

---

## Phase 1 — Docker registry credentials

**Why first:** small, self-contained, unblocks every later phase. Mirrors the existing source-control credentials pattern.

**Backend**

- New module `internal/modules/dockerregistry/` (or extend `git/source_control` if the abstraction is reusable — TBD on read).
- DB table `docker_registries`:
  - `id`, `team_id`
  - `name` (display label)
  - `type` (`docker_hub` | `ghcr` | `generic`)
  - `url` (defaults: `https://index.docker.io/v1/`, `ghcr.io`, user-specified)
  - `username`
  - `password` (encrypted at rest, like other secrets)
  - timestamps
- API `/api/docker-registries` — Index / Create / Show / Update / Delete (team-scoped).
- Used at deploy time: `docker login <url> -u <user> -p <pass>` before `docker pull`.

**UI**

- New tab on the global credentials/settings page: "Docker registries".
- List + create dialog with type selector (auto-fills URL placeholder per type).
- Reuse the existing credentials list/dialog scaffolding from source-control.

**Out of scope for v1:** AWS ECR (needs IAM auth, not username/password), GCR with service account JSON. Add later as separate types.

---

## Phase 2 — Applications: model + image-source lifecycle

**Why next:** the core resource. Once this exists, the detail page (Phase 3) has something to render.

**Backend**

- New module `internal/modules/dockerapp/`.
- DB tables:
  - `dockerapps`
    - `id`, `team_id`, `server_id`
    - `name` (slug-safe — used as container name `launch-app-<name>`)
    - `source` (`image` for v1; `compose` and `git` later)
    - `image`, `tag` (image only)
    - `registry_credential_id` (FK, nullable)
    - `restart_policy` (`unless-stopped` | `always` | `no` | `on-failure`)
    - `status` (`pending` | `deploying` | `running` | `stopped` | `failed`)
    - `last_error`, `task_id`
    - `created_at`, `updated_at`
  - `dockerapp_env_vars` (id, app_id, key, value (encrypted))
  - `dockerapp_ports` (id, app_id, container_port, host_port, protocol)
  - `dockerapp_volumes` (id, app_id, name, mount_path) — named volumes only, no bind mounts in v1
  - `dockerapp_domains` (id, app_id, domain, container_port (the upstream service port), tls (bool))
- Routes (team + server scoped):
  - `GET    /servers/:serverId/apps`
  - `POST   /servers/:serverId/apps` — create + deploy
  - `GET    /servers/:serverId/apps/:id`
  - `PUT    /servers/:serverId/apps/:id` — edit settings
  - `DELETE /servers/:serverId/apps/:id` — uninstall (remove container + record)
  - `POST   /servers/:serverId/apps/:id/deploy` — pull + redeploy
  - `POST   /servers/:serverId/apps/:id/start|stop|restart`
  - `GET    /servers/:serverId/apps/:id/logs?tail=N`
  - Sub-resources: env vars, ports, volumes, domains as nested CRUD.
- Lifecycle:
  - **Deploy** (async job): if registry credential set → `docker login`. `docker pull <image>:<tag>`. `docker rm -f <container>`. `docker run -d --name <container> --restart <policy> --network launch-network -e ... -p ... -v ... --label <traefik labels> <image>:<tag>`.
  - **Lifecycle ops** (sync): `docker start|stop|restart <container>`.
  - **Logs** (sync): `docker logs --tail N <container>`.
  - **Uninstall** (async): `docker rm -f <container>`. Volumes preserved by default; `?remove_data=true` drops them.
- Templates under `internal/modules/dockerapp/tasks/templates/dockerapp/`:
  - `deploy.sh` — login (if creds) → pull → rm → run with all flags
  - `uninstall.sh`, `lifecycle.sh`, `logs.sh` (mirror dockerservice)
- Traefik labels generated server-side from `dockerapp_domains`:
  - `traefik.enable=true`
  - `traefik.http.routers.<app>.rule=Host(\`<domain>\`)`
  - `traefik.http.routers.<app>.entrypoints=web,websecure`
  - `traefik.http.routers.<app>.tls.certresolver=letsencrypt` (when `tls=true`)
  - `traefik.http.services.<app>.loadbalancer.server.port=<container_port>`
- Broadcast events: `app.progress` (team channel) carrying `{team_id, server_id, app_id, status, message}`.

**UI** — minimal "Add Application" flow only this phase

- "Apps" tab on Docker server detail page (replaces or sits alongside "Sites" — Sites is hidden on Docker servers anyway since site is a PHP concept).
- Apps list: name, image, status badge, deployed-at, action menu.
- "Add Application" dialog — image-source form: name, image, tag, registry credential (dropdown of team registries, "Public" option), restart policy. Env vars / ports / volumes / domains can be added after creation in Phase 3.

---

## Phase 3 — Application detail page

**Why now:** Phase 2 ships an app you can deploy but not configure. Phase 3 is where the daily-driver UX lives.

**UI tabs** (each is a sub-route under `/servers/:serverId/apps/:id`):

1. **Overview**
   - Status badge, image, container name, deployed-at, last error
   - Buttons: Redeploy, Start / Stop / Restart, Uninstall
   - Quick links to other tabs

2. **Environment**
   - Key/value list (one row per env var)
   - Add / edit / delete inline
   - Mark a value as a secret (UI hides it after save) — value column is encrypted regardless
   - Save → autoredeploy toggle: "Apply on next deploy" vs "Redeploy now"

3. **Volumes**
   - Named volumes mounted into the container
   - Add: volume name (auto-prefixed `launch-app-<app>-<name>`), mount path
   - Delete: prompts "Drop volume too?" (matches dockerservice uninstall pattern)

4. **Domains**
   - List of domains routing to this app via Traefik
   - Add: domain, container port (defaults to first published port), TLS (Let's Encrypt) toggle
   - Save triggers a Traefik dynamic-config update + redeploy

5. **Logs**
   - Reuse existing `DockerServiceLogsDialog` shape inline (live-tail + manual refresh + tail length)

6. **Terminal**
   - Reuse `ServerTerminal` component, but wrap the SSH session in `docker exec -it <container> /bin/sh` (or bash if available — fallback to sh).

7. **Settings**
   - Rename
   - Change restart policy
   - Change registry credential
   - Danger zone: Uninstall (with checkbox to drop named volumes)

---

## Phase 4 — Compose source

**Why deferred:** the data model and UI work for Phase 2/3 are reusable. Compose is mostly an alternate deploy script.

**Backend**

- Extend `dockerapps` with `source = "compose"` and add columns:
  - `compose_yaml` (text, what the user types/pastes)
  - `compose_env` (text, the `.env` file contents) — could reuse `dockerapp_env_vars` but flat-text matches how compose actually consumes them
- On deploy:
  - Render `compose_yaml` → `/etc/launch/apps/<name>/compose.yaml`
  - Render `compose_env` → `/etc/launch/apps/<name>/.env`
  - If a registry credential is set → `docker login`
  - `docker compose -f compose.yaml --env-file .env pull`
  - `docker compose -f compose.yaml --env-file .env up -d --remove-orphans`
- On uninstall:
  - `docker compose -f compose.yaml --env-file .env down` (preserve volumes by default)
  - `docker compose ... down --volumes` if `?remove_data=true`
- Status detection: parse `docker compose ps --format json` to surface per-service state. The app-level `status` rolls up: any `failed` → failed, all `running` → running, anything else → mixed/deploying.

**UI**

- "Add Application" dialog gains a source picker: **Image** | **Compose**
- Compose form: name, paste/edit YAML in CodeMirror (already used elsewhere in the project), env vars editor (key/value pairs that get rendered to `.env`), registry credential
- Domains tab gets a service-name dropdown — pick which compose service to route, helper injects Traefik labels into the YAML on save (with a preview diff)
- Volumes tab: read-only list of named volumes declared in the compose (since compose owns them)
- Terminal tab: pick a service to exec into

**Note:** users pasting their own YAML with their own Traefik labels work fine; the Domains UI is purely a convenience layer.

---

## Phase 5 — Git source (later, separate spec)

- `source = "git"`
- Source control credential FK + repo URL + branch + Dockerfile path or `compose.yaml` path
- Build on the server (`docker build` or `docker compose build`)
- Webhook auto-deploy
- This is a substantial follow-up; keep it out of the initial cut.

---

## Phase 6 — Polish (incremental)

In rough priority:

- Health checks (auto-restart on failing healthcheck)
- Resource limits (cpu, memory)
- Auto-update schedules ("redeploy every Sunday 03:00 with `--pull always`")
- Per-app log retention configs
- Domain-level basic auth (Traefik middleware)

Explicitly out of scope, ever: replicas, swarm services, multi-node placement.

---

## Sequencing summary

| Phase | Title | Reviewable? | Depends on |
|------:|-------|-------------|------------|
| 1 | Docker registry credentials | yes (self-contained) | — |
| 2 | App model + image-source lifecycle | yes | Phase 1 (for private images) |
| 3 | App detail page (env / volumes / domains / logs / terminal / settings) | yes | Phase 2 |
| 4 | Compose source | yes | Phase 2 (data model), 3 (UI) |
| 5 | Git source | yes (separate spec) | Phase 1, 2 |
| 6 | Polish | per-feature | Phase 2 |

Each phase ships behind its own commit on `feat/docker-provisioning`. We pause after each one to confirm before starting the next.
