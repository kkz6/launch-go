# Site Deployment Fixes TODO

## Critical Issues

### 1. [x] RollbackJob Not Implemented
**File:** `internal/modules/site/jobs/rollback.go`
**Status:** COMPLETED
**Fix Applied:**
- Implemented full rollback functionality
- Gets site, deployment, and target release
- Runs rollback task to update symlink
- Updates deployment status
- Broadcasts rollback events
- Restarts queue workers if auto-restart is enabled

### 2. [x] InstalledAt Never Set After First Deployment
**File:** `internal/modules/site/jobs/install_caddyfile.go`
**Status:** COMPLETED
**Fix Applied:**
- Set `site.InstalledAt = &now` after successful Caddyfile install
- Added failure handling to set `InstallationFailedAt`

### 3. [x] Queue Worker Jobs Not Implemented
**Files:** Created `install_queue.go`, `restart_queue.go`, `uninstall_queue.go`, `tasks/queue.go`
**Status:** COMPLETED
**Fix Applied:**
- Created InstallQueueJob - generates supervisor config, uploads, reloads supervisor
- Created RestartQueueJob - restarts queue worker via supervisorctl
- Created UninstallQueueJob - removes supervisor config, reloads
- Created RestartAllSiteQueuesJob for batch restart
- Created queue task helpers (BuildQueueSupervisorConfig, UploadQueueConfig, etc.)
- Registered all jobs in register.go

### 4. [x] App-Based Git Auth Not Integrated
**Files:** `internal/modules/site/jobs/deploy.go`, `internal/modules/git/providers/*.go`
**Status:** COMPLETED
**Fix Applied:**
- Added GetInstallationToken() to Provider interface
- Implemented token generation for GitHub, GitLab, Bitbucket
- Deploy jobs now fetch source control and get installation tokens
- Build authenticated HTTPS URLs for each provider type
- Updated JobContext to include SourceControlRepo and ProviderFactory

---

## Medium Issues

### 5. [x] Environment Variable Auto-Generation
**File:** `internal/modules/site/models/site.go`
**Status:** COMPLETED
**Fix Applied:**
- Added Site.GenerateEnvironmentVariables() method
- Generate APP_KEY and APP_URL for Laravel sites
- Generate WordPress security keys (AUTH_KEY, AUTH_SALT, etc.)
- Environment variables passed to deploy config for first deployments

### 6. [x] Site Deletion Job Missing
**File:** `internal/modules/site/jobs/uninstall_site.go`
**Status:** COMPLETED
**Fix Applied:**
- Created UninstallSiteJob
- Uninstalls all queue workers
- Updates Caddyfile to remove site imports
- Deletes site files from server
- Deletes related database records (deployments, certificates, commands, redirects, releases)
- Deletes site record
- Broadcasts site.deleted event

### 7. [x] Feature Tracking Not Implemented
**Files:** `internal/modules/site/models/site.go`, `internal/modules/site/enums/enums.go`
**Status:** COMPLETED
**Fix Applied:**
- Added EnabledFeatures field (JSON array with metadata)
- Added PendingFeatures field (string slice)
- Created EnabledFeature struct with Name, QueueID, CronID, EnabledAt
- Added helper methods: HasEnabledFeature, GetEnabledFeature, HasPendingFeature
- Added mutation methods: AddEnabledFeature, RemoveEnabledFeature, AddPendingFeature, RemovePendingFeature
- Created LaravelFeature enum with Scheduler, Queue, Horizon, Inertia, Octane, Reverb

---

## Progress

| # | Issue | Status | Date |
|---|-------|--------|------|
| 1 | RollbackJob | COMPLETED | 2026-01-15 |
| 2 | InstalledAt | COMPLETED | 2026-01-15 |
| 3 | Queue Worker Jobs | COMPLETED | 2026-01-15 |
| 4 | App-Based Git Auth | COMPLETED | 2026-01-15 |
| 5 | Env Var Generation | COMPLETED | 2026-01-15 |
| 6 | Site Deletion Job | COMPLETED | 2026-01-15 |
| 7 | Feature Tracking | COMPLETED | 2026-01-15 |

## Summary

All 7 deployment fixes have been implemented:

**Files Created:**
- `internal/modules/site/jobs/install_queue.go`
- `internal/modules/site/jobs/restart_queue.go`
- `internal/modules/site/jobs/uninstall_queue.go`
- `internal/modules/site/jobs/uninstall_site.go`
- `internal/modules/site/tasks/queue.go`

**Files Modified:**
- `internal/modules/site/jobs/rollback.go` - Full rollback implementation
- `internal/modules/site/jobs/install_caddyfile.go` - InstalledAt setting
- `internal/modules/site/jobs/deploy.go` - App auth and env vars integration
- `internal/modules/site/jobs/types.go` - New payload types
- `internal/modules/site/jobs/register.go` - New job registrations
- `internal/modules/site/jobs/context.go` - Added git dependencies
- `internal/modules/site/models/site.go` - Env var generation, feature tracking
- `internal/modules/site/enums/enums.go` - LaravelFeature enum
- `internal/modules/site/module.go` - Git provider integration
- `internal/modules/git/providers/provider.go` - GetInstallationToken interface
- `internal/modules/git/providers/github.go` - Token generation
- `internal/modules/git/providers/gitlab.go` - Token generation
- `internal/modules/git/providers/bitbucket.go` - Token generation
