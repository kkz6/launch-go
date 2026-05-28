# Build with GitHub Actions — implementation plan

**Status:** active
**Date:** 2026-05-28
**Branch:** `feature/gha-builds` (both repos use the same branch name)
**Design reference:** [docs/plans/2026-05-27-github-actions-builds-design.md](./2026-05-27-github-actions-builds-design.md)

This is the execution plan that pairs with the design doc. The design
captures *what* and *why*; this captures *which file, in which order,
with what verification gate*.

## Prerequisite findings (audited 2026-05-28)

- ✅ `SourceType=image` deploy path is fully shipped.
  `internal/modules/docker/tasks/deploy_application.go:125-128` already
  switches on `dockertypes.SourceTypeImage`, calls `buildImageStanza`,
  does `docker login --password-stdin` + `docker pull`. 8+ tests
  exercise it. GHA's wire-up (slice F) can synthesize a `DeployConfig`
  pointing at this branch verbatim.

- ❌ GitHub App can't yet commit files or write Actions secrets /
  variables. `internal/modules/git/providers/github.go` has
  `GetInstallationToken`, `GetRepository`, `DeployKey`,
  `CreateDeployment` — but no `PutContents`, `PutActionsSecret`,
  `PutActionsVariable`, no sealed-box helper. **Slice D builds these
  primitives before slice E can use them.**

- ✅ `docker_applications`, `docker_composes`, `docker_deployments`
  schemas live and have the columns the design assumes (source_type,
  source_config jsonb, image_ref, action, task_id, etc.). Column
  additions in slice A fit cleanly.

## Slicing

| # | Slice | Touches | Verification gate |
|---|---|---|---|
| A | Schema + model + DTO | 3 migrations, models, DTO | `go test ./...` green; new apps default `build_location=server` |
| B | YAML templates + golden tests | `internal/modules/docker/templates/` | byte-stable rendering of fixed inputs |
| C | Webhook routes + auth + idempotency | `internal/modules/docker/routes.go`, new handler file | curl with bearer → 202 + pending row; bad token → 401 |
| D | GitHub App write primitives | `providers/github.go`, NaCl sealed-box helper | unit tests against fake transport |
| E | `gha:bootstrap_workflow` job | new job file under `docker/jobs/` | manual: enable on a real repo, verify file + secret + variables land |
| F | Webhook → deploy wire-up | enqueue stanza in handler from C | manual end-to-end: push → workflow → deploy succeeds |
| G | UI | launch-nuxt create sheets + detail subtab + deployment badge | click-through |
| H | Compose path | matrix render in B's template, `ServiceImages` in deploy_compose | compose deploy via GHA |
| I | Polish | rotate-token, re-sync, disable cleanup, broken-installation banner | every subtab affordance works |

Each slice ships as its own PR off `feature/gha-builds`. Slices A→F
are MVP for applications. G lights up UI. H extends to compose. I polishes.

---

## Slice A — Schema + model + DTO

### Migrations

**`0050_05_28_000002_add_build_location_to_docker_applications.go`**

```sql
ALTER TABLE docker_applications
    ADD COLUMN build_location VARCHAR(32) NOT NULL DEFAULT 'server',
    ADD COLUMN gha_deploy_token_hash VARCHAR(255) NULL;
```

Existing rows pick up `'server'`, preserving today's behavior.

**`0050_05_28_000003_add_build_location_to_docker_composes.go`**

Same shape; same defaults.

**`0050_05_28_000004_add_gha_columns_to_docker_deployments.go`**

```sql
ALTER TABLE docker_deployments
    ADD COLUMN trigger_source VARCHAR(32) NOT NULL DEFAULT 'manual',
    ADD COLUMN gha_run_id VARCHAR(64) NULL,
    ADD COLUMN gha_run_url VARCHAR(512) NULL;

-- One run_id == one deployment row per workload. Webhook idempotency
-- relies on this UNIQUE; the migration adds a partial-unique so NULL
-- (= non-GHA deployments) doesn't collide.
CREATE UNIQUE INDEX idx_docker_deployments_gha_run_id
    ON docker_deployments (target_type, target_id, gha_run_id)
    WHERE gha_run_id IS NOT NULL;
```

