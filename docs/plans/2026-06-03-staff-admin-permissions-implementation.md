# Staff Admin & Permissions Refactor — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a product-wide staff admin (two tiers: `support`, `super_admin`) with read-only user impersonation, on a single nullable `users.staff_role` column, and delete the dormant Spatie permission tables — without touching customer team roles.

**Architecture:** Two independent authorization axes. Tenant authz stays in `team_user.role` + `RequireRole` (untouched). Staff authz is a new `users.staff_role` column read by a new `RequireStaff` middleware guarding an `/admin/*` route group. Impersonation issues a short-lived JWT (`sub`=target, `impersonator_id`=staff, `read_only`=true) audited in a new `impersonation_sessions` table. The 5 Spatie tables are dropped after their 2 query call-sites are repointed at `staff_role`.

**Tech Stack:** Go, Fiber v2, GORM, golang-jwt/v5, testify. Migrations via `internal/database/migrations` (`Register` + raw SQL). Enums via `internal/pkg/enumtypes`. Frontend: Nuxt (`launch-go/client`).

**Reference design:** `docs/plans/2026-06-03-staff-admin-permissions-design.md`

**Conventions observed in this repo:**
- Enums mirror `internal/modules/auth/types/types.go` (`TeamRole`) using `enumtypes` helpers + `Scan`/`Value`.
- Migrations: `init()` calls `Register(Migration{ID, Name, Timestamp, Up, Down})`; column adds use raw `db.Exec("ALTER TABLE ...")`. Latest existing ID is `0056_06_01_000001`; new ones are `0057`, `0058`, `0059`.
- Middleware reads `c.Locals("userID")`; respond helpers in `internal/pkg/fiber` (`RespondForbidden`, `RespondUnauthorized`).
- Run a single test: `go test ./internal/modules/staff/... -run TestX -v`. Build: `go build ./...`.

**Worktree:** `/Users/karthick/launch-app/launch-go/.worktrees/staff-admin` (branch `feature/staff-admin`). All paths below are relative to the repo root.

---

## Phase 1 — Staff role foundation

### Task 1: `StaffRole` enum

**Files:**
- Create: `internal/modules/staff/types/types.go`
- Test: `internal/modules/staff/types_test.go`

**Step 1: Write the failing test**

Create `internal/modules/staff/types_test.go`:
```go
package staff

import (
	"testing"

	"github.com/stretchr/testify/assert"

	stafftypes "github.com/kkz6/launch-go/internal/modules/staff/types"
)

func TestStaffRole_IsValid(t *testing.T) {
	assert.True(t, stafftypes.StaffRoleSupport.IsValid())
	assert.True(t, stafftypes.StaffRoleSuperAdmin.IsValid())
	assert.False(t, stafftypes.StaffRole("").IsValid())
	assert.False(t, stafftypes.StaffRole("owner").IsValid())
}

func TestStaffRole_Level(t *testing.T) {
	assert.Equal(t, 2, stafftypes.StaffRoleSuperAdmin.Level())
	assert.Equal(t, 1, stafftypes.StaffRoleSupport.Level())
	assert.Equal(t, 0, stafftypes.StaffRole("nonsense").Level())
}

func TestStaffRole_Capabilities(t *testing.T) {
	// support: view + impersonate + bypass subscription, but NOT config
	assert.True(t, stafftypes.StaffRoleSupport.CanViewCustomerData())
	assert.True(t, stafftypes.StaffRoleSupport.CanImpersonate())
	assert.True(t, stafftypes.StaffRoleSupport.CanBypassSubscription())
	assert.False(t, stafftypes.StaffRoleSupport.CanManageProductConfig())

	// super_admin: everything
	assert.True(t, stafftypes.StaffRoleSuperAdmin.CanManageProductConfig())
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/modules/staff/... -v`
Expected: FAIL (package `types` does not exist / build error).

**Step 3: Write minimal implementation**

