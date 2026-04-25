package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// TeamRepository handles team database operations
type TeamRepository struct {
	repository.Base[models.Team]
}

// NewTeamRepository creates a new TeamRepository instance
func NewTeamRepository(db *gorm.DB) *TeamRepository {
	return &TeamRepository{
		Base: repository.NewBase[models.Team](db),
	}
}

// FindByID finds a team by its ID with preloaded relations
func (r *TeamRepository) FindByID(ctx context.Context, id string) (*models.Team, error) {
	var team models.Team
	err := r.DB.WithContext(ctx).
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

// Delete deletes a team by its ID
func (r *TeamRepository) Delete(ctx context.Context, id string) error {
	return r.Transaction(ctx, func(tx *gorm.DB) error {
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
	if err := r.DB.WithContext(ctx).
		Where("user_id = ?", userID).
		Find(&ownedTeams).Error; err != nil {
		return nil, err
	}

	// Get teams where user is a member
	var memberTeams []models.Team
	if err := r.DB.WithContext(ctx).
		Joins("JOIN team_user ON team_user.team_id = teams.id").
		Where("team_user.user_id = ?", userID).
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
	err := r.DB.WithContext(ctx).
		Preload("User").
		Where("team_id = ?", teamID).
		Find(&members).Error

	return members, err
}
