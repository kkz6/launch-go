package services

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/auth/contracts"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/pkg/cache"
	launchcache "github.com/kkz6/launch-go/internal/pkg/launch/cache"
)

// Service aggregates all auth-related services.
// Access sub-services directly via exported fields.
type Service struct {
	Auth              *AuthService
	User              *UserService
	EmailVerification *EmailVerificationService
	PasswordReset     *PasswordResetService
	TwoFactor         *TwoFactorService
	Team              *TeamService
	TeamMember        *TeamMemberService
	Passkey           *PasskeyService

	repos           contracts.RepositoryRegistry
	config          *config.Config
	logger          *zerolog.Logger
	membershipCache *launchcache.TeamMembershipCache
}

// SetMembershipCache wires the team-membership cache used to resolve the
// caller's role in their current team. Set during module bootstrap; when
// nil (e.g. in unit tests) CurrentTeamRole returns an empty string.
func (s *Service) SetMembershipCache(c *launchcache.TeamMembershipCache) {
	s.membershipCache = c
	s.Auth.SetMembershipCache(c)
	s.Team.SetMembershipCache(c)
	s.TeamMember.SetMembershipCache(c)
}

// CurrentTeamRole returns the user's role in the given team (owner / admin
// / editor / member), or an empty string when there's no team or the
// membership cache isn't wired. Drives frontend UI gating only — the
// backend remains the authorization source of truth.
func (s *Service) CurrentTeamRole(ctx context.Context, userID, teamID string) string {
	if s.membershipCache == nil || teamID == "" {
		return ""
	}
	role, err := s.membershipCache.GetRole(ctx, userID, teamID)
	if err != nil {
		return ""
	}
	return role
}

// NewService creates a new Service instance
func NewService(repos contracts.RepositoryRegistry, cfg *config.Config, logger *zerolog.Logger, emailSender channels.EmailSender, c cache.Cache) (*Service, error) {
	passkeyService, err := NewPasskeyService(repos, cfg, c, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create passkey service: %w", err)
	}

	authService := NewAuthService(repos, cfg, logger, c)

	return &Service{
		repos:             repos,
		config:            cfg,
		logger:            logger,
		Auth:              authService,
		User:              NewUserService(repos),
		EmailVerification: NewEmailVerificationService(repos, cfg, emailSender),
		PasswordReset:     NewPasswordResetService(repos, cfg, emailSender),
		TwoFactor:         NewTwoFactorService(repos, cfg, authService),
		Team:              NewTeamService(repos),
		TeamMember:        NewTeamMemberService(repos, cfg, emailSender, logger),
		Passkey:           passkeyService,
	}, nil
}

// Repos returns the repository registry
func (s *Service) Repos() contracts.RepositoryRegistry {
	return s.repos
}

// IsTeamSubscribed checks if a team has an active subscription
func (s *Service) IsTeamSubscribed(ctx context.Context, teamID string) bool {
	return s.repos.IsTeamSubscribed(ctx, teamID)
}

// IsUserAdmin checks if a user has admin or manager role
func (s *Service) IsUserAdmin(ctx context.Context, userID string) bool {
	return s.repos.IsUserAdmin(ctx, userID)
}

// IsTeamSubscribedOrUserAdmin checks if a team is subscribed or the user is an admin.
// Admins bypass subscription requirements.
func (s *Service) IsTeamSubscribedOrUserAdmin(ctx context.Context, teamID, userID string) bool {
	if s.repos.IsUserAdmin(ctx, userID) {
		return true
	}
	return s.repos.IsTeamSubscribed(ctx, teamID)
}

// IsUserSubscribed checks subscription for the user's current team.
func (s *Service) IsUserSubscribed(ctx context.Context, user *models.User) bool {
	if user == nil || user.CurrentTeamID == nil {
		return false
	}

	return s.IsTeamSubscribedOrUserAdmin(ctx, *user.CurrentTeamID, user.ID)
}

// SetOnboarded updates the user's onboarded status
func (s *Service) SetOnboarded(ctx context.Context, userID string, onboarded bool) error {
	return s.repos.User().SetOnboarded(ctx, userID, onboarded)
}
