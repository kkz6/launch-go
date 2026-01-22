package services

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/enums"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/auth/repositories"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/security"
)

// AuthService handles authentication-related operations
type AuthService struct {
	repos  *repositories.Registry
	config *config.Config
}

// NewAuthService creates a new AuthService instance
func NewAuthService(repos *repositories.Registry, cfg *config.Config) *AuthService {
	return &AuthService{
		repos:  repos,
		config: cfg,
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

	// Create user
	user := &models.User{
		Name:     req.Name,
		Email:    req.Email,
		Password: hashedPassword,
		Timezone: &timezone,
	}

	if err := s.repos.User().Create(ctx, user); err != nil {
		return nil, err
	}

	// Handle invitation if provided
	if req.InvitationID != nil && *req.InvitationID != "" {
		s.handleInvitation(ctx, user, *req.InvitationID)
	}

	// Create personal team if requested and not joining via invitation
	if req.CreatePersonalTeam && (req.InvitationID == nil || *req.InvitationID == "") {
		if err := s.createPersonalTeam(ctx, user); err != nil {
			return nil, err
		}
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

	return &dto.AuthResponse{
		User:         dto.ToUserResponse(user),
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    s.config.JWT.Expiration * 3600,
		TokenType:    "Bearer",
	}, nil
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

	accessToken, err := s.generateAccessToken(user)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.generateRefreshToken(user)
	if err != nil {
		return nil, err
	}

	return &dto.AuthResponse{
		User:         dto.ToUserResponse(user),
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    s.config.JWT.Expiration * 3600,
		TokenType:    "Bearer",
	}, nil
}

// Logout invalidates the user's session
func (s *AuthService) Logout(ctx context.Context, userID string) error {
	// In a production environment, you would implement token blacklisting here
	return nil
}

// RefreshToken generates new access and refresh tokens
func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (*dto.AuthResponse, error) {
	// Parse and validate refresh token
	token, err := jwt.Parse(refreshToken, func(token *jwt.Token) (interface{}, error) {
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

	accessToken, err := s.generateAccessToken(user)
	if err != nil {
		return nil, err
	}

	newRefreshToken, err := s.generateRefreshToken(user)
	if err != nil {
		return nil, err
	}

	return &dto.AuthResponse{
		User:         dto.ToUserResponse(user),
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    s.config.JWT.Expiration * 3600,
		TokenType:    "Bearer",
	}, nil
}

// handleInvitation handles team invitation during registration
func (s *AuthService) handleInvitation(ctx context.Context, user *models.User, invitationID string) {
	invitation, err := s.repos.TeamInvitation().FindByID(ctx, invitationID)
	if err != nil || invitation == nil {
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
	if err := s.repos.TeamMember().AddUser(ctx, invitation.TeamID, user.ID, role); err != nil {
		return
	}

	// Set current team
	if err := s.repos.User().SetCurrentTeam(ctx, user.ID, invitation.TeamID); err != nil {
		return
	}

	user.CurrentTeamID = &invitation.TeamID

	// Delete invitation
	s.repos.TeamInvitation().Delete(ctx, invitation.ID)
}

// createPersonalTeam creates a personal team for a new user
func (s *AuthService) createPersonalTeam(ctx context.Context, user *models.User) error {
	team := &models.Team{
		Name:         user.Name + "'s Team",
		UserID:       user.ID,
		PersonalTeam: true,
	}

	if err := s.repos.Team().Create(ctx, team); err != nil {
		return err
	}

	// Add user to team as owner
	if err := s.repos.TeamMember().AddUser(ctx, team.ID, user.ID, enums.TeamRoleOwner.String()); err != nil {
		return err
	}

	// Set current team
	if err := s.repos.User().SetCurrentTeam(ctx, user.ID, team.ID); err != nil {
		return err
	}

	user.CurrentTeamID = &team.ID
	user.CurrentTeam = team

	return nil
}

// generateAccessToken generates a JWT access token
// Note: team_id is NOT included in the token. Team context is passed via X-Team-ID header
// and validated by the TeamContext middleware with cached membership checks.
func (s *AuthService) generateAccessToken(user *models.User) (string, error) {
	claims := jwt.MapClaims{
		"sub":   user.ID,
		"email": user.Email,
		"type":  "access",
		"exp":   time.Now().Add(time.Hour * time.Duration(s.config.JWT.Expiration)).Unix(),
		"iat":   time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(s.config.JWT.Secret))
}

// generateRefreshToken generates a JWT refresh token
func (s *AuthService) generateRefreshToken(user *models.User) (string, error) {
	claims := jwt.MapClaims{
		"sub":  user.ID,
		"type": "refresh",
		"exp":  time.Now().Add(time.Hour * 24 * 30).Unix(), // 30 days
		"iat":  time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(s.config.JWT.Secret))
}
