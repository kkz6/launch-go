package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/auth/contracts"
	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/mail/templates"
	"github.com/kkz6/launch-go/internal/pkg/signedurl"
)

// TeamMemberService handles team member management operations
type TeamMemberService struct {
	repos       contracts.RepositoryRegistry
	config      *config.Config
	emailSender channels.EmailSender
	logger      *zerolog.Logger
}

// NewTeamMemberService creates a new TeamMemberService instance
func NewTeamMemberService(repos contracts.RepositoryRegistry, cfg *config.Config, emailSender channels.EmailSender, logger *zerolog.Logger) *TeamMemberService {
	return &TeamMemberService{repos: repos, config: cfg, emailSender: emailSender, logger: logger}
}

// canManageMembers checks if a team member has permission to manage other members
func (s *TeamMemberService) canManageMembers(member *models.TeamMember) bool {
	if member == nil || member.Role == nil {
		return false
	}
	return *member.Role == authtypes.TeamRoleOwner.String() || *member.Role == authtypes.TeamRoleAdmin.String()
}

// InviteTeamMember invites a user to a team
func (s *TeamMemberService) InviteTeamMember(ctx context.Context, userID, teamID string, req *dto.InviteTeamMemberRequest) error {
	req.Normalize()

	team, err := s.requireManageMembers(ctx, teamID, userID)
	if err != nil {
		return err
	}

	// Check if user is already a member
	existingUser, err := s.repos.User().FindByEmail(ctx, req.Email)
	if err != nil {
		return err
	}

	if existingUser != nil {
		isMember, err := s.repos.TeamMember().IsMember(ctx, teamID, existingUser.ID)
		if err != nil {
			return err
		}

		if isMember {
			return errors.New("user is already a team member")
		}
	}

	// Check if invitation already exists
	existingInvitation, err := s.repos.TeamInvitation().FindByEmail(ctx, teamID, req.Email)
	if err != nil {
		return err
	}

	if existingInvitation != nil {
		return errors.New("invitation already sent to this email")
	}

	// Create invitation
	invitation := &models.TeamInvitation{
		TeamID: teamID,
		Email:  req.Email,
		Role:   &req.Role,
	}

	if err := s.repos.TeamInvitation().Create(ctx, invitation); err != nil {
		return err
	}

	// Send invitation email
	s.sendInvitationEmail(ctx, invitation, team.Name, existingUser != nil)

	return nil
}

// AcceptTeamInvitation accepts a team invitation
func (s *TeamMemberService) AcceptTeamInvitation(ctx context.Context, userID, invitationID string) error {
	invitation, err := s.repos.TeamInvitation().FindByID(ctx, invitationID)
	if err != nil {
		return err
	}

	if invitation == nil {
		return fiberutil.NotFound()
	}

	user, err := s.repos.User().FindByID(ctx, userID)
	if err != nil {
		return err
	}

	if user == nil {
		return fiberutil.NotFound()
	}

	// Verify invitation is for this user
	if !strings.EqualFold(invitation.Email, user.Email) {
		return fiberutil.Forbidden()
	}

	// Add user to team
	role := "member"
	if invitation.Role != nil {
		role = *invitation.Role
	}
	if err := s.repos.TeamMember().AddUser(ctx, invitation.TeamID, userID, role); err != nil {
		return err
	}

	// Delete invitation
	return s.repos.TeamInvitation().Delete(ctx, invitationID)
}

// ResendTeamInvitation resends the invitation email for a pending invitation
func (s *TeamMemberService) ResendTeamInvitation(ctx context.Context, userID, teamID, invitationID string) error {
	team, err := s.requireManageMembers(ctx, teamID, userID)
	if err != nil {
		return err
	}

	invitation, err := s.repos.TeamInvitation().FindByID(ctx, invitationID)
	if err != nil {
		return err
	}

	if invitation == nil || invitation.TeamID != teamID {
		return fiberutil.NotFound()
	}

	// Check if the invitee already has an account
	existingUser, err := s.repos.User().FindByEmail(ctx, invitation.Email)
	if err != nil {
		return err
	}

	s.sendInvitationEmail(ctx, invitation, team.Name, existingUser != nil)

	return nil
}

