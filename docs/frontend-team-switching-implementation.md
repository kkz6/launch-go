# Frontend Implementation: Team Switching with X-Team-ID Header

## Overview

The backend has been updated to use a header-based approach for team context instead of embedding `team_id` in the JWT token. This provides:

- **Instant access revocation** - When a user is removed from a team, they're immediately blocked
- **No token regeneration** - Switching teams doesn't require getting new JWT tokens
- **Better security** - Team membership is validated on every request (with caching)

## Breaking Changes

1. **JWT no longer contains `team_id`** - Don't rely on extracting team from the token
2. **All team-scoped API requests require `X-Team-ID` header** - Requests without this header will fail with 400 error

---

## Implementation Guide

### 1. Update Authentication State

When storing auth state, track the current team separately:

```typescript
interface AuthState {
  user: User;
  accessToken: string;
  refreshToken: string;
  currentTeamId: string | null;  // Store separately from token
}

interface User {
  id: string;
  name: string;
  email: string;
  current_team_id: string | null;
  current_team: Team | null;
  // ... other fields
}
```

### 2. Update Login Flow

After login, extract and store the current team:

```typescript
async function login(email: string, password: string) {
  const response = await fetch('/api/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password })
  });

  const data = await response.json();

  // Store auth state
  setAuthState({
    user: data.data.user,
    accessToken: data.data.access_token,
    refreshToken: data.data.refresh_token,
    currentTeamId: data.data.user.current_team_id  // Extract from user response
  });
}
```

### 3. Update API Client to Include X-Team-ID Header

Configure your HTTP client to automatically include the header:

#### Using Axios

```typescript
import axios from 'axios';

const apiClient = axios.create({
  baseURL: '/api',
});

// Add interceptor to include X-Team-ID
apiClient.interceptors.request.use((config) => {
  const token = getAccessToken();
  const teamId = getCurrentTeamId();

  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }

  if (teamId) {
    config.headers['X-Team-ID'] = teamId;
  }

  return config;
});
```

#### Using Fetch Wrapper

```typescript
async function apiFetch(url: string, options: RequestInit = {}) {
  const token = getAccessToken();
  const teamId = getCurrentTeamId();

  const headers = new Headers(options.headers);

  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }

  if (teamId) {
    headers.set('X-Team-ID', teamId);
  }

  return fetch(url, { ...options, headers });
}
```

#### Using React Query / TanStack Query

```typescript
import { QueryClient } from '@tanstack/react-query';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      queryFn: async ({ queryKey }) => {
        const response = await fetch(queryKey[0] as string, {
          headers: {
            'Authorization': `Bearer ${getAccessToken()}`,
            'X-Team-ID': getCurrentTeamId() || '',
          },
        });
        return response.json();
      },
    },
  },
});
```

### 4. Implement Team Switching

Team switching is now much simpler - just update the stored team ID:

```typescript
async function switchTeam(teamId: string) {
  // Call the switch endpoint to update user's current team in database
  const response = await fetch(`/api/teams/${teamId}/switch`, {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${getAccessToken()}`,
      'Content-Type': 'application/json',
    },
  });

  const data = await response.json();

  if (data.success) {
    // Update local state with new team
    setCurrentTeamId(data.data.current_team_id);
    setCurrentTeam(data.data.current_team);

    // Optionally refresh data for the new team context
    // queryClient.invalidateQueries();
  }

  return data;
}
```

### 5. Handle Team Context Errors

The API will return specific errors when team context is invalid:

```typescript
// Error responses to handle:

// 400 Bad Request - Missing X-Team-ID header
{
  "success": false,
  "message": "X-Team-ID header is required"
}

// 403 Forbidden - Not a member of the team
{
  "success": false,
  "message": "You are not a member of this team"
}

