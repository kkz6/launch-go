# Launch Go - Migration Reference Guide

This document contains all the context needed to migrate features from the Laravel project (`/Users/karthick/PhpstormProjects/launch`) to this Go implementation.

## Source Project Reference

**Laravel Project Path**: `/Users/karthick/PhpstormProjects/launch`

When implementing features, always reference the Laravel codebase for:
- Business logic
- Database schema
- API contracts
- Job implementations
- SSH scripts

## Project Overview

Launch is a **zero-downtime deployment platform** that allows teams to:
- Provision servers on multiple cloud providers
- Deploy applications with zero downtime
- Manage databases, backups, SSL certificates
- Monitor server health and performance

## Architecture Mapping

### Laravel Module -> Go Module

| Laravel Module | Go Module | Status |
|---------------|-----------|--------|
| `modules/auth` | `internal/modules/auth` | Scaffold |
| `modules/server` | `internal/modules/server` | Scaffold |
| `modules/site` | `internal/modules/site` | Scaffold |
| `modules/deployment` | `internal/modules/site` (merged) | Scaffold |
| `modules/database` | `internal/modules/database` | TODO |
| `modules/backup` | `internal/modules/backup` | TODO |
| `modules/dns` | `internal/modules/dns` | TODO |
| `modules/git` | `internal/modules/git` | TODO |
| `modules/billing` | `internal/modules/billing` | TODO |
| `modules/notification` | `internal/modules/notification` | TODO |
| `modules/task-runner` | `internal/queue` + jobs | Scaffold |

## Database Schema Reference

The Laravel migrations are in: `modules/*/database/migrations/`

Key tables to migrate:
- `users` - User accounts
- `teams` - Multi-tenancy
- `team_members` - Team membership
- `servers` - Server records
- `sites` - Site/application records
- `deployments` - Deployment history
- `releases` - Release snapshots for rollback
- `databases` - Database records on servers
- `database_users` - Database user credentials
- `backups` - Backup records
- `backup_jobs` - Scheduled backup jobs
- `ssh_keys` - SSH keys on servers
- `firewall_rules` - UFW rules
- `cron_jobs` - Scheduled tasks
- `certificates` - SSL certificates
- `redirects` - URL redirects
- `source_controls` - Git provider connections

### Primary Key Format
Laravel uses **ULIDs** (26 characters). Go implementation uses `github.com/oklog/ulid/v2`.

## Laravel Jobs -> Go Jobs Reference

### Server Module Jobs
Location: `modules/server/src/Jobs/`

| Laravel Job | Go Task Type | Description |
|------------|--------------|-------------|
| `ProvisionServerJob` | `server:provision` | Full server provisioning |
| `InstallPhpJob` | `server:install_php` | Install PHP version |
| `RemovePhpJob` | `server:remove_php` | Remove PHP version |
| `InstallDatabaseJob` | `server:install_database` | Install MySQL/PostgreSQL |
| `CreateDatabaseJob` | `server:create_database` | Create database |
| `CreateDatabaseUserJob` | `server:create_db_user` | Create database user |
| `ConfigureFirewallJob` | `server:configure_firewall` | Configure UFW rules |
| `AddFirewallRuleJob` | `server:add_firewall_rule` | Add single rule |
| `RemoveFirewallRuleJob` | `server:remove_firewall_rule` | Remove rule |
| `AddSshKeyJob` | `server:add_ssh_key` | Add SSH key |
| `RemoveSshKeyJob` | `server:remove_ssh_key` | Remove SSH key |
| `InstallCronJob` | `server:install_cron` | Add cron entry |
| `RemoveCronJob` | `server:remove_cron` | Remove cron entry |
| `RebootServerJob` | `server:reboot` | Reboot server |
| `UpdateOpcacheConfigJob` | `server:update_opcache` | Configure OPcache |
| `RunVulnerabilityAuditJob` | `server:vulnerability_audit` | Security audit |

### Site Module Jobs
Location: `modules/site/src/Jobs/`

