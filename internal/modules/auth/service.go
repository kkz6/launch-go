package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pquerna/otp/totp"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"

	"github.com/kkz6/launch-go/internal/config"
	apperrors "github.com/kkz6/launch-go/internal/pkg/errors"
)

// ServiceInterface defines the interface for auth service operations
type ServiceInterface interface {
	// Authentication
	Register(ctx context.Context, req *RegisterRequest) (*AuthResponse, error)
	Login(ctx context.Context, req *LoginRequest) (*AuthResponse, error)
	Logout(ctx context.Context, userID string) error
	RefreshToken(ctx context.Context, refreshToken string) (*AuthResponse, error)

	// User Management
	GetUser(ctx context.Context, userID string) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	UpdateProfile(ctx context.Context, userID string, req *UpdateProfileRequest) (*User, error)
	ChangePassword(ctx context.Context, userID string, req *ChangePasswordRequest) error
	DeleteAccount(ctx context.Context, userID string) error

	// Email Verification
	VerifyEmail(ctx context.Context, userID, hash string) error
	ResendVerificationEmail(ctx context.Context, userID string) error

	// Password Reset
	SendPasswordResetLink(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, req *ResetPasswordRequest) error

	// Two-Factor Authentication
	EnableTwoFactor(ctx context.Context, userID string) (*TwoFactorResponse, error)
	ConfirmTwoFactor(ctx context.Context, userID, code string) error
	DisableTwoFactor(ctx context.Context, userID, password string) error
	VerifyTwoFactor(ctx context.Context, userID, code string) (bool, error)
	GetRecoveryCodes(ctx context.Context, userID string) ([]string, error)
	RegenerateRecoveryCodes(ctx context.Context, userID string) ([]string, error)
	HasTwoFactorEnabled(ctx context.Context, userID string) (bool, error)

	// Team Management
	CreateTeam(ctx context.Context, userID string, req *CreateTeamRequest) (*Team, error)
	UpdateTeam(ctx context.Context, userID, teamID string, req *UpdateTeamRequest) (*Team, error)
	DeleteTeam(ctx context.Context, userID, teamID string) error
	GetTeam(ctx context.Context, teamID string) (*Team, error)
	GetUserTeams(ctx context.Context, userID string) ([]Team, error)
	SwitchTeam(ctx context.Context, userID, teamID string) (*User, error)

	// Team Member Management
	InviteTeamMember(ctx context.Context, userID, teamID string, req *InviteTeamMemberRequest) error
	AcceptTeamInvitation(ctx context.Context, userID, invitationID string) error
	CancelTeamInvitation(ctx context.Context, userID, teamID, invitationID string) error
	UpdateTeamMemberRole(ctx context.Context, userID, teamID, memberID string, req *UpdateTeamMemberRequest) error
	RemoveTeamMember(ctx context.Context, userID, teamID, memberID string) error
	GetTeamMembers(ctx context.Context, teamID string) ([]TeamMember, error)
	GetTeamInvitations(ctx context.Context, teamID string) ([]TeamInvitation, error)

	// User Status
	CheckUserStatus(ctx context.Context, email string) (*UserStatusResponse, error)
}

// Service implements ServiceInterface for authentication business logic
type Service struct {
	repo   *Repository
	config *config.Config
	logger *zerolog.Logger
}

// NewService creates a new Service instance
func NewService(repo *Repository, cfg *config.Config, logger *zerolog.Logger) *Service {
	return &Service{
		repo:   repo,
		config: cfg,
		logger: logger,
	}
}

// Authentication Methods

