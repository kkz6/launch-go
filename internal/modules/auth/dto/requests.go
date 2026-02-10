package dto

import (
	pkgdto "github.com/kkz6/launch-go/internal/pkg/dto"
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
	IPAddress            string  `json:"-"`
	UserAgent            string  `json:"-"`
}

// Normalize normalizes the email to lowercase
func (r *RegisterRequest) Normalize() {
	pkgdto.NormalizeEmail(&r.Email)
	pkgdto.NormalizeTrim(&r.Name)
}

// LoginRequest represents a user login request
type LoginRequest struct {
	Email     string `json:"email" validate:"required,email"`
	Password  string `json:"password" validate:"required"`
	Remember  bool   `json:"remember"`
	IPAddress string `json:"-"`
	UserAgent string `json:"-"`
}

// Normalize normalizes the email to lowercase
func (r *LoginRequest) Normalize() {
	pkgdto.NormalizeEmail(&r.Email)
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
	pkgdto.NormalizeEmail(&r.Email)
	pkgdto.NormalizeTrim(&r.Name)
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
	pkgdto.NormalizeEmail(&r.Email)
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
	pkgdto.NormalizeEmail(&r.Email)
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
	Role  string `json:"role" validate:"required,oneof=admin editor member"`
}

// Normalize normalizes the email to lowercase
func (r *InviteTeamMemberRequest) Normalize() {
	pkgdto.NormalizeEmail(&r.Email)
}

// UpdateTeamMemberRequest represents a team member role update request
type UpdateTeamMemberRequest struct {
	Role string `json:"role" validate:"required,oneof=admin editor member"`
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
	pkgdto.NormalizeEmail(&r.Email)
}

// PasskeyRegisterRequest represents a request to name a passkey during registration
type PasskeyRegisterRequest struct {
	Name string `json:"name" validate:"omitempty,max=255"`
}

// PasskeyLoginRequest represents a request to begin passkey authentication
type PasskeyLoginRequest struct {
	Email string `json:"email" validate:"omitempty,email"`
}

// Normalize normalizes the email to lowercase
func (r *PasskeyLoginRequest) Normalize() {
	pkgdto.NormalizeEmail(&r.Email)
}
