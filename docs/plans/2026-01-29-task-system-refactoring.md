# Task System Refactoring Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Eliminate duplication and fix bugs across the task runner, recovery, and callback systems.

**Architecture:** Move shared constants to `server/types`, add helper methods to `CallbackContext` and `StreamResult`, consolidate duplicated broadcast/callback logic into package-level functions in `tasks`, and fix the recovery timeout bug.

**Tech Stack:** Go, GORM, asynq, zerolog

---

### Task 1: Move TaskStatus constants to `server/types`

**Files:**
- Modify: `internal/modules/server/types/types.go` (append new section)
- Modify: `internal/modules/server/tasks/runner.go:22-30` (remove constants, add import alias)
- Modify: `internal/modules/server/tasks/recovery.go` (use constants from types)
- Modify: `internal/modules/server/models/task.go` (use constants from types)

**Step 1: Add TaskStatus to `internal/modules/server/types/types.go`**

Append after the last section (RuleAction):

```go
// =============================================================================
// TaskStatus
// =============================================================================

// TaskStatus represents the status of a task execution
type TaskStatus string

const (
	TaskStatusPending  TaskStatus = "pending"
	TaskStatusRunning  TaskStatus = "running"
	TaskStatusFinished TaskStatus = "finished"
	TaskStatusFailed   TaskStatus = "failed"
	TaskStatusTimeout  TaskStatus = "timeout"
)

func (s TaskStatus) String() string {
	return string(s)
}

func (s TaskStatus) IsTerminal() bool {
	return s == TaskStatusFinished || s == TaskStatusFailed || s == TaskStatusTimeout
}
```

**Step 2: Update `runner.go` to use the shared type**

Remove lines 22-30 (the `TaskStatus` type and constants). Add `servertypes "github.com/kkz6/launch-go/internal/modules/server/types"` import. Replace all `TaskStatus*` references with `servertypes.TaskStatus*`. E.g.:
- `TaskStatusPending` -> `servertypes.TaskStatusPending`
- `TaskStatus` (the type) -> `servertypes.TaskStatus`

**Step 3: Update `models/task.go` to use the shared type**

Add import for `servertypes`. Replace hardcoded strings:
- `"pending"` in `SetDefaultStatus` -> `string(servertypes.TaskStatusPending)`
- `"pending"` in `IsPending` -> `string(servertypes.TaskStatusPending)`
- `"running"` in `IsRunning` -> `string(servertypes.TaskStatusRunning)`
- `"finished"` and `"failed"` in `IsFinished` -> `string(servertypes.TaskStatusFinished)` and `string(servertypes.TaskStatusFailed)`

**Step 4: Update `recovery.go` to use the shared type**

Replace `"running"` in the DB query with `string(servertypes.TaskStatusRunning)` and all `TaskStatus*` references with `servertypes.TaskStatus*`.

**Step 5: Build and verify**

Run: `go build ./...`

**Step 6: Commit**

```
feat: move TaskStatus constants to server/types package
```

---

### Task 2: Add `ToTaskResult()` to `StreamResult`

**Files:**
- Modify: `internal/pkg/taskrunner/stream_monitor.go:80-88` (add method)
- Modify: `internal/pkg/taskrunner/dispatcher.go:334-349` (use new method)
- Modify: `internal/modules/server/tasks/recovery.go:226-239` (use new method)

**Step 1: Add `ToTaskResult()` method to `StreamResult`**

After the `StreamResult` struct definition (line 88):

```go
// ToTaskResult converts a StreamResult to a TaskResult.
func (r *StreamResult) ToTaskResult() *TaskResult {
	result := &TaskResult{
		TaskID:     r.TaskID,
		Output:     r.Output,
		ExitCode:   r.ExitCode,
		FinishedAt: r.FinishedAt,
	}
	if r.Status == "timeout" {
		result.TimedOut = true
	}
	if r.Error != nil {
		result.Error = r.Error
	}
	return result
}
```

