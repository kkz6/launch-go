package auth

import (
	"strings"
	"time"
)

// RegisterRequest represents a user registration request
type RegisterRequest struct {
	Name                 string  `json:"name" validate:"required,min=2,max=255"`
	Email                string  `json:"email" validate:"required,email"`
	Password             string  `json:"password" validate:"required,min=8"`
	PasswordConfirmation string  `json:"password_confirmation" validate:"required,eqfield=Password"`
	Timezone             string  `json:"timezone" validate:"omitempty,max=50"`
	InvitationID         *string `json:"invitation_id" validate:"omitempty"`
	CreatePersonalTeam   bool    `json:"create_personal_team"`
}

// Normalize normalizes the email to lowercase
func (r *RegisterRequest) Normalize() {
	r.Email = strings.ToLower(strings.TrimSpace(r.Email))
	r.Name = strings.TrimSpace(r.Name)
}

// LoginRequest represents a user login request
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
	Remember bool   `json:"remember"`
}

// Normalize normalizes the email to lowercase
func (r *LoginRequest) Normalize() {
	r.Email = strings.ToLower(strings.TrimSpace(r.Email))
}

// RefreshTokenRequest represents a token refresh request
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// UpdateProfileRequest represents a profile update request
type UpdateProfileRequest struct {
	Name     string `json:"name" validate:"required,min=2,max=255"`
	Email    string `json:"email" validate:"required,email"`
	Timezone string `json:"timezone" validate:"omitempty,max=50"`
}

// Normalize normalizes the email to lowercase
func (r *UpdateProfileRequest) Normalize() {
	r.Email = strings.ToLower(strings.TrimSpace(r.Email))
	r.Name = strings.TrimSpace(r.Name)
}

// ChangePasswordRequest represents a password change request
type ChangePasswordRequest struct {
	CurrentPassword      string `json:"current_password" validate:"required"`
	Password             string `json:"password" validate:"required,min=8"`
	PasswordConfirmation string `json:"password_confirmation" validate:"required,eqfield=Password"`
}

// ForgotPasswordRequest represents a password reset request initiation
type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// Normalize normalizes the email to lowercase
func (r *ForgotPasswordRequest) Normalize() {
	r.Email = strings.ToLower(strings.TrimSpace(r.Email))
}

// ResetPasswordRequest represents a password reset request
type ResetPasswordRequest struct {
	Email                string `json:"email" validate:"required,email"`
	Token                string `json:"token" validate:"required"`
	Password             string `json:"password" validate:"required,min=8"`
	PasswordConfirmation string `json:"password_confirmation" validate:"required,eqfield=Password"`
}

// Normalize normalizes the email to lowercase
func (r *ResetPasswordRequest) Normalize() {
	r.Email = strings.ToLower(strings.TrimSpace(r.Email))
}

// VerifyEmailRequest represents an email verification request
type VerifyEmailRequest struct {
	ID   string `json:"id" validate:"required"`
	Hash string `json:"hash" validate:"required"`
}

// TwoFactorChallengeRequest represents a 2FA challenge verification request
type TwoFactorChallengeRequest struct {
	Code         string `json:"code" validate:"required_without=RecoveryCode"`
	RecoveryCode string `json:"recovery_code" validate:"required_without=Code"`
}

// EnableTwoFactorRequest represents a request to enable 2FA
type EnableTwoFactorRequest struct {
	Password string `json:"password" validate:"required"`
}

// ConfirmTwoFactorRequest represents a request to confirm 2FA setup
type ConfirmTwoFactorRequest struct {
	Code string `json:"code" validate:"required,len=6"`
}

// CreateTeamRequest represents a team creation request
type CreateTeamRequest struct {
	Name         string `json:"name" validate:"required,min=2,max=255"`
	PersonalTeam bool   `json:"personal_team"`
}

// UpdateTeamRequest represents a team update request
type UpdateTeamRequest struct {
	Name string `json:"name" validate:"required,min=2,max=255"`
}

// InviteTeamMemberRequest represents a team member invitation request
type InviteTeamMemberRequest struct {
	Email string `json:"email" validate:"required,email"`
	Role  string `json:"role" validate:"required,oneof=admin member"`
}

// Normalize normalizes the email to lowercase
func (r *InviteTeamMemberRequest) Normalize() {
	r.Email = strings.ToLower(strings.TrimSpace(r.Email))
}

// UpdateTeamMemberRequest represents a team member role update request
type UpdateTeamMemberRequest struct {
	Role string `json:"role" validate:"required,oneof=admin member"`
}

// SwitchTeamRequest represents a request to switch the current team
type SwitchTeamRequest struct {
	TeamID string `json:"team_id" validate:"required"`
}

// CheckUserStatusRequest represents a request to check user status
type CheckUserStatusRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// Normalize normalizes the email to lowercase
func (r *CheckUserStatusRequest) Normalize() {
	r.Email = strings.ToLower(strings.TrimSpace(r.Email))
}

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
func ToUserResponse(user *User) UserResponse {
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
func ToTeamResponse(team Team) TeamResponse {
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
func ToTeamResponsePtr(team *Team) *TeamResponse {
	if team == nil {
		return nil
	}

	resp := ToTeamResponse(*team)

	return &resp
}

// ToTeamMemberResponse converts a User model with team membership to a TeamMemberResponse
func ToTeamMemberResponse(user *User, role string, joinedAt time.Time) TeamMemberResponse {
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
func ToTeamInvitationResponse(invitation *TeamInvitation) TeamInvitationResponse {
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
func ToTeamsResponse(teams []Team) []TeamResponse {
	responses := make([]TeamResponse, len(teams))
	for i, team := range teams {
		responses[i] = ToTeamResponse(team)
	}

	return responses
}

// ToTeamMembersResponse converts a slice of TeamMember models to TeamMemberResponse DTOs
func ToTeamMembersResponse(members []TeamMember) []TeamMemberResponse {
	responses := make([]TeamMemberResponse, len(members))
	for i, member := range members {
		if member.User != nil {
			responses[i] = ToTeamMemberResponse(member.User, member.Role, member.CreatedAt)
		}
	}

	return responses
}

// ToTeamInvitationsResponse converts a slice of TeamInvitation models to TeamInvitationResponse DTOs
func ToTeamInvitationsResponse(invitations []TeamInvitation) []TeamInvitationResponse {
	responses := make([]TeamInvitationResponse, len(invitations))
	for i := range invitations {
		responses[i] = ToTeamInvitationResponse(&invitations[i])
	}

	return responses
}
