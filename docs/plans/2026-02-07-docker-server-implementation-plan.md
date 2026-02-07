# Docker Application Server Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a Docker application server type to Launch that provisions servers with Docker + Traefik, manages containerized services, handles routing/SSL via Traefik dynamic YAML, and supports CI/CD webhook deployments.

**Architecture:** New `docker` server type alongside existing `php`, `database`, `loadbalancer`. New `internal/modules/docker/` module with its own models, types, handlers, services, repositories, tasks, and jobs. Traefik runs as a Docker container, routing configured via YAML files in `/etc/launch/traefik/dynamic/`. All containers join a shared `launch-network`.

**Tech Stack:** Go, GORM, Fiber, Asynq, SSH task runner, Traefik v3, Docker Engine

**Design Doc:** `docs/plans/2026-02-07-docker-application-server-design.md`

---

## Phase 1: Types & Server Module Changes

### Task 1: Add Docker server type and features to server types

**Files:**
- Modify: `internal/modules/server/types/types.go`

**Step 1: Add `ServerTypeDocker` to ServerType enum**

Add after `ServerTypeLoadBalancer`:
```go
ServerTypeDocker ServerType = "docker"
```

Update `Label()` map:
```go
ServerTypeDocker: "Docker Application Server",
```

Update `IsValid()` switch to include `ServerTypeDocker`.

Update `GetFeatures()` with new case:
```go
case ServerTypeDocker:
    return []ServerFeature{
        ServerFeatureDockerServices,
        ServerFeatureSSLCertificates,
        ServerFeatureServices,
    }
```

Update `GetProcessManager()`:
```go
case ServerTypeDocker:
    return ProcessManagerNone
```

Update `AllServerTypes()` to include `ServerTypeDocker`.

**Step 2: Add `ServerFeatureDockerServices` to ServerFeature enum**

Add constant:
```go
ServerFeatureDockerServices ServerFeature = "docker_services"
```

Update `Label()`, `IsValid()`, `NavigationKey()` (key: `"docker"`), `AllServerFeatures()`.

**Step 3: Add Docker and Traefik to ServiceType enum**

Add constants:
```go
ServiceTypeDocker  ServiceType = "docker"
ServiceTypeTraefik ServiceType = "traefik"
```

Update `Label()`, `IsValid()`.

**Step 4: Run existing tests**

