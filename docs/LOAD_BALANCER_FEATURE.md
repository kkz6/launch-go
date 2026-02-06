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
                          └─────────────┬───────────────────┘
                                        │
                    ┌───────────────────┼───────────────────┐
                    ▼                                       ▼
        ┌───────────────────────┐               ┌───────────────────────┐
        │      PHP Server 1      │               │      PHP Server 2      │
        ├───────────────────────┤               ├───────────────────────┤
        │ app.example.com       │◄──────────────│ app.example.com       │
        │ (Caddy disabled,      │   LB Traffic  │ (Caddy disabled,      │
        │  PHP-FPM only)        │               │  PHP-FPM only)        │
        ├───────────────────────┤               ├───────────────────────┤
        │ api.example.com       │               │ blog.example.com      │
        │ (Caddy active,        │               │ (Caddy active,        │
        │  serves directly)     │               │  serves directly)     │
        └───────────────────────┘               └───────────────────────┘
```

### What Happens to Backend Site's Caddy?

**Important**: Backend Caddy keeps running, but the Caddyfile IS modified:

1. **PHP-FPM uses Unix sockets**, not HTTP
2. Load balancer can only proxy HTTP requests
3. Backend Caddy is needed to translate HTTP → FastCGI → PHP-FPM

**Traffic flow**:
```
Client → Load Balancer (Caddy) → Backend Server (Caddy) → PHP-FPM
                                  ↑
                                  Host: app.example.com header
```

### Backend Caddyfile Changes

When a site is added to a load balancer, its Caddyfile is regenerated:

**Before (normal mode)**:
```caddyfile
app.example.com {
    # Caddy auto-manages TLS
    root * /home/launch/app.example.com/current/public
    php_fastcgi unix//run/php/php8.3-fpm.sock
    file_server
}
```

**After (load balanced mode)**:
```caddyfile
http://app.example.com {
    # http:// prefix disables auto-TLS (LB handles SSL)

    # Only accept requests from load balancer
    @notlb not remote_ip {{ .LoadBalancerIP }}
    respond @notlb "Forbidden" 403

    root * /home/launch/app.example.com/current/public
    php_fastcgi unix//run/php/php8.3-fpm.sock
    file_server
}
```

**Key changes**:
1. **`http://` prefix** - Disables Caddy's auto-TLS (no Let's Encrypt cert)
2. **IP restriction** - Only accepts requests from load balancer IP
3. **Domain kept** - Matches the `Host` header sent by load balancer

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

### Security: IP Restriction

The backend only accepts requests from the load balancer:
- Direct access to backend IP returns 403 Forbidden
- Prevents bypassing the load balancer
- Load balancer IP is stored in upstream config

### Failover Capability

If the load balancer fails:
1. Regenerate backend Caddyfile to normal mode (remove `http://`, remove IP restriction)
2. Update DNS to point directly to backend
3. Or: Add secondary load balancer for HA

---

## Domain Conflict Detection and Migration

### Scenario: Creating Upstream for Existing Site

When creating an upstream with address `app.example.com`:

1. **System searches** for existing sites with that address in the team
2. **If found**, UI shows:
   - Which servers have this site installed
   - Warning that Caddyfile will be removed from those servers
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
│  │    └─ Caddyfile will be removed, PHP-FPM will continue      │    │
│  │                                                               │    │
│  │  ☑ Server 2 (192.168.1.11) - app.example.com                 │    │
│  │    └─ Caddyfile will be removed, PHP-FPM will continue      │    │
│  │                                                               │    │
│  │  These sites will become backends for this upstream.         │    │
│  │  Traffic will flow: Client → Load Balancer → Backend Sites   │    │
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
│  │  • Remove Caddyfile from Server 2                            │    │
│  │  • Route traffic through Load Balancer                       │    │
│  │  • Keep PHP-FPM running for application requests             │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                      │
│  Weight:        [1                                             ]    │
│  Backup Only:   [ ] Use only when primary backends are down        │
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
    weight          INT DEFAULT 1,                  -- Traffic weight (1-100)
    max_fails       INT DEFAULT 3,                  -- Failures before marking unhealthy
    fail_timeout    VARCHAR(20) DEFAULT '30s',      -- How long to mark as unhealthy
    is_backup       BOOLEAN DEFAULT FALSE,          -- Only use when others are down
    is_down         BOOLEAN DEFAULT FALSE,          -- Manually marked as down

    -- Health status (updated by health checks)
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

