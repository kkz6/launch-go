# Task Runner Package Refactoring Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Refactor the taskrunner package for better interface segregation, consistent naming, improved maintainability, AND implement missing production-mode features (output polling from server).

**Architecture:** The refactored taskrunner will have cleaner interfaces, unified naming conventions, split TaskRunner into focused components, AND proper production-mode output fetching via SSH polling jobs.

**Tech Stack:** Go 1.21+, GORM, asynq, zerolog, golang.org/x/crypto/ssh

---

## Part 1: Missing Features (Production Mode)

These features are required for proper production mode execution where HTTP callbacks are used.

### Task 1: Create GetFile Task

**Files:**
- Create: `internal/modules/server/tasks/get_file.go`

**Step 1: Create GetFile task**

This task SSHes into a server and fetches file content (used by FetchTaskOutput job).

```go
package tasks

import (
	"fmt"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// GetFileConfig holds configuration for fetching a file
type GetFileConfig struct {
	FilePath string
	MaxBytes int // Maximum bytes to fetch (default 1MB)
}

// GetFile creates a task to fetch file content from a remote server.
// Uses tail -c to fetch the last N bytes (default 1MB).
func GetFile(config GetFileConfig) *taskrunner.BaseTask {
	maxBytes := config.MaxBytes
	if maxBytes <= 0 {
		maxBytes = 1024 * 1024 // 1MB default
	}

	script := fmt.Sprintf("tail -c %d %s 2>/dev/null || echo ''", maxBytes, config.FilePath)

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Get File"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}
```

**Step 2: Commit**

```bash
git add internal/modules/server/tasks/get_file.go
git commit -m "feat(server): add GetFile task for fetching remote file content"
```

---

### Task 2: Create FetchTaskOutput Job

**Files:**
- Create: `internal/modules/server/jobs/fetch_task_output.go`
- Modify: `internal/modules/server/jobs/register.go`

**Step 1: Create the job that SSHes to fetch output**

