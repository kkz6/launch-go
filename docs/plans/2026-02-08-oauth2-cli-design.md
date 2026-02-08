# OAuth 2.0 Authorization Server + CLI Tool (`launchctl`)

## Overview

Add an OAuth 2.0 Authorization Server to launch-go and build a CLI tool (`launchctl`) that authenticates via browser-based login. The OAuth server will also serve future mobile app and third-party integration needs.

## Goals

1. OAuth 2.0 Authorization Server embedded in the existing Fiber app
2. Support **Authorization Code + PKCE** (CLI, mobile, web)
3. Support **Device Authorization Grant** (CLI on headless/SSH servers)
4. CLI tool `launchctl` with browser-based login
5. Personal Access Token management endpoints

## Architecture

### Clients Using the OAuth Server

| Client        | Redirect URI                        | Grant Type              |
|---------------|-------------------------------------|-------------------------|
| Web App       | `https://app.launch.dev/callback`   | Auth Code + PKCE        |
| Mobile App    | `launch://auth/callback` (deep link)| Auth Code + PKCE        |
| CLI (desktop) | `http://localhost:{port}/callback`  | Auth Code + PKCE        |
| CLI (headless)| N/A (polls for token)               | Device Authorization    |

### Library Choice: `zitadel/oidc` v3

**Why:**
- Supports both Authorization Code + PKCE and Device Authorization Grant natively
- OpenID Certified (Basic OP, Config OP)
- Active development (v3.45.3, Jan 2026)
- 1.8k GitHub stars, strong community
- Uses standard `net/http` interfaces (integrates with Fiber via adaptor middleware)
- OpenTelemetry support built-in

**Alternatives considered:**
- `ory/fosite` — battle-tested but Device Flow not cleanly exposed via compose API
- `go-oauth2/oauth2` — popular but missing PKCE and Device Flow entirely
- `luikyv/go-oidc` — most feature-complete but only 87 stars (adoption risk)

**Integration with Fiber:**
The `zitadel/oidc` server package uses `net/http` handlers. Fiber's adaptor middleware (`github.com/gofiber/adaptor/v2`) bridges `http.Handler` to Fiber handlers seamlessly.

---

## Data Model Changes

### New Tables

#### `oauth_clients`

Registered OAuth clients (CLI, mobile app, web app, future third-party).

```sql
CREATE TABLE oauth_clients (
    id              CHAR(26) PRIMARY KEY,          -- ULID
    name            VARCHAR(255) NOT NULL,          -- "Launch CLI", "Launch Mobile"
    client_id       VARCHAR(255) NOT NULL UNIQUE,   -- Public client identifier
    client_secret   VARCHAR(255) NULL,              -- NULL for public clients (CLI, mobile)
    redirect_uris   JSON NOT NULL,                  -- Allowed redirect URIs
    grant_types     JSON NOT NULL,                  -- ["authorization_code", "device_code", "refresh_token"]
    scopes          JSON NOT NULL,                  -- Allowed scopes
    is_public       BOOLEAN NOT NULL DEFAULT FALSE, -- Public clients don't use client_secret
    team_id         CHAR(26) NULL,                  -- NULL = first-party client
    created_at      TIMESTAMP NULL,
    updated_at      TIMESTAMP NULL,
    deleted_at      TIMESTAMP NULL
);
```

#### `oauth_authorization_codes`

Short-lived authorization codes (exchanged for tokens).

```sql
CREATE TABLE oauth_authorization_codes (
    id              CHAR(26) PRIMARY KEY,
    client_id       VARCHAR(255) NOT NULL,
    user_id         CHAR(26) NOT NULL,
    code            VARCHAR(512) NOT NULL UNIQUE,
    redirect_uri    VARCHAR(2048) NOT NULL,
    scope           VARCHAR(1024) NOT NULL,
    code_challenge  VARCHAR(128) NULL,              -- PKCE
    code_method     VARCHAR(10) NULL,               -- "S256" or "plain"
    expires_at      TIMESTAMP NOT NULL,
    created_at      TIMESTAMP NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
```

#### `oauth_access_tokens`

Issued access tokens (replaces direct JWT for OAuth clients).