Create `internal/modules/staff/types/types.go`:
```go
// Package types provides staff-authorization domain enums. Staff roles are a
// product-wide axis (who on our team can access the back-office), entirely
// separate from customer team roles in internal/modules/auth/types.
package types

import (
	"database/sql/driver"

	"github.com/kkz6/launch-go/internal/pkg/enumtypes"
)

// StaffRole represents an internal staff member's product-wide access tier.
// A nil/absent staff role means an ordinary customer (no back-office access).
type StaffRole string

const (
	StaffRoleSupport    StaffRole = "support"
	StaffRoleSuperAdmin StaffRole = "super_admin"
)

var allStaffRoles = []StaffRole{StaffRoleSupport, StaffRoleSuperAdmin}

var staffRoleLabels = map[StaffRole]string{
	StaffRoleSupport:    "Support",
	StaffRoleSuperAdmin: "Super Admin",
}

// staffRoleHierarchy ranks tiers; higher = more access.
var staffRoleHierarchy = map[StaffRole]int{
	StaffRoleSupport:    1,
	StaffRoleSuperAdmin: 2,
}

func AllStaffRoles() []StaffRole { return allStaffRoles }

func (r StaffRole) String() string { return string(r) }

func (r StaffRole) Label() string {
	return enumtypes.Label(r, staffRoleLabels, string(r))
}

func (r StaffRole) IsValid() bool {
	return enumtypes.IsValid(r, allStaffRoles...)
}

// Level returns the hierarchy rank (super_admin:2, support:1, invalid:0).
func (r StaffRole) Level() int { return staffRoleHierarchy[r] }

// Capability helpers — call sites ask these instead of comparing strings.
func (r StaffRole) CanViewCustomerData() bool   { return r.IsValid() }
func (r StaffRole) CanImpersonate() bool         { return r.IsValid() }
func (r StaffRole) CanBypassSubscription() bool  { return r.IsValid() }
func (r StaffRole) CanManageProductConfig() bool { return r == StaffRoleSuperAdmin }

func (r *StaffRole) Scan(value any) error          { return enumtypes.ScanString(r, value) }
func (r StaffRole) Value() (driver.Value, error)   { return enumtypes.ValueString(r) }

func ParseStaffRole(s string) (StaffRole, error) {
	return enumtypes.ParseEnum(s, allStaffRoles)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/modules/staff/... -v`
Expected: PASS (3 tests).

**Step 5: Commit**
```bash
git add internal/modules/staff/types/types.go internal/modules/staff/types_test.go
git commit -m "feat(staff): add StaffRole enum with capability helpers"
```

---

### Task 2: Migration A — add `staff_role` + `impersonation_sessions` + seed

**Files:**
- Create: `internal/database/migrations/0057_06_03_000000_add_staff_role_and_impersonation.go`

**Step 1: Write the migration**

> Note: this is schema/data, verified by running migrations + an assertion query rather than a Go unit test. There is no failing-test-first step; the verification step (3) is the gate.

Create the file:
```go
package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0057_06_03_000000_add_staff_role_and_impersonation",
		Name:      "Add users.staff_role, impersonation_sessions, seed existing admins",
		Timestamp: time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC),
		Up:        addStaffRoleAndImpersonationUp,
		Down:      addStaffRoleAndImpersonationDown,
	})
}

// impersonationSessionMigration is the audit log of every "spectate as user"
// session. Append-only: a row is written on start, ended_at stamped on exit.
type impersonationSessionMigration struct {
	ID           string     `gorm:"type:char(26);primaryKey"`
	StaffID      string     `gorm:"column:staff_id;type:char(26);not null;index"`
	TargetUserID string     `gorm:"column:target_user_id;type:char(26);not null;index"`
	TeamID       *string    `gorm:"column:team_id;type:char(26)"`
	StartedAt    time.Time  `gorm:"column:started_at;type:timestamp;not null"`
	EndedAt      *time.Time `gorm:"column:ended_at;type:timestamp null"`
	Reason       *string    `gorm:"column:reason;type:text"`
}

func (impersonationSessionMigration) TableName() string { return "impersonation_sessions" }

func addStaffRoleAndImpersonationUp(db *gorm.DB) error {
	// 1. Add nullable staff_role to users (null = ordinary customer).
	if err := db.Exec(
		"ALTER TABLE users ADD COLUMN staff_role VARCHAR(20) NULL",
	).Error; err != nil {
		return err
	}

	// 2. Create impersonation_sessions.
	if err := db.Migrator().CreateTable(&impersonationSessionMigration{}); err != nil {
		return err
	}

	// 3. Seed: anyone currently holding the Spatie global admin/manager role
	//    becomes super_admin so nobody loses back-office access at cutover.
	//    Guarded so it is a no-op if the Spatie tables are already gone.
	if db.Migrator().HasTable("model_has_roles") {
		if err := db.Exec(`
			UPDATE users SET staff_role = 'super_admin'
			WHERE id IN (
				SELECT mhr.model_id FROM model_has_roles mhr
				JOIN roles r ON r.id = mhr.role_id
				WHERE r.name IN ('admin', 'manager')
				AND mhr.model_type IN ('Modules\\Auth\\Models\\User', 'App\\Models\\User')
			)`).Error; err != nil {
			return err
		}
	}
	return nil
}

func addStaffRoleAndImpersonationDown(db *gorm.DB) error {
	if err := db.Migrator().DropTable(&impersonationSessionMigration{}); err != nil {
		return err
	}
	return db.Exec("ALTER TABLE users DROP COLUMN IF EXISTS staff_role").Error
}
```

