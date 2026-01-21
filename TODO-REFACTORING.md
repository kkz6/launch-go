# Launch-Go Refactoring Plan

A comprehensive refactoring plan to improve code quality, reduce duplication, and strengthen package infrastructure following Laravel-inspired patterns.

**Generated:** 2026-01-21
**Total Go Files:** 662
**Modules:** 13
**Priority Levels:** P0 (Critical), P1 (High), P2 (Medium), P3 (Low)

---

## Table of Contents

1. [Critical Security & Stability (P0)](#1-critical-security--stability-p0)
2. [Package Infrastructure (P1)](#2-package-infrastructure-p1)
3. [Handler Layer Refactoring (P1)](#3-handler-layer-refactoring-p1)
4. [Service Layer Improvements (P1)](#4-service-layer-improvements-p1)
5. [Repository Pattern Enhancements (P2)](#5-repository-pattern-enhancements-p2)
6. [Task Runner Framework (P2)](#6-task-runner-framework-p2)
7. [DTO & Model Improvements (P2)](#7-dto--model-improvements-p2)
8. [Error Handling Standardization (P2)](#8-error-handling-standardization-p2)
9. [Configuration & Constants (P3)](#9-configuration--constants-p3)
10. [Cross-Cutting Concerns (P3)](#10-cross-cutting-concerns-p3)
11. [Module-Specific Refactoring](#11-module-specific-refactoring)
12. [New Package Proposals](#12-new-package-proposals)

---

## 1. Critical Security & Stability (P0)

### 1.1 Safe Context Locals Extraction
**Issue:** 177 occurrences of unsafe type assertions for user/team context
**Risk:** Panic on missing middleware or incorrect setup

**Files Affected:**
- All handlers in `internal/modules/*/handlers/`
- 115 occurrences of `c.Locals("teamID").(string)`
- 62 occurrences of `c.Locals("userID").(string)`

**Solution:** Create `internal/pkg/fiber/context.go`
```go
package fiber

import (
    "errors"
    "github.com/gofiber/fiber/v2"
)

var (
    ErrTeamIDNotFound = errors.New("team ID not found in context")
    ErrUserIDNotFound = errors.New("user ID not found in context")
    ErrUserNotFound   = errors.New("user not found in context")
)

// GetTeamID safely extracts team ID from context
func GetTeamID(c *fiber.Ctx) (string, error) {
    v, ok := c.Locals("teamID").(string)
    if !ok || v == "" {
        return "", ErrTeamIDNotFound
    }
    return v, nil
}

// MustGetTeamID extracts team ID or panics (use only in middleware-protected routes)
func MustGetTeamID(c *fiber.Ctx) string {
    v, err := GetTeamID(c)
    if err != nil {
        panic(err)
    }
    return v
}

// GetUserID safely extracts user ID from context
func GetUserID(c *fiber.Ctx) (string, error) {
    v, ok := c.Locals("userID").(string)
    if !ok || v == "" {
        return "", ErrUserIDNotFound
    }
    return v, nil
}

// MustGetUserID extracts user ID or panics
func MustGetUserID(c *fiber.Ctx) string {
    v, err := GetUserID(c)
    if err != nil {
        panic(err)
    }
    return v
}

// GetUser safely extracts full user object from context
func GetUser[T any](c *fiber.Ctx) (*T, error) {
    v, ok := c.Locals("user").(*T)
    if !ok || v == nil {
        return nil, ErrUserNotFound
    }
    return v, nil
}
```

**Refactoring Steps:**
- [ ] Create `internal/pkg/fiber/context.go`
- [ ] Create `internal/pkg/fiber/context_test.go`
- [ ] Update all handlers to use safe extraction
- [ ] Add panic recovery middleware for graceful handling

---

### 1.2 Path Parameter Validation ✅
**Issue:** 30+ handlers accept path parameters without validation
**Risk:** Empty strings passed to database queries, potential injection

**Files Affected:**
- `internal/modules/server/handlers/database_handler.go:14`
- `internal/modules/site/handlers/site_handler.go:94-96`
- All handlers using `c.Params("id")`, `c.Params("serverId")`, etc.

**Solution:** Created `internal/pkg/fiber/params.go` with handler-level helpers
```go
package fiber

import (
    "github.com/gofiber/fiber/v2"
    "github.com/kkz6/launch-go/internal/pkg/response"
    "github.com/oklog/ulid/v2"
)

// GetID extracts and validates the "id" path parameter as a ULID.
func GetID(c *fiber.Ctx) (string, error) {
    return GetULIDParam(c, "id")
}

// GetServerID extracts and validates the "serverId" path parameter as a ULID.
func GetServerID(c *fiber.Ctx) (string, error) {
    return GetULIDParam(c, "serverId")
}

// GetULIDParam extracts and validates a path parameter as a ULID.
func GetULIDParam(c *fiber.Ctx, name string) (string, error) {
    value := c.Params(name)
    if value == "" {
        return "", response.BadRequest(c, "Missing required parameter: "+name)
    }
    if _, err := ulid.Parse(value); err != nil {
        return "", response.BadRequest(c, "Invalid parameter format: "+name)
    }
    return value, nil
}
```

**Usage in handlers:**
```go
func (h *Handler) Show(c *fiber.Ctx) error {
    id, err := fiberctx.GetID(c)
    if err != nil {
        return err
    }
    // ... use validated id
}
```

**Refactoring Steps:**
- [x] Create `internal/pkg/fiber/params.go`
- [x] Create `internal/pkg/fiber/params_test.go`
- [x] Create middleware alternative in `internal/middleware/params.go`
- [ ] Update all handlers to use safe extraction (gradual refactoring)

---

### 1.3 Silent Error Handling Audit ✅
**Issue:** Errors silently ignored with blank identifier
**Risk:** Data inconsistency, debugging difficulty

**Files Affected:**
- `internal/modules/site/services/site_service.go:84-88` - Silent error in loop
- Multiple similar patterns across services (58 instances identified)

**Example of Bad Pattern:**
```go
for i := range sites {
    deployment, _ := s.Repos().Deployment().FindLatestBySite(ctx, sites[i].ID)
    sites[i].LatestDeployment = deployment
}
```

**Solution:** Add error logging or handling
```go
for i := range sites {
    deployment, err := s.Repos().Deployment().FindLatestBySite(ctx, sites[i].ID)
    if err != nil && !errors.Is(err, repositories.ErrDeploymentNotFound) {
        s.LogWarn("Failed to fetch latest deployment", "siteID", sites[i].ID, "error", err)
    }
    sites[i].LatestDeployment = deployment
}
```

**Refactoring Steps:**
- [x] Audit all uses of `_` for error return values (58 instances found)
- [x] Fix site_service.go (7 instances)
- [x] Fix deployment_service.go (4 instances - critical: active deployment checks)
- [x] Fix metrics_webhook_handler.go (6 instances - data parsing)
- [x] Fix git webhook handlers/jobs
- [x] Fix billing services and handlers
- [x] Fix DNS handlers and providers
- [x] Fix server services (server_service, composer_service, installed_service_service)
- [ ] Create linter rule to flag silent error handling (optional)

---

## 2. Package Infrastructure (P1)

### 2.1 Generic Job Context Factory
**Issue:** Job context pattern duplicated across modules
**Location:** `internal/modules/database/jobs/context.go` (182 lines)

**Create:** `internal/pkg/jobs/context.go`
```go
package jobs

import (
    "context"
    "github.com/hibiken/asynq"
    "github.com/rs/zerolog"
    "gorm.io/gorm"
)

// JobContext provides common dependencies for all jobs
type JobContext struct {
    db         *gorm.DB
    logger     *zerolog.Logger
    broadcaster broadcast.ModelBroadcaster
    queue      *queue.Client
    dispatcher *taskrunner.Dispatcher
}

// NewJobContext creates a new job context with all dependencies
func NewJobContext(opts ...JobContextOption) *JobContext {
    ctx := &JobContext{}
    for _, opt := range opts {
        opt(ctx)
    }
    return ctx
}

type JobContextOption func(*JobContext)

func WithDB(db *gorm.DB) JobContextOption {
    return func(c *JobContext) { c.db = db }
}

func WithLogger(l *zerolog.Logger) JobContextOption {
    return func(c *JobContext) { c.logger = l }
}

func WithBroadcaster(b broadcast.ModelBroadcaster) JobContextOption {
    return func(c *JobContext) { c.broadcaster = b }
}

func WithQueue(q *queue.Client) JobContextOption {
    return func(c *JobContext) { c.queue = q }
}

func WithDispatcher(d *taskrunner.Dispatcher) JobContextOption {
    return func(c *JobContext) { c.dispatcher = d }
}

// Log methods
func (c *JobContext) LogInfo(msg string, fields ...interface{}) {
    logWithFields(c.logger.Info(), msg, fields...)
}

func (c *JobContext) LogError(msg string, fields ...interface{}) {
    logWithFields(c.logger.Error(), msg, fields...)
}

func (c *JobContext) LogWarn(msg string, fields ...interface{}) {
    logWithFields(c.logger.Warn(), msg, fields...)
}

func logWithFields(event *zerolog.Event, msg string, fields ...interface{}) {
    for i := 0; i < len(fields)-1; i += 2 {
        if key, ok := fields[i].(string); ok {
            event = event.Interface(key, fields[i+1])
        }
    }
    event.Msg(msg)
}
```

**Refactoring Steps:**
- [x] Create `internal/pkg/jobs/context.go`
- [x] Create `internal/pkg/jobs/context_test.go`
- [x] Refactor `internal/modules/database/jobs/context.go` to use pkg
- [x] Refactor `internal/modules/server/jobs/context.go` to use pkg
- [x] Refactor `internal/modules/site/jobs/context.go` to use pkg
- [x] Refactor `internal/modules/script/jobs/context.go` to use pkg
- [x] Refactor `internal/modules/backup/jobs/context.go` to use pkg
- [x] Remove duplicated logging helpers (LogInfo, LogError methods now inherited from Base)

---

### 2.2 Service Factory Pattern
**Issue:** Inconsistent service initialization across modules
**Evidence:**
- Site module uses `ServiceDeps` struct
- Server module uses individual parameters

**Create:** `internal/pkg/service/factory.go`
```go
package service

import (
    "github.com/rs/zerolog"
    "gorm.io/gorm"
)

// Dependencies holds all common service dependencies
type Dependencies struct {
    DB          *gorm.DB
    Queue       *queue.Client
    Broadcaster broadcast.ModelBroadcaster
    Logger      *zerolog.Logger
    Dispatcher  *taskrunner.Dispatcher
}

// ServiceFactory creates services with injected dependencies
type ServiceFactory struct {
    deps Dependencies
}

func NewServiceFactory(deps Dependencies) *ServiceFactory {
    return &ServiceFactory{deps: deps}
}

func (f *ServiceFactory) Dependencies() Dependencies {
    return f.deps
}

// CreateBase creates a new Base service with all dependencies
func (f *ServiceFactory) CreateBase() *Base {
    return NewBase(f.deps.Queue, f.deps.Broadcaster, f.deps.Logger)
}
```

**Refactoring Steps:**
- [x] Create `internal/pkg/service/base.go` with Dependencies struct
- [x] Add NewBaseFromDeps and NewDependencies functions
- [x] Add ServiceDeps() method to module.Builder and module.Deps
- [x] Update server module to use ServiceDeps with embedded Dependencies
- [x] Update site module to embed service.Dependencies in ServiceDeps
- [x] Update dns module to embed service.Dependencies in ServiceDeps
- [x] Update backup module to embed service.Dependencies in ServiceDeps
- [x] Add tests for service package

---

### 2.3 Repository Registry Generator
**Issue:** Registry pattern copy-pasted across all modules
**Files Affected:**
- `internal/modules/auth/repositories/repository.go`
- `internal/modules/database/repositories/base.go`
- `internal/modules/server/repositories/`
- All other modules

**Create:** `internal/pkg/repository/registry.go`
```go
package repository

import "gorm.io/gorm"

// RegistryBase provides common registry functionality
type RegistryBase struct {
    db *gorm.DB
}

func NewRegistryBase(db *gorm.DB) RegistryBase {
    return RegistryBase{db: db}
}

func (r RegistryBase) DB() *gorm.DB {
    return r.db
}

// Example module registry using composition
// type Registry struct {
//     RegistryBase
//     user   *UserRepository
//     team   *TeamRepository
// }
```

**Refactoring Steps:**
- [ ] Create `internal/pkg/repository/registry.go`
- [ ] Add `RegistryBase` with common DB accessor
- [ ] Refactor all module registries to use composition
- [ ] Add interface enforcement via compile-time checks

---

### 2.4 Task Template Engine
**Issue:** Bash functions duplicated across script templates
**Locations:**
- `internal/modules/server/tasks/templates/`
- `internal/modules/database/tasks/templates/`
- `internal/modules/site/tasks/templates/`

**Create:** `internal/pkg/taskrunner/templates/`
```go
package templates

import "embed"

//go:embed common/*.sh
var CommonScripts embed.FS

// Common bash functions available to all tasks
const CommonFunctions = `
# Error handling
set -euo pipefail

# Logging functions
log_info() { echo "[INFO] $1"; }
log_error() { echo "[ERROR] $1" >&2; }
log_success() { echo "[SUCCESS] $1"; }

# Package management
apt_install() {
    DEBIAN_FRONTEND=noninteractive apt-get install -y "$@"
}

apt_update() {
    apt-get update -qq
}

# Service management
service_restart() {
    systemctl restart "$1"
}

service_enable() {
    systemctl enable "$1"
}

# HTTP helpers
http_get() {
    curl -fsSL "$1"
}

http_download() {
    curl -fsSL -o "$2" "$1"
}
`

// BuildScript combines common functions with task-specific script
func BuildScript(taskScript string) string {
    return CommonFunctions + "\n" + taskScript
}
```

**Refactoring Steps:**
- [ ] Create `internal/pkg/taskrunner/templates/`
- [ ] Extract common bash functions to `common/*.sh`
- [ ] Create template builder with function injection
- [ ] Refactor all task scripts to use common functions
- [ ] Add script validation/shellcheck integration

---

## 3. Handler Layer Refactoring (P1)

### 3.1 Extract Handler Boilerplate
**Issue:** 67 handlers repeat identical request parsing pattern

**Current Pattern (repeated 67 times):**
```go
if err := c.BodyParser(&req); err != nil {
    return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
}
if errs := validator.Validate(&req); errs != nil {
    return response.ValidationError(c, errs)
}
```

**Create:** `internal/pkg/fiber/request.go`
```go
package fiber

import (
    "github.com/gofiber/fiber/v2"
    "github.com/kkz6/launch-go/internal/pkg/response"
    "github.com/kkz6/launch-go/internal/pkg/validator"
)

// ParseAndValidate parses request body and validates it
func ParseAndValidate[T any](c *fiber.Ctx, req *T) error {
    if err := c.BodyParser(req); err != nil {
        return response.BadRequest(c, "Invalid request body")
    }
    if errs := validator.Validate(req); errs != nil {
        return response.ValidationError(c, errs)
    }
    return nil
}

// MustParseAndValidate parses and validates, returning the request or error response
func MustParseAndValidate[T any](c *fiber.Ctx) (*T, error) {
    var req T
    if err := ParseAndValidate(c, &req); err != nil {
        return nil, err
    }
    return &req, nil
}

// ParseQuery parses query parameters into struct
func ParseQuery[T any](c *fiber.Ctx) (*T, error) {
    var req T
    if err := c.QueryParser(&req); err != nil {
        return nil, response.BadRequest(c, "Invalid query parameters")
    }
    return &req, nil
}
```

**Refactoring Steps:**
- [x] Create `internal/pkg/fiber/request.go`
- [x] Create generics-based parsing helpers
- [~] Refactor all 67 handlers to use new helpers (2/67 done: cron_handler, daemon_handler)
- [ ] Remove hardcoded "Invalid request body" strings

---

### 3.2 Handler Base Class
**Issue:** Handlers lack common functionality (logging, error handling)

**Create:** `internal/pkg/handler/base.go`
```go
package handler

import (
    "github.com/gofiber/fiber/v2"
    "github.com/rs/zerolog"
)

// Base provides common handler functionality
type Base struct {
    logger *zerolog.Logger
}

func NewBase(logger *zerolog.Logger) Base {
    return Base{logger: logger}
}

func (h *Base) Logger() *zerolog.Logger {
    return h.logger
}

func (h *Base) LogRequest(c *fiber.Ctx, action string) {
    h.logger.Info().
        Str("action", action).
        Str("method", c.Method()).
        Str("path", c.Path()).
        Str("ip", c.IP()).
        Msg("Request received")
}

func (h *Base) LogError(c *fiber.Ctx, err error, msg string) {
    h.logger.Error().
        Err(err).
        Str("method", c.Method()).
        Str("path", c.Path()).
        Msg(msg)
}
```

**Refactoring Steps:**
- [x] Create `internal/pkg/handler/base.go`
- [x] Add logging and error helpers
- [ ] Embed Base in all module handlers (gradual adoption)
- [x] Add request tracing support (GetTraceID method)

---

### 3.3 Standardize Error Response Handling
**Issue:** Mixed error handling patterns across handlers

**Inconsistent Examples:**
- Auth module: Always returns 400 regardless of error type
- Site module: Checks specific error types for proper status codes
- Server module: Mix of both approaches

**Create standardized approach:** `internal/pkg/fiber/errors.go`
```go
package fiber

import (
    "errors"
    "github.com/gofiber/fiber/v2"
    apperrors "github.com/kkz6/launch-go/internal/pkg/errors"
    "github.com/kkz6/launch-go/internal/pkg/response"
)

// HandleServiceError converts service errors to appropriate HTTP responses
func HandleServiceError(c *fiber.Ctx, err error) error {
    if err == nil {
        return nil
    }

    var appErr *apperrors.AppError
    if errors.As(err, &appErr) {
        return response.Error(c, appErr.StatusCode, appErr.Message)
    }

    // Check for common error patterns
    switch {
    case errors.Is(err, gorm.ErrRecordNotFound):
        return response.NotFound(c, "Resource not found")
    case errors.Is(err, context.Canceled):
        return response.Error(c, fiber.StatusRequestTimeout, "Request cancelled")
    case errors.Is(err, context.DeadlineExceeded):
        return response.Error(c, fiber.StatusGatewayTimeout, "Request timeout")
    default:
        return response.InternalError(c, "An unexpected error occurred")
    }
}
```

**Refactoring Steps:**
- [ ] Create centralized error-to-response mapping
- [ ] Define error types for all common scenarios
- [ ] Refactor all handlers to use `HandleServiceError`
- [ ] Add logging for unexpected errors

---

## 4. Service Layer Improvements (P1)

### 4.1 Interface-Based Module Dependencies
**Issue:** Site service imports directly from 5 other modules (tight coupling)

**Current (Bad):**
```go
// internal/modules/site/services/site_service.go
import (
    databaseservices "github.com/kkz6/launch-go/internal/modules/database/services"
    serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
    serverservices "github.com/kkz6/launch-go/internal/modules/server/services"
    // ... more direct imports
)
```

**Solution:** Use contracts/interfaces

**Create:** `internal/modules/site/contracts/dependencies.go`
```go
package contracts

import "context"

// ServerService defines server operations needed by site module
type ServerService interface {
    GetServer(ctx context.Context, id, teamID string) (*ServerInfo, error)
    IsProvisioned(ctx context.Context, serverID string) (bool, error)
}

// DatabaseService defines database operations needed by site module
type DatabaseService interface {
    ListDatabases(ctx context.Context, serverID string) ([]DatabaseInfo, error)
    CreateDatabase(ctx context.Context, opts CreateDatabaseOpts) error
}

// Simple DTOs to avoid circular imports
type ServerInfo struct {
    ID         string
    Name       string
    IP         string
    Status     string
    Connection ConnectionInfo
}

type ConnectionInfo struct {
    Host     string
    Port     int
    User     string
    KeyPath  string
}
```

**Refactoring Steps:**
- [ ] Define interface contracts in each module's `contracts/` directory
- [ ] Create adapter implementations where modules interact
- [ ] Use dependency injection via module builder
- [ ] Remove direct cross-module imports in services

---

### 4.2 Service Method Consistency
**Issue:** Service methods have inconsistent signatures

**Inconsistent Examples:**
- Some return `(result, error)`
- Some return `(*result, error)`
- Some return `error` only

**Standard Pattern:**
```go
// For queries - return pointer for nil safety
func (s *Service) FindByID(ctx context.Context, id string) (*Model, error)

// For lists - return slice (empty slice vs error)
func (s *Service) List(ctx context.Context, opts ListOpts) ([]Model, error)

// For mutations - return created/updated entity
func (s *Service) Create(ctx context.Context, req CreateRequest) (*Model, error)
func (s *Service) Update(ctx context.Context, id string, req UpdateRequest) (*Model, error)

// For deletions - return error only
func (s *Service) Delete(ctx context.Context, id string) error

// For actions - return error only
func (s *Service) Trigger(ctx context.Context, id string) error
```

**Refactoring Steps:**
- [ ] Document standard return type conventions
- [ ] Audit all service methods for consistency
- [ ] Refactor methods to follow conventions
- [ ] Update handler error handling accordingly

---

### 4.3 Request DTO Normalization
**Issue:** Normalization done in handlers instead of DTOs

**Current (Bad) - Handler doing normalization:**
```go
// internal/modules/site/handlers/site_handler.go:64-90
normalizeEmptyStringPtr(&req.DeployScript)
normalizeEmptyStringPtr(&req.BuildScript)
// ... 7 times
```

**Solution:** DTO self-normalization

```go
// internal/modules/site/dto/requests.go
type CreateSiteRequest struct {
    Name         string  `json:"name" validate:"required"`
    DeployScript *string `json:"deploy_script"`
    BuildScript  *string `json:"build_script"`
    // ...
}

func (r *CreateSiteRequest) Normalize() {
    normalizePtr(&r.DeployScript)
    normalizePtr(&r.BuildScript)
    // ...
}

func normalizePtr(s **string) {
    if s != nil && *s != nil && strings.TrimSpace(**s) == "" {
        *s = nil
    }
}
```

**Refactoring Steps:**
- [ ] Add `Normalize()` method to all request DTOs
- [ ] Call normalize in `ParseAndValidate` helper
- [ ] Remove normalization code from handlers
- [ ] Add normalization for common patterns (trim, lowercase email, etc.)

---

## 5. Repository Pattern Enhancements (P2)

### 5.1 Generic Query Builder Improvements
**Location:** `internal/pkg/repository/query.go`

**Add missing features:**
```go
package repository

// QueryBuilder enhancements
type QueryBuilder[T any] struct {
    db      *gorm.DB
    preload []string
    joins   []string
    selects []string
}

// Preload adds eager loading
func (q *QueryBuilder[T]) Preload(associations ...string) *QueryBuilder[T] {
    q.preload = append(q.preload, associations...)
    return q
}

// Join adds join clause
func (q *QueryBuilder[T]) Join(join string, args ...interface{}) *QueryBuilder[T] {
    q.joins = append(q.joins, join)
    return q
}

// Select specifies fields to retrieve
func (q *QueryBuilder[T]) Select(fields ...string) *QueryBuilder[T] {
    q.selects = append(q.selects, fields...)
    return q
}

// First returns first matching record
func (q *QueryBuilder[T]) First(ctx context.Context) (*T, error) {
    var result T
    db := q.applyAll(q.db.WithContext(ctx))
    if err := db.First(&result).Error; err != nil {
        return nil, err
    }
    return &result, nil
}

// All returns all matching records
func (q *QueryBuilder[T]) All(ctx context.Context) ([]T, error) {
    var results []T
    db := q.applyAll(q.db.WithContext(ctx))
    if err := db.Find(&results).Error; err != nil {
        return nil, err
    }
    return results, nil
}
```

**Refactoring Steps:**
- [ ] Enhance QueryBuilder with more operations
- [ ] Add method chaining support
- [ ] Add pagination integration
- [ ] Refactor repositories to use builder

---

### 5.2 Duplicate Find Methods
**Issue:** Every repository has similar FindBy* methods

**Example from database repository (lines 18-87):**
- `FindByID`
- `FindByIDAndServer`
- `FindByIDAndTeam`
- `FindByIDAndServerAndTeam`

All with identical error handling pattern.

**Solution:** Generic scope-based queries

```go
// internal/pkg/repository/scopes.go
package repository

// Scope defines a query modification function
type Scope func(*gorm.DB) *gorm.DB

// Common scopes
func WithID(id string) Scope {
    return func(db *gorm.DB) *gorm.DB {
        return db.Where("id = ?", id)
    }
}

func WithServerID(serverID string) Scope {
    return func(db *gorm.DB) *gorm.DB {
        return db.Where("server_id = ?", serverID)
    }
}

func WithTeamID(teamID string) Scope {
    return func(db *gorm.DB) *gorm.DB {
        return db.Where("team_id = ?", teamID)
    }
}

// FindOne with multiple scopes
func FindOne[T any](ctx context.Context, db *gorm.DB, scopes ...Scope) (*T, error) {
    query := db.WithContext(ctx)
    for _, scope := range scopes {
        query = scope(query)
    }
    var result T
    if err := query.First(&result).Error; err != nil {
        return nil, err
    }
    return &result, nil
}
```

**Usage:**
```go
// Instead of FindByIDAndServerAndTeam
db, _ := FindOne[Database](ctx, r.db,
    WithID(id),
    WithServerID(serverID),
    WithTeamID(teamID),
)
```

**Refactoring Steps:**
- [ ] Create `internal/pkg/repository/scopes.go`
- [ ] Define common scopes
- [ ] Create `FindOne`, `FindAll` generic functions
- [ ] Refactor repositories to use scope-based queries

---

## 6. Task Runner Framework (P2)

### 6.1 Move Core Task Runner to pkg
**Issue:** Task runner logic scattered across modules

**Current Location:** `internal/modules/server/tasks/runner.go` (825 lines)

**Should be:** `internal/pkg/taskrunner/runner.go`

**Refactoring Steps:**
- [ ] Extract generic runner to `internal/pkg/taskrunner/`
- [ ] Keep module-specific task definitions in modules
- [ ] Create task factory interface in pkg
- [ ] Update all modules to use pkg runner

---

### 6.2 Callback Registration Consolidation
**Issue:** Each module has its own `register.go` for callbacks

**Files:**
- `internal/modules/server/tasks/register.go`
- `internal/modules/site/tasks/register.go`
- `internal/modules/database/tasks/register.go`

**Create:** Centralized registration in `internal/pkg/taskrunner/registry.go`
```go
package taskrunner

var callbackRegistry = make(map[string]CallbackStateFactory)

// RegisterCallback registers a callback factory for a task type
func RegisterCallback(taskType string, factory CallbackStateFactory) {
    callbackRegistry[taskType] = factory
}

// GetCallbackFactory returns the factory for a task type
func GetCallbackFactory(taskType string) (CallbackStateFactory, bool) {
    f, ok := callbackRegistry[taskType]
    return f, ok
}
```

**Refactoring Steps:**
- [ ] Create centralized callback registry
- [ ] Keep registration calls in modules but use central registry
- [ ] Add validation for duplicate registrations
- [ ] Add discovery/listing of registered callbacks

---

### 6.3 Script Validation Integration
**Issue:** No automatic validation of bash scripts

**Create:** `internal/pkg/taskrunner/validate.go`
```go
package taskrunner

import (
    "os/exec"
    "strings"
)

// ValidateScript runs shellcheck on a script
func ValidateScript(script string) error {
    cmd := exec.Command("shellcheck", "-s", "bash", "-")
    cmd.Stdin = strings.NewReader(script)
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("shellcheck failed: %s\n%s", err, output)
    }
    return nil
}

// ValidateScriptWithSeverity validates with minimum severity
func ValidateScriptWithSeverity(script string, minSeverity string) error {
    cmd := exec.Command("shellcheck", "-s", "bash", "--severity", minSeverity, "-")
    cmd.Stdin = strings.NewReader(script)
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("shellcheck failed: %s\n%s", err, output)
    }
    return nil
}
```

**Refactoring Steps:**
- [ ] Add script validation to task builder
- [ ] Create pre-commit hook for script validation
- [ ] Add validation to CI pipeline
- [ ] Create `make shellcheck` target

---

## 7. DTO & Model Improvements (P2)

### 7.1 Time Format Helpers
**Issue:** `time.RFC3339` formatting repeated 81+ times

**Create:** `internal/pkg/dto/time.go`
```go
package dto

import "time"

// FormatTime converts time pointer to RFC3339 string pointer
func FormatTime(t *time.Time) *string {
    if t == nil {
        return nil
    }
    s := t.Format(time.RFC3339)
    return &s
}

// FormatTimeValue converts time value to RFC3339 string
func FormatTimeValue(t time.Time) string {
    return t.Format(time.RFC3339)
}

// FormatTimeOrEmpty returns empty string for nil time
func FormatTimeOrEmpty(t *time.Time) string {
    if t == nil {
        return ""
    }
    return t.Format(time.RFC3339)
}

// ParseTime parses RFC3339 string to time
func ParseTime(s string) (*time.Time, error) {
    if s == "" {
        return nil, nil
    }
    t, err := time.Parse(time.RFC3339, s)
    if err != nil {
        return nil, err
    }
    return &t, nil
}
```

**Refactoring Steps:**
- [ ] Create `internal/pkg/dto/time.go`
- [ ] Refactor all DTO response builders
- [ ] Replace direct `time.Format(time.RFC3339)` calls

---

### 7.2 Response Builder Pattern
**Issue:** DTO conversion code duplicated

**Current Pattern (auth/dto/responses.go has good examples):**
```go
func ToUserResponse(u *models.User) *UserResponse { ... }
func ToUserResponsePtr(u *models.User) *UserResponse { ... }
func ToUserResponses(users []models.User) []UserResponse { ... }
```

**Standardize with generics:**
```go
// internal/pkg/dto/converter.go
package dto

// ToResponse converts single model to response
type ToResponse[M, R any] func(*M) *R

// ConvertSlice converts slice of models to responses
func ConvertSlice[M, R any](models []M, convert func(*M) *R) []R {
    if models == nil {
        return nil
    }
    results := make([]R, len(models))
    for i := range models {
        results[i] = *convert(&models[i])
    }
    return results
}

// ConvertPtr safely converts pointer, returning nil for nil input
func ConvertPtr[M, R any](model *M, convert func(*M) *R) *R {
    if model == nil {
        return nil
    }
    return convert(model)
}
```

**Refactoring Steps:**
- [ ] Create generic conversion helpers
- [ ] Refactor DTO converters to use helpers
- [ ] Reduce boilerplate in response builders

---

### 7.3 Model Broadcast Helpers
**Issue:** Broadcast payload creation duplicated

**Create:** `internal/pkg/models/broadcast_helpers.go`
```go
package models

// BroadcastAction represents the type of change
type BroadcastAction string

const (
    BroadcastCreated BroadcastAction = "created"
    BroadcastUpdated BroadcastAction = "updated"
    BroadcastDeleted BroadcastAction = "deleted"
)

// BroadcastPayload creates standard broadcast payload
func BroadcastPayload[T Broadcastable](model T, action BroadcastAction) map[string]interface{} {
    return map[string]interface{}{
        "action": action,
        "model":  model.BroadcastName(),
        "data":   model.BroadcastPayload(),
    }
}
```

**Refactoring Steps:**
- [ ] Create broadcast payload helpers
- [ ] Standardize action types
- [ ] Refactor service broadcast calls

---

## 8. Error Handling Standardization (P2)

### 8.1 Module Error Types
**Issue:** Inconsistent error definitions across modules

**Create standard pattern for each module:**
```go
// internal/modules/site/errors/errors.go
package errors

import apperrors "github.com/kkz6/launch-go/internal/pkg/errors"

var (
    ErrSiteNotFound      = apperrors.NotFound("Site not found")
    ErrSiteNotDeployed   = apperrors.BadRequest("Site has not been deployed")
    ErrDeploymentFailed  = apperrors.InternalError("Deployment failed")
    ErrInvalidRepository = apperrors.BadRequest("Invalid repository URL")
)

// Repository errors (re-exported for handler convenience)
var (
    ErrDeploymentNotFound = repositories.ErrDeploymentNotFound
)
```

**Refactoring Steps:**
- [ ] Create `errors/errors.go` in each module
- [ ] Define all module-specific errors
- [ ] Use consistent error types (NotFound, BadRequest, etc.)
- [ ] Add error codes for client handling

---

### 8.2 Error Wrapping Convention
**Issue:** Errors not wrapped with context

**Standard Pattern:**
```go
// Good - wrapped with context
if err := r.db.Create(&model).Error; err != nil {
    return fmt.Errorf("failed to create site: %w", err)
}

// Bad - raw error
if err := r.db.Create(&model).Error; err != nil {
    return err
}
```

**Refactoring Steps:**
- [ ] Audit all error returns
- [ ] Add context to error wrapping
- [ ] Create error wrapping helpers if needed

---

## 9. Configuration & Constants (P3)

### 9.1 Module Constants Files
**Issue:** Magic strings and numbers scattered throughout code

**Create for each module:** `internal/modules/*/constants/constants.go`

**Example - Auth Module:**
```go
// internal/modules/auth/constants/constants.go
package constants

// Defaults
const (
    DefaultTimezone = "UTC"
    DefaultPageSize = 15
    MaxPageSize     = 100
)

// Role names
const (
    RoleAdmin  = "admin"
    RoleEditor = "editor"
    RoleMember = "member"
)

// Role descriptions
var RoleDescriptions = map[string]string{
    RoleAdmin:  "Full access to all resources",
    RoleEditor: "Can edit but not delete",
    RoleMember: "Read-only access",
}

// Token types
const (
    TokenTypeAccess  = "access"
    TokenTypeRefresh = "refresh"
)
```

**Example - Site Module:**
```go
// internal/modules/site/constants/constants.go
package constants

const (
    DefaultDeploymentRetention = 10
    DefaultAutoDeployment      = true
    DefaultAutoRestartQueue    = true
)

// Deployment statuses
const (
    DeploymentPending   = "pending"
    DeploymentRunning   = "running"
    DeploymentSucceeded = "succeeded"
    DeploymentFailed    = "failed"
)
```

**Refactoring Steps:**
- [ ] Create constants files for all modules
- [ ] Extract all hardcoded values
- [ ] Reference constants throughout codebase
- [ ] Document purpose of each constant

---

### 9.2 Response Messages
**Issue:** "Invalid request body" repeated 65 times

**Create:** `internal/pkg/response/messages.go`
```go
package response

// Standard error messages
const (
    MsgInvalidRequestBody   = "Invalid request body"
    MsgValidationFailed     = "Validation failed"
    MsgResourceNotFound     = "Resource not found"
    MsgUnauthorized         = "Unauthorized"
    MsgForbidden            = "Access denied"
    MsgInternalError        = "An unexpected error occurred"
    MsgRateLimitExceeded    = "Rate limit exceeded"
)

// Standard success messages
const (
    MsgCreated  = "Resource created successfully"
    MsgUpdated  = "Resource updated successfully"
    MsgDeleted  = "Resource deleted successfully"
)
```

**Refactoring Steps:**
- [ ] Create messages constants file
- [ ] Replace all hardcoded message strings
- [ ] Consider i18n support for future

---

### 9.3 Configuration Loader Generic Pattern
**Issue:** Each config file repeats load/setDefaults pattern

**Create:** `internal/pkg/config/loader.go`
```go
package config

import "github.com/spf13/viper"

// Loadable interface for configs that can be loaded
type Loadable interface {
    SetDefaults()
    Load()
}

// LoadAll loads multiple configs in order
func LoadAll(configs ...Loadable) {
    for _, c := range configs {
        c.SetDefaults()
        c.Load()
    }
}

// GetEnvOrDefault gets environment variable with default
func GetEnvOrDefault(key, defaultValue string) string {
    if v := viper.GetString(key); v != "" {
        return v
    }
    return defaultValue
}

// GetIntOrDefault gets int env var with default
func GetIntOrDefault(key string, defaultValue int) int {
    if viper.IsSet(key) {
        return viper.GetInt(key)
    }
    return defaultValue
}
```

**Refactoring Steps:**
- [ ] Create generic config loader helpers
- [ ] Refactor all config files to use helpers
- [ ] Add config validation

---

## 10. Cross-Cutting Concerns (P3)

### 10.1 Logging in Handlers
**Issue:** No logging in any handlers

**Solution:** Add structured logging

```go
// Before service call
h.Logger().Debug().
    Str("action", "create_site").
    Str("teamID", teamID).
    Str("serverID", serverID).
    Msg("Creating site")

// After error
h.Logger().Error().
    Err(err).
    Str("action", "create_site").
    Msg("Failed to create site")
```

**Refactoring Steps:**
- [ ] Add logger to handler base
- [ ] Add debug logging before service calls
- [ ] Add error logging for failures
- [ ] Add request ID for tracing

---

### 10.2 Request Tracing
**Issue:** No request tracing across services

**Create:** `internal/middleware/trace.go`
```go
package middleware

import (
    "github.com/gofiber/fiber/v2"
    "github.com/oklog/ulid/v2"
)

const TraceIDKey = "traceID"

// Trace adds trace ID to request context
func Trace() fiber.Handler {
    return func(c *fiber.Ctx) error {
        traceID := c.Get("X-Trace-ID")
        if traceID == "" {
            traceID = ulid.Make().String()
        }
        c.Locals(TraceIDKey, traceID)
        c.Set("X-Trace-ID", traceID)
        return c.Next()
    }
}
```

**Refactoring Steps:**
- [ ] Add trace middleware
- [ ] Include trace ID in all logs
- [ ] Pass trace ID to service layer
- [ ] Include in job payloads

---

### 10.3 Middleware Formatting Helpers
**Issue:** Formatting functions in middleware should be in pkg

**Move from:** `internal/middleware/logger.go`
**Move to:** `internal/pkg/logger/formatters.go`

```go
package logger

import "github.com/charmbracelet/lipgloss"

// HTTP method icons
var MethodIcons = map[string]string{
    "GET":    "→",
    "POST":   "●",
    "PUT":    "◆",
    "PATCH":  "◇",
    "DELETE": "✕",
}

// Status code colors
func StatusColor(code int) lipgloss.Style {
    switch {
    case code >= 500:
        return lipgloss.NewStyle().Foreground(lipgloss.Color("red"))
    case code >= 400:
        return lipgloss.NewStyle().Foreground(lipgloss.Color("yellow"))
    case code >= 300:
        return lipgloss.NewStyle().Foreground(lipgloss.Color("cyan"))
    default:
        return lipgloss.NewStyle().Foreground(lipgloss.Color("green"))
    }
}

// FormatDuration formats duration for logging
func FormatDuration(d time.Duration) string {
    if d < time.Millisecond {
        return fmt.Sprintf("%dµs", d.Microseconds())
    }
    if d < time.Second {
        return fmt.Sprintf("%dms", d.Milliseconds())
    }
    return fmt.Sprintf("%.2fs", d.Seconds())
}
```

**Refactoring Steps:**
- [ ] Extract formatting to pkg/logger
- [ ] Use in middleware
- [ ] Make available for CLI output

---

## 11. Module-Specific Refactoring

### 11.1 Auth Module (46 files)

| Item | File | Issue | Solution |
|------|------|-------|----------|
| 1 | `handlers/auth_handler.go:34-35` | Always returns 400 | Use HandleServiceError |
| 2 | `services/auth_service.go:56` | Hardcoded "UTC" | Use constant |
| 3 | `dto/responses.go:286-289` | Hardcoded role names | Use constants |
| 4 | Multiple handlers | Unsafe context extraction | Use safe helpers |

### 11.2 Server Module (97 files)

| Item | File | Issue | Solution |
|------|------|-------|----------|
| 1 | `handlers/database_handler.go:18,53,70` | Mixed error handling | Standardize |
| 2 | `services/base.go:16-26` | Service errors defined locally | Move to errors/ |
| 3 | `tasks/runner.go` | 825 lines | Extract to pkg |
| 4 | Multiple jobs | Duplicated logging | Use job context |

### 11.3 Site Module (78 files)

| Item | File | Issue | Solution |
|------|------|-------|----------|
| 1 | `services/site_service.go:12-27` | 5 direct module imports | Use interfaces |
| 2 | `services/site_service.go:84-88` | Silent error | Add logging |
| 3 | `handlers/site_handler.go:64-90` | Handler normalization | Move to DTO |
| 4 | `handlers/site_handler.go:337-346` | Domain logic in handler | Move to service |

### 11.4 Database Module (20 files)

| Item | File | Issue | Solution |
|------|------|-------|----------|
| 1 | `repositories/database_repository.go:21-51` | Duplicate FindBy* | Use scopes |
| 2 | `jobs/context.go` | 182 lines of context | Use pkg/jobs |

### 11.5 Other Modules

Each module needs similar audit for:
- [ ] Error handling consistency
- [ ] Handler boilerplate
- [ ] Service dependencies
- [ ] Constants extraction
- [ ] Logging additions

---

## 12. New Package Proposals

### 12.1 `internal/pkg/fiber/`
**Purpose:** Fiber-specific helpers

**Files to create:**
- `context.go` - Safe context extraction
- `request.go` - Request parsing helpers
- `errors.go` - Error-to-response mapping
- `middleware.go` - Common middleware helpers

### 12.2 `internal/pkg/dto/`
**Purpose:** DTO utilities

**Files to create:**
- `time.go` - Time formatting
- `converter.go` - Generic converters
- `normalizer.go` - Input normalization

### 12.3 `internal/pkg/handler/`
**Purpose:** Handler base and utilities

**Files to create:**
- `base.go` - Handler base class
- `helpers.go` - Common handler operations

### 12.4 `internal/pkg/config/`
**Purpose:** Configuration utilities

**Files to create:**
- `loader.go` - Generic config loading

### 12.5 `internal/pkg/domain/`
**Purpose:** Domain utilities (extracted from handlers)

**Files to create:**
- `domain.go` - Domain parsing/validation
- `url.go` - URL utilities

---

## Implementation Priority

### Phase 1: Critical (Week 1-2) ✅
1. [x] Safe context extraction (P0 - 1.1)
2. [x] Path parameter validation (P0 - 1.2)
3. [x] Silent error audit (P0 - 1.3) - Complete (58 instances fixed)

### Phase 2: Infrastructure (Week 3-4)
4. [x] Generic job context (P1 - 2.1) - Complete (pkgjobs.Base embedded in all 5 module job contexts)
5. [x] Service factory (P1 - 2.2) - Complete (service.Dependencies embedded in module ServiceDeps)
6. [~] Handler boilerplate extraction (P1 - 3.1) - In progress (helpers created, 2 handlers refactored)
7. [x] Handler base class (P1 - 3.2)

### Phase 3: Services (Week 5-6)
8. [ ] Interface-based dependencies (P1 - 4.1)
9. [ ] Service method consistency (P1 - 4.2)
10. [ ] DTO normalization (P1 - 4.3)

### Phase 4: Repository & Tasks (Week 7-8)
11. [ ] Repository scopes (P2 - 5.2)
12. [ ] Task runner to pkg (P2 - 6.1)
13. [ ] Task templates consolidation (P1 - 2.4)

### Phase 5: Polish (Week 9-10)
14. [ ] Time format helpers (P2 - 7.1)
15. [ ] Error handling standardization (P2 - 8.1)
16. [ ] Constants extraction (P3 - 9.1)
17. [ ] Logging additions (P3 - 10.1)

---

## Success Metrics

After completing this refactoring:

1. **Code Duplication:** Reduce handler boilerplate by 70%
2. **Type Safety:** 100% safe context extraction
3. **Error Handling:** Consistent error responses across all endpoints
4. **Test Coverage:** Easier to test with interface-based dependencies
5. **Maintainability:** Clear module boundaries, no circular imports
6. **Consistency:** All modules follow same patterns

---

## Notes

- Always run tests after each refactoring step
- Create feature branches for each major change
- Update CLAUDE.md with new patterns
- Consider backward compatibility for API responses
- Document breaking changes

---

*This document should be updated as refactoring progresses.*
