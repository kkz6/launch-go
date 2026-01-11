package dto

import (
	"time"

	"github.com/kkz6/launch-go/internal/modules/auth/models"
)

// AuthResponse represents the authentication response
type AuthResponse struct {
	User         UserResponse `json:"user"`
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token,omitempty"`
	ExpiresIn    int          `json:"expires_in"`
	TokenType    string       `json:"token_type"`
}

// UserResponse represents the user data in responses
type UserResponse struct {
	ID               string        `json:"id"`
	Name             string        `json:"name"`
	Email            string        `json:"email"`
	EmailVerifiedAt  *string       `json:"email_verified_at,omitempty"`
	ProfilePhotoURL  string        `json:"profile_photo_url"`
	CurrentTeamID    *string       `json:"current_team_id,omitempty"`
	CurrentTeam      *TeamResponse `json:"current_team,omitempty"`
	Timezone         string        `json:"timezone"`
	Onboarded        bool          `json:"onboarded"`
	TwoFactorEnabled bool          `json:"two_factor_enabled"`
	CreatedAt        string        `json:"created_at"`
}

// TeamResponse represents the team data in responses
type TeamResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	OwnerID      string `json:"owner_id"`
	PersonalTeam bool   `json:"personal_team"`
	ImageURL     string `json:"image_url"`
	CreatedAt    string `json:"created_at"`
}

// TeamMemberResponse represents a team member in responses
type TeamMemberResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	JoinedAt  string `json:"joined_at"`
	AvatarURL string `json:"avatar_url"`
}

// TeamInvitationResponse represents a team invitation in responses
type TeamInvitationResponse struct {
	ID        string       `json:"id"`
	Email     string       `json:"email"`
	Role      string       `json:"role"`
	Team      TeamResponse `json:"team"`
	CreatedAt string       `json:"created_at"`
}

// TwoFactorResponse represents the 2FA setup response
type TwoFactorResponse struct {
	QRCodeURL     string   `json:"qr_code_url"`
	SecretKey     string   `json:"secret_key"`
	RecoveryCodes []string `json:"recovery_codes,omitempty"`
}

// UserStatusResponse represents the user status check response
type UserStatusResponse struct {
	UserExists           bool `json:"user_exists"`
	RequiresVerification bool `json:"requires_verification"`
	HasTwoFactor         bool `json:"has_two_factor"`
}

// PasswordResetResponse represents the password reset response
type PasswordResetResponse struct {
	Message string `json:"message"`
}

// ToUserResponse converts a User model to a UserResponse DTO
func ToUserResponse(user *models.User) UserResponse {
	resp := UserResponse{
		ID:               user.ID,
		Name:             user.Name,
		Email:            user.Email,
		ProfilePhotoURL:  user.ProfilePhotoURL(),
		CurrentTeamID:    user.CurrentTeamID,
		Timezone:         user.Timezone,
		Onboarded:        user.Onboarded,
		TwoFactorEnabled: user.TwoFactorEnabled(),
		CreatedAt:        user.CreatedAt.Format(time.RFC3339),
	}

	if user.EmailVerifiedAt != nil {
		verified := user.EmailVerifiedAt.Format(time.RFC3339)
		resp.EmailVerifiedAt = &verified
	}

	if user.CurrentTeam != nil {
		resp.CurrentTeam = ToTeamResponsePtr(user.CurrentTeam)
	}

	return resp
}

// ToTeamResponse converts a Team model to a TeamResponse DTO
func ToTeamResponse(team models.Team) TeamResponse {
	return TeamResponse{
		ID:           team.ID,
		Name:         team.Name,
		OwnerID:      team.OwnerID,
		PersonalTeam: team.PersonalTeam,
		ImageURL:     team.ImageURL(),
		CreatedAt:    team.CreatedAt.Format(time.RFC3339),
	}
}

// ToTeamResponsePtr converts a Team model pointer to a TeamResponse DTO pointer
func ToTeamResponsePtr(team *models.Team) *TeamResponse {
	if team == nil {
		return nil
	}

	resp := ToTeamResponse(*team)

	return &resp
}

// ToTeamMemberResponse converts a User model with team membership to a TeamMemberResponse
func ToTeamMemberResponse(user *models.User, role string, joinedAt time.Time) TeamMemberResponse {
	return TeamMemberResponse{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		Role:      role,
		JoinedAt:  joinedAt.Format(time.RFC3339),
		AvatarURL: user.ProfilePhotoURL(),
	}
}

// ToTeamInvitationResponse converts a TeamInvitation model to a TeamInvitationResponse DTO
func ToTeamInvitationResponse(invitation *models.TeamInvitation) TeamInvitationResponse {
	resp := TeamInvitationResponse{
		ID:        invitation.ID,
		Email:     invitation.Email,
		Role:      invitation.Role,
		CreatedAt: invitation.CreatedAt.Format(time.RFC3339),
	}

	if invitation.Team != nil {
		resp.Team = ToTeamResponse(*invitation.Team)
	}

	return resp
}

// ToTeamsResponse converts a slice of Team models to TeamResponse DTOs
func ToTeamsResponse(teams []models.Team) []TeamResponse {
	responses := make([]TeamResponse, len(teams))
	for i, team := range teams {
		responses[i] = ToTeamResponse(team)
	}

	return responses
}

// ToTeamMembersResponse converts a slice of TeamMember models to TeamMemberResponse DTOs
func ToTeamMembersResponse(members []models.TeamMember) []TeamMemberResponse {
	responses := make([]TeamMemberResponse, len(members))
	for i, member := range members {
		if member.User != nil {
			responses[i] = ToTeamMemberResponse(member.User, member.Role, member.CreatedAt)
		}
	}

	return responses
}

// ToTeamInvitationsResponse converts a slice of TeamInvitation models to TeamInvitationResponse DTOs
func ToTeamInvitationsResponse(invitations []models.TeamInvitation) []TeamInvitationResponse {
	responses := make([]TeamInvitationResponse, len(invitations))
	for i := range invitations {
		responses[i] = ToTeamInvitationResponse(&invitations[i])
	}

	return responses
}
