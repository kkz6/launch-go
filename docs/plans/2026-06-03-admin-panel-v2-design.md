# Admin Panel v2 — Design

**Date:** 2026-06-03
**Status:** Approved design, ready for implementation plan
**Builds on:** the staff-admin foundation (`staff_role`, `/admin` gate, impersonation) already on `main`.

## Goal

Turn the read-only admin panel into a back-office for running the business:
revenue dashboard, customer (user) management with billing visibility, a failed-
provision/task monitor, and account-lifecycle actions (invite-with-trial,
suspend, delete). Staff use the product without a paid subscription.

## Scope (8 features)

1. Fold teams into user rows (drop the separate Teams tab).
2. Show subscription status per team in user rows.
3. Earnings/MRR overview dashboard.
4. Failed-provision / failed-task monitor.
5. Invite a user with a trial period (email link, self-register).
6. Suspend a user (block login + freeze actions).
7. Delete a user (only if never had a paying/active subscription).
8. Staff don't need a subscription to use the product.

## Existing systems (verified)

- **Billing** (`internal/modules/billing`): `subscriptions` (per-team, `billable_type`
  Team; status `on_trial|active|paused|past_due|unpaid|cancelled|expired`,
  `trial_ends_at`, `renews_at`, `ends_at`, `product_id`); `orders` (revenue:
  `total` cents, `status` paid|pending|failed|refunded|disputed, `ordered_at`,
  `currency`); plans hardcoded in `billing/models/plan.go` (Hobby/Compact/Turbo,
  pricing in cents). Provider: Dodo Payments.
- **Failures (queryable today)**: `tasks` (`status` incl. `failed`/`timeout`,
  `output` encrypted, `exit_code`, `server_id`, `type`); `servers.provision_error`
  + `status='failed'`; `deployments` (`status`, `task_id` → join tasks);
  `script_executions` (`status` indexed, `output`, `exit_code`). Asynq job
  failures are Redis-only (out of scope; surfaced as a caveat).
- **Users/Teams**: `users` has NO status/suspend/trial/deleted_at. Subscriptions
  are per-team; a user's personal team has `personal_team=true`. Registration:
  `auth_service.Register` creates user + personal team. Team invites exist
  (`team_invitations`); no platform-level user invite.
- **Email**: `notification/channels` `EmailSender.Send`; team-invite email
  template pattern in `internal/pkg/mail/templates`.
- **Staff bypass**: `VerifySubscription`/`isUserAdmin` already let staff bypass
  the subscription requirement at the API.

## Data model changes (minimal)

1. **`users.status`** `VARCHAR(20) NOT NULL DEFAULT 'active'` — `active|suspended`.
2. **`platform_invitations`** (new):
   `id` ULID PK, `email`, `token` (random url-safe), `trial_ends_at` timestamp,
   `invited_by` char(26), `accepted_at` timestamp null, `expires_at` timestamp,
   `created_at`/`updated_at`. Index on `token` (unique) and `email`.

No new failures table — failures are surfaced from existing tables.

## Admin tab structure

Replaces current Users/Teams/Servers tabs with:
- **Overview** — earnings/MRR dashboard (new).
- **Users** — users + their owned teams (chips with subscription status) inline;
  row actions Spectate / Suspend / Delete; top-level Invite button. (Teams tab removed.)
- **Servers** — kept as-is.
- **Failures** — failed provisions/tasks monitor (new).

All under `StaffChain`. Reads = support tier. Writes (invite/suspend/delete) = `super_admin`.

## Feature designs

### 1+2. Users with teams + subscription status
`GET /admin/users` extended: for each user, include their **owned teams**
(`teams.user_id = user.id`) and each team's **current subscription** (status +
`trial_ends_at`). One batched query (load users page, then teams for those user
ids, then subscriptions for those team ids) — no N+1, no secrets. Response adds
`teams: [{id, name, personal_team, subscription: {status, trial_ends_at}|null}]`.
Frontend renders teams as chips with a colored status badge.

### 3. Overview dashboard — `GET /admin/overview`
Computed read-only:
- **MRR**: for each `active`/`on_trial` subscription, look up its plan by
  `product_id` (monthly vs yearly id) and add the monthly-equivalent price
  (yearly/12). Sum, in cents → dollars.
- **Total revenue**: Σ `orders.total` where `status='paid'`.
- **Active subscriptions** count; **on-trial** count; **new this month**;
  **cancelled/expired this month**.
- **Revenue this month** vs **last month** (Σ paid orders by `ordered_at` month).
- **Recent payments**: latest ~10 paid orders (team name, amount, currency, date).
- **Revenue trend**: last 6 months of Σ paid-order totals (for a small bar chart).
Service does plain SQL aggregates + a plan-price lookup map built from config.

