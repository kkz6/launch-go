package auth

import (
	"context"

	"github.com/kkz6/launch-go/internal/middleware"
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
	team, err := a.service.Team.GetTeam(ctx, teamID)
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

// GetUser implements middleware.EmailVerifiedService
func (a *MiddlewareAdapter) GetUser(ctx context.Context, userID string) (middleware.UserInfo, error) {
	user, err := a.service.User.GetUser(ctx, userID)
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
	return a.service.TwoFactor.HasTwoFactorEnabled(ctx, userID)
}

// IsSessionTwoFactorVerified implements middleware.TwoFactorService
func (a *MiddlewareAdapter) IsSessionTwoFactorVerified(ctx context.Context, sessionID string) (bool, error) {
	return a.service.Repos().Session().IsTwoFactorVerified(ctx, sessionID)
}

var (
	_ middleware.TeamService          = (*MiddlewareAdapter)(nil)
	_ middleware.EmailVerifiedService = (*MiddlewareAdapter)(nil)
	_ middleware.TwoFactorService     = (*MiddlewareAdapter)(nil)
)
