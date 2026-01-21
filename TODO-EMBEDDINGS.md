# Go Struct Embedding Consolidation Plan

A comprehensive plan for leveraging Go's struct embedding to reduce code duplication, following Laravel's trait-like composition patterns.

**Generated:** 2026-01-21
**Updated:** 2026-01-21
**Focus:** Struct embedding & composition patterns (NOT covered in TODO-REFACTORING.md)
**Estimated LOC Reduction:** 4000+ lines

---

## Completed Infrastructure (Phase 1)

The following foundational infrastructure has been implemented:

| # | Work Package | Status | Commit |
|---|-------------|--------|--------|
| 46 | GitLab Webhook Timing Attack Fix | ✅ DONE | `4c7a6b3` |
| 47 | Broadcast Event Constants | ✅ DONE | (in broadcast/events.go) |
| 49 | Crypto Package Consolidation | ✅ DONE | `4c7a6b3` - cryptoutil/ |
| 51 | HTTP Client Base with Request Builder | ✅ DONE | `f14f33f` - httpclient/ |
| 63 | Error Type Consolidation | ✅ DONE | `fde7a4f` - errors/ |
| 75 | GORM Query Scope - Order By Latest | ✅ DONE | `ed049ad` - repository/scopes.go |
| 79 | Job Struct Boilerplate | ✅ DONE | `d699376` - jobs/builder.go, payload.go |
| 81 | DTO Timestamp Formatter | ✅ DONE | `f14f33f` - dto/response.go |
| 106 | Enum Boilerplate Consolidation | ✅ DONE | `065e17c` - enums/ |
| 108 | Slice Transformation Generic Helper | ✅ DONE | `de817a9` - utils/slice.go |
| 109 | Safe Map Access Helper | ✅ DONE | `de817a9` - utils/map.go |
| 115 | Nil-Safe Dereference Helpers | ✅ DONE | (already in ptr/) |
| 148 | Generic Slice Mapper | ✅ DONE | `de817a9` - utils/slice.go |
| 149 | Enum Response Helper | ✅ DONE | dto/enum.go |
| 150 | Pointer Dereference Utilities | ✅ DONE | (already in ptr/) |
| 151 | URL Builder Package | ✅ DONE | (already in urlbuilder/) |
| 152 | Generic UpdateStatus Repository Method | ✅ DONE | `ed049ad` - repository/base.go |
| 154 | Preload Scope Helpers | ✅ DONE | `ed049ad` - repository/scopes.go |
| - | Model Mixins (Named, Described, etc.) | ✅ DONE | `44808a1` - models/mixins.go |
| - | Constants/Limits | ✅ DONE | `1cf762a` - constants/limits.go |
| 1 | Job Base Payload Generic | ✅ DONE | `13ebce5` - jobs/context.go BaseJob[C,P] |
| 2 | Installable Repository Mixin | ✅ DONE | `5a998b7` - Go embedding promotes methods |
| 3 | Queue Dispatch Unification | ✅ DONE | `81a797c` - jobs.Base.Dispatch* methods |
| 4 | Pagination Embedding | ✅ DONE | repository.Paginate, PaginatedResult[T] |
| 5 | Task Builder Pattern | ✅ DONE | taskrunner.TaskBuilder[C], BuiltTask[C] |

---

## Table of Contents

*All embedding tasks completed.*

---

## Implementation Phases

### Phase 1: High Priority (Week 1-2)
| Item | Priority | Est. LOC Saved | Complexity |
|------|----------|----------------|------------|
| WebSocket Handler Base | P1 | 75 | Low |
| Webhook Handler Base | P1 | 40 | Low |
| Repository BaseRepository | P1 | 450 | Medium |
| Service Base Enforcement | P1 | 200 | Medium |
| Module Handler Fields | P1 | 150 | Low |

### Phase 2: Medium Priority (Week 3-4)
| Item | Priority | Est. LOC Saved | Complexity |
|------|----------|----------------|------------|
| Job Base Payload | P2 | 100 | Medium |
| Installable Repository Mixin | P2 | 200 | Medium |
| Queue Dispatch Unification | P2 | 100 | Medium |
| Pagination Embedding | P2 | 50 | Low |
| Task Builder Pattern | P2 | 600 | High |

### Phase 3: Lower Priority (Week 5-6)
| Item | Priority | Est. LOC Saved | Complexity |
|------|----------|----------------|------------|
| Model Scoped Fields | P2 | 100 | Low |
| Activity Logging Builder | P2 | 150 | Medium |
| Broadcast Payload Builder | P3 | 30 | Low |
| DTO Timestamp Embedding | P3 | 400 | Medium |

---

## Success Metrics

After implementing these embeddings:

1. **Code Reduction:** ~2500 lines eliminated
2. **Consistency:** All modules follow identical patterns
3. **Type Safety:** Compile-time checks for embedded fields
4. **Maintainability:** Single point of change for common functionality
5. **Discoverability:** IDE autocomplete works with embedded methods
6. **Testing:** Base classes can be unit tested once

---

## Notes

- Always run tests after each embedding change
- Verify GORM handles embedded fields correctly
- Check JSON serialization of embedded structs
- Update CLAUDE.md with new embedding patterns
- Consider backward compatibility for API responses

---

## 36. Systemd Config Script Consolidation (P1)

**Issue:** Identical chmod/chown script blocks repeated in 3 task files.

**Files Affected:**
- `internal/modules/server/tasks/daemon.go:26-55` - 30 lines
- `internal/modules/server/tasks/cron.go:23-45` - 23 lines
- `internal/modules/site/tasks/queue.go:54-82` - 29 lines

**Current Pattern (repeated 3 times):**
```go
script := `#!/bin/bash
set -euo pipefail

cat > "` + config.Path + `" << 'EOF'
` + config.Contents + `
EOF

chmod 644 "` + config.Path + `"

LOG_DIR=$(dirname "` + config.LogPath + `")
mkdir -p "$LOG_DIR"

if [ ! -f "` + config.LogPath + `" ]; then
    touch "` + config.LogPath + `"
    chown ` + config.User + `:` + config.User + ` "` + config.LogPath + `"
    chmod 644 "` + config.LogPath + `"
fi
// ... error log setup identical
`
```

**Solution:** Create `internal/pkg/script/systemd.go`
```go
package script

// SystemdConfigParams holds parameters for systemd config scripts
type SystemdConfigParams struct {
    ConfigPath   string
    Contents     string
    User         string
    LogPath      string
    ErrorLogPath string // Optional
    Heredoc      string // EOF marker name
}

// WriteSystemdConfig generates script to write systemd config with logging setup
func WriteSystemdConfig(params SystemdConfigParams) string {
    if params.Heredoc == "" {
        params.Heredoc = "CONFIGEOF"
    }

    return NewBuilder().
        Shebang().
        SetOptions().
        Blank().
        Comment("Write config file").
        Linef("cat > %q << '%s'", params.ConfigPath, params.Heredoc).
        Line(params.Contents).
        Line(params.Heredoc).
        Blank().
        Linef("chmod 644 %q", params.ConfigPath).
        Blank().
        Comment("Setup log directory").
        Linef("LOG_DIR=$(dirname %q)", params.LogPath).
        Line("mkdir -p \"$LOG_DIR\"").
        Blank().
        When(params.LogPath != "", func(b *Builder) {
            b.setupLogFile(params.LogPath, params.User)
        }).
        When(params.ErrorLogPath != "", func(b *Builder) {
            b.setupLogFile(params.ErrorLogPath, params.User)
        }).
        Linef("chown %s:%s \"$LOG_DIR\"", params.User, params.User).
        String()
}

func (b *Builder) setupLogFile(path, user string) *Builder {
    return b.
        Linef("if [ ! -f %q ]; then", path).
        Linef("    touch %q", path).
        Linef("    chown %s:%s %q", user, user, path).
        Linef("    chmod 644 %q", path).
        Line("fi")
}
```

**Refactoring Steps:**
- [ ] Create `internal/pkg/script/systemd.go`
- [ ] Refactor daemon.go to use WriteSystemdConfig
- [ ] Refactor cron.go to use WriteSystemdConfig
- [ ] Refactor queue.go to use WriteSystemdConfig

**Impact:** ~60 lines eliminated, consistent config file handling

---

## 37. Home Directory Helper (P2)

**Issue:** Home directory construction repeated 4 times in same file.

**Files Affected:**
- `internal/modules/server/tasks/ssh_keys.go:59-68, 79-82, 96-99, 110-113`

**Current Pattern (repeated 4 times):**
```go
homeDir := fmt.Sprintf("/home/%s", user)
if user == "root" {
    homeDir = "/root"
}
```

**Solution:** Add to `internal/pkg/shell/defaults.go`
```go
// GetHomeDir returns the home directory for a user
func GetHomeDir(user string) string {
    if user == "root" {
        return "/root"
    }
    return fmt.Sprintf("/home/%s", user)
}

// GetSSHDir returns the .ssh directory for a user
func GetSSHDir(user string) string {
    return filepath.Join(GetHomeDir(user), ".ssh")
}

// GetAuthorizedKeysPath returns the authorized_keys path for a user
func GetAuthorizedKeysPath(user string) string {
    return filepath.Join(GetSSHDir(user), "authorized_keys")
}
```

**Impact:** ~20 lines eliminated, consistent path handling

---

## 38. Transaction + Activity Logging Helper (P2)

**Issue:** Activity logging with transaction is only combined in 1 place; 50+ activity logs use inconsistent DB access.

**Files Affected:**
- `internal/modules/site/services/site_service.go:244-255` - Only combined example
- 50+ activity logging instances across all services
- 3 different DB access patterns: `s.repos.DB()`, `s.Repos().Site().DB`, `s.DB()`

**Current Patterns:**
```go
// Pattern 1: Direct repos
activity.New(s.repos.DB()).WithContext(ctx).UseLog("database")...

// Pattern 2: Module-specific
activity.New(s.Repos().Site().DB).WithContext(ctx).UseLog("site")...

// Pattern 3: Service base
activity.New(s.DB()).WithContext(ctx).UseLog("backup")...
```

**Solution:** Add to `internal/pkg/service/base.go`
```go
// LogActivity provides standardized activity logging
func (b *Base) LogActivity(ctx context.Context, model interface{}, event, message string, userID *string) error {
    logger := activity.New(b.DB()).
        WithContext(ctx).
        UseLog(b.getLogName(model)).
        On(model).
        WithEvent(event)

    if userID != nil {
        logger.CausedByUser(*userID)
    }

    return logger.Log(message)
}

// LogCreation logs a creation event
func (b *Base) LogCreation(ctx context.Context, model interface{}, userID *string) error {
    return b.LogActivity(ctx, model, "created", b.getCreationMessage(model), userID)
}

// LogUpdate logs an update event
func (b *Base) LogUpdate(ctx context.Context, model interface{}, userID *string) error {
    return b.LogActivity(ctx, model, "updated", b.getUpdateMessage(model), userID)
}

// LogDeletion logs a deletion event
func (b *Base) LogDeletion(ctx context.Context, model interface{}, userID *string) error {
    return b.LogActivity(ctx, model, "deleted", b.getDeletionMessage(model), userID)
}

// WithTransaction executes function in transaction with optional activity logging
func (b *Base) WithTransaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
    return b.DB().WithContext(ctx).Transaction(fn)
}
```

**Impact:** ~150 lines eliminated, consistent activity logging

---

## 40. Webhook Response Standardization (P1)

**Issue:** Webhook handlers bypass response package, use inconsistent formats.

**Files Affected:**
- `internal/modules/git/handlers/webhook_handler.go:53-100` - 7 SendString calls
- `internal/modules/billing/handlers/webhook_handler.go:58-107` - 6 "error" key uses
- `internal/modules/site/handlers/webhook_handler.go:51-66` - 2 "error" key uses
- `internal/pkg/signedurl/middleware.go:76-97` - "error": true format

**Current Patterns:**
```go
// Git webhook - plain text instead of JSON
return c.Status(fiber.StatusBadRequest).SendString("Invalid provider")

// Billing webhook - wrong key
return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Missing signature"})

// SignedURL - wrong format
return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": true, "message": "..."})
```

**Solution:** Update all to use response package
```go
// Correct pattern
return response.BadRequest(c, "Invalid provider")
return response.Unauthorized(c, "Missing signature")
return response.Forbidden(c, "URL has expired")
```

**Refactoring Steps:**
- [ ] Update git webhook handler (7 instances)
- [ ] Update billing webhook handler (6 instances)
- [ ] Update site webhook handler (2 instances)
- [ ] Update signedurl middleware (2 instances)

**Impact:** ~30 lines fixed, consistent API responses

---

## 41. AppError HTTPStatus Interface (P1)

**Issue:** AppError struct missing HTTPStatus() method, not caught by error middleware.

**Files Affected:**
- `internal/pkg/errors/errors.go:22-45` - AppError definition

**Current Code:**
```go
type AppError struct {
    Err     error
    Message string
    Code    int
}

// Missing: func (e *AppError) HTTPStatus() int { return e.Code }
```

**Fix Required:**
```go
// Add to internal/pkg/errors/errors.go
func (e *AppError) HTTPStatus() int {
    return e.Code
}
```

**Impact:** Bug fix, enables error middleware to catch AppError

---

## 42. Timestamp Formatting Standardization (P2)

**Issue:** Mixed timestamp formats across modules.

**Files Affected:**
- 26 instances use `time.RFC3339`
- 5 instances use `"Jan 2, 2006"` (billing, auth passkey)

**Inconsistent Files:**
- `internal/modules/billing/dto/responses.go:50,55,76` - Custom format
- `internal/modules/auth/handlers/passkey_handler.go:47,53` - Custom format

**Solution:** Standardize on RFC3339 for API responses, create display formatter for UI
```go
// internal/pkg/dto/time.go
const (
    APITimeFormat     = time.RFC3339
    DisplayTimeFormat = "Jan 2, 2006"
)

// FormatAPITime formats for API responses
func FormatAPITime(t time.Time) string {
    return t.Format(APITimeFormat)
}

// FormatDisplayTime formats for human-readable display
func FormatDisplayTime(t time.Time) string {
    return t.Format(DisplayTimeFormat)
}
```

**Impact:** Consistent API contracts

---

## 43. Validation Error Message Expansion (P2)

**Issue:** Validation error messages missing for commonly used tags.

**Files Affected:**
- `internal/pkg/validator/validator.go:43-62` - Only 7 cases handled

**Missing Tags (used in DTOs):**
- `len` - Used in auth DTOs
- `ulid` - Used in 20+ places
- `dive` - Nested validation
- `fqdn` - DNS module
- `ip`, `ip|cidr` - Server module
- `eqfield` - Password confirmation
- `required_if`, `required_unless` - Conditional validation

**Solution:** Expand error message handler
```go
func getErrorMessage(err validator.FieldError) string {
    switch err.Tag() {
    // ... existing cases ...
    case "len":
        return "Must be exactly " + err.Param() + " characters"
    case "ulid":
        return "Must be a valid ULID"
    case "fqdn":
        return "Must be a valid domain name"
    case "ip":
        return "Must be a valid IP address"
    case "cidr":
        return "Must be a valid CIDR notation"
    case "eqfield":
        return "Must match " + err.Param()
    case "required_if":
        return "This field is required"
    case "required_unless":
        return "This field is required"
    default:
        return "Invalid value"
    }
}
```

**Impact:** Better user experience, clearer validation errors

---

## 44. Duplicate Interface Consolidation (P1 - CRITICAL)

**Issue:** Multiple interfaces defined identically in different packages.

**Duplicate Interfaces Found:**

1. **Broadcaster Interface (EXACT DUPLICATE)**
   - `internal/websocket/broadcaster.go:5-20` - Broadcaster
   - `internal/pkg/broadcast/broadcaster.go:6-21` - TeamBroadcaster
   - Same methods: BroadcastToServer, BroadcastToSite, BroadcastToTeam, etc.

2. **HTTPStatusError Interface (EXACT DUPLICATE)**
   - `internal/pkg/response/errors.go:74-77`
   - `internal/middleware/error.go:13-16`
   - Both define: `HTTPStatus() int`

3. **ModelBroadcaster Interface (DUPLICATE)**
   - `internal/websocket/broadcaster.go:24-35`
   - `internal/pkg/broadcast/broadcaster.go:25-36`

4. **Notification Interface (3 VERSIONS)**
   - `internal/pkg/taskrunner/callback.go:32-34` - Minimal (RawText only)
   - `internal/modules/notification/channels/types.go:63-78` - Full
   - `internal/modules/notification/models/notification.go:9-27` - Full with Type()

**Solution:**
```go
// Keep ONE canonical location for each:

// 1. internal/pkg/broadcast/broadcaster.go - Keep TeamBroadcaster
// Delete: internal/websocket/broadcaster.go (use pkg version)

// 2. internal/pkg/errors/http.go - Create shared location
type HTTPStatusError interface {
    HTTPStatus() int
    Error() string
}

// 3. internal/pkg/notification/notification.go - Create shared interface
type Notification interface {
    Type() string
    RawText() string
    ToEmail() string
    ToSlack() string
    ToDiscord() string
    ToTelegram() string
}
```

**Refactoring Steps:**
- [ ] Delete `internal/websocket/broadcaster.go`, use pkg/broadcast
- [ ] Create `internal/pkg/errors/http.go` for HTTPStatusError
- [ ] Update middleware/error.go to import from pkg/errors
- [ ] Consolidate Notification interfaces to single location
- [ ] Update all imports across codebase

**Impact:** Eliminates confusion, single source of truth for interfaces

---

## 45. GORM Query Helper Methods (P1)

**Issue:** Common query patterns repeated across 15+ repositories.

**Files Affected:**
- All repositories in `internal/modules/*/repositories/*.go`

**Pattern 1: Preload("Server") - Repeated 4+ times**
```go
// daemon_repository.go:46, firewall_rule_repository.go:46, cron_repository.go:46, database_repository.go:50
err := r.DB().WithContext(ctx).Preload("Server").First(&model, "id = ?", id).Error
```

**Pattern 2: FindByID Error Handling - Repeated 15+ times**
```go
err := r.DB().WithContext(ctx).First(&model, "id = ?", id).Error
if err != nil {
    if errors.Is(err, gorm.ErrRecordNotFound) {
        return nil, ErrModelNotFound
    }
    return nil, err
}
```

**Pattern 3: FindByIDAndServer - Repeated 8+ times**
```go
err := r.DB().WithContext(ctx).First(&model, "id = ? AND server_id = ?", id, serverID).Error
```

**Pattern 4: Order("created_at DESC") - Repeated 15+ times**
```go
Where("server_id = ?", serverID).Order("created_at DESC").Find(&models).Error
```

**Solution:** Add to `internal/pkg/repository/base.go`
```go
// FindByIDWithPreload finds a record with preloading
func (r *BaseRepository) FindByIDWithPreload[T any](ctx context.Context, id string, preloads ...string) (*T, error) {
    var model T
    query := r.db.WithContext(ctx)
    for _, p := range preloads {
        query = query.Preload(p)
    }
    if err := query.First(&model, "id = ?", id).Error; err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, ErrNotFound
        }
        return nil, err
    }
    return &model, nil
}

// FindByIDAndScope finds by ID with additional scope condition
func (r *BaseRepository) FindByIDAndScope[T any](ctx context.Context, id, scopeCol, scopeVal string) (*T, error) {
    var model T
    if err := r.db.WithContext(ctx).First(&model, "id = ? AND "+scopeCol+" = ?", id, scopeVal).Error; err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, ErrNotFound
        }
        return nil, err
    }
    return &model, nil
}

// FindAllByScope finds all records matching scope, ordered by created_at DESC
func (r *BaseRepository) FindAllByScope[T any](ctx context.Context, scopeCol, scopeVal string) ([]T, error) {
    var models []T
    err := r.db.WithContext(ctx).
        Where(scopeCol+" = ?", scopeVal).
        Order("created_at DESC").
        Find(&models).Error
    return models, err
}

// LatestPreload scope for preloading with order and limit
func LatestPreload(limit int) func(*gorm.DB) *gorm.DB {
    return func(db *gorm.DB) *gorm.DB {
        return db.Order("created_at DESC").Limit(limit)
    }
}
```

**Refactored Repository:**
```go
// Before
func (r *DaemonRepository) FindByID(ctx context.Context, id string) (*models.Daemon, error) {
    var daemon models.Daemon
    err := r.DB().WithContext(ctx).First(&daemon, "id = ?", id).Error
    if err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, ErrDaemonNotFound
        }
        return nil, err
    }
    return &daemon, nil
}

// After
func (r *DaemonRepository) FindByID(ctx context.Context, id string) (*models.Daemon, error) {
    return r.FindByIDWithPreload[models.Daemon](ctx, id)
}

func (r *DaemonRepository) FindByIDWithServer(ctx context.Context, id string) (*models.Daemon, error) {
    return r.FindByIDWithPreload[models.Daemon](ctx, id, "Server")
}
```

**Refactoring Steps:**
- [ ] Add generic helper methods to BaseRepository
- [ ] Add LatestPreload scope helper
- [ ] Refactor daemon_repository to use helpers
- [ ] Refactor firewall_rule_repository
- [ ] Refactor cron_repository
- [ ] Refactor all other repositories

**Impact:** ~300 lines eliminated, consistent query patterns

---

## 46. ✅ COMPLETED - GitLab Webhook Timing Attack Fix

**Issue:** GitLab webhook validation uses direct string comparison - vulnerable to timing attacks.

**File Affected:**
- `internal/modules/git/providers/gitlab.go:87-94`

**Current Code (VULNERABLE):**
```go
func (p *GitLabProvider) ValidateWebhook(payload []byte, signature string) bool {
    // VULNERABLE: Direct string comparison
    return signature == p.config.WebhookSecret
}
```

**Fixed Code:**
```go
func (p *GitLabProvider) ValidateWebhook(payload []byte, signature string) bool {
    if p.config.WebhookSecret == "" {
        return false
    }
    // Use constant-time comparison to prevent timing attacks
    return hmac.Equal([]byte(signature), []byte(p.config.WebhookSecret))
}
```

**Impact:** Security fix - prevents timing-based secret extraction

---

## 47. ✅ COMPLETED - Broadcast Event Constants

**Issue:** Event names hardcoded as strings throughout codebase.

**Files Affected:**
- Deployment events: 10+ locations in `internal/modules/site/`
- Server events: 15+ locations in `internal/modules/server/`
- Site events: 12+ locations in `internal/modules/site/`

**Current Pattern:**
```go
// Repeated string literals
s.BroadcastToDeployment(id, "deployment.progress", data)
s.BroadcastToTeam(teamID, "server.provisioning", data)
s.BroadcastToTeam(teamID, "site.created", data)
```

**Solution:** Create `internal/pkg/broadcast/events.go`
```go
package broadcast

// Deployment Events
const (
    EventDeploymentProgress         = "deployment.progress"
    EventDeploymentFinished         = "deployment.finished"
    EventDeploymentFailed           = "deployment.failed"
    EventDeploymentTimeout          = "deployment.timeout"
    EventDeploymentRollbackStarted  = "deployment.rollback.started"
    EventDeploymentRollbackCompleted = "deployment.rollback.completed"
    EventDeploymentRollbackFailed   = "deployment.rollback.failed"
)

// Server Events
const (
    EventServerProvisioning          = "server.provisioning"
    EventServerProvisionFailed       = "server.provision_failed"
    EventServerProvisioned           = "server.provisioned"
    EventServerConnectivity          = "server.connectivity"
    EventServerRebooting             = "server.rebooting"
    EventServerArchived              = "server.archived"
    EventServerMetrics               = "server.metrics"
    EventServerVulnerabilityAudit    = "server.vulnerability_audit_completed"
)

// Site Events
const (
    EventSiteCreated         = "site.created"
    EventSiteUpdated         = "site.updated"
    EventSiteDeleted         = "site.deleted"
    EventSiteHorizonEnabled  = "site.horizon_enabled"
    EventSiteHorizonDisabled = "site.horizon_disabled"
    EventSiteInertiaEnabled  = "site.inertia_enabled"
    EventSiteInertiaDisabled = "site.inertia_disabled"
)

// CRUD Events (for model broadcasting)
const (
    EventCreated = "created"
    EventUpdated = "updated"
    EventDeleted = "deleted"
)
```

**Refactored Usage:**
```go
// Before
s.BroadcastToDeployment(id, "deployment.progress", data)

// After
s.BroadcastToDeployment(id, broadcast.EventDeploymentProgress, data)
```

**Refactoring Steps:**
- [ ] Create `internal/pkg/broadcast/events.go`
- [ ] Define all deployment event constants
- [ ] Define all server event constants
- [ ] Define all site event constants
- [ ] Update all broadcast calls to use constants
- [ ] Add compile-time checking

**Impact:** Type safety, IDE autocomplete, no typos in event names

---

## 48. Logging Helper Methods (P2)

**Issue:** Error logging patterns repeated 28+ times, base handler log methods unused.

**Files Affected:**
- `internal/modules/websocket/handlers/*.go` - 8 occurrences
- `internal/modules/git/handlers/webhook_handler.go` - 5 occurrences
- `internal/modules/billing/services/*.go` - 4 occurrences
- `internal/pkg/handler/base.go` - Has LogError, LogWarn but UNUSED

**Current Pattern (repeated 28+ times):**
```go
h.logger.Error().Err(err).Str("server_id", serverID).Msg("Server not found")
h.logger.Warn().Err(err).Str("server_id", serverID).Msg("Authentication failed")
```

**Solution:** Use and extend base handler methods
```go
// internal/pkg/handler/base.go - Already exists, needs promotion

// Add to BaseHandler
func (h *BaseHandler) LogErrorWithFields(err error, msg string, fields map[string]string) {
    event := h.Logger.Error().Err(err)
    for k, v := range fields {
        event = event.Str(k, v)
    }
    event.Msg(msg)
}

func (h *BaseHandler) LogWarnWithFields(err error, msg string, fields map[string]string) {
    event := h.Logger.Warn().Err(err)
    for k, v := range fields {
        event = event.Str(k, v)
    }
    event.Msg(msg)
}

// Common field helpers
func (h *BaseHandler) LogServerError(err error, serverID, msg string) {
    h.LogErrorWithFields(err, msg, map[string]string{"server_id": serverID})
}

func (h *BaseHandler) LogSiteError(err error, siteID, msg string) {
    h.LogErrorWithFields(err, msg, map[string]string{"site_id": siteID})
}
```

**Refactoring Steps:**
- [ ] Audit why base handler log methods are unused
- [ ] Add field-based logging helpers
- [ ] Add common entity logging helpers (server, site, user)
- [ ] Refactor WebSocket handlers to use base methods
- [ ] Refactor git handlers to use base methods

**Impact:** ~100 lines eliminated, consistent logging

---

## 49. ✅ COMPLETED - Crypto Package Consolidation

**Issue:** Cryptographic operations scattered across 5+ files.

**Files Affected:**
- Password hashing: `internal/modules/auth/services/` (3 files)
- Token generation: `internal/pkg/utils/token.go` + `auth/models/password_reset_token.go`
- HMAC signatures: `git/providers/` (3 files) + `billing/handlers/` + `signedurl/`

**Current State:**
```go
// Scattered password hashing
bcrypt.GenerateFromPassword(...)  // auth_service.go:48
bcrypt.GenerateFromPassword(...)  // user_service.go:102
bcrypt.GenerateFromPassword(...)  // password_reset_service.go:104

// Duplicate token generation
utils.GenerateBase64Token(length)           // pkg/utils/token.go:21
models.GenerateToken(length)                // auth/models/password_reset_token.go:32
```

**Solution:** Create `internal/pkg/crypto/` package
```go
// internal/pkg/crypto/password.go
package crypto

import "golang.org/x/crypto/bcrypt"

const DefaultCost = bcrypt.DefaultCost

func HashPassword(password string) (string, error) {
    hash, err := bcrypt.GenerateFromPassword([]byte(password), DefaultCost)
    return string(hash), err
}

func VerifyPassword(hash, password string) bool {
    return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// internal/pkg/crypto/token.go
package crypto

func GenerateToken(length int) (string, error) {
    bytes := make([]byte, length)
    if _, err := rand.Read(bytes); err != nil {
        return "", err
    }
    return base64.URLEncoding.EncodeToString(bytes)[:length], nil
}

func MustGenerateToken(length int) string {
    token, err := GenerateToken(length)
    if err != nil {
        panic(err)
    }
    return token
}

// internal/pkg/crypto/hmac.go
package crypto

func VerifyHMACSHA256(secret, data, signature string) bool {
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write([]byte(data))
    expected := hex.EncodeToString(mac.Sum(nil))
    return hmac.Equal([]byte(expected), []byte(signature))
}
```

**Refactoring Steps:**
- [ ] Create `internal/pkg/crypto/` package
- [ ] Add password.go with HashPassword, VerifyPassword
- [ ] Add token.go with unified token generation
- [ ] Add hmac.go with signature verification
- [ ] Refactor auth services to use crypto package
- [ ] Remove duplicate token generation from models

**Impact:** ~50 lines eliminated, centralized security code

---

## 50. N+1 Query Fix in Site Repository (P1)

**Issue:** Manual loop-based query causes N+1 problem.

**File Affected:**
- `internal/modules/site/repositories/site_repository.go:137-145`

**Current Code (N+1 PROBLEM):**
```go
func (r *SiteRepository) FindByServerWithLatestDeployment(ctx context.Context, serverID string) ([]models.Site, error) {
    var sites []models.Site
    err := r.DB().WithContext(ctx).Where("server_id = ?", serverID).Find(&sites).Error
    if err != nil {
        return nil, err
    }

    // N+1: One query per site!
    for i := range sites {
        var deployment models.Deployment
        r.DB().Where("site_id = ?", sites[i].ID).Order("created_at DESC").First(&deployment)
        sites[i].Deployments = []models.Deployment{deployment}
    }
    return sites, nil
}
```

**Fixed Code:**
```go
func (r *SiteRepository) FindByServerWithLatestDeployment(ctx context.Context, serverID string) ([]models.Site, error) {
    var sites []models.Site
    err := r.DB().WithContext(ctx).
        Where("server_id = ?", serverID).
        Preload("Deployments", func(db *gorm.DB) *gorm.DB {
            // Subquery to get only latest deployment per site
            return db.Where("id IN (?)",
                r.DB().Model(&models.Deployment{}).
                    Select("DISTINCT ON (site_id) id").
                    Order("site_id, created_at DESC"),
            )
        }).
        Find(&sites).Error
    return sites, err
}

// Or use raw SQL for better performance:
func (r *SiteRepository) FindByServerWithLatestDeployment(ctx context.Context, serverID string) ([]models.Site, error) {
    var sites []models.Site
    err := r.DB().WithContext(ctx).
        Where("server_id = ?", serverID).
        Preload("Deployments", LatestPreload(1)).
        Find(&sites).Error
    return sites, err
}
```

**Impact:** Performance fix - reduces N+1 queries to 2 queries

---

## Extended Summary (Items 44-50)

| Category | Items | Est. LOC Saved |
|----------|-------|----------------|
| Duplicate Interface Consolidation | Item 44 | ~100 lines |
| GORM Query Helpers | Item 45 | ~300 lines |
| GitLab Security Fix | Item 46 | Security |
| Broadcast Event Constants | Item 47 | ~50 lines |
| Logging Helpers | Item 48 | ~100 lines |
| Crypto Consolidation | Item 49 | ~50 lines |
| N+1 Query Fix | Item 50 | Performance |
| **Additional Total** | **7 patterns** | **~600 lines** |

---

## Final Summary

| Category | Items | Total LOC Saved |
|----------|-------|-----------------|
| Handler Bases | 5 patterns | ~400 lines |
| Repository Patterns | 5 patterns | ~1200 lines |
| Service Patterns | 3 patterns | ~450 lines |
| Job/Task Patterns | 3 patterns | ~800 lines |
| Model Mixins | 5 patterns | ~270 lines |
| Provider API Clients | 3 patterns | ~450 lines |
| Middleware Patterns | 1 pattern | ~30 lines |
| Config Patterns | 1 pattern | ~86 lines |
| DTO/Request Patterns | 2 patterns | ~550 lines |
| Testing Patterns | 2 patterns | ~250 lines |
| Infrastructure Patterns | 6 patterns | ~1075 lines |
| Validation & Response | 5 patterns | ~610 lines |
| Template & Script | 3 patterns | ~160 lines |
| Interface & Query | 4 patterns | ~500 lines |
| Security & Events | 3 patterns | ~100 lines |
| **Grand Total** | **50 patterns** | **~6931 lines** |

---

# Deep Analysis Round 3 (Items 51-62)

## 51. ✅ COMPLETED - HTTP Client Base with Request Builder

**Problem:** HTTP client initialization and request handling duplicated across 12+ provider files.

**Files affected:**
- `internal/modules/git/providers/github.go` (5+ locations)
- `internal/modules/git/providers/gitlab.go` (4+ locations)
- `internal/modules/git/providers/bitbucket.go` (4+ locations)
- `internal/modules/server/providers/digitalocean.go`
- `internal/modules/server/providers/hetzner.go`
- `internal/modules/server/providers/linode.go`
- `internal/modules/server/providers/vultr.go`
- `internal/modules/dns/providers/cloudflare.go`
- `internal/modules/dns/providers/digitalocean.go`
- `internal/modules/billing/providers/lemon_squeezy.go`
- `internal/modules/backup/storage/dropbox.go`
- `internal/modules/notification/channels/http_client.go`

**Current (duplicated 12+ times):**
```go
client := &http.Client{Timeout: 30 * time.Second}

req, err := http.NewRequestWithContext(ctx, method, url, body)
req.Header.Set("Authorization", "Bearer "+token)
req.Header.Set("Content-Type", "application/json")
resp, err := client.Do(req)
defer resp.Body.Close()
if resp.StatusCode < 200 || resp.StatusCode >= 300 {
    return nil, fmt.Errorf("request failed: %d", resp.StatusCode)
}
```

**Solution - Create `internal/pkg/httpclient/client.go`:**
```go
package httpclient

type Client struct {
    http      *http.Client
    baseURL   string
    authToken string
    headers   map[string]string
}

type ClientOption func(*Client)

func WithTimeout(d time.Duration) ClientOption {
    return func(c *Client) {
        c.http.Timeout = d
    }
}

func WithBearerToken(token string) ClientOption {
    return func(c *Client) {
        c.authToken = token
    }
}

func WithHeader(key, value string) ClientOption {
    return func(c *Client) {
        c.headers[key] = value
    }
}

func New(baseURL string, opts ...ClientOption) *Client {
    c := &Client{
        http:    &http.Client{Timeout: 30 * time.Second},
        baseURL: baseURL,
        headers: make(map[string]string),
    }
    for _, opt := range opts {
        opt(c)
    }
    return c
}

func (c *Client) Do(ctx context.Context, method, path string, body, result any) error {
    var reqBody io.Reader
    if body != nil {
        data, err := json.Marshal(body)
        if err != nil {
            return fmt.Errorf("marshal body: %w", err)
        }
        reqBody = bytes.NewReader(data)
    }

    req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
    if err != nil {
        return fmt.Errorf("create request: %w", err)
    }

    if c.authToken != "" {
        req.Header.Set("Authorization", "Bearer "+c.authToken)
    }
    req.Header.Set("Content-Type", "application/json")
    for k, v := range c.headers {
        req.Header.Set(k, v)
    }

    resp, err := c.http.Do(req)
    if err != nil {
        return fmt.Errorf("execute request: %w", err)
    }
    defer resp.Body.Close()

    respBody, _ := io.ReadAll(resp.Body)
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return &HTTPError{StatusCode: resp.StatusCode, Body: respBody}
    }

    if result != nil {
        if err := json.Unmarshal(respBody, result); err != nil {
            return fmt.Errorf("decode response: %w", err)
        }
    }
    return nil
}

func (c *Client) Get(ctx context.Context, path string, result any) error {
    return c.Do(ctx, http.MethodGet, path, nil, result)
}

func (c *Client) Post(ctx context.Context, path string, body, result any) error {
    return c.Do(ctx, http.MethodPost, path, body, result)
}

func (c *Client) Put(ctx context.Context, path string, body, result any) error {
    return c.Do(ctx, http.MethodPut, path, body, result)
}

func (c *Client) Delete(ctx context.Context, path string) error {
    return c.Do(ctx, http.MethodDelete, path, nil, nil)
}
```

**Usage in providers:**
```go
// Before (50+ lines per provider)
func (p *GitHubProvider) CreateWebhook(...) { ... lots of HTTP code ... }

// After (2 lines)
func (p *GitHubProvider) CreateWebhook(ctx context.Context, repo, url, secret string) (*Webhook, error) {
    var result Webhook
    return &result, p.client.Post(ctx, "/repos/"+repo+"/hooks", webhookPayload{...}, &result)
}
```

**Impact:** ~500 lines saved, consistent error handling, retry logic can be added centrally

---

## 52. Context Extraction Helpers (P1)

**Problem:** Unsafe context value extraction duplicated across 15+ middleware and handler files.

**Files affected:**
- `internal/middleware/auth.go` (lines 45-46, 88-89)
- `internal/middleware/team.go` (lines 71-72, 105-106, 171-172)
- `internal/middleware/team_role.go` (lines 32-35, 62-65, 92-95)
- `internal/middleware/email.go` (lines 24-26)
- `internal/middleware/two_factor.go` (lines 19-22)
- `internal/middleware/subscription.go` (lines 41-44)
- All handler files using `c.Locals("userID").(string)`

**Current (duplicated 15+ times):**
```go
userID, ok := c.Locals("userID").(string)
if !ok || userID == "" {
    return response.Unauthorized(c, "User not authenticated")
}
```

**Solution - Extend `internal/pkg/fiber/context.go`:**
```go
package fiber

import (
    "github.com/gofiber/fiber/v2"
    "launch-go/internal/pkg/response"
)

// Context keys
const (
    KeyUserID   = "userID"
    KeyTeamID   = "teamID"
    KeyTeamRole = "teamRole"
    KeyEmail    = "email"
)

// MustGetUserID returns userID or sends 401 response
func MustGetUserID(c *fiber.Ctx) (string, error) {
    userID, ok := c.Locals(KeyUserID).(string)
    if !ok || userID == "" {
        return "", response.Unauthorized(c, "User not authenticated")
    }
    return userID, nil
}

// MustGetTeamID returns teamID or sends 400 response
func MustGetTeamID(c *fiber.Ctx) (string, error) {
    teamID, ok := c.Locals(KeyTeamID).(string)
    if !ok || teamID == "" {
        return "", response.Error(c, fiber.StatusBadRequest, "Team context required")
    }
    return teamID, nil
}

// GetUserID returns userID or empty string (for optional auth)
func GetUserID(c *fiber.Ctx) string {
    if id, ok := c.Locals(KeyUserID).(string); ok {
        return id
    }
    return ""
}

// GetTeamRole returns role or empty string
func GetTeamRole(c *fiber.Ctx) string {
    if role, ok := c.Locals(KeyTeamRole).(string); ok {
        return role
    }
    return ""
}

// SetUserContext sets user-related context values
func SetUserContext(c *fiber.Ctx, userID, email string) {
    c.Locals(KeyUserID, userID)
    c.Locals(KeyEmail, email)
}

// SetTeamContext sets team-related context values
func SetTeamContext(c *fiber.Ctx, teamID, role string) {
    c.Locals(KeyTeamID, teamID)
    c.Locals(KeyTeamRole, role)
}
```

**Usage:**
```go
// Before (4 lines per extraction)
userID, ok := c.Locals("userID").(string)
if !ok || userID == "" {
    return response.Unauthorized(c, "User not authenticated")
}

// After (1 line)
userID, err := fiber.MustGetUserID(c)
if err != nil { return err }
```

**Impact:** ~200 lines saved, type safety, consistent error messages

---

## 53. JWT Token Parser Helper (P1)

**Problem:** JWT parsing logic duplicated in auth.go (Auth and OptionalAuth functions).

**Files affected:**
- `internal/middleware/auth.go` (lines 24-29, 67-72)
- `internal/websocket/auth.go`

**Current (duplicated 2+ times):**
```go
token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
    if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
        return nil, fiber.ErrUnauthorized
    }
    return []byte(jwtSecret), nil
})
```

**Solution - Create `internal/pkg/jwt/parser.go`:**
```go
package jwt

import (
    "github.com/golang-jwt/jwt/v5"
)

type Claims map[string]interface{}

func ParseToken(tokenString, secret string) (Claims, error) {
    token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, ErrInvalidSigningMethod
        }
        return []byte(secret), nil
    })
    if err != nil {
        return nil, ErrInvalidToken
    }

    claims, ok := token.Claims.(jwt.MapClaims)
    if !ok || !token.Valid {
        return nil, ErrInvalidToken
    }

    return Claims(claims), nil
}

func (c Claims) GetString(key string) string {
    if v, ok := c[key].(string); ok {
        return v
    }
    return ""
}

func (c Claims) GetUserID() string {
    return c.GetString("sub")
}

func (c Claims) GetEmail() string {
    return c.GetString("email")
}
```

**Impact:** ~100 lines saved, centralized JWT handling

---

## 54. Redis Client Factory (P1)

**Problem:** Redis client initialization duplicated 5 times across codebase.

**Files affected:**
- `internal/pkg/cache/redis.go` (lines 19-26)
- `internal/websocket/redis_broadcaster.go` (lines 20-31, 129-144)
- `internal/queue/client.go` (lines 14-25)
- `internal/queue/scheduler.go` (lines 15-25)

**Current (duplicated 5 times):**
```go
client := redis.NewClient(&redis.Options{
    Addr:     cfg.Address,
    Password: cfg.Password,
    DB:       cfg.DB,
})
```

**Solution - Create `internal/pkg/redis/client.go`:**
```go
package redis

import (
    "github.com/redis/go-redis/v9"
    "launch-go/internal/config"
)

var defaultClient *redis.Client

func InitClient(cfg config.RedisConfig) *redis.Client {
    defaultClient = redis.NewClient(&redis.Options{
        Addr:     cfg.Address,
        Password: cfg.Password,
        DB:       cfg.DB,
    })
    return defaultClient
}

func Client() *redis.Client {
    return defaultClient
}

func Options(cfg config.RedisConfig) *redis.Options {
    return &redis.Options{
        Addr:     cfg.Address,
        Password: cfg.Password,
        DB:       cfg.DB,
    }
}
```

**Impact:** ~100 lines saved, single source of Redis configuration

---

## 55. Home Directory Helper (P1)

**Problem:** Home directory path resolution duplicated 8+ times.

**Files affected:**
- `internal/modules/server/tasks/ssh_keys.go` (lines 59-61, 79-81, 96-98, 110-112)
- `internal/modules/site/models/queue.go` (lines 55-61, 64-70)
- `internal/pkg/taskrunner/connection.go` (lines 13-20)
- `internal/modules/server/models/task.go` (lines 69-86)

**Current (duplicated 8+ times):**
```go
homeDir := fmt.Sprintf("/home/%s", user)
if user == "root" {
    homeDir = "/root"
}
```

**Solution - Create `internal/pkg/paths/home.go`:**
```go
package paths

import "fmt"

// GetHomeDir returns the home directory for a user
func GetHomeDir(user string) string {
    if user == "root" {
        return "/root"
    }
    // Handle system users like ubuntu
    if user == "ubuntu" {
        return "/ubuntu"
    }
    return fmt.Sprintf("/home/%s", user)
}

// GetSSHDir returns the .ssh directory for a user
func GetSSHDir(user string) string {
    return GetHomeDir(user) + "/.ssh"
}

// GetTaskDir returns the .launch-tasks directory for a user
func GetTaskDir(user string) string {
    return GetHomeDir(user) + "/.launch-tasks"
}

// GetAuthorizedKeysPath returns the authorized_keys file path
func GetAuthorizedKeysPath(user string) string {
    return GetSSHDir(user) + "/authorized_keys"
}
```

**Impact:** ~80 lines saved, consistent path handling

---

## 56. Task File Path Builder (P2)

**Problem:** Task file naming pattern duplicated 5+ times.

**Files affected:**
- `internal/pkg/taskrunner/dispatcher.go` (lines 170-171, 358-359, 444)
- `internal/pkg/taskrunner/stream_monitor.go` (lines 119, 327)

**Current (duplicated 5+ times):**
```go
scriptFile := filepath.Join(scriptPath, fmt.Sprintf("task-%s.sh", taskID))
outputFile := filepath.Join(scriptPath, fmt.Sprintf("task-%s.log", taskID))
```

**Solution - Add to `internal/pkg/paths/tasks.go`:**
```go
package paths

import (
    "fmt"
    "path/filepath"
)

type TaskPaths struct {
    Dir    string
    Script string
    Log    string
    PID    string
}

func GetTaskPaths(taskDir, taskID string) TaskPaths {
    return TaskPaths{
        Dir:    taskDir,
        Script: filepath.Join(taskDir, fmt.Sprintf("task-%s.sh", taskID)),
        Log:    filepath.Join(taskDir, fmt.Sprintf("task-%s.log", taskID)),
        PID:    filepath.Join(taskDir, fmt.Sprintf("task-%s.pid", taskID)),
    }
}
```

**Impact:** ~60 lines saved, consistent task file naming

---

## 57. BeforeCreate Hook Consolidation (P2)

**Problem:** 10 models follow identical BeforeCreate pattern with default setting.

**Files affected:**
- `internal/modules/site/models/site.go` (lines 78-89)
- `internal/modules/server/models/server.go` (lines 71-85)
- `internal/modules/database/models/database_user.go` (lines 28-38)
- `internal/modules/backup/models/backup.go` (lines 35-53)
- `internal/modules/backup/models/backup_job.go` (lines 26-36)
- `internal/modules/server/models/installed_service.go` (lines 29-39)
- `internal/modules/server/models/task.go` (lines 29-39)
- `internal/modules/server/models/firewall_rule.go` (lines 29-39)
- `internal/modules/server/models/cron.go` (lines 27-37)
- `internal/modules/server/models/daemon.go` (lines 31-49)

**Current (duplicated 10 times):**
```go
func (m *Model) BeforeCreate(tx *gorm.DB) error {
    if err := m.BaseModel.BeforeCreate(tx); err != nil {
        return err
    }
    if m.Status == "" {
        m.Status = "pending"
    }
    return nil
}
```

**Solution - Add default setters to BaseModel:**
```go
// internal/pkg/models/base.go

type DefaultsProvider interface {
    SetDefaults()
}

func (b *BaseModel) BeforeCreate(tx *gorm.DB) error {
    if b.ID == "" {
        b.ID = utils.NewULID()
    }
    // Call SetDefaults if model implements it
    if dp, ok := tx.Statement.Dest.(DefaultsProvider); ok {
        dp.SetDefaults()
    }
    return nil
}

// Models just implement SetDefaults()
func (s *Site) SetDefaults() {
    if s.DeployToken == "" {
        s.DeployToken = utils.GenerateRandomString(32)
    }
}

func (t *Task) SetDefaults() {
    if t.Status == "" {
        t.Status = "pending"
    }
}
```

**Impact:** ~150 lines saved, cleaner model definitions

---

## 58. JSON Timestamp Helper (P2)

**Problem:** Timestamp formatting to string duplicated 26+ times across DTO files.

**Files affected:**
- `internal/modules/site/dto/responses.go` (lines 248-266)
- `internal/modules/backup/dto/responses.go` (lines 83-91)
- `internal/modules/database/dto/responses.go` (lines 62-75)
- `internal/modules/auth/dto/responses.go` (lines 122-140)
- `internal/modules/git/dto/responses.go` (lines 80-88)

**Current (duplicated 60+ times):**
```go
if model.CreatedAt != nil {
    createdAt = model.CreatedAt.Format(time.RFC3339)
}

if model.OptionalTime != nil {
    t := model.OptionalTime.Format(time.RFC3339)
    resp.OptionalTime = &t
}
```

**Solution - Create `internal/pkg/helpers/time.go`:**
```go
package helpers

import "time"

// FormatTimestamp formats a time pointer to RFC3339 string or empty
func FormatTimestamp(t *time.Time) string {
    if t == nil {
        return ""
    }
    return t.Format(time.RFC3339)
}

// FormatTimestampPtr returns a pointer to RFC3339 string or nil
func FormatTimestampPtr(t *time.Time) *string {
    if t == nil {
        return nil
    }
    s := t.Format(time.RFC3339)
    return &s
}

// ParseTimestamp parses RFC3339 string to time pointer
func ParseTimestamp(s string) *time.Time {
    if s == "" {
        return nil
    }
    t, err := time.Parse(time.RFC3339, s)
    if err != nil {
        return nil
    }
    return &t
}
```

**Usage:**
```go
// Before (4 lines)
if model.CreatedAt != nil {
    t := model.CreatedAt.Format(time.RFC3339)
    resp.CreatedAt = &t
}

// After (1 line)
resp.CreatedAt = helpers.FormatTimestampPtr(model.CreatedAt)
```

**Impact:** ~150 lines saved across DTO files

---

## 59. Optional/Required Middleware Factory (P2)

**Problem:** Auth and OptionalAuth middleware have 90% duplicated logic.

**Files affected:**
- `internal/middleware/auth.go` (Auth: lines 12-53, OptionalAuth: lines 55-95)
- `internal/middleware/team.go` (TeamScope: lines 37-76, OptionalTeamScope: lines 83-110)
- `internal/pkg/signedurl/middleware.go`

**Current pattern:**
```go
// Auth() returns error on failure
// OptionalAuth() calls c.Next() on failure

// 90% of logic is identical
```

**Solution - Create middleware factory:**
```go
package middleware

type AuthConfig struct {
    Required  bool
    JWTSecret string
}

func AuthMiddleware(cfg AuthConfig) fiber.Handler {
    return func(c *fiber.Ctx) error {
        claims, err := parseAuthHeader(c, cfg.JWTSecret)
        if err != nil {
            if cfg.Required {
                return response.Unauthorized(c, "Authentication required")
            }
            return c.Next() // Optional - continue without auth
        }

        fiber.SetUserContext(c, claims.GetUserID(), claims.GetEmail())
        return c.Next()
    }
}

// Convenience functions
func Auth(jwtSecret string) fiber.Handler {
    return AuthMiddleware(AuthConfig{Required: true, JWTSecret: jwtSecret})
}

func OptionalAuth(jwtSecret string) fiber.Handler {
    return AuthMiddleware(AuthConfig{Required: false, JWTSecret: jwtSecret})
}
```

**Impact:** ~100 lines saved, consistent auth behavior

---

## 60. Config Loading with Struct Tags (P2)

**Problem:** 10 config files repeat identical load/setDefaults pattern.

**Files affected:**
- `internal/config/app.go`
- `internal/config/database.go`
- `internal/config/redis.go`
- `internal/config/jwt.go`
- `internal/config/queue.go`
- `internal/config/cors.go`
- `internal/config/billing.go`
- `internal/config/git.go`
- `internal/config/sentry.go`
- `internal/config/slack.go`

**Current (repeated 10 times):**
```go
func loadRedisConfig() RedisConfig {
    return RedisConfig{
        Address:  viper.GetString("REDIS_ADDRESS"),
        Password: viper.GetString("REDIS_PASSWORD"),
        DB:       viper.GetInt("REDIS_DB"),
    }
}

func setRedisDefaults() {
    viper.SetDefault("REDIS_ADDRESS", "localhost:6379")
    viper.SetDefault("REDIS_PASSWORD", "")
    viper.SetDefault("REDIS_DB", 0)
}
```

**Solution - Use struct tags with reflection:**
```go
package config

type RedisConfig struct {
    Address  string `env:"REDIS_ADDRESS" default:"localhost:6379"`
    Password string `env:"REDIS_PASSWORD" default:""`
    DB       int    `env:"REDIS_DB" default:"0"`
}

// Generic loader using reflection
func Load[T any]() (*T, error) {
    var cfg T
    v := reflect.ValueOf(&cfg).Elem()
    t := v.Type()

    for i := 0; i < t.NumField(); i++ {
        field := t.Field(i)
        envKey := field.Tag.Get("env")
        defaultVal := field.Tag.Get("default")

        if envKey != "" {
            viper.SetDefault(envKey, defaultVal)
            switch field.Type.Kind() {
            case reflect.String:
                v.Field(i).SetString(viper.GetString(envKey))
            case reflect.Int:
                v.Field(i).SetInt(int64(viper.GetInt(envKey)))
            case reflect.Bool:
                v.Field(i).SetBool(viper.GetBool(envKey))
            }
        }
    }
    return &cfg, nil
}
```

**Impact:** ~200 lines saved, eliminates boilerplate config functions

---

## 61. Broadcast Channel Builder (P2)

**Problem:** Broadcast methods duplicate channel prefix patterns.

**Files affected:**
- `internal/websocket/redis_broadcaster.go` (lines 33-56)

**Current:**
```go
func (r *RedisBroadcaster) BroadcastToTeam(teamID, event string, data interface{}) {
    r.publish("team."+teamID, event, data)
}
func (r *RedisBroadcaster) BroadcastToServer(serverID, event string, data interface{}) {
    r.publish("server."+serverID, event, data)
}
func (r *RedisBroadcaster) BroadcastToSite(siteID, event string, data interface{}) {
    r.publish("site."+siteID, event, data)
}
func (r *RedisBroadcaster) BroadcastToDeployment(deploymentID, event string, data interface{}) {
    r.publish("deployment."+deploymentID, event, data)
}
```

**Solution - Use single generic method:**
```go
type ChannelType string

const (
    ChannelTeam       ChannelType = "team"
    ChannelServer     ChannelType = "server"
    ChannelSite       ChannelType = "site"
    ChannelDeployment ChannelType = "deployment"
)

func (r *RedisBroadcaster) Broadcast(channelType ChannelType, id, event string, data interface{}) {
    r.publish(string(channelType)+"."+id, event, data)
}

// Keep convenience methods as wrappers for backwards compatibility
func (r *RedisBroadcaster) BroadcastToTeam(teamID, event string, data interface{}) {
    r.Broadcast(ChannelTeam, teamID, event, data)
}
```

**Impact:** ~50 lines, cleaner API, type-safe channels

---

## 62. Pagination Query Helpers (P2)

**Problem:** Limit parameter extraction and validation duplicated across handlers.

**Files affected:**
- `internal/modules/server/handlers/metric_handler.go` (lines 33-36)
- `internal/modules/server/handlers/task_handler.go` (lines 16-19)

**Current:**
```go
limit, _ := strconv.Atoi(c.Query("limit", "100"))
if limit < 1 || limit > 1000 {
    limit = 100
}
```

**Solution - Extend `internal/pkg/pagination/pagination.go`:**
```go
package pagination

type LimitParams struct {
    Limit  int
    Offset int
}

type LimitConfig struct {
    Default int
    Max     int
}

func GetLimitParams(c *fiber.Ctx, cfg LimitConfig) LimitParams {
    limit, _ := strconv.Atoi(c.Query("limit", strconv.Itoa(cfg.Default)))
    offset, _ := strconv.Atoi(c.Query("offset", "0"))

    if limit < 1 || limit > cfg.Max {
        limit = cfg.Default
    }
    if offset < 0 {
        offset = 0
    }

    return LimitParams{Limit: limit, Offset: offset}
}

// Default configs
var (
    DefaultPagination = LimitConfig{Default: 15, Max: 100}
    MetricsPagination = LimitConfig{Default: 100, Max: 1000}
    TasksPagination   = LimitConfig{Default: 50, Max: 100}
)
```

**Impact:** ~100 lines, consistent pagination across handlers

---

## Extended Summary (Items 51-62)

| Category | Items | Est. LOC Saved |
|----------|-------|----------------|
| HTTP Client Base | Item 51 | ~500 lines |
| Context Extraction | Item 52 | ~200 lines |
| JWT Token Parser | Item 53 | ~100 lines |
| Redis Client Factory | Item 54 | ~100 lines |
| Home Directory Helper | Item 55 | ~80 lines |
| Task File Paths | Item 56 | ~60 lines |
| BeforeCreate Hooks | Item 57 | ~150 lines |
| JSON Timestamp Helper | Item 58 | ~150 lines |
| Middleware Factory | Item 59 | ~100 lines |
| Config Struct Tags | Item 60 | ~200 lines |
| Broadcast Channel Builder | Item 61 | ~50 lines |
| Pagination Helpers | Item 62 | ~100 lines |
| **Round 3 Total** | **12 patterns** | **~1790 lines** |

---

## Updated Final Summary

| Category | Items | Total LOC Saved |
|----------|-------|-----------------|
| Handler Bases | 5 patterns | ~400 lines |
| Repository Patterns | 5 patterns | ~1200 lines |
| Service Patterns | 3 patterns | ~450 lines |
| Job/Task Patterns | 3 patterns | ~800 lines |
| Model Mixins | 5 patterns | ~270 lines |
| Provider API Clients | 3 patterns | ~450 lines |
| Middleware Patterns | 1 pattern | ~30 lines |
| Config Patterns | 1 pattern | ~86 lines |
| DTO/Request Patterns | 2 patterns | ~550 lines |
| Testing Patterns | 2 patterns | ~250 lines |
| Infrastructure Patterns | 6 patterns | ~1075 lines |
| Validation & Response | 5 patterns | ~610 lines |
| Template & Script | 3 patterns | ~160 lines |
| Interface & Query | 4 patterns | ~500 lines |
| Security & Events | 3 patterns | ~100 lines |
| HTTP & Helpers (Round 3) | 12 patterns | ~1790 lines |
| **Grand Total** | **62 patterns** | **~8721 lines** |

---

# Deep Analysis Round 4 (Items 63-74)

## 63. ✅ COMPLETED - Error Type Consolidation

**Problem:** 5 different error types doing similar work across 4 packages.

**Files affected:**
- `internal/pkg/errors/errors.go` - `AppError` struct (lines 22-45)
- `internal/pkg/errors/notfound.go` - `ResourceError` struct (lines 7-21)
- `internal/pkg/response/errors.go` - `AppError` struct (lines 11-15)
- `internal/pkg/repository/errors.go` - `ModelError` struct (lines 20-26)
- `internal/middleware/error.go` - `HTTPStatusError` interface (lines 10-16)

**Also duplicated:**
- `HTTPStatusError` interface defined in BOTH `middleware/error.go` AND `response/errors.go`
- `ErrConnectionFailed` defined in 3 locations (notification channels, notification services, backup services)
- `ErrInvalidSignature` defined in both git and billing modules

**Current:**
```go
// 5 different error types!
type ResourceError struct { Message string; Status int }
type AppError struct { Err error; Status int; Message string }
type ModelError struct { Err error; Model string; ID string; Message string; Status int }
// Plus legacy sentinel errors
```

**Solution - Consolidate to single error package:**
```go
// internal/pkg/errors/error.go

// HTTPStatusError is THE interface for HTTP-aware errors
type HTTPStatusError interface {
    error
    HTTPStatus() int
}

// Error is the unified error type
type Error struct {
    Err      error  `json:"-"`
    Message  string `json:"message"`
    Status   int    `json:"-"`
    Resource string `json:"resource,omitempty"` // Optional: for "not found" errors
    Code     string `json:"code,omitempty"`     // Optional: machine-readable code
}

func (e *Error) Error() string { return e.Message }
func (e *Error) HTTPStatus() int { return e.Status }
func (e *Error) Unwrap() error { return e.Err }

// Factory functions
func NotFound(resource string) *Error {
    return &Error{Status: 404, Message: resource + " not found", Resource: resource}
}

func BadRequest(message string) *Error {
    return &Error{Status: 400, Message: message}
}

// ... other factories
```

**Impact:** ~200 lines saved, single source of truth for errors

---

## 64. WebSocket SSH Client Consolidation (P1)

**Problem:** WebSocket handlers duplicate 140+ lines of SSH client code instead of using existing abstraction.

**Files affected:**
- `internal/modules/websocket/handlers/terminal.go` (lines 132-175, 192-256)
- `internal/modules/websocket/handlers/script_execution.go` (lines 142-177, 191-266)

**Current (duplicated in both files):**
```go
signer, err := ssh.ParsePrivateKey([]byte(privateKey))
config := &ssh.ClientConfig{
    User: username,
    Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
    HostKeyCallback: ssh.InsecureIgnoreHostKey(),
    Timeout: 10 * time.Second,
}
conn, err := ssh.Dial("tcp", addr, config)
defer conn.Close()
session, err := conn.NewSession()
defer session.Close()
// ... manual stdout/stderr pipe handling
```

**Solution - Use existing taskrunner.SSHClient:**
```go
// Before (70+ lines per handler)
func (h *TerminalHandler) handleTerminal(...) {
    // ... all the SSH boilerplate
}

// After (10 lines)
func (h *TerminalHandler) handleTerminal(...) {
    sshClient, err := taskrunner.NewSSHClient(taskrunner.SSHConfig{
        Host:       server.GetIPAddress(),
        Port:       server.GetSSHPort(),
        User:       username,
        PrivateKey: privateKey,
        Timeout:    10 * time.Second,
    })
    if err != nil { return err }
    defer sshClient.Close()

    return sshClient.StreamOutput(ctx, command, func(line string) {
        wsConn.WriteMessage(websocket.TextMessage, []byte(line))
    })
}
```

**Impact:** ~140 lines saved, consistent SSH handling

---

## 65. Template Engine Consolidation (P1)

**Problem:** 4 separate template engine implementations with duplicate code.

**Files affected:**
- `internal/pkg/taskrunner/template_engine.go` (central - 176 lines)
- `internal/modules/site/tasks/templates/templates.go` (duplicate - 79 lines)
- `internal/modules/server/tasks/templates/templates.go` (duplicate - 138 lines)
- `internal/modules/database/tasks/templates/templates.go` (duplicate - 76 lines)

**Duplicated patterns:**
- `ShellDefaults()` defined 3 times with different versions (`set -eu` vs `set -euo pipefail`)
- `Render()` and `MustRender()` functions duplicated
- Each module has own embed.FS and template loading

**Solution - Centralize to taskrunner template engine:**
```go
// internal/pkg/taskrunner/template_engine.go - Enhanced

// RegisterModuleTemplates allows modules to register their templates
func (e *TemplateEngine) RegisterModuleTemplates(name string, fs embed.FS, pattern string) error

// ShellDefaults returns standard shell script header
func ShellDefaults() string {
    return `#!/bin/bash
set -euo pipefail
`
}

// CommonFunctions returns shared bash helper functions
func CommonFunctions() string {
    return `
function log_info() { echo "[INFO] $1"; }
function log_error() { echo "[ERROR] $1" >&2; }
function check_command() { command -v "$1" >/dev/null 2>&1; }
`
}
```

**Usage in modules:**
```go
// Instead of defining own template engine
func init() {
    taskrunner.DefaultEngine.RegisterModuleTemplates("site", templatesFS, "templates/*.sh")
}
```

**Impact:** ~200 lines saved, consistent shell headers

---

## 66. Slack Admin Alert Base (P2)

**Problem:** 4 Slack admin alert implementations with nearly identical builder patterns.

**Files affected:**
- `internal/modules/notification/slack/server_provisioning_failed_alert.go` (lines 48-79)
- `internal/modules/notification/slack/php_installation_failed_alert.go` (lines 30-68)
- `internal/modules/notification/slack/php_extension_install_failed_alert.go` (lines 32-80)
- `internal/modules/notification/slack/php_extension_uninstall_failed_alert.go` (lines 32-80)

**Duplicated builder methods (4x each):**
```go
func (a *Alert) WithUser(user *UserInfo) *Alert { a.user = user; return a }
func (a *Alert) WithOutput(output string) *Alert { a.output = output; return a }
func (a *Alert) WithErrorMessage(errorMessage string) *Alert { ... }
func (a *Alert) WithOutputRetrievalError(err string) *Alert { ... }
```

**Solution - Create base alert struct:**
```go
// internal/modules/notification/slack/base_alert.go

type BaseAlert struct {
    server               *ServerInfo
    user                 *UserInfo
    output               string
    errorMessage         string
    outputRetrievalError string
}

func (a *BaseAlert) WithUser(user *UserInfo) *BaseAlert { a.user = user; return a }
func (a *BaseAlert) WithOutput(output string) *BaseAlert { a.output = output; return a }
func (a *BaseAlert) WithErrorMessage(msg string) *BaseAlert { a.errorMessage = msg; return a }
func (a *BaseAlert) WithOutputRetrievalError(err string) *BaseAlert { a.outputRetrievalError = err; return a }

// BuildServerSection returns common server info section
func (a *BaseAlert) BuildServerSection() []slack.Block { ... }

// BuildOutputSection returns truncated output section
func (a *BaseAlert) BuildOutputSection() []slack.Block { ... }

// Concrete alerts only implement title and specific fields
type ServerProvisioningFailedAlert struct {
    BaseAlert
}

func (a *ServerProvisioningFailedAlert) Title() string {
    return "🚨 Server Provisioning Failed"
}
```

**Impact:** ~120 lines saved across 4 alert types

---

## 67. Notification Format Helper (P2)

**Problem:** 6+ notification types duplicate identical `formatWithLogs()` pattern.

**Files affected:**
- `internal/modules/notification/notifications/server_provisioning_failed.go`
- `internal/modules/notification/notifications/php_installation_failed.go`
- `internal/modules/notification/notifications/php_extension_install_failed.go`
- `internal/modules/notification/notifications/php_extension_uninstall_failed.go`
- `internal/modules/notification/notifications/deployment_failed.go`
- `internal/modules/notification/notifications/site_installation_failed.go`

**Duplicated pattern (6+ times):**
```go
func (n *Notification) ToSlack() string { return n.formatWithLogs() }
func (n *Notification) ToDiscord() string { return n.formatWithLogs() }
func (n *Notification) ToTelegram() string { return n.formatWithLogs() }

func (n *Notification) formatWithLogs() string {
    message := "*EMOJI Action Type*\n\n"
    message += fmt.Sprintf("Main message: %s", n.Field)
    if n.Output != "" {
        message += fmt.Sprintf("\n\n*Last lines of output:*\n```\n%s\n```", n.Output)
    }
    if n.ErrorMessage != "" {
        message += fmt.Sprintf("\n\n*Error message:*\n```\n%s\n```", n.ErrorMessage)
    }
    return message
}
```

**Solution - Create notification formatter:**
```go
// internal/modules/notification/notifications/formatter.go

type FailureNotification struct {
    Emoji        string
    Title        string
    MainMessage  string
    Output       string
    ErrorMessage string
}

func (f *FailureNotification) FormatMarkdown() string {
    var b strings.Builder
    b.WriteString(fmt.Sprintf("*%s %s*\n\n", f.Emoji, f.Title))
    b.WriteString(f.MainMessage)

    if f.Output != "" {
        b.WriteString(fmt.Sprintf("\n\n*Last lines of output:*\n```\n%s\n```", f.Output))
    }
    if f.ErrorMessage != "" {
        b.WriteString(fmt.Sprintf("\n\n*Error message:*\n```\n%s\n```", f.ErrorMessage))
    }
    return b.String()
}

// Usage in notifications
func (n *DeploymentFailedNotification) ToSlack() string {
    return (&FailureNotification{
        Emoji:        "🚨",
        Title:        "Deployment Failed",
        MainMessage:  fmt.Sprintf("Deployment for site '%s' failed", n.SiteName),
        Output:       n.Output,
        ErrorMessage: n.ErrorMessage,
    }).FormatMarkdown()
}
```

**Impact:** ~150 lines saved across notification types

---

## 68. Webhook Signature Verification Consolidation (P2)

**Problem:** 4 different signature verification approaches across webhook handlers.

**Files affected:**
- `internal/modules/git/handlers/webhook_handler.go` - provider-specific
- `internal/modules/billing/handlers/webhook_handler.go` - custom HMAC-SHA256
- `internal/modules/server/handlers/task_webhook_handler.go` - signedurl package
- `internal/modules/server/handlers/metrics_webhook_handler.go` - signedurl package

**Current inconsistencies:**
```go
// Git - provider-specific header extraction
func (h *WebhookHandler) getSignature(c *fiber.Ctx, providerType) string {
    switch providerType {
    case GitHub: return c.Get("X-Hub-Signature-256")
    case GitLab: return c.Get("X-Gitlab-Token")
    }
}

// Billing - custom HMAC
func (h *WebhookHandler) VerifySignature(payload []byte, signature string) bool {
    mac := hmac.New(sha256.New, []byte(h.webhookSecret))
    mac.Write(payload)
    expected := hex.EncodeToString(mac.Sum(nil))
    return hmac.Equal([]byte(expected), []byte(signature))
}

// Task/Metrics - signedurl
return signedurl.ValidateSignedURL(c, h.signer)
```

**Solution - Create webhook verification package:**
```go
// internal/pkg/webhook/verify.go

type Verifier interface {
    Verify(payload []byte, signature string) bool
}

// HMAC-SHA256 verifier
type HMACVerifier struct {
    secret []byte
}

func NewHMACVerifier(secret string) *HMACVerifier {
    return &HMACVerifier{secret: []byte(secret)}
}

func (v *HMACVerifier) Verify(payload []byte, signature string) bool {
    mac := hmac.New(sha256.New, v.secret)
    mac.Write(payload)
    expected := hex.EncodeToString(mac.Sum(nil))
    return hmac.Equal([]byte(expected), []byte(signature))
}

// Header extractors
func GetGitHubSignature(c *fiber.Ctx) string { return c.Get("X-Hub-Signature-256") }
func GetGitLabSignature(c *fiber.Ctx) string { return c.Get("X-Gitlab-Token") }
func GetLemonSqueezySignature(c *fiber.Ctx) string { return c.Get("X-Signature") }
```

**Impact:** ~80 lines saved, consistent webhook handling

---

## 69. Route Middleware Chain Helper (P1)

**Problem:** 20 instances of identical middleware chain across all route files.

**Files affected (all routes.go files):**
- `internal/modules/backup/routes.go` (2 instances)
- `internal/modules/database/routes.go` (2 instances)
- `internal/modules/dns/routes.go` (2 instances)
- `internal/modules/git/routes.go` (4 instances)
- `internal/modules/notification/routes.go` (2 instances)
- `internal/modules/script/routes.go` (2 instances)
- `internal/modules/server/routes.go` (3 instances)
- `internal/modules/site/routes.go` (2 instances)

**Current (duplicated 20 times):**
```go
router.Group("/path", authMiddleware, middleware.TeamScope(), middleware.VerifySubscription())
```

**Solution - Create route helper:**
```go
// internal/pkg/fiber/group.go

// TeamProtectedGroup creates a route group with standard team authentication
func TeamProtectedGroup(router fiber.Router, path string, authMiddleware fiber.Handler) fiber.Router {
    return router.Group(path, authMiddleware, middleware.TeamScope(), middleware.VerifySubscription())
}

// TeamProtectedGroupNoSubscription for routes that skip subscription check
func TeamProtectedGroupNoSubscription(router fiber.Router, path string, authMiddleware fiber.Handler) fiber.Router {
    return router.Group(path, authMiddleware, middleware.TeamScope())
}

// Usage in routes
providers := fiber.TeamProtectedGroup(router, "/server-providers", authMiddleware)
```

**Impact:** ~60 lines saved, single source of truth for auth chain

---

## 70. Module ServiceDeps Generic (P1)

**Problem:** 5 modules define nearly identical ServiceDeps struct.

**Files affected:**
- `internal/modules/git/services/base.go` (lines 13-22)
- `internal/modules/backup/services/base.go` (lines 13-23)
- `internal/modules/site/services/base.go` (lines 15-22)
- `internal/modules/dns/services/base.go` (lines 25-35)
- `internal/modules/notification/services/base.go` (lines 12-24)

**Current (duplicated 5 times):**
```go
type ServiceDeps struct {
    service.Dependencies
    Repos *repositories.Registry
    // Module-specific optional fields
}

type BaseService struct {
    service.Base
    deps  *ServiceDeps
    repos *repositories.Registry
}

func NewBaseService(deps *ServiceDeps) *BaseService {
    return &BaseService{
        Base:  service.NewBaseFromDeps(deps.Dependencies),
        deps:  deps,
        repos: deps.Repos,
    }
}
```

**Solution - Create generic base in service package:**
```go
// internal/pkg/service/module_base.go

// ModuleDeps is a generic service dependencies container
type ModuleDeps[R any] struct {
    Dependencies
    Repos R
}

// ModuleBase is a generic base service for modules
type ModuleBase[R any] struct {
    Base
    deps  *ModuleDeps[R]
    repos R
}

func NewModuleBase[R any](deps *ModuleDeps[R]) *ModuleBase[R] {
    return &ModuleBase[R]{
        Base:  NewBaseFromDeps(deps.Dependencies),
        deps:  deps,
        repos: deps.Repos,
    }
}

func (s *ModuleBase[R]) Repos() R { return s.repos }

// Usage in modules
type SiteService struct {
    *service.ModuleBase[*repositories.Registry]
    // site-specific fields
}
```

**Impact:** ~150 lines saved across 5 modules

---

## 71. JobContext Generic Base (P2)

**Problem:** 6 modules define nearly identical JobContext struct.

**Files affected:**
- `internal/modules/site/jobs/context.go` (lines 21-38)
- `internal/modules/server/jobs/context.go` (lines 19-29)
- `internal/modules/backup/jobs/context.go` (line 15)
- `internal/modules/script/jobs/context.go` (line 19)
- `internal/modules/git/jobs/process_webhook.go` (line 320)
- `internal/modules/database/jobs/` (implicit)

**Current pattern (duplicated 6 times):**
```go
type JobContext struct {
    pkgjobs.Base
    repos *repositories.Registry
    // Module-specific fields
}

func NewJobContext(deps pkgjobs.Dependencies, repos *repositories.Registry) *JobContext {
    return &JobContext{
        Base:  pkgjobs.NewBase(deps),
        repos: repos,
    }
}
```

**Solution - Create generic in jobs package:**
```go
// internal/pkg/jobs/module_context.go

type ModuleContext[R any] struct {
    Base
    repos R
}

func NewModuleContext[R any](deps Dependencies, repos R) *ModuleContext[R] {
    return &ModuleContext[R]{
        Base:  NewBase(deps),
        repos: repos,
    }
}

func (c *ModuleContext[R]) Repos() R { return c.repos }

// Usage in modules
type SiteJobContext = pkgjobs.ModuleContext[*repositories.Registry]
```

**Impact:** ~100 lines saved across 6 modules

---

## 72. Token Generation Consolidation (P2)

**Problem:** Duplicate token generation implementations.

**Files affected:**
- `internal/pkg/utils/token.go` - `GenerateBase64Token()` (lines 19-27)
- `internal/modules/auth/models/password_reset_token.go` - `GenerateToken()` (lines 32-39)
- `internal/modules/site/models/helpers.go` - deprecated wrappers (lines 7-17)
- `internal/modules/site/models/site.go` - `generateLaravelAppKey()` duplicate (lines 187-196)

**Current duplicates:**
```go
// utils/token.go
func GenerateBase64Token(length int) string {
    bytes := make([]byte, length)
    rand.Read(bytes)
    return base64.URLEncoding.EncodeToString(bytes)[:length]
}

// auth/models/password_reset_token.go - DUPLICATE with different error handling
func GenerateToken(length int) (string, error) {
    bytes := make([]byte, length)
    rand.Read(bytes)
    return base64.URLEncoding.EncodeToString(bytes), nil
}
```

**Solution - Remove duplicates, use centralized:**
```go
// Delete auth/models/password_reset_token.go:GenerateToken()
// Delete site/models/helpers.go deprecated wrappers
// Delete site/models/site.go:generateLaravelAppKey()

// Update call sites to use:
utils.GenerateBase64Token(length)
utils.GenerateAppKey()
```

**Impact:** ~50 lines saved, remove deprecated code

---

## 73. Git Ref Trimming Helper (P2)

**Problem:** Git ref trimming pattern duplicated 6 times.

**Files affected:**
- `internal/modules/git/handlers/webhook_handler.go` (2 occurrences)
- `internal/modules/git/jobs/process_webhook.go` (2 occurrences)
- `internal/modules/git/providers/types.go` (2 occurrences)

**Current (duplicated 6 times):**
```go
branch := strings.TrimPrefix(ref, "refs/heads/")
```

**Solution - Create git helper:**
```go
// internal/modules/git/helpers.go

const (
    RefHeadsPrefix = "refs/heads/"
    RefTagsPrefix  = "refs/tags/"
)

// ExtractBranchName extracts branch name from git ref
func ExtractBranchName(ref string) string {
    return strings.TrimPrefix(ref, RefHeadsPrefix)
}

// ExtractTagName extracts tag name from git ref
func ExtractTagName(ref string) string {
    return strings.TrimPrefix(ref, RefTagsPrefix)
}

// IsBranchRef checks if ref is a branch reference
func IsBranchRef(ref string) bool {
    return strings.HasPrefix(ref, RefHeadsPrefix)
}
```

**Impact:** ~30 lines saved, clearer intent

---

## 74. Channel Validation Rules Consolidation (P2)

**Problem:** Notification channel validation rules duplicated across 4 drivers.

**Files affected:**
- `internal/modules/notification/channels/email.go` (lines 67-73)
- `internal/modules/notification/channels/slack.go` (lines 83-89)
- `internal/modules/notification/channels/discord.go` (lines 83-89)
- `internal/modules/notification/channels/telegram.go` (lines 110-117)

**Current (repeated validation rules):**
```go
// Common rules duplicated in each channel
"appDeploy":      "boolean"
"databaseBackup": "boolean"
```

**Solution - Create shared validation rules:**
```go
// internal/modules/notification/channels/validation.go

// CommonNotificationRules are shared across all channels
var CommonNotificationRules = map[string]string{
    "appDeploy":      "boolean",
    "databaseBackup": "boolean",
}

// MergeValidationRules combines common and channel-specific rules
func MergeValidationRules(channelRules map[string]string) map[string]string {
    result := make(map[string]string, len(CommonNotificationRules)+len(channelRules))
    for k, v := range CommonNotificationRules {
        result[k] = v
    }
    for k, v := range channelRules {
        result[k] = v
    }
    return result
}

// Usage in channels
func (c *SlackChannel) GetCreateRules() map[string]string {
    return MergeValidationRules(map[string]string{
        "webhook_url": "required,url",
    })
}
```

**Impact:** ~40 lines saved, consistent validation

---

## Extended Summary (Items 63-74)

| Category | Items | Est. LOC Saved |
|----------|-------|----------------|
| Error Type Consolidation | Item 63 | ~200 lines |
| WebSocket SSH Client | Item 64 | ~140 lines |
| Template Engine Consolidation | Item 65 | ~200 lines |
| Slack Admin Alert Base | Item 66 | ~120 lines |
| Notification Format Helper | Item 67 | ~150 lines |
| Webhook Verification | Item 68 | ~80 lines |
| Route Middleware Helper | Item 69 | ~60 lines |
| Module ServiceDeps Generic | Item 70 | ~150 lines |
| JobContext Generic Base | Item 71 | ~100 lines |
| Token Generation Cleanup | Item 72 | ~50 lines |
| Git Ref Helper | Item 73 | ~30 lines |
| Channel Validation Rules | Item 74 | ~40 lines |
| **Round 4 Total** | **12 patterns** | **~1320 lines** |

---

## Updated Final Summary

| Category | Items | Total LOC Saved |
|----------|-------|-----------------|
| Handler Bases | 5 patterns | ~400 lines |
| Repository Patterns | 5 patterns | ~1200 lines |
| Service Patterns | 3 patterns | ~450 lines |
| Job/Task Patterns | 3 patterns | ~800 lines |
| Model Mixins | 5 patterns | ~270 lines |
| Provider API Clients | 3 patterns | ~450 lines |
| Middleware Patterns | 1 pattern | ~30 lines |
| Config Patterns | 1 pattern | ~86 lines |
| DTO/Request Patterns | 2 patterns | ~550 lines |
| Testing Patterns | 2 patterns | ~250 lines |
| Infrastructure Patterns | 6 patterns | ~1075 lines |
| Validation & Response | 5 patterns | ~610 lines |
| Template & Script | 3 patterns | ~160 lines |
| Interface & Query | 4 patterns | ~500 lines |
| Security & Events | 3 patterns | ~100 lines |
| HTTP & Helpers (Round 3) | 12 patterns | ~1790 lines |
| Error & DI (Round 4) | 12 patterns | ~1320 lines |
| **Grand Total** | **74 patterns** | **~10,041 lines** |

---

# Deep Analysis Round 5 (Items 75-86)

## 75. ✅ COMPLETED - GORM Query Scope - Order By Latest

**Problem:** `Order("created_at DESC")` appears 40+ times across repositories.

**Files affected:**
- `internal/pkg/repository/base.go` (lines 64, 74, 84)
- `internal/pkg/repository/installable.go` (lines 65, 75, 85, 95)
- `internal/pkg/activity/repository.go` (lines 29, 38, 47, 122)
- `internal/modules/git/repositories/source_control_repository.go` (lines 82, 94)
- `internal/modules/backup/repositories/backup_repository.go` (lines 107, 163)
- `internal/modules/site/repositories/site_repository.go` (lines 100, 110, 130)
- `internal/modules/database/repositories/database_repository.go` (lines 96, 109)
- `internal/modules/notification/repositories/notification_channel_repository.go` (4 occurrences)
- 10+ more repository files

**Current (duplicated 40+ times):**
```go
err := r.DB.WithContext(ctx).
    Where("server_id = ?", serverID).
    Order("created_at DESC").  // DUPLICATED
    Find(&entities).Error
```

**Solution - Create query scopes:**
```go
// internal/pkg/repository/scopes.go

func ScopeLatest(db *gorm.DB) *gorm.DB {
    return db.Order("created_at DESC")
}

func ScopeOldest(db *gorm.DB) *gorm.DB {
    return db.Order("created_at ASC")
}

func ScopeByServer(serverID string) func(*gorm.DB) *gorm.DB {
    return func(db *gorm.DB) *gorm.DB {
        return db.Where("server_id = ?", serverID)
    }
}

func ScopeByTeam(teamID string) func(*gorm.DB) *gorm.DB {
    return func(db *gorm.DB) *gorm.DB {
        return db.Where("team_id = ?", teamID)
    }
}

// Usage
err := r.DB.WithContext(ctx).
    Scopes(ScopeByServer(serverID), ScopeLatest).
    Find(&entities).Error
```

**Impact:** ~120 lines saved, consistent query patterns

---

## 76. Sync Associations Transaction Pattern (P2)

**Problem:** Backup repository has identical transaction pattern repeated 3 times.

**Files affected:**
- `internal/modules/backup/repositories/backup_repository.go` (lines 33-50, 119-142, 189-209)

**Current (duplicated 3 times):**
```go
func (r *BackupRepository) SyncBackupDatabases(ctx context.Context, backupID string, databaseIDs []string) error {
    return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        // Delete existing associations
        if err := tx.Where("backup_id = ?", backupID).Delete(&models.BackupDatabase{}).Error; err != nil {
            return err
        }
        // Create new associations
        for _, dbID := range databaseIDs {
            if err := tx.Create(&models.BackupDatabase{BackupID: backupID, DatabaseID: dbID}).Error; err != nil {
                return err
            }
        }
        return nil
    })
}
```

**Solution - Create generic sync helper:**
```go
// internal/pkg/repository/associations.go

func SyncAssociations[T any](db *gorm.DB, ctx context.Context,
    foreignKey string, foreignID string,
    associations []T) error {
    return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        // Delete existing
        if err := tx.Where(foreignKey+" = ?", foreignID).Delete(new(T)).Error; err != nil {
            return err
        }
        // Create new
        for _, assoc := range associations {
            if err := tx.Create(&assoc).Error; err != nil {
                return err
            }
        }
        return nil
    })
}
```

**Impact:** ~60 lines saved

---

## 77. WebSocket Handler Base Struct (P1)

**Problem:** 5 WebSocket handlers have identical struct and initialization.

**Files affected:**
- `internal/modules/websocket/handlers/terminal.go` (lines 32-47)
- `internal/modules/websocket/handlers/logs.go` (lines 23-38)
- `internal/modules/websocket/handlers/metrics.go` (lines 20-35)
- `internal/modules/websocket/handlers/service_status.go` (lines 42-57)
- `internal/modules/websocket/handlers/script_execution.go` (lines 23-38)

**Current (duplicated 5 times):**
```go
type TerminalHandler struct {
    db              *gorm.DB
    jwtSecret       string
    logger          zerolog.Logger
    membershipCache *cache.TeamMembershipCache
}

func NewTerminalHandler(db *gorm.DB, jwtSecret string, logger zerolog.Logger,
    membershipCache *cache.TeamMembershipCache) *TerminalHandler {
    return &TerminalHandler{
        db:              db,
        jwtSecret:       jwtSecret,
        logger:          logger.With().Str("component", "terminal").Logger(),
        membershipCache: membershipCache,
    }
}
```

**Solution - Create base WebSocket handler:**
```go
// internal/modules/websocket/handlers/base.go

type BaseHandler struct {
    DB              *gorm.DB
    JWTSecret       string
    Logger          zerolog.Logger
    MembershipCache *cache.TeamMembershipCache
}

func NewBaseHandler(db *gorm.DB, jwtSecret string, logger zerolog.Logger,
    cache *cache.TeamMembershipCache, component string) BaseHandler {
    return BaseHandler{
        DB:              db,
        JWTSecret:       jwtSecret,
        Logger:          logger.With().Str("component", component).Logger(),
        MembershipCache: cache,
    }
}

// Handlers embed BaseHandler
type TerminalHandler struct {
    BaseHandler
}

func NewTerminalHandler(...) *TerminalHandler {
    return &TerminalHandler{BaseHandler: NewBaseHandler(db, jwtSecret, logger, cache, "terminal")}
}
```

**Impact:** ~100 lines saved

---

## 78. WebSocket Message Builder (P2)

**Problem:** Inconsistent JSON message building across WebSocket handlers.

**Files affected:**
- `internal/modules/websocket/handlers/script_execution.go` (lines 331-358) - manual string building
- `internal/modules/websocket/handlers/metrics.go` (lines 84-86) - fmt.Sprintf
- `internal/modules/websocket/handlers/service_status.go` (lines 212-234) - json.Marshal
- `internal/modules/websocket/handlers/logs.go` (lines 60-78) - raw WriteMessage

**Current inconsistencies:**
```go
// Pattern A: Manual string building (error-prone)
msg := "{"
for k, v := range data {
    msg += fmt.Sprintf(`"%s":"%s",`, k, v)
}

// Pattern B: fmt.Sprintf (unescaped)
c.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`{"event":"error","message":"%s"}`, msg)))

// Pattern C: json.Marshal (correct)
data, _ := json.Marshal(msg)
c.WriteMessage(websocket.TextMessage, data)
```

**Solution - Create message builder:**
```go
// internal/modules/websocket/message.go

type WSMessage struct {
    Event   string      `json:"event"`
    Data    interface{} `json:"data,omitempty"`
    Message string      `json:"message,omitempty"`
}

func SendJSON(c *websocket.Conn, event string, data interface{}) error {
    msg := WSMessage{Event: event, Data: data}
    bytes, err := json.Marshal(msg)
    if err != nil {
        return err
    }
    return c.WriteMessage(websocket.TextMessage, bytes)
}

func SendError(c *websocket.Conn, message string) {
    SendJSON(c, "error", map[string]string{"message": message})
    c.Close()
}
```

**Impact:** ~80 lines saved, eliminates JSON injection risks

---

## 79. ✅ COMPLETED - Job Struct Boilerplate

**Problem:** 77 job files have identical struct definition and constructor.

**Files affected:** All 77 job files across modules:
- `internal/modules/server/jobs/*.go` (42 files)
- `internal/modules/site/jobs/*.go` (23 files)
- `internal/modules/database/jobs/*.go` (6 files)
- `internal/modules/backup/jobs/*.go` (4 files)
- `internal/modules/script/jobs/*.go` (2 files)

**Current (duplicated 77 times):**
```go
type InstallDaemonJob struct {
    ctx     *JobContext
    Payload InstallDaemonPayload
}

func NewInstallDaemonJob(ctx *JobContext, payload InstallDaemonPayload) *InstallDaemonJob {
    return &InstallDaemonJob{
        ctx:     ctx,
        Payload: payload,
    }
}
```

**Solution - Use generics or code generation:**
```go
// Option 1: Generic base (requires Go 1.18+)
// internal/pkg/jobs/base_job.go

type BaseJob[P any, C any] struct {
    Ctx     *C
    Payload P
}

func NewJob[P any, C any](ctx *C, payload P) *BaseJob[P, C] {
    return &BaseJob[P, C]{Ctx: ctx, Payload: payload}
}

// Option 2: Embed pattern
type InstallDaemonJob struct {
    *pkgjobs.BaseJob[InstallDaemonPayload, JobContext]
}
```

**Impact:** ~400 lines saved across 77 files

---

## 80. Cloud Provider HTTP Client (P1)

**Problem:** 6 provider files duplicate identical HTTP request handling.

**Files affected:**
- `internal/modules/server/providers/digitalocean.go` (lines 263-306)
- `internal/modules/server/providers/hetzner.go` (lines 222-256)
- `internal/modules/server/providers/linode.go` (lines 211-245)
- `internal/modules/server/providers/vultr.go` (lines 218-252)
- `internal/modules/dns/providers/digitalocean.go` (lines 429-441)
- `internal/modules/dns/providers/cloudflare.go` (lines 513-525)

**Current (duplicated 6 times with inconsistent error handling):**
```go
func (p *Provider) doRequest(ctx context.Context, token, method, path string, body interface{}) (map[string]interface{}, error) {
    var reqBody io.Reader
    if body != nil {
        jsonBody, err := json.Marshal(body)  // Some providers ignore this error!
        if err != nil {
            return nil, err
        }
        reqBody = bytes.NewBuffer(jsonBody)
    }
    req, err := http.NewRequestWithContext(ctx, method, baseURL+path, reqBody)
    req.Header.Set("Authorization", "Bearer "+token)
    // ... 20 more lines
}
```

**Solution - Use shared HTTP client from Pattern 51:**
```go
// Already proposed in Pattern 51: internal/pkg/httpclient/client.go
// Providers should use:

func NewDigitalOceanProvider(config *ProviderConfig) *DigitalOceanProvider {
    return &DigitalOceanProvider{
        client: httpclient.New("https://api.digitalocean.com/v2",
            httpclient.WithBearerToken(config.Token),
        ),
    }
}
```

**Impact:** ~250 lines saved, consistent error handling

---

## 81. ✅ COMPLETED - DTO Timestamp Formatter

**Problem:** Timestamp formatting duplicated 50+ times across DTO files.

**Files affected:**
- `internal/modules/auth/dto/responses.go` (lines 121-140)
- `internal/modules/server/dto/responses.go` (lines 78-86, 123-136, 191-197)
- `internal/modules/site/dto/responses.go` (lines 196-266)
- `internal/modules/database/dto/responses.go` (lines 58-75, 95-112)
- `internal/modules/backup/dto/responses.go` (lines 42-91, 137-145)
- `internal/modules/git/dto/responses.go` (lines 38-87)

**Current (duplicated 50+ times):**
```go
createdAt := ""
if server.CreatedAt != nil {
    createdAt = server.CreatedAt.Format(time.RFC3339)
}

// Optional pointer version
if server.ProvisionedAt != nil {
    formatted := server.ProvisionedAt.Format(time.RFC3339)
    resp.ProvisionedAt = &formatted
}
```

**Solution - Create DTO helpers (extends Pattern 58):**
```go
// internal/pkg/dto/time.go

func FormatTime(t *time.Time) string {
    if t == nil {
        return ""
    }
    return t.Format(time.RFC3339)
}

func FormatTimePtr(t *time.Time) *string {
    if t == nil {
        return nil
    }
    s := t.Format(time.RFC3339)
    return &s
}

// Usage
resp.CreatedAt = dto.FormatTime(model.CreatedAt)
resp.ProvisionedAt = dto.FormatTimePtr(model.ProvisionedAt)
```

**Impact:** ~200 lines saved

---

## 82. DTO List Transformation (P2)

**Problem:** Slice transformation repeated 35+ times in handlers and DTOs.

**Files affected:**
- `internal/modules/server/handlers/server_handler.go` (lines 46-49, 63-66)
- `internal/modules/site/handlers/site_handler.go` (lines 45-48)
- `internal/modules/database/dto/responses.go` (lines 126-143)
- `internal/modules/notification/dto/responses.go` (lines 79-86)
- 25+ more locations

**Current (duplicated 35+ times):**
```go
result := make([]dto.ServerResponse, len(servers))
for i := range servers {
    result[i] = dto.ToServerResponse(&servers[i])
}
```

**Solution - Create generic transformer:**
```go
// internal/pkg/dto/transform.go

func TransformSlice[T any, R any](items []T, transform func(*T) R) []R {
    result := make([]R, len(items))
    for i := range items {
        result[i] = transform(&items[i])
    }
    return result
}

// Usage
result := dto.TransformSlice(servers, dto.ToServerResponse)
```

**Impact:** ~100 lines saved

---

## 83. Activity Logging Helper (P1)

**Problem:** Activity logging pattern duplicated 75+ times across services and jobs.

**Files affected:**
- `internal/modules/auth/services/team_service.go` (lines 42-48, 75-81, 107-113)
- `internal/modules/server/services/cron_service.go` (3 occurrences)
- `internal/modules/server/services/firewall_rule_service.go` (3 occurrences)
- `internal/modules/server/services/daemon_service.go` (4 occurrences)
- `internal/modules/database/services/database_service.go` (2 occurrences)
- `internal/modules/site/services/site_service.go` (3 occurrences)
- 25+ job files with similar pattern

**Current (duplicated 75+ times):**
```go
logger := activity.New(s.repos.DB()).
    WithContext(ctx).
    UseLog("server").
    On(model).
    WithEvent("created")
if userID != nil {
    logger.CausedByUser(*userID)
}
logger.Log("Firewall rule was created")
```

**Solution - Create service mixin:**
```go
// internal/pkg/activity/mixin.go

type ActivityMixin struct {
    db      *gorm.DB
    logName string
}

func (m *ActivityMixin) LogActivity(ctx context.Context, model activity.Subject,
    event, description string, userID *string) {
    logger := activity.New(m.db).
        WithContext(ctx).
        UseLog(m.logName).
        On(model).
        WithEvent(event)
    if userID != nil {
        logger.CausedByUser(*userID)
    }
    logger.Log(description)
}

// Usage in services
type FirewallRuleService struct {
    activity.ActivityMixin
    // ...
}

func (s *FirewallRuleService) Create(ctx context.Context, rule *models.FirewallRule, userID *string) error {
    // ... create logic
    s.LogActivity(ctx, rule, "created", "Firewall rule was created", userID)
}
```

**Impact:** ~200 lines saved, consistent audit trail

---

## 84. ParseAndValidate Adoption (P1)

**Problem:** `ParseAndValidate` helper exists but 50+ handlers use manual parsing.

**Files affected (manual parsing should be replaced):**
- `internal/modules/auth/handlers/team_handler.go` (lines 27-33, 66-72, 113-119)
- `internal/modules/server/handlers/server_handler.go` (lines 77-83, 118-124, 214-220)
- `internal/modules/site/handlers/site_handler.go` (lines 109-115)
- `internal/modules/database/handlers/database_handler.go` (lines 47-53)
- `internal/modules/backup/handlers/backup_handler.go` (lines 47-53, 94-100)
- `internal/modules/dns/handlers/dns_record_handler.go` (lines 50-56, 76-82)
- 40+ more handler methods

**Current (duplicated 50+ times):**
```go
if err := c.BodyParser(&req); err != nil {
    return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
}

if errs := validator.Validate(&req); errs != nil {
    return response.ValidationError(c, errs)
}
```

**Already exists at `internal/pkg/fiber/request.go` (lines 11-28):**
```go
func ParseAndValidate[T any](c *fiber.Ctx, req *T) error {
    if err := c.BodyParser(req); err != nil {
        return response.BadRequest(c, "Invalid request body")
    }
    if normalizable, ok := any(req).(dto.Normalizable); ok {
        normalizable.Normalize()
    }
    if errs := validator.Validate(req); errs != nil {
        return response.ValidationError(c, errs)
    }
    return nil
}
```

**Solution - Replace all manual parsing:**
```go
// Before (6 lines)
if err := c.BodyParser(&req); err != nil {
    return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
}
if errs := validator.Validate(&req); errs != nil {
    return response.ValidationError(c, errs)
}

// After (3 lines)
if err := fiberctx.ParseAndValidate(c, &req); err != nil {
    return err
}
```

**Impact:** ~150 lines saved, enables automatic normalization

---

## 85. MockRepository Code Generation (P2)

**Problem:** MockRepository has 900+ lines of repetitive CRUD methods.

**File affected:**
- `tests/testutil/mock_repository.go` (902 lines)

**Current pattern repeated for each entity (8 entities × ~100 lines each):**
```go
func (m *MockRepository) CreateServer(ctx context.Context, server *models.Server) error {
    if err := m.getError("CreateServer"); err != nil {
        return err
    }
    m.Servers[server.ID] = server
    return nil
}

func (m *MockRepository) FindServerByID(ctx context.Context, id string) (*models.Server, error) {
    if err := m.getError("FindServerByID"); err != nil {
        return nil, err
    }
    server, ok := m.Servers[id]
    if !ok {
        return nil, gorm.ErrRecordNotFound
    }
    return server, nil
}
// Repeated for Update, Delete, FindAll, etc.
```

**Solution - Create generic mock:**
```go
// tests/testutil/generic_mock.go

type MockStore[T any] struct {
    items  map[string]*T
    errors map[string]error
}

func (m *MockStore[T]) Create(item *T, getID func(*T) string) error {
    if err := m.errors["Create"]; err != nil {
        return err
    }
    m.items[getID(item)] = item
    return nil
}

func (m *MockStore[T]) FindByID(id string) (*T, error) {
    if err := m.errors["FindByID"]; err != nil {
        return nil, err
    }
    item, ok := m.items[id]
    if !ok {
        return nil, gorm.ErrRecordNotFound
    }
    return item, nil
}

// MockRepository uses generic stores
type MockRepository struct {
    Servers *MockStore[models.Server]
    Crons   *MockStore[models.Cron]
    // ...
}
```

**Impact:** ~600 lines saved in test code

---

## 86. Email Normalization Consolidation (P2)

**Problem:** Email normalization duplicated in 6 auth DTOs.

**Files affected:**
- `internal/modules/auth/dto/requests.go` (6 Normalize implementations)
  - Line 19-22: RegisterRequest
  - Line 32-34: LoginRequest
  - Line 49-52: UpdateProfileRequest
  - Line 67-69: ForgotPasswordRequest
  - Line 80-82: ResetPasswordRequest
  - Line 124-126: InviteTeamMemberRequest

**Current (duplicated 6 times):**
```go
func (r *RegisterRequest) Normalize() {
    r.Email = strings.ToLower(strings.TrimSpace(r.Email))
    r.Name = strings.TrimSpace(r.Name)
}
```

**Helper already exists at `internal/pkg/dto/normalizer.go` (lines 61-66):**
```go
func NormalizeEmail(s *string) {
    if s != nil {
        *s = strings.ToLower(strings.TrimSpace(*s))
    }
}
```

**Solution - Use existing helper:**
```go
func (r *RegisterRequest) Normalize() {
    pkgdto.NormalizeEmail(&r.Email)
    pkgdto.TrimSpace(&r.Name)
}
```

**Impact:** ~30 lines saved, consistent normalization

---

## Extended Summary (Items 75-86)

| Category | Items | Est. LOC Saved |
|----------|-------|----------------|
| GORM Query Scopes | Item 75 | ~120 lines |
| Sync Associations Helper | Item 76 | ~60 lines |
| WebSocket Handler Base | Item 77 | ~100 lines |
| WebSocket Message Builder | Item 78 | ~80 lines |
| Job Struct Boilerplate | Item 79 | ~400 lines |
| Cloud Provider HTTP | Item 80 | ~250 lines |
| DTO Timestamp Formatter | Item 81 | ~200 lines |
| DTO List Transformation | Item 82 | ~100 lines |
| Activity Logging Helper | Item 83 | ~200 lines |
| ParseAndValidate Adoption | Item 84 | ~150 lines |
| MockRepository Generic | Item 85 | ~600 lines |
| Email Normalization | Item 86 | ~30 lines |
| **Round 5 Total** | **12 patterns** | **~2290 lines** |

---

## Updated Final Summary

| Category | Items | Total LOC Saved |
|----------|-------|-----------------|
| Handler Bases | 5 patterns | ~400 lines |
| Repository Patterns | 5 patterns | ~1200 lines |
| Service Patterns | 3 patterns | ~450 lines |
| Job/Task Patterns | 3 patterns | ~800 lines |
| Model Mixins | 5 patterns | ~270 lines |
| Provider API Clients | 3 patterns | ~450 lines |
| Middleware Patterns | 1 pattern | ~30 lines |
| Config Patterns | 1 pattern | ~86 lines |
| DTO/Request Patterns | 2 patterns | ~550 lines |
| Testing Patterns | 2 patterns | ~250 lines |
| Infrastructure Patterns | 6 patterns | ~1075 lines |
| Validation & Response | 5 patterns | ~610 lines |
| Template & Script | 3 patterns | ~160 lines |
| Interface & Query | 4 patterns | ~500 lines |
| Security & Events | 3 patterns | ~100 lines |
| HTTP & Helpers (Round 3) | 12 patterns | ~1790 lines |
| Error & DI (Round 4) | 12 patterns | ~1320 lines |
| GORM & Jobs (Round 5) | 12 patterns | ~2290 lines |
| **Grand Total** | **86 patterns** | **~12,331 lines** |

---

## 87. Request Context Extraction Consolidation (P1)

**Problem:** 319 context extraction points with unsafe direct casts scattered across 44 handler files.

**Files affected:**
- `internal/modules/server/handlers/server_handler.go` (10+ extractions)
- `internal/modules/site/handlers/site_handler.go` (10+ extractions)
- `internal/modules/git/handlers/source_control_handler.go` (12+ extractions)
- `internal/modules/dns/handlers/domain_handler.go` (8+ extractions)
- `internal/modules/billing/handlers/billing_handler.go` (8+ extractions)
- 39 more handler files...

**Current (unsafe, duplicated 98+ times):**
```go
func (h *Handler) List(c *fiber.Ctx) error {
    teamID := c.Locals("teamID").(string)  // Will panic if not set!
    userID := c.Locals("userID").(string)  // Will panic if not set!
    // ...
}
```

**Database module already has safe helpers at `internal/modules/database/handlers/helper.go:9-23`:**
```go
func getUserIDFromContext(c *fiber.Ctx) *string {
    if userID, ok := c.Locals("userID").(string); ok && userID != "" {
        return &userID
    }
    return nil
}
```

**Solution - Create shared `internal/pkg/fiber/context.go`:**
```go
package fiber

type RequestContext struct {
    UserID   string
    TeamID   string
    TeamRole string
    Email    string
    TraceID  string
}

func FromFiber(c *fiber.Ctx) (*RequestContext, error) {
    teamID, ok := c.Locals("teamID").(string)
    if !ok || teamID == "" {
        return nil, ErrMissingTeamContext
    }
    userID, _ := c.Locals("userID").(string)
    return &RequestContext{
        UserID:   userID,
        TeamID:   teamID,
        TeamRole: c.Locals("teamRole").(string),
    }, nil
}

// Context key constants
const (
    KeyTeamID   = "teamID"
    KeyUserID   = "userID"
    KeyTeamRole = "teamRole"
)
```

**Refactored handlers:**
```go
func (h *Handler) List(c *fiber.Ctx) error {
    ctx, err := fiberctx.FromFiber(c)
    if err != nil {
        return response.BadRequest(c, err.Error())
    }
    servers, err := h.service.ListServers(c.Context(), ctx.TeamID)
    // ...
}
```

**Impact:** ~300 lines saved, eliminates runtime panic risk

---

## 88. Repository Authorization Method Consolidation (P1)

**Problem:** 60+ repository methods duplicating `FindByIDAndTeam` authorization patterns across 12+ repositories.

**Files affected:**
- `internal/modules/server/repositories/server_repository.go:46-58`
- `internal/modules/site/repositories/site_repository.go:67-78`
- `internal/modules/database/repositories/database_repository.go:53-68`
- `internal/modules/git/repositories/source_control_repository.go:62-73`
- `internal/modules/notification/repositories/notification_channel_repository.go:69-84`
- `internal/modules/dns/repositories/domain_provider_repository.go:41-51`
- 6 more repositories...

**Current (repeated 12+ times):**
```go
func (r *ServerRepository) FindByIDAndTeam(ctx context.Context, id, teamID string) (*models.Server, error) {
    var server models.Server
    err := r.DB().WithContext(ctx).
        Preload("Services").
        First(&server, "id = ? AND team_id = ?", id, teamID).Error
    if err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, ErrServerNotFound
        }
        return nil, err
    }
    return &server, nil
}
```

**Solution - Add to `internal/pkg/repository/base.go`:**
```go
func (r *Base[T]) FindByIDAndTeam(ctx context.Context, id, teamID string, preloads ...string) (*T, error) {
    var entity T
    query := r.db.WithContext(ctx)
    for _, p := range preloads {
        query = query.Preload(p)
    }
    err := query.First(&entity, "id = ? AND team_id = ?", id, teamID).Error
    if err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, r.notFoundError
        }
        return nil, err
    }
    return &entity, nil
}

func (r *Base[T]) FindByServerAndTeam(ctx context.Context, serverID, teamID string) ([]T, error) {
    var entities []T
    err := r.db.WithContext(ctx).
        Where("server_id = ? AND team_id = ?", serverID, teamID).
        Order("created_at DESC").
        Find(&entities).Error
    return entities, err
}
```

**Usage:**
```go
// Before: 12 lines per repository
server, err := r.repos.Server().FindByIDAndTeam(ctx, id, teamID)

// After: 0 lines needed, inherited from Base[T]
```

**Impact:** ~800 lines saved across 12+ repositories

---

## 89. Preload Chain Consolidation (P2)

**Problem:** 40+ identical preload chains duplicated within repositories.

**Files affected:**
- `internal/modules/backup/repositories/backup_repository.go` (3 identical chains)
- `internal/modules/database/repositories/database_repository.go` (7 identical preloads)
- `internal/modules/git/repositories/source_control_repository.go` (7 identical preloads)
- `internal/modules/dns/repositories/domain_repository.go` (3 identical chains)

**Current (duplicated 7 times in database_repository.go):**
```go
func (r *DatabaseRepository) FindByID(...) { Preload("Users")... }
func (r *DatabaseRepository) FindByIDAndServer(...) { Preload("Users")... }
func (r *DatabaseRepository) FindByIDAndTeam(...) { Preload("Users")... }
func (r *DatabaseRepository) FindByIDAndServerAndTeam(...) { Preload("Users")... }
// ... 3 more methods with same Preload
```

**Solution - Add default preloads to repository:**
```go
type DatabaseRepository struct {
    repository.Base[models.Database]
    defaultPreloads []string
}

func NewDatabaseRepository(db *gorm.DB) *DatabaseRepository {
    return &DatabaseRepository{
        Base:            repository.NewBase[models.Database](db),
        defaultPreloads: []string{"Users"},
    }
}

func (r *DatabaseRepository) withPreloads(query *gorm.DB) *gorm.DB {
    for _, p := range r.defaultPreloads {
        query = query.Preload(p)
    }
    return query
}
```

**Impact:** ~400 lines saved, DRY preload configuration

---

## 90. Service BaseService Accessor Duplication (P2)

**Problem:** All 6 module BaseService implementations have identical accessor methods.

**Files affected:**
- `internal/modules/dns/services/base.go:79-96`
- `internal/modules/backup/services/base.go:74-92`
- `internal/modules/site/services/base.go:45-62`
- `internal/modules/git/services/base.go:48-65`
- `internal/modules/notification/services/base.go:55-72`
- `internal/modules/database/services/base.go:52-69`

**Current (duplicated 6 times):**
```go
func (s *BaseService) Repos() *repositories.Registry { return s.repos }
func (s *BaseService) DB() *gorm.DB { return s.deps.DB }
func (s *BaseService) Logger() *zerolog.Logger { return s.logger }
func (s *BaseService) Services() *ServiceRegistry { return s.deps.registry }
```

**Solution - Create generic module base at `internal/pkg/service/module_base.go`:**
```go
type ModuleBase[R any, S any] struct {
    Base
    deps     *Dependencies
    repos    R
    registry S
}

func (m *ModuleBase[R, S]) Repos() R { return m.repos }
func (m *ModuleBase[R, S]) DB() *gorm.DB { return m.deps.DB }
func (m *ModuleBase[R, S]) Services() S { return m.registry }
```

**Impact:** ~120 lines saved, consistent service base

---

## 91. Retry Logic Consolidation (P2)

**Problem:** 7 distinct retry patterns with no central backoff strategy.

**Files affected:**
- `internal/modules/server/jobs/wait_for_server_to_connect.go:132-209` (exponential backoff)
- `internal/modules/site/jobs/deploy.go:42-50, 473-481` (fixed 500ms, DUPLICATED)
- `internal/pkg/taskrunner/ssh_client.go:340-361` (fixed 10s)
- `internal/modules/server/jobs/create_on_provider.go:194-231` (fixed 10s polling)
- `internal/modules/billing/repositories/webhook_event_repository.go:39-48` (DB-backed counter)

**Current (duplicated in deploy.go):**
```go
var deployment *models.Deployment
for i := 0; i < 3; i++ {
    deployment, err = j.ctx.DeploymentRepo.FindByID(ctx, j.Payload.DeploymentID)
    if err == nil { break }
    if i < 2 { time.Sleep(500 * time.Millisecond) }
}
```

**Solution - Create `internal/pkg/retry/retry.go`:**
```go
package retry

type Config struct {
    MaxAttempts int
    Initial     time.Duration
    Max         time.Duration
    Multiplier  float64
    Jitter      float64
}

var (
    DefaultFixed       = Config{MaxAttempts: 3, Initial: 500 * time.Millisecond}
    DefaultExponential = Config{MaxAttempts: 30, Initial: 10 * time.Second, Max: 30 * time.Second, Multiplier: 1.2}
)

func WithBackoff[T any](ctx context.Context, cfg Config, fn func() (T, error)) (T, error) {
    var result T
    var lastErr error
    delay := cfg.Initial

    for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
        result, lastErr = fn()
        if lastErr == nil { return result, nil }

        select {
        case <-ctx.Done():
            return result, ctx.Err()
        case <-time.After(delay):
        }

        if cfg.Multiplier > 0 {
            delay = time.Duration(float64(delay) * cfg.Multiplier)
            if cfg.Max > 0 && delay > cfg.Max { delay = cfg.Max }
        }
    }
    return result, lastErr
}
```

**Usage:**
```go
deployment, err := retry.WithBackoff(ctx, retry.DefaultFixed, func() (*models.Deployment, error) {
    return j.ctx.DeploymentRepo.FindByID(ctx, j.Payload.DeploymentID)
})
```

**Impact:** ~200 lines saved, consistent retry behavior

---

## 92. HTTP Client Factory Consolidation (P2)

**Problem:** 8+ identical HTTP client instances with same 30-second timeout.

**Files affected:**
- `internal/modules/git/providers/github.go:35-37`
- `internal/modules/server/providers/digitalocean.go:35`
- `internal/modules/server/providers/hetzner.go`
- `internal/modules/server/providers/linode.go`
- `internal/modules/server/providers/vultr.go`
- `internal/modules/notification/channels/http_client.go:20-24`
- `internal/modules/backup/storage/dropbox.go:23-24`
- `internal/modules/dns/providers/cloudflare.go:29-30`

**Current (repeated 8+ times):**
```go
httpClient := &http.Client{
    Timeout: 30 * time.Second,
}
```

**Solution - Create `internal/pkg/httpclient/client.go`:**
```go
package httpclient

var (
    DefaultTimeout = 30 * time.Second
    defaultClient  *http.Client
    once           sync.Once
)

func Default() *http.Client {
    once.Do(func() {
        defaultClient = &http.Client{
            Timeout: DefaultTimeout,
            Transport: &http.Transport{
                MaxIdleConns:        100,
                MaxIdleConnsPerHost: 10,
                IdleConnTimeout:     90 * time.Second,
            },
        }
    })
    return defaultClient
}

func WithTimeout(timeout time.Duration) *http.Client {
    return &http.Client{Timeout: timeout}
}
```

**Impact:** ~150 lines saved, connection pooling benefits

---

## 93. JobContext Field Duplication (P2)

**Problem:** All module JobContexts duplicate fields already in `jobs.Base`.

**Files affected:**
- `internal/modules/server/jobs/context.go:19-29`
- `internal/modules/site/jobs/context.go:21-38`
- `internal/modules/backup/jobs/context.go`
- `internal/modules/database/jobs/context.go`
- `internal/modules/script/jobs/context.go`

**Current (5 modules):**
```go
type JobContext struct {
    pkgjobs.Base                    // Already has DB, Logger, WS, Queue
    DB              *gorm.DB        // DUPLICATE!
    Logger          *zerolog.Logger // DUPLICATE!
    WS              broadcast.TeamBroadcaster // DUPLICATE!
    Queue           *queue.Client   // DUPLICATE!
    Repos           contracts.RepositoryRegistry
    // ... module-specific fields
}
```

**Solution - Remove duplicates, use embedded fields:**
```go
type JobContext struct {
    pkgjobs.Base
    Repos contracts.RepositoryRegistry
    // Only module-specific fields here
}

// Access via embedded Base methods:
// j.ctx.DB(), j.ctx.Logger(), j.ctx.WS(), j.ctx.Queue()
```

**Impact:** ~100 lines saved, cleaner inheritance

---

## 94. Double-Check Lock Pattern Extraction (P3)

**Problem:** Same double-check locking duplicated for cached values.

**File affected:**
- `internal/modules/billing/services/team_subscription_options.go:227-268`

**Current (duplicated twice in same file):**
```go
func (t *TeamSubscriptionOptions) PlanOptions(ctx context.Context) models.PlanOptions {
    t.mu.RLock()
    if t.cachedOptions != nil {
        defer t.mu.RUnlock()
        return *t.cachedOptions
    }
    t.mu.RUnlock()

    t.mu.Lock()
    defer t.mu.Unlock()

    if t.cachedOptions != nil { return *t.cachedOptions }

    options := t.resolvePlanOptions(ctx)
    t.cachedOptions = &options
    return options
}
```

**Solution - Create `internal/pkg/sync/cached.go`:**
```go
package sync

type Cached[T any] struct {
    mu    sync.RWMutex
    value *T
}

func (c *Cached[T]) GetOrCompute(compute func() T) T {
    c.mu.RLock()
    if c.value != nil {
        defer c.mu.RUnlock()
        return *c.value
    }
    c.mu.RUnlock()

    c.mu.Lock()
    defer c.mu.Unlock()

    if c.value != nil { return *c.value }

    result := compute()
    c.value = &result
    return result
}
```

**Usage:**
```go
type TeamSubscriptionOptions struct {
    cachedOptions sync.Cached[models.PlanOptions]
    cachedIsAdmin sync.Cached[bool]
}

func (t *TeamSubscriptionOptions) PlanOptions(ctx context.Context) models.PlanOptions {
    return t.cachedOptions.GetOrCompute(func() models.PlanOptions {
        return t.resolvePlanOptions(ctx)
    })
}
```

**Impact:** ~50 lines saved, reusable pattern

---

## 95. NotifierFake Lock Wrapper (P3)

**Problem:** 16+ identical lock/defer patterns in test fake.

**File affected:**
- `internal/modules/notification/testing/notifier_fake.go` (lines 49-243)

**Current (repeated 16+ times):**
```go
func (n *NotifierFake) SendToTeam(...) error {
    n.mu.Lock()
    defer n.mu.Unlock()
    // method body
}

func (n *NotifierFake) SendToChannel(...) error {
    n.mu.Lock()
    defer n.mu.Unlock()
    // method body
}
// ... 14 more identical patterns
```

**Solution - Create wrapper method:**
```go
func (n *NotifierFake) withLock(fn func()) {
    n.mu.Lock()
    defer n.mu.Unlock()
    fn()
}

func (n *NotifierFake) withLockReturn[T any](fn func() T) T {
    n.mu.Lock()
    defer n.mu.Unlock()
    return fn()
}

// Usage:
func (n *NotifierFake) SendToTeam(...) error {
    n.withLock(func() {
        n.sendToTeamCalled++
        n.sentNotifications = append(n.sentNotifications, ...)
    })
    return nil
}
```

**Impact:** ~100 lines saved in test code

---

## 96. ServiceRegistry Boilerplate (P3)

**Problem:** Every module repeats identical registry struct + getter pattern.

**Files affected:**
- `internal/modules/site/services/base.go:24-45`
- `internal/modules/dns/services/base.go:34-52`
- `internal/modules/backup/services/base.go:24-42`
- `internal/modules/git/services/base.go:24-42`
- `internal/modules/notification/services/base.go:24-42`

**Current (repeated 5+ times):**
```go
type ServiceRegistry struct {
    site       *SiteService
    deployment *DeploymentService
    // ...
}

func (r *ServiceRegistry) Site() *SiteService { return r.site }
func (r *ServiceRegistry) Deployment() *DeploymentService { return r.deployment }
// ... more getters
```

**Solution - Use generic container or code generation:**
```go
// Option 1: Generic container
type Registry[T any] struct {
    services map[string]T
}

// Option 2: Documented pattern with interface
type ServiceRegistry interface {
    Get(name string) any
}
```

**Impact:** ~90 lines saved across 5 modules

---

## 97. Soft Delete Pattern Standardization (P3)

**Problem:** Inconsistent soft-delete approaches (GORM DeletedAt vs ArchivedAt vs status-based).

**Files affected:**
- `internal/modules/server/models/server.go:56` (ArchivedAt field)
- `internal/modules/auth/models/personal_access_token.go:12` (SoftDeleteModel)
- `internal/modules/billing/repositories/subscription_repository.go:61-77` (status-based)

**Current inconsistency:**
```go
// Pattern 1: Custom ArchivedAt (Server)
type Server struct {
    ArchivedAt *time.Time `gorm:"column:archived_at"`
}

// Pattern 2: GORM SoftDeleteModel (PersonalAccessToken)
type PersonalAccessToken struct {
    basemodels.SoftDeleteModel  // DeletedAt gorm.DeletedAt
}

// Pattern 3: Status-based (Subscription)
Where("status IN ?", []enums.SubscriptionStatus{Active, OnTrial})
```

**Solution - Create unified archivable interface:**
```go
// internal/pkg/models/archivable.go
type Archivable interface {
    IsArchived() bool
    Archive()
    Unarchive()
}

type ArchivableModel struct {
    ArchivedAt *time.Time `gorm:"column:archived_at;type:timestamp null"`
}

func (m *ArchivableModel) IsArchived() bool { return m.ArchivedAt != nil }
func (m *ArchivableModel) Archive() { now := time.Now(); m.ArchivedAt = &now }
func (m *ArchivableModel) Unarchive() { m.ArchivedAt = nil }
```

**Impact:** ~150 lines saved, consistent soft-delete semantics

---

## 98. Timeout Configuration Consolidation (P3)

**Problem:** Timeout values scattered across 85+ files with no central configuration.

**Files affected:**
- `internal/pkg/taskrunner/task.go:59` (10 min default)
- `internal/pkg/taskrunner/dispatcher.go:153` (30s SSH)
- `internal/pkg/taskrunner/ssh_client.go:78` (30s default)
- `cmd/api/main.go:110-111` (30s Fiber)
- All provider files (30s HTTP)
- Task definition files (600s-3600s)

**Current (scattered):**
```go
// dispatcher.go:153
sshClient, _ := NewSSHClient(SSHConfig{Timeout: 30 * time.Second})

// ssh_client.go:78
if timeout == 0 { timeout = 30 * time.Second }

// main.go:110
fiberApp := fiber.New(fiber.Config{ReadTimeout: 30 * time.Second})
```

**Solution - Create `internal/pkg/timeout/config.go`:**
```go
package timeout

var (
    SSH          = 30 * time.Second
    HTTP         = 30 * time.Second
    Fiber        = 30 * time.Second
    TaskDefault  = 10 * time.Minute
    NetworkDial  = 5 * time.Second
    RetryDelay   = 10 * time.Second
)

type Config struct {
    SSH         time.Duration
    HTTP        time.Duration
    TaskDefault time.Duration
}

func Default() *Config {
    return &Config{
        SSH:         SSH,
        HTTP:        HTTP,
        TaskDefault: TaskDefault,
    }
}
```

**Impact:** ~100 lines saved, centralized timeout management

---

## Extended Summary (Items 87-98)

| Category | Items | Est. LOC Saved |
|----------|-------|----------------|
| Request Context Extraction | Item 87 | ~300 lines |
| Repository Authorization | Item 88 | ~800 lines |
| Preload Chain Consolidation | Item 89 | ~400 lines |
| Service Accessor Duplication | Item 90 | ~120 lines |
| Retry Logic Consolidation | Item 91 | ~200 lines |
| HTTP Client Factory | Item 92 | ~150 lines |
| JobContext Field Duplication | Item 93 | ~100 lines |
| Double-Check Lock Pattern | Item 94 | ~50 lines |
| NotifierFake Lock Wrapper | Item 95 | ~100 lines |
| ServiceRegistry Boilerplate | Item 96 | ~90 lines |
| Soft Delete Standardization | Item 97 | ~150 lines |
| Timeout Configuration | Item 98 | ~100 lines |
| **Round 6 Total** | **12 patterns** | **~2560 lines** |

---

## Updated Final Summary

| Category | Items | Total LOC Saved |
|----------|-------|-----------------|
| Handler Bases | 5 patterns | ~400 lines |
| Repository Patterns | 5 patterns | ~1200 lines |
| Service Patterns | 3 patterns | ~450 lines |
| Job/Task Patterns | 3 patterns | ~800 lines |
| Model Mixins | 5 patterns | ~270 lines |
| Provider API Clients | 3 patterns | ~450 lines |
| Middleware Patterns | 1 pattern | ~30 lines |
| Config Patterns | 1 pattern | ~86 lines |
| DTO/Request Patterns | 2 patterns | ~550 lines |
| Testing Patterns | 2 patterns | ~250 lines |
| Infrastructure Patterns | 6 patterns | ~1075 lines |
| Validation & Response | 5 patterns | ~610 lines |
| Template & Script | 3 patterns | ~160 lines |
| Interface & Query | 4 patterns | ~500 lines |
| Security & Events | 3 patterns | ~100 lines |
| HTTP & Helpers (Round 3) | 12 patterns | ~1790 lines |
| Error & DI (Round 4) | 12 patterns | ~1320 lines |
| GORM & Jobs (Round 5) | 12 patterns | ~2290 lines |
| Context & Auth (Round 6) | 12 patterns | ~2560 lines |
| **Grand Total** | **98 patterns** | **~14,891 lines** |

---

## 99. Duplicate AppError Type Consolidation (P1)

**Problem:** Two identical `AppError` types defined in separate packages.

**Files affected:**
- `internal/pkg/errors/errors.go:22-45` - `type AppError struct { Err, Message, Code }`
- `internal/pkg/response/errors.go:11-33` - `type AppError struct { Err, Status, Message }`

**Current (duplicated):**
```go
// internal/pkg/errors/errors.go
type AppError struct {
    Err     error
    Message string
    Code    int
}

// internal/pkg/response/errors.go
type AppError struct {
    Err     error
    Status  int
    Message string
}
```

**Solution - Consolidate to single type:**
```go
// internal/pkg/errors/app_error.go
type AppError struct {
    Err     error  `json:"-"`
    Message string `json:"message"`
    Code    int    `json:"code"`
}

func (e *AppError) Error() string { return e.Message }
func (e *AppError) Unwrap() error { return e.Err }
func (e *AppError) HTTPStatus() int { return e.Code }
```

**Impact:** ~50 lines saved, single source of truth for application errors

---

## 100. Validation Field Embedding (P1)

**Problem:** 74 Request DTOs repeat identical validation tags for email, name, password fields.

**Files affected:**
- `internal/modules/auth/dto/requests.go` (7 email fields, 5 name fields, 5 password fields)
- `internal/modules/server/dto/requests.go` (name, email fields)
- `internal/modules/site/dto/requests.go` (email field)
- `internal/modules/notification/dto/requests.go` (email fields)
- `internal/modules/database/dto/requests.go` (password fields)

**Current (repeated 7+ times):**
```go
Email string `json:"email" validate:"required,email"`
Name  string `json:"name" validate:"required,min=2,max=255"`
Password string `json:"password" validate:"required,min=8"`
PasswordConfirmation string `json:"password_confirmation" validate:"required,eqfield=Password"`
```

**Solution - Create embeddable field structs:**
```go
// internal/pkg/dto/fields.go
type EmailField struct {
    Email string `json:"email" validate:"required,email"`
}

type NameField struct {
    Name string `json:"name" validate:"required,min=2,max=255"`
}

type PasswordWithConfirmation struct {
    Password             string `json:"password" validate:"required,min=8"`
    PasswordConfirmation string `json:"password_confirmation" validate:"required,eqfield=Password"`
}

// Usage
type RegisterRequest struct {
    EmailField
    NameField
    PasswordWithConfirmation
}
```

**Impact:** ~200 lines saved, consistent validation across all DTOs

---

## 101. Handler Validation Flow Consolidation (P1)

**Problem:** 40+ handlers repeat identical body parsing and validation pattern.

**Files affected:**
- `internal/modules/auth/handlers/auth_handler.go` (3 handlers)
- `internal/modules/auth/handlers/team_handler.go` (2 handlers)
- `internal/modules/server/handlers/server_handler.go` (2 handlers)
- `internal/modules/site/handlers/site_handler.go` (2 handlers)
- 30+ more handlers...

**Current (repeated 40+ times):**
```go
func (h *Handler) Create(c *fiber.Ctx) error {
    var req dto.CreateRequest
    if err := c.BodyParser(&req); err != nil {
        return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
    }
    if errs := validator.Validate(&req); errs != nil {
        return response.ValidationError(c, errs)
    }
    // ...
}
```

**Solution - Use existing ParseAndValidate helper:**
```go
// Already exists at internal/pkg/fiber/request.go:14-27
func ParseAndValidate[T any](c *fiber.Ctx, req *T) error

// Adoption across all handlers:
func (h *Handler) Create(c *fiber.Ctx) error {
    var req dto.CreateRequest
    if err := fiberctx.ParseAndValidate(c, &req); err != nil {
        return err
    }
    // ...
}
```

**Impact:** ~200 lines saved, consistent error handling

---

## 102. Broadcaster Interface Deduplication (P1)

**Problem:** Identical broadcaster interfaces defined in two packages.

**Files affected:**
- `internal/websocket/broadcaster.go:5-35` - Broadcaster + ModelBroadcaster interfaces
- `internal/pkg/broadcast/broadcaster.go` - Same interfaces duplicated

**Current (duplicated):**
```go
// Both files define identical:
type Broadcaster interface {
    Broadcast(channel, event string, data interface{})
    BroadcastToServer(serverID, event string, data interface{})
    BroadcastToSite(siteID, event string, data interface{})
    BroadcastToDeployment(deploymentID, event string, data interface{})
    BroadcastToTeam(teamID, event string, data interface{})
}

type ModelBroadcaster interface {
    Broadcaster
    BroadcastModelCreated(teamID, modelName, modelID string, payload interface{})
    BroadcastModelUpdated(teamID, modelName, modelID string, payload interface{})
    BroadcastModelDeleted(teamID, modelName, modelID string, payload interface{})
}
```

**Solution - Keep single definition:**
```go
// Keep only internal/pkg/broadcast/broadcaster.go
// Remove internal/websocket/broadcaster.go
// Update all imports
```

**Impact:** ~35 lines saved, single source of truth

---

## 103. Model Broadcast Payload Deduplication (P2)

**Problem:** Identical BroadcastModelCreated/Updated/Deleted implementations in Hub and RedisBroadcaster.

**Files affected:**
- `internal/websocket/hub.go:179-210`
- `internal/websocket/redis_broadcaster.go:81-112`

**Current (duplicated in both files):**
```go
func (h *Hub) BroadcastModelCreated(teamID, modelName, modelID string, payload interface{}) {
    h.BroadcastToTeam(teamID, modelName+".created", map[string]interface{}{
        "id":      modelID,
        "model":   modelName,
        "action":  "created",
        "team_id": teamID,
        "data":    payload,
    })
}
// Same implementation in redis_broadcaster.go
```

**Solution - Extract to shared helper:**
```go
// internal/pkg/broadcast/model_event.go
func BuildModelEventPayload(modelName, modelID, action, teamID string, data interface{}) map[string]interface{} {
    return map[string]interface{}{
        "id":      modelID,
        "model":   modelName,
        "action":  action,
        "team_id": teamID,
        "data":    data,
    }
}

// Both Hub and Redis use:
func (h *Hub) BroadcastModelCreated(teamID, modelName, modelID string, payload interface{}) {
    h.BroadcastToTeam(teamID, modelName+".created",
        broadcast.BuildModelEventPayload(modelName, modelID, "created", teamID, payload))
}
```

**Impact:** ~60 lines saved, consistent event payloads

---

## 104. Logger Field Iteration Helper (P2)

**Problem:** Same field iteration pattern duplicated across 4 files for variadic logging.

**Files affected:**
- `internal/pkg/jobs/context.go:155-161`
- `internal/pkg/service/base.go:141-147`
- `internal/pkg/handler/base.go:61-68`
- `internal/modules/dns/services/base.go:104-109`

**Current (duplicated 4 times):**
```go
func logWithFields(event *zerolog.Event, msg string, fields ...any) {
    for i := 0; i < len(fields)-1; i += 2 {
        if key, ok := fields[i].(string); ok {
            event = event.Interface(key, fields[i+1])
        }
    }
    event.Msg(msg)
}
```

**Solution - Create shared utility:**
```go
// internal/pkg/logger/fields.go
func ApplyFields(event *zerolog.Event, fields ...any) *zerolog.Event {
    for i := 0; i < len(fields)-1; i += 2 {
        if key, ok := fields[i].(string); ok {
            event = event.Interface(key, fields[i+1])
        }
    }
    return event
}

// Usage in all Base structs:
func (s *Base) LogInfo(msg string, fields ...any) {
    logger.ApplyFields(s.Logger.Info(), fields...).Msg(msg)
}
```

**Impact:** ~80 lines saved, single implementation

---

## 105. BeforeCreate Hook Status Initialization (P2)

**Problem:** 8 models implement identical BeforeCreate pattern for status/default initialization.

**Files affected:**
- `internal/modules/server/models/server.go:74-88`
- `internal/modules/server/models/task.go:29-39`
- `internal/modules/server/models/firewall_rule.go:29-39`
- `internal/modules/server/models/installed_service.go:29-39`
- `internal/modules/server/models/daemon.go:31-49`
- `internal/modules/server/models/cron.go:27-37`
- `internal/modules/backup/models/backup_job.go:26-36`
- `internal/modules/database/models/database_user.go:28-38`

**Current (repeated 8 times):**
```go
func (s *Server) BeforeCreate(tx *gorm.DB) error {
    if err := s.BaseModel.BeforeCreate(tx); err != nil {
        return err
    }
    if s.Status == "" {
        s.Status = enums.ServerStatusNew
    }
    return nil
}
```

**Solution - Create status mixin:**
```go
// internal/pkg/models/status_model.go
type StatusModel[T ~string] struct {
    Status T
}

func (m *StatusModel[T]) InitializeStatus(defaultStatus T) {
    if m.Status == "" {
        m.Status = defaultStatus
    }
}

// Usage in models:
func (s *Server) BeforeCreate(tx *gorm.DB) error {
    if err := s.BaseModel.BeforeCreate(tx); err != nil {
        return err
    }
    s.InitializeStatus(enums.ServerStatusNew)
    return nil
}
```

**Impact:** ~100 lines saved, consistent initialization

---

## 106. ✅ COMPLETED - Enum Boilerplate Consolidation

**Problem:** 40+ enum types repeat identical IsValid/Label/Scan/Value/All/Parse method patterns.

**Files affected:**
- `internal/modules/server/enums/` (13 enum files)
- `internal/modules/site/enums/enums.go` (11 enum types)
- `internal/modules/git/enums/enums.go` (4 enum types)
- `internal/modules/billing/enums/` (5 enum files)
- Other modules with 10+ enum types

**Current (repeated 40+ times per method):**
```go
func (t ServerStatus) IsValid() bool {
    switch t {
    case StatusNew, StatusStarting, StatusProvisioning, ...:
        return true
    }
    return false
}

func (t ServerStatus) Label() string {
    labels := map[ServerStatus]string{
        StatusNew: "New",
        StatusStarting: "Starting",
        // ...
    }
    if label, ok := labels[t]; ok {
        return label
    }
    return "Unknown"
}

func (t *ServerStatus) Scan(value interface{}) error { ... }
func (t ServerStatus) Value() (driver.Value, error) { ... }
func AllServerStatuses() []ServerStatus { ... }
func ParseServerStatus(s string) (ServerStatus, error) { ... }
```

**Solution - Generic enum helpers:**
```go
// internal/pkg/enums/helpers.go
func GenericScan[T ~string](t *T, value interface{}) error {
    switch v := value.(type) {
    case []byte:
        *t = T(v)
    case string:
        *t = T(v)
    default:
        return fmt.Errorf("cannot scan type %T", value)
    }
    return nil
}

func GenericValue[T ~string](t T) (driver.Value, error) {
    return string(t), nil
}

// Usage in enums:
func (t *ServerStatus) Scan(v interface{}) error { return enums.GenericScan(t, v) }
func (t ServerStatus) Value() (driver.Value, error) { return enums.GenericValue(t) }
```

**Impact:** ~400 lines saved across all enum files

---

## 107. Repository Registry Factory (P2)

**Problem:** Every module has identical Registry factory pattern with manual repository creation.

**Files affected:**
- `internal/modules/server/repositories/registry.go:26-41`
- `internal/modules/site/repositories/registry.go:17-27`
- `internal/modules/backup/repositories/registry.go:14-21`
- `internal/modules/script/repositories/registry.go:12-17`
- `internal/modules/dns/repositories/registry.go`

**Current (repeated 5+ times):**
```go
type Registry struct {
    db           *gorm.DB
    server       *ServerRepository
    service      *ServiceRepository
    firewallRule *FirewallRuleRepository
    // ... 10 more repos
}

func NewRegistry(db *gorm.DB) *Registry {
    return &Registry{
        db:           db,
        server:       NewServerRepository(db),
        service:      NewServiceRepository(db),
        firewallRule: NewFirewallRuleRepository(db),
        // ... 10 more manual creations
    }
}

func (r *Registry) Server() *ServerRepository { return r.server }
func (r *Registry) Service() *ServiceRepository { return r.service }
// ... 10 more getters
```

**Solution - Generic registry builder:**
```go
// internal/pkg/repository/registry.go
type RegistryBuilder[T any] struct {
    db    *gorm.DB
    repos map[string]any
}

func NewRegistryBuilder[T any](db *gorm.DB) *RegistryBuilder[T] {
    return &RegistryBuilder[T]{db: db, repos: make(map[string]any)}
}

func Register[R any](b *RegistryBuilder[any], name string, factory func(*gorm.DB) R) {
    b.repos[name] = factory(b.db)
}
```

**Impact:** ~200 lines saved across 5+ modules

---

## 108. ✅ COMPLETED - Slice Transformation Generic Helper

**Problem:** 40+ handlers repeat identical Model→DTO slice transformation loop.

**Files affected:**
- `internal/modules/server/handlers/server_handler.go:46-50`
- `internal/modules/site/handlers/site_handler.go:45-48`
- `internal/modules/site/handlers/deployment_handler.go:60-63`
- `internal/modules/dns/handlers/domain_handler.go:99-105`
- 36+ more handlers with list operations

**Current (repeated 40+ times):**
```go
result := make([]dto.ServerResponse, len(servers))
for i := range servers {
    result[i] = dto.ToServerResponse(&servers[i])
}
```

**Solution - Generic Map function:**
```go
// internal/pkg/collection/transform.go
func Map[T, U any](slice []T, transform func(*T) U) []U {
    result := make([]U, len(slice))
    for i := range slice {
        result[i] = transform(&slice[i])
    }
    return result
}

// Usage in handlers:
result := collection.Map(servers, dto.ToServerResponse)
```

**Impact:** ~250 lines saved, cleaner handler code

---

## 109. ✅ COMPLETED - Safe Map Access Helper

**Problem:** 50+ map type assertion patterns repeated in services.

**Files affected:**
- `internal/modules/site/services/deployment_service.go:481-617`
- `internal/modules/git/handlers/webhook_handler.go`
- `internal/modules/billing/handlers/webhook_handler.go`

**Current (repeated 50+ times):**
```go
result := make(map[string]interface{})
if id, ok := commit["id"].(string); ok {
    result["commit_id"] = id
    if len(id) >= 7 {
        result["sha"] = id[:7]
    }
}
if author, ok := commit["author"].(map[string]any); ok {
    if name, ok := author["name"].(string); ok {
        result["name"] = name
    }
}
```

**Solution - Safe accessor helpers:**
```go
// internal/pkg/maputil/safe.go
func GetString(m map[string]any, key string) string {
    if v, ok := m[key].(string); ok {
        return v
    }
    return ""
}

func GetStringPtr(m map[string]any, key string) *string {
    if v, ok := m[key].(string); ok {
        return &v
    }
    return nil
}

func GetNestedString(m map[string]any, keys ...string) string {
    current := m
    for _, key := range keys[:len(keys)-1] {
        if nested, ok := current[key].(map[string]any); ok {
            current = nested
        } else {
            return ""
        }
    }
    return GetString(current, keys[len(keys)-1])
}
```

**Impact:** ~150 lines saved, safer map access

---

## 110. Collection Filter-Map Pattern (P3)

**Problem:** Repeated append-in-loop with filtering, causing inefficient allocations.

**Files affected:**
- `internal/modules/server/handlers/log_handler.go:34-55`
- `internal/modules/auth/services/team_member_service.go:256-284`
- `internal/modules/auth/dto/responses.go:252-271`

**Current (inefficient):**
```go
var logs []LogInfo
for _, service := range server.Services {
    software := enums.Software(service.Software)
    if software.HasLogPath() {
        logs = append(logs, LogInfo{...})  // Inefficient append
    }
}
if logs == nil {
    logs = []LogInfo{}
}
```

**Solution - Generic FilterMap:**
```go
// internal/pkg/collection/filter.go
func FilterMap[T, U any](slice []T, predicate func(*T) bool, transform func(*T) U) []U {
    result := make([]U, 0, len(slice)/2)  // Estimate half will match
    for i := range slice {
        if predicate(&slice[i]) {
            result = append(result, transform(&slice[i]))
        }
    }
    return result
}

// Usage:
logs := collection.FilterMap(server.Services,
    func(s *models.Service) bool { return enums.Software(s.Software).HasLogPath() },
    func(s *models.Service) LogInfo { return buildLogInfo(s) },
)
```

**Impact:** ~80 lines saved, better performance

---

## Extended Summary (Items 99-110)

| Category | Items | Est. LOC Saved |
|----------|-------|----------------|
| Duplicate AppError Types | Item 99 | ~50 lines |
| Validation Field Embedding | Item 100 | ~200 lines |
| Handler Validation Flow | Item 101 | ~200 lines |
| Broadcaster Interface Dedup | Item 102 | ~35 lines |
| Model Broadcast Payload | Item 103 | ~60 lines |
| Logger Field Iteration | Item 104 | ~80 lines |
| BeforeCreate Hook Status | Item 105 | ~100 lines |
| Enum Boilerplate | Item 106 | ~400 lines |
| Repository Registry Factory | Item 107 | ~200 lines |
| Slice Transformation Helper | Item 108 | ~250 lines |
| Safe Map Access | Item 109 | ~150 lines |
| Collection Filter-Map | Item 110 | ~80 lines |
| **Round 7 Total** | **12 patterns** | **~1805 lines** |

---

## 111. Route Middleware Chain Consolidation (P1)

**Problem:** 17+ repeated middleware chain declarations across route registrations.

**Files affected:**
- `internal/modules/server/routes.go:20-45`
- `internal/modules/site/routes.go:18-42`
- `internal/modules/auth/routes.go:15-38`
- `internal/modules/git/routes.go:12-35`
- `internal/modules/database/routes.go:10-28`
- `internal/modules/backup/routes.go:8-25`
- `internal/modules/dns/routes.go:10-30`

**Current (repeated 17+ times):**
```go
func RegisterRoutes(app *fiber.App, h *handlers.ServerHandler, auth *middleware.AuthMiddleware) {
    api := app.Group("/api")

    servers := api.Group("/servers",
        auth.RequireAuth(),
        auth.RequireTeam(),
    )

    servers.Get("/", h.List)
    servers.Post("/", h.Create)
    servers.Get("/:serverId", auth.RequireServerAccess(), h.Get)
    servers.Put("/:serverId", auth.RequireServerAccess(), h.Update)
    servers.Delete("/:serverId", auth.RequireServerAccess(), h.Delete)
}
```

**Solution - Fluent Route Builder:**
```go
// internal/pkg/router/builder.go
package router

type RouteBuilder struct {
    app        *fiber.App
    auth       AuthMiddleware
    basePath   string
    middleware []fiber.Handler
}

type AuthMiddleware interface {
    RequireAuth() fiber.Handler
    RequireTeam() fiber.Handler
    RequireServerAccess() fiber.Handler
    RequireSiteAccess() fiber.Handler
}

func New(app *fiber.App, auth AuthMiddleware) *RouteBuilder {
    return &RouteBuilder{app: app, auth: auth, basePath: "/api"}
}

func (b *RouteBuilder) Resource(name string) *ResourceBuilder {
    return &ResourceBuilder{
        builder: b,
        name:    name,
        group:   b.app.Group(b.basePath + "/" + name, b.auth.RequireAuth(), b.auth.RequireTeam()),
    }
}

type ResourceBuilder struct {
    builder     *RouteBuilder
    name        string
    group       fiber.Router
    paramName   string
    accessCheck fiber.Handler
}

func (r *ResourceBuilder) WithParam(param string, accessCheck fiber.Handler) *ResourceBuilder {
    r.paramName = param
    r.accessCheck = accessCheck
    return r
}

func (r *ResourceBuilder) CRUD(h CRUDHandler) *ResourceBuilder {
    r.group.Get("/", h.List)
    r.group.Post("/", h.Create)

    if r.accessCheck != nil {
        r.group.Get("/:"+r.paramName, r.accessCheck, h.Get)
        r.group.Put("/:"+r.paramName, r.accessCheck, h.Update)
        r.group.Delete("/:"+r.paramName, r.accessCheck, h.Delete)
    }
    return r
}

// CRUDHandler interface for standard handlers
type CRUDHandler interface {
    List(c *fiber.Ctx) error
    Create(c *fiber.Ctx) error
    Get(c *fiber.Ctx) error
    Update(c *fiber.Ctx) error
    Delete(c *fiber.Ctx) error
}
```

**Refactored Usage:**
```go
func RegisterRoutes(app *fiber.App, h *handlers.ServerHandler, auth *middleware.AuthMiddleware) {
    router.New(app, auth).
        Resource("servers").
        WithParam("serverId", auth.RequireServerAccess()).
        CRUD(h)
}
```

**Impact:** ~340 lines saved across all route files

---

## 112. Handler Module Aggregation (P2)

**Problem:** Each module repeats handler creation with identical dependency injection patterns.

**Files affected:**
- `internal/modules/server/module.go:45-78`
- `internal/modules/site/module.go:42-75`
- `internal/modules/auth/module.go:38-65`
- `internal/modules/database/module.go:35-58`
- `internal/modules/backup/module.go:32-52`

**Current (repeated 8+ times):**
```go
type Module struct {
    db     *gorm.DB
    redis  *redis.Client
    config *config.Config
    logger zerolog.Logger
    queue  *asynq.Client
}

func NewModule(db *gorm.DB, redis *redis.Client, cfg *config.Config, logger zerolog.Logger, queue *asynq.Client) *Module {
    return &Module{
        db:     db,
        redis:  redis,
        config: cfg,
        logger: logger,
        queue:  queue,
    }
}

func (m *Module) Handlers() *Handlers {
    repo := repositories.NewServerRepository(m.db)
    service := services.NewServerService(repo, m.queue, m.logger)
    return &Handlers{
        Server: handlers.NewServerHandler(service, m.logger),
    }
}
```

**Solution - Base Module with Factory:**
```go
// internal/pkg/module/base.go
package module

type Dependencies struct {
    DB     *gorm.DB
    Redis  *redis.Client
    Config *config.Config
    Logger zerolog.Logger
    Queue  *asynq.Client
}

type BaseModule struct {
    deps Dependencies
}

func NewBaseModule(deps Dependencies) BaseModule {
    return BaseModule{deps: deps}
}

func (m *BaseModule) DB() *gorm.DB           { return m.deps.DB }
func (m *BaseModule) Redis() *redis.Client   { return m.deps.Redis }
func (m *BaseModule) Config() *config.Config { return m.deps.Config }
func (m *BaseModule) Logger() zerolog.Logger { return m.deps.Logger }
func (m *BaseModule) Queue() *asynq.Client   { return m.deps.Queue }

func (m *BaseModule) ChildLogger(name string) zerolog.Logger {
    return m.deps.Logger.With().Str("module", name).Logger()
}
```

**Refactored Module:**
```go
type Module struct {
    module.BaseModule
}

func NewModule(deps module.Dependencies) *Module {
    return &Module{BaseModule: module.NewBaseModule(deps)}
}

func (m *Module) Handlers() *Handlers {
    repo := repositories.NewServerRepository(m.DB())
    service := services.NewServerService(repo, m.Queue(), m.ChildLogger("server"))
    return &Handlers{Server: handlers.NewServerHandler(service, m.ChildLogger("handler"))}
}
```

**Impact:** ~200 lines saved, consistent module initialization

---

## 113. Webhook Route Standardization (P2)

**Problem:** Webhook routes have inconsistent patterns for signature verification and payload parsing.

**Files affected:**
- `internal/modules/git/routes.go:45-68` - GitHub/GitLab/Bitbucket webhooks
- `internal/modules/billing/routes.go:35-52` - Stripe/Paddle webhooks
- `internal/modules/server/routes.go:78-92` - Provider callback webhooks

**Current (repeated 5+ times):**
```go
webhooks := api.Group("/webhooks")
webhooks.Post("/github", h.GitHubWebhook)
webhooks.Post("/gitlab", h.GitLabWebhook)
webhooks.Post("/bitbucket", h.BitbucketWebhook)

// Each handler repeats:
func (h *WebhookHandler) GitHubWebhook(c *fiber.Ctx) error {
    signature := c.Get("X-Hub-Signature-256")
    if !verifyGitHubSignature(c.Body(), signature, h.secret) {
        return fiber.ErrUnauthorized
    }

    eventType := c.Get("X-GitHub-Event")
    // parse and dispatch...
}
```

**Solution - Webhook Middleware Factory:**
```go
// internal/pkg/webhook/middleware.go
package webhook

type Provider string
const (
    GitHub    Provider = "github"
    GitLab    Provider = "gitlab"
    Bitbucket Provider = "bitbucket"
    Stripe    Provider = "stripe"
)

type Config struct {
    Provider       Provider
    Secret         string
    SignatureHeader string
    EventHeader    string
    Verifier       func(body []byte, signature, secret string) bool
}

var providerConfigs = map[Provider]Config{
    GitHub: {
        SignatureHeader: "X-Hub-Signature-256",
        EventHeader:     "X-GitHub-Event",
        Verifier:        verifyHMACSHA256,
    },
    GitLab: {
        SignatureHeader: "X-Gitlab-Token",
        EventHeader:     "X-Gitlab-Event",
        Verifier:        verifyTokenMatch,
    },
    // ... other providers
}

func Verify(provider Provider, secret string) fiber.Handler {
    cfg := providerConfigs[provider]
    return func(c *fiber.Ctx) error {
        sig := c.Get(cfg.SignatureHeader)
        if !cfg.Verifier(c.Body(), sig, secret) {
            return fiber.ErrUnauthorized
        }
        c.Locals("webhook_event", c.Get(cfg.EventHeader))
        return c.Next()
    }
}
```

**Refactored Routes:**
```go
webhooks := api.Group("/webhooks")
webhooks.Post("/github", webhook.Verify(webhook.GitHub, h.githubSecret), h.HandleGitWebhook)
webhooks.Post("/gitlab", webhook.Verify(webhook.GitLab, h.gitlabSecret), h.HandleGitWebhook)
```

**Impact:** ~180 lines saved, consistent signature verification

---

## 114. Template Engine Consolidation (P1)

**Problem:** 3 duplicate template engine implementations across modules for shell script generation.

**Files affected:**
- `internal/modules/site/tasks/templates/engine.go:1-180`
- `internal/modules/server/tasks/templates/engine.go:1-165`
- `internal/modules/database/tasks/templates/engine.go:1-120`

**Current (duplicated 3 times):**
```go
type TemplateEngine struct {
    templates map[string]*template.Template
    funcMap   template.FuncMap
}

func NewTemplateEngine() *TemplateEngine {
    return &TemplateEngine{
        templates: make(map[string]*template.Template),
        funcMap: template.FuncMap{
            "quote":   shellQuote,
            "join":    strings.Join,
            "indent":  indentLines,
            "default": defaultValue,
        },
    }
}

func (e *TemplateEngine) Load(name, content string) error {
    tmpl, err := template.New(name).Funcs(e.funcMap).Parse(content)
    if err != nil {
        return err
    }
    e.templates[name] = tmpl
    return nil
}

func (e *TemplateEngine) Execute(name string, data any) (string, error) {
    tmpl, ok := e.templates[name]
    if !ok {
        return "", fmt.Errorf("template %s not found", name)
    }
    var buf bytes.Buffer
    if err := tmpl.Execute(&buf, data); err != nil {
        return "", err
    }
    return buf.String(), nil
}

// Shell-specific helpers (duplicated)
func shellQuote(s string) string {
    return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func indentLines(indent int, s string) string {
    prefix := strings.Repeat(" ", indent)
    lines := strings.Split(s, "\n")
    for i := range lines {
        lines[i] = prefix + lines[i]
    }
    return strings.Join(lines, "\n")
}
```

**Solution - Unified Script Template Engine:**
```go
// internal/pkg/script/template.go
package script

import (
    "bytes"
    "embed"
    "strings"
    "text/template"
)

type TemplateEngine struct {
    templates *template.Template
}

// DefaultFuncs provides shell-aware template functions
var DefaultFuncs = template.FuncMap{
    "quote":     ShellQuote,
    "join":      strings.Join,
    "indent":    IndentLines,
    "default":   DefaultValue,
    "env":       EnvVar,
    "condition": Conditional,
    "loop":      LoopRange,
}

func NewTemplateEngine() *TemplateEngine {
    return &TemplateEngine{
        templates: template.New("").Funcs(DefaultFuncs),
    }
}

func (e *TemplateEngine) LoadFS(fs embed.FS, pattern string) error {
    var err error
    e.templates, err = e.templates.ParseFS(fs, pattern)
    return err
}

func (e *TemplateEngine) LoadString(name, content string) error {
    _, err := e.templates.New(name).Parse(content)
    return err
}

func (e *TemplateEngine) Execute(name string, data any) (string, error) {
    var buf bytes.Buffer
    if err := e.templates.ExecuteTemplate(&buf, name, data); err != nil {
        return "", err
    }
    return buf.String(), nil
}

// Shell helper functions
func ShellQuote(s string) string {
    return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func IndentLines(indent int, s string) string {
    prefix := strings.Repeat(" ", indent)
    lines := strings.Split(s, "\n")
    for i := range lines {
        if lines[i] != "" {
            lines[i] = prefix + lines[i]
        }
    }
    return strings.Join(lines, "\n")
}

func DefaultValue(def, val any) any {
    if val == nil || val == "" {
        return def
    }
    return val
}

func EnvVar(name string) string {
    return fmt.Sprintf("${%s}", name)
}

func Conditional(cond bool, trueVal, falseVal string) string {
    if cond {
        return trueVal
    }
    return falseVal
}
```

**Impact:** ~465 lines saved (3x 155 line duplicates), single source of truth

---

## 115. ✅ COMPLETED - Nil-Safe Dereference Helpers

**Problem:** 150+ pointer nil check + dereference patterns across codebase.

**Files affected:**
- `internal/modules/server/dto/responses.go:45-180` (30+ instances)
- `internal/modules/site/dto/responses.go:38-165` (25+ instances)
- `internal/modules/auth/dto/responses.go:52-142` (20+ instances)
- `internal/modules/site/services/deployment_service.go:280-450` (35+ instances)
- `internal/modules/server/services/server_service.go:120-280` (25+ instances)

**Current (repeated 150+ times):**
```go
// Pattern 1: Simple dereference with default
var installedAt string
if site.InstalledAt != nil {
    installedAt = site.InstalledAt.Format(time.RFC3339)
}

// Pattern 2: Conditional field assignment
response := SiteResponse{
    Name: site.Name,
}
if site.Repository != nil {
    response.Repository = *site.Repository
}

// Pattern 3: Nested pointer access
var branchName string
if site.DeploymentConfig != nil && site.DeploymentConfig.Branch != nil {
    branchName = *site.DeploymentConfig.Branch
}
```

**Solution - Generic Nil-Safe Helpers:**
```go
// internal/pkg/ptr/safe.go
package ptr

// Deref safely dereferences a pointer, returning zero value if nil
func Deref[T any](p *T) T {
    if p == nil {
        var zero T
        return zero
    }
    return *p
}

// DerefOr safely dereferences a pointer with a default value
func DerefOr[T any](p *T, def T) T {
    if p == nil {
        return def
    }
    return *p
}

// DerefMap applies a transformation to a non-nil pointer value
func DerefMap[T, U any](p *T, fn func(T) U) *U {
    if p == nil {
        return nil
    }
    result := fn(*p)
    return &result
}

// DerefFormat formats a time pointer with default empty string
func DerefFormat(t *time.Time, layout string) string {
    if t == nil {
        return ""
    }
    return t.Format(layout)
}

// Of creates a pointer to the given value
func Of[T any](v T) *T {
    return &v
}

// Coalesce returns the first non-nil pointer value
func Coalesce[T any](ptrs ...*T) *T {
    for _, p := range ptrs {
        if p != nil {
            return p
        }
    }
    return nil
}

// CoalesceValue returns the first non-nil pointer's value or default
func CoalesceValue[T any](def T, ptrs ...*T) T {
    for _, p := range ptrs {
        if p != nil {
            return *p
        }
    }
    return def
}
```

**Refactored Usage:**
```go
response := SiteResponse{
    Name:        site.Name,
    InstalledAt: ptr.DerefFormat(site.InstalledAt, time.RFC3339),
    Repository:  ptr.Deref(site.Repository),
    Branch:      ptr.DerefOr(site.DeploymentConfig.Branch, "main"),
}
```

**Impact:** ~450 lines saved, cleaner DTO conversions

---

## 116. JSONMap Type Deduplication (P2)

**Problem:** 3 duplicate JSONMap type implementations for JSON column handling.

**Files affected:**
- `internal/modules/server/models/server.go:185-220`
- `internal/modules/site/models/site.go:165-200`
- `internal/modules/billing/models/subscription.go:142-175`

**Current (duplicated 3 times):**
```go
type JSONMap map[string]interface{}

func (j *JSONMap) Scan(value interface{}) error {
    if value == nil {
        *j = nil
        return nil
    }
    bytes, ok := value.([]byte)
    if !ok {
        return errors.New("type assertion to []byte failed")
    }
    return json.Unmarshal(bytes, j)
}

func (j JSONMap) Value() (driver.Value, error) {
    if j == nil {
        return nil, nil
    }
    return json.Marshal(j)
}

func (j JSONMap) GetString(key string) string {
    if v, ok := j[key].(string); ok {
        return v
    }
    return ""
}

func (j JSONMap) GetInt(key string) int {
    if v, ok := j[key].(float64); ok {
        return int(v)
    }
    return 0
}

func (j JSONMap) GetBool(key string) bool {
    if v, ok := j[key].(bool); ok {
        return v
    }
    return false
}
```

**Solution - Centralized JSON Types:**
```go
// internal/pkg/dbtype/json.go
package dbtype

import (
    "database/sql/driver"
    "encoding/json"
    "errors"
)

// JSONMap is a map that can be stored in a JSON column
type JSONMap map[string]any

func (j *JSONMap) Scan(value any) error {
    if value == nil {
        *j = nil
        return nil
    }
    bytes, ok := value.([]byte)
    if !ok {
        return errors.New("type assertion to []byte failed")
    }
    return json.Unmarshal(bytes, j)
}

func (j JSONMap) Value() (driver.Value, error) {
    if j == nil {
        return nil, nil
    }
    return json.Marshal(j)
}

// Typed accessors with generics
func Get[T any](j JSONMap, key string) (T, bool) {
    val, ok := j[key]
    if !ok {
        var zero T
        return zero, false
    }
    typed, ok := val.(T)
    return typed, ok
}

func GetOr[T any](j JSONMap, key string, def T) T {
    if val, ok := Get[T](j, key); ok {
        return val
    }
    return def
}

// JSONSlice is a slice that can be stored in a JSON column
type JSONSlice[T any] []T

func (j *JSONSlice[T]) Scan(value any) error {
    if value == nil {
        *j = nil
        return nil
    }
    bytes, ok := value.([]byte)
    if !ok {
        return errors.New("type assertion to []byte failed")
    }
    return json.Unmarshal(bytes, j)
}

func (j JSONSlice[T]) Value() (driver.Value, error) {
    if j == nil {
        return nil, nil
    }
    return json.Marshal(j)
}
```

**Impact:** ~110 lines saved, type-safe accessors

---

## 117. Enum SQL Scanner Generator (P2)

**Problem:** 40+ enum types repeat identical Scan/Value implementations.

**Files affected:**
- `internal/modules/server/enums/status.go:25-48`
- `internal/modules/site/enums/deployment_status.go:28-52`
- `internal/modules/site/enums/site_type.go:22-45`
- `internal/modules/database/enums/database_type.go:20-42`
- `internal/modules/backup/enums/backup_status.go:18-40`
- Plus 35+ more enum files

**Current (repeated 40+ times):**
```go
type ServerStatus string

const (
    ServerStatusPending   ServerStatus = "pending"
    ServerStatusActive    ServerStatus = "active"
    ServerStatusFailed    ServerStatus = "failed"
    ServerStatusDeleting  ServerStatus = "deleting"
)

func (s *ServerStatus) Scan(value interface{}) error {
    if value == nil {
        *s = ""
        return nil
    }
    sv, ok := value.(string)
    if !ok {
        bs, ok := value.([]byte)
        if !ok {
            return errors.New("invalid type for ServerStatus")
        }
        sv = string(bs)
    }
    *s = ServerStatus(sv)
    return nil
}

func (s ServerStatus) Value() (driver.Value, error) {
    return string(s), nil
}

func (s ServerStatus) String() string {
    return string(s)
}

func (s ServerStatus) IsValid() bool {
    switch s {
    case ServerStatusPending, ServerStatusActive, ServerStatusFailed, ServerStatusDeleting:
        return true
    }
    return false
}
```

**Solution - Enum Base Type with Generics:**
```go
// internal/pkg/enum/base.go
package enum

import (
    "database/sql/driver"
    "errors"
)

// StringEnum is a constraint for string-based enum types
type StringEnum interface {
    ~string
    IsValid() bool
}

// Scan provides a generic Scan implementation for string enums
func Scan[E StringEnum](e *E, value any) error {
    if value == nil {
        *e = E("")
        return nil
    }
    switch v := value.(type) {
    case string:
        *e = E(v)
    case []byte:
        *e = E(string(v))
    default:
        return errors.New("invalid type for enum")
    }
    return nil
}

// Value provides a generic Value implementation for string enums
func Value[E StringEnum](e E) (driver.Value, error) {
    return string(e), nil
}

// internal/pkg/enum/generator.go (for code generation)
// go:generate enumgen -type=ServerStatus -values=pending,active,failed,deleting
```

**Refactored Enum:**
```go
type ServerStatus string

const (
    ServerStatusPending  ServerStatus = "pending"
    ServerStatusActive   ServerStatus = "active"
    ServerStatusFailed   ServerStatus = "failed"
    ServerStatusDeleting ServerStatus = "deleting"
)

var validServerStatuses = map[ServerStatus]bool{
    ServerStatusPending: true, ServerStatusActive: true,
    ServerStatusFailed: true, ServerStatusDeleting: true,
}

func (s ServerStatus) IsValid() bool { return validServerStatuses[s] }
func (s *ServerStatus) Scan(v any) error { return enum.Scan(s, v) }
func (s ServerStatus) Value() (driver.Value, error) { return enum.Value(s) }
```

**Impact:** ~800 lines saved across 40+ enum files (~20 lines each)

---

## 118. SSH Stream Reader Pattern (P2)

**Problem:** SSH output streaming logic repeated across WebSocket handlers and task execution.

**Files affected:**
- `internal/modules/websocket/handlers/logs.go:85-145`
- `internal/modules/websocket/handlers/terminal.go:92-158`
- `internal/modules/websocket/handlers/script_execution.go:78-135`
- `internal/pkg/ssh/client.go:180-240`
- `internal/modules/server/services/command_service.go:95-155`

**Current (repeated 5+ times):**
```go
func streamOutput(session *ssh.Session, ws *websocket.Conn, done chan struct{}) {
    stdout, _ := session.StdoutPipe()
    stderr, _ := session.StderrPipe()

    var wg sync.WaitGroup
    wg.Add(2)

    go func() {
        defer wg.Done()
        reader := bufio.NewReader(stdout)
        for {
            line, err := reader.ReadString('\n')
            if err != nil {
                return
            }
            ws.WriteMessage(websocket.TextMessage, []byte(line))
        }
    }()

    go func() {
        defer wg.Done()
        reader := bufio.NewReader(stderr)
        for {
            line, err := reader.ReadString('\n')
            if err != nil {
                return
            }
            ws.WriteMessage(websocket.TextMessage, []byte("[stderr] " + line))
        }
    }()

    wg.Wait()
    close(done)
}
```

**Solution - Unified Stream Handler:**
```go
// internal/pkg/ssh/stream.go
package ssh

import (
    "bufio"
    "context"
    "io"
    "sync"
)

type StreamHandler interface {
    OnStdout(line string) error
    OnStderr(line string) error
    OnComplete(exitCode int) error
}

type StreamOptions struct {
    BufferSize   int
    LineMode     bool  // true for line-by-line, false for chunk-based
    MergeStderr  bool  // merge stderr into stdout
}

func DefaultStreamOptions() StreamOptions {
    return StreamOptions{
        BufferSize: 4096,
        LineMode:   true,
    }
}

func StreamSession(ctx context.Context, session *Session, handler StreamHandler, opts StreamOptions) error {
    stdout, _ := session.StdoutPipe()
    stderr, _ := session.StderrPipe()

    var wg sync.WaitGroup
    errCh := make(chan error, 2)

    streamReader := func(r io.Reader, isStderr bool) {
        defer wg.Done()
        reader := bufio.NewReaderSize(r, opts.BufferSize)

        for {
            select {
            case <-ctx.Done():
                return
            default:
            }

            line, err := reader.ReadString('\n')
            if line != "" {
                var sendErr error
                if isStderr && !opts.MergeStderr {
                    sendErr = handler.OnStderr(line)
                } else {
                    sendErr = handler.OnStdout(line)
                }
                if sendErr != nil {
                    errCh <- sendErr
                    return
                }
            }
            if err != nil {
                return
            }
        }
    }

    wg.Add(2)
    go streamReader(stdout, false)
    go streamReader(stderr, true)

    wg.Wait()

    exitCode := 0
    if err := session.Wait(); err != nil {
        if exitErr, ok := err.(*ssh.ExitError); ok {
            exitCode = exitErr.ExitStatus()
        }
    }

    return handler.OnComplete(exitCode)
}

// WebSocketStreamHandler implements StreamHandler for WebSocket connections
type WebSocketStreamHandler struct {
    conn *websocket.Conn
    mu   sync.Mutex
}

func NewWebSocketStreamHandler(conn *websocket.Conn) *WebSocketStreamHandler {
    return &WebSocketStreamHandler{conn: conn}
}

func (h *WebSocketStreamHandler) OnStdout(line string) error {
    h.mu.Lock()
    defer h.mu.Unlock()
    return h.conn.WriteMessage(websocket.TextMessage, []byte(line))
}

func (h *WebSocketStreamHandler) OnStderr(line string) error {
    h.mu.Lock()
    defer h.mu.Unlock()
    return h.conn.WriteMessage(websocket.TextMessage, []byte("[stderr] " + line))
}

func (h *WebSocketStreamHandler) OnComplete(exitCode int) error {
    h.mu.Lock()
    defer h.mu.Unlock()
    msg := fmt.Sprintf("[exit] %d", exitCode)
    return h.conn.WriteMessage(websocket.TextMessage, []byte(msg))
}
```

**Impact:** ~300 lines saved, consistent streaming behavior

---

## 119. Home Directory Helper (P2)

**Problem:** 6+ duplications of home directory path construction logic.

**Files affected:**
- `internal/modules/site/services/site_service.go:145-165`
- `internal/modules/site/tasks/templates/deploy.go:42-58`
- `internal/modules/server/services/ssh_key_service.go:78-92`
- `internal/modules/backup/services/backup_service.go:55-68`
- `internal/pkg/ssh/client.go:112-125`
- `internal/modules/database/services/database_service.go:88-102`

**Current (repeated 6+ times):**
```go
// Pattern 1: Get user home directory
func getHomeDir(username string) string {
    if username == "root" {
        return "/root"
    }
    return fmt.Sprintf("/home/%s", username)
}

// Pattern 2: Get SSH authorized keys path
func getAuthorizedKeysPath(username string) string {
    home := getHomeDir(username)
    return filepath.Join(home, ".ssh", "authorized_keys")
}

// Pattern 3: Get site directory
func getSiteDir(username, siteName string) string {
    home := getHomeDir(username)
    return filepath.Join(home, siteName)
}

// Pattern 4: Get release directory
func getReleaseDir(username, siteName string) string {
    return filepath.Join(getSiteDir(username, siteName), "releases")
}
```

**Solution - Unified Path Builder:**
```go
// internal/pkg/serverpath/path.go
package serverpath

import (
    "fmt"
    "path/filepath"
    "time"
)

// User represents a server user for path construction
type User struct {
    Name string
}

func ForUser(username string) User {
    return User{Name: username}
}

func (u User) Home() string {
    if u.Name == "root" {
        return "/root"
    }
    return fmt.Sprintf("/home/%s", u.Name)
}

func (u User) SSH() string {
    return filepath.Join(u.Home(), ".ssh")
}

func (u User) AuthorizedKeys() string {
    return filepath.Join(u.SSH(), "authorized_keys")
}

func (u User) KnownHosts() string {
    return filepath.Join(u.SSH(), "known_hosts")
}

// Site represents a site for path construction
type Site struct {
    User     User
    Name     string
    Domain   string
}

func ForSite(username, siteName string) Site {
    return Site{User: ForUser(username), Name: siteName}
}

func (s Site) Root() string {
    return filepath.Join(s.User.Home(), s.Name)
}

func (s Site) Current() string {
    return filepath.Join(s.Root(), "current")
}

func (s Site) Releases() string {
    return filepath.Join(s.Root(), "releases")
}

func (s Site) Release(timestamp time.Time) string {
    return filepath.Join(s.Releases(), timestamp.Format("20060102150405"))
}

func (s Site) Shared() string {
    return filepath.Join(s.Root(), "shared")
}

func (s Site) Storage() string {
    return filepath.Join(s.Shared(), "storage")
}

func (s Site) Env() string {
    return filepath.Join(s.Shared(), ".env")
}

func (s Site) Logs() string {
    return filepath.Join(s.Root(), "logs")
}

// System paths
func CaddyConfig() string {
    return "/etc/caddy/Caddyfile"
}

func CaddySites() string {
    return "/etc/caddy/sites"
}

func PHPFPMPool(version string) string {
    return fmt.Sprintf("/etc/php/%s/fpm/pool.d", version)
}
```

**Refactored Usage:**
```go
site := serverpath.ForSite(server.Username, site.Name)

releaseDir := site.Release(time.Now())
envPath := site.Env()
authKeys := site.User.AuthorizedKeys()
```

**Impact:** ~180 lines saved, consistent path handling

---

## 120. HMAC Signature Validator (P1)

**Problem:** 5+ implementations of HMAC signature verification for webhooks.

**Files affected:**
- `internal/modules/git/handlers/webhook_handler.go:85-118`
- `internal/modules/billing/handlers/stripe_handler.go:52-78`
- `internal/modules/billing/handlers/paddle_handler.go:48-72`
- `internal/modules/server/handlers/callback_handler.go:65-88`
- `internal/modules/auth/handlers/oauth_handler.go:92-115`

**Current (repeated 5+ times):**
```go
// GitHub webhook verification
func verifyGitHubSignature(payload []byte, signature string, secret string) bool {
    if signature == "" {
        return false
    }

    parts := strings.SplitN(signature, "=", 2)
    if len(parts) != 2 || parts[0] != "sha256" {
        return false
    }

    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write(payload)
    expected := hex.EncodeToString(mac.Sum(nil))

    return hmac.Equal([]byte(parts[1]), []byte(expected))
}

// Stripe webhook verification (similar but different format)
func verifyStripeSignature(payload []byte, header string, secret string) bool {
    parts := strings.Split(header, ",")
    var timestamp, signature string
    for _, part := range parts {
        kv := strings.SplitN(part, "=", 2)
        if len(kv) == 2 {
            switch kv[0] {
            case "t":
                timestamp = kv[1]
            case "v1":
                signature = kv[1]
            }
        }
    }
    // verify...
}
```

**Solution - Unified Signature Verifier:**
```go
// internal/pkg/signature/hmac.go
package signature

import (
    "crypto/hmac"
    "crypto/sha256"
    "crypto/sha512"
    "encoding/hex"
    "hash"
    "strings"
    "time"
)

type Algorithm string

const (
    SHA256 Algorithm = "sha256"
    SHA512 Algorithm = "sha512"
)

type SignatureFormat string

const (
    // FormatPrefixed: "sha256=abc123..."
    FormatPrefixed SignatureFormat = "prefixed"
    // FormatRaw: "abc123..."
    FormatRaw SignatureFormat = "raw"
    // FormatStripe: "t=timestamp,v1=signature"
    FormatStripe SignatureFormat = "stripe"
)

type Verifier struct {
    algorithm Algorithm
    format    SignatureFormat
    maxAge    time.Duration  // For timestamp-based signatures
}

func NewVerifier(algo Algorithm, format SignatureFormat) *Verifier {
    return &Verifier{
        algorithm: algo,
        format:    format,
    }
}

func (v *Verifier) WithMaxAge(d time.Duration) *Verifier {
    v.maxAge = d
    return v
}

func (v *Verifier) Verify(payload []byte, signatureHeader, secret string) bool {
    switch v.format {
    case FormatPrefixed:
        return v.verifyPrefixed(payload, signatureHeader, secret)
    case FormatRaw:
        return v.verifyRaw(payload, signatureHeader, secret)
    case FormatStripe:
        return v.verifyStripe(payload, signatureHeader, secret)
    }
    return false
}

func (v *Verifier) verifyPrefixed(payload []byte, header, secret string) bool {
    parts := strings.SplitN(header, "=", 2)
    if len(parts) != 2 {
        return false
    }
    expected := v.compute(payload, secret)
    return hmac.Equal([]byte(parts[1]), []byte(expected))
}

func (v *Verifier) compute(payload []byte, secret string) string {
    var h func() hash.Hash
    switch v.algorithm {
    case SHA256:
        h = sha256.New
    case SHA512:
        h = sha512.New
    default:
        h = sha256.New
    }
    mac := hmac.New(h, []byte(secret))
    mac.Write(payload)
    return hex.EncodeToString(mac.Sum(nil))
}

func (v *Verifier) Sign(payload []byte, secret string) string {
    return v.compute(payload, secret)
}

// Pre-configured verifiers for common providers
var (
    GitHub    = NewVerifier(SHA256, FormatPrefixed)
    GitLab    = NewVerifier(SHA256, FormatRaw) // GitLab uses token match, but we can adapt
    Stripe    = NewVerifier(SHA256, FormatStripe).WithMaxAge(5 * time.Minute)
    Paddle    = NewVerifier(SHA256, FormatPrefixed)
)
```

**Impact:** ~200 lines saved, secure signature verification

---

## 121. Token Generator Consolidation (P2)

**Problem:** Multiple token generation patterns with inconsistent entropy and formatting.

**Files affected:**
- `internal/modules/auth/services/auth_service.go:125-152` - API tokens
- `internal/modules/server/services/provision_service.go:88-108` - Callback tokens
- `internal/modules/site/services/deployment_service.go:165-182` - Deploy keys
- `internal/modules/git/services/webhook_service.go:72-88` - Webhook secrets
- `internal/modules/backup/services/backup_service.go:95-110` - Encryption keys

**Current (repeated 5+ times):**
```go
// Pattern 1: Random hex token
func generateToken() string {
    b := make([]byte, 32)
    rand.Read(b)
    return hex.EncodeToString(b)
}

// Pattern 2: URL-safe base64
func generateURLSafeToken() string {
    b := make([]byte, 32)
    rand.Read(b)
    return base64.URLEncoding.EncodeToString(b)
}

// Pattern 3: Prefixed token (like Stripe)
func generateAPIToken() string {
    b := make([]byte, 24)
    rand.Read(b)
    return "lnch_" + base64.RawURLEncoding.EncodeToString(b)
}

// Pattern 4: Short numeric code
func generateVerificationCode() string {
    n, _ := rand.Int(rand.Reader, big.NewInt(1000000))
    return fmt.Sprintf("%06d", n.Int64())
}
```

**Solution - Unified Token Generator:**
```go
// internal/pkg/token/generator.go
package token

import (
    "crypto/rand"
    "encoding/base64"
    "encoding/hex"
    "fmt"
    "math/big"
    "strings"
)

type Encoding int

const (
    Hex Encoding = iota
    Base64
    Base64URL
    Base64URLRaw
)

type Generator struct {
    length   int
    encoding Encoding
    prefix   string
}

func New(byteLength int) *Generator {
    return &Generator{
        length:   byteLength,
        encoding: Hex,
    }
}

func (g *Generator) WithEncoding(e Encoding) *Generator {
    g.encoding = e
    return g
}

func (g *Generator) WithPrefix(prefix string) *Generator {
    g.prefix = prefix
    return g
}

func (g *Generator) Generate() (string, error) {
    b := make([]byte, g.length)
    if _, err := rand.Read(b); err != nil {
        return "", err
    }

    var encoded string
    switch g.encoding {
    case Hex:
        encoded = hex.EncodeToString(b)
    case Base64:
        encoded = base64.StdEncoding.EncodeToString(b)
    case Base64URL:
        encoded = base64.URLEncoding.EncodeToString(b)
    case Base64URLRaw:
        encoded = base64.RawURLEncoding.EncodeToString(b)
    }

    if g.prefix != "" {
        return g.prefix + encoded, nil
    }
    return encoded, nil
}

func (g *Generator) MustGenerate() string {
    t, err := g.Generate()
    if err != nil {
        panic(err)
    }
    return t
}

// Convenience functions for common token types
func APIToken() string {
    return New(24).WithEncoding(Base64URLRaw).WithPrefix("lnch_").MustGenerate()
}

func WebhookSecret() string {
    return New(32).WithEncoding(Hex).MustGenerate()
}

func CallbackToken() string {
    return New(16).WithEncoding(Base64URLRaw).MustGenerate()
}

func VerificationCode(digits int) string {
    max := int64(1)
    for i := 0; i < digits; i++ {
        max *= 10
    }
    n, _ := rand.Int(rand.Reader, big.NewInt(max))
    return fmt.Sprintf("%0*d", digits, n.Int64())
}

func EncryptionKey() []byte {
    key := make([]byte, 32)
    rand.Read(key)
    return key
}
```

**Impact:** ~150 lines saved, consistent token generation

---

## 122. Time Expiration Checker (P2)

**Problem:** 8+ expiration check patterns repeated across auth, billing, and cache logic.

**Files affected:**
- `internal/modules/auth/services/token_service.go:85-112` - Token expiration
- `internal/modules/auth/services/password_reset_service.go:62-78` - Reset link expiration
- `internal/modules/billing/services/subscription_service.go:145-175` - Subscription status
- `internal/modules/server/services/provision_service.go:182-198` - Callback expiration
- `internal/modules/site/services/ssl_service.go:125-142` - Certificate expiration
- `internal/modules/backup/services/backup_service.go:188-205` - Retention policy
- `internal/pkg/cache/cache.go:75-92` - Cache TTL checks
- `internal/modules/auth/services/session_service.go:55-72` - Session expiration

**Current (repeated 8+ times):**
```go
// Pattern 1: Simple expiration check
func isExpired(expiresAt time.Time) bool {
    return time.Now().After(expiresAt)
}

// Pattern 2: With grace period
func isExpiredWithGrace(expiresAt time.Time, grace time.Duration) bool {
    return time.Now().After(expiresAt.Add(grace))
}

// Pattern 3: Check if expiring soon
func isExpiringSoon(expiresAt time.Time, threshold time.Duration) bool {
    return time.Now().Add(threshold).After(expiresAt)
}

// Pattern 4: Subscription status with trial
func getSubscriptionStatus(sub *Subscription) string {
    now := time.Now()
    if sub.TrialEndsAt != nil && now.Before(*sub.TrialEndsAt) {
        return "trialing"
    }
    if sub.EndsAt != nil && now.After(*sub.EndsAt) {
        return "expired"
    }
    if sub.EndsAt != nil && now.Add(7*24*time.Hour).After(*sub.EndsAt) {
        return "expiring_soon"
    }
    return "active"
}

// Pattern 5: Certificate expiration status
func getCertStatus(cert *Certificate) string {
    daysUntilExpiry := time.Until(cert.ExpiresAt).Hours() / 24
    if daysUntilExpiry < 0 {
        return "expired"
    } else if daysUntilExpiry < 7 {
        return "critical"
    } else if daysUntilExpiry < 30 {
        return "warning"
    }
    return "valid"
}
```

**Solution - Unified Expiration Helper:**
```go
// internal/pkg/timeutil/expiration.go
package timeutil

import "time"

// Expiration provides expiration checking utilities
type Expiration struct {
    at   time.Time
    now  func() time.Time // Allow mocking for tests
}

func ExpiresAt(t time.Time) Expiration {
    return Expiration{at: t, now: time.Now}
}

func ExpiresAtPtr(t *time.Time) *Expiration {
    if t == nil {
        return nil
    }
    e := ExpiresAt(*t)
    return &e
}

func (e Expiration) IsExpired() bool {
    return e.now().After(e.at)
}

func (e Expiration) IsExpiredWithGrace(grace time.Duration) bool {
    return e.now().After(e.at.Add(grace))
}

func (e Expiration) ExpiresWithin(d time.Duration) bool {
    return e.now().Add(d).After(e.at)
}

func (e Expiration) TimeRemaining() time.Duration {
    remaining := e.at.Sub(e.now())
    if remaining < 0 {
        return 0
    }
    return remaining
}

func (e Expiration) DaysRemaining() int {
    return int(e.TimeRemaining().Hours() / 24)
}

// Status returns a status string based on thresholds
type StatusThresholds struct {
    Critical time.Duration
    Warning  time.Duration
}

func DefaultThresholds() StatusThresholds {
    return StatusThresholds{
        Critical: 7 * 24 * time.Hour,
        Warning:  30 * 24 * time.Hour,
    }
}

func (e Expiration) Status(t StatusThresholds) string {
    if e.IsExpired() {
        return "expired"
    }
    if e.ExpiresWithin(t.Critical) {
        return "critical"
    }
    if e.ExpiresWithin(t.Warning) {
        return "warning"
    }
    return "valid"
}

// SubscriptionStatus handles complex subscription state
type SubscriptionStatus struct {
    TrialEndsAt *time.Time
    EndsAt      *time.Time
    GracePeriod time.Duration
}

func (s SubscriptionStatus) Status() string {
    now := time.Now()

    // Check trial
    if s.TrialEndsAt != nil && now.Before(*s.TrialEndsAt) {
        return "trialing"
    }

    // Check main subscription
    if s.EndsAt == nil {
        return "active"
    }

    exp := ExpiresAt(*s.EndsAt)
    if exp.IsExpired() {
        if exp.IsExpiredWithGrace(s.GracePeriod) {
            return "expired"
        }
        return "grace_period"
    }
    if exp.ExpiresWithin(7 * 24 * time.Hour) {
        return "expiring_soon"
    }
    return "active"
}
```

**Refactored Usage:**
```go
// Simple expiration
exp := timeutil.ExpiresAt(token.ExpiresAt)
if exp.IsExpired() {
    return errors.New("token expired")
}

// Certificate status
certExp := timeutil.ExpiresAt(cert.ExpiresAt)
status := certExp.Status(timeutil.DefaultThresholds())

// Subscription status
subStatus := timeutil.SubscriptionStatus{
    TrialEndsAt: sub.TrialEndsAt,
    EndsAt:      sub.EndsAt,
    GracePeriod: 7 * 24 * time.Hour,
}.Status()
```

**Impact:** ~240 lines saved, consistent time handling

---

## Extended Summary (Items 111-122)

| Category | Items | Est. LOC Saved |
|----------|-------|----------------|
| Route Middleware Chain | Item 111 | ~340 lines |
| Handler Module Aggregation | Item 112 | ~200 lines |
| Webhook Route Standardization | Item 113 | ~180 lines |
| Template Engine Consolidation | Item 114 | ~465 lines |
| Nil-Safe Dereference Helpers | Item 115 | ~450 lines |
| JSONMap Type Deduplication | Item 116 | ~110 lines |
| Enum SQL Scanner Generator | Item 117 | ~800 lines |
| SSH Stream Reader Pattern | Item 118 | ~300 lines |
| Home Directory Helper | Item 119 | ~180 lines |
| HMAC Signature Validator | Item 120 | ~200 lines |
| Token Generator Consolidation | Item 121 | ~150 lines |
| Time Expiration Checker | Item 122 | ~240 lines |
| **Round 8 Total** | **12 patterns** | **~3615 lines** |

---

## 123. HTTP Client Base Pattern (P1)

**Problem:** 14 provider files duplicate identical HTTP client creation with 30-second timeouts.

**Files affected:**
- `internal/modules/server/providers/digitalocean.go:35`
- `internal/modules/server/providers/hetzner.go:35`
- `internal/modules/server/providers/linode.go:35`
- `internal/modules/server/providers/vultr.go:35`
- `internal/modules/git/providers/github.go:35-37`
- `internal/modules/git/providers/gitlab.go:31-33`
- `internal/modules/git/providers/bitbucket.go:31-33`
- `internal/modules/dns/providers/cloudflare.go:29-31`
- `internal/modules/dns/providers/digitalocean.go:26-28`
- `internal/modules/backup/storage/dropbox.go:23-25`
- `internal/modules/billing/providers/lemon_squeezy.go:59-61`
- `internal/modules/notification/slack/admin_alert.go:21-23`

**Current (repeated 14 times):**
```go
type DigitalOceanProvider struct {
    BaseProvider
    client *http.Client
}

func NewDigitalOceanProvider() *DigitalOceanProvider {
    return &DigitalOceanProvider{
        client: &http.Client{Timeout: 30 * time.Second},
    }
}
```

**Solution - Unified HTTP Client Factory:**
```go
// internal/pkg/httpclient/client.go
package httpclient

import (
    "net/http"
    "time"
)

type Config struct {
    Timeout       time.Duration
    MaxRetries    int
    RetryWaitMin  time.Duration
    RetryWaitMax  time.Duration
}

func DefaultConfig() Config {
    return Config{
        Timeout:      30 * time.Second,
        MaxRetries:   3,
        RetryWaitMin: 1 * time.Second,
        RetryWaitMax: 5 * time.Second,
    }
}

type Client struct {
    http   *http.Client
    config Config
}

func New(opts ...Option) *Client {
    cfg := DefaultConfig()
    for _, opt := range opts {
        opt(&cfg)
    }
    return &Client{
        http:   &http.Client{Timeout: cfg.Timeout},
        config: cfg,
    }
}

func WithTimeout(d time.Duration) Option {
    return func(c *Config) { c.Timeout = d }
}

// Shared client for providers
var DefaultClient = New()
```

**Refactored Provider:**
```go
type DigitalOceanProvider struct {
    BaseProvider
    client *httpclient.Client
}

func NewDigitalOceanProvider() *DigitalOceanProvider {
    return &DigitalOceanProvider{
        client: httpclient.DefaultClient,
    }
}
```

**Impact:** ~280 lines saved, configurable retry behavior

---

## 124. API Request Builder (P1)

**Problem:** 20+ repetitions of Bearer token auth, 15+ Content-Type header settings.

**Files affected:**
- `internal/modules/git/providers/github.go:96, 134, 178, 233, 283, 375, 410, 493, 585`
- `internal/modules/git/providers/gitlab.go:174, 249, 326, 372`
- `internal/modules/git/providers/bitbucket.go:168, 244`
- `internal/modules/server/providers/digitalocean.go:278-279`
- `internal/modules/server/providers/hetzner.go:234-235`
- `internal/modules/dns/providers/cloudflare.go:520-522`
- `internal/modules/billing/providers/lemon_squeezy.go:84-86`

**Current (repeated 20+ times):**
```go
req, _ := http.NewRequestWithContext(ctx, method, url, body)
req.Header.Set("Authorization", "Bearer "+token)
req.Header.Set("Content-Type", "application/json")
req.Header.Set("Accept", "application/json")
```

**Solution - Fluent Request Builder:**
```go
// internal/pkg/httpclient/request.go
package httpclient

type RequestBuilder struct {
    method  string
    url     string
    headers map[string]string
    body    io.Reader
    ctx     context.Context
}

func NewRequest(ctx context.Context, method, url string) *RequestBuilder {
    return &RequestBuilder{
        ctx:     ctx,
        method:  method,
        url:     url,
        headers: make(map[string]string),
    }
}

func (r *RequestBuilder) BearerAuth(token string) *RequestBuilder {
    r.headers["Authorization"] = "Bearer " + token
    return r
}

func (r *RequestBuilder) BasicAuth(user, pass string) *RequestBuilder {
    auth := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
    r.headers["Authorization"] = "Basic " + auth
    return r
}

func (r *RequestBuilder) JSON() *RequestBuilder {
    r.headers["Content-Type"] = "application/json"
    r.headers["Accept"] = "application/json"
    return r
}

func (r *RequestBuilder) JSONBody(v any) *RequestBuilder {
    data, _ := json.Marshal(v)
    r.body = bytes.NewReader(data)
    return r.JSON()
}

func (r *RequestBuilder) Build() (*http.Request, error) {
    req, err := http.NewRequestWithContext(r.ctx, r.method, r.url, r.body)
    if err != nil {
        return nil, err
    }
    for k, v := range r.headers {
        req.Header.Set(k, v)
    }
    return req, nil
}
```

**Refactored Usage:**
```go
req, err := httpclient.NewRequest(ctx, "POST", url).
    BearerAuth(token).
    JSONBody(payload).
    Build()
```

**Impact:** ~400 lines saved, type-safe request building

---

## 125. Response Handler Consolidation (P1)

**Problem:** Duplicated status code validation and body parsing across 10+ providers.

**Files affected:**
- `internal/modules/server/providers/digitalocean.go:287-305`
- `internal/modules/server/providers/hetzner.go:239-252`
- `internal/modules/server/providers/linode.go:232-245`
- `internal/modules/server/providers/vultr.go:239-252`
- `internal/modules/git/providers/github.go:105-116, 143-150`
- `internal/modules/billing/providers/lemon_squeezy.go:99-114`
- `internal/modules/dns/providers/cloudflare.go:114-120`

**Current (repeated 7+ times):**
```go
respBody, err := io.ReadAll(resp.Body)
if err != nil {
    return nil, fmt.Errorf("failed to read response: %w", err)
}

if resp.StatusCode < 200 || resp.StatusCode >= 300 {
    return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(respBody))
}

if len(respBody) == 0 {
    return nil, nil
}

var result map[string]interface{}
if err := json.Unmarshal(respBody, &result); err != nil {
    return nil, fmt.Errorf("failed to parse response: %w", err)
}
```

**Solution - Generic Response Handler:**
```go
// internal/pkg/httpclient/response.go
package httpclient

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
)

type Response struct {
    StatusCode int
    Body       []byte
    Raw        *http.Response
}

func HandleResponse(resp *http.Response) (*Response, error) {
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read response: %w", err)
    }

    return &Response{
        StatusCode: resp.StatusCode,
        Body:       body,
        Raw:        resp,
    }, nil
}

func (r *Response) IsSuccess() bool {
    return r.StatusCode >= 200 && r.StatusCode < 300
}

func (r *Response) CheckSuccess() error {
    if !r.IsSuccess() {
        return fmt.Errorf("request failed with status %d: %s", r.StatusCode, string(r.Body))
    }
    return nil
}

func (r *Response) Decode(v any) error {
    if err := r.CheckSuccess(); err != nil {
        return err
    }
    if len(r.Body) == 0 {
        return nil
    }
    return json.Unmarshal(r.Body, v)
}

func (r *Response) DecodeMap() (map[string]any, error) {
    var result map[string]any
    if err := r.Decode(&result); err != nil {
        return nil, err
    }
    return result, nil
}

// Status-specific error handling
type StatusHandler struct {
    handlers map[int]error
}

func NewStatusHandler() *StatusHandler {
    return &StatusHandler{handlers: make(map[int]error)}
}

func (h *StatusHandler) On(code int, err error) *StatusHandler {
    h.handlers[code] = err
    return h
}

func (h *StatusHandler) Check(r *Response) error {
    if err, ok := h.handlers[r.StatusCode]; ok {
        return err
    }
    return r.CheckSuccess()
}
```

**Refactored Usage:**
```go
resp, err := client.Do(req)
if err != nil {
    return nil, err
}

r, err := httpclient.HandleResponse(resp)
if err != nil {
    return nil, err
}

// Simple case
var result ServerResponse
if err := r.Decode(&result); err != nil {
    return nil, err
}

// With specific error mapping
err = httpclient.NewStatusHandler().
    On(http.StatusNotFound, ErrNotFound).
    On(http.StatusUnauthorized, ErrUnauthorized).
    Check(r)
```

**Impact:** ~350 lines saved, consistent error handling

---

## 126. Pagination Parameter Parser (P2)

**Problem:** 4+ handlers manually parse limit parameters with different validation.

**Files affected:**
- `internal/modules/server/handlers/task_handler.go:21-25`
- `internal/modules/server/handlers/metric_handler.go:42-46`
- `internal/modules/websocket/handlers/logs.go:49-56`
- `internal/modules/websocket/handlers/metrics.go:40-49`
- `internal/modules/websocket/handlers/service_status.go:63-67`

**Current (repeated 5+ times with different ranges):**
```go
// task_handler.go - max 100
limit, _ := strconv.Atoi(c.Query("limit", "50"))
if limit < 1 || limit > 100 {
    limit = 50
}

// metric_handler.go - max 1000
limit, _ := strconv.Atoi(c.Query("limit", "100"))
if limit < 1 || limit > 1000 {
    limit = 100
}

// logs.go - different param name
tail, _ := strconv.Atoi(c.Query("tail", "100"))
if tail <= 0 {
    tail = 100
}
```

**Solution - Generic Parameter Parser:**
```go
// internal/pkg/fiber/params.go
package fiber

import (
    "strconv"
    "github.com/gofiber/fiber/v2"
)

type IntParamConfig struct {
    Name    string
    Default int
    Min     int
    Max     int
}

func ParseIntParam(c *fiber.Ctx, cfg IntParamConfig) int {
    val, err := strconv.Atoi(c.Query(cfg.Name, strconv.Itoa(cfg.Default)))
    if err != nil || val < cfg.Min {
        return cfg.Default
    }
    if cfg.Max > 0 && val > cfg.Max {
        return cfg.Max
    }
    return val
}

// Pre-configured parsers
func ParseLimit(c *fiber.Ctx, defaultVal, maxVal int) int {
    return ParseIntParam(c, IntParamConfig{
        Name:    "limit",
        Default: defaultVal,
        Min:     1,
        Max:     maxVal,
    })
}

func ParseTail(c *fiber.Ctx) int {
    return ParseIntParam(c, IntParamConfig{
        Name:    "tail",
        Default: 100,
        Min:     1,
        Max:     10000,
    })
}

func ParseInterval(c *fiber.Ctx, defaultVal int) int {
    return ParseIntParam(c, IntParamConfig{
        Name:    "interval",
        Default: defaultVal,
        Min:     1,
        Max:     60,
    })
}
```

**Refactored Usage:**
```go
limit := fiber.ParseLimit(c, 50, 100)
tail := fiber.ParseTail(c)
interval := fiber.ParseInterval(c, 5)
```

**Impact:** ~100 lines saved, consistent validation

---

## 127. Dynamic Sort Handler (P2)

**Problem:** 25+ hardcoded `.Order()` clauses with no dynamic sorting support.

**Files affected:**
- `internal/modules/server/repositories/server_repository.go:87, 107, 122`
- `internal/modules/site/repositories/site_repository.go:100, 110, 130, 141`
- `internal/modules/site/repositories/deployment_repository.go:58, 69, 85, 101, 154`
- `internal/modules/server/repositories/task_repository.go:47, 62`
- `internal/modules/server/repositories/metric_repository.go:44, 59`
- `internal/modules/backup/repositories/backup_repository.go:58, 80, 102`
- `internal/modules/dns/repositories/domain_repository.go:48, 65`
- Plus 10+ more repository files

**Current (hardcoded everywhere):**
```go
// All repositories hardcode sort
query.Order("created_at DESC")
query.Order("recorded_at DESC")
query.Order("type ASC, name ASC")  // DNS special case
```

**Solution - Dynamic Sort with Validation:**
```go
// internal/pkg/repository/sort.go
package repository

import (
    "fmt"
    "gorm.io/gorm"
)

type SortConfig struct {
    AllowedFields map[string]string  // user field -> DB column
    DefaultField  string
    DefaultDir    string
}

type SortParams struct {
    Field     string
    Direction string
}

func (s SortParams) Apply(db *gorm.DB, cfg SortConfig) *gorm.DB {
    field := s.Field
    dir := s.Direction

    // Use defaults if not specified
    if field == "" {
        field = cfg.DefaultField
    }
    if dir == "" {
        dir = cfg.DefaultDir
    }

    // Validate field
    dbColumn, ok := cfg.AllowedFields[field]
    if !ok {
        dbColumn = cfg.AllowedFields[cfg.DefaultField]
    }

    // Validate direction
    if dir != "asc" && dir != "desc" {
        dir = cfg.DefaultDir
    }

    return db.Order(fmt.Sprintf("%s %s", dbColumn, dir))
}

// Pre-built configs
var ServerSortConfig = SortConfig{
    AllowedFields: map[string]string{
        "created_at": "created_at",
        "name":       "name",
        "status":     "status",
    },
    DefaultField: "created_at",
    DefaultDir:   "desc",
}

var DeploymentSortConfig = SortConfig{
    AllowedFields: map[string]string{
        "created_at": "created_at",
        "status":     "status",
    },
    DefaultField: "created_at",
    DefaultDir:   "desc",
}
```

**Refactored Usage:**
```go
func (r *ServerRepository) FindAll(ctx context.Context, teamID string, sort SortParams) ([]models.Server, error) {
    var servers []models.Server
    query := r.DB().WithContext(ctx).Where("team_id = ?", teamID)
    query = sort.Apply(query, ServerSortConfig)
    return servers, query.Find(&servers).Error
}
```

**Impact:** ~200 lines saved, flexible sorting with security

---

## 128. Filter Options Pattern (P2)

**Problem:** Repeated WHERE clause building with optional filters across repositories.

**Files affected:**
- `internal/modules/server/repositories/metric_repository.go:31-51`
- `internal/modules/git/repositories/source_control_repository.go:236-266`
- `internal/modules/site/repositories/deployment_repository.go:80-94`
- `internal/modules/server/repositories/server_repository.go:80-89`

**Current (repeated conditional WHERE building):**
```go
query := r.DB().WithContext(ctx).Where("server_id = ?", serverID)
if from != nil {
    query = query.Where("recorded_at >= ?", from)
}
if to != nil {
    query = query.Where("recorded_at <= ?", to)
}
if limit > 0 {
    query = query.Limit(limit)
}
```

**Solution - Generic Filter Options:**
```go
// internal/pkg/repository/filter.go
package repository

import (
    "time"
    "gorm.io/gorm"
)

type FilterOption func(*gorm.DB) *gorm.DB

func WithDateRange(column string, from, to *time.Time) FilterOption {
    return func(db *gorm.DB) *gorm.DB {
        if from != nil {
            db = db.Where(column+" >= ?", from)
        }
        if to != nil {
            db = db.Where(column+" <= ?", to)
        }
        return db
    }
}

func WithOptionalLimit(limit int) FilterOption {
    return func(db *gorm.DB) *gorm.DB {
        if limit > 0 {
            return db.Limit(limit)
        }
        return db
    }
}

func WithStatus(status string) FilterOption {
    return func(db *gorm.DB) *gorm.DB {
        if status != "" {
            return db.Where("status = ?", status)
        }
        return db
    }
}

func WithStatuses(statuses []string) FilterOption {
    return func(db *gorm.DB) *gorm.DB {
        if len(statuses) > 0 {
            return db.Where("status IN ?", statuses)
        }
        return db
    }
}

func ApplyFilters(db *gorm.DB, opts ...FilterOption) *gorm.DB {
    for _, opt := range opts {
        db = opt(db)
    }
    return db
}
```

**Refactored Usage:**
```go
func (r *MetricRepository) FindByServer(ctx context.Context, serverID string, from, to *time.Time, limit int) ([]models.Metric, error) {
    var metrics []models.Metric
    query := r.DB().WithContext(ctx).Where("server_id = ?", serverID)

    query = repository.ApplyFilters(query,
        repository.WithDateRange("recorded_at", from, to),
        repository.WithOptionalLimit(limit),
    )

    return metrics, query.Order("recorded_at DESC").Find(&metrics).Error
}
```

**Impact:** ~180 lines saved, composable filters

---

## 129. Retry Strategy Consolidation (P2)

**Problem:** Scattered retry logic with different backoff strategies across modules.

**Files affected:**
- `internal/modules/server/jobs/wait_for_server_to_connect.go:17-23, 132-212`
- `internal/modules/site/jobs/deploy.go:39-50`
- `internal/modules/billing/models/webhook_event.go:18-32`
- `internal/modules/server/jobs/create_on_provider.go` (polling)

**Current (different retry implementations):**
```go
// wait_for_server_to_connect.go - Exponential backoff
const (
    maxConnectionAttempts = 30
    initialRetryDelay     = 10 * time.Second
    maxRetryDelay         = 30 * time.Second
)

delay := initialRetryDelay
for attempt := 1; attempt <= maxConnectionAttempts; attempt++ {
    // ... attempt connection ...
    time.Sleep(delay)
    delay = time.Duration(float64(delay) * 1.2)
    if delay > maxRetryDelay {
        delay = maxRetryDelay
    }
}

// deploy.go - Fixed retry
for i := 0; i < 3; i++ {
    deployment, err = j.ctx.DeploymentRepo.FindByID(ctx, j.Payload.DeploymentID)
    if err == nil {
        break
    }
    if i < 2 {
        time.Sleep(500 * time.Millisecond)
    }
}
```

**Solution - Unified Retry Package:**
```go
// internal/pkg/retry/retry.go
package retry

import (
    "context"
    "time"
)

type Strategy interface {
    NextDelay(attempt int) time.Duration
    MaxAttempts() int
}

type ExponentialBackoff struct {
    Initial    time.Duration
    Max        time.Duration
    Multiplier float64
    Attempts   int
}

func (e ExponentialBackoff) NextDelay(attempt int) time.Duration {
    delay := time.Duration(float64(e.Initial) * pow(e.Multiplier, float64(attempt-1)))
    if delay > e.Max {
        return e.Max
    }
    return delay
}

func (e ExponentialBackoff) MaxAttempts() int { return e.Attempts }

type FixedDelay struct {
    Delay    time.Duration
    Attempts int
}

func (f FixedDelay) NextDelay(_ int) time.Duration { return f.Delay }
func (f FixedDelay) MaxAttempts() int              { return f.Attempts }

func Do[T any](ctx context.Context, s Strategy, fn func() (T, error)) (T, error) {
    var result T
    var err error

    for attempt := 1; attempt <= s.MaxAttempts(); attempt++ {
        result, err = fn()
        if err == nil {
            return result, nil
        }

        if attempt < s.MaxAttempts() {
            select {
            case <-ctx.Done():
                return result, ctx.Err()
            case <-time.After(s.NextDelay(attempt)):
            }
        }
    }
    return result, err
}

// Pre-configured strategies
var (
    ConnectionRetry = ExponentialBackoff{
        Initial:    10 * time.Second,
        Max:        30 * time.Second,
        Multiplier: 1.2,
        Attempts:   30,
    }

    QuickRetry = FixedDelay{
        Delay:    500 * time.Millisecond,
        Attempts: 3,
    }
)
```

**Refactored Usage:**
```go
// Connection with exponential backoff
connected, err := retry.Do(ctx, retry.ConnectionRetry, func() (bool, error) {
    return j.attemptConnection(ctx)
})

// Quick DB read retry
deployment, err := retry.Do(ctx, retry.QuickRetry, func() (*models.Deployment, error) {
    return j.ctx.DeploymentRepo.FindByID(ctx, j.Payload.DeploymentID)
})
```

**Impact:** ~250 lines saved, consistent retry behavior

---

## 130. Metrics Parsing Helper (P2)

**Problem:** 6 identical strconv.ParseFloat patterns in metrics webhook handler.

**Files affected:**
- `internal/modules/server/handlers/metrics_webhook_handler.go:104-127`

**Current (repeated 6 times):**
```go
diskTotal, err := strconv.ParseFloat(req.Data.DiskTotal, 64)
if err != nil {
    h.logger.Warn().Err(err).Str("server_id", serverID).Str("value", req.Data.DiskTotal).Msg("Failed to parse disk_total")
}
diskFree, err := strconv.ParseFloat(req.Data.DiskFree, 64)
if err != nil {
    h.logger.Warn().Err(err).Str("server_id", serverID).Str("value", req.Data.DiskFree).Msg("Failed to parse disk_free")
}
diskUsed, err := strconv.ParseFloat(req.Data.DiskUsed, 64)
// ... repeated for memory_total, memory_free, memory_used
```

**Solution - Metrics Parser Helper:**
```go
// internal/pkg/metrics/parser.go
package metrics

import (
    "strconv"
    "github.com/rs/zerolog"
)

type Parser struct {
    logger   zerolog.Logger
    serverID string
    errors   []string
}

func NewParser(logger zerolog.Logger, serverID string) *Parser {
    return &Parser{logger: logger, serverID: serverID}
}

func (p *Parser) ParseFloat(value, fieldName string) float64 {
    if value == "" {
        return 0
    }
    v, err := strconv.ParseFloat(value, 64)
    if err != nil {
        p.logger.Warn().
            Err(err).
            Str("server_id", p.serverID).
            Str("field", fieldName).
            Str("value", value).
            Msg("Failed to parse metric")
        p.errors = append(p.errors, fieldName)
        return 0
    }
    return v
}

func (p *Parser) ParseInt(value, fieldName string) int64 {
    if value == "" {
        return 0
    }
    v, err := strconv.ParseInt(value, 10, 64)
    if err != nil {
        p.logger.Warn().
            Err(err).
            Str("server_id", p.serverID).
            Str("field", fieldName).
            Str("value", value).
            Msg("Failed to parse metric")
        p.errors = append(p.errors, fieldName)
        return 0
    }
    return v
}

func (p *Parser) HasErrors() bool {
    return len(p.errors) > 0
}
```

**Refactored Usage:**
```go
parser := metrics.NewParser(h.logger, serverID)

metric := &models.Metric{
    DiskTotal:   parser.ParseFloat(req.Data.DiskTotal, "disk_total"),
    DiskFree:    parser.ParseFloat(req.Data.DiskFree, "disk_free"),
    DiskUsed:    parser.ParseFloat(req.Data.DiskUsed, "disk_used"),
    MemoryTotal: parser.ParseFloat(req.Data.MemoryTotal, "memory_total"),
    MemoryFree:  parser.ParseFloat(req.Data.MemoryFree, "memory_free"),
    MemoryUsed:  parser.ParseFloat(req.Data.MemoryUsed, "memory_used"),
}
```

**Impact:** ~80 lines saved, consistent error logging

---

## 131. Service Status Checker (P2)

**Problem:** Duplicated service status parsing and memory formatting in WebSocket and job handlers.

**Files affected:**
- `internal/modules/websocket/handlers/service_status.go:325-405`
- `internal/modules/server/jobs/check_service_status.go:135-234`

**Current (duplicated formatting):**
```go
// WebSocket handler - lines 353-370
func formatMemory(bytes int64) string {
    const unit = 1024
    if bytes < unit {
        return fmt.Sprintf("%d B", bytes)
    }
    div, exp := int64(unit), 0
    for n := bytes / unit; n >= unit; n /= unit {
        div *= unit
        exp++
    }
    return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// Job handler - lines 215-234 (similar but different implementation)
func formatBytes(bytesStr string) string {
    // Similar logic...
}
```

**Solution - Unified Status Package:**
```go
// internal/pkg/status/format.go
package status

import (
    "fmt"
    "time"
)

func FormatBytes(bytes int64) string {
    const unit = 1024
    if bytes < unit {
        return fmt.Sprintf("%d B", bytes)
    }
    div, exp := int64(unit), 0
    for n := bytes / unit; n >= unit; n /= unit {
        div *= unit
        exp++
    }
    return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func FormatUptime(seconds int64) string {
    d := time.Duration(seconds) * time.Second
    days := int(d.Hours() / 24)
    hours := int(d.Hours()) % 24
    minutes := int(d.Minutes()) % 60

    if days > 0 {
        return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
    }
    if hours > 0 {
        return fmt.Sprintf("%dh %dm", hours, minutes)
    }
    return fmt.Sprintf("%dm", minutes)
}

// internal/pkg/status/parser.go
type ServiceStatus struct {
    ID       string `json:"id"`
    Software string `json:"software"`
    Name     string `json:"name"`
    Status   string `json:"status"`
    IsActive bool   `json:"is_active"`
    Memory   string `json:"memory,omitempty"`
    Uptime   string `json:"uptime,omitempty"`
    PID      int    `json:"pid,omitempty"`
}

func ParseSystemctlOutput(output string) *ServiceStatus {
    // Unified parsing logic
}
```

**Impact:** ~150 lines saved, consistent formatting

---

## 132. Daemon Status Task Deduplication (P1)

**Problem:** 85+ lines of identical bash script duplicated between server and site modules.

**Files affected:**
- `internal/modules/server/tasks/daemon_status.go:18-85`
- `internal/modules/site/tasks/daemon_status.go:18-85`

**Current (IDENTICAL 85+ lines in both files):**
```go
// server/tasks/daemon_status.go
const CheckServerDaemonStatusTaskType = "server:check_daemon_status"

// site/tasks/daemon_status.go
const CheckDaemonStatusTaskType = "site:check_daemon_status"

// Both contain identical bash script:
script := `#!/bin/bash
set -euo pipefail

# Get supervisorctl status and parse it
supervisorctl status 2>/dev/null | while read -r line; do
    # ... 60+ lines of identical parsing logic ...
done

echo "===STATUS_CHECK_COMPLETE==="`
```

**Solution - Single Shared Task:**
```go
// internal/pkg/tasks/daemon_status.go
package tasks

import (
    "github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

const CheckDaemonStatusTaskType = "common:check_daemon_status"

var daemonStatusScript = `#!/bin/bash
set -euo pipefail

# Get supervisorctl status and parse it
supervisorctl status 2>/dev/null | while read -r line; do
    program=$(echo "$line" | awk '{print $1}')
    status=$(echo "$line" | awk '{print $2}')
    # ... parsing logic ...
done

echo "===STATUS_CHECK_COMPLETE==="`

func CheckDaemonStatus() *taskrunner.BaseTask {
    return taskrunner.NewBaseTask(
        taskrunner.WithName("Check Daemon Status"),
        taskrunner.WithScript(daemonStatusScript),
        taskrunner.WithTimeoutSeconds(30),
    )
}

// Result parser
type DaemonStatus struct {
    DaemonID      string `json:"daemon_id"`
    Status        string `json:"status"`
    PID           string `json:"pid"`
    UptimeSeconds int64  `json:"uptime_seconds"`
    Description   string `json:"description"`
    Error         string `json:"error,omitempty"`
}

func ParseDaemonStatusOutput(output string) []DaemonStatus {
    // Unified parsing logic
}
```

**Refactored Usage:**
```go
// Both modules use the same task
import "github.com/kkz6/launch-go/internal/pkg/tasks"

task := tasks.CheckDaemonStatus()
result, err := runner.RunTask(task).Dispatch(ctx)
statuses := tasks.ParseDaemonStatusOutput(result.GetOutput())
```

**Impact:** ~170 lines saved (85 lines x 2 - shared code)

---

## 133. Health Check Aggregator (P2)

**Problem:** Basic health endpoint with no dependency checks.

**Files affected:**
- `cmd/api/main.go:230-234` - Minimal health check
- `internal/pkg/cache/redis.go:65-68` - Redis Ping exists but unused

**Current (minimal implementation):**
```go
// cmd/api/main.go
func (a *Application) healthCheck(c *fiber.Ctx) error {
    return c.JSON(fiber.Map{
        "status": "ok",
        "time":   time.Now().UTC(),
    })
}
```

**Solution - Comprehensive Health Package:**
```go
// internal/pkg/health/health.go
package health

import (
    "context"
    "sync"
    "time"
)

type Status string

const (
    StatusHealthy   Status = "healthy"
    StatusDegraded  Status = "degraded"
    StatusUnhealthy Status = "unhealthy"
)

type Check struct {
    Name    string        `json:"name"`
    Status  Status        `json:"status"`
    Message string        `json:"message,omitempty"`
    Latency time.Duration `json:"latency_ms"`
}

type Result struct {
    Status    Status    `json:"status"`
    Timestamp time.Time `json:"timestamp"`
    Checks    []Check   `json:"checks"`
}

type Checker interface {
    Name() string
    Check(ctx context.Context) error
}

type Aggregator struct {
    checkers []Checker
}

func NewAggregator() *Aggregator {
    return &Aggregator{}
}

func (a *Aggregator) Add(c Checker) *Aggregator {
    a.checkers = append(a.checkers, c)
    return a
}

func (a *Aggregator) Check(ctx context.Context) Result {
    result := Result{
        Status:    StatusHealthy,
        Timestamp: time.Now().UTC(),
        Checks:    make([]Check, len(a.checkers)),
    }

    var wg sync.WaitGroup
    for i, checker := range a.checkers {
        wg.Add(1)
        go func(idx int, c Checker) {
            defer wg.Done()
            start := time.Now()
            err := c.Check(ctx)
            check := Check{
                Name:    c.Name(),
                Latency: time.Since(start),
                Status:  StatusHealthy,
            }
            if err != nil {
                check.Status = StatusUnhealthy
                check.Message = err.Error()
                result.Status = StatusDegraded
            }
            result.Checks[idx] = check
        }(i, checker)
    }
    wg.Wait()

    return result
}

// Built-in checkers
type DatabaseChecker struct {
    db *gorm.DB
}

func (c *DatabaseChecker) Name() string { return "database" }
func (c *DatabaseChecker) Check(ctx context.Context) error {
    sqlDB, err := c.db.DB()
    if err != nil {
        return err
    }
    return sqlDB.PingContext(ctx)
}

type RedisChecker struct {
    client *redis.Client
}

func (c *RedisChecker) Name() string { return "redis" }
func (c *RedisChecker) Check(ctx context.Context) error {
    return c.client.Ping(ctx).Err()
}
```

**Refactored Usage:**
```go
healthChecker := health.NewAggregator().
    Add(&health.DatabaseChecker{db: db}).
    Add(&health.RedisChecker{client: redis})

func (a *Application) healthCheck(c *fiber.Ctx) error {
    result := a.healthChecker.Check(c.Context())
    status := fiber.StatusOK
    if result.Status != health.StatusHealthy {
        status = fiber.StatusServiceUnavailable
    }
    return c.Status(status).JSON(result)
}
```

**Impact:** ~120 lines saved, proper dependency checking

---

## 134. Config Loader Helper (P2)

**Problem:** Repeating load/setDefaults pattern in all 10 config files.

**Files affected:**
- `internal/config/app.go:21-40`
- `internal/config/database.go:35-57`
- `internal/config/redis.go:12-24`
- `internal/config/jwt.go:11-21`
- `internal/config/cors.go:10-18`
- `internal/config/queue.go:10-18`
- `internal/config/billing.go:20-36`
- `internal/config/git.go:36-68`
- `internal/config/slack.go:17-21` (inconsistent key naming)
- `internal/config/sentry.go:21-45`

**Current (repeated in every config file):**
```go
func loadAppConfig() AppConfig {
    return AppConfig{
        Name:        viper.GetString("APP_NAME"),
        Environment: viper.GetString("APP_ENV"),
        Port:        viper.GetString("APP_PORT"),
        Debug:       viper.GetBool("APP_DEBUG"),
    }
}

func setAppDefaults() {
    viper.SetDefault("APP_NAME", "Launch")
    viper.SetDefault("APP_ENV", "development")
    viper.SetDefault("APP_PORT", "8080")
    viper.SetDefault("APP_DEBUG", true)
}
```

**Solution - Declarative Config Loader:**
```go
// internal/pkg/config/loader.go
package config

import (
    "reflect"
    "github.com/spf13/viper"
)

// Tag-based config loading
// Field tag: `env:"APP_NAME" default:"Launch"`

func Load(cfg interface{}) error {
    v := reflect.ValueOf(cfg).Elem()
    t := v.Type()

    for i := 0; i < t.NumField(); i++ {
        field := t.Field(i)
        envKey := field.Tag.Get("env")
        defaultVal := field.Tag.Get("default")

        if envKey == "" {
            continue
        }

        // Set default if specified
        if defaultVal != "" {
            viper.SetDefault(envKey, defaultVal)
        }

        // Load value based on field type
        switch field.Type.Kind() {
        case reflect.String:
            v.Field(i).SetString(viper.GetString(envKey))
        case reflect.Bool:
            v.Field(i).SetBool(viper.GetBool(envKey))
        case reflect.Int, reflect.Int64:
            v.Field(i).SetInt(viper.GetInt64(envKey))
        }
    }
    return nil
}

// Validation interface
type Validator interface {
    Validate() error
}

func LoadAndValidate(cfg interface{}) error {
    if err := Load(cfg); err != nil {
        return err
    }
    if v, ok := cfg.(Validator); ok {
        return v.Validate()
    }
    return nil
}
```

**Refactored Config:**
```go
type AppConfig struct {
    Name        string `env:"APP_NAME" default:"Launch"`
    Environment string `env:"APP_ENV" default:"development"`
    Port        string `env:"APP_PORT" default:"8080"`
    Debug       bool   `env:"APP_DEBUG" default:"true"`
    URL         string `env:"APP_URL" default:"http://localhost:8080"`
    Key         string `env:"APP_KEY"`
    LocalMode   bool   `env:"APP_LOCAL_MODE" default:"false"`
}

func (c AppConfig) Validate() error {
    if c.Key == "" && c.Environment == "production" {
        return errors.New("APP_KEY is required in production")
    }
    return nil
}

func loadAppConfig() AppConfig {
    var cfg AppConfig
    config.LoadAndValidate(&cfg)
    return cfg
}
```

**Impact:** ~200 lines saved, declarative configuration

---

## Extended Summary (Items 123-134)

| Category | Items | Est. LOC Saved |
|----------|-------|----------------|
| HTTP Client Base Pattern | Item 123 | ~280 lines |
| API Request Builder | Item 124 | ~400 lines |
| Response Handler Consolidation | Item 125 | ~350 lines |
| Pagination Parameter Parser | Item 126 | ~100 lines |
| Dynamic Sort Handler | Item 127 | ~200 lines |
| Filter Options Pattern | Item 128 | ~180 lines |
| Retry Strategy Consolidation | Item 129 | ~250 lines |
| Metrics Parsing Helper | Item 130 | ~80 lines |
| Service Status Checker | Item 131 | ~150 lines |
| Daemon Status Task Deduplication | Item 132 | ~170 lines |
| Health Check Aggregator | Item 133 | ~120 lines |
| Config Loader Helper | Item 134 | ~200 lines |
| **Round 9 Total** | **12 patterns** | **~2480 lines** |

---

## 135. Webhook Channel Base Class (P1)

**Problem:** 3 webhook-based notification channels duplicate identical HTTP posting and validation logic.

**Files affected:**
- `internal/modules/notification/channels/slack.go` (95 lines)
- `internal/modules/notification/channels/discord.go` (95 lines)
- `internal/modules/notification/channels/telegram.go` (123 lines)

**Current (repeated 3 times):**
```go
type SlackChannel struct {
    webhookURL string
    httpClient HTTPClient
}

func (s *SlackChannel) Connect(ctx context.Context) error {
    webhookURL := s.GetWebhookURL()
    if webhookURL == "" {
        return ErrInvalidConfiguration
    }
    if s.httpClient == nil {
        return ErrConnectionFailed
    }

    payload := slackMessage{Text: "*Connected to Launch*..."}
    _, statusCode, err := s.httpClient.Post(ctx, webhookURL, payload)
    if statusCode < 200 || statusCode >= 300 {
        return fmt.Errorf("%w: received status code %d", ErrConnectionFailed, statusCode)
    }
    return nil
}

func (s *SlackChannel) Send(ctx context.Context, n Notification) error {
    // Identical pattern: build message -> POST -> check status
}
```

**Solution - Base Webhook Channel:**
```go
// internal/modules/notification/channels/webhook_base.go
package channels

type WebhookChannel struct {
    httpClient     HTTPClient
    webhookURL     string
    connectMessage string
}

func (w *WebhookChannel) ValidateConfig() error {
    if w.webhookURL == "" {
        return ErrInvalidConfiguration
    }
    if w.httpClient == nil {
        return ErrConnectionFailed
    }
    return nil
}

func (w *WebhookChannel) Post(ctx context.Context, payload any) error {
    if err := w.ValidateConfig(); err != nil {
        return err
    }
    _, statusCode, err := w.httpClient.Post(ctx, w.webhookURL, payload)
    if err != nil {
        return fmt.Errorf("%w: %v", ErrSendFailed, err)
    }
    if statusCode < 200 || statusCode >= 300 {
        return fmt.Errorf("%w: received status code %d", ErrSendFailed, statusCode)
    }
    return nil
}

// Subclass only overrides message formatting
type SlackChannel struct {
    WebhookChannel
}

func (s *SlackChannel) Connect(ctx context.Context) error {
    return s.Post(ctx, slackMessage{Text: "*Connected to Launch*..."})
}

func (s *SlackChannel) Send(ctx context.Context, n Notification) error {
    return s.Post(ctx, slackMessage{Text: n.ToSlack()})
}
```

**Impact:** ~190 lines saved, consistent webhook handling

---

## 136. Admin Alert Base Class (P1)

**Problem:** 4 admin alert types duplicate 500+ lines of identical builder methods and message construction.

**Files affected:**
- `internal/modules/notification/slack/server_provisioning_failed_alert.go` (174 lines)
- `internal/modules/notification/slack/php_installation_failed_alert.go` (143 lines)
- `internal/modules/notification/slack/php_extension_install_failed_alert.go` (141 lines)
- `internal/modules/notification/slack/php_extension_uninstall_failed_alert.go` (~140 lines)

**Current (repeated in all 4 files):**
```go
type ServerProvisioningFailedAdminAlert struct {
    alerter              *AdminAlerter
    server               ServerInfo
    user                 *UserInfo
    output               string
    errorMessage         string
    outputRetrievalError string
}

// These 4 builder methods are IDENTICAL in all files:
func (a *ServerProvisioningFailedAdminAlert) WithUser(user *UserInfo) *ServerProvisioningFailedAdminAlert {
    a.user = user
    return a
}

func (a *ServerProvisioningFailedAdminAlert) WithOutput(output string) *ServerProvisioningFailedAdminAlert {
    a.output = output
    return a
}

func (a *ServerProvisioningFailedAdminAlert) WithErrorMessage(msg string) *ServerProvisioningFailedAdminAlert {
    a.errorMessage = msg
    return a
}

// buildMessage() is 80% identical with minor header/description differences
func (a *ServerProvisioningFailedAdminAlert) buildMessage() *BlockKitMessage {
    message := NewBlockKitMessage(fallbackText)
    message.AddHeader("🚨 Server Provisioning Failed")  // Only this differs
    message.AddSection("Description specific to alert type")  // And this
    message.AddDivider()

    // IDENTICAL: Server details section (25 lines)
    message.AddFieldsSection(
        fmt.Sprintf("*Server Name:*\n%s", a.server.Name),
        fmt.Sprintf("*Team:*\n%s", a.server.TeamName),
    )

    // IDENTICAL: User details section (15 lines)
    userName := "Unknown"
    if a.user != nil && a.user.Name != "" {
        userName = a.user.Name
    }
    message.AddFieldsSection(...)

    // IDENTICAL: Output handling (10 lines)
    if a.output != "" {
        message.AddSection(fmt.Sprintf("```\n%s\n```", TruncateOutput(a.output, 2900)))
    }

    // IDENTICAL: Context footer (5 lines)
    message.AddContext(fmt.Sprintf("Server ID: %s | Team ID: %s | %s", ...))

    return message
}
```

**Solution - Base Admin Alert:**
```go
// internal/modules/notification/slack/base_admin_alert.go
package slack

type BaseAdminAlert struct {
    alerter              *AdminAlerter
    server               ServerInfo
    user                 *UserInfo
    output               string
    errorMessage         string
    outputRetrievalError string
}

func (b *BaseAdminAlert) WithUser(user *UserInfo) *BaseAdminAlert {
    b.user = user
    return b
}

func (b *BaseAdminAlert) WithOutput(output string) *BaseAdminAlert {
    b.output = output
    return b
}

func (b *BaseAdminAlert) WithErrorMessage(msg string) *BaseAdminAlert {
    b.errorMessage = msg
    return b
}

type AlertConfig struct {
    Header       string
    Description  string
    ExtraFields  func(*BlockKitMessage)  // Hook for type-specific fields
}

func (b *BaseAdminAlert) BuildMessage(cfg AlertConfig) *BlockKitMessage {
    message := NewBlockKitMessage(cfg.Header)
    message.AddHeader(cfg.Header)
    message.AddSection(cfg.Description)
    message.AddDivider()

    // Standard server details
    b.addServerDetails(message)

    // Type-specific fields
    if cfg.ExtraFields != nil {
        cfg.ExtraFields(message)
    }

    // Standard user, output, context
    b.addUserDetails(message)
    b.addOutputSection(message)
    b.addContextFooter(message)

    return message
}

// Specific alert only defines config
type ServerProvisioningFailedAdminAlert struct {
    BaseAdminAlert
}

func (a *ServerProvisioningFailedAdminAlert) buildMessage() *BlockKitMessage {
    return a.BuildMessage(AlertConfig{
        Header:      "🚨 Server Provisioning Failed",
        Description: "Server provisioning has failed. Details below.",
    })
}
```

**Impact:** ~400 lines saved, extensible alert system

---

## 137. SSH Client Factory (P1)

**Problem:** 7+ instances of identical SSH client creation with hardcoded 30-second timeout.

**Files affected:**
- `internal/pkg/taskrunner/dispatcher.go:148-154, 336-342`
- `internal/pkg/taskrunner/stream_monitor.go:97-103, 305-311`
- `internal/modules/server/services/server_service.go:262-268`

**Current (repeated 7+ times):**
```go
sshClient, err := taskrunner.NewSSHClient(taskrunner.SSHConfig{
    Host:       conn.Host,
    Port:       conn.Port,
    User:       conn.User,
    PrivateKey: conn.PrivateKey,
    Timeout:    30 * time.Second,  // Hardcoded everywhere
})
if err != nil {
    return nil, fmt.Errorf("failed to create SSH client: %w", err)
}
defer sshClient.Close()

if err := sshClient.Connect(); err != nil {
    return nil, fmt.Errorf("failed to connect: %w", err)
}
```

**Solution - SSH Client Factory:**
```go
// internal/pkg/ssh/factory.go
package ssh

import (
    "context"
    "time"
)

type ClientFactory struct {
    defaultTimeout time.Duration
}

func NewClientFactory(timeout time.Duration) *ClientFactory {
    if timeout == 0 {
        timeout = 30 * time.Second
    }
    return &ClientFactory{defaultTimeout: timeout}
}

var DefaultFactory = NewClientFactory(30 * time.Second)

func (f *ClientFactory) Connect(ctx context.Context, conn *Connection) (*Client, error) {
    client, err := NewSSHClient(SSHConfig{
        Host:       conn.Host,
        Port:       conn.Port,
        User:       conn.User,
        PrivateKey: conn.PrivateKey,
        Timeout:    f.defaultTimeout,
    })
    if err != nil {
        return nil, fmt.Errorf("failed to create SSH client: %w", err)
    }

    if err := client.Connect(); err != nil {
        client.Close()
        return nil, fmt.Errorf("failed to connect: %w", err)
    }

    return client, nil
}

// WithConnection provides a session scoped to a connection
func (f *ClientFactory) WithConnection(ctx context.Context, conn *Connection, fn func(*Client) error) error {
    client, err := f.Connect(ctx, conn)
    if err != nil {
        return err
    }
    defer client.Close()
    return fn(client)
}
```

**Refactored Usage:**
```go
err := ssh.DefaultFactory.WithConnection(ctx, conn, func(client *ssh.Client) error {
    return client.Run(ctx, command)
})
```

**Impact:** ~140 lines saved, configurable timeout

---

## 138. Cron Schedule Constants (P2)

**Problem:** Cron expressions and frequencies duplicated across job files with redundant Frequency field.

**Files affected:**
- `internal/modules/site/jobs/enable_laravel_scheduler.go:75-83`
- `internal/modules/site/jobs/install_wordpress_cron.go:69-77`
- `internal/modules/server/jobs/install_task_cleanup_cron.go:52-59`
- `internal/modules/server/jobs/cleanup_old_metrics.go:67-71`
- `internal/modules/server/models/cron.go:18-19` (redundant fields)

**Current (Expression and Frequency both stored):**
```go
cron := &servermodels.Cron{
    ServerID:   server.ID,
    Expression: "* * * * *",    // Hardcoded
    Frequency:  "every_minute", // Redundant!
    Hidden:     true,
}
```

**Solution - Cron Schedule Enum:**
```go
// internal/modules/server/enums/cron_schedule.go
package enums

type CronSchedule string

const (
    EveryMinute CronSchedule = "* * * * *"
    Hourly      CronSchedule = "0 * * * *"
    Daily2AM    CronSchedule = "0 2 * * *"
    Daily3AM    CronSchedule = "0 3 * * *"
    Weekly      CronSchedule = "0 0 * * 0"
)

func (c CronSchedule) Expression() string {
    return string(c)
}

func (c CronSchedule) Description() string {
    switch c {
    case EveryMinute:
        return "every_minute"
    case Hourly:
        return "hourly"
    case Daily2AM, Daily3AM:
        return "daily"
    case Weekly:
        return "weekly"
    }
    return "custom"
}

// Remove Frequency field from Cron model - derive from Expression
func (c *Cron) GetFrequency() string {
    return CronSchedule(c.Expression).Description()
}
```

**Impact:** ~60 lines saved, single source of truth

---

## 139. Job Task Builder Generator (P1)

**Problem:** 78+ nearly identical NewTaskName() functions wrapping pkgjobs.NewTask().

**Files affected:**
- `internal/modules/site/jobs/*.go` (20+ task builders)
- `internal/modules/server/jobs/*.go` (30+ task builders)
- `internal/modules/database/jobs/*.go` (10+ task builders)
- `internal/modules/backup/jobs/*.go` (5+ task builders)

**Current (repeated 78+ times):**
```go
func NewCreateDeploymentTask(siteID string, userID *string, gitHash *string, branch *string) (*asynq.Task, error) {
    return pkgjobs.NewTask(TypeCreateDeployment, CreateDeploymentPayload{
        SiteID:  siteID,
        UserID:  userID,
        GitHash: gitHash,
        Branch:  branch,
    })
}

func NewInstallQueueTask(siteID, queueID string, userID *string) (*asynq.Task, error) {
    return pkgjobs.NewTask(TypeInstallQueue, InstallQueuePayload{
        SiteID:  siteID,
        QueueID: queueID,
        UserID:  userID,
    })
}
```

**Solution - Generic Task Builder:**
```go
// internal/pkg/jobs/builder.go
package jobs

import (
    "encoding/json"
    "github.com/hibiken/asynq"
)

type TaskBuilder[P any] struct {
    taskType string
    opts     []asynq.Option
}

func NewTaskBuilder[P any](taskType string) *TaskBuilder[P] {
    return &TaskBuilder[P]{taskType: taskType}
}

func (b *TaskBuilder[P]) WithTaskID(id string) *TaskBuilder[P] {
    b.opts = append(b.opts, asynq.TaskID(id))
    return b
}

func (b *TaskBuilder[P]) WithRetry(n int) *TaskBuilder[P] {
    b.opts = append(b.opts, asynq.MaxRetry(n))
    return b
}

func (b *TaskBuilder[P]) Build(payload P) (*asynq.Task, error) {
    data, err := json.Marshal(payload)
    if err != nil {
        return nil, err
    }
    return asynq.NewTask(b.taskType, data, b.opts...), nil
}

// Usage becomes simpler
var CreateDeploymentBuilder = NewTaskBuilder[CreateDeploymentPayload](TypeCreateDeployment)

// Call site
task, err := CreateDeploymentBuilder.Build(CreateDeploymentPayload{
    SiteID: siteID,
    UserID: userID,
})
```

**Impact:** ~400 lines saved, type-safe builders

---

## 140. Job Context Base Consolidation (P1)

**Problem:** 5-6 JobContext implementations with ~70% duplicated code.

**Files affected:**
- `internal/modules/site/jobs/context.go:40-87`
- `internal/modules/server/jobs/context.go:31-64`
- `internal/modules/database/jobs/context.go:37-68`
- `internal/modules/backup/jobs/context.go:25-46`
- `internal/modules/script/jobs/context.go`

**Current (repeated in each module):**
```go
type JobContext struct {
    *pkgjobs.Base
    DB         *gorm.DB
    Logger     *zerolog.Logger
    WS         broadcast.TeamBroadcaster
    Dispatcher taskrunner.TaskDispatcher
    Queue      *queue.Client
    // Module-specific repos...
    SiteRepo   *repositories.SiteRepository
}

func NewJobContext(
    db *gorm.DB,
    logger *zerolog.Logger,
    ws broadcast.TeamBroadcaster,
    dispatcher taskrunner.TaskDispatcher,
    queueClient *queue.Client,
    // 5+ more parameters...
) *JobContext {
    return &JobContext{
        Base: pkgjobs.NewBase(pkgjobs.BaseDeps{
            DB:         db,
            Logger:     logger,
            WS:         ws,
            Dispatcher: dispatcher,
        }),
        DB:         db,
        Logger:     logger,
        // Repeat all assignments...
    }
}
```

**Solution - Composable Job Context:**
```go
// internal/pkg/jobs/context.go
package jobs

type CoreDeps struct {
    DB         *gorm.DB
    Logger     *zerolog.Logger
    WS         broadcast.TeamBroadcaster
    Dispatcher taskrunner.TaskDispatcher
    Queue      *queue.Client
}

type CoreContext struct {
    *Base
    deps CoreDeps
}

func NewCoreContext(deps CoreDeps) *CoreContext {
    return &CoreContext{
        Base: NewBase(BaseDeps{
            DB:         deps.DB,
            Logger:     deps.Logger,
            WS:         deps.WS,
            Dispatcher: deps.Dispatcher,
        }),
        deps: deps,
    }
}

func (c *CoreContext) DB() *gorm.DB           { return c.deps.DB }
func (c *CoreContext) Logger() *zerolog.Logger { return c.deps.Logger }
func (c *CoreContext) Queue() *queue.Client    { return c.deps.Queue }

// Module contexts embed core and add repos
type SiteJobContext struct {
    *CoreContext
    repos *SiteRepositories
}

func NewSiteJobContext(core *CoreContext, repos *SiteRepositories) *SiteJobContext {
    return &SiteJobContext{CoreContext: core, repos: repos}
}
```

**Impact:** ~250 lines saved, consistent initialization

---

## 141. DNS Provider HTTP Base (P1)

**Problem:** Cloudflare (559 lines) and DigitalOcean (494 lines) DNS providers duplicate HTTP handling.

**Files affected:**
- `internal/modules/dns/providers/cloudflare.go:513-540`
- `internal/modules/dns/providers/digitalocean.go:428-456`

**Current (duplicated request/response handling):**
```go
// Both providers have identical HTTP patterns:
func (p *CloudflareProvider) doRequest(ctx context.Context, method, path string, body any) ([]byte, error) {
    req, _ := http.NewRequestWithContext(ctx, method, p.baseURL+path, bodyReader)
    req.Header.Set("Authorization", "Bearer "+p.token)
    req.Header.Set("Content-Type", "application/json")

    resp, err := p.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    respBody, _ := io.ReadAll(resp.Body)

    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return nil, p.parseError(respBody)
    }

    return respBody, nil
}
```

**Solution - DNS Provider Base:**
```go
// internal/modules/dns/providers/base.go
package providers

type BaseHTTPProvider struct {
    httpClient *http.Client
    baseURL    string
    token      string
}

func (p *BaseHTTPProvider) DoRequest(ctx context.Context, method, path string, body any) ([]byte, error) {
    var bodyReader io.Reader
    if body != nil {
        data, _ := json.Marshal(body)
        bodyReader = bytes.NewReader(data)
    }

    req, err := http.NewRequestWithContext(ctx, method, p.baseURL+path, bodyReader)
    if err != nil {
        return nil, err
    }

    req.Header.Set("Authorization", "Bearer "+p.token)
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Accept", "application/json")

    resp, err := p.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    respBody, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }

    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(respBody))
    }

    return respBody, nil
}

// Cloudflare embeds base
type CloudflareProvider struct {
    BaseHTTPProvider
    accountID string
    zoneCache map[string]string
}
```

**Impact:** ~200 lines saved, consistent HTTP handling

---

## 142. Backup Job Payload Base (P2)

**Problem:** 3 backup jobs have identical payload structures.

**Files affected:**
- `internal/modules/backup/jobs/install_backup.go:14-18`
- `internal/modules/backup/jobs/run_manual_backup.go:14-18`
- `internal/modules/backup/jobs/sync_launch_config.go:15-18`

**Current (identical structures):**
```go
// install_backup.go
type InstallBackupPayload struct {
    ServerID string  `json:"server_id"`
    BackupID string  `json:"backup_id"`
    UserID   *string `json:"user_id,omitempty"`
}

// run_manual_backup.go
type RunManualBackupPayload struct {
    ServerID string  `json:"server_id"`
    BackupID string  `json:"backup_id"`
    UserID   *string `json:"user_id,omitempty"`
}
```

**Solution - Shared Backup Payload:**
```go
// internal/modules/backup/jobs/payloads.go
package jobs

// BackupJobPayload is the common payload for backup-related jobs
type BackupJobPayload struct {
    ServerID string  `json:"server_id"`
    BackupID string  `json:"backup_id"`
    UserID   *string `json:"user_id,omitempty"`
}

// Type aliases for clarity (optional)
type InstallBackupPayload = BackupJobPayload
type RunManualBackupPayload = BackupJobPayload
type SyncLaunchConfigPayload = BackupJobPayload
```

**Impact:** ~30 lines saved, single payload definition

---

## 143. Service Dispatch Helper (P2)

**Problem:** 3 nearly identical job dispatch helper methods in backup service.

**Files affected:**
- `internal/modules/backup/services/backup_service.go:224-291`

**Current (repeated 3 times):**
```go
func (s *BackupService) dispatchInstallBackup(serverID, backupID string) {
    if s.Queue == nil {
        s.Logger.Warn().Msg("Queue not configured, skipping InstallBackup job")
        return
    }
    task, err := jobs.NewInstallBackupTask(serverID, backupID, nil)
    if err != nil {
        s.Logger.Error().Err(err).Msg("Failed to create InstallBackup task")
        return
    }
    if _, err := s.Queue.EnqueueDefault(task); err != nil {
        s.Logger.Error().Err(err).Msg("Failed to enqueue InstallBackup job")
    }
}

// dispatchDeleteBackup and dispatchRunManualBackup are identical patterns
```

**Solution - Generic Dispatch Helper:**
```go
// internal/pkg/service/dispatch.go
package service

type TaskFactory func() (*asynq.Task, error)

func (s *Base) DispatchTask(name string, factory TaskFactory) {
    if s.Queue == nil {
        s.Logger.Warn().Str("task", name).Msg("Queue not configured, skipping task")
        return
    }
    task, err := factory()
    if err != nil {
        s.Logger.Error().Err(err).Str("task", name).Msg("Failed to create task")
        return
    }
    if _, err := s.Queue.EnqueueDefault(task); err != nil {
        s.Logger.Error().Err(err).Str("task", name).Msg("Failed to enqueue task")
    }
}

// Usage
s.DispatchTask("InstallBackup", func() (*asynq.Task, error) {
    return jobs.NewInstallBackupTask(serverID, backupID, nil)
})
```

**Impact:** ~50 lines saved, reusable dispatch

---

## 144. Webhook Signature Verifier (P1)

**Problem:** 4 separate HMAC-SHA256 signature verification implementations.

**Files affected:**
- `internal/modules/git/providers/github.go:319-321`
- `internal/modules/git/providers/gitlab.go:159`
- `internal/modules/git/providers/bitbucket.go:90`
- `internal/modules/billing/handlers/webhook_handler.go:112`

**Current (repeated 4 times):**
```go
// GitHub
mac := hmac.New(sha256.New, []byte(p.config.WebhookSecret))
mac.Write(payload)
expectedSignature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
    return ErrInvalidSignature
}

// GitLab (slightly different format)
// Bitbucket (different header)
// Billing (Stripe format)
```

**Solution - Unified Signature Verifier:**
```go
// internal/pkg/webhook/signature.go
package webhook

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "strings"
)

type SignatureFormat int

const (
    FormatSHA256Prefix   SignatureFormat = iota // "sha256=..."
    FormatRaw                                    // Raw hex signature
    FormatStripe                                 // "t=timestamp,v1=signature"
)

type Verifier struct {
    secret string
    format SignatureFormat
}

func NewVerifier(secret string, format SignatureFormat) *Verifier {
    return &Verifier{secret: secret, format: format}
}

func (v *Verifier) Verify(payload []byte, signature string) bool {
    expected := v.computeSignature(payload)

    switch v.format {
    case FormatSHA256Prefix:
        return hmac.Equal([]byte("sha256="+expected), []byte(signature))
    case FormatRaw:
        return hmac.Equal([]byte(expected), []byte(signature))
    case FormatStripe:
        return v.verifyStripe(payload, signature)
    }
    return false
}

func (v *Verifier) computeSignature(payload []byte) string {
    mac := hmac.New(sha256.New, []byte(v.secret))
    mac.Write(payload)
    return hex.EncodeToString(mac.Sum(nil))
}

// Pre-configured verifiers
func GitHubVerifier(secret string) *Verifier {
    return NewVerifier(secret, FormatSHA256Prefix)
}

func GitLabVerifier(secret string) *Verifier {
    return NewVerifier(secret, FormatRaw)
}
```

**Impact:** ~80 lines saved, secure verification

---

## 145. Activity Logger Helper (P2)

**Problem:** 18+ files repeat identical activity logging boilerplate with optional user attribution.

**Files affected:**
- `internal/modules/server/jobs/archive_server.go:41-49`
- `internal/modules/server/jobs/unarchive_server.go:41-49`
- `internal/modules/server/jobs/delete_server.go:92-100`
- `internal/modules/database/jobs/install_database.go:79-87`
- Plus 14+ more job files

**Current (repeated 20+ times):**
```go
logger := activity.New(j.ctx.DB).
    WithContext(ctx).
    UseLog("server").
    On(server).
    WithEvent("archived")
if j.Payload.UserID != nil {
    logger.CausedByUser(*j.Payload.UserID)
}
logger.Log("Server has been archived")
```

**Solution - Activity Logger Helper:**
```go
// internal/pkg/activity/helpers.go
package activity

// LogAction is a helper for common activity logging pattern
func LogAction(db *gorm.DB, ctx context.Context, opts LogOptions) {
    logger := New(db).
        WithContext(ctx).
        UseLog(opts.Log).
        On(opts.Subject).
        WithEvent(opts.Event)

    if opts.UserID != nil {
        logger.CausedByUser(*opts.UserID)
    }

    if opts.Properties != nil {
        logger.WithProperties(opts.Properties)
    }

    logger.Log(opts.Description)
}

type LogOptions struct {
    Log         string
    Subject     Subject
    Event       string
    Description string
    UserID      *string
    Properties  map[string]any
}

// Usage becomes one-liner
activity.LogAction(j.ctx.DB, ctx, activity.LogOptions{
    Log:         "server",
    Subject:     server,
    Event:       "archived",
    Description: "Server has been archived",
    UserID:      j.Payload.UserID,
})
```

**Impact:** ~180 lines saved, consistent activity logging

---

## 146. Token Generation Consolidation (P2)

**Problem:** Duplicate token generation in model and utils package.

**Files affected:**
- `internal/pkg/utils/token.go:11-46` (4 functions)
- `internal/modules/auth/models/password_reset_token.go:31-39` (duplicate)

**Current (duplicated):**
```go
// utils/token.go
func GenerateSecureToken(byteLength int) (string, error) {
    bytes := make([]byte, byteLength)
    if _, err := rand.Read(bytes); err != nil {
        return "", err
    }
    return base64.URLEncoding.EncodeToString(bytes), nil
}

// models/password_reset_token.go - DUPLICATE
func GenerateToken(length int) (string, error) {
    bytes := make([]byte, length)
    if _, err := rand.Read(bytes); err != nil {
        return "", err
    }
    return base64.URLEncoding.EncodeToString(bytes)[:length], nil
}
```

**Solution - Single Token Generator:**
```go
// internal/pkg/token/generator.go
package token

import (
    "crypto/rand"
    "encoding/base64"
    "encoding/hex"
)

type Encoding int

const (
    Base64URL Encoding = iota
    Hex
)

func Generate(byteLength int, encoding Encoding) (string, error) {
    bytes := make([]byte, byteLength)
    if _, err := rand.Read(bytes); err != nil {
        return "", err
    }

    switch encoding {
    case Hex:
        return hex.EncodeToString(bytes), nil
    default:
        return base64.URLEncoding.EncodeToString(bytes), nil
    }
}

// Convenience functions
func SecureToken() (string, error)     { return Generate(32, Base64URL) }
func ShortToken() (string, error)      { return Generate(8, Base64URL) }
func HexToken(length int) string       { s, _ := Generate(length, Hex); return s }

// Remove duplicate from models/password_reset_token.go
```

**Impact:** ~40 lines saved, single token source

---

## Extended Summary (Items 135-146)

| Category | Items | Est. LOC Saved |
|----------|-------|----------------|
| Webhook Channel Base Class | Item 135 | ~190 lines |
| Admin Alert Base Class | Item 136 | ~400 lines |
| SSH Client Factory | Item 137 | ~140 lines |
| Cron Schedule Constants | Item 138 | ~60 lines |
| Job Task Builder Generator | Item 139 | ~400 lines |
| Job Context Base Consolidation | Item 140 | ~250 lines |
| DNS Provider HTTP Base | Item 141 | ~200 lines |
| Backup Job Payload Base | Item 142 | ~30 lines |
| Service Dispatch Helper | Item 143 | ~50 lines |
| Webhook Signature Verifier | Item 144 | ~80 lines |
| Activity Logger Helper | Item 145 | ~180 lines |
| Token Generation Consolidation | Item 146 | ~40 lines |
| **Round 10 Total** | **12 patterns** | **~2020 lines** |

---

## Updated Final Summary

| Category | Items | Total LOC Saved |
|----------|-------|-----------------|
| Handler Bases | 5 patterns | ~400 lines |
| Repository Patterns | 5 patterns | ~1200 lines |
| Service Patterns | 3 patterns | ~450 lines |
| Job/Task Patterns | 3 patterns | ~800 lines |
| Model Mixins | 5 patterns | ~270 lines |
| Provider API Clients | 3 patterns | ~450 lines |
| Middleware Patterns | 1 pattern | ~30 lines |
| Config Patterns | 1 pattern | ~86 lines |
| DTO/Request Patterns | 2 patterns | ~550 lines |
| Testing Patterns | 2 patterns | ~250 lines |
| Infrastructure Patterns | 6 patterns | ~1075 lines |
| Validation & Response | 5 patterns | ~610 lines |
| Template & Script | 3 patterns | ~160 lines |
| Interface & Query | 4 patterns | ~500 lines |
| Security & Events | 3 patterns | ~100 lines |
| HTTP & Helpers (Round 3) | 12 patterns | ~1790 lines |
| Error & DI (Round 4) | 12 patterns | ~1320 lines |
| GORM & Jobs (Round 5) | 12 patterns | ~2290 lines |
| Context & Auth (Round 6) | 12 patterns | ~2560 lines |
| Validation & Enums (Round 7) | 12 patterns | ~1805 lines |
| Routes & Crypto (Round 8) | 12 patterns | ~3615 lines |
| HTTP Client & Config (Round 9) | 12 patterns | ~2480 lines |
| Notification & Jobs (Round 10) | 12 patterns | ~2020 lines |
| **Grand Total** | **146 patterns** | **~24,811 lines** |

---

## Round 11: Model Hooks, DTO Mapping, Status Patterns

---

## 147. BeforeCreate Hook Consolidation (P1)

**Problem:** 10+ models have BeforeCreate hooks with identical token/status initialization patterns.

**Files affected:**
- `internal/modules/server/models/server.go:74-88`
- `internal/modules/backup/models/backup_job.go:26-36`
- `internal/modules/site/models/site.go` (status init)
- `internal/modules/site/models/deployment.go` (status init)
- `internal/modules/database/models/database.go` (status init)
- `internal/modules/auth/models/user.go` (token init)
- `internal/modules/notification/models/notification_channel.go` (status init)
- `internal/modules/git/models/source_control.go` (status init)

**Current (repeated in 10+ models):**
```go
func (s *Server) BeforeCreate(tx *gorm.DB) error {
    if err := s.BaseModel.BeforeCreate(tx); err != nil {
        return err
    }
    if s.Status == "" {
        s.Status = enums.ServerStatusNew
    }
    if s.ConnectionToken == "" {
        s.ConnectionToken, _ = token.SecureToken()
    }
    return nil
}
```

**Solution - Embeddable Init Mixins:**
```go
// internal/pkg/models/init_mixins.go
package models

// StatusInitMixin for models with status field
type StatusInitMixin[T ~string] struct {
    defaultStatus T
}

func (m *StatusInitMixin[T]) InitStatus(current *T) {
    if *current == "" {
        *current = m.defaultStatus
    }
}

// TokenInitMixin for models needing secure tokens
type TokenInitMixin struct{}

func (m *TokenInitMixin) InitToken(field *string) {
    if *field == "" {
        *field, _ = token.SecureToken()
    }
}

// Combined mixin for common case
type StatusTokenMixin[T ~string] struct {
    StatusInitMixin[T]
    TokenInitMixin
}

// Model usage becomes:
type Server struct {
    BaseModel
    StatusTokenMixin[enums.ServerStatus]
    // ...
}

func (s *Server) BeforeCreate(tx *gorm.DB) error {
    if err := s.BaseModel.BeforeCreate(tx); err != nil {
        return err
    }
    s.InitStatus(&s.Status)
    s.InitToken(&s.ConnectionToken)
    return nil
}
```

**Impact:** ~80 lines saved, consistent initialization

---

## 148. ✅ COMPLETED - Generic Slice Mapper

**Problem:** Identical slice transformation pattern repeated in 8+ DTO files.

**Files affected:**
- `internal/modules/auth/dto/team.go:45-52`
- `internal/modules/auth/dto/user.go:38-45`
- `internal/modules/database/dto/database.go:35-42`
- `internal/modules/database/dto/database_user.go:28-35`
- `internal/modules/backup/dto/backup.go:40-47`
- `internal/modules/notification/dto/channel.go:30-37`
- `internal/modules/git/dto/source_control.go:25-32`
- `internal/modules/site/dto/site.go:55-62`

**Current (repeated 8+ times):**
```go
func ToTeamsResponse(teams []models.Team) []TeamResponse {
    responses := make([]TeamResponse, len(teams))
    for i, team := range teams {
        responses[i] = ToTeamResponse(team)
    }
    return responses
}

func ToDatabasesResponse(dbs []models.Database) []DatabaseResponse {
    responses := make([]DatabaseResponse, len(dbs))
    for i, db := range dbs {
        responses[i] = ToDatabaseResponse(db)
    }
    return responses
}
```

**Solution - Generic Mapper:**
```go
// internal/pkg/dto/mapper.go
package dto

// MapSlice transforms a slice using a mapping function
func MapSlice[T, R any](items []T, mapper func(T) R) []R {
    if items == nil {
        return nil
    }
    result := make([]R, len(items))
    for i, item := range items {
        result[i] = mapper(item)
    }
    return result
}

// Usage becomes one-liner:
func ToTeamsResponse(teams []models.Team) []TeamResponse {
    return dto.MapSlice(teams, ToTeamResponse)
}

// Or inline:
responses := dto.MapSlice(servers, dto.ToServerResponse)
```

**Impact:** ~120 lines saved, type-safe generic mapper

---

## 149. ✅ COMPLETED - Enum Response Helper

**Problem:** Status enums all have identical String() and Label() boilerplate.

**Files affected:**
- `internal/modules/server/enums/server_status.go:18-45`
- `internal/modules/site/enums/site_status.go:15-42`
- `internal/modules/site/enums/deployment_status.go:12-38`
- `internal/modules/database/enums/database_status.go:10-35`
- `internal/modules/backup/enums/backup_status.go:8-32`
- `internal/modules/billing/enums/subscription_status.go:10-36`

**Current (repeated per enum):**
```go
func (s ServerStatus) String() string {
    return string(s)
}

func (s ServerStatus) Label() string {
    labels := map[ServerStatus]string{
        ServerStatusNew:          "New",
        ServerStatusProvisioning: "Provisioning",
        ServerStatusRunning:      "Running",
        ServerStatusFailed:       "Failed",
    }
    return labels[s]
}

func (s ServerStatus) IsTerminal() bool {
    return s == ServerStatusRunning || s == ServerStatusFailed
}
```

**Solution - Enum Helper Generator:**
```go
// internal/pkg/enums/helper.go
package enums

// StringEnum provides common enum methods
type StringEnum interface {
    ~string
}

// Labels returns human-readable labels for enums
type Labels[T StringEnum] map[T]string

func (l Labels[T]) Label(e T) string {
    if label, ok := l[e]; ok {
        return label
    }
    return string(e)
}

// Usage in enum file:
var serverStatusLabels = enums.Labels[ServerStatus]{
    ServerStatusNew:          "New",
    ServerStatusProvisioning: "Provisioning",
    ServerStatusRunning:      "Running",
    ServerStatusFailed:       "Failed",
}

func (s ServerStatus) Label() string {
    return serverStatusLabels.Label(s)
}

// String() is trivial - just use string(s) inline
```

**Impact:** ~90 lines saved, consistent enum handling

---

## 150. ✅ COMPLETED - Pointer Dereference Utilities

**Problem:** Scattered nil-check pointer dereference patterns throughout handlers and DTOs.

**Files affected:**
- `internal/modules/server/handlers/server_handler.go` (multiple nil checks)
- `internal/modules/site/handlers/site_handler.go` (multiple nil checks)
- `internal/modules/auth/dto/user.go:25-30` (pointer fields)
- `internal/modules/backup/dto/backup.go:20-25` (pointer fields)
- Various job files with nullable payload fields

**Current (scattered):**
```go
// Pattern 1: Inline nil check
var name string
if req.Name != nil {
    name = *req.Name
}

// Pattern 2: Conditional assignment
if user.AvatarURL != nil {
    response.AvatarURL = *user.AvatarURL
}

// Pattern 3: Default value
timezone := "UTC"
if server.Timezone != nil {
    timezone = *server.Timezone
}
```

**Solution - Pointer Helpers:**
```go
// internal/pkg/ptr/deref.go
package ptr

// Deref returns the dereferenced value or zero value
func Deref[T any](p *T) T {
    if p == nil {
        var zero T
        return zero
    }
    return *p
}

// DerefOr returns the dereferenced value or default
func DerefOr[T any](p *T, defaultVal T) T {
    if p == nil {
        return defaultVal
    }
    return *p
}

// Ptr returns a pointer to the value
func Ptr[T any](v T) *T {
    return &v
}

// Usage:
name := ptr.Deref(req.Name)
timezone := ptr.DerefOr(server.Timezone, "UTC")
response.AvatarURL = ptr.Deref(user.AvatarURL)
```

**Impact:** ~60 lines saved, cleaner pointer handling

---

## 151. ✅ COMPLETED - URL Builder Package

**Problem:** API endpoint and webhook URL construction scattered with fmt.Sprintf.

**Files affected:**
- `internal/modules/git/services/webhook_service.go:45-52` (webhook URLs)
- `internal/modules/git/providers/github/client.go:30-45` (API URLs)
- `internal/modules/git/providers/gitlab/client.go:28-42` (API URLs)
- `internal/modules/git/providers/bitbucket/client.go:32-48` (API URLs)
- `internal/modules/server/providers/digitalocean/client.go:25-40` (API URLs)
- `internal/modules/server/providers/hetzner/client.go:22-38` (API URLs)
- `internal/modules/site/services/deployment_service.go:80-95` (callback URLs)

**Current (scattered):**
```go
// In webhook service
url := fmt.Sprintf("%s/api/webhooks/github/%s", config.AppURL, site.ID)

// In GitHub provider
url := fmt.Sprintf("https://api.github.com/repos/%s/hooks", repo)

// In deployment service  
callbackURL := fmt.Sprintf("%s/api/tasks/%s/callback", config.AppURL, taskID)
```

**Solution - URL Builder:**
```go
// internal/pkg/urlbuilder/builder.go
package urlbuilder

import (
    "fmt"
    "net/url"
    "path"
)

type Builder struct {
    base string
}

func New(baseURL string) *Builder {
    return &Builder{base: baseURL}
}

func (b *Builder) Path(segments ...string) string {
    u, _ := url.Parse(b.base)
    u.Path = path.Join(append([]string{u.Path}, segments...)...)
    return u.String()
}

func (b *Builder) Pathf(format string, args ...any) string {
    return b.Path(fmt.Sprintf(format, args...))
}

// API-specific builders
var (
    GitHub    = New("https://api.github.com")
    GitLab    = New("https://gitlab.com/api/v4")
    Bitbucket = New("https://api.bitbucket.org/2.0")
)

// App URL builder (from config)
func App() *Builder {
    return New(config.Get().AppURL)
}

// Usage:
url := urlbuilder.GitHub.Path("repos", repo, "hooks")
callbackURL := urlbuilder.App().Path("api", "tasks", taskID, "callback")
webhookURL := urlbuilder.App().Path("api", "webhooks", "github", site.ID)
```

**Impact:** ~100 lines saved, consistent URL construction

---

## 152. ✅ COMPLETED - Generic UpdateStatus Repository Method

**Problem:** Identical UpdateStatus methods in 5+ repositories.

**Files affected:**
- `internal/modules/server/repositories/server_repository.go:85-92`
- `internal/modules/site/repositories/site_repository.go:78-85`
- `internal/modules/site/repositories/deployment_repository.go:65-72`
- `internal/modules/database/repositories/database_repository.go:55-62`
- `internal/modules/backup/repositories/backup_repository.go:48-55`

**Current (repeated 5+ times):**
```go
func (r *ServerRepository) UpdateStatus(ctx context.Context, id string, status enums.ServerStatus) error {
    return r.DB().WithContext(ctx).
        Model(&models.Server{}).
        Where("id = ?", id).
        Update("status", status).Error
}

func (r *SiteRepository) UpdateStatus(ctx context.Context, id string, status enums.SiteStatus) error {
    return r.DB().WithContext(ctx).
        Model(&models.Site{}).
        Where("id = ?", id).
        Update("status", status).Error
}
```

**Solution - Generic Repository Method:**
```go
// internal/pkg/repository/status.go
package repository

import (
    "context"
    "gorm.io/gorm"
)

// StatusUpdater provides generic status update
type StatusUpdater[M any, S ~string] struct {
    db func() *gorm.DB
}

func NewStatusUpdater[M any, S ~string](dbFn func() *gorm.DB) *StatusUpdater[M, S] {
    return &StatusUpdater[M, S]{db: dbFn}
}

func (u *StatusUpdater[M, S]) UpdateStatus(ctx context.Context, id string, status S) error {
    var model M
    return u.db().WithContext(ctx).
        Model(&model).
        Where("id = ?", id).
        Update("status", status).Error
}

// Embed in repository:
type ServerRepository struct {
    BaseRepository[models.Server]
    *repository.StatusUpdater[models.Server, enums.ServerStatus]
}

func NewServerRepository(db *gorm.DB) *ServerRepository {
    base := NewBaseRepository[models.Server](db)
    return &ServerRepository{
        BaseRepository:  base,
        StatusUpdater:   repository.NewStatusUpdater[models.Server, enums.ServerStatus](base.DB),
    }
}
```

**Impact:** ~80 lines saved, type-safe status updates

---

## 153. Status Enum Base Generator (P2)

**Problem:** All status enums have identical structure with Values(), IsValid(), terminal states.

**Files affected:**
- `internal/modules/server/enums/server_status.go:50-75`
- `internal/modules/site/enums/site_status.go:45-70`
- `internal/modules/site/enums/deployment_status.go:40-65`
- `internal/modules/database/enums/database_status.go:35-58`
- `internal/modules/backup/enums/backup_status.go:30-52`
- `internal/modules/billing/enums/subscription_status.go:38-60`

**Current (repeated per enum):**
```go
func (s ServerStatus) Values() []ServerStatus {
    return []ServerStatus{
        ServerStatusNew,
        ServerStatusProvisioning,
        ServerStatusRunning,
        ServerStatusFailed,
    }
}

func (s ServerStatus) IsValid() bool {
    for _, v := range s.Values() {
        if s == v {
            return true
        }
    }
    return false
}

func (s ServerStatus) IsTerminal() bool {
    return s == ServerStatusRunning || s == ServerStatusFailed
}
```

**Solution - Enum Set Helper:**
```go
// internal/pkg/enums/set.go
package enums

// Set provides common enum operations
type Set[T comparable] struct {
    values   []T
    terminal map[T]bool
}

func NewSet[T comparable](values []T, terminal ...T) *Set[T] {
    s := &Set[T]{
        values:   values,
        terminal: make(map[T]bool),
    }
    for _, t := range terminal {
        s.terminal[t] = true
    }
    return s
}

func (s *Set[T]) Values() []T        { return s.values }
func (s *Set[T]) IsValid(v T) bool   { 
    for _, val := range s.values {
        if v == val { return true }
    }
    return false
}
func (s *Set[T]) IsTerminal(v T) bool { return s.terminal[v] }

// Usage:
var serverStatusSet = enums.NewSet(
    []ServerStatus{ServerStatusNew, ServerStatusProvisioning, ServerStatusRunning, ServerStatusFailed},
    ServerStatusRunning, ServerStatusFailed, // terminal states
)

func (s ServerStatus) IsValid() bool    { return serverStatusSet.IsValid(s) }
func (s ServerStatus) IsTerminal() bool { return serverStatusSet.IsTerminal(s) }
```

**Impact:** ~120 lines saved, declarative enum definitions

---

## 154. ✅ COMPLETED - Preload Scope Helpers

**Problem:** Identical preload chains repeated across repository methods.

**Files affected:**
- `internal/modules/backup/repositories/backup_repository.go:57-61,79-83,101-105` (3 identical)
- `internal/modules/site/repositories/site_repository.go:40-48,65-73` (2 identical)
- `internal/modules/server/repositories/server_repository.go:35-42,58-65` (2 identical)
- `internal/modules/git/repositories/source_control_repository.go:30-38,52-60` (2 identical)

**Current (backup repo - 3 EXACT duplicates):**
```go
// Lines 57-61, 79-83, 101-105 - IDENTICAL
.Preload("Jobs", func(db *gorm.DB) *gorm.DB {
    return db.Order("created_at DESC").Limit(50)
}).
.Preload("StorageProvider").
.Preload("Databases")
```

**Solution - Preload Scopes:**
```go
// internal/pkg/repository/preload.go
package repository

import "gorm.io/gorm"

// PreloadScope is a reusable preload configuration
type PreloadScope func(*gorm.DB) *gorm.DB

// Common preload scopes
func PreloadAll(scopes ...PreloadScope) func(*gorm.DB) *gorm.DB {
    return func(db *gorm.DB) *gorm.DB {
        for _, scope := range scopes {
            db = scope(db)
        }
        return db
    }
}

func PreloadOrdered(relation, orderBy string, limit int) PreloadScope {
    return func(db *gorm.DB) *gorm.DB {
        return db.Preload(relation, func(tx *gorm.DB) *gorm.DB {
            return tx.Order(orderBy).Limit(limit)
        })
    }
}

func Preload(relations ...string) PreloadScope {
    return func(db *gorm.DB) *gorm.DB {
        for _, rel := range relations {
            db = db.Preload(rel)
        }
        return db
    }
}

// Usage in backup repository:
var backupPreloads = repository.PreloadAll(
    repository.PreloadOrdered("Jobs", "created_at DESC", 50),
    repository.Preload("StorageProvider", "Databases"),
)

func (r *BackupRepository) FindByID(ctx context.Context, id string) (*models.Backup, error) {
    var backup models.Backup
    err := r.DB().WithContext(ctx).
        Scopes(backupPreloads).
        First(&backup, "id = ?", id).Error
    return &backup, err
}
```

**Impact:** ~90 lines saved, DRY preload configurations

---

## 155. Soft Delete Scope Unification (P2)

**Problem:** Mixed ArchivedAt and DeletedAt approaches with inconsistent filtering.

**Files affected:**
- `internal/modules/server/models/server.go:45` (ArchivedAt)
- `internal/modules/site/models/site.go:38` (ArchivedAt)
- `internal/modules/auth/models/team.go:28` (DeletedAt - GORM default)
- `internal/modules/backup/models/backup.go:22` (no soft delete)
- `internal/modules/server/repositories/server_repository.go:95-98` (manual WHERE)
- `internal/modules/site/repositories/site_repository.go:88-91` (manual WHERE)

**Current (inconsistent):**
```go
// Some models use ArchivedAt
type Server struct {
    ArchivedAt *time.Time `json:"archived_at,omitempty"`
}

// Manual filtering required
func (r *ServerRepository) FindActive(ctx context.Context) ([]models.Server, error) {
    var servers []models.Server
    err := r.DB().WithContext(ctx).
        Where("archived_at IS NULL").  // Manual!
        Find(&servers).Error
    return servers, err
}

// Other models use GORM's DeletedAt
type Team struct {
    gorm.DeletedAt
}
// GORM auto-filters
```

**Solution - Unified Archive Mixin:**
```go
// internal/pkg/models/archive.go
package models

import (
    "time"
    "gorm.io/gorm"
)

// ArchiveMixin provides consistent soft-delete via archiving
type ArchiveMixin struct {
    ArchivedAt *time.Time `gorm:"index" json:"archived_at,omitempty"`
}

func (m *ArchiveMixin) IsArchived() bool {
    return m.ArchivedAt != nil
}

func (m *ArchiveMixin) Archive() {
    now := time.Now()
    m.ArchivedAt = &now
}

func (m *ArchiveMixin) Restore() {
    m.ArchivedAt = nil
}

// Scope for active records
func ActiveScope(db *gorm.DB) *gorm.DB {
    return db.Where("archived_at IS NULL")
}

// Scope for archived records
func ArchivedScope(db *gorm.DB) *gorm.DB {
    return db.Where("archived_at IS NOT NULL")
}

// Usage in repository:
func (r *ServerRepository) FindActive(ctx context.Context) ([]models.Server, error) {
    var servers []models.Server
    err := r.DB().WithContext(ctx).
        Scopes(models.ActiveScope).
        Find(&servers).Error
    return servers, err
}
```

**Impact:** ~50 lines saved, consistent archiving

---

## 156. Site Type Helper Methods (P2)

**Problem:** Scattered site type checks throughout the codebase.

**Files affected:**
- `internal/modules/site/jobs/deploy_site.go:45-52` (IsLaravel check)
- `internal/modules/site/jobs/enable_laravel_queue.go:30-35` (IsLaravel check)
- `internal/modules/site/services/deployment_service.go:88-95` (multiple type checks)
- `internal/modules/site/handlers/site_handler.go:120-128` (type validation)
- `internal/modules/site/tasks/deploy.go:60-75` (feature-by-type logic)

**Current (scattered):**
```go
// Repeated pattern
if site.Type == enums.SiteTypeLaravel {
    // Laravel-specific logic
}

// Or with multiple checks
if site.Type == enums.SiteTypeLaravel || site.Type == enums.SiteTypeLaravelZeroDowntime {
    // Laravel-like logic
}

// Feature checks
supportsQueue := site.Type == enums.SiteTypeLaravel
supportsScheduler := site.Type == enums.SiteTypeLaravel
supportsHorizon := site.Type == enums.SiteTypeLaravel
```

**Solution - Site Type Helpers:**
```go
// internal/modules/site/enums/site_type_helpers.go
package enums

// Helper methods on SiteType
func (t SiteType) IsLaravel() bool {
    return t == SiteTypeLaravel || t == SiteTypeLaravelZeroDowntime
}

func (t SiteType) IsPHP() bool {
    return t.IsLaravel() || t == SiteTypeWordPress || t == SiteTypePHP
}

func (t SiteType) IsStatic() bool {
    return t == SiteTypeStatic || t == SiteTypeNextJS || t == SiteTypeNuxt
}

func (t SiteType) SupportsZeroDowntime() bool {
    return t == SiteTypeLaravelZeroDowntime
}

// Feature support helpers
func (t SiteType) SupportsQueue() bool      { return t.IsLaravel() }
func (t SiteType) SupportsScheduler() bool  { return t.IsLaravel() }
func (t SiteType) SupportsHorizon() bool    { return t.IsLaravel() }
func (t SiteType) SupportsMigrations() bool { return t.IsLaravel() }

// Usage becomes:
if site.Type.IsLaravel() {
    // Laravel-specific logic
}

if site.Type.SupportsQueue() {
    // Enable queue features
}
```

**Impact:** ~80 lines saved, centralized type logic

---

## 157. Feature Enable Base Job (P1)

**Problem:** 4 Laravel feature enable jobs have 90% identical structure.

**Files affected:**
- `internal/modules/site/jobs/enable_laravel_queue.go` (~150 lines)
- `internal/modules/site/jobs/enable_laravel_scheduler.go` (~150 lines)
- `internal/modules/site/jobs/enable_laravel_horizon.go` (~150 lines)
- `internal/modules/site/jobs/enable_laravel_inertia.go` (~140 lines)

**Current (near-identical jobs):**
```go
// enable_laravel_queue.go
type EnableLaravelQueueJob struct {
    ctx     *jobs.Context
    Payload EnableQueuePayload
}

func (j *EnableLaravelQueueJob) Handle() error {
    site, err := j.loadSite()
    if err != nil {
        return err
    }
    
    server, err := j.loadServer(site.ServerID)
    if err != nil {
        return err
    }
    
    j.broadcastStart(site)
    
    task := tasks.EnableLaravelQueue(site, server)
    result := j.ctx.TaskRunner.Execute(task, server)
    
    if result.Error != nil {
        j.broadcastFailed(site, result.Error)
        return result.Error
    }
    
    j.updateSiteFeature(site, "queue_enabled", true)
    j.broadcastSuccess(site)
    
    return nil
}
```

**Solution - Base Feature Enable Job:**
```go
// internal/modules/site/jobs/base_feature_job.go
package jobs

import (
    "context"
)

// FeatureConfig defines a feature to enable
type FeatureConfig struct {
    Name        string  // "queue", "scheduler", "horizon"
    DBField     string  // "queue_enabled", "scheduler_enabled"
    TaskBuilder func(site *models.Site, server *models.Server) taskrunner.Task
}

// BaseFeatureJob handles common feature enable logic
type BaseFeatureJob struct {
    ctx     *Context
    config  FeatureConfig
    payload BaseFeaturePayload
}

type BaseFeaturePayload struct {
    SiteID string `json:"site_id"`
}

func (j *BaseFeatureJob) Handle() error {
    site, err := j.loadSite()
    if err != nil {
        return err
    }
    
    server, err := j.loadServer(site.ServerID)
    if err != nil {
        return err
    }
    
    j.broadcastStart(site)
    
    task := j.config.TaskBuilder(site, server)
    result := j.ctx.TaskRunner.Execute(task, server)
    
    if result.Error != nil {
        j.broadcastFailed(site, result.Error)
        return result.Error
    }
    
    j.updateFeature(site, j.config.DBField, true)
    j.broadcastSuccess(site)
    
    return nil
}

// Specific jobs become minimal:
func NewEnableLaravelQueueJob(ctx *Context, payload EnableQueuePayload) *BaseFeatureJob {
    return &BaseFeatureJob{
        ctx: ctx,
        config: FeatureConfig{
            Name:        "queue",
            DBField:     "queue_enabled",
            TaskBuilder: tasks.EnableLaravelQueue,
        },
        payload: BaseFeaturePayload{SiteID: payload.SiteID},
    }
}
```

**Impact:** ~400 lines saved, feature jobs become config

---

## 158. Broadcast Event Builder (P2)

**Problem:** Identical nil-check wrapper methods in 3+ files for WebSocket broadcasting.

**Files affected:**
- `internal/modules/site/services/base.go:45-55` (3 wrapper methods)
- `internal/modules/site/jobs/context.go:60-72` (3 wrapper methods)
- `internal/pkg/taskrunner/callback.go:80-92` (3 wrapper methods)

**Current (repeated in 3 files):**
```go
func (s *Base) BroadcastToTeam(teamID, event string, data interface{}) {
    if s.WS == nil {
        return
    }
    s.WS.BroadcastToTeam(teamID, event, data)
}

func (s *Base) BroadcastToUser(userID, event string, data interface{}) {
    if s.WS == nil {
        return
    }
    s.WS.BroadcastToUser(userID, event, data)
}

func (s *Base) BroadcastToChannel(channel, event string, data interface{}) {
    if s.WS == nil {
        return
    }
    s.WS.BroadcastToChannel(channel, event, data)
}
```

**Solution - Broadcast Mixin:**
```go
// internal/pkg/broadcast/mixin.go
package broadcast

import "launch/internal/pkg/websocket"

// Mixin provides nil-safe broadcasting methods
type Mixin struct {
    WS *websocket.Hub
}

func (m *Mixin) BroadcastToTeam(teamID, event string, data interface{}) {
    if m.WS == nil {
        return
    }
    m.WS.BroadcastToTeam(teamID, event, data)
}

func (m *Mixin) BroadcastToUser(userID, event string, data interface{}) {
    if m.WS == nil {
        return
    }
    m.WS.BroadcastToUser(userID, event, data)
}

func (m *Mixin) BroadcastToChannel(channel, event string, data interface{}) {
    if m.WS == nil {
        return
    }
    m.WS.BroadcastToChannel(channel, event, data)
}

// Event builder for common patterns
type Event struct {
    Channel string
    Name    string
    Data    interface{}
}

func (m *Mixin) Broadcast(e Event) {
    m.BroadcastToChannel(e.Channel, e.Name, e.Data)
}

// Usage - embed in services/jobs:
type DeploymentService struct {
    broadcast.Mixin
    // ...
}

// Or with event builder:
s.Broadcast(broadcast.Event{
    Channel: fmt.Sprintf("team.%s", teamID),
    Name:    "deployment.started",
    Data:    map[string]any{"deployment_id": id},
})
```

**Impact:** ~60 lines saved, consistent broadcasting

---

## Extended Summary (Items 147-158)

| Category | Items | Est. LOC Saved |
|----------|-------|----------------|
| BeforeCreate Hook Consolidation | Item 147 | ~80 lines |
| Generic Slice Mapper | Item 148 | ~120 lines |
| Enum Response Helper | Item 149 | ~90 lines |
| Pointer Dereference Utilities | Item 150 | ~60 lines |
| URL Builder Package | Item 151 | ~100 lines |
| Generic UpdateStatus Repository | Item 152 | ~80 lines |
| Status Enum Base Generator | Item 153 | ~120 lines |
| Preload Scope Helpers | Item 154 | ~90 lines |
| Soft Delete Scope Unification | Item 155 | ~50 lines |
| Site Type Helper Methods | Item 156 | ~80 lines |
| Feature Enable Base Job | Item 157 | ~400 lines |
| Broadcast Event Builder | Item 158 | ~60 lines |
| **Round 11 Total** | **12 patterns** | **~1330 lines** |

---

## Updated Final Summary

| Category | Items | Total LOC Saved |
|----------|-------|-----------------|
| Handler Bases | 5 patterns | ~400 lines |
| Repository Patterns | 5 patterns | ~1200 lines |
| Service Patterns | 3 patterns | ~450 lines |
| Job/Task Patterns | 3 patterns | ~800 lines |
| Model Mixins | 5 patterns | ~270 lines |
| Provider API Clients | 3 patterns | ~450 lines |
| Middleware Patterns | 1 pattern | ~30 lines |
| Config Patterns | 1 pattern | ~86 lines |
| DTO/Request Patterns | 2 patterns | ~550 lines |
| Testing Patterns | 2 patterns | ~250 lines |
| Infrastructure Patterns | 6 patterns | ~1075 lines |
| Validation & Response | 5 patterns | ~610 lines |
| Template & Script | 3 patterns | ~160 lines |
| Interface & Query | 4 patterns | ~500 lines |
| Security & Events | 3 patterns | ~100 lines |
| HTTP & Helpers (Round 3) | 12 patterns | ~1790 lines |
| Error & DI (Round 4) | 12 patterns | ~1320 lines |
| GORM & Jobs (Round 5) | 12 patterns | ~2290 lines |
| Context & Auth (Round 6) | 12 patterns | ~2560 lines |
| Validation & Enums (Round 7) | 12 patterns | ~1805 lines |
| Routes & Crypto (Round 8) | 12 patterns | ~3615 lines |
| HTTP Client & Config (Round 9) | 12 patterns | ~2480 lines |
| Notification & Jobs (Round 10) | 12 patterns | ~2020 lines |
| Model Hooks & DTO (Round 11) | 12 patterns | ~1330 lines |
| **Grand Total** | **158 patterns** | **~26,141 lines** |

---

# CONSOLIDATED IMPLEMENTATION PLAN

This section analyzes the 158 patterns above and groups them into **15 actionable work packages** that can be implemented incrementally. Patterns within each package build upon each other and should be done together.

---

## Work Package Overview

| # | Package Name | Items | Est. LOC Saved | Priority | Effort |
|---|--------------|-------|----------------|----------|--------|
| 1 | Handler Infrastructure | 15 patterns | ~1,200 lines | P1 | High |
| 2 | Repository Infrastructure | 14 patterns | ~1,800 lines | P1 | High |
| 3 | Job/Task Infrastructure | 12 patterns | ~1,500 lines | P1 | Medium |
| 4 | Model Mixins | 11 patterns | ~600 lines | P2 | Medium |
| 5 | DTO/Response Infrastructure | 13 patterns | ~1,100 lines | P1 | Medium |
| 6 | HTTP Client Infrastructure | 11 patterns | ~900 lines | P1 | Medium |
| 7 | Template/Script Engine | 9 patterns | ~800 lines | P2 | Medium |
| 8 | Notification/Webhook | 10 patterns | ~600 lines | P2 | Low |
| 9 | Enum/Type Infrastructure | 8 patterns | ~500 lines | P2 | Low |
| 10 | Security/Crypto | 7 patterns | ~350 lines | P1 | Low |
| 11 | Broadcast/WebSocket | 9 patterns | ~500 lines | P2 | Medium |
| 12 | Config/Constants | 6 patterns | ~300 lines | P3 | Low |
| 13 | Error Handling | 5 patterns | ~200 lines | P1 | Low |
| 14 | Testing Infrastructure | 4 patterns | ~300 lines | P3 | Low |
| 15 | Utility Helpers | 24 patterns | ~800 lines | P3 | Low |
| **Total** | | **158 patterns** | **~11,450 lines** | | |

---

## Work Package 1: Handler Infrastructure (P1)

**Goal:** Create a unified handler base with context extraction, validation, and error handling.

### Patterns Consolidated:
| Item | Pattern Name | LOC Saved |
|------|--------------|-----------|
| 1 | WebSocket Handler Base Embedding | ~60 lines |
| 2 | Webhook Handler Base Embedding | ~35 lines |
| ~~5~~ | ~~Module Handler Field Consolidation~~ | ~~✅ COMPLETED~~ |
| 33 | ParseAndValidate Adoption | ~300 lines |
| 34 | Global Context Extraction Helpers | ~200 lines |
| 52 | Context Extraction Helpers | ~100 lines |
| 77 | WebSocket Handler Base Struct | ~60 lines |
| 84 | ParseAndValidate Adoption (duplicate) | (merged with 33) |
| 87 | Request Context Extraction | ~80 lines |
| 101 | Handler Validation Flow | ~80 lines |
| 111 | Route Middleware Chain | ~40 lines |
| 112 | Handler Module Aggregation | ~45 lines |

### Implementation Order:

**Step 1: Create Core Handler Base** (`internal/pkg/handler/`)
```go
// base.go - Handler base with logging and context
type Base struct {
    Logger *zerolog.Logger
}

// context.go - Safe context extraction
func GetTeamID(c *fiber.Ctx) (string, error)
func GetUserID(c *fiber.Ctx) (string, error)
func MustGetTeamID(c *fiber.Ctx) string

// request.go - Request parsing with validation
func ParseAndValidate[T any](c *fiber.Ctx) (*T, error)
func ParseQuery[T any](c *fiber.Ctx) (*T, error)

// errors.go - Service error to HTTP response mapping
func HandleServiceError(c *fiber.Ctx, err error) error
```

**Step 2: Create WebSocket Handler Base** (`internal/modules/websocket/handlers/base.go`)
```go
type BaseWSHandler struct {
    handler.Base
    DB              *gorm.DB
    JWTSecret       string
    MembershipCache *cache.TeamMembershipCache
}
```

**Step 3: Create Webhook Handler Base** (`internal/pkg/webhook/base.go`)
```go
type BaseWebhookHandler struct {
    handler.Base
    Signer *signedurl.Signer
}

func (h *BaseWebhookHandler) VerifySignature(c *fiber.Ctx) bool
func (h *BaseWebhookHandler) LogWebhookReceived(c *fiber.Ctx, webhookType string)
```

**Step 4: Refactor All Module Handlers**
- Embed `handler.Base` in all HTTP handlers
- Embed `BaseWSHandler` in WebSocket handlers
- Embed `BaseWebhookHandler` in webhook handlers
- Replace raw `c.Locals()` with safe extractors
- Replace BodyParser+Validate with `ParseAndValidate`

### Files to Create:
- [x] `internal/pkg/handler/base.go` (previously created)
- [x] `internal/pkg/handler/context.go` (previously created)
- [x] `internal/pkg/handler/request.go` (previously created)
- [x] `internal/pkg/handler/errors.go` (previously created)
- [x] `internal/pkg/webhook/base.go` ✅ COMPLETED
- [x] `internal/modules/websocket/handlers/base.go` ✅ COMPLETED

### Estimated Effort: 2-3 days

---

## Work Package 2: Repository Infrastructure (P2)

**Goal:** Create unified repository base with generic query builders, scopes, and common methods.

### Patterns Consolidated:
| Item | Pattern Name | LOC Saved |
|------|--------------|-----------|
| 3 | Repository BaseRepository Enforcement | ~450 lines |
| 7 | Installable Repository Mixin | ~200 lines |
| 39 | Backup Repository Many-to-Many Pattern | ~50 lines |
| 45 | GORM Query Helper Methods | ~300 lines |
| 50 | N+1 Query Fix | ~50 lines |
| 75 | GORM Query Scope - Order By Latest | ~40 lines |
| 88 | Repository Authorization Method | ~80 lines |
| 89 | Preload Chain Consolidation | ~60 lines |
| 90 | Service BaseService Accessor | ~40 lines |
| 96 | ServiceRegistry Boilerplate | ~30 lines |
| 107 | Repository Registry Factory | ~50 lines |
| 152 | Generic UpdateStatus Repository | ~80 lines |
| 154 | Preload Scope Helpers | ~90 lines |
| 155 | Soft Delete Scope Unification | ~50 lines |

### Implementation Order:

**Step 1: Enhance Repository Base** (`internal/pkg/repository/`)
```go
// base.go - Repository base with common methods
type Base[T any] struct {
    db *gorm.DB
}

func (r *Base[T]) DB() *gorm.DB
func (r *Base[T]) FindByID(ctx context.Context, id string) (*T, error)
func (r *Base[T]) Create(ctx context.Context, entity *T) error
func (r *Base[T]) Update(ctx context.Context, entity *T) error
func (r *Base[T]) Delete(ctx context.Context, id string) error

// status.go - Generic status update
func (r *Base[T]) UpdateStatus(ctx context.Context, id string, status any) error

// query.go - Fluent query builder
type Query[T any] struct { ... }
func (q *Query[T]) Where(cond string, args ...any) *Query[T]
func (q *Query[T]) Preload(rel string) *Query[T]
func (q *Query[T]) OrderByLatest() *Query[T]
func (q *Query[T]) First(ctx context.Context) (*T, error)
func (q *Query[T]) All(ctx context.Context) ([]T, error)
```

**Step 2: Create Scope Library** (`internal/pkg/repository/scopes.go`)
```go
// Already exists - extend with:
func PreloadOrdered(rel, order string, limit int) Scope
func WithActive() Scope  // archived_at IS NULL
func WithArchived() Scope
func JoinLatest(rel string) Scope
```

**Step 3: Create Authorization Mixin**
```go
// auth.go - Authorization helpers
type Authorizable[T any] struct {
    Base[T]
}

func (r *Authorizable[T]) FindByIDAndTeam(ctx context.Context, id, teamID string) (*T, error)
func (r *Authorizable[T]) FindByIDAndServer(ctx context.Context, id, serverID string) (*T, error)
```

**Step 4: Create Registry Factory**
```go
// registry.go - Already exists, enhance with:
func NewRegistry[R any](db *gorm.DB, factory func(*gorm.DB) R) *Registry[R]
```

### Files to Modify/Create:
- [x] Enhance `internal/pkg/repository/base.go` - Added FindByIDAndTeam, FindByIDAndTeamOrFail, UpdateStatus, UpdateStatusByServer
- [x] Enhance `internal/pkg/repository/scopes.go` - Added PreloadOrdered, PreloadWithScope, WithActive, WithArchived, OrderByLatest, OrderByOldest
- [x] Existing `internal/pkg/repository/query.go` - Already has fluent Query builder
- [x] `FindByIDAndTeam` added to base.go (no separate auth.go needed)
- [x] `UpdateStatus` added to base.go (no separate status.go needed)

### Refactored Files:
- [x] `internal/modules/server/repositories/server_repository.go` - Uses WithActive(), WithTeamID() scopes
- [x] `internal/modules/dashboard/services/dashboard_service.go` - Uses WithActive(), WithTeamID() scopes

### Estimated Effort: 2-3 days

---

## Work Package 3: Job/Task Infrastructure (P1)

**Goal:** Unified job context, task builders, and callback handling.

### Patterns Consolidated:
| Item | Pattern Name | LOC Saved |
|------|--------------|-----------|
| 6 | Job Base Payload Generic | ~100 lines |
| 10 | Task Builder Pattern | ~600 lines |
| 71 | JobContext Generic Base | ~80 lines |
| 79 | Job Struct Boilerplate | ~150 lines |
| 93 | JobContext Field Duplication | ~60 lines |
| 139 | Job Task Builder Generator | ~400 lines |
| 140 | Job Context Base Consolidation | ~250 lines |
| 142 | Backup Job Payload Base | ~30 lines |
| 143 | Service Dispatch Helper | ~50 lines |
| 157 | Feature Enable Base Job | ~400 lines |

### Implementation Order:

**Step 1: Create Generic Job Base** (`internal/pkg/jobs/`)
```go
// base.go - Job base with context
type Base struct {
    ctx *Context
}

func (j *Base) Context() *Context
func (j *Base) DB() *gorm.DB
func (j *Base) Queue() *queue.Client
func (j *Base) Dispatcher() *taskrunner.Dispatcher
func (j *Base) LogInfo(msg string, fields ...any)
func (j *Base) LogError(msg string, fields ...any)
func (j *Base) Broadcast(channel, event string, data any)

// payload.go - Generic payload handling
type BasePayload struct {
    TeamID   string `json:"team_id"`
    UserID   string `json:"user_id,omitempty"`
}

// builder.go - Task builder helper
type TaskBuilder struct { ... }
func (b *TaskBuilder) WithScript(script string) *TaskBuilder
func (b *TaskBuilder) WithTimeout(seconds int) *TaskBuilder
func (b *TaskBuilder) Build() taskrunner.Task
```

**Step 2: Create Feature Job Template**
```go
// feature_job.go - Base for feature enable jobs
type FeatureJob[P any] struct {
    Base
    Payload P
    Config  FeatureConfig
}

type FeatureConfig struct {
    Name        string
    DBField     string
    TaskBuilder func(*models.Site, *models.Server) taskrunner.Task
}

func (j *FeatureJob[P]) Handle() error {
    // Standard feature enable flow
}
```

**Step 3: Refactor Module Jobs**
- Embed `jobs.Base` in all job structs
- Use `TaskBuilder` for task creation
- Convert feature enable jobs to `FeatureJob`

### Files to Create:
- [ ] Enhance `internal/pkg/jobs/base.go`
- [ ] Create `internal/pkg/jobs/payload.go`
- [ ] Create `internal/pkg/jobs/builder.go`
- [ ] Create `internal/pkg/jobs/feature_job.go`

### Estimated Effort: 2 days

---

## Work Package 4: Model Mixins (P2)

**Goal:** Composable model mixins for common patterns.

### Patterns Consolidated:
| Item | Pattern Name | LOC Saved |
|------|--------------|-----------|
| 11 | Model Scoped Fields Adoption | ~100 lines |
| 15 | Model NamedModel Mixin | ~26 lines |
| 16 | Model StateTrackingModel Mixin | ~30 lines |
| 17 | Model ProgressTrackingModel Mixin | ~10 lines |
| 18 | Compound Scoping Mixins | ~100 lines |
| 57 | BeforeCreate Hook Consolidation | ~60 lines |
| 97 | Soft Delete Pattern | ~40 lines |
| 105 | BeforeCreate Hook Status Init | ~50 lines |
| 147 | BeforeCreate Hook Consolidation (R11) | ~80 lines |
| 155 | Soft Delete Scope Unification | ~50 lines |
| 156 | Site Type Helper Methods | ~80 lines |

### Implementation Order:

**Step 1: Create Model Mixins** (`internal/pkg/models/`)
```go
// mixins.go - Composable mixins
type Named struct {
    Name string `gorm:"size:255;not null" json:"name"`
}

type Described struct {
    Description *string `gorm:"type:text" json:"description,omitempty"`
}

type StatusTracking[S ~string] struct {
    Status S `gorm:"size:50;not null" json:"status"`
}

type ProgressTracking struct {
    Progress   int     `gorm:"default:0" json:"progress"`
    ProgressMessage *string `json:"progress_message,omitempty"`
}

type Archivable struct {
    ArchivedAt *time.Time `gorm:"index" json:"archived_at,omitempty"`
}

func (a *Archivable) IsArchived() bool
func (a *Archivable) Archive()
func (a *Archivable) Restore()

// hooks.go - BeforeCreate helpers
type StatusInitializer[S ~string] interface {
    DefaultStatus() S
}

func InitializeStatus[T StatusInitializer[S], S ~string](model *T) {
    // Set default status if empty
}

type TokenInitializer interface {
    TokenField() *string
}

func InitializeToken(model TokenInitializer) {
    // Generate secure token if empty
}
```

**Step 2: Create Scope Mixins**
```go
// scopes.go - Model-aware scopes
type TeamScoped struct {
    TeamID string `gorm:"size:26;not null;index" json:"team_id"`
}

type ServerScoped struct {
    ServerID string `gorm:"size:26;not null;index" json:"server_id"`
}

type SiteScoped struct {
    SiteID string `gorm:"size:26;not null;index" json:"site_id"`
}

// Compound scopes
type TeamServerScoped struct {
    TeamScoped
    ServerScoped
}
```

### Files to Create:
- [ ] Create `internal/pkg/models/mixins.go`
- [ ] Create `internal/pkg/models/hooks.go`
- [ ] Create `internal/pkg/models/scopes.go`

### Estimated Effort: 1-2 days

---

## Work Package 5: DTO/Response Infrastructure (P1)

**Goal:** Generic DTO helpers for timestamps, mapping, and pointer handling.

### Patterns Consolidated:
| Item | Pattern Name | LOC Saved |
|------|--------------|-----------|
| 14 | DTO Timestamp Embedding | ~400 lines |
| 24 | Request DTO Mixins | ~150 lines |
| 42 | Timestamp Formatting | ~40 lines |
| 58 | JSON Timestamp Helper | ~50 lines |
| 81 | DTO Timestamp Formatter | ~60 lines |
| 82 | DTO List Transformation | ~80 lines |
| 100 | Validation Field Embedding | ~40 lines |
| 103 | Model Broadcast Payload | ~50 lines |
| 108 | Slice Transformation Generic | ~60 lines |
| 115 | Nil-Safe Dereference Helpers | ~80 lines |
| 148 | Generic Slice Mapper | ~120 lines |
| 149 | Enum Response Helper | ~90 lines |
| 150 | Pointer Dereference Utilities | ~60 lines |

### Implementation Order:

**Step 1: Create DTO Helpers** (`internal/pkg/dto/`)
```go
// time.go - Already exists, enhance
func FormatTime(t *time.Time) *string
func FormatTimeValue(t time.Time) string
func ParseTime(s string) (*time.Time, error)

// ptr.go - Pointer utilities
func Deref[T any](p *T) T
func DerefOr[T any](p *T, defaultVal T) T
func Ptr[T any](v T) *T
func NilIfEmpty(s *string) *string

// mapper.go - Slice mapping
func MapSlice[T, R any](items []T, fn func(T) R) []R
func MapSlicePtr[T, R any](items []T, fn func(T) *R) []*R
func FilterSlice[T any](items []T, fn func(T) bool) []T

// enum.go - Enum response helpers
type EnumResponse struct {
    Value string `json:"value"`
    Label string `json:"label"`
}

func EnumToResponse[E ~string](e E, labelFn func(E) string) EnumResponse
func EnumsToResponses[E ~string](values []E, labelFn func(E) string) []EnumResponse
```

**Step 2: Create Response Mixins**
```go
// response.go - Common response fields
type TimestampResponse struct {
    CreatedAt string  `json:"created_at"`
    UpdatedAt string  `json:"updated_at"`
}

type AuditResponse struct {
    TimestampResponse
    CreatedBy *string `json:"created_by,omitempty"`
    UpdatedBy *string `json:"updated_by,omitempty"`
}

func NewTimestampResponse(createdAt, updatedAt time.Time) TimestampResponse
```

**Step 3: Create Request Mixins**
```go
// request.go - Common request fields
type PaginationRequest struct {
    Page    int `query:"page" validate:"min=1"`
    PerPage int `query:"per_page" validate:"min=1,max=100"`
}

func (r *PaginationRequest) Normalize()

type SortRequest struct {
    SortBy    string `query:"sort_by"`
    SortOrder string `query:"sort_order" validate:"omitempty,oneof=asc desc"`
}
```

### Files to Create/Modify:
- [ ] Enhance `internal/pkg/dto/time.go`
- [ ] Create `internal/pkg/dto/ptr.go`
- [ ] Create `internal/pkg/dto/mapper.go`
- [ ] Create `internal/pkg/dto/enum.go`
- [ ] Create `internal/pkg/dto/response.go`
- [ ] Create `internal/pkg/dto/request.go`

### Estimated Effort: 1-2 days

---

## Work Package 6: HTTP Client Infrastructure (P1)

**Goal:** Unified HTTP client base for all API providers.

### Patterns Consolidated:
| Item | Pattern Name | LOC Saved |
|------|--------------|-----------|
| 19 | Cloud Provider BaseAPIClient | ~200 lines |
| 20 | Git Provider BaseGitAPIClient | ~150 lines |
| 51 | HTTP Client Base with Request Builder | ~200 lines |
| 80 | Cloud Provider HTTP Client | ~100 lines |
| 91 | Retry Logic Consolidation | ~100 lines |
| 92 | HTTP Client Factory | ~50 lines |
| 123 | HTTP Client Base Pattern | ~100 lines |
| 124 | API Request Builder | ~80 lines |
| 125 | Response Handler Consolidation | ~100 lines |
| 129 | Retry Strategy Consolidation | ~100 lines |
| 141 | DNS Provider HTTP Base | ~100 lines |

### Implementation Order:

**Step 1: Create HTTP Client Base** (`internal/pkg/httpclient/`)
```go
// client.go - Base HTTP client
type Client struct {
    baseURL    string
    httpClient *http.Client
    headers    map[string]string
    retryConfig RetryConfig
    logger     *zerolog.Logger
}

func New(baseURL string, opts ...Option) *Client
func (c *Client) Get(ctx context.Context, path string) (*Response, error)
func (c *Client) Post(ctx context.Context, path string, body any) (*Response, error)
func (c *Client) Put(ctx context.Context, path string, body any) (*Response, error)
func (c *Client) Delete(ctx context.Context, path string) (*Response, error)

// options.go - Client options
type Option func(*Client)
func WithTimeout(d time.Duration) Option
func WithHeader(key, value string) Option
func WithBearerToken(token string) Option
func WithRetry(config RetryConfig) Option
func WithLogger(logger *zerolog.Logger) Option

// retry.go - Retry logic
type RetryConfig struct {
    MaxRetries int
    BackoffFunc func(attempt int) time.Duration
    RetryableStatus []int
}

func ExponentialBackoff(base time.Duration) func(int) time.Duration

// request.go - Request builder
type RequestBuilder struct { ... }
func (b *RequestBuilder) WithQuery(key, value string) *RequestBuilder
func (b *RequestBuilder) WithHeader(key, value string) *RequestBuilder
func (b *RequestBuilder) WithBody(body any) *RequestBuilder
func (b *RequestBuilder) Execute(ctx context.Context) (*Response, error)
```

**Step 2: Create Provider-Specific Bases**
```go
// internal/pkg/providers/cloud/base.go
type BaseCloudClient struct {
    *httpclient.Client
}

func (c *BaseCloudClient) HandleAPIError(resp *httpclient.Response) error

// internal/pkg/providers/git/base.go
type BaseGitClient struct {
    *httpclient.Client
}

func (c *BaseGitClient) Paginate(ctx context.Context, path string, handler func([]byte) bool) error
```

### Files to Create:
- [ ] Create `internal/pkg/httpclient/client.go`
- [ ] Create `internal/pkg/httpclient/options.go`
- [ ] Create `internal/pkg/httpclient/retry.go`
- [ ] Create `internal/pkg/httpclient/request.go`
- [ ] Create `internal/pkg/httpclient/response.go`
- [ ] Create `internal/pkg/providers/cloud/base.go`
- [ ] Create `internal/pkg/providers/git/base.go`

### Estimated Effort: 2-3 days

---

## Work Package 7: Template/Script Engine (P2)

**Goal:** Unified template loading and script building.

### Patterns Consolidated:
| Item | Pattern Name | LOC Saved |
|------|--------------|-----------|
| 10 | Task Builder Pattern | (shared with WP3) |
| 30 | Script Builder Interface | ~500 lines |
| 31 | Shell Defaults Registry | ~60 lines |
| 35 | Template Loading Consolidation | ~80 lines |
| 36 | Systemd Config Script | ~60 lines |
| 37 | Home Directory Helper | ~20 lines |
| 55 | Home Directory Helper (dup) | (merged) |
| 56 | Task File Path Builder | ~40 lines |
| 65 | Template Engine Consolidation | ~100 lines |
| 114 | Template Engine Consolidation (dup) | (merged) |
| 119 | Home Directory Helper (dup) | (merged) |

### Implementation Order:

**Step 1: Create Script Builder** (`internal/pkg/script/`)
```go
// builder.go - Script builder
type Builder struct {
    sections []Section
}

type Section struct {
    Name    string
    Content string
}

func New() *Builder
func (b *Builder) ShellDefaults(strict bool) *Builder
func (b *Builder) Section(name, content string) *Builder
func (b *Builder) Function(name, body string) *Builder
func (b *Builder) Command(cmd string, args ...string) *Builder
func (b *Builder) Conditional(condition string, body *Builder) *Builder
func (b *Builder) Build() string

// paths.go - Path helpers
func HomeDir(user string) string
func SiteDir(user, site string) string
func ReleaseDir(user, site, release string) string
func SharedDir(user, site string) string
```

**Step 2: Create Template Engine**
```go
// internal/pkg/templates/engine.go
type Engine struct {
    templates *template.Template
    funcMap   template.FuncMap
}

func NewEngine() *Engine
func (e *Engine) LoadFromFS(fs embed.FS, pattern string) error
func (e *Engine) LoadFromGlob(pattern string) error
func (e *Engine) Render(name string, data any) (string, error)
func (e *Engine) MustRender(name string, data any) string

// funcs.go - Standard template functions
func CommonFuncMap() template.FuncMap  // Already exists, enhance
```

### Files to Create:
- [ ] Create `internal/pkg/script/builder.go`
- [ ] Create `internal/pkg/script/paths.go`
- [ ] Enhance `internal/pkg/taskrunner/templates/engine.go`

### Estimated Effort: 1-2 days

---

## Work Package 8: Notification/Webhook (P2)

**Goal:** Unified notification channel and webhook handling.

### Patterns Consolidated:
| Item | Pattern Name | LOC Saved |
|------|--------------|-----------|
| 21 | Notification BaseWebhookChannel | ~100 lines |
| 40 | Webhook Response Standardization | ~30 lines |
| 66 | Slack Admin Alert Base | ~80 lines |
| 67 | Notification Format Helper | ~60 lines |
| 68 | Webhook Signature Verification | ~50 lines |
| 113 | Webhook Route Standardization | ~40 lines |
| 135 | Webhook Channel Base Class | ~90 lines |
| 136 | Admin Alert Base Class | ~100 lines |
| 144 | Webhook Signature Verifier | ~80 lines |

### Implementation Order:

**Step 1: Create Webhook Channel Base** (`internal/pkg/notification/`)
```go
// channel.go - Base webhook channel
type BaseWebhookChannel struct {
    Name       string
    WebhookURL string
    httpClient *httpclient.Client
    logger     *zerolog.Logger
}

func (c *BaseWebhookChannel) Send(ctx context.Context, payload any) error
func (c *BaseWebhookChannel) FormatMessage(template string, data any) string

// slack.go - Slack-specific
type SlackChannel struct {
    BaseWebhookChannel
}

func (c *SlackChannel) SendAlert(ctx context.Context, level, title, message string) error
func (c *SlackChannel) SendBlocks(ctx context.Context, blocks []slack.Block) error

// discord.go - Discord-specific
type DiscordChannel struct {
    BaseWebhookChannel
}
```

**Step 2: Create Admin Alert System**
```go
// alert.go - Admin alerting
type AdminAlerter struct {
    channels []AlertChannel
}

type AlertChannel interface {
    SendAlert(ctx context.Context, alert Alert) error
}

type Alert struct {
    Level   AlertLevel
    Title   string
    Message string
    Fields  map[string]string
}

func (a *AdminAlerter) Alert(ctx context.Context, alert Alert) error
```

### Files to Create:
- [ ] Create `internal/pkg/notification/channel.go`
- [ ] Create `internal/pkg/notification/slack.go`
- [ ] Create `internal/pkg/notification/discord.go`
- [ ] Create `internal/pkg/notification/alert.go`

### Estimated Effort: 1 day

---

## Work Package 9: Enum/Type Infrastructure (P2)

**Goal:** Reduce enum boilerplate with generators and type helpers.

### Patterns Consolidated:
| Item | Pattern Name | LOC Saved |
|------|--------------|-----------|
| 28 | Enum Scan/Value Boilerplate | ~315 lines |
| 32 | Encrypted Field Types | ~30 lines |
| 106 | Enum Boilerplate Consolidation | ~80 lines |
| 116 | JSONMap Type Deduplication | ~50 lines |
| 117 | Enum SQL Scanner Generator | ~60 lines |
| 149 | Enum Response Helper | (shared with WP5) |
| 153 | Status Enum Base Generator | ~120 lines |

### Implementation Order:

**Step 1: Create Enum Helpers** (`internal/pkg/enums/`)
```go
// base.go - Enum base types
type StringEnum interface {
    ~string
    String() string
}

// scanner.go - GORM scanner for enums
type Scanner[E StringEnum] struct {
    Value E
}

func (s *Scanner[E]) Scan(value any) error
func (s Scanner[E]) Value() (driver.Value, error)

// set.go - Enum set with validation
type Set[E comparable] struct {
    values   []E
    terminal map[E]bool
    labels   map[E]string
}

func NewSet[E comparable](values []E) *Set[E]
func (s *Set[E]) WithTerminal(values ...E) *Set[E]
func (s *Set[E]) WithLabels(labels map[E]string) *Set[E]
func (s *Set[E]) IsValid(v E) bool
func (s *Set[E]) IsTerminal(v E) bool
func (s *Set[E]) Label(v E) string
func (s *Set[E]) Values() []E
```

**Step 2: Create Custom Types**
```go
// types.go - Reusable custom types
type JSONMap map[string]any

func (m *JSONMap) Scan(value any) error
func (m JSONMap) Value() (driver.Value, error)

type EncryptedString struct {
    value     string
    encrypted string
}

func (e *EncryptedString) Set(value string, key []byte) error
func (e *EncryptedString) Get(key []byte) (string, error)
```

### Files to Create:
- [ ] Create `internal/pkg/enums/base.go`
- [ ] Create `internal/pkg/enums/scanner.go`
- [ ] Create `internal/pkg/enums/set.go`
- [ ] Create `internal/pkg/types/json.go`
- [ ] Create `internal/pkg/types/encrypted.go`

### Estimated Effort: 1 day

---

## Work Package 10: Security/Crypto (P1)

**Goal:** Consolidated crypto utilities for tokens, signatures, and encryption.

### Patterns Consolidated:
| Item | Pattern Name | LOC Saved |
|------|--------------|-----------|
| 46 | GitLab Webhook Timing Attack Fix | ~20 lines |
| 49 | Crypto Package Consolidation | ~50 lines |
| 72 | Token Generation Consolidation | ~40 lines |
| 120 | HMAC Signature Validator | ~80 lines |
| 121 | Token Generator Consolidation | ~40 lines |
| 122 | Time Expiration Checker | ~50 lines |
| 146 | Token Generation Consolidation (dup) | (merged) |

### Implementation Order:

**Step 1: Create Crypto Package** (`internal/pkg/crypto/`)
```go
// token.go - Token generation
func SecureToken(byteLength int) (string, error)
func SecureTokenHex(byteLength int) (string, error)
func URLSafeToken(byteLength int) (string, error)

// hmac.go - HMAC signatures
func SignHMAC(message, secret []byte) []byte
func VerifyHMAC(message, signature, secret []byte) bool
func VerifyHMACConstantTime(message, signature, secret []byte) bool  // Timing-safe

// expiry.go - Expiration checking
func IsExpired(expiresAt *time.Time) bool
func ExpiresIn(duration time.Duration) time.Time
func TimeRemaining(expiresAt time.Time) time.Duration
```

### Files to Create:
- [ ] Create `internal/pkg/crypto/token.go`
- [ ] Create `internal/pkg/crypto/hmac.go`
- [ ] Create `internal/pkg/crypto/expiry.go`

### Estimated Effort: 0.5 days

---

## Work Package 11: Broadcast/WebSocket (P2)

**Goal:** Unified broadcasting and WebSocket message building.

### Patterns Consolidated:
| Item | Pattern Name | LOC Saved |
|------|--------------|-----------|
| 8 | Queue Dispatch Unification | ~100 lines |
| 12 | Activity Logging Builder | ~150 lines |
| 13 | Broadcast Payload Builder | ~50 lines |
| 47 | Broadcast Event Constants | ~60 lines |
| 61 | Broadcast Channel Builder | ~40 lines |
| 78 | WebSocket Message Builder | ~50 lines |
| 102 | Broadcaster Interface Dedup | ~30 lines |
| 158 | Broadcast Event Builder | ~60 lines |

### Implementation Order:

**Step 1: Create Broadcast Helpers** (`internal/pkg/broadcast/`)
```go
// events.go - Event constants
const (
    EventCreated = "created"
    EventUpdated = "updated"
    EventDeleted = "deleted"
    EventProgress = "progress"
    EventStatus = "status"
)

// channels.go - Channel builders
func TeamChannel(teamID string) string
func UserChannel(userID string) string
func ServerChannel(serverID string) string
func DeploymentChannel(deploymentID string) string

// payload.go - Already exists, enhance
type Payload struct {
    Event string         `json:"event"`
    Model string         `json:"model"`
    ID    string         `json:"id"`
    Data  any            `json:"data"`
    Meta  map[string]any `json:"meta,omitempty"`
}

func NewPayload(model, id, event string, data any) Payload
func (p *Payload) WithMeta(key string, value any) *Payload

// mixin.go - Broadcast mixin for embedding
type Mixin struct {
    WS *websocket.Hub
}

func (m *Mixin) Broadcast(channel string, payload Payload)
func (m *Mixin) BroadcastToTeam(teamID string, payload Payload)
```

### Files to Modify/Create:
- [ ] Create `internal/pkg/broadcast/events.go`
- [ ] Create `internal/pkg/broadcast/channels.go`
- [ ] Enhance `internal/pkg/broadcast/payload.go`
- [ ] Create `internal/pkg/broadcast/mixin.go`

### Estimated Effort: 1 day

---

## Work Package 12: Config/Constants (P3)

**Goal:** Standardized configuration loading and constants.

### Patterns Consolidated:
| Item | Pattern Name | LOC Saved |
|------|--------------|-----------|
| 23 | Config Builder Pattern | ~86 lines |
| 60 | Config Loading with Struct Tags | ~80 lines |
| 98 | Timeout Configuration | ~50 lines |
| 134 | Config Loader Helper | ~80 lines |
| 138 | Cron Schedule Constants | ~30 lines |

### Implementation: Already largely complete in `internal/pkg/config/loader.go`

**Enhancements needed:**
```go
// internal/pkg/config/loader.go - Add:
func LoadFromEnvWithPrefix[T any](prefix string) (*T, error)
func ValidateConfig(cfg any) error

// internal/pkg/constants/timeouts.go
const (
    DefaultHTTPTimeout     = 30 * time.Second
    DefaultSSHTimeout      = 2 * time.Minute
    DefaultDeployTimeout   = 10 * time.Minute
    DefaultProvisionTimeout = 30 * time.Minute
)

// internal/pkg/constants/cron.go
const (
    CronEveryMinute     = "* * * * *"
    CronEvery5Minutes   = "*/5 * * * *"
    CronEveryHour       = "0 * * * *"
    CronDaily           = "0 0 * * *"
    CronWeekly          = "0 0 * * 0"
)
```

### Files to Create:
- [ ] Create `internal/pkg/constants/timeouts.go`
- [ ] Create `internal/pkg/constants/cron.go`
- [ ] Enhance `internal/pkg/config/loader.go`

### Estimated Effort: 0.5 days

---

## Work Package 13: Error Handling (P1)

**Goal:** Unified error types with HTTP status mapping.

### Patterns Consolidated:
| Item | Pattern Name | LOC Saved |
|------|--------------|-----------|
| 41 | AppError HTTPStatus Interface | ~30 lines |
| 63 | Error Type Consolidation | ~80 lines |
| 99 | Duplicate AppError Type | ~50 lines |
| 83 | Activity Logging Helper | (shared with WP11) |

### Implementation: Already largely complete in `internal/pkg/errors/`

**Enhancements needed:**
```go
// internal/pkg/errors/app.go - Ensure single AppError
type AppError struct {
    Code       string `json:"code"`
    Message    string `json:"message"`
    StatusCode int    `json:"-"`
    Cause      error  `json:"-"`
}

func (e *AppError) Error() string
func (e *AppError) Unwrap() error
func (e *AppError) HTTPStatus() int  // For fiber error handler

// Constructors
func NewAppError(code, message string, status int) *AppError
func Wrap(err error, message string) *AppError
func WrapWithCode(err error, code, message string, status int) *AppError
```

### Files to Modify:
- [ ] Consolidate to single `internal/pkg/errors/app.go`
- [ ] Remove duplicate AppError definitions

### Estimated Effort: 0.5 days

---

## Work Package 14: Testing Infrastructure (P3)

**Goal:** Test base classes and mocking utilities.

### Patterns Consolidated:
| Item | Pattern Name | LOC Saved |
|------|--------------|-----------|
| 25 | Test Base Classes | ~200 lines |
| 85 | MockRepository Code Generation | ~80 lines |
| 95 | NotifierFake Lock Wrapper | ~20 lines |

### Implementation Order:

**Step 1: Create Test Helpers** (`internal/pkg/testing/`)
```go
// base.go - Test suite base
type Suite struct {
    suite.Suite
    DB     *gorm.DB
    Ctx    context.Context
    Cancel context.CancelFunc
}

func (s *Suite) SetupTest()
func (s *Suite) TearDownTest()
func (s *Suite) CreateTestDB() *gorm.DB
func (s *Suite) TruncateTables(tables ...string)

// mock.go - Mock helpers
type MockRepository[T any] struct {
    mock.Mock
}

func (m *MockRepository[T]) FindByID(ctx context.Context, id string) (*T, error)
// ... other common methods

// fixtures.go - Test data factories
type Factory[T any] struct {
    build func() *T
}

func NewFactory[T any](build func() *T) *Factory[T]
func (f *Factory[T]) Create() *T
func (f *Factory[T]) CreateN(n int) []*T
```

### Files to Create:
- [ ] Create `internal/pkg/testing/base.go`
- [ ] Create `internal/pkg/testing/mock.go`
- [ ] Create `internal/pkg/testing/fixtures.go`

### Estimated Effort: 1 day

---

## Work Package 15: Utility Helpers (P3)

**Goal:** Misc utility helpers that don't fit other packages.

### Patterns Consolidated:
| Item | Pattern Name |
|------|--------------|
| 37, 55, 119 | Home Directory Helper |
| 53 | JWT Token Parser Helper |
| 54 | Redis Client Factory |
| 73 | Git Ref Trimming Helper |
| 74 | Channel Validation Rules |
| 76 | Sync Associations Pattern |
| 86 | Email Normalization |
| 94 | Double-Check Lock Pattern |
| 109 | Safe Map Access Helper |
| 110 | Collection Filter-Map Pattern |
| 126 | Pagination Parameter Parser |
| 127 | Dynamic Sort Handler |
| 128 | Filter Options Pattern |
| 130 | Metrics Parsing Helper |
| 131 | Service Status Checker |
| 132 | Daemon Status Task Dedup |
| 133 | Health Check Aggregator |
| 151 | URL Builder Package |

### Implementation: Create as needed, these are lower priority standalone utilities.

**Key utilities to create:**
```go
// internal/pkg/utils/path.go
func HomeDir(user string) string
func ExpandPath(path string) string

// internal/pkg/utils/string.go
func NormalizeEmail(email string) string
func TrimGitRef(ref string) string
func Truncate(s string, maxLen int) string

// internal/pkg/utils/map.go
func SafeGet[K comparable, V any](m map[K]V, key K) (V, bool)
func SafeGetOr[K comparable, V any](m map[K]V, key K, defaultVal V) V
func Keys[K comparable, V any](m map[K]V) []K
func Values[K comparable, V any](m map[K]V) []V

// internal/pkg/urlbuilder/builder.go
type Builder struct { baseURL string }
func New(baseURL string) *Builder
func (b *Builder) Path(segments ...string) string
func (b *Builder) WithQuery(params map[string]string) string
```

### Estimated Effort: 1-2 days (as needed)

---

## Implementation Phases

### Phase 1: Core Infrastructure (Week 1-2)
Focus on high-impact, foundational packages:

1. **Work Package 1: Handler Infrastructure** - Creates foundation for all handlers
2. **Work Package 5: DTO/Response Infrastructure** - Used by all handlers
3. **Work Package 10: Security/Crypto** - Critical security fixes
4. **Work Package 13: Error Handling** - Consolidate error types

**Expected Outcome:** ~3,000 lines reduced, unified handler pattern

### Phase 2: Data Layer (Week 3-4)
Focus on repository and model patterns:

5. **Work Package 2: Repository Infrastructure** - Unify all repositories
6. **Work Package 4: Model Mixins** - Reduce model boilerplate
7. **Work Package 9: Enum/Type Infrastructure** - Reduce enum boilerplate

**Expected Outcome:** ~3,000 lines reduced, consistent data access

### Phase 3: Service Layer (Week 5-6)
Focus on jobs, tasks, and HTTP clients:

8. **Work Package 3: Job/Task Infrastructure** - Unify all jobs
9. **Work Package 6: HTTP Client Infrastructure** - Unify API providers
10. **Work Package 7: Template/Script Engine** - Unify script building

**Expected Outcome:** ~3,200 lines reduced, consistent service patterns

### Phase 4: Polish (Week 7-8)
Focus on remaining patterns:

11. **Work Package 8: Notification/Webhook** - Unify notifications
12. **Work Package 11: Broadcast/WebSocket** - Unify broadcasting
13. **Work Package 12: Config/Constants** - Standardize config
14. **Work Package 14: Testing Infrastructure** - Improve testability
15. **Work Package 15: Utility Helpers** - Misc utilities

**Expected Outcome:** ~2,250 lines reduced, complete consolidation

---

## Success Metrics

After completing all work packages:

| Metric | Before | After | Improvement |
|--------|--------|-------|-------------|
| Total LOC | ~65,000 | ~53,550 | -17.6% |
| Handler boilerplate | ~1,200 lines | ~200 lines | -83% |
| Repository duplication | ~1,800 lines | ~300 lines | -83% |
| Job boilerplate | ~1,500 lines | ~300 lines | -80% |
| Enum boilerplate | ~500 lines | ~100 lines | -80% |
| Type safety issues | 177 unsafe casts | 0 | -100% |

---

## Dependency Graph

```
Work Package Dependencies:

WP1 (Handler) ─────────────────────────────────────┐
    │                                               │
    ├── WP5 (DTO) ←── Used by handlers             │
    │       │                                       │
    │       └── WP9 (Enum) ←── Used by DTOs        │
    │                                               │
WP2 (Repository) ──────────────────────────────────┤
    │                                               │
    ├── WP4 (Model) ←── Used by repositories       │
    │                                               │
WP3 (Job/Task) ────────────────────────────────────┤
    │                                               │
    ├── WP7 (Template) ←── Used by tasks           │
    │                                               │
WP6 (HTTP Client) ─────────────────────────────────┤
    │                                               │
    ├── WP8 (Notification) ←── Uses HTTP client    │
    │                                               │
WP10 (Crypto) ─────────────────────────────────────┤
    │                                               │
WP11 (Broadcast) ──────────────────────────────────┤
    │                                               │
WP12 (Config) ─────────────────────────────────────┤
    │                                               │
WP13 (Error) ──────────────────────────────────────┘
    │
WP14 (Testing) ←── Can be done anytime
    │
WP15 (Utility) ←── Can be done anytime
```

---

## Quick Wins (< 1 hour each)

These can be done immediately without full work package:

1. **Item 46: GitLab Timing Attack Fix** - Security fix, 10 minutes
2. **Item 151: URL Builder** - Simple utility, 30 minutes
3. **Item 150: Pointer Helpers** - Simple utility, 20 minutes
4. **Item 148: Generic Slice Mapper** - Simple utility, 20 minutes
5. **Item 138: Cron Constants** - Simple constants, 10 minutes

---

## Notes for Implementation

1. **Start with interfaces** - Define interfaces before implementations
2. **Use generics wisely** - Go 1.18+ generics reduce boilerplate significantly
3. **Maintain backwards compatibility** - Keep old methods as deprecated wrappers
4. **Write tests first** - Each new package should have comprehensive tests
5. **Document patterns** - Update CLAUDE.md with new patterns
6. **Gradual adoption** - Don't refactor all at once, do module by module

---

*Last Updated: 2026-01-21*
*Total Patterns: 158*
*Estimated Total LOC Reduction: ~11,450 lines (17.6% of codebase)*