// Register creates a new user account
func (s *Service) Register(ctx context.Context, req *RegisterRequest) (*AuthResponse, error) {
	req.Normalize()

	// Check if user exists
	exists, err := s.repo.UserExistsByEmail(ctx, req.Email)
	if err != nil {
		return nil, err
	}

	if exists {
		return nil, errors.New("email already registered")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// Set default timezone
	timezone := req.Timezone
	if timezone == "" {
		timezone = "UTC"
	}

	// Create user
	user := &User{
		Name:     req.Name,
		Email:    req.Email,
		Password: string(hashedPassword),
		Timezone: timezone,
	}

	if err := s.repo.CreateUser(ctx, user); err != nil {
		return nil, err
	}

	// Handle invitation if provided
	if req.InvitationID != nil && *req.InvitationID != "" {
		invitation, err := s.repo.FindTeamInvitationByID(ctx, *req.InvitationID)
		if err != nil {
			s.logger.Warn().Err(err).Msg("Failed to find invitation")
		} else if invitation != nil && strings.EqualFold(invitation.Email, req.Email) {
			// Add user to team
			if err := s.repo.AddUserToTeam(ctx, invitation.TeamID, user.ID, invitation.Role); err != nil {
				s.logger.Warn().Err(err).Msg("Failed to add user to team")
			} else {
				// Set current team
				if err := s.repo.SetCurrentTeam(ctx, user.ID, invitation.TeamID); err != nil {
					s.logger.Warn().Err(err).Msg("Failed to set current team")
				}

				user.CurrentTeamID = &invitation.TeamID

				// Delete invitation
				s.repo.DeleteTeamInvitation(ctx, invitation.ID)
			}
		}
	}

	// Create personal team if requested and not joining via invitation
	if req.CreatePersonalTeam && (req.InvitationID == nil || *req.InvitationID == "") {
		team := &Team{
			Name:         user.Name + "'s Team",
			OwnerID:      user.ID,
			PersonalTeam: true,
		}

		if err := s.repo.CreateTeam(ctx, team); err != nil {
			return nil, err
		}

		// Add user to team as owner
		if err := s.repo.AddUserToTeam(ctx, team.ID, user.ID, TeamRoleOwner.String()); err != nil {
			return nil, err
		}

		// Set current team
		if err := s.repo.SetCurrentTeam(ctx, user.ID, team.ID); err != nil {
			return nil, err
		}

		user.CurrentTeamID = &team.ID
		user.CurrentTeam = team
	}

	// Generate tokens
	accessToken, err := s.generateAccessToken(user)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.generateRefreshToken(user)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{
		User:         ToUserResponse(user),
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    s.config.JWT.Expiration * 3600,
		TokenType:    "Bearer",
	}, nil
}

// Login authenticates a user
func (s *Service) Login(ctx context.Context, req *LoginRequest) (*AuthResponse, error) {
	req.Normalize()

	user, err := s.repo.FindUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, apperrors.ErrUnauthorized
	}

	if user == nil {
		return nil, apperrors.ErrUnauthorized
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, apperrors.ErrUnauthorized
	}

	accessToken, err := s.generateAccessToken(user)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.generateRefreshToken(user)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{
		User:         ToUserResponse(user),
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    s.config.JWT.Expiration * 3600,
		TokenType:    "Bearer",
	}, nil
}

// Logout invalidates the user's session (placeholder for token blacklisting)
func (s *Service) Logout(ctx context.Context, userID string) error {
	// In a production environment, you would implement token blacklisting here
	// For now, we just log the logout event
	s.logger.Info().Str("user_id", userID).Msg("User logged out")

	return nil
}

// RefreshToken generates new access and refresh tokens
func (s *Service) RefreshToken(ctx context.Context, refreshToken string) (*AuthResponse, error) {
	// Parse and validate refresh token
	token, err := jwt.Parse(refreshToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}

		return []byte(s.config.JWT.Secret), nil
	})

	if err != nil || !token.Valid {
		return nil, apperrors.ErrUnauthorized
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, apperrors.ErrUnauthorized
	}

	// Check token type
	tokenType, ok := claims["type"].(string)
	if !ok || tokenType != "refresh" {
		return nil, apperrors.ErrUnauthorized
	}

	userID, ok := claims["sub"].(string)
	if !ok {
		return nil, apperrors.ErrUnauthorized
	}

	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil || user == nil {
		return nil, apperrors.ErrUnauthorized
	}

	accessToken, err := s.generateAccessToken(user)
	if err != nil {
		return nil, err
	}

	newRefreshToken, err := s.generateRefreshToken(user)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{
		User:         ToUserResponse(user),
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    s.config.JWT.Expiration * 3600,
		TokenType:    "Bearer",
	}, nil
}

