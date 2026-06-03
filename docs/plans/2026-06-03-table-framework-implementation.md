# Server-Driven Table Framework Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Port the gj-hrms server-driven table framework (Go `pkg/table` + Vue `data-table` suite) into launch-go/launch-nuxt, extend it to support custom-query/resolver tables with secret-safe column projection, and convert the admin-panel tables (Invitations, Servers, Failures, Users) to use it.

**Architecture:** Backend `internal/pkg/table` (Fiber+GORM) serves `/meta` (schema) + `/data` (paginated/sorted/filtered/searched) + `/action/:name`; a generic Vue `<DataTable endpoint>` renders it via the `useTable` composable. Two added hooks (`BaseQueryProvider`, `Resolver`) let non-single-model tables plug in; `fetchRows` projects only declared columns (secret-safe). Admin tables become declarative Go defs reusing existing staff services.

**Tech Stack:** Go, Fiber v2, GORM, testify. Nuxt 3, Vue 3, shadcn-vue/reka-ui, `useApi()`.

**Reference design:** `docs/plans/2026-06-03-table-framework-design.md`

**Source to port FROM (read-only, another repo):**
- Backend: `/Users/karthick/gjc/gj-hrms-api/pkg/table/*.go` (13 files).
- Frontend: `/Users/karthick/gjc/gj-hrms-web/app/components/data-table/*`, `/Users/karthick/gjc/gj-hrms-web/app/composables/useTable.ts`, `/Users/karthick/gjc/gj-hrms-web/app/types/data-table/*`.
- A real backend table def to mirror: `/Users/karthick/gjc/gj-hrms-api/internal/modules/roles/tables/roles_table.go` + that module's `module.go`/`routes.go` (how Mount is wired).

**Worktree:** backend `/Users/karthick/launch-app/launch-go/.worktrees/table-framework` (branch `feature/table-framework`, base `main`). Frontend `/Users/karthick/launch-app/launch-go/client` (create branch `feature/table-framework`). Postgres+Redis running; psql at `/opt/homebrew/Cellar/libpq/18.4/bin/psql`; `.env` present in worktree. Node is v22 — if Nuxt fails on better-sqlite3, run `npm rebuild better-sqlite3` in the client.

**launch-go conventions (verified):**
- `internal/pkg/fiber` (alias `fiberctx`): `OK(c,msg,data)`, `Created(c,msg,data)`, `MustParseAndValidate[T](c)`, `RespondBadRequest(c,msg)`, `RespondNotFound(c,msg)`, `HandleError(c,err)`, `GetUserID(c)(string,error)`, `KeyUserID`.
- `internal/pkg/models` (`basemodels.BaseModel`): `ID char(26)` ULID + `BeforeCreate`.
- Staff module `internal/modules/staff/module.go` `RegisterRoutes(router, authMiddleware)`: `admin := router.Group("/admin", middleware.StaffChain(authMiddleware, StaffRoleSupport)...)`; super_admin routes add inline `middleware.RequireStaff(StaffRoleSuperAdmin)`. `deps.DB` is the `*gorm.DB`.
- Latest migration `0059_06_03_000002`.

All paths relative to the backend worktree unless noted.

---

## Phase 1 — Backend: port + extend the framework

### Task 1: Port `internal/pkg/table` (copy 13 files, adapt deps, stub views)
**Files:** Create `internal/pkg/table/{action,column,empty_state,enums,filter,handler,meta,query,registry,request,table,views}.go` + `parsing_test.go`.

