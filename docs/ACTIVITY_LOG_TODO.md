# Activity Log Implementation TODO

This document tracks all locations where activity logging needs to be implemented.

## Overview

- **Total Locations**: 52
- **Package**: `internal/pkg/activity`
- **Migration**: `0009_01_01_000000_create_activity_log_table.go`

## Usage Pattern

```go
import "github.com/kkz6/launch-go/internal/pkg/activity"

// In service methods with userID
activity.New(db).
    CausedByUser(userID).
    On(server).
    WithEvent("created").
    Log("Server was created")

// In jobs with Payload.UserID
if j.Payload.UserID != nil {
    activity.New(j.DB).
        CausedByUser(*j.Payload.UserID).
        On(database).
        WithEvent("installed").
        Log("Database was installed")
}
```

---

## Server Module (27 locations)

### Jobs

| # | File | Action | Subject | Has UserID |
|---|------|--------|---------|------------|
| 1 | `jobs/delete_server.go:27-98` | Server deleted | Server | Yes (Payload.UserID) |
| 2 | `jobs/provision_server.go:25-85` | Server provisioned | Server | Yes (Payload.UserID) |
| 3 | `jobs/create_on_provider.go:28-146` | Server created on provider | Server | Yes (Payload.UserID) |
| 4 | `jobs/add_ssh_key.go:26-71` | SSH key added to server | SshKey | No |
| 5 | `jobs/remove_ssh_key.go:26-72` | SSH key removed from server | SshKey | No |
| 6 | `jobs/archive_server.go:21-44` | Server archived | Server | Yes (Payload.UserID) |
| 7 | `jobs/reboot_server.go:23-56` | Server rebooted | Server | Yes (Payload.UserID) |
| 8 | `jobs/install_firewall_rule.go:27-76` | Firewall rule installed | FirewallRule | Yes (Payload.UserID) |
| 9 | `jobs/uninstall_firewall_rule.go:27-75` | Firewall rule uninstalled | FirewallRule | Yes (Payload.UserID) |
| 10 | `jobs/install_cron.go:23-66` | Cron job installed | Cron | Yes (Payload.UserID) |
| 11 | `jobs/uninstall_cron.go:27-68` | Cron job uninstalled | Cron | Yes (Payload.UserID) |
| 12 | `jobs/install_daemon.go:24-77` | Daemon installed | Daemon | Yes (Payload.UserID) |
| 13 | `jobs/uninstall_daemon.go:27-70` | Daemon uninstalled | Daemon | Yes (Payload.UserID) |

### Services

| # | File | Action | Subject | Has UserID |
|---|------|--------|---------|------------|
| 14 | `services/server_service.go:46-127` | Server created | Server | Yes (param) |
| 15 | `services/server_service.go:130-159` | Server updated | Server | No |
| 16 | `services/server_service.go:162-183` | Server deleted | Server | No |
| 17 | `services/ssh_key_service.go:26-41` | SSH key created | SshKey | Yes (param) |
| 18 | `services/ssh_key_service.go:103-114` | SSH key deleted | SshKey | No |
| 19 | `services/firewall_rule_service.go:23-55` | Firewall rule created | FirewallRule | No |
| 20 | `services/firewall_rule_service.go:58-102` | Firewall rule updated | FirewallRule | No |
| 21 | `services/firewall_rule_service.go:105-131` | Firewall rule deleted | FirewallRule | No |
| 22 | `services/cron_service.go:23-66` | Cron job created | Cron | No |
| 23 | `services/cron_service.go:69-106` | Cron job updated | Cron | No |
| 24 | `services/cron_service.go:109-151` | Cron job deleted | Cron | No |
| 25 | `services/daemon_service.go:22-69` | Daemon created | Daemon | No |
| 26 | `services/daemon_service.go:72-111` | Daemon updated | Daemon | No |
| 27 | `services/daemon_service.go:114-140` | Daemon deleted | Daemon | No |

---

## Database Module (10 locations)

### Jobs

