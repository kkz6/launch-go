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

---

## 27. SSH Client Connection Factory (P1)

**Issue:** SSH client creation repeated 7+ times with identical configuration patterns.

**Files Affected:**
- `internal/modules/taskrunner/dispatcher.go` - Multiple SSH client creations
- `internal/modules/taskrunner/stream_monitor.go` - SSH connection handling
- `internal/modules/server/services/server_service.go` - Server connectivity
- `internal/modules/websocket/handlers/terminal.go` - Terminal connections

**Current Pattern (repeated 7+ times):**
```go
client, err := ssh.NewClient(
    ssh.WithHost(server.IP),
    ssh.WithPort(server.SSHPort),
    ssh.WithUser("launch"),
    ssh.WithPrivateKey(privateKey),
    ssh.WithTimeout(30 * time.Second),
)
if err != nil {
    logger.Error().Err(err).Str("server_id", serverID).Msg("Failed to create SSH client")
    return nil, err
}
defer client.Close()
```

**Solution:** Create `internal/pkg/ssh/factory.go`
```go
package ssh

import (
    "context"
    "time"
    "github.com/rs/zerolog"
)

// ConnectionFactory creates SSH connections with consistent configuration
type ConnectionFactory struct {
    defaultUser    string
    defaultTimeout time.Duration
    keyProvider    PrivateKeyProvider
    logger         *zerolog.Logger
}

// PrivateKeyProvider retrieves private keys for servers
type PrivateKeyProvider interface {
    GetPrivateKey(ctx context.Context, serverID string) (string, error)
}

// NewConnectionFactory creates a new SSH connection factory
func NewConnectionFactory(keyProvider PrivateKeyProvider, logger *zerolog.Logger) *ConnectionFactory {
    return &ConnectionFactory{
        defaultUser:    "launch",
        defaultTimeout: 30 * time.Second,
        keyProvider:    keyProvider,
        logger:         logger,
    }
}

// ServerConnectable interface for models that can be SSH connected
type ServerConnectable interface {
    GetIP() string
    GetSSHPort() int
    GetID() string
}

// Connect creates an SSH connection to a server
func (f *ConnectionFactory) Connect(ctx context.Context, server ServerConnectable) (*Client, error) {
    privateKey, err := f.keyProvider.GetPrivateKey(ctx, server.GetID())
    if err != nil {
        f.logger.Error().Err(err).Str("server_id", server.GetID()).Msg("Failed to get private key")
        return nil, err
    }

    client, err := NewClient(
        WithHost(server.GetIP()),
        WithPort(server.GetSSHPort()),
        WithUser(f.defaultUser),
        WithPrivateKey(privateKey),
        WithTimeout(f.defaultTimeout),
    )
    if err != nil {
        f.logger.Error().Err(err).Str("server_id", server.GetID()).Msg("Failed to create SSH client")
        return nil, err
    }

    return client, nil
}

// ConnectWithOptions creates an SSH connection with custom options
func (f *ConnectionFactory) ConnectWithOptions(ctx context.Context, server ServerConnectable, opts ...ClientOption) (*Client, error) {
    privateKey, err := f.keyProvider.GetPrivateKey(ctx, server.GetID())
    if err != nil {
        return nil, err
    }

    defaultOpts := []ClientOption{
        WithHost(server.GetIP()),
        WithPort(server.GetSSHPort()),
        WithUser(f.defaultUser),
        WithPrivateKey(privateKey),
        WithTimeout(f.defaultTimeout),
    }

    allOpts := append(defaultOpts, opts...)
    return NewClient(allOpts...)
}
```

**Refactored Usage:**
```go
// Before
client, err := ssh.NewClient(
    ssh.WithHost(server.IP),
    ssh.WithPort(server.SSHPort),
    ssh.WithUser("launch"),
    ssh.WithPrivateKey(privateKey),
    ssh.WithTimeout(30 * time.Second),
)

// After
client, err := sshFactory.Connect(ctx, server)
```

**Refactoring Steps:**
- [ ] Create `internal/pkg/ssh/factory.go`
- [ ] Define `ServerConnectable` interface
- [ ] Implement `PrivateKeyProvider` interface
- [ ] Refactor `taskrunner/dispatcher.go` to use factory
- [ ] Refactor `stream_monitor.go` to use factory
- [ ] Refactor terminal handler to use factory
- [ ] Add connection pooling option for future optimization

**Impact:** ~100 lines eliminated, centralized connection management, easier to add connection pooling

---

## 28. Enum Scan/Value Boilerplate (P1)

**Issue:** 31 enum types repeat ~10 lines of Scan/Value methods (315+ lines total).

