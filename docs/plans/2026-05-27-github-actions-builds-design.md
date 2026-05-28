# Build with GitHub Actions — design

**Status:** draft / accepted
**Date:** 2026-05-27
**Spans:** `launch-go` (backend, data model, jobs, webhook) + `launch-nuxt` (UI)
**References:** cleavr.io ("Build your NodeJS apps with GitHub Actions, no
additional configuration required"); existing `internal/modules/git` GitHub
App integration; the application/compose deploy pipeline in
`internal/modules/docker/tasks/deploy_application.go`.

---

## 1. Goal

Today, when a user picks a git source for a docker application or compose
stack, the `docker build` runs on the target server. That's fine for small
apps but turns the server into a build host — CPU/RAM spikes during deploy,
the disk fills with build cache, and a heavy Nixpacks build can pin the
machine for minutes.

Add a third option alongside "build on server" and "pull a pre-built
image": **build on GitHub Actions, then deploy the resulting image**. The
user picks a connected GitHub repo; Launch commits a workflow file via the
existing GitHub App, the workflow builds + pushes to GHCR, and on success
calls back to Launch which runs its normal image-deploy pipeline. No
manual YAML, no manual webhook setup.

This applies to both `docker_applications` and `docker_composes`.

---

## 2. Scope

### In scope
- New `build_location` discriminator on `docker_applications` and
  `docker_composes` (`server` | `github_actions`).
- Per-workload deploy token (hashed at rest) used to authenticate inbound
  webhook calls from GitHub Actions.
- A `gha:bootstrap_workflow` job that, on enable / rotate / re-sync:
  - commits `.github/workflows/launch-deploy.yml` to the user's repo via
    the existing GitHub App;
  - writes `LAUNCH_DEPLOY_TOKEN` as an encrypted repo Actions secret;
  - writes `LAUNCH_APP_ID` and `LAUNCH_WEBHOOK_URL` as plain Actions
    variables.
- Two new public webhook endpoints (token-authenticated, not team-scoped
  via middleware):
  - `POST /api/webhooks/docker/applications/{app_id}/deploy`
  - `POST /api/webhooks/docker/applications/{app_id}/status` (failures)
  - same pair for `composes/{compose_id}/...`.
- Workflow YAML templates for the application and compose paths, both
  embedded via `embed.FS` and unit-tested for deterministic rendering.
- UI additions:
  - "Build location" toggle in the Create Application / Create Compose
    sheets when source = git.
  - A new "GitHub Actions" subtab on the application/compose detail pages
    surfacing workflow sync status, last run, the rotate-token action,
    and re-sync.
  - A `via GitHub Actions` badge + run-URL link in the deployment list.

### Out of scope (deferred)
- **Build-time env vars / secrets.** Today's `application_env_vars` are
  injected at `docker run` time. Build-time secrets (`NPM_TOKEN`,
  private package registries) need a `build_arg` boolean on each row and
  a corresponding `secrets:` / `build-args:` block in the workflow.
  Defer; data model has room for it.
- **Image retention.** Every push leaves a tag in GHCR; pruning is a
  follow-up scheduled job (`gha:prune_images`, keep last N).
- **Multi-arch builds.** Default to `linux/amd64`. Add a
  `build_config.platforms` knob once ARM servers exist in the platform.
- **Non-GHCR registries.** Cleavr-style zero-config means GHCR only.
  Letting users target Docker Hub / ECR / a private quay is a future
  toggle on the source config; the data model doesn't preclude it.
- **OIDC-authenticated webhook.** Bearer token first; OIDC is a possible
  follow-up if customers ask for it.
- **GitLab / Bitbucket.** Same model could work via GitLab CI /
  Bitbucket Pipelines, but the bootstrap path is provider-specific.
  Ship GitHub first.

### Existing functionality we are NOT touching
- The server-side build path (`SourceType=git`, on-server `docker build`
  / `nixpacks`) stays exactly as it is. Existing applications are
  unaffected — they read `build_location=server` from the migration
  default.
- The image-pull deploy script in
  `internal/modules/docker/tasks/deploy_application.go` is reused
  verbatim — GHA builds simply hand it a freshly-built GHCR image.
- The GitHub App / source-control flow in `internal/modules/git` is
  unchanged; we add new consumers but no schema changes.

---

## 3. Decisions captured during brainstorming

| Question | Decision |
|---|---|
| Where does the option appear? | Applications **and** Composes |
| Build/deploy flow | GHA builds & pushes → GHA webhooks Launch → Launch deploys via existing image pipeline |
| Target registry | **GHCR only** (zero-config; uses repo `GITHUB_TOKEN` for push, GitHub App installation token for pull) |
| How does the workflow file land? | Launch commits it via the GitHub App; user does not paste anything |
| Trigger | `push` to configured branch **and** `workflow_dispatch` |
| Builder inside the workflow | Auto-detect: Dockerfile if present, otherwise Nixpacks. For compose, `docker compose build` then push each service |
| Webhook authentication | Per-app deploy token, hashed at rest, written as a GH Actions secret on enable; rotatable |

---

## 4. Data model (launch-go)

No new tables. Three columns added to each of `docker_applications` and
`docker_composes`:

| Column | Type | Notes |
|---|---|---|
| `build_location` | `varchar(32) NOT NULL DEFAULT 'server'` | enum: `server`, `github_actions`. Existing rows pick up `server` via the migration default. |
| `gha_deploy_token_hash` | `varchar(255) NULL` | SHA-256 of the deploy token. Raw value is shown once on enable, then never persisted. |
| (none — see `source_config`) | | All other GHA settings live in the existing JSON column. |

The JSON `source_config` gains these keys when `build_location =
github_actions`:

- `source_control_id` — already used by the on-server git path; same key.
- `repository` — `owner/repo` slug (also already used).
- `branch` — the deploy branch.
- `dockerfile_path` — optional; default `Dockerfile`.
- `gha_image_repository` — derived (`ghcr.io/<owner>/<repo>`) but cached
  here so a renamed repo doesn't desynchronize.
- `gha_workflow_path` — default `.github/workflows/launch-deploy.yml`.
- `gha_workflow_sha` — the last commit SHA Launch wrote on the workflow
  file. Used to detect drift and pass `If-Match`-style checks when
  re-committing.

`docker_deployments` gains two columns to make GHA runs first-class:

| Column | Type | Notes |
|---|---|---|
| `trigger_source` | `varchar(32) NOT NULL DEFAULT 'manual'` | extends an enum we already have (`manual`, `auto`, etc.) with `github_actions`. |
| `gha_run_id` | `varchar(64) NULL UNIQUE per app` | webhook idempotency key. Same `run_id` posting twice (network retry) reuses the existing row instead of creating a duplicate. |

A nullable `gha_run_url` column on the same table powers the
deployment-list deep-link to the Actions run.

---

## 5. Flow

```
[ Enable GHA builds in UI ]
        │
        ▼
[ gha:bootstrap_workflow job ]
   - mint token, store sha256
   - render workflow YAML
   - GET repo public key
   - PUT LAUNCH_DEPLOY_TOKEN secret (sealed-box encrypted)
   - PUT LAUNCH_APP_ID + LAUNCH_WEBHOOK_URL variables
   - PUT .github/workflows/launch-deploy.yml (create or update with sha)
   - persist commit sha into source_config.gha_workflow_sha
        │
        ▼
[ User pushes to deploy branch ]
        │
        ▼
[ GitHub Actions run ]
   - checkout
   - compute image tag: ghcr.io/<owner>/<repo>:launch-<sha7>
   - docker login ghcr.io with GITHUB_TOKEN
   - detect builder (Dockerfile present? -> docker build, else nixpacks)
   - build + push (buildx with cache-from/to: type=gha)
   - on success: POST /webhooks/.../deploy with bearer token,
                 body = {image_tag, commit_sha, branch, run_id, run_url}
   - on failure (any prior step): POST /webhooks/.../status,
                 body = {status: "failed", run_id, run_url}
        │
        ▼
[ Launch webhook handler ]
   - Authorization: Bearer <token> -> sha256 -> constant-time compare
   - validate image_tag prefix matches stored gha_image_repository
   - upsert docker_deployments by (app_id, gha_run_id)
     status = pending, trigger_source = github_actions
   - enqueue deploy_application job synthesizing:
       SourceType = image
       Image = <image_tag from webhook>
       RegistryURL = ghcr.io
       RegistryUsername = "x-access-token"
       RegistryPassword = freshly-fetched GitHub App installation token
   - return 202 { deployment_id }
        │
        ▼
[ Existing image-deploy pipeline ]
   - docker login ghcr.io (via the credentials above)
   - docker pull / stop old / docker run
   - emit ::LAUNCH::container_id::... markers as today
   - websocket events fire on the deployment channel as today
```

The compose path is identical except:

- Webhook payload carries `service_images: { <service>: <ghcr image> }`
  instead of a single `image_tag`.
- The compose deploy job rewrites `image:` directives in the rendered
  YAML before `docker compose up`.

---

## 6. UI (launch-nuxt)

### Create sheets
`CreateApplicationSheet.vue` and `CreateComposeSheet.vue` get a **Build
location** radio group, visible only when source type = git:

```
Build location:
  ( ) On the server (default)
  (•) GitHub Actions  — Recommended for big apps
```

When **GitHub Actions** is selected:

1. The repo picker requires a connected **GitHub** source control. If
   none, the form inlines a "Connect GitHub" CTA pointing at
   Settings → Connections (existing flow).
2. Deploy branch defaults to the repo's default branch but is editable.
3. Dockerfile path is optional, defaulting to `Dockerfile`.
4. A read-only preview line:
   `Image will be published to ghcr.io/{owner}/{repo}:launch-{sha7}`
   (for compose: `…:launch-{service}-{sha7}`).

Submit triggers: persist the row, mint the deploy token, enqueue
`gha:bootstrap_workflow`. The toast shows the raw token **once** — this
is the only time it is shown.

### Detail page subtab
A new **GitHub Actions** subtab on the application and compose detail
pages, visible only when `build_location = github_actions`:

- **Workflow status**: "Synced to commit `abc1234` on branch `main`",
  plus a **Re-sync workflow** button (re-renders + re-commits if drifted).
- **Deploy token**: masked; **Rotate token** regenerates it, writes the
  new GH Actions secret, hashes the new value, shows the raw value once.
- **Last run**: status, duration, image tag, and a link to the GitHub
  Actions run URL.
- **Disable GHA builds**: deletes the secret + variables in GH, leaves
  the workflow file in place with an "(orphaned by Launch)" comment
  prepended so the user can decide whether to delete it.

### Deployment list
Existing deployment-list components get a `via GitHub Actions` badge
when `trigger_source = github_actions`. The badge links to the run URL.

---

## 7. Generated workflow YAML

Two templates in `internal/modules/docker/templates/`, both embedded via
`embed.FS`:

- `gha_application.yml.tmpl`
- `gha_compose.yml.tmpl`

Both render via Go `text/template` for deterministic, unit-testable
output.

### Application template

```yaml
# Managed by Launch. Edits will be overwritten on next sync.
name: Launch Deploy

on:
  push:
    branches: ["{{.Branch}}"]
  workflow_dispatch:

concurrency:
  group: launch-deploy-${{ github.ref }}
  cancel-in-progress: false

jobs:
  build-and-deploy:
    runs-on: ubuntu-latest
    timeout-minutes: 30
    permissions:
      contents: read
      packages: write
    steps:
      - uses: actions/checkout@v4

      - name: Compute image tag
        id: meta
        run: |
          echo "image=ghcr.io/${{ github.repository }}:launch-${GITHUB_SHA::7}" >> $GITHUB_OUTPUT

      - name: Log in to GHCR
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Detect builder
        id: builder
        run: |
          if [ -f "{{.DockerfilePath}}" ]; then
            echo "kind=dockerfile" >> $GITHUB_OUTPUT
          else
            echo "kind=nixpacks"   >> $GITHUB_OUTPUT
          fi

      - name: Build (Dockerfile)
        if: steps.builder.outputs.kind == 'dockerfile'
        uses: docker/build-push-action@v6
        with:
          context: .
          file: {{.DockerfilePath}}
          push: true
          tags: ${{ steps.meta.outputs.image }}
          cache-from: type=gha
          cache-to: type=gha,mode=max

      - name: Build (Nixpacks)
        if: steps.builder.outputs.kind == 'nixpacks'
        uses: iloveitaly/nixpacks-action@v0.1
        with:
          tags: ${{ steps.meta.outputs.image }}
          push: true

      - name: Notify Launch (success)
        env:
          TOKEN: ${{ secrets.LAUNCH_DEPLOY_TOKEN }}
        run: |
          curl --fail-with-body -sS -X POST \
            -H "Authorization: Bearer $TOKEN" \
            -H "Content-Type: application/json" \
            -d "$(jq -nc \
              --arg image  '${{ steps.meta.outputs.image }}' \
              --arg sha    '${{ github.sha }}' \
              --arg branch '${{ github.ref_name }}' \
              --arg run_id '${{ github.run_id }}' \
              --arg url    '${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }}' \
              '{image_tag:$image, commit_sha:$sha, branch:$branch, run_id:$run_id, run_url:$url}')" \
            "{{.LaunchBaseURL}}/api/webhooks/docker/applications/{{.AppID}}/deploy"

      - name: Notify Launch (failure)
        if: failure()
        env:
          TOKEN: ${{ secrets.LAUNCH_DEPLOY_TOKEN }}
        run: |
          curl -sS -X POST \
            -H "Authorization: Bearer $TOKEN" \
            -H "Content-Type: application/json" \
            -d "$(jq -nc \
              --arg run_id '${{ github.run_id }}' \
              --arg url    '${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }}' \
              '{status:"failed", run_id:$run_id, run_url:$url}')" \
            "{{.LaunchBaseURL}}/api/webhooks/docker/applications/{{.AppID}}/status"
```

### Compose template

Differs in:

- A matrix step that reads `docker-compose.yml`, extracts services with a
  `build:` directive, and builds + pushes each as
  `ghcr.io/{repo}:launch-{service}-{sha7}` in parallel.
- The notify payload sends `service_images: { <service>: <image> }`
  instead of a single `image_tag`.
- Webhook URLs point at `/composes/{compose_id}/...`.

### Notes on the YAML
- `cache-from/cache-to: type=gha` — buildx GHA cache. Free, fast; second
  build of an app is typically 5–10× faster than the first.
- `concurrency` with `cancel-in-progress: false` — a cancelled run would
  leave Launch waiting on a webhook that never arrives; we'd rather
  queue.
- `--fail-with-body` on the success notify — if Launch rejects the
  request (bad token, prefix mismatch, etc.) the Action job fails
  visibly in the GH UI.
- `timeout-minutes: 30` — matches the on-server 1800s timeout; bounds CI
  minute burn on a hung build.

---

## 8. Backend changes (launch-go)

### New: `internal/modules/docker/jobs/gha_bootstrap_workflow.go`
Steps as in Section 5. Idempotent. Uses the GitHub App installation
token (fetched via existing `providers/github.go`) for all repo writes.
The Actions-secret encryption uses libsodium sealed boxes via
`golang.org/x/crypto/nacl/box`.

### New: `internal/modules/docker/templates/`
`embed.FS` containing `gha_application.yml.tmpl` and
`gha_compose.yml.tmpl`. Rendered through `text/template`. Unit tests
assert byte-for-byte stable output for fixed inputs so accidental
template drift is caught in CI.

### New: webhook routes in `internal/modules/docker/routes.go`
Two pairs of public routes, mounted outside the team-auth middleware:

```
POST /api/webhooks/docker/applications/{app_id}/deploy
POST /api/webhooks/docker/applications/{app_id}/status
POST /api/webhooks/docker/composes/{compose_id}/deploy
POST /api/webhooks/docker/composes/{compose_id}/status
```

Each handler:

1. Loads the row by ID; 404 if missing or `build_location != github_actions`.
2. Parses `Authorization: Bearer <token>`, `sha256` of the raw, and
   `subtle.ConstantTimeCompare` against the stored hash. 401 on
   mismatch — no further error detail.
3. Validates the payload (image prefix must match `gha_image_repository`,
   `run_id` non-empty, etc.).
4. Upserts `docker_deployments` by `(workload_id, gha_run_id)` — gives
   us free idempotency on GHA's notify retries.
5. Enqueues `deploy_application` (or `deploy_compose`) with the
   synthesized image-source config.
6. Returns 202 with the deployment ID.

### Modified: `deploy_application` job
No behavioral change. The GHA webhook simply builds a `DeployConfig`
with `SourceType=image` and the GHCR image + installation-token
credentials before enqueuing. The existing image stanza in
`tasks/deploy_application.go` handles login/pull/logout unchanged.

### Modified: `deploy_compose` job
Adds optional `ServiceImages` to the deploy config. When populated, the
renderer rewrites the `image:` line of every matching service in the
compose YAML before writing it to the build directory. When nil, the
existing behavior (pull-and-up) runs.

### Migrations
- `00XX_add_build_location_to_docker_applications.go`
- `00XX_add_build_location_to_docker_composes.go`
- `00XX_add_gha_columns_to_docker_deployments.go`

Each follows the project rule (up only — no down methods in migrations).

---

## 9. Testing

- **Unit:** template rendering (golden file per template), webhook auth
  (token compare, prefix check, idempotency-on-retry), source-config
  validation when toggling `build_location`.
- **Integration:** end-to-end with a fake GitHub API server — the
  bootstrap job exercises the real `providers/github.go` code path
  against a stubbed transport that records the secret PUT, the workflow
  PUT, and the variable PUTs.
- **Manual:** dogfood against a real GitHub App on a personal repo;
  verify the run-URL deep link, token rotation, re-sync after a manual
  edit of the workflow file, and the "Disable GHA builds" cleanup.

---

## 10. Open items (not blockers)

- Token rotation behavior when the GitHub App installation has been
  uninstalled — today we'd 500 trying to fetch the installation token.
  Should we mark the workload `build_location_broken` and surface a
  banner? Probably yes; track in follow-up.
- Whether to expose **manual deploy** (workflow_dispatch) as a button in
  the Launch UI. Easy to add once the trigger lands; defer to v1.1.
- Surfacing live build logs from GitHub Actions inside Launch's
  deployment-detail view. Possible via the Actions API but doubles the
  UI work; out of scope until customers ask.