**Step 2: Update `dispatcher.go` `runRemoteBackground` onComplete callback**

Replace lines 335-348 (inside the `func(result *StreamResult)` callback):

```go
func(result *StreamResult) {
	taskResult := result.ToTaskResult()
	taskResult.Duration = time.Since(startTime)

	if ch := pt.GetCompletionChannel(); ch != nil {
		ch <- taskResult
	}

	switch result.Status {
	case "finished":
		pt.Task.OnFinished(streamCtx, taskResult)
	case "timeout":
		pt.Task.OnTimeout(streamCtx, taskResult)
	default:
		pt.Task.OnFailed(streamCtx, taskResult)
	}
},
```

**Step 3: Update `recovery.go` `remonitorRunningTask` onComplete callback**

Replace lines 226-239:

```go
func(result *taskrunner.StreamResult) {
	completionChan <- result.ToTaskResult()
},
```

**Step 4: Build and verify**

Run: `go build ./...`

**Step 5: Commit**

```
refactor: add ToTaskResult conversion method to StreamResult
```

---

### Task 3: Add `GetTaskOutputTail` to `CallbackContext`

**Files:**
- Modify: `internal/pkg/taskrunner/callback.go` (add method)
- Modify: `internal/modules/server/tasks/provision_fresh_server.go:314-336` (remove method, use cbCtx)
- Modify: `internal/modules/site/tasks/deploy.go:298-320` (remove method, use cbCtx)

**Step 1: Add helper to `CallbackContext` in `callback.go`**

Add the import `"strings"` to the imports, then add after the `DispatchJob` method:

```go
// GetTaskOutputTail retrieves the last N lines of task output from the database.
// This queries a generic "tasks" table with an "output" column.
// Returns empty string if DB is nil, task not found, or output is empty.
func (c *CallbackContext) GetTaskOutputTail(taskID string, lines int) string {
	if c.DB == nil {
		return ""
	}

	var output string
	if err := c.DB.Table("tasks").Select("output").Where("id = ?", taskID).Scan(&output).Error; err != nil || output == "" {
		return ""
	}

	parts := strings.Split(output, "\n")
	if len(parts) > lines {
		parts = parts[len(parts)-lines:]
	}
	return strings.Join(parts, "\n")
}
```

**Step 2: Update `provision_fresh_server.go`**

Remove the `getTaskOutput` method (lines 314-336). Replace all call sites:
- `t.getTaskOutput(cbCtx, taskID)` -> `cbCtx.GetTaskOutputTail(taskID, 30)`

**Step 3: Update `site/tasks/deploy.go`**

Remove the `getTaskOutput` method (lines 298-320). Replace all call sites:
- `t.getTaskOutput(cbCtx, taskID)` -> `cbCtx.GetTaskOutputTail(taskID, 30)`

**Step 4: Build and verify**

Run: `go build ./...`

**Step 5: Commit**

```
refactor: extract getTaskOutput into CallbackContext.GetTaskOutputTail
```

---

### Task 4: Extract shared broadcast payload builder for tasks

**Files:**
- Modify: `internal/modules/server/models/task.go` (add method)
- Modify: `internal/modules/server/tasks/runner.go:665-690` (use model method)
- Modify: `internal/modules/server/tasks/recovery.go:412-435` (use model method, remove method)

**Step 1: Add `BroadcastData` method to `models.Task`**

In `models/task.go`, add:

```go
// BroadcastData returns the standard broadcast payload for task events.
func (t *Task) BroadcastData(output string) map[string]interface{} {
	data := map[string]interface{}{
		"task_id":   t.ID,
		"server_id": t.ServerID,
		"name":      t.Name,
		"status":    t.Status,
		"user":      t.User,
	}
	if t.ExitCode != nil {
		data["exit_code"] = *t.ExitCode
	}
	if output != "" {
		data["output"] = output
	}
	return data
}
```

**Step 2: Update `runner.go` `broadcastTaskEvent`**

