# Docker Deployment Guide

This guide covers building, running, and deploying Launch Go using Docker.

## Quick Start

```bash
# Start all services (API, Worker, MySQL, Redis, Asynqmon)
docker-compose up -d

# View logs
docker-compose logs -f

# Stop all services
docker-compose down
```

## Building the Image

```bash
# Build using docker-compose
docker-compose build

# Or build directly
docker build -t launch-go .

# Multi-platform build (for production)
docker buildx build --platform linux/amd64,linux/arm64 -t launch-go .
```

## Services

| Service | Port | Description |
|---------|------|-------------|
| api | 8080 | HTTP API server |
| worker | - | Background job processor |
| db | 3306 | MySQL 8.0 database |
| redis | 6379 | Redis for job queues |
| asynqmon | 8081 | Queue monitoring UI |

## Database Migrations

Migrations are handled automatically using the `RUN_MIGRATIONS` environment variable.

### How It Works

- Set `RUN_MIGRATIONS=true` on **one container only** (typically the API)
- Migrations run before the application starts
- Other containers (worker, additional API replicas) should NOT have this flag

### docker-compose (Development)

By default, the API service runs migrations on startup:

```yaml
api:
  environment:
    - RUN_MIGRATIONS=true  # Runs migrations before starting
```

### Production Deployment

When scaling to multiple API replicas, only one should run migrations:

```bash
# First deployment or schema changes - run migrations
docker run -e RUN_MIGRATIONS=true launch-go api

# Additional replicas - no migrations
docker run launch-go api
```

### Manual Migration Commands

```bash
# Check migration status
docker run --rm launch-go ./migrate status

# Run pending migrations
docker run --rm launch-go ./migrate migrate

# Rollback last batch
docker run --rm launch-go ./migrate rollback

# Fresh install (drop all tables and re-run)
docker run --rm launch-go ./migrate fresh
```

## Container Modes

The container supports two modes via command argument:

```bash
# API server (default)
docker run launch-go
docker run launch-go api

# Background worker
docker run launch-go worker
```

## Environment Variables

### Application

| Variable | Description | Default |
|----------|-------------|---------|
| `APP_ENV` | Environment (development/production) | - |
| `APP_PORT` | HTTP server port | 8080 |
| `RUN_MIGRATIONS` | Run migrations on startup | false |

### Database

| Variable | Description | Default |
|----------|-------------|---------|
| `DB_DRIVER` | Database driver (mysql/postgres) | - |
| `DB_HOST` | Database host | - |
| `DB_PORT` | Database port | - |
| `DB_DATABASE` | Database name | - |
| `DB_USERNAME` | Database user | - |
| `DB_PASSWORD` | Database password | - |

### Redis

| Variable | Description | Default |
|----------|-------------|---------|
| `REDIS_ADDRESS` | Redis address (host:port) | - |

### Authentication

| Variable | Description | Default |
|----------|-------------|---------|
| `JWT_SECRET` | Secret key for JWT tokens | - |

## Health Checks

The API exposes a health endpoint:

```
GET /api/health
```

docker-compose configures automatic health checks for the API service.

## Production Recommendations

### 1. Use Secrets Management

Never hardcode secrets in docker-compose files. Use Docker secrets or environment files:

```bash
# Using env file
docker run --env-file .env.production launch-go api
```

### 2. Resource Limits

Add resource constraints in production:

```yaml
services:
  api:
    deploy:
      resources:
        limits:
          cpus: '1'
          memory: 512M
```

### 3. Logging

Configure log drivers for production:

```yaml
services:
  api:
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"
```

### 4. Network Isolation

Use custom networks to isolate services:

```yaml
services:
  api:
    networks:
      - frontend
      - backend
  db:
    networks:
      - backend

networks:
  frontend:
  backend:
    internal: true
```

## Kubernetes Deployment

For Kubernetes, use an init container for migrations:

```yaml
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      initContainers:
        - name: migrations
          image: launch-go
          command: ["./migrate", "migrate"]
          env:
            - name: DB_HOST
              value: "mysql-service"
      containers:
        - name: api
          image: launch-go
          args: ["api"]
```

## Troubleshooting

### Container won't start

Check logs for errors:
```bash
docker-compose logs api
```

### Database connection refused

Ensure the database is healthy before the API starts:
```bash
docker-compose ps  # Check health status
```

### Migrations failed

Run migrations manually to see detailed errors:
```bash
docker-compose run --rm api ./migrate migrate
```

### Reset everything

```bash
# Stop and remove containers, volumes
docker-compose down -v

# Rebuild and start fresh
docker-compose up --build
```
