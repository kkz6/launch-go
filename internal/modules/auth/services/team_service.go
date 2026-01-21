package services

import (
	"context"
	"errors"

	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/enums"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/auth/repositories"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
	apperrors "github.com/kkz6/launch-go/internal/pkg/errors"
)

// TeamService handles team management operations
type TeamService struct {
	repos *repositories.Registry
}

// NewTeamService creates a new TeamService instance
func NewTeamService(repos *repositories.Registry) *TeamService {
	return &TeamService{repos: repos}
}

// CreateTeam creates a new team
func (s *TeamService) CreateTeam(ctx context.Context, userID string, req *dto.CreateTeamRequest) (*models.Team, error) {
	team := &models.Team{
		Name:         req.Name,
		UserID:       userID,
		PersonalTeam: req.PersonalTeam,
	}

	if err := s.repos.Team().Create(ctx, team); err != nil {
		return nil, err
	}

	// Add owner as team member
	if err := s.repos.TeamMember().AddUser(ctx, team.ID, userID, enums.TeamRoleOwner.String()); err != nil {
		return nil, err
	}

	activity.LogWithLog(ctx, s.repos.DB(), "auth", "created", userID, team, "Team was created")

	return team, nil
}

// UpdateTeam updates a team
func (s *TeamService) UpdateTeam(ctx context.Context, userID, teamID string, req *dto.UpdateTeamRequest) (*models.Team, error) {
	team, err := s.repos.Team().FindByID(ctx, teamID)
	if err != nil {
		return nil, err
	}

	if team == nil {
		return nil, apperrors.ErrNotFound
	}

	// Check ownership
	if team.UserID != userID {
		return nil, apperrors.ErrForbidden
	}

	team.Name = req.Name

	if err := s.repos.Team().Update(ctx, team); err != nil {
		return nil, err
	}

	activity.LogWithLog(ctx, s.repos.DB(), "auth", "updated", userID, team, "Team was updated")

	return team, nil
}

// DeleteTeam deletes a team
func (s *TeamService) DeleteTeam(ctx context.Context, userID, teamID string) error {
	team, err := s.repos.Team().FindByID(ctx, teamID)
	if err != nil {
		return err
	}

	if team == nil {
		return apperrors.ErrNotFound
	}

	// Check ownership
	if team.UserID != userID {
		return apperrors.ErrForbidden
	}

	// Cannot delete personal team
	if team.PersonalTeam {
		return errors.New("cannot delete personal team")
	}

	activity.LogWithLog(ctx, s.repos.DB(), "auth", "deleted", userID, team, "Team was deleted")

	return s.repos.Team().Delete(ctx, teamID)
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
		return nil, nil, nil, apperrors.ErrNotFound
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
		return nil, apperrors.ErrForbidden
	}

	if err := s.repos.User().SetCurrentTeam(ctx, userID, teamID); err != nil {
		return nil, err
	}

	return s.repos.User().FindByID(ctx, userID)
}
