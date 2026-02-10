package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/auth/repositories"
	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/security"
)

const defaultRefreshTokenHours = 24 * 30 // 30 days

// AuthService handles authentication-related operations
type AuthService struct {
	repos  *repositories.Registry
	config *config.Config
	logger *zerolog.Logger
}

// NewAuthService creates a new AuthService instance
func NewAuthService(repos *repositories.Registry, cfg *config.Config, logger *zerolog.Logger) *AuthService {
	return &AuthService{
		repos:  repos,
		config: cfg,
		logger: logger,
	}
}

// Register creates a new user account
func (s *AuthService) Register(ctx context.Context, req *dto.RegisterRequest) (*dto.AuthResponse, error) {
	req.Normalize()

	// Check if user exists
	exists, err := s.repos.User().ExistsByEmail(ctx, req.Email)
	if err != nil {
		return nil, err
	}

	if exists {
		return nil, errors.New("email already registered")
	}

	// Hash password
	hashedPassword, err := security.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	// Set default timezone
	timezone := req.Timezone
	if timezone == "" {
		timezone = "UTC"
	}

	// Create user, team, and handle invitation in a transaction
	user := &models.User{
		Name:     req.Name,
		Email:    req.Email,
		Password: hashedPassword,
		Timezone: &timezone,
	}

	err = s.repos.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			return err
		}

		// Handle invitation if provided
		if req.InvitationID != nil && *req.InvitationID != "" {
			s.handleInvitation(ctx, tx, user, *req.InvitationID)
		}

		// Create personal team if requested and not joining via invitation
		if req.CreatePersonalTeam && (req.InvitationID == nil || *req.InvitationID == "") {
			if err := s.createPersonalTeam(ctx, tx, user); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Create session
	sessionID, err := s.createSession(ctx, user.ID, req.IPAddress, req.UserAgent)
	if err != nil {
		return nil, err
	}

	return s.buildAuthResponse(ctx, user, sessionID)
}

// Login authenticates a user
func (s *AuthService) Login(ctx context.Context, req *dto.LoginRequest) (*dto.AuthResponse, error) {
	req.Normalize()

	user, err := s.repos.User().FindByEmail(ctx, req.Email)
	if err != nil {
		return nil, fiberutil.Unauthorized()
	}

	if user == nil {
		return nil, fiberutil.Unauthorized()
	}

	if !security.VerifyPassword(user.Password, req.Password) {
		return nil, fiberutil.Unauthorized()
	}

	// Create session
	sessionID, err := s.createSession(ctx, user.ID, req.IPAddress, req.UserAgent)
	if err != nil {
		return nil, err
	}

	return s.buildAuthResponse(ctx, user, sessionID)
}

// Logout invalidates the user's session
func (s *AuthService) Logout(ctx context.Context, userID, sessionID string) error {
	if sessionID != "" {
		return s.repos.Session().DeleteByUser(ctx, sessionID, userID)
	}

	return nil
}

// RefreshToken generates new access and refresh tokens
func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (*dto.AuthResponse, error) {
	// Parse and validate refresh token
	token, err := jwt.Parse(refreshToken, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}

		return []byte(s.config.JWT.Secret), nil
	})

	if err != nil || !token.Valid {
		return nil, fiberutil.Unauthorized()
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fiberutil.Unauthorized()
	}

	// Check token type
	tokenType, ok := claims["type"].(string)
	if !ok || tokenType != "refresh" {
		return nil, fiberutil.Unauthorized()
	}

	userID, ok := claims["sub"].(string)
	if !ok {
		return nil, fiberutil.Unauthorized()
	}

	user, err := s.repos.User().FindByID(ctx, userID)
	if err != nil || user == nil {
		return nil, fiberutil.Unauthorized()
	}

	// Extract session_id from refresh token claims and validate session still exists
	var sessionID string
	if sid, ok := claims["session_id"].(string); ok {
		sessionID = sid

		// Verify the session has not been revoked
		exists, err := s.repos.Session().Exists(ctx, sessionID)
		if err != nil || !exists {
			return nil, fiberutil.Unauthorized()
		}

		_ = s.repos.Session().UpdateLastActivity(ctx, sessionID)
	}

	accessToken, err := s.generateAccessToken(user, sessionID)
	if err != nil {
		return nil, err
	}

	newRefreshToken, err := s.generateRefreshToken(user, sessionID)
	if err != nil {
		return nil, err
	}

	isSubscribed := s.isTeamSubscribedOrUserAdmin(ctx, user)

	return &dto.AuthResponse{
		User:         dto.ToUserResponseWithStatus(user, isSubscribed, user.Onboarded),
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    s.config.JWT.Expiration * 3600,
		TokenType:    "Bearer",
	}, nil
}

// LoginWithPasskey creates a session and tokens for an already-verified passkey user
func (s *AuthService) LoginWithPasskey(ctx context.Context, user *models.User, ipAddress, userAgent string) (*dto.AuthResponse, error) {
	sessionID, err := s.createSession(ctx, user.ID, ipAddress, userAgent)
	if err != nil {
		return nil, err
	}

	return s.buildAuthResponse(ctx, user, sessionID)
}

