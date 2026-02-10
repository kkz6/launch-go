package services

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/auth/contracts"
	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
)

// TeamService handles team management operations
type TeamService struct {
	repos contracts.RepositoryRegistry
}

// NewTeamService creates a new TeamService instance
func NewTeamService(repos contracts.RepositoryRegistry) *TeamService {
	return &TeamService{repos: repos}
}

// CreateTeam creates a new team
func (s *TeamService) CreateTeam(ctx context.Context, userID string, req *dto.CreateTeamRequest) (*models.Team, error) {
	team := &models.Team{
		Name:         req.Name,
		UserID:       userID,
		PersonalTeam: req.PersonalTeam,
	}

	err := s.repos.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(team).Error; err != nil {
			return err
		}

		// Add owner as team member
		ownerRole := authtypes.TeamRoleOwner.String()
		return tx.Create(&models.TeamMember{
			TeamID: team.ID,
			UserID: userID,
			Role:   &ownerRole,
		}).Error
	})
	if err != nil {
		return nil, err
	}

	activity.RecordWithLog(ctx, "auth", "created", userID, team, "Team was created")

	return team, nil
}

// UpdateTeam updates a team
func (s *TeamService) UpdateTeam(ctx context.Context, userID, teamID string, req *dto.UpdateTeamRequest) (*models.Team, error) {
	team, err := s.repos.Team().FindByID(ctx, teamID)
	if err != nil {
		return nil, err
	}

	if team == nil {
		return nil, fiberutil.NotFound()
	}

	// Check ownership
	if team.UserID != userID {
		return nil, fiberutil.Forbidden()
	}

	team.Name = req.Name

	if err := s.repos.Team().Update(ctx, team); err != nil {
		return nil, err
	}

	activity.RecordWithLog(ctx, "auth", "updated", userID, team, "Team was updated")

	return team, nil
}

// DeleteTeam deletes a team
func (s *TeamService) DeleteTeam(ctx context.Context, userID, teamID string) error {
	team, err := s.repos.Team().FindByID(ctx, teamID)
	if err != nil {
		return err
	}

	if team == nil {
		return fiberutil.NotFound()
	}

	// Check ownership
	if team.UserID != userID {
		return fiberutil.Forbidden()
	}

	// Cannot delete personal team
	if team.PersonalTeam {
		return errors.New("cannot delete personal team")
	}

	if err := s.repos.Team().Delete(ctx, teamID); err != nil {
		return err
	}

	activity.RecordWithLog(ctx, "auth", "deleted", userID, team, "Team was deleted")

	return nil
}

// GetTeam retrieves a team by ID
func (s *TeamService) GetTeam(ctx context.Context, teamID string) (*models.Team, error) {
	return s.repos.Team().FindByID(ctx, teamID)
}

// GetTeamWithDetails retrieves a team with its members and invitations
func (s *TeamService) GetTeamWithDetails(ctx context.Context, teamID string) (*models.Team, []models.TeamMember, []models.TeamInvitation, error) {
	team, err := s.repos.Team().FindByID(ctx, teamID)
	if err != nil {
		return nil, nil, nil, err
	}

	if team == nil {
		return nil, nil, nil, fiberutil.NotFound()
	}

	members, err := s.repos.Team().GetMembers(ctx, teamID)
	if err != nil {
		return nil, nil, nil, err
	}

	invitations, err := s.repos.TeamInvitation().GetByTeam(ctx, teamID)
	if err != nil {
		return nil, nil, nil, err
	}

	return team, members, invitations, nil
}

// GetUserTeams retrieves all teams for a user
func (s *TeamService) GetUserTeams(ctx context.Context, userID string) ([]models.Team, error) {
	return s.repos.Team().GetUserTeams(ctx, userID)
}

// SwitchTeam switches the user's current team
func (s *TeamService) SwitchTeam(ctx context.Context, userID, teamID string) (*models.User, error) {
	// Verify user is member of team
	isMember, err := s.repos.TeamMember().IsMember(ctx, teamID, userID)
	if err != nil {
		return nil, err
	}

	if !isMember {
		return nil, fiberutil.Forbidden()
	}

	if err := s.repos.User().SetCurrentTeam(ctx, userID, teamID); err != nil {
		return nil, err
	}

	return s.repos.User().FindByID(ctx, userID)
}