```go
package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/launch/paths"
)

const TypeFetchTaskOutput = "server:fetch_task_output"

// FetchTaskOutputPayload holds data for fetching task output from server
type FetchTaskOutputPayload struct {
	TaskID                string `json:"task_id"`
	ServerID              string `json:"server_id"`
	RescheduleIntervalSec int    `json:"reschedule_interval_sec"` // 0 = no reschedule
}

// FetchTaskOutputJob SSHes into the server to download task output.
// If RescheduleIntervalSec > 0 and task is still pending, it reschedules itself.
type FetchTaskOutputJob struct {
	Deps    *JobDeps
	Payload FetchTaskOutputPayload

	task   *models.Task
	server *models.Server
}

func NewFetchTaskOutputJob(p FetchTaskOutputPayload) pkgjobs.Handler {
	return &FetchTaskOutputJob{Deps: deps, Payload: p}
}

// Handle executes the fetch task output job
func (j *FetchTaskOutputJob) Handle(ctx context.Context) error {
	var err error

	// Load task
	j.task, err = j.Deps.Repos.Task().FindByID(ctx, j.Payload.TaskID)
	if err != nil {
		return fmt.Errorf("failed to find task: %w", err)
	}

	// Load server
	j.server, err = j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.Deps.Logger.Info().
		Str("task_id", j.task.ID).
		Str("server_id", j.server.ID).
		Msg("Fetching task output from server")

	// Build output file path
	taskDir := paths.GetTaskDir(j.task.User)
	taskPaths := paths.GetTaskPaths(taskDir, "task-"+j.task.ID)

	// Create GetFile task to fetch the output
	getFileTask := tasks.GetFile(tasks.GetFileConfig{
		FilePath: taskPaths.Output,
		MaxBytes: 1024 * 1024, // 1MB
	})

	// Run as root to ensure we can read the file
	result, err := j.Deps.RunTask(j.server, getFileTask).
		AsRoot().
		Run(ctx)

	if err != nil {
		j.Deps.Logger.Error().Err(err).
			Str("task_id", j.task.ID).
			Msg("Failed to fetch task output")
		return err
	}

	if result.TaskResult != nil && result.TaskResult.IsSuccessful() {
		// Update task output in database
		output := result.TaskResult.Output
		if err := j.Deps.Repos.Task().UpdateOutput(ctx, j.task.ID, output); err != nil {
			j.Deps.Logger.Error().Err(err).
				Str("task_id", j.task.ID).
				Msg("Failed to update task output")
		} else {
			j.Deps.Logger.Info().
				Str("task_id", j.task.ID).
				Msg("Task output updated")
		}

		// Broadcast output update
		j.Deps.BroadcastServerEvent(j.server, "task.output", map[string]any{
			"task_id": j.task.ID,
			"output":  output,
			"status":  j.task.Status,
		})
	}

	// Check if task has timed out (status still pending but exceeded timeout)
	if j.task.Status == "pending" || j.task.Status == "running" {
		if j.task.CreatedAt != nil {
			timeoutDuration := time.Duration(j.task.Timeout) * time.Second
			if time.Since(*j.task.CreatedAt) > timeoutDuration {
				exitCode := 124
				j.Deps.Repos.Task().Update(ctx, j.task.ID, map[string]any{
					"status":    "timeout",
					"exit_code": exitCode,
				})
				j.Deps.Logger.Info().
					Str("task_id", j.task.ID).
					Msg("Task marked as timeout")
			}
		}
	}

	// Self-reschedule if interval > 0 and task is still running
	if j.Payload.RescheduleIntervalSec > 0 {
		// Reload task to check current status
		j.task, _ = j.Deps.Repos.Task().FindByID(ctx, j.Payload.TaskID)
		if j.task != nil && (j.task.Status == "pending" || j.task.Status == "running") {
			j.reschedule()
		}
	}

	return nil
}

// reschedule dispatches a new job with the same parameters after the interval
func (j *FetchTaskOutputJob) reschedule() {
	if j.Deps.Queue == nil {
		return
	}

	newTask, err := NewFetchTaskOutputTask(
		j.Payload.TaskID,
		j.Payload.ServerID,
		j.Payload.RescheduleIntervalSec,
	)
	if err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to create reschedule task")
		return
	}

	delay := time.Duration(j.Payload.RescheduleIntervalSec) * time.Second
	_, err = j.Deps.Queue.Enqueue(newTask, asynq.ProcessIn(delay))
	if err != nil {
		j.Deps.Logger.Error().Err(err).Msg("Failed to reschedule fetch task output job")
	} else {
		j.Deps.Logger.Info().
			Str("task_id", j.Payload.TaskID).
			Int("delay_seconds", j.Payload.RescheduleIntervalSec).
			Msg("Rescheduled fetch task output job")
	}
}

// Failed handles job failure
func (j *FetchTaskOutputJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("task_id", j.Payload.TaskID).
		Msg("Failed to fetch task output")
}

// NewFetchTaskOutputTask creates a fetch task output asynq task
func NewFetchTaskOutputTask(taskID, serverID string, rescheduleIntervalSec int) (*asynq.Task, error) {
	return pkgjobs.Task(TypeFetchTaskOutput, FetchTaskOutputPayload{
		TaskID:                taskID,
		ServerID:              serverID,
		RescheduleIntervalSec: rescheduleIntervalSec,
	})
}
```

**Step 2: Register the job in register.go**

Add to registerHandlers function:
```go
pkgjobs.RegisterTyped(mux, TypeFetchTaskOutput, NewFetchTaskOutputJob)
```

**Step 3: Commit**

```bash
git add internal/modules/server/jobs/fetch_task_output.go internal/modules/server/jobs/register.go
git commit -m "feat(server): add FetchTaskOutput job for SSH-based output polling"
```

---

### Task 3: Update Webhook Handler to Fetch Output

**Files:**
- Modify: `internal/modules/server/handlers/task_webhook_handler.go`

**Step 1: Add import for jobs package**

```go
import (
	// ... existing imports
	"github.com/kkz6/launch-go/internal/modules/server/jobs"
)
```

**Step 2: Add output fetching after callback processing**

Update each webhook endpoint (MarkAsFinished, MarkAsFailed, MarkAsTimeout) to dispatch output fetch after updating task status. Add before the final return statement:

```go
// MarkAsFinished - add before return:
h.dispatchOutputFetch(task)

// MarkAsFailed - add before return:
h.dispatchOutputFetch(task)

// MarkAsTimeout - add before return:
h.dispatchOutputFetch(task)
```

**Step 3: Add the helper method**