### New enum

`internal/modules/docker/types/`:

```go
type BuildLocation string

const (
    BuildLocationServer        BuildLocation = "server"
    BuildLocationGitHubActions BuildLocation = "github_actions"
)
```

`internal/modules/docker/types/deployment.go` (extend an existing
trigger_source enum, or add it if not present — design assumed it
exists; if not we add a new typed string here):

```go
type DeploymentTriggerSource string

const (
    DeploymentTriggerManual         DeploymentTriggerSource = "manual"
    DeploymentTriggerAuto           DeploymentTriggerSource = "auto"
    DeploymentTriggerGitHubActions  DeploymentTriggerSource = "github_actions"
)
```

### Model additions

`internal/modules/docker/models/application.go`:

```go
BuildLocation        types.BuildLocation `gorm:"column:build_location;type:varchar(32);not null;default:'server'"`
GHADeployTokenHash   *string             `gorm:"column:gha_deploy_token_hash;type:varchar(255)"`
```

`internal/modules/docker/models/compose.go`: identical pair.

`internal/modules/docker/models/deployment.go`:

```go
TriggerSource types.DeploymentTriggerSource `gorm:"column:trigger_source;type:varchar(32);not null;default:'manual'"`
GHARunID      *string                       `gorm:"column:gha_run_id;type:varchar(64)"`
GHARunURL     *string                       `gorm:"column:gha_run_url;type:varchar(512)"`
```

### DTO additions

`internal/modules/docker/dto/responses.go` —
`ApplicationResponse`, `ComposeResponse`, `DeploymentResponse`:

```go
// Applications + Composes
BuildLocation string `json:"build_location"`        // "server" | "github_actions"
GHABuildReady bool   `json:"gha_build_ready"`       // true once bootstrap_workflow has run successfully
// (no raw token field — never serialized after the one-time create)

// Deployments
TriggerSource string  `json:"trigger_source"`
GHARunURL     *string `json:"gha_run_url,omitempty"`
```

`GHABuildReady` is derived: `BuildLocation == github_actions && GHADeployTokenHash != nil && SourceConfig.gha_workflow_sha != nil`.

### Tests

`models/application_test.go`, `models/compose_test.go`,
`models/deployment_test.go` — assert defaults, JSON marshalling
includes the new fields.

### Gate

`go test ./internal/modules/docker/... && make migrate` against the
local DB, then `SELECT build_location FROM docker_applications LIMIT 1`
returns `'server'` for existing rows.

---

## Slice B — Workflow YAML templates + golden tests

### Files

`internal/modules/docker/templates/`:

- `templates.go` — `//go:embed gha_*.yml.tmpl` + `var FS embed.FS`.
- `gha_application.yml.tmpl` — straight copy of the design doc §7
  "Application template" block.
- `gha_compose.yml.tmpl` — matrix variant per §7 notes.

### Render API

`internal/modules/docker/tasks/gha_template.go`:

```go
type ApplicationWorkflowData struct {
    Branch         string  // deploy branch
    DockerfilePath string  // default "Dockerfile"
    LaunchBaseURL  string  // e.g. "https://launchctl.io"
    AppID          string  // ULID
}

func RenderApplicationWorkflow(data ApplicationWorkflowData) (string, error)
func RenderComposeWorkflow(data ComposeWorkflowData) (string, error)
```

Use `text/template`. Strict mode (no unknown fields). Output normalised
through a `strings.TrimSpace(...) + "\n"` so trailing-whitespace
differences don't break golden tests.

### Tests

`gha_template_test.go` with two golden files in
`testdata/gha_application.golden.yml` and
`testdata/gha_compose.golden.yml`. Test renders against fixed inputs
and `assert.Equal` against the golden bytes. Updating a template
requires updating the golden file in the same commit.

