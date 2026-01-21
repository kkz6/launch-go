# Launch-Go Framework Guide

A comprehensive guide for developers working on the Launch-Go codebase. This document covers architecture patterns, conventions, and how to implement new features consistently.

**Last Updated:** 2026-01-21

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

Jobs are background tasks processed by the queue worker.

### Job Structure

```go
package jobs

import (
    "context"
    "encoding/json"

    "github.com/hibiken/asynq"
)

const TypeProcessItem = "item:process"

type ProcessItemPayload struct {
    ItemID   string `json:"item_id"`
    TeamID   string `json:"team_id"`
    ServerID string `json:"server_id"`
}

type ProcessItemJob struct {
    ctx *JobContext
}

func NewProcessItemJob(ctx *JobContext) *ProcessItemJob {
    return &ProcessItemJob{ctx: ctx}
}

func (j *ProcessItemJob) ProcessTask(ctx context.Context, task *asynq.Task) error {
    var payload ProcessItemPayload
    if err := json.Unmarshal(task.Payload(), &payload); err != nil {
        return err
    }

    // Process the item...

    return nil
}
```

### Dispatching Jobs

```go
// In service
func (s *Service) TriggerProcess(ctx context.Context, itemID, teamID, serverID string) error {
    payload := jobs.ProcessItemPayload{
        ItemID:   itemID,
        TeamID:   teamID,
        ServerID: serverID,
    }

    data, err := json.Marshal(payload)
    if err != nil {
        return err
    }

    task := asynq.NewTask(jobs.TypeProcessItem, data)
    _, err = s.queue.Enqueue(task)
    return err
}
```

---

## 10. Task Runner (SSH Tasks)

Tasks are SSH scripts that run on remote servers.

### Task Definition

```go
package tasks

import (
    "github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

const InstallItemTaskType = "item:install"

type InstallItemTask struct {
    *taskrunner.BaseTask
    callback callbackData
}

type callbackData struct {
    ItemID   string `json:"item_id"`
    ServerID string `json:"server_id"`
    TeamID   string `json:"team_id"`
}

func NewInstallItemTask(item *models.Item, server *models.Server) *InstallItemTask {
    script := buildInstallScript(item)

    return &InstallItemTask{
        BaseTask: taskrunner.NewBaseTask(
            taskrunner.WithName("Install Item"),
            taskrunner.WithScript(script),
            taskrunner.WithTimeoutSeconds(300),
        ),
        callback: callbackData{
            ItemID:   item.ID,
            ServerID: server.ID,
            TeamID:   item.TeamID,
        },
    }
}

func (t *InstallItemTask) TypeName() string {
    return InstallItemTaskType
}

func (t *InstallItemTask) MarshalPayload() ([]byte, error) {
    return json.Marshal(t.callback)
}

// Callbacks
func (t *InstallItemTask) OnSuccess(ctx context.Context, cbCtx *taskrunner.CallbackContext, taskID string) error {
    return cbCtx.DB.Model(&models.Item{}).
        Where("id = ?", t.callback.ItemID).
        Update("status", "installed").Error
}

func (t *InstallItemTask) OnFailure(ctx context.Context, cbCtx *taskrunner.CallbackContext, taskID string, exitCode int) error {
    return cbCtx.DB.Model(&models.Item{}).
        Where("id = ?", t.callback.ItemID).
        Update("status", "failed").Error
}
```

### Task Registration

```go
// In tasks/register.go
package tasks

import "github.com/kkz6/launch-go/internal/pkg/taskrunner"

func RegisterTaskCallbacks() {
    taskrunner.RegisterCallbackState[callbackData](InstallItemTaskType)
}
```

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

1. Define job type constant
2. Create payload struct
3. Create job struct with `ProcessTask` method
4. Register in worker
5. Dispatch from service

### Adding an SSH Task

1. Define task type constant
2. Create task struct with callback data
3. Implement `OnSuccess`, `OnFailure`, `OnExpired`
4. Register callback in `register.go`
5. Dispatch via task runner

---

*This document should be updated as new patterns emerge.*