**Files Affected:**
- `internal/modules/site/enums/enums.go` - 10 enums (860+ lines, ~100 lines Scan/Value)
- `internal/modules/server/enums/enums.go` - 8 enums (~80 lines Scan/Value)
- `internal/modules/git/enums/enums.go` - 5 enums (~50 lines Scan/Value)
- `internal/modules/database/enums/enums.go` - 3 enums (~30 lines Scan/Value)
- `internal/modules/billing/enums/enums.go` - 2 enums (~20 lines Scan/Value)
- `internal/modules/backup/enums/enums.go` - 2 enums (~20 lines Scan/Value)
- `internal/modules/notification/enums/enums.go` - 1 enum (~10 lines Scan/Value)

**Current Pattern (repeated 31 times):**
```go
type SiteType string

const (
    SiteTypeLaravel SiteType = "laravel"
    SiteTypeWordPress SiteType = "wordpress"
    // ...
)

// Repeated for every enum type
func (s *SiteType) Scan(value interface{}) error {
    if value == nil {
        return nil
    }
    str, ok := value.(string)
    if !ok {
        bytes, ok := value.([]byte)
        if !ok {
            return fmt.Errorf("failed to scan SiteType: %v", value)
        }
        str = string(bytes)
    }
    *s = SiteType(str)
    return nil
}

func (s SiteType) Value() (driver.Value, error) {
    return string(s), nil
}
```

**Solution:** Create `internal/pkg/enum/scannable.go` with generics
```go
package enum

import (
    "database/sql/driver"
    "fmt"
)

// Scannable is an interface for enum types that can be scanned from DB
type Scannable interface {
    ~string
}

// ScanString scans a database value into a string-based enum
func ScanString[T Scannable](target *T, value interface{}) error {
    if value == nil {
        return nil
    }
    switch v := value.(type) {
    case string:
        *target = T(v)
    case []byte:
        *target = T(string(v))
    default:
        return fmt.Errorf("failed to scan %T: %v", *target, value)
    }
    return nil
}

// ValueString returns the driver value for a string-based enum
func ValueString[T Scannable](e T) (driver.Value, error) {
    return string(e), nil
}
```

**Alternative: Code Generation with `//go:generate`**
```go
// internal/pkg/enum/generate.go
//go:generate go run ./generate_enum.go

// generate_enum.go - generates Scan/Value methods
package main

func main() {
    enums := []string{"SiteType", "ServerStatus", "DeploymentStatus", ...}
    for _, e := range enums {
        generateScanValue(e)
    }
}
```

**Using embed tag approach (preferred):**
```go
// internal/pkg/enum/string_enum.go
package enum

// StringEnum provides Scan/Value for string-based enums
type StringEnum string

func (e *StringEnum) Scan(value interface{}) error {
    return ScanString(e, value)
}

func (e StringEnum) Value() (driver.Value, error) {
    return ValueString(e)
}
```

**Refactored Enum:**
```go
// Before - 10 lines per enum
type SiteType string

func (s *SiteType) Scan(value interface{}) error { ... } // 10 lines
func (s SiteType) Value() (driver.Value, error) { ... }  // 3 lines

// After - Type alias approach
type SiteType = enum.StringEnum  // Uses embedded Scan/Value

// Or with constants still working:
type SiteType string
func (s *SiteType) Scan(v interface{}) error { return enum.ScanString(s, v) }
func (s SiteType) Value() (driver.Value, error) { return enum.ValueString(s) }
```

**Refactoring Steps:**
- [ ] Create `internal/pkg/enum/scannable.go`
- [ ] Create generic `ScanString` and `ValueString` functions
- [ ] Refactor site enums (10 types)
- [ ] Refactor server enums (8 types)
- [ ] Refactor git enums (5 types)
- [ ] Refactor remaining module enums (8 types)
- [ ] Consider `go:generate` for full automation

**Impact:** ~315 lines eliminated (10 lines × 31 enums), consistent scanning logic

---

## 29. Cache Infrastructure Base (P2)

**Issue:** Cache implementations repeat key generation, TTL management, and eviction patterns.

**Files Affected:**
- `internal/modules/websocket/cache/team_membership_cache.go`
- `internal/pkg/cache/server_cache.go` (implied usage)
- `internal/pkg/cache/site_address_cache.go` (implied usage)
- Multiple services with inline Redis caching

