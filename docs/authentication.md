# Authentication

This document covers how authentication works across all platforms: web app, mobile app, and programmatic API access.

## Overview

Launch uses **JWT-based authentication** for web and mobile clients, and **Personal Access Tokens (PATs)** for API integrations. All authenticated requests use `Bearer` tokens in the `Authorization` header.

## Web App Authentication

### Login Flow

1. User submits email and password to `POST /api/auth/login`
2. Server validates credentials and creates a **session** record (IP, user agent, timestamp)
3. Server returns a JWT access token (with `session_id` in claims) and a refresh token
4. Client stores tokens and includes the access token in all subsequent requests

### Token Refresh

Access tokens expire after the configured `JWT_EXPIRATION` hours (default: 72h). To refresh:

1. Client sends the refresh token to `POST /api/auth/refresh`
2. Server validates the refresh token and issues new access + refresh tokens
3. The session's `last_activity` is updated

### Logout

`POST /api/auth/logout` deletes the session record associated with the current JWT.

### Team Context

Team context is **not** stored in the JWT. Instead, the client sends `X-Team-ID` header with each request. The server validates team membership using a Redis-cached lookup.

## Mobile App Authentication

Mobile apps use the exact same JWT login API as the web app:

1. `POST /api/auth/login` with email/password
2. Store access token and refresh token securely (iOS Keychain / Android Keystore)
3. Include `Authorization: Bearer <access_token>` on all requests
4. Refresh tokens before they expire using `POST /api/auth/refresh`

### Deep Linking

For features like email verification and password reset, use deep links with the scheme `launch://` to redirect back to the mobile app.

### Token Storage

- **iOS**: Store tokens in the Keychain
- **Android**: Store tokens in EncryptedSharedPreferences or the Keystore

## API & Programmatic Authentication

Programmatic clients (CI, scripts, third-party integrations) authenticate with
**Personal Access Tokens**. PATs don't expire by default (unless set at
creation) and are simpler than OAuth flows.

### Creating a PAT

PATs can be created through:

1. **Web UI**: Settings > Security > Personal Access Tokens
2. **API**: `POST /api/user/tokens` with `{"name": "my-token"}`

The plain text token is only shown once at creation. Store it securely.

### How It Works

1. User creates a PAT via the web UI or API
2. The plain text token is shown once; a SHA-256 hash is stored in the database
3. The client sends `Authorization: Bearer <pat>` on all API requests
4. The middleware hashes the incoming token and looks it up in the `personal_access_tokens` table

### PAT Management (API)

```bash
# List your tokens
GET /api/user/tokens

# Create a new token
POST /api/user/tokens   {"name": "deployment-ci"}

# Revoke a token
DELETE /api/user/tokens/{id}
```

A PAT can also be exchanged for short-lived JWTs via `POST /api/auth/token`.

## Session Management

Sessions track active logins with device information (IP address, user agent, last activity).

### API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/user/sessions` | List all active sessions |
| `DELETE` | `/api/user/sessions/:id` | Revoke a specific session |
| `DELETE` | `/api/user/sessions` | Revoke all other sessions |

### Session Response Format

```json
{
  "id": "01HXYZ...",
  "agent": {
    "browser": "Chrome",
    "is_desktop": true,
    "platform": "macOS"
  },
  "ip_address": "192.168.1.1",
  "is_current_device": true,
  "last_active": "2025-01-15T10:30:00Z"
}
```

### Web UI

Active sessions are shown in **Settings > Security > Active Sessions**. Users can:
- See all active sessions with device info
- Identify their current session
- Revoke individual sessions
- Log out all other sessions at once

## Authentication Middleware

The auth middleware validates requests in this order:

1. **JWT**: Parse the `Authorization: Bearer` token as a JWT. If valid, extract `sub` (user ID) and `session_id` from claims.
2. **PAT**: If JWT parsing fails, hash the bearer token (SHA-256) and look it up in the `personal_access_tokens` table.

PAT requests update the token's `last_used_at` timestamp on each use.

## Security Considerations

### Token Expiry
- **Access tokens**: Configurable via `JWT_EXPIRATION` (default: 72 hours)
- **Refresh tokens**: 30 days
- **PATs**: No expiry by default; optional expiry can be set at creation

### Token Revocation
- **JWT sessions**: Revoke by deleting the session record. The token remains valid until expiry, but session-aware endpoints will reject it.
- **PATs**: Revoke via `DELETE /api/user/tokens/:id`. Takes effect immediately since PATs are validated against the database on every request.

### Storage
- Never log or expose tokens in error messages
- PATs are stored as SHA-256 hashes; the plain text is never persisted
- API clients should store PATs in a secret manager or `0600` file, never in source
- Mobile apps should use platform-specific secure storage (Keychain/Keystore)