| # | File | Action | Subject | Has UserID |
|---|------|--------|---------|------------|
| 28 | `jobs/install_database.go:24-67` | Database installed | Database | Yes (Payload.UserID) |
| 29 | `jobs/uninstall_database.go:24-64` | Database uninstalled | Database | Yes (Payload.UserID) |
| 30 | `jobs/install_database_user.go:25-84` | Database user installed | DatabaseUser | Yes (Payload.UserID) |
| 31 | `jobs/uninstall_database_user.go:24-65` | Database user uninstalled | DatabaseUser | Yes (Payload.UserID) |
| 32 | `jobs/update_database_user.go:23-72` | Database user updated | DatabaseUser | No |

### Services

| # | File | Action | Subject | Has UserID |
|---|------|--------|---------|------------|
| 33 | `services/database_service.go:13-91` | Database created | Database | Yes (param, optional) |
| 34 | `services/database_service.go:104-124` | Database deleted | Database | Yes (param, optional) |
| 35 | `services/database_user_service.go:13-56` | Database user created | DatabaseUser | Yes (param, optional) |
| 36 | `services/database_user_service.go:69-107` | Database user updated | DatabaseUser | Yes (param, optional) |
| 37 | `services/database_user_service.go:110-130` | Database user deleted | DatabaseUser | Yes (param, optional) |

---

## Site Module (5 locations)

### Services

| # | File | Action | Subject | Has UserID |
|---|------|--------|---------|------------|
| 38 | `services/site_service.go:68-190` | Site created | Site | Yes (param) |
| 39 | `services/site_service.go:207-311` | Site updated | Site | Yes (param) |
| 40 | `services/site_service.go:314-332` | Site deletion requested | Site | No |
| 41 | `services/redirect_service.go:50-71` | Redirect created | Redirect | Yes (param) |
| 42 | `services/redirect_service.go:83-95` | Redirect deleted | Redirect | No |

---

## Auth Module (6 locations)

### Services

| # | File | Action | Subject | Has UserID |
|---|------|--------|---------|------------|
| 43 | `services/user_service.go:37-76` | User profile updated | User | Yes (param) |
| 44 | `services/user_service.go:79-101` | User password changed | User | Yes (param) |
| 45 | `services/user_service.go:104-127` | User account deleted | User | Yes (param) |
| 46 | `services/team_service.go:25-42` | Team created | Team | Yes (param) |
| 47 | `services/team_service.go:45-67` | Team updated | Team | Yes (param) |
| 48 | `services/team_service.go:70-91` | Team deleted | Team | Yes (param) |

---

## Backup Module (3 locations)

### Services

| # | File | Action | Subject | Has UserID |
|---|------|--------|---------|------------|
| 49 | `services/backup_service.go:42-94` | Backup created | Backup | Yes (param) |
| 50 | `services/backup_service.go:97-143` | Backup updated | Backup | No |
| 51 | `services/backup_service.go:146+` | Backup deleted | Backup | No |

---

## DNS Module (1 location)

### Services

| # | File | Action | Subject | Has UserID |
|---|------|--------|---------|------------|
| 52 | `services/domain_service.go:42-100+` | Domain created | Domain | Yes (param) |

---

## Implementation Priority

### Phase 1: Core Models with User Context (High Priority)
- [x] Server create/update/delete (service)
- [x] Database create/delete (service)
- [x] Site create/update/delete (service)
- [x] Team create/update/delete (service)
- [x] User profile/password changes (service)

### Phase 2: Job Completions (Medium Priority)
- [ ] Server provisioned (job) - Note: Runs in background, completion handled elsewhere
- [x] Database installed/uninstalled (job)
- [x] Database user installed/uninstalled (job)

### Phase 3: Secondary Operations (Lower Priority)
- [ ] Firewall rules
- [ ] Cron jobs
- [ ] Daemons
- [ ] SSH keys
- [ ] Redirects
- [ ] Backups

---

## Notes

1. **Models already implement `GetID() string`** via `BaseModel` embedding
2. **Services without userID context** can still log activities, just without the causer
3. **Log names** should follow pattern: `server`, `database`, `site`, `auth`, `backup`, `dns`
4. **Events** should be: `created`, `updated`, `deleted`, `installed`, `uninstalled`, `provisioned`, `archived`, `rebooted`, `password_changed`
5. **DB access added** to server module's contracts.Repository interface via `DB() *gorm.DB` method