**Step 2: Build**

Run: `go build ./...`
Expected: builds clean.

**Step 3: Run migration + verify**

Run:
```bash
go run ./cmd/migrate up
/opt/homebrew/Cellar/libpq/18.4/bin/psql -h 127.0.0.1 -p 5432 -U root -d launch_go -c "\d users" | grep staff_role
/opt/homebrew/Cellar/libpq/18.4/bin/psql -h 127.0.0.1 -p 5432 -U root -d launch_go -c "\dt impersonation_sessions"
/opt/homebrew/Cellar/libpq/18.4/bin/psql -h 127.0.0.1 -p 5432 -U root -d launch_go -c "SELECT email, staff_role FROM users WHERE staff_role IS NOT NULL;"
```
Expected: `staff_role` column present; `impersonation_sessions` table present; `karthickk1996@hotmail.com` shows `super_admin` (it held the Spatie `admin` role — verified earlier).

> If `go run ./cmd/migrate up` is the wrong invocation, discover the right one: `ls cmd/migrate` and read its `main.go` / check the `Makefile` `migrate` target (`make migrate`).

**Step 4: Commit**
```bash
git add internal/database/migrations/0057_06_03_000000_add_staff_role_and_impersonation.go
git commit -m "feat(staff): migration adds staff_role + impersonation_sessions, seeds admins"
```

---

### Task 3: Add `StaffRole` to the `User` model

**Files:**
- Modify: `internal/modules/auth/models/user.go` (struct fields, after `Onboarded`)

**Step 1: Add the field + helpers**

In the `User` struct add:
```go
	StaffRole *stafftypes.StaffRole `gorm:"column:staff_role;type:varchar(20)" json:"staff_role,omitempty"`
```
Add import `stafftypes "github.com/kkz6/launch-go/internal/modules/staff/types"`.

Add methods on `*User`:
```go
// IsStaff reports whether the user has any back-office staff access.
func (u *User) IsStaff() bool {
	return u.StaffRole != nil && u.StaffRole.IsValid()
}

// HasStaffRole reports whether the user's staff tier is at least minRole.
func (u *User) HasStaffRole(minRole stafftypes.StaffRole) bool {
	return u.StaffRole != nil && u.StaffRole.Level() >= minRole.Level()
}
```

**Step 2: Build + test**

Run: `go build ./... && go test ./internal/modules/auth/...`
Expected: builds, existing auth tests still pass.

**Step 3: Commit**
```bash
git add internal/modules/auth/models/user.go
git commit -m "feat(staff): add StaffRole field + helpers to User model"
```

---

## Phase 2 — Middleware + repoint Spatie queries

### Task 4: `RequireStaff` middleware + `StaffChain`

**Files:**
- Create: `internal/middleware/staff.go`
- Modify: `internal/middleware/chains.go` (add `StaffChain`)
- Test: `internal/middleware/staff_test.go`

**Step 1: Write the failing test**