### Gate

`go test ./internal/modules/docker/tasks/ -run Workflow` green.

---

## Slice C — Webhook routes + auth + idempotency

### Routes

`internal/modules/docker/routes.go`, mounted **outside** the team-auth
middleware (these are external callbacks, authenticated only by the
bearer token):

```go
public := router.Group("/api/webhooks/docker")
public.Post("/applications/:id/deploy", handler.GHAApplicationDeploy)
public.Post("/applications/:id/status", handler.GHAApplicationStatus)
public.Post("/composes/:id/deploy",     handler.GHAComposeDeploy)
public.Post("/composes/:id/status",     handler.GHAComposeStatus)
```

### Handler skeleton

`internal/modules/docker/handlers/gha_webhook_handler.go`:

```go
type GHAWebhookHandler struct {
    db          *gorm.DB
    broadcaster broadcast.TeamBroadcaster
    queue       *asynq.Client  // unused in slice C; F wires the enqueue
}

func (h *GHAWebhookHandler) GHAApplicationDeploy(c *fiber.Ctx) error {
    // 1. Load application by ID.
    //    404 if missing or build_location != github_actions.
    // 2. Parse Authorization: Bearer <token>.
    //    sha256(token) → constant-time compare against
    //    GHADeployTokenHash. 401 on miss; no body.
    // 3. Decode body into ApplicationDeployPayload (image_tag,
    //    commit_sha, branch, run_id, run_url).
    // 4. Validate image_tag prefix matches the configured
    //    gha_image_repository in source_config. 422 if not.
    // 5. Upsert docker_deployments by (target_id, gha_run_id).
    //    status = pending, trigger_source = github_actions.
    //    DO NOT enqueue yet — that's slice F.
    // 6. Return 202 { deployment_id }.
}
```

Same shape for compose; payload carries `service_images` instead of
`image_tag`.

### Status route

The `/status` handler is for "the build failed before reaching the
deploy notify." Marks the deployment as `failed` with the run_url
in `error`. Broadcasts a `docker.application.deploy_failed` event.

### Tests

`gha_webhook_handler_test.go`:

- Happy-path: build a request with valid bearer + payload, assert
  202 + DB row.
- Bad-token: 401, no DB row.
- Idempotent retry: same `(app_id, run_id)` twice → one row.
- Image prefix mismatch: 422.
- Status webhook marks failed.

### Gate

`go test ./internal/modules/docker/handlers/ -run GHA` green. Manual
curl smoke against a running stack.

---

## Slice D — GitHub App write primitives

### Files

`internal/modules/git/providers/github.go` — add the methods. Keep them
near the existing repo-read helpers so the file's organisation reads
naturally.

`internal/modules/git/providers/github_sealedbox.go` — NaCl sealed-box
encryption helper, factored out for unit testing without spinning the
full GitHub mock. Uses `golang.org/x/crypto/nacl/box`.

### API contracts

```go
// PutContents creates or updates a file in the repo via
// PUT /repos/{owner}/{repo}/contents/{path}.
// existingSHA == "" → create; non-empty → update (acts as If-Match).
// Returns the new file's commit SHA.
func (p *GitHubProvider) PutContents(
    ctx context.Context,
    installationID, owner, repo, path, content, message, existingSHA, branch string,
) (commitSHA string, err error)

// GetActionsPublicKey fetches the repo's public key for sealed-box
// encryption of Actions secrets.
// GET /repos/{owner}/{repo}/actions/secrets/public-key
func (p *GitHubProvider) GetActionsPublicKey(
    ctx context.Context,
    installationID, owner, repo string,
) (keyID string, publicKey []byte, err error)

// PutActionsSecret encrypts and writes an Actions secret.
// PUT /repos/{owner}/{repo}/actions/secrets/{secret_name}
// The body is { "encrypted_value": "<base64 sealed box>", "key_id": "<from GetActionsPublicKey>" }.
func (p *GitHubProvider) PutActionsSecret(
    ctx context.Context,
    installationID, owner, repo, secretName, value string,
) error

// PutActionsVariable creates or updates a repo Actions variable.
// Variables are plaintext (no sealed-box). Tries POST then PATCH on 422.
// POST /repos/{owner}/{repo}/actions/variables   (create)
// PATCH /repos/{owner}/{repo}/actions/variables/{name}  (update)
func (p *GitHubProvider) PutActionsVariable(
    ctx context.Context,
    installationID, owner, repo, name, value string,
) error

// DeleteActionsSecret + DeleteActionsVariable for "Disable GHA builds"
// cleanup in slice I.
func (p *GitHubProvider) DeleteActionsSecret(...) error
func (p *GitHubProvider) DeleteActionsVariable(...) error
```