```go
const (
    ServerTypePhp          ServerType = "php"
    ServerTypeDatabase     ServerType = "database"
    ServerTypeLoadBalancer ServerType = "loadbalancer"  // NEW
)

// Features for load balancer
var loadBalancerFeatures = []ServerFeature{
    ServerFeatureLoadBalancing,  // NEW feature
    ServerFeatureSSLCertificates,
    ServerFeatureServices,
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

    Weight      int    `gorm:"default:1"`
    MaxFails    int    `gorm:"default:3"`
    FailTimeout string `gorm:"size:20;default:30s"`
    IsBackup    bool   `gorm:"default:false"`
    IsDown      bool   `gorm:"default:false"`

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

```go
func (s *ServerService) createServicesForServer(ctx context.Context, server *models.Server, opts CreateServerOptions) error {
    switch server.Type {
    case servertypes.ServerTypePhp:
        return s.createPhpServerServices(ctx, server, opts)
    case servertypes.ServerTypeDatabase:
        return s.createDatabaseServerServices(ctx, server, opts)
    case servertypes.ServerTypeLoadBalancer:
        return s.createLoadBalancerServerServices(ctx, server, opts)  // NEW
    }
    return nil
}

func (s *ServerService) createLoadBalancerServerServices(ctx context.Context, server *models.Server, opts CreateServerOptions) error {
    // Load balancer only needs Caddy and Launch Agent
    services := []serviceData{
        {serviceType: servertypes.ServiceTypeCaddy, software: servertypes.SoftwareCaddy2},
    }

    if opts.InstallAgent {
        services = append(services, serviceData{
            serviceType: servertypes.ServiceTypeLaunchAgent,
            software:    servertypes.SoftwareLaunchAgent,
        })
    }

    return s.createServices(ctx, server.ID, services)
}
```

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
                Weight:   1,
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

    backend := &models.LoadBalancerBackend{
        UpstreamID:  upstreamID,
        SiteID:      req.SiteID,
        ServerID:    site.ServerID,
        Weight:      req.Weight,
        MaxFails:    req.MaxFails,
        FailTimeout: req.FailTimeout,
        IsBackup:    req.IsBackup,
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
    // 1. Regenerate backend site's Caddyfile (http:// mode, IP restricted)
    // 2. Update load balancer Caddyfile to include new backend
    s.dispatchRegenerateSiteCaddyfile(site, upstream.Server.PublicIPv4)
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
    // 1. Regenerate backend site's Caddyfile (normal mode, with TLS)
    // 2. Update load balancer Caddyfile to remove backend
    //
    // Note: Jobs are processed via the queue system (Asynq), which handles
    // concurrency. Multiple Caddyfile updates for the same server are
    // serialized through the queue, preventing race conditions.
    s.dispatchRegenerateSiteCaddyfile(site, "") // Empty LB IP = normal mode
    upstream, _ := s.Repos().LoadBalancerUpstream().FindByID(ctx, upstreamID)
    s.dispatchUpdateUpstream(upstream)

    return nil
}

// Note on Concurrency:
// All Caddyfile operations (install, update, remove) are dispatched as jobs
// to the queue system. The queue worker processes jobs sequentially per queue,
// ensuring that concurrent add/remove operations don't cause race conditions
// when regenerating Caddyfiles.
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
    # TLS configuration
    tls {
        # auto-managed by default
    }

    # Security headers
    header {
        -Server
        X-Content-Type-Options nosniff
        X-Frame-Options SAMEORIGIN
        X-Powered-By "Launch"
    }

    # Reverse proxy to backends with load balancing
    reverse_proxy {
        # Backend servers
        to 192.168.1.10:443 192.168.1.11:443

        # Load balancing policy
        lb_policy round_robin

        # Health checks
        health_uri /health
        health_interval 30s
        health_timeout 10s
        health_status 200

        # TLS to backends (if using HTTPS)
        transport http {
            tls
            tls_insecure_skip_verify  # For self-signed certs on backends
        }

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

    // Backend addresses
    var addrs []string
    for _, backend := range backends {
        if !backend.IsDown {
            addr := fmt.Sprintf("%s:443", backend.Server.PublicIPv4)
            if backend.IsBackup {
                addr += " # backup"
            }
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

    // TLS transport
    sb.WriteString(`        transport http {
            tls
            tls_insecure_skip_verify
        }

