package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/auth/models"
)

// TeamMemberRepository handles team member database operations
type TeamMemberRepository struct {
	db *gorm.DB
}

// NewTeamMemberRepository creates a new TeamMemberRepository instance
func NewTeamMemberRepository(db *gorm.DB) *TeamMemberRepository {
	return &TeamMemberRepository{db: db}
}

// AddUser adds a user to a team with a specified role
func (r *TeamMemberRepository) AddUser(ctx context.Context, teamID, userID, role string) error {
	member := models.TeamMember{
		TeamID: teamID,
		UserID: userID,
		Role:   &role,
	}

	return r.db.WithContext(ctx).Create(&member).Error
}

// RemoveUser removes a user from a team
func (r *TeamMemberRepository) RemoveUser(ctx context.Context, teamID, userID string) error {
	return r.db.WithContext(ctx).
		Where("team_id = ? AND user_id = ?", teamID, userID).
		Delete(&models.TeamMember{}).Error
}

// UpdateRole updates a team member's role
func (r *TeamMemberRepository) UpdateRole(ctx context.Context, teamID, userID, role string) error {
	return r.db.WithContext(ctx).
		Model(&models.TeamMember{}).
		Where("team_id = ? AND user_id = ?", teamID, userID).
		Update("role", role).Error
}

// Get gets a specific team member
func (r *TeamMemberRepository) Get(ctx context.Context, teamID, userID string) (*models.TeamMember, error) {
	var member models.TeamMember
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

// IsMember checks if a user is a member of a team
func (r *TeamMemberRepository) IsMember(ctx context.Context, teamID, userID string) (bool, error) {
	var count int64

	// Check if user is the owner
	var team models.Team
	if err := r.db.WithContext(ctx).First(&team, "id = ?", teamID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}

		return false, err
	}

	if team.UserID == userID {
		return true, nil
	}

	// Check if user is a member
	err := r.db.WithContext(ctx).
		Model(&models.TeamMember{}).
		Where("team_id = ? AND user_id = ?", teamID, userID).
		Count(&count).Error

	return count > 0, err
}
