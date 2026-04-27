# Docker Server Type — Design

**Date:** 2026-04-27
**Branch:** `feat/docker-provisioning`
**Status:** approved for implementation

## Goal

Add a new server type (`ServerTypeDocker`) that, when provisioned, leaves
the host with Docker Engine, Docker Compose plugin, the launch directory
tree, an attachable bridge network, and a Traefik reverse proxy
container ready to route HTTP/HTTPS traffic to user-deployed containers.

This is the foundation for Docker-based application management. App
deploy/lifecycle is a separate, later concern.

## Non-goals (explicitly deferred)

- Docker Swarm. Single-node Docker + Compose only.
- Buildpacks / Nixpacks / Railpack. Build-tooling lands separately.
- RClone or other backup tooling.
- Docker app CRUD endpoints, deploy flows, log streaming. Later PRs.
- ACME via Let's Encrypt. Traefik ships HTTP-only in v1; users add TLS
  later by setting `traefik_admin_email` (configuration update flow,
  separate PR).
- Frontend / Nuxt UI. The new server type is API-only in v1.

## Architecture

The Docker stack slots into the existing software-driven provisioning
pipeline. No new jobs, no new HTTP routes, no schema changes.

### New software entries

`internal/modules/server/types/software.go`

| Const                | Label   | Version | Group      | Install order |
|----------------------|---------|---------|------------|---------------|
| `SoftwareDocker`     | Docker  | 28.5    | docker     | 25            |
| `SoftwareTraefik`    | Traefik | 3.6     | traefik    | 35            |

Each gets:
- `InstallTemplateName`: `software/install_docker.sh` /
  `software/install_traefik.sh`
- `RemoveTemplateName`: `software/remove_docker.sh` /
  `software/remove_traefik.sh` (stub for now; full removal lands later)
- `Group()`, label, version map, `IsValid()` via `enumtypes`
- Entry in `GetAllSoftwareGroups()` with `HasStart/Stop/Restart=true`,
  `HasRemove=true`, `HasStatus=true`

### New service-type constants

`internal/modules/server/types/types.go`

```go
ServiceTypeDocker  ServiceType = "container_runtime"
ServiceTypeTraefik ServiceType = "reverse_proxy"
```

`Software.GetServiceType()` returns these for the new entries.

### New server type

`internal/modules/server/types/types.go`

```go
ServerTypeDocker ServerType = "docker"
```

- Label: "Docker Server"
- Features: `Services`, `Backups`, `SSLCertificates` (Traefik handles
  cert issuance once email is set)
- ProcessManager: `ProcessManagerNone`

`dto.CreateServerRequest`'s `Type` field validation is updated:
`oneof=php database loadbalancer docker`.

### Service wiring

`internal/modules/server/services/server_service.go`

```go
func (s *Service) createServicesForServer(...) error {
    switch req.Type {
    case ...
    case ServerTypeDocker:
        return s.createDockerServerServices(ctx, server, req)
    }
}

func (s *Service) createDockerServerServices(...) error {
    if err := s.createService(ctx, server, types.SoftwareDocker); err != nil {
        return err
    }
    if err := s.createService(ctx, server, types.SoftwareTraefik); err != nil {
        return err
    }
    if req.InstallAgent {
        if err := s.createService(ctx, server, types.SoftwareLaunchAgent); err != nil {
            return err
        }
    }
    return nil
}
```

Existing `SortSoftwareStack` orders by `InstallOrder`, so Docker (25) →
Traefik (35) → LaunchAgent (100). The provisioning loop in
`tasks/provision_fresh_server.go` renders each install template in
order. No further changes to the provisioning machinery.

### Install scripts

#### `software/install_docker.sh`
- Reject snap-installed Docker (early failure, clear message).
- Add Docker's official apt repo + GPG key.
- Install: `docker-ce`, `docker-ce-cli`, `containerd.io`,
  `docker-buildx-plugin`, `docker-compose-plugin`.
- `systemctl enable --now docker`.
- `usermod -aG docker {{ .Username }}`.
- Create directory tree under `/etc/launch/`:
  - `traefik/dynamic/`, `apps/`, `compose/`, `logs/`
- Owner `{{ .Username }}`, mode 700.
- Create attachable bridge network `launch-network` (idempotent).

