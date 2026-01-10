# Auth Module

Handles user authentication, registration, and team management for multi-tenancy.

## Features

- User registration and login with JWT tokens
- Password hashing with bcrypt
- Team-based multi-tenancy
- Personal team creation on registration
- Team switching

## API Endpoints

| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| POST | `/api/auth/register` | Register new user | No |
| POST | `/api/auth/login` | Login | No |
| GET | `/api/auth/user` | Get current user | Yes |
| PUT | `/api/auth/profile` | Update profile | Yes |
| PUT | `/api/auth/password` | Change password | Yes |
| POST | `/api/auth/switch-team/:teamId` | Switch current team | Yes |

## Models

### User
```go
type User struct {
    ID              string
    Name            string
    Email           string
    Password        string    // bcrypt hash
    EmailVerifiedAt *time.Time
    TwoFactorSecret *string
    CurrentTeamID   *string
    Teams           []Team    // many-to-many
    OwnedTeams      []Team    // one-to-many
}
```

### Team
```go
type Team struct {
    ID           string
    Name         string
    OwnerID      string
    PersonalTeam bool
    Owner        *User
    Members      []User
}
```

## JWT Token Claims

```json
{
  "sub": "user_id",
  "email": "user@example.com",
  "team_id": "current_team_id",
  "exp": 1234567890,
  "iat": 1234567890
}
```

The `team_id` claim is used by middleware to scope all subsequent requests to the user's current team.

## Request/Response Examples

### Register
```bash
POST /api/auth/register
{
  "name": "John Doe",
  "email": "john@example.com",
  "password": "secretpassword"
}

# Response
{
  "success": true,
  "message": "Registration successful",
  "data": {
    "user": { ... },
    "access_token": "eyJ...",
    "expires_in": 259200
  }
}
```

### Login
```bash
POST /api/auth/login
{
  "email": "john@example.com",
  "password": "secretpassword"
}
```

## Laravel Migration Reference

| Laravel | Go |
|---------|-----|
| `modules/auth/src/Models/User.php` | `models.go` |
| `modules/auth/src/Models/Team.php` | `models.go` |
| `modules/auth/src/Http/Controllers/AuthController.php` | `handler.go` |
| `modules/auth/src/Services/AuthService.php` | `service.go` |
| Laravel Sanctum | JWT with `golang-jwt` |

## Configuration

Environment variables:
- `JWT_SECRET` - Secret key for signing tokens
- `JWT_EXPIRATION` - Token expiration in hours (default: 72)