// User Management Methods

// GetUser retrieves a user by ID
func (s *Service) GetUser(ctx context.Context, userID string) (*User, error) {
	return s.repo.FindUserByID(ctx, userID)
}

// GetUserByEmail retrieves a user by email
func (s *Service) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	return s.repo.FindUserByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
}

// UpdateProfile updates a user's profile
func (s *Service) UpdateProfile(ctx context.Context, userID string, req *UpdateProfileRequest) (*User, error) {
	req.Normalize()

	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, apperrors.ErrNotFound
	}

	// Check if email is changing and already exists
	if user.Email != req.Email {
		exists, err := s.repo.UserExistsByEmail(ctx, req.Email)
		if err != nil {
			return nil, err
		}

		if exists {
			return nil, errors.New("email already in use")
		}

		// Reset email verification if email changed
		user.EmailVerifiedAt = nil
	}

	user.Name = req.Name
	user.Email = req.Email

	if req.Timezone != "" {
		user.Timezone = req.Timezone
	}

	if err := s.repo.UpdateUser(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

// ChangePassword changes a user's password
func (s *Service) ChangePassword(ctx context.Context, userID string, req *ChangePasswordRequest) error {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return err
	}

	if user == nil {
		return apperrors.ErrNotFound
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.CurrentPassword)); err != nil {
		return errors.New("current password is incorrect")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user.Password = string(hashedPassword)

	return s.repo.UpdateUser(ctx, user)
}

// DeleteAccount deletes a user's account
func (s *Service) DeleteAccount(ctx context.Context, userID string) error {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return err
	}

	if user == nil {
		return apperrors.ErrNotFound
	}

	// Delete owned teams
	ownedTeams, err := s.repo.GetUserTeams(ctx, userID)
	if err != nil {
		return err
	}

	for _, team := range ownedTeams {
		if team.OwnerID == userID {
			if err := s.repo.DeleteTeam(ctx, team.ID); err != nil {
				s.logger.Warn().Err(err).Str("team_id", team.ID).Msg("Failed to delete owned team")
			}
		}
	}

	return s.repo.DeleteUser(ctx, userID)
}

// Email Verification Methods

// VerifyEmail verifies a user's email address
func (s *Service) VerifyEmail(ctx context.Context, userID, hash string) error {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return err
	}

	if user == nil {
		return apperrors.ErrNotFound
	}

	// Verify hash matches email
	expectedHash := s.generateEmailHash(user.Email)
	if hash != expectedHash {
		return errors.New("invalid verification link")
	}

	if user.HasVerifiedEmail() {
		return nil // Already verified
	}

	return s.repo.MarkEmailAsVerified(ctx, userID)
}

// ResendVerificationEmail resends the email verification
func (s *Service) ResendVerificationEmail(ctx context.Context, userID string) error {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return err
	}

	if user == nil {
		return apperrors.ErrNotFound
	}

	if user.HasVerifiedEmail() {
		return errors.New("email already verified")
	}

	// In a production environment, you would send the verification email here
	s.logger.Info().Str("user_id", userID).Str("email", user.Email).Msg("Verification email requested")

	return nil
}

// Password Reset Methods

