package services

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/auth/repositories"
)

// Service aggregates all auth-related services
type Service struct {
	repos             *repositories.Registry
	config            *config.Config
	logger            *zerolog.Logger
	auth              *AuthService
	user              *UserService
	emailVerification *EmailVerificationService
	passwordReset     *PasswordResetService
	twoFactor         *TwoFactorService
	team              *TeamService
	teamMember        *TeamMemberService
	passkey           *PasskeyService
}

// NewService creates a new Service instance
func NewService(repos *repositories.Registry, cfg *config.Config, logger *zerolog.Logger) *Service {
	return &Service{
		repos:             repos,
		config:            cfg,
		logger:            logger,
		auth:              NewAuthService(repos, cfg, logger),
		user:              NewUserService(repos),
		emailVerification: NewEmailVerificationService(repos, cfg),
		passwordReset:     NewPasswordResetService(repos),
		twoFactor:         NewTwoFactorService(repos, cfg),
		team:              NewTeamService(repos),
		teamMember:        NewTeamMemberService(repos),
		passkey:           NewPasskeyService(repos),
	}
}

// Authentication Methods

func (s *Service) Register(ctx context.Context, req *dto.RegisterRequest) (*dto.AuthResponse, error) {
	return s.auth.Register(ctx, req)
}

func (s *Service) Login(ctx context.Context, req *dto.LoginRequest) (*dto.AuthResponse, error) {
	return s.auth.Login(ctx, req)
}

func (s *Service) Logout(ctx context.Context, userID string) error {
	s.logger.Info().Str("user_id", userID).Msg("User logged out")

	return s.auth.Logout(ctx, userID)
}

func (s *Service) RefreshToken(ctx context.Context, refreshToken string) (*dto.AuthResponse, error) {
	return s.auth.RefreshToken(ctx, refreshToken)
}

// User Management Methods

func (s *Service) GetUser(ctx context.Context, userID string) (*models.User, error) {
	return s.user.GetUser(ctx, userID)
}

func (s *Service) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	return s.user.GetUserByEmail(ctx, email)
}

func (s *Service) UpdateProfile(ctx context.Context, userID string, req *dto.UpdateProfileRequest) (*models.User, error) {
	return s.user.UpdateProfile(ctx, userID, req)
}

func (s *Service) ChangePassword(ctx context.Context, userID string, req *dto.ChangePasswordRequest) error {
	return s.user.ChangePassword(ctx, userID, req)
}

func (s *Service) DeleteAccount(ctx context.Context, userID string) error {
	return s.user.DeleteAccount(ctx, userID)
}

func (s *Service) CheckUserStatus(ctx context.Context, email string) (*dto.UserStatusResponse, error) {
	return s.user.CheckUserStatus(ctx, email)
}

// Email Verification Methods

func (s *Service) VerifyEmail(ctx context.Context, userID, hash string) error {
	return s.emailVerification.VerifyEmail(ctx, userID, hash)
}

func (s *Service) ResendVerificationEmail(ctx context.Context, userID string) error {
	s.logger.Info().Str("user_id", userID).Msg("Verification email requested")

	return s.emailVerification.ResendVerificationEmail(ctx, userID)
}

// Password Reset Methods

func (s *Service) SendPasswordResetLink(ctx context.Context, email string) error {
	s.logger.Info().Str("email", email).Msg("Password reset email requested")

	return s.passwordReset.SendPasswordResetLink(ctx, email)
}

func (s *Service) ResetPassword(ctx context.Context, req *dto.ResetPasswordRequest) error {
	return s.passwordReset.ResetPassword(ctx, req)
}

// Two-Factor Authentication Methods

func (s *Service) EnableTwoFactor(ctx context.Context, userID string) (*dto.TwoFactorResponse, error) {
	return s.twoFactor.EnableTwoFactor(ctx, userID)
}

func (s *Service) ConfirmTwoFactor(ctx context.Context, userID, code string) error {
	return s.twoFactor.ConfirmTwoFactor(ctx, userID, code)
}

func (s *Service) DisableTwoFactor(ctx context.Context, userID, password string) error {
	return s.twoFactor.DisableTwoFactor(ctx, userID, password)
}

func (s *Service) VerifyTwoFactor(ctx context.Context, userID, code string) (bool, error) {
	return s.twoFactor.VerifyTwoFactor(ctx, userID, code)
}

func (s *Service) GetRecoveryCodes(ctx context.Context, userID string) ([]string, error) {
	return s.twoFactor.GetRecoveryCodes(ctx, userID)
}

func (s *Service) RegenerateRecoveryCodes(ctx context.Context, userID string) ([]string, error) {
	return s.twoFactor.RegenerateRecoveryCodes(ctx, userID)
}

