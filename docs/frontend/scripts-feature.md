# Scripts Feature - Frontend Implementation Guide

## Overview

Scripts (similar to Laravel Forge Recipes) are reusable Bash scripts that can be executed across multiple servers with variable interpolation and real-time output streaming.

## Key Concepts

- **Personal Scripts**: Scripts owned by a user (team_id is null)
- **Team Scripts**: Scripts shared with the team (team_id is set)
- **Batch Execution**: When executing on multiple servers, all executions share a `batch_id`
- **Variable Interpolation**: Scripts support `{{variable}}` syntax that gets replaced with server values

---

## API Endpoints

### Base URL
All endpoints require:
- `Authorization: Bearer <token>` header
- `X-Team-ID: <team_id>` header

### 1. List Scripts

```
GET /api/scripts
```

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "id": "01JCXYZ...",
      "user_id": "01JCABC...",
      "team_id": "01JCDEF..." | null,
      "name": "Clear Application Cache",
      "user": "root",
      "content": "#!/bin/bash\ncd /home/{{username}}/{{server_name}}\nphp artisan cache:clear",
      "created_at": "2026-01-19T10:00:00Z",
      "updated_at": "2026-01-19T10:00:00Z"
    }
  ]
}
```

### 2. Get Single Script

```
GET /api/scripts/:id
```

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "01JCXYZ...",
    "user_id": "01JCABC...",
    "team_id": null,
    "name": "Clear Application Cache",
    "user": "root",
    "content": "#!/bin/bash\nphp artisan cache:clear",
    "created_at": "2026-01-19T10:00:00Z",
    "updated_at": "2026-01-19T10:00:00Z"
  }
}
```

### 3. Create Script

```
POST /api/scripts
```

**Request Body:**
```json
{
  "name": "Clear Application Cache",
  "user": "root",
  "content": "#!/bin/bash\nphp artisan cache:clear",
  "team_id": "01JCDEF..." // optional - omit for personal script
}
```

**Validation:**
- `name`: required, max 255 characters
- `user`: required, max 255 characters (unix user)
- `content`: required
- `team_id`: optional

**Response:** Returns created script object

### 4. Update Script

```
PUT /api/scripts/:id
```

**Request Body (all fields optional):**
```json
{
  "name": "Updated Name",
  "user": "deploy",
  "content": "#!/bin/bash\nnew content",
  "team_id": "01JCDEF..." // set to share, null to make personal
}
```

**Note:** Only the script owner can update

### 5. Delete Script

```
DELETE /api/scripts/:id
```

**Note:** Only the script owner can delete

**Response:** 204 No Content

### 6. Execute Script

```
POST /api/scripts/:id/execute
```

**Request Body:**
```json
{
  "server_ids": ["01JSERVER1...", "01JSERVER2..."],
  "user": "deploy" // optional - overrides script's default user
}
```

**Validation:**
- `server_ids`: required, array with at least 1 server ID
- `user`: optional

**Response:**
```json
{
  "success": true,
  "data": {
    "batch_id": "01JBATCH...",
    "executions": [
      {
        "id": 1,
        "script_id": "01JCXYZ...",
        "server_id": "01JSERVER1...",
        "batch_id": "01JBATCH...",
        "user": "deploy",
        "status": "pending",
        "exit_code": null,
        "output": null,
        "started_at": null,
        "finished_at": null,
        "created_at": "2026-01-19T10:00:00Z"
      },
      {
        "id": 2,
        "script_id": "01JCXYZ...",
        "server_id": "01JSERVER2...",
        "batch_id": "01JBATCH...",
        "user": "deploy",
        "status": "pending",
        ...
      }
    ]
  }
}
```

### 7. List Script Executions

