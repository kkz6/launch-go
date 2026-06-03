# Admin Panel v2 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Extend the staff admin panel into a business back-office: revenue/MRR overview, users-with-billing, failed-task monitor, and account lifecycle (invite-with-trial, suspend, delete), plus letting staff use the product without a paid subscription.

**Architecture:** Backend in `launch-go` (Fiber + GORM, `staff` module + `Auth()` chokepoint). Frontend in `launch-nuxt` (`launch-go/client`, Nuxt 3). New schema: `users.status`, `platform_invitations`. Reads gated to staff `support`; writes (invite/suspend/delete) to `super_admin`. Failures and revenue are surfaced from existing tables (no new failure store).

**Tech Stack:** Go, Fiber v2, GORM, golang-jwt/v5, testify, sqlite (tests). Nuxt 3, shadcn-vue, Tailwind.

**Reference design:** `docs/plans/2026-06-03-admin-panel-v2-design.md`

**Worktree:** `/Users/karthick/launch-app/launch-go/.worktrees/admin-v2` (branch `feature/admin-v2`). Frontend: `launch-go/client` (branch `feature/admin-v2` — create it). Postgres + Redis already running; psql at `/opt/homebrew/Cellar/libpq/18.4/bin/psql`; DB `launch_go`. `.env` present in the worktree.

**Conventions (verified):**
- Enums: mirror `internal/modules/staff/types/types.go` using `internal/pkg/enumtypes`.
- Migrations: `init()`→`Register(Migration{ID,Name,Timestamp,Up,Down})`, raw SQL for column adds, `Migrator().CreateTable` for new tables; latest ID `0058_06_03_000001`; new ones `0059`, `0060`. Run with `go run ./cmd/migrate migrate`.
- Staff module: `internal/modules/staff/{types,models,repositories,services,handlers,module.go}`; handlers use `fiberutil.SuccessWithMeta`/`OK`/`HandleError`, `fiberutil.ParseLimit/ParseOffset`, `dto.NewPaginationMeta`. Routes in `module.go` under `StaffChain(authMiddleware, StaffRoleSupport)`; add `RequireStaff(StaffRoleSuperAdmin)` on write routes.
- `Auth()` chokepoint: `internal/middleware/auth.go` — already hosts the impersonation write-block; add the suspended check there.
- Billing: `internal/modules/billing` (`subscriptions`, `orders` models/repos; `models/plan.go` plan config; `types` enums). `BillableTypeTeam = "Modules\\Auth\\Models\\Team"`.
- Email: `internal/modules/notification/channels` `EmailSender`; templates in `internal/pkg/mail/templates`.
- Frontend admin: `components/admin/Tabs.vue`, `pages/admin/*`, `services/adminService.ts`, `composables/useImpersonation.ts`, `middleware/staff.ts`.

All paths relative to the worktree root unless noted.

---

## Phase 1 — Schema & enums

### Task 1: `UserStatus` enum
**Files:** Create `internal/modules/auth/types/` addition or a new file; Test alongside.
- Add a `UserStatus` string enum (`UserStatusActive="active"`, `UserStatusSuspended="suspended"`) to `internal/modules/auth/types/types.go` (the file that holds `TeamRole`), mirroring the enum pattern (`IsValid`, `String`, `Scan`, `Value`, `AllUserStatuses`).
- Test: validity + the two values.
- Build + `go test ./internal/modules/auth/...`. Commit `feat(admin): add UserStatus enum`.

### Task 2: Migration — `users.status` + `platform_invitations`
**Files:** Create `internal/database/migrations/0059_06_03_000002_add_user_status_and_platform_invitations.go`.
- Up: `ALTER TABLE users ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'active'`; create `platform_invitations` via `Migrator().CreateTable` with model:
  `ID char(26) PK, Email varchar(255) not null index, Token varchar(64) not null uniqueIndex, TrialEndsAt timestamp not null, InvitedBy char(26) not null, AcceptedAt *timestamp, ExpiresAt timestamp not null, CreatedAt/UpdatedAt`.
  FKs intentionally omitted (comment), like `impersonation_sessions`.
