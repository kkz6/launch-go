package services

import (
	"context"
	"errors"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/auth/repositories"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
	"github.com/kkz6/launch-go/internal/pkg/security"
)

// UserService handles user management operations
type UserService struct {
	repos *repositories.Registry
}

// NewUserService creates a new UserService instance
func NewUserService(repos *repositories.Registry) *UserService {
	return &UserService{repos: repos}
}

// GetUser retrieves a user by ID
func (s *UserService) GetUser(ctx context.Context, userID string) (*models.User, error) {
	return s.repos.User().FindByID(ctx, userID)
}

// GetUserByEmail retrieves a user by email
func (s *UserService) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	return s.repos.User().FindByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
}

// UpdateProfile updates a user's profile
func (s *UserService) UpdateProfile(ctx context.Context, userID string, req *dto.UpdateProfileRequest) (*models.User, error) {
	req.Normalize()

	user, err := s.repos.User().FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, fiberutil.NotFound()
	}

	// Check if email is changing and already exists
	if user.Email != req.Email {
		exists, err := s.repos.User().ExistsByEmail(ctx, req.Email)
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
		user.Timezone = &req.Timezone
	}

	if err := s.repos.User().Update(ctx, user); err != nil {
		return nil, err
	}

	activity.RecordWithLog(ctx, "auth", "updated", userID, user, "User profile was updated")

	return user, nil
}

// ChangePassword changes a user's password
func (s *UserService) ChangePassword(ctx context.Context, userID string, req *dto.ChangePasswordRequest) error {
	user, err := s.repos.User().FindByID(ctx, userID)
	if err != nil {
		return err
	}

	if user == nil {
		return fiberutil.NotFound()
	}

	if !security.VerifyPassword(user.Password, req.CurrentPassword) {
		return errors.New("current password is incorrect")
	}

	hashedPassword, err := security.HashPassword(req.Password)
	if err != nil {
		return err
	}

	user.Password = hashedPassword

	if err := s.repos.User().Update(ctx, user); err != nil {
		return err
	}

	activity.RecordWithLog(ctx, "auth", "password_changed", userID, user, "User password was changed")

	return nil
}

// DeleteAccount deletes a user's account
func (s *UserService) DeleteAccount(ctx context.Context, userID string) error {
	user, err := s.repos.User().FindByID(ctx, userID)
	if err != nil {
		return err
	}

	if user == nil {
		return fiberutil.NotFound()
	}

	activity.RecordWithLog(ctx, "auth", "deleted", userID, user, "User account was deleted")

	// Delete owned teams
	ownedTeams, err := s.repos.Team().GetUserTeams(ctx, userID)
	if err != nil {
		return err
	}

	for _, team := range ownedTeams {
		if team.UserID == userID {
			s.repos.Team().Delete(ctx, team.ID)
		}
	}

	return s.repos.User().Delete(ctx, userID)
}

// CheckUserStatus checks a user's status by email
func (s *UserService) CheckUserStatus(ctx context.Context, email string) (*dto.UserStatusResponse, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	user, err := s.repos.User().FindByEmail(ctx, email)
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
