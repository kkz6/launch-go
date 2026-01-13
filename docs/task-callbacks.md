# Task Callback System

This document explains how the task callback system works in Launch Go, enabling continuation logic after tasks complete on remote servers.

## Overview

When running long-running tasks on servers (like installing PHP, provisioning, or deployments), we need a way to:

1. Track task status in the database
2. Execute continuation logic when the task completes
3. Handle success, failure, and timeout scenarios differently

The callback system is designed to be **simple like Laravel** - you just define what should happen on completion, and the system handles the rest.

## Quick Start

### Option 1: Task with Built-in Completion (Recommended)

Implement `TaskWithCompletion` interface on your task:

```go
type InstallPhpTask struct {
    taskrunner.BaseTask
    ServerID   string
    PhpVersion string
}

// Define what job to dispatch on completion
func (t *InstallPhpTask) CompletionJobType() string {
    return "server:php-installed"  // Your asynq job type
}

func (t *InstallPhpTask) CompletionPayload() interface{} {
    return map[string]string{
        "server_id":   t.ServerID,
        "php_version": t.PhpVersion,
    }
}

// Usage - just run it!
task := NewInstallPhpTask(serverID, "8.3")
runner.NewRunner(server, task).
    AsRoot().
    TrackInBackground().
    Dispatch(ctx)
```

### Option 2: Ad-hoc Completion with Chaining

For any task, chain `OnComplete`:

```go
runner.NewRunner(server, anyTask).
    AsRoot().
    TrackInBackground().
    OnComplete("my-job:done", payload).
    OnFailed("my-job:failed", payload).
    Dispatch(ctx)
```

### Option 3: No Completion Logic

Just track the task, no continuation:

```go
runner.NewRunner(server, task).
    AsRoot().
    TrackInBackground().
    Dispatch(ctx)
```

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                         TASK CREATION                                │
├─────────────────────────────────────────────────────────────────────┤
│  1. Task optionally implements TaskWithCompletion                    │
│  2. Or use .OnComplete() chain to specify completion job             │
│  3. CompletionConfig stored in task.Instance field                   │
│  4. Script wrapped with callback URLs and executed on server         │
└─────────────────────────────────────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────────┐
│                       SERVER EXECUTION                               │
├─────────────────────────────────────────────────────────────────────┤
│  1. Wrapper script executes actual task script                       │
│  2. Captures exit code                                               │
│  3. POSTs to callback URL based on result:                           │
│     - Exit 0   → /webhooks/tasks/:id/finished                        │
│     - Exit 124 → /webhooks/tasks/:id/timeout                         │
│     - Other    → /webhooks/tasks/:id/failed                          │
└─────────────────────────────────────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      CALLBACK HANDLING                               │
├─────────────────────────────────────────────────────────────────────┤
│  1. Webhook handler receives POST request                            │
│  2. Updates task status in database                                  │
│  3. Parses CompletionConfig from task.Instance                       │
│  4. Dispatches the appropriate asynq job:                            │
│     - OnFinished job for success                                     │
│     - OnFailed job for failure                                       │
│     - OnTimeout job for timeout                                      │
└─────────────────────────────────────────────────────────────────────┘
```

## Core Interfaces

### TaskWithCompletion (Simple Approach)

Implement this on your task to define what job should run on completion:

```go
type TaskWithCompletion interface {
    Task

    // CompletionJobType returns the asynq job type to dispatch
    CompletionJobType() string

    // CompletionPayload returns the payload for the completion job
    CompletionPayload() interface{}
}
```

### CompletionConfig

Stored in the task's Instance field:

```go
type CompletionConfig struct {
    OnFinished *JobRef `json:"on_finished,omitempty"`
    OnFailed   *JobRef `json:"on_failed,omitempty"`
    OnTimeout  *JobRef `json:"on_timeout,omitempty"`
}

type JobRef struct {
    Type    string          `json:"type"`    // Asynq job type
    Payload json.RawMessage `json:"payload"` // Job payload
}
```

## Implementation Guide

### Method 1: TaskWithCompletion Interface (Recommended)

The simplest approach - your task defines what job to dispatch:

```go
type InstallPhpTask struct {
    taskrunner.BaseTask
    ServerID   string
    PhpVersion string
}

func (t *InstallPhpTask) Name() string {
    return fmt.Sprintf("Install PHP %s", t.PhpVersion)
}

func (t *InstallPhpTask) Script() string {
    return `#!/bin/bash
set -euo pipefail
apt-get update
apt-get install -y php` + t.PhpVersion + `
echo "Done"`
}

func (t *InstallPhpTask) Timeout() time.Duration {
    return 10 * time.Minute
}

// These two methods make it a TaskWithCompletion:

func (t *InstallPhpTask) CompletionJobType() string {
    return "server:php-installed"
}

func (t *InstallPhpTask) CompletionPayload() interface{} {
    return map[string]string{
        "server_id":   t.ServerID,
        "php_version": t.PhpVersion,
    }
}
```

Usage:

```go
task := NewInstallPhpTask(serverID, "8.3")

// Completion job is auto-detected!
runner.NewRunner(server, task).
    AsRoot().
    TrackInBackground().
    Dispatch(ctx)
```

### Method 2: OnComplete Chain (Ad-hoc)

For any task, specify completion jobs with chaining:

```go
task := tasks.SomeTask(...)

