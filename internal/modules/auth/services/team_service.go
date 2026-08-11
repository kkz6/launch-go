package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/auth/contracts"
	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
	launchcache "github.com/kkz6/launch-go/internal/pkg/launch/cache"
	"github.com/kkz6/launch-go/internal/pkg/mail/templates"
)

// TeamService handles team management operations
type TeamService struct {
	repos           contracts.RepositoryRegistry
	membershipCache *launchcache.TeamMembershipCache
	emailSender     channels.EmailSender
	logger          *zerolog.Logger
}

var buildTeamDeletedEmail = templates.TeamDeletedEmail

// NewTeamService creates a new TeamService instance
func NewTeamService(repos contracts.RepositoryRegistry, emailSender channels.EmailSender, logger *zerolog.Logger) *TeamService {
	return &TeamService{repos: repos, emailSender: emailSender, logger: logger}
}

func (s *TeamService) SetMembershipCache(c *launchcache.TeamMembershipCache) {
	s.membershipCache = c
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
func (s *TeamService) DeleteTeam(ctx context.Context, userID, teamID, transferToTeamID string) (*models.Team, error) {
	team, err := s.repos.Team().FindByID(ctx, teamID)
	if err != nil {
		return nil, err
	}

	if team == nil {
		return nil, fiberutil.NotFound()
	}

	if team.UserID != userID {
		return nil, fiberutil.Forbidden()
	}

	if team.PersonalTeam {
		return nil, errors.New("cannot delete personal team")
	}

	if transferToTeamID == teamID {
		return nil, fiberutil.Validation("resources must be transferred to a different team")
	}

	destination, err := s.repos.Team().FindByID(ctx, transferToTeamID)
	if err != nil {
		return nil, err
	}
	if destination == nil {
		return nil, fiberutil.Validation("transfer destination was not found")
	}
	if destination.UserID != userID {
		return nil, fiberutil.Forbidden("resources can only be transferred to a team you own")
	}

	members, err := s.repos.Team().GetMembers(ctx, teamID)
	if err != nil {
		return nil, err
	}
	owner, err := s.repos.User().FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if err := s.transferResourcesAndDelete(ctx, userID, teamID, transferToTeamID); err != nil {
		return nil, err
	}

	s.invalidateDeletedTeamMemberships(ctx, userID, teamID, members)
	uid := userID
	activity.RecordWithLogAndPropsPtr(ctx, "auth", "deleted", &uid, team, "Team was deleted and resources were transferred", map[string]any{
		"transfer_to_team_id": transferToTeamID,
		"transfer_to_team":    destination.Name,
	})
	s.sendTeamDeletedEmail(ctx, owner, team.Name, destination.Name)

	return destination, nil
}

func (s *TeamService) sendTeamDeletedEmail(ctx context.Context, owner *models.User, teamName, destinationName string) {
	if s.emailSender == nil || owner == nil || owner.Email == "" {
		return
	}

	html, _, err := buildTeamDeletedEmail(teamName, destinationName)
	if err == nil {
		err = s.emailSender.Send(ctx, owner.Email, fmt.Sprintf("Team deleted: %s", teamName), html, true)
	}
	if err != nil && s.logger != nil {
		s.logger.Error().Err(err).Str("email", owner.Email).Str("team", teamName).Msg("Failed to send team deletion email")
	}
}

var teamTransferExcludedTables = map[string]struct{}{
	"impersonation_sessions":   {},
	"notification_preferences": {},
	"team_invitations":         {},
	"team_user":                {},
}

func (s *TeamService) transferResourcesAndDelete(ctx context.Context, userID, sourceTeamID, destinationTeamID string) error {
	db := s.repos.DB().WithContext(ctx)
	tables, err := transferableTeamTables(db)
	if err != nil {
		return fmt.Errorf("failed to discover team resources: %w", err)
	}

	return db.Transaction(func(tx *gorm.DB) error {
		for _, table := range tables {
			if err := tx.Table(table).
				Where("team_id = ?", sourceTeamID).
				Update("team_id", destinationTeamID).Error; err != nil {
				return fmt.Errorf("failed to transfer resources from %s: %w", table, err)
			}
		}

		if err := tx.Model(&models.User{}).
			Where("current_team_id = ?", sourceTeamID).
			Update("current_team_id", nil).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.User{}).
			Where("id = ?", userID).
			Update("current_team_id", destinationTeamID).Error; err != nil {
			return err
		}
		if err := tx.Where("team_id = ?", sourceTeamID).Delete(&models.TeamMember{}).Error; err != nil {
			return err
		}
		if err := tx.Where("team_id = ?", sourceTeamID).Delete(&models.TeamInvitation{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.Team{}, "id = ?", sourceTeamID).Error
	})
}

func transferableTeamTables(db *gorm.DB) ([]string, error) {
	tables, err := db.Migrator().GetTables()
	if err != nil {
		return nil, err
	}
	return filterTransferableTeamTables(tables, func(table string) ([]gorm.ColumnType, error) {
		return db.Migrator().ColumnTypes(table)
	})
}

func filterTransferableTeamTables(tables []string, columnsFor func(string) ([]gorm.ColumnType, error)) ([]string, error) {
	transferable := make([]string, 0, len(tables))
	for _, table := range tables {
		if _, excluded := teamTransferExcludedTables[table]; excluded {
			continue
		}
		columns, err := columnsFor(table)
		if err != nil {
			return nil, err
		}
		for _, column := range columns {
			if column.Name() == "team_id" {
				transferable = append(transferable, table)
				break
			}
		}
	}
	return transferable, nil
}

func (s *TeamService) invalidateDeletedTeamMemberships(ctx context.Context, userID, teamID string, members []models.TeamMember) {
	if s.membershipCache == nil {
		return
	}
	_ = s.membershipCache.InvalidateMembership(ctx, userID, teamID)
	for _, member := range members {
		_ = s.membershipCache.InvalidateMembership(ctx, member.UserID, teamID)
	}
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