Create `internal/middleware/staff_test.go`. Use a Fiber app + a fake staff-role loader injected via `InitStaffMiddleware`:
```go
package middleware_test

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/middleware"
	stafftypes "github.com/kkz6/launch-go/internal/modules/staff/types"
)

func appWithStaff(role *stafftypes.StaffRole, min stafftypes.StaffRole) *fiber.App {
	middleware.InitStaffMiddleware(func(userID string) *stafftypes.StaffRole { return role })
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { c.Locals("userID", "u1"); return c.Next() })
	app.Get("/admin", middleware.RequireStaff(min), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})
	return app
}

func TestRequireStaff_NullRoleForbidden(t *testing.T) {
	app := appWithStaff(nil, stafftypes.StaffRoleSupport)
	resp, err := app.Test(httptest.NewRequest("GET", "/admin", nil))
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

func TestRequireStaff_SupportBlockedFromSuperAdmin(t *testing.T) {
	role := stafftypes.StaffRoleSupport
	app := appWithStaff(&role, stafftypes.StaffRoleSuperAdmin)
	resp, _ := app.Test(httptest.NewRequest("GET", "/admin", nil))
	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

func TestRequireStaff_SuperAdminAllowed(t *testing.T) {
	role := stafftypes.StaffRoleSuperAdmin
	app := appWithStaff(&role, stafftypes.StaffRoleSupport)
	resp, _ := app.Test(httptest.NewRequest("GET", "/admin", nil))
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/middleware/ -run TestRequireStaff -v`
Expected: FAIL (build error — `RequireStaff`/`InitStaffMiddleware` undefined).

**Step 3: Write minimal implementation**

Create `internal/middleware/staff.go`:
```go
package middleware

import (
	"github.com/gofiber/fiber/v2"

	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	stafftypes "github.com/kkz6/launch-go/internal/modules/staff/types"
)

// StaffRoleLoader resolves a user's staff role (nil = not staff).
type StaffRoleLoader func(userID string) *stafftypes.StaffRole

var staffMiddleware struct {
	load StaffRoleLoader
}

// InitStaffMiddleware wires the staff-role lookup. Call during bootstrap
// before routes are registered.
func InitStaffMiddleware(load StaffRoleLoader) {
	staffMiddleware.load = load
}

// RequireStaff ensures the authenticated user has at least minRole of staff
// access. Must run after the auth middleware (needs c.Locals("userID")).
//
//	router.Group("/admin", middleware.RequireStaff(stafftypes.StaffRoleSupport)...)
func RequireStaff(minRole stafftypes.StaffRole) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if staffMiddleware.load == nil {
			return fiberctx.RespondForbidden(c, "Staff access not configured")
		}
		userID, ok := c.Locals("userID").(string)
		if !ok || userID == "" {
			return fiberctx.RespondUnauthorized(c, "Authentication required")
		}
		role := staffMiddleware.load(userID)
		if role == nil || role.Level() < minRole.Level() {
			return fiberctx.RespondForbidden(c, "Staff access required")
		}
		return c.Next()
	}
}
```

