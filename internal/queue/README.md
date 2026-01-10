# Queue Module

Provides background job processing using Asynq (Redis-based).

## Features

- Redis-backed persistent queue
- Priority queues (critical, default, low)
- Automatic retries with backoff
- Job scheduling
- Job monitoring via Asynq dashboard

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Queue System                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  API Server              Redis              Worker               │
│  ├─ Enqueue job    →    Queue storage  →   Process job          │
│  └─ Client              ├─ critical        ├─ Handler           │
│                         ├─ default         └─ Retry/fail        │
│                         └─ low                                   │
│                                                                  │
│  Dashboard (asynqmon)                                            │
│  └─ Monitor jobs, retries, failures                             │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Queue Priorities

| Queue | Priority | Use Case |
|-------|----------|----------|
| `critical` | 6 | Server provisioning, deployments |
| `default` | 3 | PHP installation, SSL, etc. |
| `low` | 1 | Cleanup, non-urgent tasks |

## Usage

### Enqueue Job
```go
// Create task
task, err := jobs.NewProvisionTask(serverID, teamID)

// Enqueue with priority
client.EnqueueCritical(task)
client.EnqueueDefault(task)
client.EnqueueLow(task)

// With options
client.Enqueue(task,
    asynq.Queue("critical"),
    asynq.MaxRetry(3),
    asynq.Timeout(10*time.Minute),
)
```

### Define Job Handler
```go
const TypeMyJob = "my:job"

type MyJobPayload struct {
    ID   string `json:"id"`
    Data string `json:"data"`
}

func NewMyJob(payload MyJobPayload) (*asynq.Task, error) {
    data, err := json.Marshal(payload)
    if err != nil {
        return nil, err
    }
    return asynq.NewTask(TypeMyJob, data), nil
}

func HandleMyJob(ctx context.Context, t *asynq.Task) error {
    var payload MyJobPayload
    if err := json.Unmarshal(t.Payload(), &payload); err != nil {
        return err
    }

    // Process job
    return nil
}
```

### Register Handler
```go
// In worker main.go
mux := asynq.NewServeMux()
mux.HandleFunc(TypeMyJob, HandleMyJob)
```

## Job Types

### Server Jobs
- `server:provision` - Full server provisioning
- `server:install_php` - Install PHP version
- `server:configure_firewall` - Apply firewall rules
- `server:install_database` - Install database
- `server:reboot` - Reboot server

### Site Jobs
- `site:deploy` - Standard deployment
- `site:deploy_zero_downtime` - Zero-downtime deployment
- `site:rollback` - Rollback to release
- `site:install_ssl` - Provision SSL certificate

### Task Jobs
- `task:run` - Execute remote task
- `task:update_output` - Fetch task output
- `task:check_status` - Check background task status

## Configuration

```env
REDIS_ADDRESS=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0
QUEUE_CONCURRENCY=10
```

## Monitoring

Asynq provides a web dashboard (asynqmon):

```yaml
# docker-compose.yml
asynqmon:
  image: hibiken/asynqmon:latest
  ports:
    - "8081:8080"
  environment:
    - REDIS_ADDR=redis:6379
```

Access at: http://localhost:8081

## Laravel Migration Reference

| Laravel | Go (Asynq) |
|---------|------------|
| `dispatch(new Job)` | `client.Enqueue(task)` |
| `Job::dispatch()->onQueue('high')` | `client.EnqueueCritical(task)` |
| Queue workers | `asynq.Server` |
| `php artisan queue:work` | `go run ./cmd/worker` |
| Laravel Horizon | Asynqmon dashboard |
| Failed jobs table | Redis + asynqmon |
| Job batching | Asynq groups (manual) |

## Error Handling

```go
func HandleMyJob(ctx context.Context, t *asynq.Task) error {
    // Return error to retry
    if err := doWork(); err != nil {
        return fmt.Errorf("work failed: %w", err)
    }

    // Return nil on success
    return nil

    // Skip retry (permanent failure)
    // return asynq.SkipRetry
}
```

## Retry Configuration

```go
// Per-job retry
client.Enqueue(task, asynq.MaxRetry(5))

// Global in server config
srv := asynq.NewServer(redisOpt, asynq.Config{
    RetryDelayFunc: func(n int, err error, t *asynq.Task) time.Duration {
        return time.Duration(n) * time.Minute
    },
})
```