- Down: drop table + drop column.
- Apply with `go run ./cmd/migrate migrate`; verify with psql (`\d users` shows status; `\dt platform_invitations`; `SELECT count(*) FROM users WHERE status='active'` = all rows). Commit.

### Task 3: Models — User.Status + PlatformInvitation + ToUserResponse staff is_subscribed
**Files:** Modify `internal/modules/auth/models/user.go`; Create `internal/modules/auth/models/platform_invitation.go`; Modify `internal/modules/auth/dto/responses.go`.
- Add `Status authtypes.UserStatus` field to `User` (gorm `column:status;type:varchar(20);default:active`) + helper `IsSuspended() bool`.
- `PlatformInvitation` model (matches migration columns), `TableName()="platform_invitations"`.
- In `ToUserResponseWithStatus`: also set `resp.StaffRole` (already done) and **override `is_subscribed`**: the `UserResponse` already carries the team's `is_subscribed` via `CurrentTeam`; add a top-level `IsStaffSubscribed`/adjust so that when `user.IsStaff()` the `CurrentTeam.IsSubscribed` is reported `true`. Simplest: in `ToTeamResponsePtrWithSubscription`, OR after building resp, if `user.StaffRole != nil && resp.CurrentTeam != nil { resp.CurrentTeam.IsSubscribed = true }`. Add a `Status` field to `UserResponse` too (`status` json) for the admin UI.
- Build + `go test ./internal/modules/auth/...`. Commit.

---

## Phase 2 — Staff: suspend enforcement

### Task 4: Suspended-user enforcement in `Auth()`
**Files:** Modify `internal/middleware/auth.go`, `internal/middleware/staff.go` (or a new `internal/middleware/user_status.go`); Modify `cmd/api/main.go` (wire loader); Test `internal/middleware/*_test.go`.
- Add a `UserStatusLoader func(userID string) authtypes.UserStatus` + `InitUserStatus(load)` (mirror `InitStaffMiddleware`). In `Auth()`, after `tryAuthenticate` succeeds and after the impersonation block, if `userStatus.load != nil && load(userID) == UserStatusSuspended` → `RespondForbidden(c, "Account suspended")`. Place so it covers every authenticated route.
- Wire `InitUserStatus` in `cmd/api/main.go` from a staff/auth service that reads `users.status` (a thin column read, mirror `StaffRoleForUser`; put `UserStatus(ctx,userID)` on the staff repo or auth repo).
- TDD: build a Fiber app with `Auth(secret, db)` (sqlite) seeding a suspended + an active user; suspended POST/GET → 403; active → 200. Mint JWTs inline.
- Also gate `Login`: in `auth_service.Login`, after password check, if `user.IsSuspended()` return an auth error "Account suspended". Add a service test.
- Build + tests. Commit `feat(admin): suspend enforcement at Auth chokepoint + login`.

---

## Phase 3 — Staff: read services & endpoints (Overview, Users+teams, Failures)

### Task 5: Users endpoint with teams + subscription status
**Files:** Modify `internal/modules/staff/repositories/repository.go`, `services/service.go`, `handlers/admin_handler.go`, `dto/` (new `user_with_teams.go`).
- Repo: `ListUsersWithBilling(ctx, limit, offset)` — page users (created_at desc), then batch-load owned teams (`teams.user_id IN (...)`) and current subscriptions (`subscriptions WHERE billable_id IN (teamIDs) AND status IN active/on_trial/past_due/...` newest per team). Avoid N+1.
- DTO `AdminUserRow{ id,name,email,staff_role,status,created_at, teams:[{id,name,personal_team, subscription:{status,trial_ends_at}|null}] }`.
- Service `ListUsersWithBilling` + handler `ListUsers` returns the new DTO with pagination meta. (Replace the old raw-User ListUsers, or add alongside and switch the route.)
- Test repo against sqlite (users+teams+subscriptions tables created), assert teams + subscription fold in. Build + tests. Commit.