| Laravel Job | Go Task Type | Description |
|------------|--------------|-------------|
| `DeploySiteJob` | `site:deploy` | Standard deployment |
| `DeployWithoutDowntimeJob` | `site:deploy_zero_downtime` | Zero-downtime deploy |
| `RollbackJob` | `site:rollback` | Rollback to release |
| `InstallSslJob` | `site:install_ssl` | Let's Encrypt SSL |
| `RemoveSslJob` | `site:remove_ssl` | Remove SSL |
| `UpdateCaddyConfigJob` | `site:update_caddy` | Update Caddy config |
| `InstallQueueJob` | `site:install_queue` | Setup Laravel queue |
| `RestartQueueJob` | `site:restart_queue` | Restart queue workers |
| `RunCommandJob` | `site:run_command` | Run arbitrary command |

## Cloud Provider Integrations

### Reference Files
- DigitalOcean: `modules/server/src/Services/Providers/DigitalOcean/`
- Hetzner: `modules/server/src/Services/Providers/Hetzner/`
- AWS: `modules/server/src/Services/Providers/Aws/`
- Linode: `modules/server/src/Services/Providers/Linode/`
- Vultr: `modules/server/src/Services/Providers/Vultr/`

### Provider Interface
Each provider must implement:
```go
type Provider interface {
    CreateServer(ctx context.Context, opts CreateOptions) (*ProviderServer, error)
    DeleteServer(ctx context.Context, serverID string) error
    RebootServer(ctx context.Context, serverID string) error
    GetServer(ctx context.Context, serverID string) (*ProviderServer, error)
    ListRegions(ctx context.Context) ([]Region, error)
    ListSizes(ctx context.Context) ([]Size, error)
}
```

## Git Provider Integrations

### Reference Files
- GitHub: `modules/git/src/Providers/GitHub/`
- GitLab: `modules/git/src/Providers/GitLab/`
- Bitbucket: `modules/git/src/Providers/Bitbucket/`

### Provider Interface
```go
type GitProvider interface {
    ListRepositories(ctx context.Context) ([]Repository, error)
    GetRepository(ctx context.Context, repo string) (*Repository, error)
    GetBranches(ctx context.Context, repo string) ([]Branch, error)
    GetCommit(ctx context.Context, repo, sha string) (*Commit, error)
    CreateWebhook(ctx context.Context, repo, url, secret string) (*Webhook, error)
    DeleteWebhook(ctx context.Context, repo, webhookID string) error
}
```

## Server Provisioning Scripts

### Reference Location
`modules/server/src/Scripts/` and `modules/server/resources/scripts/`

### Key Scripts to Port
1. **Base Provisioning**
   - Update system packages
   - Install essential tools (git, curl, unzip, etc.)
   - Configure timezone
   - Setup swap file
   - Configure SSH

2. **PHP Installation**
   - Add Ondrej PPA
   - Install PHP with extensions
   - Configure PHP-FPM
   - Configure OPcache

3. **Web Server (Caddy)**
   - Install Caddy
   - Configure Caddyfile
   - Setup automatic HTTPS

4. **Database**
   - Install MySQL/PostgreSQL
   - Secure installation
   - Create databases/users

5. **Security**
   - Configure UFW firewall
   - Fail2ban setup
   - SSH hardening

## Deployment Flow Reference

### Zero-Downtime Deployment Steps
Reference: `modules/site/src/Jobs/DeployWithoutDowntimeJob.php`

1. Create new release directory
2. Clone/checkout repository
3. Copy shared files (storage, .env)
4. Install Composer dependencies
5. Install NPM dependencies
6. Build assets
7. Run Laravel optimizations
8. Run migrations
9. Update `current` symlink
10. Restart PHP-FPM
11. Restart queue workers
12. Clean up old releases

### Directory Structure on Server
```
/home/{user}/{site}/
├── current -> releases/20240115120000
├── releases/
│   ├── 20240115120000/
│   ├── 20240114100000/
│   └── 20240113090000/
└── shared/
    ├── storage/
    └── .env
```

## API Response Format

All API responses should follow this format:

```json
{
  "success": true,
  "message": "Resource retrieved successfully",
  "data": { ... },
  "meta": {
    "current_page": 1,
    "per_page": 15,
    "total": 100,
    "last_page": 7
  }
}
```

Error responses:
```json
{
  "success": false,
  "message": "Validation failed",
  "errors": {
    "field": ["Error message"]
  }
}
```

## WebSocket Events

### Server Events
- `server.status` - Server status change
- `server.progress` - Provisioning progress