### 4. Failures monitor — `GET /admin/failures?kind=`
Unified newest-first list, each item normalized to
`{kind, id, title, team_id, server_id, when, error, detail}`:
- **provision**: `servers WHERE status='failed' OR provision_error<>''`
  (error = `provision_error`).
- **task**: `tasks WHERE status IN ('failed','timeout')` (error = exit_code +
  truncated `output`; output is encrypted → decrypt via model read).
- **deployment**: `deployments WHERE status='failed'` joined to tasks for output.
Optional `kind` filter. Pagination. A response field notes asynq job failures
aren't included (Redis-only) so coverage isn't silently overstated.

### 5. Invite with trial — `POST /admin/invitations` (super_admin)
Body `{email, trial_ends_at}`. Creates `platform_invitations` row (random token,
`expires_at` ~14 days) and emails an invite link
(`{frontend}/register?invite={token}`) via `EmailSender` + a new template.
- `GET /admin/invitations` lists pending; `DELETE /admin/invitations/:id` revokes.
- **Registration consumes it**: extend `auth_service.Register` to accept a
  platform-invite token. On success (after user + personal team are created),
  create an `on_trial` subscription for the personal team:
  `status='on_trial'`, `trial_ends_at` = invite's date, `billable_type` Team,
  `billable_id` = personal team id, provider `dodo_payments`, no provider id yet.
  Mark invite `accepted_at`. The existing `isTeamSubscribed` (status in
  active/on_trial) then treats them as subscribed until the date.
- Frontend `/register` reads `?invite=` and passes it through signup.

### 6. Suspend / Unsuspend — `POST /admin/users/:id/suspend` · `/unsuspend` (super_admin)
Sets `users.status`. Enforcement at the **`Auth()` chokepoint** (same place as the
impersonation write-block): after `tryAuthenticate` resolves the userID, if that
user is `suspended`, reject with 403 on **every** authenticated request → freezes
actions instantly; infra untouched; reversible. `Login` also checks status and
returns a clear error. A small cache or a cheap indexed lookup keeps the
per-request check fast (mirror the staff-role loader pattern: a `UserStatusLoader`
injected into the middleware).

### 7. Delete user — `DELETE /admin/users/:id` (super_admin)
Allowed only if the user **never had a paying/active subscription**: no `orders`
with `status='paid'` for their owned teams AND no subscription whose status is in
`('active','past_due','unpaid','cancelled','expired')` for those teams (i.e. only
ever `on_trial` or nothing). Otherwise 409 with the reason; UI disables the button
and shows why. When allowed, delete in a transaction; existing DB cascades remove
their teams/servers/sites/keys/etc. The handler also defends: never delete a
user holding a global staff role (avoid self/peer lockout) unless explicitly forced.

### 8. Staff don't need a subscription
API bypass already done. Close the **frontend** gap: `ToUserResponseWithStatus`
reports `is_subscribed = true` when the user is staff (any `staff_role`), so the
Nuxt `auth.ts` gate (which restricts non-subscribed users to `/dashboard`) treats
staff as fully subscribed everywhere. No real subscription row is created.

## Enforcement / security summary

- New writes (invite, suspend, unsuspend, delete) require `super_admin`
  (`RequireStaff(super_admin)` on those routes); reads require support.
- Suspend + impersonation-read-only both enforced in `Auth()` — the one
  chokepoint every authenticated route passes through.
- Delete is guarded by the never-paid rule + staff-role guard + transaction.
- No secret fields leak: overview/failures/users return DTOs/explicit fields;
  server data continues through `ServerSummary`; task `output` is truncated.

## Testing

- Enum/helpers (UserStatus) unit tests.
- Migrations apply; seed unaffected.
- Middleware: suspended user → 403 on authenticated request and login; active → pass.
- Overview service: MRR/revenue aggregates against a seeded sqlite set.
- Failures service: each kind surfaced; kind filter; output truncation.
- Invite: create → email sent (fake sender) → register-with-token creates the
  on_trial subscription + marks accepted; expired/used token rejected.
- Delete: blocked when paid order / active-ever subscription exists; allowed and
  cascades when clean; staff user not deletable.
- Frontend: staff `is_subscribed=true` unlocks the app; admin tabs render;
  suspend/delete/invite gated to super_admin in UI.

## Out of scope (YAGNI)

- Asynq job-failure capture (Redis-only) — surfaced as a caveat, not stored.
- Retry-from-UI and failure alerting (could be a follow-up).
- Editing plans/prices from the admin (plans stay config-driven).
- Per-permission staff RBAC (still two fixed tiers).
