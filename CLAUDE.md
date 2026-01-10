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

## Notes

- The Laravel project uses Spatie packages extensively (permissions, activity log, data)
- JWT tokens include `team_id` for multi-tenancy
- All timestamps are in UTC
- File paths use forward slashes even on the server
- SSH connections default to port 22 but can be customized