**Current Pattern:**
```go
// Repeated in each cache implementation
type TeamMembershipCache struct {
    redis  *redis.Client
    ttl    time.Duration
    prefix string
}

func (c *TeamMembershipCache) key(teamID string) string {
    return fmt.Sprintf("%s:%s", c.prefix, teamID)
}

func (c *TeamMembershipCache) Get(ctx context.Context, teamID string) (*TeamMembership, error) {
    data, err := c.redis.Get(ctx, c.key(teamID)).Bytes()
    if err == redis.Nil {
        return nil, ErrCacheMiss
    }
    // unmarshal...
}

func (c *TeamMembershipCache) Set(ctx context.Context, teamID string, value *TeamMembership) error {
    data, _ := json.Marshal(value)
    return c.redis.Set(ctx, c.key(teamID), data, c.ttl).Err()
}

func (c *TeamMembershipCache) Delete(ctx context.Context, teamID string) error {
    return c.redis.Del(ctx, c.key(teamID)).Err()
}
```

**Solution:** Create `internal/pkg/cache/base_cache.go`
```go
package cache

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/redis/go-redis/v9"
)

// BaseCache provides common caching operations
type BaseCache[T any] struct {
    redis  *redis.Client
    prefix string
    ttl    time.Duration
}

// NewBaseCache creates a new base cache
func NewBaseCache[T any](redis *redis.Client, prefix string, ttl time.Duration) *BaseCache[T] {
    return &BaseCache[T]{
        redis:  redis,
        prefix: prefix,
        ttl:    ttl,
    }
}

// Key generates a cache key
func (c *BaseCache[T]) Key(parts ...string) string {
    key := c.prefix
    for _, part := range parts {
        key = fmt.Sprintf("%s:%s", key, part)
    }
    return key
}

// Get retrieves a value from cache
func (c *BaseCache[T]) Get(ctx context.Context, key string) (*T, error) {
    data, err := c.redis.Get(ctx, key).Bytes()
    if err == redis.Nil {
        return nil, ErrCacheMiss
    }
    if err != nil {
        return nil, err
    }

    var value T
    if err := json.Unmarshal(data, &value); err != nil {
        return nil, err
    }
    return &value, nil
}

// Set stores a value in cache
func (c *BaseCache[T]) Set(ctx context.Context, key string, value *T) error {
    data, err := json.Marshal(value)
    if err != nil {
        return err
    }
    return c.redis.Set(ctx, key, data, c.ttl).Err()
}

// Delete removes a value from cache
func (c *BaseCache[T]) Delete(ctx context.Context, key string) error {
    return c.redis.Del(ctx, key).Err()
}

// GetOrSet gets from cache or calls loader function
func (c *BaseCache[T]) GetOrSet(ctx context.Context, key string, loader func() (*T, error)) (*T, error) {
    value, err := c.Get(ctx, key)
    if err == nil {
        return value, nil
    }
    if err != ErrCacheMiss {
        return nil, err
    }

    value, err = loader()
    if err != nil {
        return nil, err
    }

    _ = c.Set(ctx, key, value) // Ignore cache set errors
    return value, nil
}

// DeletePattern deletes all keys matching pattern
func (c *BaseCache[T]) DeletePattern(ctx context.Context, pattern string) error {
    keys, err := c.redis.Keys(ctx, c.Key(pattern)).Result()
    if err != nil {
        return err
    }
    if len(keys) > 0 {
        return c.redis.Del(ctx, keys...).Err()
    }
    return nil
}
```

**Refactored Cache:**
```go
type TeamMembershipCache struct {
    *cache.BaseCache[TeamMembership]
}

func NewTeamMembershipCache(redis *redis.Client) *TeamMembershipCache {
    return &TeamMembershipCache{
        BaseCache: cache.NewBaseCache[TeamMembership](redis, "team_membership", 5*time.Minute),
    }
}

func (c *TeamMembershipCache) GetByTeam(ctx context.Context, teamID string) (*TeamMembership, error) {
    return c.Get(ctx, c.Key(teamID))
}
```

**Refactoring Steps:**
- [ ] Create `internal/pkg/cache/base_cache.go`
- [ ] Add generic `Get`, `Set`, `Delete`, `GetOrSet` methods
- [ ] Add `DeletePattern` for bulk invalidation
- [ ] Refactor `TeamMembershipCache` to embed
- [ ] Create `ServerCache` using base
- [ ] Create `SiteAddressCache` using base

**Impact:** ~80 lines eliminated, consistent cache behavior, built-in GetOrSet pattern

---

## 30. Script Builder Interface (P2)

**Issue:** Complex bash scripts are built with string concatenation and inconsistent patterns.

**Files Affected:**
- `internal/modules/site/tasks/templates/*.go` - All deployment templates
- `internal/modules/server/tasks/templates/*.go` - Provisioning templates
- `internal/modules/database/tasks/templates/*.go` - Database scripts
- Multiple inline script builders across modules

