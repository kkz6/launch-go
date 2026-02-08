# Load Balancer Setup Guide

This guide explains how to set up and configure a load balancer after provisioning.

## Prerequisites

- A provisioned **Load Balancer** server (server type: `loadbalancer`)
- One or more provisioned **application servers** with deployed sites
- All sites that will be load balanced must share the **same domain/address**

## Overview

The load balancer uses Caddy as a reverse proxy. Traffic flows as:

```
Client -> Load Balancer (Caddy) -> Backend Servers (port 8080)
```

When a site is added as a backend:
- The LB's Caddy config is updated with the backend server IP
- The backend site's Caddy config switches to port 8080 (internal only)
- A firewall rule is added on the backend server allowing traffic from the LB IP
- Health checks begin automatically

## Step-by-Step Setup

### 1. Create an Upstream

Navigate to your load balancer server and go to the **Upstreams** tab.

Click **Create Upstream** and configure:

| Field | Description | Default |
|-------|-------------|---------|
| **Name** | A label for this upstream (e.g., "Production API") | Required |
| **Address** | The domain name (e.g., `app.example.com`) | Required |
| **Port** | The port Caddy listens on for this upstream | `443` |
| **TLS Setting** | How TLS is handled | `auto` |
| **LB Policy** | Load balancing strategy | `round_robin` |
| **Health Check Path** | Endpoint for health checks | `/health` |
| **Health Check Interval** | How often to check backends | `30s` |
| **Health Check Timeout** | Timeout for each health check | `10s` |

**Auto-add existing sites**: If enabled, any existing sites on your servers with a matching address will be automatically added as backends.

### 2. Add Backends

After creating an upstream, add backend servers by clicking **Add Backend**.

| Field | Description | Default |
|-------|-------------|---------|
| **Site** | Select a site with the same address as the upstream | Required |
| **Port** | The internal port the backend listens on | `8080` |

When a backend is added, the system automatically:
- Updates the LB Caddyfile to route traffic to the new backend
- Reconfigures the backend site's Caddyfile to listen on port 8080 (restricted to LB IP only)
- Adds a UFW firewall rule on the backend server allowing the LB's IP

### 3. Verify Health Checks

Health checks run automatically every minute. You can view backend health status in the upstream detail page:

- **Healthy** - Backend responding with HTTP 200-399
- **Unhealthy** - Backend not responding or returning errors
- **Unknown** - Not yet checked

Health checks are routed through the load balancer's reverse proxy, not directly to backends.

## Load Balancing Policies

| Policy | Description |
|--------|-------------|
| **Round Robin** | Distributes requests evenly across all backends (default) |
| **Least Connections** | Routes to the backend with the fewest active connections |
| **IP Hash** | Sticky sessions - same client IP always goes to the same backend |
| **First Available** | Always uses the first available backend |
| **Random** | Random backend selection |

## TLS Settings

| Setting | Description |
|---------|-------------|
| **Auto** | Caddy automatically manages TLS certificates (recommended) |
| **Internal** | Uses Caddy's internal CA |
| **Off** | No TLS encryption |
| **Custom** | User-provided certificates |

## Managing Backends

### Toggle Backend Down/Up

You can manually mark a backend as down without removing it. This temporarily excludes it from the load balancer rotation while keeping the configuration intact. Useful for maintenance windows.

### Remove a Backend

When a backend is removed:
- The site's Caddyfile reverts to normal mode (HTTPS on port 443)
- The firewall rule allowing LB traffic is removed from the backend server
- The LB Caddyfile is updated to exclude the removed backend

If all backends are removed, the upstream responds with **503 Service Unavailable**.

## Architecture

```
                          ┌──────────────────────┐
                          │    Load Balancer      │
                          │   (Caddy on :443)     │
                          │                       │
                          │  ┌─────────────────┐  │
          HTTPS           │  │ Upstream Config  │  │
Client ────────────────>  │  │ - LB Policy      │  │
                          │  │ - Health Checks  │  │
                          │  │ - TLS Settings   │  │
                          │  └────────┬────────┘  │
                          └───────────┼───────────┘
                                      │
                         ┌────────────┼────────────┐
                         │            │            │
                    ┌────▼────┐  ┌────▼────┐  ┌────▼────┐
                    │Backend 1│  │Backend 2│  │Backend 3│
                    │ :8080   │  │ :8080   │  │ :8080   │
                    │(FW: LB) │  │(FW: LB) │  │(FW: LB) │
                    └─────────┘  └─────────┘  └─────────┘
```

## Important Notes

- Backend sites must have the **same address** (domain) as the upstream
- Each site can only belong to **one upstream** at a time
- The load balancer server does **not** run databases, PHP, or application code
- Backend servers' port 8080 is only accessible from the load balancer IP via firewall rules
- Health checks start automatically after adding backends (every minute)
- Upstream addresses must be unique per load balancer server
