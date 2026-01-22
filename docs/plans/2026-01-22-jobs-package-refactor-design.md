# Jobs Package Refactor Design

## Overview

Simplify the jobs package by flattening the context hierarchy and removing unnecessary abstractions while preserving the `Failed()` callback pattern.

## Problem

The current architecture has:
- 4-level embedding: `Base` → `ModuleContext[R]` → `ServerContext[R]` → module `JobContext`
- Naming collision with Go's `context.Context`
- Overlapping task creation APIs (`NewTask`, `TaskBuilder`, etc.)
- Wrapper methods that don't add value (`LogInfo`, `LogError`)
- 7-parameter constructors

## Solution

Flatten to a single `Deps` struct, keep struct-based jobs for `Failed()` support, consolidate APIs.

---

## Design

### 1. Flat Deps Struct

```go
// internal/pkg/jobs/deps.go

package jobs

type Deps struct {
    DB          *gorm.DB
    Logger      *zerolog.Logger
    Queue       *queue.Client
    Broadcaster broadcast.TeamBroadcaster
    Dispatcher  taskrunner.TaskDispatcher  // nil if module doesn't need SSH
}

func (d *Deps) Dispatch(jobType string, payload any, opts ...asynq.Option) error {
    if d.Queue == nil {
        return nil
    }
    task, err := Task(jobType, payload, opts...)
    if err != nil {
        return err
    }
    _, err = d.Queue.Enqueue(task)
    return err
}

func (d *Deps) DispatchIn(jobType string, payload any, delay time.Duration, opts ...asynq.Option) error {
    opts = append(opts, asynq.ProcessIn(delay))
    return d.Dispatch(jobType, payload, opts...)
}

func (d *Deps) DispatchTask(task *asynq.Task, opts ...asynq.Option) error {
    if d.Queue == nil {
        return nil
    }
    _, err := d.Queue.Enqueue(task, opts...)
    return err
}

func (d *Deps) BroadcastToTeam(teamID, event string, data any) {
    if d.Broadcaster != nil && teamID != "" {
        d.Broadcaster.BroadcastToTeam(teamID, event, data)
    }
}
```

### 2. Handler Interfaces

```go
// internal/pkg/jobs/handler.go

package jobs

import "context"

type Handler interface {
    Handle(ctx context.Context) error
}

type FailableHandler interface {
    Handler
    Failed(ctx context.Context, err error)
}
```

### 3. Registration

```go
// internal/pkg/jobs/register.go

package jobs

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/hibiken/asynq"
)

type JobFactory func(payload []byte) (Handler, error)

func Register(mux *asynq.ServeMux, jobType string, factory JobFactory) {
    mux.HandleFunc(jobType, func(ctx context.Context, t *asynq.Task) error {
        job, err := factory(t.Payload())
        if err != nil {
            return fmt.Errorf("create job: %w", err)
        }

        if err := job.Handle(ctx); err != nil {
            if failable, ok := job.(FailableHandler); ok {
                failable.Failed(ctx, err)
            }
            return err
        }

        return nil
    })
}

func RegisterTyped[P any](mux *asynq.ServeMux, jobType string, newJob func(P) Handler) {
    Register(mux, jobType, func(payload []byte) (Handler, error) {
        var p P
        if err := json.Unmarshal(payload, &p); err != nil {
            return nil, fmt.Errorf("unmarshal payload: %w", err)
        }
        return newJob(p), nil
    })
}
```

### 4. Task Creation

```go
// internal/pkg/jobs/task.go

package jobs

import (
    "encoding/json"
    "fmt"

    "github.com/hibiken/asynq"
)

const DefaultMaxRetry = 0

func Task[P any](jobType string, payload P, opts ...asynq.Option) (*asynq.Task, error) {
    data, err := json.Marshal(payload)
    if err != nil {
        return nil, fmt.Errorf("marshal payload: %w", err)
    }
    opts = append([]asynq.Option{asynq.MaxRetry(DefaultMaxRetry)}, opts...)
    return asynq.NewTask(jobType, data, opts...), nil
}

func TaskWithID[P any](jobType string, payload P, taskID string, opts ...asynq.Option) (*asynq.Task, error) {
    opts = append(opts, asynq.TaskID(taskID))
    return Task(jobType, payload, opts...)
}

func Dedup(prefix string, parts ...string) string {
    id := prefix
    for _, p := range parts {
        id += ":" + p
    }
    return id
}

func UnmarshalPayload[T any](t *asynq.Task) (T, error) {
    var payload T
    if err := json.Unmarshal(t.Payload(), &payload); err != nil {
        return payload, fmt.Errorf("unmarshal payload: %w", err)
    }
    return payload, nil
}
```