### Sealed-box helper

```go
// sealActionsSecret encrypts plaintext for a GitHub Actions secret
// using the repo's libsodium public key. NaCl sealed boxes are the
// exact construction GitHub uses; box.SealAnonymous with the same
// 32-byte public key produces a wire-compatible ciphertext.
func sealActionsSecret(publicKey []byte, plaintext string) (string, error)
```

### Tests

`github_sealedbox_test.go` — round-trip with a known keypair, confirm
the output is decodable by NaCl's `box.OpenAnonymous`.

`github_writes_test.go` — fake HTTP transport via `httptest.Server` or
a `http.RoundTripper` stub. Each method asserts the right URL, method,
auth header (`Bearer <installation token>`), and body shape. No real
GitHub call.

### Gate

`go test ./internal/modules/git/providers/ -run "PutContents|PutActionsSecret|PutActionsVariable|SealedBox"` green.

---

## Slice E — `gha:bootstrap_workflow` job

### Files

`internal/modules/docker/jobs/gha_bootstrap_workflow.go` — new job type
`TypeGHABootstrapWorkflow = "docker:gha_bootstrap_workflow"`.

```go
type GHABootstrapWorkflowPayload struct {
    WorkloadKind string `json:"workload_kind"` // "application" | "compose"
    WorkloadID   string `json:"workload_id"`
}
```

### Steps

