package services

import (
	"context"
	"encoding/base32"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pquerna/otp/totp"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/auth/repositories"
	apperrors "github.com/kkz6/launch-go/internal/pkg/errors"
)

// TwoFactorService handles two-factor authentication operations
type TwoFactorService struct {
	repo   *repositories.Repository
	config *config.Config
}

// NewTwoFactorService creates a new TwoFactorService instance
func NewTwoFactorService(repo *repositories.Repository, cfg *config.Config) *TwoFactorService {
	return &TwoFactorService{
		repo:   repo,
		config: cfg,
	}
}

// EnableTwoFactor initiates 2FA setup
func (s *TwoFactorService) EnableTwoFactor(ctx context.Context, userID string) (*dto.TwoFactorResponse, error) {
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

	return &dto.TwoFactorResponse{
		QRCodeURL: key.URL(),
		SecretKey: key.Secret(),
	}, nil
}

// ConfirmTwoFactor confirms 2FA setup
func (s *TwoFactorService) ConfirmTwoFactor(ctx context.Context, userID, code string) error {
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
func (s *TwoFactorService) DisableTwoFactor(ctx context.Context, userID, password string) error {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return err
	}

	if user == nil {
		return apperrors.ErrNotFound
	}

	// Verify password
	if err := verifyPassword(user.Password, password); err != nil {
		return errors.New("invalid password")
	}

	user.TwoFactorSecret = nil
	user.TwoFactorConfirmedAt = nil
	user.TwoFactorRecoveryCodes = nil

	return s.repo.UpdateUser(ctx, user)
}

// VerifyTwoFactor verifies a 2FA code
func (s *TwoFactorService) VerifyTwoFactor(ctx context.Context, userID, code string) (bool, error) {
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
func (s *TwoFactorService) GetRecoveryCodes(ctx context.Context, userID string) ([]string, error) {
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
func (s *TwoFactorService) RegenerateRecoveryCodes(ctx context.Context, userID string) ([]string, error) {
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
func (s *TwoFactorService) HasTwoFactorEnabled(ctx context.Context, userID string) (bool, error) {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return false, err
	}

	if user == nil {
		return false, nil
	}

	return user.HasEnabledTwoFactorAuthentication(), nil
}

// generateRecoveryCodes generates random recovery codes
func (s *TwoFactorService) generateRecoveryCodes() ([]string, error) {
	codes := make([]string, 8)
	for i := range codes {
		token, err := models.GenerateToken(8)
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
