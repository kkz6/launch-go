package repositories

import (
	"context"

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
	return repository.FindOneOrNil[models.Team](ctx, r.DB,
		repository.WithID(id),
		repository.PreloadMany("Owner", "Members"),
	)
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
	err := r.DB.WithContext(ctx).
		Distinct("teams.*").
		Joins("LEFT JOIN team_user ON team_user.team_id = teams.id").
		Where("teams.user_id = ? OR team_user.user_id = ?", userID, userID).
		Order("teams.personal_team DESC").
		Order("LOWER(teams.name) ASC").
		Order("teams.id ASC").
		Find(&teams).Error
	return teams, err
}

// GetMembers gets all members of a team
func (r *TeamRepository) GetMembers(ctx context.Context, teamID string) ([]models.TeamMember, error) {
	return repository.FindAll[models.TeamMember](ctx, r.DB,
		repository.WithTeamID(teamID),
		repository.Preload("User"),
	)
}
