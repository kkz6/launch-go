# Enum Refactoring Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Reorganize Go enum implementations to follow Go idioms, consolidate patterns, remove over-engineering, and ensure consistency across all modules.

**Architecture:** Rename `enums/` folders to `types/` (Go idiomatic). Remove the registry system (over-engineered). Standardize all enum implementations with a consistent pattern. Each module gets a single `types.go` file for simple enums, with separate files only for complex enums with many methods.

**Tech Stack:** Go 1.21+, GORM (database/sql interfaces)

---

## Summary of Changes

### 1. Folder Naming: `enums/` → `types/`
Go doesn't have enums - we use typed constants. The idiomatic package name is `types` or keeping them in the module's root package. We'll use `types/` to maintain separation.

### 2. Remove Over-Engineered Registry
The `internal/pkg/enums/registry.go` with its builder pattern and global singleton is not used meaningfully. Remove it.

### 3. Simplify Base Package
Keep only the essential helpers in `internal/pkg/types/`:
- `ScanString()` / `ValueString()` for database scanning
- `StringEnum` / `LabeledEnum` interfaces

### 4. Consistent Enum Pattern
Every enum follows this pattern:
```go
type Status string

const (
    StatusActive   Status = "active"
    StatusInactive Status = "inactive"
)

// All values
var allStatuses = []Status{StatusActive, StatusInactive}

func AllStatuses() []Status { return allStatuses }

// Methods
func (s Status) String() string { return string(s) }
func (s Status) IsValid() bool  { /* switch */ }
func (s Status) Label() string  { /* map lookup */ }

// Database
func (s *Status) Scan(value interface{}) error { return types.ScanString(s, value) }
func (s Status) Value() (driver.Value, error)  { return types.ValueString(s) }
```

### 5. File Organization Per Module
- Small enums (< 50 lines each): Consolidate into `types/types.go`
- Large enums (many methods): Separate file like `types/software.go`

---

## Task 1: Create New Base Types Package

**Files:**
- Create: `internal/pkg/types/types.go`
- Create: `internal/pkg/types/scan.go`

**Step 1: Create the base types package**

```go
// internal/pkg/types/types.go
package types

// StringEnum is the interface that all string-based enums should implement.
type StringEnum interface {
	~string
	String() string
	IsValid() bool
}

// LabeledEnum extends StringEnum with a human-readable label.
type LabeledEnum interface {
	StringEnum
	Label() string
}
```

**Step 2: Create the database scanning helpers**

```go
// internal/pkg/types/scan.go
package types

import (
	"database/sql/driver"
	"fmt"
)

// ScanString is a helper for implementing sql.Scanner for string enums.
func ScanString[T ~string](dest *T, value interface{}) error {
	if value == nil {
		*dest = ""
		return nil
	}

	switch v := value.(type) {
	case string:
		*dest = T(v)
	case []byte:
		*dest = T(string(v))
	default:
		return fmt.Errorf("cannot scan type %T into enum", value)
	}

	return nil
}

// ValueString is a helper for implementing driver.Valuer for string enums.
func ValueString[T ~string](v T) (driver.Value, error) {
	return string(v), nil
}

// ScanInt is a helper for implementing sql.Scanner for int enums.
func ScanInt[T ~int](dest *T, value interface{}) error {
	if value == nil {
		*dest = 0
		return nil
	}

	switch v := value.(type) {
	case int64:
		*dest = T(v)
	case int:
		*dest = T(v)
	default:
		return fmt.Errorf("cannot scan type %T into int enum", value)
	}

	return nil
}

// ValueInt is a helper for implementing driver.Valuer for int enums.
func ValueInt[T ~int](v T) (driver.Value, error) {
	return int64(v), nil
}
```

**Step 3: Run tests to verify compilation**

Run: `cd /Users/karthick/launch-app/launch-go && go build ./internal/pkg/types/...`
Expected: No errors

**Step 4: Commit**

```bash
git add internal/pkg/types/
git commit -m "Add new base types package for enum helpers"
```