```sql
CREATE TABLE oauth_access_tokens (
    id              CHAR(26) PRIMARY KEY,
    client_id       VARCHAR(255) NOT NULL,
    user_id         CHAR(26) NOT NULL,
    token           VARCHAR(512) NOT NULL UNIQUE,
    scopes          JSON NULL,
    expires_at      TIMESTAMP NOT NULL,
    created_at      TIMESTAMP NULL,
    revoked_at      TIMESTAMP NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
```

#### `oauth_refresh_tokens`

Long-lived refresh tokens.

```sql
CREATE TABLE oauth_refresh_tokens (
    id              CHAR(26) PRIMARY KEY,
    access_token_id CHAR(26) NOT NULL,
    token           VARCHAR(512) NOT NULL UNIQUE,
    expires_at      TIMESTAMP NOT NULL,
    created_at      TIMESTAMP NULL,
    revoked_at      TIMESTAMP NULL,
    FOREIGN KEY (access_token_id) REFERENCES oauth_access_tokens(id) ON DELETE CASCADE
);
```

#### `oauth_device_codes`

Device authorization flow state.

```sql
CREATE TABLE oauth_device_codes (
    id              CHAR(26) PRIMARY KEY,
    client_id       VARCHAR(255) NOT NULL,
    device_code     VARCHAR(512) NOT NULL UNIQUE,   -- Secret, used by CLI to poll
    user_code       VARCHAR(16) NOT NULL UNIQUE,    -- Short code shown to user (e.g. "ABCD-1234")
    user_id         CHAR(26) NULL,                  -- Set when user approves
    scope           VARCHAR(1024) NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, approved, denied, expired
    expires_at      TIMESTAMP NOT NULL,
    poll_interval   INT NOT NULL DEFAULT 5,         -- Seconds between polls
    last_polled_at  TIMESTAMP NULL,
    created_at      TIMESTAMP NULL
);
```

### Existing Table Changes

#### `personal_access_tokens` (already exists)

No schema changes needed. The existing PAT model will be used for long-lived API tokens created by users. OAuth tokens and PATs serve different purposes:
- **OAuth tokens**: Short-lived, issued by OAuth flow, auto-refreshed
- **PATs**: Long-lived, manually created by users, for scripts/automation

---

## OAuth Scopes

Define granular scopes for access control:

| Scope               | Description                          |
|---------------------|--------------------------------------|
| `user:read`         | Read user profile                    |
| `user:write`        | Update user profile                  |
| `teams:read`        | List and view teams                  |
| `teams:write`       | Create/update teams                  |
| `servers:read`      | List and view servers                |
| `servers:write`     | Create/manage servers                |
| `sites:read`        | List and view sites                  |
| `sites:write`       | Create/manage/deploy sites           |
| `databases:read`    | List and view databases              |
| `databases:write`   | Create/manage databases              |
| `*`                 | Full access (first-party clients)    |

CLI and mobile app (first-party) will request `*` scope by default.

---

## API Endpoints

### OAuth Server Endpoints

These are the standard OAuth 2.0 / OpenID Connect endpoints:

```
# Discovery
GET  /.well-known/openid-configuration     # OIDC Discovery document

# Authorization
GET  /oauth/authorize                       # Authorization endpoint (browser)
POST /oauth/token                           # Token endpoint
POST /oauth/revoke                          # Token revocation
POST /oauth/introspect                      # Token introspection

# Device Flow
POST /oauth/device/code                     # Request device + user code
GET  /oauth/device                          # User verification page (browser)
POST /oauth/device/verify                   # User submits code (browser)

# User Info
GET  /oauth/userinfo                        # OIDC UserInfo endpoint

# Keys
GET  /oauth/keys                            # JWKS endpoint (public keys)
```

### OAuth Client Management Endpoints (Admin)

```
GET    /api/oauth/clients                   # List registered clients
POST   /api/oauth/clients                   # Register new client
GET    /api/oauth/clients/:id               # Get client details
PUT    /api/oauth/clients/:id               # Update client
DELETE /api/oauth/clients/:id               # Delete client
```

### Personal Access Token Endpoints (User)

