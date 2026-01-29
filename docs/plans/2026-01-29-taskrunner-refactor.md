# Taskrunner Refactor: nohup + SSH Monitor with Real-Time Markers

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make RunInBackground use the resilient nohup + SSH monitor path with real-time marker processing during script execution.

**Architecture:** Upload script via SCP, start with nohup (survives SSH disconnects), open a separate SSH connection to monitor via tail -f, process markers in real-time as lines arrive, invoke callbacks immediately on marker detection.

**Tech Stack:** Go, golang.org/x/crypto/ssh, taskrunner package

---

## Summary of Changes

1. Fix `runner.go` to use nohup + monitor path (dispatcher.Run with InBackground) instead of RunWithStreaming
2. Wire marker handler from runner.go into the StreamMonitor so MonitorBackgroundTask processes markers in real-time
3. Fix path inconsistency in `paths.go` (`.launch-tasks` -> `.launch`)
4. Fix ubuntu home directory bug in `server.go`
5. Remove `RunWithStreaming` from dispatcher (dead code after refactor)
6. Remove `OnLine` from PendingTask (only used by RunWithStreaming)
7. Update tests

---

### Task 1: Fix paths.TaskDir to use `.launch` instead of `.launch-tasks`

**Files:**
- Modify: `internal/pkg/launch/paths/paths.go:72-73`
- Modify: `internal/pkg/launch/paths/paths_test.go`
- Modify: `internal/pkg/taskrunner/connection.go:16` (comment)

### Task 2: Fix ubuntu home directory bug

**Files:**
- Modify: `internal/modules/server/models/server.go:212`

### Task 3: Refactor runner.go RunInBackground to use nohup + monitor path

**Files:**
- Modify: `internal/modules/server/tasks/runner.go` (runLongRunning method)

Key changes:
- Use `dispatcher.Run()` with `InBackground()` + `WithCompletionChannel()` instead of `RunWithStreaming()`
- Set marker handler on stream monitor before dispatching
- Wait on completion channel for the result
- Handle completion callbacks

### Task 4: Remove RunWithStreaming and OnLine from dispatcher and PendingTask

**Files:**
- Modify: `internal/pkg/taskrunner/dispatcher.go` (remove RunWithStreaming method)
- Modify: `internal/pkg/taskrunner/pending_task.go` (remove OnLine)
- Modify: `internal/pkg/taskrunner/fake_dispatcher.go` (remove RunWithStreaming)
- Modify: `internal/pkg/taskrunner/dispatcher.go` (remove from TaskDispatcher interface)

### Task 5: Update tests

**Files:**
- Modify: `internal/pkg/launch/paths/paths_test.go`