// CancelTeamInvitation cancels a team invitation
func (s *TeamMemberService) CancelTeamInvitation(ctx context.Context, userID, teamID, invitationID string) error {
	if _, err := s.requireManageMembers(ctx, teamID, userID); err != nil {
		return err
	}

	invitation, err := s.repos.TeamInvitation().FindByID(ctx, invitationID)
	if err != nil {
		return err
	}

	if invitation == nil || invitation.TeamID != teamID {
		return fiberutil.NotFound()
	}

	return s.repos.TeamInvitation().Delete(ctx, invitationID)
}

// UpdateTeamMemberRole updates a team member's role
func (s *TeamMemberService) UpdateTeamMemberRole(ctx context.Context, userID, teamID, memberID string, req *dto.UpdateTeamMemberRequest) error {
	team, err := s.getTeam(ctx, teamID)
	if err != nil {
		return err
	}

	// Only owner can update roles
	if team.UserID != userID {
		return fiberutil.Forbidden()
	}

	// Cannot update owner's role
	if memberID == team.UserID {
		return errors.New("cannot update owner's role")
	}

	return s.repos.TeamMember().UpdateRole(ctx, teamID, memberID, req.Role)
}

// RemoveTeamMember removes a member from a team
func (s *TeamMemberService) RemoveTeamMember(ctx context.Context, userID, teamID, memberID string) error {
	team, err := s.getTeam(ctx, teamID)
	if err != nil {
		return err
	}

	// Check permission (owner or self-removal)
	if team.UserID != userID && userID != memberID {
		return fiberutil.Forbidden()
	}

	// Cannot remove owner
	if memberID == team.UserID {
		return errors.New("cannot remove team owner")
	}

	// Remove from team
	if err := s.repos.TeamMember().RemoveUser(ctx, teamID, memberID); err != nil {
		return err
	}

	// If this was their current team, switch to another.
	// Errors here are logged but not returned since the member removal already succeeded.
	member, err := s.repos.User().FindByID(ctx, memberID)
	if err != nil {
		s.logger.Error().Err(err).
			Str("user_id", memberID).
			Msg("Failed to find removed member for current team cleanup")
		return nil
	}

	if member != nil && member.CurrentTeamID != nil && *member.CurrentTeamID == teamID {
		teams, err := s.repos.Team().GetUserTeams(ctx, memberID)
		if err != nil {
			s.logger.Error().Err(err).
				Str("user_id", memberID).
				Msg("Failed to get user teams for current team cleanup")
			return nil
		}

		if len(teams) > 0 {
			if err := s.repos.User().SetCurrentTeam(ctx, memberID, teams[0].ID); err != nil {
				s.logger.Error().Err(err).
					Str("user_id", memberID).
					Str("team_id", teams[0].ID).
					Msg("Failed to switch user to new team after removal")
			}
		}
	}

	return nil
}

// GetTeamMembers gets all members of a team (excluding owner)
func (s *TeamMemberService) GetTeamMembers(ctx context.Context, teamID string) ([]models.TeamMember, error) {
	return s.repos.Team().GetMembers(ctx, teamID)
}

// GetAllTeamMembers gets all members of a team including the owner
// This matches Laravel's allUsers() behavior
func (s *TeamMemberService) GetAllTeamMembers(ctx context.Context, teamID string) ([]dto.TeamMemberResponse, error) {
	// Get team with owner
	team, err := s.getTeam(ctx, teamID)
	if err != nil {
		return nil, err
	}

	// Get members from pivot table
	members, err := s.repos.Team().GetMembers(ctx, teamID)
	if err != nil {
		return nil, err
	}

	// Build response with owner first, then members
	var allMembers []dto.TeamMemberResponse

	// Add owner with "owner" role
	if team.Owner != nil {
		ownerJoinedAt := team.CreatedAt
		if ownerJoinedAt == nil {
			now := time.Now()
			ownerJoinedAt = &now
		}
		allMembers = append(allMembers, dto.ToTeamMemberResponse(team.Owner, "owner", *ownerJoinedAt))
	}

	// Add other members from pivot table (skip owner since already added)
	for _, member := range members {
		if member.User != nil && member.UserID != team.UserID {
			role := ""
			if member.Role != nil {
				role = *member.Role
			}

			joinedAt := time.Time{}
			if member.CreatedAt != nil {
				joinedAt = *member.CreatedAt
			}

			allMembers = append(allMembers, dto.ToTeamMemberResponse(member.User, role, joinedAt))
		}
	}

	return allMembers, nil
}

