# Go Struct Embedding Consolidation Plan

A comprehensive plan for leveraging Go's struct embedding to reduce code duplication, following Laravel's trait-like composition patterns.

## 142. ✅ COMPLETED - Backup Job Payload Base (P2)

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

## 144. ✅ COMPLETED - Webhook Signature Verifier (P1)

**Problem:** 4 separate HMAC-SHA256 signature verification implementations.

**Status:** Already consolidated in `internal/pkg/webhook/base.go` and `internal/modules/git/providers/base.go` with unified verification methods.

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

## 153. ✅ COMPLETED - Status Enum Base Generator (P2)

**Problem:** All status enums have identical structure with Values(), IsValid(), terminal states.

**Status:** Implemented in `internal/pkg/enums/set.go` with generic Set[T comparable] helper.

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

## 155. ✅ COMPLETED - Soft Delete Scope Unification (P2)

**Problem:** Mixed ArchivedAt and DeletedAt approaches with inconsistent filtering.

**Status:** Implemented in `internal/pkg/models/archivable.go` with ArchivableModel mixin and scopes in `internal/pkg/repository/scopes.go` (WithActive, WithArchived, NotArchived).

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

## 156. ✅ COMPLETED - Site Type Helper Methods (P2)

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

### Estimated Effort: 2-3 days - ✅ COMPLETED

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
- [x] Enhance `internal/pkg/jobs/base.go` ✅ (context.go with Base, ModuleContext, ServerContext)
- [x] Create `internal/pkg/jobs/payload.go` ✅ (backup/jobs/payloads.go for backup jobs)
- [x] Create `internal/pkg/jobs/builder.go` ✅ (TaskBuilder with fluent API)
- [x] Create `internal/pkg/jobs/feature_job.go` ✅ (site/jobs/feature_job_helpers.go)

### Estimated Effort: 2 days - ✅ COMPLETED

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
- [x] Create `internal/pkg/models/mixins.go` ✅ (Named, Described, StatusTracking, ProgressTracking, Tokenized)
- [x] Create `internal/pkg/models/hooks.go` ✅ (StatusInitializer, TokenInitializer, SetDefaultStatus, SetDefaultToken)
- [x] Create `internal/pkg/models/scopes.go` ✅ (TeamScoped, ServerScoped, SiteScoped, compound scopes)

### Estimated Effort: 1-2 days - ✅ COMPLETED

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
- [x] Enhance `internal/pkg/dto/time.go` ✅ (FormatTime, ParseTime, TimeAgo, display formats)
- [x] Create `internal/pkg/dto/ptr.go` ✅ (in internal/pkg/ptr/ptr.go - Deref, DerefOr, Ptr, Map, Coalesce)
- [x] Create `internal/pkg/dto/mapper.go` ✅ (in converter.go - MapSlice, FilterSlice, GroupBy, Unique)
- [x] Create `internal/pkg/dto/enum.go` ✅ (EnumResponse, EnumToResponse, EnumsToResponses)
- [x] Create `internal/pkg/dto/response.go` ✅ (TimestampResponse, PaginationMeta, APIResponse)
- [x] Create `internal/pkg/dto/request.go` ✅ (in request_mixins.go and fields.go)

### Estimated Effort: 1-2 days - ✅ COMPLETED

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
- [x] Create `internal/pkg/broadcast/events.go` ✅ (50+ event constants)
- [x] Create `internal/pkg/broadcast/channels.go` ✅ (TeamChannel, ServerChannel, SiteChannel, etc.)
- [x] Enhance `internal/pkg/broadcast/payload.go` ✅ (helpers.go with typed payloads)
- [x] Create `internal/pkg/broadcast/mixin.go` ✅ (Mixin and ModelMixin with nil-safe broadcasting)

### Estimated Effort: 1 day - ✅ COMPLETED

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