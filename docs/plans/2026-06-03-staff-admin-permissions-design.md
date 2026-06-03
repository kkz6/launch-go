# Staff Admin & Permissions Refactor — Design

**Date:** 2026-06-03
**Status:** Approved design, ready for implementation plan

## Problem

We need a **product-wide admin** for internal staff (us), distinct from the
customer-facing team roles that let a customer invite teammates into their
account. Today there are two tangled concerns:

1. **Tenant authorization** — customer team roles (`owner`/`admin`/`editor`/
   `member`) stored in `team_user.role`, enforced by `RequireRole`. These work
   well and are **unchanged** by this design.
2. **Staff authorization** — a leftover Spatie permission system (5 tables:
   `roles`, `permissions`, `model_has_roles`, `role_has_permissions`,
   `model_has_permissions`) that, in the Go codebase, is queried in only **two**
   places — both just checking for a global `admin`/`manager` to bypass
   subscription checks. No admin panel, no management, no real use.

The Spatie infrastructure is dead weight. We are **not** refactoring onto a
bigger system — we are removing it in favor of a minimal native concept.

## Decisions (from brainstorming)

- Staff access is a **nullable `staff_role` column on `users`**, not a
  dedicated table and not Spatie. Two fixed tiers, never edited at runtime.
- Two tiers: **`support`** and **`super_admin`**.
  - `support`: view all customer data, view logs, impersonate (read-only).
  - `super_admin`: everything above + manage product config / billing plans.
- **Drop all 5 Spatie tables** after repointing the two queries.
- **Impersonation** ("spectate as user") is read-only by default and audited
  via a dedicated `impersonation_sessions` log.
- **No "manage staff" UI** — `staff_role` is set directly in the DB/seeder
  (YAGNI; additive later if ever needed).

## The two-axis model

Staff access and team membership are independent axes that never touch.

| | Tenant axis (customer) | Staff axis (employees) |
|---|---|---|
| Answers | "What can this person do inside their team?" | "Can this person access the back-office?" |
| Storage | `team_user.role` (per team) | `users.staff_role` (per person, global) |
| Values | owner / admin / editor / member | null / support / super_admin |
| Enforced by | `RequireRole(tier)` on `/teams/...` | `RequireStaff(tier)` on `/admin/...` |
| Set by | Customers via invite UI | Us, via seeder/DB |

A person can sit on both axes simultaneously without interference (e.g. a
`super_admin` who also `owns` a personal test team; a `support` employee in no
team at all). Customer flows never read `staff_role`; `/admin` routes never
read team roles.

### Safety verification (done before design sign-off)

- `team_user.role` is a plain nullable `varchar` with its own Go enum —
  **independent of Spatie**. Dropping Spatie does not affect customer roles.
- The only foreign keys touching the Spatie tables are **among the 5 tables
  themselves**. No external table references them; they drop cleanly as a unit.
- Exactly **two** Go queries read the Spatie tables:
  `internal/middleware/subscription.go:88` and
  `internal/modules/auth/repositories/repository.go:86`. Both must be repointed
  before the drop.

## Data model

Add to `users`:

```
staff_role  VARCHAR(20)  NULL    -- null | 'support' | 'super_admin'
```

`null` = ordinary customer (the vast majority). No backfill required.

New table `impersonation_sessions`:

```
id             ulid        PK
staff_id       char(26)    FK users.id   -- who spectated
target_user_id char(26)    FK users.id   -- who they viewed as
team_id        char(26)    NULL          -- tenant context, if scoped
started_at     timestamptz
ended_at       timestamptz NULL          -- null = still active
reason         text        NULL          -- optional "ticket #123"
```

Append-only audit log; answers "who accessed this customer's account, when,
why."

Drop (after queries repointed): `model_has_roles`, `role_has_permissions`,
`model_has_permissions`, `roles`, `permissions`.

## Staff enum + middleware

New module `internal/modules/staff`, mirroring the existing `TeamRole` pattern.

`internal/modules/staff/types/types.go`:

```go
type StaffRole string

const (
    StaffRoleSupport    StaffRole = "support"
    StaffRoleSuperAdmin StaffRole = "super_admin"
)

func (r StaffRole) IsValid() bool
func (r StaffRole) Level() int // support:1, super_admin:2

func (r StaffRole) CanViewCustomerData() bool    { return r.IsValid() }
func (r StaffRole) CanImpersonate() bool          { return r.IsValid() }
func (r StaffRole) CanManageProductConfig() bool  { return r == StaffRoleSuperAdmin }
func (r StaffRole) CanBypassSubscription() bool   { return r.IsValid() }
```