```go
// dispatchOutputFetch dispatches a job to fetch the task output from the server
func (h *TaskWebhookHandler) dispatchOutputFetch(task *models.Task) {
	if h.queue == nil {
		return
	}

	// Get server ID from task
	serverID := task.ServerID
	if serverID == "" {
		return
	}

	asynqTask, err := jobs.NewFetchTaskOutputTask(task.ID, serverID, 0) // 0 = no reschedule
	if err != nil {
		if h.Logger != nil {
			h.LogError(err, "Failed to create fetch output task", "task_id", task.ID)
		}
		return
	}

	if _, err := h.queue.Enqueue(asynqTask); err != nil {
		if h.Logger != nil {
			h.LogError(err, "Failed to dispatch fetch output job", "task_id", task.ID)
		}
	} else if h.Logger != nil {
		h.LogInfo("Dispatched output fetch job", "task_id", task.ID)
	}
}
```

**Step 4: Commit**

```bash
git add internal/modules/server/handlers/task_webhook_handler.go
git commit -m "feat(server): dispatch output fetch job after webhook callbacks"
```

---

### Task 4: Add Custom Callback Endpoint

**Files:**
- Modify: `internal/modules/server/handlers/task_webhook_handler.go`
- Modify: `internal/modules/server/routes.go` (or wherever webhook routes are defined)

**Step 1: Add CustomCallback endpoint to handler**

```go
// CustomCallback handles custom callbacks from running tasks (for progress updates)
// This can be called by scripts during execution to trigger output fetch or custom logic
func (h *TaskWebhookHandler) CustomCallback(c *fiber.Ctx) error {
	taskID := c.Params("id")
	ctx := c.Context()

	if !h.VerifySignature(c) {
		return response.Unauthorized(c, "Invalid signature")
	}

	task, err := h.repo.FindTaskByID(ctx, taskID)
	if err != nil {
		return response.NotFound(c, "Task not found")
	}

	// Only allow callbacks for pending/running tasks
	if task.Status != "pending" && task.Status != "running" {
		return response.OK(c, "Task already completed", nil)
	}

	// Fetch the latest output from the server
	h.dispatchOutputFetch(task)

	// Handle callback if task has instance data
	h.handleCallback(ctx, task, taskrunner.CallbackCustom, 0)

	return response.OK(c, "Callback processed", nil)
}
```

Note: `CallbackCustom` already exists in `internal/pkg/taskrunner/callback.go`.

**Step 2: Add route for custom callback**

Find the webhook routes file and add:
```go
app.Post("/webhooks/tasks/:id/callback", handler.CustomCallback)
```

**Step 3: Commit**

```bash
git add internal/modules/server/handlers/task_webhook_handler.go internal/modules/server/routes.go
git commit -m "feat(server): add custom callback endpoint for task progress updates"
```

---

### Task 5: Add Output Polling to TaskRunner

**Files:**
- Modify: `internal/modules/server/tasks/runner.go`

**Step 1: Add import for jobs package**

```go
import (
	// ... existing imports
	"github.com/kkz6/launch-go/internal/modules/server/jobs"
)
```

**Step 2: Add outputPollingIntervalSec field to TaskRunner struct**

```go
type TaskRunner struct {
	// ... existing fields
	outputPollingIntervalSec int // Polling interval for background tasks (0 = disabled)
}
```

**Step 3: Add WithOutputPolling builder method**

```go
// WithOutputPolling sets the interval for fetching output during background execution.
// The job will SSH into the server at this interval to fetch the task output file.
// Set to 0 (default) to disable polling.
func (r *TaskRunner) WithOutputPolling(intervalSeconds int) *TaskRunner {
	r.outputPollingIntervalSec = intervalSeconds
	return r
}
```

**Step 4: Update runWithCallbacks to dispatch polling job**

Add after updating status to running (around line 412-413):

```go
// runWithCallbacks - add after r.broadcastTaskRunning(taskModel):

// Start output polling if configured
if r.outputPollingIntervalSec > 0 && r.queue != nil {
	r.dispatchOutputPolling(taskModel)
}
```

**Step 5: Add dispatchOutputPolling helper method**

```go
// dispatchOutputPolling schedules the first output fetch job with self-rescheduling
func (r *TaskRunner) dispatchOutputPolling(taskModel *models.Task) {
	task, err := jobs.NewFetchTaskOutputTask(
		taskModel.ID,
		r.server.ID,
		r.outputPollingIntervalSec,
	)
	if err != nil {
		if r.logger != nil {
			r.logger.Error().Err(err).Msg("Failed to create output polling task")
		}
		return
	}

	// Delay first poll by 5 seconds to let the task start
	_, err = r.queue.Enqueue(task, asynq.ProcessIn(5*time.Second))
	if err != nil {
		if r.logger != nil {
			r.logger.Error().Err(err).Msg("Failed to dispatch output polling job")
		}
	} else if r.logger != nil {
		r.logger.Info().
			Str("task_id", taskModel.ID).
			Int("interval_sec", r.outputPollingIntervalSec).
			Msg("Started output polling")
	}
}
```