Replace the body of `broadcastTaskEvent` (lines 666-690):

```go
func (r *TaskRunner) broadcastTaskEvent(event string, taskModel *models.Task, output string) {
	if r.broadcaster == nil || r.server == nil {
		return
	}
	r.broadcaster.BroadcastToTeam(r.server.TeamID, event, taskModel.BroadcastData(output))
}
```

**Step 3: Update `recovery.go` `broadcastTaskUpdate`**

Replace the body (lines 412-435):

```go
func (r *TaskRecoverer) broadcastTaskUpdate(task *models.Task, output string) {
	if r.broadcaster == nil || task.Server == nil {
		return
	}
	r.broadcaster.BroadcastToTeam(task.Server.TeamID, "task.updated", task.BroadcastData(output))
}
```

**Step 4: Build and verify**

Run: `go build ./...`

**Step 5: Commit**

```
refactor: extract task broadcast payload into models.Task.BroadcastData
```

---

### Task 5: Fix recovery bug — add timeout detection and notifier support

This task fixes the bug where recovery never calls `OnExpired` for timed-out tasks, and adds the missing `notifier` dependency.

**Files:**
- Modify: `internal/modules/server/tasks/recovery.go` (multiple changes)
- Modify: `cmd/worker/main.go` (pass notifier if available)

**Step 1: Add `notifier` field to `TaskRecoverer` and update constructor**

Update the struct and constructor:

```go
type TaskRecoverer struct {
	db          *gorm.DB
	logger      *zerolog.Logger
	dispatcher  *taskrunner.Dispatcher
	queue       *queue.Client
	broadcaster broadcast.TeamBroadcaster
	notifier    taskrunner.NotifierService
}

func NewTaskRecoverer(
	db *gorm.DB,
	logger *zerolog.Logger,
	dispatcher *taskrunner.Dispatcher,
	queue *queue.Client,
	broadcaster broadcast.TeamBroadcaster,
	notifier taskrunner.NotifierService,
) *TaskRecoverer {
	return &TaskRecoverer{
		db:          db,
		logger:      logger,
		dispatcher:  dispatcher,
		queue:       queue,
		broadcaster: broadcaster,
		notifier:    notifier,
	}
}
```

**Step 2: Fix `invokeCallbacks` to handle timeout (exit code 124) and pass notifier**

The `timeout` command returns exit code 124 when a command times out. Update `invokeCallbacks`:

```go
func (r *TaskRecoverer) invokeCallbacks(ctx context.Context, task *models.Task, exitCode int) error {
	instanceData := task.Instance.String()
	if instanceData == "" {
		return nil
	}

	handler, err := taskrunner.ReconstructFromInstance(instanceData)
	if err != nil {
		r.dispatchCompletionJobs(task, exitCode, instanceData)
		return nil
	}

	if handler == nil {
		return nil
	}

	cbCtx := &taskrunner.CallbackContext{
		DB:       r.db,
		Queue:    r.queue,
		Logger:   r.logger,
		Notifier: r.notifier,
	}
	cbCtx.SetBroadcaster(r.broadcaster)

	timedOut := exitCode == 124 || task.Status == string(servertypes.TaskStatusTimeout)

	if exitCode == 0 {
		if err := handler.OnSuccess(ctx, cbCtx, task.ID); err != nil {
			r.logger.Error().Err(err).Str("task_id", task.ID).Msg("Task recovery: OnSuccess callback failed")
			return err
		}
	} else if timedOut {
		if err := handler.OnExpired(ctx, cbCtx, task.ID); err != nil {
			r.logger.Error().Err(err).Str("task_id", task.ID).Msg("Task recovery: OnExpired callback failed")
			return err
		}
	} else {
		if err := handler.OnFailure(ctx, cbCtx, task.ID, exitCode); err != nil {
			r.logger.Error().Err(err).Str("task_id", task.ID).Msg("Task recovery: OnFailure callback failed")
			return err
		}
	}

	return nil
}
```

