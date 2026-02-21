# Codebase Reduction Plan

**Date**: 2026-02-21
**Scope**: Reduce boilerplate and duplication across the entire codebase
**Estimated reduction**: ~18,000-20,000 lines (13-14% of 141,825 total)

---

## Codebase Overview

- **888 Go files**, **141,825 lines of code**
- **13 modules** under `internal/modules/`
- Largest modules: server (27,615 LOC), site (17,194 LOC), notification (9,813 LOC)

---

## Phase 1: Quick Wins (Low Risk, High ROI)

### 1.1 Migrate Remaining Enums to `enumtypes` Helpers

**Impact**: ~150 lines removed
**Risk**: Low (mechanical replacement)

The `enumtypes.ScanString()` / `ValueString()` and `ScanInt()` / `ValueInt()` helpers exist at `internal/pkg/enumtypes/scan.go` but 8 string enums + 1 int enum still use hand-written Scan/Value methods.

#### Enums to migrate (all in `internal/modules/server/types/types.go`):

| Enum | Line (approx) | Nil Default | Notes |
|------|--------------|-------------|-------|
| `ServerProvider` | 158-178 | `ProviderCustom` | Custom nil handling |
| `ServerType` | 289-309 | `""` | Custom nil handling |
| `ServiceType` | 494-513 | `""` | Custom nil handling |
| `ServiceStatus` | 584-604 | `ServiceStatusPending` | Custom nil handling |
| `OperatingSystem` | 773-793 | `""` | Custom nil handling |
| `RuleAction` | 847-867 | `""` | Custom nil handling |

#### In `internal/modules/server/types/software.go`:

| Enum | Line (approx) | Notes |
|------|--------------|-------|
| `Software` | 187-206 | Basic handling |

#### In `internal/modules/server/types/provision_step.go`:

| Enum | Line (approx) | Notes |
|------|--------------|-------|
| `ProvisionStep` | 142-161 | Basic handling |

#### In `internal/modules/site/types/types.go`:

| Enum | Line (approx) | Notes |
|------|--------------|-------|
| `RedirectMode` (int) | 510-530 | Use `ScanInt`/`ValueInt` |

#### Not candidates (JSON-based, appropriate as-is):
- `EnabledFeaturesSlice` in `site/models/site.go` — JSON slice, not an enum
- `ChannelData` in `notification/models/channel_data.go` — JSON struct

#### Implementation approach:

For enums **without** custom nil defaults, replace directly:
```go
// Before (15+ lines)
func (s *Software) Scan(value interface{}) error {
    if value == nil { *s = ""; return nil }
    switch v := value.(type) {
    case []byte: *s = Software(v)
    case string: *s = Software(v)
    default: return fmt.Errorf("cannot scan type %T", value)
    }
    return nil
}
func (s Software) Value() (driver.Value, error) { return string(s), nil }

// After (2 lines)
func (s *Software) Scan(value interface{}) error { return enumtypes.ScanString(s, value) }
func (s Software) Value() (driver.Value, error)  { return enumtypes.ValueString(s) }
```

For enums **with** custom nil defaults (e.g., `ServerProvider` defaults to `ProviderCustom`):
- Option A: Keep hand-written Scan with the custom default, but simplify Value to use helper
- Option B: Add a `ScanStringWithDefault[T](dest *T, value any, defaultVal T)` to enumtypes — one-time addition that handles all cases

**Recommendation**: Option B — add `ScanStringWithDefault` helper, then all enums can use it.

---

### 1.2 Replace Manual DTO Transform Loops with `TransformSlice`

**Impact**: ~116 lines removed (29 instances x 4 lines each)
**Risk**: Very low (exact same logic)

The helper at `internal/pkg/dto/converter.go` provides:
- `ConvertSlice[M, R](models []M, convert func(*M) R) []R`
- `TransformSlice[T, R]` (alias for ConvertSlice)
- `ConvertSlicePtr[M, R]` (for pointer return types)

