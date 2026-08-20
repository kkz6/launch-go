package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/auth/contracts"
	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/i18n"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
	"github.com/kkz6/launch-go/internal/pkg/security"
)

// UserService handles user management operations
type UserService struct {
	repos contracts.RepositoryRegistry
}

// NewUserService creates a new UserService instance
func NewUserService(repos contracts.RepositoryRegistry) *UserService {
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

// UpdateLocale persists a user's explicit locale. Automatic mode is stored as
// NULL so each request can continue to follow Accept-Language.
func (s *UserService) UpdateLocale(ctx context.Context, userID string, req *dto.UpdateLocaleRequest) (*models.User, error) {
	req.Normalize()
	if req.Locale != i18n.LocaleAuto && req.Locale != i18n.LocaleEnglish && req.Locale != i18n.LocaleJapanese {
		return nil, fiberutil.NewValidationError(map[string][]string{
			"locale": {"Must be one of: auto en ja"},
		})
	}

	user, err := s.repos.User().FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, fiberutil.NotFound()
	}

	user.Locale = nil
	if req.Locale != i18n.LocaleAuto {
		locale := req.Locale
		user.Locale = &locale
	}
	if err := s.repos.User().Update(ctx, user); err != nil {
		return nil, err
	}

	activity.RecordWithLog(ctx, "auth", "locale_updated", userID, user, "User locale was updated")
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

// DeleteAccount deletes a user's account after verifying their password
func (s *UserService) DeleteAccount(ctx context.Context, userID, password string) error {
	user, err := s.repos.User().FindByID(ctx, userID)
	if err != nil {
		return err
	}

	if user == nil {
		return fiberutil.NotFound()
	}

	// Verify password before destructive action
	if !security.VerifyPassword(user.Password, password) {
		return errors.New("invalid password")
	}

	// Delete owned teams
	ownedTeams, err := s.repos.Team().GetUserTeams(ctx, userID)
	if err != nil {
		return err
	}

	err = s.repos.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, team := range ownedTeams {
			if team.UserID != userID {
				continue
			}

			// Delete team members
			if err := tx.Where("team_id = ?", team.ID).Delete(&models.TeamMember{}).Error; err != nil {
				return fmt.Errorf("failed to delete team members for team %s: %w", team.ID, err)
			}

			// Delete team invitations
			if err := tx.Where("team_id = ?", team.ID).Delete(&models.TeamInvitation{}).Error; err != nil {
				return fmt.Errorf("failed to delete team invitations for team %s: %w", team.ID, err)
			}

			// Clear current_team_id for users referencing this team
			if err := tx.Model(&models.User{}).
				Where("current_team_id = ?", team.ID).
				Update("current_team_id", nil).Error; err != nil {
				return fmt.Errorf("failed to clear current team references for team %s: %w", team.ID, err)
			}

			// Delete the team
			if err := tx.Delete(&models.Team{}, "id = ?", team.ID).Error; err != nil {
				return fmt.Errorf("failed to delete team %s: %w", team.ID, err)
			}
		}

		return tx.Delete(&models.User{}, "id = ?", userID).Error
	})
	if err != nil {
		return err
	}

	activity.RecordWithLog(ctx, "auth", "deleted", userID, user, "User account was deleted")

	return nil
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
			HasPasskeys:          false,
			PasskeyCount:         0,
		}, nil
	}

	passkeyCount, _ := s.repos.Passkey().CountByUserID(ctx, user.ID)

	return &dto.UserStatusResponse{
		UserExists:           true,
		RequiresVerification: !user.HasVerifiedEmail(),
		HasTwoFactor:         user.HasEnabledTwoFactorAuthentication(),
		HasPasskeys:          passkeyCount > 0,
		PasskeyCount:         int(passkeyCount),
	}, nil
}
