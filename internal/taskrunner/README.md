# Task Runner Module

The task runner module provides a framework for executing bash scripts locally or remotely on servers via SSH, with support for background execution, output streaming, and webhook callbacks.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Task Runner                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Task (Interface)      PendingTask         Dispatcher           │
│  ├─ Name()            ├─ Task             ├─ Run()              │
│  ├─ Timeout()         ├─ Connection       ├─ runLocal()         │
│  ├─ Script()          ├─ Background       └─ runRemote()        │
│  ├─ OnOutput()        └─ WithID()                               │
│  ├─ OnFinished()                                                │
│  ├─ OnFailed()        Connection          WebhookHandler        │
│  └─ OnTimeout()       ├─ Host             ├─ MarkAsFinished()   │
│                       ├─ Port             ├─ MarkAsFailed()     │
│                       ├─ User             └─ MarkAsTimeout()    │
│                       └─ PrivateKey                             │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Components

### Task Interface
Base interface that all tasks must implement:
- `Name()` - Human-readable task name
- `Timeout()` - Maximum execution time
- `Script()` - Generate bash script from template
- `OnOutput()` - Called when output is received
- `OnFinished()` - Called on successful completion
- `OnFailed()` - Called on failure
- `OnTimeout()` - Called when task times out

### PendingTask
Wrapper that configures execution options:
```go
task := NewPendingTask(myTask).
    OnConnection(sshConnection).
    InBackground().
    WithID("task-123")
```

### Dispatcher
Handles task execution routing:
- Local execution via `exec.Command`
- Remote execution via SSH
- Background execution with `nohup`
- Output streaming via tee

### TaskModel
Database representation for task tracking:
- Status tracking (pending, running, finished, failed, timeout)
- Output storage
- Exit code
- Timestamps

### WebhookHandler
Handles callbacks for background tasks:
- Signed URLs for security
- Automatic status updates
- Output fetching

## Usage

### Simple Local Task
```go
task := &BaseTask{
    TaskName:    "hello-world",
    TaskTimeout: 1 * time.Minute,
    Template:    "echo 'Hello, World!'",
}

dispatcher := NewDispatcher(logger, wsHub)
result, err := NewPendingTask(task).Dispatch(ctx, dispatcher)
```

### Remote Task via SSH
```go
conn := &Connection{
    Host:       "192.168.1.100",
    Port:       22,
    User:       "root",
    PrivateKey: privateKeyPEM,
}

result, err := NewPendingTask(task).
    OnConnection(conn).
    Dispatch(ctx, dispatcher)
```

### Background Task with Callbacks
```go
// Generate webhook URLs
urls := webhookHandler.GenerateCallbackURLs(baseURL, taskID, 60)

// Wrap script with callback handling
wrappedScript, _ := CreateBackgroundWrapper(script, timeout, urls)

task := &BaseTask{
    Template: wrappedScript,
}

result, err := NewPendingTask(task).
    OnConnection(conn).
    InBackground().
    WithID(taskID).
    Dispatch(ctx, dispatcher)
```

## Execution Flow

### Foreground Execution
```
1. Generate script from template
2. Upload script to server (if remote)
3. Execute: bash script 2>&1 | tee output.log
4. Wait for completion
5. Return result with output, exit code
```

### Background Execution
```
1. Generate script from template
2. Wrap with callback handler
3. Upload to server
4. Execute: nohup timeout Ns bash script > output.log 2>&1 &
5. Return immediately with PID
6. Script calls webhook on completion:
   - Exit 0 → /webhooks/tasks/{id}/finished
   - Exit 124 → /webhooks/tasks/{id}/timeout
   - Exit >0 → /webhooks/tasks/{id}/failed
7. Webhook updates database and broadcasts event
```

## Script Templates

Pre-built templates in `scripts.go`:
- `background_wrapper` - Wraps any script with callback handling
- `provision_base` - Base server provisioning
- `install_php` - PHP installation
- `install_caddy` - Caddy web server installation
- `deploy_site` - Site deployment with zero-downtime
- `rollback` - Rollback to previous release

## Laravel Migration Reference

| Laravel Component | Go Equivalent |
|------------------|---------------|
| `Task` class | `Task` interface + `BaseTask` |
| `PendingTask` | `PendingTask` |
| `TaskDispatcher` | `Dispatcher` |
| `Connection` | `Connection` |
| `RemoteProcessRunner` | Part of `Dispatcher.runRemote()` |
| `TaskModel` | `TaskModel` |
| `TaskWebhookController` | `WebhookHandler` |
| Blade templates | Go `text/template` |
| `HasCallbacks` trait | Callback functions on `BaseTask` |

## Database Schema

```sql
CREATE TABLE tasks (
    id VARCHAR(26) PRIMARY KEY,
    server_id VARCHAR(26) NOT NULL,
    user_id VARCHAR(26),
    name VARCHAR(255) NOT NULL,
    user VARCHAR(100) DEFAULT 'root',
    type VARCHAR(500),
    script TEXT,
    timeout INT DEFAULT 600,
    status VARCHAR(50) DEFAULT 'pending',
    output TEXT,
    exit_code INT,
    pid VARCHAR(20),
    started_at TIMESTAMP,
    finished_at TIMESTAMP,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,
    callback_data TEXT
);
```

## Events

WebSocket events broadcast on task updates:
- `task.updated` - Status change
- `task.output` - New output received

Channel: `task.{task_id}`