// GetTeamInvitations gets all invitations for a team
func (s *TeamMemberService) GetTeamInvitations(ctx context.Context, teamID string) ([]models.TeamInvitation, error) {
	return s.repos.TeamInvitation().GetByTeam(ctx, teamID)
}

// GetInvitationByID retrieves an invitation by its ID (for public access)
func (s *TeamMemberService) GetInvitationByID(ctx context.Context, invitationID string) (*models.TeamInvitation, error) {
	invitation, err := s.repos.TeamInvitation().FindByID(ctx, invitationID)
	if err != nil {
		return nil, err
	}

	if invitation == nil {
		return nil, fiberutil.NotFound()
	}

	return invitation, nil
}

// InvitationUserExists reports whether the invited email already has an
// account, so the accept page can show "log in to join" (existing user)
// vs "create your account" (new user) — #71.
func (s *TeamMemberService) InvitationUserExists(ctx context.Context, email string) bool {
	user, err := s.repos.User().FindByEmail(ctx, email)
	if err != nil {
		return false
	}
	return user != nil
}

// GenerateInvitationURL generates a permanent signed URL for accepting a team invitation
// Team invitations don't expire - they remain valid until cancelled
func (s *TeamMemberService) GenerateInvitationURL(invitationID string) string {
	path := fmt.Sprintf("/auth/team-invitations/%s/accept", invitationID)
	return signedurl.PermanentSign(path, nil)
}

// sendInvitationEmail sends an invitation email to the invitee
func (s *TeamMemberService) sendInvitationEmail(ctx context.Context, invitation *models.TeamInvitation, teamName string, hasAccount bool) {
	if s.emailSender == nil {
		s.logger.Warn().Str("email", invitation.Email).Msg("Skipping invitation email: email sender not configured (check MAIL_DRIVER/RESEND_API_KEY/SMTP_HOST)")
		return
	}

	// User-facing links: send users to the frontend, not the API.
	frontend := s.config.App.Frontend()
	registerURL := fmt.Sprintf("%s/invite/%s", frontend, invitation.ID)

	acceptURL := s.GenerateInvitationURL(invitation.ID)
	if frontend != "" {
		acceptURL = fmt.Sprintf("%s%s", frontend, acceptURL)
	}

	htmlContent, _, err := templates.TeamInvitationEmail(teamName, acceptURL, registerURL, !hasAccount)
	if err != nil {
		s.logger.Error().Err(err).Str("email", invitation.Email).Msg("Failed to build invitation email template")
		return
	}

	if err := s.emailSender.Send(ctx, invitation.Email, fmt.Sprintf("You've been invited to join %s", teamName), htmlContent, true); err != nil {
		s.logger.Error().Err(err).Str("email", invitation.Email).Msg("Failed to send invitation email")
	}
}

func (s *TeamMemberService) getTeam(ctx context.Context, teamID string) (*models.Team, error) {
	team, err := s.repos.Team().FindByID(ctx, teamID)
	if err != nil {
		return nil, err
	}

	if team == nil {
		return nil, fiberutil.NotFound()
	}

	return team, nil
}

func (s *TeamMemberService) requireManageMembers(ctx context.Context, teamID, userID string) (*models.Team, error) {
	team, err := s.getTeam(ctx, teamID)
	if err != nil {
		return nil, err
	}

	if team.UserID == userID {
		return team, nil
	}

	member, err := s.repos.TeamMember().Get(ctx, teamID, userID)
	if err != nil {
		return nil, err
	}

	if !s.canManageMembers(member) {
		return nil, fiberutil.Forbidden()
	}

	return team, nil
}
