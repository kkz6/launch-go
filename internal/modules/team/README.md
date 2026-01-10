# Team Module

Handles team management for multi-tenancy support.

## Features

- Team CRUD operations
- Team member invitations
- Member management
- Role-based access within teams

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/teams` | List user's teams |
| POST | `/api/teams` | Create new team |
| GET | `/api/teams/:id` | Get team details |
| PUT | `/api/teams/:id` | Update team |
| DELETE | `/api/teams/:id` | Delete team |
| POST | `/api/teams/:id/invite` | Invite member |
| DELETE | `/api/teams/:id/members/:userId` | Remove member |

## Multi-Tenancy

All resources (servers, sites, etc.) are scoped to teams. The current team is stored in the JWT token and enforced by middleware.

### Team Scoping Flow
```
1. User authenticates → JWT includes team_id
2. Request hits API → Auth middleware extracts team_id
3. TeamScope middleware validates team membership
4. All queries filtered by team_id
```

## Roles

| Role | Description |
|------|-------------|
| `owner` | Full access, can delete team |
| `admin` | Can manage members and resources |
| `member` | Can manage resources |

## Laravel Migration Reference

| Laravel | Go |
|---------|-----|
| `modules/auth/src/Models/Team.php` | `auth/models.go` (Team model) |
| Team invitations | TODO |
| Spatie permissions | TODO |

## Status

**Current**: Scaffold only - methods return empty responses.

**TODO**:
- [ ] Full team CRUD implementation
- [ ] Team invitation system with email
- [ ] Role and permission management
- [ ] Team settings and billing association