---

## Task 2: Refactor Server Module Types

**Files:**
- Create: `internal/modules/server/types/types.go` (consolidate small enums)
- Create: `internal/modules/server/types/software.go` (keep separate - large)
- Create: `internal/modules/server/types/provision_step.go` (keep separate - has template logic)
- Create: `internal/modules/server/types/cron_schedule.go` (keep separate - has expression logic)
- Delete: `internal/modules/server/enums/` (after migration)

**Step 1: Create consolidated types.go with small enums**

This file will contain:
- ServerStatus
- ServerProvider
- ServerType
- ServerFeature
- ServiceType
- ServiceStatus
- ServiceOption
- ProcessManager
- OperatingSystem
- RuleAction

```go
// internal/modules/server/types/types.go
package types

import (
	"database/sql/driver"

	basetypes "github.com/kkz6/launch-go/internal/pkg/types"
)

// ============================================================================
// ServerStatus
// ============================================================================

type ServerStatus string

const (
	ServerStatusNew          ServerStatus = "new"
	ServerStatusStarting     ServerStatus = "starting"
	ServerStatusProvisioning ServerStatus = "provisioning"
	ServerStatusRunning      ServerStatus = "running"
	ServerStatusPaused       ServerStatus = "paused"
	ServerStatusStopped      ServerStatus = "stopped"
	ServerStatusDeleting     ServerStatus = "deleting"
	ServerStatusArchived     ServerStatus = "archived"
	ServerStatusUnknown      ServerStatus = "unknown"
	ServerStatusFailed       ServerStatus = "failed"
)

var allServerStatuses = []ServerStatus{
	ServerStatusNew, ServerStatusStarting, ServerStatusProvisioning,
	ServerStatusRunning, ServerStatusPaused, ServerStatusStopped,
	ServerStatusDeleting, ServerStatusArchived, ServerStatusUnknown, ServerStatusFailed,
}

func AllServerStatuses() []ServerStatus { return allServerStatuses }

func (s ServerStatus) String() string { return string(s) }

func (s ServerStatus) Label() string {
	switch s {
	case ServerStatusNew:
		return "Connecting"
	case ServerStatusStarting:
		return "Starting"
	case ServerStatusProvisioning:
		return "Provisioning"
	case ServerStatusRunning:
		return "Running"
	case ServerStatusPaused:
		return "Paused"
	case ServerStatusStopped:
		return "Stopped"
	case ServerStatusDeleting:
		return "Deleting"
	case ServerStatusArchived:
		return "Archived"
	case ServerStatusUnknown:
		return "Unknown"
	case ServerStatusFailed:
		return "Failed"
	default:
		return "Unknown"
	}
}

func (s ServerStatus) IsValid() bool {
	switch s {
	case ServerStatusNew, ServerStatusStarting, ServerStatusProvisioning,
		ServerStatusRunning, ServerStatusPaused, ServerStatusStopped,
		ServerStatusDeleting, ServerStatusArchived, ServerStatusUnknown, ServerStatusFailed:
		return true
	}
	return false
}

func (s ServerStatus) IsActive() bool {
	return s == ServerStatusRunning || s == ServerStatusProvisioning || s == ServerStatusStarting
}

func (s ServerStatus) IsTerminal() bool {
	return s == ServerStatusFailed || s == ServerStatusArchived || s == ServerStatusDeleting
}

func (s *ServerStatus) Scan(value interface{}) error { return basetypes.ScanString(s, value) }
func (s ServerStatus) Value() (driver.Value, error)  { return basetypes.ValueString(s) }

// ============================================================================
// ServerProvider
// ============================================================================

type ServerProvider string

const (
	ServerProviderDigitalOcean ServerProvider = "digitalocean"
	ServerProviderHetzner      ServerProvider = "hetzner"
	ServerProviderLinode       ServerProvider = "linode"
	ServerProviderVultr        ServerProvider = "vultr"
	ServerProviderAWS          ServerProvider = "aws"
	ServerProviderCustom       ServerProvider = "custom_server"
)

var allServerProviders = []ServerProvider{
	ServerProviderDigitalOcean, ServerProviderHetzner, ServerProviderLinode,
	ServerProviderVultr, ServerProviderAWS, ServerProviderCustom,
}

func AllServerProviders() []ServerProvider { return allServerProviders }

func (p ServerProvider) String() string { return string(p) }

func (p ServerProvider) Label() string {
	switch p {
	case ServerProviderDigitalOcean:
		return "DigitalOcean"
	case ServerProviderHetzner:
		return "Hetzner Cloud"
	case ServerProviderLinode:
		return "Linode"
	case ServerProviderVultr:
		return "Vultr"
	case ServerProviderAWS:
		return "AWS"
	case ServerProviderCustom:
		return "Custom Server"
	default:
		return string(p)
	}
}

func (p ServerProvider) IsValid() bool {
	switch p {
	case ServerProviderDigitalOcean, ServerProviderHetzner, ServerProviderLinode,
		ServerProviderVultr, ServerProviderAWS, ServerProviderCustom:
		return true
	}
	return false
}

func (p ServerProvider) IsCloud() bool {
	return p != ServerProviderCustom
}

func (p ServerProvider) GetDefaultUsername(os string) string {
	switch p {
	case ServerProviderDigitalOcean, ServerProviderVultr:
		return "root"
	case ServerProviderHetzner:
		return "root"
	case ServerProviderLinode:
		return "root"
	case ServerProviderAWS:
		return "ubuntu"
	default:
		return "root"
	}
}

func (p *ServerProvider) Scan(value interface{}) error { return basetypes.ScanString(p, value) }
func (p ServerProvider) Value() (driver.Value, error)  { return basetypes.ValueString(p) }

// ============================================================================
// ServerType
// ============================================================================

type ServerType string

const (
	ServerTypePhp      ServerType = "php"
	ServerTypeDatabase ServerType = "database"
)

var allServerTypes = []ServerType{ServerTypePhp, ServerTypeDatabase}

func AllServerTypes() []ServerType { return allServerTypes }

func (t ServerType) String() string { return string(t) }

func (t ServerType) Label() string {
	switch t {
	case ServerTypePhp:
		return "PHP Application Server"
	case ServerTypeDatabase:
		return "Database Server"
	default:
		return string(t)
	}
}

func (t ServerType) IsValid() bool {
	return t == ServerTypePhp || t == ServerTypeDatabase
}

func (t ServerType) GetFeatures() []ServerFeature {
	switch t {
	case ServerTypePhp:
		return []ServerFeature{
			ServerFeatureSites, ServerFeatureSSLCertificates, ServerFeaturePhpManagement,
			ServerFeatureComposer, ServerFeatureDatabaseManagement, ServerFeatureQueueWorkers,
			ServerFeatureDaemons, ServerFeatureScheduler, ServerFeatureRedis, ServerFeatureBackups,
			ServerFeatureServices,
		}
	case ServerTypeDatabase:
		return []ServerFeature{
			ServerFeatureDatabaseManagement, ServerFeatureBackups, ServerFeatureServices,
		}
	default:
		return nil
	}
}

func (t ServerType) HasFeature(f ServerFeature) bool {
	for _, feature := range t.GetFeatures() {
		if feature == f {
			return true
		}
	}
	return false
}

func (t ServerType) GetProcessManager() ProcessManager {
	if t == ServerTypePhp {
		return ProcessManagerSupervisor
	}
	return ProcessManagerNone
}

func (t *ServerType) Scan(value interface{}) error { return basetypes.ScanString(t, value) }
func (t ServerType) Value() (driver.Value, error)  { return basetypes.ValueString(t) }

// ============================================================================
// ServerFeature
// ============================================================================

type ServerFeature string

const (
	ServerFeatureSites              ServerFeature = "sites"
	ServerFeatureSSLCertificates    ServerFeature = "ssl_certificates"
	ServerFeaturePhpManagement      ServerFeature = "php_management"
	ServerFeatureComposer           ServerFeature = "composer"
	ServerFeatureDatabaseManagement ServerFeature = "database_management"
	ServerFeatureQueueWorkers       ServerFeature = "queue_workers"
	ServerFeatureDaemons            ServerFeature = "daemons"
	ServerFeatureScheduler          ServerFeature = "scheduler"
	ServerFeatureRedis              ServerFeature = "redis"
	ServerFeatureBackups            ServerFeature = "backups"
	ServerFeatureServices           ServerFeature = "services"
)

var allServerFeatures = []ServerFeature{
	ServerFeatureSites, ServerFeatureSSLCertificates, ServerFeaturePhpManagement,
	ServerFeatureComposer, ServerFeatureDatabaseManagement, ServerFeatureQueueWorkers,
	ServerFeatureDaemons, ServerFeatureScheduler, ServerFeatureRedis, ServerFeatureBackups,
	ServerFeatureServices,
}

func AllServerFeatures() []ServerFeature { return allServerFeatures }

func (f ServerFeature) String() string { return string(f) }

func (f ServerFeature) Label() string {
	switch f {
	case ServerFeatureSites:
		return "Sites"
	case ServerFeatureSSLCertificates:
		return "SSL Certificates"
	case ServerFeaturePhpManagement:
		return "PHP Management"
	case ServerFeatureComposer:
		return "Composer"
	case ServerFeatureDatabaseManagement:
		return "Database Management"
	case ServerFeatureQueueWorkers:
		return "Queue Workers"
	case ServerFeatureDaemons:
		return "Daemons"
	case ServerFeatureScheduler:
		return "Scheduler"
	case ServerFeatureRedis:
		return "Redis"
	case ServerFeatureBackups:
		return "Backups"
	case ServerFeatureServices:
		return "Services"
	default:
		return string(f)
	}
}

func (f ServerFeature) IsValid() bool {
	switch f {
	case ServerFeatureSites, ServerFeatureSSLCertificates, ServerFeaturePhpManagement,
		ServerFeatureComposer, ServerFeatureDatabaseManagement, ServerFeatureQueueWorkers,
		ServerFeatureDaemons, ServerFeatureScheduler, ServerFeatureRedis, ServerFeatureBackups,
		ServerFeatureServices:
		return true
	}
	return false
}

func (f ServerFeature) NavigationKey() string {
	return string(f)
}

// ============================================================================
// ServiceType
// ============================================================================

type ServiceType string

const (
	ServiceTypePhp         ServiceType = "php"
	ServiceTypeMySql       ServiceType = "mysql"
	ServiceTypePostgreSql  ServiceType = "postgresql"
	ServiceTypeSupervisor  ServiceType = "process_manager"
	ServiceTypeRedis       ServiceType = "memory_database"
	ServiceTypeCaddy       ServiceType = "webserver"
	ServiceTypeComposer    ServiceType = "package_manager"
	ServiceTypeNode        ServiceType = "node"
	ServiceTypeBun         ServiceType = "bun"
	ServiceTypeLaunchAgent ServiceType = "launch_agent"
)

var allServiceTypes = []ServiceType{
	ServiceTypePhp, ServiceTypeMySql, ServiceTypePostgreSql, ServiceTypeSupervisor,
	ServiceTypeRedis, ServiceTypeCaddy, ServiceTypeComposer, ServiceTypeNode,
	ServiceTypeBun, ServiceTypeLaunchAgent,
}

func AllServiceTypes() []ServiceType { return allServiceTypes }

func (t ServiceType) String() string { return string(t) }

func (t ServiceType) Label() string {
	switch t {
	case ServiceTypePhp:
		return "PHP"
	case ServiceTypeMySql:
		return "MySQL"
	case ServiceTypePostgreSql:
		return "PostgreSQL"
	case ServiceTypeSupervisor:
		return "Supervisor"
	case ServiceTypeRedis:
		return "Redis"
	case ServiceTypeCaddy:
		return "Caddy"
	case ServiceTypeComposer:
		return "Composer"
	case ServiceTypeNode:
		return "Node.js"
	case ServiceTypeBun:
		return "Bun"
	case ServiceTypeLaunchAgent:
		return "Launch Agent"
	default:
		return string(t)
	}
}

func (t ServiceType) IsValid() bool {
	switch t {
	case ServiceTypePhp, ServiceTypeMySql, ServiceTypePostgreSql, ServiceTypeSupervisor,
		ServiceTypeRedis, ServiceTypeCaddy, ServiceTypeComposer, ServiceTypeNode,
		ServiceTypeBun, ServiceTypeLaunchAgent:
		return true
	}
	return false
}

func (t ServiceType) IsDatabase() bool {
	return t == ServiceTypeMySql || t == ServiceTypePostgreSql
}

func (t ServiceType) GetDatabasePort() int {
	switch t {
	case ServiceTypeMySql:
		return 3306
	case ServiceTypePostgreSql:
		return 5432
	default:
		return 0
	}
}

func (t ServiceType) GetDatabaseConnection() string {
	switch t {
	case ServiceTypeMySql:
		return "mysql"
	case ServiceTypePostgreSql:
		return "pgsql"
	default:
		return ""
	}
}

func (t *ServiceType) Scan(value interface{}) error { return basetypes.ScanString(t, value) }
func (t ServiceType) Value() (driver.Value, error)  { return basetypes.ValueString(t) }

// ============================================================================
// ServiceStatus
// ============================================================================

type ServiceStatus string

const (
	ServiceStatusPending    ServiceStatus = "pending"
	ServiceStatusInstalling ServiceStatus = "installing"
	ServiceStatusFailed     ServiceStatus = "failed"
	ServiceStatusInstalled  ServiceStatus = "installed"
	ServiceStatusStopped    ServiceStatus = "stopped"
	ServiceStatusRunning    ServiceStatus = "running"
)

var allServiceStatuses = []ServiceStatus{
	ServiceStatusPending, ServiceStatusInstalling, ServiceStatusFailed,
	ServiceStatusInstalled, ServiceStatusStopped, ServiceStatusRunning,
}

func AllServiceStatuses() []ServiceStatus { return allServiceStatuses }

func (s ServiceStatus) String() string { return string(s) }

func (s ServiceStatus) Label() string {
	switch s {
	case ServiceStatusPending:
		return "Pending"
	case ServiceStatusInstalling:
		return "Installing"
	case ServiceStatusFailed:
		return "Failed"
	case ServiceStatusInstalled:
		return "Installed"
	case ServiceStatusStopped:
		return "Stopped"
	case ServiceStatusRunning:
		return "Running"
	default:
		return string(s)
	}
}

func (s ServiceStatus) IsValid() bool {
	switch s {
	case ServiceStatusPending, ServiceStatusInstalling, ServiceStatusFailed,
		ServiceStatusInstalled, ServiceStatusStopped, ServiceStatusRunning:
		return true
	}
	return false
}

func (s *ServiceStatus) Scan(value interface{}) error { return basetypes.ScanString(s, value) }
func (s ServiceStatus) Value() (driver.Value, error)  { return basetypes.ValueString(s) }

// ============================================================================
// ServiceOption
// ============================================================================

type ServiceOption string

const (
	ServiceOptionStart   ServiceOption = "start"
	ServiceOptionStop    ServiceOption = "stop"
	ServiceOptionRestart ServiceOption = "restart"
	ServiceOptionRemove  ServiceOption = "remove"
	ServiceOptionStatus  ServiceOption = "status"
)

var allServiceOptions = []ServiceOption{
	ServiceOptionStart, ServiceOptionStop, ServiceOptionRestart,
	ServiceOptionRemove, ServiceOptionStatus,
}

func AllServiceOptions() []ServiceOption { return allServiceOptions }

func (o ServiceOption) String() string { return string(o) }

func (o ServiceOption) Label() string {
	switch o {
	case ServiceOptionStart:
		return "Start"
	case ServiceOptionStop:
		return "Stop"
	case ServiceOptionRestart:
		return "Restart"
	case ServiceOptionRemove:
		return "Remove"
	case ServiceOptionStatus:
		return "Status"
	default:
		return string(o)
	}
}

func (o ServiceOption) IsValid() bool {
	switch o {
	case ServiceOptionStart, ServiceOptionStop, ServiceOptionRestart,
		ServiceOptionRemove, ServiceOptionStatus:
		return true
	}
	return false
}

// ============================================================================
// ProcessManager
// ============================================================================

type ProcessManager string

const (
	ProcessManagerSupervisor ProcessManager = "supervisor"
	ProcessManagerNone       ProcessManager = "none"
)

var allProcessManagers = []ProcessManager{ProcessManagerSupervisor, ProcessManagerNone}

func AllProcessManagers() []ProcessManager { return allProcessManagers }

func (p ProcessManager) String() string { return string(p) }

func (p ProcessManager) Label() string {
	switch p {
	case ProcessManagerSupervisor:
		return "Supervisor"
	case ProcessManagerNone:
		return "None"
	default:
		return string(p)
	}
}

func (p ProcessManager) IsValid() bool {
	return p == ProcessManagerSupervisor || p == ProcessManagerNone
}

func (p ProcessManager) ServiceName() string {
	if p == ProcessManagerSupervisor {
		return "supervisor"
	}
	return ""
}

// ============================================================================
// OperatingSystem
// ============================================================================

type OperatingSystem string

const (
	OperatingSystemUbuntu20 OperatingSystem = "ubuntu_20"
	OperatingSystemUbuntu22 OperatingSystem = "ubuntu_22"
	OperatingSystemUbuntu24 OperatingSystem = "ubuntu_24"
)

var allOperatingSystems = []OperatingSystem{
	OperatingSystemUbuntu20, OperatingSystemUbuntu22, OperatingSystemUbuntu24,
}

func AllOperatingSystems() []OperatingSystem { return allOperatingSystems }

func (o OperatingSystem) String() string { return string(o) }

func (o OperatingSystem) Label() string {
	switch o {
	case OperatingSystemUbuntu20:
		return "Ubuntu 20.04"
	case OperatingSystemUbuntu22:
		return "Ubuntu 22.04"
	case OperatingSystemUbuntu24:
		return "Ubuntu 24.04"
	default:
		return string(o)
	}
}

func (o OperatingSystem) IsValid() bool {
	switch o {
	case OperatingSystemUbuntu20, OperatingSystemUbuntu22, OperatingSystemUbuntu24:
		return true
	}
	return false
}

func (o *OperatingSystem) Scan(value interface{}) error { return basetypes.ScanString(o, value) }
func (o OperatingSystem) Value() (driver.Value, error)  { return basetypes.ValueString(o) }

// ============================================================================
// RuleAction (Firewall)
// ============================================================================

type RuleAction string

const (
	RuleActionAllow  RuleAction = "allow"
	RuleActionDeny   RuleAction = "deny"
	RuleActionReject RuleAction = "reject"
)

var allRuleActions = []RuleAction{RuleActionAllow, RuleActionDeny, RuleActionReject}

func AllRuleActions() []RuleAction { return allRuleActions }

func (a RuleAction) String() string { return string(a) }

func (a RuleAction) Label() string {
	switch a {
	case RuleActionAllow:
		return "Allow"
	case RuleActionDeny:
		return "Deny"
	case RuleActionReject:
		return "Reject"
	default:
		return string(a)
	}
}

func (a RuleAction) IsValid() bool {
	switch a {
	case RuleActionAllow, RuleActionDeny, RuleActionReject:
		return true
	}
	return false
}

func (a *RuleAction) Scan(value interface{}) error { return basetypes.ScanString(a, value) }
func (a RuleAction) Value() (driver.Value, error)  { return basetypes.ValueString(a) }
```