// SendPasswordResetLink sends a password reset email
func (s *Service) SendPasswordResetLink(ctx context.Context, email string) error {
	email = strings.ToLower(strings.TrimSpace(email))

	user, err := s.repo.FindUserByEmail(ctx, email)
	if err != nil {
		return err
	}

	// Always return success to prevent email enumeration
	if user == nil {
		s.logger.Debug().Str("email", email).Msg("Password reset requested for non-existent email")

		return nil
	}

	// Generate token
	token, err := GenerateToken(32)
	if err != nil {
		return err
	}

	// Hash the token for storage
	hashedToken := s.hashToken(token)

	// Store token
	resetToken := &PasswordResetToken{
		Email:     email,
		Token:     hashedToken,
		CreatedAt: time.Now(),
	}

	if err := s.repo.CreatePasswordResetToken(ctx, resetToken); err != nil {
		return err
	}

	// In a production environment, you would send the reset email here
	s.logger.Info().Str("email", email).Str("token", token).Msg("Password reset email requested")

	return nil
}

// ResetPassword resets a user's password
func (s *Service) ResetPassword(ctx context.Context, req *ResetPasswordRequest) error {
	req.Normalize()

	resetToken, err := s.repo.FindPasswordResetToken(ctx, req.Email)
	if err != nil {
		return err
	}

	if resetToken == nil {
		return errors.New("invalid or expired reset token")
	}

	// Check if token expired
	if resetToken.IsExpired() {
		s.repo.DeletePasswordResetToken(ctx, req.Email)

		return errors.New("invalid or expired reset token")
	}

	// Verify token
	hashedToken := s.hashToken(req.Token)
	if hashedToken != resetToken.Token {
		return errors.New("invalid or expired reset token")
	}

	// Find user
	user, err := s.repo.FindUserByEmail(ctx, req.Email)
	if err != nil {
		return err
	}

	if user == nil {
		return errors.New("user not found")
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user.Password = string(hashedPassword)

	if err := s.repo.UpdateUser(ctx, user); err != nil {
		return err
	}

	// Delete the reset token
	return s.repo.DeletePasswordResetToken(ctx, req.Email)
}

// Two-Factor Authentication Methods

// EnableTwoFactor initiates 2FA setup
func (s *Service) EnableTwoFactor(ctx context.Context, userID string) (*TwoFactorResponse, error) {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, apperrors.ErrNotFound
	}

	// Generate TOTP secret
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      s.config.App.Name,
		AccountName: user.Email,
	})
	if err != nil {
		return nil, err
	}

	// Store secret (not confirmed yet)
	secret := key.Secret()
	user.TwoFactorSecret = &secret
	user.TwoFactorConfirmedAt = nil

	if err := s.repo.UpdateUser(ctx, user); err != nil {
		return nil, err
	}

	return &TwoFactorResponse{
		QRCodeURL: key.URL(),
		SecretKey: key.Secret(),
	}, nil
}

// ConfirmTwoFactor confirms 2FA setup
func (s *Service) ConfirmTwoFactor(ctx context.Context, userID, code string) error {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return err
	}

	if user == nil {
		return apperrors.ErrNotFound
	}

	if user.TwoFactorSecret == nil {
		return errors.New("two-factor authentication not initiated")
	}

	// Verify code
	valid := totp.Validate(code, *user.TwoFactorSecret)
	if !valid {
		return errors.New("invalid verification code")
	}

	// Confirm 2FA
	now := time.Now()
	user.TwoFactorConfirmedAt = &now

	// Generate recovery codes
	recoveryCodes, err := s.generateRecoveryCodes()
	if err != nil {
		return err
	}

	codesStr := strings.Join(recoveryCodes, ",")
	user.TwoFactorRecoveryCodes = &codesStr

	return s.repo.UpdateUser(ctx, user)
}

// DisableTwoFactor disables 2FA
func (s *Service) DisableTwoFactor(ctx context.Context, userID, password string) error {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return err
	}

	if user == nil {
		return apperrors.ErrNotFound
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return errors.New("invalid password")
	}

	user.TwoFactorSecret = nil
	user.TwoFactorConfirmedAt = nil
	user.TwoFactorRecoveryCodes = nil

	return s.repo.UpdateUser(ctx, user)
}

