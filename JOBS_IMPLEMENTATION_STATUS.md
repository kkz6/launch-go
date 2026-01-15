# Jobs Implementation Status: Laravel vs Go

This document provides a comprehensive comparison of jobs between the Laravel and Go implementations, including both job definitions and their triggers.

## Executive Summary

| Module | Laravel Jobs | Go Jobs | Triggers Working | Coverage |
|--------|-------------|---------|------------------|----------|
| Server | 37 | 24 | 18/30 | 60% |
| Site | 26 | 13 | 8/25 | 32% |
| Database | 6 | 6 | 7/7 | **100%** ✅ |
| Git | 2 | 2 | 1/2 | 50% |
| Backup | 4 | 0 | 0/4 | **0%** ❌ |
| DNS | 2 | 0 | 0/2 | **0%** ❌ |
| **Total** | **77** | **45** | **34/70** | **49%** |

---

## SERVER MODULE

### Implemented & Triggered ✅

| Laravel Job | Go Job | Laravel Trigger | Go Trigger |
|-------------|--------|-----------------|------------|
| ProvisionServer | ProvisionServer | ServerService.install() | ServerService.CreateServer() |
| DeleteServerFromInfrastructure | DeleteServer | ServerService.deleteServer() | ServerService.DeleteServer() |
| - | RebootServer | - | ServerService.RebootServer() |
| ArchiveServer | ArchiveServer | ServerService.archive() | ServerService.ArchiveServer() |
| UnarchiveServer | UnarchiveServer | ServerService.unarchive() | ServerService.UnarchiveServer() |
| RunVulnerabilityAudit | VulnerabilityAudit | ServerService.runVulnerabilityAudit() | ServerService.RunVulnerabilityAudit() |
| InstallCron | InstallCron | CronService.create() | CronService.CreateCron() |
| UninstallCron | UninstallCron | CronService.delete() | CronService.DeleteCron() |
| InstallDaemon | InstallDaemon | DaemonService.create() | DaemonService.CreateDaemon() |
| UninstallDaemon | UninstallDaemon | DaemonService.delete() | DaemonService.DeleteDaemon() |
| - | SyncDaemons | - | DaemonService.SyncDaemonsStatus() |
| InstallFirewallRule | InstallFirewallRule | FirewallRuleService.create() | FirewallRuleService.CreateFirewallRule() |
| UninstallFirewallRule | UninstallFirewallRule | FirewallRuleService.delete() | FirewallRuleService.DeleteFirewallRule() |
| AddSshKeyToServer | AddSshKey | SSHKeyService.attachToServer() | SshKeyService.AttachSshKey() |
| RemoveSshKeyFromServer | RemoveSshKey | SSHKeyService.detachFromServer() | SshKeyService.DetachSshKey() |
| AddServiceToServer | AddService | SoftwareService.install() | InstalledServiceService.CreateService() |
| RemoveServiceOnServer | RemoveService | SoftwareService.uninstall() | InstalledServiceService.DeleteService() |
| Start/Stop/RestartServiceOnServer | ServiceOperation | SoftwareService methods | InstalledServiceService methods |
| CheckServiceStatusOnServer | CheckServiceStatus | SoftwareService.checkStatus() | InstalledServiceService.CheckStatus() |
| ConfigureOpcacheOnServer | ConfigureOpcache | PhpService.updateOpcache() | OpcacheService.ConfigureOpcache() |
| - | CreateOnProvider | - | ServerService.CreateServer() |

### Implemented but NOT Triggered ⚠️

| Go Job | Status | Issue |
|--------|--------|-------|
| RestartDaemon | Job exists | DaemonService.RestartDaemon() method missing |

### NOT Implemented ❌

| Laravel Job | Description | Laravel Trigger | Priority |
|-------------|-------------|-----------------|----------|
| **WaitForServerToConnect** | Wait for SSH connection | Bus::chain first job in provisioning | **CRITICAL** |
| **AddPhpVersionToServer** | Install PHP version | PhpService.installVersion() | **HIGH** |
| **RemovePhpVersionFromServer** | Remove PHP version | PhpService.uninstallVersion() | **HIGH** |
| **UpdatePhpDefault** | Set default PHP version | PhpService.setDefaultVersion() | **HIGH** |
| **InstallPhpExtensionOnServer** | Install PHP extension | PhpService.installExtension() | **HIGH** |
| **UninstallPhpExtensionOnServer** | Remove PHP extension | PhpService.uninstallExtension() | **HIGH** |
| **CleanupFailedServerProvisioning** | Cleanup on provisioning failure | WaitForServerToConnect/ProvisionServer (on fail) | **HIGH** |
| CleanupFailedPhpInstallation | Cleanup failed PHP install | AddPhpVersionToServer (on fail) | MEDIUM |
| CleanupFailedPhpExtensionInstall | Cleanup failed extension install | InstallPhpExtension (on fail) | MEDIUM |
| CleanupFailedPhpExtensionUninstall | Cleanup failed extension uninstall | UninstallPhpExtension (on fail) | MEDIUM |
| UpdateServerConnectivity | Check server SSH connectivity | Scheduled command | MEDIUM |
| CheckDaemonStatus | Check daemon/supervisor status | Scheduled command | MEDIUM |
| UpdateTaskOutput | Stream task logs to database | Self-dispatching job | MEDIUM |
| UpdateUserPublicKey | Update user's SSH public key | ProvisionFreshServer job | LOW |
| InstallTaskCleanupCron | Setup task cleanup cron | ProvisionFreshServer job | LOW |
| RunAfterUpdateJob | Post-update commands | Server update events | LOW |

