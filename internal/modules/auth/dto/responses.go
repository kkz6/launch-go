package dto

import (
	"time"

	"github.com/kkz6/launch-go/internal/modules/auth/models"
	pkgdto "github.com/kkz6/launch-go/internal/pkg/dto"
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
	UserID       string `json:"user_id"`
	PersonalTeam bool   `json:"personal_team"`
	ImageURL     string `json:"image_url"`
	IsSubscribed bool   `json:"is_subscribed"`
	IsOwner      bool   `json:"is_owner"`
	CreatedAt    string `json:"created_at"`
}

// TeamDetailResponse represents a detailed team response with members and permissions
type TeamDetailResponse struct {
	TeamResponse
	Owner          *UserResponse            `json:"owner,omitempty"`
	Members        []TeamMemberResponse     `json:"members,omitempty"`
	Invitations    []TeamInvitationResponse `json:"invitations,omitempty"`
	AvailableRoles []RoleOption             `json:"available_roles"`
	Permissions    TeamPermissions          `json:"permissions"`
}

// RoleOption represents an available role option
type RoleOption struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// TeamPermissions represents the user's permissions on a team
type TeamPermissions struct {
	CanUpdateTeam        bool `json:"can_update_team"`
	CanDeleteTeam        bool `json:"can_delete_team"`
	CanAddTeamMembers    bool `json:"can_add_team_members"`
	CanUpdateTeamMembers bool `json:"can_update_team_members"`
	CanRemoveTeamMembers bool `json:"can_remove_team_members"`
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
	HasPasskeys          bool `json:"has_passkeys"`
	PasskeyCount         int  `json:"passkey_count"`
}

// PasskeyResponse represents a passkey in API responses
type PasskeyResponse struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	CreatedAt  string   `json:"created_at"`
	LastUsedAt *string  `json:"last_used_at,omitempty"`
	Transports []string `json:"transports,omitempty"`
}

// PasswordResetResponse represents the password reset response
type PasswordResetResponse struct {
	Message string `json:"message"`
}

// ToUserResponse converts a User model to a UserResponse DTO
func ToUserResponse(user *models.User) UserResponse {
	return ToUserResponseWithStatus(user, false, user.Onboarded)
}

// ToUserResponseWithSubscription converts a User model to a UserResponse DTO with subscription status
func ToUserResponseWithSubscription(user *models.User, isSubscribed bool) UserResponse {
	return ToUserResponseWithStatus(user, isSubscribed, user.Onboarded)
}

// ToUserResponseWithStatus converts a User model to a UserResponse DTO with explicit status flags
func ToUserResponseWithStatus(user *models.User, isSubscribed bool, onboarded bool) UserResponse {
	timezone := ""
	if user.Timezone != nil {
		timezone = *user.Timezone
	}

	resp := UserResponse{
		ID:               user.ID,
		Name:             user.Name,
		Email:            user.Email,
		ProfilePhotoURL:  user.ProfilePhotoURL(),
		CurrentTeamID:    user.CurrentTeamID,
		Timezone:         timezone,
		Onboarded:        onboarded,
		TwoFactorEnabled: user.TwoFactorEnabled(),
		CreatedAt:        pkgdto.FormatTimeOrEmpty(user.CreatedAt),
		EmailVerifiedAt:  pkgdto.FormatTime(user.EmailVerifiedAt),
	}

	if user.CurrentTeam != nil {
		resp.CurrentTeam = ToTeamResponsePtrWithSubscription(user.CurrentTeam, isSubscribed)
	}

	return resp
}

// ToTeamResponse converts a Team model to a TeamResponse DTO
func ToTeamResponse(team models.Team) TeamResponse {
	return ToTeamResponseForUser(team, "")
}

// ToTeamResponseForUser converts a Team model to a TeamResponse DTO with ownership info
func ToTeamResponseForUser(team models.Team, userID string) TeamResponse {
	return TeamResponse{
		ID:           team.ID,
		Name:         team.Name,
		UserID:       team.UserID,
		PersonalTeam: team.PersonalTeam,
		ImageURL:     team.ImageURL(),
		IsSubscribed: false,
		IsOwner:      userID != "" && team.UserID == userID,
		CreatedAt:    pkgdto.FormatTimeOrEmpty(team.CreatedAt),
	}
}

// ToTeamResponseWithSubscription converts a Team model to a TeamResponse DTO with subscription status
func ToTeamResponseWithSubscription(team models.Team, isSubscribed bool) TeamResponse {
	resp := ToTeamResponse(team)
	resp.IsSubscribed = isSubscribed
	return resp
}

