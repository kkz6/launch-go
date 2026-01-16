# Jobs Implementation Status: Laravel vs Go

This document provides a comprehensive comparison of jobs between the Laravel and Go implementations, including both job definitions and their triggers.

## Executive Summary

| Module | Laravel Jobs | Go Jobs | Triggers Working | Coverage |
|--------|-------------|---------|------------------|----------|
| Server | 37 | 29 | 23/30 | 77% |
| Site | 26 | 24 | 19/25 | 76% |
| Database | 6 | 6 | 7/7 | **100%** ✅ |
| Git | 2 | 2 | 2/2 | **100%** ✅ |
| Backup | 4 | 3 | 3/4 | **75%** ✅ |
| DNS | 2 | 0 | 2/2 | **Sync** ✅ |
| **Total** | **77** | **64** | **56/70** | **80%** |

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

### Implemented & Triggered ✅ (Recently Fixed)

| Go Job | Status | Go Trigger |
|--------|--------|------------|
| RestartDaemon | Job exists | DaemonService.RestartDaemon() |

### Implemented & Triggered ✅ (PHP Management)

| Go Job | Description | Go Trigger |
|--------|-------------|------------|
| SetDefaultPhp | Set default PHP version | InstalledServiceService.SetDefaultPhpVersion() |
| InstallPhpExtension | Install PHP extension | InstalledServiceService.InstallPhpExtension() |
| UninstallPhpExtension | Remove PHP extension | InstalledServiceService.UninstallPhpExtension() |

Note: PHP version installation/removal is handled through the generic AddService/RemoveService jobs.

### Implemented & Triggered ✅ (Job Chaining)

| Go Job | Description | Go Trigger |
|--------|-------------|------------|
| WaitForServerToConnect | Wait for SSH connection | CreateOnProviderJob.dispatchWaitForConnection() |
| CleanupFailedProvisioning | Cleanup on provisioning failure | WaitForServerToConnectJob.Failed() / ProvisionServerJob.Failed() |

**Job Chain**: CreateOnProvider → WaitForServerToConnect → ProvisionServer

**Failure Handler Chain**: WaitForServerToConnect (fail) → CleanupFailedProvisioning
                          ProvisionServer (fail) → CleanupFailedProvisioning

### NOT Implemented ❌

| Laravel Job | Description | Laravel Trigger | Priority |
|-------------|-------------|-----------------|----------|
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

### Implemented & Triggered ✅ (Recently Fixed)

| Go Job | Status | Go Trigger |
|--------|--------|------------|
| InstallQueue | Job exists | QueueService.Create() |
| UninstallQueue | Job exists | QueueService.Delete() |
| UninstallSite | Job exists | SiteService.Delete() |

### Already Implemented (in Deploy Job) ✅

| Laravel Job | Description | Go Implementation |
|-------------|-------------|-------------------|
| SourceControlDeploymentCreated | Notify git provider deploy started | `DeployJob.createProviderDeployment()` |
| SourceControlDeploymentCompleted | Notify git provider deploy succeeded | `DeployJob.updateProviderDeploymentStatus(Success)` |
| SourceControlDeploymentFailed | Notify git provider deploy failed | `DeployJob.updateProviderDeploymentStatus(Failure)` |

### Already Implemented (Synchronous) ✅

| Laravel Job | Description | Go Implementation |
|-------------|-------------|-------------------|
| CreateDnsRecord | Create DNS record for site domain | `SiteService.handleDnsRecordCreation()` via `DnsRecordService.CreateRecordForSite()` |

### Implemented & Triggered ✅ (Laravel Feature Jobs)

| Go Job | Description | Go Trigger |
|--------|-------------|------------|
| EnableLaravelScheduler | Enable scheduler cron | SiteService / API endpoint |
| DisableLaravelScheduler | Disable scheduler cron | SiteService / API endpoint |
| EnableLaravelQueue | Enable queue workers | SiteService / API endpoint |
| DisableLaravelQueue | Disable queue workers | SiteService / API endpoint |
| EnableLaravelHorizon | Enable Horizon | SiteService / API endpoint |
| DisableLaravelHorizon | Disable Horizon | SiteService / API endpoint |
| EnableLaravelInertia | Enable Inertia SSR | SiteService / API endpoint |
| DisableLaravelInertia | Disable Inertia SSR | SiteService / API endpoint |
| AnalyzeLaravelFeatures | Detect Laravel features | DeploySite job / API endpoint |
| RestartAllSiteQueues | Restart all site queues | DeploySite (autoRestart) / API endpoint |

### Implemented & Triggered ✅ (WordPress Jobs)

| Go Job | Description | Go Trigger |
|--------|-------------|------------|
| InstallWordpressCron | Setup WordPress cron | DeploySite (WordPress sites) / API endpoint |

### NOT Implemented ❌

