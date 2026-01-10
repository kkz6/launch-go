# Launch Go

Zero-downtime deployment platform built with Go.

## Tech Stack

- **Framework**: [Fiber](https://gofiber.io/) - Express-inspired web framework
- **ORM**: [GORM](https://gorm.io/) - Full-featured ORM
- **Queue**: [Asynq](https://github.com/hibiken/asynq) - Redis-based async task processing
- **WebSocket**: Real-time updates for deployments and server status
- **SSH**: Native Go SSH client for server management

## Project Structure

```
launch-go/
├── cmd/
│   ├── api/          # API server entry point
│   └── worker/       # Queue worker entry point
├── internal/
│   ├── config/       # Configuration
│   ├── database/     # Database connection
│   ├── middleware/   # HTTP middleware
│   ├── modules/      # Feature modules
│   │   ├── auth/     # Authentication
│   │   ├── team/     # Team management
│   │   ├── server/   # Server provisioning
│   │   └── site/     # Site deployment
│   ├── pkg/          # Shared utilities
│   ├── queue/        # Queue client
│   ├── ssh/          # SSH client
│   └── websocket/    # WebSocket hub
├── migrations/       # Database migrations
└── scripts/          # Server provisioning scripts
```

## Getting Started

### Prerequisites

- Go 1.22+
- MySQL 8.0+ or PostgreSQL 15+
- Redis 7+

### Installation

1. Clone the repository:
```bash
git clone https://github.com/kkz6/launch-go.git
cd launch-go
```

2. Copy environment file:
```bash
cp .env.example .env
```

3. Install dependencies:
```bash
go mod download
```

4. Run migrations:
```bash
make migrate-up
```

5. Start the API server:
```bash
make run
```

6. In another terminal, start the worker:
```bash
make worker
```

### Using Docker

```bash
# Start all services
docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

## API Endpoints

### Authentication
- `POST /api/auth/register` - Register new user
- `POST /api/auth/login` - Login
- `GET /api/auth/user` - Get current user
- `PUT /api/auth/profile` - Update profile
- `PUT /api/auth/password` - Change password

### Servers
- `GET /api/servers` - List servers
- `POST /api/servers` - Create server
- `GET /api/servers/:id` - Get server
- `PUT /api/servers/:id` - Update server
- `DELETE /api/servers/:id` - Delete server
- `POST /api/servers/:id/reboot` - Reboot server

### Sites
- `GET /api/sites` - List sites
- `POST /api/sites` - Create site
- `GET /api/sites/:id` - Get site
- `PUT /api/sites/:id` - Update site
- `DELETE /api/sites/:id` - Delete site
- `POST /api/sites/:id/deploy` - Deploy site
- `POST /api/sites/:id/rollback/:releaseId` - Rollback

### WebSocket
- `GET /ws?token=<jwt>` - WebSocket connection for real-time updates

## Development

```bash
# Run with hot reload (requires air)
make dev

# Run tests
make test

# Run linter
make lint

# Format code
make fmt
```

## License

MIT