#### Currently using TransformSlice (correct): 5 instances
- `server/handlers/server_handler.go:55` — `ToServerResponse`
- `server/handlers/server_handler.go:70` — `ToServerResponse`
- `site/handlers/site_handler.go:49` — `ToSiteResponse`
- `server/handlers/load_balancer_handler.go:39` — `ToLoadBalancerUpstreamResponse`
- `server/handlers/load_balancer_handler.go:193` — `ToLoadBalancerBackendResponse`

#### Handler files to migrate (29 instances):

**Server module** (8 instances):
- `daemon_handler.go:27-30` — `ToDaemonResponse`
- `options_handler.go:28-31` — `ToServerProviderResponse`
- `service_handler.go:28-31` — `ToServiceResponse`
- `firewall_handler.go:27-30` — `ToFirewallRuleResponse`
- `cron_handler.go:27-30` — `ToCronResponse`
- `ssh_key_handler.go:31-34` — `ToSSHKeyResponse`
- `ssh_key_handler.go:112-115` — `ToSSHKeyResponse`
- `metric_handler.go:53-56` — `ToMetricResponse`
- `task_handler.go:29-32` — `ToTaskResponse`

**Site module** (5 instances):
- `deployment_handler.go:93-96` — `ToDeploymentResponse`
- `redirect_handler.go:68-71` — `ToRedirectResponse`
- `ssl_handler.go:67-70` — `ToCertificateResponse`
- `queue_handler.go:68-71` — `ToQueueResponse`
- `command_handler.go:68-71` — `ToCommandResponse`

**Git module** (3 instances):
- `source_control_handler.go:35-38` — `ToSourceControlResponse`
- `source_control_handler.go:72-75` — `ToRepositoryResponse`
- `source_control_handler.go:247-250` — `ToRepositoryResponse`

**Script module** (2 instances):
- `script_handler.go:35-38` — `ToScriptResponse` (pointer — use `ConvertSlicePtr`)
- `script_handler.go:169-172` — `ToExecutionResponse` (pointer — use `ConvertSlicePtr`)

**Auth module** (2 instances with named converters):
- `pat_handler.go:83-86` — `toPATResponse`
- Passkey and session handlers use complex inline transforms — skip these

**Billing module** (4 instances):
- `billing_handler.go:57-60` — `ToPlanResponse`
- `billing_handler.go:165-168` — `ToOrderResponse`
- `billing_handler.go:223-226` — `ToPlanResponse`
- `billing_handler.go:131-135` — `ToSubscriptionResponse` (takes extra params — skip)

**Backup module** (2 instances):
- `storage_provider_handler.go:36-39` — `ToStorageProviderResponse`
- `backup_handler.go:30-33` — `ToBackupResponse`

**DNS module** — check for instances

**Notification module** (1 instance with complex inline — skip):
- `notification_channel_handler.go:309-315`

#### Also in DTO/service files (27+ instances):

These are inside DTO response builders and service methods. Same pattern, same fix:
- `git/dto/responses.go:78-82`
- `auth/dto/responses.go:255-260, 265-270, 287-292`
- `server/dto/responses.go:277-285, 693-700, 707-714, 721-728, 735-742, 771-778, 1009-1014`
- `dns/dto/domain.go:86-91`
- `backup/dto/responses.go:77-82`
- `billing/services/billing_service.go:204-209, 218-223`
- `server/services/server_service.go:360-382` (5 occurrences)
- `server/services/installed_service_service.go:227-242, 317-322, 374-379`
- `dns/services/domain_provider_service.go:99-104`
- `dns/services/domain_service.go:142-147, 194-199`
- `dashboard/services/dashboard_service.go:94-99, 147-152`
- `platform/services/platform_update_service.go:45-50, 75-80`
- `platform/dto/dto.go:41-46`

#### Transformation pattern:

