package auth

import (
	"context"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
)

// MiddlewareAdapter wraps auth service to implement middleware interfaces
type MiddlewareAdapter struct {
	service *services.Service
}

// NewMiddlewareAdapter creates a new middleware adapter
func NewMiddlewareAdapter(service *services.Service) *MiddlewareAdapter {
	return &MiddlewareAdapter{service: service}
}

// GetTeam implements middleware.TeamService
func (a *MiddlewareAdapter) GetTeam(ctx context.Context, teamID string) (middleware.TeamInfo, error) {
	team, err := a.service.GetTeam(ctx, teamID)
	if err != nil {
		return nil, err
	}
	if team == nil {
		return nil, nil
	}
	return team, nil
}

// IsTeamMember implements middleware.TeamService
func (a *MiddlewareAdapter) IsTeamMember(ctx context.Context, teamID, userID string) (bool, error) {
	return a.service.Repos().TeamMember().IsMember(ctx, teamID, userID)
}

// GetTeamMember implements middleware.TeamService
func (a *MiddlewareAdapter) GetTeamMember(ctx context.Context, teamID, userID string) (middleware.TeamMemberInfo, error) {
	member, err := a.service.Repos().TeamMember().Get(ctx, teamID, userID)
	if err != nil {
		return nil, err
	}
	if member == nil {
		return nil, nil
	}
	return member, nil
}

// GetUser implements middleware.UserService
func (a *MiddlewareAdapter) GetUser(ctx context.Context, userID string) (middleware.UserInfo, error) {
	user, err := a.service.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, nil
	}
	return user, nil
}

// HasTwoFactorEnabled implements middleware.TwoFactorService
func (a *MiddlewareAdapter) HasTwoFactorEnabled(ctx context.Context, userID string) (bool, error) {
	return a.service.HasTwoFactorEnabled(ctx, userID)
}

// Ensure MiddlewareAdapter implements the required interfaces
var (
	_ middleware.TeamService      = (*MiddlewareAdapter)(nil)
	_ middleware.UserService      = (*MiddlewareAdapter)(nil)
	_ middleware.TwoFactorService = (*MiddlewareAdapter)(nil)
)

// teamWrapper wraps *models.Team to satisfy middleware.TeamInfo
type teamWrapper struct {
	*models.Team
}

func (t *teamWrapper) GetUserID() string {
	return t.UserID
}

// memberWrapper wraps *models.TeamMember to satisfy middleware.TeamMemberInfo
type memberWrapper struct {
	*models.TeamMember
}

func (m *memberWrapper) GetRole() string {
	if m.Role == nil {
		return ""
	}
	return *m.Role
}

func (m *memberWrapper) IsAdmin() bool {
	return m.Role != nil && *m.Role == "admin"
}
