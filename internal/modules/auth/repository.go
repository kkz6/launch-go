package auth

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// RepositoryInterface defines the interface for auth repository operations
type RepositoryInterface interface {
	// User operations
	CreateUser(ctx context.Context, user *User) error
	FindUserByID(ctx context.Context, id string) (*User, error)
	FindUserByEmail(ctx context.Context, email string) (*User, error)
	UpdateUser(ctx context.Context, user *User) error
	DeleteUser(ctx context.Context, id string) error
	UserExistsByEmail(ctx context.Context, email string) (bool, error)
	SetCurrentTeam(ctx context.Context, userID, teamID string) error
	MarkEmailAsVerified(ctx context.Context, userID string) error

	// Team operations
	CreateTeam(ctx context.Context, team *Team) error
	FindTeamByID(ctx context.Context, id string) (*Team, error)
	UpdateTeam(ctx context.Context, team *Team) error
	DeleteTeam(ctx context.Context, id string) error
	GetUserTeams(ctx context.Context, userID string) ([]Team, error)
	GetTeamMembers(ctx context.Context, teamID string) ([]TeamMember, error)

	// Team Member operations
	AddUserToTeam(ctx context.Context, teamID, userID, role string) error
	RemoveUserFromTeam(ctx context.Context, teamID, userID string) error
	UpdateTeamMemberRole(ctx context.Context, teamID, userID, role string) error
	GetTeamMember(ctx context.Context, teamID, userID string) (*TeamMember, error)
	IsTeamMember(ctx context.Context, teamID, userID string) (bool, error)

	// Team Invitation operations
	CreateTeamInvitation(ctx context.Context, invitation *TeamInvitation) error
	FindTeamInvitationByID(ctx context.Context, id string) (*TeamInvitation, error)
	FindTeamInvitationByEmail(ctx context.Context, teamID, email string) (*TeamInvitation, error)
	GetTeamInvitations(ctx context.Context, teamID string) ([]TeamInvitation, error)
	DeleteTeamInvitation(ctx context.Context, id string) error

	// Password Reset operations
	CreatePasswordResetToken(ctx context.Context, token *PasswordResetToken) error
	FindPasswordResetToken(ctx context.Context, email string) (*PasswordResetToken, error)
	DeletePasswordResetToken(ctx context.Context, email string) error

	// Personal Access Token operations
	CreatePersonalAccessToken(ctx context.Context, token *PersonalAccessToken) error
	FindPersonalAccessToken(ctx context.Context, token string) (*PersonalAccessToken, error)
	UpdatePersonalAccessTokenLastUsed(ctx context.Context, id string) error
	DeletePersonalAccessToken(ctx context.Context, id string) error
	GetUserPersonalAccessTokens(ctx context.Context, userID string) ([]PersonalAccessToken, error)
}

// Repository implements RepositoryInterface for database operations
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new Repository instance
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// User operations

// CreateUser creates a new user in the database
func (r *Repository) CreateUser(ctx context.Context, user *User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

// FindUserByID finds a user by their ID
func (r *Repository) FindUserByID(ctx context.Context, id string) (*User, error) {
	var user User
	err := r.db.WithContext(ctx).
		Preload("CurrentTeam").
		Preload("Teams").
		First(&user, "id = ?", id).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &user, nil
}

// FindUserByEmail finds a user by their email
func (r *Repository) FindUserByEmail(ctx context.Context, email string) (*User, error) {
	var user User
	err := r.db.WithContext(ctx).
		Preload("CurrentTeam").
		Preload("Teams").
		First(&user, "email = ?", email).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &user, nil
}

// UpdateUser updates an existing user
func (r *Repository) UpdateUser(ctx context.Context, user *User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

// DeleteUser deletes a user by their ID
func (r *Repository) DeleteUser(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&User{}, "id = ?", id).Error
}

// UserExistsByEmail checks if a user with the given email exists
func (r *Repository) UserExistsByEmail(ctx context.Context, email string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&User{}).
		Where("email = ?", email).
		Count(&count).Error

	return count > 0, err
}

// SetCurrentTeam sets the user's current team
func (r *Repository) SetCurrentTeam(ctx context.Context, userID, teamID string) error {
	return r.db.WithContext(ctx).
		Model(&User{}).
		Where("id = ?", userID).
		Update("current_team_id", teamID).Error
}

