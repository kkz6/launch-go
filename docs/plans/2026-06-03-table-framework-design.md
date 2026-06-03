# Server-Driven Table Framework — Design

**Date:** 2026-06-03
**Status:** Approved design, ready for implementation plan
**Source:** Port of `github.com/GlobalJapan/gj-hrms-api/pkg/table` (backend) +
`gj-hrms-web/app/components/data-table` (frontend) into launch-go / launch-nuxt.

## Goal

Adopt a Filament/Nova-style server-driven table framework: declare a table's
columns, filters, sorting, search, and actions in Go; the framework returns a
JSON schema (`/meta`) + paginated/sorted/filtered/searched data (`/data`); a
generic Vue `<DataTable>` renders it. Convert the admin-panel tables (Users,
Servers, Failures, Invitations) to use it. Full framework, both ends.

## Why / what exists

- gj `pkg/table` is **Fiber v2 + GORM** (same stack as launch-go) — 13 files,
  self-contained except three internal deps.
- It is **model-driven**: `Config.Model()` → `SELECT <cols> FROM <one table>`.
  No scope/join/custom-query hook exists.
- The matching frontend is `gj-hrms-web/app/components/data-table/*` (Nuxt 4,
  Vue 3.5, **shadcn-vue on reka-ui**) — a renderer + `useTable.ts` composable +
  cell/filter/action subcomponents.
- launch-nuxt is Nuxt 3, Vue 3, **also shadcn-vue/reka-ui**, and already has
  `useApi()` (the wrapper `useTable` expects) and all needed UI primitives
  (button, dropdown-menu, input, badge, dialog, …). It also has an unrelated
  `components/shared/DataTable.vue` (client-side, local columns) used by ~10
  server/site pages — left untouched.

## Decisions

1. **Full framework, both ends** — port the Go package and the frontend suite.
2. **Extend the framework** with custom-query hooks + secret-safe projection so
   our non-model tables (Users, Failures) fit; all admin tables use one
   component.

## Backend

### 1. Port `internal/pkg/table` + adapt deps
Copy the 13 files to `internal/pkg/table`. Map the three gj-internal deps:

| gj dep | use | launch-go replacement |
|---|---|---|
| `pkg/fiber` (`fiberctx`) | `OK`/`Created`, `GetUserID`, `MustParseAndValidate` | `internal/pkg/fiber` (has `OK`, `MustParseAndValidate`; add `Created` if missing; user id via `c.Locals(fiberctx.KeyUserID)`) |
| `pkg/apperror` | `ErrNotFound`/`ErrBadRequest.WithMessage` | `fiberctx.RespondNotFound`/`RespondBadRequest` or `fiber.NewError` |
| `pkg/models` | BaseModel (ID/DeletedAt) + saved-view model | `internal/pkg/models` (`basemodels.BaseModel`) |

HTTP surface unchanged: `table.Mount(group.Group("/table"), tableImpl,
queryService, viewService)` → `/meta`, `/data`, `/action/:name`, `/views/*`.
`QueryService(db)` and `ViewService(db)` constructed once at boot, shared.

**Saved views** (`/views`) need a `table_views` table + model + migration
(per-user saved filter/sort presets). Included as the full framework, but it is
the one OPTIONAL sub-feature — gated by a plan toggle; can be cut (YAGNI) for the
admin first cut by stubbing the `/views` endpoints to no-op/empty.

### 2. Extend the framework
Add two optional capabilities detected via Go interface assertions (existing
simple tables untouched):

- `BaseQueryProvider`: `BaseQuery(db *gorm.DB) *gorm.DB` — supplies a
  scoped/joined base query that `QueryService.Execute` uses instead of
  `db.Model(cfg.Model())`.
- `Resolver`: `Resolve(ctx, req Request) (*TableResponse, error)` — the table
  fully owns its data fetch; `Execute` delegates entirely. Declared
  columns/filters still drive `/meta`. Resolvers reuse existing staff services
  (`ListUsersWithBilling`, `Failures`) — no duplicated logic.