`)

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
                <span v-if="backend.is_backup" class="badge badge-yellow">Backup</span>
                <span class="text-sm">Weight: {{ backend.weight }}</span>
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
  weight: number;
  max_fails: number;
  fail_timeout: string;
  is_backup: boolean;
  is_down: boolean;
  health_status: 'healthy' | 'unhealthy' | 'unknown';
  last_health_check_at?: string;
}
```

---

## API Endpoints

### New Endpoints for Load Balancer Management

```
# Domain Conflict Detection (call before creating upstream)
GET    /api/servers/:serverId/upstreams/check-domain?address=app.example.com
       # Returns: { exists: true, sites: [...] } if domain is already used

# Upstreams
GET    /api/servers/:serverId/upstreams           # List upstreams for LB server
POST   /api/servers/:serverId/upstreams           # Create upstream
GET    /api/servers/:serverId/upstreams/:id       # Get upstream details
PUT    /api/servers/:serverId/upstreams/:id       # Update upstream config
DELETE /api/servers/:serverId/upstreams/:id       # Delete upstream

# Backends (site-level)
GET    /api/upstreams/:upstreamId/backends                 # List backend sites
POST   /api/upstreams/:upstreamId/backends                 # Add site as backend
PUT    /api/upstreams/:upstreamId/backends/:id             # Update backend config
DELETE /api/upstreams/:upstreamId/backends/:id             # Remove site from upstream

# Available Sites for Backend (sites matching upstream address, not yet added)
GET    /api/upstreams/:upstreamId/available-sites          # Sites that can be added

# Health
GET    /api/upstreams/:upstreamId/health                   # Get health status of all backends
POST   /api/upstreams/:upstreamId/backends/:id/toggle-down # Toggle backend up/down
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

---

## Implementation Phases

### Phase 1: Database & Models (Backend Foundation)
- [ ] Create migration for `load_balancer_upstreams` table
- [ ] Create migration for `load_balancer_backends` table (with `site_id`)
- [ ] Create migration to add `load_balanced_upstream_id` to `sites` table
- [ ] Create `LoadBalancerUpstream` model
- [ ] Create `LoadBalancerBackend` model (with site relation)
- [ ] Add `IsLoadBalanced()` method to Site model
- [ ] Create `LBPolicy` enum type
- [ ] Add `ServerTypeLoadBalancer` to server types
- [ ] Add `ServerFeatureLoadBalancing` feature

### Phase 2: Server Provisioning
- [ ] Update server creation to handle load balancer type
- [ ] Create `createLoadBalancerServerServices()` function
- [ ] Create `install_caddy2_loadbalancer.sh` script (or parameterize existing)
- [ ] Test load balancer server provisioning

**See: [Load Balancer Provisioning Details](#load-balancer-provisioning-details)**

### Phase 3: Upstream Management (Backend)
- [ ] Create `LoadBalancerUpstreamRepository`
- [ ] Create `LoadBalancerBackendRepository`
- [ ] Create `LoadBalancerService`
- [ ] Implement `FindExistingSites()` for domain conflict detection
- [ ] Create upstream handlers (CRUD)
- [ ] Create backend handlers (add/remove/update) - site-level
- [ ] Create domain check endpoint (`/check-domain`)
- [ ] Register routes

### Phase 4: Backend Site Caddyfile Changes
- [ ] Modify `generateCaddyfile()` to check `IsLoadBalanced()`
- [ ] Add `http://` prefix when load balanced (disables auto-TLS)
- [ ] Add IP restriction matcher when load balanced
- [ ] Store load balancer IP in site or fetch from upstream
- [ ] Test Caddyfile regeneration on add/remove backend