Add to `internal/middleware/chains.go`:
```go
// StaffChain returns the middleware chain for back-office /admin routes:
// JWT auth then a staff-tier requirement. No team scope or subscription —
// staff access is a product-wide axis independent of any team.
//
//	router.Group("/admin", middleware.StaffChain(authMiddleware, stafftypes.StaffRoleSupport)...)
func StaffChain(authMiddleware fiber.Handler, minRole stafftypes.StaffRole) []fiber.Handler {
	return []fiber.Handler{
		authMiddleware,
		RequireStaff(minRole),
	}
}
```
Add import `stafftypes "github.com/kkz6/launch-go/internal/modules/staff/types"` to `chains.go`.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/middleware/ -run TestRequireStaff -v`
Expected: PASS (3 tests).

**Step 5: Commit**
```bash
git add internal/middleware/staff.go internal/middleware/chains.go internal/middleware/staff_test.go
git commit -m "feat(staff): add RequireStaff middleware + StaffChain"
```

---

### Task 5: Repoint `subscription.go` admin bypass at `staff_role`

**Files:**
- Modify: `internal/middleware/subscription.go` (`isUserAdmin`, lines ~80-96)

**Step 1: Replace the Spatie query**

Replace the body of `isUserAdmin` with a `staff_role` column read:
```go
// isUserAdmin reports whether a user has any staff role (support or
// super_admin) — staff bypass subscription checks. Replaces the legacy
// Spatie model_has_roles lookup.
func isUserAdmin(userID string) bool {
	if subscriptionMiddleware.db == nil {
		return false
	}
	var role *string
	subscriptionMiddleware.db.Table("users").
		Select("staff_role").
		Where("id = ?", userID).
		Scan(&role)
	return role != nil && *role != ""
}
```
(Any non-null `staff_role` is a valid staff tier → bypass, matching `CanBypassSubscription()`.)

**Step 2: Build + test**

Run: `go build ./... && go test ./internal/middleware/...`
Expected: builds, passes.

**Step 3: Commit**
```bash
git add internal/middleware/subscription.go
git commit -m "refactor(staff): repoint subscription admin bypass to users.staff_role"
```

---

### Task 6: Repoint `auth/repositories/repository.go` `IsUserAdmin`

**Files:**
- Modify: `internal/modules/auth/repositories/repository.go` (`IsUserAdmin`, lines ~85-94)

**Step 1: Replace the Spatie query**

```go
// IsUserAdmin reports whether the user holds any staff role. Replaces the
// legacy Spatie model_has_roles lookup (now reads users.staff_role).
func (r *Registry) IsUserAdmin(ctx context.Context, userID string) bool {
	var role *string
	r.db.WithContext(ctx).Table("users").
		Select("staff_role").
		Where("id = ?", userID).
		Scan(&role)
	return role != nil && *role != ""
}
```

**Step 2: Build + test**

Run: `go build ./... && go test ./internal/modules/auth/...`
Expected: builds, passes.

**Step 3: Grep-verify no Spatie reads remain**

Run: `grep -rn "model_has_roles\|role_has_permissions\|model_has_permissions" internal --include="*.go" | grep -v _test`
Expected: only the migration file(s) reference these names; **no** runtime/query code outside migrations.

**Step 4: Commit**
```bash
git add internal/modules/auth/repositories/repository.go
git commit -m "refactor(staff): repoint auth IsUserAdmin to users.staff_role"
```

---

## Phase 3 — Staff module + admin read endpoints

### Task 7: Staff module scaffold + role-loader wiring

**Files:**
- Create: `internal/modules/staff/module.go`
- Create: `internal/modules/staff/services/service.go` (staff-role lookup + admin read queries)
- Create: `internal/modules/staff/repositories/repository.go`
- Modify: `cmd/api/main.go` (construct module, register, call `InitStaffMiddleware`)

**Step 1: Implement the service role loader (with test)**

Create `internal/modules/staff/services/service_test.go` covering `StaffRoleForUser` against an in-memory/sqlite or the existing test DB harness (mirror how other module service tests set up GORM — check `internal/modules/dns` or `database` for the pattern; if they use a real test DB, follow that).

Minimum behavior to test: a user with `staff_role='support'` returns `&StaffRoleSupport`; a user with NULL returns `nil`.

**Step 2: Implement service + repository**

`services/service.go` exposes:
```go
func (s *Service) StaffRoleForUser(ctx context.Context, userID string) *stafftypes.StaffRole
func (s *Service) ListUsers(ctx context.Context, q ListQuery) ([]UserSummary, error)
func (s *Service) ListTeams(ctx context.Context, q ListQuery) ([]TeamSummary, error)
func (s *Service) ListServers(ctx context.Context, q ListQuery) ([]ServerSummary, error)
```
Keep `StaffRoleForUser` a thin column read. The list methods are cross-tenant reads (no team scoping) — they exist to power the back-office. For server logs, reuse the existing server module's log retrieval via a cross-module reader interface (mirror how `siteModule.SetDomainRepository(...)` cross-wires modules in `cmd/api/main.go`); do **not** duplicate log logic.

**Step 3: Module + wiring**

`module.go` mirrors `internal/modules/database/module.go` (implements `app.Module`, `app.RouteRegistrar`). In `cmd/api/main.go`:
- Construct: `staffModule := staff.NewModule(builder)` near the other `NewModule` calls.
- Wire the middleware loader BEFORE routes boot:
  ```go
  middleware.InitStaffMiddleware(func(userID string) *stafftypes.StaffRole {
      return staffModule.Service().StaffRoleForUser(context.Background(), userID)
  })
  ```
- Add `.Register(staffModule)` to the kernel chain.

**Step 4: Build + test + run**

Run: `go build ./... && go test ./internal/modules/staff/...`
Then smoke-run the API (see "Manual verification" at end) and confirm boot logs show `module=staff Routes registered`.

**Step 5: Commit**
```bash
git add internal/modules/staff cmd/api/main.go
git commit -m "feat(staff): staff module, role loader wiring, admin read services"
```

---

### Task 8: Admin read endpoints

**Files:**
- Create: `internal/modules/staff/handlers/admin_handler.go`
- Modify: `internal/modules/staff/module.go` (`RegisterRoutes`)

**Step 1: Define routes**

In `RegisterRoutes(router, authMiddleware)`:
```go
admin := router.Group("/admin", middleware.StaffChain(authMiddleware, stafftypes.StaffRoleSupport)...)
admin.Get("/users", h.ListUsers)
admin.Get("/teams", h.ListTeams)
admin.Get("/servers", h.ListServers)
admin.Get("/servers/:id/logs", h.ServerLogs)
// config endpoints require the higher tier:
admin.Get("/config", middleware.RequireStaff(stafftypes.StaffRoleSuperAdmin), h.GetConfig)
admin.Put("/config", middleware.RequireStaff(stafftypes.StaffRoleSuperAdmin), h.UpdateConfig)
```

**Step 2: Handlers**

Thin handlers calling the service; return the repo's standard response envelope (`{success, message, data, meta}` per `CLAUDE.md`). For `/config`, scope to the minimum you actually need now (e.g. feature flags / billing-plan visibility) — do **not** speculatively build config UI. (YAGNI: if config has no concrete backing yet, return the known global settings read-only and leave `UpdateConfig` returning 501 with a TODO, or omit the config routes entirely until there's a real setting to manage. Decide with the user at execution time.)

**Step 3: Test**

Add a handler-level test asserting `/admin/users` returns 403 without staff and 200 with `super_admin` (reuse the Fiber test-app pattern from Task 4, injecting a staff loader).

**Step 4: Build + test + commit**
```bash
go build ./... && go test ./internal/modules/staff/...
git add internal/modules/staff
git commit -m "feat(staff): admin read endpoints (users, teams, servers, logs)"
```

---

## Phase 4 — Impersonation

### Task 9: `impersonation_sessions` model + repository

**Files:**
- Create: `internal/modules/staff/models/impersonation_session.go`
- Create/extend: `internal/modules/staff/repositories/repository.go`
- Test: repository test for `Create` + `End`

**Step 1: Model** — mirror the migration columns; `TableName() => "impersonation_sessions"`; ID via `util.NewULID()`.

**Step 2: Repository** — `Create(ctx, session) error`, `End(ctx, sessionID string, endedAt time.Time) error`, `ActiveForStaff(ctx, staffID) (*Session, error)`.

**Step 3: Test** — create a session, assert row exists with `ended_at IS NULL`; call `End`, assert `ended_at` set.

**Step 4: Commit**
```bash
git add internal/modules/staff
git commit -m "feat(staff): impersonation_sessions model + repository"
```

---

### Task 10: Impersonation service (start/stop + token minting)

**Files:**
- Create: `internal/modules/staff/services/impersonation_service.go`
- Test: `internal/modules/staff/services/impersonation_service_test.go`

**Step 1: Write the failing test**

Assert `Start(ctx, staffID, targetUserID, reason)`:
- writes a session row (queryable via repo),
- returns a signed JWT whose parsed claims contain `sub == targetUserID`, `impersonator_id == staffID`, `read_only == true`, a non-empty `impersonation_sid`, `type == "access"`, and an `exp` ~30 min out.

Parse with `security.ParseJWTToken(token, secret)`.

**Step 2: Implement**

Mint the token mirroring `auth.generateAccessToken` claims plus impersonation claims (do not reuse the private method — build the `jwt.MapClaims` here using `config.JWT.Secret`):
```go
claims := jwt.MapClaims{
	"jti":             util.NewULID(),
	"sub":             target.ID,
	"email":           target.Email,
	"name":            target.Name,
	"type":            "access",
	"iat":             now.Unix(),
	"exp":             now.Add(30 * time.Minute).Unix(),
	"impersonator_id": staffID,
	"impersonation_sid": sessionID,
	"read_only":       true,
}
```
`Stop(ctx, sessionID)` stamps `ended_at = now`.

**Step 3: Run test → pass. Commit.**
```bash
git add internal/modules/staff/services
git commit -m "feat(staff): impersonation start/stop service with scoped JWT minting"
```

---

### Task 11: Impersonation endpoints + read-only enforcement

**Files:**
- Modify: `internal/middleware/auth.go` (`setAuthContext`: surface impersonation claims to `c.Locals`)
- Create: `internal/middleware/impersonation.go` (`BlockImpersonationWrites`)
- Modify: `internal/modules/staff/handlers/admin_handler.go` + `module.go` (routes)
- Test: `internal/middleware/impersonation_test.go`

**Step 1: Surface claims** — in `setAuthContext` add:
```go
if sid, ok := claims["impersonation_sid"].(string); ok && sid != "" {
	c.Locals("impersonationSID", sid)
	if ro, ok := claims["read_only"].(bool); ok {
		c.Locals("impersonationReadOnly", ro)
	}
	if imp, ok := claims["impersonator_id"].(string); ok {
		c.Locals("impersonatorID", imp)
	}
}
```

**Step 2: Failing test** for `BlockImpersonationWrites`: a request with `impersonationReadOnly=true` local + method `POST` → 403; method `GET` → passes; no impersonation local + `POST` → passes.

**Step 3: Implement middleware**
```go
func BlockImpersonationWrites() fiber.Handler {
	return func(c *fiber.Ctx) error {
		ro, _ := c.Locals("impersonationReadOnly").(bool)
		if ro {
			switch c.Method() {
			case fiber.MethodPost, fiber.MethodPut, fiber.MethodPatch, fiber.MethodDelete:
				return fiberctx.RespondForbidden(c, "Read-only impersonation session")
			}
		}
		return c.Next()
	}
}
```
Mount it globally on the authenticated API group (so it protects every customer write path during impersonation), in `cmd/api/main.go` where the authenticated router/group is built — append after the auth middleware. Confirm placement by reading the BootHTTP wiring around `AuthMiddleware`.

**Step 4: Endpoints** — on the `/admin` group:
```go
admin.Post("/impersonate/:userId", h.StartImpersonation)
admin.Post("/impersonate/stop", h.StopImpersonation)
```
`StartImpersonation` reads `userID` local as `staffID`, path `:userId` as target, optional `reason` from body; returns the minted token in the response envelope. `StopImpersonation` reads `impersonationSID` local (or body) and calls `Stop`.

**Step 5: Build + test + commit**
```bash
go build ./... && go test ./internal/middleware/... ./internal/modules/staff/...
git add internal/middleware/auth.go internal/middleware/impersonation.go internal/modules/staff cmd/api/main.go
git commit -m "feat(staff): impersonation endpoints + read-only write enforcement"
```

---

## Phase 5 — Drop Spatie

### Task 12: Migration B — drop the 5 Spatie tables

**Precondition:** Tasks 5 + 6 merged and deployed-equivalent (no runtime code reads Spatie). Grep gate from Task 6 must be clean.

**Files:**
- Create: `internal/database/migrations/0058_06_03_000001_drop_spatie_tables.go`

**Step 1: Write the migration** (up-only-meaningful; `Down` left as a no-op returning nil since recreating Laravel's polymorphic permission schema is not worth maintaining — note this in a comment):
```go
func dropSpatieTablesUp(db *gorm.DB) error {
	// FK-safe order: junctions first, then roles/permissions.
	stmts := []string{
		"DROP TABLE IF EXISTS role_has_permissions",
		"DROP TABLE IF EXISTS model_has_permissions",
		"DROP TABLE IF EXISTS model_has_roles",
		"DROP TABLE IF EXISTS roles",
		"DROP TABLE IF EXISTS permissions",
	}
	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			return err
		}
	}
	return nil
}