### 5. Module JobDeps (Server Example)

```go
// internal/modules/server/jobs/deps.go

package jobs

import (
    "github.com/kkz6/launch-go/internal/modules/server/contracts"
    "github.com/kkz6/launch-go/internal/modules/server/models"
    "github.com/kkz6/launch-go/internal/modules/server/providers"
    "github.com/kkz6/launch-go/internal/modules/server/tasks"
    pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
    "github.com/kkz6/launch-go/internal/pkg/app"
    "github.com/kkz6/launch-go/internal/pkg/launch/sshkey"
)

type JobDeps struct {
    *pkgjobs.Deps
    Repos           contracts.RepositoryRegistry
    ProviderFactory *providers.Factory
    TaskRunnerDeps  *tasks.TaskRunnerDeps
}

func NewJobDeps(appDeps app.Deps, repos contracts.RepositoryRegistry) *JobDeps {
    return &JobDeps{
        Deps: &pkgjobs.Deps{
            DB:          appDeps.DB,
            Logger:      appDeps.Logger,
            Queue:       appDeps.Queue,
            Broadcaster: appDeps.WebSocket,
            Dispatcher:  appDeps.Dispatcher,
        },
        Repos:           repos,
        ProviderFactory: providers.NewFactory(sshkey.NewGenerator()),
        TaskRunnerDeps:  tasks.NewTaskRunnerDeps(appDeps),
    }
}

func (d *JobDeps) ForServer(server *models.Server) *tasks.ServerTaskRunner {
    return d.TaskRunnerDeps.NewRunner(server)
}

func (d *JobDeps) BroadcastServerEvent(server *models.Server, event string, data any) {
    if server != nil {
        d.BroadcastToTeam(server.TeamID, event, data)
    }
}
```

### 6. Module Registration

```go
// internal/modules/server/jobs/register.go

package jobs

import (
    "github.com/hibiken/asynq"

    "github.com/kkz6/launch-go/internal/modules/server/contracts"
    pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
    "github.com/kkz6/launch-go/internal/pkg/app"
)

var deps *JobDeps

func Register(mux *asynq.ServeMux, appDeps app.Deps, repos contracts.RepositoryRegistry) {
    deps = NewJobDeps(appDeps, repos)
    registerHandlers(mux)
}

func registerHandlers(mux *asynq.ServeMux) {
    pkgjobs.RegisterTyped(mux, TypeRebootServer, NewRebootServerJob)
    pkgjobs.RegisterTyped(mux, TypeProvisionServer, NewProvisionServerJob)
    pkgjobs.RegisterTyped(mux, TypeDeleteServer, NewDeleteServerJob)
    // ... etc
}
```

### 7. Example Job