**Current Pattern:**
```go
// Repeated string building pattern
func buildDeployScript(opts DeployOptions) string {
    var script strings.Builder
    script.WriteString("#!/bin/bash\n")
    script.WriteString("set -e\n\n")
    script.WriteString(fmt.Sprintf("SITE_DIR=%s\n", opts.SiteDir))
    script.WriteString(fmt.Sprintf("RELEASE_DIR=%s\n", opts.ReleaseDir))

    if opts.CloneRepository {
        script.WriteString(fmt.Sprintf("git clone %s $RELEASE_DIR\n", opts.RepoURL))
    }

    if opts.RunMigrations {
        script.WriteString("cd $RELEASE_DIR && php artisan migrate --force\n")
    }
    // ... 100+ lines of conditionals
    return script.String()
}
```

**Solution:** Create `internal/pkg/script/builder.go`
```go
package script

import (
    "fmt"
    "strings"
)

// Builder provides fluent interface for building bash scripts
type Builder struct {
    lines   []string
    vars    map[string]string
    shell   string
    options ScriptOptions
}

// ScriptOptions configures script behavior
type ScriptOptions struct {
    ExitOnError  bool
    PipeFail     bool
    NoUnset      bool
    Debug        bool
    ShellDefault string
}

// DefaultOptions returns standard script options
func DefaultOptions() ScriptOptions {
    return ScriptOptions{
        ExitOnError:  true,
        PipeFail:     true,
        NoUnset:      false,
        ShellDefault: "/bin/bash",
    }
}

// NewBuilder creates a new script builder
func NewBuilder(opts ...ScriptOptions) *Builder {
    options := DefaultOptions()
    if len(opts) > 0 {
        options = opts[0]
    }
    return &Builder{
        lines:   []string{},
        vars:    make(map[string]string),
        shell:   options.ShellDefault,
        options: options,
    }
}

// Shebang adds the shebang line
func (b *Builder) Shebang() *Builder {
    b.lines = append(b.lines, fmt.Sprintf("#!%s", b.shell))
    return b
}

// SetOptions adds set options (set -e, set -o pipefail, etc.)
func (b *Builder) SetOptions() *Builder {
    var opts []string
    if b.options.ExitOnError {
        opts = append(opts, "e")
    }
    if b.options.NoUnset {
        opts = append(opts, "u")
    }
    if b.options.Debug {
        opts = append(opts, "x")
    }

    if len(opts) > 0 {
        b.lines = append(b.lines, fmt.Sprintf("set -%s", strings.Join(opts, "")))
    }
    if b.options.PipeFail {
        b.lines = append(b.lines, "set -o pipefail")
    }
    return b
}

// Var adds a variable definition
func (b *Builder) Var(name, value string) *Builder {
    b.vars[name] = value
    b.lines = append(b.lines, fmt.Sprintf("%s=%q", name, value))
    return b
}

// VarRaw adds a variable without quoting
func (b *Builder) VarRaw(name, value string) *Builder {
    b.vars[name] = value
    b.lines = append(b.lines, fmt.Sprintf("%s=%s", name, value))
    return b
}

// Line adds a command line
func (b *Builder) Line(line string) *Builder {
    b.lines = append(b.lines, line)
    return b
}

// Linef adds a formatted command line
func (b *Builder) Linef(format string, args ...interface{}) *Builder {
    b.lines = append(b.lines, fmt.Sprintf(format, args...))
    return b
}

// Comment adds a comment
func (b *Builder) Comment(text string) *Builder {
    b.lines = append(b.lines, fmt.Sprintf("# %s", text))
    return b
}

// Blank adds a blank line
func (b *Builder) Blank() *Builder {
    b.lines = append(b.lines, "")
    return b
}

// If adds conditional block
func (b *Builder) If(condition string) *Builder {
    b.lines = append(b.lines, fmt.Sprintf("if %s; then", condition))
    return b
}

// ElseIf adds else-if block
func (b *Builder) ElseIf(condition string) *Builder {
    b.lines = append(b.lines, fmt.Sprintf("elif %s; then", condition))
    return b
}

// Else adds else block
func (b *Builder) Else() *Builder {
    b.lines = append(b.lines, "else")
    return b
}

// Fi closes if block
func (b *Builder) Fi() *Builder {
    b.lines = append(b.lines, "fi")
    return b
}

// When conditionally adds lines
func (b *Builder) When(condition bool, fn func(*Builder)) *Builder {
    if condition {
        fn(b)
    }
    return b
}

// CD changes directory
func (b *Builder) CD(dir string) *Builder {
    b.lines = append(b.lines, fmt.Sprintf("cd %s", dir))
    return b
}

// Exec runs a command
func (b *Builder) Exec(cmd string, args ...string) *Builder {
    if len(args) > 0 {
        b.lines = append(b.lines, fmt.Sprintf("%s %s", cmd, strings.Join(args, " ")))
    } else {
        b.lines = append(b.lines, cmd)
    }
    return b
}

// String returns the built script
func (b *Builder) String() string {
    return strings.Join(b.lines, "\n") + "\n"
}
```

