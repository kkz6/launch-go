package contracts

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
)

// AuthService defines the interface for authentication operations
type AuthService interface {
	Register(ctx context.Context, req *dto.RegisterRequest) (*dto.AuthResponse, error)
	Login(ctx context.Context, req *dto.LoginRequest) (*dto.AuthResponse, error)
	Logout(ctx context.Context, userID string) error
	RefreshToken(ctx context.Context, refreshToken string) (*dto.AuthResponse, error)
}

// UserService defines the interface for user management operations
type UserService interface {
	GetUser(ctx context.Context, userID string) (*models.User, error)
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	UpdateProfile(ctx context.Context, userID string, req *dto.UpdateProfileRequest) (*models.User, error)
	ChangePassword(ctx context.Context, userID string, req *dto.ChangePasswordRequest) error
	DeleteAccount(ctx context.Context, userID string) error
	CheckUserStatus(ctx context.Context, email string) (*dto.UserStatusResponse, error)
}

// EmailVerificationService defines the interface for email verification operations
type EmailVerificationService interface {
	VerifyEmail(ctx context.Context, userID, hash string) error
	ResendVerificationEmail(ctx context.Context, userID string) error
}

// PasswordResetService defines the interface for password reset operations
type PasswordResetService interface {
	SendPasswordResetLink(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, req *dto.ResetPasswordRequest) error
}

// TwoFactorService defines the interface for two-factor authentication operations
type TwoFactorService interface {
	EnableTwoFactor(ctx context.Context, userID string) (*dto.TwoFactorResponse, error)
	ConfirmTwoFactor(ctx context.Context, userID, code string) error
	DisableTwoFactor(ctx context.Context, userID, password string) error
	VerifyTwoFactor(ctx context.Context, userID, code string) (bool, error)
	GetRecoveryCodes(ctx context.Context, userID string) ([]string, error)
	RegenerateRecoveryCodes(ctx context.Context, userID string) ([]string, error)
	HasTwoFactorEnabled(ctx context.Context, userID string) (bool, error)
}

// TeamService defines the interface for team management operations
type TeamService interface {
	CreateTeam(ctx context.Context, userID string, req *dto.CreateTeamRequest) (*models.Team, error)
	UpdateTeam(ctx context.Context, userID, teamID string, req *dto.UpdateTeamRequest) (*models.Team, error)
	DeleteTeam(ctx context.Context, userID, teamID string) error
	GetTeam(ctx context.Context, teamID string) (*models.Team, error)
	GetUserTeams(ctx context.Context, userID string) ([]models.Team, error)
	SwitchTeam(ctx context.Context, userID, teamID string) (*models.User, error)
}

// TeamMemberService defines the interface for team member management operations
type TeamMemberService interface {
	InviteTeamMember(ctx context.Context, userID, teamID string, req *dto.InviteTeamMemberRequest) error
	AcceptTeamInvitation(ctx context.Context, userID, invitationID string) error
	CancelTeamInvitation(ctx context.Context, userID, teamID, invitationID string) error
	UpdateTeamMemberRole(ctx context.Context, userID, teamID, memberID string, req *dto.UpdateTeamMemberRequest) error
	RemoveTeamMember(ctx context.Context, userID, teamID, memberID string) error
	GetTeamMembers(ctx context.Context, teamID string) ([]models.TeamMember, error)
	GetTeamInvitations(ctx context.Context, teamID string) ([]models.TeamInvitation, error)
}

// Service combines all service interfaces
type Service interface {
	AuthService
	UserService
	EmailVerificationService
	PasswordResetService
	TwoFactorService
	TeamService
	TeamMemberService
}