```go
// internal/modules/server/jobs/reboot_server.go

package jobs

import (
    "context"
    "fmt"

    "github.com/hibiken/asynq"

    "github.com/kkz6/launch-go/internal/modules/server/models"
    "github.com/kkz6/launch-go/internal/modules/server/tasks"
    pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeRebootServer = "server:reboot"

type RebootServerPayload struct {
    ServerID string  `json:"server_id"`
    UserID   *string `json:"user_id,omitempty"`
}

type RebootServerJob struct {
    Deps    *JobDeps
    Payload RebootServerPayload

    // Computed state available in Failed()
    server *models.Server
}

func NewRebootServerJob(p RebootServerPayload) pkgjobs.Handler {
    return &RebootServerJob{Deps: deps, Payload: p}
}

func (j *RebootServerJob) Handle(ctx context.Context) error {
    var err error
    j.server, err = j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
    if err != nil {
        return fmt.Errorf("find server: %w", err)
    }

    task := tasks.RebootServer()
    _, err = j.Deps.ForServer(j.server).
        WithTask(task).
        AsRoot().
        Dispatch(ctx)

    // Connection drop expected
    j.Deps.Logger.Info().
        Str("server_id", j.server.ID).
        Str("server_name", j.server.Name).
        Msg("reboot initiated")

    j.Deps.BroadcastServerEvent(j.server, "server.rebooting", map[string]any{
        "server_id": j.server.ID,
    })

    return nil
}

func (j *RebootServerJob) Failed(ctx context.Context, err error) {
    j.Deps.Logger.Error().Err(err).
        Str("server_id", j.Payload.ServerID).
        Msg("reboot failed")
}

func NewRebootServerTask(serverID string, userID *string) (*asynq.Task, error) {
    return pkgjobs.TaskWithID(TypeRebootServer,
        RebootServerPayload{ServerID: serverID, UserID: userID},
        pkgjobs.Dedup("reboot", serverID),
    )
}
```

---

## File Structure

### pkg/jobs (after)

```
internal/pkg/jobs/
├── deps.go        # Deps struct + Dispatch, BroadcastToTeam
├── handler.go     # Handler, FailableHandler interfaces
├── register.go    # Register, RegisterTyped
├── task.go        # Task, TaskWithID, Dedup, UnmarshalPayload
```

### Files to Delete

| File | Reason |
|------|--------|
| `context.go` | Replaced by `deps.go` |
| `module_context.go` | Flat hierarchy |
| `server_context.go` | Merged into module deps |
| `builder.go` | Use direct `Task()` calls |
| `helpers.go` | Consolidated into `task.go` |

---

## Migration Plan

1. **Phase 1: Create new pkg/jobs files**
   - Add `deps.go`, `handler.go`, `register.go`, `task.go`
   - Keep old files temporarily for compatibility

2. **Phase 2: Migrate server module**
   - Create `server/jobs/deps.go`
   - Update `server/jobs/register.go`
   - Convert one job (e.g., `reboot_server.go`)
   - Run tests

3. **Phase 3: Migrate remaining server jobs**
   - Convert all jobs to new pattern
   - Run tests after each batch

4. **Phase 4: Migrate other modules**
   - site, database, etc.
   - Same process as server

5. **Phase 5: Cleanup**
   - Delete old pkg/jobs files
   - Remove unused code

---

## Comparison

### Before

```go
// 4-level hierarchy
type Base struct { broadcast.Mixin; db; logger; dispatcher; queue }
type ModuleContext[R] struct { Base; repos R }
type ServerContext[R] struct { *ModuleContext[R]; taskDeps }
type JobContext struct { *ServerContext[repos]; extras }

// Generic ceremony
pkgjobs.RegisterHandler[*JobContext, Payload, *Job](mux, type, ctx, NewJob)

// Job with embedding
type RebootServerJob struct {
    pkgjobs.BaseJob[*JobContext, RebootServerPayload]
}
func (j *RebootServerJob) Handle(ctx context.Context) error {
    j.Ctx.LogInfo("message")  // Wrapper method
}
```

### After

```go
// Flat
type Deps struct { DB; Logger; Queue; Broadcaster; Dispatcher }
type JobDeps struct { *Deps; Repos; extras }

// Simple registration
pkgjobs.RegisterTyped(mux, TypeRebootServer, NewRebootServerJob)

// Plain struct
type RebootServerJob struct {
    Deps    *JobDeps
    Payload RebootServerPayload
    server  *models.Server
}
func (j *RebootServerJob) Handle(ctx context.Context) error {
    j.Deps.Logger.Info().Msg("message")  // Direct logger use
}
```

---

## Benefits

1. **Simpler mental model** - One level, not four
2. **No naming confusion** - `Deps` not `Context`
3. **Direct API usage** - `j.Deps.Logger.Info()` not `j.Ctx.LogInfo()`
4. **Cleaner registration** - No generic ceremony
5. **State in Failed()** - Computed values available for cleanup
6. **Fewer files** - 4 instead of 7 in pkg/jobs
7. **Easier testing** - Pass mock Deps directly