| Laravel Job | Description | Laravel Trigger | Priority |
|-------------|-------------|-----------------|----------|
| CreateDeployment | Create deployment record | (may be in service) | MEDIUM |
| UpdateSiteTlsSetting | Update TLS settings | SiteService.updateTlsSetting() | LOW |
| CleanupPendingSiteDeployment | Cleanup stuck deployments | DeploySite (delayed) | LOW |

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

### Implemented & Triggered ✅

| Go Job | Status | Go Trigger |
|--------|--------|------------|
| SyncInstallationRepos | Job exists | WebhookHandler.handleRepositoriesChanged() |

---

## BACKUP MODULE ✅ (75% Complete)

### Implemented & Triggered ✅

| Laravel Job | Go Job | Laravel Trigger | Go Trigger |
|-------------|--------|-----------------|------------|
| InstallBackup | InstallBackup | BackupService.store() | BackupService.CreateBackup() |
| DeleteBackup | DeleteBackup | BackupService.destroy() | BackupService.DeleteBackup() |
| RunManualBackup | RunManualBackup | BackupService.runManual() | BackupService.RunBackup() |

### NOT Implemented ❌

| Laravel Job | Description | Laravel Trigger | Priority |
|-------------|-------------|-----------------|----------|
| **SyncServerLaunchConfig** | Sync backup config to server | Event listener / Jobs | **MEDIUM** |

---

## DNS MODULE ✅ (Synchronous Implementation)

The DNS functionality is implemented synchronously in services (not as background jobs):

| Functionality | Go Implementation | Notes |
|--------------|-------------------|-------|
| SyncDomainRecords | DomainService.SyncDomainRecords() | Syncs records from DNS provider |
| CreateDnsRecord | DnsRecordService.CreateRecord() | Creates DNS record via API |
| CreateRecordForSite | DnsRecordService.CreateRecordForSite() | Creates A record for site |

The synchronous implementation is appropriate for DNS operations as they are quick API calls.

---

## Missing Infrastructure

### Services Missing in Go

| Service | Jobs It Would Trigger | Status |
|---------|----------------------|--------|
| ~~**PhpService**~~ | ~~AddPhpVersion, RemovePhpVersion, UpdateDefault, Install/Uninstall Extension~~ | ✅ Implemented in InstalledServiceService |
| ~~**BackupService**~~ | ~~InstallBackup, DeleteBackup, RunManualBackup~~ | ✅ Implemented |
| **BackupService** | SyncServerLaunchConfig | Remaining |

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

### Phase 2: Site Module Completion ✅ COMPLETED
1. ~~Add **QueueService** methods:~~ ✅
   - ~~`CreateQueue()` → InstallQueue~~ ✅ (QueueService.Create())
   - ~~`DeleteQueue()` → UninstallQueue~~ ✅ (QueueService.Delete())

2. ~~Implement **SourceControl deployment notifications**:~~ ✅ (Already in DeployJob)
   - ~~SourceControlDeploymentCreated~~ ✅
   - ~~SourceControlDeploymentCompleted~~ ✅
   - ~~SourceControlDeploymentFailed~~ ✅

3. ~~Add **UninstallSite** trigger to site deletion flow~~ ✅ (SiteService.Delete())

4. ~~Add **DaemonService.RestartDaemon()** method~~ ✅

### Phase 3: Git Module ✅ COMPLETED
1. ~~Implement **SyncInstallationRepositories** trigger for GitHub app webhooks~~ ✅

### Phase 4: Scheduled Commands
1. Create scheduled command for **UpdateServerConnectivity**
2. Create scheduled command for **CheckDaemonStatus**

### Phase 5: Backup Module ✅ COMPLETED
1. ~~Create backup module structure~~ ✅
2. ~~Implement BackupService jobs~~ ✅ (InstallBackup, DeleteBackup, RunManualBackup)
3. Add event listener for SyncServerLaunchConfig (remaining)

### Phase 6: DNS Jobs ✅ COMPLETED
1. ~~Implement DNS record management~~ ✅ (Synchronous implementation in DnsRecordService)
2. ~~Implement domain syncing~~ ✅ (Synchronous implementation in DomainService)

### Phase 7: Laravel Features & Cleanup ✅ COMPLETED
1. ~~Implement Laravel feature detection (AnalyzeLaravelFeatures)~~ ✅ COMPLETED
2. ~~Add Enable/Disable jobs for Queue, Scheduler, Horizon~~ ✅ COMPLETED
3. Implement cleanup jobs for failed operations
4. ~~Add RestartAllSiteQueues job~~ ✅ COMPLETED
5. ~~Add Enable/Disable jobs for Inertia~~ ✅ COMPLETED
6. ~~Add InstallWordpressCron job~~ ✅ COMPLETED

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
internal/modules/server/jobs/    - 29 jobs
internal/modules/site/jobs/      - 24 jobs
internal/modules/database/jobs/  - 6 jobs
internal/modules/git/jobs/       - 2 jobs
internal/modules/backup/jobs/    - 3 jobs
```

### Go Services (where triggers live)
```
internal/modules/server/services/
internal/modules/site/services/
internal/modules/database/services/
internal/modules/git/services/
internal/modules/backup/services/
```