```go
// Before (4 lines)
result := make([]dto.FirewallRuleResponse, len(rules))
for i, rule := range rules {
    result[i] = dto.ToFirewallRuleResponse(&rule)
}
return fiberctx.OK(c, "Firewall rules retrieved", result)

// After (1 line + return)
return fiberctx.OK(c, "Firewall rules retrieved", pkgdto.TransformSlice(rules, dto.ToFirewallRuleResponse))
```

For pointer converters:
```go
// Before
result := make([]*dto.ScriptResponse, len(scripts))
for i, script := range scripts {
    result[i] = dto.ToScriptResponse(&script)
}

// After
result := pkgdto.ConvertSlicePtr(scripts, dto.ToScriptResponse)
```

---

### 1.3 Merge Deploy and DeployZeroDowntime Jobs

**Impact**: ~290 lines removed
**Risk**: Medium (needs testing, but changes are structural not behavioral)
**File**: `internal/modules/site/jobs/deploy.go` (954 lines currently)

#### Current state:

Two nearly identical job structs in the same file:
- `DeployJob` (lines ~34-469)
- `DeployZeroDowntimeJob` (lines ~471-904)

#### What's 100% duplicated (9 methods, ~290 lines):

| Method | Lines | Purpose |
|--------|-------|---------|
| `getRepositoryAndSourceControl()` | 22 | DB lookups |
| `getAppAuthToken()` | 31 | Git provider token |
| `buildSourceControlData()` | 32 | Build provider data struct |
| `broadcastDeploymentProgress()` | 19 | WebSocket broadcast |
| `handleDeploymentFailure()` | 23 | Failure handling (only log msg differs) |
| `processNextQueuedDeployment()` | 44 | Queue processing |
| `createProviderDeployment()` | 60 | Git provider status (only description differs) |
| `updateProviderDeploymentStatus()` | 47 | Provider status update |
| `Failed()` | 13 | Job failure callback (only log msg differs) |

#### What actually differs:

1. **`buildDeployConfig()`** — ZeroDowntime adds one line: `releaseTimestamp := time.Now().Format("20060102150405")` and passes it in the config
2. **`Handle()`** — Different broadcast message ("Deployment started" vs "Zero-downtime deployment started")
3. **Minor text differences** — Log messages and git provider description strings

#### Recommended approach:

Merge into a single `DeployJob` with a `ZeroDowntime bool` field in the payload:

```go
type DeployPayload struct {
    SiteID       string `json:"site_id"`
    DeploymentID string `json:"deployment_id"`
    UserID       *string `json:"user_id,omitempty"`
    ZeroDowntime bool   `json:"zero_downtime"` // <-- new field
}

type DeployJob struct {
    Deps    *JobDeps
    Payload DeployPayload
    // ... model fields
}

func (j *DeployJob) buildDeployConfig() DeployOptions {
    // shared logic...
    var releaseTimestamp string
    if j.Payload.ZeroDowntime {
        releaseTimestamp = time.Now().Format("20060102150405")
    }
    return DeployOptions{
        // ...
        ReleaseTimestamp: releaseTimestamp,
    }
}
```

Keep both task types (`TypeDeploy` and `TypeDeployZeroDowntime`) for backward compatibility but have them create the same job struct with different payload:

```go
func NewDeployTask(siteID, deploymentID string, userID *string) (*asynq.Task, error) {
    return pkgjobs.TaskWithID(TypeDeploy, DeployPayload{
        SiteID: siteID, DeploymentID: deploymentID, UserID: userID,
        ZeroDowntime: false,
    }, pkgjobs.Dedup("deploy", siteID))
}

func NewDeployZeroDowntimeTask(siteID, deploymentID string, userID *string) (*asynq.Task, error) {
    return pkgjobs.TaskWithID(TypeDeployZeroDowntime, DeployPayload{
        SiteID: siteID, DeploymentID: deploymentID, UserID: userID,
        ZeroDowntime: true,
    }, pkgjobs.Dedup("deploy", siteID))
}
```

---

### 1.4 Standardize Error Handling in Handlers

**Impact**: Consistency improvement, ~20 lines of changes
**Risk**: Low (behavioral changes in how errors are displayed to users)