### Deployment Events
- `deployment.started` - Deployment started
- `deployment.progress` - Step progress
- `deployment.log` - Log output
- `deployment.finished` - Deployment complete
- `deployment.failed` - Deployment failed

### Site Events
- `site.updated` - Site configuration changed

## Task Enums Reference

The Laravel project uses task enums for organizing jobs. Reference:
- `modules/server/src/Enums/ServerTaskEnum.php`
- `modules/site/src/Enums/SiteTaskEnum.php`

## Type Definitions (Enums)

Go doesn't have native enums, so we use typed constants. All type definitions follow this pattern:

### Location
- Module-specific types: `internal/modules/{module}/types/`
- Shared helpers: `internal/pkg/enumtypes/`

### Import Aliases
Each module's types are imported with a descriptive alias:
- `servertypes "github.com/kkz6/launch-go/internal/modules/server/types"`
- `sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"`
- `authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"`
- `dnstypes`, `backuptypes`, `billingtypes`, `gittypes`, `notificationtypes`, `scripttypes`

### Pattern
```go
type Status string

const (
    StatusActive   Status = "active"
    StatusInactive Status = "inactive"
)

var allStatuses = []Status{StatusActive, StatusInactive}

func AllStatuses() []Status { return allStatuses }

func (s Status) String() string { return string(s) }
func (s Status) IsValid() bool  { /* switch on all values */ }
func (s Status) Label() string  { /* switch returning display text */ }

// Database scanning (if stored in DB)
func (s *Status) Scan(value any) error      { return enumtypes.ScanString(s, value) }
func (s Status) Value() (driver.Value, error) { return enumtypes.ValueString(s) }
```

### File Organization
- Small enums (< 50 lines): Consolidate in `types/types.go`
- Large enums with many methods: Separate file like `types/software.go`

## Testing Reference

Laravel tests location: `modules/*/tests/`

Key test patterns:
- Unit tests for services
- Feature tests for API endpoints
- Job tests with fake queue

## Environment Variables

See `.env.example` for all configuration options. Key ones:
- Database connection
- Redis for queues
- JWT secret
- Cloud provider credentials
- Email service credentials

## Commands for Development

```bash
# Run API server
make run

# Run queue worker
make worker

# Run tests
make test

# Create migration
make migrate-create name=create_servers_table

# Run migrations
make migrate-up
```

## Implementation Priority

1. **Phase 1**: Auth, Teams (current scaffold)
2. **Phase 2**: Servers, Cloud Providers, SSH
3. **Phase 3**: Sites, Deployments, Git integrations
4. **Phase 4**: Databases, Backups, DNS
5. **Phase 5**: Billing, Notifications

## When Implementing a Feature

1. Read the Laravel implementation first
2. Check the database schema in migrations
3. Review the job/service logic
4. Check for any SSH scripts used
5. Implement in Go following the existing patterns
6. Write tests
7. Update this document with any new patterns

## Task Definition with Callbacks

Tasks are SSH scripts that run on servers. Tasks that need post-completion logic (like updating deployment status) implement callbacks.

### Task Structure

A task with callbacks has three parts:

