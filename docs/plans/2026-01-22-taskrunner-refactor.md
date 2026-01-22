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

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/launch/paths"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
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
	pkgjobs.BaseJob[*JobContext, FetchTaskOutputPayload]
}

// NewFetchTaskOutputJob creates a new FetchTaskOutputJob
func NewFetchTaskOutputJob(ctx *JobContext, payload FetchTaskOutputPayload) *FetchTaskOutputJob {
	return &FetchTaskOutputJob{
		BaseJob: pkgjobs.NewBaseJob(ctx, payload),
	}
}

// Handle executes the fetch task output job
func (j *FetchTaskOutputJob) Handle(ctx context.Context) error {
	// Load task
	task, err := j.Ctx.Repos().Task().FindByID(ctx, j.Payload.TaskID)
	if err != nil {
		return fmt.Errorf("failed to find task: %w", err)
	}

	// Load server
	server, err := j.Ctx.Repos().Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.Ctx.LogInfo("Fetching task output from server",
		"task_id", task.ID,
		"server_id", server.ID,
	)

	// Build output file path
	taskDir := paths.GetTaskDir(task.User)
	taskPaths := paths.GetTaskPaths(taskDir, "task-"+task.ID)

	// Create GetFile task to fetch the output
	getFileTask := tasks.GetFile(tasks.GetFileConfig{
		FilePath: taskPaths.Output,
		MaxBytes: 1024 * 1024, // 1MB
	})

	// Run as root to ensure we can read the file
	result, err := j.Ctx.TaskRunnerDeps.NewRunner(server, getFileTask).
		AsRoot().
		Run(ctx)

	if err != nil {
		j.Ctx.LogError(err, "Failed to fetch task output", "task_id", task.ID)
		return err
	}

	if result.TaskResult != nil && result.TaskResult.IsSuccessful() {
		// Update task output in database
		output := result.TaskResult.Output
		if err := j.Ctx.Repos().Task().UpdateOutput(ctx, task.ID, output); err != nil {
			j.Ctx.LogError(err, "Failed to update task output", "task_id", task.ID)
		} else {
			j.Ctx.LogInfo("Task output updated", "task_id", task.ID)
		}

		// Broadcast output update
		j.Ctx.BroadcastServerEvent(server, "task.output", map[string]interface{}{
			"task_id": task.ID,
			"output":  output,
			"status":  task.Status,
		})
	}

	// Check if task has timed out (status still pending but exceeded timeout)
	if task.Status == "pending" || task.Status == "running" {
		if task.CreatedAt != nil {
			timeoutDuration := time.Duration(task.Timeout) * time.Second
			if time.Since(*task.CreatedAt) > timeoutDuration {
				exitCode := 124
				j.Ctx.Repos().Task().Update(ctx, task.ID, map[string]interface{}{
					"status":    "timeout",
					"exit_code": exitCode,
				})
				j.Ctx.LogInfo("Task marked as timeout", "task_id", task.ID)
			}
		}
	}

	// Self-reschedule if interval > 0 and task is still running
	if j.Payload.RescheduleIntervalSec > 0 {
		// Reload task to check current status
		task, _ = j.Ctx.Repos().Task().FindByID(ctx, j.Payload.TaskID)
		if task != nil && (task.Status == "pending" || task.Status == "running") {
			j.reschedule()
		}
	}

	return nil
}

// reschedule dispatches a new job with the same parameters after the interval
func (j *FetchTaskOutputJob) reschedule() {
	if j.Ctx.Queue() == nil {
		return
	}

	newTask, err := NewFetchTaskOutputTask(
		j.Payload.TaskID,
		j.Payload.ServerID,
		j.Payload.RescheduleIntervalSec,
	)
	if err != nil {
		j.Ctx.LogError(err, "Failed to create reschedule task")
		return
	}

	delay := time.Duration(j.Payload.RescheduleIntervalSec) * time.Second
	_, err = j.Ctx.Queue().Enqueue(newTask, asynq.ProcessIn(delay))
	if err != nil {
		j.Ctx.LogError(err, "Failed to reschedule fetch task output job")
	} else {
		j.Ctx.LogInfo("Rescheduled fetch task output job",
			"task_id", j.Payload.TaskID,
			"delay_seconds", j.Payload.RescheduleIntervalSec,
		)
	}
}

// Failed handles job failure
func (j *FetchTaskOutputJob) Failed(ctx context.Context, err error) {
	j.Ctx.LogError(err, "Failed to fetch task output",
		"task_id", j.Payload.TaskID,
	)
}

// RetryConfig returns retry configuration with exponential backoff
func (j *FetchTaskOutputJob) RetryConfig() pkgjobs.RetryConfig {
	return pkgjobs.RetryConfig{
		MaxRetries: 5,
		Backoff:    []time.Duration{5 * time.Second, 10 * time.Second, 30 * time.Second, 60 * time.Second, 120 * time.Second},
	}
}