// handleInvitation handles team invitation during registration
func (s *AuthService) handleInvitation(ctx context.Context, tx *gorm.DB, user *models.User, invitationID string) {
	invitation, err := s.repos.TeamInvitation().FindByID(ctx, invitationID)
	if err != nil || invitation == nil {
		if err != nil {
			s.logger.Error().Err(err).Str("invitation_id", invitationID).Msg("Failed to find invitation")
		}
		return
	}

	if invitation.Email != user.Email {
		return
	}

	// Add user to team
	role := "member"
	if invitation.Role != nil {
		role = *invitation.Role
	}

	if err := tx.Create(&models.TeamMember{
		TeamID: invitation.TeamID,
		UserID: user.ID,
		Role:   &role,
	}).Error; err != nil {
		s.logger.Error().Err(err).
			Str("team_id", invitation.TeamID).
			Str("user_id", user.ID).
			Msg("Failed to add user to team during invitation handling")
		return
	}

	// Set current team
	if err := tx.Model(user).Update("current_team_id", invitation.TeamID).Error; err != nil {
		s.logger.Error().Err(err).
			Str("user_id", user.ID).
			Str("team_id", invitation.TeamID).
			Msg("Failed to set current team during invitation handling")
		return
	}

	user.CurrentTeamID = &invitation.TeamID

	// Delete invitation
	if err := tx.Delete(invitation).Error; err != nil {
		s.logger.Error().Err(err).
			Str("invitation_id", invitation.ID).
			Msg("Failed to delete invitation during registration")
	}
}

// createPersonalTeam creates a personal team for a new user
func (s *AuthService) createPersonalTeam(_ context.Context, tx *gorm.DB, user *models.User) error {
	team := &models.Team{
		Name:         user.Name + "'s Team",
		UserID:       user.ID,
		PersonalTeam: true,
	}

	if err := tx.Create(team).Error; err != nil {
		return err
	}

	// Add user to team as owner
	ownerRole := authtypes.TeamRoleOwner.String()
	if err := tx.Create(&models.TeamMember{
		TeamID: team.ID,
		UserID: user.ID,
		Role:   &ownerRole,
	}).Error; err != nil {
		return err
	}

	// Set current team
	if err := tx.Model(user).Update("current_team_id", team.ID).Error; err != nil {
		return err
	}

	user.CurrentTeamID = &team.ID
	user.CurrentTeam = team

	return nil
}

// createSession creates a session record and returns the session ID
func (s *AuthService) createSession(ctx context.Context, userID, ipAddress, userAgent string) (string, error) {
	session := &models.Session{
		UserID:       &userID,
		IPAddress:    nilIfEmpty(ipAddress),
		UserAgent:    nilIfEmpty(userAgent),
		Payload:      "",
		LastActivity: int(time.Now().Unix()),
	}

	if err := s.repos.Session().Create(ctx, session); err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}

	return session.ID, nil
}

func (s *AuthService) buildAuthResponse(ctx context.Context, user *models.User, sessionID string) (*dto.AuthResponse, error) {
	accessToken, err := s.generateAccessToken(user, sessionID)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.generateRefreshToken(user, sessionID)
	if err != nil {
		return nil, err
	}

	isSubscribed := s.isTeamSubscribedOrUserAdmin(ctx, user)

	return &dto.AuthResponse{
		User:         dto.ToUserResponseWithStatus(user, isSubscribed, user.Onboarded),
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    s.config.JWT.Expiration * 3600,
		TokenType:    "Bearer",
	}, nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}

// generateAccessToken generates a JWT access token
// Note: team_id is NOT included in the token. Team context is passed via X-Team-ID header
// and validated by the TeamContext middleware with cached membership checks.
func (s *AuthService) generateAccessToken(user *models.User, sessionID string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":   user.ID,
		"email": user.Email,
		"name":  user.Name,
		"type":  "access",
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour * time.Duration(s.config.JWT.Expiration)).Unix(),
	}

	if sessionID != "" {
		claims["session_id"] = sessionID
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(s.config.JWT.Secret))
}

// generateRefreshToken generates a JWT refresh token
func (s *AuthService) generateRefreshToken(user *models.User, sessionID string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":  user.ID,
		"type": "refresh",
		"iat":  now.Unix(),
		"exp":  now.Add(time.Hour * defaultRefreshTokenHours).Unix(),
	}

	if sessionID != "" {
		claims["session_id"] = sessionID
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(s.config.JWT.Secret))
}

// isTeamSubscribedOrUserAdmin checks if the user's current team is subscribed or user is admin
func (s *AuthService) isTeamSubscribedOrUserAdmin(ctx context.Context, user *models.User) bool {
	if user.CurrentTeamID == nil {
		return false
	}

	// Admin users bypass subscription check
	if s.repos.IsUserAdmin(ctx, user.ID) {
		return true
	}

	return s.repos.IsTeamSubscribed(ctx, *user.CurrentTeamID)
}
