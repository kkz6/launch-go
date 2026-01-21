# Go Struct Embedding Consolidation Plan

A comprehensive plan for leveraging Go's struct embedding to reduce code duplication, following Laravel's trait-like composition patterns.

**Generated:** 2026-01-21
**Focus:** Struct embedding & composition patterns (NOT covered in TODO-REFACTORING.md)
**Estimated LOC Reduction:** 4000+ lines

---

## Table of Contents

1. [WebSocket Handler Base Embedding (P1)](#1-websocket-handler-base-embedding-p1)
2. [Webhook Handler Base Embedding (P1)](#2-webhook-handler-base-embedding-p1)
3. [Repository BaseRepository Enforcement (P1)](#3-repository-baserepository-enforcement-p1)
4. [Service Base Enforcement (P1)](#4-service-base-enforcement-p1)
5. [Module Handler Field Consolidation (P1)](#5-module-handler-field-consolidation-p1)
6. [Job Base Payload Generic (P2)](#6-job-base-payload-generic-p2)
7. [Installable Repository Mixin (P2)](#7-installable-repository-mixin-p2)
8. [Queue Dispatch Unification (P2)](#8-queue-dispatch-unification-p2)
9. [Pagination Embedding (P2)](#9-pagination-embedding-p2)
10. [Task Builder Pattern (P2)](#10-task-builder-pattern-p2)
11. [Model Scoped Fields Adoption (P2)](#11-model-scoped-fields-adoption-p2)
12. [Activity Logging Builder (P2)](#12-activity-logging-builder-p2)
13. [Broadcast Payload Builder (P3)](#13-broadcast-payload-builder-p3)
14. [DTO Timestamp Embedding (P3)](#14-dto-timestamp-embedding-p3)

---

## 1. WebSocket Handler Base Embedding (P1)

**Issue:** All 5 WebSocket handlers have IDENTICAL struct field declarations.

**Files Affected:**
- `internal/modules/websocket/handlers/logs.go:23-38`
- `internal/modules/websocket/handlers/metrics.go:20-35`
- `internal/modules/websocket/handlers/service_status.go:42-47`
- `internal/modules/websocket/handlers/terminal.go:32-37`
- `internal/modules/websocket/handlers/script_execution.go:23-28`

**Current Pattern (repeated 5 times):**
```go
type LogsHandler struct {
    db              *gorm.DB
    jwtSecret       string
    logger          zerolog.Logger
    membershipCache *cache.TeamMembershipCache
}

func NewLogsHandler(db *gorm.DB, jwtSecret string, logger zerolog.Logger, membershipCache *cache.TeamMembershipCache) *LogsHandler {
    return &LogsHandler{
        db:              db,
        jwtSecret:       jwtSecret,
        logger:          logger,
        membershipCache: membershipCache,
    }
}
```

**Solution:** Create `internal/modules/websocket/handlers/base.go`
```go
package handlers

import (
    "github.com/rs/zerolog"
    "gorm.io/gorm"
    "github.com/kkz6/launch-go/internal/modules/websocket/cache"
)

// BaseWebSocketHandler provides common dependencies for all WebSocket handlers
type BaseWebSocketHandler struct {
    DB              *gorm.DB
    JWTSecret       string
    Logger          zerolog.Logger
    MembershipCache *cache.TeamMembershipCache
}

// NewBaseWebSocketHandler creates a new base handler with all dependencies
func NewBaseWebSocketHandler(db *gorm.DB, jwtSecret string, logger zerolog.Logger, cache *cache.TeamMembershipCache) BaseWebSocketHandler {
    return BaseWebSocketHandler{
        DB:              db,
        JWTSecret:       jwtSecret,
        Logger:          logger,
        MembershipCache: cache,
    }
}
```

**Refactored Handler:**
```go
type LogsHandler struct {
    BaseWebSocketHandler
}

func NewLogsHandler(base BaseWebSocketHandler) *LogsHandler {
    return &LogsHandler{BaseWebSocketHandler: base}
}
```

**Refactoring Steps:**
- [ ] Create `internal/modules/websocket/handlers/base.go`
- [ ] Refactor `LogsHandler` to embed base
- [ ] Refactor `MetricsHandler` to embed base
- [ ] Refactor `ServiceStatusHandler` to embed base
- [ ] Refactor `TerminalHandler` to embed base
- [ ] Refactor `ScriptExecutionHandler` to embed base
- [ ] Update `builder.go` to construct base once

**Impact:** ~50-75 lines eliminated, single point of change for dependencies

---

## 2. Webhook Handler Base Embedding (P1)

**Issue:** Webhook handlers repeat signer, logger, and signature verification logic.

**Files Affected:**
- `internal/modules/server/handlers/metrics_webhook_handler.go:39-54`
- `internal/modules/server/handlers/task_webhook_handler.go:28-58`
- `internal/modules/git/handlers/webhook_handler.go:24-44`

**Current Pattern:**
```go
type MetricsWebhookHandler struct {
    repo   metricsWebhookRepository
    signer *signedurl.Signer
    hub    *websocket.Hub
    logger *zerolog.Logger
}

// Repeated in task_webhook_handler.go
type TaskWebhookHandler struct {
    registry *repositories.Registry
    queue    *queue.Client
    signer   *signedurl.Signer
    db       *gorm.DB
    // ...
    logger   *zerolog.Logger
}
```

**Solution:** Create `internal/pkg/webhook/base.go`
```go
package webhook

import (
    "github.com/gofiber/fiber/v2"
    "github.com/rs/zerolog"
    "github.com/kkz6/launch-go/internal/pkg/signedurl"
)

// BaseWebhookHandler provides common webhook handling infrastructure
type BaseWebhookHandler struct {
    Signer *signedurl.Signer
    Logger *zerolog.Logger
}

// NewBaseWebhookHandler creates base with signer from secret key
func NewBaseWebhookHandler(secretKey string, logger *zerolog.Logger) BaseWebhookHandler {
    return BaseWebhookHandler{
        Signer: signedurl.NewSigner(secretKey),
        Logger: logger,
    }
}

// VerifySignature validates the request signature
func (h *BaseWebhookHandler) VerifySignature(c *fiber.Ctx) bool {
    return signedurl.ValidateSignedURL(c, h.Signer)
}

// LogWebhookReceived logs incoming webhook
func (h *BaseWebhookHandler) LogWebhookReceived(c *fiber.Ctx, webhookType string) {
    h.Logger.Debug().
        Str("type", webhookType).
        Str("path", c.Path()).
        Str("ip", c.IP()).
        Msg("Webhook received")
}

// LogWebhookError logs webhook processing error
func (h *BaseWebhookHandler) LogWebhookError(c *fiber.Ctx, err error, msg string) {
    h.Logger.Error().
        Err(err).
        Str("path", c.Path()).
        Msg(msg)
}
```

**Refactored Handler:**
```go
type MetricsWebhookHandler struct {
    webhook.BaseWebhookHandler
    repo metricsWebhookRepository
    hub  *websocket.Hub
}

func NewMetricsWebhookHandler(secretKey string, logger *zerolog.Logger, repo metricsWebhookRepository, hub *websocket.Hub) *MetricsWebhookHandler {
    return &MetricsWebhookHandler{
        BaseWebhookHandler: webhook.NewBaseWebhookHandler(secretKey, logger),
        repo:               repo,
        hub:                hub,
    }
}
```

**Refactoring Steps:**
- [ ] Create `internal/pkg/webhook/base.go`
- [ ] Add `VerifySignature()` helper method
- [ ] Add logging helper methods
- [ ] Refactor `MetricsWebhookHandler` to embed
- [ ] Refactor `TaskWebhookHandler` to embed
- [ ] Refactor git `WebhookHandler` to embed

**Impact:** ~30-40 lines eliminated, centralized signature verification

---

## 3. Repository BaseRepository Enforcement (P1)

**Issue:** Many repositories define `db *gorm.DB` field directly instead of embedding BaseRepository.

**Files Affected:**
- `internal/modules/auth/repositories/user_repository.go:14-15`
- `internal/modules/auth/repositories/team_repository.go` (implied)
- `internal/modules/notification/repositories/notification_channel_repository.go:20-21`
- `internal/modules/database/repositories/base.go:55-57`
- ~25 more repositories with bare `db *gorm.DB` field

**Current Pattern (Inconsistent):**
```go
// Auth module - NO base embedding
type UserRepository struct {
    db *gorm.DB
}

// Server module - Uses embedded base (CORRECT)
type ServerRepository struct {
    BaseRepository
}
```

**Solution:** Create central `internal/pkg/repository/base.go` and enforce usage
```go
package repository

import (
    "context"
    "gorm.io/gorm"
)

// BaseRepository provides common database operations for all repositories
type BaseRepository struct {
    db *gorm.DB
}

// NewBaseRepository creates a new base repository
func NewBaseRepository(db *gorm.DB) BaseRepository {
    return BaseRepository{db: db}
}

// DB returns the database connection
func (r *BaseRepository) DB() *gorm.DB {
    return r.db
}

// WithContext returns DB with context
func (r *BaseRepository) WithContext(ctx context.Context) *gorm.DB {
    return r.db.WithContext(ctx)
}

// Transaction executes function within a transaction
func (r *BaseRepository) Transaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
    return r.db.WithContext(ctx).Transaction(fn)
}
```

**Refactored Repository:**
```go
// auth/repositories/user_repository.go
type UserRepository struct {
    repository.BaseRepository
}

func NewUserRepository(db *gorm.DB) *UserRepository {
    return &UserRepository{
        BaseRepository: repository.NewBaseRepository(db),
    }
}

// Now use r.DB() or r.WithContext(ctx) instead of r.db
```

**Refactoring Steps:**
- [ ] Ensure `internal/pkg/repository/base.go` exists with all helpers
- [ ] Audit all 30+ repositories for `db *gorm.DB` field
- [ ] Refactor auth module repositories (5 files)
- [ ] Refactor notification module repositories (3 files)
- [ ] Refactor database module repositories (2 files)
- [ ] Refactor remaining modules
- [ ] Remove per-module BaseRepository definitions

**Impact:** ~450 lines eliminated (15 lines × 30 repos), consistent DB access pattern

---

## 4. Service Base Enforcement (P1)

**Issue:** Service base class usage is inconsistent across modules.

**Files Affected:**
- `internal/pkg/service/base.go` - Central base (exists but not universally used)
- `internal/modules/auth/services/service.go:15-18` - Has bare fields, no base
- `internal/modules/notification/services/base.go:54-57` - Defines own BaseService
- `internal/modules/server/services/base.go:40` - Has own base
- `internal/modules/database/services/base.go:38-50` - Uses `service.Base` (correct)
- `internal/modules/site/services/base.go:112-126` - Uses `service.Base` (correct)

**Current Pattern (Inconsistent):**
```go
// Auth module - NO embedding
type Service struct {
    repos        *repositories.Registry
    config       *config.Config
    logger       *zerolog.Logger
    emailService EmailService
    // ... more fields
}

// Notification module - Own base
type BaseService struct {
    deps   *ServiceDeps
    repos  *repositories.Registry
    logger *zerolog.Logger
}

// Database module - Uses pkg base (CORRECT)
type Service struct {
    service.Base
    repos      *repositories.Registry
    serverRepo ServerRepository
}
```

**Solution:** Enforce `internal/pkg/service/base.go` across ALL modules
```go
// Ensure pkg/service/base.go has all needed methods:
package service

type Base struct {
    queue       *queue.Client
    broadcaster broadcast.ModelBroadcaster
    logger      *zerolog.Logger
}

// All modules should embed:
type AuthService struct {
    service.Base
    repos  *repositories.Registry
    config *config.Config
}
```

**Refactoring Steps:**
- [ ] Audit `internal/pkg/service/base.go` for completeness
- [ ] Refactor auth module services to embed `service.Base`
- [ ] Refactor notification module to use pkg base (remove local BaseService)
- [ ] Refactor server module to use pkg base
- [ ] Remove all per-module `BaseService` definitions
- [ ] Update all constructors

**Impact:** ~200 lines eliminated, single source of service infrastructure

---

## 5. Module Handler Field Consolidation (P1)

**Issue:** Individual handlers within a module repeat the same service injection field.

**Files Affected (Auth Module - 9 handlers):**
- `internal/modules/auth/handlers/auth_handler.go:13-14`
- `internal/modules/auth/handlers/user_handler.go:13-14`
- `internal/modules/auth/handlers/email_handler.go:11-12`
- `internal/modules/auth/handlers/profile_handler.go`
- `internal/modules/auth/handlers/token_handler.go`
- `internal/modules/auth/handlers/password_handler.go`
- `internal/modules/auth/handlers/team_handler.go`
- `internal/modules/auth/handlers/two_factor_handler.go`
- `internal/modules/auth/handlers/invitation_handler.go`

**Current Pattern:**
```go
// auth_handler.go
type AuthHandler struct {
    service *services.Service
}

// user_handler.go
type UserHandler struct {
    service *services.Service
}

// email_handler.go (9+ files repeat this)
type EmailHandler struct {
    service *services.Service
}
```

**Solution:** Create per-module handler base
```go
// internal/modules/auth/handlers/base.go
package handlers

import "github.com/kkz6/launch-go/internal/modules/auth/services"

// BaseHandler provides common dependencies for auth handlers
type BaseHandler struct {
    service *services.Service
}

// Service returns the auth service
func (h *BaseHandler) Service() *services.Service {
    return h.service
}

// NewBaseHandler creates a base handler
func NewBaseHandler(service *services.Service) BaseHandler {
    return BaseHandler{service: service}
}
```

**Refactored Handlers:**
```go
// auth_handler.go
type AuthHandler struct {
    BaseHandler
}

func NewAuthHandler(service *services.Service) *AuthHandler {
    return &AuthHandler{BaseHandler: NewBaseHandler(service)}
}

// user_handler.go
type UserHandler struct {
    BaseHandler
}

// All other handlers follow same pattern
```

**Refactoring Steps:**
- [ ] Create `internal/modules/auth/handlers/base.go`
- [ ] Refactor all 9 auth handlers to embed base
- [ ] Create similar bases for other modules with multiple handlers
- [ ] Update builder registration

**Impact:** ~50 lines per module eliminated, easier dependency injection

---

## 6. Job Base Payload Generic (P2)

**Issue:** All jobs repeat context field and handler setup pattern.

**Files Affected:**
- `internal/modules/database/jobs/install_database.go:26-36`
- `internal/modules/database/jobs/uninstall_database.go:26-36`
- `internal/modules/database/jobs/update_database_user.go:26-36`
- `internal/modules/site/jobs/*.go` (10+ files)
- `internal/modules/server/jobs/*.go` (5+ files)

**Current Pattern:**
```go
type InstallDatabaseJob struct {
    ctx     *JobContext
    traits.InstallationTracker
    Payload InstallDatabasePayload
}

func NewInstallDatabaseJob(ctx *JobContext, payload InstallDatabasePayload) *InstallDatabaseJob {
    return &InstallDatabaseJob{
        ctx:     ctx,
        Payload: payload,
    }
}
```

**Solution:** Create generic job base `internal/pkg/jobs/base_job.go`
```go
package jobs

import "context"

// BaseJob provides common job structure with typed payload
type BaseJob[P any] struct {
    ctx     *JobContext
    Payload P
}

// NewBaseJob creates a new base job
func NewBaseJob[P any](ctx *JobContext, payload P) BaseJob[P] {
    return BaseJob[P]{
        ctx:     ctx,
        Payload: payload,
    }
}

// Context returns the job context
func (j *BaseJob[P]) Context() *JobContext {
    return j.ctx
}

// DB returns database connection
func (j *BaseJob[P]) DB() *gorm.DB {
    return j.ctx.DB()
}

// Queue returns the queue client
func (j *BaseJob[P]) Queue() *queue.Client {
    return j.ctx.Queue
}

// Logger returns the logger
func (j *BaseJob[P]) Logger() *zerolog.Logger {
    return j.ctx.Logger
}
```

**Refactored Job:**
```go
type InstallDatabaseJob struct {
    jobs.BaseJob[InstallDatabasePayload]
    traits.InstallationTracker
}

func NewInstallDatabaseJob(ctx *JobContext, payload InstallDatabasePayload) *InstallDatabaseJob {
    return &InstallDatabaseJob{
        BaseJob: jobs.NewBaseJob(ctx, payload),
    }
}
```

**Refactoring Steps:**
- [ ] Create `internal/pkg/jobs/base_job.go` with generics
- [ ] Refactor database module jobs (5 files)
- [ ] Refactor site module jobs (10+ files)
- [ ] Refactor server module jobs
- [ ] Update job registration

**Impact:** ~100 lines eliminated across 20+ job files

---

## 7. Installable Repository Mixin (P2)

**Issue:** Repositories with installable models repeat delegation to `Installable` trait.

**Files Affected:**
- `internal/modules/database/repositories/base.go:54-79`
- Similar patterns in server and site modules

**Current Pattern:**
```go
type DatabaseRepository struct {
    db          *gorm.DB
    installable repository.Installable[models.Database]
}

// Repeated delegation methods
func (r *DatabaseRepository) Create(ctx context.Context, database *models.Database) error {
    return r.installable.Create(ctx, database)
}

func (r *DatabaseRepository) Update(ctx context.Context, database *models.Database) error {
    return r.installable.Update(ctx, database)
}

func (r *DatabaseRepository) MarkAsInstalled(ctx context.Context, id string) error {
    return r.installable.MarkAsInstalled(ctx, id)
}
// ... 5+ more delegation methods
```

**Solution:** Create `internal/pkg/repository/installable_repository.go`
```go
package repository

import (
    "context"
    "gorm.io/gorm"
)

// InstallableRepository provides CRUD operations for installable models
type InstallableRepository[T any] struct {
    BaseRepository
    installable Installable[T]
}

// NewInstallableRepository creates repository with installable trait
func NewInstallableRepository[T any](db *gorm.DB) *InstallableRepository[T] {
    return &InstallableRepository[T]{
        BaseRepository: NewBaseRepository(db),
        installable:    NewInstallable[T](db),
    }
}

// All common methods implemented once
func (r *InstallableRepository[T]) Create(ctx context.Context, model *T) error {
    return r.installable.Create(ctx, model)
}

func (r *InstallableRepository[T]) Update(ctx context.Context, model *T) error {
    return r.installable.Update(ctx, model)
}

func (r *InstallableRepository[T]) MarkAsInstalled(ctx context.Context, id string) error {
    return r.installable.MarkAsInstalled(ctx, id)
}

func (r *InstallableRepository[T]) MarkAsInstallationFailed(ctx context.Context, id string) error {
    return r.installable.MarkAsInstallationFailed(ctx, id)
}

func (r *InstallableRepository[T]) MarkAsUninstallationRequested(ctx context.Context, id string) error {
    return r.installable.MarkAsUninstallationRequested(ctx, id)
}

func (r *InstallableRepository[T]) Delete(ctx context.Context, id string) error {
    return r.installable.Delete(ctx, id)
}
```

**Refactored Repository:**
```go
type DatabaseRepository struct {
    *repository.InstallableRepository[models.Database]
}

func NewDatabaseRepository(db *gorm.DB) *DatabaseRepository {
    return &DatabaseRepository{
        InstallableRepository: repository.NewInstallableRepository[models.Database](db),
    }
}

// Only add model-specific methods, all installable methods inherited
func (r *DatabaseRepository) FindByServer(ctx context.Context, serverID string) ([]models.Database, error) {
    // custom query
}
```

**Refactoring Steps:**
- [ ] Create `internal/pkg/repository/installable_repository.go`
- [ ] Refactor `DatabaseRepository` to embed
- [ ] Refactor `DatabaseUserRepository` to embed
- [ ] Audit other installable repositories
- [ ] Remove delegation boilerplate

**Impact:** ~200 lines eliminated across 5+ repositories

---

## 8. Queue Dispatch Unification (P2)

**Issue:** Job dispatching uses 3+ different patterns across codebase.

**Files Affected:**
- `internal/modules/site/tasks/deploy.go:320-331` - Has `dispatchJob()` helper
- `internal/modules/site/jobs/deploy.go:335-346` - Direct `Queue.Enqueue()`
- `internal/modules/server/handlers/task_webhook_handler.go:219-220` - Inline construction
- `internal/modules/server/tasks/runner.go:585-586` - Direct dispatch
- 50+ other locations

**Current Patterns:**
```go
// Pattern 1: Helper method (site/tasks/deploy.go)
func (t *deploySiteTask) dispatchJob(cbCtx *taskrunner.CallbackContext, jobType string, payload any, taskID string) {
    // implementation
}

// Pattern 2: Direct (site/jobs/deploy.go)
task := asynq.NewTask(jobType, data)
if _, err := j.ctx.Queue.Enqueue(task); err != nil { }

// Pattern 3: Inline (server/handlers/task_webhook_handler.go)
asynqTask := asynq.NewTask(jobRef.Type, jobRef.Payload)
if _, err := h.queue.Enqueue(asynqTask); err != nil { }
```

**Solution:** Create `internal/pkg/jobs/dispatcher.go`
```go
package jobs

import (
    "encoding/json"
    "github.com/hibiken/asynq"
    "github.com/rs/zerolog"
    "github.com/kkz6/launch-go/internal/pkg/queue"
)

// Dispatcher provides unified job dispatching
type Dispatcher struct {
    queue  *queue.Client
    logger *zerolog.Logger
}

// NewDispatcher creates a new job dispatcher
func NewDispatcher(queue *queue.Client, logger *zerolog.Logger) *Dispatcher {
    return &Dispatcher{queue: queue, logger: logger}
}

// Dispatch sends a job to the queue
func (d *Dispatcher) Dispatch(jobType string, payload any, opts ...asynq.Option) error {
    data, err := json.Marshal(payload)
    if err != nil {
        d.logger.Error().Err(err).Str("job_type", jobType).Msg("Failed to marshal job payload")
        return err
    }

    task := asynq.NewTask(jobType, data, opts...)
    info, err := d.queue.Enqueue(task)
    if err != nil {
        d.logger.Error().Err(err).Str("job_type", jobType).Msg("Failed to enqueue job")
        return err
    }

    d.logger.Debug().
        Str("job_type", jobType).
        Str("task_id", info.ID).
        Str("queue", info.Queue).
        Msg("Job dispatched")

    return nil
}

// DispatchWithDelay sends a job with delay
func (d *Dispatcher) DispatchWithDelay(jobType string, payload any, delay time.Duration, opts ...asynq.Option) error {
    opts = append(opts, asynq.ProcessIn(delay))
    return d.Dispatch(jobType, payload, opts...)
}

// DispatchToQueue sends a job to specific queue
func (d *Dispatcher) DispatchToQueue(jobType string, payload any, queueName string, opts ...asynq.Option) error {
    opts = append(opts, asynq.Queue(queueName))
    return d.Dispatch(jobType, payload, opts...)
}
```

**Usage:**
```go
// Before
data, _ := json.Marshal(payload)
task := asynq.NewTask(jobType, data)
queue.Enqueue(task)

// After
dispatcher.Dispatch(jobType, payload)
```

**Refactoring Steps:**
- [ ] Create `internal/pkg/jobs/dispatcher.go`
- [ ] Add to service base or job context
- [ ] Refactor site tasks to use dispatcher
- [ ] Refactor server handlers to use dispatcher
- [ ] Refactor all direct `Enqueue()` calls
- [ ] Add logging and error handling uniformly

**Impact:** Centralized logging, consistent error handling, ~100 lines of dispatch code consolidated

---

## 9. Pagination Embedding (P2)

**Issue:** Pagination logic is ad-hoc and not reusable across repositories.

**Files Affected:**
- `internal/modules/server/repositories/server_repository.go:93-100`
- `internal/modules/server/services/server_service.go:35`
- Most repositories lack pagination

**Current Pattern:**
```go
func (r *ServerRepository) FindAllByTeamPaginated(ctx context.Context, teamID string, page, perPage int) ([]models.Server, int64, error) {
    var servers []models.Server
    var total int64

    query := r.db.WithContext(ctx).Where("team_id = ?", teamID)
    if err := query.Model(&models.Server{}).Count(&total).Error; err != nil {
        return nil, 0, err
    }

    offset := (page - 1) * perPage
    if err := query.Offset(offset).Limit(perPage).Find(&servers).Error; err != nil {
        return nil, 0, err
    }

    return servers, total, nil
}
```

**Solution:** Add pagination to `internal/pkg/repository/base.go`
```go
package repository

// PaginatedResult holds paginated query results
type PaginatedResult[T any] struct {
    Data       []T   `json:"data"`
    Total      int64 `json:"total"`
    Page       int   `json:"page"`
    PerPage    int   `json:"per_page"`
    TotalPages int   `json:"total_pages"`
}

// Paginate executes a paginated query
func (r *BaseRepository) Paginate[T any](ctx context.Context, query *gorm.DB, page, perPage int) (*PaginatedResult[T], error) {
    var total int64
    if err := query.Count(&total).Error; err != nil {
        return nil, err
    }

    var results []T
    offset := (page - 1) * perPage
    if err := query.Offset(offset).Limit(perPage).Find(&results).Error; err != nil {
        return nil, err
    }

    totalPages := int(total) / perPage
    if int(total)%perPage > 0 {
        totalPages++
    }

    return &PaginatedResult[T]{
        Data:       results,
        Total:      total,
        Page:       page,
        PerPage:    perPage,
        TotalPages: totalPages,
    }, nil
}
```

**Refactored Usage:**
```go
func (r *ServerRepository) FindAllByTeamPaginated(ctx context.Context, teamID string, page, perPage int) (*repository.PaginatedResult[models.Server], error) {
    query := r.WithContext(ctx).Where("team_id = ?", teamID)
    return r.Paginate[models.Server](ctx, query, page, perPage)
}
```

**Refactoring Steps:**
- [ ] Add `Paginate` method to `BaseRepository`
- [ ] Create `PaginatedResult` struct
- [ ] Refactor `ServerRepository.FindAllByTeamPaginated`
- [ ] Add pagination to other repositories as needed
- [ ] Standardize pagination in handlers

**Impact:** Consistent pagination, ~50 lines of reusable code

---

## 10. Task Builder Pattern (P2)

**Issue:** Each task file repeats callback data struct and task creation boilerplate.

**Files Affected:**
- `internal/modules/site/tasks/deploy.go:52-120`
- `internal/modules/site/tasks/run_command.go`
- `internal/modules/server/tasks/*.go` (20+ files)
- `internal/modules/database/tasks/*.go`

**Current Pattern:**
```go
// callback data struct
type callbackData struct {
    SiteID       string `json:"site_id"`
    ServerID     string `json:"server_id"`
    DeploymentID string `json:"deployment_id"`
    // ...
}

// task struct
type deploySiteTask struct {
    *taskrunner.BaseTask
    opts     DeployOptions
    callback callbackData
}

// constructor
func DeploySiteTask(opts DeployOptions) *deploySiteTask {
    return &deploySiteTask{
        BaseTask: taskrunner.NewBaseTask(
            taskrunner.WithName("Deploy Site"),
            taskrunner.WithScript(buildScript(opts)),
            taskrunner.WithTimeoutSeconds(600),
        ),
        opts: opts,
        callback: callbackData{
            SiteID:       opts.Site.ID,
            ServerID:     opts.Site.ServerID,
            // ...
        },
    }
}
```

**Solution:** Create `internal/pkg/taskrunner/task_builder.go`
```go
package taskrunner

// TaskBuilder provides fluent interface for creating tasks
type TaskBuilder[C any] struct {
    name     string
    script   string
    timeout  int
    callback C
    handlers CallbackHandlers
}

// CallbackHandlers holds callback functions
type CallbackHandlers struct {
    OnSuccess func(ctx context.Context, cbCtx *CallbackContext, taskID string) error
    OnFailure func(ctx context.Context, cbCtx *CallbackContext, taskID string, exitCode int) error
    OnExpired func(ctx context.Context, cbCtx *CallbackContext, taskID string) error
}

// NewTaskBuilder creates a new task builder
func NewTaskBuilder[C any]() *TaskBuilder[C] {
    return &TaskBuilder[C]{
        timeout: 300, // default 5 minutes
    }
}

func (b *TaskBuilder[C]) WithName(name string) *TaskBuilder[C] {
    b.name = name
    return b
}

func (b *TaskBuilder[C]) WithScript(script string) *TaskBuilder[C] {
    b.script = script
    return b
}

func (b *TaskBuilder[C]) WithTimeout(seconds int) *TaskBuilder[C] {
    b.timeout = seconds
    return b
}

func (b *TaskBuilder[C]) WithCallback(callback C) *TaskBuilder[C] {
    b.callback = callback
    return b
}

func (b *TaskBuilder[C]) OnSuccess(fn func(ctx context.Context, cbCtx *CallbackContext, taskID string) error) *TaskBuilder[C] {
    b.handlers.OnSuccess = fn
    return b
}

func (b *TaskBuilder[C]) OnFailure(fn func(ctx context.Context, cbCtx *CallbackContext, taskID string, exitCode int) error) *TaskBuilder[C] {
    b.handlers.OnFailure = fn
    return b
}

func (b *TaskBuilder[C]) Build() *BuiltTask[C] {
    return &BuiltTask[C]{
        BaseTask: NewBaseTask(
            WithName(b.name),
            WithScript(b.script),
            WithTimeoutSeconds(b.timeout),
        ),
        callback: b.callback,
        handlers: b.handlers,
    }
}
```

**Refactored Task:**
```go
func DeploySiteTask(opts DeployOptions) *taskrunner.BuiltTask[callbackData] {
    return taskrunner.NewTaskBuilder[callbackData]().
        WithName("Deploy Site").
        WithScript(buildDeployScript(opts)).
        WithTimeout(600).
        WithCallback(callbackData{
            SiteID:       opts.Site.ID,
            ServerID:     opts.Site.ServerID,
            DeploymentID: opts.Deployment.ID,
        }).
        OnSuccess(func(ctx context.Context, cbCtx *taskrunner.CallbackContext, taskID string) error {
            // success handling
            return nil
        }).
        OnFailure(func(ctx context.Context, cbCtx *taskrunner.CallbackContext, taskID string, exitCode int) error {
            // failure handling
            return nil
        }).
        Build()
}
```

**Refactoring Steps:**
- [ ] Create `internal/pkg/taskrunner/task_builder.go`
- [ ] Create `BuiltTask` generic type
- [ ] Refactor deploy task as proof of concept
- [ ] Refactor remaining site tasks
- [ ] Refactor server tasks
- [ ] Update task registration

**Impact:** ~600 lines eliminated across 30+ task files, consistent task creation

---

## 11. Model Scoped Fields Adoption (P2)

**Issue:** Models define scope fields manually instead of using existing mixins.

**Files Affected:**
- `internal/modules/database/models/database.go:11-12`
- `internal/modules/database/models/database_user.go:13-14`
- `internal/modules/site/models/site.go`
- 30+ other models

**Existing Mixins (Not Used):**
```go
// internal/pkg/models/base.go:124-142
type ServerScopedModel struct {
    ServerID string `gorm:"column:server_id;type:char(26);not null;index" json:"server_id"`
}

type TeamScopedModel struct {
    TeamID string `gorm:"column:team_id;type:char(26);not null;index" json:"team_id"`
}

type SiteScopedModel struct {
    SiteID string `gorm:"column:site_id;type:char(26);not null;index" json:"site_id"`
}
```

**Current Pattern (NOT using mixins):**
```go
type Database struct {
    basemodels.BaseModel
    basemodels.InstallableModel
    ServerID string `gorm:"column:server_id;type:char(26);not null;index" json:"server_id"`
    TeamID   string `gorm:"column:team_id;type:char(26);not null;index" json:"team_id"`
    Name     string `gorm:"type:varchar(255);not null" json:"name"`
}
```

**Should Be (using mixins):**
```go
type Database struct {
    basemodels.BaseModel
    basemodels.InstallableModel
    basemodels.ServerScopedModel
    basemodels.TeamScopedModel
    Name string `gorm:"type:varchar(255);not null" json:"name"`
}
```

**Refactoring Steps:**
- [ ] Audit `internal/pkg/models/base.go` for all available mixins
- [ ] Refactor `database/models/database.go` to use mixins
- [ ] Refactor `database/models/database_user.go` to use mixins
- [ ] Audit and refactor all other models
- [ ] Ensure GORM handles embedded fields correctly

**Impact:** Improved clarity, consistent field definitions, ~100 lines eliminated

---

## 12. Activity Logging Builder (P2)

**Issue:** Activity logging pattern repeated with builder chain across 40+ locations.

**Files Affected:**
- `internal/modules/database/services/database_service.go:36-44`
- `internal/modules/database/jobs/install_database.go:79-87`
- Similar patterns in site, server, auth modules

**Current Pattern:**
```go
logger := activity.New(s.repos.DB()).
    WithContext(ctx).
    UseLog("database").
    On(database).
    WithEvent("created")
if userID != nil {
    logger.CausedByUser(*userID)
}
logger.Log("Database was created")
```

**Solution:** Create standardized activity patterns `internal/pkg/activity/helpers.go`
```go
package activity

// LogCreation logs a creation event
func LogCreation(db *gorm.DB, ctx context.Context, model interface{}, logName string, userID *string, message string) error {
    logger := New(db).
        WithContext(ctx).
        UseLog(logName).
        On(model).
        WithEvent("created")

    if userID != nil {
        logger.CausedByUser(*userID)
    }

    return logger.Log(message)
}

// LogUpdate logs an update event
func LogUpdate(db *gorm.DB, ctx context.Context, model interface{}, logName string, userID *string, message string) error {
    // similar pattern
}

// LogDeletion logs a deletion event
func LogDeletion(db *gorm.DB, ctx context.Context, model interface{}, logName string, userID *string, message string) error {
    // similar pattern
}

// LogWithProperties logs with additional properties
func LogWithProperties(db *gorm.DB, ctx context.Context, model interface{}, logName, event string, userID *string, props map[string]interface{}, message string) error {
    logger := New(db).
        WithContext(ctx).
        UseLog(logName).
        On(model).
        WithEvent(event).
        WithProperties(props)

    if userID != nil {
        logger.CausedByUser(*userID)
    }

    return logger.Log(message)
}
```

**Refactored Usage:**
```go
// Before
logger := activity.New(s.repos.DB()).
    WithContext(ctx).
    UseLog("database").
    On(database).
    WithEvent("created")
if userID != nil {
    logger.CausedByUser(*userID)
}
logger.Log("Database was created")

// After
activity.LogCreation(s.repos.DB(), ctx, database, "database", userID, "Database was created")
```

**Refactoring Steps:**
- [ ] Create `internal/pkg/activity/helpers.go`
- [ ] Add `LogCreation`, `LogUpdate`, `LogDeletion` helpers
- [ ] Refactor database services
- [ ] Refactor site services
- [ ] Refactor server services
- [ ] Refactor jobs

**Impact:** ~150 lines eliminated, consistent logging pattern

---

## 13. Broadcast Payload Builder (P3)

**Issue:** Broadcast payloads created inline with inconsistent structure.

**Files Affected:**
- `internal/modules/server/handlers/metrics_webhook_handler.go:152-167`
- Multiple services with broadcast calls

**Current Pattern:**
```go
h.hub.BroadcastToTeam(server.TeamID, "server.metrics", map[string]interface{}{
    "server_id": serverID,
    "load":      req.Data.Load,
    "memory": map[string]interface{}{
        "total": memoryTotal,
        "used":  memoryUsed,
        "free":  memoryFree,
    },
    "disk": map[string]interface{}{
        "total": diskTotal,
        "used":  diskUsed,
        "free":  diskFree,
    },
})
```

**Solution:** Create typed broadcast payloads
```go
// internal/pkg/broadcast/payloads.go
package broadcast

// ServerMetricsPayload is the typed payload for server metrics events
type ServerMetricsPayload struct {
    ServerID string          `json:"server_id"`
    Load     float64         `json:"load"`
    Memory   ResourceMetrics `json:"memory"`
    Disk     ResourceMetrics `json:"disk"`
}

type ResourceMetrics struct {
    Total int64 `json:"total"`
    Used  int64 `json:"used"`
    Free  int64 `json:"free"`
}

// ToMap converts to map for broadcasting
func (p ServerMetricsPayload) ToMap() map[string]interface{} {
    return map[string]interface{}{
        "server_id": p.ServerID,
        "load":      p.Load,
        "memory": map[string]interface{}{
            "total": p.Memory.Total,
            "used":  p.Memory.Used,
            "free":  p.Memory.Free,
        },
        "disk": map[string]interface{}{
            "total": p.Disk.Total,
            "used":  p.Disk.Used,
            "free":  p.Disk.Free,
        },
    }
}
```

**Refactoring Steps:**
- [ ] Identify all broadcast event types
- [ ] Create typed payload structs in `internal/pkg/broadcast/payloads.go`
- [ ] Refactor metrics webhook handler
- [ ] Refactor other broadcast calls
- [ ] Add payload validation

**Impact:** Type safety, IDE autocompletion, consistent structure

---

## 14. DTO Timestamp Embedding (P3)

**Issue:** Timestamp formatting repeated 81+ times in DTO converters.

**Files Affected:**
- `internal/modules/database/dto/responses.go:52-85`
- `internal/modules/server/dto/responses.go:293-330`
- All other module DTOs

**Current Pattern:**
```go
if db.InstalledAt != nil {
    t := db.InstalledAt.Format(time.RFC3339)
    resp.InstalledAt = &t
}

if db.InstallationFailedAt != nil {
    t := db.InstallationFailedAt.Format(time.RFC3339)
    resp.InstallationFailedAt = &t
}
// Repeated 81+ times
```

**Solution:** Create `internal/pkg/dto/time.go` (as mentioned in TODO-REFACTORING.md but extending with embedding)
```go
package dto

import "time"

// TimeFormatter handles time-to-string conversion
type TimeFormatter time.Time

func (t TimeFormatter) RFC3339() string {
    return time.Time(t).Format(time.RFC3339)
}

func (t TimeFormatter) RFC3339Ptr() *string {
    s := time.Time(t).Format(time.RFC3339)
    return &s
}

// FormatTimePtr converts *time.Time to *string
func FormatTimePtr(t *time.Time) *string {
    if t == nil {
        return nil
    }
    s := t.Format(time.RFC3339)
    return &s
}

// TimestampFields provides common timestamp response fields
// Embed in response DTOs that have these fields
type TimestampFields struct {
    CreatedAt string  `json:"created_at"`
    UpdatedAt string  `json:"updated_at"`
    DeletedAt *string `json:"deleted_at,omitempty"`
}

// FromModel populates timestamps from a model
func (f *TimestampFields) FromModel(createdAt, updatedAt time.Time, deletedAt *time.Time) {
    f.CreatedAt = createdAt.Format(time.RFC3339)
    f.UpdatedAt = updatedAt.Format(time.RFC3339)
    f.DeletedAt = FormatTimePtr(deletedAt)
}

// InstallableTimestampFields provides installable model timestamp fields
type InstallableTimestampFields struct {
    InstalledAt               *string `json:"installed_at,omitempty"`
    InstallationFailedAt      *string `json:"installation_failed_at,omitempty"`
    UninstallationRequestedAt *string `json:"uninstallation_requested_at,omitempty"`
}

// FromInstallable populates from installable model fields
func (f *InstallableTimestampFields) FromInstallable(installedAt, failedAt, uninstallReqAt *time.Time) {
    f.InstalledAt = FormatTimePtr(installedAt)
    f.InstallationFailedAt = FormatTimePtr(failedAt)
    f.UninstallationRequestedAt = FormatTimePtr(uninstallReqAt)
}
```

**Refactored DTO:**
```go
type DatabaseResponse struct {
    ID     string `json:"id"`
    Name   string `json:"name"`

    dto.TimestampFields
    dto.InstallableTimestampFields
}

func ToDatabaseResponse(db *models.Database) DatabaseResponse {
    resp := DatabaseResponse{
        ID:   db.ID,
        Name: db.Name,
    }
    resp.TimestampFields.FromModel(db.CreatedAt, db.UpdatedAt, nil)
    resp.InstallableTimestampFields.FromInstallable(db.InstalledAt, db.InstallationFailedAt, db.UninstallationRequestedAt)
    return resp
}
```

**Refactoring Steps:**
- [ ] Create `internal/pkg/dto/time.go`
- [ ] Create `TimestampFields` embeddable struct
- [ ] Create `InstallableTimestampFields` embeddable struct
- [ ] Refactor database DTOs
- [ ] Refactor server DTOs
- [ ] Refactor all other module DTOs

**Impact:** ~400 lines eliminated, consistent timestamp handling

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

## 15. Model NamedModel Mixin (P1)

**Issue:** 13+ models repeat Name + Description field definitions.

**Files Affected:**
- `internal/modules/server/models/server.go:22-23`
- `internal/modules/server/models/ssh_key.go:18-20`
- `internal/modules/server/models/firewall_rule.go:18-23`
- `internal/modules/billing/models/plan.go:5-7`
- `internal/modules/dns/models/domain.go:13-14`
- `internal/modules/backup/models/storage_provider.go:16`
- `internal/modules/script/models/script.go:13`

**Current Pattern:**
```go
// Repeated in 13+ models
Name        string  `gorm:"type:varchar(255);not null" json:"name"`
Description *string `gorm:"type:varchar(255)" json:"description,omitempty"`
```

**Solution:** Create `internal/pkg/models/named.go`
```go
package models

// NamedModel provides Name and Description fields for models
type NamedModel struct {
    Name        string  `gorm:"type:varchar(255);not null" json:"name"`
    Description *string `gorm:"type:text" json:"description,omitempty"`
}

// SetName sets the name field
func (m *NamedModel) SetName(name string) {
    m.Name = name
}

// SetDescription sets the description field
func (m *NamedModel) SetDescription(desc *string) {
    m.Description = desc
}
```

**Refactored Model:**
```go
type Server struct {
    basemodels.BaseModel
    basemodels.NamedModel
    basemodels.TeamScopedModel
    // ... other fields
}
```

**Refactoring Steps:**
- [ ] Create `internal/pkg/models/named.go`
- [ ] Refactor server model to embed `NamedModel`
- [ ] Refactor ssh_key model
- [ ] Refactor firewall_rule model
- [ ] Refactor all other affected models

**Impact:** ~26 lines eliminated, consistent naming pattern

---

## 16. Model StateTrackingModel Mixin (P2)

**Issue:** 10+ models track state transitions with similar timestamp fields.

**Files Affected:**
- `internal/modules/server/models/server.go:46-47` - ProvisionedAt, LastUpdateCheck, LastConnectivityCheck
- `internal/modules/server/models/daemon.go:23` - LastStatusCheck
- `internal/modules/site/models/queue.go:37` - LastStatusCheck
- `internal/modules/git/models/source_control.go:27-33` - ConnectedAt, LastSyncedAt, TokenExpiresAt

**Current Pattern:**
```go
// Repeated across models
LastCheckedAt *time.Time `gorm:"column:last_checked_at" json:"last_checked_at,omitempty"`
LastSyncedAt  *time.Time `gorm:"column:last_synced_at" json:"last_synced_at,omitempty"`
ConnectedAt   *time.Time `gorm:"column:connected_at" json:"connected_at,omitempty"`
```

**Solution:** Create `internal/pkg/models/state_tracking.go`
```go
package models

import "time"

// StateTrackingModel provides common state/sync tracking timestamps
type StateTrackingModel struct {
    LastCheckedAt *time.Time `gorm:"column:last_checked_at;type:timestamp null" json:"last_checked_at,omitempty"`
    LastSyncedAt  *time.Time `gorm:"column:last_synced_at;type:timestamp null" json:"last_synced_at,omitempty"`
}

// MarkChecked updates the last checked timestamp
func (m *StateTrackingModel) MarkChecked() {
    now := time.Now()
    m.LastCheckedAt = &now
}

// MarkSynced updates the last synced timestamp
func (m *StateTrackingModel) MarkSynced() {
    now := time.Now()
    m.LastSyncedAt = &now
}

// ConnectableModel for models tracking connection state
type ConnectableModel struct {
    ConnectedAt *time.Time `gorm:"column:connected_at;type:timestamp null" json:"connected_at,omitempty"`
}

// MarkConnected updates the connected timestamp
func (m *ConnectableModel) MarkConnected() {
    now := time.Now()
    m.ConnectedAt = &now
}
```

**Impact:** ~30 lines eliminated, consistent state tracking

---

## 17. Model ProgressTrackingModel Mixin (P2)

**Issue:** 3 models track operation progress with identical fields.

**Files Affected:**
- `internal/modules/server/models/server.go:52-53`
- `internal/modules/site/models/site.go:58`

**Current Pattern:**
```go
Progress     int     `gorm:"type:int;not null;default:0" json:"progress,omitempty"`
ProgressStep *string `gorm:"column:progress_step;type:varchar(255)" json:"progress_step,omitempty"`
```

**Solution:** Create `internal/pkg/models/progress.go`
```go
package models

// ProgressTrackingModel provides progress tracking for long-running operations
type ProgressTrackingModel struct {
    Progress     int     `gorm:"type:int;not null;default:0" json:"progress"`
    ProgressStep *string `gorm:"column:progress_step;type:varchar(255)" json:"progress_step,omitempty"`
}

// SetProgress updates progress percentage
func (m *ProgressTrackingModel) SetProgress(percent int, step string) {
    m.Progress = percent
    if step != "" {
        m.ProgressStep = &step
    }
}

// ResetProgress clears progress tracking
func (m *ProgressTrackingModel) ResetProgress() {
    m.Progress = 0
    m.ProgressStep = nil
}

// IsComplete returns true if progress is 100
func (m *ProgressTrackingModel) IsComplete() bool {
    return m.Progress >= 100
}
```

**Impact:** ~10 lines eliminated, reusable progress tracking

---

## 18. Compound Scoping Mixins (P2)

**Issue:** Models frequently combine the same scope fields.

**Files Affected:**
- Site-related models: Site, Certificate, Command, Queue, Redirect, Deployment
- Server-related models: Database, DatabaseUser, Cron, Daemon, FirewallRule

**Current Pattern:**
```go
// Repeated for site resources
SiteID   string `gorm:"column:site_id;type:char(26);not null;index" json:"site_id"`
TeamID   string `gorm:"column:team_id;type:char(26);not null;index" json:"team_id"`
UserID   string `gorm:"column:user_id;type:char(26);not null;index" json:"user_id"`
```

**Solution:** Create compound mixins `internal/pkg/models/scopes.go`
```go
package models

// SiteResourceModel for resources belonging to a site
type SiteResourceModel struct {
    SiteScopedModel
    TeamScopedModel
    UserScopedModel
}

// ServerResourceModel for resources belonging to a server
type ServerResourceModel struct {
    ServerScopedModel
    TeamScopedModel
}

// TeamResourceModel for team-owned resources with user tracking
type TeamResourceModel struct {
    TeamScopedModel
    UserScopedModel
}
```

**Refactored Model:**
```go
type Certificate struct {
    basemodels.BaseModel
    basemodels.SiteResourceModel  // Includes SiteID, TeamID, UserID
    Domain string `json:"domain"`
    // ...
}
```

**Impact:** ~100 lines eliminated across 15+ models

---

## 19. Cloud Provider BaseAPIClient (P1)

**Issue:** All cloud providers repeat identical HTTP request/response handling.

**Files Affected:**
- `internal/modules/server/providers/digitalocean.go:263-306` - doRequest()
- `internal/modules/server/providers/hetzner.go:222-256` - doRequest()
- `internal/modules/server/providers/linode.go:211-245` - doRequest()
- `internal/modules/server/providers/vultr.go:218-252` - doRequest()

**Current Pattern (repeated 4 times with slight variations):**
```go
func (p *DigitalOceanProvider) doRequest(ctx context.Context, token, method, path string, body interface{}) (map[string]interface{}, error) {
    var reqBody io.Reader
    if body != nil {
        jsonBody, err := json.Marshal(body)
        if err != nil { return nil, err }
        reqBody = bytes.NewBuffer(jsonBody)
    }
    req, err := http.NewRequestWithContext(ctx, method, digitalOceanAPIURL+path, reqBody)
    req.Header.Set("Authorization", "Bearer "+token)
    req.Header.Set("Content-Type", "application/json")
    resp, err := p.client.Do(req)
    // ... response handling
}
```

**Solution:** Create `internal/modules/server/providers/api_client.go`
```go
package providers

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
)

// BaseAPIClient provides common HTTP operations for cloud providers
type BaseAPIClient struct {
    client   *http.Client
    baseURL  string
    getToken func() string
}

// NewBaseAPIClient creates a new API client
func NewBaseAPIClient(baseURL string, tokenFn func() string) *BaseAPIClient {
    return &BaseAPIClient{
        client:   &http.Client{Timeout: 30 * time.Second},
        baseURL:  baseURL,
        getToken: tokenFn,
    }
}

// DoRequest performs an authenticated HTTP request
func (c *BaseAPIClient) DoRequest(ctx context.Context, method, path string, body interface{}) (map[string]interface{}, error) {
    var reqBody io.Reader
    if body != nil {
        jsonBody, err := json.Marshal(body)
        if err != nil {
            return nil, fmt.Errorf("failed to marshal request: %w", err)
        }
        reqBody = bytes.NewBuffer(jsonBody)
    }

    req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }

    req.Header.Set("Authorization", "Bearer "+c.getToken())
    req.Header.Set("Content-Type", "application/json")

    resp, err := c.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("request failed: %w", err)
    }
    defer resp.Body.Close()

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
    return result, nil
}

// Get performs a GET request
func (c *BaseAPIClient) Get(ctx context.Context, path string) (map[string]interface{}, error) {
    return c.DoRequest(ctx, http.MethodGet, path, nil)
}

// Post performs a POST request
func (c *BaseAPIClient) Post(ctx context.Context, path string, body interface{}) (map[string]interface{}, error) {
    return c.DoRequest(ctx, http.MethodPost, path, body)
}

// Delete performs a DELETE request
func (c *BaseAPIClient) Delete(ctx context.Context, path string) error {
    _, err := c.DoRequest(ctx, http.MethodDelete, path, nil)
    return err
}
```

**Refactored Provider:**
```go
type DigitalOceanProvider struct {
    BaseProvider
    apiClient *BaseAPIClient
}

func NewDigitalOceanProvider(keyGenerator sshkey.Generator) *DigitalOceanProvider {
    return &DigitalOceanProvider{
        BaseProvider: BaseProvider{keyGenerator: keyGenerator},
        apiClient: NewBaseAPIClient(
            "https://api.digitalocean.com/v2",
            func() string { return credentials["token"].(string) },
        ),
    }
}
```

**Refactoring Steps:**
- [ ] Create `internal/modules/server/providers/api_client.go`
- [ ] Refactor DigitalOcean provider to embed
- [ ] Refactor Hetzner provider to embed
- [ ] Refactor Linode provider to embed
- [ ] Refactor Vultr provider to embed
- [ ] Add retry logic to base client

**Impact:** ~200 lines eliminated, consistent error handling, single point for retry/timeout logic

---

## 20. Git Provider BaseGitAPIClient (P1)

**Issue:** Git providers repeat HTTP client creation, pagination, and auth handling.

**Files Affected:**
- `internal/modules/git/providers/github.go:32-37, 89-119, 216-268`
- `internal/modules/git/providers/gitlab.go:28-34, 164-182, 359-405`
- `internal/modules/git/providers/bitbucket.go:28-34, 159-176, 234-277`

**Current Pattern:**
```go
// Repeated in all 3 providers
httpClient := &http.Client{Timeout: 30 * time.Second}

// Pagination logic repeated
for url != "" {
    body, statusCode, err := c.Get(ctx, url)
    if statusCode != http.StatusOK {
        return nil, fmt.Errorf("pagination request failed")
    }
    // ... process page
}
```

**Solution:** Create `internal/modules/git/providers/api_client.go`
```go
package providers

// BaseGitAPIClient provides common operations for git providers
type BaseGitAPIClient struct {
    httpClient   *http.Client
    baseURL      string
    getAuthToken func(ctx context.Context) (string, error)
}

// NewBaseGitAPIClient creates a new git API client
func NewBaseGitAPIClient(baseURL string, tokenFn func(ctx context.Context) (string, error)) *BaseGitAPIClient {
    return &BaseGitAPIClient{
        httpClient:   &http.Client{Timeout: 30 * time.Second},
        baseURL:      baseURL,
        getAuthToken: tokenFn,
    }
}

// Get performs authenticated GET request
func (c *BaseGitAPIClient) Get(ctx context.Context, path string) ([]byte, int, error) {
    token, err := c.getAuthToken(ctx)
    if err != nil {
        return nil, 0, err
    }

    req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
    if err != nil {
        return nil, 0, err
    }

    req.Header.Set("Authorization", "Bearer "+token)
    req.Header.Set("Accept", "application/json")

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, 0, err
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)
    return body, resp.StatusCode, nil
}

// PaginatedFetch handles pagination with cursor/page parameters
func (c *BaseGitAPIClient) PaginatedFetch(
    ctx context.Context,
    firstPageURL string,
    processor func([]byte) (nextURL string, items []interface{}, err error),
) ([]interface{}, error) {
    var allItems []interface{}
    url := firstPageURL

    for url != "" {
        body, statusCode, err := c.Get(ctx, url)
        if err != nil {
            return nil, err
        }
        if statusCode != http.StatusOK {
            return nil, fmt.Errorf("pagination request failed: status %d", statusCode)
        }

        nextURL, items, err := processor(body)
        if err != nil {
            return nil, err
        }

        allItems = append(allItems, items...)
        url = nextURL
    }

    return allItems, nil
}
```

**Impact:** ~150 lines eliminated, consistent pagination handling

---

## 21. Notification BaseWebhookChannel (P1)

**Issue:** Webhook-based notification channels repeat identical send/connect logic.

**Files Affected:**
- `internal/modules/notification/channels/slack.go:30-80`
- `internal/modules/notification/channels/discord.go:30-80`
- `internal/modules/notification/channels/telegram.go:45-90`

**Current Pattern (repeated 3 times):**
```go
func (s *SlackChannel) Send(ctx context.Context, notif Notification) error {
    webhookURL := s.channel.GetWebhookURL()
    if webhookURL == "" {
        return ErrInvalidConfiguration
    }
    if s.httpClient == nil {
        return ErrSendFailed
    }
    payload := slackMessage{Text: notif.ToSlack()}
    _, statusCode, err := s.httpClient.Post(ctx, webhookURL, payload)
    if err != nil {
        return fmt.Errorf("%w: %v", ErrSendFailed, err)
    }
    if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
        return fmt.Errorf("%w: received status code %d", ErrSendFailed, statusCode)
    }
    return nil
}
```

**Solution:** Create `internal/modules/notification/channels/webhook_channel.go`
```go
package channels

import (
    "context"
    "fmt"
    "net/http"
)

// BaseWebhookChannel provides common webhook operations
type BaseWebhookChannel struct {
    channel    *NotificationChannel
    httpClient HTTPClient
    getURL     func() string
    payloadFn  func(Notification) interface{}
}

// NewBaseWebhookChannel creates a base webhook channel
func NewBaseWebhookChannel(
    channel *NotificationChannel,
    httpClient HTTPClient,
    urlFn func() string,
    payloadFn func(Notification) interface{},
) BaseWebhookChannel {
    return BaseWebhookChannel{
        channel:    channel,
        httpClient: httpClient,
        getURL:     urlFn,
        payloadFn:  payloadFn,
    }
}

// Send sends a notification via webhook
func (b *BaseWebhookChannel) Send(ctx context.Context, notif Notification) error {
    url := b.getURL()
    if url == "" {
        return ErrInvalidConfiguration
    }
    if b.httpClient == nil {
        return ErrSendFailed
    }

    payload := b.payloadFn(notif)
    _, statusCode, err := b.httpClient.Post(ctx, url, payload)
    if err != nil {
        return fmt.Errorf("%w: %v", ErrSendFailed, err)
    }

    if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
        return fmt.Errorf("%w: received status code %d", ErrSendFailed, statusCode)
    }
    return nil
}

// Connect tests the webhook connection
func (b *BaseWebhookChannel) Connect(ctx context.Context) error {
    url := b.getURL()
    if url == "" {
        return ErrInvalidConfiguration
    }
    if b.httpClient == nil {
        return ErrConnectionFailed
    }

    testPayload := map[string]string{"text": "Connected to Launch webhook"}
    _, statusCode, err := b.httpClient.Post(ctx, url, testPayload)
    if err != nil {
        return fmt.Errorf("%w: %v", ErrConnectionFailed, err)
    }

    if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
        return fmt.Errorf("%w: received status code %d", ErrConnectionFailed, statusCode)
    }
    return nil
}
```

**Refactored Channel:**
```go
type SlackChannel struct {
    BaseWebhookChannel
}

func NewSlackChannel(channel *NotificationChannel, httpClient HTTPClient) *SlackChannel {
    return &SlackChannel{
        BaseWebhookChannel: NewBaseWebhookChannel(
            channel,
            httpClient,
            channel.GetWebhookURL,
            func(notif Notification) interface{} {
                return slackMessage{Text: notif.ToSlack()}
            },
        ),
    }
}
```

**Refactoring Steps:**
- [ ] Create `internal/modules/notification/channels/webhook_channel.go`
- [ ] Refactor SlackChannel to embed
- [ ] Refactor DiscordChannel to embed
- [ ] Refactor TelegramChannel to embed (may need URL builder)

**Impact:** ~100 lines eliminated, consistent webhook handling

---

## 22. Middleware Composition Helpers (P1)

**Issue:** All route files repeat the same middleware chains.

**Files Affected:**
- `internal/modules/server/routes.go:30`
- `internal/modules/site/routes.go:38`
- `internal/modules/database/routes.go:20`
- `internal/modules/git/routes.go:22`
- 7+ more route files

**Current Pattern:**
```go
// Repeated in 11 route files
providers := router.Group("/providers", authMiddleware, middleware.TeamScope(), middleware.VerifySubscription())
servers := router.Group("/servers", authMiddleware, middleware.TeamScope(), middleware.VerifySubscription())
sites := router.Group("/sites", authMiddleware, middleware.TeamScope(), middleware.VerifySubscription())
```

**Solution:** Create `internal/middleware/composition.go`
```go
package middleware

import "github.com/gofiber/fiber/v2"

// AuthRequired returns basic authentication middleware chain
func AuthRequired(authMiddleware fiber.Handler) []fiber.Handler {
    return []fiber.Handler{authMiddleware}
}

// TeamScopedAPI returns middleware chain for team-scoped API endpoints
func TeamScopedAPI(authMiddleware fiber.Handler) []fiber.Handler {
    return []fiber.Handler{authMiddleware, TeamScope(), VerifySubscription()}
}

// AdminAPI returns middleware chain for admin endpoints
func AdminAPI(authMiddleware fiber.Handler) []fiber.Handler {
    return []fiber.Handler{authMiddleware, TeamScope(), TeamRole("admin")}
}

// VerifiedUserAPI returns middleware for verified users only
func VerifiedUserAPI(authMiddleware fiber.Handler) []fiber.Handler {
    return []fiber.Handler{authMiddleware, VerifyEmail(), TeamScope()}
}
```

**Refactored Routes:**
```go
// Before
servers := router.Group("/servers", authMiddleware, middleware.TeamScope(), middleware.VerifySubscription())

// After
servers := router.Group("/servers", middleware.TeamScopedAPI(authMiddleware)...)
```

**Refactoring Steps:**
- [ ] Create `internal/middleware/composition.go`
- [ ] Add `TeamScopedAPI`, `AdminAPI`, `VerifiedUserAPI` helpers
- [ ] Refactor all route files to use composition helpers
- [ ] Update documentation

**Impact:** ~30 repeated middleware chains consolidated, easier to modify globally

---

## 23. Config Builder Pattern (P2)

**Issue:** All 9 config files repeat the same load/setDefaults pattern.

**Files Affected:**
- `internal/config/app.go`
- `internal/config/jwt.go`
- `internal/config/redis.go`
- `internal/config/database.go`
- `internal/config/queue.go`
- `internal/config/billing.go`
- `internal/config/git.go`
- `internal/config/slack.go`
- `internal/config/sentry.go`

**Current Pattern:**
```go
func loadAppConfig() AppConfig {
    return AppConfig{
        Name:        viper.GetString("APP_NAME"),
        Environment: viper.GetString("APP_ENV"),
        // ... 10 more fields
    }
}

func setAppDefaults() {
    viper.SetDefault("APP_NAME", "Launch")
    viper.SetDefault("APP_ENV", "development")
    // ... 10 more defaults
}
```

**Solution:** Create `internal/config/builder.go`
```go
package config

import "github.com/spf13/viper"

// ConfigBuilder provides fluent config loading with defaults
type ConfigBuilder struct{}

// NewBuilder creates a new config builder
func NewBuilder() *ConfigBuilder {
    return &ConfigBuilder{}
}

// String gets a string config with default
func (b *ConfigBuilder) String(envVar, defaultVal string) string {
    viper.SetDefault(envVar, defaultVal)
    return viper.GetString(envVar)
}

// Int gets an int config with default
func (b *ConfigBuilder) Int(envVar string, defaultVal int) int {
    viper.SetDefault(envVar, defaultVal)
    return viper.GetInt(envVar)
}

// Bool gets a bool config with default
func (b *ConfigBuilder) Bool(envVar string, defaultVal bool) bool {
    viper.SetDefault(envVar, defaultVal)
    return viper.GetBool(envVar)
}

// Duration gets a duration config with default
func (b *ConfigBuilder) Duration(envVar string, defaultVal time.Duration) time.Duration {
    viper.SetDefault(envVar, defaultVal)
    return viper.GetDuration(envVar)
}

// StringSlice gets a string slice config with default
func (b *ConfigBuilder) StringSlice(envVar string, defaultVal []string) []string {
    viper.SetDefault(envVar, defaultVal)
    return viper.GetStringSlice(envVar)
}
```

**Refactored Config:**
```go
// Before
func loadAppConfig() AppConfig {
    return AppConfig{
        Name:        viper.GetString("APP_NAME"),
        Environment: viper.GetString("APP_ENV"),
    }
}

func setAppDefaults() {
    viper.SetDefault("APP_NAME", "Launch")
    viper.SetDefault("APP_ENV", "development")
}

// After
func loadAppConfig() AppConfig {
    b := NewBuilder()
    return AppConfig{
        Name:        b.String("APP_NAME", "Launch"),
        Environment: b.String("APP_ENV", "development"),
    }
}
// No separate setDefaults function needed!
```

**Impact:** ~86 lines eliminated across 9 config files

---

## 24. Request DTO Mixins (P2)

**Issue:** Request DTOs repeat common field patterns.

**Files Affected:**
- Email validation: 9+ occurrences across 5 modules
- Password+Confirmation: 8+ occurrences
- ULID ID fields: 15+ occurrences
- Enum/oneof validation: 35+ occurrences

**Solution:** Create `internal/pkg/dto/request_mixins.go`
```go
package dto

// EmailField provides validated email field
type EmailField struct {
    Email string `json:"email" validate:"required,email"`
}

// OptionalEmailField provides optional email validation
type OptionalEmailField struct {
    Email string `json:"email" validate:"omitempty,email"`
}

// PasswordFields provides password with confirmation
type PasswordFields struct {
    Password             string `json:"password" validate:"required,min=8"`
    PasswordConfirmation string `json:"password_confirmation" validate:"required,eqfield=Password"`
}

// NameField provides common name validation
type NameField struct {
    Name string `json:"name" validate:"required,min=2,max=255"`
}

// DescriptionField provides optional description
type DescriptionField struct {
    Description *string `json:"description" validate:"omitempty,max=1000"`
}

// NamedFields combines name and description
type NamedFields struct {
    NameField
    DescriptionField
}

// PaginationFields for list requests
type PaginationFields struct {
    Page    int `json:"page" validate:"omitempty,min=1"`
    PerPage int `json:"per_page" validate:"omitempty,min=1,max=100"`
}

// SortFields for sortable list requests
type SortFields struct {
    SortBy    string `json:"sort_by" validate:"omitempty"`
    SortOrder string `json:"sort_order" validate:"omitempty,oneof=asc desc"`
}

// Normalize normalizes email to lowercase
func (e *EmailField) Normalize() {
    e.Email = strings.ToLower(strings.TrimSpace(e.Email))
}
```

**Refactored Request:**
```go
// Before
type RegisterRequest struct {
    Name                 string `json:"name" validate:"required,min=2,max=255"`
    Email                string `json:"email" validate:"required,email"`
    Password             string `json:"password" validate:"required,min=8"`
    PasswordConfirmation string `json:"password_confirmation" validate:"required,eqfield=Password"`
}

// After
type RegisterRequest struct {
    dto.NameField
    dto.EmailField
    dto.PasswordFields
}
```

**Refactoring Steps:**
- [ ] Create `internal/pkg/dto/request_mixins.go`
- [ ] Add `EmailField`, `PasswordFields`, `NameField`, etc.
- [ ] Refactor auth module requests
- [ ] Refactor other module requests
- [ ] Add `Normalize()` methods to mixins

**Impact:** ~150 lines eliminated, consistent validation rules

---

## 25. Test Base Classes (P2)

**Issue:** Test files repeat setup patterns for channels, providers, and jobs.

**Files Affected:**
- `internal/modules/notification/channels/slack_test.go:10-54`
- `internal/modules/notification/channels/discord_test.go:10-54`
- `internal/modules/dns/providers/provider_test.go:58-85`
- `internal/modules/git/providers/provider_test.go:37-113`
- `tests/jobs/provision_server_test.go:9-60`

**Solution:** Create test base classes in `tests/testutil/`

**Channel Test Base:**
```go
// tests/testutil/channel_test_base.go
package testutil

import (
    "context"
    "testing"
    "github.com/stretchr/testify/assert"
)

// ChannelTestBase provides common channel testing utilities
type ChannelTestBase struct {
    T          *testing.T
    MockClient *MockHTTPClient
    Ctx        context.Context
}

// NewChannelTestBase creates a new channel test base
func NewChannelTestBase(t *testing.T) *ChannelTestBase {
    return &ChannelTestBase{
        T:          t,
        MockClient: NewMockHTTPClient(),
        Ctx:        context.Background(),
    }
}

// AssertSendSuccess tests successful send
func (b *ChannelTestBase) AssertSendSuccess(channel Channel, notif Notification) {
    b.MockClient.SetResponse(200, nil)
    err := channel.Send(b.Ctx, notif)
    assert.NoError(b.T, err)
}

// AssertSendFailure tests failed send
func (b *ChannelTestBase) AssertSendFailure(channel Channel, notif Notification, statusCode int) {
    b.MockClient.SetResponse(statusCode, nil)
    err := channel.Send(b.Ctx, notif)
    assert.Error(b.T, err)
}

// AssertConnectSuccess tests successful connect
func (b *ChannelTestBase) AssertConnectSuccess(channel Channel) {
    b.MockClient.SetResponse(200, nil)
    err := channel.Connect(b.Ctx)
    assert.NoError(b.T, err)
}
```

**Provider Test Base:**
```go
// tests/testutil/provider_test_base.go
package testutil

// ProviderTestBase provides common provider testing utilities
type ProviderTestBase struct {
    T       *testing.T
    Factory interface{}
}

// AssertProviderType checks that factory creates correct provider type
func (b *ProviderTestBase) AssertProviderType(providerName string, expectedType interface{}) {
    provider := b.Factory.Create(providerName)
    assert.IsType(b.T, expectedType, provider)
}
```

**Impact:** ~200 lines of test boilerplate eliminated

---

## 26. Module Builder Factory (P2)

**Issue:** All modules repeat identical initialization pattern in NewModule().

**Files Affected:**
- `internal/modules/server/module.go:30-38`
- `internal/modules/auth/module.go:26-35`
- `internal/modules/site/module.go:49-57`
- `internal/modules/git/module.go:47-62`
- 8+ more modules

**Current Pattern:**
```go
func NewModule(b *module.Builder) *Module {
    deps := b.Deps()
    repos := repositories.NewRegistry(deps.DB)
    service := services.NewService(repos, deps.Queue, deps.WebSocket, deps.Logger)

    return &Module{
        Base:    module.NewBase(ModuleName, b),
        repos:   repos,
        service: service,
    }
}
```

**Solution:** Create module factory helpers in `internal/pkg/module/factory.go`
```go
package module

// ModuleFactory helps create modules with standard pattern
type ModuleFactory struct {
    builder *Builder
    name    string
}

// NewModuleFactory creates a new module factory
func NewModuleFactory(name string, b *Builder) *ModuleFactory {
    return &ModuleFactory{
        builder: b,
        name:    name,
    }
}

// CreateBase creates the module base
func (f *ModuleFactory) CreateBase() Base {
    return NewBase(f.name, f.builder)
}

// Deps returns builder dependencies
func (f *ModuleFactory) Deps() Deps {
    return f.builder.Deps()
}

// DB returns database connection
func (f *ModuleFactory) DB() *gorm.DB {
    return f.builder.DB()
}
```

**Impact:** ~50 lines eliminated, standardized module creation

---

## Extended Implementation Phases

### Phase 1: Critical Infrastructure (Week 1-2)
| Item | Priority | Est. LOC Saved | Complexity |
|------|----------|----------------|------------|
| Cloud Provider BaseAPIClient | P1 | 200 | Medium |
| Git Provider BaseGitAPIClient | P1 | 150 | Medium |
| Notification BaseWebhookChannel | P1 | 100 | Low |
| Middleware Composition | P1 | 30 | Low |
| Model NamedModel Mixin | P1 | 26 | Low |

### Phase 2: Model & DTO Patterns (Week 3-4)
| Item | Priority | Est. LOC Saved | Complexity |
|------|----------|----------------|------------|
| Compound Scoping Mixins | P2 | 100 | Low |
| StateTrackingModel Mixin | P2 | 30 | Low |
| ProgressTrackingModel Mixin | P2 | 10 | Low |
| Request DTO Mixins | P2 | 150 | Medium |
| Config Builder Pattern | P2 | 86 | Low |

### Phase 3: Testing & Module Patterns (Week 5-6)
| Item | Priority | Est. LOC Saved | Complexity |
|------|----------|----------------|------------|
| Test Base Classes | P2 | 200 | Medium |
| Module Builder Factory | P2 | 50 | Low |

---

## Updated Success Metrics

After implementing ALL embeddings:

1. **Code Reduction:** ~4000+ lines eliminated
2. **Consistency:** All modules follow identical patterns
3. **Type Safety:** Compile-time checks for embedded fields
4. **Maintainability:** Single point of change for common functionality
5. **Discoverability:** IDE autocomplete works with embedded methods
6. **Testing:** Base classes can be unit tested once
7. **Provider Reliability:** Centralized retry/timeout/error handling
8. **Validation Consistency:** Single source of truth for field validation

---

## Summary by Category

| Category | Items | Total LOC Saved |
|----------|-------|-----------------|
| Handler Bases | 5 patterns | ~400 lines |
| Repository Patterns | 3 patterns | ~850 lines |
| Service Patterns | 2 patterns | ~300 lines |
| Job/Task Patterns | 3 patterns | ~800 lines |
| Model Mixins | 5 patterns | ~270 lines |
| Provider API Clients | 3 patterns | ~450 lines |
| Middleware Patterns | 1 pattern | ~30 lines |
| Config Patterns | 1 pattern | ~86 lines |
| DTO/Request Patterns | 2 patterns | ~550 lines |
| Testing Patterns | 2 patterns | ~250 lines |
| **Total** | **27 patterns** | **~4000 lines** |

---

## Notes

- Always run tests after each embedding change
- Verify GORM handles embedded fields correctly
- Check JSON serialization of embedded structs
- Update CLAUDE.md with new embedding patterns
- Consider backward compatibility for API responses

---

*This document complements TODO-REFACTORING.md with embedding-specific patterns.*