// MarkEmailAsVerified marks a user's email as verified
func (r *Repository) MarkEmailAsVerified(ctx context.Context, userID string) error {
	now := time.Now()

	return r.db.WithContext(ctx).
		Model(&User{}).
		Where("id = ?", userID).
		Update("email_verified_at", &now).Error
}

// Team operations

// CreateTeam creates a new team
func (r *Repository) CreateTeam(ctx context.Context, team *Team) error {
	return r.db.WithContext(ctx).Create(team).Error
}

// FindTeamByID finds a team by its ID
func (r *Repository) FindTeamByID(ctx context.Context, id string) (*Team, error) {
	var team Team
	err := r.db.WithContext(ctx).
		Preload("Owner").
		Preload("Members").
		First(&team, "id = ?", id).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &team, nil
}

// UpdateTeam updates an existing team
func (r *Repository) UpdateTeam(ctx context.Context, team *Team) error {
	return r.db.WithContext(ctx).Save(team).Error
}

// DeleteTeam deletes a team by its ID
func (r *Repository) DeleteTeam(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete team members first
		if err := tx.Where("team_id = ?", id).Delete(&TeamMember{}).Error; err != nil {
			return err
		}

		// Delete team invitations
		if err := tx.Where("team_id = ?", id).Delete(&TeamInvitation{}).Error; err != nil {
			return err
		}

		// Update users who have this as their current team
		if err := tx.Model(&User{}).
			Where("current_team_id = ?", id).
			Update("current_team_id", nil).Error; err != nil {
			return err
		}

		// Delete the team
		return tx.Delete(&Team{}, "id = ?", id).Error
	})
}

// GetUserTeams gets all teams for a user (owned and member of)
func (r *Repository) GetUserTeams(ctx context.Context, userID string) ([]Team, error) {
	var teams []Team

	// Get teams where user is owner
	var ownedTeams []Team
	if err := r.db.WithContext(ctx).
		Where("owner_id = ?", userID).
		Find(&ownedTeams).Error; err != nil {
		return nil, err
	}

	// Get teams where user is a member
	var memberTeams []Team
	if err := r.db.WithContext(ctx).
		Joins("JOIN team_members ON team_members.team_id = teams.id").
		Where("team_members.user_id = ?", userID).
		Find(&memberTeams).Error; err != nil {
		return nil, err
	}

	// Combine and deduplicate
	teamMap := make(map[string]Team)
	for _, t := range ownedTeams {
		teamMap[t.ID] = t
	}

	for _, t := range memberTeams {
		if _, exists := teamMap[t.ID]; !exists {
			teamMap[t.ID] = t
		}
	}

	for _, t := range teamMap {
		teams = append(teams, t)
	}

	return teams, nil
}

// GetTeamMembers gets all members of a team
func (r *Repository) GetTeamMembers(ctx context.Context, teamID string) ([]TeamMember, error) {
	var members []TeamMember
	err := r.db.WithContext(ctx).
		Preload("User").
		Where("team_id = ?", teamID).
		Find(&members).Error

	return members, err
}

// Team Member operations

// AddUserToTeam adds a user to a team with a specified role
func (r *Repository) AddUserToTeam(ctx context.Context, teamID, userID, role string) error {
	member := TeamMember{
		TeamID: teamID,
		UserID: userID,
		Role:   role,
	}

	return r.db.WithContext(ctx).Create(&member).Error
}

// RemoveUserFromTeam removes a user from a team
func (r *Repository) RemoveUserFromTeam(ctx context.Context, teamID, userID string) error {
	return r.db.WithContext(ctx).
		Where("team_id = ? AND user_id = ?", teamID, userID).
		Delete(&TeamMember{}).Error
}

// UpdateTeamMemberRole updates a team member's role
func (r *Repository) UpdateTeamMemberRole(ctx context.Context, teamID, userID, role string) error {
	return r.db.WithContext(ctx).
		Model(&TeamMember{}).
		Where("team_id = ? AND user_id = ?", teamID, userID).
		Update("role", role).Error
}

// GetTeamMember gets a specific team member
func (r *Repository) GetTeamMember(ctx context.Context, teamID, userID string) (*TeamMember, error) {
	var member TeamMember
	err := r.db.WithContext(ctx).
		Preload("User").
		Where("team_id = ? AND user_id = ?", teamID, userID).
		First(&member).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &member, nil
}

