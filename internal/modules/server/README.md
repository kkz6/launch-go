# Server Module

Manages server provisioning, configuration, and lifecycle across multiple cloud providers.

## Features

- Multi-cloud server provisioning (DigitalOcean, Hetzner, AWS, Linode, Vultr)
- Custom server support via SSH
- SSH key management
- Firewall rule configuration
- Database provisioning (MySQL/PostgreSQL)
- PHP version management
- Cron job management
- Real-time provisioning progress via WebSocket

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/servers` | List all servers |
| POST | `/api/servers` | Create/provision server |
| GET | `/api/servers/:id` | Get server details |
| PUT | `/api/servers/:id` | Update server |
| DELETE | `/api/servers/:id` | Delete server |
| POST | `/api/servers/:id/reboot` | Reboot server |
| GET | `/api/servers/:id/databases` | List databases |
| POST | `/api/servers/:id/databases` | Create database |

## Models

### Server
```go
type Server struct {
    ID               string
    TeamID           string
    Name             string
    Provider         ServerProvider  // digitalocean, hetzner, aws, etc.
    ProviderServerID *string
    IPAddress        *string
    Region           string
    Size             string
    Status           ServerStatus    // pending, provisioning, active, failed
    SSHPort          int
    SSHUser          string
    PrivateKey       string          // SSH private key
    PublicKey        string          // SSH public key
    PHPVersion       string
    DatabaseType     *string
    WebServer        string          // caddy
}
```

## Server Provisioning Flow

```
1. Create server record (status: pending)
2. Dispatch provision job
3. Create server on cloud provider
4. Wait for server to be ready
5. Connect via SSH
6. Run provisioning scripts:
   - Update system packages
   - Install essential tools
   - Configure swap
   - Install PHP
   - Install Caddy
   - Install database (if requested)
   - Configure firewall
7. Update status to active
```

## Background Jobs

| Job Type | Description |
|----------|-------------|
| `server:provision` | Full server provisioning |
| `server:install_php` | Install/update PHP version |
| `server:configure_firewall` | Apply firewall rules |
| `server:install_database` | Install MySQL/PostgreSQL |
| `server:reboot` | Reboot server |

## WebSocket Events

Channel: `server.{server_id}`

| Event | Description |
|-------|-------------|
| `server.status` | Status change |
| `server.progress` | Provisioning step progress |

## Cloud Providers

### Provider Interface (TODO)
```go
type Provider interface {
    CreateServer(ctx context.Context, opts CreateOptions) (*ProviderServer, error)
    DeleteServer(ctx context.Context, serverID string) error
    RebootServer(ctx context.Context, serverID string) error
    GetServer(ctx context.Context, serverID string) (*ProviderServer, error)
    ListRegions(ctx context.Context) ([]Region, error)
    ListSizes(ctx context.Context) ([]Size, error)
}
```

### Supported Providers
- **DigitalOcean** - Using official Go SDK
- **Hetzner** - Using hcloud-go
- **AWS EC2** - Using aws-sdk-go-v2
- **Linode** - Using linode-go
- **Vultr** - Using govultr
- **Custom** - Direct SSH connection

## Laravel Migration Reference

| Laravel | Go |
|---------|-----|
| `modules/server/src/Models/Server.php` | `models.go` |
| `modules/server/src/Services/ServerService.php` | `service.go` |
| `modules/server/src/Jobs/ProvisionServerJob.php` | `jobs/jobs.go` |
| `modules/server/src/Services/Providers/` | `providers/` (TODO) |
| `modules/server/src/Tasks/` | Uses `taskrunner` module |

## Directory Structure

```
server/
├── dto.go          # Request/Response DTOs
├── handler.go      # HTTP handlers
├── models.go       # Database models
├── module.go       # Module setup and routes
├── repository.go   # Data access layer
├── service.go      # Business logic
├── jobs/
│   └── jobs.go     # Background jobs
├── providers/      # Cloud provider implementations (TODO)
│   ├── provider.go
│   ├── digitalocean.go
│   ├── hetzner.go
│   └── ...
└── ssh/            # SSH operations (TODO)
    └── commands.go
```