// ToTeamResponsePtr converts a Team model pointer to a TeamResponse DTO pointer
func ToTeamResponsePtr(team *models.Team) *TeamResponse {
	if team == nil {
		return nil
	}

	resp := ToTeamResponse(*team)

	return &resp
}

// ToTeamResponsePtrWithSubscription converts a Team model pointer to a TeamResponse DTO pointer with subscription status
func ToTeamResponsePtrWithSubscription(team *models.Team, isSubscribed bool) *TeamResponse {
	if team == nil {
		return nil
	}

	resp := ToTeamResponseWithSubscription(*team, isSubscribed)

	return &resp
}

// ToTeamMemberResponse converts a User model with team membership to a TeamMemberResponse
func ToTeamMemberResponse(user *models.User, role string, joinedAt time.Time) TeamMemberResponse {
	return TeamMemberResponse{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		Role:      role,
		JoinedAt:  pkgdto.FormatTimeValue(joinedAt),
		AvatarURL: user.ProfilePhotoURL(),
	}
}

// ToTeamInvitationResponse converts a TeamInvitation model to a TeamInvitationResponse DTO
func ToTeamInvitationResponse(invitation *models.TeamInvitation) TeamInvitationResponse {
	role := ""
	if invitation.Role != nil {
		role = *invitation.Role
	}

	resp := TeamInvitationResponse{
		ID:        invitation.ID,
		Email:     invitation.Email,
		Role:      role,
		CreatedAt: pkgdto.FormatTimeOrEmpty(invitation.CreatedAt),
	}

	if invitation.Team != nil {
		resp.Team = ToTeamResponse(*invitation.Team)
	}

	return resp
}

// ToTeamsResponse converts a slice of Team models to TeamResponse DTOs
func ToTeamsResponse(teams []models.Team) []TeamResponse {
	return ToTeamsResponseForUser(teams, "")
}

// ToTeamsResponseForUser converts a slice of Team models to TeamResponse DTOs with ownership info
func ToTeamsResponseForUser(teams []models.Team, userID string) []TeamResponse {
	responses := make([]TeamResponse, len(teams))
	for i, team := range teams {
		responses[i] = ToTeamResponseForUser(team, userID)
	}

	return responses
}

// ToTeamMembersResponse converts a slice of TeamMember models to TeamMemberResponse DTOs
func ToTeamMembersResponse(members []models.TeamMember) []TeamMemberResponse {
	responses := make([]TeamMemberResponse, len(members))
	for i, member := range members {
		if member.User != nil {
			role := ""
			if member.Role != nil {
				role = *member.Role
			}

			joinedAt := time.Time{}
			if member.CreatedAt != nil {
				joinedAt = *member.CreatedAt
			}

			responses[i] = ToTeamMemberResponse(member.User, role, joinedAt)
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

// GetAvailableRoles returns the available roles for team members
func GetAvailableRoles() []RoleOption {
	return []RoleOption{
		{Key: "admin", Name: "Administrator", Description: "Can manage members and settings"},
		{Key: "editor", Name: "Editor", Description: "Can create and edit resources"},
		{Key: "member", Name: "Member", Description: "Can view resources"},
	}
}

// ToTeamDetailResponse converts a Team model to a detailed TeamDetailResponse DTO
func ToTeamDetailResponse(team *models.Team, members []models.TeamMember, invitations []models.TeamInvitation, userID string) TeamDetailResponse {
	isOwner := team.UserID == userID
	isAdmin := isOwner

	if !isOwner {
		for _, member := range members {
			if member.UserID == userID && member.Role != nil && *member.Role == "admin" {
				isAdmin = true

				break
			}
		}
	}

	resp := TeamDetailResponse{
		TeamResponse:   ToTeamResponse(*team),
		AvailableRoles: GetAvailableRoles(),
		Permissions: TeamPermissions{
			CanUpdateTeam:        isOwner,
			CanDeleteTeam:        isOwner && !team.PersonalTeam,
			CanAddTeamMembers:    isOwner || isAdmin,
			CanUpdateTeamMembers: isOwner,
			CanRemoveTeamMembers: isOwner || isAdmin,
		},
	}

	if team.Owner != nil {
		ownerResp := ToUserResponse(team.Owner)
		resp.Owner = &ownerResp
	}

	resp.Members = ToTeamMembersResponse(members)
	resp.Invitations = ToTeamInvitationsResponse(invitations)

	return resp
}
