package repositories

import (
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/auth/contracts"
)

// Registry holds all auth module repositories
type Registry struct {
	db                  *gorm.DB
	user                *UserRepository
	team                *TeamRepository
	teamMember          *TeamMemberRepository
	teamInvitation      *TeamInvitationRepository
	passwordResetToken  *PasswordResetTokenRepository
	personalAccessToken *PersonalAccessTokenRepository
	passkey             *PasskeyRepository
}

// NewRegistry creates all repositories
func NewRegistry(db *gorm.DB) *Registry {
	return &Registry{
		db:                  db,
		user:                NewUserRepository(db),
		team:                NewTeamRepository(db),
		teamMember:          NewTeamMemberRepository(db),
		teamInvitation:      NewTeamInvitationRepository(db),
		passwordResetToken:  NewPasswordResetTokenRepository(db),
		personalAccessToken: NewPersonalAccessTokenRepository(db),
		passkey:             NewPasskeyRepository(db),
	}
}

// User returns the user repository
func (r *Registry) User() contracts.UserRepository { return r.user }

// Team returns the team repository
func (r *Registry) Team() contracts.TeamRepository { return r.team }

// TeamMember returns the team member repository
func (r *Registry) TeamMember() contracts.TeamMemberRepository { return r.teamMember }

// TeamInvitation returns the team invitation repository
func (r *Registry) TeamInvitation() contracts.TeamInvitationRepository { return r.teamInvitation }

// PasswordResetToken returns the password reset token repository
func (r *Registry) PasswordResetToken() contracts.PasswordResetTokenRepository {
	return r.passwordResetToken
}

// PersonalAccessToken returns the personal access token repository
func (r *Registry) PersonalAccessToken() contracts.PersonalAccessTokenRepository {
	return r.personalAccessToken
}

// Passkey returns the passkey repository
func (r *Registry) Passkey() contracts.PasskeyRepository { return r.passkey }

// DB returns the underlying database connection
func (r *Registry) DB() *gorm.DB { return r.db }