**Refactored Script:**
```go
// Before
func buildDeployScript(opts DeployOptions) string {
    var script strings.Builder
    script.WriteString("#!/bin/bash\n")
    script.WriteString("set -e\n\n")
    script.WriteString(fmt.Sprintf("SITE_DIR=%s\n", opts.SiteDir))
    if opts.CloneRepository {
        script.WriteString(fmt.Sprintf("git clone %s $RELEASE_DIR\n", opts.RepoURL))
    }
    return script.String()
}

// After
func buildDeployScript(opts DeployOptions) string {
    return script.NewBuilder().
        Shebang().
        SetOptions().
        Blank().
        Var("SITE_DIR", opts.SiteDir).
        Var("RELEASE_DIR", opts.ReleaseDir).
        Blank().
        When(opts.CloneRepository, func(b *script.Builder) {
            b.Linef("git clone %s $RELEASE_DIR", opts.RepoURL)
        }).
        When(opts.RunMigrations, func(b *script.Builder) {
            b.CD("$RELEASE_DIR").
              Exec("php", "artisan", "migrate", "--force")
        }).
        String()
}
```

**Refactoring Steps:**
- [ ] Create `internal/pkg/script/builder.go`
- [ ] Add fluent methods for common bash constructs
- [ ] Add `When()` for conditional sections
- [ ] Refactor site deployment templates
- [ ] Refactor server provisioning templates
- [ ] Refactor database scripts

**Impact:** ~500 lines of cleaner script building, consistent formatting, testable structure

---

## 31. Shell Defaults Registry (P2)

**Issue:** Shell/binary paths and defaults scattered across modules with inconsistency.

**Files Affected:**
- Multiple task templates with hardcoded paths
- Service files with different shell defaults
- Inconsistent use of `/bin/bash` vs `/usr/bin/bash`

**Current Pattern:**
```go
// Inconsistent across modules
script.WriteString("#!/bin/bash\n")           // Some files
script.WriteString("#!/usr/bin/bash\n")       // Other files
script.WriteString("/usr/bin/php artisan\n")  // Hardcoded paths
script.WriteString("systemctl restart\n")     // Missing full path
```

**Solution:** Create `internal/pkg/shell/defaults.go`
```go
package shell

// Paths contains standard binary paths
var Paths = struct {
    Bash       string
    PHP        string
    Composer   string
    NPM        string
    Node       string
    Git        string
    Systemctl  string
    Service    string
    Caddy      string
    MySQL      string
    PostgreSQL string
}{
    Bash:       "/bin/bash",
    PHP:        "/usr/bin/php",
    Composer:   "/usr/local/bin/composer",
    NPM:        "/usr/bin/npm",
    Node:       "/usr/bin/node",
    Git:        "/usr/bin/git",
    Systemctl:  "/usr/bin/systemctl",
    Service:    "/usr/sbin/service",
    Caddy:      "/usr/bin/caddy",
    MySQL:      "/usr/bin/mysql",
    PostgreSQL: "/usr/bin/psql",
}

// Commands contains common command patterns
var Commands = struct {
    PHPArtisan     func(args ...string) string
    ComposerInstall func() string
    NPMInstall      func() string
    SystemctlRestart func(service string) string
    CaddyReload     func() string
}{
    PHPArtisan: func(args ...string) string {
        return fmt.Sprintf("%s artisan %s", Paths.PHP, strings.Join(args, " "))
    },
    ComposerInstall: func() string {
        return fmt.Sprintf("%s install --no-interaction --prefer-dist --optimize-autoloader", Paths.Composer)
    },
    NPMInstall: func() string {
        return fmt.Sprintf("%s install", Paths.NPM)
    },
    SystemctlRestart: func(service string) string {
        return fmt.Sprintf("%s restart %s", Paths.Systemctl, service)
    },
    CaddyReload: func() string {
        return fmt.Sprintf("%s reload", Paths.Caddy)
    },
}
```

**Refactored Usage:**
```go
// Before
script.WriteString("/usr/bin/php artisan migrate --force\n")

// After
script.Line(shell.Commands.PHPArtisan("migrate", "--force"))
```

**Impact:** Consistent paths, easy to update for different distributions

---

## 32. Encrypted Field Types (P2)

