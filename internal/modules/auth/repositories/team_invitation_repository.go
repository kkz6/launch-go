package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/auth/models"
)

// TeamInvitationRepository handles team invitation database operations
type TeamInvitationRepository struct {
	db *gorm.DB
}

// NewTeamInvitationRepository creates a new TeamInvitationRepository instance
func NewTeamInvitationRepository(db *gorm.DB) *TeamInvitationRepository {
	return &TeamInvitationRepository{db: db}
}

// Create creates a new team invitation
func (r *TeamInvitationRepository) Create(ctx context.Context, invitation *models.TeamInvitation) error {
	return r.db.WithContext(ctx).Create(invitation).Error
}

// FindByID finds a team invitation by its ID
func (r *TeamInvitationRepository) FindByID(ctx context.Context, id string) (*models.TeamInvitation, error) {
	var invitation models.TeamInvitation
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

// FindByEmail finds a team invitation by team ID and email
func (r *TeamInvitationRepository) FindByEmail(ctx context.Context, teamID, email string) (*models.TeamInvitation, error) {
	var invitation models.TeamInvitation
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

// GetByTeam gets all invitations for a team
func (r *TeamInvitationRepository) GetByTeam(ctx context.Context, teamID string) ([]models.TeamInvitation, error) {
	var invitations []models.TeamInvitation
	err := r.db.WithContext(ctx).
		Preload("Team").
		Where("team_id = ?", teamID).
		Find(&invitations).Error

	return invitations, err
}

// Delete deletes a team invitation
func (r *TeamInvitationRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&models.TeamInvitation{}, "id = ?", id).Error
}
