package contracts

import (
	"context"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/auth/models"
)

// UserRepository defines the interface for user repository operations
type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	FindByID(ctx context.Context, id string) (*models.User, error)
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	Update(ctx context.Context, user *models.User) error
	Delete(ctx context.Context, id string) error
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	SetCurrentTeam(ctx context.Context, userID, teamID string) error
	MarkEmailAsVerified(ctx context.Context, userID string) error
	SetOnboarded(ctx context.Context, userID string, onboarded bool) error
}

// TeamRepository defines the interface for team repository operations
type TeamRepository interface {
	Create(ctx context.Context, team *models.Team) error
	FindByID(ctx context.Context, id string) (*models.Team, error)
	Update(ctx context.Context, team *models.Team) error
	Delete(ctx context.Context, id string) error
	GetUserTeams(ctx context.Context, userID string) ([]models.Team, error)
	GetMembers(ctx context.Context, teamID string) ([]models.TeamMember, error)
}

// TeamMemberRepository defines the interface for team member repository operations
type TeamMemberRepository interface {
	AddUser(ctx context.Context, teamID, userID, role string) error
	RemoveUser(ctx context.Context, teamID, userID string) error
	UpdateRole(ctx context.Context, teamID, userID, role string) error
	Get(ctx context.Context, teamID, userID string) (*models.TeamMember, error)
	IsMember(ctx context.Context, teamID, userID string) (bool, error)
}

// TeamInvitationRepository defines the interface for team invitation repository operations
type TeamInvitationRepository interface {
	Create(ctx context.Context, invitation *models.TeamInvitation) error
	FindByID(ctx context.Context, id string) (*models.TeamInvitation, error)
	FindByEmail(ctx context.Context, teamID, email string) (*models.TeamInvitation, error)
	GetByTeam(ctx context.Context, teamID string) ([]models.TeamInvitation, error)
	Delete(ctx context.Context, id string) error
}

// PasswordResetTokenRepository defines the interface for password reset token repository operations
type PasswordResetTokenRepository interface {
	Create(ctx context.Context, token *models.PasswordResetToken) error
	FindByEmail(ctx context.Context, email string) (*models.PasswordResetToken, error)
	Delete(ctx context.Context, email string) error
}

// PersonalAccessTokenRepository defines the interface for personal access token repository operations
type PersonalAccessTokenRepository interface {
	Create(ctx context.Context, token *models.PersonalAccessToken) error
	FindByToken(ctx context.Context, token string) (*models.PersonalAccessToken, error)
	UpdateLastUsed(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
	DeleteByUser(ctx context.Context, id, userID string) (int64, error)
	GetByUser(ctx context.Context, userID string) ([]models.PersonalAccessToken, error)
}

// PasskeyRepository defines the interface for passkey repository operations
type PasskeyRepository interface {
	Create(ctx context.Context, passkey *models.Passkey) error
	FindByID(ctx context.Context, id string) (*models.Passkey, error)
	FindByCredentialID(ctx context.Context, credentialID string) (*models.Passkey, error)
	FindByUserID(ctx context.Context, userID string) ([]models.Passkey, error)
	Update(ctx context.Context, passkey *models.Passkey) error
	Delete(ctx context.Context, id string) error
	DeleteByUserID(ctx context.Context, id, userID string) error
	CountByUserID(ctx context.Context, userID string) (int64, error)
}

// RepositoryRegistry provides access to all auth repositories
type RepositoryRegistry interface {
	User() UserRepository
	Team() TeamRepository
	TeamMember() TeamMemberRepository
	TeamInvitation() TeamInvitationRepository
	PasswordResetToken() PasswordResetTokenRepository
	PersonalAccessToken() PersonalAccessTokenRepository
	Passkey() PasskeyRepository
	DB() *gorm.DB
}