1. Load the workload row. Bail if `build_location != github_actions`.
2. Mint deploy token (32 bytes, hex-encoded). Compute sha256.
   `UPDATE … SET gha_deploy_token_hash = <sha256>` — the raw value
   is returned via the toast in the create flow (handled by the
   service in slice F's caller), not from this job.
3. Resolve installation token via existing `GetInstallationToken`.
4. Resolve repo public key via D's `GetActionsPublicKey`.
5. `PutActionsSecret("LAUNCH_DEPLOY_TOKEN", rawToken)`.
6. `PutActionsVariable("LAUNCH_APP_ID", workload.ID)` and
   `PutActionsVariable("LAUNCH_WEBHOOK_URL", appConfig.BaseURL)`.
7. Render the workflow YAML via slice B's template renderer.
8. `PutContents(".github/workflows/launch-deploy.yml", yaml,
   "Configure Launch deploy workflow", existingSHA, branch)`.
9. Persist returned commit SHA into `source_config.gha_workflow_sha`.
10. Broadcast `docker.application.gha_synced` (new event) with the
    workload ID + commit SHA so the UI subtab refreshes.

### Idempotency

`workload_id` is the dedup key. Re-running re-mints the token only on
explicit rotate; default re-sync keeps the existing token and only
re-PUTs the workflow if the SHA drifted.

The token-mint vs no-mint behaviour is controlled by a payload field:

```go
type GHABootstrapWorkflowPayload struct {
    WorkloadKind string `json:"workload_kind"`
    WorkloadID   string `json:"workload_id"`
    RotateToken  bool   `json:"rotate_token,omitempty"`
}
```

### Gate

Manual: enable GHA on a real repo (test repo on user's account),
verify in the GitHub UI:
- `.github/workflows/launch-deploy.yml` exists with the expected content
- `LAUNCH_DEPLOY_TOKEN` secret is set (mask visible, can't read value)
- `LAUNCH_APP_ID` + `LAUNCH_WEBHOOK_URL` variables are set
- `source_config.gha_workflow_sha` populated in DB

---

## Slice F — Webhook → deploy wire-up

### Touches

`internal/modules/docker/handlers/gha_webhook_handler.go` from slice C —
swap the placeholder "row recorded, TODO enqueue" for the real
enqueue.

### Code

```go
// inside GHAApplicationDeploy, after the deployment row upsert:

installationToken, err := gitProvider.GetInstallationToken(ctx, src.InstallationID)
if err != nil {
    // log + 500. The webhook will retry; if GitHub App was uninstalled
    // we surface a banner via the broken-installation flow in slice I.
    return err
}

deployTask, err := jobs.NewDeployApplicationTask(jobs.DeployApplicationPayload{
    ApplicationID: app.ID,
    TeamID:        app.TeamID,
    DeploymentID:  deployment.ID,
    SourceType:    dockertypes.SourceTypeImage,
    Image:         payload.ImageTag,
    RegistryURL:   "ghcr.io",
    RegistryUsername: "x-access-token",
    RegistryPassword: installationToken,
})
if err := queue.Enqueue(deployTask); err != nil { ... }
return fiberctx.OK(c, "Deployment queued", map[string]any{"deployment_id": deployment.ID})
```

### Verification

Manual end-to-end on the user's test repo:
1. Enable GHA in the UI (slice G), which dispatches slice E.
2. Push a commit to the deploy branch.
3. GitHub Actions runs, builds the image, pushes to GHCR, calls our
   webhook with a bearer token.
4. Webhook records the deployment, enqueues `deploy_application`.
5. Worker SSHes into the server, `docker login ghcr.io`, `docker pull`,
   `docker run`. Container comes up.

### Gate

End-to-end deploy completes against a real server.

---

## Slice G — UI

### launch-nuxt files

- `components/docker/CreateApplicationSheet.vue` — extend with a
  "Build location" radio group (visible only when `source_type === 'git'`).
  Selecting "GitHub Actions" reveals the deploy-branch + Dockerfile-
  path inputs and a "Image will be published to …" preview line.
  Submit hits a new endpoint (or extends the existing create) that
  also enqueues the bootstrap job.
- `components/docker/CreateComposeSheet.vue` — same, with the compose-
  specific preview ("…:launch-{service}-{sha7}").
- `components/docker/application/GitHubActionsSubtab.vue` (new) —
  rendered under the app detail when `build_location === github_actions`.
  Shows workflow status, rotate-token, re-sync, last-run link.
- `components/docker/compose/GitHubActionsSubtab.vue` (new) — mirror.
- `components/docker/DeploymentListRow.vue` — add a `via GitHub Actions`
  badge when `trigger_source === 'github_actions'`, linking to
  `gha_run_url`.

### launch-go API changes for the UI

- `POST /docker/applications/:id/gha/enable` — toggle `build_location` +
  enqueue bootstrap. Returns the raw deploy token once.
- `POST /docker/applications/:id/gha/rotate-token` — enqueue bootstrap
  with `rotate_token: true`. Returns the new raw token once.
- `POST /docker/applications/:id/gha/resync` — enqueue bootstrap with
  `rotate_token: false`. No-op token, re-puts the file/secret/vars.
- `POST /docker/applications/:id/gha/disable` — clear
  `build_location`, delete secret + variables (D's primitives),
  prepend "(orphaned by Launch)" comment to the workflow file.

Compose mirrors.

### Gate

Click through every affordance on a dev instance; verify GitHub UI
reflects each action.

---

## Slice H — Compose path

### Differences from application

- **Workflow YAML** matrix step extracts services with a `build:` directive:
  ```yaml
  - id: discover
    run: |
      yq '.services | with_entries(select(.value.build != null)) | keys | .[]' \
        docker-compose.yml > /tmp/services.txt
      echo "services=$(jq -R -s -c 'split("\n") | map(select(length > 0))' /tmp/services.txt)" >> $GITHUB_OUTPUT

  - name: Build services
    strategy:
      matrix:
        service: ${{ fromJson(needs.discover.outputs.services) }}
    run: |
      docker compose build "${{ matrix.service }}"
      docker tag <service-image> ghcr.io/.../launch-${{ matrix.service }}-${GITHUB_SHA::7}
      docker push ghcr.io/...:launch-${{ matrix.service }}-${GITHUB_SHA::7}
  ```
  (The exact step shape lands during implementation — design's job is
  the contract, not the bash.)

- **Webhook payload** carries `service_images: map[string]string` —
  one image per built service. The compose webhook validates each
  image's prefix against `gha_image_repository`.

- **`deploy_compose` task** gains an optional `ServiceImages` field
  on its config. When populated, the renderer rewrites the `image:`
  directive on every matching service in the compose YAML before
  writing it to the build directory on the server. Nil → existing
  pull-and-up behaviour.

### Tests

- Golden test for the compose template matrix step.
- Unit test for the YAML rewriter (input compose + service_images map
  → expected output compose).
- Integration: compose webhook with service_images → deploy job runs
  → container fleet up.

---

## Slice I — Polish

### Token rotation
Subtab button → `POST /gha/rotate-token` → enqueue bootstrap with
`rotate_token: true` → returns new raw token (one-time visible).
UI toast renders the raw token in a copy field; closing the toast
permanently hides it.

### Re-sync
Idempotent: bootstrap with `rotate_token: false`. The workflow YAML
gets re-rendered and re-PUT only if the rendered bytes differ from
what's at `gha_workflow_sha`. Cheap no-op when in sync.

### Disable cleanup
- DELETE secret via D's primitive.
- DELETE both variables.
- PUT the workflow file with the original content prepended by a
  comment: `# (orphaned by Launch — this file is no longer managed)`.
- `UPDATE docker_applications SET build_location = 'server', gha_deploy_token_hash = NULL, source_config = source_config - 'gha_workflow_sha' WHERE id = $1`.

### Broken-installation banner
If `GetInstallationToken` fails with 404 / 410 (installation gone), the
bootstrap and webhook handlers both surface a `gha_installation_broken`
status on the workload and broadcast a `docker.application.gha_broken`
event. UI shows an amber banner: "GitHub App installation removed.
Reconnect from Connections to resume Actions-based deploys."

---

## Verification matrix

| Slice | Unit | Integration | Manual |
|---|---|---|---|
| A | model defaults, JSON roundtrip | — | `\d docker_applications` shows new cols |
| B | golden YAML | — | — |
| C | handler auth/idempotency | — | curl smoke |
| D | provider PUTs against fake transport | — | — |
| E | — | bootstrap_workflow against fake GitHub | enable on real test repo |
| F | — | end-to-end with fake registry | enable + push → app live |
| G | — | — | click-through every affordance |
| H | template golden, YAML rewriter | compose deploy against fake registry | compose push → multi-service up |
| I | rotate / re-sync / disable handlers | — | manual cycle: enable → rotate → re-sync → disable |

---

## Open follow-ups (not blockers — captured for tracking)

- `gha:prune_images` scheduled job, keep last N tags per workload.
- Multi-arch builds (`platforms` knob).
- Build-time env vars / secrets via `build_args` + `secrets` blocks.
- Live build-log streaming from GitHub Actions into the deploy detail.
- GitLab CI / Bitbucket Pipelines analogues (provider-specific
  bootstrap).
- OIDC-authenticated webhooks (replace bearer with short-lived OIDC).
- Manual deploy button (`workflow_dispatch` trigger from the UI).