Capability methods (not raw string compares) at call sites, so a future tier
change is one edit.

`internal/middleware/staff.go`:

```go
func RequireStaff(minRole staff.StaffRole) fiber.Handler
```

Loads `users.staff_role`, 403s if null or below `minRole`. Add a parallel
`StaffChain(minRole)` in `chains.go` for a consistent auth → staff → handler
stack.

Both repointed queries collapse to a column read, deleting the polymorphic
`model_type IN ('Modules\\Auth\\Models\\User', ...)` Laravel cruft:

```go
var role *staff.StaffRole
db.Model(&User{}).Select("staff_role").Where("id = ?", userID).Scan(&role)
return role != nil && role.CanBypassSubscription()
```

## Impersonation flow ("spectate as user")

JWT-based, read-only by default, fully audited.

1. **Start** — `POST /admin/impersonate/:userId` (`RequireStaff(support)`):
   verify target, write an `impersonation_sessions` row, mint a short-lived
   (~30 min) JWT:

   ```
   sub: <target_user_id>          // app behaves as the customer
   team_id: <target's team>       // tenant context
   impersonator_id: <staff_id>    // who is really driving (audit + exit)
   impersonation_sid: <session_id>
   read_only: true                // spectate, not act
   ```

   The token *is* the target user to every existing handler — no per-endpoint
   changes.

2. **Read-only enforcement** — middleware: if token carries
   `impersonation_sid` and `read_only: true`, reject non-safe methods
   (`POST/PUT/PATCH/DELETE`) with 403. One guard instead of auditing every
   write path. (A `read_only: false` super-admin variant is a future option;
   default-deny is the safe start.)

3. **Exit** — `POST /admin/impersonate/stop`: stamp `ended_at`, frontend
   discards the impersonation token and restores the staff session.

4. **Frontend** — when the active token has `impersonator_id`, render a
   persistent non-dismissible banner: "Viewing as {name} — Exit".

5. **Auditability** — `impersonator_id` rides in the token, so every action is
   attributable to the real staff member even if writes are ever enabled.

## Admin surface

Backend `/admin/*` group, separate from `/teams/*`:

```
/admin                          RequireStaff(support)      // group guard
  GET  /admin/users
  GET  /admin/teams
  GET  /admin/servers
  GET  /admin/servers/:id/logs
  POST /admin/impersonate/:userId
  POST /admin/impersonate/stop
  GET  /admin/config            RequireStaff(super_admin)
  PUT  /admin/config            RequireStaff(super_admin)
```

Frontend: an `/admin` section in the **same** Nuxt app (no separate app).
Route middleware on `/admin/**` checks `staff_role` and redirects non-staff
(defense-in-depth; backend is the real gate). Impersonation banner in the root
layout. Config UI renders only for `super_admin`. **No manage-staff UI.**

## Migration order, rollout & testing

1. **Migration A** — add `users.staff_role` + create `impersonation_sessions`;
   seed existing global admins into `staff_role='super_admin'` so nobody loses
   access.
2. **Code change** — add `staff` module, repoint the two Spatie queries, add
   `/admin` routes + impersonation. Deploy. Nothing reads Spatie tables now.
3. **Migration B** — drop the 5 Spatie tables (junction tables first, then
   `roles`/`permissions`). Up-only, no down method (per repo convention).

The drop is a separate later migration so a problem in step 2 can be rolled
back without having lost the tables.

Tests (Go table tests):

- `StaffRole`: validity, level ordering, each capability method.
- `RequireStaff`: null → 403; support on super-admin route → 403; super-admin →
  pass.
- Impersonation: start writes session row + correct token claims; read-only
  middleware blocks mutating methods; stop stamps `ended_at`.
- Subscription bypass regression: a `super_admin` still bypasses after repoint.
- Separation guarantee: a customer `owner` with `staff_role=null` gets 403 on
  `/admin/*`; a `support` staff in no team reaches `/admin/*`.

## Explicit non-goals (YAGNI)

- No runtime-editable permissions / granular RBAC.
- No staff-managing-staff UI.
- No act-as (write) impersonation initially — read-only spectate only.
- No separate admin frontend app.