**Step 2: Move Software to separate file (large enum with many methods)**

Copy existing `internal/modules/server/enums/software.go` to `internal/modules/server/types/software.go` and update imports.

**Step 3: Move ProvisionStep to separate file**

Copy existing `internal/modules/server/enums/provision_step.go` to `internal/modules/server/types/provision_step.go` and update imports.

**Step 4: Move CronSchedule to separate file**

Copy existing `internal/modules/server/enums/cron_schedule.go` to `internal/modules/server/types/cron_schedule.go` and update imports.

**Step 5: Update all imports across the codebase**

Change all occurrences of:
- `"github.com/kkz6/launch-go/internal/modules/server/enums"` → `"github.com/kkz6/launch-go/internal/modules/server/types"`
- `enums.ServerStatus` → `types.ServerStatus` (or use alias if needed)

**Step 6: Run tests**

Run: `cd /Users/karthick/launch-app/launch-go && go build ./...`
Expected: No errors

**Step 7: Delete old enums folder**

Run: `rm -rf internal/modules/server/enums/`

**Step 8: Commit**

```bash
git add -A
git commit -m "Refactor server module: rename enums/ to types/"
```

---

## Task 3: Refactor Site Module Types

**Files:**
- Create: `internal/modules/site/types/types.go` (consolidate all site enums)
- Create: `internal/modules/site/types/site_file.go` (keep separate - has path logic)
- Delete: `internal/modules/site/enums/` (after migration)

