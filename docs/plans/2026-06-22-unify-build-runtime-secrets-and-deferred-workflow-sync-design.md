# Unify build/runtime secrets + deferred workflow sync — design

- **Status:** validated (brainstorm complete) — awaiting review before implementation
- **Date:** 2026-06-22
- **Spans:** `launch-nuxt` (Docker application/compose detail UI) + `launch-go` (docker module: build-secret service, GHA bootstrap/resync, app payload, WS)
- **References:** [Build with GitHub Actions — design](2026-05-27-github-actions-builds-design.md); `internal/modules/docker/services/build_secret_service.go`; `internal/modules/docker/jobs/gha_bootstrap_workflow.go`; `components/application/{GHA,Environment,Advanced}.vue`.

---

## 1. Goal

Three changes to the Docker application (and compose) detail experience:

1. **Unify secrets in one place.** Today runtime env vars live on the **Environment** tab and build-time secrets live on a separate **GitHub Actions** tab. Put both in the **Environment** tab as two clearly-separated sections — *Runtime* and *Build-time* — so "what runs in the container" vs "what the image is built with" is one screen.
2. **Stop committing the workflow YAML on every change.** Today every build-secret create/update/delete immediately re-renders **and commits** `.github/workflows/launch-deploy-<app>.yml` via `queueResyncIfGHA()`. Instead, build-secret edits stay local; the workflow is reconciled only when the user clicks **Re-sync workflow**, with a banner showing how many changes are pending.
3. **Fold the rest of the GitHub Actions tab into Advanced** and remove the standalone tab.

## 2. Current state (what exists)

- **Tabs** (`pages/.../applications/[applicationId]/index.vue`): general · deployments · **Environment** · domains · redirects · volumes · schedules · **GitHub Actions** (only when `build_location === "github_actions"`) · logs · **Advanced**.
- **Runtime env vars** — `ApplicationEnvVar`, `docker run -e`, maskable, `${{project.*}}` refs. Endpoint `/env-vars`. UI: `Environment.vue` → `shared/EnvVarsEditor.vue`.
- **Build-time secrets** — `ApplicationBuildSecret`, `docker build --mount=type=secret`, **write-only**, pushed to GitHub as `LAUNCH_BUILD_<NAME>`, rendered into the workflow `secrets:` block. Endpoint `/build-secrets`. UI: `GHA.vue` → `shared/BuildSecretsEditor.vue`.
- **GHA tab** (`GHA.vue`): read-only config (repo/branch/workflow/image), build-secrets editor, maintenance (rotate token, **re-sync workflow**, auto-deploy toggle, disable GHA), status badge, "GitHub access lost" banner.
- **Sync today:** `build_secret_service.go` calls `queueResyncIfGHA()` on every CRUD → enqueues the bootstrap job → pushes the secret + **commits the YAML**. `source_config.gha_workflow_sha` stores the last committed *commit* SHA; there is **no content hash / dirty flag**.

## 3. Decisions captured during brainstorming

| Question | Decision |
|---|---|
| Where do build-time + runtime secrets live? | **Environment tab**, two separated sections. |
| What happens to the rest of the GHA tab? | Moves to **Advanced** as a "GitHub Actions" card; GHA tab removed. |
| On saving a build secret, what hits GitHub? | **Nothing immediately — defer everything to Re-sync.** Saving writes Launch's DB only; Re-sync pushes all secret values *and* commits the YAML atomically. |
| Banner | "Workflow out of sync — N pending changes" + **Re-sync workflow** button, on the Build-time section (GHA apps only). |
| Non-GHA (server build) build secrets | Still saved to DB, read at deploy time; same Build-time section, **no banner**. |

## 4. UI changes (launch-nuxt)

### Environment tab — two sections
`Environment.vue` renders:
- **Runtime** — existing `EnvVarsEditor` unchanged. Caption: *"Injected when the container runs (`docker run -e`)."*
- **Build-time** — `BuildSecretsEditor` relocated here. Caption: *"Available only while the image builds (`docker build --mount`)."* For `build_location === github_actions`, this section shows the **out-of-sync banner** (see §6) and the **Re-sync workflow** button.

Visual separation: two distinct cards/headers, not interleaved rows.