### Task 6: Overview dashboard endpoint
**Files:** Create `internal/modules/staff/services/overview_service.go` (or methods on Service), `dto/overview.go`; Modify handler + `module.go` route `GET /admin/overview`.
- Build a plan-price map from `billing/models/plan.go` (product_id → monthly-equivalent cents; yearly id → price/12). Read how plans expose their ids/prices.
- Compute: MRR (Σ monthly-equiv for subs status in active/on_trial), total_revenue (Σ orders.total where status='paid'), active_count, trial_count, new_subs_this_month, cancelled_this_month, revenue_this_month, revenue_last_month, recent_payments (latest 10 paid orders: team name, total, currency, ordered_at), revenue_trend (last 6 months Σ paid totals).
- DTO with cents→dollars left to the frontend (return cents + currency). Handler returns it; route under `StaffChain(support)`.
- Test the aggregates against a seeded sqlite set (orders + subscriptions + a stub plan map). Build + tests. Commit.

### Task 7: Failures monitor endpoint
**Files:** Modify staff repo/service/handler; `dto/failure.go`; `module.go` route `GET /admin/failures`.
- Repo methods: `FailedProvisions` (`servers WHERE status='failed' OR (provision_error IS NOT NULL AND provision_error<>'')`), `FailedTasks` (`tasks WHERE status IN ('failed','timeout')`), `FailedDeployments` (`deployments d JOIN tasks t ON d.task_id=t.id WHERE d.status='failed'`). Each mapped to a normalized `AdminFailure{kind,id,title,team_id,server_id,when,error,detail}`. Truncate `output`/`detail` to ~2KB.
- Service merges + sorts newest-first, supports `?kind=provision|task|deployment`, paginates in-memory after a bounded per-source limit; include a `caveat` field noting asynq job failures are not captured. Handler + route (support tier).
- Test each source surfaces + kind filter + truncation, against sqlite. Build + tests. Commit.

---

## Phase 4 — Staff: lifecycle write actions (super_admin)

### Task 8: Suspend / Unsuspend endpoints
**Files:** staff service/handler; `module.go` routes `POST /admin/users/:id/suspend` and `/unsuspend` with `RequireStaff(StaffRoleSuperAdmin)`.
- Service `SetUserStatus(ctx, userID, status)` updates `users.status`. Guard: cannot suspend a user who holds a staff role (avoid locking out staff) — return an error. Handler validates id, returns the updated row.
- Tests: suspend sets status; unsuspend clears; suspending a staff user is rejected. Build + tests. Commit.

### Task 9: Delete user endpoint
**Files:** staff service/handler; `module.go` route `DELETE /admin/users/:id` (super_admin).
- Service `CanDeleteUser(ctx, userID) (bool, reason)`: false if user holds a staff role; false if any owned team has a `paid` order OR a subscription with status in `('active','past_due','unpaid','cancelled','expired')`. `DeleteUser` runs in a transaction (`db.Transaction`) deleting the user (cascades handle the rest); re-check the guard inside the tx.
- Handler: 409 + reason when not allowed; 200 on success.
- Tests: blocked when paid order / active-ever subscription / staff; allowed + row gone when clean (sqlite with cascade not enforced — assert the guard logic + that delete is called). Build + tests. Commit.