---

## SITE MODULE

### Implemented & Triggered ✅

| Laravel Job | Go Job | Laravel Trigger | Go Trigger |
|-------------|--------|-----------------|------------|
| DeploySite | Deploy | Site.queueDeployment() | DeploymentService.Deploy() |
| - | DeployZeroDowntime | (part of Deploy) | DeploymentService.createDeployment() |
| RollbackDeployment | Rollback | Site.rollback() | DeploymentService.Rollback() |
| RunCommand | RunCommand | CommandService.create() | CommandService.Create() |
| InstallCertificate | InstallSSL | SiteService.installCertificate() | SslService.UpdateSSL() |
| InstallSiteCaddyfile | InstallCaddyfile | DeploySite job | DeployJob.handleDeploymentSuccess() |
| UpdateSiteCaddyfile | UpdateCaddyfile | Various | Various |
| - | UninstallCaddyfile | - | Available |
| - | RestartQueue | DeploySite (autoRestart) | DeployJob.restartQueueWorkers() |
| - | SyncQueues | - | QueueService.SyncStatus() |

### Implemented but NOT Triggered ⚠️

| Go Job | Status | Issue |
|--------|--------|-------|
| InstallQueue | Job exists | QueueService.CreateQueue() method missing |
| UninstallQueue | Job exists | QueueService.DeleteQueue() method missing |
| UninstallSite | Job exists | Not triggered from site deletion flow |

### NOT Implemented ❌

| Laravel Job | Description | Laravel Trigger | Priority |
|-------------|-------------|-----------------|----------|
| **SourceControlDeploymentCreated** | Notify git provider deploy started | Event listener | **HIGH** |
| **SourceControlDeploymentCompleted** | Notify git provider deploy succeeded | Event listener | **HIGH** |
| **SourceControlDeploymentFailed** | Notify git provider deploy failed | Event listener | **HIGH** |
| **CreateDnsRecord** | Create DNS record for site domain | SiteService.store() | **HIGH** |
| EnableLaravelQueue | Enable queue workers | LaravelFeature dispatch | MEDIUM |
| DisableLaravelQueue | Disable queue workers | LaravelFeature dispatch | MEDIUM |
| EnableLaravelScheduler | Enable scheduler cron | LaravelFeature dispatch | MEDIUM |
| DisableLaravelScheduler | Disable scheduler cron | LaravelFeature dispatch | MEDIUM |
| EnableLaravelHorizon | Enable Horizon | LaravelFeature dispatch | MEDIUM |
| DisableLaravelHorizon | Disable Horizon | LaravelFeature dispatch | MEDIUM |
| RestartAllSiteQueues | Restart all queues | AutoRestartQueues listener | MEDIUM |
| AnalyzeLaravelFeatures | Detect Laravel features | DeploySite job / Command | MEDIUM |
| CreateDeployment | Create deployment record | (may be in service) | MEDIUM |
| InstallWordpressCron | Setup WordPress cron | DeploySite (WordPress sites) | LOW |
| UpdateSiteTlsSetting | Update TLS settings | SiteService.updateTlsSetting() | LOW |
| CleanupPendingSiteDeployment | Cleanup stuck deployments | DeploySite (delayed) | LOW |
| EnableLaravelInertia | Enable Inertia SSR | LaravelFeature dispatch | LOW |
| DisableLaravelInertia | Disable Inertia SSR | LaravelFeature dispatch | LOW |

---

## DATABASE MODULE ✅ (100% Complete)

| Laravel Job | Go Job | Laravel Trigger | Go Trigger |
|-------------|--------|-----------------|------------|
| InstallDatabase | InstallDatabase | DatabaseService.store() | DatabaseService.CreateDatabase() |
| UninstallDatabase | UninstallDatabase | DatabaseService.destroy() | DatabaseService.DeleteDatabase() |
| InstallDatabaseUser | InstallDatabaseUser | DatabaseUserService.store() | DatabaseUserService.CreateDatabaseUser() |
| UpdateDatabaseUser | UpdateDatabaseUser | DatabaseUserService.update() | DatabaseUserService.UpdateDatabaseUser() |
| UninstallDatabaseUser | UninstallDatabaseUser | DatabaseUserService.destroy() | DatabaseUserService.DeleteDatabaseUser() |
| SyncDatabasesFromServer | SyncDatabases | DatabaseService.sync() | DatabaseService.SyncDatabases() |