// VerifyTwoFactor verifies a 2FA code
func (s *Service) VerifyTwoFactor(ctx context.Context, userID, code string) (bool, error) {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return false, err
	}

	if user == nil {
		return false, apperrors.ErrNotFound
	}

	if !user.HasEnabledTwoFactorAuthentication() {
		return true, nil // 2FA not enabled, consider verified
	}

	// Try TOTP code first
	if totp.Validate(code, *user.TwoFactorSecret) {
		return true, nil
	}

	// Try recovery code
	if user.TwoFactorRecoveryCodes != nil {
		codes := strings.Split(*user.TwoFactorRecoveryCodes, ",")
		for i, c := range codes {
			if c == code {
				// Remove used recovery code
				codes = append(codes[:i], codes[i+1:]...)
				codesStr := strings.Join(codes, ",")
				user.TwoFactorRecoveryCodes = &codesStr
				s.repo.UpdateUser(ctx, user)

				return true, nil
			}
		}
	}

	return false, nil
}

// GetRecoveryCodes returns the user's recovery codes
func (s *Service) GetRecoveryCodes(ctx context.Context, userID string) ([]string, error) {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, apperrors.ErrNotFound
	}

	if user.TwoFactorRecoveryCodes == nil {
		return []string{}, nil
	}

	return strings.Split(*user.TwoFactorRecoveryCodes, ","), nil
}

// RegenerateRecoveryCodes generates new recovery codes
func (s *Service) RegenerateRecoveryCodes(ctx context.Context, userID string) ([]string, error) {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, apperrors.ErrNotFound
	}

	if !user.HasEnabledTwoFactorAuthentication() {
		return nil, errors.New("two-factor authentication not enabled")
	}

	codes, err := s.generateRecoveryCodes()
	if err != nil {
		return nil, err
	}

	codesStr := strings.Join(codes, ",")
	user.TwoFactorRecoveryCodes = &codesStr

	if err := s.repo.UpdateUser(ctx, user); err != nil {
		return nil, err
	}

	return codes, nil
}

// HasTwoFactorEnabled checks if user has 2FA enabled
func (s *Service) HasTwoFactorEnabled(ctx context.Context, userID string) (bool, error) {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return false, err
	}

	if user == nil {
		return false, nil
	}

	return user.HasEnabledTwoFactorAuthentication(), nil
}

// Team Management Methods

// CreateTeam creates a new team
func (s *Service) CreateTeam(ctx context.Context, userID string, req *CreateTeamRequest) (*Team, error) {
	team := &Team{
		Name:         req.Name,
		OwnerID:      userID,
		PersonalTeam: req.PersonalTeam,
	}

	if err := s.repo.CreateTeam(ctx, team); err != nil {
		return nil, err
	}

	// Add owner as team member
	if err := s.repo.AddUserToTeam(ctx, team.ID, userID, TeamRoleOwner.String()); err != nil {
		return nil, err
	}

	return team, nil
}