#### Current state (250 total error handling calls):

| Pattern | Count | Percentage | Behavior |
|---------|-------|------------|----------|
| `HandleError(c, err)` | 160 | 64% | Preserves fiber.Error codes, handles ValidationError |
| `RespondInternalError(c, "msg")` | 59 | 23.6% | Always returns 500 with custom message |
| `HandleErrorOrInternal(c, err, "msg")` | 31 | 12.4% | Preserves fiber.Error, falls back to 500 with msg |

#### Definitions (from `internal/pkg/fiber/response.go`):

- **`HandleError(c, err)`** — Checks for `fiber.Error` (returns its status code) or `ValidationError` (returns 422). Unknown errors become 500.
- **`HandleErrorOrInternal(c, err, "msg")`** — Same as HandleError for fiber.Error, but unknown errors get the custom message instead of the raw error.
- **`RespondInternalError(c, "msg")`** — Always 500, ignores the error entirely.

#### Recommendation:

**Standard**: Use `HandleError(c, err)` as the default for all handlers where services return typed errors (404, 400, 409, etc.)

**Keep `HandleErrorOrInternal`** only for read operations where you want a user-friendly fallback message.

**Replace `RespondInternalError`** with `HandleError` in these files (services already return proper fiber.Error types):

| File | Lines | Current | Reason to change |
|------|-------|---------|-----------------|
| `backup/handlers/backup_handler.go` | 27, 69, 86 | `RespondInternalError` | Service returns 404 for not found |
| `backup/handlers/storage_provider_handler.go` | 33, 53, 169 | `RespondInternalError` | Service returns 404 |
| `dns/handlers/domain_handler.go` | 34, 40, 93 | `RespondInternalError` | Service returns 404 |
| `dns/handlers/domain_provider_handler.go` | 32 | `RespondInternalError` | Service returns 404 |
| `git/handlers/source_control_handler.go` | 32, 131, 155, 218, 244, 278, 354 | `RespondInternalError` | Service returns 404 |
| `notification/handlers/notification_channel_handler.go` | 33, 59, 87, 120, 148, 183, 207, 231, 259, 274, 294 | `RespondInternalError` | Service returns 404 |
| `script/handlers/script_handler.go` | 32, 57, 166, 192 | `RespondInternalError` | Service returns 404 |
| `billing/handlers/billing_handler.go` | (check) | `RespondInternalError` | Service returns typed errors |