---

## GIT MODULE

### Implemented & Triggered ✅

| Laravel Job | Go Job | Laravel Trigger | Go Trigger |
|-------------|--------|-----------------|------------|
| ProcessGitWebhook | ProcessGitWebhook | AppController.webhook() | WebhookHandler.HandleWebhook() |

### Implemented but NOT Triggered ⚠️

| Go Job | Status | Issue |
|--------|--------|-------|
| SyncInstallationRepos | Job exists | No GitHub app installation webhook handler |

---

## BACKUP MODULE ❌ (Not Implemented)

| Laravel Job | Description | Laravel Trigger | Priority |
|-------------|-------------|-----------------|----------|
| **InstallBackup** | Install backup agent on server | BackupService.store() | **HIGH** |
| **DeleteBackup** | Delete backup file | BackupService.destroy() | **HIGH** |
| **RunManualBackup** | Trigger manual backup | BackupService.runManual() | **HIGH** |
| **SyncServerLaunchConfig** | Sync backup config to server | Event listener / Jobs | **HIGH** |

---

## DNS MODULE ❌ (Jobs Not Implemented)

| Laravel Job | Description | Laravel Trigger | Priority |
|-------------|-------------|-----------------|----------|
| **SyncDomainsJob** | Sync domains from DNS provider | DnsProviderService.syncDomains() | MEDIUM |
| **CreateDnsRecord** | Create DNS record via API | SiteService (domain creation) | MEDIUM |

---

## Missing Infrastructure

### Services Missing in Go

| Service | Jobs It Would Trigger |
|---------|----------------------|
| **PhpService** | AddPhpVersion, RemovePhpVersion, UpdateDefault, Install/Uninstall Extension |
| **BackupService** | InstallBackup, DeleteBackup, RunManualBackup, SyncServerLaunchConfig |

### Features Missing in Go

| Feature | Purpose | Jobs Affected |
|---------|---------|---------------|
| **Scheduled Commands** | Background monitoring | UpdateServerConnectivity, CheckDaemonStatus |
| **Event Listeners** | React to state changes | SourceControlDeployment*, RestartAllSiteQueues |
| **Job Chaining** | Sequential execution | WaitForServerToConnect → ProvisionServer |
| **Failure Handlers** | Cleanup on job failure | CleanupFailed* jobs |

---

## Implementation Roadmap

### Phase 1: Critical Server Operations
1. Create **PhpService** with methods:
   - `InstallVersion()` → AddPhpVersionToServer
   - `UninstallVersion()` → RemovePhpVersionFromServer
   - `SetDefault()` → UpdatePhpDefault
   - `InstallExtension()` → InstallPhpExtensionOnServer
   - `UninstallExtension()` → UninstallPhpExtensionOnServer

2. Implement **WaitForServerToConnect** job and add to provisioning chain

3. Implement **CleanupFailedServerProvisioning** for failure handling

### Phase 2: Site Module Completion
1. Add **QueueService** methods:
   - `CreateQueue()` → InstallQueue
   - `DeleteQueue()` → UninstallQueue

2. Implement **SourceControl deployment notifications**:
   - SourceControlDeploymentCreated
   - SourceControlDeploymentCompleted
   - SourceControlDeploymentFailed

3. Add **UninstallSite** trigger to site deletion flow

4. Add **DaemonService.RestartDaemon()** method

### Phase 3: Git Module
1. Implement **SyncInstallationRepositories** trigger for GitHub app webhooks

### Phase 4: Scheduled Commands
1. Create scheduled command for **UpdateServerConnectivity**
2. Create scheduled command for **CheckDaemonStatus**

### Phase 5: Backup Module
1. Create backup module structure
2. Implement BackupService with all 4 jobs
3. Add event listener for SyncServerLaunchConfig

### Phase 6: DNS Jobs
1. Implement **CreateDnsRecord** job
2. Implement **SyncDomainsJob** job
3. Add triggers to DNS services

### Phase 7: Laravel Features & Cleanup
1. Implement Laravel feature detection (AnalyzeLaravelFeatures)
2. Add Enable/Disable jobs for Queue, Scheduler, Horizon, Inertia
3. Implement cleanup jobs for failed operations
4. Add RestartAllSiteQueues listener

---

## File Locations Reference

### Laravel Jobs
```
modules/server/src/Jobs/     - 37 jobs
modules/site/src/Jobs/       - 26 jobs
modules/backup/src/Jobs/     - 4 jobs
modules/dns/src/Jobs/        - 2 jobs
modules/git/src/Jobs/        - 2 jobs
```

### Go Jobs
```
internal/modules/server/jobs/    - 24 jobs
internal/modules/site/jobs/      - 13 jobs
internal/modules/database/jobs/  - 6 jobs
internal/modules/git/jobs/       - 2 jobs
```

### Go Services (where triggers live)
```
internal/modules/server/services/
internal/modules/site/services/
internal/modules/database/services/
internal/modules/git/services/
```
