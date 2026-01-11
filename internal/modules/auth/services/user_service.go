package services

import (
	"context"
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/auth/repositories"
	apperrors "github.com/kkz6/launch-go/internal/pkg/errors"
)

// UserService handles user management operations
type UserService struct {
	repo *repositories.Repository
}

// NewUserService creates a new UserService instance
func NewUserService(repo *repositories.Repository) *UserService {
	return &UserService{repo: repo}
}

// GetUser retrieves a user by ID
func (s *UserService) GetUser(ctx context.Context, userID string) (*models.User, error) {
	return s.repo.FindUserByID(ctx, userID)
}

// GetUserByEmail retrieves a user by email
func (s *UserService) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	return s.repo.FindUserByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
}

// UpdateProfile updates a user's profile
func (s *UserService) UpdateProfile(ctx context.Context, userID string, req *dto.UpdateProfileRequest) (*models.User, error) {
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
func (s *UserService) ChangePassword(ctx context.Context, userID string, req *dto.ChangePasswordRequest) error {
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
func (s *UserService) DeleteAccount(ctx context.Context, userID string) error {
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
		if team.UserID == userID {
			s.repo.DeleteTeam(ctx, team.ID)
		}
	}

	return s.repo.DeleteUser(ctx, userID)
}

// CheckUserStatus checks a user's status by email
func (s *UserService) CheckUserStatus(ctx context.Context, email string) (*dto.UserStatusResponse, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	user, err := s.repo.FindUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	if user == nil {
		return &dto.UserStatusResponse{
			UserExists:           false,
			RequiresVerification: false,
			HasTwoFactor:         false,
		}, nil
	}

	return &dto.UserStatusResponse{
		UserExists:           true,
		RequiresVerification: !user.HasVerifiedEmail(),
		HasTwoFactor:         user.HasEnabledTwoFactorAuthentication(),
	}, nil
}