**Step 1: Create consolidated types.go**

This file will contain:
- SiteType
- SiteStatus
- DeploymentStatus
- TLSSetting
- RedirectMode
- CommandStatus
- QueueStatus
- LaravelFeature
- CertificateType
- PhpVersion

Follow the same pattern as server module.

**Step 2: Move SiteFileType to separate file**

Keep file path logic in its own file.

**Step 3: Update all imports**

Change all occurrences of:
- `"github.com/kkz6/launch-go/internal/modules/site/enums"` → `"github.com/kkz6/launch-go/internal/modules/site/types"`

**Step 4: Run tests**

Run: `cd /Users/karthick/launch-app/launch-go && go build ./...`

**Step 5: Delete old enums folder**

**Step 6: Commit**

```bash
git add -A
git commit -m "Refactor site module: rename enums/ to types/"
```

---

## Task 4: Refactor Auth Module Types

**Files:**
- Create: `internal/modules/auth/types/types.go`
- Delete: `internal/modules/auth/enums/` (after migration)

**Step 1: Create types.go with TeamRole**

```go
// internal/modules/auth/types/types.go
package types

import (
	"database/sql/driver"

	basetypes "github.com/kkz6/launch-go/internal/pkg/types"
)

type TeamRole string

const (
	TeamRoleOwner  TeamRole = "owner"
	TeamRoleAdmin  TeamRole = "admin"
	TeamRoleEditor TeamRole = "editor"
	TeamRoleMember TeamRole = "member"
)

var allTeamRoles = []TeamRole{TeamRoleOwner, TeamRoleAdmin, TeamRoleEditor, TeamRoleMember}

func AllTeamRoles() []TeamRole { return allTeamRoles }

func (r TeamRole) String() string { return string(r) }

func (r TeamRole) Label() string {
	switch r {
	case TeamRoleOwner:
		return "Owner"
	case TeamRoleAdmin:
		return "Admin"
	case TeamRoleEditor:
		return "Editor"
	case TeamRoleMember:
		return "Member"
	default:
		return string(r)
	}
}

func (r TeamRole) IsValid() bool {
	switch r {
	case TeamRoleOwner, TeamRoleAdmin, TeamRoleEditor, TeamRoleMember:
		return true
	}
	return false
}

func (r TeamRole) CanManageTeam() bool {
	return r == TeamRoleOwner || r == TeamRoleAdmin
}

func (r TeamRole) CanManageMembers() bool {
	return r == TeamRoleOwner || r == TeamRoleAdmin
}

func (r *TeamRole) Scan(value interface{}) error { return basetypes.ScanString(r, value) }
func (r TeamRole) Value() (driver.Value, error)  { return basetypes.ValueString(r) }
```