**Step 6: Commit**

```bash
git add internal/modules/server/tasks/runner.go
git commit -m "feat(server): add output polling option to TaskRunner"
```

---

### Task 6: Generate Custom Callback URL

**Files:**
- Modify: `internal/modules/server/handlers/task_webhook_handler.go`

**Step 1: Update CallbackURLs struct to include Custom field**

```go
// CallbackURLs contains the four webhook URLs for task completion
type CallbackURLs struct {
	Finished string
	Failed   string
	Timeout  string
	Custom   string // For progress updates from script
}
```

**Step 2: Update GenerateCallbackURLs to include custom callback**

```go
func (h *TaskWebhookHandler) GenerateCallbackURLs(baseURL, taskID string, expireMinutes int) CallbackURLs {
	expireDuration := time.Duration(expireMinutes) * time.Minute

	basePath := util.New("").Path("webhooks", "tasks", taskID)
	finishedPath := basePath.Clone().Path("finished").String()
	failedPath := basePath.Clone().Path("failed").String()
	timeoutPath := basePath.Clone().Path("timeout").String()
	customPath := basePath.Clone().Path("callback").String()

	signerWithBase := h.Signer.WithBaseURL(baseURL)

	return CallbackURLs{
		Finished: signerWithBase.SignedURL(finishedPath, nil, expireDuration),
		Failed:   signerWithBase.SignedURL(failedPath, nil, expireDuration),
		Timeout:  signerWithBase.SignedURL(timeoutPath, nil, expireDuration),
		Custom:   signerWithBase.SignedURL(customPath, nil, expireDuration),
	}
}
```

**Step 3: Update CallbackURLs in runner.go (if it has a separate struct)**

Check if `internal/modules/server/tasks/runner.go` has its own `CallbackURLs` struct and add the `Custom` field there too (or remove duplication).

**Step 4: Commit**

```bash
git add internal/modules/server/handlers/task_webhook_handler.go internal/modules/server/tasks/runner.go
git commit -m "feat(server): add custom callback URL generation"
```

---

## Part 2: Code Refactoring

### Task 7: Update Task Interface Naming

**Files:**
- Modify: `internal/pkg/taskrunner/task.go`

**Changes:**
- `OnFinished` → `OnSuccess`
- `OnFailed` → `OnFailure`
- `FinishedCallback` → `SuccessCallback`
- `FailedCallback` → `FailureCallback`
- Remove unused setters: `SetScript`, `SetName`, `SetTimeout`

**Step 1: Update interface and BaseTask**

```go
type Task interface {
	Name() string
	Script() string
	Timeout() time.Duration
	OnOutput(output string)
	OnSuccess(ctx context.Context, result *TaskResult)
	OnFailure(ctx context.Context, result *TaskResult)
	OnTimeout(ctx context.Context, result *TaskResult)
}

type BaseTask struct {
	name        string
	script      string
	timeout     time.Duration
	callbackURL string

	OutputCallback  func(output string)
	SuccessCallback func(ctx context.Context, result *TaskResult)
	FailureCallback func(ctx context.Context, result *TaskResult)
	TimeoutCallback func(ctx context.Context, result *TaskResult)
}

func (t *BaseTask) OnSuccess(ctx context.Context, result *TaskResult) {
	if t.SuccessCallback != nil {
		t.SuccessCallback(ctx, result)
	}
}

func (t *BaseTask) OnFailure(ctx context.Context, result *TaskResult) {
	if t.FailureCallback != nil {
		t.FailureCallback(ctx, result)
	}
}

func (t *BaseTask) OnTimeout(ctx context.Context, result *TaskResult) {
	if t.TimeoutCallback != nil {
		t.TimeoutCallback(ctx, result)
	}
}
```

**Step 2: Commit**

```bash
git add internal/pkg/taskrunner/task.go
git commit -m "refactor(taskrunner): rename Task callbacks for consistency"
```

---

### Task 8: Update Dispatcher Callback Invocations

**Files:**
- Modify: `internal/pkg/taskrunner/dispatcher.go`
- Modify: `internal/pkg/taskrunner/fake_dispatcher.go`

**Changes:**
- `pt.Task.OnFinished` → `pt.Task.OnSuccess`
- `pt.Task.OnFailed` → `pt.Task.OnFailure`

**Step 1: Update all callback invocations in dispatcher.go**