### Advanced tab — GitHub Actions card
`Advanced.vue` gains a card (only when `build_location === github_actions`), relocating from `GHA.vue`: workflow status + synced commit, repo·branch·image (read-only), **rotate token**, **deploy-on-push toggle**, **re-sync workflow**, **disable GitHub Actions** (danger), and the existing "GitHub App access was lost" banner.

### Removals
- Drop `"gha"` from the tab arrays in `index.vue` and `Navbar.vue`.
- `GHA.vue` is gutted — build-secrets section → Environment; everything else → Advanced. Remove the component once both homes exist.

## 5. Backend changes (launch-go)

1. **Remove auto-sync.** Delete the `queueResyncIfGHA()` calls in `CreateBuildSecret/UpdateBuildSecret/DeleteBuildSecret`. Build-secret writes now only persist + broadcast.
2. **Dirty tracking.** Add `gha_last_synced_at` (timestamp) to the workload. `gha_pending_changes` is derived: count of build secrets with `updated_at > gha_last_synced_at`, plus any build secret soft-deleted since `gha_last_synced_at`. (A `created < last_synced` then `updated > last_synced` secret counts once.)
3. **Template drift.** Store `gha_workflow_content_hash` (hash of the last committed rendered YAML). `out_of_sync` is true when `gha_pending_changes > 0` **OR** a freshly rendered YAML hash ≠ `gha_workflow_content_hash` (catches Launch-side template changes after a platform update, branch change, etc.).
4. **Payload.** The application GET response (and compose) expose `gha_out_of_sync: bool` and `gha_pending_changes: int`.
5. **Re-sync = existing bootstrap job**, unchanged in behavior: it already pushes every build secret as `LAUNCH_BUILD_<NAME>` and commits the rendered YAML. On success it now additionally sets `gha_last_synced_at = now`, recomputes/stores `gha_workflow_content_hash`, and broadcasts `gha_synced` (already emitted).

## 6. WebSocket + data flow

New event **`docker.application.gha_out_of_sync`** (team channel), carrying `team_id` + `application_id` + `pending_changes`. Per the WS rules it must be added to the broadcaster *and* `launch-nuxt/composables/useChannelEvents.ts`. Compose gets the analogous `docker.compose.gha_out_of_sync`.

```
Edit build secret ─▶ persist row, broadcast gha_out_of_sync(pending=N)
                         │
                         ▼
        Environment ▸ Build-time banner: "Workflow out of sync — N pending changes"
                         │  user clicks Re-sync
                         ▼
        enqueue gha:bootstrap_workflow (RotateToken=false)
            push LAUNCH_BUILD_<NAME> for each secret · commit YAML
                         │ success
                         ▼
        set gha_last_synced_at, store content hash, broadcast gha_synced ─▶ banner clears
```

## 7. Edge cases & error handling

- **Value-only edit** still counts as pending (the value isn't in GitHub until re-sync) — consistent with "defer everything."
- **Delete** a build secret → row soft-deleted, pending++; re-sync removes it from GitHub secrets *and* the YAML `secrets:` block.
- **Re-sync fails** (permissions missing / installation gone) → pending stays set; surface the existing broken-install banner; do not advance `gha_last_synced_at`.
- **Auto-deploy toggle / rotate token** remain immediate (each is itself a deliberate single sync, and the bootstrap run also flushes any pending build secrets, clearing the banner).
- **Non-GHA apps** never compute out-of-sync; build secrets read at deploy time.

## 8. Testing

- **Backend unit:** `gha_pending_changes` derivation (create/update/delete vs `gha_last_synced_at`); `out_of_sync` true on content-hash drift with zero edits; re-sync resets pending + stores hash; existing workflow-render golden tests still pass.
- **Frontend:** banner appears on `gha_out_of_sync` with the right count and clears on `gha_synced`; Re-sync button hits the resync endpoint; Environment renders both sections; Advanced renders the GHA card only for GHA apps; non-GHA apps show Build-time with no banner.

## 9. Out of scope / open items

- Applying the same defer-to-resync model to the **auto-deploy toggle** (kept immediate for now).
- Exposing build-time secrets for **server-build** apps was already supported by the endpoint; this surfaces the section but adds no GitHub behavior for them.
- Compose parity is included in the data model but the UI work mirrors the application changes; track as a sibling slice if it grows.