**Step 3: Fix `finalizeCompletedTask` to detect timeout status**

Update the status assignment in `finalizeCompletedTask`:

```go
func (r *TaskRecoverer) finalizeCompletedTask(ctx context.Context, task *models.Task, exitCode int, output string) error {
	task.ExitCode = &exitCode
	task.Output = dbtype.EncryptedString(output)

	if exitCode == 0 {
		task.Status = string(servertypes.TaskStatusFinished)
	} else if exitCode == 124 {
		task.Status = string(servertypes.TaskStatusTimeout)
	} else {
		task.Status = string(servertypes.TaskStatusFailed)
	}

	if err := r.db.Save(task).Error; err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}

	r.broadcastTaskUpdate(task, output)
	return r.invokeCallbacks(ctx, task, exitCode)
}
```

**Step 4: Fix `dispatchCompletionJobs` to handle timeout**

```go
func (r *TaskRecoverer) dispatchCompletionJobs(task *models.Task, exitCode int, instanceData string) {
	config, err := taskrunner.UnmarshalCompletionConfig(instanceData)
	if err != nil || config == nil {
		return
	}

	if r.queue == nil {
		return
	}

	var jobRef *taskrunner.JobRef
	if exitCode == 0 {
		jobRef = config.OnFinished
	} else if exitCode == 124 {
		jobRef = config.OnTimeout
	} else {
		jobRef = config.OnFailed
	}

	if jobRef == nil || jobRef.Type == "" {
		return
	}

	r.logger.Info().
		Str("task_id", task.ID).
		Str("job_type", jobRef.Type).
		Msg("Task recovery: dispatching completion job")

	asynqTask := asynq.NewTask(jobRef.Type, jobRef.Payload)
	if _, err := r.queue.Enqueue(asynqTask); err != nil {
		r.logger.Error().Err(err).
			Str("task_id", task.ID).
			Str("job_type", jobRef.Type).
			Msg("Task recovery: failed to dispatch completion job")
	}
}
```

**Step 5: Update `cmd/worker/main.go` to pass nil notifier**

Since the notifier isn't wired into the worker yet, pass `nil`:

```go
recoverer := servertasks.NewTaskRecoverer(db, appLogger, dispatcher, queueClient, redisBroadcaster, nil)
```

**Step 6: Build and verify**

Run: `go build ./...`

**Step 7: Commit**

```
fix: add timeout detection and notifier support to task recovery
```

---

### Task 6: Add connection validation to recovery's `getConnection`

**Files:**
- Modify: `internal/modules/server/tasks/recovery.go` (update `getConnection`)

**Step 1: Add validation**

Replace `getConnection`:

```go
func (r *TaskRecoverer) getConnection(task *models.Task) (*taskrunner.Connection, error) {
	if task.Server.PublicIPv4 == nil || *task.Server.PublicIPv4 == "" {
		return nil, fmt.Errorf("server %s has no public IP address", task.ServerID)
	}
	if task.Server.PrivateKey.IsEmpty() {
		return nil, fmt.Errorf("server %s has no private key", task.ServerID)
	}

	if task.User == task.Server.RootUsername() {
		return task.Server.ConnectionAsRoot(), nil
	}
	return task.Server.ConnectionAsUser(task.User), nil
}
```

**Step 2: Update `recoverTask` to handle the error**

Change `conn := r.getConnection(task)` to:

```go
conn, err := r.getConnection(task)
if err != nil {
	return err
}
```

**Step 3: Build and verify**

Run: `go build ./...`

**Step 4: Commit**

```
fix: add server validation to recovery SSH connection
```

---

### Task 7: Final build and test

**Step 1: Full build**

Run: `go build ./...`

**Step 2: Run tests**

Run: `go test ./internal/pkg/taskrunner/... ./internal/modules/server/... ./internal/modules/site/...`

**Step 3: Verify git status is clean**

Run: `git diff --stat`
