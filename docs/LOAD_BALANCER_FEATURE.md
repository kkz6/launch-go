# Load Balancer Server Feature - Analysis Document

## Overview

This document outlines the implementation plan for adding a new **Load Balancer** server type to the Launch platform. Load balancer servers will use Caddy's reverse proxy capabilities to distribute traffic across multiple backend servers.

---

## Table of Contents

1. [Current Architecture](#current-architecture)
2. [Proposed Architecture](#proposed-architecture)
3. [Database Schema Changes](#database-schema-changes)
4. [Backend Implementation](#backend-implementation)
5. [Caddy Load Balancer Configuration](#caddy-load-balancer-configuration)
6. [Frontend UI Changes](#frontend-ui-changes)
7. [API Endpoints](#api-endpoints)
8. [Implementation Phases](#implementation-phases)
9. [Edge Cases and Considerations](#edge-cases-and-considerations)

---

## Current Architecture

### Server Types
Currently, the platform supports two server types:

| Type | Features |
|------|----------|
| `php` | Sites, PHP management, databases, queues, daemons, scheduler, SSL, backups |
| `database` | Database management, backups only |

### Site-Server Relationship
- **One-to-Many**: Each server can host multiple sites
- **Strict Binding**: Each site belongs to exactly one server (via `server_id`)
- **No Multi-Server**: No failover, clustering, or load balancing concepts exist

### Caddy Configuration
- Per-site Caddyfile at `{site_path}/Caddyfile`
- Global imports in `/etc/caddy/Sites.caddy`
- Direct PHP-FPM proxy (no upstream/load balancing)

---

## Proposed Architecture

### New Server Type: `loadbalancer`

A load balancer server will:
1. Run Caddy as a reverse proxy (no PHP-FPM, no local sites)
2. Distribute traffic to multiple backend PHP servers
3. Support health checks for backend servers
4. Support multiple load balancing strategies

### Relationship Model

```
┌─────────────────────────────────────────────────────────────┐
│                         Team                                 │
└─────────────────────────────────────────────────────────────┘
                              │
         ┌────────────────────┼────────────────────┐
         ▼                    ▼                    ▼
   ┌──────────┐        ┌──────────┐        ┌──────────────┐
   │ PHP      │        │ PHP      │        │ Load         │
   │ Server 1 │        │ Server 2 │        │ Balancer     │
   └──────────┘        └──────────┘        └──────────────┘
         │                    │                    │
         │                    │                    │
         ▼                    ▼                    ▼
   ┌──────────┐        ┌──────────┐        ┌──────────────┐
   │ Sites    │        │ Sites    │        │ LB Upstreams │
   └──────────┘        └──────────┘        └──────────────┘
                                                   │
                              ┌────────────────────┴──────────┐
                              ▼                               ▼
                       ┌─────────────┐                ┌─────────────┐
                       │ PHP Server 1│                │ PHP Server 2│
                       │ (Backend)   │                │ (Backend)   │
                       └─────────────┘                └─────────────┘
```

### Key Concepts

1. **Load Balancer Server**: A server of type `loadbalancer` that only runs Caddy
2. **Upstream**: A group of backend servers for a specific domain/site
3. **Backend Server**: A PHP server that can be added to an upstream
4. **Load Balanced Site**: A site that exists on multiple backend servers, served through a load balancer

---

## Critical Design Decision: Site-Level vs Server-Level Load Balancing

### The Problem: Multiple Sites on One Server

Consider this scenario:
```
PHP Server 1 hosts:
├── app.example.com      ← Want to load balance this
├── api.example.com      ← Keep serving directly
└── admin.example.com    ← Keep serving directly

PHP Server 2 hosts:
├── app.example.com      ← Backend for load balancing
└── blog.example.com     ← Keep serving directly
```

### Solution: Site-Level Backends (Not Server-Level)

Instead of adding entire servers as backends, we link **specific sites** to upstreams:

```
Upstream: app.example.com
├── Backend 1: Site "app.example.com" on Server 1
└── Backend 2: Site "app.example.com" on Server 2
```

This allows:
- Same server to have both load-balanced and non-load-balanced sites
- Granular control over which sites are distributed
- Each site maintains its own deployment configuration

### Traffic Flow After Load Balancing

```
                          ┌─────────────────────────────────┐
                          │         Load Balancer           │
                          │    Handles: app.example.com     │
                          │    TLS termination here         │
                          └─────────────┬───────────────────┘
                                        │ HTTP to :8080
                    ┌───────────────────┼───────────────────┐
                    ▼                                       ▼
        ┌───────────────────────┐               ┌───────────────────────┐
        │      PHP Server 1      │               │      PHP Server 2      │
        ├───────────────────────┤               ├───────────────────────┤
        │ app.example.com:8080  │               │ app.example.com:8080  │
        │ (Caddy on :8080,      │   LB Traffic  │ (Caddy on :8080,      │
        │  HTTP only, IP locked) │◄─────────────│  HTTP only, IP locked) │
        ├───────────────────────┤               ├───────────────────────┤
        │ api.example.com :443  │               │ blog.example.com :443 │
        │ (Caddy active,        │               │ (Caddy active,        │
        │  serves directly)     │               │  serves directly)     │
        └───────────────────────┘               └───────────────────────┘
```

### What Happens to Backend Site's Caddy?

**Important**: Backend Caddy keeps running. A **new Caddyfile block on a dedicated port (8080)** is added for load-balanced traffic. The original site Caddyfile on ports 80/443 is removed (since DNS now points to the LB).

1. **PHP-FPM uses Unix sockets**, not HTTP
2. Load balancer can only proxy HTTP requests
3. Backend Caddy is needed to translate HTTP → FastCGI → PHP-FPM
4. **Dedicated port 8080** avoids conflicts with other sites on 80/443

**Traffic flow**:
```
Client → Load Balancer (Caddy :443) → Backend Server (Caddy :8080) → PHP-FPM
                                        ↑
                                        Host: app.example.com header
                                        Plain HTTP (no TLS overhead)
```

### Backend Caddyfile Changes

When a site is added to a load balancer, a **new Caddyfile block on port 8080** is created for load-balanced traffic. The original site block on 80/443 is removed (since DNS points to the LB now).

**Before (normal mode)**:
```caddyfile
app.example.com {
    # Caddy auto-manages TLS on port 443
    root * /home/launch/app.example.com/current/public
    php_fastcgi unix//run/php/php8.3-fpm.sock
    file_server
}
```

**After (load balanced mode)**:
```caddyfile
http://app.example.com:8080 {
    # Dedicated port for LB traffic, HTTP only (no TLS)

    # Only accept requests from load balancer
    @notlb not remote_ip {{ .LoadBalancerIP }}
    respond @notlb "Forbidden" 403

    root * /home/launch/app.example.com/current/public
    php_fastcgi unix//run/php/php8.3-fpm.sock
    file_server
}
```

**Key changes**:
1. **Dedicated port 8080** - Completely separate from other sites on 80/443
2. **`http://` prefix** - Plain HTTP only (no TLS overhead, LB handles SSL termination)
3. **IP restriction** - Only accepts requests from load balancer IP
4. **Domain kept** - Matches the `Host` header sent by load balancer
5. **Firewall rule** - UFW rule added: `allow from <LB_IP> to any port 8080`

### Why Keep the Domain?

Even though DNS no longer points to the backend, the domain is needed for **Host header matching**:

```
# Load balancer sends:
GET /page HTTP/1.1
Host: app.example.com
X-Forwarded-For: 203.0.113.100

# Backend Caddy matches on "Host: app.example.com"
# and routes to the correct site
```

### DNS Consideration

- **Before load balancing**: `app.example.com` → PHP Server 1 IP
- **After load balancing**: `app.example.com` → Load Balancer IP

User must update DNS to point to load balancer. UI should show this warning.

### Security: IP Restriction + Firewall

The backend is protected by two layers:
1. **Firewall (UFW)**: Port 8080 only allows traffic from load balancer IP
2. **Caddy IP restriction**: Requests from non-LB IPs get 403 Forbidden
- Direct access to backend IP on port 8080 is blocked at firewall level
- Even if firewall is misconfigured, Caddy IP matcher provides backup protection
- Load balancer IP is stored in upstream config

When a backend is added to an upstream, the system automatically:
- Creates UFW rule: `allow from <LB_IP> to any port 8080`
When a backend is removed:
- Removes the UFW rule for port 8080

### Failover Capability

If the load balancer fails:
1. Remove the :8080 Caddyfile block on backends
2. Regenerate original site Caddyfile on 80/443 (normal mode with TLS)
3. Remove the port 8080 firewall rule
4. Update DNS to point directly to backend
5. Or: Add secondary load balancer for HA

---

## Domain Conflict Detection and Migration

### Scenario: Creating Upstream for Existing Site

When creating an upstream with address `app.example.com`:

1. **System searches** for existing sites with that address in the team
2. **If found**, UI shows:
   - Which servers have this site installed
   - Warning that site Caddyfile will be moved to port 8080
   - Option to auto-add those servers as backends

### UI Flow for Upstream Creation

```
┌─────────────────────────────────────────────────────────────────────┐
│                     Create Load Balancer Upstream                    │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Domain/Address: [app.example.com                              ]    │
│                                                                      │
│  ⚠️  EXISTING SITE DETECTED                                         │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │  This domain exists as a site on the following servers:      │    │
│  │                                                               │    │
│  │  ☑ Server 1 (192.168.1.10) - app.example.com                 │    │
│  │    └─ Caddy moved to port 8080, PHP-FPM will continue       │    │
│  │                                                               │    │
│  │  ☑ Server 2 (192.168.1.11) - app.example.com                 │    │
│  │    └─ Caddy moved to port 8080, PHP-FPM will continue       │    │
│  │                                                               │    │
│  │  These sites will become backends for this upstream.         │    │
│  │  Traffic will flow: Client → Load Balancer → Backend :8080   │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                      │
│  Load Balancing Policy: [Round Robin                          ▼]   │
│                                                                      │
│  ⚠️  DNS UPDATE REQUIRED                                            │
│  After creating this upstream, update your DNS:                     │
│  app.example.com → 203.0.113.50 (Load Balancer IP)                 │
│                                                                      │
│                              [Cancel]  [Create Upstream]             │
└─────────────────────────────────────────────────────────────────────┘
```

### Adding Backend to Existing Upstream

When adding a backend server that doesn't have the site:

```
┌─────────────────────────────────────────────────────────────────────┐
│                       Add Backend Server                             │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Select Server: [Server 3 (192.168.1.12)                      ▼]   │
│                                                                      │
│  ⚠️  SITE NOT FOUND ON THIS SERVER                                  │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │  The site "app.example.com" does not exist on Server 3.      │    │
│  │                                                               │    │
│  │  To use this server as a backend, you need to:               │    │
│  │  1. Deploy the same application to Server 3                  │    │
│  │  2. Ensure the site is installed at the same path            │    │
│  │                                                               │    │
│  │  [ ] Create site on Server 3 automatically (clone settings)  │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                      │
│                              [Cancel]  [Add Backend]                 │
└─────────────────────────────────────────────────────────────────────┘
```

### Backend Options for Servers WITH the Site

```
┌─────────────────────────────────────────────────────────────────────┐
│                       Add Backend Server                             │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Select Server: [Server 2 (192.168.1.11)                      ▼]   │
│                                                                      │
│  ✅  SITE FOUND                                                      │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │  Site: app.example.com                                        │    │
│  │  Path: /home/launch/app.example.com                          │    │
│  │  PHP Version: 8.3                                             │    │
│  │  Status: Installed                                            │    │
│  │                                                               │    │
│  │  Adding this backend will:                                    │    │
│  │  • Create Caddyfile on port 8080 for LB traffic              │    │
│  │  • Remove original Caddyfile on 80/443                       │    │
│  │  • Add firewall rule allowing LB IP on port 8080             │    │
│  │  • Route traffic through Load Balancer                       │    │
│  │  • Keep PHP-FPM running for application requests             │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                      │
│  Port:          [8080                                         ]    │
│                                                                      │
│                              [Cancel]  [Add Backend]                 │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Updated Relationship Model (Site-Level)

```
┌──────────────────────────────────────────────────────────────────────┐
│                              Team                                     │
└──────────────────────────────────────────────────────────────────────┘
                                  │
       ┌──────────────────────────┼──────────────────────────┐
       ▼                          ▼                          ▼
┌──────────────┐          ┌──────────────┐          ┌───────────────┐
│ PHP Server 1 │          │ PHP Server 2 │          │ Load Balancer │
├──────────────┤          ├──────────────┤          ├───────────────┤
│ Sites:       │          │ Sites:       │          │ Upstreams:    │
│ • app.com ──►│──────────│─• app.com ──►│──────────│─► app.com     │
│   (LB'd)     │          │   (LB'd)     │          │   Backends:   │
│ • api.com    │          │ • blog.com   │          │   ├─ Site@S1  │
│   (direct)   │          │   (direct)   │          │   └─ Site@S2  │
└──────────────┘          └──────────────┘          └───────────────┘
```

---

## Database Schema Changes

### 1. New Table: `load_balancer_upstreams`

Represents a load balanced endpoint configuration.

```sql
CREATE TABLE load_balancer_upstreams (
    id              CHAR(26) PRIMARY KEY,
    team_id         CHAR(26) NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    server_id       CHAR(26) NOT NULL REFERENCES servers(id) ON DELETE CASCADE,  -- The LB server
    name            VARCHAR(255) NOT NULL,          -- Display name
    address         VARCHAR(255) NOT NULL,          -- Domain (e.g., myapp.com)
    port            INT DEFAULT 443,                -- Listen port
    tls_setting     VARCHAR(50) DEFAULT 'auto',     -- auto, custom, internal, off

    -- Load balancing configuration
    lb_policy       VARCHAR(50) DEFAULT 'round_robin',  -- round_robin, least_conn, ip_hash, first, random
    health_check_path       VARCHAR(255) DEFAULT '/health',
    health_check_interval   VARCHAR(20) DEFAULT '30s',
    health_check_timeout    VARCHAR(20) DEFAULT '10s',

    -- Status tracking
    installed_at                TIMESTAMP NULL,
    installation_failed_at      TIMESTAMP NULL,
    pending_config_update_since TIMESTAMP NULL,

    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(server_id, address)
);

CREATE INDEX idx_lb_upstreams_server ON load_balancer_upstreams(server_id);
CREATE INDEX idx_lb_upstreams_team ON load_balancer_upstreams(team_id);
```

### 2. New Table: `load_balancer_backends`

Links **specific sites** (not just servers) to upstreams. This is the key difference from server-level load balancing.

```sql
CREATE TABLE load_balancer_backends (
    id              CHAR(26) PRIMARY KEY,
    upstream_id     CHAR(26) NOT NULL REFERENCES load_balancer_upstreams(id) ON DELETE CASCADE,
    site_id         CHAR(26) NOT NULL REFERENCES sites(id) ON DELETE CASCADE,    -- The specific site
    server_id       CHAR(26) NOT NULL REFERENCES servers(id) ON DELETE CASCADE,  -- Denormalized for quick lookups

    -- Backend configuration
    port            INT DEFAULT 8080,               -- Dedicated port for LB traffic on backend
    is_down         BOOLEAN DEFAULT FALSE,          -- Manually marked as down

    -- Health status (updated by Caddy's built-in health checks)
    health_status   VARCHAR(50) DEFAULT 'unknown',  -- healthy, unhealthy, unknown
    last_health_check_at TIMESTAMP NULL,

    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(upstream_id, site_id)  -- Each site can only be a backend once per upstream
);

CREATE INDEX idx_lb_backends_upstream ON load_balancer_backends(upstream_id);
CREATE INDEX idx_lb_backends_site ON load_balancer_backends(site_id);
CREATE INDEX idx_lb_backends_server ON load_balancer_backends(server_id);
```

### 3. Modify `sites` Table

Add column to track load balancing status:

```sql
-- Track if this site is behind a load balancer
ALTER TABLE sites ADD COLUMN load_balanced_upstream_id CHAR(26) NULL
    REFERENCES load_balancer_upstreams(id) ON DELETE SET NULL;

CREATE INDEX idx_sites_lb_upstream ON sites(load_balanced_upstream_id);
```

When `load_balanced_upstream_id` is set:
- Caddyfile is regenerated with `http://` prefix (no TLS)
- IP restriction added to only accept load balancer traffic
- Domain is kept for Host header matching

### 4. NO Changes to `servers` Table

Since we're tracking at the site level, servers don't need load balancing columns.
A server can have some sites load balanced and others not.

---

## Backend Implementation

### 1. New Server Type

**File**: `internal/modules/server/types/types.go`

Changes needed in the following methods:

```go
// Add new constant
const (
    ServerTypePhp          ServerType = "php"
    ServerTypeDatabase     ServerType = "database"
    ServerTypeLoadBalancer ServerType = "loadbalancer"  // NEW
)

// Update Label()
func (t ServerType) Label() string {
    labels := map[ServerType]string{
        ServerTypePhp:          "PHP Application Server",
        ServerTypeDatabase:     "Database Server",
        ServerTypeLoadBalancer: "Load Balancer",  // NEW
    }
    // ...
}

// Update IsValid()
func (t ServerType) IsValid() bool {
    switch t {
    case ServerTypePhp, ServerTypeDatabase, ServerTypeLoadBalancer:  // ADD
        return true
    }
    return false
}

// Update GetFeatures() - add new case
func (t ServerType) GetFeatures() []ServerFeature {
    switch t {
    // ... existing cases ...
    case ServerTypeLoadBalancer:
        return []ServerFeature{
            ServerFeatureLoadBalancing,  // NEW feature
            ServerFeatureSSLCertificates,
            ServerFeatureServices,
        }
    }
    return nil
}

// Update GetProcessManager() - LB has no process manager
func (t ServerType) GetProcessManager() ProcessManager {
    switch t {
    case ServerTypePhp:
        return ProcessManagerSupervisor
    case ServerTypeDatabase, ServerTypeLoadBalancer:  // ADD
        return ProcessManagerNone
    }
    return ProcessManagerNone
}

// Update AllServerTypes()
func AllServerTypes() []ServerType {
    return []ServerType{ServerTypePhp, ServerTypeDatabase, ServerTypeLoadBalancer}  // ADD
}
```

### 2. New Feature Enum

**File**: `internal/modules/server/types/types.go`

```go
const (
    // ... existing features ...
    ServerFeatureLoadBalancing ServerFeature = "load_balancing"  // NEW
)
```

### 2b. Update CreateServerRequest Validation

**File**: `internal/modules/server/dto/requests.go`

The `CreateServerRequest` must accept `loadbalancer` as a type and conditionally skip
PHP/database fields since load balancers don't need them:

```go
type CreateServerRequest struct {
    Name            string   `json:"name" validate:"required,min=2,max=255"`
    Description     *string  `json:"description" validate:"omitempty,max=1000"`
    Provider        string   `json:"provider" validate:"required,oneof=digitalocean hetzner linode vultr aws custom_server"`
    Type            string   `json:"type" validate:"required,oneof=php database loadbalancer"`  // ADD loadbalancer
    OperatingSystem string   `json:"operating_system" validate:"omitempty,oneof=ubuntu_20 ubuntu_22 ubuntu_24"`
    Region          string   `json:"region" validate:"required_unless=Provider custom_server"`
    Size            string   `json:"size" validate:"required_unless=Provider custom_server"`
    PHPVersion      string   `json:"php_version" validate:"omitempty,oneof=none php56 php70 php71 php72 php73 php74 php80 php81 php82 php83 php84"`
    DatabaseType    string   `json:"database_type" validate:"omitempty,oneof=none mysql80 postgresql16"`
    CredentialID    string   `json:"credential_id" validate:"required_unless=Provider custom_server,omitempty,ulid"`
    SSHKeyIDs       []string `json:"ssh_key_ids" validate:"omitempty"`
    InstallAgent    *bool    `json:"install_agent"`

    // For custom servers
    IP      string `json:"ip" validate:"required_if=Provider custom_server,omitempty,ip"`
    SSHPort int    `json:"port" validate:"omitempty,min=1,max=65535"`
    SSHUser string `json:"ssh_user" validate:"omitempty,max=100"`
}
```

Note: `PHPVersion` and `DatabaseType` are already `omitempty`, so they're not required.
The `createServicesForServer` switch handles ignoring these for load balancer type.
No additional validation changes needed — the service layer handles the logic.

### 3. New Models

**File**: `internal/modules/server/models/load_balancer_upstream.go`

```go
package models

type LoadBalancerUpstream struct {
    basemodels.BaseModel
    basemodels.TeamScoped
    basemodels.ServerScoped
    basemodels.InstallableModel

    Name    string `gorm:"size:255;not null"`
    Address string `gorm:"size:255;not null"`
    Port    int    `gorm:"default:443"`

    TLSSetting           string `gorm:"size:50;default:auto"`
    LBPolicy             string `gorm:"column:lb_policy;size:50;default:round_robin"`
    HealthCheckPath      string `gorm:"size:255;default:/health"`
    HealthCheckInterval  string `gorm:"size:20;default:30s"`
    HealthCheckTimeout   string `gorm:"size:20;default:10s"`

    PendingConfigUpdateSince *time.Time `gorm:"column:pending_config_update_since"`

    // Relations
    Backends []LoadBalancerBackend `gorm:"foreignKey:UpstreamID"`
    Server   *Server               `gorm:"foreignKey:ServerID"`
}
```

**File**: `internal/modules/server/models/load_balancer_backend.go`

```go
package models

type LoadBalancerBackend struct {
    basemodels.BaseModel

    UpstreamID string `gorm:"size:26;not null;index"`
    SiteID     string `gorm:"size:26;not null;index"`     // The specific site
    ServerID   string `gorm:"size:26;not null;index"`     // Denormalized from site

    Port   int  `gorm:"default:8080"`  // Dedicated port for LB traffic on backend
    IsDown bool `gorm:"default:false"` // Manually marked as down

    // Health status (updated by Caddy's built-in health checks)
    HealthStatus      string     `gorm:"size:50;default:unknown"`
    LastHealthCheckAt *time.Time

    // Relations
    Upstream *LoadBalancerUpstream `gorm:"foreignKey:UpstreamID"`
    Site     *Site                 `gorm:"foreignKey:SiteID"`      // Link to site model
    Server   *Server               `gorm:"foreignKey:ServerID"`
}
```

**File**: `internal/modules/site/models/site.go` (additions)

```go
// Add to Site struct
type Site struct {
    // ... existing fields ...

    // Load balancing field (for UI display only - Caddy keeps running)
    LoadBalancedUpstreamID *string `gorm:"size:26;index"`

    // Relation to upstream (if load balanced)
    LoadBalancedUpstream *LoadBalancerUpstream `gorm:"foreignKey:LoadBalancedUpstreamID"`
}

// IsLoadBalanced returns true if this site is behind a load balancer
func (s *Site) IsLoadBalanced() bool {
    return s.LoadBalancedUpstreamID != nil && *s.LoadBalancedUpstreamID != ""
}
```

### 4. New Load Balancing Policy Enum

**File**: `internal/modules/server/types/load_balancer.go`

```go
package types

type LBPolicy string

const (
    LBPolicyRoundRobin LBPolicy = "round_robin"
    LBPolicyLeastConn  LBPolicy = "least_conn"
    LBPolicyIPHash     LBPolicy = "ip_hash"
    LBPolicyFirst      LBPolicy = "first"
    LBPolicyRandom     LBPolicy = "random"
)

func (p LBPolicy) String() string { return string(p) }

func (p LBPolicy) Label() string {
    switch p {
    case LBPolicyRoundRobin:
        return "Round Robin"
    case LBPolicyLeastConn:
        return "Least Connections"
    case LBPolicyIPHash:
        return "IP Hash (Sticky Sessions)"
    case LBPolicyFirst:
        return "First Available"
    case LBPolicyRandom:
        return "Random"
    default:
        return string(p)
    }
}

func AllLBPolicies() []LBPolicy {
    return []LBPolicy{
        LBPolicyRoundRobin,
        LBPolicyLeastConn,
        LBPolicyIPHash,
        LBPolicyFirst,
        LBPolicyRandom,
    }
}
```

### 5. Server Creation Changes

**File**: `internal/modules/server/services/server_service.go`

The existing `createServicesForServer` uses a switch on `serverType` and calls
`s.createService(ctx, serverID, software)` which creates an `InstalledService` record.
Follow the exact same pattern as `createPhpServerServices` and `createDatabaseServerServices`:

```go
// Update the switch to include load balancer (line ~453)
func (s *Service) createServicesForServer(ctx context.Context, server *models.Server, req *dto.CreateServerRequest) error {
    serverType, _ := types.ParseServerType(req.Type)

    switch serverType {
    case types.ServerTypeDatabase:
        return s.createDatabaseServerServices(ctx, server, req)
    case types.ServerTypeLoadBalancer:                              // NEW
        return s.createLoadBalancerServerServices(ctx, server, req) // NEW
    default:
        return s.createPhpServerServices(ctx, server, req)
    }
}

// NEW: Load balancer only needs Caddy (LB mode) and optionally Launch Agent
func (s *Service) createLoadBalancerServerServices(ctx context.Context, server *models.Server, req *dto.CreateServerRequest) error {
    // Caddy in load balancer mode (uses install_caddy2_loadbalancer.sh template)
    if err := s.createService(ctx, server.ID, types.SoftwareCaddy2LB); err != nil {
        return fmt.Errorf("failed to create Caddy LB service: %w", err)
    }

    // Add Launch Agent if install_agent is not explicitly false
    if req.InstallAgent == nil || *req.InstallAgent {
        if err := s.createService(ctx, server.ID, types.SoftwareLaunchAgent); err != nil {
            return fmt.Errorf("failed to create Launch Agent service: %w", err)
        }
    }

    return nil
}
```

Note: `s.createService(ctx, serverID, software)` internally calls `software.GetServiceType()`,
`software.Label()`, `software.GetVersion()`, and `software.String()` to populate the
`InstalledService` record. This is why `SoftwareCaddy2LB` must implement all these methods.

### 6. New Service Layer

**File**: `internal/modules/server/services/load_balancer_service.go`

```go
package services

type LoadBalancerService struct {
    *app.BaseService
}

func NewLoadBalancerService(builder *app.Builder) *LoadBalancerService {
    return &LoadBalancerService{BaseService: app.NewBaseService(builder)}
}

// FindExistingSites searches for sites matching the upstream address across all team servers
// This is used to detect domain conflicts and auto-suggest backends
func (s *LoadBalancerService) FindExistingSites(ctx context.Context, teamID, address string) ([]models.Site, error) {
    return s.Repos().Site().FindByAddressAndTeam(ctx, address, teamID)
}

// CreateUpstream creates a new load balancer upstream
// If autoAddBackends is true, it will automatically add any existing sites as backends
func (s *LoadBalancerService) CreateUpstream(ctx context.Context, serverID string, req dto.CreateUpstreamRequest) (*models.LoadBalancerUpstream, error) {
    // Validate server is a load balancer
    server, err := s.Repos().Server().FindByID(ctx, serverID)
    if err != nil {
        return nil, err
    }
    if server.Type != servertypes.ServerTypeLoadBalancer {
        return nil, errors.New("server is not a load balancer")
    }

    // Check for existing sites with this address (for conflict warning)
    existingSites, err := s.FindExistingSites(ctx, server.TeamID, req.Address)
    if err != nil {
        return nil, err
    }

    upstream := &models.LoadBalancerUpstream{
        ServerID:            serverID,
        TeamID:              server.TeamID,
        Name:                req.Name,
        Address:             req.Address,
        Port:                req.Port,
        TLSSetting:          req.TLSSetting,
        LBPolicy:            req.LBPolicy,
        HealthCheckPath:     req.HealthCheckPath,
        HealthCheckInterval: req.HealthCheckInterval,
        HealthCheckTimeout:  req.HealthCheckTimeout,
    }

    if err := s.Repos().LoadBalancerUpstream().Create(ctx, upstream); err != nil {
        return nil, err
    }

    // If requested, auto-add existing sites as backends
    if req.AutoAddExistingSites && len(existingSites) > 0 {
        for _, site := range existingSites {
            _, _ = s.AddBackend(ctx, upstream.ID, dto.AddBackendRequest{
                SiteID:   site.ID,
                ServerID: site.ServerID,
                Port:     8080,
            })
        }
    }

    // Dispatch job to install Caddyfile on load balancer
    s.dispatchInstallUpstream(upstream)

    return upstream, nil
}

// AddBackend adds a specific site as a backend to an upstream
func (s *LoadBalancerService) AddBackend(ctx context.Context, upstreamID string, req dto.AddBackendRequest) (*models.LoadBalancerBackend, error) {
    upstream, err := s.Repos().LoadBalancerUpstream().FindByID(ctx, upstreamID)
    if err != nil {
        return nil, err
    }

    // Validate site exists
    site, err := s.Repos().Site().FindByID(ctx, req.SiteID)
    if err != nil {
        return nil, fmt.Errorf("site not found: %w", err)
    }

    // Validate site address matches upstream address
    if site.Address != upstream.Address {
        return nil, fmt.Errorf("site address '%s' does not match upstream address '%s'", site.Address, upstream.Address)
    }

    // Validate site belongs to same team
    if site.TeamID != upstream.TeamID {
        return nil, errors.New("site must belong to the same team")
    }

    // Validate site is not already load balanced
    if site.IsLoadBalanced() {
        return nil, errors.New("site is already part of a load balancer upstream")
    }

    // Get server for denormalized field
    server, err := s.Repos().Server().FindByID(ctx, site.ServerID)
    if err != nil {
        return nil, err
    }
    if server.Type != servertypes.ServerTypePhp {
        return nil, errors.New("site's server must be a PHP server")
    }

    port := req.Port
    if port == 0 {
        port = 8080
    }

    backend := &models.LoadBalancerBackend{
        UpstreamID: upstreamID,
        SiteID:     req.SiteID,
        ServerID:   site.ServerID,
        Port:       port,
    }

    if err := s.Repos().LoadBalancerBackend().Create(ctx, backend); err != nil {
        return nil, err
    }

    // Mark site as load balanced
    if err := s.Repos().Site().Update(ctx, req.SiteID, map[string]any{
        "load_balanced_upstream_id": upstreamID,
    }); err != nil {
        return nil, err
    }

    // Dispatch jobs:
    // 1. Add firewall rule on backend server: allow LB IP on dedicated port
    // 2. Create backend site Caddyfile on dedicated port (http://domain:8080, IP restricted)
    // 3. Remove original site Caddyfile on 80/443 (DNS now points to LB)
    // 4. Update load balancer Caddyfile to include new backend
    s.dispatchAddBackendFirewallRule(server, upstream.Server.PublicIPv4, port)
    s.dispatchRegenerateSiteCaddyfile(site, upstream.Server.PublicIPv4, port)
    s.dispatchUpdateUpstream(upstream)

    return backend, nil
}

// RemoveBackend removes a site from an upstream
func (s *LoadBalancerService) RemoveBackend(ctx context.Context, upstreamID, backendID string) error {
    backend, err := s.Repos().LoadBalancerBackend().FindByID(ctx, backendID)
    if err != nil {
        return err
    }

    // Get site before clearing
    site, _ := s.Repos().Site().FindByID(ctx, backend.SiteID)

    // Clear load balanced status on site
    if err := s.Repos().Site().Update(ctx, backend.SiteID, map[string]any{
        "load_balanced_upstream_id": nil,
    }); err != nil {
        return err
    }

    if err := s.Repos().LoadBalancerBackend().Delete(ctx, backendID); err != nil {
        return err
    }

    // Dispatch jobs:
    // 1. Remove firewall rule on backend server for the dedicated port
    // 2. Remove the :8080 Caddyfile block from backend
    // 3. Regenerate original site Caddyfile on 80/443 (normal mode, with TLS)
    // 4. Update load balancer Caddyfile to remove backend
    //
    // Note: Jobs are processed via the queue system (Asynq), which handles
    // concurrency. Multiple Caddyfile updates for the same server are
    // serialized through the queue, preventing race conditions.
    server, _ := s.Repos().Server().FindByID(ctx, backend.ServerID)
    upstream, _ := s.Repos().LoadBalancerUpstream().FindByID(ctx, upstreamID)
    s.dispatchRemoveBackendFirewallRule(server, upstream.Server.PublicIPv4, backend.Port)
    s.dispatchRegenerateSiteCaddyfile(site, "", 0) // Empty LB IP + port 0 = normal mode
    s.dispatchUpdateUpstream(upstream)

    return nil
}

// Note on Concurrency:
// All Caddyfile operations (install, update, remove) are dispatched as jobs
// to the queue system. The queue worker processes jobs sequentially per queue,
// ensuring that concurrent add/remove operations don't cause race conditions
// when regenerating Caddyfiles.
```

### 7. Repository Registry Updates

**File**: `internal/modules/server/repositories/registry.go`

The server module's registry must include the new repositories. Follow the existing pattern:

```go
type Registry struct {
    db             *gorm.DB
    server         *ServerRepository
    service        *ServiceRepository
    firewallRule   *FirewallRuleRepository
    cron           *CronRepository
    daemon         *DaemonRepository
    sshKey         *SSHKeyRepository
    task           *TaskRepository
    metric         *MetricRepository
    serverProvider *ServerProviderRepository
    database       *DatabaseRepository
    lbUpstream     *LoadBalancerUpstreamRepository  // NEW
    lbBackend      *LoadBalancerBackendRepository   // NEW
}

func NewRegistry(db *gorm.DB) *Registry {
    return &Registry{
        db:             db,
        server:         NewServerRepository(db),
        service:        NewServiceRepository(db),
        // ... existing repos ...
        lbUpstream:     NewLoadBalancerUpstreamRepository(db),  // NEW
        lbBackend:      NewLoadBalancerBackendRepository(db),   // NEW
    }
}

// NEW: Accessor methods
func (r *Registry) LoadBalancerUpstream() *LoadBalancerUpstreamRepository { return r.lbUpstream }
func (r *Registry) LoadBalancerBackend() *LoadBalancerBackendRepository  { return r.lbBackend }
```

**New repository files**:

**File**: `internal/modules/server/repositories/load_balancer_upstream_repository.go`

```go
type LoadBalancerUpstreamRepository struct {
    db *gorm.DB
}

func NewLoadBalancerUpstreamRepository(db *gorm.DB) *LoadBalancerUpstreamRepository {
    return &LoadBalancerUpstreamRepository{db: db}
}

func (r *LoadBalancerUpstreamRepository) Create(ctx context.Context, upstream *models.LoadBalancerUpstream) error { ... }
func (r *LoadBalancerUpstreamRepository) FindByID(ctx context.Context, id string) (*models.LoadBalancerUpstream, error) { ... }
func (r *LoadBalancerUpstreamRepository) FindByServerID(ctx context.Context, serverID string) ([]models.LoadBalancerUpstream, error) { ... }
func (r *LoadBalancerUpstreamRepository) Update(ctx context.Context, id string, updates map[string]any) error { ... }
func (r *LoadBalancerUpstreamRepository) Delete(ctx context.Context, id string) error { ... }
```

**File**: `internal/modules/server/repositories/load_balancer_backend_repository.go`

```go
type LoadBalancerBackendRepository struct {
    db *gorm.DB
}

func NewLoadBalancerBackendRepository(db *gorm.DB) *LoadBalancerBackendRepository {
    return &LoadBalancerBackendRepository{db: db}
}

func (r *LoadBalancerBackendRepository) Create(ctx context.Context, backend *models.LoadBalancerBackend) error { ... }
func (r *LoadBalancerBackendRepository) FindByID(ctx context.Context, id string) (*models.LoadBalancerBackend, error) { ... }
func (r *LoadBalancerBackendRepository) FindByUpstreamID(ctx context.Context, upstreamID string) ([]models.LoadBalancerBackend, error) { ... }
func (r *LoadBalancerBackendRepository) CountByUpstreamID(ctx context.Context, upstreamID string) (int64, error) { ... }
func (r *LoadBalancerBackendRepository) Delete(ctx context.Context, id string) error { ... }
```

### 8. Cross-Module Contracts

The `LoadBalancerService` needs to access the **site module's** repository to look up sites by address.
The codebase uses **explicit injection** for cross-module dependencies — never import another module's
repos directly in service code.

**Pattern**: Define an interface in the server module, implement it with the site repo, inject at bootstrap.

**File**: `internal/modules/server/contracts/site_reader.go`

```go
package contracts

type SiteReader interface {
    FindByAddressAndTeam(ctx context.Context, address, teamID string) ([]SiteInfo, error)
    FindByID(ctx context.Context, id string) (*SiteInfo, error)
    IsLoadBalanced(ctx context.Context, siteID string) (bool, error)
    UpdateLoadBalancedUpstreamID(ctx context.Context, siteID string, upstreamID *string) error
}

// SiteInfo is a read-only projection — avoids importing site models
type SiteInfo struct {
    ID       string
    ServerID string
    TeamID   string
    Address  string
    Type     string
    LoadBalancedUpstreamID *string
}
```

**File**: `internal/modules/server/module.go` — add setter

```go
type Module struct {
    app.Base
    repos      *repositories.Registry
    service    *services.Service
    siteReader contracts.SiteReader  // NEW
}

func (m *Module) SetSiteReader(reader contracts.SiteReader) {
    m.siteReader = reader
    m.service.SetSiteReader(reader)  // Pass down to service
}
```

**File**: `cmd/api/main.go` — wire at bootstrap

```go
// After creating modules
serverModule.SetSiteReader(siteModule.SiteReader())

// Site module exposes a SiteReader adapter:
func (m *Module) SiteReader() servercontracts.SiteReader {
    return &siteReaderAdapter{repo: m.repos.Site()}
}
```

This follows the exact same pattern as:
- `siteModule.SetDomainRepository(dnsModule.Repos().Domain())`
- `serverModule.RegisterRoutes(api, authMiddleware, siteModule.SiteRepository())`

### 9. Job Dependencies for Load Balancer Jobs

Load balancer jobs that run in the queue worker need cross-module repos. Follow the site module's
`JobDeps` pattern:

**File**: `internal/modules/server/jobs/lb_deps.go`

```go
type LBJobDeps struct {
    *pkgjobs.Deps
    Repos          *repositories.Registry
    SiteReader     contracts.SiteReader
    TaskRunnerDeps *tasks.TaskRunnerDeps
}
```

**File**: `internal/modules/server/jobs/register.go` — register LB job handlers

```go
func Register(mux *asynq.ServeMux, deps *JobDeps) {
    // ... existing job registrations ...

    // Load Balancer jobs
    mux.HandleFunc("server:install_lb_caddyfile", handleInstallLBCaddyfile(deps))
    mux.HandleFunc("server:update_lb_caddyfile", handleUpdateLBCaddyfile(deps))
    mux.HandleFunc("server:remove_lb_caddyfile", handleRemoveLBCaddyfile(deps))
    mux.HandleFunc("server:add_backend_firewall", handleAddBackendFirewall(deps))
    mux.HandleFunc("server:remove_backend_firewall", handleRemoveBackendFirewall(deps))
}
```

---

## Caddy Load Balancer Configuration

### 1. Load Balancer Caddyfile Structure

For a load balancer server, the Caddyfile will look like:

```caddyfile
# /etc/caddy/Caddyfile (on load balancer server)

{
    # Global options
    admin off
}

# Import all upstream configurations
import /etc/caddy/Upstreams.caddy
```

### 2. Per-Upstream Caddyfile

Each upstream gets its own Caddyfile at `/etc/caddy/upstreams/{upstream_id}.caddy`:

```caddyfile
# Upstream: myapp.com
# Policy: round_robin
# Backends: 2

myapp.com {
    # TLS auto-managed by Caddy (Let's Encrypt)

    # Security headers
    header {
        -Server
        X-Content-Type-Options nosniff
        X-Frame-Options SAMEORIGIN
        X-Powered-By "Launch"
    }

    # Reverse proxy to backends with load balancing
    reverse_proxy {
        # Backend servers (port 8080 - dedicated LB port, plain HTTP)
        to 192.168.1.10:8080 192.168.1.11:8080

        # Load balancing policy
        lb_policy round_robin

        # Health checks (on the dedicated LB port)
        health_uri /health
        health_interval 30s
        health_timeout 10s
        health_status 200

        # No TLS transport needed - backends serve plain HTTP on port 8080

        # Headers
        header_up Host {upstream_hostport}
        header_up X-Real-IP {remote_host}
        header_up X-Forwarded-For {remote_host}
        header_up X-Forwarded-Proto {scheme}
    }

    # Logging
    log {
        output file /var/log/caddy/myapp.com.log {
            roll_size 100mb
            roll_keep 30
            roll_keep_for 720h
        }
    }
}
```

### 3. Caddyfile Generation for Load Balancer

**File**: `internal/modules/server/jobs/install_lb_caddyfile.go`

```go
func generateLBCaddyfile(upstream *models.LoadBalancerUpstream, backends []models.LoadBalancerBackend) string {
    var sb strings.Builder

    // Header comment
    sb.WriteString(fmt.Sprintf("# Upstream: %s\n", upstream.Address))
    sb.WriteString(fmt.Sprintf("# Policy: %s\n", upstream.LBPolicy))
    sb.WriteString(fmt.Sprintf("# Backends: %d\n\n", len(backends)))

    // Domain block
    if upstream.Port != 443 {
        sb.WriteString(fmt.Sprintf("%s:%d {\n", upstream.Address, upstream.Port))
    } else {
        sb.WriteString(fmt.Sprintf("%s {\n", upstream.Address))
    }

    // TLS configuration
    switch upstream.TLSSetting {
    case "auto":
        // Default - Caddy auto-manages
    case "internal":
        sb.WriteString("    tls internal\n")
    case "off":
        sb.WriteString("    # TLS disabled\n")
    }

    // Security headers
    sb.WriteString(`    header {
        -Server
        X-Content-Type-Options nosniff
        X-Frame-Options SAMEORIGIN
        X-Powered-By "Launch"
    }

`)

    // Reverse proxy block
    sb.WriteString("    reverse_proxy {\n")

    // Backend addresses (port 8080 - dedicated LB port, plain HTTP)
    var addrs []string
    for _, backend := range backends {
        if !backend.IsDown {
            port := backend.Port
            if port == 0 {
                port = 8080
            }
            addr := fmt.Sprintf("%s:%d", backend.Server.PublicIPv4, port)
            addrs = append(addrs, addr)
        }
    }
    sb.WriteString(fmt.Sprintf("        to %s\n\n", strings.Join(addrs, " ")))

    // Load balancing policy
    sb.WriteString(fmt.Sprintf("        lb_policy %s\n\n", upstream.LBPolicy))

    // Health checks
    sb.WriteString(fmt.Sprintf("        health_uri %s\n", upstream.HealthCheckPath))
    sb.WriteString(fmt.Sprintf("        health_interval %s\n", upstream.HealthCheckInterval))
    sb.WriteString(fmt.Sprintf("        health_timeout %s\n", upstream.HealthCheckTimeout))
    sb.WriteString("        health_status 200\n\n")

    // No TLS transport needed - backends serve plain HTTP on dedicated port

    // Forwarding headers
    sb.WriteString(`        header_up Host {upstream_hostport}
        header_up X-Real-IP {remote_host}
        header_up X-Forwarded-For {remote_host}
        header_up X-Forwarded-Proto {scheme}
`)

    sb.WriteString("    }\n\n")

    // Logging
    logPath := fmt.Sprintf("/var/log/caddy/%s.log", strings.ReplaceAll(upstream.Address, ".", "_"))
    sb.WriteString(fmt.Sprintf(`    log {
        output file %s {
            roll_size 100mb
            roll_keep 30
            roll_keep_for 720h
        }
    }
`, logPath))

    sb.WriteString("}\n")

    return sb.String()
}
```

### 4. Caddy Load Balancing Policies

| Policy | Caddy Directive | Description |
|--------|-----------------|-------------|
| Round Robin | `lb_policy round_robin` | Default, cycles through backends |
| Least Connections | `lb_policy least_conn` | Routes to backend with fewest active requests |
| IP Hash | `lb_policy ip_hash` | Sticky sessions based on client IP |
| First | `lb_policy first` | Uses first available backend |
| Random | `lb_policy random` | Random selection |

---

## Frontend UI Changes

### 1. Server List Page (`/servers/index.vue`)

**Changes needed:**

1. **Visual Indicator for Server Type**
   - Add icon/badge for load balancer servers
   - Show "Load Balancer" label instead of site count

2. **Load Balanced Server Indicator**
   - For PHP servers that are load balanced, show a badge/icon
   - Tooltip showing which load balancer manages this server

3. **Card Layout Updates**
```vue
<template>
  <div class="server-card">
    <!-- Existing content -->

    <!-- NEW: Load balancer indicator -->
    <div v-if="server.type === 'loadbalancer'" class="badge badge-purple">
      Load Balancer
    </div>

    <!-- NEW: "Load Balanced" indicator for PHP servers -->
    <div v-if="server.load_balanced_by_id" class="flex items-center gap-1 text-sm text-purple-600">
      <ScaleIcon class="h-4 w-4" />
      <span>Load Balanced</span>
    </div>
  </div>
</template>
```

### 2. Server Detail Page - Load Balancer (`/servers/[id]/index.vue`)

**New Tab Structure for Load Balancer Servers:**

```typescript
const loadBalancerTabs = [
  { key: 'upstreams', label: 'Upstreams', icon: 'scale' },
  { key: 'metrics', label: 'Metrics', icon: 'chart' },
  { key: 'networks', label: 'Networks', icon: 'globe' },
  { key: 'advanced', label: 'Advanced', icon: 'cog' },
];

// NOT included: sites, databases, daemons, schedulers, php
```

### 3. New Component: Upstreams Management

**File**: `/client/components/server/LoadBalancerUpstreams.vue`

```vue
<template>
  <div>
    <!-- Header -->
    <div class="flex justify-between items-center mb-6">
      <h2 class="text-lg font-semibold">Load Balancer Upstreams</h2>
      <Button @click="showCreateDialog = true">Add Upstream</Button>
    </div>

    <!-- Upstream List -->
    <div class="space-y-4">
      <div v-for="upstream in upstreams" :key="upstream.id" class="border rounded-lg p-4">
        <div class="flex justify-between items-start">
          <div>
            <h3 class="font-medium">{{ upstream.name }}</h3>
            <p class="text-sm text-gray-500">{{ upstream.address }}</p>
          </div>
          <StatusBadge :status="upstream.installed_at ? 'active' : 'pending'" />
        </div>

        <!-- Backend Servers -->
        <div class="mt-4">
          <h4 class="text-sm font-medium mb-2">Backend Servers ({{ upstream.backends.length }})</h4>
          <div class="space-y-2">
            <div v-for="backend in upstream.backends" :key="backend.id"
                 class="flex items-center justify-between p-2 bg-gray-50 rounded">
              <div class="flex items-center gap-2">
                <HealthIndicator :status="backend.health_status" />
                <span>{{ backend.server.name }}</span>
                <span class="text-sm text-gray-500">{{ backend.server.public_ipv4 }}</span>
              </div>
              <div class="flex items-center gap-2">
                <span class="text-sm text-gray-500">Port: {{ backend.port }}</span>
                <Button size="sm" variant="ghost" @click="removeBackend(upstream.id, backend.id)">
                  Remove
                </Button>
              </div>
            </div>
          </div>
          <Button size="sm" variant="outline" class="mt-2" @click="showAddBackendDialog(upstream)">
            Add Backend Server
          </Button>
        </div>

        <!-- Configuration -->
        <div class="mt-4 grid grid-cols-3 gap-4 text-sm">
          <div>
            <span class="text-gray-500">Policy:</span>
            <span class="ml-1">{{ upstream.lb_policy_label }}</span>
          </div>
          <div>
            <span class="text-gray-500">Health Check:</span>
            <span class="ml-1">{{ upstream.health_check_path }}</span>
          </div>
          <div>
            <span class="text-gray-500">TLS:</span>
            <span class="ml-1">{{ upstream.tls_setting }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Create Upstream Dialog -->
    <Dialog v-model="showCreateDialog">
      <CreateUpstreamForm @created="onUpstreamCreated" />
    </Dialog>

    <!-- Add Backend Dialog -->
    <Dialog v-model="showAddBackendDialog">
      <AddBackendForm :upstream="selectedUpstream" @added="onBackendAdded" />
    </Dialog>
  </div>
</template>
```

### 4. PHP Server Detail - Load Balanced Indicator

**File**: `/client/components/server/LoadBalancedBanner.vue`

```vue
<template>
  <div v-if="server.load_balanced_by_id" class="bg-purple-50 border border-purple-200 rounded-lg p-4 mb-6">
    <div class="flex items-center gap-3">
      <ScaleIcon class="h-6 w-6 text-purple-600" />
      <div>
        <h3 class="font-medium text-purple-900">This server is load balanced</h3>
        <p class="text-sm text-purple-700">
          Traffic is distributed by
          <router-link :to="`/servers/${server.load_balanced_by_id}`" class="underline font-medium">
            {{ loadBalancerName }}
          </router-link>
        </p>
      </div>
    </div>

    <!-- Quick stats -->
    <div class="mt-3 flex gap-6 text-sm">
      <div>
        <span class="text-purple-600">Upstream:</span>
        <span class="ml-1 font-medium">{{ upstreamAddress }}</span>
      </div>
      <div>
        <span class="text-purple-600">Other Backends:</span>
        <span class="ml-1 font-medium">{{ otherBackendsCount }}</span>
      </div>
      <div>
        <span class="text-purple-600">Health Status:</span>
        <HealthIndicator :status="healthStatus" size="sm" />
      </div>
    </div>
  </div>
</template>
```

### 5. Server Creation Form Updates

**File**: `/client/pages/servers/create.vue`

Add load balancer as a server type option:

```typescript
// Server type options (from API)
const serverTypes = {
  php: "PHP Application Server",
  database: "Database Server",
  loadbalancer: "Load Balancer",  // NEW
};

// Conditional form fields
const showPhpOptions = computed(() => form.type === 'php');
const showDatabaseOptions = computed(() => form.type === 'php' || form.type === 'database');
const showLoadBalancerOptions = computed(() => form.type === 'loadbalancer');  // NEW
```

### 6. TypeScript Type Updates

**File**: `/client/types/index.ts`

```typescript
interface Server {
  // ... existing fields ...

  // NEW: Load balancing fields
  load_balanced_by_id?: string;
  load_balanced_upstream_id?: string;
  load_balancer?: Server;  // If this server is a load balancer, null otherwise
}

interface LoadBalancerUpstream {
  id: string;
  server_id: string;
  team_id: string;
  name: string;
  address: string;
  port: number;
  tls_setting: string;
  lb_policy: string;
  lb_policy_label: string;
  health_check_path: string;
  health_check_interval: string;
  health_check_timeout: string;
  installed_at?: string;
  backends: LoadBalancerBackend[];
}

interface LoadBalancerBackend {
  id: string;
  upstream_id: string;
  server_id: string;
  server: Server;
  port: number;           // Dedicated port for LB traffic (default 8080)
  is_down: boolean;
  health_status: 'healthy' | 'unhealthy' | 'unknown';
  last_health_check_at?: string;
}
```

---

## API Endpoints

### New Endpoints for Load Balancer Management

```
# All routes nested under /servers/:serverId for middleware consistency
# (RequireProvisionedServer, team-scoping, authorization)

# Domain Conflict Detection (call before creating upstream)
GET    /api/servers/:serverId/upstreams/check-domain?address=app.example.com
       # Returns: { exists: true, sites: [...] } if domain is already used

# Upstreams
GET    /api/servers/:serverId/upstreams                    # List upstreams for LB server
POST   /api/servers/:serverId/upstreams                    # Create upstream
GET    /api/servers/:serverId/upstreams/:upstreamId        # Get upstream details
PUT    /api/servers/:serverId/upstreams/:upstreamId        # Update upstream config
DELETE /api/servers/:serverId/upstreams/:upstreamId        # Delete upstream

# Backends (nested under upstreams)
GET    /api/servers/:serverId/upstreams/:upstreamId/backends           # List backend sites
POST   /api/servers/:serverId/upstreams/:upstreamId/backends           # Add site as backend
PUT    /api/servers/:serverId/upstreams/:upstreamId/backends/:id       # Update backend config
DELETE /api/servers/:serverId/upstreams/:upstreamId/backends/:id       # Remove site from upstream

# Available Sites for Backend (sites matching upstream address, not yet added)
GET    /api/servers/:serverId/upstreams/:upstreamId/available-sites    # Sites that can be added

# Health
GET    /api/servers/:serverId/upstreams/:upstreamId/health             # Get health status of all backends
POST   /api/servers/:serverId/upstreams/:upstreamId/backends/:id/toggle-down  # Toggle backend up/down
```

### Domain Check Response Example

```json
// GET /api/servers/:serverId/upstreams/check-domain?address=app.example.com
{
  "success": true,
  "data": {
    "address": "app.example.com",
    "exists": true,
    "sites": [
      {
        "id": "01ARZ3NDEKTSV4RRFFQ69G5FAV",
        "server_id": "01ARZ3NDEKTSV4RRFFQ69G5ABC",
        "server_name": "PHP Server 1",
        "server_ip": "192.168.1.10",
        "address": "app.example.com",
        "path": "/home/launch/app.example.com",
        "php_version": "8.3",
        "installed_at": "2024-01-15T10:30:00Z"
      },
      {
        "id": "01ARZ3NDEKTSV4RRFFQ69G5XYZ",
        "server_id": "01ARZ3NDEKTSV4RRFFQ69G5DEF",
        "server_name": "PHP Server 2",
        "server_ip": "192.168.1.11",
        "address": "app.example.com",
        "path": "/home/launch/app.example.com",
        "php_version": "8.3",
        "installed_at": "2024-01-16T14:20:00Z"
      }
    ],
    "warning": "Creating this upstream will remove Caddyfiles from these sites. Traffic will be routed through the load balancer."
  }
}
```

### Modified Endpoints

```
# Server creation - accept "loadbalancer" as type
POST   /api/servers

# Server details - include load_balanced_by info
GET    /api/servers/:id

# Create options - include load balancer type
GET    /api/servers/create-options
```

### Server Response: SitesCount vs UpstreamsCount

**Problem**: The server list API returns `sites_count` for each server (via `SiteCounter` interface).
Load balancer servers have zero sites — showing "0 sites" is misleading.

**Solution**: Add `upstreams_count` to the response and use it contextually in the UI.

**File**: `internal/modules/server/dto/responses.go`

```go
type ServerResponse struct {
    // ... existing fields ...
    SitesCount     int `json:"sites_count,omitempty"`
    UpstreamsCount int `json:"upstreams_count,omitempty"`  // NEW: only for LB servers
}

func ToServerResponse(server *models.Server) ServerResponse {
    resp := ServerResponse{
        // ... existing fields ...
        SitesCount: int(server.SitesCount),
    }

    // For load balancer servers, include upstream count
    if server.Type == servertypes.ServerTypeLoadBalancer {
        resp.UpstreamsCount = int(server.UpstreamsCount)
    }

    return resp
}
```

**File**: `internal/modules/server/models/server.go` — add computed field

```go
type Server struct {
    // ... existing fields ...
    SitesCount     int64 `gorm:"-"` // Existing: computed by SiteCounter
    UpstreamsCount int64 `gorm:"-"` // NEW: computed by UpstreamCounter
}
```

**File**: `internal/modules/server/handlers/server_handler.go` — add UpstreamCounter

```go
type UpstreamCounter interface {
    CountByServer(ctx context.Context, serverID string) (int64, error)
}

type Handler struct {
    service         *services.Service
    taskRunner      *tasks.TaskRunnerDeps
    siteCounter     SiteCounter
    upstreamCounter UpstreamCounter  // NEW
}
```

**UI handling**: The server card component uses `upstreams_count` for LB servers:

```vue
<template>
  <!-- For PHP/Database servers -->
  <span v-if="server.type !== 'loadbalancer'">
    {{ server.sites_count }} {{ server.sites_count === 1 ? 'site' : 'sites' }}
  </span>
  <!-- For Load Balancer servers -->
  <span v-else>
    {{ server.upstreams_count }} {{ server.upstreams_count === 1 ? 'upstream' : 'upstreams' }}
  </span>
</template>
```

---

## Implementation Phases

### Phase 1: Database & Models (Backend Foundation) ✅
- [x] Create migration for `load_balancer_upstreams` table
- [x] Create migration for `load_balancer_backends` table (with `site_id`)
- [x] Create migration to add `load_balanced_upstream_id` to `sites` table
- [x] Create `LoadBalancerUpstream` model
- [x] Create `LoadBalancerBackend` model (with site relation)
- [x] Add `IsLoadBalanced()` method to Site model
- [x] Create `LBPolicy` enum type
- [x] Add `ServerTypeLoadBalancer` to server types
- [x] Add `ServerFeatureLoadBalancing` feature
- [x] Add `SoftwareCaddy2LB` to software types (all ~15 methods)
- [x] Update `CreateServerRequest` validation: `oneof=php database loadbalancer`

### Phase 2: Server Provisioning ✅
- [x] Update `createServicesForServer` switch for load balancer type
- [x] Create `createLoadBalancerServerServices()` function
- [x] Create `install_caddy2_loadbalancer.sh` script (or parameterize existing)
- [x] Verify all 8 provision steps work for LB servers (no changes needed)
- [x] Test load balancer server provisioning end-to-end

**See: [Load Balancer Provisioning Details](#load-balancer-provisioning-details)**

### Phase 3: Repositories & Cross-Module Contracts ✅
- [x] Create `LoadBalancerUpstreamRepository` with CRUD methods
- [x] Create `LoadBalancerBackendRepository` with CRUD methods
- [x] Add both repos to server module `Registry` struct
- [x] Create `SiteReader` contract interface in `server/contracts/`
- [x] Implement `SiteReader` adapter in site module
- [x] Wire `SetSiteReader()` in `cmd/api/main.go`
- [x] Add `UpstreamCounter` interface to server handler

### Phase 4: Service Layer & Handlers ✅
- [x] Create `LoadBalancerService`
- [x] Implement `FindExistingSites()` for domain conflict detection
- [x] Create upstream handlers (CRUD)
- [x] Create backend handlers (add/remove/update) - site-level
- [x] Create domain check endpoint (`/check-domain`)
- [x] Register routes under `/servers/:serverId/upstreams/...`
- [x] Update `ServerResponse` to include `upstreams_count` for LB servers

### Phase 5: Backend Site Caddyfile Changes ✅
- [x] Modify `generateCaddyfile()` to check `IsLoadBalanced()`
- [x] Generate dedicated port Caddyfile: `http://domain:8080` (plain HTTP, no TLS)
- [x] Add IP restriction matcher (`@notlb not remote_ip`) when load balanced
- [x] Store load balancer IP and backend port from upstream/backend models
- [x] Auto-create UFW firewall rule: `allow from <LB_IP> to any port 8080`
- [x] Remove original site Caddyfile on 80/443 when adding to upstream
- [x] Restore original site Caddyfile on 80/443 when removing from upstream
- [x] Auto-remove UFW firewall rule on backend removal
- [x] Implement task callbacks (`OnSuccess`/`OnFailure`/`OnExpired`) for all jobs
- [x] Test Caddyfile regeneration on add/remove backend

### Phase 6: Load Balancer Caddy Configuration ✅
- [x] Create `generateLBCaddyfile()` function
- [x] Create `InstallUpstreamCaddyfile` job with task callbacks
- [x] Create `UpdateUpstreamCaddyfile` job with task callbacks
- [x] Create `RemoveUpstreamCaddyfile` job with task callbacks
- [x] Create `/etc/caddy/Upstreams.caddy` import management
- [x] Register LB job handlers in `jobs/register.go`
- [x] Create `LBJobDeps` struct for cross-module deps
- [x] Test Caddy configuration generation

### Phase 7: WebSocket Events & Error Recovery ✅
- [x] Add LB event constants to `broadcast/events.go`
- [x] Broadcast events from service methods and job callbacks
- [x] Implement error recovery for add/remove backend failures
- [x] Create `CleanupLBBackendsJob` for orphaned state cleanup
- [x] Configure retry policy for LB jobs (max 3, exponential backoff)
- [x] Test failure scenarios (partial add, partial remove)

### Phase 8: Frontend - Server List & Types ✅
- [x] Update server type display for load balancer
- [x] Add load balancer badge/icon in server cards
- [x] Show `upstreams_count` instead of `sites_count` for LB servers
- [x] Add "load balanced" indicator for sites in site list
- [x] Update TypeScript types for new models

### Phase 9: Frontend - Upstream Creation with Domain Detection ✅
- [x] Create domain check API call on address input (debounced)
- [x] Show warning when existing sites found
- [x] Display which servers have the site
- [x] Checkbox to auto-add existing sites as backends
- [x] DNS update warning display

### Phase 10: Frontend - Load Balancer Detail ✅
- [x] Create `LoadBalancerUpstreams` component (`ShowUpstreams.vue`)
- [x] Create `CreateUpstreamForm` component with domain detection (`CreateUpstream.vue`)
- [x] Create `AddBackendForm` component (`ManageBackends.vue` sheet with add/toggle/remove)
- [x] Create `HealthIndicator` component (health badges in `ShowUpstreams.vue`)
- [x] Update tab structure for load balancer servers (conditional "Upstreams" tab)
- [x] Wire WebSocket events for real-time upstream/backend updates (`useLoadBalancerEvents`)

### Phase 11: Frontend - Site Updates ✅
- [x] Add "Load Balanced" badge to site cards (`ShowSites.vue` "LB" badge)
- [x] Create `LoadBalancedBanner` component for site detail
- [x] Show upstream info on site detail page
- [x] Warning about deployment sync (banner includes sync message)

### Phase 12: Health Checks & Monitoring ✅
- [x] Implement health check result storage
- [x] Create scheduled job for health check polling
- [x] Broadcast `backend.health_changed` events
- [x] UI updates for real-time health status

### Phase 13: Testing & Polish (Partial)
- [x] Unit tests for services (LoadBalancerService CheckDomain - 5 tests)
- [ ] Unit tests for repositories (upstream, backend CRUD)
- [ ] Integration tests for API endpoints
- [x] Test domain conflict detection flow (5 tests)
- [x] Test LB Caddyfile generation (13 tests)
- [x] Test backend site Caddyfile changes (8 tests)
- [ ] Test error recovery (partial add/remove failures)
- [ ] Test WebSocket event broadcasting
- [ ] E2E tests for UI flows
- [x] Documentation updates

---

## Edge Cases and Considerations

### 1. Backend Server Deletion
- When a PHP server is deleted that's part of an upstream:
  - Automatically remove from all upstreams
  - Remove :8080 Caddyfile blocks (no need since server is being deleted)
  - Update LB Caddyfiles to remove this backend
  - Send notification about reduced capacity

### 2. Load Balancer Deletion
- When a load balancer is deleted:
  - Clear `load_balanced_upstream_id` on all backend sites
  - Delete all associated upstreams and backends
  - Remove :8080 Caddyfile blocks from all backend servers
  - Restore original site Caddyfiles on 80/443
  - Remove firewall rules for port 8080 on backend servers
  - Notify affected team members

### 3. Backend Server Unreachable
- Health checks mark server as unhealthy
- Caddy automatically stops routing traffic
- UI shows health status
- Send notification after N consecutive failures

### 4. SSL Certificate Management
- Load balancer handles SSL termination (Caddy auto-manages Let's Encrypt)
- Backends serve plain HTTP on :8080 (no TLS needed)
- No certificate sync needed between LB and backends

### 5. Same Site on Multiple Backends
- Sites with same address on different backend servers
- Deployment sync across backends
- Rollback coordination

### 6. Private Network Support
- Use private IPs when available
- Fallback to public IPs
- Security group/firewall configuration

### 7. Session Persistence
- IP hash policy for sticky sessions
- Cookie-based affinity option
- Redis session storage recommendation

### 8. Graceful Backend Removal
- Drain connections before removing
- Wait for active requests to complete
- Configurable drain timeout

### 9. Site Deletion When Load Balanced
**Policy**: Sites that are part of a load balancer upstream **cannot be deleted** directly.

**UI Behavior**:
- Delete button shows warning or is disabled
- Message: "This site is part of a load balancer upstream. Remove it from the load balancer first."
- Link to the load balancer upstream management page

**Backend Validation**:
```go
func (s *SiteService) Delete(ctx context.Context, siteID string) error {
    site, err := s.repo.FindByID(ctx, siteID)
    if err != nil {
        return err
    }

    if site.IsLoadBalanced() {
        return errors.New("cannot delete site that is part of a load balancer upstream - remove from load balancer first")
    }

    // ... proceed with deletion
}
```

### 10. Zero Backends Validation
**Policy**: Upstreams require at least one backend to be functional.

**UI Behavior**:
- When creating upstream: Must select at least one backend OR auto-add existing sites
- When removing last backend: Show warning "This is the last backend. Removing it will make the upstream non-functional."
- Upstream list shows warning badge for upstreams with 0 backends

**Backend Validation**:
```go
func (s *LoadBalancerService) RemoveBackend(ctx context.Context, upstreamID, backendID string) error {
    // Count remaining backends
    count, _ := s.repo.CountBackends(ctx, upstreamID)
    if count <= 1 {
        // Allow removal but return warning
        // Don't block - user might be replacing backends
    }
    // ... proceed with removal
}
```

**Caddyfile Handling**:
- Upstreams with 0 backends generate a placeholder config:
```caddyfile
myapp.com {
    respond "Service Unavailable - No backends configured" 503
}
```

### 11. Session Persistence Warning (UI)
**Important**: Users must understand session handling implications.

**UI Warning on Upstream Creation**:
```
┌─────────────────────────────────────────────────────────────────────┐
│  ⚠️  SESSION PERSISTENCE                                            │
│                                                                      │
│  If your application uses file-based or in-memory sessions,         │
│  users may lose their session when requests go to different         │
│  backends.                                                           │
│                                                                      │
│  Recommendations:                                                    │
│  • Use "IP Hash" load balancing policy for sticky sessions          │
│  • Or configure your app to use Redis/Database sessions             │
│                                                                      │
│  For Laravel apps, set SESSION_DRIVER=redis or SESSION_DRIVER=database │
└─────────────────────────────────────────────────────────────────────┘
```

**Show this warning when**:
- Creating a new upstream with `round_robin`, `least_conn`, or `random` policy
- Adding a second backend (not needed for single backend)

### 12. WebSocket and Long-Lived Connections
**Caddy Behavior**: Caddy properly handles WebSocket upgrade requests and maintains the connection to the same backend for the duration of that connection.

**Limitation**: If a client reconnects (new WebSocket connection), it may go to a different backend with `round_robin`.

**Recommendation**:
- For apps with WebSockets, recommend `ip_hash` policy
- Document in UI: "For WebSocket applications, use IP Hash policy to ensure reconnections go to the same backend"

**Future Enhancement**: Cookie-based sticky sessions (`lb_policy cookie`) would solve this better but requires more Caddy configuration.

### 13. Deployment Version Mismatch Warning
**Challenge**: When sites on different backends have different deployed versions, it can cause issues.

**Solution**: Track and display deployment status per backend.

**UI in Upstream Detail View**:
```
┌─────────────────────────────────────────────────────────────────────┐
│  Upstream: app.example.com                                           │
│  ────────────────────────────────────────────────────────────────── │
│                                                                      │
│  ⚠️  VERSION MISMATCH DETECTED                                       │
│  Backends are running different versions. This may cause            │
│  inconsistent behavior.                                              │
│                                                                      │
│  Backend Servers:                                                    │
│  ┌────────────────────────────────────────────────────────────────┐ │
│  │ ● Server 1 (192.168.1.10)                                      │ │
│  │   Commit: abc123d "Fix login bug"                              │ │
│  │   Deployed: 2 hours ago                           ← Latest     │ │
│  ├────────────────────────────────────────────────────────────────┤ │
│  │ ● Server 2 (192.168.1.11)                                      │ │
│  │   Commit: def456a "Add feature X"                              │ │
│  │   Deployed: 3 days ago                            ⚠️ Outdated  │ │
│  └────────────────────────────────────────────────────────────────┘ │
│                                                                      │
│  [Deploy to All Backends]  (Future feature)                         │
└─────────────────────────────────────────────────────────────────────┘
```

**Data Required**:
- Store `last_deployed_at` and `last_deployed_commit` on sites table (may already exist)
- Query all backend sites when displaying upstream
- Compare commit hashes to detect mismatch

### 14. Health Check Endpoint Validation
**Challenge**: Backend sites may not have a `/health` endpoint, causing all health checks to fail.

**UI Behavior - When Adding Backend**:
```
┌─────────────────────────────────────────────────────────────────────┐
│  ⚠️  HEALTH CHECK ENDPOINT                                          │
│                                                                      │
│  The health check path is set to: /health                           │
│                                                                      │
│  Make sure your application has this endpoint returning HTTP 200.   │
│                                                                      │
│  For Laravel apps, add this route:                                  │
│  ┌────────────────────────────────────────────────────────────────┐ │
│  │  Route::get('/health', fn() => response('OK', 200));           │ │
│  └────────────────────────────────────────────────────────────────┘ │
│                                                                      │
│  Or use Laravel 11+'s built-in /up route and change the health     │
│  check path in upstream settings.                                   │
└─────────────────────────────────────────────────────────────────────┘
```

**Optional: Test Health Endpoint**
- Add "Test Health Endpoint" button that makes a request to `http://{backend_ip}/health`
- Show success/failure before adding backend
- This is optional - don't block adding backend if test fails (endpoint might only work after Caddyfile configured)

### 9. Site UI When Load Balanced

When viewing a site that's part of a load balancer:

```
┌──────────────────────────────────────────────────────────────────┐
│  Sites on Server 1                                               │
├──────────────────────────────────────────────────────────────────┤
│                                                                   │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │  app.example.com                                      ⚖️ LB  │ │
│  │  Laravel • PHP 8.3                                          │ │
│  │  ────────────────────────────────────────────────────────── │ │
│  │  ⚠️ This site is load balanced                               │ │
│  │                                                              │ │
│  │  Traffic flows through: Load Balancer 1                     │ │
│  │  Other backends: 2 servers                                  │ │
│  │                                                              │ │
│  │  [View in Load Balancer]  [Deploy]  [Settings]              │ │
│  └─────────────────────────────────────────────────────────────┘ │
│                                                                   │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │  api.example.com                                             │ │
│  │  Laravel • PHP 8.3                        (Normal site)      │ │
│  └─────────────────────────────────────────────────────────────┘ │
│                                                                   │
└──────────────────────────────────────────────────────────────────┘
```

### 10. Deployment for Load Balanced Sites

**Challenge**: When a site exists on multiple backends, how do deployments work?

**Options**:

**Option A: Independent Deployments (Recommended Initially)**
- Each site is still deployed independently via its own server
- User deploys to Server 1's site, then Server 2's site
- Simple, no coordination needed
- Risk: Sites can be out of sync

**Option B: Coordinated Deployment (Future Enhancement)**
- Deploy from load balancer context
- System deploys to all backends sequentially or in parallel
- Rolling deployment: take backend out, deploy, put back
- Complex but ensures consistency

**Initial Implementation**: Option A
- Keep existing deployment flow unchanged
- Sites are deployed independently
- Add UI warning about keeping sites in sync
- Future: Add "Deploy to All Backends" button in LB upstream view

### 11. Health Check Endpoint on Backend Sites

Backend sites need a health endpoint for the load balancer to check:

**Recommendation**:
- Default health check path: `/health` or `/up`
- For Laravel: Use built-in `/up` route (Laravel 11+) or add custom route
- Health endpoint should return 200 OK if app is healthy

```php
// routes/web.php (Laravel example)
Route::get('/health', function () {
    // Optional: check database, cache, etc.
    return response('OK', 200);
});
```

**Caddy Health Check**:
```
health_uri /health
health_interval 30s
health_timeout 10s
health_status 200
```

---

## Error Recovery Strategy

Adding/removing a backend involves multiple sequential jobs. If any job fails mid-way,
the system must handle partial state gracefully. Follow the existing callback pattern
(`OnSuccess`/`OnFailure`/`OnExpired` from `taskrunner.CallbackHandler`).

### Add Backend: Failure Scenarios

When `AddBackend()` dispatches jobs in this order:
1. Add firewall rule on backend server (port 8080)
2. Create :8080 Caddyfile on backend server
3. Remove original :80/:443 Caddyfile from backend
4. Update LB Caddyfile to include new backend

| Failure Point | State | Recovery |
|---------------|-------|----------|
| Step 1 fails (firewall) | Backend record exists, no Caddy changes | Mark backend as `pending`, retry job. Site still serves normally on 80/443. |
| Step 2 fails (:8080 Caddyfile) | Firewall rule exists, no Caddy | Mark backend as `pending`. Firewall rule is harmless (port 8080 open but nothing listening). Retry job. |
| Step 3 fails (remove :80/:443) | :8080 Caddyfile exists AND :80/:443 still active | Both ports serve the site. Not harmful — site works via both LB and direct. Retry removal. |
| Step 4 fails (LB Caddyfile) | Backend ready but LB not routing to it | LB doesn't send traffic yet. Retry job. Previous backends still work. |

**Key principle**: Each step is idempotent. Retrying a step that already succeeded is safe
(firewall rule already exists → UFW ignores, Caddyfile already present → overwrite is fine).

### Remove Backend: Failure Scenarios

When `RemoveBackend()` dispatches jobs:
1. Remove firewall rule on backend server
2. Remove :8080 Caddyfile from backend
3. Restore original :80/:443 Caddyfile
4. Update LB Caddyfile to remove backend

| Failure Point | State | Recovery |
|---------------|-------|----------|
| Step 1 fails (firewall) | Backend record deleted, firewall rule remains | Orphaned rule for port 8080. Harmless but should be cleaned up. Log warning. |
| Step 2 fails (remove :8080) | Firewall removed, :8080 still serves | :8080 Caddy block remains but firewall blocks access. Retry removal. |
| Step 3 fails (restore :80/:443) | :8080 removed, original not restored | **Critical**: Site has no Caddy config at all. Retry immediately. On persistent failure, notify user. |
| Step 4 fails (LB Caddyfile) | Backend restored but LB still routes to it | LB sends traffic to :8080 which no longer exists → Caddy health check marks it unhealthy. Self-healing. |

### Implementation: Task Callbacks

Each multi-step operation uses task callbacks to track state:

```go
// Example: AddBackendCaddyfile task callback
type addBackendCaddyfileCallback struct {
    BackendID  string `json:"backend_id"`
    UpstreamID string `json:"upstream_id"`
    SiteID     string `json:"site_id"`
    ServerID   string `json:"server_id"`
}

func (t *addBackendCaddyfileTask) OnSuccess(ctx context.Context, cbCtx *taskrunner.CallbackContext, taskID string) error {
    // Mark backend as installed
    cbCtx.DB.Model(&models.LoadBalancerBackend{}).
        Where("id = ?", t.callback.BackendID).
        Update("installed_at", time.Now())

    // Broadcast success
    cbCtx.BroadcastToTeam(t.callback.TeamID, broadcast.BackendCaddyInstalled, map[string]any{
        "backend_id":  t.callback.BackendID,
        "upstream_id": t.callback.UpstreamID,
    })

    // Dispatch next step: remove original :80/:443 Caddyfile
    t.dispatchRemoveOriginalCaddyfile(cbCtx)

    return nil
}

func (t *addBackendCaddyfileTask) OnFailure(ctx context.Context, cbCtx *taskrunner.CallbackContext, taskID string, exitCode int) error {
    // Mark backend status as failed
    cbCtx.DB.Model(&models.LoadBalancerBackend{}).
        Where("id = ?", t.callback.BackendID).
        Update("health_status", "install_failed")

    // Broadcast failure
    cbCtx.BroadcastToTeam(t.callback.TeamID, broadcast.UpstreamInstallFailed, map[string]any{
        "backend_id":  t.callback.BackendID,
        "upstream_id": t.callback.UpstreamID,
        "exit_code":   exitCode,
    })

    // Notify team
    cbCtx.Notifier.NotifyTeam(t.callback.TeamID, notification.LBBackendInstallFailed{
        BackendID: t.callback.BackendID,
        ExitCode:  exitCode,
    })

    return nil
}
```

### Cleanup Job for Orphaned State

If a load balancer server is deleted while jobs are still queued, orphaned state may remain
on backend servers (firewall rules, :8080 Caddyfile blocks). A cleanup job handles this:

```go
// server:cleanup_lb_backends - dispatched when deleting a LB server
type CleanupLBBackendsJob struct {
    BackendSiteIDs []string // All sites that were backends
    LBServerIP     string   // For removing firewall rules
}

func (j *CleanupLBBackendsJob) Process(ctx context.Context) error {
    for _, siteID := range j.BackendSiteIDs {
        // 1. Remove firewall rule for port 8080
        // 2. Remove :8080 Caddyfile
        // 3. Restore original Caddyfile on 80/443
        // 4. Clear load_balanced_upstream_id on site
    }
    return nil
}
```

### Retry Policy

LB jobs use the default Asynq retry policy:
- **Max retries**: 3
- **Retry delay**: Exponential backoff (10s, 30s, 90s)
- **Dead letter**: After max retries, job moves to dead queue
- **Monitoring**: Dead letter jobs trigger notification to team

---

## Security Considerations

1. **Backend Communication**
   - TLS between load balancer and backends
   - Optional mutual TLS authentication
   - Private network preferred

2. **Health Check Endpoints**
   - Rate limiting on health endpoints
   - Authentication option for health checks
   - Don't expose sensitive info in health responses

3. **Access Control**
   - Only PHP servers from same team can be backends
   - Load balancer server owner controls upstreams

4. **Firewall Rules**
   - Auto-configure firewall on backends to allow LB IP
   - Restrict direct access to backends (optional)

---

## WebSocket Events

Load balancer operations broadcast events via the existing WebSocket system.
Follow the pattern in `internal/pkg/broadcast/events.go`.

### New Event Constants

**File**: `internal/pkg/broadcast/events.go`

```go
// Load Balancer Events
const (
    // Upstream lifecycle
    UpstreamCreated        = "upstream.created"
    UpstreamUpdated        = "upstream.updated"
    UpstreamDeleted        = "upstream.deleted"
    UpstreamInstalled      = "upstream.installed"       // Caddyfile applied on LB
    UpstreamInstallFailed  = "upstream.install_failed"

    // Backend lifecycle
    BackendAdded           = "backend.added"
    BackendRemoved         = "backend.removed"
    BackendUpdated         = "backend.updated"
    BackendCaddyInstalled  = "backend.caddy_installed"  // :8080 Caddyfile applied
    BackendCaddyRemoved    = "backend.caddy_removed"    // :8080 Caddyfile removed
    BackendRestored        = "backend.restored"         // Original Caddyfile restored on 80/443

    // Health
    BackendHealthChanged   = "backend.health_changed"   // Health status transition
    BackendMarkedDown      = "backend.marked_down"      // Manually toggled down
    BackendMarkedUp        = "backend.marked_up"        // Manually toggled up
)
```

### Broadcasting Pattern

Events are broadcast to the **team channel** (`team.{teamID}`) so all team members
see real-time updates. Follow existing patterns:

```go
// In LoadBalancerService or job callbacks
s.Broadcaster().BroadcastToTeam(upstream.TeamID, broadcast.UpstreamCreated, map[string]any{
    "upstream_id": upstream.ID,
    "server_id":   upstream.ServerID,
    "address":     upstream.Address,
})

// Backend health changes also broadcast to the server channel
s.Broadcaster().BroadcastToServer(backend.ServerID, broadcast.BackendHealthChanged, map[string]any{
    "backend_id":    backend.ID,
    "upstream_id":   backend.UpstreamID,
    "health_status": newStatus,
    "previous":      oldStatus,
})
```

### When Events Fire

| Event | Triggered By | Channel |
|-------|-------------|---------|
| `upstream.created` | `CreateUpstream()` service method | `team.{id}` |
| `upstream.installed` | Upstream Caddyfile job `OnSuccess` callback | `team.{id}` |
| `upstream.install_failed` | Upstream Caddyfile job `OnFailure` callback | `team.{id}` |
| `upstream.updated` | `UpdateUpstream()` service method | `team.{id}` |
| `upstream.deleted` | `DeleteUpstream()` service method | `team.{id}` |
| `backend.added` | `AddBackend()` service method | `team.{id}` |
| `backend.caddy_installed` | Backend :8080 Caddyfile job `OnSuccess` callback | `team.{id}`, `server.{id}` |
| `backend.removed` | `RemoveBackend()` service method | `team.{id}` |
| `backend.restored` | Restore original Caddyfile job `OnSuccess` callback | `team.{id}`, `server.{id}` |
| `backend.health_changed` | Health check polling job | `team.{id}`, `server.{id}` |
| `backend.marked_down` | `ToggleBackendDown()` handler | `team.{id}` |

### Frontend WebSocket Handling

```typescript
// In LoadBalancerUpstreams.vue composable
const channel = `team.${teamId}`;

ws.on(channel, 'upstream.created', (data) => {
    upstreams.value.push(data.upstream);
});

ws.on(channel, 'upstream.installed', (data) => {
    const upstream = upstreams.value.find(u => u.id === data.upstream_id);
    if (upstream) upstream.installed_at = data.installed_at;
});

ws.on(channel, 'backend.health_changed', (data) => {
    const backend = findBackend(data.backend_id);
    if (backend) backend.health_status = data.health_status;
});
```

---

## Future Enhancements

1. **High Availability Load Balancer**
   - Multiple load balancer servers sharing configuration
   - Floating IP support for failover
   - Active-passive or active-active setup
   - Currently: Single load balancer is supported; for HA, use cloud provider's LB in front

2. **Coordinated Deployment**
   - "Deploy to All Backends" button in upstream view
   - Rolling deployment: take backend out, deploy, health check, put back
   - Automatic version sync across backends

3. **Geographic Load Balancing**
   - Route based on client location
   - Multi-region deployments

4. **Advanced Health Checks**
   - Custom health check scripts
   - Database connectivity checks
   - Queue health monitoring

5. **Auto-scaling Integration**
   - Automatically add/remove backends
   - Scale based on metrics

6. **Blue-Green Deployments**
   - Zero-downtime deploys via load balancer
   - Traffic shifting between versions

7. **A/B Testing**
   - Route percentage of traffic to different backends
   - Canary deployments

8. **Cookie-Based Sticky Sessions**
   - Caddy supports `lb_policy cookie`
   - Better than IP hash for users behind NAT/proxies

---

## Load Balancer Provisioning Details

### Provisioning Steps (System-Level)

Server provisioning runs the same base provision steps regardless of server type
(defined in `types/provision_step.go` → `ForFreshServer()`). All 8 steps apply to load balancers:

| Order | Step | Template | LB Notes |
|-------|------|----------|----------|
| 1 | Configure Swap | `provision/configure_swap.sh` | Same — swap based on memory |
| 2 | Configure Firewall | `provision/configure_firewall.sh` | Same — opens SSH (22), HTTP (80), HTTPS (443) |
| 3 | Apt Update & Upgrade | `provision/apt_update_upgrade.sh` | Same |
| 4 | Install Essential Packages | `provision/install_essential_packages.sh` | Same — curl, git, fail2ban, ufw, etc. |
| 5 | Setup Unattended Upgrades | `provision/setup_unattended_upgrades.sh` | Same |
| 6 | Setup Root | `provision/setup_root.sh` | Same — SSH keys, git keyscan |
| 7 | SSH Security | `provision/ssh_security.sh` | Same — disable password auth |
| 8 | Setup Default User | `provision/setup_default_user.sh` | Same — create user, SSH keys, working directory |

No per-server-type variation in provision steps — the differentiation is **only in the software stack**.

### Current PHP Server Provisioning

For reference, the current PHP server installs these services:
```
PHP Server Services:
├── Supervisor (process manager)      [Install order: 10]
├── Caddy 2 (web server)              [Install order: 20]
├── PHP (8.3 default)                 [Install order: 30]
├── Composer 2                        [Install order: 40, requires PHP]
├── Database (MySQL 8.0 or PG 16)     [Install order: 50]
└── Launch Agent (optional)           [Install order: 100]
```

### Load Balancer Server Services

Load balancer needs minimal services:
```
Load Balancer Services:
├── Caddy 2 LB (reverse proxy)       [Install order: 20, uses install_caddy2_loadbalancer.sh]
└── Launch Agent (optional)           [Install order: 100]
```

**NOT installed**: Supervisor, PHP, Composer, Database, Redis, Node

### Software Stack Determination

The provisioning task builds the software stack from `InstalledService` records
(created by `createServicesForServer` during server creation). The task's
`ProvisionFreshServerConfig` includes:

```go
type ProvisionFreshServerConfig struct {
    // ... other fields ...
    SoftwareStack []types.Software  // Built from server's services
}
```

For a load balancer, `SoftwareStack` will contain:
- `types.SoftwareCaddy2LB` (always)
- `types.SoftwareLaunchAgent` (if agent enabled)

The `SortSoftwareStack()` function handles dependency ordering — Caddy2LB has no
dependencies so it installs at order 20, same as regular Caddy.

### Progress Tracking

The provisioning task calculates progress as:
- Total steps = 8 (provision steps) + len(SoftwareStack) = 8 + 2 = **10 steps**
- Steps 1-8: Provision steps (0-72%)
- Steps 9-10: Software install (72-90%)
- Final: Cleanup (90-100%)

Progress is reported via `BashEchoProgress()` markers parsed by the task runner.

### Service Creation Function

**File**: `internal/modules/server/services/server_service.go`

Add case for load balancer in `createServicesForServer`:

```go
func (s *Service) createServicesForServer(ctx context.Context, server *models.Server, req *dto.CreateServerRequest) error {
    serverType, _ := types.ParseServerType(req.Type)

    switch serverType {
    case types.ServerTypeDatabase:
        return s.createDatabaseServerServices(ctx, server, req)
    case types.ServerTypeLoadBalancer:  // NEW
        return s.createLoadBalancerServerServices(ctx, server, req)
    default:
        return s.createPhpServerServices(ctx, server, req)
    }
}

// createLoadBalancerServerServices creates services for a Load Balancer server type
func (s *Service) createLoadBalancerServerServices(ctx context.Context, server *models.Server, req *dto.CreateServerRequest) error {
    // Load balancer only needs Caddy configured for reverse proxy
    // Use a special software type or flag to indicate LB mode
    if err := s.createService(ctx, server.ID, types.SoftwareCaddy2LB); err != nil {
        return fmt.Errorf("failed to create Caddy LB service: %w", err)
    }

    // Add Launch Agent if not explicitly disabled
    if req.InstallAgent == nil || *req.InstallAgent {
        if err := s.createService(ctx, server.ID, types.SoftwareLaunchAgent); err != nil {
            return fmt.Errorf("failed to create Launch Agent service: %w", err)
        }
    }

    return nil
}
```

### Option 1: New Caddy Script for Load Balancer

**File**: `internal/modules/server/tasks/templates/software/install_caddy2_loadbalancer.sh`

```bash
#!/bin/bash
{{ shellDefaults }}

{{ aptFunctions }}

echo "Install Caddy webserver (Load Balancer mode)"

waitForAptUnlock
sudo rm -f /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --batch --yes --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
waitForAptUnlock
sudo apt-get update
waitForAptUnlock
sudo apt-get install -y caddy=2.*

echo "Configure Caddy for Load Balancer mode"

# Create upstreams directory
sudo mkdir -p /etc/caddy/upstreams

# Set up the Upstreams import file (different from Sites.caddy)
sudo tee /etc/caddy/Upstreams.caddy > /dev/null <<EOF
# Load balancer upstream configurations
# import /etc/caddy/upstreams/example_upstream.caddy
EOF

# Create load balancer Caddyfile (different structure from PHP server)
sudo tee /etc/caddy/Caddyfile > /dev/null <<EOF
{
    # Global options for load balancer
    admin off

    # Enable access logging
    log {
        output file /var/log/caddy/access.log {
            roll_size 100mb
            roll_keep 10
        }
    }
}

# Default handler for server IP (health check endpoint)
{{ .PublicIPv4 }}:80 {
    respond /health "OK" 200
    respond "Launch Load Balancer" 200
}

# Import all upstream configurations
import /etc/caddy/Upstreams.caddy
EOF

# Create log directory
sudo mkdir -p /var/log/caddy
sudo chown {{ .Username }}:{{ .Username }} /var/log/caddy

echo "Update Caddy service config to run as user"

sudo service caddy stop
sudo mkdir -p /etc/systemd/system/caddy.service.d

sudo tee /etc/systemd/system/caddy.service.d/override.conf > /dev/null <<EOF
[Service]
User={{ .Username }}
Group={{ .Username }}
EOF

# Reload systemd configuration and start Caddy service
sudo systemctl daemon-reload
sudo service caddy start

# Add caddy reload command to sudoers file without requiring password
sudo tee -a /etc/sudoers.d/caddy > /dev/null <<EOF
{{ .Username }} ALL=(root) NOPASSWD: /usr/sbin/service caddy reload
EOF

echo "Caddy Load Balancer installation complete"
```

### Option 2: Parameterized Existing Script

Alternatively, modify `install_caddy2.sh` to accept a mode parameter:

```bash
#!/bin/bash
{{ shellDefaults }}

{{ aptFunctions }}

CADDY_MODE="{{ .CaddyMode }}"  # "sites" or "loadbalancer"

echo "Install Caddy webserver (Mode: ${CADDY_MODE})"

# ... installation steps same ...

if [ "$CADDY_MODE" = "loadbalancer" ]; then
    echo "Configure Caddy for Load Balancer mode"

    sudo mkdir -p /etc/caddy/upstreams

    sudo tee /etc/caddy/Upstreams.caddy > /dev/null <<EOF
# Load balancer upstream configurations
EOF

    sudo tee /etc/caddy/Caddyfile > /dev/null <<EOF
{
    admin off
}

{{ .PublicIPv4 }}:80 {
    respond /health "OK" 200
    respond "Launch Load Balancer" 200
}

import /etc/caddy/Upstreams.caddy
EOF

else
    echo "Configure Caddy for Sites mode"

    sudo tee /etc/caddy/Sites.caddy > /dev/null <<EOF
# import /home/{{ .Username }}/example.com/Caddyfile
EOF

    sudo tee /etc/caddy/Caddyfile > /dev/null <<EOF
{{ .PublicIPv4 }}:80 {
    root * /home/{{ .Username }}/default
    file_server
}

import /etc/caddy/Sites.caddy
EOF
fi

# ... rest of script same ...
```

### New Software Type: `SoftwareCaddy2LB`

**File**: `internal/modules/server/types/software.go`

Adding a new `Software` enum requires implementing **all** methods. The Caddy binary is identical
but the install script creates `/etc/caddy/Upstreams.caddy` instead of `/etc/caddy/Sites.caddy`.

**Changes to existing methods** (add `SoftwareCaddy2LB` cases):

```go
// Add constant
const (
    SoftwareCaddy2   Software = "caddy2"
    SoftwareCaddy2LB Software = "caddy2_lb"  // NEW: Load Balancer mode
    // ...
)

// Label() - add case
func (s Software) Label() string {
    labels := map[Software]string{
        // ... existing ...
        SoftwareCaddy2LB: "Caddy 2 (Load Balancer)",  // NEW
    }
    // ...
}

// IsValid() - add to switch
func (s Software) IsValid() bool {
    switch s {
    case SoftwareCaddy2, SoftwareCaddy2LB, /* ... existing ... */:  // ADD
        return true
    }
    return false
}

// GetVersion() - same version as Caddy2
func (s Software) GetVersion() string {
    versions := map[Software]string{
        // ... existing ...
        SoftwareCaddy2LB: "2.0",  // NEW - same binary version
    }
    // ...
}

// GetServiceType() - maps to same Caddy service type
func (s Software) GetServiceType() ServiceType {
    switch s {
    // ... existing ...
    case SoftwareCaddy2, SoftwareCaddy2LB:  // ADD to existing Caddy case
        return ServiceTypeCaddy
    }
    return ""
}

// Group() - same group as Caddy
func (s Software) Group() string {
    switch s {
    // ... existing ...
    case SoftwareCaddy2, SoftwareCaddy2LB:  // ADD to existing Caddy case
        return "caddy"
    }
    return ""
}

// LogPath() - same log path
func (s Software) LogPath() string {
    paths := map[Software]string{
        // ... existing ...
        SoftwareCaddy2LB: "/var/log/caddy/access.log",  // NEW - same path
    }
    // ...
}

// InstallTemplateName() - different install script
func (s Software) InstallTemplateName() string {
    templateNames := map[Software]string{
        // ... existing ...
        SoftwareCaddy2:   "software/install_caddy2.sh",
        SoftwareCaddy2LB: "software/install_caddy2_loadbalancer.sh",  // NEW
    }
    // ...
}

// RemoveTemplateName() - same removal as regular Caddy
func (s Software) RemoveTemplateName() string {
    // SoftwareCaddy2LB falls through to default: "software/remove_caddy2_lb.sh"
    // Or handle explicitly to reuse caddy2 removal
}

// InstallOrder() - same priority as Caddy (20)
func (s Software) InstallOrder() int {
    orders := map[Software]int{
        // ... existing ...
        SoftwareCaddy2LB: 20,  // NEW - same as Caddy2
    }
    // ...
}

// AllSoftware() - add to list
func AllSoftware() []Software {
    return []Software{
        SoftwareCaddy2, SoftwareCaddy2LB, /* ... rest ... */  // ADD
    }
}
```

**No changes needed** for these methods (not applicable to Caddy):
- `IsPhp()` — returns false (no change needed)
- `IsDatabase()` — returns false (no change needed)
- `MaxConnections()` — only for databases (no change needed)
- `MaxChildren()` — only for PHP (no change needed)
- `BinaryPath()` — only for PHP (no change needed)
- `FPMServiceName()` — only for PHP (no change needed)
- `RequiresPhp()` — returns false (no change needed)
- `SortSoftwareStack()` — no special dependencies (no change needed)

### Firewall Rules

**Load Balancer Server** (same as PHP server, no changes needed):
- Port 22 (SSH)
- Port 80 (HTTP - for Let's Encrypt validation)
- Port 443 (HTTPS - main traffic)

**Backend PHP Servers** (auto-managed when adding/removing backends):
- Port 8080: `allow from <LB_IP> to any port 8080` — added when site is added as backend
- Rule is removed when site is removed from upstream
- This is handled by `dispatchAddBackendFirewallRule` and `dispatchRemoveBackendFirewallRule`

### Comparison: PHP Server vs Load Balancer Provisioning

| Aspect | PHP Server | Load Balancer |
|--------|------------|---------------|
| **Caddy Config** | `/etc/caddy/Sites.caddy` | `/etc/caddy/Upstreams.caddy` |
| **Caddyfile Structure** | File server + PHP-FPM on :80/:443 | Reverse proxy to backends on :8080 |
| **Services Installed** | Supervisor, Caddy, PHP, Composer, DB | Caddy only |
| **User Home** | Sites deployed here | Not used for sites |
| **Default Page** | File server on IP:80 | "Launch Load Balancer" response |
| **Health Endpoint** | None (site-level) | `/health` on IP:80 |

---

## Summary of Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| **Backend Granularity** | Site-level (not server-level) | Allows mixed scenarios where some sites on a server are load balanced and others are served directly |
| **Domain Conflict Detection** | Check on upstream creation | Prevents accidental overwriting; shows user which sites will be affected |
| **Backend Caddy** | Dedicated port 8080 (not modify 80/443) | Separate port avoids conflicts with direct sites; `http://domain:8080` with IP restriction |
| **Backend Port** | Port 8080 (configurable per backend) | Dedicated port for LB traffic; firewall rule auto-created; no TLS overhead |
| **Deployment Strategy** | Independent per-site (initially) | Simple, uses existing flow; coordinated deployment is a future enhancement |
| **Health Check** | Caddy built-in with configurable path | No custom agent needed; uses standard HTTP health checks on :8080 |
| **TLS Termination** | At load balancer only | Backend uses plain HTTP on :8080; firewall + IP restriction ensures security |
| **Site Deletion** | Block when load balanced | Prevent orphaned configs; user must remove from LB first |
| **Zero Backends** | Allow but show warnings | Return 503 placeholder; don't block removal |
| **Session Warning** | UI warning only | Inform users about ip_hash or Redis sessions; no forced changes |
| **Version Mismatch** | UI warning in upstream view | Show commit differences; future: coordinated deploy |
| **Task Concurrency** | Queue system handles | All Caddyfile updates go through job queue; serialized per server |
| **HA Load Balancer** | Single LB initially | Future enhancement; use cloud LB for HA today |

### What Happens When...

| Scenario | System Behavior |
|----------|-----------------|
| **Create upstream with existing domain** | UI shows warning, offers to auto-add sites as backends |
| **Add site as backend** | Caddyfile created on :8080 (IP restricted), original :80/443 removed, firewall rule added, LB Caddyfile updated |
| **Remove backend** | :8080 Caddyfile removed, original site Caddyfile restored on :80/443, firewall rule removed, LB Caddyfile updated |
| **Remove last backend** | Warning shown; upstream returns 503; site restored to normal mode |
| **Delete load balancer** | All backend sites restored to normal (:8080 removed, :80/443 restored), firewall rules removed, upstreams deleted |
| **Try to delete load balanced site** | **Blocked** - user must remove from load balancer first |
| **Delete backend server** | All its sites removed from upstreams, LB Caddyfiles updated |
| **Deploy to load balanced site** | Normal deployment; other backends NOT automatically updated; version mismatch warning shown |
| **Health check fails** | Caddy stops routing to that backend on :8080; UI shows unhealthy status |
| **Direct access to backend :8080** | Blocked by firewall (UFW) + Caddy IP restriction (403 Forbidden) |
| **WebSocket connection** | Caddy maintains connection to same backend; reconnects may go elsewhere |
| **Use round_robin with sessions** | Sessions may be lost; UI warns to use ip_hash or Redis sessions |

### File Changes Summary

| File/Location | Changes |
|---------------|---------|
| **Backend - Types & Models** | |
| `internal/modules/server/types/types.go` | Add `ServerTypeLoadBalancer`, `ServerFeatureLoadBalancing`, update `Label()`, `IsValid()`, `GetFeatures()`, `GetProcessManager()`, `AllServerTypes()` |
| `internal/modules/server/types/software.go` | Add `SoftwareCaddy2LB` with all ~15 methods |
| `internal/modules/server/types/load_balancer.go` | New file: `LBPolicy` enum |
| `internal/modules/server/models/` | New: `LoadBalancerUpstream`, `LoadBalancerBackend` |
| `internal/modules/server/models/server.go` | Add `UpstreamsCount` computed field |
| `internal/modules/site/models/site.go` | Add `LoadBalancedUpstreamID` |
| `internal/modules/server/dto/requests.go` | Add `loadbalancer` to `Type` validation |
| `internal/modules/server/dto/responses.go` | Add `UpstreamsCount` to `ServerResponse` |
| **Backend - Repositories** | |
| `internal/modules/server/repositories/registry.go` | Add `lbUpstream`, `lbBackend` fields and accessors |
| `internal/modules/server/repositories/load_balancer_upstream_repository.go` | **New**: upstream CRUD |
| `internal/modules/server/repositories/load_balancer_backend_repository.go` | **New**: backend CRUD |
| **Backend - Contracts & Cross-Module** | |
| `internal/modules/server/contracts/site_reader.go` | **New**: `SiteReader` interface for cross-module access |
| `internal/modules/server/handlers/server_handler.go` | Add `UpstreamCounter` interface |
| `cmd/api/main.go` | Wire `SetSiteReader()` injection |
| **Backend - Services & Handlers** | |
| `internal/modules/server/services/server_service.go` | Add `createLoadBalancerServerServices()` function |
| `internal/modules/server/services/load_balancer_service.go` | **New**: `LoadBalancerService` |
| `internal/modules/server/handlers/` | New: upstream/backend handlers |
| `internal/modules/server/routes.go` | Register new endpoints |
| **Backend - Provisioning Scripts** | |
| `internal/modules/server/tasks/templates/software/install_caddy2_loadbalancer.sh` | **New**: Caddy install script for LB mode |
| **Backend - Jobs & Events** | |
| `internal/modules/site/jobs/install_caddyfile.go` | Modify to generate :8080 Caddyfile block for load balanced sites |
| `internal/modules/server/jobs/` | New: LB Caddyfile install/update/remove, firewall add/remove, cleanup jobs |
| `internal/modules/server/jobs/lb_deps.go` | **New**: `LBJobDeps` struct |
| `internal/modules/server/jobs/register.go` | Register LB job handlers |
| `internal/pkg/broadcast/events.go` | Add upstream/backend WebSocket event constants |
| **Frontend** | |
| `client/types/index.ts` | Add `LoadBalancerUpstream`, `LoadBalancerBackend` types |
| `client/components/server/` | New: `LoadBalancerUpstreams`, `HealthIndicator`, etc. |
| `client/pages/servers/[id]/index.vue` | Conditional tabs for load balancer type |
| `client/pages/servers/create.vue` | Add load balancer type option |