func dropSpatieTablesDown(db *gorm.DB) error {
	// Intentional no-op: the Spatie permission schema is retired and not
	// recreated. staff_role on users replaces its only live use.
	return nil
}
```

**Step 2: Run + verify**

Run:
```bash
go run ./cmd/migrate up
/opt/homebrew/Cellar/libpq/18.4/bin/psql -h 127.0.0.1 -p 5432 -U root -d launch_go -c "\dt" | grep -E "roles|permissions" || echo "spatie tables gone"
```
Expected: `spatie tables gone`.

**Step 3: Full build + test**

Run: `go build ./... && go test ./...`
Expected: all green (the seed in Task 2 is guarded by `HasTable`, so re-running migrations on a fresh DB stays safe even with Spatie gone).

**Step 4: Commit**
```bash
git add internal/database/migrations/0058_06_03_000001_drop_spatie_tables.go
git commit -m "refactor(staff): drop dormant Spatie permission tables"
```

---

## Phase 6 — Frontend (Nuxt `launch-go/client`)

### Task 13: `/admin` gate + impersonation banner

**Files (discover exact conventions first** — read `client/middleware/`, `client/layouts/default.vue`, `client/stores`, and `client/types/index.ts`):**
- Modify: `client/types/index.ts` — add `staff_role?: 'support' | 'super_admin' | null` to the `User` interface.
- Create: `client/middleware/staff.ts` — route middleware redirecting non-staff away from `/admin/**` (read staff_role from the auth store/profile). Backend is the real gate; this is UX.
- Create: `client/pages/admin/` — an index page hitting `GET /admin/users` etc. (scope to what's needed; start with users + a "spectate" button calling `POST /admin/impersonate/:userId`).
- Modify: root layout (`client/layouts/default.vue` or equivalent) — render a persistent, non-dismissible banner when the active token carries `impersonator_id` ("Viewing as {name} — Exit"); Exit calls `POST /admin/impersonate/stop`, drops the impersonation token, restores the staff session.
- Config UI: render only for `super_admin`. **No manage-staff UI.**

**Steps:** Follow the client's existing data-fetching + store patterns (mirror an existing authenticated page). Verify by running the app (`make client-dev` or the `client` dev server on port 6124 as configured) and:
1. As a non-staff user, navigate to `/admin` → redirected away.
2. As `super_admin`, `/admin` loads the user list.
3. Click "spectate" on a user → banner appears, app shows that user's data, write actions are blocked (server returns 403), Exit restores your session.

**Commit** after each coherent piece (types, middleware, page, banner).

---

## Phase 7 — Final verification

### Task 14: Separation guarantee + full regression

**Step 1: Integration test** (`internal/modules/staff/separation_test.go` or middleware test):
- A customer with `staff_role = null` (even team `owner`) → 403 on an `/admin` route.
- A `support` staff user belonging to **no team** → reaches `/admin` read routes.
- A `super_admin` still bypasses subscription (regression for Task 5): mount `VerifySubscription` on a route, assert a `super_admin` with an unsubscribed team still passes.

**Step 2: Full suite**

Run: `go build ./... && go test ./...`
Expected: all packages pass (baseline was 61 ok).

**Step 3: Manual end-to-end** (per the `run`/`verify` skills):
- API on 6123, Nuxt on 6124 (env overrides as established earlier).
- `super_admin` logs in → `/admin` works → impersonate a customer → read-only confirmed → exit.
- A normal customer → `/admin` 403, their team flows unaffected.

**Step 4: Finish the branch** — use superpowers:finishing-a-development-branch (merge/PR/cleanup of the worktree).

---

## Out of scope (do NOT build)

- Runtime-editable permissions / granular RBAC.
- Manage-staff UI (staff_role is set via DB/seeder).
- Write ("act-as") impersonation — read-only spectate only.
- Separate admin frontend app.

## Risks / watch-outs

- **Seed timing:** Migration A's seed must run while Spatie tables still exist; the `HasTable` guard keeps it safe but the *ordering* (A before B) is essential. Never combine them.
- **Read-only middleware placement:** must sit on the authenticated API group so it covers every customer write path, not just `/admin`. Verify it does not block the `POST /admin/impersonate/stop` exit (that route's token is the staff token, not the impersonation token — confirm Stop is reachable; if Exit uses the impersonation token, special-case the stop route before the block middleware).
- **Token type:** impersonation token uses `type: "access"` so existing auth middleware accepts it. Confirm no refresh path tries to refresh an impersonation token (it has no refresh counterpart; it just expires in 30 min and the user exits).
