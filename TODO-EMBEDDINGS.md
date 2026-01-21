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

## 46. GitLab Webhook Timing Attack Fix (P1 - SECURITY)

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

## 47. Broadcast Event Constants (P2)

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

## 49. Crypto Package Consolidation (P2)

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

## 51. HTTP Client Base with Request Builder (P1 - CRITICAL)

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

## 63. Error Type Consolidation (P1 - CRITICAL)

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

## 75. GORM Query Scope - Order By Latest (P1)

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

## 79. Job Struct Boilerplate (P1 - CRITICAL)

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

## 81. DTO Timestamp Formatter (P1)

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

## 106. Enum Boilerplate Consolidation (P2)

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

## 108. Slice Transformation Generic Helper (P1)

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

## 109. Safe Map Access Helper (P3)

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

## 115. Nil-Safe Dereference Helpers (P1)

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
| **Grand Total** | **122 patterns** | **~20,311 lines** |