**Declared-column projection (secret safety):** change `fetchRows` to `SELECT`
only the declared columns (+ PK, + `deleted_at` when soft-deletes) instead of
`SELECT *`. The declared column set IS the allow-list — a model-driven Servers
or Invitations table can never surface an undeclared secret column. This retires
the need for the `ServerSummary` DTO on the table path.

## Frontend

Port `gj-hrms-web/app/components/data-table/*` into launch-nuxt
`components/data-table/*` (separate from `shared/DataTable.vue`). ~25 files:
- `DataTable.vue` + `SearchInput`, `TablePagination`, `ToggleColumnsDropdown`,
  `EmptyState`, `ConfirmDialog`, `ExportButton`/`ExportProgressOverlay`.
- `columns/`: `CellRenderer` + Text/Badge/Boolean/Date/DateTime/Image/Numeric/
  Action cells.
- `filters/`: `ActiveFilters`, `AddFilterDropdown`, `FilterChip` + Text/Numeric/
  Boolean/Date/Set inputs.
- `actions/`: `RowActions`, `BulkActionsDropdown`.
- `composables/useTable.ts`, `types/data-table/*`.

Adjustments: Nuxt 4 `app/`+`~/` → Nuxt 3 root paths; shadcn UI import repointing;
icons → `<Icon name="lucide:…">`; reconcile `useApi().get` signature + the
`/data` query-string contract (`?page&limit&sort=col:dir&search&
filters[k][clause]=v&columns=`). `useTable(endpoint)` already fetches `/data`.

Result: `<DataTable endpoint="/admin/users/table" />` renders columns,
header-click sort, search, filter chips, column toggle, pagination, row/bulk
actions — all from `/meta` + `/data`.

## Converting the admin tables

Declarative defs under `internal/modules/staff/tables/`, mounted at
`/admin/<name>/table`:

| Table | Backend | Columns / actions |
|---|---|---|
| **Invitations** | model-driven (`platform_invitations`) | email, trial_ends_at, expires_at, accepted; action Revoke (super_admin) |
| **Servers** | model-driven (`servers`) + declared-column projection | name, provider, public_ipv4, status, created_at |
| **Failures** | `Resolve` → `Failures` service | kind, title, when, error; row View; `kind` Set filter |
| **Users** | `Resolve` → `ListUsersWithBilling` | name, email, teams (nested `teams[]` badge), staff_role, status, created_at; actions Spectate / Suspend·Unsuspend / Delete (super_admin, confirm) |

Actions dispatch to existing staff service methods (`SuspendUser`, `DeleteUser`,
`RevokeInvitation`, impersonation start). **Authorization stays server-side**:
mutating action routes require `RequireStaff(super_admin)`; action
`Hidden`/`Disabled` fns hide the buttons for non-super-admins (UX only).
**Overview is NOT a table** — it stays the custom dashboard.

Frontend admin pages become thin: `pages/admin/index.vue` →
`<DataTable endpoint="/admin/users/table">`, etc. The hand-rolled v2-T13 tables/
pagination are replaced; lifecycle dialogs fold into table actions
(ConfirmDialog from the suite).

## Sequencing

1. Port `internal/pkg/table` + adapt deps (compiles, `parsing_test.go` ported).
2. Extend framework (BaseQuery/Resolve detection + declared-column projection) + tests.
3. (optional) Saved-views migration + ViewService.
4. Port frontend suite + `useTable` (typechecks; a smoke render).
5. Convert tables simplest-first: Invitations → Servers → Failures → Users,
   each = a Go table def + Mount + swap the Vue page to `<DataTable>`.
6. E2E per table (sort/search/filter/paginate/action) + finish both repos.

## Testing

- Go: BaseQuery/Resolve detection; declared-column projection excludes
  undeclared (secret) columns; action dispatch + authz; ported request-parsing
  tests.
- Frontend: typecheck; per-table browser E2E (sort, search, filter, paginate,
  row action).

## Out of scope (YAGNI)

- Converting the ~10 `shared/DataTable.vue` client-side tables (server/site
  pages) — left as-is; the framework is available if wanted later.
- Export to CSV/Excel beyond what the ported `ExportButton` provides out of the
  box (wire it only if trivial).
- Saved views may be cut for the first cut (toggle).