runner.NewRunner(server, task).
    AsRoot().
    TrackInBackground().
    OnComplete("server:task-done", map[string]string{
        "server_id": serverID,
    }).
    OnFailed("server:task-failed", map[string]string{
        "server_id": serverID,
        "reason": "installation",
    }).
    OnTimeout("server:task-timeout", map[string]string{
        "server_id": serverID,
    }).
    Dispatch(ctx)
```

### Method 3: Create the Completion Job

The completion job handles the actual continuation logic:

```go
// internal/modules/server/jobs/php_installed.go

const TypePhpInstalled = "server:php-installed"

type PhpInstalledPayload struct {
    ServerID   string `json:"server_id"`
    PhpVersion string `json:"php_version"`
}

type PhpInstalledJob struct {
    jobs.BaseJob
    Payload PhpInstalledPayload
}

func (j *PhpInstalledJob) Handle(ctx context.Context) error {
    server, err := j.Repo().FindServerByID(ctx, j.Payload.ServerID)
    if err != nil {
        return err
    }

    // Update server's installed PHP versions
    server.AddInstalledPhp(j.Payload.PhpVersion)
    if err := j.Repo().UpdateServer(ctx, server); err != nil {
        return err
    }

    // Broadcast WebSocket event
    j.BroadcastToTeam(server.TeamID, "php.installed", map[string]interface{}{
        "server_id": server.ID,
        "version":   j.Payload.PhpVersion,
    })

    return nil
}
```

Register the job:

```go
// internal/modules/server/jobs/register.go

func Register(mux *asynq.ServeMux) {
    // ... other jobs ...
    mux.HandleFunc(TypePhpInstalled, handlePhpInstalled)
}

func handlePhpInstalled(ctx context.Context, t *asynq.Task) error {
    var payload PhpInstalledPayload
    json.Unmarshal(t.Payload(), &payload)

    job := &PhpInstalledJob{Payload: payload}
    job.SetContext(getJobContext())
    return job.Handle(ctx)
}
```

## Database Schema

The `tasks` table stores completion config:

```sql
CREATE TABLE tasks (
    id CHAR(26) PRIMARY KEY,
    server_id CHAR(26) NOT NULL,
    name VARCHAR(255) NOT NULL,
    user VARCHAR(255) NOT NULL,
    type VARCHAR(255) NOT NULL,
    instance LONGTEXT,                     -- CompletionConfig JSON
    script LONGTEXT NOT NULL,
    timeout INT NOT NULL,
    status VARCHAR(255) NOT NULL,          -- pending, running, finished, failed, timeout
    output LONGTEXT,
    exit_code INT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);
```

## Instance Field Format

The `instance` field stores JSON with this structure:

```json
{
    "on_finished": {
        "type": "server:php-installed",
        "payload": {"server_id": "01HXYZ...", "php_version": "8.3"}
    },
    "on_failed": {
        "type": "server:php-install-failed",
        "payload": {"server_id": "01HXYZ..."}
    }
}
```

## Webhook Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/webhooks/tasks/:id/finished` | POST | Called when task exits with code 0 |
| `/webhooks/tasks/:id/failed` | POST | Called when task exits with non-zero code |
| `/webhooks/tasks/:id/timeout` | POST | Called when task times out (exit 124) |

All endpoints require a valid signature query parameter for security.

## Best Practices

### 1. Keep Payloads Minimal

Only store IDs and essential data:

```go
// Good
func (t *MyTask) CompletionPayload() interface{} {
    return map[string]string{
        "server_id": t.ServerID,
        "version":   t.Version,
    }
}

// Bad - too much data
func (t *MyTask) CompletionPayload() interface{} {
    return t.Server // Don't serialize whole models
}
```

### 2. Make Jobs Idempotent

Completion jobs may run multiple times:

```go
func (j *PhpInstalledJob) Handle(ctx context.Context) error {
    server, _ := j.Repo().FindServerByID(ctx, j.Payload.ServerID)

    // Check if already done
    if server.HasPhpVersion(j.Payload.PhpVersion) {
        return nil // Already installed, skip
    }

    // Do the work...
}
```

### 3. Use Existing Job Infrastructure

The completion jobs use the same asynq system as other jobs:

```go
// Register in your jobs package
func Register(mux *asynq.ServeMux) {
    mux.HandleFunc("server:php-installed", handlePhpInstalled)
    mux.HandleFunc("server:php-install-failed", handlePhpInstallFailed)
}
```

## Comparison with Laravel

| Aspect | Laravel | Go |
|--------|---------|-----|
| Define completion | `implements HasCallbacks` | `implements TaskWithCompletion` |
| Or chain | `.keepTrackInBackground()` | `.OnComplete(jobType, payload)` |
| Serialization | `serialize($task)` | JSON CompletionConfig |
| Continuation | Task method called | Asynq job dispatched |
| Registration | Automatic | Jobs already registered |

## Flow Summary

```
1. Task with TaskWithCompletion
   └─> Auto-extracts CompletionConfig
       └─> Stores in task.Instance

2. Or chain .OnComplete()
   └─> Creates CompletionConfig
       └─> Stores in task.Instance

3. Server runs script
   └─> POSTs to callback URL

4. Webhook handler
   └─> Updates task status
       └─> Parses CompletionConfig
           └─> Dispatches asynq job

5. Job handles continuation logic
```

## Files Reference

| File | Purpose |
|------|---------|
| `internal/pkg/taskrunner/completion.go` | TaskWithCompletion interface, CompletionConfig |
| `internal/modules/server/tasks/runner.go` | Task execution with OnComplete chaining |
| `internal/modules/server/handlers/task_webhook_handler.go` | Webhook handling and job dispatch |