```
GET    /api/user/tokens                     # List user's PATs
POST   /api/user/tokens                     # Create new PAT
DELETE /api/user/tokens/:id                 # Revoke/delete PAT
```

---

## Authentication Flow Details

### Flow 1: Authorization Code + PKCE (CLI with browser)

```
┌──────────┐                    ┌──────────┐                    ┌──────────┐
│ launchctl│                    │  Browser  │                    │ Launch   │
│  (CLI)   │                    │           │                    │  Server  │
└────┬─────┘                    └─────┬─────┘                    └────┬─────┘
     │                                │                               │
     │ 1. Generate PKCE verifier +    │                               │
     │    challenge (S256)            │                               │
     │                                │                               │
     │ 2. Start local HTTP server     │                               │
     │    on localhost:{random_port}  │                               │
     │                                │                               │
     │ 3. Open browser ──────────────►│                               │
     │    /oauth/authorize?           │                               │
     │     client_id=launchctl&       │                               │
     │     redirect_uri=localhost&    │                               │
     │     code_challenge=xxx&        │                               │
     │     code_challenge_method=S256&│                               │
     │     scope=*&state=yyy          │                               │
     │                                │                               │
     │                                │ 4. User logs in ─────────────►│
     │                                │    (if not already)           │
     │                                │                               │
     │                                │ 5. User approves scope ──────►│
     │                                │                               │
     │                                │◄── 6. Redirect to localhost ──│
     │                                │     ?code=zzz&state=yyy      │
     │                                │                               │
     │◄── 7. Receive code on ─────────│                               │
     │       local server             │                               │
     │                                                                │
     │ 8. POST /oauth/token ─────────────────────────────────────────►│
     │    grant_type=authorization_code                               │
     │    code=zzz                                                    │
     │    code_verifier=original_verifier                             │
     │                                                                │
     │◄──────────────── 9. access_token + refresh_token ──────────────│
     │                                                                │
     │ 10. Store tokens in                                            │
     │     ~/.config/launchctl/credentials.json                       │
     │                                                                │
```

### Flow 2: Device Authorization Grant (CLI on headless server)

```
┌──────────┐                    ┌──────────┐                    ┌──────────┐
│ launchctl│                    │  Browser  │                    │ Launch   │
│  (CLI)   │                    │ (any device)│                  │  Server  │
└────┬─────┘                    └─────┬─────┘                    └────┬─────┘
     │                                │                               │
     │ 1. POST /oauth/device/code ───────────────────────────────────►│
     │    client_id=launchctl                                         │
     │                                                                │
     │◄──── 2. device_code, user_code, verification_uri ─────────────│
     │          (e.g. user_code: "ABCD-1234")                        │
     │                                                                │
     │ 3. Display to user:                                            │
     │    "Open https://app.launch.dev/device"                        │
     │    "Enter code: ABCD-1234"                                     │
     │                                │                               │
     │                                │ 4. User opens URL ───────────►│
     │                                │    Logs in, enters code       │
     │                                │                               │
     │                                │◄── 5. "Code approved!" ──────│
     │                                │                               │
     │ 6. Poll POST /oauth/token ───────────────────────────────────►│
     │    grant_type=urn:ietf:params:oauth:grant-type:device_code    │
     │    device_code=xxx                                             │
     │    (every 5 seconds)                                           │
     │                                                                │
     │◄──────────────── 7. access_token + refresh_token ──────────────│
     │                                                                │
     │ 8. Store tokens in                                             │
     │     ~/.config/launchctl/credentials.json                       │
     │                                                                │
```

---

## CLI Tool Design (`launchctl`)

### Location

```
cmd/launchctl/
├── main.go                     # Entry point, root command
├── commands/
│   ├── auth.go                 # login, logout, status, whoami
│   ├── server.go               # server list, server show, server ssh
│   ├── site.go                 # site list, site deploy, site logs
│   ├── team.go                 # team list, team switch
│   ├── database.go             # db list, db shell
│   └── config.go               # config set, config get
├── client/
│   ├── api.go                  # HTTP client for Launch API
│   ├── auth.go                 # OAuth flow handlers (PKCE + Device)
│   └── config.go               # Token storage, config management
└── ui/
    ├── spinner.go              # Terminal spinners
    ├── table.go                # Table output formatting
    └── prompt.go               # Interactive prompts
```

