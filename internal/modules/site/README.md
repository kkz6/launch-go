# Site Module

Manages site/application deployment with zero-downtime support, SSL certificates, and environment configuration.

## Features

- Zero-downtime deployments using symlink strategy
- Git integration (GitHub, GitLab, Bitbucket)
- Rollback to previous releases
- Environment variable management
- SSL certificate provisioning (Let's Encrypt via Caddy)
- Queue worker management
- Deployment history and logs
- Real-time deployment progress via WebSocket

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/sites` | List all sites |
| POST | `/api/sites` | Create site |
| GET | `/api/sites/:id` | Get site details |
| PUT | `/api/sites/:id` | Update site |
| DELETE | `/api/sites/:id` | Delete site |
| POST | `/api/sites/:id/deploy` | Trigger deployment |
| GET | `/api/sites/:id/deployments` | List deployments |
| GET | `/api/sites/:id/deployments/:deploymentId` | Get deployment details |
| POST | `/api/sites/:id/rollback/:releaseId` | Rollback to release |
| GET | `/api/sites/:id/environment` | Get environment variables |
| PUT | `/api/sites/:id/environment` | Update environment variables |
| POST | `/api/sites/:id/ssl` | Enable SSL |
| DELETE | `/api/sites/:id/ssl` | Disable SSL |

## Models

### Site
```go
type Site struct {
    ID                 string
    TeamID             string
    ServerID           string
    Name               string
    Domain             string
    Aliases            string          // JSON array
    Type               SiteType        // laravel, wordpress, static
    Status             SiteStatus
    Path               string
    PublicPath         string          // default: public
    PHPVersion         string
    RepositoryProvider *string         // github, gitlab, bitbucket
    RepositoryURL      *string
    RepositoryBranch   string          // default: main
    DeploymentScript   string
    EnvironmentVars    string          // Encrypted
    SSLEnabled         bool
    ZeroDowntime       bool
    ReleasesToKeep     int             // default: 5
    CurrentRelease     *string
    QueueEnabled       bool
    SchedulerEnabled   bool
}
```

### Deployment
```go
type Deployment struct {
    ID            string
    SiteID        string
    ReleaseID     *string
    UserID        string
    Status        DeploymentStatus  // pending, running, succeeded, failed
    CommitHash    *string
    CommitMessage *string
    Branch        string
    Log           string
    StartedAt     *time.Time
    FinishedAt    *time.Time
    Duration      *int
    IsRollback    bool
}
```

### Release
```go
type Release struct {
    ID         string
    SiteID     string
    Path       string
    CommitHash *string
    CreatedAt  time.Time
}
```

## Directory Structure on Server

```
/home/{user}/{site}/
├── current -> releases/20240115120000    # Symlink to active release
├── releases/
│   ├── 20240115120000/                   # Current release
│   ├── 20240114100000/                   # Previous release
│   └── 20240113090000/                   # Older release
└── shared/
    ├── storage/                          # Persistent storage
    │   ├── app/
    │   ├── framework/
    │   └── logs/
    └── .env                              # Environment file
```

## Deployment Flow

### Zero-Downtime Deployment
```
1. Create deployment record (status: pending)
2. Create new release directory
3. Clone/checkout repository to release directory
4. Link shared directories (storage, .env)
5. Install Composer dependencies
6. Install NPM dependencies
7. Build frontend assets
8. Run Laravel optimizations (config:cache, route:cache)
9. Run migrations (if enabled)
10. Update 'current' symlink atomically
11. Restart PHP-FPM
12. Restart queue workers (if enabled)
13. Clean up old releases (keep N releases)
14. Update deployment status to succeeded
```

### Rollback
```
1. Verify target release exists
2. Update 'current' symlink to target release
3. Restart PHP-FPM
4. Restart queue workers (if enabled)
```

## Background Jobs

| Job Type | Description |
|----------|-------------|
| `site:deploy` | Standard deployment |
| `site:deploy_zero_downtime` | Zero-downtime deployment |
| `site:rollback` | Rollback to specific release |
| `site:install_ssl` | Provision SSL certificate |

## WebSocket Events

Channel: `deployment.{deployment_id}`

| Event | Description |
|-------|-------------|
| `deployment.started` | Deployment started |
| `deployment.progress` | Step progress update |
| `deployment.log` | Log output |
| `deployment.finished` | Deployment completed |
| `deployment.failed` | Deployment failed |

## Laravel Migration Reference

| Laravel | Go |
|---------|-----|
| `modules/site/src/Models/Site.php` | `models.go` |
| `modules/site/src/Models/Deployment.php` | `models.go` |
| `modules/site/src/Models/Release.php` | `models.go` |
| `modules/site/src/Jobs/DeployWithoutDowntimeJob.php` | `jobs/jobs.go` |
| `modules/site/src/Enums/SiteTaskEnum.php` | Task types in jobs |
| `modules/site/src/Tasks/DeploySite.php` | Uses `taskrunner` module |

## Environment Variables

Environment variables are stored encrypted and synced to the server's `.env` file during deployment.

```go
// Stored in site.EnvironmentVars (encrypted JSON)
{
  "APP_ENV": "production",
  "APP_KEY": "base64:...",
  "DB_HOST": "localhost",
  "DB_DATABASE": "myapp"
}
```
