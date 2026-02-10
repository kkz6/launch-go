package repositories

import (
	"context"

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
	session             *SessionRepository
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
		session:             NewSessionRepository(db),
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

// Session returns the session repository
func (r *Registry) Session() contracts.SessionRepository { return r.session }

// DB returns the underlying database connection
func (r *Registry) DB() *gorm.DB { return r.db }

// IsTeamSubscribed checks if a team has an active subscription
// This queries the subscriptions table directly to avoid circular dependencies
func (r *Registry) IsTeamSubscribed(ctx context.Context, teamID string) bool {
	var count int64
	r.db.WithContext(ctx).Table("subscriptions").
		Where("billable_id = ?", teamID).
		Where("billable_type IN ?", []string{"Modules\\Auth\\Models\\Team", "App\\Models\\Team"}).
		Where("status IN ?", []string{"active", "on_trial"}).
		Count(&count)

	return count > 0
}

// IsUserAdmin checks if a user has admin or manager role (from Spatie Permission tables)
func (r *Registry) IsUserAdmin(ctx context.Context, userID string) bool {
	var count int64
	r.db.WithContext(ctx).Table("model_has_roles").
		Joins("JOIN roles ON roles.id = model_has_roles.role_id").
		Where("model_has_roles.model_id = ?", userID).
		Where("model_has_roles.model_type IN ?", []string{"Modules\\Auth\\Models\\User", "App\\Models\\User"}).
		Where("roles.name IN ?", []string{"admin", "manager"}).
		Count(&count)

	return count > 0
}