### Dependencies

- `github.com/spf13/cobra` — CLI framework (or `github.com/urfave/cli/v2`)
- `github.com/pkg/browser` — Open URLs in default browser
- `github.com/charmbracelet/lipgloss` — Terminal styling (optional)
- `github.com/charmbracelet/bubbletea` — TUI components (optional)

### Command Structure

```
launchctl
├── auth
│   ├── login                   # Browser-based login (PKCE or Device flow)
│   ├── login --headless        # Force device flow (for SSH sessions)
│   ├── logout                  # Revoke tokens and clear local credentials
│   ├── status                  # Show current auth status
│   └── token                   # Print current access token (for scripts)
├── server
│   ├── list                    # List servers in current team
│   ├── show <id>               # Server details
│   ├── ssh <id>                # SSH into server
│   ├── reboot <id>             # Reboot server
│   └── provision <id>          # Re-provision server
├── site
│   ├── list                    # List sites
│   ├── show <id>               # Site details
│   ├── deploy <id>             # Trigger deployment
│   ├── logs <id>               # Stream deployment logs
│   └── rollback <id>           # Rollback to previous release
├── team
│   ├── list                    # List teams
│   ├── switch <id>             # Switch active team
│   └── current                 # Show current team
├── db
│   ├── list                    # List databases
│   └── shell <id>              # Open database shell
├── token
│   ├── list                    # List personal access tokens
│   ├── create <name>           # Create new PAT
│   └── revoke <id>             # Revoke a PAT
└── config
    ├── set <key> <value>       # Set config (api_url, default_team)
    └── get <key>               # Get config value
```

### Credential Storage

```
~/.config/launchctl/
├── config.json                 # API URL, default team, preferences
│   {
│     "api_url": "https://api.launch.dev",
│     "default_team_id": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
│   }
└── credentials.json            # OAuth tokens (file permissions: 0600)
    {
      "access_token": "eyJhbGciOi...",
      "refresh_token": "dGhpcyBpcyBh...",
      "token_type": "Bearer",
      "expires_at": "2026-02-08T15:30:00Z",
      "scopes": ["*"]
    }
```

### Auto-Refresh

The CLI HTTP client will automatically refresh expired access tokens using the refresh token before making API calls. If the refresh token is also expired, it prompts the user to `launchctl auth login` again.

### Login Flow Selection

```
$ launchctl auth login

? How would you like to authenticate?
> Login with browser (opens default browser)
  Login with device code (for headless/SSH sessions)
  Enter a personal access token
```

On headless servers (no `$DISPLAY`, no `$BROWSER`), automatically defaults to device code flow.

---

## Server-Side Implementation Phases

### Phase 1: OAuth Module Setup

- Create `internal/modules/oauth/` module
- Add `zitadel/oidc` dependency
- Define models: `OAuthClient`, `OAuthAuthorizationCode`, `OAuthAccessToken`, `OAuthRefreshToken`, `OAuthDeviceCode`
- Create migrations for all new tables
- Implement GORM-backed storage adapter for `zitadel/oidc`

### Phase 2: OAuth Authorization Server

- Implement OIDC Provider configuration
- Wire up `zitadel/oidc` server handlers via Fiber adaptor
- Configure signing keys (RSA or ECDSA for JWT)
- Implement authorization endpoint with consent screen
- Implement token endpoint (auth code exchange, refresh)
- Implement PKCE validation (S256)
- Add JWKS endpoint

### Phase 3: Device Authorization Grant

- Implement device code generation endpoint
- Build device verification page (web UI where user enters code)
- Implement device token polling endpoint
- Handle rate limiting on polling (RFC 8628 `slow_down` response)
- Handle code expiration and cleanup

### Phase 4: Seed First-Party Clients

- Create migration/seeder for default OAuth clients:
  - `launchctl` (public client, auth_code + device_code + refresh_token)
  - `launch-mobile` (public client, auth_code + refresh_token)
  - `launch-web` (confidential client, auth_code + refresh_token)