```
GET /api/scripts/:id/executions
```

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "id": 1,
      "script_id": "01JCXYZ...",
      "server_id": "01JSERVER1...",
      "batch_id": "01JBATCH...",
      "user": "root",
      "status": "finished",
      "exit_code": 0,
      "output": "Cache cleared successfully!\n",
      "started_at": "2026-01-19T10:00:05Z",
      "finished_at": "2026-01-19T10:00:10Z",
      "created_at": "2026-01-19T10:00:00Z"
    }
  ]
}
```

### 8. Get Single Execution

```
GET /api/script-executions/:id
```

**Response:** Single execution object

---

## Execution Status Values

| Status | Description |
|--------|-------------|
| `pending` | Execution created, waiting to start |
| `running` | Script is currently executing on server |
| `finished` | Script completed successfully (exit code 0) |
| `failed` | Script failed (non-zero exit code or error) |

---

## WebSocket Streaming (Real-time Output)

Script execution uses a **dedicated WebSocket connection per execution** (similar to logs/terminal features). This provides real-time output streaming directly from the server.

### Connection URL

```
ws://host/scripts/execute?executionId={execution_id}&token={jwt_token}&team_id={team_id}
```

**Parameters:**
- `executionId`: The execution ID returned from the execute API
- `token`: JWT authentication token
- `team_id`: Current team ID

### Connection Flow

1. **Call execute API** → Returns execution records with IDs
2. **Open WebSocket connection** per execution (one per server)
3. **Receive messages** until connection closes

### Message Types

#### 1. Started Message

Sent when script execution begins on the server.

```json
{
  "type": "started",
  "status": "running"
}
```

#### 2. Output Stream (Raw Text)

Script output (stdout/stderr) is streamed as raw text messages, not JSON. Simply append each message to your output display.

```
Clearing cache...
Application cache cleared!
```

**Note:** Output arrives incrementally. Append each chunk to display real-time output in terminal-like UI.

#### 3. Completed Message

Sent when script finishes execution.

**Success:**
```json
{
  "type": "completed",
  "status": "finished",
  "exit_code": 0
}
```

**Failure:**
```json
{
  "type": "completed",
  "status": "failed",
  "exit_code": 1
}
```

#### 4. Error Message

Sent if there's a connection/authentication error.

```json
{
  "type": "error",
  "message": "Authentication failed"
}
```

### Already Completed Executions

If you connect to an execution that has already finished:
1. You receive the stored output (if any)
2. You receive a `completed` message with status and exit_code
3. Connection closes automatically

### Batch Execution (Multiple Servers)

For batch executions across multiple servers:
1. Call execute API once → Returns array of executions
2. Open **separate WebSocket connection** for each execution
3. Display output in tabs or split view per server

**Example:**
```javascript
// After calling execute API
const { batch_id, executions } = response.data;

// Open WebSocket per server
executions.forEach(exec => {
  const ws = new WebSocket(
    `ws://host/scripts/execute?executionId=${exec.id}&token=${token}&team_id=${teamId}`
  );

  ws.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data);
      if (msg.type === 'started') {
        updateStatus(exec.server_id, 'running');
      } else if (msg.type === 'completed') {
        updateStatus(exec.server_id, msg.status);
      } else if (msg.type === 'error') {
        showError(exec.server_id, msg.message);
      }
    } catch {
      // Raw output text
      appendOutput(exec.server_id, event.data);
    }
  };
});
```

---

## Variable Interpolation

Users can use these variables in script content. They are replaced with actual server values at execution time.

| Variable | Description | Example Value |
|----------|-------------|---------------|
| `{{server_id}}` | Server's unique ID | `01JSERVER123...` |
| `{{server_name}}` | Server's display name | `production-web-01` |
| `{{ip_address}}` | Server's public IPv4 | `164.92.105.42` |
| `{{private_ip_address}}` | Server's private IPv4 | `10.0.0.5` |
| `{{username}}` | SSH username | `forge` |
| `{{db_password}}` | Database password | `secret123` |
| `{{server_type}}` | Server type | `web` |

**Example Script:**
```bash
#!/bin/bash
echo "Deploying to {{server_name}} ({{ip_address}})"
cd /home/{{username}}/myapp
git pull origin main
php artisan migrate --force
```

---

## UI Components Needed

### 1. Scripts List Page
- List all scripts (personal + team shared)
- Show badge/indicator for "Team" vs "Personal" scripts
- Actions: Edit, Delete, Execute
- Quick execute button

### 2. Create/Edit Script Form
- Name input
- Unix User input (default: "root")
- Content textarea (code editor with syntax highlighting recommended)
- Team sharing toggle (checkbox to share with team)
- Variable reference/help section

### 3. Execute Script Modal/Page
- Multi-select server picker
- Optional user override input
- Execute button
- Show which servers will run the script

### 4. Execution Monitor
- Real-time output display (terminal-like)
- For batch executions: tabs or split view per server
- Status indicators per server
- Exit code display
- Timestamps

### 5. Execution History
- List past executions for a script
- Filter by status
- View output of past executions

---

## UI Flow Example

1. **User navigates to Scripts page** → Shows list of scripts
2. **User clicks "New Script"** → Create form opens
3. **User fills form and saves** → Script created, returns to list
4. **User clicks "Execute" on a script** → Server selection modal opens
5. **User selects servers and clicks "Run"** →
   - API returns batch_id and execution records
   - UI opens execution monitor
   - WebSocket events update output in real-time
6. **Execution completes** → Final status shown, output saved

---

## Error Handling

| HTTP Status | Meaning |
|-------------|---------|
| 400 | Validation error (check `errors` field) |
| 401 | Unauthorized (invalid/expired token) |
| 404 | Script or execution not found |
| 403 | Forbidden (not owner, trying to update/delete) |
| 500 | Server error |

**Validation Error Response:**
```json
{
  "success": false,
  "message": "Validation failed",
  "errors": {
    "name": ["The name field is required"],
    "server_ids": ["At least one server must be selected"]
  }
}
```

---

## Permissions

- **View**: User can see their own scripts + team scripts
- **Create**: Any authenticated user
- **Update**: Only script owner
- **Delete**: Only script owner
- **Execute**: User can execute any script they can view, on servers in their team
