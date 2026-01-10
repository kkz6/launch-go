package auth

import (
	"context"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateUser(ctx context.Context, user *User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *Repository) FindUserByID(ctx context.Context, id string) (*User, error) {
	var user User
	err := r.db.WithContext(ctx).
		Preload("CurrentTeam").
		First(&user, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) FindUserByEmail(ctx context.Context, email string) (*User, error) {
	var user User
	err := r.db.WithContext(ctx).
		Preload("CurrentTeam").
		First(&user, "email = ?", email).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) UpdateUser(ctx context.Context, user *User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

func (r *Repository) CreateTeam(ctx context.Context, team *Team) error {
	return r.db.WithContext(ctx).Create(team).Error
}

func (r *Repository) FindTeamByID(ctx context.Context, id string) (*Team, error) {
	var team Team
	err := r.db.WithContext(ctx).First(&team, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &team, nil
}

func (r *Repository) AddUserToTeam(ctx context.Context, teamID, userID, role string) error {
	member := TeamMember{
		TeamID: teamID,
		UserID: userID,
		Role:   role,
	}
	return r.db.WithContext(ctx).Create(&member).Error
}

func (r *Repository) GetUserTeams(ctx context.Context, userID string) ([]Team, error) {
	var teams []Team
	err := r.db.WithContext(ctx).
		Joins("JOIN team_members ON team_members.team_id = teams.id").
		Where("team_members.user_id = ?", userID).
		Find(&teams).Error
	return teams, err
}

func (r *Repository) SetCurrentTeam(ctx context.Context, userID, teamID string) error {
	return r.db.WithContext(ctx).
		Model(&User{}).
		Where("id = ?", userID).
		Update("current_team_id", teamID).Error
}

func (r *Repository) UserExistsByEmail(ctx context.Context, email string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&User{}).
		Where("email = ?", email).
		Count(&count).Error
	return count > 0, err
}
