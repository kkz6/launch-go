# WebSocket Module

Provides real-time communication for deployment progress, server status updates, and task output streaming.

## Features

- JWT-authenticated WebSocket connections
- Channel-based pub/sub messaging
- Automatic team channel subscription
- Resource-specific channels (server, site, deployment, task)

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         WebSocket Hub                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Client                  Hub                    Channels         │
│  ├─ ID                  ├─ clients map         server.{id}      │
│  ├─ UserID              ├─ channels map        site.{id}        │
│  ├─ TeamID              ├─ broadcast chan      deployment.{id}  │
│  ├─ Channels set        ├─ register chan       task.{id}        │
│  └─ Send chan           └─ unregister chan     team.{id}        │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Connection

### Endpoint
```
GET /ws?token=<jwt_token>
```

### Authentication
The JWT token must contain:
- `sub` - User ID
- `team_id` - Current team ID

### Auto-Subscription
On connection, clients are automatically subscribed to their team channel:
```
team.{team_id}
```

## Message Format

### Server → Client
```json
{
  "event": "deployment.progress",
  "channel": "deployment.abc123",
  "data": {
    "deployment_id": "abc123",
    "status": "running",
    "message": "Installing dependencies..."
  }
}
```

### Client → Server
```json
{
  "action": "subscribe",
  "channel": "server.xyz789"
}
```

## Channel Types

| Channel Pattern | Use Case |
|----------------|----------|
| `team.{id}` | Team-wide notifications |
| `server.{id}` | Server provisioning, status |
| `site.{id}` | Site updates |
| `deployment.{id}` | Deployment progress, logs |
| `task.{id}` | Task output streaming |

## Usage in Go

### Broadcasting Events
```go
// From any service
hub.BroadcastToServer(serverID, "server.status", map[string]interface{}{
    "server_id": serverID,
    "status":    "active",
})

hub.BroadcastToDeployment(deploymentID, "deployment.progress", map[string]interface{}{
    "step":     "Installing dependencies",
    "progress": 45,
})
```

### Available Broadcast Methods
```go
hub.Broadcast(channel, event, data)
hub.BroadcastToServer(serverID, event, data)
hub.BroadcastToSite(siteID, event, data)
hub.BroadcastToDeployment(deploymentID, event, data)
hub.BroadcastToTeam(teamID, event, data)
```

## Frontend Integration (Nuxt.js)

```typescript
// composables/useWebSocket.ts
const { connected, subscribe, on } = useWebSocket()

// Subscribe to deployment updates
subscribe('deployment.' + deploymentId)

// Listen for events
on('deployment.progress', (data) => {
  console.log('Progress:', data.message)
})

on('deployment.log', (data) => {
  logs.value += data.output
})
```

## Laravel Migration Reference

| Laravel | Go |
|---------|-----|
| Laravel Echo | Custom WebSocket Hub |
| Pusher/Reverb | Gorilla WebSocket |
| Private channels | JWT + channel validation |
| Presence channels | Not implemented (TODO) |
| `broadcast(new Event)` | `hub.Broadcast()` |

## Events Reference

### Server Events
- `server.status` - Status change (provisioning, active, failed)
- `server.progress` - Provisioning step progress

### Deployment Events
- `deployment.started` - Deployment initiated
- `deployment.progress` - Step progress
- `deployment.log` - Log output chunk
- `deployment.finished` - Completed successfully
- `deployment.failed` - Failed with error

### Task Events
- `task.updated` - Status change
- `task.output` - Output chunk received