1. **Callback Data** - Only store IDs, never full objects (they can't be serialized)
2. **Task Struct** - Implements `Task` and `CallbackPayload` interfaces
3. **Registration** - Register in `register.go` for callback reconstruction

### Example: Defining a Task with Callbacks

```go
// internal/modules/site/tasks/deploy.go

const DeploySiteTaskType = "site:deploy"

// 1. Callback data - ONLY IDs and simple values (serialized to DB)
type callbackData struct {
    SiteID           string `json:"site_id"`
    ServerID         string `json:"server_id"`    // Include ALL IDs needed by callbacks
    DeploymentID     string `json:"deployment_id"`
    SiteType         string `json:"site_type"`
    IsFirstDeploy    bool   `json:"is_first_deploy"`
}

// 2. Task struct
type deploySiteTask struct {
    *taskrunner.BaseTask
    opts     DeployOptions
    callback callbackData
}

// Constructor - populate callback data from models
func DeploySiteTask(opts DeployOptions) *deploySiteTask {
    return &deploySiteTask{
        BaseTask: taskrunner.NewBaseTask(
            taskrunner.WithName("Deploy Site"),
            taskrunner.WithScript(buildScript(opts)),
            taskrunner.WithTimeoutSeconds(600),
        ),
        opts: opts,
        callback: callbackData{
            SiteID:        opts.Site.ID,
            ServerID:      opts.Site.ServerID,  // Don't forget related IDs!
            DeploymentID:  opts.Deployment.ID,
            SiteType:      string(opts.Site.Type),
            IsFirstDeploy: opts.Site.InstalledAt == nil,
        },
    }
}

// Required for CallbackPayload interface
func (t *deploySiteTask) TypeName() string {
    return DeploySiteTaskType
}

func (t *deploySiteTask) MarshalPayload() ([]byte, error) {
    return json.Marshal(t.callback)
}

// Callback handlers - called after task completes
func (t *deploySiteTask) OnSuccess(ctx context.Context, cbCtx *taskrunner.CallbackContext, taskID string) error {
    // Update status, dispatch follow-up jobs, etc.
    return cbCtx.DB.Model(&models.Deployment{}).
        Where("id = ?", t.callback.DeploymentID).
        Update("status", "finished").Error
}

func (t *deploySiteTask) OnFailure(ctx context.Context, cbCtx *taskrunner.CallbackContext, taskID string, exitCode int) error {
    return cbCtx.DB.Model(&models.Deployment{}).
        Where("id = ?", t.callback.DeploymentID).
        Update("status", "failed").Error
}

func (t *deploySiteTask) OnExpired(ctx context.Context, cbCtx *taskrunner.CallbackContext, taskID string) error {
    return cbCtx.DB.Model(&models.Deployment{}).
        Where("id = ?", t.callback.DeploymentID).
        Update("status", "timeout").Error
}

// 3. CallbackStateFactory - enables reconstruction from serialized data
func (s callbackData) NewTask() taskrunner.CallbackHandler {
    return &deploySiteTask{
        BaseTask: taskrunner.NewBaseTask(),
        callback: s,
    }
}
```

### Registration

```go
// internal/modules/site/tasks/register.go

func RegisterTaskCallbacks() {
    taskrunner.RegisterCallbackState[callbackData](DeploySiteTaskType)
}
```

### Type-Safe Job Dispatch from Callbacks

When dispatching jobs from callbacks, use typed payload structs to catch missing fields at compile time:

```go
// Define payload structs that mirror job payloads (avoids circular imports)
type analyzeFeaturesPayload struct {
    SiteID   string `json:"site_id"`
    ServerID string `json:"server_id"`  // Required - compiler won't catch if missing from map[string]string!
}

// Type-safe dispatch method
func (t *deploySiteTask) dispatchAnalyzeFeatures(cbCtx *taskrunner.CallbackContext) {
    t.dispatchJob(cbCtx, "site:analyze_features", analyzeFeaturesPayload{
        SiteID:   t.callback.SiteID,
        ServerID: t.callback.ServerID,  // IDE shows available fields
    })
}

// Generic dispatch helper
func (t *deploySiteTask) dispatchJob(cbCtx *taskrunner.CallbackContext, jobType string, payload any) {
    if cbCtx.Queue == nil {
        return
    }
    data, _ := json.Marshal(payload)
    task := asynq.NewTask(jobType, data)
    cbCtx.Queue.Enqueue(task)
}
```

### Key Rules

1. **Callback data must only contain IDs** - Never store model pointers or complex objects
2. **Include ALL IDs needed by callbacks** - If a callback dispatches a job that needs `server_id`, include it in callback data
3. **Use typed payloads for job dispatch** - Avoid `map[string]string` which has no compile-time safety
4. **Register in register.go** - Don't use `init()` magic, explicit registration is preferred
5. **Callbacks run in ALL execution modes** - Whether sync, async, or background with HTTP callbacks

### Execution Modes

Tasks can run in different modes based on environment:

- **Local/Dev mode**: Long-running SSH connection, output streamed in real-time
- **Production mode**: Script uploaded, runs in background, status updated via HTTP callbacks

The TaskRunner automatically handles both modes - callbacks are invoked regardless of execution mode.

## Notes

- The Laravel project uses Spatie packages extensively (permissions, activity log, data)
- JWT tokens include `team_id` for multi-tenancy
- All timestamps are in UTC
- File paths use forward slashes even on the server
- SSH connections default to port 22 but can be customized