#### `software/install_traefik.sh`
- Write `/etc/launch/traefik/traefik.yml` (entry points, docker
  provider, file provider). ACME resolver only included when
  `{{ .TraefikAdminEmail }}` is non-empty.
- Create `/etc/launch/traefik/acme.json` with mode 600.
- `docker rm -f launch-traefik` (idempotent), then `docker run -d` with:
  - `--name launch-traefik --restart unless-stopped`
  - `--network launch-network`
  - `-p 80:80 -p 443:443`
  - Read-only mounts of docker.sock, traefik.yml, dynamic dir
  - Read-write mount of acme.json
  - Image `traefik:v{{ .Version }}`

### Config plumbing

`tasks/SoftwareInstallConfig` gains one optional field:

```go
TraefikAdminEmail string
```

Threaded through `renderSoftwareInstall` in
`tasks/provision_fresh_server.go` and through `jobs/add_service.go` for
post-provision installs. Empty by default.

## Test strategy (TDD discipline)

Three layers, mirroring existing patterns.

### Layer 1 — Type/enum unit tests
File: `internal/modules/server/types/software_test.go` (extend or
create), `internal/modules/server/types/types_test.go` (extend).

```
TestServerTypeDocker_IsValid
TestServerTypeDocker_Label
TestServerTypeDocker_Features                  // Services, Backups, SSL only
TestServerTypeDocker_ProcessManager            // None
TestSoftwareDocker_InstallTemplate             // "software/install_docker.sh"
TestSoftwareDocker_InstallOrder                // 25
TestSoftwareTraefik_InstallTemplate
TestSoftwareTraefik_InstallOrder               // 35 — after Docker
TestSoftwareDocker_GetServiceType              // ServiceTypeDocker
TestSoftwareTraefik_GetServiceType             // ServiceTypeTraefik
```

### Layer 2 — Task / template snapshot tests
Files: `tests/tasks/install_docker_test.go`,
`tests/tasks/install_traefik_test.go`.

```
TestInstallDocker                              // ScriptContainsAll the key lines
TestInstallDocker_RejectsSnap
TestInstallDocker_CreatesLaunchDirectoryTree
TestInstallDocker_CreatesNetwork
TestInstallTraefik
TestInstallTraefik_NoEmailMeansNoACME          // ScriptDoesNotContain
TestInstallTraefik_WithEmailEnablesACME
```

If `testutil.AssertTask` lacks `ScriptDoesNotContain`, add it (TDD a
small helper).

### Layer 3 — Provisioning snapshot
File: `tests/tasks/provision_test.go` (extend).

```
TestProvisionFreshServer_DockerType
```

Covers the integrated script for a Docker server: base provisioning
steps + install_docker.sh + install_traefik.sh + agent. Snapshot file at
`tests/snapshots/tasks/provision_fresh_docker_server.txt`.

### Layer 4 — Service wiring test
File: `internal/modules/server/services/server_service_test.go` (extend
if exists, otherwise small in-memory stub test).

```
TestCreateServicesForServer_DockerType_Default        // Docker, Traefik, LaunchAgent
TestCreateServicesForServer_DockerType_NoAgent        // Docker, Traefik only
```

## Behaviour preservation

- Existing server types (`php`, `database`, `loadbalancer`) unchanged.
- No HTTP API contract changes (new `type: "docker"` value is purely
  additive on the request side).
- Existing tests remain green.

## Files touched

```
docs/plans/2026-04-27-docker-server-type-design.md          NEW
internal/modules/server/types/software.go                   MODIFIED
internal/modules/server/types/types.go                      MODIFIED
internal/modules/server/types/software_test.go              NEW or extended
internal/modules/server/types/types_test.go                 EXTENDED
internal/modules/server/services/server_service.go          MODIFIED
internal/modules/server/services/server_service_test.go     NEW or extended
internal/modules/server/dto/requests.go                     MODIFIED (validate tag)
internal/modules/server/tasks/software.go                   MODIFIED (config field)
internal/modules/server/tasks/templates/software/install_docker.sh   NEW
internal/modules/server/tasks/templates/software/install_traefik.sh  NEW
tests/tasks/install_docker_test.go                          NEW
tests/tasks/install_traefik_test.go                         NEW
tests/tasks/provision_test.go                               EXTENDED
tests/testutil/tasks.go                                     MODIFIED if ScriptDoesNotContain missing
```
