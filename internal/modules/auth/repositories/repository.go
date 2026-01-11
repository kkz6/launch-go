package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/auth/models"
)

// Repository aggregates all auth-related repositories
type Repository struct {
	db                 *gorm.DB
	user               *UserRepository
	team               *TeamRepository
	teamMember         *TeamMemberRepository
	teamInvitation     *TeamInvitationRepository
	passwordResetToken *PasswordResetTokenRepository
	personalAccessToken *PersonalAccessTokenRepository
}

// NewRepository creates a new Repository instance
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{
		db:                 db,
		user:               NewUserRepository(db),
		team:               NewTeamRepository(db),
		teamMember:         NewTeamMemberRepository(db),
		teamInvitation:     NewTeamInvitationRepository(db),
		passwordResetToken: NewPasswordResetTokenRepository(db),
		personalAccessToken: NewPersonalAccessTokenRepository(db),
	}
}

// User Repository Methods

func (r *Repository) CreateUser(ctx context.Context, user *models.User) error {
	return r.user.Create(ctx, user)
}

func (r *Repository) FindUserByID(ctx context.Context, id string) (*models.User, error) {
	return r.user.FindByID(ctx, id)
}

func (r *Repository) FindUserByEmail(ctx context.Context, email string) (*models.User, error) {
	return r.user.FindByEmail(ctx, email)
}

func (r *Repository) UpdateUser(ctx context.Context, user *models.User) error {
	return r.user.Update(ctx, user)
}

func (r *Repository) DeleteUser(ctx context.Context, id string) error {
	return r.user.Delete(ctx, id)
}

func (r *Repository) UserExistsByEmail(ctx context.Context, email string) (bool, error) {
	return r.user.ExistsByEmail(ctx, email)
}

func (r *Repository) SetCurrentTeam(ctx context.Context, userID, teamID string) error {
	return r.user.SetCurrentTeam(ctx, userID, teamID)
}

func (r *Repository) MarkEmailAsVerified(ctx context.Context, userID string) error {
	return r.user.MarkEmailAsVerified(ctx, userID)
}

// Team Repository Methods

func (r *Repository) CreateTeam(ctx context.Context, team *models.Team) error {
	return r.team.Create(ctx, team)
}

func (r *Repository) FindTeamByID(ctx context.Context, id string) (*models.Team, error) {
	return r.team.FindByID(ctx, id)
}

func (r *Repository) UpdateTeam(ctx context.Context, team *models.Team) error {
	return r.team.Update(ctx, team)
}

func (r *Repository) DeleteTeam(ctx context.Context, id string) error {
	return r.team.Delete(ctx, id)
}

func (r *Repository) GetUserTeams(ctx context.Context, userID string) ([]models.Team, error) {
	return r.team.GetUserTeams(ctx, userID)
}

func (r *Repository) GetTeamMembers(ctx context.Context, teamID string) ([]models.TeamMember, error) {
	return r.team.GetMembers(ctx, teamID)
}

// Team Member Repository Methods

func (r *Repository) AddUserToTeam(ctx context.Context, teamID, userID, role string) error {
	return r.teamMember.AddUser(ctx, teamID, userID, role)
}

func (r *Repository) RemoveUserFromTeam(ctx context.Context, teamID, userID string) error {
	return r.teamMember.RemoveUser(ctx, teamID, userID)
}

func (r *Repository) UpdateTeamMemberRole(ctx context.Context, teamID, userID, role string) error {
	return r.teamMember.UpdateRole(ctx, teamID, userID, role)
}

func (r *Repository) GetTeamMember(ctx context.Context, teamID, userID string) (*models.TeamMember, error) {
	return r.teamMember.Get(ctx, teamID, userID)
}

func (r *Repository) IsTeamMember(ctx context.Context, teamID, userID string) (bool, error) {
	return r.teamMember.IsMember(ctx, teamID, userID)
}

// Team Invitation Repository Methods

func (r *Repository) CreateTeamInvitation(ctx context.Context, invitation *models.TeamInvitation) error {
	return r.teamInvitation.Create(ctx, invitation)
}

func (r *Repository) FindTeamInvitationByID(ctx context.Context, id string) (*models.TeamInvitation, error) {
	return r.teamInvitation.FindByID(ctx, id)
}

func (r *Repository) FindTeamInvitationByEmail(ctx context.Context, teamID, email string) (*models.TeamInvitation, error) {
	return r.teamInvitation.FindByEmail(ctx, teamID, email)
}

func (r *Repository) GetTeamInvitations(ctx context.Context, teamID string) ([]models.TeamInvitation, error) {
	return r.teamInvitation.GetByTeam(ctx, teamID)
}

func (r *Repository) DeleteTeamInvitation(ctx context.Context, id string) error {
	return r.teamInvitation.Delete(ctx, id)
}

// Password Reset Token Repository Methods

func (r *Repository) CreatePasswordResetToken(ctx context.Context, token *models.PasswordResetToken) error {
	return r.passwordResetToken.Create(ctx, token)
}

func (r *Repository) FindPasswordResetToken(ctx context.Context, email string) (*models.PasswordResetToken, error) {
	return r.passwordResetToken.FindByEmail(ctx, email)
}

func (r *Repository) DeletePasswordResetToken(ctx context.Context, email string) error {
	return r.passwordResetToken.Delete(ctx, email)
}

// Personal Access Token Repository Methods

func (r *Repository) CreatePersonalAccessToken(ctx context.Context, token *models.PersonalAccessToken) error {
	return r.personalAccessToken.Create(ctx, token)
}

func (r *Repository) FindPersonalAccessToken(ctx context.Context, token string) (*models.PersonalAccessToken, error) {
	return r.personalAccessToken.FindByToken(ctx, token)
}

func (r *Repository) UpdatePersonalAccessTokenLastUsed(ctx context.Context, id string) error {
	return r.personalAccessToken.UpdateLastUsed(ctx, id)
}

func (r *Repository) DeletePersonalAccessToken(ctx context.Context, id string) error {
	return r.personalAccessToken.Delete(ctx, id)
}

func (r *Repository) GetUserPersonalAccessTokens(ctx context.Context, userID string) ([]models.PersonalAccessToken, error) {
	return r.personalAccessToken.GetByUser(ctx, userID)
}

// Transaction executes a function within a database transaction
func (r *Repository) Transaction(ctx context.Context, fn func(tx *Repository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(NewRepository(tx))
	})
}

// DB returns the underlying database connection
func (r *Repository) DB() *gorm.DB {
	return r.db
}
