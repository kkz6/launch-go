# Docker Application Server Design

## Overview

Add a new **Docker application server** type to Launch that provisions servers with Docker + Traefik, manages containerized services, handles routing/SSL via Traefik dynamic configuration, and supports CI/CD webhook-triggered deployments.

**Key decisions:**
- Docker image deploy only (no build-from-source; users use GitHub Actions or similar for builds)
- Separate `docker` server type (not mixed with PHP servers)
- New `docker` module (separate from `site` module)
- Two service kinds: `application` (gets Traefik routing) and `service` (everything else)
- Traefik routing via dynamic YAML files (file provider)
- Shared Docker network (`launch-network`) for inter-container communication
- SSL via Traefik ACME (Let's Encrypt) + custom certificate support
- CI/CD webhook endpoint for automated deployments

**Reference implementation:** Dokploy (`/Users/karthick/projects/dokploy`) - specifically its Traefik integration, dynamic YAML routing, and container lifecycle management. We skip Docker Swarm, build-from-source, and preview deployments.

---

## 1. Core Architecture

### New Server Type

Add `docker` to the `ServerType` enum in `internal/modules/server/types/types.go`.

### Provisioning

A `docker` server gets provisioned with:

| Software | Purpose |
|----------|---------|
| Docker Engine | Container runtime (official apt repo) |
| Traefik | Reverse proxy + SSL (runs as Docker container) |
| Launch Agent | Server communication |
| UFW | Firewall (ports 22, 80, 443) |

**Provisioning order:**
1. Base setup (user, swap, firewall, SSH hardening, essentials)
2. Install Docker Engine
3. Create shared network: `docker network create launch-network`
4. Start Traefik container
5. Install Launch Agent

### Traefik Container

Traefik runs as a Docker container on the server:

```bash
docker run -d \
  --name launch-traefik \
  --network launch-network \
  --restart always \
  -p 80:80 \
  -p 443:443 \
  -v /etc/launch/traefik/traefik.yml:/etc/traefik/traefik.yml:ro \
  -v /etc/launch/traefik/dynamic:/etc/traefik/dynamic:ro \
  -v launch-traefik-letsencrypt:/letsencrypt \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  traefik:v3
```

### Directory Layout on Server

```
/etc/launch/traefik/
├── traefik.yml                    # Static config
├── dynamic/
│   ├── {container-name-1}.yml     # Per-app routing
│   ├── {container-name-2}.yml
│   └── custom-certs.yml           # Custom certificate references
```

---

## 2. Data Model

### docker_services

The core entity for any Docker container managed by Launch.

| Column | Type | Description |
|--------|------|-------------|
| id | ULID | Primary key |
| team_id | FK → teams | Multi-tenancy |
| server_id | FK → servers | Which docker server |
| user_id | FK → users | Creator |
| name | string | Display name |
| container_name | string | Docker container name (unique per server) |
| kind | enum(`application`, `service`) | Determines if Traefik routing applies |
| status | enum(`pending`, `running`, `stopped`, `failed`, `deploying`) | Current state |
| image | string | Docker image reference |
| registry_id | FK → docker_registries, nullable | Private registry credentials |
| command | text, nullable | Override container CMD |
| entrypoint | text, nullable | Override container ENTRYPOINT |
| restart_policy | enum(`no`, `always`, `unless-stopped`, `on-failure`) | Default: `unless-stopped` |
| cpu_limit | float, nullable | CPU cores limit |
| memory_limit | int, nullable | Memory limit in MB |
| health_check_cmd | text, nullable | Health check command |
| health_check_interval | int | Seconds, default 30 |
| health_check_timeout | int | Seconds, default 10 |
| health_check_retries | int | Default 3 |
| deploy_token | string | Unique token for CI/CD webhook |
| compose_project_id | FK → docker_compose_projects, nullable | If from compose import |
| installed_at | timestamp, nullable | First successful deploy |
| installation_failed_at | timestamp, nullable | If first deploy failed |
| created_at | timestamp | |
| updated_at | timestamp | |

### docker_service_env_vars

| Column | Type | Description |
|--------|------|-------------|
| id | ULID | Primary key |
| docker_service_id | FK → docker_services | Parent service |
| key | string | Variable name |
| value | text | Variable value (encrypted at rest) |
| is_secret | bool | Hide value in API responses |

### docker_service_volumes

| Column | Type | Description |
|--------|------|-------------|
| id | ULID | Primary key |
| docker_service_id | FK → docker_services | Parent service |
| mount_type | enum(`volume`, `bind`) | Volume type |
| source | string | Volume name or host path |
| target | string | Container mount path |
| read_only | bool | Default false |

### docker_service_ports

| Column | Type | Description |
|--------|------|-------------|
| id | ULID | Primary key |
| docker_service_id | FK → docker_services | Parent service |
| host_port | int | Port on host |
| container_port | int | Port in container |
| protocol | enum(`tcp`, `udp`) | Default: tcp |

### docker_domains

Only for services with kind = `application`.

| Column | Type | Description |
|--------|------|-------------|
| id | ULID | Primary key |
| docker_service_id | FK → docker_services | Parent service |
| host | string | Domain hostname |
| path | string | Path prefix, default `/` |
| container_port | int | Which port to route to |
| https | bool | Enable HTTPS |
| certificate_type | enum(`letsencrypt`, `custom`, `none`) | SSL method |
| custom_certificate_id | FK, nullable | For custom certs |
| force_ssl | bool | Redirect HTTP → HTTPS |

### docker_deployments

| Column | Type | Description |
|--------|------|-------------|
| id | ULID | Primary key |
| docker_service_id | FK → docker_services | Parent service |
| image | string | Image that was deployed |
| status | enum(`pending`, `running`, `finished`, `failed`) | Deploy status |
| triggered_by | enum(`manual`, `webhook`, `compose_update`) | Trigger source |
| log | text | Deployment log output |
| started_at | timestamp | |
| finished_at | timestamp, nullable | |

### docker_registries

Team-level private registry credentials.

| Column | Type | Description |
|--------|------|-------------|
| id | ULID | Primary key |
| team_id | FK → teams | Multi-tenancy |
| name | string | Display name |
| url | string | Registry URL |
| username | string | Auth username |
| password | text | Auth password (encrypted) |

### docker_compose_projects

When user imports a docker-compose.yml.

| Column | Type | Description |
|--------|------|-------------|
| id | ULID | Primary key |
| team_id | FK → teams | |
| server_id | FK → servers | |
| name | string | Project name |
| raw_compose | text | Original docker-compose.yml content |

---

## 3. Traefik Routing & SSL

### Static Configuration

Generated during provisioning at `/etc/launch/traefik/traefik.yml`:

```yaml
entryPoints:
  web:
    address: ":80"
    http:
      redirections:
        entryPoint:
          to: websecure
          scheme: https
  websecure:
    address: ":443"

certificatesResolvers:
  letsencrypt:
    acme:
      email: "{admin_email}"
      storage: "/letsencrypt/acme.json"
      httpChallenge:
        entryPoint: web

providers:
  file:
    directory: "/etc/traefik/dynamic"
    watch: true

api:
  dashboard: false

log:
  level: "WARN"
```

### Dynamic Per-App Configuration

When a domain is added to a docker service, Launch generates a YAML file via SSH:

**File**: `/etc/launch/traefik/dynamic/{container_name}.yml`

```yaml
http:
  routers:
    {container_name}-router:
      rule: "Host(`app.example.com`)"
      service: "{container_name}-service"
      entryPoints:
        - websecure
      tls:
        certResolver: "letsencrypt"
    {container_name}-router-http:
      rule: "Host(`app.example.com`)"
      service: "{container_name}-service"
      entryPoints:
        - web

  services:
    {container_name}-service:
      loadBalancer:
        servers:
          - url: "http://{container_name}:{port}"
        passHostHeader: true
```

Multiple domains per service → multiple routers in the same file.

Path-based routing uses `PathPrefix` in the rule:
```yaml
rule: "Host(`example.com`) && PathPrefix(`/api`)"
```

### SSL

- **Let's Encrypt**: Traefik auto-provisions via ACME HTTP-01 challenge
- **Custom certificates**: Uploaded to server, referenced in `/etc/launch/traefik/dynamic/custom-certs.yml`:
  ```yaml
  tls:
    certificates:
      - certFile: "/etc/launch/certs/{domain}.crt"
        keyFile: "/etc/launch/certs/{domain}.key"
  ```

### Domain CRUD Flow

1. User adds domain via API
2. Launch saves to `docker_domains` table
3. Task SSHs to server and writes/updates the YAML file
4. Traefik detects file change and updates routing (sub-second)
5. For Let's Encrypt domains, Traefik auto-provisions certificate

---

## 4. Deployment Flow

### Manual Deploy / Redeploy

1. API receives `POST /servers/:serverId/docker-services/:id/deploy`
2. Create `docker_deployment` record (status: `pending`)
3. Enqueue job to `critical` queue
4. Task SSHs to server and runs:

```bash
# Login to registry (if private)
echo "{password}" | docker login {registry_url} -u {username} --password-stdin

# Pull image
docker pull {image}

# Stop and remove old container
docker stop {container_name} 2>/dev/null || true
docker rm {container_name} 2>/dev/null || true

# Run new container
docker run -d \
  --name {container_name} \
  --network launch-network \
  --restart {restart_policy} \
  {--memory {limit}m} \
  {--cpus {limit}} \
  {-v source:target[:ro]} \
  {-p host:container[/protocol]} \
  {-e KEY=value} \
  {--health-cmd "{cmd}"} \
  {--health-interval {interval}s} \
  {--health-timeout {timeout}s} \
  {--health-retries {retries}} \
  {--entrypoint "{entrypoint}"} \
  {image} {command}
```

5. Callback updates deployment status + docker_service status
6. Broadcasts deployment event via WebSocket

### Container Actions

- **Stop**: `docker stop {container_name}` → update status to `stopped`
- **Start**: `docker start {container_name}` → update status to `running`
- **Restart**: `docker restart {container_name}` → status stays `running`
- **Logs**: `docker logs --tail 500 {container_name}` → stream via WebSocket

### CI/CD Webhook

**Endpoint**: `POST /webhooks/docker/:serviceId/:token`

**Payload** (optional):
```json
{
  "image": "ghcr.io/org/myapp:abc123"
}
```

If no image provided, redeploys with the currently configured image (useful for `latest` tag).

**GitHub Actions example**:
```yaml
deploy:
  runs-on: ubuntu-latest
  needs: build
  steps:
    - name: Deploy to Launch
      run: |
        curl -X POST \
          https://launch.example.com/webhooks/docker/${{ secrets.SERVICE_ID }}/${{ secrets.DEPLOY_TOKEN }} \
          -H "Content-Type: application/json" \
          -d '{"image": "ghcr.io/${{ github.repository }}:${{ github.sha }}"}'
```

---

## 5. Docker Compose Import

### Flow

1. User submits `docker-compose.yml` content via `POST /servers/:serverId/docker-compose`
2. Launch parses YAML using Go's `gopkg.in/yaml.v3`
3. Creates a `docker_compose_project` record
4. For each service in the compose file, creates a `docker_service`:
   - `image` → mapped directly
   - `environment` → creates `docker_service_env_vars`
   - `volumes` → creates `docker_service_volumes`
   - `ports` → creates `docker_service_ports`
   - `command` → mapped to command field
   - `restart` → mapped to restart_policy
   - `deploy.resources.limits` → mapped to cpu_limit, memory_limit
   - `healthcheck` → mapped to health_check fields
5. Auto-detect kind: services with host-bound ports → `application`, others → `service`
6. Return created services for user review before deployment
7. User configures domains for application services
8. Deploy all services

### Compose Parsing Rules

- Only `image` is required (no build support)
- `build` directive is ignored with a warning
- `depends_on` is noted for deploy ordering
- `networks` are ignored (all go on `launch-network`)
- Service names become container names prefixed with project name

---

## 6. API Routes

```
# Docker Services CRUD
GET    /servers/:serverId/docker-services
POST   /servers/:serverId/docker-services
GET    /servers/:serverId/docker-services/:id
PUT    /servers/:serverId/docker-services/:id
DELETE /servers/:serverId/docker-services/:id

# Service Actions
POST   /servers/:serverId/docker-services/:id/deploy
POST   /servers/:serverId/docker-services/:id/stop
POST   /servers/:serverId/docker-services/:id/start
POST   /servers/:serverId/docker-services/:id/restart
GET    /servers/:serverId/docker-services/:id/logs

# Domains (application kind only)
GET    /servers/:serverId/docker-services/:id/domains
POST   /servers/:serverId/docker-services/:id/domains
PUT    /servers/:serverId/docker-services/:id/domains/:domainId
DELETE /servers/:serverId/docker-services/:id/domains/:domainId

# Environment Variables
GET    /servers/:serverId/docker-services/:id/env
PUT    /servers/:serverId/docker-services/:id/env

# Volumes
GET    /servers/:serverId/docker-services/:id/volumes
POST   /servers/:serverId/docker-services/:id/volumes
DELETE /servers/:serverId/docker-services/:id/volumes/:volumeId

# Deployments History
GET    /servers/:serverId/docker-services/:id/deployments

# Docker Compose Import
POST   /servers/:serverId/docker-compose
GET    /servers/:serverId/docker-compose-projects
DELETE /servers/:serverId/docker-compose-projects/:id

# Docker Registries (team-level)
GET    /registries
POST   /registries
PUT    /registries/:id
DELETE /registries/:id

# CI/CD Webhook (public, token-authenticated)
POST   /webhooks/docker/:serviceId/:token
```

---

## 7. Module Structure

```
internal/modules/docker/
├── module.go                    # Module init, implements app.Module
├── routes.go                    # Route registration
├── types/
│   └── types.go                 # ServiceKind, ServiceStatus, etc.
├── models/
│   ├── docker_service.go
│   ├── docker_domain.go
│   ├── docker_deployment.go
│   ├── docker_service_env_var.go
│   ├── docker_service_volume.go
│   ├── docker_service_port.go
│   ├── docker_registry.go
│   └── docker_compose_project.go
├── dto/
│   ├── requests.go
│   └── responses.go
├── repositories/
│   ├── registry.go
│   ├── docker_service_repository.go
│   ├── docker_domain_repository.go
│   ├── docker_deployment_repository.go
│   └── docker_registry_repository.go
├── services/
│   ├── docker_service.go        # Business logic
│   ├── traefik_service.go       # YAML generation
│   └── compose_parser.go        # Docker Compose parsing
├── handlers/
│   ├── docker_service_handler.go
│   ├── docker_domain_handler.go
│   ├── docker_deployment_handler.go
│   ├── docker_registry_handler.go
│   └── docker_webhook_handler.go
├── tasks/
│   ├── provision_docker_server.go
│   ├── deploy_docker_service.go
│   ├── manage_docker_service.go  # stop/start/restart
│   ├── update_traefik_config.go
│   ├── register.go
│   └── templates/
│       ├── provision/
│       │   ├── install_docker.sh
│       │   └── install_traefik.sh
│       └── docker/
│           ├── deploy.sh
│           ├── stop.sh
│           ├── start.sh
│           └── restart.sh
├── jobs/
│   ├── deploy_docker_service_job.go
│   └── manage_docker_service_job.go
└── contracts/
    └── service.go
```

---

## 8. Server Module Changes

### types/types.go
- Add `ServerTypeDocker ServerType = "docker"` to ServerType enum

### types/software.go
- Add `SoftwareDocker Software = "docker"`
- Add `SoftwareTraefik Software = "traefik"`

### Provisioning
- Add Docker + Traefik to the software install order (priority 15 for Docker, 20 for Traefik)
- Extend `provision_fresh_server.go` to handle docker server type
- Docker servers skip PHP, Composer, Caddy, Supervisor installs

### Server Features
- Add `ServerFeatureDocker ServerFeature = "docker_services"` for feature gating

---

## 9. Implementation Phases

| Phase | Scope | Key Deliverables |
|-------|-------|-----------------|
| **1** | Types & migrations | New enums, all migration files, model structs |
| **2** | Server provisioning | Docker + Traefik install scripts, provisioning task |
| **3** | Docker services CRUD | Models, repositories, services, handlers, routes |
| **4** | Deployment pipeline | Deploy task, callbacks, deployment history, container actions |
| **5** | Traefik routing | Domain CRUD, YAML generation, SSL support |
| **6** | CI/CD webhook | Webhook endpoint, deploy tokens |
| **7** | Docker Compose | Compose parser, multi-service import |
| **8** | Registries | Registry CRUD, docker login integration |

---

## 10. WebSocket Events

New events for real-time updates:

- `docker_service.status` - Container status change
- `docker_service.deploying` - Deployment started
- `docker_deployment.log` - Deployment log output
- `docker_deployment.finished` - Deployment complete
- `docker_deployment.failed` - Deployment failed