**Issue:** Some sensitive fields missing encryption, and encrypted types not consistently applied.

**Files Affected:**
- `internal/modules/database/models/database_user.go:15` - Password should be encrypted
- `internal/modules/auth/models/user.go` - TwoFactorSecret should be encrypted
- `internal/database/serializers/encrypted.go` - EncryptedString type exists
- `internal/pkg/models/encrypted_types.go` - Has EncryptedString definition

**Current Pattern:**
```go
// database_user.go - Password stored as plain varchar!
type DatabaseUser struct {
    Password string `gorm:"type:varchar(255)" json:"-"` // Security risk!
}

// Existing encrypted type (not widely used)
type EncryptedString struct {
    value     string
    encrypted string
}
```

**Issue Details:**
- `DatabaseUser.Password` - Stored as plain text varchar
- `User.TwoFactorSecret` - May not be using encrypted type
- Inconsistent application of encryption across models

**Solution:** Audit and apply `EncryptedString` type consistently
```go
// Refactored database_user.go
type DatabaseUser struct {
    basemodels.BaseModel
    basemodels.InstallableModel
    basemodels.ServerScopedModel
    DatabaseID string                      `gorm:"column:database_id;type:char(26);not null" json:"database_id"`
    Username   string                      `gorm:"type:varchar(255);not null" json:"username"`
    Password   models.EncryptedString      `gorm:"type:text" json:"-"` // Now encrypted!
    Privileges []DatabaseUserPrivilege     `gorm:"foreignKey:DatabaseUserID" json:"privileges,omitempty"`
}
```

**Encryption Mixin for Models:**
```go
// internal/pkg/models/secure_credentials.go
package models

// SecureCredentials provides encrypted credential fields
type SecureCredentials struct {
    EncryptedPassword EncryptedString `gorm:"column:password;type:text" json:"-"`
}

// SetPassword encrypts and sets the password
func (s *SecureCredentials) SetPassword(password string) error {
    s.EncryptedPassword.Set(password)
    return nil
}

// GetPassword decrypts and returns the password
func (s *SecureCredentials) GetPassword() (string, error) {
    return s.EncryptedPassword.Get()
}

// SecureTwoFactor provides encrypted 2FA fields
type SecureTwoFactor struct {
    TwoFactorSecret EncryptedString `gorm:"column:two_factor_secret;type:text" json:"-"`
}
```

**Refactoring Steps:**
- [ ] Audit all models for sensitive fields
- [ ] Create migration for DatabaseUser.Password to TEXT type
- [ ] Update DatabaseUser to use EncryptedString
- [ ] Verify User.TwoFactorSecret uses encrypted type
- [ ] Create SecureCredentials mixin
- [ ] Document encryption requirements in CLAUDE.md

**Impact:** Security improvement, ~30 lines of mixin code, consistent encryption

---

## 33. ParseAndValidate Adoption (P1)

**Issue:** Helper function `ParseAndValidate` exists but only 2 of 50+ handlers use it.

**Files Affected:**
- `internal/pkg/fiber/request.go:12-28` - Helper exists
- `internal/modules/auth/handlers/auth_handler.go:24-30` - Manual pattern
- `internal/modules/server/handlers/server_handler.go:76-83` - Manual pattern
- 48+ more handlers with manual BodyParser + Validate pattern

**Current Pattern (repeated 50+ times):**
```go
var req dto.SomeRequest
if err := c.BodyParser(&req); err != nil {
    return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
}
if errors := validator.Validate(&req); errors != nil {
    return response.ValidationError(c, errors)
}
```

**Existing Solution (underutilized):**
```go
// internal/pkg/fiber/request.go - Already exists!
func MustParseAndValidate[T any](c *fiber.Ctx) (*T, error) {
    var req T
    if err := c.BodyParser(&req); err != nil {
        return nil, response.BadRequest(c, "Invalid request body")
    }
    if errs := validator.Validate(&req); errs != nil {
        return nil, response.ValidationError(c, errs)
    }
    return &req, nil
}
```

**Refactoring Steps:**
- [ ] Audit all handlers for manual BodyParser + Validate pattern
- [ ] Refactor auth handlers (9 files)
- [ ] Refactor server handlers (15 files)
- [ ] Refactor site handlers (10 files)
- [ ] Refactor remaining module handlers
- [ ] Add request normalization hook to MustParseAndValidate

**Impact:** ~300 lines eliminated, consistent validation

---

## 34. Global Context Extraction Helpers (P1)

**Issue:** Context extraction helpers exist in one module but not shared globally.

