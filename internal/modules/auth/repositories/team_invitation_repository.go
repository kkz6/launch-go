package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

func (r *TeamInvitationRepository) Create(ctx context.Context, invitation *models.TeamInvitation) error {
	return withLockedTeam(ctx, r.DB, invitation.TeamID, func(tx *gorm.DB) error { return tx.Create(invitation).Error })
}

func (r *TeamInvitationRepository) Delete(ctx context.Context, id string) error {
	var invitation models.TeamInvitation
	if err := r.DB.WithContext(ctx).First(&invitation, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	return withLockedTeam(ctx, r.DB, invitation.TeamID, func(tx *gorm.DB) error {
		return tx.Where("id = ? AND team_id = ?", id, invitation.TeamID).Delete(&models.TeamInvitation{}).Error
	})
}

// TeamInvitationRepository handles team invitation database operations
type TeamInvitationRepository struct {
	repository.Base[models.TeamInvitation]
}

// NewTeamInvitationRepository creates a new TeamInvitationRepository instance
func NewTeamInvitationRepository(db *gorm.DB) *TeamInvitationRepository {
	return &TeamInvitationRepository{
		Base: repository.NewBase[models.TeamInvitation](db),
	}
}

// FindByID finds a team invitation by its ID with preloaded Team
func (r *TeamInvitationRepository) FindByID(ctx context.Context, id string) (*models.TeamInvitation, error) {
	return repository.FindOneOrNil[models.TeamInvitation](ctx, r.DB,
		repository.WithID(id),
		repository.Preload("Team"),
	)
}

// FindByEmail finds a team invitation by team ID and email
func (r *TeamInvitationRepository) FindByEmail(ctx context.Context, teamID, email string) (*models.TeamInvitation, error) {
	return repository.FindOneOrNil[models.TeamInvitation](ctx, r.DB,
		repository.WithTeamID(teamID),
		repository.WithEmail(email),
		repository.Preload("Team"),
	)
}

// GetByTeam gets all invitations for a team
func (r *TeamInvitationRepository) GetByTeam(ctx context.Context, teamID string) ([]models.TeamInvitation, error) {
	return repository.FindAll[models.TeamInvitation](ctx, r.DB,
		repository.WithTeamID(teamID),
		repository.Preload("Team"),
	)
}