### Phase 5: Load Balancer Caddy Configuration
- [ ] Create `generateLBCaddyfile()` function
- [ ] Create `InstallUpstreamCaddyfile` job
- [ ] Create `UpdateUpstreamCaddyfile` job
- [ ] Create `RemoveUpstreamCaddyfile` job
- [ ] Create `/etc/caddy/Upstreams.caddy` import management
- [ ] Test Caddy configuration generation

### Phase 6: Frontend - Server List & Types
- [ ] Update server type display for load balancer
- [ ] Add load balancer badge/icon in server cards
- [ ] Add "load balanced" indicator for sites in site list
- [ ] Update TypeScript types for new models

### Phase 7: Frontend - Upstream Creation with Domain Detection
- [ ] Create domain check API call on address input (debounced)
- [ ] Show warning when existing sites found
- [ ] Display which servers have the site
- [ ] Checkbox to auto-add existing sites as backends
- [ ] DNS update warning display

### Phase 8: Frontend - Load Balancer Detail
- [ ] Create `LoadBalancerUpstreams` component
- [ ] Create `CreateUpstreamForm` component with domain detection
- [ ] Create `AddBackendForm` component (site selector)
- [ ] Create `HealthIndicator` component
- [ ] Update tab structure for load balancer servers

### Phase 9: Frontend - Site Updates
- [ ] Add "Load Balanced" badge to site cards
- [ ] Create `LoadBalancedBanner` component for site detail
- [ ] Show upstream info and link to load balancer
- [ ] Warning about deployment sync

### Phase 10: Health Checks & Monitoring
- [ ] Implement health check result storage
- [ ] Create scheduled job for health check polling
- [ ] WebSocket events for health status changes
- [ ] UI updates for real-time health status

### Phase 11: Testing & Polish
- [ ] Unit tests for services
- [ ] Integration tests for API endpoints
- [ ] Test domain conflict detection flow
- [ ] Test LB Caddyfile generation
- [ ] E2E tests for UI flows
- [ ] Documentation updates

---

## Edge Cases and Considerations

### 1. Backend Server Deletion
- When a PHP server is deleted that's part of an upstream:
  - Automatically remove from all upstreams
  - Update Caddyfile on load balancers
  - Send notification about reduced capacity

### 2. Load Balancer Deletion
- When a load balancer is deleted:
  - Clear `load_balanced_upstream_id` on all backend sites
  - Delete all associated upstreams and backends
  - Regenerate backend site Caddyfiles to normal mode
  - Notify affected team members

### 3. Backend Server Unreachable
- Health checks mark server as unhealthy
- Caddy automatically stops routing traffic
- UI shows health status
- Send notification after N consecutive failures

### 4. SSL Certificate Management
- Load balancer handles SSL termination by default
- Option for SSL passthrough to backends
- Certificate sync between load balancer and backends for passthrough

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

### Current PHP Server Provisioning

For reference, the current PHP server installs these services:
```
PHP Server Services:
├── Supervisor (process manager)
├── Caddy 2 (web server with Sites.caddy imports)
├── Database (MySQL 8.0 or PostgreSQL 16)
├── PHP (8.3 default)
├── Composer 2
└── Launch Agent (optional)
```

### Load Balancer Server Services

Load balancer needs minimal services:
```
Load Balancer Services:
├── Caddy 2 (reverse proxy with Upstreams.caddy imports)
└── Launch Agent (optional)
```

**NO**: Supervisor, PHP, Composer, Database

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

### New Software Type (Recommended)

**File**: `internal/modules/server/types/software.go`

Add a new software type for load balancer Caddy:

```go
const (
    // ... existing ...
    SoftwareCaddy2   Software = "caddy2"
    SoftwareCaddy2LB Software = "caddy2_lb"  // NEW: Load Balancer mode
)

func (s Software) GetInstallTemplate() string {
    switch s {
    case SoftwareCaddy2:
        return "software/install_caddy2.sh"
    case SoftwareCaddy2LB:
        return "software/install_caddy2_loadbalancer.sh"  // NEW
    // ...
    }
}
```