// 403 Forbidden - Insufficient role permissions
{
  "success": false,
  "message": "Insufficient permissions"
}
```

Handle these in your error interceptor:

```typescript
apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 403) {
      const message = error.response?.data?.message;

      if (message === 'You are not a member of this team') {
        // User was removed from team - redirect to team selection
        redirectToTeamSelection();
      }
    }

    if (error.response?.status === 400) {
      const message = error.response?.data?.message;

      if (message === 'X-Team-ID header is required') {
        // Missing team context - redirect to team selection
        redirectToTeamSelection();
      }
    }

    return Promise.reject(error);
  }
);
```

### 6. Team Selection on App Load

When the app loads, ensure a team is selected:

```typescript
async function initializeApp() {
  const user = await fetchCurrentUser();

  if (!user.current_team_id) {
    // User has no team selected - redirect to team selection
    redirectToTeamSelection();
    return;
  }

  // Set the current team from user data
  setCurrentTeamId(user.current_team_id);
  setCurrentTeam(user.current_team);
}
```

---

## API Reference

### Endpoints That Require X-Team-ID Header

All endpoints under these paths require the `X-Team-ID` header:

- `/api/servers/*` - Server management
- `/api/sites/*` - Site management
- `/api/databases/*` - Database management
- `/api/backups/*` - Backup management
- `/api/dns/*` - DNS management
- `/api/source-controls/*` - Source control connections

### Endpoints That Don't Require X-Team-ID

These endpoints work without team context:

- `/api/auth/*` - Authentication (login, register, etc.)
- `/api/teams` - List user's teams
- `/api/teams/:teamId/switch` - Switch team
- `/api/user/*` - User profile management

### Switch Team Endpoint

```
POST /api/teams/:teamId/switch

Response:
{
  "success": true,
  "message": "Team switched. Use X-Team-ID header for subsequent requests.",
  "data": {
    "id": "01ABC...",
    "name": "John Doe",
    "email": "john@example.com",
    "current_team_id": "01XYZ...",
    "current_team": {
      "id": "01XYZ...",
      "name": "My Team",
      "personal_team": false
    }
  }
}
```

### Get User's Teams

```
GET /api/auth/teams

Response:
{
  "success": true,
  "data": [
    {
      "id": "01ABC...",
      "name": "Personal Team",
      "personal_team": true
    },
    {
      "id": "01XYZ...",
      "name": "Company Team",
      "personal_team": false
    }
  ]
}
```

---

## React/Next.js Example Implementation

### Context Provider

```tsx
// contexts/TeamContext.tsx
import { createContext, useContext, useState, useEffect, ReactNode } from 'react';

interface Team {
  id: string;
  name: string;
  personal_team: boolean;
}

interface TeamContextType {
  currentTeamId: string | null;
  currentTeam: Team | null;
  teams: Team[];
  switchTeam: (teamId: string) => Promise<void>;
  isLoading: boolean;
}

const TeamContext = createContext<TeamContextType | null>(null);

export function TeamProvider({ children }: { children: ReactNode }) {
  const [currentTeamId, setCurrentTeamId] = useState<string | null>(null);
  const [currentTeam, setCurrentTeam] = useState<Team | null>(null);
  const [teams, setTeams] = useState<Team[]>([]);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    // Load teams and current team on mount
    loadTeams();
  }, []);

  async function loadTeams() {
    try {
      const response = await fetch('/api/auth/teams', {
        headers: { 'Authorization': `Bearer ${getAccessToken()}` }
      });
      const data = await response.json();
      setTeams(data.data);

      // Get current user to find selected team
      const userResponse = await fetch('/api/auth/user', {
        headers: { 'Authorization': `Bearer ${getAccessToken()}` }
      });
      const userData = await userResponse.json();

      if (userData.data.current_team_id) {
        setCurrentTeamId(userData.data.current_team_id);
        setCurrentTeam(userData.data.current_team);
      }
    } finally {
      setIsLoading(false);
    }
  }

  async function switchTeam(teamId: string) {
    const response = await fetch(`/api/teams/${teamId}/switch`, {
      method: 'POST',
      headers: { 'Authorization': `Bearer ${getAccessToken()}` }
    });

    const data = await response.json();

    if (data.success) {
      setCurrentTeamId(data.data.current_team_id);
      setCurrentTeam(data.data.current_team);
    }
  }

  return (
    <TeamContext.Provider value={{
      currentTeamId,
      currentTeam,
      teams,
      switchTeam,
      isLoading
    }}>
      {children}
    </TeamContext.Provider>
  );
}

export function useTeam() {
  const context = useContext(TeamContext);
  if (!context) {
    throw new Error('useTeam must be used within TeamProvider');
  }
  return context;
}
```

### API Client Hook

```tsx
// hooks/useApiClient.ts
import { useTeam } from '../contexts/TeamContext';

export function useApiClient() {
  const { currentTeamId } = useTeam();

  async function apiFetch(url: string, options: RequestInit = {}) {
    const headers = new Headers(options.headers);
    headers.set('Authorization', `Bearer ${getAccessToken()}`);

    if (currentTeamId) {
      headers.set('X-Team-ID', currentTeamId);
    }

    const response = await fetch(url, { ...options, headers });

    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.message);
    }

    return response.json();
  }

  return { apiFetch };
}
```

### Team Switcher Component

```tsx
// components/TeamSwitcher.tsx
import { useTeam } from '../contexts/TeamContext';

export function TeamSwitcher() {
  const { currentTeam, teams, switchTeam, isLoading } = useTeam();

  if (isLoading) return <div>Loading...</div>;

  return (
    <select
      value={currentTeam?.id || ''}
      onChange={(e) => switchTeam(e.target.value)}
    >
      {teams.map(team => (
        <option key={team.id} value={team.id}>
          {team.name} {team.personal_team && '(Personal)'}
        </option>
      ))}
    </select>
  );
}
```

---

## WebSocket Connections

WebSocket connections also require team context. Pass `team_id` as a **query parameter** instead of in headers (WebSocket doesn't support custom headers in browser APIs).

### Connection URL Format

All WebSocket endpoints use this format:

```
wss://api.example.com/endpoint?token=JWT_TOKEN&team_id=TEAM_ID&...other_params
```

### Available WebSocket Endpoints

| Endpoint | Description | Additional Parameters |
|----------|-------------|----------------------|
| `/ws` | Main pub/sub events | None |
| `/terminal/ws` | SSH terminal | `serverId`, `siteId?`, `username?` |
| `/terminal/logs` | Log streaming | `serverId`, `entity`, `entityId?`, `software?`, `route?`, `tail?`, `search?`, `type?` |
| `/services/status` | Service status monitoring | `serverId`, `serviceId?`, `interval?` |
| `/metrics/stream` | Real-time server metrics | `serverId`, `interval?` |

### Example: Connecting to Terminal WebSocket

```typescript
function connectToTerminal(serverId: string, siteId?: string) {
  const token = getAccessToken();
  const teamId = getCurrentTeamId();

  const params = new URLSearchParams({
    token,
    team_id: teamId,
    serverId,
  });

  if (siteId) {
    params.append('siteId', siteId);
  }

  const ws = new WebSocket(`wss://api.example.com/terminal/ws?${params}`);

  ws.onopen = () => console.log('Terminal connected');
  ws.onmessage = (event) => handleTerminalOutput(event.data);
  ws.onerror = (error) => console.error('WebSocket error:', error);
  ws.onclose = (event) => {
    if (event.code === 1008) {
      // Policy violation - likely auth or team membership failure
      console.error('Authentication or team access denied');
    }
  };

  return ws;
}
```

### Example: Connecting to Metrics Stream

```typescript
function connectToMetricsStream(serverId: string, interval: number = 2) {
  const token = getAccessToken();
  const teamId = getCurrentTeamId();

  const params = new URLSearchParams({
    token,
    team_id: teamId,
    serverId,
    interval: interval.toString(),
  });

  const ws = new WebSocket(`wss://api.example.com/metrics/stream?${params}`);

  ws.onmessage = (event) => {
    const data = JSON.parse(event.data);

    switch (data.event) {
      case 'connected':
        console.log('Metrics stream connected');
        break;
      case 'system_info':
        handleSystemInfo(data);
        break;
      case 'metrics':
        handleMetricsUpdate(data);
        break;
      case 'error':
        console.error('Stream error:', data.message);
        break;
    }
  };

  return ws;
}
```

### Example: Subscribing to Real-time Events

```typescript
function connectToEventStream() {
  const token = getAccessToken();
  const teamId = getCurrentTeamId();

  const ws = new WebSocket(`wss://api.example.com/ws?token=${token}&team_id=${teamId}`);

  ws.onmessage = (event) => {
    const message = JSON.parse(event.data);

    // Handle team events
    if (message.channel === `team.${teamId}`) {
      switch (message.event) {
        case 'server.created':
        case 'server.updated':
        case 'server.deleted':
          refreshServerList();
          break;
        case 'site.created':
        case 'site.updated':
        case 'site.deleted':
          refreshSiteList();
          break;
        case 'deployment.started':
        case 'deployment.progress':
        case 'deployment.finished':
        case 'deployment.failed':
          updateDeploymentStatus(message.data);
          break;
      }
    }
  };

  return ws;
}
```

### WebSocket Authentication Errors

The WebSocket will close with specific messages for auth failures:

| Error | Meaning |
|-------|---------|
| `Authentication failed` | Invalid or expired JWT token |
| `Missing team_id parameter` | team_id query param not provided |
| `Not a member of this team` | User is not a member of the specified team |
| `Server not found` | Server doesn't exist or doesn't belong to the team |

### React Hook Example

```typescript
// hooks/useWebSocket.ts
import { useEffect, useRef, useState } from 'react';
import { useTeam } from '../contexts/TeamContext';

interface UseWebSocketOptions {
  endpoint: string;
  params?: Record<string, string>;
  onMessage?: (data: any) => void;
  onError?: (error: Event) => void;
  autoReconnect?: boolean;
}

export function useWebSocket({
  endpoint,
  params = {},
  onMessage,
  onError,
  autoReconnect = true,
}: UseWebSocketOptions) {
  const { currentTeamId } = useTeam();
  const wsRef = useRef<WebSocket | null>(null);
  const [connected, setConnected] = useState(false);

  useEffect(() => {
    if (!currentTeamId) return;

    const token = getAccessToken();
    const queryParams = new URLSearchParams({
      token,
      team_id: currentTeamId,
      ...params,
    });

    const ws = new WebSocket(`wss://api.example.com${endpoint}?${queryParams}`);
    wsRef.current = ws;

    ws.onopen = () => setConnected(true);
    ws.onclose = () => {
      setConnected(false);
      if (autoReconnect) {
        // Reconnect after 3 seconds
        setTimeout(() => {
          // Re-run effect
        }, 3000);
      }
    };
    ws.onmessage = (event) => {
      const data = JSON.parse(event.data);
      onMessage?.(data);
    };
    ws.onerror = (error) => onError?.(error);

    return () => {
      ws.close();
    };
  }, [currentTeamId, endpoint, JSON.stringify(params)]);

  const send = (data: any) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify(data));
    }
  };

  return { connected, send };
}
```

---

## Migration Checklist

### HTTP Requests
- [ ] Update auth state to store `currentTeamId` separately
- [ ] Configure HTTP client to include `X-Team-ID` header
- [ ] Update login flow to extract `current_team_id` from user response
- [ ] Implement team switching UI
- [ ] Add error handling for team context errors (403, 400)
- [ ] Test switching between teams
- [ ] Test that removed team members are immediately blocked
- [ ] Remove any code that extracts `team_id` from JWT token

### WebSocket Connections
- [ ] Update all WebSocket connections to include `team_id` query parameter
- [ ] Update terminal connection to use new format
- [ ] Update log streaming to use new format
- [ ] Update metrics streaming to use new format
- [ ] Update real-time event subscriptions to use new format
- [ ] Handle WebSocket authentication errors appropriately
- [ ] Reconnect WebSockets when team is switched

---

## Questions?

Contact the backend team if you have questions about:
- Which endpoints require team context
- Error response formats
- Role-based access control (owner, admin, editor, member)