**Step 2: Update imports and delete old folder**

**Step 3: Commit**

```bash
git add -A
git commit -m "Refactor auth module: rename enums/ to types/"
```

---

## Task 5: Refactor Remaining Modules

Apply the same pattern to:

1. **DNS Module** (`internal/modules/dns/`)
   - DnsProvider, RecordType, SyncStatus

2. **Backup Module** (`internal/modules/backup/`)
   - BackupJobStatus, StorageDriver

3. **Billing Module** (`internal/modules/billing/`)
   - PlanInterval, SubscriptionStatus, OrderStatus, UserRole, WebhookEventType

4. **Git Module** (`internal/modules/git/`)
   - GitProviderType, AccountType, RepositorySelection, WebhookEventType

5. **Notification Module** (`internal/modules/notification/`)
   - ChannelType, NotificationType

6. **Script Module** (`internal/modules/script/`)
   - RunAsUser

7. **Database Module** (`internal/modules/database/`)
   - (Currently empty, create placeholder if needed)

For each module:
1. Create `types/types.go`
2. Update all imports
3. Delete old `enums/` folder
4. Commit

---

## Task 6: Clean Up Base Package

**Files:**
- Delete: `internal/pkg/enums/registry.go`
- Delete: `internal/pkg/enums/registry_test.go`
- Delete: `internal/pkg/enums/set.go`
- Delete: `internal/pkg/enums/scanner.go`
- Delete: `internal/pkg/enums/scanner_test.go`
- Keep/Move: `internal/pkg/enums/base.go` → `internal/pkg/types/` (if not already there)
- Update: `internal/pkg/dto/enum.go` to use new types package