Copy each file VERBATIM from `/Users/karthick/gjc/gj-hrms-api/pkg/table/<same>.go`, then adapt:
- `package table` stays.
- Replace import `github.com/GlobalJapan/gj-hrms-api/pkg/fiber` (`fiberctx`) → `github.com/kkz6/launch-go/internal/pkg/fiber`. The calls used — `fiberctx.OK`, `fiberctx.Created`, `fiberctx.MustParseAndValidate`, `fiberctx.GetUserID` — all exist with the SAME signatures (verify). 
- Replace `github.com/GlobalJapan/gj-hrms-api/pkg/apperror`: `apperror.ErrNotFound.WithMessage(s)` → `fiberctx.RespondNotFound`? No — those are returned as errors, not written. Use `fiber.NewError(fiber.StatusNotFound, s)` and `fiber.NewError(fiber.StatusBadRequest, s)` (launch-go's error middleware renders `*fiber.Error`). Confirm by reading how launch-go surfaces `fiber.Error`.
- Replace `github.com/GlobalJapan/gj-hrms-api/pkg/models` (`pkgmodels`): used in `column.go`/`query.go` for the embedded BaseModel (ID + `gorm.DeletedAt`) reflection in `extractDeletedAt`/soft-delete. Map to `github.com/kkz6/launch-go/internal/pkg/models` (`basemodels`). NOTE: launch-go `BaseModel` has `ID` + created/updated but may NOT have a `gorm.DeletedAt` — check. If soft-delete reflection needs `DeletedAt` and launch-go models don't have it, make the soft-delete path defensive (no-op when no `DeletedAt` field found — `extractDeletedAt` already returns "" if not found).
- **Stub saved-views** (avoid a migration for the first cut): replace `views.go`'s DB-backed `ViewService` with a no-op stub — `List` returns an empty slice, `Create` returns `fiber.NewError(501, "saved views not enabled")` (or a stored-in-memory minimal impl), `Delete` no-op. Keep the `ViewService` type + `NewViewService(db)` signature so `Mount` is unchanged. Add a `// TODO: persist saved views (table_views migration)` comment. (Real persistence is a documented follow-up.)
- Port `parsing_test.go` (the request-parsing test) as-is.

**Verify:** `go build ./internal/pkg/table/...` clean; `go test ./internal/pkg/table/...` passes (parsing test). `go vet`. Commit `feat(table): port server-driven table framework (internal/pkg/table)`.

### Task 2: Extend — `BaseQueryProvider` + `Resolver` hooks
**Files:** Modify `internal/pkg/table/table.go` (interfaces), `internal/pkg/table/query.go` (Execute branch). Test `internal/pkg/table/query_hooks_test.go`.
- Add interfaces:
  ```go
  // BaseQueryProvider lets a table supply a scoped/joined base query instead of
  // the default db.Model(cfg.Model()).
  type BaseQueryProvider interface { BaseQuery(db *gorm.DB) *gorm.DB }
  // Resolver lets a table fully own its data fetch (assembly/unions/pagination).
  type Resolver interface { Resolve(ctx context.Context, req Request) (*TableResponse, error) }
  ```
- In `QueryService.Execute`: FIRST, `if r, ok := t.(Resolver); ok { return r.Resolve(ctx, req) }`. Otherwise build the base query: `base := db.Model(cfg.Model()); if bq, ok := t.(BaseQueryProvider); ok { base = bq.BaseQuery(s.db) }` and use `base` where `db.Model(model)` was used for count/search/filter/sort/fetch. Keep existing behavior identical when neither hook is implemented.
- TDD: a tiny in-memory sqlite table type implementing `Resolver` returns a canned `TableResponse` → assert `Execute` returns it untouched; a type implementing `BaseQueryProvider` (e.g. `WHERE x=1` scope) → assert the scope is applied (seed rows, only scoped rows returned). 
**Verify:** build + `go test ./internal/pkg/table/...`. Commit.

### Task 3: Declared-column projection (secret safety)
**Files:** Modify `internal/pkg/table/query.go` (`fetchRows` / the select). Test add to `query_hooks_test.go`.
- Change the data fetch to `Select` only the declared columns' DB attributes (`col.Attribute()` for non-nested columns) + the model PK + (`deleted_at` when `cfg.SoftDeletes`). Build the select list from `t.Columns()`. For a `Resolver` table this path is skipped (it owns its fetch).
- TDD: a model-driven table over a struct with a `secret` column NOT declared → assert the `/data` rows (the `[]map[string]any`) do NOT contain `secret`, only declared columns. This is the secret-safety guarantee.
**Verify:** build + tests. Commit `feat(table): project only declared columns (secret-safe)`.

---

## Phase 2 — Frontend: port the data-table suite

### Task 4: Port `components/data-table/*` + `useTable` + types
**Files (frontend, `/Users/karthick/launch-app/launch-go/client`, branch `feature/table-framework`):**
- Create `components/data-table/**` by copying `/Users/karthick/gjc/gj-hrms-web/app/components/data-table/**` (DataTable.vue, SearchInput, TablePagination, ToggleColumnsDropdown, EmptyState, ConfirmDialog, ExportButton, ExportProgressOverlay, and `columns/`, `filters/`, `actions/` subfolders + their `index.ts`).
- Create `composables/useTable.ts` from the gj source.
- Create `types/data-table/*` (table, column, filter, action, pagination) from gj source; re-export from `types/index.ts` or keep namespaced.

**Adapt while copying:**
- Path aliases: gj Nuxt4 uses `~/` against `app/`; launch-nuxt Nuxt3 `~/` is the project root — repoint imports of `~/components/ui/*`, `~/composables/*`, `~/types/*` to launch-nuxt locations (all shadcn ui primitives exist: button, dropdown-menu, input, badge, dialog, checkbox, popover, etc. — map any missing one to the closest existing).
- Icons: gj icon usage → launch-nuxt `<Icon name="lucide:...">` (the codebase convention).
- Data layer: `useTable.ts` fetches `${endpoint}/data` via `useApi()`. Confirm launch-nuxt `useApi().get` returns `{ data, meta }` and adjust the destructuring. The query string contract is `?page&limit&sort=col:dir&search&filters[k][clause]=v&columns=` (matches the backend `ParseRequest`).
- Remove gj-specific features that don't port cleanly (e.g. export pipeline if it depends on a gj backend route) — keep `ExportButton` only if the backend `/data` supports it trivially; otherwise stub/hide it and note it.

**Verify:** `npx nuxi typecheck` → 0 errors (fix import/type breakages). `npx prettier --write` on the new files. A minimal smoke: a throwaway page using `<DataTable endpoint="...">` typechecks (don't commit the throwaway). Commit `feat(admin): port data-table component suite + useTable`.

---

## Phase 3 — Convert admin tables (backend def + Mount + frontend swap)

> Each table = a Go def under `internal/modules/staff/tables/` + a `Mount` in the staff module. The FIRST one (Invitations) also sets up the shared `QueryService`/`ViewService` construction + the mount pattern. Reuse existing staff services for Resolver tables. Reads gated by the group's `StaffChain(support)`; the `/action` route additionally gated by `RequireStaff(super_admin)` (see Task 5 wiring). Spectate is NOT a server action — it stays a frontend call to the existing `/admin/impersonate/:userId` flow.

### Task 5: Invitations table (model-driven) + framework wiring
**Files:** Create `internal/modules/staff/tables/invitations_table.go`; Modify `internal/modules/staff/module.go` (construct `QueryService`/`ViewService`, mount), `internal/modules/staff/services/...` (a `RevokeInvitation` already exists — reuse).
- Define `InvitationsTable` (mirror `roles_table.go`): `Config{Name:"invitations", Model: func() any { return &authmodels.PlatformInvitation{} }, DefaultSort: {expires_at desc}, Searchable: ["email"]}`. Columns: email (text, sortable, searchable), trial_ends_at (datetime), expires_at (datetime), accepted_at (datetime or boolean "Accepted") , an ActionColumn. Action: `Revoke` (button, destructive, confirm) with `Handle(func(ctx, ids) error { for id: svc.RevokeInvitation(ctx, id) })`.
- In `module.go` `RegisterRoutes`: construct `tableQuery := table.NewQueryService(deps.DB)` and `tableViews := table.NewViewService(deps.DB)` ONCE (store on the module or build inline). Mount under a super_admin-gated subgroup for actions but support for reads. Concretely:
  ```go
  invTable := tables.NewInvitationsTable(m.service)
  g := admin.Group("/invitations/table")              // reads: support (inherited)
  table.Mount(g, invTable, tableQuery, tableViews)     // /meta,/data,/action,/views
  ```
  Then ensure the `/action` route requires super_admin. SIMPLEST: extend `table.Mount` to accept optional `actionMiddleware ...fiber.Handler` applied to the `POST /action/:name` route (small change in `handler.go`/Mount); pass `middleware.RequireStaff(StaffRoleSuperAdmin)` for tables whose actions mutate. Update Task-1 Mount signature accordingly (or add `MountWithActionGuard`). Report the chosen approach.
- TDD: a Go test that the table renders meta (columns present) and that `Resolve`/model path returns invitation rows via the QueryService against sqlite (seed platform_invitations). Action authz is covered by the route middleware (assert via a small Fiber test that POST /action without super_admin → 403 — or rely on the existing RequireStaff tests).
**Verify:** build + tests; run API, `GET /admin/invitations/table/meta` (with super_admin token) returns columns; `/data` returns rows. Commit.

### Task 6: Servers table (model-driven + declared-column projection)
**Files:** Create `internal/modules/staff/tables/servers_table.go`; Modify `module.go` (mount).
- `Config{Name:"servers", Model: func() any { return &servermodels.Server{} }, DefaultSort:{created_at desc}, Searchable:["name","public_ipv4"]}`. Columns: name, provider, public_ipv4, status (badge), created_at (datetime). **Only safe columns declared** → the Task-3 projection guarantees no secret columns (private_key etc.) are selected/returned. No actions (or a "view" link). Mount at `admin.Group("/servers/table")`.
- TDD/verify: `/data` for servers returns ONLY the declared columns (assert no `private_key`/`provider_data` keys in the row maps) — this is the secret-safety E2E for a real model. Commit.

### Task 7: Failures table (Resolver)
**Files:** Create `internal/modules/staff/tables/failures_table.go`; Modify `module.go`.
- `FailuresTable` implements `Resolver`: `Resolve(ctx, req)` calls the existing `m.service.Failures(ctx, kind, limit, offset)` (map `req.PerPage`/page→limit/offset; map a `kind` filter from `req.Filters["kind"]`), then adapts `AdminFailuresResponse` + pagination into a `table.TableResponse` (rows as `[]map[string]any` with keys kind,title,when,error,detail,team_id,server_id; meta with pagination). Columns (for `/meta` only): kind (badge), title (text), when (datetime), error (text); a `kind` Set filter (provision/task/deployment); a row "View" action (frontend opens detail — or a column that exposes `detail`). 
- Config.Model can be nil for a pure Resolver (ensure Execute's Resolver branch runs BEFORE touching cfg.Model()).
- TDD: Resolve returns the failures rows + kind filter narrows the set (use the existing failures sqlite harness/services test as a reference; or a focused test of the adapter mapping). Commit.

### Task 8: Users table (Resolver, nested teams)
**Files:** Create `internal/modules/staff/tables/users_table.go`; Modify `module.go`.
- `UsersTable` implements `Resolver`: `Resolve` calls `m.service.ListUsersWithBilling(ctx, limit, offset)` and maps each `AdminUserRow` → a row map including the nested `teams` array (each `{name, personal_team, subscription:{status,trial_ends_at}}`). Columns (for `/meta`): name, email, teams (a custom/badge cell over the nested array — use a column type that the frontend renders as chips; if none fits, a `text`/`badge` column with `meta` describing the nested render, or add a small "teams" cell), staff_role (badge), status (badge), created_at. Actions: Suspend/Unsuspend + Delete (super_admin, via /action → `m.service.SetUserStatus` / `DeleteUser`). Spectate is a frontend row action (links to impersonation), NOT a server action.
- The teams nested rendering: reuse the v2-T13 chip logic in the frontend cell (Task 9). The backend just emits the nested `teams[]` in the row map.
- TDD: Resolve returns users with nested teams + correct status; Suspend/Delete actions dispatch to the service (assert via the service, or a Fiber test of /action). Commit.

### Task 9: Frontend — swap admin pages to `<DataTable>`
**Files (frontend):** Modify `pages/admin/index.vue` (Users), `pages/admin/servers.vue`, `pages/admin/failures.vue`, and the Invitations area (currently inside index.vue or its own). Possibly add small custom cell components for teams chips + the row actions (Spectate/Suspend/Delete/Revoke) wired to `adminService`.
- Replace each hand-rolled table (from admin-v2) with `<DataTable endpoint="/admin/<name>/table">`. The DataTable fetches `/meta`+`/data`, renders columns/sort/search/filter/pagination/actions.
- **Row actions**: the framework renders declared actions; the action's server dispatch hits `/action/:name`. For Spectate (frontend-only), register it as a client action that calls `useImpersonation().start(row.id)`. For Suspend/Delete/Revoke, the DataTable posts to `/action/:name` with the row id(s); on success refetch. Wire the ConfirmDialog (from the suite) for destructive actions.
- **Teams chips cell**: a small cell renderer (or reuse the suite's BadgeCell with per-team mapping) that renders the nested `teams[]` with subscription-status colors (port the v2-T13 `subscriptionBadge` mapping).
- Keep the Overview page as-is (not a table). Keep the Servers/Failures/Users pages' headers + AdminTabs; only the table body swaps.
- super_admin gating in the UI: hide Suspend/Delete/Invite/Revoke for non-super-admins (the actions' `Hidden` fn or a UI `v-if="isSuperAdmin"`), backend stays the real gate.
**Verify:** `npx nuxi typecheck` 0 errors; prettier. Commit `feat(admin): render admin tables via DataTable framework`.

---

## Phase 4 — Verify + finish

### Task 10: Full regression + browser E2E + finish
- `go build ./... && go test ./...` green in the worktree.
- Run API (6123, CORS for 6124) + Nuxt (6124). With a minted super_admin JWT, for EACH admin table (Invitations, Servers, Failures, Users): load the page, confirm the DataTable renders columns + rows, click a header to sort (verify order changes), type in search (verify filtering), apply a filter (Failures kind, Users/Servers a column filter), paginate, and run a row action (Revoke an invite / Suspend a user → verify DB + UI). Confirm Servers `/data` exposes no secret columns. Confirm Spectate still works (frontend impersonation).
- superpowers:requesting-code-review on the whole diff; then superpowers:finishing-a-development-branch for both repos.

## Out of scope (do NOT build)
- Saved-views persistence (stubbed; follow-up migration).
- Converting the ~10 `shared/DataTable.vue` client-side tables.
- Export pipeline beyond trivial.

## Risks / watch-outs
- **Action authz**: mutating `/action` must be super_admin-gated (Mount action-middleware param); reads support. Spectate is frontend-only (support).
- **Resolver-before-Model**: `Execute` must check `Resolver` before dereferencing `cfg.Model()` (nil for pure-resolver tables like Failures).
- **Secret projection**: the Task-3 select-only-declared-columns is the secret allow-list for model-driven tables — verify Servers `/data` never returns key/token columns.
- **Dep signature drift**: confirm launch-go `fiberctx.OK/Created/MustParseAndValidate/GetUserID` match the gj signatures the package calls; adapt call sites if not.
- **Frontend `useApi` envelope**: `useTable` expects `{data, meta}`; confirm launch-nuxt's wrapper shape and adjust.
- **Node 22 / better-sqlite3**: `npm rebuild better-sqlite3` if Nuxt dev fails.
