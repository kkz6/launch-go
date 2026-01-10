package auth

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"

	"github.com/kkz6/launch-go/internal/config"
	apperrors "github.com/kkz6/launch-go/internal/pkg/errors"
)

type Service struct {
	repo   *Repository
	config *config.Config
	logger *zerolog.Logger
}

func NewService(repo *Repository, cfg *config.Config, logger *zerolog.Logger) *Service {
	return &Service{
		repo:   repo,
		config: cfg,
		logger: logger,
	}
}

func (s *Service) Register(ctx context.Context, req *RegisterRequest) (*AuthResponse, error) {
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

	// Create user
	user := &User{
		Name:     req.Name,
		Email:    req.Email,
		Password: string(hashedPassword),
	}

	if err := s.repo.CreateUser(ctx, user); err != nil {
		return nil, err
	}

	// Create personal team
	team := &Team{
		Name:         user.Name + "'s Team",
		OwnerID:      user.ID,
		PersonalTeam: true,
	}

	if err := s.repo.CreateTeam(ctx, team); err != nil {
		return nil, err
	}

	// Add user to team as owner
	if err := s.repo.AddUserToTeam(ctx, team.ID, user.ID, "owner"); err != nil {
		return nil, err
	}

	// Set current team
	if err := s.repo.SetCurrentTeam(ctx, user.ID, team.ID); err != nil {
		return nil, err
	}

	user.CurrentTeamID = &team.ID
	user.CurrentTeam = team

	// Generate tokens
	token, err := s.generateToken(user)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{
		User:        ToUserResponse(user),
		AccessToken: token,
		ExpiresIn:   s.config.JWT.Expiration * 3600,
	}, nil
}

func (s *Service) Login(ctx context.Context, req *LoginRequest) (*AuthResponse, error) {
	user, err := s.repo.FindUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, apperrors.ErrUnauthorized
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, apperrors.ErrUnauthorized
	}

	token, err := s.generateToken(user)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{
		User:        ToUserResponse(user),
		AccessToken: token,
		ExpiresIn:   s.config.JWT.Expiration * 3600,
	}, nil
}

func (s *Service) GetUser(ctx context.Context, userID string) (*User, error) {
	return s.repo.FindUserByID(ctx, userID)
}

func (s *Service) UpdateProfile(ctx context.Context, userID string, req *UpdateProfileRequest) (*User, error) {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, err
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
	}

	user.Name = req.Name
	user.Email = req.Email

	if err := s.repo.UpdateUser(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *Service) ChangePassword(ctx context.Context, userID string, req *ChangePasswordRequest) error {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return err
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

func (s *Service) SwitchTeam(ctx context.Context, userID, teamID string) (*User, error) {
	// Verify user is member of team
	teams, err := s.repo.GetUserTeams(ctx, userID)
	if err != nil {
		return nil, err
	}

	var isMember bool
	for _, team := range teams {
		if team.ID == teamID {
			isMember = true
			break
		}
	}

	if !isMember {
		return nil, apperrors.ErrForbidden
	}

	if err := s.repo.SetCurrentTeam(ctx, userID, teamID); err != nil {
		return nil, err
	}

	return s.repo.FindUserByID(ctx, userID)
}

func (s *Service) generateToken(user *User) (string, error) {
	claims := jwt.MapClaims{
		"sub":     user.ID,
		"email":   user.Email,
		"team_id": user.CurrentTeamID,
		"exp":     time.Now().Add(time.Hour * time.Duration(s.config.JWT.Expiration)).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.config.JWT.Secret))
}
