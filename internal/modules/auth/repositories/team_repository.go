package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/auth/models"
)

// TeamRepository handles team database operations
type TeamRepository struct {
	db *gorm.DB
}

// NewTeamRepository creates a new TeamRepository instance
func NewTeamRepository(db *gorm.DB) *TeamRepository {
	return &TeamRepository{db: db}
}

// Create creates a new team
func (r *TeamRepository) Create(ctx context.Context, team *models.Team) error {
	return r.db.WithContext(ctx).Create(team).Error
}

// FindByID finds a team by its ID
func (r *TeamRepository) FindByID(ctx context.Context, id string) (*models.Team, error) {
	var team models.Team
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

// Update updates an existing team
func (r *TeamRepository) Update(ctx context.Context, team *models.Team) error {
	return r.db.WithContext(ctx).Save(team).Error
}

// Delete deletes a team by its ID
func (r *TeamRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete team members first
		if err := tx.Where("team_id = ?", id).Delete(&models.TeamMember{}).Error; err != nil {
			return err
		}

		// Delete team invitations
		if err := tx.Where("team_id = ?", id).Delete(&models.TeamInvitation{}).Error; err != nil {
			return err
		}

		// Update users who have this as their current team
		if err := tx.Model(&models.User{}).
			Where("current_team_id = ?", id).
			Update("current_team_id", nil).Error; err != nil {
			return err
		}

		// Delete the team
		return tx.Delete(&models.Team{}, "id = ?", id).Error
	})
}

// GetUserTeams gets all teams for a user (owned and member of)
func (r *TeamRepository) GetUserTeams(ctx context.Context, userID string) ([]models.Team, error) {
	var teams []models.Team

	// Get teams where user is owner
	var ownedTeams []models.Team
	if err := r.db.WithContext(ctx).
		Where("owner_id = ?", userID).
		Find(&ownedTeams).Error; err != nil {
		return nil, err
	}

	// Get teams where user is a member
	var memberTeams []models.Team
	if err := r.db.WithContext(ctx).
		Joins("JOIN team_members ON team_members.team_id = teams.id").
		Where("team_members.user_id = ?", userID).
		Find(&memberTeams).Error; err != nil {
		return nil, err
	}

	// Combine and deduplicate
	teamMap := make(map[string]models.Team)
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

// GetMembers gets all members of a team
func (r *TeamRepository) GetMembers(ctx context.Context, teamID string) ([]models.TeamMember, error) {
	var members []models.TeamMember
	err := r.db.WithContext(ctx).
		Preload("User").
		Where("team_id = ?", teamID).
		Find(&members).Error

	return members, err
}