Run: `go test ./internal/modules/server/types/...`
Expected: All pass (new values don't break existing tests)

**Step 5: Commit**

```
feat: add docker server type, feature, and service types
```

---

### Task 2: Add Docker and Traefik to Software enum

**Files:**
- Modify: `internal/modules/server/types/software.go`

**Step 1: Add software constants**

After `SoftwareLaunchAgent`:
```go
SoftwareDocker  Software = "docker"
SoftwareTraefik Software = "traefik"
```

**Step 2: Update all Software methods**

- `Label()`: `SoftwareDocker: "Docker Engine"`, `SoftwareTraefik: "Traefik"`
- `IsValid()`: Add to switch case
- `GetVersion()`: `SoftwareDocker: "latest"`, `SoftwareTraefik: "3"`
- `GetServiceType()`: `SoftwareDocker: ServiceTypeDocker`, `SoftwareTraefik: ServiceTypeTraefik`
- `Group()`: `SoftwareDocker: "docker"`, `SoftwareTraefik: "traefik"`
- `AllSoftware()`: Append both
- `InstallTemplateName()`: `SoftwareDocker: "software/install_docker.sh"`, `SoftwareTraefik: "software/install_traefik.sh"`
- `InstallOrder()`: `SoftwareDocker: 15`, `SoftwareTraefik: 16` (after system services, before web server)

**Step 3: Run tests**

Run: `go test ./internal/modules/server/types/...`
Expected: All pass

**Step 4: Commit**

```
feat: add docker and traefik to software enum
```

---

### Task 3: Create Docker and Traefik install script templates

**Files:**
- Create: `internal/modules/server/tasks/templates/software/install_docker.sh`
- Create: `internal/modules/server/tasks/templates/software/install_traefik.sh`

**Step 1: Write install_docker.sh**

Model after `install_caddy2.sh`. Template variables available: `.Username`, `.PublicIPv4`, `.WorkingDirectory`.

```bash
#!/bin/bash
{{ shellDefaults }}

{{ aptFunctions }}

echo "Install Docker Engine"

# Remove old versions
waitForAptUnlock
sudo apt-get remove -y docker docker-engine docker.io containerd runc 2>/dev/null || true

# Add Docker official GPG key
waitForAptUnlock
sudo apt-get update
sudo apt-get install -y ca-certificates curl
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc

# Add repository
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

# Install Docker
waitForAptUnlock
sudo apt-get update
waitForAptUnlock
sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin

# Add user to docker group
sudo usermod -aG docker {{ .Username }}

# Enable and start Docker
sudo systemctl enable docker
sudo systemctl start docker

# Create shared network for all containers
docker network create launch-network 2>/dev/null || true

echo "Docker Engine installed successfully"
```

**Step 2: Write install_traefik.sh**

```bash
#!/bin/bash
{{ shellDefaults }}

echo "Install Traefik reverse proxy"

# Create directories
sudo mkdir -p /etc/launch/traefik/dynamic
sudo mkdir -p /etc/launch/certs

# Write static Traefik config
sudo tee /etc/launch/traefik/traefik.yml > /dev/null <<'EOF'
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
      email: "{{ .AdminEmail }}"
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
EOF

# Ensure launch-network exists
docker network create launch-network 2>/dev/null || true

# Stop and remove old Traefik container if exists
docker stop launch-traefik 2>/dev/null || true
docker rm launch-traefik 2>/dev/null || true

# Create Traefik Let's Encrypt volume
docker volume create launch-traefik-letsencrypt 2>/dev/null || true

# Run Traefik container
docker run -d \
  --name launch-traefik \
  --network launch-network \
  --restart always \
  -p 80:80 \
  -p 443:443 \
  -v /etc/launch/traefik/traefik.yml:/etc/traefik/traefik.yml:ro \
  -v /etc/launch/traefik/dynamic:/etc/traefik/dynamic:ro \
  -v launch-traefik-letsencrypt:/letsencrypt \
  -v /etc/launch/certs:/etc/launch/certs:ro \
  traefik:v3

echo "Traefik reverse proxy installed successfully"
```

**Step 3: Verify templates parse**

Run: `go test ./internal/modules/server/tasks/... -run Template`
If no template tests exist, verify manually: `go build ./...`

**Step 4: Commit**

```
feat: add Docker and Traefik install script templates
```

---

### Task 4: Wire docker server provisioning into existing flow

**Files:**
- Modify: `internal/modules/server/jobs/provision_server.go` (update `getSoftwareStack` fallback for docker)

**Step 1: Add docker server default stack in `getSoftwareStack`**

The current fallback returns PHP stack. We need to check server type. The function already receives a `*models.Server`. Add before the fallback:

```go
// For docker servers, use Docker + Traefik stack
if server.Type == types.ServerTypeDocker {
    if len(stack) == 0 {
        stack = []types.Software{
            types.SoftwareDocker,
            types.SoftwareTraefik,
            types.SoftwareLaunchAgent,
        }
    }
    return types.SortSoftwareStack(stack)
}
```

**Step 2: Add `AdminEmail` to ProvisionFreshServerConfig**

Check if `ProvisionFreshServerConfig` already has a field for admin email. If not, add it. The Traefik template needs `{{ .AdminEmail }}` for Let's Encrypt ACME. Add field to config struct and pass it from the job handler.

**Step 3: Build**

Run: `go build ./...`
Expected: Compiles successfully

**Step 4: Commit**

```
feat: wire docker server provisioning with docker+traefik stack
```

---

## Phase 2: Docker Module - Types & Models

### Task 5: Create docker module types

**Files:**
- Create: `internal/modules/docker/types/types.go`

**Step 1: Write the types file**

Follow the pattern from `internal/modules/server/types/types.go`:

```go
package types

import (
	"database/sql/driver"

	"github.com/kkz6/launch-go/internal/pkg/enumtypes"
)

// ServiceKind determines if a docker service gets Traefik routing
type ServiceKind string

const (
	ServiceKindApplication ServiceKind = "application"
	ServiceKindService     ServiceKind = "service"
)

func (k ServiceKind) String() string  { return string(k) }
func (k ServiceKind) IsValid() bool {
	switch k {
	case ServiceKindApplication, ServiceKindService:
		return true
	}
	return false
}
func (k ServiceKind) Label() string {
	labels := map[ServiceKind]string{
		ServiceKindApplication: "Application",
		ServiceKindService:     "Service",
	}
	if l, ok := labels[k]; ok {
		return l
	}
	return "Unknown"
}
func (k *ServiceKind) Scan(value interface{}) error { return enumtypes.ScanString(k, value) }
func (k ServiceKind) Value() (driver.Value, error)  { return enumtypes.ValueString(k) }

// ServiceStatus represents the current state of a docker service
type ServiceStatus string

const (
	ServiceStatusPending   ServiceStatus = "pending"
	ServiceStatusDeploying ServiceStatus = "deploying"
	ServiceStatusRunning   ServiceStatus = "running"
	ServiceStatusStopped   ServiceStatus = "stopped"
	ServiceStatusFailed    ServiceStatus = "failed"
)

func (s ServiceStatus) String() string  { return string(s) }
func (s ServiceStatus) IsValid() bool {
	switch s {
	case ServiceStatusPending, ServiceStatusDeploying, ServiceStatusRunning,
		ServiceStatusStopped, ServiceStatusFailed:
		return true
	}
	return false
}
func (s ServiceStatus) Label() string {
	labels := map[ServiceStatus]string{
		ServiceStatusPending:   "Pending",
		ServiceStatusDeploying: "Deploying",
		ServiceStatusRunning:   "Running",
		ServiceStatusStopped:   "Stopped",
		ServiceStatusFailed:    "Failed",
	}
	if l, ok := labels[s]; ok {
		return l
	}
	return "Unknown"
}
func (s *ServiceStatus) Scan(value interface{}) error { return enumtypes.ScanString(s, value) }
func (s ServiceStatus) Value() (driver.Value, error)  { return enumtypes.ValueString(s) }

// RestartPolicy for Docker containers
type RestartPolicy string

const (
	RestartPolicyNo            RestartPolicy = "no"
	RestartPolicyAlways        RestartPolicy = "always"
	RestartPolicyUnlessStopped RestartPolicy = "unless-stopped"
	RestartPolicyOnFailure     RestartPolicy = "on-failure"
)

func (r RestartPolicy) String() string  { return string(r) }
func (r RestartPolicy) IsValid() bool {
	switch r {
	case RestartPolicyNo, RestartPolicyAlways, RestartPolicyUnlessStopped, RestartPolicyOnFailure:
		return true
	}
	return false
}
func (r *RestartPolicy) Scan(value interface{}) error { return enumtypes.ScanString(r, value) }
func (r RestartPolicy) Value() (driver.Value, error)  { return enumtypes.ValueString(r) }

// DeploymentStatus for docker deployments
type DeploymentStatus string

const (
	DeploymentStatusPending  DeploymentStatus = "pending"
	DeploymentStatusRunning  DeploymentStatus = "running"
	DeploymentStatusFinished DeploymentStatus = "finished"
	DeploymentStatusFailed   DeploymentStatus = "failed"
)

func (d DeploymentStatus) String() string  { return string(d) }
func (d DeploymentStatus) IsValid() bool {
	switch d {
	case DeploymentStatusPending, DeploymentStatusRunning,
		DeploymentStatusFinished, DeploymentStatusFailed:
		return true
	}
	return false
}
func (d *DeploymentStatus) Scan(value interface{}) error { return enumtypes.ScanString(d, value) }
func (d DeploymentStatus) Value() (driver.Value, error)  { return enumtypes.ValueString(d) }

// DeploymentTrigger describes what triggered a deployment
type DeploymentTrigger string

const (
	DeploymentTriggerManual        DeploymentTrigger = "manual"
	DeploymentTriggerWebhook       DeploymentTrigger = "webhook"
	DeploymentTriggerComposeUpdate DeploymentTrigger = "compose_update"
)

func (t DeploymentTrigger) String() string  { return string(t) }
func (t *DeploymentTrigger) Scan(value interface{}) error { return enumtypes.ScanString(t, value) }
func (t DeploymentTrigger) Value() (driver.Value, error)  { return enumtypes.ValueString(t) }

// MountType for docker volumes
type MountType string

const (
	MountTypeVolume MountType = "volume"
	MountTypeBind   MountType = "bind"
)

func (m MountType) String() string  { return string(m) }
func (m *MountType) Scan(value interface{}) error { return enumtypes.ScanString(m, value) }
func (m MountType) Value() (driver.Value, error)  { return enumtypes.ValueString(m) }

// CertificateType for SSL
type CertificateType string

const (
	CertificateTypeLetsEncrypt CertificateType = "letsencrypt"
	CertificateTypeCustom      CertificateType = "custom"
	CertificateTypeNone        CertificateType = "none"
)

func (c CertificateType) String() string  { return string(c) }
func (c *CertificateType) Scan(value interface{}) error { return enumtypes.ScanString(c, value) }
func (c CertificateType) Value() (driver.Value, error)  { return enumtypes.ValueString(c) }

// Protocol for port mappings
type Protocol string

const (
	ProtocolTCP Protocol = "tcp"
	ProtocolUDP Protocol = "udp"
)

func (p Protocol) String() string  { return string(p) }
func (p *Protocol) Scan(value interface{}) error { return enumtypes.ScanString(p, value) }
func (p Protocol) Value() (driver.Value, error)  { return enumtypes.ValueString(p) }
```

**Step 2: Build**

Run: `go build ./internal/modules/docker/...`
Expected: Compiles

**Step 3: Commit**

```
feat: add docker module type definitions
```

---

### Task 6: Create docker module models

**Files:**
- Create: `internal/modules/docker/models/docker_service.go`
- Create: `internal/modules/docker/models/docker_domain.go`
- Create: `internal/modules/docker/models/docker_deployment.go`
- Create: `internal/modules/docker/models/docker_env_var.go`
- Create: `internal/modules/docker/models/docker_volume.go`
- Create: `internal/modules/docker/models/docker_port.go`
- Create: `internal/modules/docker/models/docker_registry.go`
- Create: `internal/modules/docker/models/docker_compose_project.go`

**Step 1: Write DockerService model**

Follow pattern from `internal/modules/server/models/server.go` using `BaseModel`, `TeamScoped`, `ServerScoped` mixins:

```go
package models

import (
	"time"

	"github.com/kkz6/launch-go/internal/database"
	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
)

type DockerService struct {
	database.BaseModel
	database.TeamScoped
	database.ServerScoped
	database.UserScoped

	Name            string                    `gorm:"type:varchar(255);not null" json:"name"`
	ContainerName   string                    `gorm:"type:varchar(255);not null" json:"container_name"`
	Kind            dockertypes.ServiceKind   `gorm:"type:varchar(50);not null;default:application" json:"kind"`
	Status          dockertypes.ServiceStatus `gorm:"type:varchar(50);not null;default:pending" json:"status"`
	Image           string                    `gorm:"type:varchar(500);not null" json:"image"`
	RegistryID      *string                   `gorm:"type:char(26)" json:"registry_id,omitempty"`
	Command         *string                   `gorm:"type:text" json:"command,omitempty"`
	Entrypoint      *string                   `gorm:"type:text" json:"entrypoint,omitempty"`
	RestartPolicy   dockertypes.RestartPolicy `gorm:"type:varchar(50);not null;default:unless-stopped" json:"restart_policy"`
	CPULimit        *float64                  `gorm:"type:decimal(5,2)" json:"cpu_limit,omitempty"`
	MemoryLimit     *int                      `gorm:"type:int" json:"memory_limit,omitempty"`
	HealthCheckCmd      *string `gorm:"type:text" json:"health_check_cmd,omitempty"`
	HealthCheckInterval int     `gorm:"type:int;default:30" json:"health_check_interval"`
	HealthCheckTimeout  int     `gorm:"type:int;default:10" json:"health_check_timeout"`
	HealthCheckRetries  int     `gorm:"type:int;default:3" json:"health_check_retries"`
	DeployToken         string  `gorm:"type:varchar(64);not null" json:"-"`
	ComposeProjectID    *string `gorm:"type:char(26)" json:"compose_project_id,omitempty"`

	InstalledAt          *time.Time `gorm:"type:timestamp null" json:"installed_at,omitempty"`
	InstallationFailedAt *time.Time `gorm:"type:timestamp null" json:"installation_failed_at,omitempty"`

	// Relations
	EnvVars        []DockerEnvVar        `gorm:"foreignKey:DockerServiceID" json:"env_vars,omitempty"`
	Volumes        []DockerVolume        `gorm:"foreignKey:DockerServiceID" json:"volumes,omitempty"`
	Ports          []DockerPort          `gorm:"foreignKey:DockerServiceID" json:"ports,omitempty"`
	Domains        []DockerDomain        `gorm:"foreignKey:DockerServiceID" json:"domains,omitempty"`
	Deployments    []DockerDeployment    `gorm:"foreignKey:DockerServiceID" json:"deployments,omitempty"`
	Registry       *DockerRegistry       `gorm:"foreignKey:RegistryID" json:"registry,omitempty"`
	ComposeProject *DockerComposeProject `gorm:"foreignKey:ComposeProjectID" json:"compose_project,omitempty"`
}

func (DockerService) TableName() string { return "docker_services" }
```

**Step 2: Write remaining models**

Each model file follows the same pattern. Create `DockerDomain`, `DockerDeployment`, `DockerEnvVar`, `DockerVolume`, `DockerPort`, `DockerRegistry`, `DockerComposeProject` - each with `database.BaseModel`, appropriate foreign keys, and a `TableName()` method.

Key details:
- `DockerEnvVar`: `DockerServiceID`, `Key`, `Value`, `IsSecret bool`
- `DockerVolume`: `DockerServiceID`, `MountType`, `Source`, `Target`, `ReadOnly bool`
- `DockerPort`: `DockerServiceID`, `HostPort int`, `ContainerPort int`, `Protocol`
- `DockerDomain`: `DockerServiceID`, `Host`, `Path`, `ContainerPort int`, `HTTPS bool`, `CertificateType`, `ForceSSL bool`
- `DockerDeployment`: `DockerServiceID`, `Image`, `Status`, `TriggeredBy`, `Log text`, `StartedAt`, `FinishedAt`
- `DockerRegistry`: `database.BaseModel`, `database.TeamScoped`, `Name`, `URL`, `Username`, `Password`
- `DockerComposeProject`: `database.BaseModel`, `database.TeamScoped`, `database.ServerScoped`, `Name`, `RawCompose text`

**Step 3: Build**

Run: `go build ./internal/modules/docker/...`

**Step 4: Commit**

```
feat: add docker module model definitions
```

---

### Task 7: Create database migrations for docker tables

**Files:**
- Create: `internal/database/migrations/0014_02_07_000000_create_docker_registries_table.go`
- Create: `internal/database/migrations/0014_02_07_000001_create_docker_compose_projects_table.go`
- Create: `internal/database/migrations/0014_02_07_000002_create_docker_services_table.go`
- Create: `internal/database/migrations/0014_02_07_000003_create_docker_service_env_vars_table.go`
- Create: `internal/database/migrations/0014_02_07_000004_create_docker_service_volumes_table.go`
- Create: `internal/database/migrations/0014_02_07_000005_create_docker_service_ports_table.go`
- Create: `internal/database/migrations/0014_02_07_000006_create_docker_domains_table.go`
- Create: `internal/database/migrations/0014_02_07_000007_create_docker_deployments_table.go`

**Step 1: Write each migration following the LB upstream pattern**

Follow the exact pattern from `0013_02_07_000000_create_load_balancer_upstreams_table.go`:
- `func init()` with `Register(Migration{...})`
- Private migration struct with GORM tags
- Private FK structs for foreign key constraints
- `Up` function creates table then constraints
- `Down` function drops table

Order matters: registries and compose_projects first (referenced by docker_services), then docker_services, then child tables.

Example for docker_services:
```go
package migrations

import (
	"time"
	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0014_02_07_000002_create_docker_services_table",
		Name:      "Create docker services table",
		Timestamp: time.Date(2014, 2, 7, 0, 0, 2, 0, time.UTC),
		Up:        createDockerServicesTableUp,
	})
}

type dockerServiceMigration struct {
	ID       string `gorm:"type:char(26);primaryKey"`
	TeamID   string `gorm:"column:team_id;type:char(26);not null;index"`
	ServerID string `gorm:"column:server_id;type:char(26);not null;index"`
	UserID   string `gorm:"column:user_id;type:char(26);not null;index"`

	Name            string  `gorm:"type:varchar(255);not null"`
	ContainerName   string  `gorm:"type:varchar(255);not null"`
	Kind            string  `gorm:"type:varchar(50);not null;default:application"`
	Status          string  `gorm:"type:varchar(50);not null;default:pending"`
	Image           string  `gorm:"type:varchar(500);not null"`
	RegistryID      *string `gorm:"column:registry_id;type:char(26)"`
	Command         *string `gorm:"type:text"`
	Entrypoint      *string `gorm:"type:text"`
	RestartPolicy   string  `gorm:"type:varchar(50);not null;default:unless-stopped"`
	CPULimit        *float64 `gorm:"type:decimal(5,2)"`
	MemoryLimit     *int    `gorm:"type:int"`
	HealthCheckCmd      *string `gorm:"type:text"`
	HealthCheckInterval int     `gorm:"type:int;default:30"`
	HealthCheckTimeout  int     `gorm:"type:int;default:10"`
	HealthCheckRetries  int     `gorm:"type:int;default:3"`
	DeployToken         string  `gorm:"type:varchar(64);not null"`
	ComposeProjectID    *string `gorm:"column:compose_project_id;type:char(26)"`

	InstalledAt          *time.Time `gorm:"type:timestamp null"`
	InstallationFailedAt *time.Time `gorm:"type:timestamp null"`

	CreatedAt *time.Time `gorm:"type:timestamp null"`
	UpdatedAt *time.Time `gorm:"type:timestamp null"`
}

func (dockerServiceMigration) TableName() string { return "docker_services" }

// FK structs follow the LB upstream pattern...

func createDockerServicesTableUp(db *gorm.DB) error {
	if err := db.Migrator().CreateTable(&dockerServiceMigration{}); err != nil {
		return err
	}
	// Add FK constraints for team_id, server_id, user_id, registry_id, compose_project_id
	// Add unique index on server_id + container_name
	return db.Exec("CREATE UNIQUE INDEX idx_docker_services_server_container ON docker_services(server_id, container_name)").Error
}
```

**Step 2: Run migrations**

Run: `make migrate-up`
Expected: All new tables created

**Step 3: Commit**

```
feat: add database migrations for docker module tables
```

---

## Phase 3: Docker Module - Scaffold

### Task 8: Create docker module scaffold (module.go, routes.go, empty service/handler/repo)

**Files:**
- Create: `internal/modules/docker/module.go`
- Create: `internal/modules/docker/routes.go`
- Create: `internal/modules/docker/repositories/registry.go`
- Create: `internal/modules/docker/repositories/docker_service_repository.go`
- Create: `internal/modules/docker/services/docker_service.go`
- Create: `internal/modules/docker/handlers/docker_service_handler.go`

**Step 1: Write module.go**

Follow `internal/modules/server/module.go` pattern:

```go
package docker

import (
	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/docker/repositories"
	"github.com/kkz6/launch-go/internal/modules/docker/services"
	"github.com/kkz6/launch-go/internal/pkg/app"
)

const ModuleName = "docker"

var (
	_ app.Module       = (*Module)(nil)
	_ app.JobRegistrar = (*Module)(nil)
)

type Module struct {
	app.Base
	repos   *repositories.Registry
	service *services.Service
}

func NewModule(b *app.Builder) *Module {
	deps := b.Deps()
	repos := repositories.NewRegistry(deps.DB)
	service := services.NewService(services.ServiceDeps{
		Dependencies: deps.ServiceDeps(),
		Repos:        repos,
	})

	return &Module{
		Base:    app.NewBase(ModuleName, b),
		repos:   repos,
		service: service,
	}
}

func (m *Module) RegisterJobs(mux *asynq.ServeMux) {
	// Will be implemented in Phase 4
}
```

**Step 2: Write routes.go**

Scaffold with empty routes that will be filled in later phases:

```go
package docker

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/docker/handlers"
)

func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	handler := handlers.NewDockerServiceHandler(m.service)

	dockerServices := router.Group("/servers/:serverId/docker-services",
		authMiddleware, middleware.TeamScope(), middleware.VerifySubscription())
	{
		dockerServices.Get("/", handler.List)
		dockerServices.Post("/", handler.Create)
		dockerServices.Get("/:id", handler.Show)
		dockerServices.Put("/:id", handler.Update)
		dockerServices.Delete("/:id", handler.Delete)
	}
}
```

**Step 3: Write repository, service, and handler scaffolds**

Each as minimal compilable stubs following existing patterns. The repository has GORM queries, the service has business logic, the handler parses requests and calls service.

**Step 4: Register module in cmd/api/main.go and cmd/worker/main.go**

Add import and `kernel.Register(dockerModule)` in both files.

**Step 5: Build**

Run: `go build ./...`
Expected: Compiles

**Step 6: Commit**

```
feat: scaffold docker module with CRUD routes
```

---

## Phase 4: Docker Services CRUD

### Task 9: Implement docker service repository with CRUD

**Files:**
- Modify: `internal/modules/docker/repositories/docker_service_repository.go`

Implement: `Create`, `FindByID`, `FindByServerID`, `Update`, `Delete`, `FindByContainerName`, `UpdateStatus`.

Follow existing repository patterns with GORM, context, preloading relations.

**Commit:** `feat: implement docker service repository CRUD`

---

### Task 10: Implement docker service business logic

**Files:**
- Modify: `internal/modules/docker/services/docker_service.go`

Implement: `CreateService` (validates, generates container name + deploy token, creates record), `GetService`, `ListServices`, `UpdateService`, `DeleteService`.

Container name generation: `{project-name}-{service-name}-{short-ulid}` (unique per server). Deploy token: `crypto/rand` 32 bytes hex-encoded.

**Commit:** `feat: implement docker service business logic`

---

### Task 11: Implement docker service HTTP handlers

**Files:**
- Modify: `internal/modules/docker/handlers/docker_service_handler.go`
- Create: `internal/modules/docker/dto/requests.go`
- Create: `internal/modules/docker/dto/responses.go`

DTO pattern follows `internal/modules/server/dto/requests.go`. Handlers follow `internal/modules/server/handlers/server_handler.go`.

**Commit:** `feat: implement docker service HTTP handlers`

---

## Phase 5: Deployment Pipeline

### Task 12: Create deploy docker service task and job

**Files:**
- Create: `internal/modules/docker/tasks/deploy_docker_service.go`
- Create: `internal/modules/docker/tasks/register.go`
- Create: `internal/modules/docker/tasks/templates/docker/deploy.sh`
- Create: `internal/modules/docker/jobs/deploy_docker_service_job.go`

The deploy task follows the callback pattern from `internal/modules/server/tasks/provision_fresh_server.go`. The bash template pulls the image, stops old container, runs new container with all config.

**Commit:** `feat: add docker service deployment task and job`

---

### Task 13: Add container management actions (stop/start/restart/logs)

**Files:**
- Create: `internal/modules/docker/tasks/manage_docker_service.go`
- Create: `internal/modules/docker/tasks/templates/docker/stop.sh`
- Create: `internal/modules/docker/tasks/templates/docker/start.sh`
- Create: `internal/modules/docker/tasks/templates/docker/restart.sh`
- Modify: `internal/modules/docker/routes.go` (add action routes)
- Modify: `internal/modules/docker/handlers/docker_service_handler.go` (add action handlers)

**Commit:** `feat: add docker service management actions`

---

## Phase 6: Traefik Routing & Domains

### Task 14: Implement Traefik YAML generation service

**Files:**
- Create: `internal/modules/docker/services/traefik_service.go`

This service generates the per-app YAML config. Key method:

```go
func (s *TraefikService) GenerateConfig(service *models.DockerService) (string, error)
```

Generates YAML with routers, services, and TLS config based on the service's domains.

**Commit:** `feat: implement Traefik dynamic YAML config generation`

---

### Task 15: Implement domain CRUD with Traefik config updates

**Files:**
- Create: `internal/modules/docker/repositories/docker_domain_repository.go`
- Create: `internal/modules/docker/handlers/docker_domain_handler.go`
- Create: `internal/modules/docker/tasks/update_traefik_config.go`
- Modify: `internal/modules/docker/routes.go` (add domain routes)

Domain CRUD: create/update/delete domain -> regenerate YAML -> SSH to server and write file to `/etc/launch/traefik/dynamic/{container_name}.yml`.

**Commit:** `feat: add domain management with Traefik routing`

---

## Phase 7: CI/CD Webhook

### Task 16: Implement webhook endpoint for CI/CD deployments

**Files:**
- Create: `internal/modules/docker/handlers/docker_webhook_handler.go`
- Modify: `internal/modules/docker/module.go` (implement `app.WebhookRegistrar`)
- Modify: `internal/modules/docker/routes.go` (add webhook route)

Webhook endpoint: `POST /webhooks/docker/:serviceId/:token`
- Validates deploy token
- Accepts optional `{"image": "..."}` payload
- Triggers deploy job
- Returns 200 with deployment ID

**Commit:** `feat: add CI/CD webhook endpoint for docker deployments`

---

## Phase 8: Docker Compose Import

### Task 17: Implement Docker Compose parser

**Files:**
- Create: `internal/modules/docker/services/compose_parser.go`

Parse `docker-compose.yml` YAML, extract services, map to `DockerService` creation params. Uses `gopkg.in/yaml.v3`.

**Commit:** `feat: add Docker Compose file parser`

---

### Task 18: Implement compose import endpoint

**Files:**
- Create: `internal/modules/docker/handlers/docker_compose_handler.go`
- Modify: `internal/modules/docker/routes.go` (add compose routes)

Endpoint: `POST /servers/:serverId/docker-compose` accepts YAML content, parses it, creates `docker_compose_project` + individual `docker_service` records, returns for user review.

**Commit:** `feat: add Docker Compose import endpoint`

---

## Phase 9: Docker Registries

### Task 19: Implement docker registry CRUD

**Files:**
- Create: `internal/modules/docker/repositories/docker_registry_repository.go`
- Create: `internal/modules/docker/handlers/docker_registry_handler.go`
- Modify: `internal/modules/docker/routes.go` (add registry routes)

Team-scoped CRUD for private Docker registries. Password encrypted at rest. Deploy task uses registry credentials for `docker login` before `docker pull`.

**Commit:** `feat: add docker registry management`

---

## Phase 10: Environment Variables & Volumes

### Task 20: Implement env var and volume CRUD

**Files:**
- Create: `internal/modules/docker/repositories/docker_env_var_repository.go`
- Create: `internal/modules/docker/repositories/docker_volume_repository.go`
- Create: `internal/modules/docker/handlers/docker_env_handler.go`
- Create: `internal/modules/docker/handlers/docker_volume_handler.go`
- Modify: `internal/modules/docker/routes.go`

Env vars: bulk update (`PUT /env` replaces all). Secrets masked in responses.
Volumes: create/delete individual volumes.

**Commit:** `feat: add env var and volume management for docker services`

---

## Final: Integration Testing

### Task 21: Write integration tests

**Files:**
- Create: `tests/docker/docker_service_test.go`

Test the full flow: create docker server -> create docker service -> add domain -> deploy -> verify Traefik config generation -> webhook deploy.

Run: `go test ./tests/docker/... -v`

**Commit:** `test: add docker module integration tests`