**Keep `RespondInternalError`** for:
- Webhook handlers (don't leak internal errors to external callers)
- Server provision script handler (security-sensitive)
- SSH key generation (crypto operations)
- Metric webhook handler (external caller)

---

## Phase 2: Medium Effort, High Impact

### 2.1 Add Preload Support to `repository.Base[T]`

**Impact**: ~500 lines removed (38+ overridden FindByIDAndX methods eliminated)
**Risk**: Medium

Currently every repository overrides `FindByIDAndTeam` etc. just to add `.Preload()`. The Base should accept default preloads.

**Change**: Add optional preload config to `Base[T]`:
```go
type Base[T any] struct {
    DB              *gorm.DB
    defaultPreloads []string
}
```

Update methods like `FindByID`, `FindByIDAndTeam`, `FindByServer`, etc. to apply preloads automatically.

### 2.2 Generic Job Dispatch Helper in `service.Base`

**Impact**: ~720 lines removed (36 dispatch methods consolidated)
**Risk**: Low

Every service repeats:
```go
func (s *Service) dispatchXxxJob(server *models.Server, entity *models.Xxx) error {
    if !s.HasQueue() { return ErrQueueNotConfigured }
    task, err := jobs.NewXxxTask(server.ID, entity.ID, nil)
    if err != nil { return err }
    return s.EnqueueTask(task)
}
```

**Solution**: Add to `service.Base`:
```go
func (s *Base) MustDispatch(taskFactory func() (*asynq.Task, error)) error {
    if !s.HasQueue() { return ErrQueueNotConfigured }
    task, err := taskFactory()
    if err != nil { return err }
    return s.EnqueueTask(task)
}
```

### 2.3 Soft-Delete / Uninstall Helper

**Impact**: ~240 lines removed (16 instances)
**Risk**: Low

Pattern repeated across firewall, cron, daemon, site, database services:
```go
if entity.IsInstalled() {
    return s.WithTransaction(ctx, func(tx *gorm.DB) error {
        now := time.Now()
        entity.UninstallationRequestedAt = &now
        if err := tx.Save(entity).Error; err != nil { return err }
        return s.dispatchUninstallJob(server, entity)
    })
}
return s.repos.Entity().Delete(ctx, entityID)
```

**Solution**: Generic helper using an `Uninstallable` interface.

### 2.4 Standardize Broadcasting

**Impact**: ~200 lines consolidated
**Risk**: Low

Create typed broadcast helpers:
```go
func (s *Base) BroadcastResourceEvent(teamID, event string, resourceID string, data any)
```

---

## Phase 3: Larger Refactors

### 3.1 Generic CRUD Handler Factories

**Impact**: ~3,000 lines across all handlers
**Risk**: Medium-High (changes handler structure)

Most handlers follow the exact same pattern: extract params, call service, transform response. A generic factory could generate these.

### 3.2 Base Job with Standard Patterns

**Impact**: ~3,000+ lines across 91 job files
**Risk**: Medium

- Standard `Failed()` handler with re-fetch + mark-failed + broadcast
- Model lookup helpers
- Task execution + success-check wrapper

### 3.3 Enum Code Generation

**Impact**: ~2,000 lines of boilerplate
**Risk**: Low (generated code replaces hand-written)

Use `go:generate` with a custom tool to generate `String()`, `IsValid()`, `Label()`, `Scan()`, `Value()`, `AllXxx()` methods from enum definitions.

---

## Implementation Checklist

### Phase 1 (do now)

- [ ] Add `ScanStringWithDefault` helper to `internal/pkg/enumtypes/scan.go`
- [ ] Migrate `ServerProvider.Scan` to use `ScanStringWithDefault`
- [ ] Migrate `ServerType.Scan` to use `ScanString`
- [ ] Migrate `ServiceType.Scan` to use `ScanString`
- [ ] Migrate `ServiceStatus.Scan` to use `ScanStringWithDefault`
- [ ] Migrate `OperatingSystem.Scan` to use `ScanString`
- [ ] Migrate `RuleAction.Scan` to use `ScanString`
- [ ] Migrate `Software.Scan` to use `ScanString`
- [ ] Migrate `ProvisionStep.Scan` to use `ScanString`
- [ ] Migrate `RedirectMode.Scan` to use `ScanInt`
- [ ] Migrate all `.Value()` methods to use `ValueString`/`ValueInt`
- [ ] Replace 29 manual DTO loops in handlers with `TransformSlice`/`ConvertSlicePtr`
- [ ] Replace 27+ manual DTO loops in dto/service files with `TransformSlice`
- [ ] Merge `DeployJob` and `DeployZeroDowntimeJob` into single struct
- [ ] Replace `RespondInternalError` with `HandleError` where services return typed errors
- [ ] Keep `RespondInternalError` for webhook/security-sensitive handlers
- [ ] Run full test suite after each sub-task
- [ ] Verify no behavioral changes (error codes, response formats)

### Phase 2 (next session)

- [ ] Add preload support to `repository.Base[T]`
- [ ] Remove overridden `FindByIDAndX` methods from individual repositories
- [ ] Create `MustDispatch` helper in `service.Base`
- [ ] Replace 36 dispatch helper methods across services
- [ ] Create uninstall helper with `Uninstallable` interface
- [ ] Replace 16 soft-delete patterns

### Phase 3 (future)

- [ ] Design generic CRUD handler factory
- [ ] Design base job with standard patterns
- [ ] Evaluate enum code generation tool
