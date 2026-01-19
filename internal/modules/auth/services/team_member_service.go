package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/enums"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/auth/repositories"
	apperrors "github.com/kkz6/launch-go/internal/pkg/errors"
	"github.com/kkz6/launch-go/internal/pkg/signedurl"
)

// TeamMemberService handles team member management operations
type TeamMemberService struct {
	repos *repositories.Registry
}

// NewTeamMemberService creates a new TeamMemberService instance
func NewTeamMemberService(repos *repositories.Registry) *TeamMemberService {
	return &TeamMemberService{repos: repos}
}

// InviteTeamMember invites a user to a team
func (s *TeamMemberService) InviteTeamMember(ctx context.Context, userID, teamID string, req *dto.InviteTeamMemberRequest) error {
	req.Normalize()

	team, err := s.repos.Team().FindByID(ctx, teamID)
	if err != nil {
		return err
	}

	if team == nil {
		return apperrors.ErrNotFound
	}

	// Check if user has permission to invite
	if team.UserID != userID {
		member, err := s.repos.TeamMember().Get(ctx, teamID, userID)
		if err != nil {
			return err
		}

		if member == nil || member.Role == nil || *member.Role != enums.TeamRoleAdmin.String() {
			return apperrors.ErrForbidden
		}
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

	return s.repos.TeamInvitation().Create(ctx, invitation)
}

// AcceptTeamInvitation accepts a team invitation
func (s *TeamMemberService) AcceptTeamInvitation(ctx context.Context, userID, invitationID string) error {
	invitation, err := s.repos.TeamInvitation().FindByID(ctx, invitationID)
	if err != nil {
		return err
	}

	if invitation == nil {
		return apperrors.ErrNotFound
	}

	user, err := s.repos.User().FindByID(ctx, userID)
	if err != nil {
		return err
	}

	if user == nil {
		return apperrors.ErrNotFound
	}

	// Verify invitation is for this user
	if !strings.EqualFold(invitation.Email, user.Email) {
		return apperrors.ErrForbidden
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

// CancelTeamInvitation cancels a team invitation
func (s *TeamMemberService) CancelTeamInvitation(ctx context.Context, userID, teamID, invitationID string) error {
	team, err := s.repos.Team().FindByID(ctx, teamID)
	if err != nil {
		return err
	}

	if team == nil {
		return apperrors.ErrNotFound
	}

	// Check permission
	if team.UserID != userID {
		member, err := s.repos.TeamMember().Get(ctx, teamID, userID)
		if err != nil {
			return err
		}

		if member == nil || member.Role == nil || *member.Role != enums.TeamRoleAdmin.String() {
			return apperrors.ErrForbidden
		}
	}

	invitation, err := s.repos.TeamInvitation().FindByID(ctx, invitationID)
	if err != nil {
		return err
	}

	if invitation == nil || invitation.TeamID != teamID {
		return apperrors.ErrNotFound
	}

	return s.repos.TeamInvitation().Delete(ctx, invitationID)
}

// UpdateTeamMemberRole updates a team member's role
func (s *TeamMemberService) UpdateTeamMemberRole(ctx context.Context, userID, teamID, memberID string, req *dto.UpdateTeamMemberRequest) error {
	team, err := s.repos.Team().FindByID(ctx, teamID)
	if err != nil {
		return err
	}

	if team == nil {
		return apperrors.ErrNotFound
	}

	// Only owner can update roles
	if team.UserID != userID {
		return apperrors.ErrForbidden
	}

	// Cannot update owner's role
	if memberID == team.UserID {
		return errors.New("cannot update owner's role")
	}

	return s.repos.TeamMember().UpdateRole(ctx, teamID, memberID, req.Role)
}

// RemoveTeamMember removes a member from a team
func (s *TeamMemberService) RemoveTeamMember(ctx context.Context, userID, teamID, memberID string) error {
	team, err := s.repos.Team().FindByID(ctx, teamID)
	if err != nil {
		return err
	}

	if team == nil {
		return apperrors.ErrNotFound
	}

	// Check permission (owner or self-removal)
	if team.UserID != userID && userID != memberID {
		return apperrors.ErrForbidden
	}

	// Cannot remove owner
	if memberID == team.UserID {
		return errors.New("cannot remove team owner")
	}

	// Remove from team
	if err := s.repos.TeamMember().RemoveUser(ctx, teamID, memberID); err != nil {
		return err
	}

	// If this was their current team, switch to another
	member, err := s.repos.User().FindByID(ctx, memberID)
	if err != nil {
		return nil // Member removed successfully, ignore this error
	}

	if member != nil && member.CurrentTeamID != nil && *member.CurrentTeamID == teamID {
		teams, err := s.repos.Team().GetUserTeams(ctx, memberID)
		if err != nil {
			return nil
		}

		if len(teams) > 0 {
			s.repos.User().SetCurrentTeam(ctx, memberID, teams[0].ID)
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
	team, err := s.repos.Team().FindByID(ctx, teamID)
	if err != nil {
		return nil, err
	}
	if team == nil {
		return nil, apperrors.ErrNotFound
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

// GenerateInvitationURL generates a permanent signed URL for accepting a team invitation
// Team invitations don't expire - they remain valid until cancelled
func (s *TeamMemberService) GenerateInvitationURL(invitationID string) string {
	path := fmt.Sprintf("/api/auth/team-invitations/%s/accept", invitationID)
	return signedurl.PermanentSign(path, nil)
}