**Files Affected:**
- `internal/modules/database/handlers/helper.go:9-22` - Local helpers
- 62 occurrences of `c.Locals("userID").(string)` - Unsafe casting
- 115 occurrences of `c.Locals("teamID").(string)` - Unsafe casting
- 230+ `c.Params()` calls bypass existing param helpers

**Current Pattern (unsafe, repeated 177+ times):**
```go
userID := c.Locals("userID").(string)  // Panics if nil!
teamID := c.Locals("teamID").(string)  // Panics if nil!
serverID := c.Params("id")             // No validation
```

**Existing Local Helper (database module only):**
```go
// internal/modules/database/handlers/helper.go
func getUserIDFromContext(c *fiber.Ctx) *string {
    if userID, ok := c.Locals("userID").(string); ok && userID != "" {
        return &userID
    }
    return nil
}
```

**Solution:** Create `internal/pkg/fiber/context.go`
```go
package fiber

import "github.com/gofiber/fiber/v2"

// GetUserID safely extracts userID from context
func GetUserID(c *fiber.Ctx) string {
    if userID, ok := c.Locals("userID").(string); ok {
        return userID
    }
    return ""
}

// GetUserIDPtr returns pointer (nil if not set)
func GetUserIDPtr(c *fiber.Ctx) *string {
    if userID, ok := c.Locals("userID").(string); ok && userID != "" {
        return &userID
    }
    return nil
}

// GetTeamID safely extracts teamID from context
func GetTeamID(c *fiber.Ctx) string {
    if teamID, ok := c.Locals("teamID").(string); ok {
        return teamID
    }
    return ""
}

// MustGetUserID returns userID or error response
func MustGetUserID(c *fiber.Ctx) (string, error) {
    userID := GetUserID(c)
    if userID == "" {
        return "", response.Unauthorized(c, "User not authenticated")
    }
    return userID, nil
}

// MustGetTeamID returns teamID or error response
func MustGetTeamID(c *fiber.Ctx) (string, error) {
    teamID := GetTeamID(c)
    if teamID == "" {
        return "", response.BadRequest(c, "Team context required")
    }
    return teamID, nil
}
```

**Refactoring Steps:**
- [ ] Create `internal/pkg/fiber/context.go`
- [ ] Replace 62 unsafe userID extractions
- [ ] Replace 115 unsafe teamID extractions
- [ ] Update 230+ c.Params() to use param helpers
- [ ] Remove local helper from database module

**Impact:** ~200 lines safer code, prevents nil panics

---

## 35. Template Loading Consolidation (P1)

**Issue:** Template loading with fs.WalkDir repeated identically in 4 files.

**Files Affected:**
- `internal/modules/site/tasks/templates/templates.go:20-46`
- `internal/modules/server/tasks/templates/templates.go:19-46`
- `internal/modules/database/tasks/templates/templates.go:24-51`
- `internal/pkg/taskrunner/template_engine.go:46-73`

**Current Pattern (repeated 4 times, ~27 lines each):**
```go
var templates *template.Template

func init() {
    templates = template.New("").Funcs(templateFuncs)
    err := fs.WalkDir(templateFS, ".", func(path string, d fs.DirEntry, err error) error {
        if err != nil { return err }
        if d.IsDir() || filepath.Ext(path) != ".sh" { return nil }
        content, err := templateFS.ReadFile(path)
        if err != nil { return fmt.Errorf("reading %s: %w", path, err) }
        _, err = templates.New(path).Parse(string(content))
        if err != nil { return fmt.Errorf("parsing %s: %w", path, err) }
        return nil
    })
    if err != nil { panic(err) }
}

func Render(name string, data interface{}) (string, error) { ... }
func MustRender(name string, data interface{}) string { ... }
```

**Solution:** Create `internal/pkg/templates/loader.go`
```go
package templates

import (
    "embed"
    "fmt"
    "io/fs"
    "path/filepath"
    "text/template"
)

// TemplateLoader handles loading and rendering embedded templates
type TemplateLoader struct {
    templates *template.Template
    extension string
}

// NewLoader creates a template loader from embedded FS
func NewLoader(fsys embed.FS, funcs template.FuncMap, ext string) (*TemplateLoader, error) {
    tmpl := template.New("").Funcs(funcs)

    err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
        if err != nil { return err }
        if d.IsDir() || filepath.Ext(path) != ext { return nil }

        content, err := fs.ReadFile(fsys, path)
        if err != nil { return fmt.Errorf("reading %s: %w", path, err) }

        _, err = tmpl.New(path).Parse(string(content))
        if err != nil { return fmt.Errorf("parsing %s: %w", path, err) }
        return nil
    })
    if err != nil { return nil, err }

    return &TemplateLoader{templates: tmpl, extension: ext}, nil
}

// MustNewLoader panics on error (for init())
func MustNewLoader(fsys embed.FS, funcs template.FuncMap, ext string) *TemplateLoader {
    loader, err := NewLoader(fsys, funcs, ext)
    if err != nil { panic(err) }
    return loader
}

// Render renders a template by name
func (l *TemplateLoader) Render(name string, data interface{}) (string, error) {
    var buf bytes.Buffer
    if err := l.templates.ExecuteTemplate(&buf, name, data); err != nil {
        return "", err
    }
    return buf.String(), nil
}

// MustRender renders or panics
func (l *TemplateLoader) MustRender(name string, data interface{}) string {
    result, err := l.Render(name, data)
    if err != nil { panic(err) }
    return result
}
```

