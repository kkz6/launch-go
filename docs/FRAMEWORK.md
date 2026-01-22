# Launch-Go Framework Guide

A comprehensive guide for developers working on the Launch-Go codebase. This document covers architecture patterns, conventions, and how to implement new features consistently.

**Last Updated:** 2026-01-22

---

## Table of Contents

1. [Project Structure](#1-project-structure)
2. [Module Architecture](#2-module-architecture)
3. [Routes & Middleware](#3-routes--middleware)
4. [Handlers](#4-handlers)
5. [Services](#5-services)
6. [Repositories](#6-repositories)
7. [DTOs (Data Transfer Objects)](#7-dtos-data-transfer-objects)
8. [Models](#8-models)
9. [Jobs & Background Tasks](#9-jobs--background-tasks)
10. [Task Runner (SSH Tasks)](#10-task-runner-ssh-tasks)
11. [Error Handling](#11-error-handling)
12. [Validation](#12-validation)
13. [WebSocket](#13-websocket)
14. [Testing](#14-testing)

---

## 1. Project Structure

```
launch-go/
├── cmd/
│   ├── api/              # Main API server entry point
│   └── worker/           # Queue worker entry point
├── internal/
│   ├── app/              # Application bootstrap and wiring
│   ├── config/           # Configuration loading
│   ├── middleware/       # HTTP middleware (auth, team, rate limiting)
│   ├── modules/          # Feature modules (auth, server, site, etc.)
│   │   └── {module}/
│   │       ├── handlers/     # HTTP handlers
│   │       ├── services/     # Business logic
│   │       ├── repositories/ # Data access
│   │       ├── dto/          # Request/Response DTOs
│   │       ├── models/       # Database models
│   │       ├── jobs/         # Background jobs
│   │       ├── tasks/        # SSH task definitions
│   │       ├── contracts/    # Interfaces for cross-module deps
│   │       ├── routes.go     # Route registration
│   │       └── module.go     # Module initialization
│   └── pkg/              # Shared packages
│       ├── fiber/        # Fiber helpers (context, params)
│       ├── response/     # HTTP response helpers
│       ├── validator/    # Request validation
│       ├── repository/   # Generic repository patterns
│       ├── errors/       # Error types
│       └── taskrunner/   # Task execution framework
├── docs/                 # Documentation
└── migrations/           # Database migrations
```

---

## 2. Module Architecture

Each module follows a layered architecture:

```
Handler → Service → Repository → Database
    ↓         ↓          ↓
   DTO      Model      Model
```

### Creating a New Module

1. Create the module directory structure:
```bash
mkdir -p internal/modules/{module}/{handlers,services,repositories,dto,models,jobs}
```

2. Create `module.go` for initialization:
```go
package mymodule

import (
    "github.com/kkz6/launch-go/internal/modules/mymodule/repositories"
    "github.com/kkz6/launch-go/internal/modules/mymodule/services"
    "gorm.io/gorm"
)

type Module struct {
    db      *gorm.DB
    repos   *repositories.Registry
    service *services.Service
}

func NewModule(db *gorm.DB) *Module {
    repos := repositories.NewRegistry(db)
    service := services.NewService(repos)
    return &Module{
        db:      db,
        repos:   repos,
        service: service,
    }
}

func (m *Module) Service() *services.Service {
    return m.service
}
```

3. Create `routes.go` for route registration:
```go
package mymodule

import (
    "github.com/gofiber/fiber/v2"
    "github.com/kkz6/launch-go/internal/middleware"
    "github.com/kkz6/launch-go/internal/modules/mymodule/handlers"
)

func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
    handler := handlers.NewHandler(m.service)

    group := router.Group("/my-resources",
        authMiddleware,
        middleware.TeamScope(),
        middleware.VerifySubscription(),
    )
    {
        group.Get("/", handler.List)
        group.Post("/", handler.Create)
        group.Get("/:id", handler.Show)
        group.Put("/:id", handler.Update)
        group.Delete("/:id", handler.Delete)
    }
}
```

---

## 3. Routes & Middleware

### Route Registration

Routes are registered in each module's `routes.go` file and wired up in `internal/app/routes.go`.

**Standard route groups with middleware:**
```go
// Authenticated routes with team context
group := router.Group("/resources",
    authMiddleware,           // JWT authentication
    middleware.TeamScope(),   // Extract team from X-Team-ID header
    middleware.VerifySubscription(), // Check active subscription
)

// Routes requiring specific roles
adminGroup := router.Group("/admin",
    authMiddleware,
    middleware.TeamScope(),
    middleware.RequireRole(middleware.RoleAdmin),
)
```

### Available Middleware

| Middleware | Purpose | Usage |
|------------|---------|-------|
| `Auth()` | JWT authentication (required) | All authenticated routes |
| `OptionalAuth()` | JWT authentication (optional) | Public routes that may have auth |
| `TeamScope()` | Extract team ID from header | Routes requiring team context |
| `RequireRole(role)` | Check team member role | Admin-only routes |
| `VerifySubscription()` | Check active subscription | Premium features |
| `RateLimit(config)` | Rate limiting | Public endpoints |
| `ValidateULIDParams(params...)` | Validate path params (optional) | Routes with ULID params |

### Nested Routes Pattern

For resources nested under a parent (e.g., sites under servers):
```go
// Parent group with serverId validation
servers := router.Group("/servers/:serverId",
    authMiddleware,
    middleware.TeamScope(),
)

// Child resources
sites := servers.Group("/sites")
sites.Get("/", handler.List)
sites.Get("/:id", handler.Show)
```

---

## 4. Handlers

Handlers are HTTP request handlers that parse input, call services, and return responses.

### Handler Structure

Handlers can optionally embed `handler.Base` to get access to logging methods:

```go
package handlers

import (
    "github.com/gofiber/fiber/v2"
    "github.com/rs/zerolog"

    fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
    "github.com/kkz6/launch-go/internal/modules/mymodule/dto"
    "github.com/kkz6/launch-go/internal/modules/mymodule/services"
    "github.com/kkz6/launch-go/internal/pkg/handler"
    "github.com/kkz6/launch-go/internal/pkg/response"
)

type Handler struct {
    handler.Base  // Embed for logging support (optional)
    service *services.Service
}

func NewHandler(service *services.Service, logger *zerolog.Logger) *Handler {
    return &Handler{
        Base:    handler.NewBase(logger),
        service: service,
    }
}
```

### Handler Base Methods

When embedding `handler.Base`, you get access to these logging methods:

| Method | Description |
|--------|-------------|
| `LogRequest(c, action)` | Log incoming request with action |
| `LogError(c, err, msg)` | Log error with request context |
| `LogWarn(c, msg)` | Log warning with request context |
| `LogInfo(c, action, msg)` | Log info with action context |
| `LogDebug(msg, fields...)` | Log debug with custom fields |
| `GetTraceID(c)` | Get trace ID from request |

### Handler Patterns

**List Handler (GET /):**
```go
func (h *Handler) List(c *fiber.Ctx) error {
    teamID := c.Locals("teamID").(string)

    items, err := h.service.List(c.Context(), teamID)
    if err != nil {
        return response.InternalError(c, "Failed to fetch items")
    }

    return response.OK(c, "Items retrieved", dto.ToResponses(items))
}
```

**Show Handler (GET /:id):**
```go
func (h *Handler) Show(c *fiber.Ctx) error {
    teamID := c.Locals("teamID").(string)

    // Validate and extract path parameter
    id, err := fiberctx.GetID(c)
    if err != nil {
        return err
    }

    item, err := h.service.FindByID(c.Context(), id, teamID)
    if err != nil {
        return response.HandleError(c, err)
    }

    return response.OK(c, "Item retrieved", dto.ToResponse(item))
}
```

**Create Handler (POST /):**
```go
func (h *Handler) Create(c *fiber.Ctx) error {
    teamID := c.Locals("teamID").(string)
    userID := c.Locals("userID").(string)

    // Parse and validate request body (use the generic helper)
    req, err := fiberctx.MustParseAndValidate[dto.CreateRequest](c)
    if err != nil {
        return err
    }

    item, err := h.service.Create(c.Context(), teamID, userID, &req)
    if err != nil {
        return response.HandleError(c, err)
    }

    return response.Created(c, "Item created", dto.ToResponse(item))
}
```

**Update Handler (PUT /:id):**
```go
func (h *Handler) Update(c *fiber.Ctx) error {
    teamID := c.Locals("teamID").(string)

    id, err := fiberctx.GetID(c)
    if err != nil {
        return err
    }

    // Parse and validate request body
    req, err := fiberctx.MustParseAndValidate[dto.UpdateRequest](c)
    if err != nil {
        return err
    }

    item, err := h.service.Update(c.Context(), id, teamID, req)
    if err != nil {
        return response.HandleError(c, err)
    }

    return response.OK(c, "Item updated", dto.ToResponse(item))
}
```

**Delete Handler (DELETE /:id):**
```go
func (h *Handler) Delete(c *fiber.Ctx) error {
    teamID := c.Locals("teamID").(string)

    id, err := fiberctx.GetID(c)
    if err != nil {
        return err
    }

    if err := h.service.Delete(c.Context(), id, teamID); err != nil {
        return response.HandleError(c, err)
    }

    return response.OK(c, "Item deleted", nil)
}
```

### Path Parameter Validation

Use the helpers in `internal/pkg/fiber/params.go` to safely extract and validate path parameters:

```go
import fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"

// For :id parameter (most common)
id, err := fiberctx.GetID(c)
if err != nil {
    return err // Returns 400 Bad Request automatically
}

// For :serverId parameter (nested routes)
serverID, err := fiberctx.GetServerID(c)
if err != nil {
    return err
}

// For custom ULID parameters
deploymentID, err := fiberctx.GetULIDParam(c, "deploymentId")
if err != nil {
    return err
}

// For non-ULID parameters (slugs, filenames)
filename, err := fiberctx.GetRequiredParam(c, "filename")
if err != nil {
    return err
}
```

**Available Functions:**

| Function | Parameter | Validation |
|----------|-----------|------------|
| `GetID(c)` | `:id` | ULID format |
| `GetServerID(c)` | `:serverId` | ULID format |
| `GetULIDParam(c, name)` | Custom | ULID format |
| `GetRequiredParam(c, name)` | Custom | Non-empty |
| `MustGetID(c)` | `:id` | ULID (panics on error) |
| `MustGetServerID(c)` | `:serverId` | ULID (panics on error) |

### Request Body Parsing

Use the helpers in `internal/pkg/fiber/request.go` to parse and validate request bodies:

```go
import fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"

// Parse and validate in one call (recommended)
req, err := fiberctx.MustParseAndValidate[dto.CreateRequest](c)
if err != nil {
    return err // Returns appropriate error response
}

// Alternative: Parse into existing variable
var req dto.CreateRequest
if err := fiberctx.ParseAndValidate(c, &req); err != nil {
    return err
}

// Parse query parameters
params, err := fiberctx.ParseQuery[dto.ListParams](c)
if err != nil {
    return err
}

// Parse query parameters with validation
params, err := fiberctx.ParseQueryWithValidation[dto.FilterParams](c)
if err != nil {
    return err
}
```

**Available Functions:**

| Function | Description |
|----------|-------------|
| `MustParseAndValidate[T](c)` | Parse body and validate, returns `(*T, error)` |
| `ParseAndValidate(c, &req)` | Parse body into existing struct and validate |
| `ParseQuery[T](c)` | Parse query params, returns `(*T, error)` |
| `ParseQueryWithValidation[T](c)` | Parse and validate query params |

---

## 5. Services

Services contain business logic and orchestrate operations between repositories.

### Service Dependencies

Services use a standardized `service.Dependencies` struct for common dependencies:

```go
package services

import (
    "github.com/kkz6/launch-go/internal/modules/mymodule/repositories"
    "github.com/kkz6/launch-go/internal/pkg/service"
)

// ServiceDeps holds all dependencies needed for module services.
// Embedding service.Dependencies provides common dependencies.
type ServiceDeps struct {
    service.Dependencies   // DB, Logger, Queue, Broadcaster, Dispatcher
    Repos *repositories.Registry
    // ... module-specific fields
}

// BaseService embeds service.Base for common functionality
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

The `service.Dependencies` struct provides:
- `DB *gorm.DB` - Database connection
- `Logger *zerolog.Logger` - Structured logger
- `Queue *queue.Client` - Job queue client
- `Broadcaster broadcast.ModelBroadcaster` - WebSocket broadcaster
- `Dispatcher *taskrunner.Dispatcher` - SSH task dispatcher

In module initialization:

```go
func (m *Module) createServices() *services.ServiceRegistry {
    deps := m.Deps()

    svcDeps := &services.ServiceDeps{
        Dependencies: deps.ServiceDeps(),  // Converts module.Deps to service.Dependencies
        Repos:        m.repos,
    }

    return services.NewServiceRegistry(svcDeps)
}
```

### Service Structure

```go
package services

import (
    "context"

    "github.com/kkz6/launch-go/internal/modules/mymodule/dto"
    "github.com/kkz6/launch-go/internal/modules/mymodule/models"
    "github.com/kkz6/launch-go/internal/modules/mymodule/repositories"
)

type Service struct {
    repos *repositories.Registry
}

func NewService(repos *repositories.Registry) *Service {
    return &Service{repos: repos}
}

func (s *Service) Repos() *repositories.Registry {
    return s.repos
}
```

### Service Method Conventions

```go
// Queries - return pointer for nil safety
func (s *Service) FindByID(ctx context.Context, id, teamID string) (*models.Item, error)

// Lists - return slice (empty slice for no results, error for failures)
func (s *Service) List(ctx context.Context, teamID string) ([]models.Item, error)

// Mutations - return created/updated entity
func (s *Service) Create(ctx context.Context, teamID, userID string, req *dto.CreateRequest) (*models.Item, error)
func (s *Service) Update(ctx context.Context, id, teamID string, req *dto.UpdateRequest) (*models.Item, error)

// Deletions - return error only
func (s *Service) Delete(ctx context.Context, id, teamID string) error

// Actions - return error only
func (s *Service) Trigger(ctx context.Context, id, teamID string) error
```

### Service Error Handling

Always wrap errors with context:
```go
func (s *Service) Create(ctx context.Context, req *dto.CreateRequest) (*models.Item, error) {
    item := &models.Item{
        Name: req.Name,
    }

    if err := s.repos.Item().Create(ctx, item); err != nil {
        return nil, fmt.Errorf("failed to create item: %w", err)
    }

    return item, nil
}
```

---

## 6. Repositories

Repositories handle data access and database operations.

### Repository Registry

Each module has a registry that provides access to all repositories:

```go
package repositories

import "gorm.io/gorm"

type Registry struct {
    db   *gorm.DB
    item *ItemRepository
}

func NewRegistry(db *gorm.DB) *Registry {
    return &Registry{db: db}
}

func (r *Registry) Item() *ItemRepository {
    if r.item == nil {
        r.item = NewItemRepository(r.db)
    }
    return r.item
}
```

### Repository Structure

```go
package repositories

import (
    "context"
    "errors"

    "github.com/kkz6/launch-go/internal/modules/mymodule/models"
    "gorm.io/gorm"
)

var ErrItemNotFound = errors.New("item not found")

type ItemRepository struct {
    db *gorm.DB
}

func NewItemRepository(db *gorm.DB) *ItemRepository {
    return &ItemRepository{db: db}
}

// FindByID finds an item by ID with team scoping
func (r *ItemRepository) FindByID(ctx context.Context, id, teamID string) (*models.Item, error) {
    var item models.Item
    err := r.db.WithContext(ctx).
        Where("id = ? AND team_id = ?", id, teamID).
        First(&item).Error

    if errors.Is(err, gorm.ErrRecordNotFound) {
        return nil, ErrItemNotFound
    }
    if err != nil {
        return nil, err
    }
    return &item, nil
}

// List returns all items for a team
func (r *ItemRepository) List(ctx context.Context, teamID string) ([]models.Item, error) {
    var items []models.Item
    err := r.db.WithContext(ctx).
        Where("team_id = ?", teamID).
        Order("created_at DESC").
        Find(&items).Error
    return items, err
}

// Create creates a new item
func (r *ItemRepository) Create(ctx context.Context, item *models.Item) error {
    return r.db.WithContext(ctx).Create(item).Error
}

// Update updates an item
func (r *ItemRepository) Update(ctx context.Context, item *models.Item) error {
    return r.db.WithContext(ctx).Save(item).Error
}

// Delete deletes an item
func (r *ItemRepository) Delete(ctx context.Context, id, teamID string) error {
    return r.db.WithContext(ctx).
        Where("id = ? AND team_id = ?", id, teamID).
        Delete(&models.Item{}).Error
}
```

---

## 7. DTOs (Data Transfer Objects)

DTOs define the structure of API requests and responses.

### Request DTOs

Located in `dto/requests.go`:

```go
package dto

// CreateItemRequest defines the request body for creating an item
type CreateItemRequest struct {
    Name        string  `json:"name" validate:"required,min=1,max=255"`
    Description *string `json:"description" validate:"omitempty,max=1000"`
    ServerID    string  `json:"server_id" validate:"required,ulid"`
}

// UpdateItemRequest defines the request body for updating an item
type UpdateItemRequest struct {
    Name        *string `json:"name" validate:"omitempty,min=1,max=255"`
    Description *string `json:"description" validate:"omitempty,max=1000"`
}

// Normalize cleans up the request data
func (r *CreateItemRequest) Normalize() {
    if r.Description != nil && strings.TrimSpace(*r.Description) == "" {
        r.Description = nil
    }
}
```

### Response DTOs

Located in `dto/responses.go`:

```go
package dto

import (
    "time"

    "github.com/kkz6/launch-go/internal/modules/mymodule/models"
)

// ItemResponse defines the response structure for an item
type ItemResponse struct {
    ID          string  `json:"id"`
    Name        string  `json:"name"`
    Description *string `json:"description,omitempty"`
    ServerID    string  `json:"server_id"`
    CreatedAt   string  `json:"created_at"`
    UpdatedAt   string  `json:"updated_at"`
}

// ToItemResponse converts a model to response DTO
func ToItemResponse(item *models.Item) *ItemResponse {
    if item == nil {
        return nil
    }
    return &ItemResponse{
        ID:          item.ID,
        Name:        item.Name,
        Description: item.Description,
        ServerID:    item.ServerID,
        CreatedAt:   item.CreatedAt.Format(time.RFC3339),
        UpdatedAt:   item.UpdatedAt.Format(time.RFC3339),
    }
}

// ToItemResponses converts a slice of models to response DTOs
func ToItemResponses(items []models.Item) []ItemResponse {
    result := make([]ItemResponse, len(items))
    for i := range items {
        result[i] = *ToItemResponse(&items[i])
    }
    return result
}
```

### Validation Tags

Common validation tags (using go-playground/validator):

| Tag | Description | Example |
|-----|-------------|---------|
| `required` | Field must be present | `validate:"required"` |
| `omitempty` | Skip validation if empty | `validate:"omitempty,min=1"` |
| `min` | Minimum length/value | `validate:"min=1"` |
| `max` | Maximum length/value | `validate:"max=255"` |
| `email` | Valid email format | `validate:"email"` |
| `url` | Valid URL format | `validate:"url"` |
| `oneof` | One of allowed values | `validate:"oneof=draft published"` |
| `ulid` | Valid ULID format | `validate:"ulid"` |

---

## 8. Models

Models define database table structures.

### Model Structure

```go
package models

import (
    "time"

    "github.com/oklog/ulid/v2"
    "gorm.io/gorm"
)

type Item struct {
    ID          string     `gorm:"type:char(26);primaryKey"`
    TeamID      string     `gorm:"type:char(26);index"`
    ServerID    string     `gorm:"type:char(26);index"`
    Name        string     `gorm:"type:varchar(255)"`
    Description *string    `gorm:"type:text"`
    Status      string     `gorm:"type:varchar(50);default:'pending'"`
    CreatedAt   time.Time
    UpdatedAt   time.Time
    DeletedAt   gorm.DeletedAt `gorm:"index"`

    // Relations
    Team   *Team   `gorm:"foreignKey:TeamID"`
    Server *Server `gorm:"foreignKey:ServerID"`
}

// BeforeCreate generates ULID for new records
func (m *Item) BeforeCreate(tx *gorm.DB) error {
    if m.ID == "" {
        m.ID = ulid.Make().String()
    }
    return nil
}

// TableName returns the table name
func (Item) TableName() string {
    return "items"
}
```

### Model Conventions

1. **Primary Key**: Use ULID (26 characters) as `char(26)`
2. **Foreign Keys**: Name as `{relation}_id` with index
3. **Timestamps**: Include `CreatedAt`, `UpdatedAt`, and `DeletedAt` (soft delete)
4. **Status Fields**: Use `varchar(50)` with default value
5. **Relations**: Define as pointer types

---

## 9. Jobs & Background Tasks

Jobs are background tasks processed by the queue worker using asynq.

### Job Interface

Jobs implement the `Handler` interface from `internal/pkg/jobs`:

```go
// Handler is the minimal interface for jobs
type Handler interface {
    Handle(ctx context.Context) error
}

// FailableHandler is for jobs that need cleanup on failure
type FailableHandler interface {
    Handler
    Failed(ctx context.Context, err error)
}
```

### Job Structure

Each job consists of:
1. A type constant (e.g., `TypeRebootServer = "server:reboot"`)
2. A payload struct with JSON tags
3. A job struct with `Deps` and `Payload` fields
4. A constructor that returns `pkgjobs.Handler`
5. A `Handle` method implementing the job logic
6. Optional `Failed` method for cleanup
7. A task creator function for dispatching

```go
package jobs

import (
    "context"
    "fmt"

    "github.com/hibiken/asynq"

    "github.com/kkz6/launch-go/internal/modules/server/models"
    "github.com/kkz6/launch-go/internal/modules/server/tasks"
    pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

// 1. Type constant
const TypeRebootServer = "server:reboot"

// 2. Payload struct
type RebootServerPayload struct {
    ServerID string  `json:"server_id"`
    UserID   *string `json:"user_id,omitempty"`
}

// 3. Job struct - references module deps and payload
type RebootServerJob struct {
    Deps    *JobDeps
    Payload RebootServerPayload

    // Private fields for loaded data
    server *models.Server
}

// 4. Constructor - returns pkgjobs.Handler, uses package-level deps
func NewRebootServerJob(p RebootServerPayload) pkgjobs.Handler {
    return &RebootServerJob{Deps: deps, Payload: p}
}

// 5. Handle method - implements the job logic
func (j *RebootServerJob) Handle(ctx context.Context) error {
    var err error
    j.server, err = j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
    if err != nil {
        return fmt.Errorf("find server: %w", err)
    }

    task := tasks.RebootServer()
    _, err = j.Deps.RunTask(j.server, task).
        AsRoot().
        Dispatch(ctx)

    j.Deps.Logger.Info().
        Str("server_id", j.server.ID).
        Msg("server reboot initiated")

    j.Deps.BroadcastServerEvent(j.server, "server.rebooting", map[string]any{
        "server_id": j.server.ID,
    })

    return nil
}

// 6. Optional Failed method for cleanup
func (j *RebootServerJob) Failed(ctx context.Context, err error) {
    j.Deps.Logger.Error().Err(err).
        Str("server_id", j.Payload.ServerID).
        Msg("failed to reboot server")
}

// 7. Task creator for dispatching
func NewRebootServerTask(serverID string, userID *string) (*asynq.Task, error) {
    return pkgjobs.TaskWithID(TypeRebootServer,
        RebootServerPayload{ServerID: serverID, UserID: userID},
        pkgjobs.Dedup("reboot", serverID), // Deduplication ID
    )
}
```

### Job Dependencies (JobDeps)

Each module has a `JobDeps` struct that embeds `*pkgjobs.Deps` and adds module-specific dependencies:

```go
package jobs

import (
    "github.com/kkz6/launch-go/internal/modules/server/contracts"
    "github.com/kkz6/launch-go/internal/modules/server/providers"
    "github.com/kkz6/launch-go/internal/modules/server/tasks"
    "github.com/kkz6/launch-go/internal/pkg/app"
    pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

// JobDeps holds all dependencies for module jobs
type JobDeps struct {
    *pkgjobs.Deps  // DB, Logger, Queue, Broadcaster, Dispatcher
    Repos           contracts.RepositoryRegistry
    ProviderFactory *providers.Factory
    TaskRunnerDeps  *tasks.TaskRunnerDeps
}

// NewJobDeps creates JobDeps from app dependencies
func NewJobDeps(appDeps app.Deps, repos contracts.RepositoryRegistry) *JobDeps {
    return &JobDeps{
        Deps: &pkgjobs.Deps{
            DB:          appDeps.DB,
            Logger:      appDeps.Logger,
            Queue:       appDeps.Queue,
            Broadcaster: appDeps.WebSocket,
            Dispatcher:  appDeps.Dispatcher,
        },
        Repos: repos,
        // ... additional fields
    }
}

// RunTask creates a task runner for a server
func (d *JobDeps) RunTask(server *models.Server, task pkgtaskrunner.Task) *tasks.TaskRunner {
    return d.TaskRunnerDeps.NewRunner(server, task)
}

// BroadcastServerEvent broadcasts an event for a server
func (d *JobDeps) BroadcastServerEvent(server *models.Server, event string, data any) {
    if server != nil {
        d.BroadcastToTeam(server.TeamID, event, data)
    }
}
```

The `pkgjobs.Deps` struct provides:
- `DB *gorm.DB` - Database connection
- `Logger *zerolog.Logger` - Structured logger
- `Queue *queue.Client` - Job queue client
- `Broadcaster broadcast.TeamBroadcaster` - WebSocket broadcaster
- `Dispatcher taskrunner.TaskDispatcher` - SSH task dispatcher

And these helper methods:
- `Dispatch(jobType, payload, opts...)` - Dispatch a job
- `DispatchWithID(jobType, payload, taskID, opts...)` - Dispatch with deduplication
- `DispatchIn(jobType, payload, delay, opts...)` - Dispatch with delay
- `DispatchTask(task, opts...)` - Dispatch pre-built task
- `BroadcastToTeam(teamID, event, data)` - Send websocket event

### Job Registration

Jobs are registered in the module's `register.go` file:

```go
package jobs

import (
    "github.com/hibiken/asynq"

    "github.com/kkz6/launch-go/internal/modules/server/contracts"
    "github.com/kkz6/launch-go/internal/pkg/app"
    pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

// Package-level deps set during registration
var deps *JobDeps

// Register initializes and registers all module job handlers
func Register(mux *asynq.ServeMux, appDeps app.Deps, repos contracts.RepositoryRegistry) {
    deps = NewJobDeps(appDeps, repos)
    registerHandlers(mux)
}

func registerHandlers(mux *asynq.ServeMux) {
    // Use RegisterTyped for automatic JSON unmarshaling
    pkgjobs.RegisterTyped(mux, TypeRebootServer, NewRebootServerJob)
    pkgjobs.RegisterTyped(mux, TypeProvisionServer, NewProvisionServerJob)
    pkgjobs.RegisterTyped(mux, TypeDeleteServer, NewDeleteServerJob)
    // ... more registrations
}
```

### Task Creator Helpers

Use helpers from `internal/pkg/jobs/task.go` to create asynq tasks:

```go
// Basic task creation
task, err := pkgjobs.Task(TypeRebootServer, payload)

// Task with deduplication ID
task, err := pkgjobs.TaskWithID(TypeRebootServer, payload, "reboot:server123")

// Create deduplication ID from parts
id := pkgjobs.Dedup("reboot", serverID) // "reboot:server123"
```

### Dispatching Jobs

**From a service (using Deps):**
```go
func (s *Service) TriggerReboot(ctx context.Context, serverID string) error {
    return s.deps.Dispatch(jobs.TypeRebootServer, jobs.RebootServerPayload{
        ServerID: serverID,
    })
}
```

**Using task creator functions:**
```go
task, err := jobs.NewRebootServerTask(serverID, &userID)
if err != nil {
    return err
}
_, err = s.queue.Enqueue(task)
```

**With options:**
```go
s.deps.DispatchIn(jobType, payload, 5*time.Minute)  // Delayed
s.deps.DispatchToQueue(jobType, payload, "low")     // Specific queue
```

---

## 10. Task Runner (SSH Tasks)

Tasks are SSH scripts that run on remote servers. The task runner handles both local development (live SSH streaming) and production mode (HTTP callbacks with background execution).

### TaskRunner Overview

The `TaskRunner` in `internal/modules/server/tasks/runner.go` provides a fluent builder API for configuring and executing SSH tasks on servers.

```go
// Basic usage
result, err := deps.RunTask(server, task).
    AsRoot().
    Dispatch(ctx)

// Full configuration
result, err := deps.RunTask(server, task).
    AsRoot().                      // Run as root user
    TrackInDB().                   // Save task to database
    WithOutputPolling(30).         // Poll output every 30 seconds (production mode)
    OnComplete("job:type", payload). // Dispatch job on success
    OnFailed("job:cleanup", payload). // Dispatch job on failure
    Throw().                       // Return error if task fails
    Dispatch(ctx)
```

### TaskRunner Builder Methods

| Method | Description |
|--------|-------------|
| `AsRoot()` | Run script as root user |
| `AsUser(username...)` | Run as specific user (defaults to server's SSH user) |
| `TrackInDB()` | Save task record to database for tracking |
| `ThrowOnError()` / `Throw()` | Return error if task fails (instead of just result) |
| `WithCallbacks(urls)` | Set webhook URLs for background execution |
| `WithOutputPolling(seconds)` | Poll server for output at interval (production mode) |
| `OnComplete(jobType, payload)` | Dispatch job when task succeeds |
| `OnFailed(jobType, payload)` | Dispatch job when task fails |
| `OnTimeout(jobType, payload)` | Dispatch job when task times out |

### Execution Methods

| Method | Description | Use Case |
|--------|-------------|----------|
| `Run(ctx)` | Synchronous execution, waits for completion | Quick tasks, need immediate result |
| `Dispatch(ctx)` | Alias for `Run(ctx)` | Preferred name for jobs |
| `RunAsync(ctx)` | Returns immediately, task runs in goroutine | Long tasks in local mode |
| `RunInBackground(ctx)` | Uses HTTP callbacks for completion | Production mode, long-running tasks |

### Execution Modes

The TaskRunner automatically selects the execution mode based on configuration:

**Local/Dev Mode (Live SSH Streaming):**
```
1. Task starts → persistent SSH connection maintained
2. StreamMonitor attaches to stdout/stderr
3. Output broadcast to WebSocket in real-time
4. Task completes → callbacks invoked directly in-process
```

Use when: Development, short tasks, need live output streaming.

**Production Mode (HTTP Callbacks):**
```
1. Task starts → script wrapped with callback URLs
2. Script runs in background on server (nohup)
3. FetchTaskOutput job polls server for output (if WithOutputPolling set)
4. Script completes → calls webhook endpoint with status
5. Webhook updates database, fetches final output
6. CallbackHandler methods invoked to handle completion
```

Use when: Production, long-running tasks, unreliable connections.

```go
// Production mode with output polling
result, err := deps.RunTask(server, task).
    AsRoot().
    TrackInDB().
    WithCallbacks(callbackURLs).    // Enable HTTP callbacks
    WithOutputPolling(30).          // Poll every 30 seconds
    RunInBackground(ctx)
```

### Output Polling (Production Mode)

When `WithOutputPolling(seconds)` is set and the task runs in background mode, the TaskRunner dispatches a `FetchTaskOutput` job that:

1. SSHes into the server at the specified interval
2. Reads the task's output file using `tail -c`
3. Updates the task output in the database
4. Broadcasts output updates via WebSocket
5. Self-reschedules until the task completes

```go
// Enable 30-second output polling
deps.RunTask(server, task).
    WithOutputPolling(30).
    RunInBackground(ctx)
```

### Webhook Endpoints

Production mode uses these webhook endpoints for task completion:

| Endpoint | Purpose |
|----------|---------|
| `POST /webhooks/tasks/:id/finished` | Task completed successfully |
| `POST /webhooks/tasks/:id/failed` | Task failed with exit code |
| `POST /webhooks/tasks/:id/timeout` | Task timed out |
| `POST /webhooks/tasks/:id/callback` | Custom callback for progress updates |

The custom callback endpoint can be called by scripts during execution to trigger output fetch and custom logic without changing task status.

### Task Interfaces

**Task Interface** - For basic script execution:
```go
type Task interface {
    Name() string
    Script() string
    Timeout() time.Duration
    OnOutput(output string)
    OnFinished(ctx context.Context, result *TaskResult)
    OnFailed(ctx context.Context, result *TaskResult)
    OnTimeout(ctx context.Context, result *TaskResult)
}
```

**CallbackHandler Interface** - For tasks with server-side callbacks (different signatures to allow types to implement both):
```go
type CallbackHandler interface {
    OnSuccess(ctx context.Context, cbCtx *CallbackContext, taskID string) error
    OnFailure(ctx context.Context, cbCtx *CallbackContext, taskID string, exitCode int) error
    OnExpired(ctx context.Context, cbCtx *CallbackContext, taskID string) error
}
```

**CallbackPayload Interface** - For tasks that can be serialized/reconstructed:
```go
type CallbackPayload interface {
    CallbackHandler
    TypeName() string
    MarshalPayload() ([]byte, error)
}
```

### Task Definition

A task with callbacks consists of:
1. A type constant for registration
2. A callback data struct (IDs only - no models)
3. A task struct embedding `*taskrunner.BaseTask`
4. A constructor that builds the script and callback data
5. `TypeName()` and `MarshalPayload()` for serialization
6. Callback methods: `OnSuccess`, `OnFailure`, `OnExpired`
7. A factory method implementing `CallbackStateFactory`

```go
package tasks

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/kkz6/launch-go/internal/modules/server/models"
    "github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// 1. Type constant
const InstallItemTaskType = "item:install"

// 2. Callback data - ONLY IDs and simple values (serialized to DB)
type installCallbackData struct {
    ItemID   string `json:"item_id"`
    ServerID string `json:"server_id"`  // Include ALL IDs needed by callbacks
    TeamID   string `json:"team_id"`
}

// 3. Task struct
type InstallItemTask struct {
    *taskrunner.BaseTask
    callback installCallbackData
}

// 4. Constructor - builds script and populates callback data
func NewInstallItemTask(item *models.Item, server *models.Server) *InstallItemTask {
    script := buildInstallScript(item)

    return &InstallItemTask{
        BaseTask: taskrunner.NewBaseTask(
            taskrunner.WithName("Install Item"),
            taskrunner.WithScript(script),
            taskrunner.WithTimeoutSeconds(300),
        ),
        callback: installCallbackData{
            ItemID:   item.ID,
            ServerID: server.ID,  // Needed for follow-up jobs
            TeamID:   item.TeamID,
        },
    }
}

// 5. Serialization methods
func (t *InstallItemTask) TypeName() string {
    return InstallItemTaskType
}

func (t *InstallItemTask) MarshalPayload() ([]byte, error) {
    return json.Marshal(t.callback)
}

// 6. Callback methods
func (t *InstallItemTask) OnSuccess(ctx context.Context, cbCtx *taskrunner.CallbackContext, taskID string) error {
    // Update status
    if err := cbCtx.DB.Model(&models.Item{}).
        Where("id = ?", t.callback.ItemID).
        Update("status", "installed").Error; err != nil {
        return fmt.Errorf("failed to update item status: %w", err)
    }

    // Broadcast event
    cbCtx.BroadcastToTeam(t.callback.TeamID, "item.installed", map[string]any{
        "item_id": t.callback.ItemID,
    })

    // Dispatch follow-up job
    return cbCtx.DispatchJob("item:configure", map[string]string{
        "item_id":   t.callback.ItemID,
        "server_id": t.callback.ServerID,
    })
}

func (t *InstallItemTask) OnFailure(ctx context.Context, cbCtx *taskrunner.CallbackContext, taskID string, exitCode int) error {
    cbCtx.Logger.Error().
        Str("task_id", taskID).
        Int("exit_code", exitCode).
        Msg("Item installation failed")

    return cbCtx.DB.Model(&models.Item{}).
        Where("id = ?", t.callback.ItemID).
        Update("status", "failed").Error
}

func (t *InstallItemTask) OnExpired(ctx context.Context, cbCtx *taskrunner.CallbackContext, taskID string) error {
    cbCtx.Logger.Error().
        Str("task_id", taskID).
        Msg("Item installation timed out")

    return cbCtx.DB.Model(&models.Item{}).
        Where("id = ?", t.callback.ItemID).
        Update("status", "timeout").Error
}

// 7. Factory method - enables reconstruction from serialized data
func (c installCallbackData) NewTask() taskrunner.CallbackHandler {
    return &InstallItemTask{
        BaseTask: taskrunner.NewBaseTask(),
        callback: c,
    }
}
```

### Task Registration

Register tasks in the module's `tasks/register.go`:

```go
package tasks

import "github.com/kkz6/launch-go/internal/pkg/taskrunner"

func RegisterTaskCallbacks() {
    // Use RegisterCallbackState when callback data implements NewTask()
    taskrunner.RegisterCallbackState[installCallbackData](InstallItemTaskType)
    taskrunner.RegisterCallbackState[provisionCallbackData](ProvisionServerTaskType)
}
```

### Callback Context

The `CallbackContext` provides dependencies to callback handlers:

```go
type CallbackContext struct {
    broadcast.Mixin  // BroadcastToTeam, BroadcastToServer, etc.
    DB       *gorm.DB
    Queue    QueueClient
    Logger   *zerolog.Logger
    Notifier NotifierService
}
```

Available methods:
- `BroadcastToTeam(teamID, event, data)` - Send websocket event
- `DispatchJob(jobType, payload)` - Dispatch a follow-up job
- `NotifyTeam(ctx, teamID, notification)` - Send notification
- `NotifyChannel(ctx, channelID, notification)` - Send to specific channel

### Running Tasks

Tasks are run via the module's TaskRunner (usually through `JobDeps.RunTask()`):

```go
// Simple synchronous execution
result, err := j.Deps.RunTask(server, task).
    AsRoot().
    Run(ctx)

if err != nil {
    return fmt.Errorf("task execution failed: %w", err)
}

if result.TaskResult != nil && result.TaskResult.IsSuccessful() {
    output := result.TaskResult.Output
    // Process output...
}
```

**Background execution with output polling (production mode):**
```go
// Long-running task with periodic output updates
taskModel, err := j.Deps.RunTask(server, task).
    AsRoot().
    TrackInDB().
    WithOutputPolling(30).  // Poll every 30 seconds
    RunInBackground(ctx)

if err != nil {
    return fmt.Errorf("failed to start task: %w", err)
}

// Task is now running in background
// Output will be polled and broadcast via WebSocket
// Callbacks will be invoked when task completes
```

**Using completion jobs instead of callbacks:**
```go
// Dispatch jobs on completion (simpler than implementing CallbackHandler)
j.Deps.RunTask(server, task).
    AsRoot().
    TrackInDB().
    OnComplete("server:configure", ConfigurePayload{ServerID: server.ID}).
    OnFailed("server:cleanup", CleanupPayload{ServerID: server.ID}).
    OnTimeout("server:notify_timeout", NotifyPayload{ServerID: server.ID}).
    RunInBackground(ctx)
```

### TaskResult

The `TaskRunnerResult` contains:

```go
type TaskRunnerResult struct {
    TaskModel  *models.Task           // Database record (if TrackInDB)
    TaskResult *taskrunner.TaskResult // Execution result
    Error      error                  // Execution error
}

// Helper methods
result.IsSuccessful() bool  // True if exit code 0
result.GetOutput() string   // Task output
result.GetExitCode() int    // Exit code (-1 if unknown)
```

### Script Helpers

The taskrunner provides helpers for building scripts:

```go
// Wrap script with shell defaults
script := taskrunner.WrapScript(myScript)
// Result: #!/bin/bash\nset -euo pipefail\nexport DEBIAN_FRONTEND=noninteractive\n\n...

// Get shell defaults for embedding
defaults := taskrunner.ShellDefaults()

// Get common bash functions
funcs := taskrunner.CommonFunctions()  // httpPostSilently, etc.
funcs := taskrunner.AptFunctions()     // waitForAptUnlock, etc.
```

### Key Rules

1. **Callback data must only contain IDs** - Never store model pointers or complex objects
2. **Include ALL IDs needed by callbacks** - If a callback dispatches a job that needs `server_id`, include it
3. **Use typed payloads for job dispatch** - Avoid `map[string]string` which has no compile-time safety
4. **Register in register.go** - Explicit registration, not `init()` magic
5. **Callbacks run in ALL execution modes** - Whether sync, async, or background with HTTP callbacks

---

## 11. Error Handling

### Response Helpers

Use helpers from `internal/pkg/response/`:

```go
// Success responses
response.OK(c, "Message", data)           // 200
response.Created(c, "Message", data)      // 201
response.NoContent(c)                      // 204

// Error responses
response.BadRequest(c, "Message")          // 400
response.Unauthorized(c, "Message")        // 401
response.Forbidden(c, "Message")           // 403
response.NotFound(c, "Message")            // 404
response.Conflict(c, "Message")            // 409
response.ValidationError(c, errors)        // 422
response.InternalError(c, "Message")       // 500

// Auto-detect error type
response.HandleError(c, err)               // Maps error to appropriate status
```

### Custom Errors

Define module-specific errors in `errors/errors.go`:

```go
package errors

import apperrors "github.com/kkz6/launch-go/internal/pkg/errors"

var (
    ErrItemNotFound    = apperrors.NotFound("Item not found")
    ErrItemExists      = apperrors.Conflict("Item already exists")
    ErrInvalidStatus   = apperrors.BadRequest("Invalid status")
)
```

### Error Response Format

```json
{
    "success": false,
    "message": "Error message here"
}
```

Validation error format:
```json
{
    "success": false,
    "message": "Validation failed",
    "errors": {
        "name": ["Name is required"],
        "email": ["Invalid email format"]
    }
}
```

---

## 12. Validation

### Request Validation

```go
import "github.com/kkz6/launch-go/internal/pkg/validator"

var req dto.CreateRequest
if err := c.BodyParser(&req); err != nil {
    return response.BadRequest(c, "Invalid request body")
}

if errs := validator.Validate(&req); errs != nil {
    return response.ValidationError(c, errs)
}
```

### Custom Validation Rules

Register custom validation in `internal/pkg/validator/validator.go`:

```go
func init() {
    validate.RegisterValidation("ulid", validateULID)
}

func validateULID(fl validator.FieldLevel) bool {
    _, err := ulid.Parse(fl.Field().String())
    return err == nil
}
```

---

## 13. WebSocket

WebSocket handlers are in `internal/modules/websocket/handlers/`.

### WebSocket Handler Pattern

```go
func (h *Handler) StreamLogs(c *websocket.Conn) {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Read parameters
    serverID := c.Query("server_id")

    // Validate...

    // Stream data
    ticker := time.NewTicker(time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            data := fetchData()
            if err := c.WriteJSON(data); err != nil {
                return
            }
        case <-ctx.Done():
            return
        }
    }
}
```

---

## 14. Testing

### Test File Location

Tests are placed alongside the code they test with `_test.go` suffix:
```
services/
  item_service.go
  item_service_test.go
```

### Test Structure

```go
package services

import (
    "context"
    "testing"
)

func TestService_Create(t *testing.T) {
    // Setup
    db := setupTestDB(t)
    repos := repositories.NewRegistry(db)
    service := NewService(repos)

    // Test
    req := &dto.CreateRequest{Name: "Test"}
    item, err := service.Create(context.Background(), "team-id", "user-id", req)

    // Assert
    if err != nil {
        t.Fatalf("Expected no error, got %v", err)
    }
    if item.Name != "Test" {
        t.Errorf("Expected name 'Test', got '%s'", item.Name)
    }
}
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run specific package tests
go test ./internal/modules/mymodule/...

# Run with verbose output
go test -v ./...

# Run specific test
go test -v -run TestService_Create ./internal/modules/mymodule/services/
```

---

## Quick Reference

### Creating a New Endpoint

1. Add route in `routes.go`
2. Create handler method
3. Add request/response DTOs
4. Implement service method
5. Add repository method if needed
6. Write tests

### Adding a Background Job

1. Define job type constant (e.g., `TypeProcessItem = "item:process"`)
2. Create payload struct with JSON tags
3. Create job struct with `Deps *JobDeps` and `Payload` fields
4. Create constructor returning `pkgjobs.Handler` (uses package-level `deps`)
5. Implement `Handle(ctx context.Context) error` method
6. Optional: Implement `Failed(ctx context.Context, err error)` for cleanup
7. Create task creator function for dispatching
8. Register with `pkgjobs.RegisterTyped(mux, TypeXxx, NewXxxJob)` in `register.go`

### Adding an SSH Task

1. Define task type constant (e.g., `InstallItemTaskType = "item:install"`)
2. Create callback data struct (IDs only - no model pointers)
3. Create task struct embedding `*taskrunner.BaseTask` and callback data
4. Create constructor that builds script and populates callback data
5. Implement `TypeName()` and `MarshalPayload()` for serialization
6. Implement callback methods: `OnSuccess`, `OnFailure`, `OnExpired`
7. Implement `NewTask()` on callback data struct (for `CallbackStateFactory`)
8. Register with `taskrunner.RegisterCallbackState[callbackData](TaskType)` in `register.go`
9. Run via `deps.RunTask(server, task).Dispatch(ctx)`

### Running SSH Tasks (Quick Examples)

**Simple synchronous task:**
```go
result, err := deps.RunTask(server, task).AsRoot().Run(ctx)
```

**Background task with output polling:**
```go
deps.RunTask(server, task).
    AsRoot().
    TrackInDB().
    WithOutputPolling(30).
    RunInBackground(ctx)
```

**Task with completion jobs:**
```go
deps.RunTask(server, task).
    AsRoot().
    TrackInDB().
    OnComplete("item:configure", payload).
    OnFailed("item:cleanup", payload).
    RunInBackground(ctx)
```

---

*This document should be updated as new patterns emerge.*