**Step 2: Commit**

```bash
git add internal/pkg/taskrunner/dispatcher.go internal/pkg/taskrunner/fake_dispatcher.go
git commit -m "refactor(taskrunner): update dispatcher to use new callback names"
```

---

### Task 9: Update CallbackHandler Interface

**Files:**
- Modify: `internal/pkg/taskrunner/callback.go`

**Changes:**
- `OnExpired` → `OnTimeout`

```go
type CallbackHandler interface {
	OnSuccess(ctx context.Context, cbCtx *CallbackContext, taskID string) error
	OnFailure(ctx context.Context, cbCtx *CallbackContext, taskID string, exitCode int) error
	OnTimeout(ctx context.Context, cbCtx *CallbackContext, taskID string) error
}
```

**Step 1: Update interface**

**Step 2: Commit**

```bash
git add internal/pkg/taskrunner/callback.go
git commit -m "refactor(taskrunner): rename OnExpired to OnTimeout in CallbackHandler"
```

---

### Task 10: Update TaskBuilder

**Files:**
- Modify: `internal/pkg/taskrunner/task_builder.go`

**Changes:**
- `ExpiredHandler` → `TimeoutHandler`
- `OnExpired` → `OnTimeout`

**Step 1: Update types and methods**

**Step 2: Commit**

```bash
git add internal/pkg/taskrunner/task_builder.go
git commit -m "refactor(taskrunner): rename ExpiredHandler to TimeoutHandler"
```

---

### Task 11: Update Module Tasks

**Files:**
- Modify: `internal/modules/server/tasks/provision_fresh_server.go`
- Modify: `internal/modules/site/tasks/deploy.go`

**Changes:**
- `OnExpired` → `OnTimeout` method names

**Step 1: Update all task implementations**

**Step 2: Commit**

```bash
git add internal/modules/server/tasks/ internal/modules/site/tasks/
git commit -m "refactor: update all module tasks to use OnTimeout naming"
```

---

### Task 12: Update Webhook Handler

**Files:**
- Modify: `internal/modules/server/handlers/task_webhook_handler.go`

**Changes:**
- `handler.OnExpired` → `handler.OnTimeout`

**Step 1: Update callback invocation**

**Step 2: Commit**

```bash
git add internal/modules/server/handlers/task_webhook_handler.go
git commit -m "refactor(server): update webhook handler to use OnTimeout"
```

---

### Task 13: Final Verification

**Step 1: Run all tests**

```bash
go test ./internal/pkg/taskrunner/...
go test ./internal/modules/server/...
go test ./internal/modules/site/...
```

**Step 2: Build to verify no compile errors**

```bash
go build ./...
```

**Step 3: Final commit**

```bash
git add .
git commit -m "refactor(taskrunner): complete refactoring with production mode features

Summary:
- Added FetchTaskOutput job for SSH-based output polling
- Added GetFile task for remote file fetching
- Added custom callback endpoint for progress updates
- Added output polling option to TaskRunner
- Unified callback naming: OnSuccess/OnFailure/OnTimeout everywhere
- Removed unused setters from BaseTask
- All modules updated to new naming conventions"
```

---

## Summary

### Part 1: Missing Features Added
| Feature | Description |
|---------|-------------|
| `GetFile` task | Fetches remote file content via SSH |
| `FetchTaskOutput` job | SSHes to server, downloads log, self-reschedules |
| Custom callback endpoint | `/webhooks/tasks/:id/callback` for progress |
| Output polling dispatch | `WithOutputPolling(seconds)` on TaskRunner |
| Webhook output fetching | Fetches output after status callbacks |

### Part 2: Refactoring
| Change | Before | After |
|--------|--------|-------|
| Task interface | `OnFinished/OnFailed` | `OnSuccess/OnFailure` |
| CallbackHandler | `OnExpired` | `OnTimeout` |
| BaseTask fields | `FinishedCallback` | `SuccessCallback` |
| Unused setters | Present | Removed |

### Execution Modes (Complete)

**Production Mode (HTTP Callbacks):**
```
1. Task starts → script wrapped with callback URLs
2. FetchTaskOutput dispatched with reschedule interval
3. Job polls server every N seconds
4. Script completes → calls webhook
5. Webhook updates status, fetches final output
6. Callbacks invoked
```

**Local Mode (Live SSH Streaming):**
```
1. Task starts → persistent SSH connection
2. StreamMonitor attaches to output
3. Output broadcast to WebSocket in real-time
4. Task completes → callbacks invoked directly
```