### Firewall Rules for Load Balancer

Load balancer needs:
- Port 22 (SSH)
- Port 80 (HTTP - for Let's Encrypt validation)
- Port 443 (HTTPS - main traffic)

Same as PHP server, so no changes needed to `CreateDefaultFirewallRules`.

### Comparison: PHP Server vs Load Balancer Provisioning

| Aspect | PHP Server | Load Balancer |
|--------|------------|---------------|
| **Caddy Config** | `/etc/caddy/Sites.caddy` | `/etc/caddy/Upstreams.caddy` |
| **Caddyfile Structure** | File server + PHP-FPM | Reverse proxy + health check |
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
| **Backend Caddy** | Modify Caddyfile (not disable) | Use `http://` prefix to disable TLS, add IP restriction, keep domain for Host matching |
| **Deployment Strategy** | Independent per-site (initially) | Simple, uses existing flow; coordinated deployment is a future enhancement |
| **Health Check** | Caddy built-in with configurable path | No custom agent needed; uses standard HTTP health checks |
| **TLS Termination** | At load balancer only | Backend uses HTTP (no TLS overhead); IP restriction ensures security |
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
| **Add site as backend** | Site Caddyfile regenerated (http://, IP restricted), LB Caddyfile updated |
| **Remove backend** | Site Caddyfile regenerated (normal mode with TLS), LB Caddyfile updated |
| **Remove last backend** | Warning shown; upstream returns 503; site restored to normal |
| **Delete load balancer** | All backend sites' Caddyfiles regenerated to normal, upstreams deleted |
| **Try to delete load balanced site** | **Blocked** - user must remove from load balancer first |
| **Delete backend server** | All its sites removed from upstreams, LB Caddyfiles updated |
| **Deploy to load balanced site** | Normal deployment; other backends NOT automatically updated; version mismatch warning shown |
| **Health check fails** | Caddy stops routing to that backend; UI shows unhealthy status |
| **Direct access attempt** | Returns 403 Forbidden (IP restriction in backend Caddyfile) |
| **WebSocket connection** | Caddy maintains connection to same backend; reconnects may go elsewhere |
| **Use round_robin with sessions** | Sessions may be lost; UI warns to use ip_hash or Redis sessions |

### File Changes Summary

| File/Location | Changes |
|---------------|---------|
| **Backend - Types & Models** | |
| `internal/modules/server/types/types.go` | Add `ServerTypeLoadBalancer`, `ServerFeatureLoadBalancing` |
| `internal/modules/server/types/software.go` | Add `SoftwareCaddy2LB` for load balancer Caddy |
| `internal/modules/server/types/load_balancer.go` | New file: `LBPolicy` enum |
| `internal/modules/server/models/` | New: `LoadBalancerUpstream`, `LoadBalancerBackend` |
| `internal/modules/site/models/site.go` | Add `LoadBalancedUpstreamID`, `CaddyfileDisabled` |
| **Backend - Services & Handlers** | |
| `internal/modules/server/services/server_service.go` | Add `createLoadBalancerServerServices()` function |
| `internal/modules/server/services/load_balancer_service.go` | New: `LoadBalancerService` |
| `internal/modules/server/handlers/` | New: upstream/backend handlers |
| `internal/modules/server/routes.go` | Register new endpoints |
| **Backend - Provisioning Scripts** | |
| `internal/modules/server/tasks/templates/software/install_caddy2_loadbalancer.sh` | **New**: Caddy install script for LB mode |
| **Backend - Caddy Jobs** | |
| `internal/modules/site/jobs/install_caddyfile.go` | Modify `generateCaddyfile()` to handle load balanced mode |
| `internal/modules/server/jobs/` | New: LB upstream Caddyfile install/update/remove jobs |
| **Frontend** | |
| `client/types/index.ts` | Add `LoadBalancerUpstream`, `LoadBalancerBackend` types |
| `client/components/server/` | New: `LoadBalancerUpstreams`, `HealthIndicator`, etc. |
| `client/pages/servers/[id]/index.vue` | Conditional tabs for load balancer type |
| `client/pages/servers/create.vue` | Add load balancer type option |