// NewFetchTaskOutputTask creates a fetch task output asynq task
func NewFetchTaskOutputTask(taskID, serverID string, rescheduleIntervalSec int) (*asynq.Task, error) {
	return pkgjobs.NewTask(TypeFetchTaskOutput, FetchTaskOutputPayload{
		TaskID:                taskID,
		ServerID:              serverID,
		RescheduleIntervalSec: rescheduleIntervalSec,
	})
}
```

**Step 2: Register the job in register.go**

Add to the job registration:
```go
TypeFetchTaskOutput: pkgjobs.NewHandler(func(ctx *JobContext, p FetchTaskOutputPayload) pkgjobs.JobHandler {
    return NewFetchTaskOutputJob(ctx, p)
}),
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

**Step 1: Add output fetching after callback processing**

After each webhook endpoint updates the task status, dispatch FetchTaskOutput:

```go
// Add to MarkAsFinished, MarkAsFailed, MarkAsTimeout:

// Dispatch job to fetch final output from server
h.dispatchOutputFetch(task)
```

Add the helper method:

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
	}
}
```

**Step 2: Commit**

```bash
git add internal/modules/server/handlers/task_webhook_handler.go
git commit -m "feat(server): dispatch output fetch job after webhook callbacks"
```

---

### Task 4: Add Custom Callback Endpoint

**Files:**
- Modify: `internal/modules/server/handlers/task_webhook_handler.go`

**Step 1: Add CustomCallback endpoint**

```go
// CustomCallback handles custom callbacks from running tasks (for progress updates)
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

	// Handle callback if task has instance data
	h.handleCallback(ctx, task, taskrunner.CallbackCustom, 0)

	return response.OK(c, "Callback processed", nil)
}
```

**Step 2: Add CallbackCustom constant to callback.go**

```go
const (
	CallbackFinished CallbackType = "finished"
	CallbackFailed   CallbackType = "failed"
	CallbackTimeout  CallbackType = "timeout"
	CallbackCustom   CallbackType = "custom"
)
```

**Step 3: Update routes to include custom callback**

```go
// Add to webhook routes:
app.Post("/webhooks/tasks/:id/callback", handler.CustomCallback)
```

**Step 4: Commit**

```bash
git add internal/modules/server/handlers/task_webhook_handler.go internal/pkg/taskrunner/callback.go
git commit -m "feat(server): add custom callback endpoint for task progress updates"
```

---

### Task 5: Add Output Polling to TaskRunner

**Files:**
- Modify: `internal/modules/server/tasks/runner.go`

**Step 1: Add updateLogIntervalInSeconds option**

```go
type TaskRunner struct {
	// ... existing fields
	updateLogIntervalSec int // Polling interval for background tasks
}

// WithOutputPolling sets the interval for fetching output during background execution
func (r *TaskRunner) WithOutputPolling(intervalSeconds int) *TaskRunner {
	r.updateLogIntervalSec = intervalSeconds
	return r
}
```

**Step 2: Dispatch polling job in runWithCallbacks**

After starting the background task, dispatch the first FetchTaskOutput job:

```go
func (r *TaskRunner) runWithCallbacks(ctx context.Context) (*models.Task, error) {
	// ... existing code to start task ...

	// Start output polling if configured
	if r.updateLogIntervalSec > 0 && r.queue != nil {
		r.dispatchOutputPolling(taskModel)
	}

	return taskModel, nil
}

func (r *TaskRunner) dispatchOutputPolling(taskModel *models.Task) {
	task, err := jobs.NewFetchTaskOutputTask(
		taskModel.ID,
		r.server.ID,
		r.updateLogIntervalSec,
	)
	if err != nil {
		if r.logger != nil {
			r.logger.Error().Err(err).Msg("Failed to create output polling task")
		}
		return
	}

	// Delay first poll by 5 seconds
	_, err = r.queue.Enqueue(task, asynq.ProcessIn(5*time.Second))
	if err != nil && r.logger != nil {
		r.logger.Error().Err(err).Msg("Failed to dispatch output polling job")
	}
}
```

**Step 3: Commit**

```bash
git add internal/modules/server/tasks/runner.go
git commit -m "feat(server): add output polling option to TaskRunner"
```

---

### Task 6: Generate Custom Callback URL

**Files:**
- Modify: `internal/modules/server/handlers/task_webhook_handler.go`

**Step 1: Update GenerateCallbackURLs to include custom callback**

```go
type CallbackURLs struct {
	Finished string
	Failed   string
	Timeout  string
	Custom   string // For progress updates from script
}

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

**Step 2: Commit**

```bash
git add internal/modules/server/handlers/task_webhook_handler.go
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