- Store client IDs in config for easy reference

### Phase 5: Personal Access Token API

- Add handlers for PAT CRUD (list, create, delete)
- Wire up routes under `/api/user/tokens`
- Scope validation for PATs
- Update auth middleware to accept PATs (check `Authorization: Bearer` against both JWT and PAT)

### Phase 6: Auth Middleware Updates

- Update existing `Auth()` middleware to accept:
  1. JWT access tokens (existing behavior)
  2. OAuth access tokens (new)
  3. Personal Access Tokens (new)
- Token type detection order: JWT -> OAuth -> PAT
- Add scope-based authorization middleware

---

## CLI Implementation Phases

### Phase 7: CLI Scaffold

- Create `cmd/launchctl/` directory
- Set up Cobra (or urfave/cli) root command
- Add `--api-url`, `--team`, `--format` global flags
- Implement config file management (`~/.config/launchctl/`)

### Phase 8: CLI Auth Commands

- Implement `auth login` with PKCE flow (local HTTP callback server)
- Implement `auth login --headless` with Device flow
- Implement `auth logout` (revoke + clear local tokens)
- Implement `auth status` / `auth whoami`
- Implement credential storage with file permissions (0600)
- Implement auto-refresh in HTTP client

### Phase 9: CLI Core Commands

- Implement `team list`, `team switch`, `team current`
- Implement `server list`, `server show`
- Implement `site list`, `site deploy`, `site logs`
- Implement `token list`, `token create`, `token revoke`

### Phase 10: CLI Polish

- Add interactive prompts for missing arguments
- Add `--json` output format for scripting
- Add shell completion (bash, zsh, fish)
- Add `--verbose` / `--debug` flags
- Build and distribution setup (Makefile targets, Homebrew formula)

---

## Configuration

### New Environment Variables

```env
# OAuth Server
OAUTH_ISSUER_URL=https://api.launch.dev       # OAuth issuer URL
OAUTH_SIGNING_KEY_PATH=./keys/oauth.pem        # Path to RSA/ECDSA private key
OAUTH_DEVICE_CODE_EXPIRY=900                   # Device code TTL in seconds (15 min)
OAUTH_AUTH_CODE_EXPIRY=600                     # Auth code TTL in seconds (10 min)
OAUTH_ACCESS_TOKEN_EXPIRY=900                  # Access token TTL in seconds (15 min)
OAUTH_REFRESH_TOKEN_EXPIRY=2592000             # Refresh token TTL in seconds (30 days)
```

---

## Security Considerations

1. **PKCE is mandatory** for all public clients (CLI, mobile). `code_challenge_method=S256` required.
2. **No client secrets for public clients** — CLI and mobile app are public OAuth clients; security relies on PKCE.
3. **Device codes** expire after 15 minutes. User codes are short (8 chars) and single-use.
4. **Token storage** on CLI uses file permissions `0600` (owner read/write only).
5. **Refresh token rotation** — issue new refresh token on each refresh, revoke the old one.
6. **Rate limit** device code polling to prevent brute force (RFC 8628 `slow_down`).
7. **Consent screen** for third-party OAuth clients. First-party clients (CLI, mobile) can auto-approve.
8. **Signing keys** — use asymmetric keys (RSA-256 or ECDSA P-256) for JWT signing so resource servers can verify without shared secrets.

---

## Open Questions

1. **Consent screen**: Should first-party clients (CLI, mobile) skip the consent screen and auto-approve? (Recommended: yes, auto-approve for first-party)
2. **Third-party OAuth clients**: Do we need to support external developers registering OAuth apps? (Recommended: not in v1, add later if needed)
3. **OIDC compliance**: Do we need full OpenID Connect compliance (ID tokens, userinfo endpoint), or is plain OAuth 2.0 sufficient? (Recommended: implement OIDC since `zitadel/oidc` provides it and mobile apps benefit from ID tokens)
4. **CLI distribution**: Homebrew only, or also curl-based install script? (Can decide during Phase 10)
5. **Web app migration**: Should the existing web app also migrate to OAuth for login, or keep using the current JWT flow? (Recommended: keep current flow for now, migrate later)