### Task 10: Invite-with-trial — invitation service + endpoints
**Files:** staff repo/service/handler for invitations; `dto/invitation.go`; `module.go` routes; a mail template.
- Repo: `CreatePlatformInvitation`, `ListPendingInvitations`, `FindInvitationByToken`, `MarkInvitationAccepted`, `DeleteInvitation`.
- Service `InviteUser(ctx, email, trialEndsAt, invitedBy)`: reject if a user with that email exists or a pending non-expired invite exists; create row with random token (`util` random/ULID-based, 32+ chars) + `expires_at = now + 14d`; send email via `EmailSender` with link `{config.App.FrontendURL}/register?invite={token}` using a new `PlatformInvitationEmail` template (mirror `TeamInvitationEmail`). Inject `EmailSender` into the staff module from main.go (it's already constructed there for other modules).
- Handlers: `POST /admin/invitations` (super_admin), `GET /admin/invitations` (support), `DELETE /admin/invitations/:id` (super_admin).
- Tests: invite creates row + calls fake sender; duplicate email/pending rejected. Build + tests. Commit.

### Task 11: Registration consumes the invite → on_trial subscription
**Files:** Modify `internal/modules/auth/services/auth_service.go` `Register` (+ its request DTO/handler) to accept an optional `platform_invite_token`; cross-module hook to billing to create the trial subscription.
- In `Register`: if a platform-invite token is provided and valid (exists, not expired, not accepted, email matches), after creating the user + personal team, create an `on_trial` subscription for the personal team: `status='on_trial'`, `trial_ends_at`=invite.TrialEndsAt, `billable_type`=Team, `billable_id`=personalTeam.ID, `provider='dodo_payments'`. Then mark invite accepted. Use the billing subscription repo (inject or via a small cross-module interface to avoid a cycle; mirror existing cross-module wiring like `SetServerLogReader`).
- Validate token via the staff/invitation repo (or move invitation repo to a shared place; simplest: a thin `PlatformInviteReader` interface the auth module is given).
- Tests: register with valid token → user + on_trial subscription on personal team + invite accepted; expired/used/email-mismatch token → registers WITHOUT trial (or rejects per design — choose: ignore invalid token, register normally, and report). Build + `go test ./internal/modules/auth/... ./internal/modules/billing/...`. Commit.

---

## Phase 5 — Frontend (launch-nuxt)

> Create branch `feature/admin-v2` in `launch-go/client` first. Mirror existing admin pages/components. Verify with `npx nuxi typecheck` and the running app (chrome-devtools), as in the prior epic. Commit incrementally.

### Task 12: Admin shell — tabs, types, services
**Files:** `components/admin/Tabs.vue` (update tabs to Overview/Users/Servers/Failures, drop Teams), `services/adminService.ts` (add `overview`, `failures`, `invitations` CRUD, `suspendUser`, `unsuspendUser`, `deleteUser`), `types/index.ts` (add `status` to User; admin row/overview/failure types). Delete `pages/admin/teams.vue`.

### Task 13: Admin pages
**Files:**
- `pages/admin/index.vue` (Users): render teams as chips with subscription-status badges; add per-row Suspend/Unsuspend + Delete (delete disabled w/ tooltip when not allowed — backend returns the flag/reason or UI calls and shows 409), all gated to super_admin in UI; an **Invite user** dialog (email + trial date) calling `POST /admin/invitations`; a pending-invites section.
- `pages/admin/overview.vue` (new, set as the default `/admin`? keep `/admin` = Users; add `/admin/overview`): cards (MRR, revenue, active, trials), recent payments table, simple revenue bars. Cents→USD formatting.
- `pages/admin/failures.vue` (new): unified failures table with kind filter + expandable error/detail; show the asynq caveat.
- Keep `pages/admin/servers.vue`.
- Frontend `is_subscribed` staff fix is backend-side (Task 3); verify in-app that a staff user can reach all pages.

### Task 14: Verify + finish
- `go build ./... && go test ./...` green in the worktree.
- Run API (6123, CORS for 6124) + Nuxt (6124); drive with chrome-devtools (mint super_admin JWT): Overview renders numbers; Users shows teams+subscription chips; suspend a test user then confirm their token is rejected (403) and unsuspend restores; Failures lists real failed tasks; Invite dialog creates an invite (check DB row + dev mail log); staff user reaches all app pages (is_subscribed bypass).
- superpowers:requesting-code-review on the whole diff; then superpowers:finishing-a-development-branch for both repos.

## Out of scope (do NOT build)
- Asynq job-failure persistence, retry-from-UI, failure alerting.
- Editing plans/prices in admin. Granular staff RBAC.

## Risks / watch-outs
- **Import cycles**: staff↔billing and auth↔billing for trial creation — use thin reader/creator interfaces wired in `cmd/api/main.go` (mirror `SetServerLogReader`), never direct cross-module repo imports.
- **MRR plan mapping**: subscriptions store `product_id`; the plan map must key on the same ids the plans expose (monthly vs yearly). Verify against `plan.go`.
- **Suspend check cost**: it runs on every request — keep it a single indexed `users.status` read or a small cache; do not load the whole user.
- **Delete cascades**: verified at the DB level (FKs OnDelete CASCADE); the transaction + guard prevent deleting paying/staff users. Never delete a staff user.
- **is_subscribed staff override**: only flip the reported value for staff; never create a real subscription row for them.