**Refactored Usage:**
```go
// internal/modules/site/tasks/templates/templates.go
var loader = templates.MustNewLoader(templateFS, templateFuncs, ".sh")

func Render(name string, data interface{}) (string, error) {
    return loader.Render(name, data)
}
```

**Refactoring Steps:**
- [ ] Create `internal/pkg/templates/loader.go`
- [ ] Refactor site templates to use loader
- [ ] Refactor server templates to use loader
- [ ] Refactor database templates to use loader
- [ ] Refactor taskrunner template_engine to use loader

**Impact:** ~80 lines eliminated, consistent template loading

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

## 39. Backup Repository Many-to-Many Pattern (P2)

**Issue:** Identical transaction pattern for many-to-many sync repeated 3 times.

**Files Affected:**
- `internal/modules/backup/repositories/backup_repository.go:34-50` - CreateBackupWithDatabases
- `internal/modules/backup/repositories/backup_repository.go:120-142` - UpdateBackupWithDatabases
- `internal/modules/backup/repositories/backup_repository.go:190-208` - SyncBackupDatabases

**Current Pattern (repeated 3 times):**
```go
return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
    // Delete existing associations
    if err := tx.Where("backup_id = ?", backupID).Delete(&BackupDatabase{}).Error; err != nil {
        return err
    }
    // Create new associations
    for _, dbID := range databaseIDs {
        assoc := BackupDatabase{BackupID: backupID, DatabaseID: dbID}
        if err := tx.Create(&assoc).Error; err != nil {
            return err
        }
    }
    return nil
})
```

**Solution:** Create `internal/pkg/repository/associations.go`
```go
package repository

import (
    "context"
    "gorm.io/gorm"
)

// SyncAssociations replaces all associations for a parent entity
func SyncAssociations[T any](
    ctx context.Context,
    db *gorm.DB,
    parentColumn string,
    parentID string,
    childColumn string,
    childIDs []string,
    factory func(parentID, childID string) T,
) error {
    return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        // Delete existing
        var model T
        if err := tx.Where(parentColumn+" = ?", parentID).Delete(&model).Error; err != nil {
            return err
        }

        // Create new
        for _, childID := range childIDs {
            assoc := factory(parentID, childID)
            if err := tx.Create(&assoc).Error; err != nil {
                return err
            }
        }
        return nil
    })
}
```

**Refactored Usage:**
```go
func (r *BackupRepository) SyncBackupDatabases(ctx context.Context, backupID string, dbIDs []string) error {
    return repository.SyncAssociations(
        ctx, r.db,
        "backup_id", backupID,
        "database_id", dbIDs,
        func(bid, did string) BackupDatabase {
            return BackupDatabase{BackupID: bid, DatabaseID: did}
        },
    )
}
```

**Impact:** ~50 lines eliminated, reusable many-to-many pattern

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

## Extended Summary (Items 33-43)

| Category | Items | Est. LOC Saved |
|----------|-------|----------------|
| ParseAndValidate Adoption | Item 33 | ~300 lines |
| Context Extraction Helpers | Item 34 | ~200 lines |
| Template Loading | Item 35 | ~80 lines |
| Systemd Config Scripts | Item 36 | ~60 lines |
| Home Directory Helper | Item 37 | ~20 lines |
| Transaction + Activity | Item 38 | ~150 lines |
| Many-to-Many Pattern | Item 39 | ~50 lines |
| Webhook Response | Item 40 | ~30 lines |
| AppError Fix | Item 41 | Bug fix |
| Timestamp Formatting | Item 42 | Consistency |
| Validation Messages | Item 43 | UX improvement |
| **Additional Total** | **11 patterns** | **~890 lines** |

---

## Final Summary

| Category | Items | Total LOC Saved |
|----------|-------|-----------------|
| Handler Bases | 5 patterns | ~400 lines |
| Repository Patterns | 4 patterns | ~900 lines |
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
| **Grand Total** | **43 patterns** | **~6031 lines** |