**Step 1: Remove unused registry and complex helpers**

The registry pattern is over-engineered and not used. Remove it.

**Step 2: Update dto/enum.go imports**

Change import from `internal/pkg/enums` to `internal/pkg/types`.

**Step 3: Delete old enums folder**

Run: `rm -rf internal/pkg/enums/`

**Step 4: Run full test suite**

Run: `cd /Users/karthick/launch-app/launch-go && go test ./...`

**Step 5: Commit**

```bash
git add -A
git commit -m "Remove over-engineered enum registry, consolidate to types package"
```

---

## Task 7: Update Documentation

**Files:**
- Modify: `CLAUDE.md` - Update enum documentation section

Add a section documenting the new pattern:

```markdown
## Type Definitions (Enums)

Go doesn't have native enums, so we use typed constants. All type definitions follow this pattern:

### Location
- Module-specific types: `internal/modules/{module}/types/`
- Shared helpers: `internal/pkg/types/`

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
func (s *Status) Scan(value interface{}) error { return types.ScanString(s, value) }
func (s Status) Value() (driver.Value, error)  { return types.ValueString(s) }
```

### File Organization
- Small enums (< 50 lines): Consolidate in `types/types.go`
- Large enums with many methods: Separate file like `types/software.go`
```

**Step 1: Update CLAUDE.md**

**Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "Update documentation for types package pattern"
```

---

## Verification Checklist

After completing all tasks:

1. [ ] `go build ./...` succeeds
2. [ ] `go test ./...` passes
3. [ ] No imports reference `*/enums` packages
4. [ ] All `*/enums/` directories deleted
5. [ ] Each module has `types/` directory
6. [ ] `internal/pkg/types/` contains only scan helpers and interfaces
7. [ ] No registry or builder patterns remain
8. [ ] CLAUDE.md updated with new patterns

---

## Rollback Plan

If issues arise:
1. Git history preserved - can revert individual commits
2. Each module refactored independently - partial rollback possible
3. Import paths are the main change - easy to search/replace back
