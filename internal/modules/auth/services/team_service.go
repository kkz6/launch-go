package services

import (
	"context"
	"errors"

	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/enums"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/auth/repositories"
	apperrors "github.com/kkz6/launch-go/internal/pkg/errors"
)

// TeamService handles team management operations
type TeamService struct {
	repo *repositories.Repository
}

// NewTeamService creates a new TeamService instance
func NewTeamService(repo *repositories.Repository) *TeamService {
	return &TeamService{repo: repo}
}

// CreateTeam creates a new team
func (s *TeamService) CreateTeam(ctx context.Context, userID string, req *dto.CreateTeamRequest) (*models.Team, error) {
	team := &models.Team{
		Name:         req.Name,
		OwnerID:      userID,
		PersonalTeam: req.PersonalTeam,
	}

	if err := s.repo.CreateTeam(ctx, team); err != nil {
		return nil, err
	}

	// Add owner as team member
	if err := s.repo.AddUserToTeam(ctx, team.ID, userID, enums.TeamRoleOwner.String()); err != nil {
		return nil, err
	}

	return team, nil
}

// UpdateTeam updates a team
func (s *TeamService) UpdateTeam(ctx context.Context, userID, teamID string, req *dto.UpdateTeamRequest) (*models.Team, error) {
	team, err := s.repo.FindTeamByID(ctx, teamID)
	if err != nil {
		return nil, err
	}

	if team == nil {
		return nil, apperrors.ErrNotFound
	}

	// Check ownership
	if team.OwnerID != userID {
		return nil, apperrors.ErrForbidden
	}

	team.Name = req.Name

	if err := s.repo.UpdateTeam(ctx, team); err != nil {
		return nil, err
	}

	return team, nil
}

// DeleteTeam deletes a team
func (s *TeamService) DeleteTeam(ctx context.Context, userID, teamID string) error {
	team, err := s.repo.FindTeamByID(ctx, teamID)
	if err != nil {
		return err
	}

	if team == nil {
		return apperrors.ErrNotFound
	}

	// Check ownership
	if team.OwnerID != userID {
		return apperrors.ErrForbidden
	}

	// Cannot delete personal team
	if team.PersonalTeam {
		return errors.New("cannot delete personal team")
	}

	return s.repo.DeleteTeam(ctx, teamID)
}

// GetTeam retrieves a team by ID
func (s *TeamService) GetTeam(ctx context.Context, teamID string) (*models.Team, error) {
	return s.repo.FindTeamByID(ctx, teamID)
}

// GetUserTeams retrieves all teams for a user
func (s *TeamService) GetUserTeams(ctx context.Context, userID string) ([]models.Team, error) {
	return s.repo.GetUserTeams(ctx, userID)
}

// SwitchTeam switches the user's current team
func (s *TeamService) SwitchTeam(ctx context.Context, userID, teamID string) (*models.User, error) {
	// Verify user is member of team
	isMember, err := s.repo.IsTeamMember(ctx, teamID, userID)
	if err != nil {
		return nil, err
	}

	if !isMember {
		return nil, apperrors.ErrForbidden
	}

	if err := s.repo.SetCurrentTeam(ctx, userID, teamID); err != nil {
		return nil, err
	}

	return s.repo.FindUserByID(ctx, userID)
}