// UpdateTeam updates a team
func (s *Service) UpdateTeam(ctx context.Context, userID, teamID string, req *UpdateTeamRequest) (*Team, error) {
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
func (s *Service) DeleteTeam(ctx context.Context, userID, teamID string) error {
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
func (s *Service) GetTeam(ctx context.Context, teamID string) (*Team, error) {
	return s.repo.FindTeamByID(ctx, teamID)
}

// GetUserTeams retrieves all teams for a user
func (s *Service) GetUserTeams(ctx context.Context, userID string) ([]Team, error) {
	return s.repo.GetUserTeams(ctx, userID)
}

// SwitchTeam switches the user's current team
func (s *Service) SwitchTeam(ctx context.Context, userID, teamID string) (*User, error) {
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

// Team Member Management Methods

// InviteTeamMember invites a user to a team
func (s *Service) InviteTeamMember(ctx context.Context, userID, teamID string, req *InviteTeamMemberRequest) error {
	req.Normalize()

	team, err := s.repo.FindTeamByID(ctx, teamID)
	if err != nil {
		return err
	}

	if team == nil {
		return apperrors.ErrNotFound
	}

	// Check if user has permission to invite
	if team.OwnerID != userID {
		member, err := s.repo.GetTeamMember(ctx, teamID, userID)
		if err != nil {
			return err
		}

		if member == nil || member.Role != TeamRoleAdmin.String() {
			return apperrors.ErrForbidden
		}
	}

	// Check if user is already a member
	existingUser, err := s.repo.FindUserByEmail(ctx, req.Email)
	if err != nil {
		return err
	}

	if existingUser != nil {
		isMember, err := s.repo.IsTeamMember(ctx, teamID, existingUser.ID)
		if err != nil {
			return err
		}

		if isMember {
			return errors.New("user is already a team member")
		}
	}

	// Check if invitation already exists
	existingInvitation, err := s.repo.FindTeamInvitationByEmail(ctx, teamID, req.Email)
	if err != nil {
		return err
	}

	if existingInvitation != nil {
		return errors.New("invitation already sent to this email")
	}

	// Create invitation
	invitation := &TeamInvitation{
		TeamID: teamID,
		Email:  req.Email,
		Role:   req.Role,
	}

	if err := s.repo.CreateTeamInvitation(ctx, invitation); err != nil {
		return err
	}

	// In a production environment, you would send the invitation email here
	s.logger.Info().
		Str("team_id", teamID).
		Str("email", req.Email).
		Str("role", req.Role).
		Msg("Team invitation created")

	return nil
}

// AcceptTeamInvitation accepts a team invitation
func (s *Service) AcceptTeamInvitation(ctx context.Context, userID, invitationID string) error {
	invitation, err := s.repo.FindTeamInvitationByID(ctx, invitationID)
	if err != nil {
		return err
	}

	if invitation == nil {
		return apperrors.ErrNotFound
	}

	user, err := s.repo.FindUserByID(ctx, userID)
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
	if err := s.repo.AddUserToTeam(ctx, invitation.TeamID, userID, invitation.Role); err != nil {
		return err
	}

	// Delete invitation
	return s.repo.DeleteTeamInvitation(ctx, invitationID)
}

// CancelTeamInvitation cancels a team invitation
func (s *Service) CancelTeamInvitation(ctx context.Context, userID, teamID, invitationID string) error {
	team, err := s.repo.FindTeamByID(ctx, teamID)
	if err != nil {
		return err
	}

	if team == nil {
		return apperrors.ErrNotFound
	}

	// Check permission
	if team.OwnerID != userID {
		member, err := s.repo.GetTeamMember(ctx, teamID, userID)
		if err != nil {
			return err
		}

		if member == nil || member.Role != TeamRoleAdmin.String() {
			return apperrors.ErrForbidden
		}
	}

	invitation, err := s.repo.FindTeamInvitationByID(ctx, invitationID)
	if err != nil {
		return err
	}

	if invitation == nil || invitation.TeamID != teamID {
		return apperrors.ErrNotFound
	}

	return s.repo.DeleteTeamInvitation(ctx, invitationID)
}

// UpdateTeamMemberRole updates a team member's role
func (s *Service) UpdateTeamMemberRole(ctx context.Context, userID, teamID, memberID string, req *UpdateTeamMemberRequest) error {
	team, err := s.repo.FindTeamByID(ctx, teamID)
	if err != nil {
		return err
	}

	if team == nil {
		return apperrors.ErrNotFound
	}

	// Only owner can update roles
	if team.OwnerID != userID {
		return apperrors.ErrForbidden
	}

	// Cannot update owner's role
	if memberID == team.OwnerID {
		return errors.New("cannot update owner's role")
	}

	return s.repo.UpdateTeamMemberRole(ctx, teamID, memberID, req.Role)
}

// RemoveTeamMember removes a member from a team
func (s *Service) RemoveTeamMember(ctx context.Context, userID, teamID, memberID string) error {
	team, err := s.repo.FindTeamByID(ctx, teamID)
	if err != nil {
		return err
	}

	if team == nil {
		return apperrors.ErrNotFound
	}

	// Check permission (owner or self-removal)
	if team.OwnerID != userID && userID != memberID {
		return apperrors.ErrForbidden
	}

	// Cannot remove owner
	if memberID == team.OwnerID {
		return errors.New("cannot remove team owner")
	}

	// Remove from team
	if err := s.repo.RemoveUserFromTeam(ctx, teamID, memberID); err != nil {
		return err
	}

	// If this was their current team, switch to another
	member, err := s.repo.FindUserByID(ctx, memberID)
	if err != nil {
		return nil // Member removed successfully, ignore this error
	}

	if member != nil && member.CurrentTeamID != nil && *member.CurrentTeamID == teamID {
		teams, err := s.repo.GetUserTeams(ctx, memberID)
		if err != nil {
			return nil
		}

		if len(teams) > 0 {
			s.repo.SetCurrentTeam(ctx, memberID, teams[0].ID)
		}
	}

	return nil
}

// GetTeamMembers gets all members of a team
func (s *Service) GetTeamMembers(ctx context.Context, teamID string) ([]TeamMember, error) {
	return s.repo.GetTeamMembers(ctx, teamID)
}

// GetTeamInvitations gets all invitations for a team
func (s *Service) GetTeamInvitations(ctx context.Context, teamID string) ([]TeamInvitation, error) {
	return s.repo.GetTeamInvitations(ctx, teamID)
}

// User Status Methods

// CheckUserStatus checks a user's status by email
func (s *Service) CheckUserStatus(ctx context.Context, email string) (*UserStatusResponse, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	user, err := s.repo.FindUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	if user == nil {
		return &UserStatusResponse{
			UserExists:           false,
			RequiresVerification: false,
			HasTwoFactor:         false,
		}, nil
	}

	return &UserStatusResponse{
		UserExists:           true,
		RequiresVerification: !user.HasVerifiedEmail(),
		HasTwoFactor:         user.HasEnabledTwoFactorAuthentication(),
	}, nil
}

// Helper Methods

// generateAccessToken generates a JWT access token
func (s *Service) generateAccessToken(user *User) (string, error) {
	claims := jwt.MapClaims{
		"sub":     user.ID,
		"email":   user.Email,
		"team_id": user.CurrentTeamID,
		"type":    "access",
		"exp":     time.Now().Add(time.Hour * time.Duration(s.config.JWT.Expiration)).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(s.config.JWT.Secret))
}

// generateRefreshToken generates a JWT refresh token
func (s *Service) generateRefreshToken(user *User) (string, error) {
	claims := jwt.MapClaims{
		"sub":  user.ID,
		"type": "refresh",
		"exp":  time.Now().Add(time.Hour * 24 * 30).Unix(), // 30 days
		"iat":  time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(s.config.JWT.Secret))
}

// generateEmailHash generates a hash for email verification
func (s *Service) generateEmailHash(email string) string {
	hash := sha256.Sum256([]byte(email + s.config.JWT.Secret))

	return hex.EncodeToString(hash[:])
}

// hashToken hashes a token for secure storage
func (s *Service) hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))

	return hex.EncodeToString(hash[:])
}

// generateRecoveryCodes generates random recovery codes
func (s *Service) generateRecoveryCodes() ([]string, error) {
	codes := make([]string, 8)
	for i := range codes {
		token, err := GenerateToken(8)
		if err != nil {
			return nil, err
		}

		// Convert to a readable format
		codes[i] = fmt.Sprintf("%s-%s",
			base32.StdEncoding.EncodeToString([]byte(token[:4]))[:5],
			base32.StdEncoding.EncodeToString([]byte(token[4:]))[:5],
		)
	}

	return codes, nil
}