// IsTeamMember checks if a user is a member of a team
func (r *Repository) IsTeamMember(ctx context.Context, teamID, userID string) (bool, error) {
	var count int64

	// Check if user is the owner
	var team Team
	if err := r.db.WithContext(ctx).First(&team, "id = ?", teamID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}

		return false, err
	}

	if team.OwnerID == userID {
		return true, nil
	}

	// Check if user is a member
	err := r.db.WithContext(ctx).
		Model(&TeamMember{}).
		Where("team_id = ? AND user_id = ?", teamID, userID).
		Count(&count).Error

	return count > 0, err
}

// Team Invitation operations

// CreateTeamInvitation creates a new team invitation
func (r *Repository) CreateTeamInvitation(ctx context.Context, invitation *TeamInvitation) error {
	return r.db.WithContext(ctx).Create(invitation).Error
}

// FindTeamInvitationByID finds a team invitation by its ID
func (r *Repository) FindTeamInvitationByID(ctx context.Context, id string) (*TeamInvitation, error) {
	var invitation TeamInvitation
	err := r.db.WithContext(ctx).
		Preload("Team").
		First(&invitation, "id = ?", id).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &invitation, nil
}

// FindTeamInvitationByEmail finds a team invitation by team ID and email
func (r *Repository) FindTeamInvitationByEmail(ctx context.Context, teamID, email string) (*TeamInvitation, error) {
	var invitation TeamInvitation
	err := r.db.WithContext(ctx).
		Preload("Team").
		Where("team_id = ? AND email = ?", teamID, email).
		First(&invitation).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &invitation, nil
}

// GetTeamInvitations gets all invitations for a team
func (r *Repository) GetTeamInvitations(ctx context.Context, teamID string) ([]TeamInvitation, error) {
	var invitations []TeamInvitation
	err := r.db.WithContext(ctx).
		Preload("Team").
		Where("team_id = ?", teamID).
		Find(&invitations).Error

	return invitations, err
}

// DeleteTeamInvitation deletes a team invitation
func (r *Repository) DeleteTeamInvitation(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&TeamInvitation{}, "id = ?", id).Error
}

// Password Reset operations

// CreatePasswordResetToken creates a new password reset token
func (r *Repository) CreatePasswordResetToken(ctx context.Context, token *PasswordResetToken) error {
	// Delete any existing token for this email first
	r.db.WithContext(ctx).Delete(&PasswordResetToken{}, "email = ?", token.Email)

	return r.db.WithContext(ctx).Create(token).Error
}

// FindPasswordResetToken finds a password reset token by email
func (r *Repository) FindPasswordResetToken(ctx context.Context, email string) (*PasswordResetToken, error) {
	var token PasswordResetToken
	err := r.db.WithContext(ctx).First(&token, "email = ?", email).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &token, nil
}

// DeletePasswordResetToken deletes a password reset token
func (r *Repository) DeletePasswordResetToken(ctx context.Context, email string) error {
	return r.db.WithContext(ctx).Delete(&PasswordResetToken{}, "email = ?", email).Error
}

// Personal Access Token operations

// CreatePersonalAccessToken creates a new personal access token
func (r *Repository) CreatePersonalAccessToken(ctx context.Context, token *PersonalAccessToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

// FindPersonalAccessToken finds a personal access token by its token value
func (r *Repository) FindPersonalAccessToken(ctx context.Context, token string) (*PersonalAccessToken, error) {
	var pat PersonalAccessToken
	err := r.db.WithContext(ctx).
		Preload("User").
		First(&pat, "token = ?", token).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &pat, nil
}

// UpdatePersonalAccessTokenLastUsed updates the last used timestamp
func (r *Repository) UpdatePersonalAccessTokenLastUsed(ctx context.Context, id string) error {
	now := time.Now()

	return r.db.WithContext(ctx).
		Model(&PersonalAccessToken{}).
		Where("id = ?", id).
		Update("last_used_at", &now).Error
}

// DeletePersonalAccessToken deletes a personal access token
func (r *Repository) DeletePersonalAccessToken(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&PersonalAccessToken{}, "id = ?", id).Error
}

// GetUserPersonalAccessTokens gets all personal access tokens for a user
func (r *Repository) GetUserPersonalAccessTokens(ctx context.Context, userID string) ([]PersonalAccessToken, error) {
	var tokens []PersonalAccessToken
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&tokens).Error

	return tokens, err
}

// Transaction executes a function within a database transaction
func (r *Repository) Transaction(ctx context.Context, fn func(tx *Repository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&Repository{db: tx})
	})
}

// DB returns the underlying database connection
func (r *Repository) DB() *gorm.DB {
	return r.db
}
