package services

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/enums"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/auth/repositories"
	apperrors "github.com/kkz6/launch-go/internal/pkg/errors"
)

// AuthService handles authentication-related operations
type AuthService struct {
	repo   *repositories.Repository
	config *config.Config
}

// NewAuthService creates a new AuthService instance
func NewAuthService(repo *repositories.Repository, cfg *config.Config) *AuthService {
	return &AuthService{
		repo:   repo,
		config: cfg,
	}
}

// Register creates a new user account
func (s *AuthService) Register(ctx context.Context, req *dto.RegisterRequest) (*dto.AuthResponse, error) {
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
	user := &models.User{
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
	invitation, err := s.repo.FindTeamInvitationByID(ctx, invitationID)
	if err != nil || invitation == nil {
		return
	}

	if invitation.Email != user.Email {
		return
	}

	// Add user to team
	if err := s.repo.AddUserToTeam(ctx, invitation.TeamID, user.ID, invitation.Role); err != nil {
		return
	}

	// Set current team
	if err := s.repo.SetCurrentTeam(ctx, user.ID, invitation.TeamID); err != nil {
		return
	}

	user.CurrentTeamID = &invitation.TeamID

	// Delete invitation
	s.repo.DeleteTeamInvitation(ctx, invitation.ID)
}

// createPersonalTeam creates a personal team for a new user
func (s *AuthService) createPersonalTeam(ctx context.Context, user *models.User) error {
	team := &models.Team{
		Name:         user.Name + "'s Team",
		UserID:      user.ID,
		PersonalTeam: true,
	}

	if err := s.repo.CreateTeam(ctx, team); err != nil {
		return err
	}

	// Add user to team as owner
	if err := s.repo.AddUserToTeam(ctx, team.ID, user.ID, enums.TeamRoleOwner.String()); err != nil {
		return err
	}

	// Set current team
	if err := s.repo.SetCurrentTeam(ctx, user.ID, team.ID); err != nil {
		return err
	}

	user.CurrentTeamID = &team.ID
	user.CurrentTeam = team

	return nil
}

// generateAccessToken generates a JWT access token
func (s *AuthService) generateAccessToken(user *models.User) (string, error) {
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