func (s *Service) HasTwoFactorEnabled(ctx context.Context, userID string) (bool, error) {
	return s.twoFactor.HasTwoFactorEnabled(ctx, userID)
}

// Team Management Methods

func (s *Service) CreateTeam(ctx context.Context, userID string, req *dto.CreateTeamRequest) (*models.Team, error) {
	return s.team.CreateTeam(ctx, userID, req)
}

func (s *Service) UpdateTeam(ctx context.Context, userID, teamID string, req *dto.UpdateTeamRequest) (*models.Team, error) {
	return s.team.UpdateTeam(ctx, userID, teamID, req)
}

func (s *Service) DeleteTeam(ctx context.Context, userID, teamID string) error {
	return s.team.DeleteTeam(ctx, userID, teamID)
}

func (s *Service) GetTeam(ctx context.Context, teamID string) (*models.Team, error) {
	return s.team.GetTeam(ctx, teamID)
}

func (s *Service) GetTeamWithDetails(ctx context.Context, teamID string) (*models.Team, []models.TeamMember, []models.TeamInvitation, error) {
	return s.team.GetTeamWithDetails(ctx, teamID)
}

func (s *Service) GetUserTeams(ctx context.Context, userID string) ([]models.Team, error) {
	return s.team.GetUserTeams(ctx, userID)
}

func (s *Service) SwitchTeam(ctx context.Context, userID, teamID string) (*models.User, error) {
	return s.team.SwitchTeam(ctx, userID, teamID)
}

// Team Member Management Methods

func (s *Service) InviteTeamMember(ctx context.Context, userID, teamID string, req *dto.InviteTeamMemberRequest) error {
	s.logger.Info().
		Str("team_id", teamID).
		Str("email", req.Email).
		Str("role", req.Role).
		Msg("Team invitation created")

	return s.teamMember.InviteTeamMember(ctx, userID, teamID, req)
}

func (s *Service) AcceptTeamInvitation(ctx context.Context, userID, invitationID string) error {
	return s.teamMember.AcceptTeamInvitation(ctx, userID, invitationID)
}

func (s *Service) CancelTeamInvitation(ctx context.Context, userID, teamID, invitationID string) error {
	return s.teamMember.CancelTeamInvitation(ctx, userID, teamID, invitationID)
}

func (s *Service) UpdateTeamMemberRole(ctx context.Context, userID, teamID, memberID string, req *dto.UpdateTeamMemberRequest) error {
	return s.teamMember.UpdateTeamMemberRole(ctx, userID, teamID, memberID, req)
}

func (s *Service) RemoveTeamMember(ctx context.Context, userID, teamID, memberID string) error {
	return s.teamMember.RemoveTeamMember(ctx, userID, teamID, memberID)
}

func (s *Service) GetTeamMembers(ctx context.Context, teamID string) ([]models.TeamMember, error) {
	return s.teamMember.GetTeamMembers(ctx, teamID)
}

func (s *Service) GetAllTeamMembers(ctx context.Context, teamID string) ([]dto.TeamMemberResponse, error) {
	return s.teamMember.GetAllTeamMembers(ctx, teamID)
}

func (s *Service) GetTeamInvitations(ctx context.Context, teamID string) ([]models.TeamInvitation, error) {
	return s.teamMember.GetTeamInvitations(ctx, teamID)
}

// Passkey Service Methods

func (s *Service) GetUserPasskeys(ctx context.Context, userID string) ([]models.Passkey, error) {
	return s.passkey.GetUserPasskeys(ctx, userID)
}

func (s *Service) GetPasskey(ctx context.Context, passkeyID string) (*models.Passkey, error) {
	return s.passkey.GetPasskey(ctx, passkeyID)
}

func (s *Service) UpdatePasskeyName(ctx context.Context, passkeyID, userID, name string) error {
	return s.passkey.UpdatePasskeyName(ctx, passkeyID, userID, name)
}

func (s *Service) DeletePasskey(ctx context.Context, passkeyID, userID string) error {
	return s.passkey.DeletePasskey(ctx, passkeyID, userID)
}

// Repos returns the repository registry
func (s *Service) Repos() *repositories.Registry {
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

// IsTeamSubscribedOrUserAdmin checks if a team is subscribed or the user is an admin
// Admins bypass subscription requirements
func (s *Service) IsTeamSubscribedOrUserAdmin(ctx context.Context, teamID, userID string) bool {
	if s.repos.IsUserAdmin(ctx, userID) {
		return true
	}
	return s.repos.IsTeamSubscribed(ctx, teamID)
}
